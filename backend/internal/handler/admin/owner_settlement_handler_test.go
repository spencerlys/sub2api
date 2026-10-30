//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type settlementRepoStub struct {
	ownerFilter *int64
	usageStart  time.Time
	created     *service.OwnerPayout
	records     map[int64]*service.RedeemCode
	notes       map[int64]string
}

func (s *settlementRepoStub) ListSettlementOwners(_ context.Context, id *int64) ([]service.OwnerIdentity, error) {
	s.ownerFilter = id
	all := []service.OwnerIdentity{{UserID: 7, Email: "owner-a@example.com", Username: "a"}, {UserID: 8, Email: "owner-b@example.com", Username: "b"}}
	if id == nil {
		return all, nil
	}
	for _, o := range all {
		if o.UserID == *id {
			return []service.OwnerIdentity{o}, nil
		}
	}
	return nil, nil
}

func (s *settlementRepoStub) AggregateOwnerUserUsage(_ context.Context, start, _ time.Time, id *int64) ([]service.OwnerUserUsage, error) {
	s.usageStart = start
	rows := []service.OwnerUserUsage{
		{OwnerUserID: 7, UserID: 100, Requests: 3, OfficialCost: 10, UserBilled: 15, AccountCost: 4, AccountCostAllTime: 8},
		{OwnerUserID: 8, UserID: 100, Requests: 1, OfficialCost: 2, UserBilled: 3, AccountCost: 1, AccountCostAllTime: 1},
	}
	var out []service.OwnerUserUsage
	for _, r := range rows {
		if id == nil || *id == r.OwnerUserID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *settlementRepoStub) ListUserCredits(context.Context, []int64) ([]service.UserCreditRow, error) {
	return []service.UserCreditRow{{UserID: 100, Credit: service.BalanceCredit{Kind: service.CreditKindAdmin, Amount: 10, Source: "offline"}}}, nil
}

func (s *settlementRepoStub) SumOwnerPayouts(context.Context, *int64) (map[int64]float64, error) {
	return map[int64]float64{7: 1}, nil
}

func (s *settlementRepoStub) ListOwnerPayouts(context.Context, service.OwnerPayoutFilter, pagination.PaginationParams) ([]service.OwnerPayout, int64, error) {
	return []service.OwnerPayout{{ID: 1, OwnerUserID: 7, Amount: 1}}, 1, nil
}

func (s *settlementRepoStub) CreateOwnerPayout(_ context.Context, p *service.OwnerPayout) error {
	p.ID = 2
	s.created = p
	return nil
}

func (s *settlementRepoStub) DeleteOwnerPayout(context.Context, int64) error { return nil }

func (s *settlementRepoStub) AggregateFinancePeriod(context.Context, time.Time, time.Time) (*service.FinancePeriodRaw, error) {
	return &service.FinancePeriodRaw{}, nil
}

func (s *settlementRepoStub) GetBalanceRecord(_ context.Context, id int64) (*service.RedeemCode, error) {
	if r := s.records[id]; r != nil {
		cp := *r
		return &cp, nil
	}
	return nil, service.ErrRedeemCodeNotFound
}

func (s *settlementRepoStub) UpdateBalanceRecordNotes(_ context.Context, id int64, notes string) error {
	s.notes[id] = notes
	return nil
}

func newSettlementRouter(ownerID int64, repo *settlementRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewOwnerSettlementHandler(service.NewOwnerSettlementService(repo))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if ownerID > 0 {
			principal := &adminaccess.Principal{OwnerUserID: ownerID, Policy: adminaccess.OwnerPolicy()}
			c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), principal))
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
	})
	r.GET("/api/v1/admin/owner-settlements", h.List)
	r.GET("/api/v1/admin/owner-settlements/me", h.Mine)
	r.GET("/api/v1/admin/owner-payouts", h.ListPayouts)
	r.POST("/api/v1/admin/owner-payouts", h.CreatePayout)
	r.DELETE("/api/v1/admin/owner-payouts/:id", h.DeletePayout)
	r.GET("/api/v1/admin/finance/summary", h.FinanceSummary)
	r.PUT("/api/v1/admin/users/:id/balance-history/:recordId/source", h.UpdateBalanceRecordSource)
	return r
}

