//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type proxyChannelMonitorRepoStub struct {
	service.ChannelMonitorRepository
	monitors map[int64]*service.ChannelMonitor
	nextID   int64
}

func (r *proxyChannelMonitorRepoStub) Create(_ context.Context, m *service.ChannelMonitor) error {
	r.nextID++
	m.ID = r.nextID
	stored := *m
	r.monitors[m.ID] = &stored
	return nil
}

func (r *proxyChannelMonitorRepoStub) GetByID(_ context.Context, id int64) (*service.ChannelMonitor, error) {
	m, ok := r.monitors[id]
	if !ok {
		return nil, service.ErrChannelMonitorNotFound
	}
	clone := *m
	return &clone, nil
}

func (r *proxyChannelMonitorRepoStub) Update(_ context.Context, m *service.ChannelMonitor) error {
	stored := *m
	r.monitors[m.ID] = &stored
	return nil
}

type proxyChannelMonitorProxyStub struct {
	proxies map[int64]*service.Proxy
}

func (s *proxyChannelMonitorProxyStub) GetByID(_ context.Context, id int64) (*service.Proxy, error) {
	if p, ok := s.proxies[id]; ok {
		return p, nil
	}
	return nil, service.ErrProxyNotFound
}

func (s *proxyChannelMonitorProxyStub) ListByIDs(_ context.Context, ids []int64) ([]service.Proxy, error) {
	out := make([]service.Proxy, 0, len(ids))
	for _, id := range ids {
		if p, ok := s.proxies[id]; ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

func setupProxyChannelMonitorRouter(t *testing.T) (*gin.Engine, *proxyChannelMonitorRepoStub) {
	t.Helper()
	repo := &proxyChannelMonitorRepoStub{monitors: map[int64]*service.ChannelMonitor{}}
	svc := service.NewChannelMonitorService(repo, duplicateChannelMonitorHandlerEncryptor{})
	svc.SetProxyRepository(&proxyChannelMonitorProxyStub{proxies: map[int64]*service.Proxy{
		5: {ID: 5, Name: "sg-egress"},
		6: {ID: 6, Name: "us-egress"},
	}})
	handler := NewChannelMonitorHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 1})
		c.Next()
	})
	router.POST("/api/v1/admin/channel-monitors", handler.Create)
	router.PUT("/api/v1/admin/channel-monitors/:id", handler.Update)
	return router, repo
}

type proxyMonitorEnvelope struct {
	Code int `json:"code"`
	Data struct {
		ID        int64  `json:"id"`
		ProxyID   *int64 `json:"proxy_id"`
		ProxyName string `json:"proxy_name"`
	} `json:"data"`
}

func doProxyMonitorRequest(t *testing.T, router *gin.Engine, method, path, body string) (int, proxyMonitorEnvelope, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var env proxyMonitorEnvelope
	if rec.Code < 300 {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	}
	return rec.Code, env, rec.Body.String()
}

const proxyMonitorCreateBody = `{
	"name": "m",
	"provider": "openai",
	"endpoint": "https://8.8.8.8",
	"api_key": "sk-test",
	"primary_model": "gpt-test",
	"interval_seconds": 60%s
}`

func TestChannelMonitorHandler_CreateWithProxyID(t *testing.T) {
	router, repo := setupProxyChannelMonitorRouter(t)

	code, env, raw := doProxyMonitorRequest(t, router, http.MethodPost, "/api/v1/admin/channel-monitors",
		strings.Replace(proxyMonitorCreateBody, "%s", `, "proxy_id": 5`, 1))
	require.Equal(t, http.StatusCreated, code, raw)
	require.NotNil(t, env.Data.ProxyID)
	require.Equal(t, int64(5), *env.Data.ProxyID)
	require.Equal(t, "sg-egress", env.Data.ProxyName)
	require.Equal(t, int64(5), *repo.monitors[env.Data.ID].ProxyID)

	// 不存在的代理 → 400。
	code, _, raw = doProxyMonitorRequest(t, router, http.MethodPost, "/api/v1/admin/channel-monitors",
		strings.Replace(proxyMonitorCreateBody, "%s", `, "proxy_id": 404`, 1))
	require.Equal(t, http.StatusBadRequest, code, raw)
	require.Contains(t, raw, "CHANNEL_MONITOR_PROXY_NOT_FOUND")

	// 缺省 / null → 不指定，响应里 proxy_id 为 null。
	code, env, raw = doProxyMonitorRequest(t, router, http.MethodPost, "/api/v1/admin/channel-monitors",
		strings.Replace(proxyMonitorCreateBody, "%s", `, "proxy_id": null`, 1))
	require.Equal(t, http.StatusCreated, code, raw)
	require.Nil(t, env.Data.ProxyID)
	require.Contains(t, raw, `"proxy_id":null`)
	require.Contains(t, raw, `"proxy_name":""`)
}

func TestChannelMonitorHandler_UpdateProxyIDTriState(t *testing.T) {
	router, repo := setupProxyChannelMonitorRouter(t)
	code, env, raw := doProxyMonitorRequest(t, router, http.MethodPost, "/api/v1/admin/channel-monitors",
		strings.Replace(proxyMonitorCreateBody, "%s", `, "proxy_id": 5`, 1))
	require.Equal(t, http.StatusCreated, code, raw)
	path := "/api/v1/admin/channel-monitors/" + jsonInt(env.Data.ID)

	// 缺省 = 不更新。
	code, env, raw = doProxyMonitorRequest(t, router, http.MethodPut, path, `{"name":"renamed"}`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, int64(5), *env.Data.ProxyID)
	require.Equal(t, "sg-egress", env.Data.ProxyName)

	// 切换。
	code, env, raw = doProxyMonitorRequest(t, router, http.MethodPut, path, `{"proxy_id":6}`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, int64(6), *env.Data.ProxyID)
	require.Equal(t, "us-egress", env.Data.ProxyName)

	// 不存在 → 400，原值保留。
	code, _, raw = doProxyMonitorRequest(t, router, http.MethodPut, path, `{"proxy_id":404}`)
	require.Equal(t, http.StatusBadRequest, code, raw)
	require.Equal(t, int64(6), *repo.monitors[env.Data.ID].ProxyID)

	// null = 清空。
	code, env, raw = doProxyMonitorRequest(t, router, http.MethodPut, path, `{"proxy_id":null}`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Nil(t, env.Data.ProxyID)
	require.Nil(t, repo.monitors[env.Data.ID].ProxyID)

	// 0 同样清空。
	code, env, raw = doProxyMonitorRequest(t, router, http.MethodPut, path, `{"proxy_id":5}`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, int64(5), *env.Data.ProxyID)
	code, env, raw = doProxyMonitorRequest(t, router, http.MethodPut, path, `{"proxy_id":0}`)
	require.Equal(t, http.StatusOK, code, raw)
	require.Nil(t, env.Data.ProxyID)
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
