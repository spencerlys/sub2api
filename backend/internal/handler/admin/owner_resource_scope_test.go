//go:build unit

package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ownerScopeTestService 按 ID 返回带归属的资源；未登记的资源视为不存在。
type ownerScopeTestService struct {
	service.AdminService
	accounts, proxies, groups map[int64]*int64
}

func ownerOf(id int64) *int64 { return &id }

func (s *ownerScopeTestService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	owner, ok := s.accounts[id]
	if !ok {
		return nil, service.ErrAccountNotFound
	}
	return &service.Account{ID: id, OwnerUserID: owner}, nil
}

func (s *ownerScopeTestService) GetAccountsByIDs(ctx context.Context, ids []int64) ([]*service.Account, error) {
	out := []*service.Account{}
	for _, id := range ids {
		if a, err := s.GetAccount(ctx, id); err == nil {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *ownerScopeTestService) GetProxy(_ context.Context, id int64) (*service.Proxy, error) {
	owner, ok := s.proxies[id]
	if !ok {
		return nil, service.ErrProxyNotFound
	}
	return &service.Proxy{ID: id, OwnerUserID: owner}, nil
}

func (s *ownerScopeTestService) GetGroup(_ context.Context, id int64) (*service.Group, error) {
	owner, ok := s.groups[id]
	if !ok {
		return nil, service.ErrGroupNotFound
	}
	return &service.Group{ID: id, OwnerUserID: owner}, nil
}

func newOwnerScopeRouter(t *testing.T, ownerID int64) (*gin.Engine, *[]string) {
	gin.SetMode(gin.TestMode)
	// 7 号所有者拥有账户 1、代理 1、分组 1；2 号资源属于其他所有者，3 号为管理员存量资源。
	svc := &ownerScopeTestService{
		accounts: map[int64]*int64{1: ownerOf(7), 2: ownerOf(8), 3: nil},
		proxies:  map[int64]*int64{1: ownerOf(7), 2: ownerOf(8), 3: nil},
		groups:   map[int64]*int64{1: ownerOf(7), 2: ownerOf(8), 3: nil},
	}
	h := &AccountHandler{adminService: svc}
	reached := []string{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if ownerID > 0 {
			principal := &adminaccess.Principal{OwnerUserID: ownerID, Policy: adminaccess.OwnerPolicy()}
			c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), principal))
		}
	}, h.OwnerResourceScope)
	record := func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		reached = append(reached, c.Request.Method+" "+c.FullPath()+" "+string(body))
		c.Status(http.StatusOK)
	}
	for _, route := range []struct{ method, path string }{
		{"PUT", "/api/v1/admin/accounts/:id"}, {"POST", "/api/v1/admin/accounts"},
		{"POST", "/api/v1/admin/accounts/today-stats/batch"}, {"POST", "/api/v1/admin/groups"},
		{"GET", "/api/v1/admin/groups/:id/model-allowlist-candidates"}, {"POST", "/api/v1/admin/proxies/:id/test"},
		{"GET", "/api/v1/admin/usage"}, {"POST", "/api/v1/admin/scheduled-test-plans/:id"},
	} {
		router.Handle(route.method, route.path, record)
	}
	return router, &reached
}

func ownerScopeCall(router *gin.Engine, method, path, body string) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w.Code
}

func TestOwnerResourceScopeRejectsForeignResources(t *testing.T) {
	router, reached := newOwnerScopeRouter(t, 7)
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"PUT", "/api/v1/admin/accounts/1", `{"name":"mine"}`, http.StatusOK},
		{"PUT", "/api/v1/admin/accounts/2", `{"name":"x"}`, http.StatusNotFound},
		{"PUT", "/api/v1/admin/accounts/3", `{"name":"x"}`, http.StatusNotFound},
		{"PUT", "/api/v1/admin/accounts/404", `{}`, http.StatusNotFound},
		{"PUT", "/api/v1/admin/accounts/abc", `{}`, http.StatusForbidden},
		{"POST", "/api/v1/admin/proxies/2/test", ``, http.StatusNotFound},
		{"POST", "/api/v1/admin/proxies/1/test", ``, http.StatusOK},
		{"POST", "/api/v1/admin/accounts", `{"group_ids":[1],"proxy_id":1}`, http.StatusOK},
		{"POST", "/api/v1/admin/accounts", `{"group_ids":[1,3]}`, http.StatusNotFound},
		{"POST", "/api/v1/admin/accounts", `{"proxy_id":2}`, http.StatusNotFound},
		{"POST", "/api/v1/admin/accounts", `{"proxy_id":0,"group_ids":null}`, http.StatusOK},
		{"POST", "/api/v1/admin/accounts", `{"proxy_id":"1"}`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/accounts", `[1]`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/accounts/today-stats/batch", `{"account_ids":[1,2]}`, http.StatusNotFound},
		{"POST", "/api/v1/admin/accounts/today-stats/batch", `{"account_ids":[1]}`, http.StatusOK},
		{"POST", "/api/v1/admin/groups", `{"name":"g","model_routing":{"claude-*":[1,3]}}`, http.StatusNotFound},
		{"POST", "/api/v1/admin/groups", `{"name":"g","model_routing":{"claude-*":[1]}}`, http.StatusOK},
		{"GET", "/api/v1/admin/groups/0/model-allowlist-candidates", ``, http.StatusOK},
		{"GET", "/api/v1/admin/groups/2/model-allowlist-candidates", ``, http.StatusNotFound},
		{"GET", "/api/v1/admin/usage?account_id=1", ``, http.StatusOK},
		{"GET", "/api/v1/admin/usage?account_id=2", ``, http.StatusNotFound},
		{"GET", "/api/v1/admin/usage?group_id=3", ``, http.StatusNotFound},
		{"GET", "/api/v1/admin/usage?user_id=5", ``, http.StatusForbidden},
		{"GET", "/api/v1/admin/usage?api_key_id=5", ``, http.StatusForbidden},
		// 未登记归属前缀的 :id 路由一律拒绝。
		{"POST", "/api/v1/admin/scheduled-test-plans/1", `{}`, http.StatusForbidden},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, ownerScopeCall(router, tc.method, tc.path, tc.body), tc.method+" "+tc.path+" "+tc.body)
	}
	// 校验后请求体原样交给处理器。
	require.Contains(t, *reached, `POST /api/v1/admin/accounts {"group_ids":[1],"proxy_id":1}`)
}

