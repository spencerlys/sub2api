//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/stretchr/testify/require"
)

func ownerGuardContext(ownerID int64) context.Context {
	return adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{OwnerUserID: ownerID, Policy: adminaccess.OwnerPolicy()})
}

// ownerGuardAccountRepo 在计费原子更新桩之上补充归属批量查询与影子查询。
type ownerGuardAccountRepo struct {
	*accountBillingSettingsAdminRepo
	owners map[string]map[int64]*int64
}

func (r *ownerGuardAccountRepo) ResourceOwnerIDs(_ context.Context, kind string, ids []int64) (map[int64]*int64, error) {
	out := map[int64]*int64{}
	for _, id := range ids {
		if owner, ok := r.owners[kind][id]; ok {
			out[id] = owner
		}
	}
	return out, nil
}

func (r *ownerGuardAccountRepo) ListShadowsByParent(context.Context, int64) ([]*Account, error) {
	return nil, nil
}

// 7 号所有者拥有分组 1、代理 1；分组/代理 2 属于其他所有者，3 为管理员资源。
func newOwnerGuardService(accounts map[int64]*Account) (*adminServiceImpl, *ownerGuardAccountRepo) {
	repo := &ownerGuardAccountRepo{
		accountBillingSettingsAdminRepo: &accountBillingSettingsAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: accounts}},
		owners: map[string]map[int64]*int64{
			OwnerResourceGroup: {1: int64Ptr(7), 2: int64Ptr(8), 3: nil},
			OwnerResourceProxy: {1: int64Ptr(7), 2: int64Ptr(8), 3: nil},
		},
	}
	return &adminServiceImpl{accountRepo: repo}, repo
}

func TestEnsureOwnerReferences(t *testing.T) {
	svc, _ := newOwnerGuardService(nil)
	owner := ownerGuardContext(7)
	require.NoError(t, svc.ensureOwnerReferences(owner, OwnerResourceGroup, []int64{1, 1, 0}))
	require.ErrorIs(t, svc.ensureOwnerReferences(owner, OwnerResourceGroup, []int64{1, 3}), ErrOwnerResourceNotFound)
	require.ErrorIs(t, svc.ensureOwnerReferences(owner, OwnerResourceGroup, []int64{2}), ErrOwnerResourceNotFound)
	require.ErrorIs(t, svc.ensureOwnerReferences(owner, OwnerResourceProxy, []int64{404}), ErrOwnerResourceNotFound)
	// 已绑定且保持不变的管理员资源放行，新增的仍拒绝。
	require.NoError(t, svc.ensureOwnerReferences(owner, OwnerResourceGroup, []int64{1, 3}, 3))
	require.ErrorIs(t, svc.ensureOwnerReferences(owner, OwnerResourceGroup, []int64{2, 3}, 3), ErrOwnerResourceNotFound)
	// 管理员不受限制；仓储缺少批量归属能力时所有者请求失败关闭。
	require.NoError(t, svc.ensureOwnerReferences(context.Background(), OwnerResourceGroup, []int64{2, 3}))
	bare := &adminServiceImpl{accountRepo: &upstreamBillingProbeAccountRepo{}}
	require.ErrorIs(t, bare.ensureOwnerReferences(owner, OwnerResourceGroup, []int64{1}), ErrOwnerResourceLookup)
}

func TestCheckOwnerOutboundHost(t *testing.T) {
	owner := ownerGuardContext(7)
	for _, host := range []string{
		"127.0.0.1", "localhost", "api.localhost", "10.1.2.3", "172.18.0.5", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "::1", "[::1]", "fd00::1", "fe80::1", "::ffff:127.0.0.1",
		"metadata.google.internal", "postgres.invalid", "",
	} {
		require.ErrorIs(t, checkOwnerOutboundHost(owner, host), ErrOwnerOutboundHostForbidden, host)
	}
	require.NoError(t, checkOwnerOutboundHost(owner, "203.0.113.10"))
	require.NoError(t, checkOwnerOutboundHost(context.Background(), "127.0.0.1"))
}

