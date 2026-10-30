package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/ownerpayout"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ownerSettlementRepository 所有者结算与财务汇总。聚合查询只返回数值与所有者身份，
// 请求用户只以 user_id 参与实付占比计算，不向调用方暴露。
type ownerSettlementRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewOwnerSettlementRepository(client *dbent.Client, sqlDB *sql.DB) service.OwnerSettlementRepository {
	return &ownerSettlementRepository{client: client, sql: sqlDB}
}

// balanceSourceSQL 从 notes 前缀提取来源原值（小写），无前缀为空串；分类规则见 service.ParseBalanceSourceNotes。
const balanceSourceSQL = `lower(COALESCE(substring(COALESCE(rc.notes, '') FROM '(?i)^\s*\[src:([a-z]+)\]'), ''))`

// accountCostSQL 账户成本（官方价 × 账户倍率），与用量统计口径一致。
const accountCostSQL = `COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)`

// 所有者应付口径（结算、我的结算、账号健康、用量明细与统计共用以下三个片段）：
//   - 号主分成基数 = 账户成本 × 号主分成倍率（请求发生时其分组生效的倍率，无分组或无历史为 1）；
//   - 应付 = 号主分成基数 × paid_cost / actual_cost（已拆分且 actual_cost > 0，否则为 0）；
//   - 内部使用（赠送）账户成本 = 号主分成基数 − 应付（仅已拆分的请求）。
//
// ownerPayoutJoinSQL 以 LATERAL 关联倍率历史（别名 opm，每行一次索引查找）；u 为 usage_logs 或含
// group_id、created_at 列的派生表的别名。索引：group_owner_payout_multiplier_history_group_effective。
func ownerPayoutJoinSQL(u string) string {
	return `LEFT JOIN LATERAL (
  SELECT h.multiplier FROM group_owner_payout_multiplier_history h
  WHERE h.group_id = ` + u + `.group_id AND h.effective_from <= ` + u + `.created_at
  ORDER BY h.effective_from DESC, h.id DESC LIMIT 1) opm ON TRUE`
}

// payoutBaseSQL 号主分成基数；accountCost 为账户成本表达式，须与 ownerPayoutJoinSQL 同用。
func payoutBaseSQL(accountCost string) string {
	return `((` + accountCost + `) * COALESCE(opm.multiplier, 1))`
}

// ownerEarningsSQL 单次请求的所有者应付；split 为 usage_cost_splits 的别名（LEFT JOIN，未拆分时为 0）。
func ownerEarningsSQL(accountCost, split string) string {
	return `(CASE WHEN ` + split + `.actual_cost > 0
  THEN ` + payoutBaseSQL(accountCost) + ` * ` + split + `.paid_cost / ` + split + `.actual_cost ELSE 0 END)`
}

// creditRowsSQL 全部余额入账明细（正负均含）：
//   - redeem_codes(type=balance) 能按 recharge_code 关联到支付订单的为在线支付（value=amount，含 bonus_amount 赠送部分），
//     其余已使用的 balance 码为发放的兑换码；
//   - redeem_codes(type=admin_balance) 为管理员调整，来源取 notes 前缀；
//   - promo_code_usages.bonus_amount 为优惠码赠送；user_affiliate_ledger(action=transfer) 为返利转入余额。
//
// %[1]s/%[2]s/%[3]s 为各分支时间或用户列上的过滤条件（以 AND 开头）；%[4]d 为默认币种的参数序号。
const creditRowsSQL = `
SELECT rc.used_by AS user_id,
  CASE WHEN rc.type = 'admin_balance' THEN 'admin'
       WHEN po.id IS NOT NULL THEN 'payment'
       ELSE 'redeem_code' END AS kind,
  CASE WHEN rc.type = 'admin_balance' THEN ` + balanceSourceSQL + ` ELSE '' END AS source,
  rc.value::double precision AS amount,
  CASE WHEN po.id IS NULL THEN 0
       ELSE LEAST(GREATEST(po.bonus_amount, 0), GREATEST(rc.value, 0))::double precision END AS bonus,
  po.pay_amount::double precision AS pay_amount,
  CASE WHEN po.id IS NULL THEN NULL
       ELSE COALESCE(NULLIF(upper(btrim(po.provider_snapshot->>'currency')), ''), $%[4]d) END AS currency
FROM redeem_codes rc
LEFT JOIN payment_orders po
  ON rc.type = 'balance' AND po.recharge_code = rc.code AND po.user_id = rc.used_by
WHERE rc.status = 'used' AND rc.used_by IS NOT NULL AND rc.type IN ('balance', 'admin_balance') %[1]s
UNION ALL
SELECT user_id, 'promo', '', bonus_amount::double precision, 0, NULL, NULL
FROM promo_code_usages WHERE TRUE %[2]s
UNION ALL
SELECT user_id, 'affiliate', '', amount::double precision, 0, NULL, NULL
FROM user_affiliate_ledger WHERE action = 'transfer' %[3]s`

