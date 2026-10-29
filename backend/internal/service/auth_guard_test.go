//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// authGuardCacheStub 内存版 AuthGuardCache，语义与 Redis 实现一致：
// 失败计数达到阈值即进入锁定（锁定期内不再递增），撤销标记按 key 存取。
type authGuardCacheStub struct {
	mu              sync.Mutex
	failures        map[string]int
	revokedAt       map[int64]int64
	revokedSessions map[string]bool
	err             error
}

func newAuthGuardCacheStub() *authGuardCacheStub {
	return &authGuardCacheStub{
		failures:        map[string]int{},
		revokedAt:       map[int64]int64{},
		revokedSessions: map[string]bool{},
	}
}

func (s *authGuardCacheStub) GetLoginFailures(_ context.Context, key string) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failures[key], nil
}

func (s *authGuardCacheStub) IncrLoginFailures(_ context.Context, key string, _ time.Duration, _ int, _ time.Duration) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[key]++
	return s.failures[key], nil
}

func (s *authGuardCacheStub) ClearLoginFailures(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, key)
	return s.err
}

func (s *authGuardCacheStub) SetUserTokensRevokedAt(_ context.Context, userID int64, revokedAt int64, _ time.Duration) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokedAt[userID] = revokedAt
	return nil
}

func (s *authGuardCacheStub) RevokeAccessSession(_ context.Context, sessionID string, _ time.Duration) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokedSessions[sessionID] = true
	return nil
}

func (s *authGuardCacheStub) GetAccessTokenRevocation(_ context.Context, userID int64, sessionID string) (int64, bool, error) {
	if s.err != nil {
		return 0, false, s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revokedAt[userID], sessionID != "" && s.revokedSessions[sessionID], nil
}

// authGuardRefreshCacheStub 仅支持登出路径所需的 refresh token 查询/删除。
type authGuardRefreshCacheStub struct {
	RefreshTokenCache
	tokens          map[string]*RefreshTokenData
	deletedFamilies []string
}

func (s *authGuardRefreshCacheStub) GetRefreshToken(_ context.Context, tokenHash string) (*RefreshTokenData, error) {
	if data, ok := s.tokens[tokenHash]; ok {
		return data, nil
	}
	return nil, ErrRefreshTokenNotFound
}

func (s *authGuardRefreshCacheStub) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(s.tokens, tokenHash)
	return nil
}

func (s *authGuardRefreshCacheStub) DeleteTokenFamily(_ context.Context, familyID string) error {
	s.deletedFamilies = append(s.deletedFamilies, familyID)
	return nil
}

func (s *authGuardRefreshCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}

const authGuardTestPassword = "correct-horse"

func newAuthGuardTestService(t *testing.T, users ...*User) (*AuthService, *authGuardCacheStub) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(authGuardTestPassword), bcrypt.MinCost)
	require.NoError(t, err)
	repo := &userRepoStub{usersByEmail: map[string]*User{}, usersByID: map[int64]*User{}}
	for _, u := range users {
		u.PasswordHash = string(hash)
		repo.usersByEmail[u.Email] = u
		repo.usersByID[u.ID] = u
	}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 24}}
	svc := NewAuthService(nil, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	guard := newAuthGuardCacheStub()
	svc.SetAuthGuardCache(guard)
	return svc, guard
}

func stubDummyPasswordCheck(t *testing.T) *int {
	t.Helper()
	calls := 0
	prev := dummyPasswordCheck
	dummyPasswordCheck = func(string) { calls++ }
	t.Cleanup(func() { dummyPasswordCheck = prev })
	return &calls
}

func TestLogin_LocksAccountAfterRepeatedFailures(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	ctx := context.Background()

	for i := 1; i < loginFailureLimit; i++ {
		_, _, err := svc.Login(ctx, user.Email, "wrong")
		require.ErrorIs(t, err, ErrInvalidCredentials, "attempt %d", i)
	}
	_, _, err := svc.Login(ctx, user.Email, "wrong")
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)

	// 锁定期内即使密码正确也拒绝
	_, _, err = svc.Login(ctx, user.Email, authGuardTestPassword)
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)
}

