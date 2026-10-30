//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type ownerSettlementRepoStub struct {
	owners       []OwnerIdentity
	usage        []OwnerSettlementUsage
	active       map[int64]int64
	payouts      map[int64]float64
	finance      *FinancePeriodRaw
	consumption  *FinanceConsumption
	records      map[int64]*RedeemCode
	ownerFilters []*int64
	usageRange   [2]time.Time
	created      *OwnerPayout
	deleted      int64
	updatedNotes map[int64]string
}

func filterOwner(id *int64, owner int64) bool { return id == nil || *id == owner }

func (s *ownerSettlementRepoStub) ListSettlementOwners(_ context.Context, ownerUserID *int64) ([]OwnerIdentity, error) {
	s.ownerFilters = append(s.ownerFilters, ownerUserID)
	var out []OwnerIdentity
	for _, o := range s.owners {
		if filterOwner(ownerUserID, o.UserID) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *ownerSettlementRepoStub) AggregateOwnerSettlement(_ context.Context, start, end time.Time, ownerUserID *int64) ([]OwnerSettlementUsage, error) {
	s.usageRange = [2]time.Time{start, end}
	var out []OwnerSettlementUsage
	for _, u := range s.usage {
		if filterOwner(ownerUserID, u.OwnerUserID) {
			out = append(out, u)
		}
	}
	return out, nil
}

func (s *ownerSettlementRepoStub) CountActiveOwnerAccounts(_ context.Context, ownerUserID *int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for id, n := range s.active {
		if filterOwner(ownerUserID, id) {
			out[id] = n
		}
	}
	return out, nil
}

func (s *ownerSettlementRepoStub) AggregateFinanceConsumption(context.Context, time.Time, time.Time) (*FinanceConsumption, error) {
	return s.consumption, nil
}

func (s *ownerSettlementRepoStub) SumOwnerPayouts(_ context.Context, ownerUserID *int64) (map[int64]float64, error) {
	out := map[int64]float64{}
	for id, v := range s.payouts {
		if filterOwner(ownerUserID, id) {
			out[id] = v
		}
	}
	return out, nil
}

func (s *ownerSettlementRepoStub) ListOwnerPayouts(context.Context, OwnerPayoutFilter, pagination.PaginationParams) ([]OwnerPayout, int64, error) {
	return nil, 0, nil
}

func (s *ownerSettlementRepoStub) CreateOwnerPayout(_ context.Context, p *OwnerPayout) error {
	p.ID = 99
	s.created = p
	return nil
}

func (s *ownerSettlementRepoStub) DeleteOwnerPayout(_ context.Context, id int64) error {
	if id != 5 {
		return ErrOwnerPayoutNotFound
	}
	s.deleted = id
	return nil
}

func (s *ownerSettlementRepoStub) AggregateFinancePeriod(context.Context, time.Time, time.Time) (*FinancePeriodRaw, error) {
	return s.finance, nil
}

func (s *ownerSettlementRepoStub) GetBalanceRecord(_ context.Context, id int64) (*RedeemCode, error) {
	if r, ok := s.records[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, ErrRedeemCodeNotFound
}

func (s *ownerSettlementRepoStub) UpdateBalanceRecordNotes(_ context.Context, id int64, notes string) error {
	if s.updatedNotes == nil {
		s.updatedNotes = map[int64]string{}
	}
	s.updatedNotes[id] = notes
	return nil
}

func settlementFixture() *ownerSettlementRepoStub {
	return &ownerSettlementRepoStub{
		owners: []OwnerIdentity{
			{UserID: 10, Email: "owner-a@example.com", Username: "a"},
			{UserID: 20, Email: "owner-b@example.com", Username: "b"},
			{UserID: 30, Email: "owner-idle@example.com", Username: "idle"},
		},
		usage: []OwnerSettlementUsage{
			{OwnerUserID: 10, Requests: 6, OfficialCost: 18, UserBilled: 21, PaidCost: 9, GiftCost: 12, AccountCost: 9,
				Earnings: 4, GiftAccountCost: 5, PendingRequests: 0, EarningsAllTime: 40},
			{OwnerUserID: 20, Requests: 1, OfficialCost: 20, UserBilled: 20, PaidCost: 10, GiftCost: 5, AccountCost: 8,
				Earnings: 3, GiftAccountCost: 1, PendingRequests: 1, PendingAccountCost: 4, EarningsAllTime: 12},
		},
		active:  map[int64]int64{10: 2, 20: 1},
		payouts: map[int64]float64{10: 25, 20: 2},
	}
}

func testPeriod(start, end time.Time) ReportPeriod {
	return ReportPeriod{Range: ReportRangeCustom, Start: start, End: end, Location: time.UTC}
}

func TestOwnerSettlementReportUsesExactSplit(t *testing.T) {
	repo := settlementFixture()
	svc := NewOwnerSettlementService(repo, nil)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	report, err := svc.GetOwnerSettlements(context.Background(), testPeriod(start, end), nil)
	require.NoError(t, err)
	require.Len(t, report.Items, 3)
	require.Equal(t, [2]time.Time{start, end}, repo.usageRange)
	require.Equal(t, ReportRangeCustom, report.Range)
	require.Equal(t, "UTC", report.Timezone)
	require.NotNil(t, report.Ledger)
	require.Equal(t, int64(-1), report.Ledger.LagSeconds, "no ledger: never ran")

	a := report.Items[0]
	require.Equal(t, int64(10), a.OwnerUserID)
	require.Equal(t, "owner-a@example.com", a.Email)
	require.Equal(t, int64(2), a.ActiveAccounts)
	require.Equal(t, int64(6), a.Requests)
	require.InDelta(t, 9, a.PaidCost, 1e-9)
	require.InDelta(t, 12, a.GiftCost, 1e-9)
	require.InDelta(t, 4, a.Earnings, 1e-9)
	require.InDelta(t, 5, a.GiftAccountCost, 1e-9)
	require.InDelta(t, 15, a.Unpaid, 1e-9, "unpaid = all-time earnings − payouts; gift-funded account cost is never payable")

	b := report.Items[1]
	require.Equal(t, int64(1), b.PendingRequests)
	require.InDelta(t, 4, b.PendingAccountCost, 1e-9)
	require.InDelta(t, 10, b.Unpaid, 1e-9)

	idle := report.Items[2]
	require.Equal(t, int64(30), idle.OwnerUserID)
	require.Zero(t, idle.Requests)
	require.Zero(t, idle.Unpaid)

	tot := report.Totals
	require.Equal(t, int64(3), tot.ActiveAccounts)
	require.Equal(t, int64(7), tot.Requests)
	require.InDelta(t, 38, tot.OfficialCost, 1e-9)
	require.InDelta(t, 41, tot.UserBilled, 1e-9)
	require.InDelta(t, 19, tot.PaidCost, 1e-9)
	require.InDelta(t, 17, tot.GiftCost, 1e-9)
	require.InDelta(t, 17, tot.AccountCost, 1e-9)
	require.InDelta(t, 7, tot.Earnings, 1e-9)
	require.InDelta(t, 6, tot.GiftAccountCost, 1e-9)
	require.Equal(t, int64(1), tot.PendingRequests)
	require.InDelta(t, 52, tot.EarningsAllTime, 1e-9)
	require.InDelta(t, 27, tot.PaidTotal, 1e-9)
	require.InDelta(t, 25, tot.Unpaid, 1e-9)
	require.Equal(t, start, report.StartTime)
	require.Equal(t, end, report.EndTime)
}

// 所有者上下文即使调用管理员报表也被强制限定为本人。
func TestOwnerSettlementOwnerContextForcesOwnFilter(t *testing.T) {
	repo := settlementFixture()
	svc := NewOwnerSettlementService(repo, nil)
	ctx := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: 20, Policy: adminaccess.OwnerPolicy()})
	other := int64(10)
	report, err := svc.GetOwnerSettlements(ctx, testPeriod(time.Now().Add(-time.Hour), time.Now()), &other)
	require.NoError(t, err)
	require.Len(t, report.Items, 1)
	require.Equal(t, int64(20), report.Items[0].OwnerUserID)
}

