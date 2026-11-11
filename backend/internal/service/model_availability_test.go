//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func anthropicAvailabilityAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{}}
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

func TestHandleUpstreamError_ModelEntitlementPausesModelNotAccount(t *testing.T) {
	bodies := map[int]string{
		http.StatusForbidden:  `{"type":"error","error":{"type":"permission_error","message":"Your organization does not have access to model claude-fable-5-2"}}`,
		http.StatusNotFound:   `{"type":"error","error":{"type":"not_found_error","message":"model: claude-fable-5-2"}}`,
		http.StatusBadRequest: `{"type":"error","error":{"type":"invalid_request_error","message":"claude-fable-5-2 is not available on your current plan"}}`,
	}
	for status, body := range bodies {
		t.Run(http.StatusText(status), func(t *testing.T) {
			repo := &availabilityRepoStub{}
			svc := &RateLimitService{accountRepo: repo}
			account := anthropicAvailabilityAccount(46)

			failover := svc.HandleUpstreamError(context.Background(), account, status, http.Header{}, []byte(body), "claude-fable-5-2")

			require.True(t, failover, "must fail over to another account")
			require.Empty(t, repo.setErrors, "account must not be marked error")
			require.Len(t, repo.rateLimits, 1)
			require.Equal(t, "claude-fable-5-2", repo.rateLimits[0].scope)
			require.Equal(t, upstreamModelUnavailableReason, repo.rateLimits[0].reason)
			require.WithinDuration(t, time.Now().Add(24*time.Hour), repo.rateLimits[0].resetAt, 5*time.Second, "paused for a fixed 24h")
			require.Empty(t, repo.extraCalls, "no stored model list exists any more")
		})
	}
}

func TestHandleUpstreamError_OrgLevel403StillDisablesAccount(t *testing.T) {
	repo := &availabilityRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := anthropicAvailabilityAccount(45)

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
	account := anthropicAvailabilityAccount(28)

	failover := svc.HandleUpstreamError(context.Background(), account, http.StatusBadRequest, http.Header{},
		[]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages: at least one message is required"}}`),
		"claude-fable-5-2")

	require.False(t, failover)
	require.Empty(t, repo.setErrors)
	require.Empty(t, repo.rateLimits)
	require.Empty(t, repo.extraCalls)
}

func TestModelEntitlementPauseIgnoresOtherPlatforms(t *testing.T) {
	require.False(t, modelAvailabilityEligible(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}))
	require.False(t, modelAvailabilityEligible(&Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}))
	require.True(t, modelAvailabilityEligible(anthropicAvailabilityAccount(1)))
}
