//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// 只使用显式指定的隔离测试数据库（OWNER_SETTLEMENT_TEST_DSN），每次创建并清理独立 schema。
// 只建结算与账本查询涉及的列；owner_payouts 与成本账本使用真实迁移 252、253。
func ownerSettlementTestDB(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	dsn := os.Getenv("OWNER_SETTLEMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("需要独立 OWNER_SETTLEMENT_TEST_DSN")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("owner_settlement_test_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = base.Close()
	})
	_, err = db.Exec(`
CREATE TABLE users(id BIGINT PRIMARY KEY, email TEXT NOT NULL, username TEXT NOT NULL DEFAULT '', role TEXT NOT NULL DEFAULT 'user', deleted_at TIMESTAMPTZ);
CREATE TABLE accounts(id BIGINT PRIMARY KEY, owner_user_id BIGINT, status TEXT NOT NULL DEFAULT 'active', schedulable BOOLEAN NOT NULL DEFAULT TRUE, deleted_at TIMESTAMPTZ);
CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, account_id BIGINT NOT NULL, group_id BIGINT,
  billing_type SMALLINT NOT NULL DEFAULT 0,
  total_cost DECIMAL(20,10) NOT NULL, actual_cost DECIMAL(20,10) NOT NULL,
  account_rate_multiplier DECIMAL(10,4), account_stats_cost DECIMAL(20,10), created_at TIMESTAMPTZ NOT NULL,
  input_tokens INT NOT NULL DEFAULT 0, output_tokens INT NOT NULL DEFAULT 0, cache_creation_tokens INT NOT NULL DEFAULT 0,
  cache_read_tokens INT NOT NULL DEFAULT 0, stream BOOLEAN NOT NULL DEFAULT FALSE, first_token_ms INT, duration_ms INT);
CREATE TABLE redeem_codes(id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, type TEXT NOT NULL, value DECIMAL(20,8) NOT NULL,
  status TEXT NOT NULL, used_by BIGINT, used_at TIMESTAMPTZ, notes TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE payment_orders(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, recharge_code TEXT NOT NULL, amount DECIMAL(20,2) NOT NULL,
  pay_amount DECIMAL(20,2) NOT NULL, bonus_amount DECIMAL(20,2) NOT NULL DEFAULT 0, order_type TEXT NOT NULL DEFAULT 'balance',
  status TEXT NOT NULL DEFAULT 'COMPLETED', provider_snapshot JSONB, subscription_group_id BIGINT,
  paid_at TIMESTAMPTZ, completed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE promo_code_usages(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, bonus_amount DECIMAL(20,8) NOT NULL, used_at TIMESTAMPTZ NOT NULL);
CREATE TABLE user_affiliate_ledger(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, action TEXT NOT NULL, amount DECIMAL(20,8) NOT NULL, created_at TIMESTAMPTZ NOT NULL);
`)
	require.NoError(t, err)
	for _, name := range []string{"252_owner_payouts.sql", "253_usage_cost_ledger.sql"} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		_, err = db.Exec(string(migration))
		require.NoError(t, err, name)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB("postgres", db)))
	return db, client
}