func TestOwnerSettlementSelfHidesBillingInternals(t *testing.T) {
	repo := settlementFixture()
	svc := NewOwnerSettlementService(repo, nil)
	me, err := svc.GetMySettlement(context.Background(), 20, testPeriod(time.Now().Add(-time.Hour), time.Now()))
	require.NoError(t, err)
	require.Equal(t, int64(1), me.Requests)
	require.InDelta(t, 20, me.OfficialCost, 1e-9)
	require.InDelta(t, 3, me.Earnings, 1e-9)
	require.Equal(t, int64(1), me.PendingRequests)
	require.InDelta(t, 10, me.Unpaid, 1e-9)
	require.NotNil(t, me.Ledger)

	raw, err := json.Marshal(me)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	for _, forbidden := range []string{"user_billed", "account_cost", "paid_cost", "gift_cost", "gift_account_cost", "email", "username", "owner_user_id", "paid_share", "items"} {
		require.NotContains(t, fields, forbidden)
	}

	_, err = svc.GetMySettlement(context.Background(), 0, testPeriod(time.Now().Add(-time.Hour), time.Now()))
	require.Error(t, err)
}

func TestOwnerPayoutCreateValidatesAndDeletes(t *testing.T) {
	repo := settlementFixture()
	svc := NewOwnerSettlementService(repo, nil)
	ctx := context.Background()
	paidAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	_, err := svc.CreateOwnerPayout(ctx, OwnerPayoutInput{OwnerUserID: 10, Amount: 0}, 1)
	require.Error(t, err)
	_, err = svc.CreateOwnerPayout(ctx, OwnerPayoutInput{OwnerUserID: 999, Amount: 5}, 1)
	require.Error(t, err, "payee must be a settlement owner")
	require.Equal(t, 400, infraerrors.Code(err))

	p, err := svc.CreateOwnerPayout(ctx, OwnerPayoutInput{OwnerUserID: 10, Amount: 12.5, PaidAt: &paidAt, Notes: "  bank  "}, 1)
	require.NoError(t, err)
	require.Equal(t, int64(99), p.ID)
	require.Equal(t, paidAt, repo.created.PaidAt)
	require.Equal(t, "bank", repo.created.Notes)
	require.NotNil(t, repo.created.CreatedBy)
	require.Equal(t, int64(1), *repo.created.CreatedBy)

	p, err = svc.CreateOwnerPayout(ctx, OwnerPayoutInput{OwnerUserID: 20, Amount: 1}, 0)
	require.NoError(t, err)
	require.False(t, p.PaidAt.IsZero(), "paid_at defaults to now")
	require.Nil(t, p.CreatedBy)

	require.NoError(t, svc.DeleteOwnerPayout(ctx, 5))
	require.ErrorIs(t, svc.DeleteOwnerPayout(ctx, 6), ErrOwnerPayoutNotFound)
}

