//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

// totpGuardCacheStub 线程安全的内存 TotpCache，计数语义与 Redis INCR 一致。
type totpGuardCacheStub struct {
	TotpCache
	mu        sync.Mutex
	attempts  map[int64]int
	usedCodes map[string]bool
	setup     *TotpSetupSession
	incrErr   error
	markErr   error
}

func newTotpGuardCacheStub() *totpGuardCacheStub {
	return &totpGuardCacheStub{attempts: map[int64]int{}, usedCodes: map[string]bool{}}
}

func (s *totpGuardCacheStub) IncrementVerifyAttempts(_ context.Context, userID int64) (int, error) {
	if s.incrErr != nil {
		return 0, s.incrErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[userID]++
	return s.attempts[userID], nil
}

func (s *totpGuardCacheStub) GetVerifyAttempts(_ context.Context, userID int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts[userID], nil
}

func (s *totpGuardCacheStub) ClearVerifyAttempts(_ context.Context, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, userID)
	return nil
}

func (s *totpGuardCacheStub) MarkCodeUsed(_ context.Context, userID int64, code string, _ time.Duration) (bool, error) {
	if s.markErr != nil {
		return false, s.markErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%d:%s", userID, code)
	if s.usedCodes[key] {
		return false, nil
	}
	s.usedCodes[key] = true
	return true, nil
}

func (s *totpGuardCacheStub) GetSetupSession(context.Context, int64) (*TotpSetupSession, error) {
	return s.setup, nil
}

func (s *totpGuardCacheStub) SetSetupSession(context.Context, int64, *TotpSetupSession, time.Duration) error {
	return nil
}

func (s *totpGuardCacheStub) DeleteSetupSession(context.Context, int64) error { return nil }

// totpGuardUserRepoStub 统计 TOTP 校验真正评估验证码的次数（每次评估都会读用户）。
type totpGuardUserRepoStub struct {
	UserRepository
	user  *User
	delay time.Duration
	reads atomic.Int32
}

func (s *totpGuardUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	s.reads.Add(1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return s.user, nil
}

func (s *totpGuardUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (s *totpGuardUserRepoStub) EnableTotp(context.Context, int64) error                { return nil }

type totpGuardEncryptorStub struct{}

func (totpGuardEncryptorStub) Encrypt(plaintext string) (string, error)  { return plaintext, nil }
func (totpGuardEncryptorStub) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }

func newTotpGuardService(t *testing.T) (*TotpService, *totpGuardCacheStub, *totpGuardUserRepoStub, string) {
	t.Helper()
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: "u@example.com"})
	require.NoError(t, err)
	secret := key.Secret()
	user := &User{ID: 7, Email: "u@example.com", TotpEnabled: true, TotpSecretEncrypted: &secret}
	cache := newTotpGuardCacheStub()
	repo := &totpGuardUserRepoStub{user: user}
	settingSvc := NewSettingService(&totpVMSettingRepoStub{values: map[string]string{SettingKeyTotpEnabled: "true"}}, nil)
	return NewTotpService(repo, totpGuardEncryptorStub{}, cache, settingSvc, nil, nil), cache, repo, secret
}

// 并发错误验证码不能突破 maxTotpAttempts：每次校验必须先原子占用一次尝试额度。
func TestTotpVerifyCode_ConcurrentWrongCodesCannotExceedAttemptCap(t *testing.T) {
	svc, _, repo, _ := newTotpGuardService(t)
	repo.delay = 20 * time.Millisecond

	const workers = 20
	var tooMany atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if errors.Is(svc.VerifyCode(context.Background(), 7, "000000"), ErrTotpTooManyAttempts) {
				tooMany.Add(1)
			}
		}()
	}
	wg.Wait()

	require.LessOrEqual(t, int(repo.reads.Load()), maxTotpAttempts, "codes evaluated beyond the attempt cap")
	require.Equal(t, int32(workers-maxTotpAttempts), tooMany.Load())
}

func TestTotpVerifyCode_RejectsReplayOfUsedCode(t *testing.T) {
	svc, _, _, secret := newTotpGuardService(t)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	require.NoError(t, svc.VerifyCode(context.Background(), 7, code))
	require.ErrorIs(t, svc.VerifyCode(context.Background(), 7, code), ErrTotpInvalidCode)
}

func TestTotpVerifyCode_AttemptCounterFailureFailsClosed(t *testing.T) {
	svc, cache, repo, secret := newTotpGuardService(t)
	cache.incrErr = errors.New("redis down")
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	require.Error(t, svc.VerifyCode(context.Background(), 7, code))
	require.Zero(t, repo.reads.Load())
}

func TestTotpVerifyCode_ReplayGuardFailureFailsClosed(t *testing.T) {
	svc, cache, _, secret := newTotpGuardService(t)
	cache.markErr = errors.New("redis down")
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	require.Error(t, svc.VerifyCode(context.Background(), 7, code))
}

func TestTotpVerifyCode_SuccessClearsAttempts(t *testing.T) {
	svc, cache, _, secret := newTotpGuardService(t)
	require.ErrorIs(t, svc.VerifyCode(context.Background(), 7, "000000"), ErrTotpInvalidCode)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, svc.VerifyCode(context.Background(), 7, code))

	attempts, _ := cache.GetVerifyAttempts(context.Background(), 7)
	require.Zero(t, attempts)
}

// 调试日志不得输出 TOTP 密钥的任何片段。
func TestTotpService_DebugLogsDoNotLeakSecret(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	svc, cache, _, secret := newTotpGuardService(t)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, svc.VerifyCode(context.Background(), 7, code))

	cache.setup = &TotpSetupSession{Secret: secret, SetupToken: "setup-token"}
	require.NoError(t, svc.CompleteSetup(context.Background(), 7, code, "setup-token"))

	require.NotContains(t, buf.String(), secret[:4])
	require.NotContains(t, buf.String(), "secret_prefix")
}