func TestLogin_LockForUnknownEmailIsIndistinguishable(t *testing.T) {
	stubDummyPasswordCheck(t)
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	ctx := context.Background()

	for i := 1; i <= loginFailureLimit+1; i++ {
		_, _, knownErr := svc.Login(ctx, user.Email, "wrong")
		_, _, unknownErr := svc.Login(ctx, "ghost@example.com", "wrong")
		require.Equal(t, knownErr.Error(), unknownErr.Error(), "attempt %d", i)
	}
}

func TestLogin_SuccessResetsFailureCounter(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, guard := newAuthGuardTestService(t, user)
	ctx := context.Background()

	for i := 1; i < loginFailureLimit; i++ {
		_, _, _ = svc.Login(ctx, user.Email, "wrong")
	}
	_, _, err := svc.Login(ctx, user.Email, authGuardTestPassword)
	require.NoError(t, err)
	require.Zero(t, guard.failures[loginFailureKey(ctx, user.Email)])

	_, _, err = svc.Login(ctx, user.Email, "wrong")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_FailureCounterUsesNormalizedEmail(t *testing.T) {
	stubDummyPasswordCheck(t)
	svc, _ := newAuthGuardTestService(t)
	ctx := context.Background()

	for i := 1; i < loginFailureLimit; i++ {
		_, _, _ = svc.Login(ctx, "Victim@Example.com", "wrong")
	}
	_, _, err := svc.Login(ctx, "  victim@example.COM ", "wrong")
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)
}

func TestLogin_UnknownEmailRunsDummyPasswordCheck(t *testing.T) {
	calls := stubDummyPasswordCheck(t)
	svc, _ := newAuthGuardTestService(t)

	_, _, err := svc.Login(context.Background(), "ghost@example.com", "whatever")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Equal(t, 1, *calls)
}

func TestLogin_GuardUnavailableFailsClosed(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, guard := newAuthGuardTestService(t, user)
	guard.err = errors.New("redis down")

	_, _, err := svc.Login(context.Background(), user.Email, authGuardTestPassword)
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestValidatePasswordCredentials_RespectsLoginLock(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	ctx := context.Background()

	for i := 1; i <= loginFailureLimit; i++ {
		_, _ = svc.ValidatePasswordCredentials(ctx, user.Email, "wrong")
	}
	_, err := svc.ValidatePasswordCredentials(ctx, user.Email, authGuardTestPassword)
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)
}

func parseTestClaims(t *testing.T, svc *AuthService, token string) *JWTClaims {
	t.Helper()
	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	return claims
}

func TestRevokeAllUserTokens_InvalidatesIssuedAccessTokens(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	other := &User{ID: 2, Email: "other@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user, other)
	ctx := context.Background()

	token, err := svc.GenerateToken(ctx, user)
	require.NoError(t, err)
	otherToken, err := svc.GenerateToken(ctx, other)
	require.NoError(t, err)
	claims := parseTestClaims(t, svc, token)
	require.NoError(t, svc.CheckAccessTokenRevoked(ctx, claims))

	require.NoError(t, svc.RevokeAllUserTokens(ctx, user.ID))

	require.ErrorIs(t, svc.CheckAccessTokenRevoked(ctx, claims), ErrTokenRevoked)
	require.NoError(t, svc.CheckAccessTokenRevoked(ctx, parseTestClaims(t, svc, otherToken)))
}

func TestCheckAccessTokenRevoked_TokenIssuedAfterRevokeAllIsValid(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, guard := newAuthGuardTestService(t, user)
	ctx := context.Background()
	guard.revokedAt[user.ID] = time.Now().Add(-10 * time.Second).Unix()

	claims := parseTestClaims(t, svc, mustGenerateToken(t, svc, user))
	require.NoError(t, svc.CheckAccessTokenRevoked(ctx, claims))

	// 缺少 iat 的 token 在存在撤销标记时一律视为已撤销
	claims.IssuedAt = nil
	require.ErrorIs(t, svc.CheckAccessTokenRevoked(ctx, claims), ErrTokenRevoked)
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	require.ErrorIs(t, svc.CheckAccessTokenRevoked(ctx, claims), ErrTokenRevoked)
}

func mustGenerateToken(t *testing.T, svc *AuthService, user *User) string {
	t.Helper()
	token, err := svc.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	return token
}

func TestRevokeLogoutSession_RevokesAccessTokenSession(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	ctx := context.Background()

	token := mustGenerateToken(t, svc, user)
	current := parseTestClaims(t, svc, token)
	otherDeviceToken := mustGenerateToken(t, svc, user)

	require.NoError(t, svc.RevokeLogoutSession(ctx, "", token))

	require.ErrorIs(t, svc.CheckAccessTokenRevoked(ctx, current), ErrTokenRevoked)
	require.NoError(t, svc.CheckAccessTokenRevoked(ctx, parseTestClaims(t, svc, otherDeviceToken)))
}

func TestRevokeLogoutSession_RevokesSessionOfRefreshToken(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	ctx := context.Background()

	refresh := &authGuardRefreshCacheStub{tokens: map[string]*RefreshTokenData{}}
	svc.refreshTokenCache = refresh
	const familyID = "family-1"
	refreshToken := refreshTokenPrefix + "abc"
	refresh.tokens[hashToken(refreshToken)] = &RefreshTokenData{UserID: user.ID, FamilyID: familyID}

	accessClaims := parseTestClaims(t, svc, mustGenerateToken(t, svc, user))
	accessClaims.SessionID = familyID

	require.NoError(t, svc.RevokeLogoutSession(ctx, refreshToken, ""))

	require.Empty(t, refresh.tokens)
	require.Equal(t, []string{familyID}, refresh.deletedFamilies)
	require.ErrorIs(t, svc.CheckAccessTokenRevoked(ctx, accessClaims), ErrTokenRevoked)
}

func TestRevokeLogoutSession_IgnoresForgedAccessToken(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, guard := newAuthGuardTestService(t, user)

	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &JWTClaims{UserID: 1, SessionID: "victim"}).SignedString([]byte("other-secret"))
	require.NoError(t, err)
	require.NoError(t, svc.RevokeLogoutSession(context.Background(), "", forged))
	require.Empty(t, guard.revokedSessions)
}

