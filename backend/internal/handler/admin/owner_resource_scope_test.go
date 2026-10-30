//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ownerScopeTestService 按 ID 返回带归属的资源；未登记的资源视为不存在。
type ownerScopeTestService struct {
	service.AdminService
	accounts, proxies, groups map[int64]*int64
	// accountDetails 为账户补充当前分组、代理与凭证；ownerLookups 记录批量归属查询次数。
	accountDetails map[int64]service.Account
	backupProxies  map[int64]int64
	ownerLookups   int
}

func ownerOf(id int64) *int64 { return &id }

func (s *ownerScopeTestService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	owner, ok := s.accounts[id]
	if !ok {
		return nil, service.ErrAccountNotFound
	}
	account := s.accountDetails[id]
	account.ID, account.OwnerUserID = id, owner
	return &account, nil
}

func (s *ownerScopeTestService) ResourceOwnerIDs(_ context.Context, kind string, ids []int64) (map[int64]*int64, error) {
	s.ownerLookups++
	source := map[string]map[int64]*int64{"account": s.accounts, "proxy": s.proxies, "group": s.groups}[kind]
	out := map[int64]*int64{}
	for _, id := range ids {
		if owner, ok := source[id]; ok {
			out[id] = owner
		}
	}
	return out, nil
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
	p := &service.Proxy{ID: id, OwnerUserID: owner}
	if backup, ok := s.backupProxies[id]; ok {
		p.BackupProxyID = &backup
	}
	return p, nil
}

func (s *ownerScopeTestService) GetGroup(_ context.Context, id int64) (*service.Group, error) {
	owner, ok := s.groups[id]
	if !ok {
		return nil, service.ErrGroupNotFound
	}
	return &service.Group{ID: id, OwnerUserID: owner}, nil
}

func newOwnerScopeRouter(t *testing.T, ownerID int64) (*gin.Engine, *[]string) {
	router, reached, _ := newOwnerScopeRouterWithService(t, ownerID)
	return router, reached
}

