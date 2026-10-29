//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func getAdminSettingsData(t *testing.T, h *SettingHandler) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	return data
}

func TestUpdateSettingsOwnerGuideURLValidHTTPSIsPersisted(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})

	rec := doUpdateSettings(t, h, map[string]any{
		"owner_guide_url": "  https://guide.example.com/owners  ",
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "https://guide.example.com/owners", repo.values[service.SettingKeyOwnerGuideURL])

	data := getAdminSettingsData(t, h)
	require.Equal(t, "https://guide.example.com/owners", data["owner_guide_url"])
}

func TestUpdateSettingsOwnerGuideURLRejectsInvalid(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "not a url", "/relative/path", "ftp://guide.example.com"} {
		t.Run(value, func(t *testing.T) {
			h, repo := newStepUpSwitchTestHandler(t, map[string]string{
				service.SettingKeyOwnerGuideURL: "https://guide.example.com/owners",
			})

			rec := doUpdateSettings(t, h, map[string]any{"owner_guide_url": value}, nil)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "Owner guide URL must be an absolute http(s) URL")
			require.Equal(t, "https://guide.example.com/owners", repo.values[service.SettingKeyOwnerGuideURL])
		})
	}
}

func TestUpdateSettingsOwnerGuideURLAllowsFragment(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})

	rec := doUpdateSettings(t, h, map[string]any{
		"owner_guide_url": "https://guide.example.com/owners#proxies",
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "https://guide.example.com/owners#proxies", repo.values[service.SettingKeyOwnerGuideURL])
}

func TestUpdateSettingsOwnerGuideURLEmptyClears(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyOwnerGuideURL: "https://guide.example.com/owners",
	})

	rec := doUpdateSettings(t, h, map[string]any{"owner_guide_url": ""}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "", repo.values[service.SettingKeyOwnerGuideURL])

	data := getAdminSettingsData(t, h)
	require.Equal(t, "", data["owner_guide_url"])
}

func TestUpdateSettingsOwnerGuideURLOmittedKeepsStoredValue(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyOwnerGuideURL: "https://guide.example.com/owners",
	})

	rec := doUpdateSettings(t, h, map[string]any{"risk_control_enabled": true}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "https://guide.example.com/owners", repo.values[service.SettingKeyOwnerGuideURL])
}
