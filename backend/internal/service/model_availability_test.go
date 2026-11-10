//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// cc2 线上 3 个账号同步到的真实列表（2026-11-11）。
var probedAnthropicModels = []any{
	"claude-fable-5", "claude-fable-5-1", "claude-fable-5-2", "claude-haiku-4-5-20251001", "claude-haiku-5-5",
	"claude-opus-4-5-20251101", "claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8", "claude-opus-5",
	"claude-opus-5-5", "claude-sonnet-4-6", "claude-sonnet-5", "claude-sonnet-5-5",
}

func anthropicAvailabilityAccount(id int64, models []any) *Account {
	a := &Account{ID: id, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{}}
	if models != nil {
		a.Extra[UpstreamModelsExtraKey] = models
	}
	return a
}

func withoutModel(models []any, drop string) []any {
	out := make([]any, 0, len(models))
	for _, m := range models {
		if m != drop {
			out = append(out, m)
		}
	}
	return out
}

func availabilityGateway(enabled bool) *GatewayService {
	return &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{ModelAvailabilityRoutingEnabled: enabled}}}
}

func TestModelAvailabilityFilter(t *testing.T) {
	withFable := anthropicAvailabilityAccount(1, probedAnthropicModels)
	noFable := anthropicAvailabilityAccount(2, withoutModel(probedAnthropicModels, "claude-fable-5-2"))
	noList := anthropicAvailabilityAccount(3, nil)
	gw := availabilityGateway(true)
	ctx := context.Background()

	// 列表里没有该模型的账号不参与调度（选号、粘性复用、404 诊断都经过这个函数）。
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, withFable, "claude-fable-5-2"))
	require.False(t, gw.isModelSupportedByAccountWithContext(ctx, noFable, "claude-fable-5-2"))
	require.False(t, gw.isModelSupportedByAccountWithContext(ctx, noFable, "claude-fable-5-2[1m]"))
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, noFable, "claude-opus-5-5"))
	// 短名与带日期的长名等价。
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, noFable, "claude-haiku-4-5"))

	// 从未同步成功的账号保持原行为。
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, noList, "claude-fable-5-2"))
	// 别名/自定义名不过滤。
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, noFable, "claude-opus-latest"))
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, noFable, "opus"))

	// 开关关闭时完全不过滤。
	require.True(t, availabilityGateway(false).isModelSupportedByAccountWithContext(ctx, noFable, "claude-fable-5-2"))
	require.True(t, (&GatewayService{}).isModelSupportedByAccountWithContext(ctx, noFable, "claude-fable-5-2"))
}

func TestModelAvailabilityFilterHonoursModelMappingTarget(t *testing.T) {
	gw := availabilityGateway(true)
	ctx := context.Background()
	// API Key 账号把别名映射到 fable-5-2：按映射目标判断。
	a := anthropicAvailabilityAccount(4, withoutModel(probedAnthropicModels, "claude-fable-5-2"))
	a.Type = AccountTypeAPIKey
	a.Credentials = map[string]any{"model_mapping": map[string]any{"best": "claude-fable-5-2", "fast": "claude-haiku-5-5"}}
	require.False(t, gw.isModelSupportedByAccountWithContext(ctx, a, "best"))
	require.True(t, gw.isModelSupportedByAccountWithContext(ctx, a, "fast"))
}

func TestModelAvailabilityIgnoresOtherPlatforms(t *testing.T) {
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{UpstreamModelsExtraKey: []any{"gpt-x"}}}
	require.True(t, a.UpstreamModelAvailable("claude-fable-5-2"))
	bedrock := anthropicAvailabilityAccount(5, []any{"claude-opus-5-5"})
	bedrock.Type = AccountTypeBedrock
	require.True(t, bedrock.UpstreamModelAvailable("claude-fable-5-2"))
}

