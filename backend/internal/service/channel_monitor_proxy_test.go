//go:build unit

package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- stubs ---

type monitorProxySourceStub struct {
	proxies map[int64]*Proxy
	err     error
}

func (s *monitorProxySourceStub) GetByID(_ context.Context, id int64) (*Proxy, error) {
	if s.err != nil {
		return nil, s.err
	}
	if p, ok := s.proxies[id]; ok {
		clone := *p
		return &clone, nil
	}
	return nil, ErrProxyNotFound
}

func (s *monitorProxySourceStub) ListByIDs(_ context.Context, ids []int64) ([]Proxy, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]Proxy, 0, len(ids))
	for _, id := range ids {
		if p, ok := s.proxies[id]; ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

type monitorProxyAccountSourceStub struct {
	accounts map[int64]*Account
	err      error
}

func (s *monitorProxyAccountSourceStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	if a, ok := s.accounts[id]; ok {
		return a, nil
	}
	return nil, ErrAccountNotFound
}

// proxyMonitorRepoStub 内存版 ChannelMonitorRepository（Create/GetByID/Update + 落库记录）。
type proxyMonitorRepoStub struct {
	ChannelMonitorRepository
	monitors map[int64]*ChannelMonitor
	nextID   int64
	history  []*ChannelMonitorHistoryRow
}

func (r *proxyMonitorRepoStub) Create(_ context.Context, m *ChannelMonitor) error {
	r.nextID++
	m.ID = r.nextID
	clone := *m
	if r.monitors == nil {
		r.monitors = make(map[int64]*ChannelMonitor)
	}
	r.monitors[m.ID] = &clone
	return nil
}

func (r *proxyMonitorRepoStub) GetByID(_ context.Context, id int64) (*ChannelMonitor, error) {
	m, ok := r.monitors[id]
	if !ok {
		return nil, ErrChannelMonitorNotFound
	}
	clone := *m
	return &clone, nil
}

func (r *proxyMonitorRepoStub) Update(_ context.Context, m *ChannelMonitor) error {
	clone := *m
	r.monitors[m.ID] = &clone
	return nil
}

func (r *proxyMonitorRepoStub) InsertHistoryBatch(_ context.Context, rows []*ChannelMonitorHistoryRow) error {
	r.history = append(r.history, rows...)
	return nil
}

func (r *proxyMonitorRepoStub) MarkChecked(context.Context, int64, time.Time) error { return nil }

// recordingHTTPProxy 一个最小 HTTP 正向代理：记录收到的 (method, host)，
// 直接以 200 + JSON 应答（不再转发），用于证明请求确实经过了代理。
type recordingHTTPProxy struct {
	server *httptest.Server
	mu     sync.Mutex
	seen   []string
}

func newRecordingHTTPProxy(t *testing.T) *recordingHTTPProxy {
	t.Helper()
	p := &recordingHTTPProxy{}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.seen = append(p.seen, r.Method+" "+r.URL.Host)
		p.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *recordingHTTPProxy) proxy(id int64) *Proxy {
	host, portStr, _ := net.SplitHostPort(p.server.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return &Proxy{ID: id, Name: "egress-" + strconv.FormatInt(id, 10), Protocol: "http", Host: host, Port: port, Status: StatusActive}
}

func (p *recordingHTTPProxy) requests() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.seen...)
}

// --- resolveMonitorProbeProxy ---

