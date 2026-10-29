package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	authLoginFailureKeyPrefix   = "auth:login_fail:"
	authTokensRevokedAtPrefix   = "auth:tokens_revoked_at:"
	authRevokedSessionKeyPrefix = "auth:revoked_sid:"
)

// loginFailureIncrScript 固定窗口失败计数：首次失败设置窗口过期，达到阈值时改为锁定期过期；
// 同时修复异常的无过期 key（与限流脚本一致）。
var loginFailureIncrScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current >= tonumber(ARGV[2]) then
  redis.call('PEXPIRE', KEYS[1], ARGV[3])
elseif current == 1 or redis.call('PTTL', KEYS[1]) == -1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`)

type authGuardCache struct {
	rdb *redis.Client
}

// NewAuthGuardCache creates the Redis-backed login failure / access token revocation cache.
func NewAuthGuardCache(rdb *redis.Client) service.AuthGuardCache {
	return &authGuardCache{rdb: rdb}
}

func (c *authGuardCache) GetLoginFailures(ctx context.Context, key string) (int, error) {
	n, err := c.rdb.Get(ctx, authLoginFailureKeyPrefix+key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get login failures: %w", err)
	}
	return n, nil
}

func (c *authGuardCache) IncrLoginFailures(ctx context.Context, key string, window time.Duration, limit int, lockTTL time.Duration) (int, error) {
	n, err := loginFailureIncrScript.Run(ctx, c.rdb, []string{authLoginFailureKeyPrefix + key},
		window.Milliseconds(), limit, lockTTL.Milliseconds()).Int()
	if err != nil {
		return 0, fmt.Errorf("incr login failures: %w", err)
	}
	return n, nil
}

func (c *authGuardCache) ClearLoginFailures(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, authLoginFailureKeyPrefix+key).Err()
}

func (c *authGuardCache) SetUserTokensRevokedAt(ctx context.Context, userID int64, revokedAt int64, ttl time.Duration) error {
	key := authTokensRevokedAtPrefix + strconv.FormatInt(userID, 10)
	return c.rdb.Set(ctx, key, revokedAt, ttl).Err()
}

func (c *authGuardCache) RevokeAccessSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	if sessionID == "" {
		return nil
	}
	return c.rdb.Set(ctx, authRevokedSessionKeyPrefix+sessionID, "1", ttl).Err()
}

// GetAccessTokenRevocation 用一次 MGET 同时读取用户级撤销时间与会话撤销标记。
func (c *authGuardCache) GetAccessTokenRevocation(ctx context.Context, userID int64, sessionID string) (int64, bool, error) {
	keys := []string{authTokensRevokedAtPrefix + strconv.FormatInt(userID, 10)}
	if sessionID != "" {
		keys = append(keys, authRevokedSessionKeyPrefix+sessionID)
	}
	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return 0, false, fmt.Errorf("get access token revocation: %w", err)
	}
	var revokedAt int64
	if raw, ok := values[0].(string); ok {
		revokedAt, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("parse tokens revoked at: %w", err)
		}
	}
	sessionRevoked := len(values) > 1 && values[1] != nil
	return revokedAt, sessionRevoked, nil
}