func newOwnerScopeRouterWithService(t *testing.T, ownerID int64) (*gin.Engine, *[]string, *ownerScopeTestService) {
	gin.SetMode(gin.TestMode)
	// 7 号所有者拥有账户 1/4/5、代理 1、分组 1；2 号资源属于其他所有者，3 号为管理员存量资源。
	// 账户 5 已被管理员绑定到分组 3 与代理 3；账户 4 的 base_url 指向回环地址（规则生效前保存）；
	// 代理 1 的备用代理被管理员设为代理 3。
	svc := &ownerScopeTestService{
		accounts: map[int64]*int64{1: ownerOf(7), 2: ownerOf(8), 3: nil, 4: ownerOf(7), 5: ownerOf(7)},
		proxies:  map[int64]*int64{1: ownerOf(7), 2: ownerOf(8), 3: nil},
		groups:   map[int64]*int64{1: ownerOf(7), 2: ownerOf(8), 3: nil},
		accountDetails: map[int64]service.Account{
			4: {Credentials: map[string]any{"base_url": "http://127.0.0.1:5432"}},
			5: {GroupIDs: []int64{1, 3}, ProxyID: ownerOf(3)},
		},
		backupProxies: map[int64]int64{1: 3},
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
		{"GET", "/api/v1/admin/accounts"}, {"POST", "/api/v1/admin/accounts/:id/test"}, {"PUT", "/api/v1/admin/proxies/:id"},
	} {
		router.Handle(route.method, route.path, record)
	}
	return router, &reached, svc
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
		// encoding/json 字段名大小写不敏感：非规范写法的引用字段必须拒绝，不能绕过校验。
		{"POST", "/api/v1/admin/accounts", `{"GROUP_IDS":[3]}`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/accounts", `{"Proxy_Id":3}`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/accounts", `{"proxy_id":1,"PROXY_ID":3}`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/accounts", `{"group_idſ":[3]}`, http.StatusBadRequest},
		{"PUT", "/api/v1/admin/accounts/1", `{"Group_IDs":[3]}`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/accounts/today-stats/batch", `{"Account_IDs":[3]}`, http.StatusBadRequest},
		{"POST", "/api/v1/admin/groups", `{"name":"g","Model_Routing":{"claude-*":[3]}}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, ownerScopeCall(router, tc.method, tc.path, tc.body), tc.method+" "+tc.path+" "+tc.body)
	}
	// 校验后请求体原样交给处理器。
	require.Contains(t, *reached, `POST /api/v1/admin/accounts {"group_ids":[1],"proxy_id":1}`)
}

// 管理员已为所有者账户绑定的分组/代理（及代理的备用代理）属于保持不变的引用，编辑表单原样回传时放行；
// 新增的他人资源引用仍拒绝。
func TestOwnerResourceScopeAllowsUnchangedAdminReferences(t *testing.T) {
	router, _ := newOwnerScopeRouter(t, 7)
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"PUT", "/api/v1/admin/accounts/5", `{"name":"x","group_ids":[1,3],"proxy_id":3}`, http.StatusOK},
		{"PUT", "/api/v1/admin/accounts/5", `{"group_ids":[1]}`, http.StatusOK},
		{"PUT", "/api/v1/admin/accounts/5", `{"group_ids":[1,3,2]}`, http.StatusNotFound},
		{"PUT", "/api/v1/admin/accounts/1", `{"group_ids":[3]}`, http.StatusNotFound},
		{"PUT", "/api/v1/admin/accounts/1", `{"proxy_id":3}`, http.StatusNotFound},
		{"PUT", "/api/v1/admin/proxies/1", `{"backup_proxy_id":3}`, http.StatusOK},
		{"PUT", "/api/v1/admin/proxies/1", `{"backup_proxy_id":2}`, http.StatusNotFound},
		// 其他接口不继承已绑定引用的放行。
		{"POST", "/api/v1/admin/accounts", `{"group_ids":[3]}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, ownerScopeCall(router, tc.method, tc.path, tc.body), tc.method+" "+tc.path+" "+tc.body)
	}
}

// 账户列表的 group 筛选只能是本人分组、0 或 ungrouped，避免探测管理员分组。
func TestOwnerResourceScopeValidatesAccountGroupFilter(t *testing.T) {
	router, _ := newOwnerScopeRouter(t, 7)
	for query, want := range map[string]int{
		"":                 http.StatusOK,
		"?group=1":         http.StatusOK,
		"?group=0":         http.StatusOK,
		"?group=ungrouped": http.StatusOK,
		"?group=3":         http.StatusNotFound,
		"?group=2":         http.StatusNotFound,
		"?group=404":       http.StatusNotFound,
		"?group=abc":       http.StatusNotFound,
	} {
		require.Equal(t, want, ownerScopeCall(router, "GET", "/api/v1/admin/accounts"+query, ""), query)
	}
}

// 归属按类批量查询：一次请求引用多个同类资源只查询一次，超过上限直接拒绝。
func TestOwnerResourceScopeBatchesOwnershipLookup(t *testing.T) {
	router, _, svc := newOwnerScopeRouterWithService(t, 7)
	require.Equal(t, http.StatusOK, ownerScopeCall(router, "POST", "/api/v1/admin/accounts/today-stats/batch", `{"account_ids":[1,4,5,1]}`))
	require.Equal(t, 1, svc.ownerLookups)

	ids := make([]string, ownerMaxReferencedIDs+1)
	for i := range ids {
		ids[i] = "1"
	}
	require.Equal(t, http.StatusBadRequest, ownerScopeCall(router, "POST", "/api/v1/admin/accounts/today-stats/batch", `{"account_ids":[`+strings.Join(ids, ",")+`]}`))
}

// 即时出站接口在执行前复查所有者账户地址，覆盖规则生效前保存的内网地址。
func TestOwnerResourceScopeRechecksOutboundBeforeAccountTest(t *testing.T) {
	router, reached := newOwnerScopeRouter(t, 7)
	require.Equal(t, http.StatusBadRequest, ownerScopeCall(router, "POST", "/api/v1/admin/accounts/4/test", `{}`))
	require.Equal(t, http.StatusOK, ownerScopeCall(router, "POST", "/api/v1/admin/accounts/1/test", `{}`))
	require.Len(t, *reached, 1)

	admin, _ := newOwnerScopeRouter(t, 0)
	require.Equal(t, http.StatusOK, ownerScopeCall(admin, "POST", "/api/v1/admin/accounts/4/test", `{}`))
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
	out, ok := adminUsageLogResponses(c, records, nil).([]gin.H)
	require.True(t, ok)
	require.Len(t, out, 1)
	for _, key := range []string{"user_id", "api_key_id", "ip_address", "user_agent", "session_id", "subscription_id", "user", "api_key"} {
		require.NotContains(t, out[0], key)
	}
	require.Equal(t, int64(1), out[0]["account_id"])
	require.Equal(t, 2.0, out[0]["total_cost"])
	// 所有者看不到用户计费金额与倍率。
	for _, key := range []string{"actual_cost", "rate_multiplier", "account_rate_multiplier", "account_stats_cost"} {
		require.NotContains(t, out[0], key)
	}
}

// 守护拒绝非规范键名的前提：处理器绑定用的 encoding/json 确实按大小写不敏感
// （含 Unicode 折叠）匹配字段，忽略这些键会放过越权引用。
func TestOwnerScopeJSONFieldMatchingIsCaseInsensitive(t *testing.T) {
	var req CreateAccountRequest
	require.NoError(t, json.Unmarshal([]byte(`{"GROUP_IDS":[3],"Proxy_Id":3}`), &req))
	require.Equal(t, []int64{3}, req.GroupIDs)
	require.NotNil(t, req.ProxyID)
	req = CreateAccountRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{"group_idſ":[4]}`), &req))
	require.Equal(t, []int64{4}, req.GroupIDs)
}

// 管理员把所有者账户加入共享分组后，所有者只能看到该分组的引用，看不到倍率等定价配置；
// group_ids 必须保留，否则编辑表单保存时会丢掉共享分组。
func TestOwnerAccountResponseHidesSharedGroupConfig(t *testing.T) {
	svc := &ownerScopeTestService{groups: map[int64]*int64{1: ownerOf(7), 2: nil}}
	h := &AccountHandler{adminService: svc}
	ctx := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()})
	account := &dto.Account{
		ID: 9, GroupIDs: []int64{1, 2},
		Groups: []*dto.Group{
			{ID: 1, Name: "own", Platform: "anthropic", RateMultiplier: 1, Status: "active"},
			{ID: 2, Name: "shared", Platform: "anthropic", RateMultiplier: 3.5, Status: "active"},
		},
		AccountGroups: []dto.AccountGroup{
			{AccountID: 9, GroupID: 1, Group: &dto.Group{ID: 1, Name: "own", RateMultiplier: 1}},
			{AccountID: 9, GroupID: 2, Group: &dto.Group{ID: 2, Name: "shared", RateMultiplier: 3.5}},
		},
	}
	item := AccountWithConcurrency{Account: account, ownerForeignGroups: h.ownerForeignGroupIDs(ctx, account)}
	require.Equal(t, map[int64]struct{}{2: {}}, item.ownerForeignGroups)

	raw, err := json.Marshal(item)
	require.NoError(t, err)
	var out struct {
		GroupIDs      []int64          `json:"group_ids"`
		Groups        []map[string]any `json:"groups"`
		AccountGroups []struct {
			GroupID int64          `json:"group_id"`
			Group   map[string]any `json:"group"`
		} `json:"account_groups"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, []int64{1, 2}, out.GroupIDs)
	require.Len(t, out.Groups, 2)
	require.Equal(t, 1.0, out.Groups[0]["rate_multiplier"])
	require.Equal(t, "shared", out.Groups[1]["name"])
	require.NotContains(t, out.Groups[1], "rate_multiplier")
	require.Len(t, out.AccountGroups, 2)
	require.Contains(t, out.AccountGroups[0].Group, "rate_multiplier")
	require.Equal(t, int64(2), out.AccountGroups[1].GroupID)
	require.NotContains(t, out.AccountGroups[1].Group, "rate_multiplier")
	require.NotContains(t, string(raw), "3.5")
}