func TestOwnerResourceScopeIgnoresAdmins(t *testing.T) {
	router, reached := newOwnerScopeRouter(t, 0)
	require.Equal(t, http.StatusOK, ownerScopeCall(router, "PUT", "/api/v1/admin/accounts/2", `{"group_ids":[3]}`))
	require.Equal(t, http.StatusOK, ownerScopeCall(router, "GET", "/api/v1/admin/usage?user_id=5", ``))
	require.Len(t, *reached, 2)
}

func TestSanitizeGroupRequestForOwnerKeepsOnlySafeFields(t *testing.T) {
	rate, limit, fallback := 0.1, 5.0, int64(3)
	create := CreateGroupRequest{
		Name: "g", Description: "d", Platform: service.PlatformAnthropic, RateMultiplier: rate, IsExclusive: false,
		SubscriptionType: service.SubscriptionTypeSubscription, PeakRateEnabled: true, ProfitControlEnabled: true,
		ImageRateMultiplier: &rate, WebSearchPricePerCall: &limit, FallbackGroupID: &fallback,
		CopyAccountsFromGroupIDs: []int64{3}, ModelPricing: []service.ChannelModelPricing{{}}, FreeOpenAIFast: true,
		ClaudeCodeOnly: true, RPMLimit: 30, ModelRouting: map[string][]int64{"a": {1}}, ModelRoutingEnabled: true,
	}
	require.NoError(t, create.DailyLimitUSD.UnmarshalJSON([]byte("5")))
	require.True(t, sanitizeCreateGroupRequestForOwner(&create))
	require.Equal(t, CreateGroupRequest{
		Name: "g", Description: "d", Platform: service.PlatformAnthropic, RateMultiplier: 1, IsExclusive: true,
		SubscriptionType: service.SubscriptionTypeStandard, ClaudeCodeOnly: true, RPMLimit: 30,
		ModelRouting: map[string][]int64{"a": {1}}, ModelRoutingEnabled: true,
	}, create)
	require.Nil(t, create.DailyLimitUSD.ToServiceInput())

	composite := CreateGroupRequest{Name: "c", Platform: service.PlatformComposite}
	require.False(t, sanitizeCreateGroupRequestForOwner(&composite))

	notExclusive := false
	status := "inactive"
	update := UpdateGroupRequest{Name: "n", Status: status, RateMultiplier: &rate, IsExclusive: &notExclusive, FallbackGroupID: &fallback, ImageRateMultiplier: &rate}
	require.NoError(t, update.MonthlyLimitUSD.UnmarshalJSON([]byte("null")))
	require.True(t, sanitizeUpdateGroupRequestForOwner(&update))
	require.Nil(t, update.RateMultiplier)
	require.Nil(t, update.FallbackGroupID)
	require.Nil(t, update.ImageRateMultiplier)
	require.Nil(t, update.MonthlyLimitUSD.ToServiceInput())
	require.NotNil(t, update.IsExclusive)
	require.True(t, *update.IsExclusive)
	require.Equal(t, "n", update.Name)
	require.Equal(t, status, update.Status)
	require.False(t, sanitizeUpdateGroupRequestForOwner(&UpdateGroupRequest{Platform: service.PlatformComposite}))
}

func TestOwnerUsageResponseHidesUserIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage", nil)
	principal := &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()}
	c.Request = req.WithContext(adminaccess.WithPrincipal(req.Context(), principal))
	ip, ua := "203.0.113.9", "curl/8"
	records := []service.UsageLog{{
		ID: 1, UserID: 5, APIKeyID: 6, AccountID: 1, TotalCost: 2, ActualCost: 3, IPAddress: &ip, UserAgent: &ua,
		User: &service.User{ID: 5, Email: "victim@example.com"}, APIKey: &service.APIKey{ID: 6, Key: "sk-secret"},
	}}
	out, ok := adminUsageLogResponses(c, records).([]gin.H)
	require.True(t, ok)
	require.Len(t, out, 1)
	for _, key := range []string{"user_id", "api_key_id", "ip_address", "user_agent", "session_id", "subscription_id", "user", "api_key"} {
		require.NotContains(t, out[0], key)
	}
	require.Equal(t, int64(1), out[0]["account_id"])
	require.Equal(t, 2.0, out[0]["total_cost"])
	require.Equal(t, 3.0, out[0]["actual_cost"])
}
