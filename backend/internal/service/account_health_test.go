//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/stretchr/testify/require"
)

type accountHealthRepoStub struct {
	accounts    []Account
	usage       map[int64]*AccountHealthUsage
	errs        map[int64]*AccountHealthErrors
	ownerFilter *int64
	usageCalls  [][]int64
	errorCalls  [][]int64
	start, end  time.Time
	labels      map[int64]OwnerIdentity
	labelCalls  [][]int64
}

func (s *accountHealthRepoStub) ListAccountOwnerLabels(_ context.Context, ownerIDs []int64) (map[int64]OwnerIdentity, error) {
	s.labelCalls = append(s.labelCalls, ownerIDs)
	return s.labels, nil
}

func (s *accountHealthRepoStub) ListAccountHealthAccounts(_ context.Context, ownerUserID *int64) ([]Account, error) {
	s.ownerFilter = ownerUserID
	return s.accounts, nil
}

func (s *accountHealthRepoStub) AggregateAccountHealthUsage(_ context.Context, ids []int64, start, end time.Time) (map[int64]*AccountHealthUsage, error) {
	s.usageCalls = append(s.usageCalls, ids)
	s.start, s.end = start, end
	return s.usage, nil
}

func (s *accountHealthRepoStub) AggregateAccountHealthErrors(_ context.Context, ids []int64, _, _ time.Time) (map[int64]*AccountHealthErrors, error) {
	s.errorCalls = append(s.errorCalls, ids)
	return s.errs, nil
}

func TestParseAccountHealthRange(t *testing.T) {
	for raw, want := range map[string]time.Duration{"": 24 * time.Hour, "1h": time.Hour, " 24H ": 24 * time.Hour, "7d": 7 * 24 * time.Hour} {
		_, d, err := ParseAccountHealthRange(raw)
		require.NoError(t, err, raw)
		require.Equal(t, want, d, raw)
	}
	for _, raw := range []string{"30d", "8d", "2h", "-1h", "abc"} {
		_, _, err := ParseAccountHealthRange(raw)
		require.ErrorIs(t, err, ErrAccountHealthInvalidRange, raw)
	}
}

func TestAccountHealthErrorRate(t *testing.T) {
	require.Zero(t, AccountHealthErrorRate(0, 0))
	require.InDelta(t, 0.1, AccountHealthErrorRate(90, 10), 1e-12)
	require.InDelta(t, 1.0, AccountHealthErrorRate(0, 3), 1e-12)
	require.Zero(t, AccountHealthErrorRate(5, 0))
}

func TestAccountHealthService_MergesAggregatesInOneQueryEach(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Minute), now.Add(-time.Minute)
	p50, p95, d50 := 300, 900, 2000
	last := now.Add(-time.Hour)
	status := 429
	repo := &accountHealthRepoStub{
		accounts: []Account{
			{ID: 1, Name: "a", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, RateLimitResetAt: &future, OverloadUntil: &past, ErrorMessage: "upstream 429"},
			{ID: 2, Name: "idle", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive},
		},
		usage: map[int64]*AccountHealthUsage{1: {RequestsOK: 90, TokensIn: 10, TokensOut: 20, TokensCache: 30, Cost: 1.5, TTFTP50Ms: &p50, TTFTP95Ms: &p95, DurationP50Ms: &d50, LastSuccessAt: &last}},
		errs:  map[int64]*AccountHealthErrors{1: {Total: 10, ByCategory: AccountHealthErrorCounts{RateLimit: 7, Timeout: 3}, LastErrorAt: &last, LastErrorCategory: AccountHealthErrorRateLimit, LastErrorStatus: &status}},
	}
	svc := NewAccountHealthService(repo)
	svc.now = func() time.Time { return now }

	report, err := svc.GetAccountHealth(context.Background(), "1h", nil)
	require.NoError(t, err)
	require.Equal(t, "1h", report.Range)
	require.Equal(t, now.Add(-time.Hour), report.StartTime)
	require.Equal(t, now, report.EndTime)
	require.Equal(t, [][]int64{{1, 2}}, repo.usageCalls)
	require.Equal(t, [][]int64{{1, 2}}, repo.errorCalls)
	require.Equal(t, now.Add(-time.Hour), repo.start)

	require.Len(t, report.Items, 2)
	a := report.Items[0]
	require.True(t, a.RateLimited)
	require.False(t, a.Overloaded)
	require.False(t, a.TempUnschedulable)
	require.Equal(t, "upstream 429", a.ErrorMessage)
	require.EqualValues(t, 90, a.RequestsOK)
	require.EqualValues(t, 10, a.ErrorsTotal)
	require.InDelta(t, 0.1, a.ErrorRate, 1e-12)
	require.Equal(t, AccountHealthErrorCounts{RateLimit: 7, Timeout: 3}, a.Errors)
	require.Equal(t, &p50, a.TTFTP50Ms)
	require.Equal(t, &p95, a.TTFTP95Ms)
	require.Equal(t, &d50, a.DurationP50Ms)
	require.Equal(t, AccountHealthErrorRateLimit, a.LastErrorCategory)
	require.Equal(t, &status, a.LastErrorStatus)

	idle := report.Items[1]
	require.Zero(t, idle.RequestsOK)
	require.Zero(t, idle.ErrorsTotal)
	require.Zero(t, idle.ErrorRate)
	require.Nil(t, idle.TTFTP50Ms)
	require.Nil(t, idle.LastSuccessAt)
}

