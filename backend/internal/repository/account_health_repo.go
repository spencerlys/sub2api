package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// accountHealthRepository 账号健康页的只读聚合。只读取账户状态列与按账户聚合的数值，
// 不查询 user_id、api_key_id、IP、请求/响应内容等字段。
type accountHealthRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewAccountHealthRepository(client *dbent.Client, sqlDB *sql.DB) service.AccountHealthRepository {
	return &accountHealthRepository{client: client, sql: sqlDB}
}

// accountHealthErrorCodeSQL 取账户侧状态码：优先上游状态码，缺失时退回最终响应码。
const accountHealthErrorCodeSQL = `CASE WHEN COALESCE(upstream_status_code, 0) > 0 THEN upstream_status_code ELSE COALESCE(status_code, 0) END`

// accountHealthErrorCategorySQL 错误分类（按顺序命中）：
//   - auth：账户鉴权失败阶段（account_auth），或 401/403；
//   - rate_limit：429；overloaded：529；
//   - timeout：network 阶段、记录了网络错误类型、408/504/524，
//     或上游未返回任何 HTTP 状态（传输层失败：最终 502，或已被其他账户兜底成功）；
//   - upstream_5xx：其余 5xx；other：其余（如上游 400/404）。
const accountHealthErrorCategorySQL = `CASE
    WHEN error_phase = 'account_auth' OR ` + accountHealthErrorCodeSQL + ` IN (401, 403) THEN 'auth'
    WHEN ` + accountHealthErrorCodeSQL + ` = 429 THEN 'rate_limit'
    WHEN ` + accountHealthErrorCodeSQL + ` = 529 THEN 'overloaded'
    WHEN error_phase = 'network'
      OR COALESCE(network_error_type, '') <> ''
      OR ` + accountHealthErrorCodeSQL + ` IN (408, 504, 524)
      OR (COALESCE(upstream_status_code, 0) = 0 AND (COALESCE(status_code, 0) < 400 OR status_code = 502)) THEN 'timeout'
    WHEN ` + accountHealthErrorCodeSQL + ` BETWEEN 500 AND 599 THEN 'upstream_5xx'
    ELSE 'other'
  END`

// accountHealthErrorsQuery 只统计归因到账户的服务方错误（error_owner=provider），
// 排除 count_tokens 与用户侧业务限制；未到达账户的错误没有 account_id，不在范围内。
// 索引：idx_ops_error_logs_account_time (account_id, created_at DESC) WHERE account_id IS NOT NULL。
const accountHealthErrorsQuery = `
WITH e AS (
  SELECT id, account_id, created_at,
    ` + accountHealthErrorCodeSQL + ` AS code,
    ` + accountHealthErrorCategorySQL + ` AS category
  FROM ops_error_logs
  WHERE account_id = ANY($1)
    AND created_at >= $2 AND created_at < $3
    AND is_count_tokens = FALSE
    AND is_business_limited = FALSE
    AND error_owner = 'provider'
),
agg AS (
  SELECT account_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE category = 'rate_limit') AS rate_limit,
    COUNT(*) FILTER (WHERE category = 'overloaded') AS overloaded,
    COUNT(*) FILTER (WHERE category = 'auth') AS auth,
    COUNT(*) FILTER (WHERE category = 'upstream_5xx') AS upstream_5xx,
    COUNT(*) FILTER (WHERE category = 'timeout') AS timeout,
    COUNT(*) FILTER (WHERE category = 'other') AS other,
    MAX(created_at) AS last_error_at
  FROM e
  GROUP BY account_id
),
last AS (
  SELECT DISTINCT ON (account_id) account_id, category, code
  FROM e
  ORDER BY account_id, created_at DESC, id DESC
)
SELECT agg.account_id, agg.total, agg.rate_limit, agg.overloaded, agg.auth, agg.upstream_5xx, agg.timeout, agg.other,
  agg.last_error_at, last.category, last.code
FROM agg
JOIN last ON last.account_id = agg.account_id`

// accountHealthAccountCostSQL 账户成本（与 accountCostSQL 同口径，本查询的 usage_logs 不带别名）。
const accountHealthAccountCostSQL = `COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)`

// accountHealthUsageQuery 成功请求聚合；TTFT 只取流式请求。官方价、账户成本与按精确账本（usage_cost_splits）的应付
// （口径见 ownerPayoutJoinSQL）：拆分以 LATERAL 子查询按主键关联，只暴露金额列，不引入用户身份。
// 索引：idx_usage_logs_account_created_at (account_id, created_at)。
var accountHealthUsageQuery = `
SELECT account_id,
  COUNT(*) AS requests_ok,
  COALESCE(SUM(input_tokens), 0) AS tokens_in,
  COALESCE(SUM(output_tokens), 0) AS tokens_out,
  COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) AS tokens_cache,
  COALESCE(SUM(usage_logs.actual_cost), 0) AS cost,
  percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE stream AND first_token_ms IS NOT NULL) AS ttft_p50,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE stream AND first_token_ms IS NOT NULL) AS ttft_p95,
  percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS duration_p50,
  MAX(created_at) AS last_success_at,
  COALESCE(SUM(total_cost), 0) AS official_cost,
  COALESCE(SUM(` + accountHealthAccountCostSQL + `), 0) AS account_cost,
  COALESCE(SUM(` + ownerEarningsSQL(accountHealthAccountCostSQL, "sp") + `), 0) AS earnings,
  COUNT(*) FILTER (WHERE sp.paid_cost IS NULL) AS pending_requests
FROM usage_logs
LEFT JOIN LATERAL (
  SELECT s.paid_cost, s.actual_cost FROM usage_cost_splits s WHERE s.usage_log_id = usage_logs.id
) sp ON TRUE
` + ownerPayoutJoinSQL("usage_logs") + `
WHERE account_id = ANY($1)
  AND created_at >= $2 AND created_at < $3
GROUP BY account_id`

