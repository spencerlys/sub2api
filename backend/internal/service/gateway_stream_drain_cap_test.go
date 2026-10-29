//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// pingingAnthropicUpstream 写完 stream 后持续发送 ping 且不结束，模拟客户端断开后
// 上游仍在长时间输出（读间隔超时永远不会触发）。
func pingingAnthropicUpstream(t *testing.T, stream string) *http.Response {
	t.Helper()
	pr, pw := io.Pipe()
	go func() {
		if _, err := io.WriteString(pw, stream); err != nil {
			return
		}
		for {
			time.Sleep(10 * time.Millisecond)
			if _, err := io.WriteString(pw, "event: ping\ndata: {\"type\":\"ping\"}\n\n"); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = pw.Close()
		_ = pr.Close()
	})
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}
}

func withStreamDrainCap(t *testing.T, d time.Duration) {
	t.Helper()
	old := streamDrainAfterClientDisconnectTimeout
	streamDrainAfterClientDisconnectTimeout = d
	t.Cleanup(func() { streamDrainAfterClientDisconnectTimeout = old })
}

// 客户端断开后为计费继续排空上游，但总时长必须受限，结束后带回已观测 usage。
func TestHandleStreamingResponse_ClientDisconnectDrainIsCapped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withStreamDrainCap(t, 200*time.Millisecond)

	writer := &disconnectOnMarkerResponseWriter{ResponseRecorder: httptest.NewRecorder(), marker: "second"}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := pingingAnthropicUpstream(t, anthropicStreamTruncatedBeforeMessageDelta())
	svc := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize, StreamDataIntervalTimeout: 5}}, rateLimitService: &RateLimitService{}}

	type outcome struct {
		result *streamingResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "claude-sonnet-4-6", "claude-sonnet-4-6", false)
		done <- outcome{result: result, err: err}
	}()

	select {
	case got := <-done:
		require.Error(t, got.err)
		require.NotNil(t, got.result)
		require.True(t, got.result.clientDisconnect)
		require.Equal(t, 20, got.result.usage.InputTokens)
	case <-time.After(3 * time.Second):
		t.Fatal("post-disconnect drain was not capped")
	}
}

func TestAnthropicCompatStreaming_ClientDisconnectDrainIsCapped(t *testing.T) {
	withStreamDrainCap(t, 200*time.Millisecond)
	for name, handle := range compatStreamHandlers() {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			writer := &disconnectOnMarkerResponseWriter{ResponseRecorder: httptest.NewRecorder(), marker: "second"}
			c, _ := gin.CreateTestContext(writer)
			resp := pingingAnthropicUpstream(t, strings.TrimSpace(anthropicStreamTruncatedBeforeMessageDelta())+"\n\n")

			got := runCompatStreamWithDeadline(t, handle, newCompatStreamIdleTestService(5), resp, c)

			require.NotNil(t, got.result)
			require.True(t, got.result.ClientDisconnect)
			require.Equal(t, 20, got.result.Usage.InputTokens)
		})
	}
}