func TestCreateAccountOwnerGuards(t *testing.T) {
	svc, _ := newOwnerGuardService(nil)
	owner := ownerGuardContext(7)
	base := func() *CreateAccountInput {
		return &CreateAccountInput{
			Name: "a", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, SkipDefaultGroupBind: true,
			Credentials: map[string]any{"api_key": "sk", "base_url": "https://203.0.113.10/v1"},
		}
	}
	in := base()
	in.GroupIDs = []int64{3}
	_, err := svc.CreateAccount(owner, in)
	require.ErrorIs(t, err, ErrOwnerResourceNotFound)

	in = base()
	in.ProxyID = int64Ptr(2)
	_, err = svc.CreateAccount(owner, in)
	require.ErrorIs(t, err, ErrOwnerResourceNotFound)

	for _, mutate := range []func(*CreateAccountInput){
		func(in *CreateAccountInput) { in.Credentials["base_url"] = "http://postgres.invalid:5432" },
		func(in *CreateAccountInput) { in.Credentials["base_url"] = "http://169.254.169.254/latest" },
		func(in *CreateAccountInput) {
			in.Credentials["api_base_urls"] = map[string]any{APIProtocolAnthropic: "http://10.0.0.8:6379"}
		},
		func(in *CreateAccountInput) { in.Extra = map[string]any{"custom_base_url": "http://127.0.0.1:8080"} },
	} {
		in = base()
		mutate(in)
		_, err = svc.CreateAccount(owner, in)
		require.ErrorIs(t, err, ErrOwnerOutboundHostForbidden)
	}

	// 所有者不能自行开启上游计费探测；新建账户保持默认关闭。
	enabled := true
	in = base()
	in.ProbeEnabled = &enabled
	in.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: true}
	created, err := svc.CreateAccount(owner, in)
	require.NoError(t, err)
	require.NotContains(t, created.Extra, UpstreamBillingProbeEnabledExtraKey)

	// 管理员不受这些限制。
	in = base()
	in.Credentials["base_url"] = "http://127.0.0.1:8080"
	in.ProbeEnabled = &enabled
	created, err = svc.CreateAccount(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, true, created.Extra[UpstreamBillingProbeEnabledExtraKey])
}

func TestUpdateAccountOwnerGuards(t *testing.T) {
	const accountID = int64(51)
	newAccount := func() *Account {
		return &Account{
			ID: accountID, Name: "a", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			OwnerUserID: int64Ptr(7), GroupIDs: []int64{1, 3}, ProxyID: int64Ptr(3),
			Credentials: map[string]any{"api_key": "sk", "base_url": "https://203.0.113.10/v1"},
			Extra:       map[string]any{UpstreamBillingProbeEnabledExtraKey: true, UpstreamBillingRateSyncEnabledExtraKey: true},
		}
	}
	owner := ownerGuardContext(7)

	svc, _ := newOwnerGuardService(map[int64]*Account{accountID: newAccount()})
	_, err := svc.UpdateAccount(owner, accountID, &UpdateAccountInput{GroupIDs: &[]int64{1, 3, 2}})
	require.ErrorIs(t, err, ErrOwnerResourceNotFound)
	_, err = svc.UpdateAccount(owner, accountID, &UpdateAccountInput{ProxyID: int64Ptr(2)})
	require.ErrorIs(t, err, ErrOwnerResourceNotFound)
	_, err = svc.UpdateAccount(owner, accountID, &UpdateAccountInput{Credentials: map[string]any{"base_url": "http://127.0.0.1:5432"}})
	require.ErrorIs(t, err, ErrOwnerOutboundHostForbidden)

	// 保持管理员绑定的代理不变、改成本人代理或清除代理均可。
	for _, proxyID := range []int64{3, 1, 0} {
		svc, _ = newOwnerGuardService(map[int64]*Account{accountID: newAccount()})
		_, err = svc.UpdateAccount(owner, accountID, &UpdateAccountInput{ProxyID: int64Ptr(proxyID)})
		require.NoError(t, err, proxyID)
	}

	// 所有者既不能关闭管理员开启的探测/同步，也不能借改类型清掉它们。
	svc, repo := newOwnerGuardService(map[int64]*Account{accountID: newAccount()})
	disabled := false
	_, err = svc.UpdateAccount(owner, accountID, &UpdateAccountInput{
		Name: "renamed", ProbeEnabled: &disabled, RateSyncEnabled: &disabled,
		Extra: map[string]any{UpstreamBillingProbeEnabledExtraKey: false, UpstreamBillingRateSyncEnabledExtraKey: false},
	})
	require.NoError(t, err)
	stored := repo.accounts[accountID]
	require.Equal(t, "renamed", stored.Name)
	require.Equal(t, true, stored.Extra[UpstreamBillingProbeEnabledExtraKey])
	require.Equal(t, true, stored.Extra[UpstreamBillingRateSyncEnabledExtraKey])
	_, err = svc.UpdateAccount(owner, accountID, &UpdateAccountInput{Type: AccountTypeOAuth})
	require.ErrorIs(t, err, ErrOwnerBillingProbeManaged)

	// 管理员仍可关闭探测。
	_, err = svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{ProbeEnabled: &disabled})
	require.NoError(t, err)
	require.Equal(t, false, repo.accounts[accountID].Extra[UpstreamBillingProbeEnabledExtraKey])
}

