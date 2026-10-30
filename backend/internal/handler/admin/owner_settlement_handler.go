package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OwnerSettlementHandler AI 账号所有者结算、打款记录与财务汇总。
// 除 Mine 外均为管理员接口：路由目录不向所有者与管理密钥开放，处理器再拒绝一次。
type OwnerSettlementHandler struct {
	svc *service.OwnerSettlementService
}

func NewOwnerSettlementHandler(svc *service.OwnerSettlementService) *OwnerSettlementHandler {
	return &OwnerSettlementHandler{svc: svc}
}

func (h *OwnerSettlementHandler) adminOnly(c *gin.Context) bool {
	ctx := c.Request.Context()
	if adminaccess.OwnerFromContext(ctx) != nil || adminaccess.FromContext(ctx) != nil {
		response.Forbidden(c, "仅管理员可访问财务与结算数据")
		return false
	}
	if h.svc == nil {
		response.InternalError(c, "结算服务不可用")
		return false
	}
	return true
}

func settlementPeriod(c *gin.Context) (time.Time, time.Time, bool) {
	start, end, err := service.ParseSettlementPeriod(c.Query("start"), c.Query("end"), service.SettlementLocation(c.Query("timezone")), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func optionalOwnerID(c *gin.Context) (*int64, bool) {
	raw := strings.TrimSpace(c.Query("owner_user_id"))
	if raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid owner_user_id")
		return nil, false
	}
	return &id, true
}

// List 所有者结算报表（每个所有者一行 + 合计）。
// GET /api/v1/admin/owner-settlements?start=YYYY-MM-DD&end=YYYY-MM-DD[&owner_user_id=][&timezone=]
func (h *OwnerSettlementHandler) List(c *gin.Context) {
	if !h.adminOnly(c) {
		return
	}
	ownerID, ok := optionalOwnerID(c)
	if !ok {
		return
	}
	start, end, ok := settlementPeriod(c)
	if !ok {
		return
	}
	report, err := h.svc.GetOwnerSettlements(c.Request.Context(), start, end, ownerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

// Mine 所有者本人结算汇总；不含用户计费、账户倍率、实付占比与用户身份。
// GET /api/v1/admin/owner-settlements/me?start=&end=[&timezone=]
func (h *OwnerSettlementHandler) Mine(c *gin.Context) {
	ownerID := adminaccess.OwnerUserID(c.Request.Context())
	if ownerID <= 0 || h.svc == nil {
		response.ErrorFrom(c, service.ErrSettlementOwnerOnly)
		return
	}
	start, end, ok := settlementPeriod(c)
	if !ok {
		return
	}
	me, err := h.svc.GetMySettlement(c.Request.Context(), ownerID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, me)
}

// FinanceSummary 期间入账按来源汇总。
// GET /api/v1/admin/finance/summary?start=&end=[&timezone=]
func (h *OwnerSettlementHandler) FinanceSummary(c *gin.Context) {
	if !h.adminOnly(c) {
		return
	}
	start, end, ok := settlementPeriod(c)
	if !ok {
		return
	}
	summary, err := h.svc.GetFinanceSummary(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// ListPayouts 打款记录（按打款时间倒序）；start/end 可选，按打款日期筛选。
// GET /api/v1/admin/owner-payouts?owner_user_id=&start=&end=&page=&page_size=[&timezone=]
func (h *OwnerSettlementHandler) ListPayouts(c *gin.Context) {
	if !h.adminOnly(c) {
		return
	}
	ownerID, ok := optionalOwnerID(c)
	if !ok {
		return
	}
	filter := service.OwnerPayoutFilter{OwnerUserID: ownerID}
	if c.Query("start") != "" || c.Query("end") != "" {
		start, end, ok := settlementPeriod(c)
		if !ok {
			return
		}
		filter.Start, filter.End = &start, &end
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.ListOwnerPayouts(c.Request.Context(), filter, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if items == nil {
		items = []service.OwnerPayout{}
	}
	response.Paginated(c, items, total, page, pageSize)
}

type createOwnerPayoutRequest struct {
	OwnerUserID int64      `json:"owner_user_id" binding:"required,gt=0"`
	Amount      float64    `json:"amount" binding:"required,gt=0"`
	PaidAt      *time.Time `json:"paid_at"`
	Notes       string     `json:"notes" binding:"max=1000"`
}

// CreatePayout 记录一笔打款。
// POST /api/v1/admin/owner-payouts
func (h *OwnerSettlementHandler) CreatePayout(c *gin.Context) {
	if !h.adminOnly(c) {
		return
	}
	var req createOwnerPayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var createdBy int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		createdBy = subject.UserID
	}
	p, err := h.svc.CreateOwnerPayout(c.Request.Context(), service.OwnerPayoutInput{
		OwnerUserID: req.OwnerUserID,
		Amount:      req.Amount,
		PaidAt:      req.PaidAt,
		Notes:       req.Notes,
	}, createdBy)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}

// DeletePayout 软删除一笔打款记录。
// DELETE /api/v1/admin/owner-payouts/:id
func (h *OwnerSettlementHandler) DeletePayout(c *gin.Context) {
	if !h.adminOnly(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid payout ID")
		return
	}
	if err := h.svc.DeleteOwnerPayout(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

type updateBalanceRecordSourceRequest struct {
	Source string `json:"source" binding:"required"`
}

// UpdateBalanceRecordSource 修改管理员余额调整记录的资金来源（只允许 admin_balance）。
// PUT /api/v1/admin/users/:id/balance-history/:recordId/source
func (h *OwnerSettlementHandler) UpdateBalanceRecordSource(c *gin.Context) {
	if !h.adminOnly(c) {
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	recordID, err := strconv.ParseInt(c.Param("recordId"), 10, 64)
	if err != nil || recordID <= 0 {
		response.BadRequest(c, "Invalid record ID")
		return
	}
	var req updateBalanceRecordSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.svc.UpdateBalanceRecordSource(c.Request.Context(), userID, recordID, req.Source)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
