//go:build unit

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// memAuthGuardCache 内存版 service.AuthGuardCache，仅覆盖 access token 撤销相关方法。
type memAuthGuardCache struct {
	mu        sync.Mutex
	revokedAt map[int64]int64
	sessions  map[string]bool
	err       error
	lookups   int
}

func newMemAuthGuardCache() *memAuthGuardCache {
	return &memAuthGuardCache{revokedAt: map[int64]int64{}, sessions: map[string]bool{}}
}

func (m *memAuthGuardCache) GetLoginFailures(context.Context, string) (int, error) { return 0, nil }
func (m *memAuthGuardCache) IncrLoginFailures(context.Context, string, time.Duration, int, time.Duration) (int, error) {
	return 0, nil
}
func (m *memAuthGuardCache) ClearLoginFailures(context.Context, string) error { return nil }

func (m *memAuthGuardCache) SetUserTokensRevokedAt(_ context.Context, userID int64, revokedAt int64, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokedAt[userID] = revokedAt
	return nil
}

func (m *memAuthGuardCache) RevokeAccessSession(_ context.Context, sessionID string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = true
	return nil
}

func (m *memAuthGuardCache) GetAccessTokenRevocation(_ context.Context, userID int64, sessionID string) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lookups++
	if m.err != nil {
		return 0, false, m.err
	}
	return m.revokedAt[userID], m.sessions[sessionID], nil
}

func jwtRevocationTestUser() *service.User {
	return &service.User{
		ID:           1,
		Email:        "test@example.com",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  1,
		TokenVersion: 1,
	}
}

func serveProtected(router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_RejectsAccessTokenAfterRevokeAll(t *testing.T) {
	user := jwtRevocationTestUser()
	router, authSvc := newJWTTestEnv(map[int64]*service.User{1: user})
	guard := newMemAuthGuardCache()
	authSvc.SetAuthGuardCache(guard)

	token, err := authSvc.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, serveProtected(router, "/protected", token).Code)
	require.Equal(t, 1, guard.lookups, "one revocation lookup per request")

	require.NoError(t, authSvc.RevokeAllUserTokens(context.Background(), user.ID))

	w := serveProtected(router, "/protected", token)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "TOKEN_REVOKED")
}

func TestJWTAuth_RejectsAccessTokenOfLoggedOutSession(t *testing.T) {
	user := jwtRevocationTestUser()
	router, authSvc := newJWTTestEnv(map[int64]*service.User{1: user})
	authSvc.SetAuthGuardCache(newMemAuthGuardCache())

	loggedOut, err := authSvc.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	otherDevice, err := authSvc.GenerateToken(context.Background(), user)
	require.NoError(t, err)

	require.NoError(t, authSvc.RevokeLogoutSession(context.Background(), "", loggedOut))

	require.Equal(t, http.StatusUnauthorized, serveProtected(router, "/protected", loggedOut).Code)
	require.Equal(t, http.StatusOK, serveProtected(router, "/protected", otherDevice).Code)
}

func TestJWTAuth_RevocationLookupFailureFailsClosed(t *testing.T) {
	user := jwtRevocationTestUser()
	router, authSvc := newJWTTestEnv(map[int64]*service.User{1: user})
	guard := newMemAuthGuardCache()
	authSvc.SetAuthGuardCache(guard)

	token, err := authSvc.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	guard.err = errors.New("redis down")

	w := serveProtected(router, "/protected", token)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAdminAuthJWT_RejectsRevokedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	authService.SetAuthGuardCache(newMemAuthGuardCache())

	admin := &service.User{ID: 1, Email: "admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive, Concurrency: 1}
	userService := service.NewUserService(&stubUserRepo{
		getByID: func(ctx context.Context, id int64) (*service.User, error) {
			clone := *admin
			return &clone, nil
		},
	}, nil, nil, nil)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(authService, userService, nil, nil)))
	router.GET("/t", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	token, err := authService.GenerateToken(context.Background(), admin)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, serveProtected(router, "/t", token).Code)

	require.NoError(t, authService.RevokeLogoutSession(context.Background(), "", token))

	w := serveProtected(router, "/t", token)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "TOKEN_REVOKED")
}