func settlementCall(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestOwnerSettlementHandler_OwnerOnlyGetsOwnSummary(t *testing.T) {
	repo := &settlementRepoStub{}
	r := newSettlementRouter(7, repo)

	w := settlementCall(r, http.MethodGet, "/api/v1/admin/owner-settlements/me?start=2026-09-01&end=2026-09-30", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, repo.ownerFilter)
	require.Equal(t, int64(7), *repo.ownerFilter)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.InDelta(t, 10, body.Data["official_cost"], 1e-9)
	require.InDelta(t, 4, body.Data["earnings"], 1e-9)
	require.InDelta(t, 7, body.Data["unpaid"], 1e-9)
	raw := w.Body.String()
	for _, forbidden := range []string{"user_billed", "account_cost", "multiplier", "paid_share", "email", "username", "user_id", "owner-b"} {
		require.NotContains(t, raw, forbidden)
	}

	// 所有者调用管理员接口一律 403（路由目录之外的第二道防线）。
	for _, call := range [][3]string{
		{http.MethodGet, "/api/v1/admin/owner-settlements", ""},
		{http.MethodGet, "/api/v1/admin/owner-settlements?owner_user_id=7", ""},
		{http.MethodGet, "/api/v1/admin/finance/summary", ""},
		{http.MethodGet, "/api/v1/admin/owner-payouts", ""},
		{http.MethodPost, "/api/v1/admin/owner-payouts", `{"owner_user_id":7,"amount":1}`},
		{http.MethodDelete, "/api/v1/admin/owner-payouts/1", ""},
		{http.MethodPut, "/api/v1/admin/users/1/balance-history/1/source", `{"source":"gift"}`},
	} {
		w := settlementCall(r, call[0], call[1], call[2])
		require.Equal(t, http.StatusForbidden, w.Code, call[0]+" "+call[1])
		require.NotContains(t, w.Body.String(), "owner-a@example.com")
	}
	require.Nil(t, repo.created)

	// 管理员没有个人结算。
	admin := newSettlementRouter(0, &settlementRepoStub{})
	require.Equal(t, http.StatusForbidden, settlementCall(admin, http.MethodGet, "/api/v1/admin/owner-settlements/me", "").Code)
}

func TestOwnerSettlementHandler_AdminReport(t *testing.T) {
	repo := &settlementRepoStub{}
	r := newSettlementRouter(0, repo)

	w := settlementCall(r, http.MethodGet, "/api/v1/admin/owner-settlements?start=2026-09-01&end=2026-09-30&timezone=UTC", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Nil(t, repo.ownerFilter)
	require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), repo.usageStart.UTC())
	var body struct {
		Data service.OwnerSettlementReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Items, 2)
	require.Equal(t, "owner-a@example.com", body.Data.Items[0].Email)
	require.Equal(t, int64(4), body.Data.Totals.Requests)

	w = settlementCall(r, http.MethodGet, "/api/v1/admin/owner-settlements?owner_user_id=8", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(8), *repo.ownerFilter)

	for _, q := range []string{"?owner_user_id=x", "?owner_user_id=-1", "?start=2026/09/01", "?start=2025-01-01&end=2026-06-01", "?start=2026-09-02&end=2026-09-01"} {
		require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodGet, "/api/v1/admin/owner-settlements"+q, "").Code, q)
	}
	require.Equal(t, http.StatusOK, settlementCall(r, http.MethodGet, "/api/v1/admin/finance/summary?start=2026-09-01&end=2026-09-30", "").Code)
	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodGet, "/api/v1/admin/finance/summary?end=bad", "").Code)
}

