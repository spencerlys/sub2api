//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newCaptchaRequiredAuthHandler 构造“验证码必需但未配置”的 handler：
// 任何走到 VerifyCaptcha 的请求都会以 TURNSTILE_NOT_CONFIGURED 失败，以此判断校验是否被执行。
func newCaptchaRequiredAuthHandler(trustedPeers []string) *AuthHandler {
	cfg := &config.Config{
		Server:    config.ServerConfig{Mode: "release"},
		JWT:       config.JWTConfig{Secret: "handler-test-secret", ExpireHour: 1},
		Turnstile: config.TurnstileConfig{Required: true, TrustedPeerCIDRs: trustedPeers},
	}
	repo := &userHandlerRepoStub{user: &service.User{ID: 1, Email: "u@example.com", Status: service.StatusActive}}
	authSvc := service.NewAuthService(nil, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	return NewAuthHandler(cfg, authSvc, nil, nil, nil, nil, nil, nil)
}

func serveAuthRequest(handle gin.HandlerFunc, path, body, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = remoteAddr
	c.Request = req
	handle(c)
	return recorder
}

const trustedPeerLoginBody = `{"email":"u@example.com","password":"wrong-password"}`

func TestLoginCaptchaRequiredWhenTrustedPeersUnset(t *testing.T) {
	h := newCaptchaRequiredAuthHandler(nil)
	rec := serveAuthRequest(h.Login, "/api/v1/auth/login", trustedPeerLoginBody, "172.30.10.1:40000", nil)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "TURNSTILE_NOT_CONFIGURED")
}

func TestLoginSkipsCaptchaForTrustedPeer(t *testing.T) {
	h := newCaptchaRequiredAuthHandler([]string{"172.30.10.1/32"})
	rec := serveAuthRequest(h.Login, "/api/v1/auth/login", trustedPeerLoginBody, "172.30.10.1:40000", nil)
	// 跳过验证码后进入密码校验：错误密码返回 401，而不是验证码错误。
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.NotContains(t, rec.Body.String(), "TURNSTILE")
}

func TestLoginIgnoresForwardedHeadersForTrustedPeer(t *testing.T) {
	h := newCaptchaRequiredAuthHandler([]string{"172.30.10.1/32"})
	headers := map[string]string{
		"X-Forwarded-For":  "172.30.10.1",
		"CF-Connecting-IP": "172.30.10.1",
		"X-Real-IP":        "172.30.10.1",
	}
	// 公网流量经 Caddy（172.30.10.20）进入，伪造的转发头不能触发豁免。
	rec := serveAuthRequest(h.Login, "/api/v1/auth/login", trustedPeerLoginBody, "172.30.10.20:40000", headers)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "TURNSTILE_NOT_CONFIGURED")
}

func TestTrustedPeerDoesNotExemptOtherCaptchaEndpoints(t *testing.T) {
	h := newCaptchaRequiredAuthHandler([]string{"172.30.10.1/32"})
	cases := []struct {
		name   string
		handle gin.HandlerFunc
		path   string
		body   string
	}{
		{"register", h.Register, "/api/v1/auth/register", `{"email":"n@example.com","password":"password123"}`},
		{"send_verify_code", h.SendVerifyCode, "/api/v1/auth/send-verify-code", `{"email":"n@example.com"}`},
		{"forgot_password", h.ForgotPassword, "/api/v1/auth/forgot-password", `{"email":"n@example.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveAuthRequest(tc.handle, tc.path, tc.body, "172.30.10.1:40000", nil)
			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			require.Contains(t, rec.Body.String(), "TURNSTILE_NOT_CONFIGURED")
		})
	}
}