// 时间：1 月 15 日、2 月 10 日 12:00 UTC 及其后若干小时。
const ownerSettlementSeed = `
INSERT INTO users(id, email, username, role) VALUES
  (1, 'admin@example.com', 'admin', 'admin'),
  (10, 'owner-a@example.com', 'owner-a', 'owner'),
  (20, 'owner-b@example.com', 'owner-b', 'owner'),
  (101, 'gift@example.com', '', 'user'),
  (102, 'offline@example.com', '', 'user'),
  (103, 'mixed@example.com', '', 'user'),
  (104, 'nothing@example.com', '', 'user'),
  (105, 'sub@example.com', '', 'user');
INSERT INTO accounts(id, owner_user_id, status, schedulable) VALUES
  (1000, 10, 'active', TRUE), (2000, 20, 'disabled', TRUE), (2001, 20, 'active', TRUE), (2002, 20, 'active', FALSE), (3000, NULL, 'active', TRUE);

-- 101：管理员赠送（无前缀）+ 发放兑换码 + 优惠码 + 返利转入，全部为赠送（共 60）。
INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES
  ('ADJ-1', 'admin_balance', 50, 'used', 101, '2026-01-15 12:00:00+00', 'welcome'),
  ('CODE-1', 'balance', 5, 'used', 101, '2026-01-15 12:00:00+00', NULL),
  ('CODE-UNUSED', 'balance', 500, 'unused', NULL, NULL, NULL);
INSERT INTO promo_code_usages(user_id, bonus_amount, used_at) VALUES (101, 3, '2026-01-15 12:00:00+00');
INSERT INTO user_affiliate_ledger(user_id, action, amount, created_at) VALUES
  (101, 'transfer', 2, '2026-01-15 12:00:00+00'), (101, 'accrue', 99, '2026-01-15 12:00:00+00');

-- 102：线下实收 80 + 更正 +40（赠送桶）+ 线下冲销 −30（先减实付），均在 2 月 10 日；1 月已有一笔透支用量。
INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES
  ('ADJ-2', 'admin_balance', 80, 'used', 102, '2026-02-10 12:00:00+00', '[src:offline] bank'),
  ('ADJ-3', 'admin_balance', 40, 'used', 102, '2026-02-10 12:00:00+00', '[SRC:Correction] typo'),
  ('ADJ-4', 'admin_balance', -30, 'used', 102, '2026-02-10 12:00:00+00', '[src:offline] refund');

-- 103：1 月赠送 25；2 月在线支付到账 100（其中 bonus 25）→ 赠送 50、实付 75。
INSERT INTO payment_orders(user_id, recharge_code, amount, pay_amount, bonus_amount, provider_snapshot) VALUES
  (103, 'PAY-1-1', 100, 520, 25, '{"currency":"cny"}'),
  (103, 'PAY-1-2', 10, 10, 0, NULL);
INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES
  ('PAY-1-1', 'balance', 100, 'used', 103, '2026-02-10 12:00:00+00', NULL),
  ('ADJ-5', 'admin_balance', 25, 'used', 103, '2026-01-15 12:00:00+00', '[src:gift]'),
  ('PAY-1-2', 'balance', 10, 'unused', NULL, NULL, NULL);

-- 105：为分组 77 购买过实付订阅；分组 78 的订阅为分配所得。
INSERT INTO payment_orders(user_id, recharge_code, amount, pay_amount, order_type, status, subscription_group_id, completed_at) VALUES
  (105, 'SUB-1', 30, 30, 'subscription', 'COMPLETED', 77, '2026-01-15 12:00:00+00');

-- 用量：账户成本 = COALESCE(account_stats_cost,total_cost) × COALESCE(account_rate_multiplier,1)。
INSERT INTO usage_logs(user_id, account_id, group_id, billing_type, total_cost, actual_cost, account_rate_multiplier, account_stats_cost, created_at) VALUES
  (101, 1000, NULL, 0, 10, 12, 0.5, NULL, '2026-02-10 13:00:00+00'),
  (102, 1000, NULL, 0, 10, 11, 0.5, NULL, '2026-02-10 13:00:00+00'),
  (102, 1000, NULL, 0, 4, 4, NULL, 2, '2026-01-15 13:00:00+00'),
  (103, 2000, NULL, 0, 20, 20, NULL, NULL, '2026-02-10 13:00:00+00'),
  (103, 2000, NULL, 0, 40, 40, NULL, NULL, '2026-02-10 14:00:00+00'),
  (103, 3000, NULL, 0, 99, 99, NULL, NULL, '2026-02-10 15:00:00+00'),
  (104, 2000, NULL, 0, 6, 6, NULL, NULL, '2026-02-10 13:00:00+00'),
  (105, 2000, 77, 1, 5, 5, NULL, NULL, '2026-02-10 13:00:00+00'),
  (105, 2000, 78, 1, 2, 2, NULL, NULL, '2026-02-10 13:00:00+00');
INSERT INTO owner_payouts(owner_user_id, amount, paid_at, notes) VALUES (10, 1.5, '2026-01-20 00:00:00+00', 'jan');
INSERT INTO owner_payouts(owner_user_id, amount, paid_at, notes, deleted_at) VALUES (10, 100, '2026-01-20 00:00:00+00', 'void', NOW());
`

type pgSplit struct{ gift, paid string }

