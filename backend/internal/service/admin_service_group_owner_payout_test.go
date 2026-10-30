//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateGroupOwnerPayoutMultiplierDefaultsAndValidation(t *testing.T) {
	repo := &groupRepoStubForAdmin{}
	svc := &adminServiceImpl{groupRepo: repo}

	created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "g", Platform: PlatformAnthropic, RateMultiplier: 1})
	require.NoError(t, err)
	require.Equal(t, 1.0, created.OwnerPayoutMultiplier, "omitted means the default 1")

	zero := 0.0
	created, err = svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "g0", Platform: PlatformAnthropic, RateMultiplier: 1, OwnerPayoutMultiplier: &zero})
	require.NoError(t, err)
	require.Equal(t, 0.0, created.OwnerPayoutMultiplier, "0 is allowed (owner earns nothing)")

	for _, bad := range []float64{-0.01, MaxOwnerPayoutMultiplier + 0.01} {
		v := bad
		repo.created = nil
		_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "bad", Platform: PlatformAnthropic, RateMultiplier: 1, OwnerPayoutMultiplier: &v})
		require.ErrorIs(t, err, ErrOwnerPayoutMultiplierInvalid, "%v", bad)
		require.Nil(t, repo.created)
	}
}

func TestUpdateGroupOwnerPayoutMultiplier(t *testing.T) {
	existing := &Group{ID: 1, Name: "g", Platform: PlatformAnthropic, Status: StatusActive, RateMultiplier: 1, OwnerPayoutMultiplier: 0.8}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := &adminServiceImpl{groupRepo: repo}

	updated, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{Name: "renamed"})
	require.NoError(t, err)
	require.Equal(t, 0.8, updated.OwnerPayoutMultiplier, "omitted keeps the current value")

	v := 1.25
	updated, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{OwnerPayoutMultiplier: &v})
	require.NoError(t, err)
	require.Equal(t, 1.25, updated.OwnerPayoutMultiplier)

	bad := -1.0
	repo.updated = nil
	_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{OwnerPayoutMultiplier: &bad})
	require.ErrorIs(t, err, ErrOwnerPayoutMultiplierInvalid)
	require.Nil(t, repo.updated)
}

type groupRepoStubNameFree struct{ *groupRepoStubForAdmin }

func (groupRepoStubNameFree) ExistsByName(context.Context, string) (bool, error) { return false, nil }

func TestGroupServiceCreateDefaultsOwnerPayoutMultiplier(t *testing.T) {
	repo := groupRepoStubNameFree{&groupRepoStubForAdmin{}}
	g, err := NewGroupService(repo, nil).Create(context.Background(), CreateGroupRequest{Name: "g", RateMultiplier: 1})
	require.NoError(t, err)
	require.Equal(t, 1.0, g.OwnerPayoutMultiplier, "the zero value would silently zero out owner payouts")
}

func TestSimpleModeDropsOwnerPayoutMultiplier(t *testing.T) {
	v := 3.0
	create := &CreateGroupInput{Name: "s", Platform: PlatformAnthropic, RateMultiplier: 1, OwnerPayoutMultiplier: &v}
	normalizeCreateGroupInputForSimpleMode(create)
	require.Nil(t, create.OwnerPayoutMultiplier)
	update := &UpdateGroupInput{Name: "s", OwnerPayoutMultiplier: &v}
	normalizeUpdateGroupInputForSimpleMode(update)
	require.Nil(t, update.OwnerPayoutMultiplier)
}

func TestOperatorUserIDContext(t *testing.T) {
	require.Nil(t, OperatorUserIDFromContext(context.Background()))
	require.Nil(t, OperatorUserIDFromContext(WithOperatorUserID(context.Background(), 0)))
	id := OperatorUserIDFromContext(WithOperatorUserID(context.Background(), 7))
	require.NotNil(t, id)
	require.Equal(t, int64(7), *id)
}
