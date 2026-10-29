//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 选中账号后 hydrate 失败：已获取的并发槽与本次注册的会话槽都必须释放，否则会泄漏。
func TestNewSelectionResult_HydrateErrorReleasesSlotAndSession(t *testing.T) {
	cache := newSessionLimitReleaseCacheStub()
	svc := &GatewayService{
		sessionLimitCache: cache,
		schedulerSnapshot: &SchedulerSnapshotService{},
	}
	acc := newSessionLimitTestAccount()

	ctx, cancel := context.WithCancel(withSelectionSessionHash(context.Background(), "session-hash-1"))
	cancel() // GetAccount 在已取消的 ctx 上返回错误，模拟 hydrate 失败

	released := 0
	selection, err := svc.newSelectionResult(ctx, acc, true, func() { released++ }, nil)

	require.Error(t, err)
	require.Nil(t, selection)
	require.Equal(t, 1, released, "hydrate 失败必须释放已获取的槽位")
	require.Equal(t, []string{"session-hash-1"}, cache.unregistered[acc.ID], "hydrate 失败必须释放会话注册")
}
