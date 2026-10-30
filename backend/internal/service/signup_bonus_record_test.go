//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// redeemCreateRecorder 记录 Create 调用；其余方法沿用 redeemRepoStub。
type redeemCreateRecorder struct {
	redeemRepoStub
	created []RedeemCode
}

func (r *redeemCreateRecorder) Create(_ context.Context, code *RedeemCode) error {
	code.ID = int64(len(r.created) + 1)
	r.created = append(r.created, *code)
	return nil
}

func newAuthServiceWithRedeem(repo *userRepoStub, redeem RedeemCodeRepository, settings map[string]string) *AuthService {
	cfg := &config.Config{
		JWT:     config.JWTConfig{Secret: "test-secret", ExpireHour: 1},
		Default: config.DefaultConfig{UserBalance: 3.5, UserConcurrency: 2},
	}
	settingService := NewSettingService(&settingRepoStub{values: settings}, cfg)
	return NewAuthService(nil, repo, redeem, nil, cfg, settingService, nil, nil, nil, nil, nil, nil, nil)
}

func requireGiftRecord(t *testing.T, rec RedeemCode, userID int64, amount float64, notes string) {
	t.Helper()
	require.Equal(t, AdjustmentTypeAdminBalance, rec.Type)
	require.Equal(t, StatusUsed, rec.Status)
	require.InDelta(t, amount, rec.Value, 1e-12)
	require.NotNil(t, rec.UsedBy)
	require.Equal(t, userID, *rec.UsedBy)
	require.NotNil(t, rec.UsedAt)
	require.NotEmpty(t, rec.Code)
	require.Equal(t, notes, rec.Notes)
	src, rest, explicit := ParseBalanceSourceNotes(rec.Notes)
	require.True(t, explicit)
	require.Equal(t, BalanceSourceGift, src, "signup credit must count as gift in the cost ledger")
	require.NotEmpty(t, rest)
}

func TestRegisterRecordsSignupBonusAsGift(t *testing.T) {
	repo := &userRepoStub{nextID: 5}
	redeem := &redeemCreateRecorder{}
	svc := newAuthServiceWithRedeem(repo, redeem, map[string]string{
		SettingKeyRegistrationEnabled: "true",
		SettingKeyDefaultBalance:      "3.5",
	})

	_, user, err := svc.Register(context.Background(), "user@example.com", "password")
	require.NoError(t, err)
	require.Equal(t, 3.5, user.Balance, "balance logic unchanged")
	require.Len(t, redeem.created, 1)
	requireGiftRecord(t, redeem.created[0], 5, 3.5, SignupBonusNotes)
}

func TestRegisterWithoutBonusWritesNoRecord(t *testing.T) {
	repo := &userRepoStub{nextID: 6}
	redeem := &redeemCreateRecorder{}
	svc := newAuthServiceWithRedeem(repo, redeem, map[string]string{
		SettingKeyRegistrationEnabled: "true",
		SettingKeyDefaultBalance:      "0",
	})

	_, user, err := svc.Register(context.Background(), "user@example.com", "password")
	require.NoError(t, err)
	require.Zero(t, user.Balance)
	require.Empty(t, redeem.created)
}

func TestAdminCreateUserRecordsInitialBalanceAsGift(t *testing.T) {
	redeem := &redeemCreateRecorder{}
	cfg := &config.Config{}
	settingService := NewSettingService(&settingRepoStub{values: map[string]string{SettingKeyDefaultBalance: "2"}}, cfg)
	svc := &adminServiceImpl{userRepo: &userRepoStub{nextID: 11}, settingService: settingService, redeemCodeRepo: redeem}

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{Email: "a@example.com", Password: "strong-pass"})
	require.NoError(t, err)
	require.Len(t, redeem.created, 1)
	requireGiftRecord(t, redeem.created[0], user.ID, 2, SignupBonusNotes)

	explicit := 7.25
	svc.userRepo = &userRepoStub{nextID: 12}
	user, err = svc.CreateUser(context.Background(), &CreateUserInput{Email: "b@example.com", Password: "strong-pass", Balance: &explicit})
	require.NoError(t, err)
	require.Len(t, redeem.created, 2)
	requireGiftRecord(t, redeem.created[1], user.ID, 7.25, AdminInitialBalanceNotes)

	zero := 0.0
	svc.userRepo = &userRepoStub{nextID: 13}
	_, err = svc.CreateUser(context.Background(), &CreateUserInput{Email: "c@example.com", Password: "strong-pass", Balance: &zero})
	require.NoError(t, err)
	require.Len(t, redeem.created, 2, "no record for a zero balance")
}