// creatingProxyRepoStub 记录创建的代理，供所有者代理校验测试使用。
type creatingProxyRepoStub struct {
	*proxyRepoStub
	created []*Proxy
}

func (s *creatingProxyRepoStub) Create(_ context.Context, proxy *Proxy) error {
	s.created = append(s.created, proxy)
	return nil
}

func TestOwnerProxyGuards(t *testing.T) {
	owner := ownerGuardContext(7)
	accounts, _ := newOwnerGuardService(nil)
	creating := &creatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}}
	svc := &adminServiceImpl{accountRepo: accounts.accountRepo, proxyRepo: creating}

	for _, host := range []string{"127.0.0.1", "redis.invalid", "169.254.169.254", "10.0.0.2"} {
		_, err := svc.CreateProxy(owner, &CreateProxyInput{Name: "p", Protocol: "http", Host: host, Port: 8080})
		require.ErrorIs(t, err, ErrOwnerOutboundHostForbidden, host)
	}
	_, err := svc.CreateProxy(owner, &CreateProxyInput{Name: "p", Protocol: "http", Host: "203.0.113.10", Port: 8080, FallbackMode: FallbackModeProxy, BackupProxyID: int64Ptr(3)})
	require.ErrorIs(t, err, ErrOwnerResourceNotFound)
	_, err = svc.CreateProxy(owner, &CreateProxyInput{Name: "p", Protocol: "http", Host: "203.0.113.10", Port: 8080})
	require.NoError(t, err)
	_, err = svc.CreateProxy(context.Background(), &CreateProxyInput{Name: "p", Protocol: "http", Host: "10.0.0.2", Port: 8080})
	require.NoError(t, err)
	require.Len(t, creating.created, 2)

	// 更新：管理员设置的备用代理保持不变放行，新指定他人代理拒绝；改到内网地址拒绝。
	updating := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: &Proxy{ID: 1, Host: "203.0.113.10", Port: 8080, FallbackMode: FallbackModeProxy, BackupProxyID: int64Ptr(3)}}
	svc.proxyRepo = updating
	_, err = svc.UpdateProxy(owner, 1, &UpdateProxyInput{Name: "renamed", BackupProxyID: int64Ptr(3)})
	require.NoError(t, err)
	_, err = svc.UpdateProxy(owner, 1, &UpdateProxyInput{BackupProxyID: int64Ptr(2)})
	require.ErrorIs(t, err, ErrOwnerResourceNotFound)
	_, err = svc.UpdateProxy(owner, 1, &UpdateProxyInput{Host: "127.0.0.1"})
	require.ErrorIs(t, err, ErrOwnerOutboundHostForbidden)
	require.Equal(t, 1, updating.updateCalls)

	// 测试与质量检测在探测前复查（规则生效前保存的内网代理）。
	updating.proxy.Host = "10.0.0.9"
	_, err = svc.TestProxy(owner, 1)
	require.ErrorIs(t, err, ErrOwnerOutboundHostForbidden)
	_, err = svc.CheckProxyQuality(owner, 1)
	require.ErrorIs(t, err, ErrOwnerOutboundHostForbidden)
}
