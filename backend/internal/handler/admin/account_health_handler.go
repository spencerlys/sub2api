package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type accountHealthReader interface {
	GetAccountHealth(ctx context.Context, rawRange string, ownerUserID *int64) (*service.AccountHealthReport, error)
}

func (h *AccountHandler) SetAccountHealthService(s *service.AccountHealthService) {
	if s != nil {
		h.accountHealth = s
	}
}

// GetHealth 账号健康概况。
// GET /api/v1/admin/accounts/health?range=1h|24h|7d[&owner_user_id=]
// 所有者只能看到本人账户；owner_user_id 仅供管理员筛选，所有者传入他人 ID 一律拒绝。
func (h *AccountHandler) GetHealth(c *gin.Context) {
	if h.accountHealth == nil {
		response.Error(c, http.StatusServiceUnavailable, "账号健康统计不可用")
		return
	}
	rangeKey, _, err := service.ParseAccountHealthRange(c.Query("range"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	var ownerFilter *int64
	if raw := strings.TrimSpace(c.Query("owner_user_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid owner_user_id")
			return
		}
		ownerFilter = &id
	}
	if owner := adminaccess.OwnerFromContext(c.Request.Context()); owner != nil {
		if ownerFilter != nil && *ownerFilter != owner.OwnerUserID {
			response.Forbidden(c, "账号所有者只能查看本人账户")
			return
		}
		// 归属由服务层与仓储按上下文再次限定，这里不透传筛选。
		ownerFilter = nil
	}

	report, err := h.accountHealth.GetAccountHealth(c.Request.Context(), rangeKey, ownerFilter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}
