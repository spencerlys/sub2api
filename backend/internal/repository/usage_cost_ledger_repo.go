package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// usageCostLedgerRepository 成本账本（usage_cost_splits / usage_cost_ledger_users / usage_cost_ledger_cursor）。
// 只读取 usage_logs 与入账记录，从不修改余额或用量。
type usageCostLedgerRepository struct {
	db *sql.DB
}

func NewUsageCostLedgerRepository(sqlDB *sql.DB) service.UsageCostLedgerRepository {
	return &usageCostLedgerRepository{db: sqlDB}
}

// ledgerCreditRowsSQL 全部余额入账事件（正负均含），与 creditRowsSQL 同一分类口径，另带稳定键与入账时间：
// 兑换码/管理员调整取 used_at（缺失时 created_at），优惠码取 used_at，返利转入取 created_at。
// %[1]s/%[2]s/%[3]s 为各分支的附加条件（以 AND 开头）。
const ledgerCreditRowsSQL = `
SELECT 'rc:' || rc.id AS ckey, rc.used_by AS user_id, COALESCE(rc.used_at, rc.created_at) AS at,
  CASE WHEN rc.type = 'admin_balance' THEN 'admin'
       WHEN po.id IS NOT NULL THEN 'payment'
       ELSE 'redeem_code' END AS kind,
  CASE WHEN rc.type = 'admin_balance' THEN ` + balanceSourceSQL + ` ELSE '' END AS source,
  rc.value::numeric AS amount,
  (CASE WHEN po.id IS NULL THEN 0
        ELSE LEAST(GREATEST(po.bonus_amount, 0), GREATEST(rc.value, 0)) END)::numeric AS bonus
FROM redeem_codes rc
LEFT JOIN payment_orders po
  ON rc.type = 'balance' AND po.recharge_code = rc.code AND po.user_id = rc.used_by
WHERE rc.status = 'used' AND rc.used_by IS NOT NULL AND rc.type IN ('balance', 'admin_balance') %[1]s
UNION ALL
SELECT 'pc:' || id, user_id, used_at, 'promo', '', bonus_amount::numeric, 0::numeric
FROM promo_code_usages WHERE TRUE %[2]s
UNION ALL
SELECT 'af:' || id, user_id, created_at, 'affiliate', '', amount::numeric, 0::numeric
FROM user_affiliate_ledger WHERE action = 'transfer' %[3]s`

// ledgerCreditFPSQL 入账指纹（同一表达式用于候选发现与用户处理）；%s 为 FILTER 条件。
const ledgerCreditFPSQL = `COALESCE(md5(string_agg(
  c.ckey || '|' || c.kind || '|' || c.source || '|' || c.amount::text || '|' || c.bonus::text || '|' ||
  to_char(c.at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US'), ',' ORDER BY c.ckey) FILTER (WHERE %s)), '')`

// ledgerSubscriptionPaidSQL 订阅计费请求是否由实付订阅订单资助：该用户为该分组有已完成、实付 > 0、
// 且完成时间不晚于请求的订阅订单。
const ledgerSubscriptionPaidSQL = `CASE WHEN ul.billing_type = 1 THEN EXISTS (
  SELECT 1 FROM payment_orders po
  WHERE po.user_id = ul.user_id AND po.order_type = 'subscription' AND po.subscription_group_id = ul.group_id
    AND po.status IN ('COMPLETED', 'PARTIALLY_REFUNDED') AND po.pay_amount > 0
    AND COALESCE(po.completed_at, po.paid_at, po.created_at) <= ul.created_at)
  ELSE FALSE END`

func userCreditRowsSQL() string {
	return fmt.Sprintf(ledgerCreditRowsSQL, "AND rc.used_by = $1", "AND user_id = $1", "AND user_id = $1")
}

