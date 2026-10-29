//go:build unit

package repository

import (
	"context"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// captureAccountQueryWhere 执行一次账户查询并返回生成 SQL 的 WHERE 子句。
func captureAccountQueryWhere(t *testing.T, run func(ctx context.Context, repo *accountRepository) error) string {
	t.Helper()
	var capturedSQL string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureEntQueryMatcher{actual: &capturedSQL}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	mock.ExpectQuery("accounts").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	require.NoError(t, run(context.Background(), newAccountRepositoryWithSQL(client, db, nil)))
	require.NoError(t, mock.ExpectationsWereMet())
	_, where, found := strings.Cut(normalizeSQLWhitespace(capturedSQL), " WHERE ")
	require.True(t, found, capturedSQL)
	return where
}

// 所有者账户只能经分组绑定进入调度：未分组池、简易模式全平台池及其模型可用性候选都必须排除它们，
// 分组池不受影响。
func TestSchedulablePoolsWithoutGroupExcludeOwnerAccounts(t *testing.T) {
	platforms := []string{service.PlatformAnthropic, service.PlatformAntigravity}
	ownerExcluded := map[string]func(ctx context.Context, repo *accountRepository) error{
		"ListSchedulableByPlatform": func(ctx context.Context, repo *accountRepository) error {
			_, err := repo.ListSchedulableByPlatform(ctx, service.PlatformAnthropic)
			return err
		},
		"ListSchedulableByPlatforms": func(ctx context.Context, repo *accountRepository) error {
			_, err := repo.ListSchedulableByPlatforms(ctx, platforms)
			return err
		},
		"ListSchedulableUngroupedByPlatform": func(ctx context.Context, repo *accountRepository) error {
			_, err := repo.ListSchedulableUngroupedByPlatform(ctx, service.PlatformAnthropic)
			return err
		},
		"ListSchedulableUngroupedByPlatforms": func(ctx context.Context, repo *accountRepository) error {
			_, err := repo.ListSchedulableUngroupedByPlatforms(ctx, platforms)
			return err
		},
		"ListModelAvailabilityCandidates(ungrouped)": func(ctx context.Context, repo *accountRepository) error {
			_, err := repo.ListModelAvailabilityCandidates(ctx, nil, platforms, false)
			return err
		},
		"ListModelAvailabilityCandidates(simple)": func(ctx context.Context, repo *accountRepository) error {
			_, err := repo.ListModelAvailabilityCandidates(ctx, nil, platforms, true)
			return err
		},
	}
	for name, run := range ownerExcluded {
		where := captureAccountQueryWhere(t, run)
		require.Contains(t, where, `"owner_user_id" IS NULL`, name)
	}

	where := captureAccountQueryWhere(t, func(ctx context.Context, repo *accountRepository) error {
		_, err := repo.ListSchedulableByGroupIDAndPlatform(ctx, 9, service.PlatformAnthropic)
		return err
	})
	require.NotContains(t, where, "owner_user_id", "分组池必须保留绑定到该分组的所有者账户")
}
