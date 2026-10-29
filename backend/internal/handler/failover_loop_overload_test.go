package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

// 529 驱动的换号在单个请求内最多一次：全局过载时继续换号只会把更多账号拖进来。
func TestHandleFailoverError_OverloadSwitchCappedAtOne(t *testing.T) {
	mock := &mockTempUnscheduler{}
	fs := NewFailoverState(3, false)
	overloaded := &service.UpstreamFailoverError{StatusCode: 529}

	require.Equal(t, FailoverContinue, fs.HandleFailoverError(context.Background(), mock, 1, service.PlatformAnthropic, 0, overloaded))
	require.Equal(t, 1, fs.SwitchCount)

	require.Equal(t, FailoverExhausted, fs.HandleFailoverError(context.Background(), mock, 2, service.PlatformAnthropic, 0, overloaded))
	require.Equal(t, 1, fs.SwitchCount)
	require.Equal(t, 529, fs.LastFailoverErr.StatusCode)
	require.Contains(t, fs.FailedAccountIDs, int64(2))
}

// 非 529 错误不受过载换号上限影响。
func TestHandleFailoverError_OverloadCapDoesNotLimitOtherErrors(t *testing.T) {
	mock := &mockTempUnscheduler{}
	fs := NewFailoverState(3, false)

	require.Equal(t, FailoverContinue, fs.HandleFailoverError(context.Background(), mock, 1, service.PlatformAnthropic, 0, &service.UpstreamFailoverError{StatusCode: 529}))
	require.Equal(t, FailoverContinue, fs.HandleFailoverError(context.Background(), mock, 2, service.PlatformAnthropic, 0, &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}))
	require.Equal(t, FailoverContinue, fs.HandleFailoverError(context.Background(), mock, 3, service.PlatformAnthropic, 0, &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway}))
	require.Equal(t, 3, fs.SwitchCount)
}
