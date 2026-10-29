//go:build unit

package routes

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 所有者目录登记的每条路由都必须真实存在，且带 :id 的路由只能落在有归属校验的前缀下。
func TestOwnerCatalogueMatchesRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAdminRoutes(r.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}}, middleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }), middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)

	owner := adminaccess.OwnerPolicy()
	allowed := 0
	for _, route := range r.Routes() {
		if !adminaccess.RouteAllowed(owner, route.Method, route.Path) {
			continue
		}
		allowed++
		if strings.Contains(route.Path, ":") {
			require.True(t,
				strings.HasPrefix(route.Path, "/api/v1/admin/accounts/:id") ||
					strings.HasPrefix(route.Path, "/api/v1/admin/proxies/:id") ||
					strings.HasPrefix(route.Path, "/api/v1/admin/groups/:id"),
				route.Method+" "+route.Path)
		}
	}
	// 与 catalog.go 中 owner.* 登记的路由总数一致；新增或拼错路由都会使此处失败。
	require.Equal(t, 64, allowed)
}