// ListSettlementOwners 结算对象：未删除的 role=owner 用户，以及名下有账户或有效打款记录的用户。
func (r *ownerSettlementRepository) ListSettlementOwners(ctx context.Context, ownerUserID *int64) (out []service.OwnerIdentity, err error) {
	rows, err := r.sql.QueryContext(ctx, `
SELECT u.id, u.email, COALESCE(u.username, '')
FROM users u
WHERE ((u.role = 'owner' AND u.deleted_at IS NULL)
    OR EXISTS (SELECT 1 FROM accounts a WHERE a.owner_user_id = u.id)
    OR EXISTS (SELECT 1 FROM owner_payouts p WHERE p.owner_user_id = u.id AND p.deleted_at IS NULL))
  AND ($1::bigint IS NULL OR u.id = $1)
ORDER BY u.id`, nullableID(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var o service.OwnerIdentity
		if err := rows.Scan(&o.UserID, &o.Email, &o.Username); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AggregateOwnerSettlement 按所有者聚合 [start,end) 内的请求数、官方价、应收（actual_cost）、实付/赠送拆分、账户成本、
// 分成基数、收益、内部使用（赠送资助）的账户成本与待结算，以及历史累计收益；收益口径见 ownerPayoutJoinSQL。
// 分成基数与内部使用成本只计已拆分的请求，保证 分成基数 − 内部使用成本 = 收益。
// 用量归属按账户当前所有者计算；请求用户不出现在结果中。
// 索引：idx_usage_logs_account_created_at (account_id, created_at)；拆分按主键关联。
func (r *ownerSettlementRepository) AggregateOwnerSettlement(ctx context.Context, start, end time.Time, ownerUserID *int64) (out []service.OwnerSettlementUsage, err error) {
	rows, err := r.sql.QueryContext(ctx, `
WITH u AS (
  SELECT a.owner_user_id,
    (ul.created_at >= $1 AND ul.created_at < $2) AS in_range,
    (s.usage_log_id IS NULL) AS pending,
    ul.total_cost, ul.actual_cost, s.paid_cost, s.gift_cost,
    (`+accountCostSQL+`) AS account_cost,
    `+payoutBaseSQL(accountCostSQL)+` AS payout_base,
    `+ownerEarningsSQL(accountCostSQL, "s")+` AS earnings
  FROM usage_logs ul
  JOIN accounts a ON a.id = ul.account_id
  LEFT JOIN usage_cost_splits s ON s.usage_log_id = ul.id
  `+ownerPayoutJoinSQL("ul")+`
  WHERE a.owner_user_id IS NOT NULL
    AND ($3::bigint IS NULL OR a.owner_user_id = $3)
)
SELECT owner_user_id,
  COUNT(*) FILTER (WHERE in_range),
  COALESCE(SUM(total_cost) FILTER (WHERE in_range), 0)::double precision,
  COALESCE(SUM(actual_cost) FILTER (WHERE in_range), 0)::double precision,
  COALESCE(SUM(paid_cost) FILTER (WHERE in_range), 0)::double precision,
  COALESCE(SUM(gift_cost) FILTER (WHERE in_range), 0)::double precision,
  COALESCE(SUM(account_cost) FILTER (WHERE in_range), 0)::double precision,
  COALESCE(SUM(payout_base) FILTER (WHERE in_range AND NOT pending), 0)::double precision,
  COALESCE(SUM(earnings) FILTER (WHERE in_range), 0)::double precision,
  COALESCE(SUM(payout_base - earnings) FILTER (WHERE in_range AND NOT pending), 0)::double precision,
  COUNT(*) FILTER (WHERE in_range AND pending),
  COALESCE(SUM(account_cost) FILTER (WHERE in_range AND pending), 0)::double precision,
  COALESCE(SUM(earnings), 0)::double precision
FROM u
GROUP BY owner_user_id`, start, end, nullableID(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var u service.OwnerSettlementUsage
		if err := rows.Scan(&u.OwnerUserID, &u.Requests, &u.OfficialCost, &u.UserBilled, &u.PaidCost, &u.GiftCost,
			&u.AccountCost, &u.PayoutBase, &u.Earnings, &u.GiftAccountCost, &u.PendingRequests, &u.PendingAccountCost, &u.EarningsAllTime); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountActiveOwnerAccounts 各所有者当前 status=active 且可调度、未删除的账户数。
func (r *ownerSettlementRepository) CountActiveOwnerAccounts(ctx context.Context, ownerUserID *int64) (out map[int64]int64, err error) {
	rows, err := r.sql.QueryContext(ctx, `
SELECT owner_user_id, COUNT(*)
FROM accounts
WHERE owner_user_id IS NOT NULL AND deleted_at IS NULL AND status = 'active' AND schedulable
  AND ($1::bigint IS NULL OR owner_user_id = $1)
GROUP BY owner_user_id`, nullableID(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	out = map[int64]int64{}
	for rows.Next() {
		var id, n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// AggregateFinanceConsumption 期间 [start,end) 内全部用量的应收与精确拆分；未拆分的计为待结算。
func (r *ownerSettlementRepository) AggregateFinanceConsumption(ctx context.Context, start, end time.Time) (out *service.FinanceConsumption, err error) {
	rows, err := r.sql.QueryContext(ctx, `
SELECT COUNT(*),
  COALESCE(SUM(ul.actual_cost), 0)::double precision,
  COALESCE(SUM(s.paid_cost), 0)::double precision,
  COALESCE(SUM(s.gift_cost), 0)::double precision,
  COUNT(*) FILTER (WHERE s.usage_log_id IS NULL),
  COALESCE(SUM(ul.actual_cost) FILTER (WHERE s.usage_log_id IS NULL), 0)::double precision
FROM usage_logs ul
LEFT JOIN usage_cost_splits s ON s.usage_log_id = ul.id
WHERE ul.created_at >= $1 AND ul.created_at < $2`, start, end)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	out = &service.FinanceConsumption{}
	if rows.Next() {
		if err := rows.Scan(&out.Requests, &out.UserBilled, &out.PaidCost, &out.GiftCost, &out.PendingRequests, &out.PendingUserBilled); err != nil {
			return nil, err
		}
	}
	return out, rows.Err()
}

// AggregateFinancePeriod 期间 [start,end) 内的入账，按种类与来源汇总；在线支付另按币种汇总实收。
func (r *ownerSettlementRepository) AggregateFinancePeriod(ctx context.Context, start, end time.Time) (*service.FinancePeriodRaw, error) {
	credits := fmt.Sprintf(creditRowsSQL,
		"AND rc.used_at >= $1 AND rc.used_at < $2",
		"AND used_at >= $1 AND used_at < $2",
		"AND created_at >= $1 AND created_at < $2", 3)
	out := &service.FinancePeriodRaw{PayAmounts: []service.FinanceCurrencyAmount{}}
	if err := r.scanFinanceBuckets(ctx, credits, start, end, out); err != nil {
		return nil, err
	}
	if err := r.scanFinanceCurrencies(ctx, credits, start, end, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ownerSettlementRepository) scanFinanceBuckets(ctx context.Context, credits string, start, end time.Time, out *service.FinancePeriodRaw) (err error) {
	rows, err := r.sql.QueryContext(ctx, `
WITH credits AS (`+credits+`)
SELECT kind, source,
  COUNT(*) FILTER (WHERE amount > 0),
  COALESCE(SUM(amount) FILTER (WHERE amount > 0), 0),
  COUNT(*) FILTER (WHERE amount < 0),
  COALESCE(SUM(amount) FILTER (WHERE amount < 0), 0),
  COALESCE(SUM(bonus) FILTER (WHERE amount > 0), 0)
FROM credits
GROUP BY kind, source
ORDER BY kind, source`, start, end, payment.DefaultPaymentCurrency)
	if err != nil {
		return err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var b service.FinanceCreditBucket
		if err := rows.Scan(&b.Kind, &b.Source, &b.Count, &b.Positive, &b.NegCount, &b.Negative, &b.Bonus); err != nil {
			return err
		}
		out.Credits = append(out.Credits, b)
	}
	return rows.Err()
}

func (r *ownerSettlementRepository) scanFinanceCurrencies(ctx context.Context, credits string, start, end time.Time, out *service.FinancePeriodRaw) (err error) {
	rows, err := r.sql.QueryContext(ctx, `
WITH credits AS (`+credits+`)
SELECT currency, COUNT(*), COALESCE(SUM(pay_amount), 0)
FROM credits
WHERE kind = 'payment' AND amount > 0
GROUP BY currency
ORDER BY currency`, start, end, payment.DefaultPaymentCurrency)
	if err != nil {
		return err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var (
			c     service.FinanceCurrencyAmount
			count int64
		)
		if err := rows.Scan(&c.Currency, &count, &c.Amount); err != nil {
			return err
		}
		out.OnlineOrders += count
		out.PayAmounts = append(out.PayAmounts, c)
	}
	return rows.Err()
}

func (r *ownerSettlementRepository) SumOwnerPayouts(ctx context.Context, ownerUserID *int64) (out map[int64]float64, err error) {
	rows, err := r.sql.QueryContext(ctx, `
SELECT owner_user_id, COALESCE(SUM(amount), 0)::double precision
FROM owner_payouts
WHERE deleted_at IS NULL AND ($1::bigint IS NULL OR owner_user_id = $1)
GROUP BY owner_user_id`, nullableID(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	out = map[int64]float64{}
	for rows.Next() {
		var (
			id  int64
			sum float64
		)
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		out[id] = sum
	}
	return out, rows.Err()
}

func (r *ownerSettlementRepository) ListOwnerPayouts(ctx context.Context, filter service.OwnerPayoutFilter, params pagination.PaginationParams) ([]service.OwnerPayout, int64, error) {
	q := r.client.OwnerPayout.Query()
	if filter.OwnerUserID != nil && *filter.OwnerUserID > 0 {
		q = q.Where(ownerpayout.OwnerUserIDEQ(*filter.OwnerUserID))
	}
	if filter.Start != nil {
		q = q.Where(ownerpayout.PaidAtGTE(*filter.Start))
	}
	if filter.End != nil {
		q = q.Where(ownerpayout.PaidAtLT(*filter.End))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.Order(dbent.Desc(ownerpayout.FieldPaidAt), dbent.Desc(ownerpayout.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.OwnerUserID)
	}
	emails := map[int64]string{}
	if len(ids) > 0 {
		// 所有者被软删除后仍需显示历史打款的收款人。
		users, err := r.client.User.Query().Where(dbuser.IDIn(ids...)).
			Select(dbuser.FieldID, dbuser.FieldEmail).All(mixins.SkipSoftDelete(ctx))
		if err != nil {
			return nil, 0, err
		}
		for _, u := range users {
			emails[u.ID] = u.Email
		}
	}
	out := make([]service.OwnerPayout, 0, len(rows))
	for _, p := range rows {
		out = append(out, service.OwnerPayout{
			ID:          p.ID,
			OwnerUserID: p.OwnerUserID,
			OwnerEmail:  emails[p.OwnerUserID],
			Amount:      p.Amount,
			PaidAt:      p.PaidAt,
			Notes:       p.Notes,
			CreatedBy:   p.CreatedBy,
			CreatedAt:   p.CreatedAt,
		})
	}
	return out, int64(total), nil
}

func (r *ownerSettlementRepository) CreateOwnerPayout(ctx context.Context, p *service.OwnerPayout) error {
	b := r.client.OwnerPayout.Create().
		SetOwnerUserID(p.OwnerUserID).
		SetAmount(p.Amount).
		SetPaidAt(p.PaidAt).
		SetNotes(p.Notes).
		SetNillableCreatedBy(p.CreatedBy)
	m, err := b.Save(ctx)
	if err != nil {
		return err
	}
	p.ID, p.CreatedAt = m.ID, m.CreatedAt
	return nil
}

// DeleteOwnerPayout 软删除（SoftDeleteMixin 将删除改写为设置 deleted_at）；已删除或不存在返回 NotFound。
func (r *ownerSettlementRepository) DeleteOwnerPayout(ctx context.Context, id int64) error {
	err := r.client.OwnerPayout.DeleteOneID(id).Exec(ctx)
	if dbent.IsNotFound(err) {
		return service.ErrOwnerPayoutNotFound
	}
	return err
}

func (r *ownerSettlementRepository) GetBalanceRecord(ctx context.Context, id int64) (out *service.RedeemCode, err error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT id, type, used_by, COALESCE(notes, '') FROM redeem_codes WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrRedeemCodeNotFound
	}
	var (
		rc     service.RedeemCode
		usedBy sql.NullInt64
	)
	if err := rows.Scan(&rc.ID, &rc.Type, &usedBy, &rc.Notes); err != nil {
		return nil, err
	}
	if usedBy.Valid {
		v := usedBy.Int64
		rc.UsedBy = &v
	}
	return &rc, nil
}

// UpdateBalanceRecordNotes 只改写管理员余额调整记录的备注。
func (r *ownerSettlementRepository) UpdateBalanceRecordNotes(ctx context.Context, id int64, notes string) error {
	res, err := r.sql.ExecContext(ctx, `UPDATE redeem_codes SET notes = $2 WHERE id = $1 AND type = 'admin_balance'`, id, notes)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return service.ErrBalanceRecordNotFound
	}
	return nil
}

func nullableID(id *int64) any {
	if id == nil || *id <= 0 {
		return nil
	}
	return *id
}

func closeRows(rows *sql.Rows, err *error) {
	if closeErr := rows.Close(); closeErr != nil && *err == nil {
		*err = closeErr
	}
}
