//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 聚合查询只读数值与状态列，绝不触及用户身份、API Key、IP 或请求内容。
func TestAccountHealthQueriesExcludeIdentityAndContent(t *testing.T) {
	forbidden := regexp.MustCompile(`\b(user_id|api_key_id|client_ip|ip_address|request_body|request_headers|error_body|user_agent|upstream_error_detail|upstream_errors)\b`)
	for name, q := range map[string]string{"usage": accountHealthUsageQuery, "errors": accountHealthErrorsQuery} {
		require.Empty(t, forbidden.FindAllString(q, -1), name)
		// 单条 GROUP BY 聚合，按账户 ID 数组与时间范围过滤（命中 (account_id, created_at) 索引）。
		require.Contains(t, q, "account_id = ANY($1)", name)
		require.Contains(t, q, "created_at >= $2 AND created_at < $3", name)
		require.Contains(t, q, "GROUP BY account_id", name)
	}
	require.Contains(t, accountHealthUsageQuery, "FILTER (WHERE stream AND first_token_ms IS NOT NULL)")
	// 拆分只按主键关联且只暴露金额列。
	require.Contains(t, accountHealthUsageQuery, "WHERE s.usage_log_id = usage_logs.id")
}

func TestAccountHealthAggregatesScanRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &accountHealthRepository{sql: db}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	at := start.Add(time.Minute)

	mock.ExpectQuery(regexp.QuoteMeta(accountHealthUsageQuery)).
		WithArgs(sqlmock.AnyArg(), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "requests_ok", "tokens_in", "tokens_out", "tokens_cache", "cost", "ttft_p50", "ttft_p95", "duration_p50", "last_success_at",
			"official_cost", "account_cost", "earnings", "pending_requests"}).
			AddRow(int64(1), int64(4), int64(10), int64(20), int64(30), 1.25, 300.4, 899.6, nil, at, 2.5, 1.0, 0.6, int64(1)))
	usage, err := repo.AggregateAccountHealthUsage(context.Background(), []int64{1, 2}, start, end)
	require.NoError(t, err)
	require.Len(t, usage, 1)
	require.EqualValues(t, 4, usage[1].RequestsOK)
	require.Equal(t, 300, *usage[1].TTFTP50Ms)
	require.Equal(t, 900, *usage[1].TTFTP95Ms)
	require.Nil(t, usage[1].DurationP50Ms)
	require.True(t, usage[1].LastSuccessAt.Equal(at))
	require.InDelta(t, 2.5, usage[1].OfficialCost, 1e-12)
	require.InDelta(t, 1.0, usage[1].AccountCost, 1e-12)
	require.InDelta(t, 0.6, usage[1].Earnings, 1e-12)
	require.EqualValues(t, 1, usage[1].PendingRequests)

	mock.ExpectQuery(regexp.QuoteMeta(accountHealthErrorsQuery)).
		WithArgs(sqlmock.AnyArg(), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "total", "rate_limit", "overloaded", "auth", "upstream_5xx", "timeout", "other", "last_error_at", "category", "code"}).
			AddRow(int64(1), int64(3), int64(1), int64(0), int64(0), int64(0), int64(2), int64(0), at, "timeout", int64(200)).
			AddRow(int64(2), int64(1), int64(1), int64(0), int64(0), int64(0), int64(0), int64(0), at, "rate_limit", int64(429)))
	errs, err := repo.AggregateAccountHealthErrors(context.Background(), []int64{1, 2}, start, end)
	require.NoError(t, err)
	require.Equal(t, service.AccountHealthErrorCounts{RateLimit: 1, Timeout: 2}, errs[1].ByCategory)
	// 已被兜底成功的记录（最终码 <400）不展示状态码。
	require.Nil(t, errs[1].LastErrorStatus)
	require.Equal(t, 429, *errs[2].LastErrorStatus)
	require.NoError(t, mock.ExpectationsWereMet())

	// 空 ID 列表不发查询。
	empty, err := repo.AggregateAccountHealthUsage(context.Background(), nil, start, end)
	require.NoError(t, err)
	require.Empty(t, empty)
}
