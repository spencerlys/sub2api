//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 粘性账号只是忙（槽位满 + 等待队列满）时，负载感知层选中别的账号不得覆盖原绑定，
// 否则会话下一轮会离开持有 prompt cache 的账号。
func TestSelectAccountWithLoadAwareness_BusyStickyKeepsBinding(t *testing.T) {
	t.Parallel()

	f := newLoadAwareRestrictionFixture(t, false, map[string]int64{"sticky": 1}, nil)
	f.concurrencyCache.acquireResults = map[int64]bool{1: false}
	f.concurrencyCache.waitCounts = map[int64]int{1: 1000}

	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "sticky", "claude-fable-5-1", nil, "", 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, int64(2), result.Account.ID)
	require.Equal(t, int64(1), f.cache.sessionBindings["sticky"], "忙碌的粘性账号绑定必须保留")
}

// 粘性账号被排除（failover）时，负载感知层应改绑到新账号。
func TestSelectAccountWithLoadAwareness_ExcludedStickyRebinds(t *testing.T) {
	t.Parallel()

	f := newLoadAwareRestrictionFixture(t, false, map[string]int64{"sticky": 1}, nil)

	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "sticky", "claude-fable-5-1", map[int64]struct{}{1: {}}, "", 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Account.ID)
	require.Equal(t, int64(2), f.cache.sessionBindings["sticky"])
}

// 无粘性绑定时，负载感知层照常创建绑定。
func TestSelectAccountWithLoadAwareness_NoStickyBinds(t *testing.T) {
	t.Parallel()

	f := newLoadAwareRestrictionFixture(t, false, nil, nil)

	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "fresh", "claude-fable-5-1", nil, "", 0)
	require.NoError(t, err)
	require.Equal(t, result.Account.ID, f.cache.sessionBindings["fresh"])
}

// 模型路由层：粘性账号在路由列表中但忙碌时，同样不得被路由选中的账号覆盖。
func TestSelectAccountWithLoadAwareness_RoutedBusyStickyKeepsBinding(t *testing.T) {
	t.Parallel()

	f := newLoadAwareRestrictionFixture(t, false, map[string]int64{"sticky": 1}, map[string][]int64{"claude-fable-5-1": {1, 2}})
	f.concurrencyCache.acquireResults = map[int64]bool{1: false}
	f.concurrencyCache.waitCounts = map[int64]int{1: 1000}

	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "sticky", "claude-fable-5-1", nil, "", 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, int64(2), result.Account.ID)
	require.Equal(t, int64(1), f.cache.sessionBindings["sticky"])
}