func readSplits(t *testing.T, db *sql.DB) map[int64]pgSplit {
	t.Helper()
	rows, err := db.Query(`SELECT usage_log_id, gift_cost, paid_cost, actual_cost FROM usage_cost_splits`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[int64]pgSplit{}
	for rows.Next() {
		var (
			id                 int64
			gift, paid, actual decimal.Decimal
		)
		require.NoError(t, rows.Scan(&id, &gift, &paid, &actual))
		require.True(t, gift.Add(paid).Equal(actual), "split of %d must add up", id)
		out[id] = pgSplit{gift.String(), paid.String()}
	}
	require.NoError(t, rows.Err())
	return out
}

func readLedgerStates(t *testing.T, db *sql.DB) map[int64]string {
	t.Helper()
	rows, err := db.Query(`SELECT user_id, gift_remaining, paid_remaining, credit_fp FROM usage_cost_ledger_users`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[int64]string{}
	for rows.Next() {
		var (
			id         int64
			gift, paid decimal.Decimal
			fp         string
		)
		require.NoError(t, rows.Scan(&id, &gift, &paid, &fp))
		out[id] = gift.String() + "/" + paid.String() + "/" + fp
	}
	require.NoError(t, rows.Err())
	return out
}

func usageID(t *testing.T, db *sql.DB, userID int64, at string, actual string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRow(`SELECT id FROM usage_logs WHERE user_id = $1 AND created_at = $2 AND actual_cost = $3`, userID, at, actual).Scan(&id))
	return id
}

func requirePGSplit(t *testing.T, splits map[int64]pgSplit, id int64, gift, paid string) {
	t.Helper()
	s, ok := splits[id]
	require.True(t, ok, "usage %d not split", id)
	require.True(t, decimal.RequireFromString(gift).Equal(decimal.RequireFromString(s.gift)), "usage %d gift: want %s got %s", id, gift, s.gift)
	require.True(t, decimal.RequireFromString(paid).Equal(decimal.RequireFromString(s.paid)), "usage %d paid: want %s got %s", id, paid, s.paid)
}

func febPeriod() service.ReportPeriod {
	return service.ReportPeriod{Range: service.ReportRangeCustom, Start: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Location: time.UTC}
}

func TestOwnerSettlementPostgresEndToEnd(t *testing.T) {
	db, client := ownerSettlementTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(ownerSettlementSeed)
	require.NoError(t, err)

	ledgerRepo := NewUsageCostLedgerRepository(db)
	ledger := service.NewUsageCostLedgerService(ledgerRepo, 0)
	svc := service.NewOwnerSettlementService(NewOwnerSettlementRepository(client, db), ledger)

	// 迁移 253 首次部署登记了一次全量重建：账本运行前全部为待结算。
	st, err := ledger.Status(ctx)
	require.NoError(t, err)
	require.True(t, st.RebuildPending)
	require.Equal(t, int64(-1), st.LagSeconds)
	report, err := svc.GetOwnerSettlements(ctx, febPeriod(), nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), report.Items[0].PendingRequests)
	require.Zero(t, report.Items[0].Earnings, "pending rows are never guessed")

	res, err := ledger.RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, res.Full)
	require.Zero(t, res.Errors)
	st, err = ledger.Status(ctx)
	require.NoError(t, err)
	require.False(t, st.RebuildPending)
	require.NotNil(t, st.LastRebuildAt)
	require.GreaterOrEqual(t, st.LagSeconds, int64(119))

	splits := readSplits(t, db)
	require.Len(t, splits, 9)
	const feb13, feb14, feb15, jan13 = "2026-02-10 13:00:00+00", "2026-02-10 14:00:00+00", "2026-02-10 15:00:00+00", "2026-01-15 13:00:00+00"
	requirePGSplit(t, splits, usageID(t, db, 101, feb13, "12"), "12", "0")
	requirePGSplit(t, splits, usageID(t, db, 102, jan13, "4"), "0", "4")   // 早于任何入账：透支记为实付
	requirePGSplit(t, splits, usageID(t, db, 102, feb13, "11"), "11", "0") // 更正 +40 进赠送桶，先被消耗
	requirePGSplit(t, splits, usageID(t, db, 103, feb13, "20"), "20", "0")
	requirePGSplit(t, splits, usageID(t, db, 103, feb14, "40"), "30", "10")
	requirePGSplit(t, splits, usageID(t, db, 103, feb15, "99"), "0", "99") // 实付 65 + 透支 34
	requirePGSplit(t, splits, usageID(t, db, 104, feb13, "6"), "0", "6")
	var subPaid, subGift int64
	require.NoError(t, db.QueryRow(`SELECT id FROM usage_logs WHERE user_id = 105 AND group_id = 77`).Scan(&subPaid))
	require.NoError(t, db.QueryRow(`SELECT id FROM usage_logs WHERE user_id = 105 AND group_id = 78`).Scan(&subGift))
	requirePGSplit(t, splits, subPaid, "0", "5")
	requirePGSplit(t, splits, subGift, "2", "0")
	states := readLedgerStates(t, db)
	require.Equal(t, "48", decimalPrefix(states[101]), "gift bucket of 101: 60 − 12")
	require.Contains(t, states[102], "/50/", "102: paid 80 − 30 offline reversal; gift 40 − 11 = 29")

	report, err = svc.GetOwnerSettlements(ctx, febPeriod(), nil)
	require.NoError(t, err)
	require.Len(t, report.Items, 2, "admin and plain users are not settlement owners")
	a := report.Items[0]
	require.Equal(t, int64(10), a.OwnerUserID)
	require.Equal(t, int64(1), a.ActiveAccounts)
	require.Equal(t, int64(2), a.Requests)
	require.InDelta(t, 20, a.OfficialCost, 1e-9)
	require.InDelta(t, 23, a.UserBilled, 1e-9)
	require.InDelta(t, 0, a.PaidCost, 1e-9)
	require.InDelta(t, 23, a.GiftCost, 1e-9)
	require.InDelta(t, 10, a.AccountCost, 1e-9)
	require.InDelta(t, 0, a.Earnings, 1e-9, "both February requests were funded by gift credit")
	require.InDelta(t, 10, a.GiftAccountCost, 1e-9)
	require.Zero(t, a.PendingRequests)
	require.InDelta(t, 2, a.EarningsAllTime, 1e-9, "January overdraft of 102 is paid: account cost 2")
	require.InDelta(t, 1.5, a.PaidTotal, 1e-9, "soft-deleted payouts are excluded")
	require.InDelta(t, 0.5, a.Unpaid, 1e-9)

	b := report.Items[1]
	require.Equal(t, int64(20), b.OwnerUserID)
	require.Equal(t, int64(1), b.ActiveAccounts, "only active and schedulable accounts")
	require.Equal(t, int64(5), b.Requests, "unowned account 3000 is excluded")
	require.InDelta(t, 73, b.UserBilled, 1e-9)
	require.InDelta(t, 21, b.PaidCost, 1e-9)
	require.InDelta(t, 52, b.GiftCost, 1e-9)
	require.InDelta(t, 73, b.AccountCost, 1e-9)
	require.InDelta(t, 21, b.Earnings, 1e-9, "10 (mixed) + 6 (overdraft) + 5 (paid subscription)")
	require.InDelta(t, 52, b.GiftAccountCost, 1e-9)
	require.InDelta(t, 21, b.Unpaid, 1e-9)
	require.Equal(t, int64(7), report.Totals.Requests)

	all, err := svc.ParsePeriod(service.ReportRangeAll, "", "")
	require.NoError(t, err)
	me, err := svc.GetMySettlement(ctx, 10, all)
	require.NoError(t, err)
	require.Equal(t, int64(3), me.Requests)
	require.InDelta(t, 2, me.Earnings, 1e-9)
	require.InDelta(t, me.EarningsAllTime, me.Earnings, 1e-9, "range=all equals the all-time figure")

	// 用量明细与统计视图：与结算同一口径。
	rowViews, err := ledgerRepo.(service.UsageCostSplitReader).GetUsageSplits(ctx, []int64{usageID(t, db, 103, feb14, "40"), usageID(t, db, 103, feb15, "99")})
	require.NoError(t, err)
	mixed := rowViews[usageID(t, db, 103, feb14, "40")]
	require.InDelta(t, 10, mixed.PaidCost, 1e-9)
	require.NotNil(t, mixed.Earnings)
	require.InDelta(t, 10, *mixed.Earnings, 1e-9)
	require.InDelta(t, 30, *mixed.GiftAccountCost, 1e-9)
	require.Nil(t, rowViews[usageID(t, db, 103, feb15, "99")].Earnings, "unowned account: no payable")
	feb := febPeriod()
	splitStats, err := ledgerRepo.(service.UsageCostSplitReader).UsageSplitStats(ctx, usagestats.UsageLogFilters{OwnerUserID: 20, StartTime: &feb.Start, EndTime: &feb.End})
	require.NoError(t, err)
	require.InDelta(t, 21, splitStats.PaidCost, 1e-9)
	require.InDelta(t, 52, splitStats.GiftCost, 1e-9)
	require.InDelta(t, 21, splitStats.Earnings, 1e-9)
	require.InDelta(t, 52, splitStats.GiftAccountCost, 1e-9)
	require.Zero(t, splitStats.PendingRequests)

	// 账号健康页的每账户应付。
	health, err := NewAccountHealthRepository(client, db).AggregateAccountHealthUsage(ctx, []int64{1000, 2000}, feb.Start, feb.End)
	require.NoError(t, err)
	require.InDelta(t, 73, health[2000].OfficialCost, 1e-9)
	require.InDelta(t, 21, health[2000].Earnings, 1e-9)
	require.InDelta(t, 73, health[2000].Cost, 1e-9)
	require.Zero(t, health[1000].Earnings)

	// 迟到的请求（时间早于水位）：显示为待结算，下一轮该用户重建。
	_, err = db.Exec(`INSERT INTO usage_logs(user_id, account_id, total_cost, actual_cost, created_at) VALUES (104, 2000, 1, 1, '2026-02-10 17:00:00+00')`)
	require.NoError(t, err)
	late := usageID(t, db, 104, "2026-02-10 17:00:00+00", "1")
	report, err = svc.GetOwnerSettlements(ctx, febPeriod(), nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), report.Items[1].PendingRequests)
	require.InDelta(t, 1, report.Items[1].PendingAccountCost, 1e-9)
	require.InDelta(t, 21, report.Items[1].Earnings, 1e-9)
	fin, err := svc.GetFinanceSummary(ctx, febPeriod())
	require.NoError(t, err)
	require.Equal(t, int64(1), fin.Consumption.PendingRequests)
	require.InDelta(t, 1, fin.Consumption.PendingUserBilled, 1e-9)

	res, err = ledger.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, service.LedgerRunResult{Users: 1, Rebuilt: 1}, res)
	requirePGSplit(t, readSplits(t, db), late, "0", "1")

	// 迟到的入账（时间早于已处理的请求）：该用户重建。
	_, err = db.Exec(`INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES ('ADJ-LATE', 'admin_balance', 3, 'used', 104, '2026-01-15 12:00:00+00', '[src:gift] late')`)
	require.NoError(t, err)
	res, err = ledger.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, service.LedgerRunResult{Users: 1, Rebuilt: 1}, res)
	splits = readSplits(t, db)
	requirePGSplit(t, splits, usageID(t, db, 104, feb13, "6"), "3", "3")
	requirePGSplit(t, splits, late, "0", "1")

	// 修改来源前缀（更正 → 线下实收）：历史入账指纹变化，该用户重建。
	var corrID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM redeem_codes WHERE code = 'ADJ-3'`).Scan(&corrID))
	_, err = svc.UpdateBalanceRecordSource(ctx, 102, corrID, "offline")
	require.NoError(t, err)
	res, err = ledger.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, service.LedgerRunResult{Users: 1, Rebuilt: 1}, res)
	requirePGSplit(t, readSplits(t, db), usageID(t, db, 102, feb13, "11"), "0", "11")

	// 无新事件：不处理任何用户。
	res, err = ledger.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, service.LedgerRunResult{}, res)

	// 增量结果 == 从头全量重建。
	incSplits, incStates := readSplits(t, db), readLedgerStates(t, db)
	res, err = ledger.RebuildAll(ctx)
	require.NoError(t, err)
	require.True(t, res.Full)
	require.Equal(t, incSplits, readSplits(t, db))
	require.Equal(t, incStates, readLedgerStates(t, db))

	// 用量被保留期清理后，拆分仍在并参与重建，结果不变。
	_, err = db.Exec(`DELETE FROM usage_logs WHERE user_id = 103 AND created_at = $1`, feb13)
	require.NoError(t, err)
	_, err = ledger.RebuildAll(ctx)
	require.NoError(t, err)
	require.Equal(t, incSplits, readSplits(t, db))
	require.Equal(t, incStates, readLedgerStates(t, db))

	// 财务汇总：入账按入账时间（二月），消耗按精确拆分。
	fin, err = svc.GetFinanceSummary(ctx, febPeriod())
	require.NoError(t, err)
	require.Equal(t, int64(1), fin.Online.Orders)
	require.Equal(t, []service.FinanceCurrencyAmount{{Currency: "CNY", Amount: 520}}, fin.Online.PayAmounts)
	require.InDelta(t, 75, fin.Online.CreditedUSD, 1e-9)
	require.InDelta(t, 25, fin.Gift.PaymentBonusUSD, 1e-9)
	require.InDelta(t, 80+40, fin.Offline.USD, 1e-9, "ADJ-3 is now tagged offline")
	require.InDelta(t, -30, fin.Deductions.USD, 1e-9)
	require.Zero(t, fin.Gift.AdminGiftUSD, "January gifts are outside the period")
	require.Zero(t, fin.Consumption.PendingRequests)
	require.InDelta(t, 12+11+40+99+6+5+2+1, fin.Consumption.UserBilled, 1e-9)

	jan := service.ReportPeriod{Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Location: time.UTC}
	finJan, err := svc.GetFinanceSummary(ctx, jan)
	require.NoError(t, err)
	require.InDelta(t, 50+25+3, finJan.Gift.AdminGiftUSD, 1e-9)
	require.InDelta(t, 5, finJan.Gift.RedeemCodeUSD, 1e-9)
	require.InDelta(t, 3, finJan.Gift.PromoUSD, 1e-9)
	require.InDelta(t, 2, finJan.Gift.AffiliateUSD, 1e-9)
	require.Equal(t, []service.FinanceCurrencyAmount{}, finJan.Online.PayAmounts)

	// 打款：创建、按所有者与时间筛选、软删除。
	paidAt := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	p, err := svc.CreateOwnerPayout(ctx, service.OwnerPayoutInput{OwnerUserID: 20, Amount: 7.5, PaidAt: &paidAt, Notes: "feb"}, 1)
	require.NoError(t, err)
	require.NotZero(t, p.ID)
	_, err = svc.CreateOwnerPayout(ctx, service.OwnerPayoutInput{OwnerUserID: 101, Amount: 1}, 1)
	require.ErrorIs(t, err, service.ErrOwnerPayoutNotOwner)

	owner := int64(20)
	items, total, err := svc.ListOwnerPayouts(ctx, service.OwnerPayoutFilter{OwnerUserID: &owner, Start: &feb.Start, End: &feb.End}, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "owner-b@example.com", items[0].OwnerEmail)
	require.InDelta(t, 7.5, items[0].Amount, 1e-9)
	require.Equal(t, int64(1), *items[0].CreatedBy)

	allPayouts, total, err := svc.ListOwnerPayouts(ctx, service.OwnerPayoutFilter{}, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "deleted payouts are hidden")
	require.Equal(t, p.ID, allPayouts[0].ID, "newest paid_at first")

	require.NoError(t, svc.DeleteOwnerPayout(ctx, p.ID))
	require.ErrorIs(t, svc.DeleteOwnerPayout(ctx, p.ID), service.ErrOwnerPayoutNotFound)
	var deletedAt sql.NullTime
	require.NoError(t, db.QueryRow(`SELECT deleted_at FROM owner_payouts WHERE id = $1`, p.ID).Scan(&deletedAt))
	require.True(t, deletedAt.Valid, "soft delete keeps the row")

	// 余额记录来源：只改写前缀。
	var adjID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM redeem_codes WHERE code = 'ADJ-2'`).Scan(&adjID))
	rec, err := svc.UpdateBalanceRecordSource(ctx, 102, adjID, "gift")
	require.NoError(t, err)
	require.Equal(t, "bank", rec.Notes)
	var notes string
	require.NoError(t, db.QueryRow(`SELECT notes FROM redeem_codes WHERE id = $1`, adjID).Scan(&notes))
	require.Equal(t, "[src:gift] bank", notes)
	var payID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM redeem_codes WHERE code = 'PAY-1-1'`).Scan(&payID))
	_, err = svc.UpdateBalanceRecordSource(ctx, 103, payID, "offline")
	require.ErrorIs(t, err, service.ErrBalanceRecordNotAdjust)
	_, err = svc.UpdateBalanceRecordSource(ctx, 101, adjID, "offline")
	require.ErrorIs(t, err, service.ErrBalanceRecordNotFound)
}

func decimalPrefix(state string) string {
	for i, ch := range state {
		if ch == '/' {
			return state[:i]
		}
	}
	return state
}
