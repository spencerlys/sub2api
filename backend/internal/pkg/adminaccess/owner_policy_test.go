//go:build unit

package adminaccess

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerPolicyAllowsOnlyOwnerCatalogue(t *testing.T) {
	owner := OwnerPolicy()
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/accounts"}, {"POST", "/api/v1/admin/accounts"}, {"PUT", "/api/v1/admin/accounts/:id"},
		{"DELETE", "/api/v1/admin/accounts/:id"}, {"POST", "/api/v1/admin/accounts/:id/test"},
		{"GET", "/api/v1/admin/accounts/:id/stats"}, {"POST", "/api/v1/admin/accounts/today-stats/batch"},
		{"GET", "/api/v1/admin/accounts/health"},
		{"POST", "/api/v1/admin/accounts/generate-auth-url"}, {"POST", "/api/v1/admin/accounts/exchange-code"},
		{"POST", "/api/v1/admin/accounts/exchange-setup-token-code"},
		{"GET", "/api/v1/admin/groups"}, {"POST", "/api/v1/admin/groups"}, {"PUT", "/api/v1/admin/groups/:id"},
		{"GET", "/api/v1/admin/proxies/all"}, {"POST", "/api/v1/admin/proxies/:id/test"},
		{"GET", "/api/v1/admin/usage"}, {"GET", "/api/v1/admin/usage/stats"},
		{"GET", "/api/v1/admin/compliance"}, {"POST", "/api/v1/admin/compliance/accept"},
		{"GET", "/api/v1/admin/owner-settlements/me"},
	} {
		require.True(t, RouteAllowed(owner, route.method, route.path), route.method+" "+route.path)
	}
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/users"}, {"PUT", "/api/v1/admin/users/:id"}, {"GET", "/api/v1/admin/users/:id/usage"},
		{"GET", "/api/v1/admin/api-keys"}, {"GET", "/api/v1/admin/dashboard/stats"}, {"PUT", "/api/v1/admin/settings"},
		{"GET", "/api/v1/admin/settings/admin-api-keys"}, {"GET", "/api/v1/admin/ops/errors"},
		{"GET", "/api/v1/admin/accounts/data"}, {"POST", "/api/v1/admin/accounts/data"},
		{"POST", "/api/v1/admin/accounts/bulk-update"}, {"POST", "/api/v1/admin/accounts/batch"},
		{"POST", "/api/v1/admin/accounts/batch-delete"}, {"POST", "/api/v1/admin/accounts/:id/duplicate"},
		{"POST", "/api/v1/admin/accounts/sync/crs"}, {"POST", "/api/v1/admin/accounts/import/codex-session"},
		{"POST", "/api/v1/admin/accounts/:id/shadow"}, {"POST", "/api/v1/admin/accounts/batch-update-credentials"},
		{"GET", "/api/v1/admin/proxies/data"}, {"POST", "/api/v1/admin/proxies/data"}, {"POST", "/api/v1/admin/proxies/batch"},
		{"POST", "/api/v1/admin/proxies/batch-delete"}, {"GET", "/api/v1/admin/proxies/:id/accounts"},
		{"PUT", "/api/v1/admin/groups/:id/rate-multipliers"}, {"GET", "/api/v1/admin/groups/:id/api-keys"},
		{"PUT", "/api/v1/admin/groups/sort-order"}, {"POST", "/api/v1/admin/groups/:id/duplicate"},
		{"GET", "/api/v1/admin/groups/:id/subscriptions"}, {"POST", "/api/v1/admin/groups/:id/composite-routes"},
		{"GET", "/api/v1/admin/usage/search-users"}, {"GET", "/api/v1/admin/usage/search-api-keys"},
		{"POST", "/api/v1/admin/usage/cleanup-tasks"}, {"GET", "/api/v1/admin/payment/orders"},
		{"POST", "/api/v1/admin/subscriptions/assign"}, {"GET", "/api/v1/admin/system/version"},
		{"GET", "/api/v1/admin/owner-settlements"}, {"GET", "/api/v1/admin/finance/summary"},
		{"POST", "/api/v1/admin/finance/rebuild-ledger"},
		{"GET", "/api/v1/admin/owner-payouts"}, {"POST", "/api/v1/admin/owner-payouts"},
		{"DELETE", "/api/v1/admin/owner-payouts/:id"}, {"PUT", "/api/v1/admin/users/:id/balance-history/:recordId/source"},
		{"GET", ""},
	} {
		require.False(t, RouteAllowed(owner, route.method, route.path), route.method+" "+route.path)
	}
}

// 管理密钥即使在存储策略里写入所有者权限 ID，也不能获得所有者目录。
func TestOwnerCatalogueNotGrantableToAdminKeys(t *testing.T) {
	var forged Policy
	require.NoError(t, json.Unmarshal([]byte(`{"global_permissions":["owner.accounts","owner.usage"],"owner":true}`), &forged))
	require.False(t, RouteAllowed(forged, "GET", "/api/v1/admin/accounts"))
	require.False(t, RouteAllowed(forged, "GET", "/api/v1/admin/usage"))
	require.Error(t, ValidatePolicy(Policy{GlobalPermissions: []string{"owner.accounts"}}))
	for _, permission := range PermissionCatalog() {
		require.NotEqual(t, "owner", permission.Scope, permission.ID)
	}
	encoded, err := json.Marshal(OwnerPolicy())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"owner":`)
}

func TestOwnerPrincipalIsSeparateFromAdminKeyPrincipal(t *testing.T) {
	owner := WithPrincipal(context.Background(), &Principal{OwnerUserID: 7, Policy: OwnerPolicy()})
	require.Nil(t, FromContext(owner))
	require.NotNil(t, OwnerFromContext(owner))
	require.Equal(t, int64(7), OwnerUserID(owner))

	key := WithPrincipal(context.Background(), &Principal{KeyID: "k", CreatorID: 1})
	require.NotNil(t, FromContext(key))
	require.Nil(t, OwnerFromContext(key))
	require.Zero(t, OwnerUserID(key))
	require.Zero(t, OwnerUserID(context.Background()))
}

// 所有者策略只在包初始化时计算一次；调用方追加权限不能污染共享的目录切片。
func TestOwnerPolicyIsComputedOnceAndAppendSafe(t *testing.T) {
	first, second := OwnerPolicy(), OwnerPolicy()
	require.NotEmpty(t, first.GlobalPermissions)
	require.Same(t, &first.GlobalPermissions[0], &second.GlobalPermissions[0])
	want := slices.Clone(first.GlobalPermissions)
	_ = append(first.GlobalPermissions, "users.read")
	require.Equal(t, want, OwnerPolicy().GlobalPermissions)
	require.False(t, RouteAllowed(OwnerPolicy(), "GET", "/api/v1/admin/users"))
	require.True(t, OwnerPolicy().owner)
}