func TestIsAnthropicModelEntitlementRejection(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"oauth disabled for org is account level", http.StatusForbidden,
			`{"type":"error","error":{"type":"permission_error","message":"OAuth authentication is currently not allowed for this organization."}}`, false},
		{"404 not_found model", http.StatusNotFound,
			`{"type":"error","error":{"type":"not_found_error","message":"model: claude-fable-5-2"}}`, true},
		{"403 no access to model", http.StatusForbidden,
			`{"type":"error","error":{"type":"permission_error","message":"Your organization does not have access to model claude-fable-5-2"}}`, true},
		{"400 model not available on plan", http.StatusBadRequest,
			`{"type":"error","error":{"type":"invalid_request_error","message":"claude-fable-5-2 is not available on your current plan"}}`, true},
		{"400 unrelated validation echoing model", http.StatusBadRequest,
			`{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: 200000 > 128000, which is the maximum allowed number of output tokens for claude-fable-5-2"}}`, false},
		{"400 request-shape not supported is not entitlement", http.StatusBadRequest,
			`{"type":"error","error":{"type":"invalid_request_error","message":"thinking.type disabled is not supported for model claude-fable-5-2"}}`, false},
		{"organization disabled 400", http.StatusBadRequest,
			`{"type":"error","error":{"type":"invalid_request_error","message":"This organization has been disabled."}}`, false},
		{"429 never", http.StatusTooManyRequests,
			`{"type":"error","error":{"type":"rate_limit_error","message":"model claude-fable-5-2 not available"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isAnthropicModelEntitlementRejection(tc.status, []byte(tc.body), "claude-fable-5-2"))
		})
	}
}

// availabilityRepoStub 记录 SetError / SetModelRateLimit / UpdateExtra 调用。
type availabilityRepoStub struct {
	mockAccountRepoForGemini
	setErrors  []int64
	rateLimits []modelNotFoundRateLimitCall
	extraCalls []map[string]any
	listed     []Account
}

func (r *availabilityRepoStub) SetError(ctx context.Context, id int64, msg string) error {
	r.setErrors = append(r.setErrors, id)
	return nil
}

func (r *availabilityRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	call := modelNotFoundRateLimitCall{accountID: id, scope: scope, resetAt: resetAt}
	if len(reason) > 0 {
		call.reason = reason[0]
	}
	r.rateLimits = append(r.rateLimits, call)
	return nil
}

func (r *availabilityRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	r.extraCalls = append(r.extraCalls, updates)
	return nil
}

func (r *availabilityRepoStub) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.listed, nil
}

func TestHandleUpstreamError_ModelEntitlementPausesModelNotAccount(t *testing.T) {
	repo := &availabilityRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	next := time.Now().Add(10 * time.Hour).UTC().Truncate(time.Second)
	account := anthropicAvailabilityAccount(46, probedAnthropicModels)
	account.Extra[UpstreamModelsNextSyncAtExtraKey] = next.Format(time.RFC3339)

	failover := svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"type":"error","error":{"type":"permission_error","message":"Your organization does not have access to model claude-fable-5-2"}}`),
		"claude-fable-5-2")

	require.True(t, failover, "must fail over to another account")
	require.Empty(t, repo.setErrors, "account must not be marked error")
	require.Len(t, repo.rateLimits, 1)
	require.Equal(t, "claude-fable-5-2", repo.rateLimits[0].scope)
	require.Equal(t, upstreamModelUnavailableReason, repo.rateLimits[0].reason)
	require.WithinDuration(t, next, repo.rateLimits[0].resetAt, time.Second, "paused until the next daily sync")
	require.Len(t, repo.extraCalls, 1)
	trimmed := repo.extraCalls[0][UpstreamModelsExtraKey].([]string)
	require.NotContains(t, trimmed, "claude-fable-5-2")
	require.Contains(t, trimmed, "claude-opus-5-5")
}

func TestHandleUpstreamError_ModelEntitlement404AndNoListFallsBackTo24h(t *testing.T) {
	repo := &availabilityRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := anthropicAvailabilityAccount(47, nil)

	failover := svc.HandleUpstreamError(context.Background(), account, http.StatusNotFound, http.Header{},
		[]byte(`{"type":"error","error":{"type":"not_found_error","message":"model: claude-fable-5-2"}}`), "claude-fable-5-2")

	require.True(t, failover)
	require.Empty(t, repo.setErrors)
	require.Len(t, repo.rateLimits, 1)
	require.WithinDuration(t, time.Now().Add(24*time.Hour), repo.rateLimits[0].resetAt, 5*time.Second)
	require.Empty(t, repo.extraCalls, "no stored list to trim")
}

