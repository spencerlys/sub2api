package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// registerOwnerSettlementRoutes 所有者结算、打款记录、财务汇总、成本账本重建与余额调整来源。
// 只有 GET /owner-settlements/me 登记在所有者目录（adminaccess/catalog.go），其余仅管理员可用。
func registerOwnerSettlementRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	settlements := admin.Group("/owner-settlements")
	{
		settlements.GET("", h.Admin.OwnerSettlement.List)
		settlements.GET("/me", h.Admin.OwnerSettlement.Mine)
	}
	payouts := admin.Group("/owner-payouts")
	{
		payouts.GET("", h.Admin.OwnerSettlement.ListPayouts)
		payouts.POST("", h.Admin.OwnerSettlement.CreatePayout)
		payouts.DELETE("/:id", h.Admin.OwnerSettlement.DeletePayout)
	}
	admin.GET("/finance/summary", h.Admin.OwnerSettlement.FinanceSummary)
	admin.POST("/finance/rebuild-ledger", h.Admin.OwnerSettlement.RebuildLedger)
	admin.PUT("/users/:id/balance-history/:recordId/source", h.Admin.OwnerSettlement.UpdateBalanceRecordSource)
}