func TestOwnerForeignGroupIDsScope(t *testing.T) {
	account := &dto.Account{Groups: []*dto.Group{{ID: 1}, {ID: 2}}}
	h := &AccountHandler{adminService: &ownerScopeTestService{groups: map[int64]*int64{1: ownerOf(7), 2: ownerOf(8)}}}
	// 管理员请求不裁剪。
	require.Nil(t, h.ownerForeignGroupIDs(context.Background(), account))
	ctx := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()})
	// 其他所有者的分组同样视为他人分组；未登记（已删除）的分组也一样。
	require.Equal(t, map[int64]struct{}{2: {}, 3: {}}, h.ownerForeignGroupIDs(ctx, account, &dto.Account{AccountGroups: []dto.AccountGroup{{GroupID: 3}}}))
	// 归属查询失败时全部视为他人分组。
	h.adminService = ownerLookupFailingService{}
	require.Equal(t, map[int64]struct{}{1: {}, 2: {}}, h.ownerForeignGroupIDs(ctx, account))
}

// 重新授权回写 Extra 前须校验出站地址，失败直接返回且不写入任何内容。
func TestApplyOAuthCredentialsRejectsOwnerInternalCustomBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := newStubAdminService()
	stub.getAccountResult = &service.Account{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, OwnerUserID: ownerOf(7)}
	h := NewAccountHandler(stub, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		principal := &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()}
		c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), principal))
	})
	router.POST("/accounts/:id/apply-oauth-credentials", h.ApplyOAuthCredentials)

	code := ownerScopeCall(router, http.MethodPost, "/accounts/1/apply-oauth-credentials",
		`{"type":"oauth","credentials":{"access_token":"t"},"extra":{"custom_base_url":"http://127.0.0.1:8443","custom_base_url_enabled":true}}`)
	require.Equal(t, http.StatusBadRequest, code)
	require.Zero(t, stub.updateAccountCalls)
	require.Zero(t, stub.updateAccountExtraCalls)
}

type ownerLookupFailingService struct{ service.AdminService }

func (ownerLookupFailingService) ResourceOwnerIDs(context.Context, string, []int64) (map[int64]*int64, error) {
	return nil, service.ErrOwnerResourceLookup
}