func TestOwnerSettlementHandler_Payouts(t *testing.T) {
	repo := &settlementRepoStub{}
	r := newSettlementRouter(0, repo)

	for _, body := range []string{`{}`, `{"owner_user_id":7,"amount":0}`, `{"owner_user_id":7,"amount":-1}`, `{"owner_user_id":7,"amount":1,"paid_at":"yesterday"}`, `{"owner_user_id":99,"amount":1}`} {
		require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodPost, "/api/v1/admin/owner-payouts", body).Code, body)
	}
	w := settlementCall(r, http.MethodPost, "/api/v1/admin/owner-payouts", `{"owner_user_id":7,"amount":12.5,"paid_at":"2026-10-01T00:00:00Z","notes":"bank"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, repo.created)
	require.InDelta(t, 12.5, repo.created.Amount, 1e-9)
	require.Equal(t, int64(1), *repo.created.CreatedBy, "created_by comes from the authenticated admin")
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), repo.created.PaidAt.UTC())

	require.Equal(t, http.StatusOK, settlementCall(r, http.MethodGet, "/api/v1/admin/owner-payouts?owner_user_id=7&start=2026-09-01&end=2026-10-31", "").Code)
	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodGet, "/api/v1/admin/owner-payouts?owner_user_id=abc", "").Code)
	require.Equal(t, http.StatusOK, settlementCall(r, http.MethodDelete, "/api/v1/admin/owner-payouts/3", "").Code)
	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodDelete, "/api/v1/admin/owner-payouts/x", "").Code)
}

func TestOwnerSettlementHandler_BalanceRecordSource(t *testing.T) {
	user := int64(5)
	repo := &settlementRepoStub{notes: map[int64]string{}, records: map[int64]*service.RedeemCode{
		1: {ID: 1, Type: service.AdjustmentTypeAdminBalance, UsedBy: &user, Notes: "wire"},
		2: {ID: 2, Type: service.RedeemTypeBalance, UsedBy: &user},
	}}
	r := newSettlementRouter(0, repo)

	w := settlementCall(r, http.MethodPut, "/api/v1/admin/users/5/balance-history/1/source", `{"source":"offline"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "[src:offline] wire", repo.notes[1])
	require.Contains(t, w.Body.String(), `"balance_source":"offline"`)

	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodPut, "/api/v1/admin/users/5/balance-history/2/source", `{"source":"offline"}`).Code, "non admin_balance record")
	require.Equal(t, http.StatusNotFound, settlementCall(r, http.MethodPut, "/api/v1/admin/users/6/balance-history/1/source", `{"source":"offline"}`).Code)
	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodPut, "/api/v1/admin/users/5/balance-history/1/source", `{"source":"paid"}`).Code)
	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodPut, "/api/v1/admin/users/5/balance-history/1/source", `{}`).Code)
	require.Equal(t, http.StatusBadRequest, settlementCall(r, http.MethodPut, "/api/v1/admin/users/x/balance-history/1/source", `{"source":"gift"}`).Code)
	require.Len(t, repo.notes, 1)
}

// 管理员手动调整余额可选择来源；来源写入备注前缀，非法来源被拒绝，返利副作用由服务层原样保留。
type balanceNotesRecorder struct {
	*stubAdminService
	notes, operation string
	calls            int
}

func (s *balanceNotesRecorder) UpdateUserBalance(ctx context.Context, userID int64, balance float64, operation string, notes string) (*service.User, error) {
	s.calls++
	s.notes, s.operation = notes, operation
	return s.stubAdminService.UpdateUserBalance(ctx, userID, balance, operation, notes)
}

func TestUpdateBalanceSourcePrefixesNotes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &balanceNotesRecorder{stubAdminService: newStubAdminService()}
	h := NewUserHandler(svc, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/api/v1/admin/users/:id/balance", h.UpdateBalance)
	post := func(body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/balance", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}

	require.Equal(t, http.StatusOK, post(`{"balance":5,"operation":"add","notes":"bank transfer","source":"offline"}`))
	require.Equal(t, "[src:offline] bank transfer", svc.notes)
	require.Equal(t, "add", svc.operation)

	require.Equal(t, http.StatusOK, post(`{"balance":5,"operation":"add","notes":"legacy client"}`))
	require.Equal(t, "legacy client", svc.notes, "no source keeps legacy notes (treated as gift)")

	calls := svc.calls
	require.Equal(t, http.StatusBadRequest, post(`{"balance":5,"operation":"add","source":"paid"}`))
	require.Equal(t, calls, svc.calls)
}