func TestUpdateBalanceRecordSource(t *testing.T) {
	user := int64(7)
	other := int64(8)
	repo := &ownerSettlementRepoStub{records: map[int64]*RedeemCode{
		1: {ID: 1, Type: AdjustmentTypeAdminBalance, UsedBy: &user, Notes: "[src:gift] wire 42", Value: 10},
		2: {ID: 2, Type: RedeemTypeBalance, UsedBy: &user, Notes: "", Value: 10},
		3: {ID: 3, Type: AdjustmentTypeAdminBalance, UsedBy: &other, Notes: "x", Value: 10},
		4: {ID: 4, Type: AdjustmentTypeAdminBalance, UsedBy: &user, Notes: "legacy", Value: 10},
	}}
	svc := NewOwnerSettlementService(repo, nil)
	ctx := context.Background()

	rec, err := svc.UpdateBalanceRecordSource(ctx, user, 1, "offline")
	require.NoError(t, err)
	require.Equal(t, "[src:offline] wire 42", repo.updatedNotes[1])
	require.Equal(t, BalanceSourceOffline, rec.Source)
	require.Equal(t, "wire 42", rec.Notes)

	_, err = svc.UpdateBalanceRecordSource(ctx, user, 4, "correction")
	require.NoError(t, err)
	require.Equal(t, "[src:correction] legacy", repo.updatedNotes[4])

	_, err = svc.UpdateBalanceRecordSource(ctx, user, 2, "offline")
	require.Error(t, err, "only admin_balance records carry a source")
	require.Equal(t, 400, infraerrors.Code(err))
	_, err = svc.UpdateBalanceRecordSource(ctx, user, 3, "offline")
	require.Equal(t, 404, infraerrors.Code(err), "record of another user")
	_, err = svc.UpdateBalanceRecordSource(ctx, user, 404, "offline")
	require.Equal(t, 404, infraerrors.Code(err))
	_, err = svc.UpdateBalanceRecordSource(ctx, user, 1, "paid")
	require.Equal(t, 400, infraerrors.Code(err))
	require.Len(t, repo.updatedNotes, 2)
}

