package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"golang.org/x/crypto/bcrypt"
)

// ErrLoginTooManyAttempts 同一邮箱 + 客户端 IP 登录失败次数过多（对存在与不存在的邮箱返回相同响应）。
var ErrLoginTooManyAttempts = infraerrors.TooManyRequests("LOGIN_TOO_MANY_ATTEMPTS", "too many failed login attempts, please try again later")

const (
	// loginFailureLimit 窗口内允许的密码错误次数，达到后锁定 loginLockDuration。
	loginFailureLimit  = 10
	loginFailureWindow = 15 * time.Minute
	loginLockDuration  = 15 * time.Minute
)

// AuthGuardCache 认证防护所需的 Redis 状态：账号维度登录失败计数，以及 access token 撤销标记。
type AuthGuardCache interface {
	// GetLoginFailures 返回当前失败计数（不存在为 0）。
	GetLoginFailures(ctx context.Context, key string) (int, error)
	// IncrLoginFailures 原子递增失败计数：首次失败设置 window 过期；计数达到 limit 时把过期时间重置为 lockTTL（锁定期）。
	IncrLoginFailures(ctx context.Context, key string, window time.Duration, limit int, lockTTL time.Duration) (int, error)
	// ClearLoginFailures 登录成功后清零。
	ClearLoginFailures(ctx context.Context, key string) error

	// SetUserTokensRevokedAt 记录"该用户在 revokedAt（unix 秒）及之前签发的 access token 全部失效"。
	SetUserTokensRevokedAt(ctx context.Context, userID int64, revokedAt int64, ttl time.Duration) error
	// RevokeAccessSession 撤销单个会话（JWT sid）的 access token。
	RevokeAccessSession(ctx context.Context, sessionID string, ttl time.Duration) error
	// GetAccessTokenRevocation 单次往返读取用户级撤销时间（0 表示无）与会话是否已撤销。
	GetAccessTokenRevocation(ctx context.Context, userID int64, sessionID string) (revokedAt int64, sessionRevoked bool, err error)
}

// SetAuthGuardCache 注入登录失败锁定与 access token 撤销所需的缓存；为 nil 时两项防护均不生效。
func (s *AuthService) SetAuthGuardCache(cache AuthGuardCache) {
	s.authGuardCache = cache
}

// loginFailureKey 按"规范化邮箱 + 客户端 IP"计数（哈希后作为 key，避免在 Redis 中存放原始邮箱）。
// 计入 IP 是为了让他人无法用错误密码把已知邮箱（如管理员）锁在面板外；跨 IP 的分布式猜测
// 由按 IP 限流与 TOTP 兜底。IP 取自 SessionBindingContext 注入的安全客户端 IP，
// 与登录限流、审计日志同源；缺失时（非 HTTP 调用）退化为仅按邮箱计数。
func loginFailureKey(ctx context.Context, email string) string {
	clientIP := ""
	if binding := SessionBindingFromContext(ctx); binding != nil {
		clientIP = strings.TrimSpace(binding.IP)
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email)) + "\n" + clientIP))
	return hex.EncodeToString(sum[:])
}

var (
	dummyPasswordHashOnce sync.Once
	dummyPasswordHash     []byte
)

// dummyPasswordCheck 在用户不存在时执行一次等价成本的 bcrypt 比较，使响应时间不暴露邮箱是否存在。
var dummyPasswordCheck = func(password string) {
	dummyPasswordHashOnce.Do(func() {
		dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("sub2api-dummy-password"), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
}

// checkLoginLocked 账号处于锁定期时拒绝登录（不区分账号是否存在）。Redis 故障时 fail-close，与登录接口限流一致。
func (s *AuthService) checkLoginLocked(ctx context.Context, key string) error {
	if s.authGuardCache == nil {
		return nil
	}
	failures, err := s.authGuardCache.GetLoginFailures(ctx, key)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Login failure counter unavailable: %v", err)
		return ErrServiceUnavailable
	}
	if failures >= loginFailureLimit {
		return ErrLoginTooManyAttempts
	}
	return nil
}

// recordLoginFailure 记录一次失败并返回应响应的错误：达到阈值返回锁定错误，否则返回凭证错误。
func (s *AuthService) recordLoginFailure(ctx context.Context, key string) error {
	if s.authGuardCache == nil {
		return ErrInvalidCredentials
	}
	failures, err := s.authGuardCache.IncrLoginFailures(ctx, key, loginFailureWindow, loginFailureLimit, loginLockDuration)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to record login failure: %v", err)
		return ErrServiceUnavailable
	}
	if failures >= loginFailureLimit {
		return ErrLoginTooManyAttempts
	}
	return ErrInvalidCredentials
}