func TestResolveMonitorProbeProxy_Matrix(t *testing.T) {
	monitorProxy := &Proxy{ID: 1, Name: "monitor", Protocol: "http", Host: "10.0.0.1", Port: 3128}
	accountProxy := &Proxy{ID: 2, Name: "account", Protocol: "socks5", Host: "10.0.0.2", Port: 1080}
	proxies := &monitorProxySourceStub{proxies: map[int64]*Proxy{1: monitorProxy, 2: accountProxy}}
	accounts := &monitorProxyAccountSourceStub{accounts: map[int64]*Account{
		10: {ID: 10, ProxyID: int64Ptr(2), Proxy: accountProxy},
		11: {ID: 11, ProxyID: int64Ptr(2)}, // Proxy 未联查：需回查代理仓储
		12: {ID: 12},                       // 账号无代理
	}}

	cases := []struct {
		name    string
		monitor *ChannelMonitor
		wantID  int64 // 0 = 直连
		wantErr bool
	}{
		{name: "nil monitor is direct", monitor: nil},
		{name: "no proxy no account is direct", monitor: &ChannelMonitor{}},
		{name: "monitor proxy wins over account proxy", monitor: &ChannelMonitor{ProxyID: int64Ptr(1), AccountID: int64Ptr(10)}, wantID: 1},
		{name: "falls back to linked account proxy", monitor: &ChannelMonitor{AccountID: int64Ptr(10)}, wantID: 2},
		{name: "account proxy loaded via repo when not joined", monitor: &ChannelMonitor{AccountID: int64Ptr(11)}, wantID: 2},
		{name: "account without proxy is direct", monitor: &ChannelMonitor{AccountID: int64Ptr(12)}},
		{name: "zero proxy id treated as unset", monitor: &ChannelMonitor{ProxyID: int64Ptr(0), AccountID: int64Ptr(12)}},
		{name: "missing monitor proxy fails closed", monitor: &ChannelMonitor{ProxyID: int64Ptr(99)}, wantErr: true},
		{name: "missing linked account fails closed", monitor: &ChannelMonitor{AccountID: int64Ptr(404)}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMonitorProbeProxy(context.Background(), tc.monitor, proxies, accounts)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			if tc.wantID == 0 {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tc.wantID, got.ID)
		})
	}
}

