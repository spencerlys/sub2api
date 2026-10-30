//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 只使用显式指定的隔离测试数据库（OWNER_SETTLEMENT_TEST_DSN），每次创建并清理独立 schema。
// 只建结算查询涉及的列，owner_payouts 使用真实迁移 252。
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
CREATE TABLE accounts(id BIGINT PRIMARY KEY, owner_user_id BIGINT, deleted_at TIMESTAMPTZ);
CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, account_id BIGINT NOT NULL,
  total_cost DECIMAL(20,10) NOT NULL, actual_cost DECIMAL(20,10) NOT NULL,
  account_rate_multiplier DECIMAL(10,4), account_stats_cost DECIMAL(20,10), created_at TIMESTAMPTZ NOT NULL);
CREATE TABLE redeem_codes(id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, type TEXT NOT NULL, value DECIMAL(20,8) NOT NULL,
  status TEXT NOT NULL, used_by BIGINT, used_at TIMESTAMPTZ, notes TEXT);
CREATE TABLE payment_orders(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, recharge_code TEXT NOT NULL, amount DECIMAL(20,2) NOT NULL,
  pay_amount DECIMAL(20,2) NOT NULL, bonus_amount DECIMAL(20,2) NOT NULL DEFAULT 0, order_type TEXT NOT NULL DEFAULT 'balance',
  status TEXT NOT NULL DEFAULT 'COMPLETED', provider_snapshot JSONB);
CREATE TABLE promo_code_usages(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, bonus_amount DECIMAL(20,8) NOT NULL, used_at TIMESTAMPTZ NOT NULL);
CREATE TABLE user_affiliate_ledger(id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, action TEXT NOT NULL, amount DECIMAL(20,8) NOT NULL, created_at TIMESTAMPTZ NOT NULL);
`)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "252_owner_payouts.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB("postgres", db)))
	return db, client
}

func TestOwnerSettlementPostgresEndToEnd(t *testing.T) {
	db, client := ownerSettlementTestDB(t)
	ctx := context.Background()
	repo := NewOwnerSettlementRepository(client, db)
	svc := service.NewOwnerSettlementService(repo)

	jan := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	seed := `
INSERT INTO users(id, email, username, role) VALUES
  (1, 'admin@example.com', 'admin', 'admin'),
  (10, 'owner-a@example.com', 'owner-a', 'owner'),
  (20, 'owner-b@example.com', 'owner-b', 'owner'),
  (101, 'gift@example.com', '', 'user'),
  (102, 'offline@example.com', '', 'user'),
  (103, 'mixed@example.com', '', 'user'),
  (104, 'nothing@example.com', '', 'user');
INSERT INTO accounts(id, owner_user_id) VALUES (1000, 10), (2000, 20), (3000, NULL);

-- 101：管理员赠送（无前缀）+ 发放兑换码 + 优惠码 + 返利转入，全部为赠送。
INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES
  ('ADJ-1', 'admin_balance', 50, 'used', 101, :jan, 'welcome'),
  ('CODE-1', 'balance', 5, 'used', 101, :jan, NULL),
  ('CODE-UNUSED', 'balance', 500, 'unused', NULL, NULL, NULL);
INSERT INTO promo_code_usages(user_id, bonus_amount, used_at) VALUES (101, 3, :jan);
INSERT INTO user_affiliate_ledger(user_id, action, amount, created_at) VALUES (101, 'transfer', 2, :jan), (101, 'accrue', 99, :jan);

-- 102：线下实收 + 更正（不计）+ 负数扣减（不计）。
INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES
  ('ADJ-2', 'admin_balance', 80, 'used', 102, :feb, '[src:offline] bank'),
  ('ADJ-3', 'admin_balance', 40, 'used', 102, :feb, '[SRC:Correction] typo'),
  ('ADJ-4', 'admin_balance', -30, 'used', 102, :feb, '[src:offline] refund');

-- 103：在线支付到账 100，其中赠送 25（实付 75）+ 管理员赠送 25 → 实付占比 0.6。
INSERT INTO payment_orders(user_id, recharge_code, amount, pay_amount, bonus_amount, provider_snapshot) VALUES
  (103, 'PAY-1-1', 100, 520, 25, '{"currency":"cny"}'),
  (103, 'PAY-1-2', 10, 10, 0, NULL);
INSERT INTO redeem_codes(code, type, value, status, used_by, used_at, notes) VALUES
  ('PAY-1-1', 'balance', 100, 'used', 103, :feb, NULL),
  ('ADJ-5', 'admin_balance', 25, 'used', 103, :jan, '[src:gift]'),
  ('PAY-1-2', 'balance', 10, 'unused', NULL, NULL, NULL);

-- 用量：账户成本 = COALESCE(account_stats_cost,total_cost) × COALESCE(account_rate_multiplier,1)。
INSERT INTO usage_logs(user_id, account_id, total_cost, actual_cost, account_rate_multiplier, account_stats_cost, created_at) VALUES
  (101, 1000, 10, 12, 0.5, NULL, :feb),
  (102, 1000, 10, 11, 0.5, NULL, :feb),
  (102, 1000, 4, 4, NULL, 2, :jan),
  (103, 2000, 20, 20, NULL, NULL, :feb),
  (104, 2000, 6, 6, NULL, NULL, :feb),
  (103, 3000, 99, 99, NULL, NULL, :feb);
