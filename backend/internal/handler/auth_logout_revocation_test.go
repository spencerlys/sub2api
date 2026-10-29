//go:build unit

package handler

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// handlerAuthGuardCacheStub 内存版 service.AuthGuardCache（仅 access token 撤销相关方法有状态）。
type handlerAuthGuardCacheStub struct {
	mu        sync.Mutex
	sessions  map[string]bool
	revokedAt map[int64]int64
}

func (s *handlerAuthGuardCacheStub) GetLoginFailures(context.Context, string) (int, error) {
	return 0, nil
}
func (s *handlerAuthGuardCacheStub) IncrLoginFailures(context.Context, string, time.Duration, int, time.Duration) (int, error) {
	return 0, nil
}
func (s *handlerAuthGuardCacheStub) ClearLoginFailures(context.Context, string) error { return nil }
func (s *handlerAuthGuardCacheStub) SetUserTokensRevokedAt(_ context.Context, userID int64, revokedAt int64, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.revokedAt == nil {
		s.revokedAt = map[int64]int64{}
	}
	s.revokedAt[userID] = revokedAt
	return nil
}

func (s *handlerAuthGuardCacheStub) RevokeAccessSession(_ context.Context, sessionID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = map[string]bool{}
	}
	s.sessions[sessionID] = true
	return nil
}

func (s *handlerAuthGuardCacheStub) GetAccessTokenRevocation(_ context.Context, userID int64, sessionID string) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revokedAt[userID], s.sessions[sessionID], nil
}

func newSessionRevocationTestAuthService() *service.AuthService {
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "handler-test-secret", ExpireHour: 1}}
	svc := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetAuthGuardCache(&handlerAuthGuardCacheStub{})
	return svc
}

func TestLogoutRevokesCurrentAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := newSessionRevocationTestAuthService()
	h := NewAuthHandler(&config.Config{}, authSvc, nil, nil, nil, nil, nil, nil)

	user := &service.User{ID: 9, Email: "u@example.com", Role: service.RoleUser, Status: service.StatusActive}
	token, err := authSvc.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	claims, err := authSvc.ValidateToken(token)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req

	h.Logout(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.ErrorIs(t, authSvc.CheckAccessTokenRevoked(context.Background(), claims), service.ErrTokenRevoked)
}

func TestLogin2FADoesNotLogTempTokenPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	totpSvc := service.NewTotpService(nil, nil, &oauthPendingFlowTotpCacheStub{}, nil, nil, nil)
	h := NewAuthHandler(&config.Config{}, newSessionRevocationTestAuthService(), nil, nil, nil, nil, totpSvc, nil)

	tempToken := "deadbeefcafebabe0123456789abcdef"
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/2fa",
		strings.NewReader(`{"temp_token":"`+tempToken+`","totp_code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.Login2FA(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, buf.String(), tempToken[:8])
}

func TestAuthHandlerRevokeAllSessionsRevokesIssuedAccessTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 29, Email: "session@example.com", Role: service.RoleUser, Status: service.StatusActive}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authSvc := service.NewAuthService(nil, &userHandlerRepoStub{user: user}, nil, &userHandlerRefreshTokenCacheStub{}, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	authSvc.SetAuthGuardCache(&handlerAuthGuardCacheStub{})
	handler := &AuthHandler{authService: authSvc}

	token, err := authSvc.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	claims, err := authSvc.ValidateToken(token)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/revoke-all-sessions", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: user.ID})

	handler.RevokeAllSessions(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.ErrorIs(t, authSvc.CheckAccessTokenRevoked(context.Background(), claims), service.ErrTokenRevoked)
}
