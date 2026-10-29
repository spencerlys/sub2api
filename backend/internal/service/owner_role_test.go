//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateUser_PromoteToOwner(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser}}
	svc := &adminServiceImpl{userRepo: &rpmUserRepoStub{userRepoStub: base}, redeemCodeRepo: &redeemRepoStub{}, authCacheInvalidator: &authCacheInvalidatorStub{}}

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleOwner})
	require.NoError(t, err)
	require.Equal(t, RoleOwner, updated.Role)
	require.True(t, updated.IsOwner())
	require.False(t, updated.IsAdmin())
}

// 把最后一个管理员改成所有者同样会失去后台，必须与降为普通用户一样受保护。
func TestAdminService_UpdateUser_LastAdminCannotBecomeOwner(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "a@example.com", Role: RoleAdmin}}
	repo := &roleGuardUserRepoStub{rpmUserRepoStub: &rpmUserRepoStub{userRepoStub: base}, adminTotal: 1}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}, authCacheInvalidator: &authCacheInvalidatorStub{}}

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleOwner})
	require.Error(t, err)
}

func TestStripOwnerAccountBilling(t *testing.T) {
	rate, sync := 0.5, true
	owner := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: 7, Policy: adminaccess.OwnerPolicy()})

	r, s := &rate, &sync
	extra := map[string]any{openAILongContextBillingEnabledKey: true, "base_rpm": 10}
	stripOwnerAccountBilling(owner, &r, &s, extra)
	require.Nil(t, r)
	require.Nil(t, s)
	require.NotContains(t, extra, openAILongContextBillingEnabledKey)
	require.Equal(t, 10, extra["base_rpm"])

	r, s = &rate, &sync
	adminExtra := map[string]any{openAILongContextBillingEnabledKey: true}
	stripOwnerAccountBilling(context.Background(), &r, &s, adminExtra)
	require.Same(t, &rate, r)
	require.Same(t, &sync, s)
	require.Contains(t, adminExtra, openAILongContextBillingEnabledKey)

	r = &rate
	stripOwnerAccountBilling(owner, &r, nil, nil)
	require.Nil(t, r)
}
