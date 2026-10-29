package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
)

// 渠道监控探活代理：
//   - 部署环境出网只允许经由已配置的代理，直连探活恒失败；
//   - 有效代理 = monitor.ProxyID；为空时回退到关联账号的代理；都没有则直连
//     （直连保持原 safeDialContext 行为不变）；
//   - 配额模式不走这里（账号侧用量服务自带账号代理）。

// monitorProxySource 代理加载（ProxyRepository 天然满足）。
type monitorProxySource interface {
	GetByID(ctx context.Context, id int64) (*Proxy, error)
	ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error)
}

// monitorProxyClientCacheMax 代理 client 缓存上限（按 timeout+proxyURL 分键）。
// 代理数量通常个位数，超限时整体清空重建即可，无需 LRU。
const monitorProxyClientCacheMax = 64

var (
	monitorProxyClientMu    sync.Mutex
	monitorProxyClientCache = make(map[string]*http.Client)
)

// resolveMonitorProbeProxy 解析一次探活应使用的代理，返回 nil 表示直连。
//
// 优先级：monitor.ProxyID > 关联账号的代理 > 直连。
// 显式指定或账号引用的代理加载失败时返回 error（fail-closed：不静默退回直连，
// 否则在只允许代理出网的环境里会产生误导性的「上游不可达」，且可能暴露出口 IP）。
func resolveMonitorProbeProxy(
	ctx context.Context,
	m *ChannelMonitor,
	proxies monitorProxySource,
	accounts monitorAccountSource,
) (*Proxy, error) {
	if m == nil {
		return nil, nil
	}
	if m.ProxyID != nil && *m.ProxyID > 0 {
		return loadMonitorProxy(ctx, proxies, *m.ProxyID)
	}
	if m.AccountID == nil || *m.AccountID <= 0 || accounts == nil {
		return nil, nil
	}
	account, err := accounts.GetByID(ctx, *m.AccountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("load linked account %d for proxy: %w", *m.AccountID, monitorProxyLoadErr(err))
	}
	if account.ProxyID == nil || *account.ProxyID <= 0 {
		return nil, nil
	}
	if account.Proxy != nil {
		return account.Proxy, nil
	}
	return loadMonitorProxy(ctx, proxies, *account.ProxyID)
}

func loadMonitorProxy(ctx context.Context, proxies monitorProxySource, id int64) (*Proxy, error) {
	if proxies == nil {
		return nil, fmt.Errorf("proxy %d: proxy repository is not configured", id)
	}
	proxy, err := proxies.GetByID(ctx, id)
	if err != nil || proxy == nil {
		return nil, fmt.Errorf("load proxy %d: %w", id, monitorProxyLoadErr(err))
	}
	return proxy, nil
}

func monitorProxyLoadErr(err error) error {
	if err != nil {
		return err
	}
	return ErrProxyNotFound
}

// monitorProxyURL 返回代理 URL（nil = 直连 = 空串）。
func monitorProxyURL(p *Proxy) string {
	if p == nil {
		return ""
	}
	return p.URL()
}

// monitorRequestClient 返回检测请求用的 client：空 proxyURL 走共享的 SSRF 安全直连 client。
func monitorRequestClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return monitorHTTPClient, nil
	}
	return monitorProxiedHTTPClient(monitorRequestTimeout, proxyURL)
}

// monitorPingClient 返回 origin HEAD ping 用的 client（超时更短）。
func monitorPingClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return monitorPingHTTPClient, nil
	}
	return monitorProxiedHTTPClient(monitorPingTimeout, proxyURL)
}

