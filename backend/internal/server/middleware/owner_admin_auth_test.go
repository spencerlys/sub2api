//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminAuthOwnerJWTUsesOwnerCatalogue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	users := map[int64]*service.User{
		7: {ID: 7, Email: "owner@example.com", Role: service.RoleOwner, Status: service.StatusActive, Concurrency: 1},
		8: {ID: 8, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1},
		9: {ID: 9, Email: "admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive, Concurrency: 1},
	}
	userService := service.NewUserService(&stubUserRepo{getByID: func(_ context.Context, id int64) (*service.User, error) {
		if u, ok := users[id]; ok {
			clone := *u
			return &clone, nil
		}
		return nil, service.ErrUserNotFound
	}}, nil, nil, nil)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(authService, userService, nil, nil)))
	seen := map[string]int64{}
	record := func(c *gin.Context) {
		seen[c.FullPath()] = adminaccess.OwnerUserID(c.Request.Context())
		require.Nil(t, adminaccess.FromContext(c.Request.Context()), "所有者不能被当作管理密钥")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
	router.GET("/api/v1/admin/accounts", record)
	router.GET("/api/v1/admin/users", record)
	router.GET("/api/v1/admin/proxies/data", record)

	call := func(userID int64, path string) int {
		u := users[userID]
		token, err := authService.GenerateToken(context.Background(), u)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)
		return w.Code
	}

	require.Equal(t, http.StatusOK, call(7, "/api/v1/admin/accounts"))
	require.Equal(t, int64(7), seen["/api/v1/admin/accounts"])
	require.Equal(t, http.StatusForbidden, call(7, "/api/v1/admin/users"))
	require.Equal(t, http.StatusForbidden, call(7, "/api/v1/admin/proxies/data"))
	require.Equal(t, http.StatusForbidden, call(7, "/api/v1/admin/unknown"))
	require.Equal(t, http.StatusForbidden, call(8, "/api/v1/admin/accounts"))

	require.Equal(t, http.StatusOK, call(9, "/api/v1/admin/users"))
	require.Zero(t, seen["/api/v1/admin/users"], "管理员不携带所有者身份")
}