// ListAccountHealthAccounts 列出范围内账户的状态列；所有者请求由 ownerAccountScope 限定为本人，
// ownerUserID 仅作为管理员的附加筛选。不加载凭证与扩展字段。
func (r *accountHealthRepository) ListAccountHealthAccounts(ctx context.Context, ownerUserID *int64) ([]service.Account, error) {
	q := r.client.Account.Query().Where(ownerAccountScope(ctx)...)
	if ownerUserID != nil && *ownerUserID > 0 {
		q = q.Where(dbaccount.OwnerUserIDEQ(*ownerUserID))
	}
	rows, err := q.Order(dbent.Asc(dbaccount.FieldID)).Select(
		dbaccount.FieldID, dbaccount.FieldName, dbaccount.FieldPlatform, dbaccount.FieldType,
		dbaccount.FieldStatus, dbaccount.FieldSchedulable, dbaccount.FieldErrorMessage, dbaccount.FieldOwnerUserID,
		dbaccount.FieldRateLimitResetAt, dbaccount.FieldOverloadUntil, dbaccount.FieldTempUnschedulableUntil,
	).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.Account, 0, len(rows))
	for _, m := range rows {
		a := service.Account{
			ID:                     m.ID,
			Name:                   m.Name,
			Platform:               m.Platform,
			Type:                   m.Type,
			Status:                 m.Status,
			Schedulable:            m.Schedulable,
			OwnerUserID:            m.OwnerUserID,
			RateLimitResetAt:       m.RateLimitResetAt,
			OverloadUntil:          m.OverloadUntil,
			TempUnschedulableUntil: m.TempUnschedulableUntil,
		}
		if m.ErrorMessage != nil {
			a.ErrorMessage = *m.ErrorMessage
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *accountHealthRepository) AggregateAccountHealthUsage(ctx context.Context, accountIDs []int64, start, end time.Time) (out map[int64]*service.AccountHealthUsage, err error) {
	out = make(map[int64]*service.AccountHealthUsage, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, accountHealthUsageQuery, pq.Array(accountIDs), start, end)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		var (
			id                       int64
			u                        service.AccountHealthUsage
			ttftP50, ttftP95, durP50 sql.NullFloat64
			lastSuccess              sql.NullTime
		)
		if err := rows.Scan(&id, &u.RequestsOK, &u.TokensIn, &u.TokensOut, &u.TokensCache, &u.Cost, &ttftP50, &ttftP95, &durP50, &lastSuccess,
			&u.OfficialCost, &u.AccountCost, &u.Earnings, &u.PendingRequests); err != nil {
			return nil, err
		}
		u.TTFTP50Ms, u.TTFTP95Ms, u.DurationP50Ms = floatToIntPtr(ttftP50), floatToIntPtr(ttftP95), floatToIntPtr(durP50)
		if lastSuccess.Valid {
			t := lastSuccess.Time
			u.LastSuccessAt = &t
		}
		out[id] = &u
	}
	return out, rows.Err()
}

func (r *accountHealthRepository) AggregateAccountHealthErrors(ctx context.Context, accountIDs []int64, start, end time.Time) (out map[int64]*service.AccountHealthErrors, err error) {
	out = make(map[int64]*service.AccountHealthErrors, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, accountHealthErrorsQuery, pq.Array(accountIDs), start, end)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		var (
			id       int64
			e        service.AccountHealthErrors
			c        = &e.ByCategory
			lastAt   sql.NullTime
			lastCat  sql.NullString
			lastCode sql.NullInt64
		)
		if err := rows.Scan(&id, &e.Total, &c.RateLimit, &c.Overloaded, &c.Auth, &c.Upstream5xx, &c.Timeout, &c.Other, &lastAt, &lastCat, &lastCode); err != nil {
			return nil, err
		}
		if lastAt.Valid {
			t := lastAt.Time
			e.LastErrorAt = &t
		}
		e.LastErrorCategory = lastCat.String
		// 已被其他账户兜底成功的记录最终码 <400，不作为该账户的错误状态码展示。
		if lastCode.Valid && lastCode.Int64 >= 400 {
			code := int(lastCode.Int64)
			e.LastErrorStatus = &code
		}
		out[id] = &e
	}
	return out, rows.Err()
}

// ListAccountOwnerLabels 所有者邮箱与用户名（含已软删除的用户，便于识别历史账户归属）。
func (r *accountHealthRepository) ListAccountOwnerLabels(ctx context.Context, ownerIDs []int64) (out map[int64]service.OwnerIdentity, err error) {
	out = make(map[int64]service.OwnerIdentity, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT id, email, COALESCE(username, '') FROM users WHERE id = ANY($1)`, pq.Array(ownerIDs))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		var o service.OwnerIdentity
		if err := rows.Scan(&o.UserID, &o.Email, &o.Username); err != nil {
			return nil, err
		}
		out[o.UserID] = o
	}
	return out, rows.Err()
}