func (s *AuthService) clearLoginFailures(ctx context.Context, key string) {
	if s.authGuardCache == nil {
		return
	}
	if err := s.authGuardCache.ClearLoginFailures(ctx, key); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to clear login failures: %v", err)
	}
}

// accessTokenRevocationTTL 撤销标记只需存活到最长 access token 过期为止（另加 1 分钟时钟余量）。
func (s *AuthService) accessTokenRevocationTTL() time.Duration {
	ttl := time.Duration(s.GetAccessTokenExpiresIn()) * time.Second
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return ttl + time.Minute
}

// CheckAccessTokenRevoked 校验 access token 是否已被"撤销所有会话"或单会话登出作废。
// 每次请求最多一次 Redis 往返；Redis 故障时 fail-close（返回 ErrServiceUnavailable），
// 与认证接口限流、refresh token 校验的既有策略一致。
func (s *AuthService) CheckAccessTokenRevoked(ctx context.Context, claims *JWTClaims) error {
	if s == nil || s.authGuardCache == nil || claims == nil {
		return nil
	}
	revokedAt, sessionRevoked, err := s.authGuardCache.GetAccessTokenRevocation(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Access token revocation lookup failed: %v", err)
		return ErrServiceUnavailable
	}
	if sessionRevoked {
		return ErrTokenRevoked
	}
	// iat 精度为秒：与撤销同一秒签发的 token 也视为已撤销（宁可多踢，不可漏放）。
	if revokedAt > 0 && (claims.IssuedAt == nil || claims.IssuedAt.Unix() <= revokedAt) {
		return ErrTokenRevoked
	}
	return nil
}

// revokeAllAccessTokens 写入用户级撤销标记，使此前签发的全部 access token 立即失效。
func (s *AuthService) revokeAllAccessTokens(ctx context.Context, userID int64) error {
	if s.authGuardCache == nil {
		return nil
	}
	return s.authGuardCache.SetUserTokensRevokedAt(ctx, userID, time.Now().Unix(), s.accessTokenRevocationTTL())
}

// RevokeLogoutSession 登出当前会话：删除 refresh token 所属会话家族，并撤销该会话（sid）的 access token。
// 会话由 refresh token 的家族 ID 或经验签的 access token 的 sid 确定；两者都缺失时无操作。
func (s *AuthService) RevokeLogoutSession(ctx context.Context, refreshToken, accessToken string) error {
	sessionIDs := make([]string, 0, 2)
	var errs []error

	if refreshToken != "" && s.refreshTokenCache != nil && strings.HasPrefix(refreshToken, refreshTokenPrefix) {
		tokenHash := hashToken(refreshToken)
		data, err := s.refreshTokenCache.GetRefreshToken(ctx, tokenHash)
		switch {
		case err == nil && data != nil && data.FamilyID != "":
			sessionIDs = append(sessionIDs, data.FamilyID)
			if err := s.refreshTokenCache.DeleteTokenFamily(ctx, data.FamilyID); err != nil {
				errs = append(errs, err)
			}
		case err != nil && !errors.Is(err, ErrRefreshTokenNotFound):
			errs = append(errs, err)
		}
		if err := s.refreshTokenCache.DeleteRefreshToken(ctx, tokenHash); err != nil {
			errs = append(errs, err)
		}
	}

	// 已过期或验签失败的 access token 无需（也不应）撤销。
	if accessToken != "" {
		if claims, err := s.ValidateToken(accessToken); err == nil && claims.SessionID != "" {
			sessionIDs = append(sessionIDs, claims.SessionID)
		}
	}

	if s.authGuardCache != nil {
		ttl := s.accessTokenRevocationTTL()
		seen := make(map[string]struct{}, len(sessionIDs))
		for _, sid := range sessionIDs {
			if _, ok := seen[sid]; ok {
				continue
			}
			seen[sid] = struct{}{}
			if err := s.authGuardCache.RevokeAccessSession(ctx, sid, ttl); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