func TestResolveMonitorProbeProxy_NilSourcesFailClosedForExplicitProxy(t *testing.T) {
	_, err := resolveMonitorProbeProxy(context.Background(), &ChannelMonitor{ProxyID: int64Ptr(1)}, nil, nil)
	require.Error(t, err)

	// 账号源未注入（测试 / 无配额抓取器）时，仅关联账号的监控按直连处理。
	got, err := resolveMonitorProbeProxy(context.Background(), &ChannelMonitor{AccountID: int64Ptr(1)}, nil, nil)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestResolveMonitorProbeProxy_AccountLoadErrorFailsClosed(t *testing.T) {
	accounts := &monitorProxyAccountSourceStub{err: errors.New("db down")}
	_, err := resolveMonitorProbeProxy(context.Background(), &ChannelMonitor{AccountID: int64Ptr(1)}, nil, accounts)
	require.ErrorContains(t, err, "db down")
}

// --- client 选择 ---

func TestMonitorClients_DirectUsesSharedSSRFSafeClients(t *testing.T) {
	c, err := monitorRequestClient("")
	require.NoError(t, err)
	require.Same(t, monitorHTTPClient, c)

	p, err := monitorPingClient("")
	require.NoError(t, err)
	require.Same(t, monitorPingHTTPClient, p)
}

func TestMonitorClients_ProxiedClientsAreCachedWithIdenticalTimeouts(t *testing.T) {
	const proxyURL = "http://user:pass@10.1.2.3:3128"

	c1, err := monitorRequestClient(proxyURL)
	require.NoError(t, err)
	c2, err := monitorRequestClient(proxyURL)
	require.NoError(t, err)
	require.Same(t, c1, c2, "same proxy URL must reuse the cached client")
	require.NotSame(t, monitorHTTPClient, c1)
	require.Equal(t, monitorHTTPClient.Timeout, c1.Timeout)

	p1, err := monitorPingClient(proxyURL)
	require.NoError(t, err)
	require.NotSame(t, c1, p1, "ping client keeps its shorter timeout")
	require.Equal(t, monitorPingHTTPClient.Timeout, p1.Timeout)

	s1, err := monitorRequestClient("socks5://10.1.2.4:1080")
	require.NoError(t, err)
	require.NotSame(t, c1, s1)
}

func TestMonitorClients_InvalidProxyURLErrors(t *testing.T) {
	_, err := monitorRequestClient("ftp://10.1.2.3:21")
	require.Error(t, err)
	_, err = monitorPingClient("ftp://10.1.2.3:21")
	require.Error(t, err)
}

func TestMonitorClients_CacheIsBounded(t *testing.T) {
	for i := 0; i < monitorProxyClientCacheMax+5; i++ {
		_, err := monitorRequestClient("http://10.9.0.1:" + strconv.Itoa(20000+i))
		require.NoError(t, err)
	}
	monitorProxyClientMu.Lock()
	size := len(monitorProxyClientCache)
	monitorProxyClientMu.Unlock()
	require.LessOrEqual(t, size, monitorProxyClientCacheMax)
}

// 经代理时检测请求与 HEAD ping 都打到代理；直连路径仍拒绝回环（safeDialContext 未变）。
func TestMonitorRequestsGoThroughProxy_CheckAndPing(t *testing.T) {
	proxy := newRecordingHTTPProxy(t)
	proxyURL := proxy.proxy(1).URL()
	const endpoint = "http://upstream.monitor-proxy.test"

	_, _, status, err := callProvider(context.Background(), MonitorProviderOpenAI, endpoint,
		"sk-test", "gpt-test", "hello", &CheckOptions{ProxyURL: proxyURL})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	ping := pingEndpointOrigin(context.Background(), endpoint, proxyURL)
	require.NotNil(t, ping)

	require.Equal(t, []string{
		"POST upstream.monitor-proxy.test",
		"HEAD upstream.monitor-proxy.test",
	}, proxy.requests())

	// 直连 ping 回环地址仍被 SSRF dialer 拒绝（行为与改动前一致）。
	require.Nil(t, pingEndpointOrigin(context.Background(), proxy.server.URL, ""))
}

// --- RunCheck 集成 ---

func newProxyMonitorService(repo *proxyMonitorRepoStub, proxies monitorProxySource) *ChannelMonitorService {
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(channelMonitorRuntimeStub{rt: ChannelMonitorRuntime{
		Enabled: true,
		Mode:    ChannelMonitorModeV1,
	}})
	svc.SetProxyRepository(proxies)
	return svc
}

func TestRunCheck_ProbeUsesMonitorProxy(t *testing.T) {
	proxy := newRecordingHTTPProxy(t)
	repo := &proxyMonitorRepoStub{monitors: map[int64]*ChannelMonitor{
		1: {
			ID:              1,
			Name:            "via-proxy",
			Provider:        MonitorProviderOpenAI,
			APIMode:         MonitorAPIModeChatCompletions,
			Endpoint:        "http://upstream.monitor-proxy.test",
			APIKey:          "NEW:sk-test",
			PrimaryModel:    "gpt-test",
			ExtraModels:     []string{"gpt-extra"},
			Enabled:         true,
			IntervalSeconds: 60,
			ProxyID:         int64Ptr(7),
		},
	}}
	svc := newProxyMonitorService(repo, &monitorProxySourceStub{proxies: map[int64]*Proxy{7: proxy.proxy(7)}})

	results, err := svc.RunCheck(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, r := range results {
		require.NotEqual(t, MonitorStatusError, r.Status, r.Message)
		require.NotNil(t, r.PingLatencyMs)
	}
	require.ElementsMatch(t, []string{
		"HEAD upstream.monitor-proxy.test",
		"POST upstream.monitor-proxy.test",
		"POST upstream.monitor-proxy.test",
	}, proxy.requests())
}

func TestRunCheck_ProbeWithMissingProxyFailsClosed(t *testing.T) {
	repo := &proxyMonitorRepoStub{monitors: map[int64]*ChannelMonitor{
		1: {
			ID:              1,
			Provider:        MonitorProviderOpenAI,
			Endpoint:        "http://upstream.monitor-proxy.test",
			APIKey:          "NEW:sk-test",
			PrimaryModel:    "gpt-test",
			Enabled:         true,
			IntervalSeconds: 60,
			ProxyID:         int64Ptr(99),
		},
	}}
	svc := newProxyMonitorService(repo, &monitorProxySourceStub{})

	results, err := svc.RunCheck(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, MonitorStatusError, results[0].Status)
	require.Contains(t, results[0].Message, "resolve probe proxy")
	require.Len(t, repo.history, 1, "failure is still recorded to keep the timeline continuous")
}

// --- Create / Update / Duplicate ---

func TestCreateChannelMonitor_ProxyIDValidatedAndNamed(t *testing.T) {
	proxies := &monitorProxySourceStub{proxies: map[int64]*Proxy{3: {ID: 3, Name: "sg-egress"}}}
	repo := &proxyMonitorRepoStub{}
	svc := newProxyMonitorService(repo, proxies)
	params := ChannelMonitorCreateParams{
		Name:            "m",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://8.8.8.8",
		APIKey:          "sk-test",
		PrimaryModel:    "gpt-test",
		Enabled:         true,
		IntervalSeconds: 60,
	}

	params.ProxyID = int64Ptr(404)
	_, err := svc.Create(context.Background(), params)
	require.ErrorIs(t, err, ErrChannelMonitorProxyNotFound)
	require.Empty(t, repo.monitors)

	params.ProxyID = int64Ptr(3)
	created, err := svc.Create(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, int64(3), *created.ProxyID)
	require.Equal(t, "sg-egress", created.ProxyName)
	require.Equal(t, int64(3), *repo.monitors[created.ID].ProxyID)

	params.ProxyID = int64Ptr(0)
	unset, err := svc.Create(context.Background(), params)
	require.NoError(t, err)
	require.Nil(t, unset.ProxyID)
	require.Empty(t, unset.ProxyName)
}

func TestUpdateChannelMonitor_ProxyIDSetKeepClear(t *testing.T) {
	proxies := &monitorProxySourceStub{proxies: map[int64]*Proxy{3: {ID: 3, Name: "sg-egress"}, 4: {ID: 4, Name: "us-egress"}}}
	repo := &proxyMonitorRepoStub{monitors: map[int64]*ChannelMonitor{
		1: {
			ID:              1,
			Name:            "m",
			Provider:        MonitorProviderOpenAI,
			Endpoint:        "https://8.8.8.8",
			APIKey:          "NEW:sk-test",
			PrimaryModel:    "gpt-test",
			IntervalSeconds: 60,
			ProxyID:         int64Ptr(3),
		},
	}}
	svc := newProxyMonitorService(repo, proxies)
	ctx := context.Background()

	// 未提供 = 不更新。
	name := "renamed"
	updated, err := svc.Update(ctx, 1, ChannelMonitorUpdateParams{Name: &name})
	require.NoError(t, err)
	require.Equal(t, int64(3), *updated.ProxyID)
	require.Equal(t, "sg-egress", updated.ProxyName)

	// 不存在的代理被拒绝，原值不变。
	_, err = svc.Update(ctx, 1, ChannelMonitorUpdateParams{ProxyID: int64Ptr(404)})
	require.ErrorIs(t, err, ErrChannelMonitorProxyNotFound)
	require.Equal(t, int64(3), *repo.monitors[1].ProxyID)

	// 切换代理。
	updated, err = svc.Update(ctx, 1, ChannelMonitorUpdateParams{ProxyID: int64Ptr(4)})
	require.NoError(t, err)
	require.Equal(t, int64(4), *updated.ProxyID)
	require.Equal(t, "us-egress", updated.ProxyName)

	// 0 = 清空。
	updated, err = svc.Update(ctx, 1, ChannelMonitorUpdateParams{ProxyID: int64Ptr(0)})
	require.NoError(t, err)
	require.Nil(t, updated.ProxyID)
	require.Empty(t, updated.ProxyName)
	require.Nil(t, repo.monitors[1].ProxyID)
}

func TestDuplicateChannelMonitor_CopiesProxyID(t *testing.T) {
	source := &ChannelMonitor{
		ID:              42,
		Name:            "primary",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://api.example.com",
		APIKey:          "OLD:top-secret",
		PrimaryModel:    "gpt-test",
		IntervalSeconds: 60,
		ProxyID:         int64Ptr(3),
	}
	repo := &duplicateChannelMonitorRepoStub{source: source}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetProxyRepository(&monitorProxySourceStub{proxies: map[int64]*Proxy{3: {ID: 3, Name: "sg-egress"}}})

	duplicate, err := svc.Duplicate(context.Background(), source.ID, 77, "admin:77", "copy-proxy")
	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.Equal(t, int64(3), *repo.created[0].ProxyID)
	require.Equal(t, "sg-egress", duplicate.ProxyName)

	*duplicate.ProxyID = 9
	require.Equal(t, int64(3), *source.ProxyID, "duplicate must not alias the source pointer")
}