func TestCheckAccessTokenRevoked_FailsClosedWhenCacheUnavailable(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, guard := newAuthGuardTestService(t, user)
	claims := parseTestClaims(t, svc, mustGenerateToken(t, svc, user))
	guard.err = errors.New("redis down")

	require.ErrorIs(t, svc.CheckAccessTokenRevoked(context.Background(), claims), ErrServiceUnavailable)
	require.Error(t, svc.RevokeAllUserTokens(context.Background(), user.ID))
}

func TestCheckAccessTokenRevoked_NoGuardConfigured(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	svc.SetAuthGuardCache(nil)
	claims := parseTestClaims(t, svc, mustGenerateToken(t, svc, user))
	require.NoError(t, svc.CheckAccessTokenRevoked(context.Background(), claims))
}

func withClientIP(ip string) context.Context {
	return WithSessionBinding(context.Background(), &SessionBinding{IP: ip})
}

func TestLogin_LockIsScopedToClientIP(t *testing.T) {
	user := &User{ID: 1, Email: "admin@example.com", Status: StatusActive, Role: RoleAdmin}
	svc, _ := newAuthGuardTestService(t, user)
	attacker := withClientIP("203.0.113.9")
	owner := withClientIP("198.51.100.7")

	for i := 1; i <= loginFailureLimit; i++ {
		_, _, _ = svc.Login(attacker, user.Email, "wrong")
	}

	// 同一 IP 被锁定（即使密码正确）
	_, _, err := svc.Login(attacker, user.Email, authGuardTestPassword)
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)

	// 其他 IP 的账号所有者不受影响
	_, _, err = svc.Login(owner, user.Email, authGuardTestPassword)
	require.NoError(t, err)

	// 所有者登录成功只重置自己的 email+IP 计数，攻击者 IP 仍被锁定
	_, _, err = svc.Login(attacker, user.Email, authGuardTestPassword)
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)
}

func TestValidatePasswordCredentials_LockIsScopedToClientIP(t *testing.T) {
	user := &User{ID: 1, Email: "user@example.com", Status: StatusActive, Role: RoleUser}
	svc, _ := newAuthGuardTestService(t, user)
	attacker := withClientIP("203.0.113.9")

	for i := 1; i <= loginFailureLimit; i++ {
		_, _ = svc.ValidatePasswordCredentials(attacker, user.Email, "wrong")
	}
	_, err := svc.ValidatePasswordCredentials(attacker, user.Email, authGuardTestPassword)
	require.ErrorIs(t, err, ErrLoginTooManyAttempts)

	_, err = svc.ValidatePasswordCredentials(withClientIP("198.51.100.7"), user.Email, authGuardTestPassword)
	require.NoError(t, err)
}