INSERT INTO owner_payouts(owner_user_id, amount, paid_at, notes) VALUES (10, 3, :jan, 'jan');
INSERT INTO owner_payouts(owner_user_id, amount, paid_at, notes, deleted_at) VALUES (10, 100, :jan, 'void', NOW());
`
	// 多语句脚本不能带参数，时间以字面量代入。
	seed = strings.NewReplacer(":jan", "'"+jan.Format(time.RFC3339)+"'", ":feb", "'"+feb.Format(time.RFC3339)+"'").Replace(seed)
	_, err := db.Exec(seed)
	require.NoError(t, err)

	febStart := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	marStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	report, err := svc.GetOwnerSettlements(ctx, febStart, marStart, nil)
	require.NoError(t, err)
	require.Len(t, report.Items, 2, "admin and plain users are not settlement owners")

	a := report.Items[0]
	require.Equal(t, int64(10), a.OwnerUserID)
	require.Equal(t, "owner-a@example.com", a.Email)
	require.Equal(t, int64(2), a.Requests)
	require.InDelta(t, 20, a.OfficialCost, 1e-9)
	require.InDelta(t, 23, a.UserBilled, 1e-9)
	require.InDelta(t, 10, a.AccountCost, 1e-9)
	require.InDelta(t, 5, a.Earnings, 1e-9, "gift user 101 earns nothing; offline user 102 earns its full account cost")
	require.InDelta(t, 7, a.EarningsAllTime, 1e-9, "January usage of 102 adds stats cost 2")
	require.InDelta(t, 3, a.PaidTotal, 1e-9, "soft-deleted payouts are excluded")
	require.InDelta(t, 4, a.Unpaid, 1e-9)

	b := report.Items[1]
	require.Equal(t, int64(20), b.OwnerUserID)
	require.Equal(t, int64(2), b.Requests)
	require.InDelta(t, 26, b.AccountCost, 1e-9)
	require.InDelta(t, 20*0.6, b.Earnings, 1e-9, "user 104 has no credits → zero share")
	require.InDelta(t, 12, b.Unpaid, 1e-9)
	require.Equal(t, int64(4), report.Totals.Requests)

	jan1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	me, err := svc.GetMySettlement(ctx, 10, jan1, febStart)
	require.NoError(t, err)
	require.Equal(t, int64(1), me.Requests)
	require.InDelta(t, 4, me.OfficialCost, 1e-9)
	require.InDelta(t, 2, me.Earnings, 1e-9)
	require.InDelta(t, 7, me.EarningsAllTime, 1e-9)

	// 财务汇总：按入账时间（二月）。
	fin, err := svc.GetFinanceSummary(ctx, febStart, marStart)
	require.NoError(t, err)
	require.Equal(t, int64(1), fin.Online.Orders)
	require.Equal(t, []service.FinanceCurrencyAmount{{Currency: "CNY", Amount: 520}}, fin.Online.PayAmounts)
	require.InDelta(t, 75, fin.Online.CreditedUSD, 1e-9)
	require.InDelta(t, 25, fin.Gift.PaymentBonusUSD, 1e-9)
	require.InDelta(t, 80, fin.Offline.USD, 1e-9)
	require.Equal(t, int64(1), fin.Offline.Count)
	require.InDelta(t, 40, fin.Corrections.NetUSD, 1e-9)
	require.InDelta(t, -30, fin.Deductions.USD, 1e-9)
	require.Zero(t, fin.Gift.AdminGiftUSD, "January gifts are outside the period")

	finJan, err := svc.GetFinanceSummary(ctx, jan1, febStart)
	require.NoError(t, err)
	require.InDelta(t, 75, finJan.Gift.AdminGiftUSD, 1e-9)
	require.InDelta(t, 5, finJan.Gift.RedeemCodeUSD, 1e-9)
	require.InDelta(t, 3, finJan.Gift.PromoUSD, 1e-9)
	require.InDelta(t, 2, finJan.Gift.AffiliateUSD, 1e-9)
	require.InDelta(t, 85, finJan.Gift.TotalUSD, 1e-9)
	require.Equal(t, []service.FinanceCurrencyAmount{}, finJan.Online.PayAmounts)

	// 打款：创建、按所有者与时间筛选、软删除。
	paidAt := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	p, err := svc.CreateOwnerPayout(ctx, service.OwnerPayoutInput{OwnerUserID: 20, Amount: 7.5, PaidAt: &paidAt, Notes: "feb"}, 1)
	require.NoError(t, err)
	require.NotZero(t, p.ID)
	_, err = svc.CreateOwnerPayout(ctx, service.OwnerPayoutInput{OwnerUserID: 101, Amount: 1}, 1)
	require.ErrorIs(t, err, service.ErrOwnerPayoutNotOwner)

	owner := int64(20)
	items, total, err := svc.ListOwnerPayouts(ctx, service.OwnerPayoutFilter{OwnerUserID: &owner, Start: &febStart, End: &marStart}, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "owner-b@example.com", items[0].OwnerEmail)
	require.InDelta(t, 7.5, items[0].Amount, 1e-9)
	require.Equal(t, int64(1), *items[0].CreatedBy)

	all, total, err := svc.ListOwnerPayouts(ctx, service.OwnerPayoutFilter{}, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "deleted payouts are hidden")
	require.Equal(t, p.ID, all[0].ID, "newest paid_at first")

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