// monitorProxiedHTTPClient 按 (timeout, proxyURL) 缓存经代理出网的 client。
//
// 与直连 client 的差异只有拨号路径：代理本身常在内网 / 回环（如本机 sidecar、
// Tailscale 地址），不能套 safeDialContext，因此 HTTP(S) 代理用普通 dialer 连代理、
// SOCKS5 由 ConfigureTransportProxy 接管 DialContext。目标 endpoint 仍由
// validateEndpoint 在提交时校验（公网 + https），超时参数与直连完全一致。
func monitorProxiedHTTPClient(timeout time.Duration, proxyURL string) (*http.Client, error) {
	key := strconv.FormatInt(int64(timeout), 10) + "|" + proxyURL
	monitorProxyClientMu.Lock()
	defer monitorProxyClientMu.Unlock()
	if c, ok := monitorProxyClientCache[key]; ok {
		return c, nil
	}

	_, parsed, err := proxyurl.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		DialContext:           monitorDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       monitorIdleConnTimeout,
		TLSHandshakeTimeout:   monitorTLSHandshakeTimeout,
		ResponseHeaderTimeout: monitorResponseHeaderTimeout,
	}
	if err := proxyutil.ConfigureTransportProxy(tr, parsed); err != nil {
		return nil, err
	}
	c := &http.Client{Timeout: timeout, Transport: servertiming.WrapRoundTripper(tr)}

	if len(monitorProxyClientCache) >= monitorProxyClientCacheMax {
		for _, old := range monitorProxyClientCache {
			old.CloseIdleConnections()
		}
		monitorProxyClientCache = make(map[string]*http.Client)
	}
	monitorProxyClientCache[key] = c
	return c, nil
}

// fillProxyNames 按 ProxyID 批量回填 ProxyName（仅响应展示用）。
// 查询失败只降级为空名称，不影响主流程。
func (s *ChannelMonitorService) fillProxyNames(ctx context.Context, monitors ...*ChannelMonitor) {
	if s == nil || s.proxies == nil {
		return
	}
	ids := make([]int64, 0, len(monitors))
	seen := make(map[int64]struct{}, len(monitors))
	for _, m := range monitors {
		if m == nil || m.ProxyID == nil {
			continue
		}
		if _, ok := seen[*m.ProxyID]; ok {
			continue
		}
		seen[*m.ProxyID] = struct{}{}
		ids = append(ids, *m.ProxyID)
	}
	if len(ids) == 0 {
		return
	}
	rows, err := s.proxies.ListByIDs(ctx, ids)
	if err != nil {
		return
	}
	names := make(map[int64]string, len(rows))
	for _, p := range rows {
		names[p.ID] = p.Name
	}
	for _, m := range monitors {
		if m != nil && m.ProxyID != nil {
			m.ProxyName = names[*m.ProxyID]
		}
	}
}

// validateMonitorProxy 校验显式指定的代理存在。nil / <= 0 表示不指定，直接放行。
func (s *ChannelMonitorService) validateMonitorProxy(ctx context.Context, proxyID *int64) error {
	if proxyID == nil || *proxyID <= 0 {
		return nil
	}
	if s.proxies == nil {
		return ErrChannelMonitorProxyNotFound
	}
	proxy, err := s.proxies.GetByID(ctx, *proxyID)
	if err != nil || proxy == nil {
		return ErrChannelMonitorProxyNotFound
	}
	return nil
}

// normalizeMonitorProxyID 把 <= 0 归一为 nil（不指定）。
func normalizeMonitorProxyID(id *int64) *int64 {
	if id == nil || *id <= 0 {
		return nil
	}
	v := *id
	return &v
}

// SetProxyRepository 由 wire 注入代理仓储（探活代理解析 + proxy_name 回填 + 校验）。
// 未注入时：显式 proxy_id 的创建/更新被拒绝，探活遇到 proxy_id 时 fail-closed 报错。
func (s *ChannelMonitorService) SetProxyRepository(repo monitorProxySource) {
	if s == nil {
		return
	}
	s.proxies = repo
}

// probeAccountSource 探活代理回退所需的账号加载源（复用配额抓取器的账号仓储）。
func (s *ChannelMonitorService) probeAccountSource() monitorAccountSource {
	if s == nil || s.quotaFetcher == nil {
		return nil
	}
	return s.quotaFetcher.accounts
}

// monitorProxyErrorResults 代理解析失败时为每个模型产出 error 结果
// （保持检测周期与历史时间线连续，与其它检测失败的处理一致）。
func monitorProxyErrorResults(models []string, err error) []*CheckResult {
	now := time.Now()
	msg := truncateMessage(sanitizeErrorMessage("resolve probe proxy: " + err.Error()))
	results := make([]*CheckResult, 0, len(models))
	for _, model := range models {
		results = append(results, &CheckResult{
			Model:     model,
			Status:    MonitorStatusError,
			Message:   msg,
			CheckedAt: now,
		})
	}
	return results
}