func (r *usageCostLedgerRepository) GetLedgerCursor(ctx context.Context) (*service.CostLedgerCursor, error) {
	var (
		c                                                   service.CostLedgerCursor
		processed, lastRun, rebuildRequested, lastRebuildAt sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
SELECT last_usage_id, processed_until, last_run_at, last_error, rebuild_requested_at, last_rebuild_at
FROM usage_cost_ledger_cursor WHERE id = 1`).Scan(&c.LastUsageID, &processed, &lastRun, &c.LastError, &rebuildRequested, &lastRebuildAt)
	if err == sql.ErrNoRows {
		return &service.CostLedgerCursor{}, nil
	}
	if err != nil {
		return nil, err
	}
	c.ProcessedUntil = nullTimePtr(processed)
	c.LastRunAt = nullTimePtr(lastRun)
	c.RebuildRequestedAt = nullTimePtr(rebuildRequested)
	c.LastRebuildAt = nullTimePtr(lastRebuildAt)
	return &c, nil
}

func (r *usageCostLedgerRepository) SaveLedgerCursor(ctx context.Context, c service.CostLedgerCursor) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO usage_cost_ledger_cursor (id, last_usage_id, processed_until, last_run_at, last_error, last_rebuild_at)
VALUES (1, $1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
  last_usage_id = EXCLUDED.last_usage_id,
  processed_until = EXCLUDED.processed_until,
  last_run_at = EXCLUDED.last_run_at,
  last_error = EXCLUDED.last_error,
  last_rebuild_at = COALESCE(EXCLUDED.last_rebuild_at, usage_cost_ledger_cursor.last_rebuild_at)`,
		c.LastUsageID, timePtrArg(c.ProcessedUntil), timePtrArg(c.LastRunAt), c.LastError, timePtrArg(c.LastRebuildAt))
	return err
}

func (r *usageCostLedgerRepository) RequestLedgerRebuild(ctx context.Context, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO usage_cost_ledger_cursor (id, rebuild_requested_at) VALUES (1, $1)
ON CONFLICT (id) DO UPDATE SET rebuild_requested_at = EXCLUDED.rebuild_requested_at`, at)
	return err
}

func (r *usageCostLedgerRepository) ListLedgerUsers(ctx context.Context) (out []int64, err error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT DISTINCT user_id FROM (
  SELECT DISTINCT user_id FROM usage_logs
  UNION SELECT user_id FROM usage_cost_splits
  UNION SELECT user_id FROM usage_cost_ledger_users
  UNION SELECT used_by FROM redeem_codes WHERE status = 'used' AND used_by IS NOT NULL AND type IN ('balance', 'admin_balance')
  UNION SELECT user_id FROM promo_code_usages
  UNION SELECT user_id FROM user_affiliate_ledger WHERE action = 'transfer'
) u WHERE user_id IS NOT NULL ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *usageCostLedgerRepository) FindLedgerCandidates(ctx context.Context, horizon time.Time, fromUsageID int64) ([]service.LedgerCandidate, error) {
	byUser := map[int64]*service.LedgerCandidate{}
	order := []int64{}
	get := func(id int64) *service.LedgerCandidate {
		if c := byUser[id]; c != nil {
			return c
		}
		c := &service.LedgerCandidate{UserID: id}
		byUser[id] = c
		order = append(order, id)
		return c
	}
	// 新的未拆分请求（id 水位之后，含少量回看）。主键范围扫描 + 主键反连接。
	if err := r.scanCandidates(ctx, func(rows *sql.Rows) error {
		var (
			id  int64
			min time.Time
		)
		if err := rows.Scan(&id, &min); err != nil {
			return err
		}
		get(id).MinNewDebitAt = &min
		return nil
	}, `
SELECT ul.user_id, MIN(ul.created_at)
FROM usage_logs ul
WHERE ul.id > $1 AND ul.created_at <= $2
  AND NOT EXISTS (SELECT 1 FROM usage_cost_splits s WHERE s.usage_log_id = ul.id)
GROUP BY ul.user_id`, fromUsageID, horizon); err != nil {
		return nil, err
	}
	// 入账有变化的用户：无状态但已有入账、水位后有新入账、或水位前的入账指纹不符（含被删除）。
	credits := fmt.Sprintf(ledgerCreditRowsSQL, "", "", "")
	if err := r.scanCandidates(ctx, func(rows *sql.Rows) error {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		get(id)
		return nil
	}, `
WITH c AS (`+credits+`),
s AS (SELECT user_id, processed_until, credit_fp FROM usage_cost_ledger_users)
SELECT COALESCE(c.user_id, s.user_id)
FROM c FULL JOIN s ON s.user_id = c.user_id
GROUP BY COALESCE(c.user_id, s.user_id), s.user_id, s.processed_until, s.credit_fp
HAVING (s.user_id IS NULL AND COALESCE(bool_or(c.at <= $1), FALSE))
    OR (s.user_id IS NOT NULL AND (
         COALESCE(bool_or(c.at > s.processed_until AND c.at <= $1), FALSE)
      OR `+fmt.Sprintf(ledgerCreditFPSQL, "c.at <= s.processed_until")+` <> s.credit_fp))`, horizon); err != nil {
		return nil, err
	}
	out := make([]service.LedgerCandidate, 0, len(order))
	for _, id := range order {
		out = append(out, *byUser[id])
	}
	return out, nil
}

func (r *usageCostLedgerRepository) scanCandidates(ctx context.Context, scan func(*sql.Rows) error, query string, args ...any) (err error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// NextUsageWatermark 新的 id 水位：prev 之后第一条晚于 horizon 的请求之前；没有这样的请求时取当前最大 id。
func (r *usageCostLedgerRepository) NextUsageWatermark(ctx context.Context, prev int64, horizon time.Time) (int64, error) {
	var w int64
	err := r.db.QueryRowContext(ctx, `
SELECT GREATEST($1::bigint, COALESCE(
  (SELECT MIN(id) - 1 FROM usage_logs WHERE id > $1 AND created_at > $2),
  (SELECT MAX(id) FROM usage_logs),
  $1::bigint))`, prev, horizon).Scan(&w)
	return w, err
}

// WithUserLedger 在读已提交事务内处理单个用户；先取用户级咨询锁，保证同一用户不会被并发处理，
// 锁之后的每条语句都能看到最新提交的数据。
func (r *usageCostLedgerRepository) WithUserLedger(ctx context.Context, userID int64, fn func(context.Context, service.LedgerUserStore) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('usage_cost_ledger:user', $1))`, userID); err != nil {
		return err
	}
	if err := fn(ctx, &usageCostLedgerStore{tx: tx, userID: userID}); err != nil {
		return err
	}
	return tx.Commit()
}