func TestHandleUpstreamError_OrgLevel403StillDisablesAccount(t *testing.T) {
	repo := &availabilityRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := anthropicAvailabilityAccount(45, probedAnthropicModels)

	failover := svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"type":"error","error":{"type":"permission_error","message":"OAuth authentication is currently not allowed for this organization."}}`),
		"claude-opus-5-5")

	require.True(t, failover)
	require.Equal(t, []int64{45}, repo.setErrors, "org-level 403 keeps the account-level behaviour")
	require.Empty(t, repo.rateLimits)
}

func TestHandleUpstreamError_Unrelated400Unchanged(t *testing.T) {
	repo := &availabilityRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := anthropicAvailabilityAccount(28, probedAnthropicModels)

	failover := svc.HandleUpstreamError(context.Background(), account, http.StatusBadRequest, http.Header{},
		[]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages: at least one message is required"}}`),
		"claude-fable-5-2")

	require.False(t, failover)
	require.Empty(t, repo.setErrors)
	require.Empty(t, repo.rateLimits)
	require.Empty(t, repo.extraCalls)
}

// --- 同步任务 ---

type availabilityFetcherStub struct {
	models map[int64][]string
	errs   map[int64]error
	calls  []int64
}

func (f *availabilityFetcherStub) FetchUpstreamSupportedModels(ctx context.Context, a *Account) ([]string, error) {
	f.calls = append(f.calls, a.ID)
	if err := f.errs[a.ID]; err != nil {
		return nil, err
	}
	return f.models[a.ID], nil
}

func fixedJitter(fraction float64) func(time.Duration) time.Duration {
	return func(max time.Duration) time.Duration { return time.Duration(float64(max) * fraction) }
}

func TestModelAvailabilitySchedule(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

	// 成功后 24~27 小时，失败后 1~3 小时，首次 0~15 分钟。
	for _, f := range []float64{0, 0.5, 0.999} {
		ok := modelAvailabilityNextAfterSuccess(now, fixedJitter(f))
		require.True(t, !ok.Before(now.Add(24*time.Hour)) && ok.Before(now.Add(27*time.Hour)), ok)
		fail := modelAvailabilityNextAfterFailure(now, fixedJitter(f))
		require.True(t, !fail.Before(now.Add(time.Hour)) && fail.Before(now.Add(3*time.Hour)), fail)
	}
	for i := 0; i < 200; i++ {
		ok := modelAvailabilityNextAfterSuccess(now, uniformJitter)
		require.True(t, !ok.Before(now.Add(24*time.Hour)) && ok.Before(now.Add(27*time.Hour)))
	}

	fresh := anthropicAvailabilityAccount(1, nil)
	due, first := planModelAvailabilitySync(fresh, now, fixedJitter(0.4))
	require.False(t, due)
	require.NotNil(t, first)
	require.Equal(t, now.Add(6*time.Minute), *first)

	scheduled := anthropicAvailabilityAccount(2, probedAnthropicModels)
	scheduled.Extra[UpstreamModelsNextSyncAtExtraKey] = now.Add(time.Minute).Format(time.RFC3339)
	due, first = planModelAvailabilitySync(scheduled, now, uniformJitter)
	require.False(t, due)
	require.Nil(t, first)
	due, _ = planModelAvailabilitySync(scheduled, now.Add(time.Minute), uniformJitter)
	require.True(t, due)
}