func TestAccountHealthService_InvalidRangeSkipsQueries(t *testing.T) {
	repo := &accountHealthRepoStub{}
	_, err := NewAccountHealthService(repo).GetAccountHealth(context.Background(), "30d", nil)
	require.ErrorIs(t, err, ErrAccountHealthInvalidRange)
	require.Nil(t, repo.usageCalls)
}

func TestAccountHealthService_NoAccountsSkipsAggregates(t *testing.T) {
	repo := &accountHealthRepoStub{}
	report, err := NewAccountHealthService(repo).GetAccountHealth(context.Background(), "", nil)
	require.NoError(t, err)
	require.Equal(t, "24h", report.Range)
	require.NotNil(t, report.Items)
	require.Empty(t, report.Items)
	require.Nil(t, repo.usageCalls)
	require.Nil(t, repo.errorCalls)
}

// 所有者请求的筛选一律改写为本人，管理员的筛选原样传递。
func TestAccountHealthService_OwnerFilterForcedToSelf(t *testing.T) {
	repo := &accountHealthRepoStub{}
	other := int64(9)
	ownerCtx := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()})
	_, err := NewAccountHealthService(repo).GetAccountHealth(ownerCtx, "24h", &other)
	require.NoError(t, err)
	require.NotNil(t, repo.ownerFilter)
	require.EqualValues(t, 7, *repo.ownerFilter)

	_, err = NewAccountHealthService(repo).GetAccountHealth(context.Background(), "24h", &other)
	require.NoError(t, err)
	require.EqualValues(t, 9, *repo.ownerFilter)

	_, err = NewAccountHealthService(repo).GetAccountHealth(context.Background(), "24h", nil)
	require.NoError(t, err)
	require.Nil(t, repo.ownerFilter)
}

// 每账户的官方价、账户成本与应付（精确账本）；用户计费金额、账户成本与所有者身份只给管理员。
func TestAccountHealthService_SettlementFieldsAndOwnerLabels(t *testing.T) {
	ownerID := int64(7)
	repo := &accountHealthRepoStub{
		accounts: []Account{
			{ID: 1, Name: "owned", Status: StatusActive, OwnerUserID: &ownerID},
			{ID: 2, Name: "admin-own", Status: StatusActive},
		},
		usage: map[int64]*AccountHealthUsage{
			1: {RequestsOK: 3, Cost: 1.6, OfficialCost: 2, AccountCost: 1, Earnings: 0.75, PendingRequests: 1},
			2: {RequestsOK: 1, Cost: 0.5, OfficialCost: 0.5, AccountCost: 0.5, Earnings: 0.5},
		},
		labels: map[int64]OwnerIdentity{7: {UserID: 7, Email: "owner@example.com", Username: "o"}},
	}
	svc := NewAccountHealthService(repo)

	report, err := svc.GetAccountHealth(context.Background(), "24h", nil)
	require.NoError(t, err)
	owned, unowned := report.Items[0], report.Items[1]
	require.NotNil(t, owned.Cost)
	require.InDelta(t, 1.6, *owned.Cost, 1e-12)
	require.InDelta(t, 2, owned.OfficialCost, 1e-12)
	require.NotNil(t, owned.AccountCost)
	require.InDelta(t, 0.75, owned.Earnings, 1e-12)
	require.EqualValues(t, 1, owned.PendingRequests)
	require.Equal(t, &ownerID, owned.OwnerUserID)
	require.Equal(t, "owner@example.com", owned.OwnerEmail)
	require.Equal(t, "o", owned.OwnerUsername)
	require.Zero(t, unowned.Earnings, "accounts without an owner have nothing payable")
	require.Nil(t, unowned.OwnerUserID)
	require.Equal(t, [][]int64{{7}}, repo.labelCalls)

	ownerCtx := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()})
	repo.accounts = repo.accounts[:1]
	repo.labelCalls = nil
	report, err = svc.GetAccountHealth(ownerCtx, "24h", nil)
	require.NoError(t, err)
	item := report.Items[0]
	require.Nil(t, item.Cost, "owners never see what users were billed")
	require.Nil(t, item.AccountCost)
	require.Empty(t, item.OwnerEmail)
	require.InDelta(t, 2, item.OfficialCost, 1e-12)
	require.InDelta(t, 0.75, item.Earnings, 1e-12)
	require.Nil(t, repo.labelCalls, "owner requests never load owner identities")
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	for _, forbidden := range []string{`"cost"`, `"account_cost"`, `"owner_email"`, `"owner_username"`} {
		require.NotContains(t, string(raw), forbidden)
	}
}
