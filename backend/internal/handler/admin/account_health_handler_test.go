//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountHealthReaderStub struct {
	calls       int
	rangeKey    string
	ownerFilter *int64
	ctxOwner    int64
}

func (s *accountHealthReaderStub) GetAccountHealth(ctx context.Context, rawRange string, ownerUserID *int64) (*service.AccountHealthReport, error) {
	s.calls++
	s.rangeKey, s.ownerFilter, s.ctxOwner = rawRange, ownerUserID, adminaccess.OwnerUserID(ctx)
	return &service.AccountHealthReport{Range: rawRange, Items: []service.AccountHealthItem{{ID: 1, Name: "a", RequestsOK: 9, ErrorsTotal: 1, ErrorRate: 0.1}}}, nil
}

func newAccountHealthRouter(ownerID int64, reader accountHealthReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &AccountHandler{adminService: &ownerScopeTestService{accounts: map[int64]*int64{}}, accountHealth: reader}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if ownerID > 0 {
			principal := &adminaccess.Principal{OwnerUserID: ownerID, Policy: adminaccess.OwnerPolicy()}
			c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), principal))
		}
	}, h.OwnerResourceScope)
	router.GET("/api/v1/admin/accounts/health", h.GetHealth)
	// /:id 同时登记，确认 health 不会被当成账户 ID。
	router.GET("/api/v1/admin/accounts/:id", func(c *gin.Context) { c.Status(http.StatusTeapot) })
	return router
}

func accountHealthCall(router *gin.Engine, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/health"+query, nil))
	return w
}

func TestAccountHealthHandler_RangeValidation(t *testing.T) {
	reader := &accountHealthReaderStub{}
	router := newAccountHealthRouter(0, reader)
	for _, q := range []string{"?range=30d", "?range=2h", "?range=0"} {
		require.Equal(t, http.StatusBadRequest, accountHealthCall(router, q).Code, q)
	}
	require.Zero(t, reader.calls)

	w := accountHealthCall(router, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "24h", reader.rangeKey)
	var body struct {
		Data service.AccountHealthReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "24h", body.Data.Range)
	require.Len(t, body.Data.Items, 1)
	// 响应不含用户身份字段。
	require.NotContains(t, w.Body.String(), "user_id")
	require.NotContains(t, w.Body.String(), "api_key")

	require.Equal(t, http.StatusOK, accountHealthCall(router, "?range=7d").Code)
	require.Equal(t, "7d", reader.rangeKey)
}

func TestAccountHealthHandler_AdminOwnerFilter(t *testing.T) {
	reader := &accountHealthReaderStub{}
	router := newAccountHealthRouter(0, reader)
	require.Equal(t, http.StatusOK, accountHealthCall(router, "?range=1h&owner_user_id=8").Code)
	require.NotNil(t, reader.ownerFilter)
	require.EqualValues(t, 8, *reader.ownerFilter)
	require.Equal(t, http.StatusBadRequest, accountHealthCall(router, "?owner_user_id=abc").Code)
	require.Equal(t, http.StatusBadRequest, accountHealthCall(router, "?owner_user_id=-1").Code)
}

func TestAccountHealthHandler_OwnerCannotFilterOtherOwners(t *testing.T) {
	reader := &accountHealthReaderStub{}
	router := newAccountHealthRouter(7, reader)

	require.Equal(t, http.StatusForbidden, accountHealthCall(router, "?owner_user_id=8").Code)
	require.Zero(t, reader.calls)
	// 所有者不得按用户或 API Key 筛选（由所有者资源中间件拦截）。
	require.Equal(t, http.StatusForbidden, accountHealthCall(router, "?user_id=1").Code)
	require.Zero(t, reader.calls)

	require.Equal(t, http.StatusOK, accountHealthCall(router, "?owner_user_id=7").Code)
	require.Nil(t, reader.ownerFilter)
	require.EqualValues(t, 7, reader.ctxOwner)

	require.Equal(t, http.StatusOK, accountHealthCall(router, "?range=1h").Code)
	require.Nil(t, reader.ownerFilter)
	require.EqualValues(t, 7, reader.ctxOwner)
}

func TestAccountHealthHandler_RouteAllowedForOwnerCatalogue(t *testing.T) {
	require.True(t, adminaccess.RouteAllowed(adminaccess.OwnerPolicy(), http.MethodGet, "/api/v1/admin/accounts/health"))
	require.False(t, adminaccess.RouteAllowed(adminaccess.OwnerPolicy(), http.MethodPost, "/api/v1/admin/accounts/health"))
}

func TestAccountHealthHandler_UnavailableWithoutService(t *testing.T) {
	router := newAccountHealthRouter(0, nil)
	require.Equal(t, http.StatusServiceUnavailable, accountHealthCall(router, "").Code)
}