func TestModelAvailabilityRunOnce(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute).Format(time.RFC3339)

	dueOK := *anthropicAvailabilityAccount(28, withoutModel(probedAnthropicModels, "claude-fable-5-2"))
	dueOK.Extra[UpstreamModelsNextSyncAtExtraKey] = past
	dueSame := *anthropicAvailabilityAccount(36, probedAnthropicModels)
	dueSame.Extra[UpstreamModelsNextSyncAtExtraKey] = past
	dueFail := *anthropicAvailabilityAccount(48, probedAnthropicModels)
	dueFail.Extra[UpstreamModelsNextSyncAtExtraKey] = past
	notDue := *anthropicAvailabilityAccount(29, probedAnthropicModels)
	notDue.Extra[UpstreamModelsNextSyncAtExtraKey] = now.Add(time.Hour).Format(time.RFC3339)
	firstSight := *anthropicAvailabilityAccount(50, nil)
	errored := *anthropicAvailabilityAccount(45, nil)
	errored.Status = StatusError
	bedrock := *anthropicAvailabilityAccount(60, nil)
	bedrock.Type = AccountTypeBedrock

	var fullList []string
	for _, m := range probedAnthropicModels {
		fullList = append(fullList, m.(string))
	}
	repo := &availabilityRepoStub{listed: []Account{dueOK, dueSame, dueFail, notDue, firstSight, errored, bedrock}}
	fetcher := &availabilityFetcherStub{
		models: map[int64][]string{28: fullList, 36: fullList},
		errs:   map[int64]error{48: errors.New("proxy timeout")},
	}
	svc := NewModelAvailabilitySyncService(repo, fetcher)
	svc.now = func() time.Time { return now }
	svc.jitter = fixedJitter(0.5)
	svc.spacing = 0

	res, err := svc.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, ModelAvailabilityRunResult{Synced: 2, Failed: 1, Scheduled: 1}, res)
	require.ElementsMatch(t, []int64{28, 36, 48}, fetcher.calls, "only due, active, eligible accounts are fetched")

	byKeys := map[string]map[string]any{}
	for _, c := range repo.extraCalls {
		switch {
		case c[UpstreamModelsLastErrorExtraKey] == "proxy timeout":
			byKeys["fail"] = c
		case c[UpstreamModelsSyncedAtExtraKey] != nil && c[UpstreamModelsExtraKey] != nil:
			byKeys["changed"] = c
		case c[UpstreamModelsSyncedAtExtraKey] != nil:
			byKeys["same"] = c
		default:
			byKeys["first"] = c
		}
	}
	// 28 列表变了（新增 fable-5-2）→ 写入列表；36 没变 → 只写时间戳。
	require.Contains(t, byKeys["changed"][UpstreamModelsExtraKey], "claude-fable-5-2")
	require.Equal(t, now.Add(25*time.Hour+30*time.Minute).Format(time.RFC3339), byKeys["changed"][UpstreamModelsNextSyncAtExtraKey])
	require.NotContains(t, byKeys["same"], UpstreamModelsExtraKey)
	// 失败：保留旧列表（不写 upstream_models），2 小时后重试。
	require.NotContains(t, byKeys["fail"], UpstreamModelsExtraKey)
	require.Equal(t, now.Add(2*time.Hour).Format(time.RFC3339), byKeys["fail"][UpstreamModelsNextSyncAtExtraKey])
	// 首次见到：只排期（7.5 分钟后），不同步。
	require.Equal(t, now.Add(450*time.Second).Format(time.RFC3339), byKeys["first"][UpstreamModelsNextSyncAtExtraKey])
}

func TestUpdateAccountKeepsUpstreamModelList(t *testing.T) {
	const accountID = int64(36)
	stored := map[string]any{
		UpstreamModelsExtraKey:           []any{"claude-opus-5-5", "claude-fable-5-2"},
		UpstreamModelsSyncedAtExtraKey:   "2026-11-11T00:00:00Z",
		UpstreamModelsNextSyncAtExtraKey: "2026-11-12T01:00:00Z",
	}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		accountID: {ID: accountID, Name: "mkyc", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive,
			Credentials: map[string]any{"access_token": "keep"}, Extra: stored},
	}}
	// 表单注入伪造列表、或打开时不带这些键，都不能改变本地列表。
	for _, input := range []map[string]any{
		{"note": "x", UpstreamModelsExtraKey: []any{"claude-opus-5-5"}, UpstreamModelsNextSyncAtExtraKey: "2030-01-01T00:00:00Z"},
		{"note": "y"},
	} {
		got, err := (&adminServiceImpl{accountRepo: repo}).UpdateAccount(context.Background(), accountID, &UpdateAccountInput{Extra: input})
		require.NoError(t, err)
		require.Equal(t, stored[UpstreamModelsExtraKey], got.Extra[UpstreamModelsExtraKey])
		require.Equal(t, stored[UpstreamModelsNextSyncAtExtraKey], got.Extra[UpstreamModelsNextSyncAtExtraKey])
	}
}
