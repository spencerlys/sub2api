//go:build unit

package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 上游 usage 带嵌套 cache_creation（5m/1h 明细）：经 OpenAI 格式入口转发后，usage_log
// 必须保留 5m/1h 拆分，缓存写入按各自 TTL 单价计费，而不是全部回退为 5m 单价。
const compatCacheTTLUpstreamStream = "event: message_start\n" +
	`data: {"type":"message_start","message":{"id":"msg_ttl","type":"message","role":"assistant","content":[],"model":"claude-sonnet-5-5","usage":{"input_tokens":1000,"output_tokens":1,"cache_creation_input_tokens":3000,"cache_read_input_tokens":4000,"cache_creation":{"ephemeral_5m_input_tokens":1000,"ephemeral_1h_input_tokens":2000}}}}` + "\n\n" +
	"event: content_block_start\n" +
	`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
	"event: content_block_delta\n" +
	`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}` + "\n\n" +
	"event: content_block_stop\n" +
	`data: {"type":"content_block_stop","index":0}` + "\n\n" +
	"event: message_delta\n" +
	`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2000}}` + "\n\n" +
	"event: message_stop\n" +
	`data: {"type":"message_stop"}` + "\n\n"

func runCompatCacheTTLUsage(t *testing.T, endpoint string, stream bool) *service.UsageLog {
	t.Helper()
	cfg := &config.Config{}
	groupID := int64(9400)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
	account := &service.Account{
		ID: 9401, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials:   map[string]any{"api_key": "test-key"},
		AccountGroups: []service.AccountGroup{{AccountID: 9401, GroupID: groupID}},
	}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
	billingCache := service.NewBillingCacheService(newHandlerInflightCache(100), nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	upstream := &compatStreamUsageUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(compatCacheTTLUpstreamStream)),
		Header:     http.Header{"X-Request-Id": []string{"compat-cache-ttl"}},
	}}
	gw := service.NewGatewayService(
		nil, &fakeGroupRepo{group: group}, usageRepo, &compatStreamUsageBillingRepo{}, nil, nil, nil, nil, cfg,
		snapshot, nil, service.NewBillingService(cfg, nil), nil, nil, nil, upstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	h := &GatewayHandler{
		gatewayService: gw, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1,
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
	}

	streamField := "false"
	if stream {
		streamField = "true"
	}
	body := `{"model":"claude-sonnet-5-5","stream":` + streamField + `,"input":"hello"}`
	if endpoint == "chat/completions" {
		body = `{"model":"claude-sonnet-5-5","stream":` + streamField + `,"messages":[{"role":"user","content":"hello"}]}`
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	apiKey := &service.APIKey{ID: 9402, UserID: 9403, GroupID: &groupID, Group: group, User: &service.User{ID: 9403, Balance: 100}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9403})

	if endpoint == "chat/completions" {
		h.ChatCompletions(c)
	} else {
		h.Responses(c)
	}
	require.Equal(t, 1, upstream.calls, rec.Body.String())

	select {
	case log := <-usageRepo.created:
		return log
	case <-time.After(5 * time.Second):
		t.Fatalf("usage was not recorded: %s", rec.Body.String())
		return nil
	}
}

func TestGatewayCompatibilityHandlers_CacheCreationTTLBreakdownBilled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"chat/completions", "responses"} {
		for _, stream := range []bool{true, false} {
			name := endpoint + "/stream"
			if !stream {
				name = endpoint + "/non_stream"
			}
			t.Run(name, func(t *testing.T) {
				log := runCompatCacheTTLUsage(t, endpoint, stream)

				require.Equal(t, 3000, log.CacheCreationTokens)
				require.Equal(t, 1000, log.CacheCreation5mTokens)
				require.Equal(t, 2000, log.CacheCreation1hTokens)
				require.InDelta(t, 1000*2.5e-6+2000*4e-6, log.CacheCreationCost, 1e-12, "1h 部分必须按 1h 单价计费")
			})
		}
	}
}
