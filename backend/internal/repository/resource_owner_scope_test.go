package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func ownerTestContext(ownerID int64) context.Context {
	return adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: ownerID, Policy: adminaccess.OwnerPolicy()})
}

func newOwnerScopeProxyRepo(t *testing.T) *proxyRepository {
	t.Helper()
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	return newProxyRepositoryWithSQL(client, db)
}

func TestProxyRepositoryOwnerStampingAndListScope(t *testing.T) {
	repo := newOwnerScopeProxyRepo(t)
	owner, other, admin := ownerTestContext(7), ownerTestContext(8), context.Background()
	newProxy := func(name string, ownerID int64) *service.Proxy {
		// 调用方传入的归属必须被忽略，只认请求身份。
		return &service.Proxy{Name: name, Protocol: "http", Host: "198.51.100.1", Port: 8080, Status: service.StatusActive, FallbackMode: "none", OwnerUserID: &ownerID}
	}
	mine, theirs, legacy := newProxy("mine", 8), newProxy("theirs", 7), newProxy("legacy", 7)
	require.NoError(t, repo.Create(owner, mine))
	require.NoError(t, repo.Create(other, theirs))
	require.NoError(t, repo.Create(admin, legacy))

	got, err := repo.GetByID(admin, mine.ID)
	require.NoError(t, err)
	require.NotNil(t, got.OwnerUserID)
	require.Equal(t, int64(7), *got.OwnerUserID)
	got, err = repo.GetByID(admin, legacy.ID)
	require.NoError(t, err)
	require.Nil(t, got.OwnerUserID)

	names := func(items []service.Proxy) []string {
		out := []string{}
		for _, p := range items {
			out = append(out, p.Name)
		}
		return out
	}
	params := pagination.PaginationParams{Page: 1, PageSize: 50}
	list, _, err := repo.ListWithFilters(owner, params, "", "", "")
	require.NoError(t, err)
	require.Equal(t, []string{"mine"}, names(list))
	active, err := repo.ListActive(owner)
	require.NoError(t, err)
	require.Equal(t, []string{"mine"}, names(active))
	all, _, err := repo.ListWithFilters(admin, params, "", "", "")
	require.NoError(t, err)
	require.Len(t, all, 3)

	// 重复检测只在本人代理内进行，不泄露他人或管理员代理是否存在。
	exists, err := repo.ExistsByHostPortAuth(ownerTestContext(9), "198.51.100.1", 8080, "", "")
	require.NoError(t, err)
	require.False(t, exists)
	exists, err = repo.ExistsByHostPortAuth(admin, "198.51.100.1", 8080, "", "")
	require.NoError(t, err)
	require.True(t, exists)
}

// 管理员账户借用所有者代理时，所有者看到的 account_count 只含本人账户；管理员仍看到全部。
func TestProxyAccountCountOwnerScope(t *testing.T) {
	repo := newOwnerScopeProxyRepo(t)
	owner, admin := ownerTestContext(7), context.Background()
	p := &service.Proxy{Name: "mine", Protocol: "http", Host: "198.51.100.2", Port: 8080, Status: service.StatusActive, FallbackMode: "none"}
	require.NoError(t, repo.Create(owner, p))
	for _, ownerID := range []*int64{ptrInt64(7), nil, nil} {
		_, err := repo.client.Account.Create().SetName("a").SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
			SetStatus(service.StatusActive).SetCredentials(map[string]any{"api_key": "sk"}).SetProxyID(p.ID).SetNillableOwnerUserID(ownerID).Save(admin)
		require.NoError(t, err)
	}
	params := pagination.PaginationParams{Page: 1, PageSize: 50}
	mine, _, err := repo.ListWithFiltersAndAccountCount(owner, params, "", "", "")
	require.NoError(t, err)
	require.Len(t, mine, 1)
	require.Equal(t, int64(1), mine[0].AccountCount)
	active, err := repo.ListActiveWithAccountCount(owner)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, int64(1), active[0].AccountCount)
	all, _, err := repo.ListWithFiltersAndAccountCount(admin, params, "", "", "")
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, int64(3), all[0].AccountCount)
}

func ptrInt64(v int64) *int64 { return &v }

// 归属批量查询：每类一次查询，返回真实归属（不受所有者列表范围影响），缺失 id 不出现。
func TestResourceOwnerIDsBatchLookup(t *testing.T) {
	proxies := newOwnerScopeProxyRepo(t)
	client, admin := proxies.client, context.Background()
	repo := newAccountRepositoryWithSQL(client, proxies.sql, nil)
	mine := &service.Proxy{Name: "p", Protocol: "http", Host: "198.51.100.3", Port: 1, Status: service.StatusActive, FallbackMode: "none"}
	require.NoError(t, proxies.Create(ownerTestContext(7), mine))
	g1, err := client.Group.Create().SetName("g1").SetOwnerUserID(7).Save(admin)
	require.NoError(t, err)
	g2, err := client.Group.Create().SetName("g2").Save(admin)
	require.NoError(t, err)
	a1, err := client.Account.Create().SetName("a").SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).SetCredentials(map[string]any{"api_key": "sk"}).SetOwnerUserID(8).Save(admin)
	require.NoError(t, err)

	// 以其他所有者身份查询也必须返回真实归属，不能被列表范围过滤成“不存在”。
	ctx := ownerTestContext(9)
	groups, err := repo.ResourceOwnerIDs(ctx, service.OwnerResourceGroup, []int64{g1.ID, g2.ID, g1.ID, 999, 0})
	require.NoError(t, err)
	require.Equal(t, map[int64]*int64{g1.ID: ptrInt64(7), g2.ID: nil}, groups)
	accounts, err := repo.ResourceOwnerIDs(ctx, service.OwnerResourceAccount, []int64{a1.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]*int64{a1.ID: ptrInt64(8)}, accounts)
	got, err := repo.ResourceOwnerIDs(ctx, service.OwnerResourceProxy, []int64{mine.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]*int64{mine.ID: ptrInt64(7)}, got)
	_, err = repo.ResourceOwnerIDs(ctx, "user", []int64{1})
	require.Error(t, err)
}

func TestUsageLogListWithFiltersOwnerScope(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM usage_logs WHERE account_id IN \\(SELECT id FROM accounts WHERE owner_user_id = \\$1\\)").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery("SELECT .* FROM usage_logs WHERE account_id IN \\(SELECT id FROM accounts WHERE owner_user_id = \\$1\\) ORDER BY id DESC LIMIT \\$2 OFFSET \\$3").
		WithArgs(int64(7), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	// 所有者范围不能走估算总数的快速分页，避免暴露全站用量规模。
	logs, _, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, usagestats.UsageLogFilters{OwnerUserID: 7})
	require.NoError(t, err)
	require.Empty(t, logs)
	require.NoError(t, mock.ExpectationsWereMet())
}