func TestFinanceSummaryBuckets(t *testing.T) {
	repo := &ownerSettlementRepoStub{finance: &FinancePeriodRaw{
		OnlineOrders: 3,
		PayAmounts:   []FinanceCurrencyAmount{{Currency: "CNY", Amount: 700}, {Currency: "USD", Amount: 20}},
		Credits: []FinanceCreditBucket{
			{Kind: CreditKindPayment, Count: 3, Positive: 120, Bonus: 15},
			{Kind: CreditKindAdmin, Source: "offline", Count: 2, Positive: 300, Negative: -40},
			{Kind: CreditKindAdmin, Source: "", Count: 4, Positive: 25, Negative: -5},
			{Kind: CreditKindAdmin, Source: "gift", Count: 1, Positive: 5},
			{Kind: CreditKindAdmin, Source: "correction", Count: 2, Positive: 10, Negative: -3},
			{Kind: CreditKindPromo, Count: 1, Positive: 2},
			{Kind: CreditKindRedeemCode, Count: 1, Positive: 6},
			{Kind: CreditKindAffiliate, Count: 1, Positive: 1.5},
		},
	}}
	svc := NewOwnerSettlementService(repo, nil)
	repo.consumption = &FinanceConsumption{Requests: 3, UserBilled: 9, PaidCost: 6, GiftCost: 2, PendingRequests: 1, PendingUserBilled: 1}
	s, err := svc.GetFinanceSummary(context.Background(), testPeriod(time.Now().Add(-time.Hour), time.Now()))
	require.NoError(t, err)
	require.Equal(t, *repo.consumption, s.Consumption)
	require.NotNil(t, s.Ledger)
	require.Equal(t, int64(3), s.Online.Orders)
	require.Len(t, s.Online.PayAmounts, 2)
	require.InDelta(t, 105, s.Online.CreditedUSD, 1e-9)
	require.InDelta(t, 15, s.Online.BonusUSD, 1e-9)
	require.Equal(t, int64(2), s.Offline.Count)
	require.InDelta(t, 300, s.Offline.USD, 1e-9)
	require.InDelta(t, 30, s.Gift.AdminGiftUSD, 1e-9)
	require.InDelta(t, 15, s.Gift.PaymentBonusUSD, 1e-9)
	require.InDelta(t, 2, s.Gift.PromoUSD, 1e-9)
	require.InDelta(t, 6, s.Gift.RedeemCodeUSD, 1e-9)
	require.InDelta(t, 1.5, s.Gift.AffiliateUSD, 1e-9)
	require.InDelta(t, 54.5, s.Gift.TotalUSD, 1e-9)
	require.Equal(t, int64(2), s.Corrections.Count)
	require.InDelta(t, 7, s.Corrections.NetUSD, 1e-9)
	require.InDelta(t, -45, s.Deductions.USD, 1e-9, "negative non-correction adjustments are reported separately")
}
