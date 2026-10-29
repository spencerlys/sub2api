package middleware

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// enforceAccessTokenNotRevoked 拒绝已被登出或"撤销所有会话"作废的 access token。
// 撤销状态存于 Redis，查询失败时 fail-close 返回 503（不返回 401，避免前端误判为登录失效而清空会话）。
// 返回 false 表示请求已被中断。
func enforceAccessTokenNotRevoked(c *gin.Context, authService *service.AuthService, claims *service.JWTClaims) bool {
	err := authService.CheckAccessTokenRevoked(c.Request.Context(), claims)
	if err == nil {
		return true
	}
	if errors.Is(err, service.ErrTokenRevoked) {
		AbortWithError(c, 401, "TOKEN_REVOKED", "Token has been revoked")
		return false
	}
	AbortWithError(c, 503, "SERVICE_UNAVAILABLE", "Service temporarily unavailable")
	return false
}
