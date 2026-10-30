//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func usageTestContext(principal *adminaccess.Principal) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage", nil)
	if principal != nil {
		req = req.WithContext(adminaccess.WithPrincipal(req.Context(), principal))
	}
	c.Request = req
	return c
}

func f64(v float64) *float64 { return &v }

func usageSplitRecords() ([]service.UsageLog, map[int64]service.UsageRowSettlement) {
	rate := 0.8
	records := []service.UsageLog{
		{ID: 1, UserID: 5, AccountID: 1, TotalCost: 2, ActualCost: 1.6, RateMultiplier: rate},
		{ID: 2, UserID: 5, AccountID: 1, TotalCost: 1, ActualCost: 0.8, RateMultiplier: rate},
	}
	splits := map[int64]service.UsageRowSettlement{
		1: {PaidCost: 1.2, GiftCost: 0.4, Earnings: f64(0.75), GiftAccountCost: f64(0.25)},
	}
	return records, splits
}

func toJSONMaps(t *testing.T, v any) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestAdminUsageRowsCarryExactSplit(t *testing.T) {
	records, splits := usageSplitRecords()
	rows := toJSONMaps(t, adminUsageLogResponses(usageTestContext(nil), records, splits))
	require.Len(t, rows, 2)

	require.Equal(t, 1.6, rows[0]["actual_cost"])
	require.Equal(t, 0.8, rows[0]["rate_multiplier"])
	require.Equal(t, 1.2, rows[0]["paid_cost"])
	require.Equal(t, 0.4, rows[0]["gift_cost"])
	require.Equal(t, 0.75, rows[0]["earnings"])
	require.Equal(t, 0.25, rows[0]["gift_account_cost"])
	require.Equal(t, false, rows[0]["split_pending"])

	// 未拆分：显式待结算，不估算。
	require.Equal(t, true, rows[1]["split_pending"])
	for _, key := range []string{"paid_cost", "gift_cost", "earnings", "gift_account_cost"} {
		require.Contains(t, rows[1], key)
		require.Nil(t, rows[1][key], key)
	}
}

func TestScopedKeyUsageRowsCarrySplit(t *testing.T) {
	records, splits := usageSplitRecords()
	principal := &adminaccess.Principal{Policy: adminaccess.Policy{GlobalPermissions: []string{"usage.read"}}}
	rows := toJSONMaps(t, adminUsageLogResponses(usageTestContext(principal), records, splits))
	require.Equal(t, 1.2, rows[0]["paid_cost"])
	require.Equal(t, true, rows[1]["split_pending"])
	require.Contains(t, rows[0], "actual_cost")
}

func TestOwnerUsageRowsShowEarningsOnly(t *testing.T) {
	records, splits := usageSplitRecords()
	principal := &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()}
	rows := toJSONMaps(t, adminUsageLogResponses(usageTestContext(principal), records, splits))
	require.Len(t, rows, 2)
	require.Equal(t, 2.0, rows[0]["total_cost"])
	require.Equal(t, 0.75, rows[0]["earnings"])
	require.Equal(t, false, rows[0]["split_pending"])
	require.Nil(t, rows[1]["earnings"])
	require.Equal(t, true, rows[1]["split_pending"])
	for _, row := range rows {
		for _, key := range []string{"actual_cost", "rate_multiplier", "account_rate_multiplier", "account_stats_cost",
			"paid_cost", "gift_cost", "gift_account_cost", "user_id", "api_key_id", "ip_address", "user_agent"} {
			require.NotContains(t, row, key)
		}
	}
}

func TestOwnerUsageStatsHideBilledAmounts(t *testing.T) {
	paid, gift, earn, giftAcct := 3.0, 1.0, 2.5, 0.5
	pending := int64(4)
	accountCost := 3.0
	stats := &usagestats.UsageStats{
		TotalRequests: 10, TotalTokens: 100, TotalCost: 5, TotalActualCost: 4, TotalAccountCost: &accountCost,
		TotalPaidCost: &paid, TotalGiftCost: &gift, TotalEarnings: &earn, TotalGiftAccountCost: &giftAcct, PendingRequests: &pending,
		Endpoints: []usagestats.EndpointStat{{Endpoint: "/v1/messages", Requests: 10, TotalTokens: 100, Cost: 5, ActualCost: 4}},
	}
	raw, err := json.Marshal(ownerUsageStatsResponse(stats))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, 5.0, out["total_cost"])
	require.Equal(t, 2.5, out["total_earnings"])
	require.Equal(t, 4.0, out["pending_requests"])
	require.Equal(t, 10.0, out["total_requests"])
	for _, key := range []string{"total_actual_cost", "total_account_cost", "total_paid_cost", "total_gift_cost", "total_gift_account_cost"} {
		require.NotContains(t, out, key)
	}
	endpoints := out["endpoints"].([]any)
	require.Len(t, endpoints, 1)
	ep := endpoints[0].(map[string]any)
	require.Equal(t, 5.0, ep["cost"])
	require.NotContains(t, ep, "actual_cost")

	// 账本未填充时收益与待结算按 0 输出，不缺字段。
	raw, err = json.Marshal(ownerUsageStatsResponse(&usagestats.UsageStats{TotalCost: 1}))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, 0.0, out["total_earnings"])
	require.Equal(t, 0.0, out["pending_requests"])
}
