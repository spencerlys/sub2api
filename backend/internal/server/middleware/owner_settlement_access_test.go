//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 所有者只能读取本人结算；结算报表、打款、财务汇总、账本重建与余额来源修改仅管理员可用。
func TestAdminAuthOwnerSettlementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	users := map[int64]*service.User{
		7: {ID: 7, Email: "owner@example.com", Role: service.RoleOwner, Status: service.StatusActive, Concurrency: 1},
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
	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	adminOnly := [][2]string{
		{http.MethodGet, "/api/v1/admin/owner-settlements"},
		{http.MethodGet, "/api/v1/admin/finance/summary"},
		{http.MethodPost, "/api/v1/admin/finance/rebuild-ledger"},
		{http.MethodGet, "/api/v1/admin/owner-payouts"},
		{http.MethodPost, "/api/v1/admin/owner-payouts"},
		{http.MethodDelete, "/api/v1/admin/owner-payouts/1"},
		{http.MethodPut, "/api/v1/admin/users/1/balance-history/2/source"},
	}
	router.GET("/api/v1/admin/owner-settlements/me", ok)
	router.GET("/api/v1/admin/owner-settlements", ok)
	router.GET("/api/v1/admin/finance/summary", ok)
	router.POST("/api/v1/admin/finance/rebuild-ledger", ok)
	router.GET("/api/v1/admin/owner-payouts", ok)
	router.POST("/api/v1/admin/owner-payouts", ok)
	router.DELETE("/api/v1/admin/owner-payouts/:id", ok)
	router.PUT("/api/v1/admin/users/:id/balance-history/:recordId/source", ok)

	call := func(userID int64, method, path string) int {
		token, err := authService.GenerateToken(context.Background(), users[userID])
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)
		return w.Code
	}

	require.Equal(t, http.StatusOK, call(7, http.MethodGet, "/api/v1/admin/owner-settlements/me"))
	for _, r := range adminOnly {
		require.Equal(t, http.StatusForbidden, call(7, r[0], r[1]), r[0]+" "+r[1])
		require.Equal(t, http.StatusOK, call(9, r[0], r[1]), r[0]+" "+r[1])
	}
}
