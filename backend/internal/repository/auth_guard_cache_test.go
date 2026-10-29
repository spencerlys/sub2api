//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newAuthGuardMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestAuthGuardCache_LoginFailuresWindowThenLock(t *testing.T) {
	mr, rdb := newAuthGuardMiniredis(t)
	cache := NewAuthGuardCache(rdb)
	ctx := context.Background()
	const key = "k"

	n, err := cache.IncrLoginFailures(ctx, key, time.Minute, 3, time.Hour)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, time.Minute, mr.TTL(authLoginFailureKeyPrefix+key))

	_, err = cache.IncrLoginFailures(ctx, key, time.Minute, 3, time.Hour)
	require.NoError(t, err)
	// 窗口为固定窗口：后续失败不延长过期时间
	require.Equal(t, time.Minute, mr.TTL(authLoginFailureKeyPrefix+key))

	n, err = cache.IncrLoginFailures(ctx, key, time.Minute, 3, time.Hour)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, time.Hour, mr.TTL(authLoginFailureKeyPrefix+key), "reaching the limit arms the lock TTL")

	got, err := cache.GetLoginFailures(ctx, key)
	require.NoError(t, err)
	require.Equal(t, 3, got)

	mr.FastForward(time.Hour + time.Second)
	got, err = cache.GetLoginFailures(ctx, key)
	require.NoError(t, err)
	require.Zero(t, got)

	_, err = cache.IncrLoginFailures(ctx, key, time.Minute, 3, time.Hour)
	require.NoError(t, err)
	require.NoError(t, cache.ClearLoginFailures(ctx, key))
	got, err = cache.GetLoginFailures(ctx, key)
	require.NoError(t, err)
	require.Zero(t, got)
}

func TestAuthGuardCache_AccessTokenRevocation(t *testing.T) {
	mr, rdb := newAuthGuardMiniredis(t)
	cache := NewAuthGuardCache(rdb)
	ctx := context.Background()

	revokedAt, sessionRevoked, err := cache.GetAccessTokenRevocation(ctx, 1, "sid-a")
	require.NoError(t, err)
	require.Zero(t, revokedAt)
	require.False(t, sessionRevoked)

	require.NoError(t, cache.SetUserTokensRevokedAt(ctx, 1, 12345, time.Hour))
	require.NoError(t, cache.RevokeAccessSession(ctx, "sid-a", 2*time.Hour))

	revokedAt, sessionRevoked, err = cache.GetAccessTokenRevocation(ctx, 1, "sid-a")
	require.NoError(t, err)
	require.Equal(t, int64(12345), revokedAt)
	require.True(t, sessionRevoked)

	revokedAt, sessionRevoked, err = cache.GetAccessTokenRevocation(ctx, 2, "sid-b")
	require.NoError(t, err)
	require.Zero(t, revokedAt)
	require.False(t, sessionRevoked)

	revokedAt, sessionRevoked, err = cache.GetAccessTokenRevocation(ctx, 1, "")
	require.NoError(t, err)
	require.Equal(t, int64(12345), revokedAt)
	require.False(t, sessionRevoked)

	mr.FastForward(time.Hour + time.Second)
	revokedAt, sessionRevoked, err = cache.GetAccessTokenRevocation(ctx, 1, "sid-a")
	require.NoError(t, err)
	require.Zero(t, revokedAt, "user marker expires with the max access token lifetime")
	require.True(t, sessionRevoked)
}

func TestTotpCache_MarkCodeUsedOnlyOnce(t *testing.T) {
	mr, rdb := newAuthGuardMiniredis(t)
	cache := NewTotpCache(rdb)
	ctx := context.Background()

	first, err := cache.MarkCodeUsed(ctx, 1, "123456", 90*time.Second)
	require.NoError(t, err)
	require.True(t, first)

	again, err := cache.MarkCodeUsed(ctx, 1, "123456", 90*time.Second)
	require.NoError(t, err)
	require.False(t, again)

	otherUser, err := cache.MarkCodeUsed(ctx, 2, "123456", 90*time.Second)
	require.NoError(t, err)
	require.True(t, otherUser)

	mr.FastForward(91 * time.Second)
	afterExpiry, err := cache.MarkCodeUsed(ctx, 1, "123456", 90*time.Second)
	require.NoError(t, err)
	require.True(t, afterExpiry)
}
