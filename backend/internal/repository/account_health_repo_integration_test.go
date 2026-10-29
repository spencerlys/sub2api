//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func accountHealthIDs(accounts []service.Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	return ids
}

// 所有者只看到本人账户（看不到其他所有者与管理员账户），管理员看到全部并可按所有者筛选；
// 错误按状态码归类，百分位只取流式请求的首字延迟。
func TestAccountHealthRepository_ScopeCategoriesAndPercentiles(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	ownerA := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("health-a-%d@example.com", time.Now().UnixNano())})
	ownerB := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("health-b-%d@example.com", time.Now().UnixNano())})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: ownerB.ID})
	accA := mustCreateAccount(t, client, &service.Account{Name: "health-a", ErrorMessage: "upstream 401"})
	accB := mustCreateAccount(t, client, &service.Account{Name: "health-b"})
	accAdmin := mustCreateAccount(t, client, &service.Account{Name: "health-admin"})
	_, err := integrationDB.ExecContext(ctx, "UPDATE accounts SET owner_user_id = $1 WHERE id = $2", ownerA.ID, accA.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE accounts SET owner_user_id = $1 WHERE id = $2", ownerB.ID, accB.ID)
	require.NoError(t, err)
	ids := []int64{accA.ID, accB.ID, accAdmin.ID}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE account_id = ANY($1)", pq.Array(ids))
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_error_logs WHERE account_id = ANY($1)", pq.Array(ids))
	})

	repo := NewAccountHealthRepository(client, integrationDB)
	ownerCtx := func(id int64) context.Context {
		return adminaccess.WithPrincipal(ctx, &adminaccess.Principal{OwnerUserID: id, Policy: adminaccess.OwnerPolicy()})
	}

	// 归属范围
	got, err := repo.ListAccountHealthAccounts(ownerCtx(ownerA.ID), nil)
	require.NoError(t, err)
	require.Equal(t, []int64{accA.ID}, accountHealthIDs(got))
	require.Equal(t, "upstream 401", got[0].ErrorMessage)
	got, err = repo.ListAccountHealthAccounts(ownerCtx(ownerB.ID), nil)
	require.NoError(t, err)
	require.Equal(t, []int64{accB.ID}, accountHealthIDs(got))
	// 所有者上下文中再附加他人筛选也拿不到他人账户。
	got, err = repo.ListAccountHealthAccounts(ownerCtx(ownerA.ID), &ownerB.ID)
	require.NoError(t, err)
	require.Empty(t, got)
	got, err = repo.ListAccountHealthAccounts(ctx, nil)
	require.NoError(t, err)
	require.Subset(t, accountHealthIDs(got), ids)
	got, err = repo.ListAccountHealthAccounts(ctx, &ownerB.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{accB.ID}, accountHealthIDs(got))

	start := time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	seq := 0
	insertUsage := func(accountID int64, at time.Time, stream bool, ttft *int, duration int) {
		t.Helper()
		seq++
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs
			(user_id, api_key_id, account_id, request_id, model, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, actual_cost, total_cost, stream, first_token_ms, duration_ms, created_at)
			VALUES ($1,$2,$3,$4,'claude-test',100,10,5,20,0.5,0.5,$5,$6,$7,$8)`,
			ownerB.ID, key.ID, accountID, fmt.Sprintf("health-%d-%d", time.Now().UnixNano(), seq), stream, ttft, duration, at)
		require.NoError(t, err)
	}
	ms := func(v int) *int { return &v }
	for i, v := range []int{100, 200, 300, 400, 1000} {
		insertUsage(accA.ID, start.Add(time.Duration(i+1)*time.Minute), true, ms(v), 2000)
	}
	insertUsage(accA.ID, start.Add(10*time.Minute), false, ms(99999), 2000) // 非流式不计入 TTFT
	insertUsage(accA.ID, start.Add(-time.Minute), true, ms(1), 1)           // 窗口外
	insertUsage(accB.ID, start.Add(time.Minute), true, ms(50), 50)

	type errRow struct {
		phase, owner     string
		status           int
		upstream         *int
		countTokens, biz bool
		account          int64
		at               time.Time
		expectCategory   string
	}
	in := start.Add(time.Hour)
	rows := []errRow{
		{"upstream", "provider", 429, ms(429), false, false, accA.ID, in, "rate_limit"},
		{"upstream", "provider", 529, nil, false, false, accA.ID, in, "overloaded"},
		{"account_auth", "provider", 502, ms(0), false, false, accA.ID, in, "auth"},
		{"upstream", "provider", 502, ms(401), false, false, accA.ID, in, "auth"},
		{"upstream", "provider", 502, ms(503), false, false, accA.ID, in, "upstream_5xx"},
		{"upstream", "provider", 502, nil, false, false, accA.ID, in, "timeout"}, // 传输层失败
		{"upstream", "provider", 200, nil, false, false, accA.ID, in, "timeout"}, // 已被兜底的传输层失败
		{"upstream", "provider", 502, ms(504), false, false, accA.ID, in, "timeout"},
		{"upstream", "provider", 400, ms(400), false, false, accA.ID, in.Add(time.Minute), "other"},
		// 以下不计入
		{"request", "client", 400, nil, false, false, accA.ID, in, ""},
		{"upstream", "provider", 429, ms(429), true, false, accA.ID, in, ""},
		{"upstream", "provider", 429, ms(429), false, true, accA.ID, in, ""},
		{"upstream", "provider", 429, ms(429), false, false, accA.ID, end, ""},
		{"upstream", "provider", 429, ms(429), false, false, accB.ID, in, "rate_limit"},
	}
	for _, r := range rows {
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO ops_error_logs
			(account_id, error_phase, error_type, error_owner, severity, status_code, upstream_status_code, is_count_tokens, is_business_limited, created_at)
			VALUES ($1,$2,'upstream_error',$3,'P1',$4,$5,$6,$7,$8)`,
			r.account, r.phase, r.owner, r.status, r.upstream, r.countTokens, r.biz, r.at)
		require.NoError(t, err)
	}

	usage, err := repo.AggregateAccountHealthUsage(ctx, []int64{accA.ID, accB.ID}, start, end)
	require.NoError(t, err)
	a := usage[accA.ID]
	require.NotNil(t, a)
	require.EqualValues(t, 6, a.RequestsOK)
	require.EqualValues(t, 600, a.TokensIn)
	require.EqualValues(t, 60, a.TokensOut)
	require.EqualValues(t, 150, a.TokensCache)
	require.InDelta(t, 3.0, a.Cost, 1e-9)
	require.NotNil(t, a.TTFTP50Ms)
	require.Equal(t, 300, *a.TTFTP50Ms)
	require.NotNil(t, a.TTFTP95Ms)
	require.Equal(t, 880, *a.TTFTP95Ms) // 400 + 0.8*(1000-400)
	require.NotNil(t, a.DurationP50Ms)
	require.Equal(t, 2000, *a.DurationP50Ms)
	require.NotNil(t, a.LastSuccessAt)
	require.True(t, a.LastSuccessAt.Equal(start.Add(10*time.Minute)))
	require.EqualValues(t, 1, usage[accB.ID].RequestsOK)

	errs, err := repo.AggregateAccountHealthErrors(ctx, []int64{accA.ID}, start, end)
	require.NoError(t, err)
	require.NotContains(t, errs, accB.ID)
	e := errs[accA.ID]
	require.NotNil(t, e)
	require.Equal(t, service.AccountHealthErrorCounts{RateLimit: 1, Overloaded: 1, Auth: 2, Upstream5xx: 1, Timeout: 3, Other: 1}, e.ByCategory)
	require.EqualValues(t, 9, e.Total)
	require.Equal(t, "other", e.LastErrorCategory)
	require.NotNil(t, e.LastErrorStatus)
	require.Equal(t, 400, *e.LastErrorStatus)
	require.True(t, e.LastErrorAt.Equal(in.Add(time.Minute)))

	// 服务层：所有者传入他人筛选也只得到本人账户。
	report, err := service.NewAccountHealthService(repo).GetAccountHealth(ownerCtx(ownerA.ID), "7d", &ownerB.ID)
	require.NoError(t, err)
	require.Len(t, report.Items, 1)
	require.Equal(t, accA.ID, report.Items[0].ID)
}
