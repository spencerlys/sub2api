//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_OwnerGuideURL(t *testing.T) {
	const guideURL = "https://guide.example.com/owners"

	t.Run("missing value defaults to empty", func(t *testing.T) {
		svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{}}, &config.Config{})

		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.Empty(t, settings.OwnerGuideURL)
	})

	t.Run("stored value is parsed", func(t *testing.T) {
		svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{
			SettingKeyOwnerGuideURL: guideURL,
		}}, &config.Config{})

		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, guideURL, settings.OwnerGuideURL)
	})

	t.Run("value is persisted", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{OwnerGuideURL: guideURL})
		require.NoError(t, err)
		require.Equal(t, guideURL, repo.updates[SettingKeyOwnerGuideURL])
	})

	t.Run("public settings and injection payload expose it", func(t *testing.T) {
		repo := &settingPublicRepoStub{values: map[string]string{SettingKeyOwnerGuideURL: guideURL}}
		svc := NewSettingService(repo, &config.Config{})

		public, err := svc.GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, guideURL, public.OwnerGuideURL)

		raw, err := svc.GetPublicSettingsForInjection(context.Background())
		require.NoError(t, err)
		payload, ok := raw.(*PublicSettingsInjectionPayload)
		require.True(t, ok)
		require.Equal(t, guideURL, payload.OwnerGuideURL)
	})
}