type usageCostLedgerStore struct {
	tx     *sql.Tx
	userID int64
}

func (s *usageCostLedgerStore) State(ctx context.Context) (*service.LedgerUserState, error) {
	var (
		st      = service.LedgerUserState{UserID: s.userID}
		rebuilt sql.NullTime
	)
	err := s.tx.QueryRowContext(ctx, `
SELECT gift_remaining, paid_remaining, processed_until, credit_fp, last_rebuild_at
FROM usage_cost_ledger_users WHERE user_id = $1`, s.userID).Scan(&st.Buckets.Gift, &st.Buckets.Paid, &st.ProcessedUntil, &st.CreditFP, &rebuilt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st.LastRebuildAt = nullTimePtr(rebuilt)
	return &st, nil
}

func (s *usageCostLedgerStore) CreditFingerprint(ctx context.Context, until time.Time) (string, error) {
	var fp string
	err := s.tx.QueryRowContext(ctx, `WITH c AS (`+userCreditRowsSQL()+`)
SELECT `+fmt.Sprintf(ledgerCreditFPSQL, "c.at <= $2")+` FROM c`, s.userID, until).Scan(&fp)
	return fp, err
}

// Credits 单条语句同时返回 (after, until] 内的入账与截至 until 的指纹，二者来自同一快照。
func (s *usageCostLedgerStore) Credits(ctx context.Context, after *time.Time, until time.Time) (out []service.LedgerCredit, fp string, err error) {
	rows, err := s.tx.QueryContext(ctx, `WITH c AS (`+userCreditRowsSQL()+`),
f AS (SELECT `+fmt.Sprintf(ledgerCreditFPSQL, "c.at <= $3")+` AS fp FROM c)
SELECT f.fp, c.ckey, c.at, c.kind, c.source, c.amount, c.bonus
FROM f LEFT JOIN c ON c.at <= $3 AND ($2::timestamptz IS NULL OR c.at > $2)
ORDER BY c.at, c.ckey`, s.userID, timePtrArg(after), until)
	if err != nil {
		return nil, "", err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var (
			key, kind, source sql.NullString
			at                sql.NullTime
			amount, bonus     decimal.NullDecimal
		)
		if err := rows.Scan(&fp, &key, &at, &kind, &source, &amount, &bonus); err != nil {
			return nil, "", err
		}
		if !key.Valid || !at.Valid {
			continue
		}
		out = append(out, service.LedgerCredit{
			Key: key.String, At: at.Time, Kind: kind.String, Source: source.String,
			Amount: amount.Decimal, Bonus: bonus.Decimal,
		})
	}
	return out, fp, rows.Err()
}

func (s *usageCostLedgerStore) Debits(ctx context.Context, after *time.Time, until, cursorAt time.Time, cursorID int64, all bool, limit int) (out []service.LedgerDebit, err error) {
	var rows *sql.Rows
	if all {
		rows, err = s.tx.QueryContext(ctx, `
SELECT id, created_at, actual_cost, billing_type, sub_paid, fixed_gift, fixed_paid FROM (
  (SELECT ul.id, ul.created_at, ul.actual_cost::numeric AS actual_cost, ul.billing_type::smallint AS billing_type,
          `+ledgerSubscriptionPaidSQL+` AS sub_paid, NULL::numeric AS fixed_gift, NULL::numeric AS fixed_paid
   FROM usage_logs ul
   WHERE ul.user_id = $1 AND ul.created_at <= $2 AND (ul.created_at, ul.id) > ($3, $4)
   ORDER BY ul.created_at, ul.id LIMIT $5)
  UNION ALL
  (SELECT s.usage_log_id, s.created_at, s.actual_cost, s.billing_type, FALSE,
          CASE WHEN s.billing_type = 1 THEN s.gift_cost END, CASE WHEN s.billing_type = 1 THEN s.paid_cost END
   FROM usage_cost_splits s
   WHERE s.user_id = $1 AND s.created_at <= $2 AND (s.created_at, s.usage_log_id) > ($3, $4)
     AND NOT EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.id = s.usage_log_id)
   ORDER BY s.created_at, s.usage_log_id LIMIT $5)
) d ORDER BY created_at, id LIMIT $5`, s.userID, until, cursorAt, cursorID, limit)
	} else {
		rows, err = s.tx.QueryContext(ctx, `
SELECT ul.id, ul.created_at, ul.actual_cost::numeric, ul.billing_type::smallint, `+ledgerSubscriptionPaidSQL+`, NULL::numeric, NULL::numeric
FROM usage_logs ul
WHERE ul.user_id = $1 AND ul.created_at > $2 AND ul.created_at <= $3 AND (ul.created_at, ul.id) > ($4, $5)
  AND NOT EXISTS (SELECT 1 FROM usage_cost_splits s WHERE s.usage_log_id = ul.id)
ORDER BY ul.created_at, ul.id LIMIT $6`, s.userID, timePtrArgOr(after, time.Time{}), until, cursorAt, cursorID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var (
			d                    service.LedgerDebit
			billingType          int16
			fixedGift, fixedPaid decimal.NullDecimal
		)
		if err := rows.Scan(&d.UsageLogID, &d.At, &d.ActualCost, &billingType, &d.SubscriptionPaid, &fixedGift, &fixedPaid); err != nil {
			return nil, err
		}
		d.BillingType = int8(billingType)
		if fixedGift.Valid && fixedPaid.Valid {
			d.Fixed = &service.CostSplit{
				UsageLogID: d.UsageLogID, At: d.At, ActualCost: d.ActualCost,
				GiftCost: fixedGift.Decimal, PaidCost: fixedPaid.Decimal, BillingType: d.BillingType,
			}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *usageCostLedgerStore) SaveSplits(ctx context.Context, splits []service.CostSplit) error {
	if len(splits) == 0 {
		return nil
	}
	ids := make([]int64, len(splits))
	users := make([]int64, len(splits))
	ats := make([]string, len(splits))
	actual := make([]string, len(splits))
	gift := make([]string, len(splits))
	paid := make([]string, len(splits))
	types := make([]int64, len(splits))
	for i, sp := range splits {
		ids[i], users[i] = sp.UsageLogID, sp.UserID
		ats[i] = sp.At.UTC().Format(time.RFC3339Nano)
		actual[i], gift[i], paid[i] = sp.ActualCost.String(), sp.GiftCost.String(), sp.PaidCost.String()
		types[i] = int64(sp.BillingType)
	}
	_, err := s.tx.ExecContext(ctx, `
INSERT INTO usage_cost_splits (usage_log_id, user_id, created_at, actual_cost, gift_cost, paid_cost, billing_type, computed_at)
SELECT t.id, t.user_id, t.created_at, t.actual_cost, t.gift_cost, t.paid_cost, t.billing_type, NOW()
FROM unnest($1::bigint[], $2::bigint[], $3::timestamptz[], $4::numeric[], $5::numeric[], $6::numeric[], $7::smallint[])
  AS t(id, user_id, created_at, actual_cost, gift_cost, paid_cost, billing_type)
ON CONFLICT (usage_log_id) DO UPDATE SET
  user_id = EXCLUDED.user_id, created_at = EXCLUDED.created_at, actual_cost = EXCLUDED.actual_cost,
  gift_cost = EXCLUDED.gift_cost, paid_cost = EXCLUDED.paid_cost, billing_type = EXCLUDED.billing_type,
  computed_at = EXCLUDED.computed_at`,
		pq.Array(ids), pq.Array(users), pq.Array(ats), pq.Array(actual), pq.Array(gift), pq.Array(paid), pq.Array(types))
	return err
}

func (s *usageCostLedgerStore) SaveState(ctx context.Context, st service.LedgerUserState) error {
	_, err := s.tx.ExecContext(ctx, `
INSERT INTO usage_cost_ledger_users (user_id, gift_remaining, paid_remaining, processed_until, credit_fp, last_rebuild_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
ON CONFLICT (user_id) DO UPDATE SET
  gift_remaining = EXCLUDED.gift_remaining, paid_remaining = EXCLUDED.paid_remaining,
  processed_until = EXCLUDED.processed_until, credit_fp = EXCLUDED.credit_fp,
  last_rebuild_at = EXCLUDED.last_rebuild_at, updated_at = NOW()`,
		s.userID, st.Buckets.Gift.String(), st.Buckets.Paid.String(), st.ProcessedUntil, st.CreditFP, timePtrArg(st.LastRebuildAt))
	return err
}

// GetUsageSplits 按用量 id 读取拆分及对应的所有者收益；未拆分（待结算）的 id 不在结果中。
// 账户无所有者时不返回收益与赠送资助的账户成本。
func (r *usageCostLedgerRepository) GetUsageSplits(ctx context.Context, ids []int64) (out map[int64]service.UsageRowSettlement, err error) {
	out = make(map[int64]service.UsageRowSettlement, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT s.usage_log_id, s.paid_cost::double precision, s.gift_cost::double precision, (a.owner_user_id IS NOT NULL),
  (CASE WHEN s.actual_cost > 0 THEN (`+accountCostSQL+`) * s.paid_cost / s.actual_cost ELSE 0 END)::double precision,
  (`+accountCostSQL+`)::double precision
FROM usage_cost_splits s
JOIN usage_logs ul ON ul.id = s.usage_log_id
LEFT JOIN accounts a ON a.id = ul.account_id
WHERE s.usage_log_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	for rows.Next() {
		var (
			id                    int64
			v                     service.UsageRowSettlement
			owned                 bool
			earnings, accountCost float64
		)
		if err := rows.Scan(&id, &v.PaidCost, &v.GiftCost, &owned, &earnings, &accountCost); err != nil {
			return nil, err
		}
		if owned {
			gift := accountCost - earnings
			v.Earnings, v.GiftAccountCost = &earnings, &gift
		}
		out[id] = v
	}
	return out, rows.Err()
}

// UsageSplitStats 与 /admin/usage/stats 相同筛选条件下的拆分汇总；收益与赠送资助的账户成本只计有所有者的账户。
func (r *usageCostLedgerRepository) UsageSplitStats(ctx context.Context, filters usagestats.UsageLogFilters) (out *service.UsageSplitStats, err error) {
	conditions, args := usageStatsConditions(filters)
	rows, err := r.db.QueryContext(ctx, `
WITH sc AS (
  SELECT id, account_id, COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1) AS account_cost
  FROM usage_logs
  `+buildWhere(conditions)+`
),
x AS (
  SELECT sc.account_cost, s.paid_cost, s.gift_cost,
    (s.usage_log_id IS NOT NULL) AS split,
    (a.owner_user_id IS NOT NULL) AS owned,
    CASE WHEN s.usage_log_id IS NOT NULL AND s.actual_cost > 0 THEN sc.account_cost * s.paid_cost / s.actual_cost ELSE 0 END AS earn
  FROM sc
  LEFT JOIN usage_cost_splits s ON s.usage_log_id = sc.id
  LEFT JOIN accounts a ON a.id = sc.account_id
)
SELECT COALESCE(SUM(paid_cost), 0)::double precision,
  COALESCE(SUM(gift_cost), 0)::double precision,
  COALESCE(SUM(earn) FILTER (WHERE owned AND split), 0)::double precision,
  COALESCE(SUM(account_cost - earn) FILTER (WHERE owned AND split), 0)::double precision,
  COUNT(*) FILTER (WHERE NOT split)
FROM x`, args...)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows, &err)
	out = &service.UsageSplitStats{}
	if rows.Next() {
		if err := rows.Scan(&out.PaidCost, &out.GiftCost, &out.Earnings, &out.GiftAccountCost, &out.PendingRequests); err != nil {
			return nil, err
		}
	}
	return out, rows.Err()
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func timePtrArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func timePtrArgOr(t *time.Time, def time.Time) time.Time {
	if t == nil {
		return def
	}
	return *t
}
