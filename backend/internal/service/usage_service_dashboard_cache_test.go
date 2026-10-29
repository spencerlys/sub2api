//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type dashboardStatsCacheRepoStub struct {
	UsageLogRepository

	userCalls atomic.Int64
	keyCalls  atomic.Int64
	release   chan struct{}
	err       error
}

func (r *dashboardStatsCacheRepoStub) GetUserDashboardStats(_ context.Context, userID int64) (*usagestats.UserDashboardStats, error) {
	n := r.userCalls.Add(1)
	if r.release != nil {
		<-r.release
	}
	if r.err != nil {
		return nil, r.err
	}
	return &usagestats.UserDashboardStats{
		TotalRequests: userID*1000 + n,
		ByPlatform:    []usagestats.PlatformDashboardStats{{Platform: "anthropic", TotalRequests: n}},
	}, nil
}

func (r *dashboardStatsCacheRepoStub) GetAPIKeyDashboardStats(_ context.Context, apiKeyID int64) (*usagestats.UserDashboardStats, error) {
	n := r.keyCalls.Add(1)
	if r.err != nil {
		return nil, r.err
	}
	return &usagestats.UserDashboardStats{TotalRequests: apiKeyID*1000 + n}, nil
}

func TestUsageServiceDashboardStatsCacheHitSkipsRepository(t *testing.T) {
	repo := &dashboardStatsCacheRepoStub{}
	svc := NewUsageService(repo, nil, nil, nil)
	ctx := context.Background()

	first, err := svc.GetAPIKeyDashboardStats(ctx, 7)
	require.NoError(t, err)
	second, err := svc.GetAPIKeyDashboardStats(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(1), repo.keyCalls.Load())
	require.Equal(t, first.TotalRequests, second.TotalRequests)

	userFirst, err := svc.GetUserDashboardStats(ctx, 3)
	require.NoError(t, err)
	// 调用方修改返回值不能污染缓存。
	userFirst.TotalRequests = -1
	userFirst.ByPlatform[0].Platform = "mutated"
	userSecond, err := svc.GetUserDashboardStats(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, int64(1), repo.userCalls.Load())
	require.Equal(t, int64(3001), userSecond.TotalRequests)
	require.Equal(t, "anthropic", userSecond.ByPlatform[0].Platform)
}

func TestUsageServiceDashboardStatsCacheExpiresAndRefreshes(t *testing.T) {
	repo := &dashboardStatsCacheRepoStub{}
	svc := NewUsageService(repo, nil, nil, nil)
	svc.dashboardStatsTTL = 20 * time.Millisecond
	ctx := context.Background()

	first, err := svc.GetUserDashboardStats(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1001), first.TotalRequests)

	require.Eventually(t, func() bool {
		stats, err := svc.GetUserDashboardStats(ctx, 1)
		return err == nil && stats.TotalRequests == 1002
	}, time.Second, 5*time.Millisecond)
	require.Equal(t, int64(2), repo.userCalls.Load())
}

func TestUsageServiceDashboardStatsCacheKeepsKeysAndUsersApart(t *testing.T) {
	repo := &dashboardStatsCacheRepoStub{}
	svc := NewUsageService(repo, nil, nil, nil)
	ctx := context.Background()

	keyA, err := svc.GetAPIKeyDashboardStats(ctx, 1)
	require.NoError(t, err)
	keyB, err := svc.GetAPIKeyDashboardStats(ctx, 2)
	require.NoError(t, err)
	user1, err := svc.GetUserDashboardStats(ctx, 1)
	require.NoError(t, err)
	user2, err := svc.GetUserDashboardStats(ctx, 2)
	require.NoError(t, err)

	// API Key 1 与用户 1 的 ID 相同，也不能共用一个缓存条目。
	require.Equal(t, int64(1001), keyA.TotalRequests)
	require.Equal(t, int64(2002), keyB.TotalRequests)
	require.Equal(t, int64(1001), user1.TotalRequests)
	require.Equal(t, int64(2002), user2.TotalRequests)
	require.Equal(t, int64(2), repo.keyCalls.Load())
	require.Equal(t, int64(2), repo.userCalls.Load())
}

func TestUsageServiceDashboardStatsCacheDoesNotCacheErrors(t *testing.T) {
	repo := &dashboardStatsCacheRepoStub{err: errors.New("db down")}
	svc := NewUsageService(repo, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.GetAPIKeyDashboardStats(ctx, 9)
	require.ErrorContains(t, err, "db down")
	repo.err = nil
	stats, err := svc.GetAPIKeyDashboardStats(ctx, 9)
	require.NoError(t, err)
	require.Equal(t, int64(9002), stats.TotalRequests)
}

func TestUsageServiceDashboardStatsCacheCoalescesConcurrentMisses(t *testing.T) {
	repo := &dashboardStatsCacheRepoStub{release: make(chan struct{})}
	svc := NewUsageService(repo, nil, nil, nil)
	ctx := context.Background()

	const callers = 8
	var wg sync.WaitGroup
	results := make([]*usagestats.UserDashboardStats, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats, err := svc.GetUserDashboardStats(ctx, 5)
			if err == nil {
				results[i] = stats
			}
		}()
	}
	require.Eventually(t, func() bool { return repo.userCalls.Load() >= 1 }, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	close(repo.release)
	wg.Wait()

	require.Equal(t, int64(1), repo.userCalls.Load())
	for _, stats := range results {
		require.NotNil(t, stats)
		require.Equal(t, int64(5001), stats.TotalRequests)
	}
}
