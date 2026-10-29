//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// 粘性账号等待超时 / 等待队列满：排除该账号重新选号一次，而不是直接 429。
func TestRecordAccountWaitFailure_ExcludesAccountOnce(t *testing.T) {
	fs := NewFailoverState(3, true)

	require.True(t, fs.RecordAccountWaitFailure(7, &ConcurrencyError{SlotType: "account", IsTimeout: true}))
	require.Contains(t, fs.FailedAccountIDs, int64(7))
	require.Error(t, fs.AccountWaitErr())
	require.Zero(t, fs.SwitchCount, "等待失败不消耗上游换号次数")

	require.False(t, fs.RecordAccountWaitFailure(8, &WaitQueueFullError{SlotType: "account"}), "每个请求只重选一次")
	require.NotContains(t, fs.FailedAccountIDs, int64(8))
}

func TestRecordAccountWaitFailure_QueueFullIsRetryable(t *testing.T) {
	fs := NewFailoverState(3, false)
	require.True(t, fs.RecordAccountWaitFailure(7, &WaitQueueFullError{SlotType: "account"}))
}

func TestRecordAccountWaitFailure_IgnoresClientCancel(t *testing.T) {
	fs := NewFailoverState(3, false)
	require.False(t, fs.RecordAccountWaitFailure(7, context.Canceled))
	require.Empty(t, fs.FailedAccountIDs)
	require.NoError(t, fs.AccountWaitErr())
}

// waitCountingConcurrencyCache 记录账号等待计数，并让账号槽位始终获取失败。
type waitCountingConcurrencyCache struct {
	fakeConcurrencyCache
	waitInc   int
	waitDec   int
	queueFull bool
}

func (f *waitCountingConcurrencyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return false, nil
}

func (f *waitCountingConcurrencyCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	if f.queueFull {
		return false, nil
	}
	f.waitInc++
	return true, nil
}

func (f *waitCountingConcurrencyCache) DecrementAccountWaitCount(context.Context, int64) error {
	f.waitDec++
	return nil
}

func newWaitSelection() *service.AccountSelectionResult {
	return &service.AccountSelectionResult{
		Account:  &service.Account{ID: 11},
		WaitPlan: &service.AccountWaitPlan{AccountID: 11, MaxConcurrency: 1, Timeout: 150 * time.Millisecond, MaxWaiting: 3},
	}
}

func TestAcquireSelectedAccountSlot_CountsWaitAndTimesOut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &waitCountingConcurrencyCache{}
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatComment, time.Hour)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	streamStarted := false

	release, err := acquireSelectedAccountSlot(c, helper, newWaitSelection(), true, &streamStarted, zap.NewNop())

	require.Nil(t, release)
	var concurrencyErr *ConcurrencyError
	require.ErrorAs(t, err, &concurrencyErr)
	require.True(t, concurrencyErr.IsTimeout)
	require.Equal(t, 1, cache.waitInc)
	require.Equal(t, 1, cache.waitDec, "等待结束必须归还等待计数")
}

func TestAcquireSelectedAccountSlot_QueueFull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &waitCountingConcurrencyCache{queueFull: true}
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatComment, time.Hour)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	streamStarted := false

	_, err := acquireSelectedAccountSlot(c, helper, newWaitSelection(), false, &streamStarted, zap.NewNop())

	var queueFullErr *WaitQueueFullError
	require.ErrorAs(t, err, &queueFullErr)
	require.Zero(t, cache.waitDec)
}

func TestAcquireSelectedAccountSlot_AlreadyAcquired(t *testing.T) {
	released := false
	selection := &service.AccountSelectionResult{Account: &service.Account{ID: 1}, Acquired: true, ReleaseFunc: func() { released = true }}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	streamStarted := false

	release, err := acquireSelectedAccountSlot(c, nil, selection, false, &streamStarted, zap.NewNop())
	require.NoError(t, err)
	release()
	require.True(t, released)
}

// OpenAI 格式入口等待槽位时只能写 SSE 注释 ping，不能写 Claude 格式的 data ping。
func TestConcurrencyHelperWithPingFormat_CommentPingForOpenAIClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &waitCountingConcurrencyCache{}
	base := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatClaude, 20*time.Millisecond)
	helper := base.withPingFormat(SSEPingFormatComment)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	streamStarted := false

	_, err := helper.AcquireAccountSlotWithWaitTimeout(c, 11, 1, 100*time.Millisecond, true, &streamStarted)
	require.Error(t, err)
	require.True(t, streamStarted)
	require.Contains(t, rec.Body.String(), ":\n\n")
	require.NotContains(t, rec.Body.String(), `"type": "ping"`)
	require.Equal(t, SSEPingFormatClaude, base.pingFormat, "原 helper 不受影响")
}

// 转发成功后 RPM 计数仅对设置了 base_rpm 的 Anthropic OAuth 账号生效。
func TestIncrementAccountRPMAfterForward(t *testing.T) {
	rpm := &countingRPMCache{}
	h := &GatewayHandler{gatewayService: service.NewGatewayService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, rpm, nil, nil, nil, nil, nil, nil, nil, nil)}

	h.incrementAccountRPMAfterForward(context.Background(), &service.Account{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Extra: map[string]any{"base_rpm": 10}}, zap.NewNop())
	h.incrementAccountRPMAfterForward(context.Background(), &service.Account{ID: 2, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Extra: map[string]any{"base_rpm": 10}}, zap.NewNop())
	h.incrementAccountRPMAfterForward(context.Background(), &service.Account{ID: 3, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth}, zap.NewNop())

	require.Equal(t, []int64{1}, rpm.increments)
}

type countingRPMCache struct {
	increments []int64
}

func (r *countingRPMCache) IncrementRPM(_ context.Context, accountID int64) (int, error) {
	r.increments = append(r.increments, accountID)
	return len(r.increments), nil
}
func (r *countingRPMCache) GetRPM(context.Context, int64) (int, error) { return 0, nil }
func (r *countingRPMCache) GetRPMBatch(context.Context, []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}
