package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ownerPayoutCtxStub 记录服务收到的上下文，用于校验操作者传递。
type ownerPayoutCtxStub struct {
	*stubAdminService
	createCtx, updateCtx context.Context
	lastSortBy           string
}

func (s *ownerPayoutCtxStub) CreateGroup(ctx context.Context, input *service.CreateGroupInput) (*service.Group, error) {
	s.createCtx = ctx
	g, err := s.stubAdminService.CreateGroup(ctx, input)
	if g != nil && input.OwnerPayoutMultiplier != nil {
		g.OwnerPayoutMultiplier = *input.OwnerPayoutMultiplier
	}
	return g, err
}

func (s *ownerPayoutCtxStub) UpdateGroup(ctx context.Context, id int64, input *service.UpdateGroupInput) (*service.Group, error) {
	s.updateCtx = ctx
	return s.stubAdminService.UpdateGroup(ctx, id, input)
}

func (s *ownerPayoutCtxStub) ListGroups(ctx context.Context, page, pageSize int, platform, status, search string, isExclusive *bool, sortBy, sortOrder string) ([]service.Group, int64, error) {
	s.lastSortBy = sortBy
	return s.stubAdminService.ListGroups(ctx, page, pageSize, platform, status, search, isExclusive, sortBy, sortOrder)
}

func newOwnerPayoutGroupRouter(svc service.AdminService, ownerUserID int64) *gin.Engine {
	h := NewGroupHandler(svc, nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		if ownerUserID > 0 {
			principal := &adminaccess.Principal{OwnerUserID: ownerUserID, Policy: adminaccess.OwnerPolicy()}
			c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), principal))
		}
	})
	r.GET("/groups", h.List)
	r.GET("/groups/all", h.GetAll)
	r.GET("/groups/:id", h.GetByID)
	r.POST("/groups", h.Create)
	r.PUT("/groups/:id", h.Update)
	return r
}

func ownerPayoutDo(t *testing.T, r *gin.Engine, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	var out map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	return res.Code, out
}

func TestGroupHandlerOwnerPayoutMultiplierAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &ownerPayoutCtxStub{stubAdminService: newStubAdminService()}
	r := newOwnerPayoutGroupRouter(svc, 0)

	code, body := ownerPayoutDo(t, r, http.MethodPost, "/groups", `{"name":"g","platform":"anthropic","rate_multiplier":1,"owner_payout_multiplier":0.8}`)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, svc.createdGroups, 1)
	require.NotNil(t, svc.createdGroups[0].OwnerPayoutMultiplier)
	require.Equal(t, 0.8, *svc.createdGroups[0].OwnerPayoutMultiplier)
	require.Equal(t, 0.8, body["data"].(map[string]any)["owner_payout_multiplier"])
	operator := service.OperatorUserIDFromContext(svc.createCtx)
	require.NotNil(t, operator, "the history row records who changed the rate")
	require.Equal(t, int64(42), *operator)

	code, _ = ownerPayoutDo(t, r, http.MethodPost, "/groups", `{"name":"g","platform":"anthropic","rate_multiplier":1}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, svc.createdGroups[1].OwnerPayoutMultiplier, "omitted means the service default 1")

	for _, bad := range []string{"-0.5", "10.01", "1e9"} {
		code, _ = ownerPayoutDo(t, r, http.MethodPost, "/groups", `{"name":"g","platform":"anthropic","rate_multiplier":1,"owner_payout_multiplier":`+bad+`}`)
		require.Equal(t, http.StatusBadRequest, code, bad)
		code, _ = ownerPayoutDo(t, r, http.MethodPut, "/groups/5", `{"owner_payout_multiplier":`+bad+`}`)
		require.Equal(t, http.StatusBadRequest, code, bad)
	}
	require.Len(t, svc.createdGroups, 2)
	require.Empty(t, svc.updatedGroups)

	code, _ = ownerPayoutDo(t, r, http.MethodPut, "/groups/5", `{"owner_payout_multiplier":0}`)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, svc.updatedGroups[0].OwnerPayoutMultiplier)
	require.Equal(t, 0.0, *svc.updatedGroups[0].OwnerPayoutMultiplier, "0 is a valid rate")
	require.NotNil(t, service.OperatorUserIDFromContext(svc.updateCtx))
	code, _ = ownerPayoutDo(t, r, http.MethodPut, "/groups/5", `{"name":"renamed"}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, svc.updatedGroups[1].OwnerPayoutMultiplier, "omitted keeps the current rate")

	svc.groups = []service.Group{{ID: 7, Name: "g", Status: service.StatusActive, OwnerPayoutMultiplier: 0.75}}
	_, list := ownerPayoutDo(t, r, http.MethodGet, "/groups?sort_by=owner_payout_multiplier", "")
	require.Equal(t, "owner_payout_multiplier", svc.lastSortBy)
	items := list["data"].(map[string]any)["items"].([]any)
	require.Equal(t, 0.75, items[0].(map[string]any)["owner_payout_multiplier"])
	_, all := ownerPayoutDo(t, r, http.MethodGet, "/groups/all", "")
	require.Equal(t, 0.75, all["data"].([]any)[0].(map[string]any)["owner_payout_multiplier"])
	_, one := ownerPayoutDo(t, r, http.MethodGet, "/groups/7", "")
	require.Contains(t, one["data"].(map[string]any), "owner_payout_multiplier")
}

// 所有者既看不到也改不了号主分成倍率：请求中的值被丢弃（创建按默认 1、更新保持原值），响应省略该字段。
func TestGroupHandlerOwnerPayoutMultiplierHiddenFromOwners(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &ownerPayoutCtxStub{stubAdminService: newStubAdminService()}
	svc.groups = []service.Group{{ID: 7, Name: "g", Status: service.StatusActive, OwnerPayoutMultiplier: 0.75}}
	r := newOwnerPayoutGroupRouter(svc, 9)

	code, body := ownerPayoutDo(t, r, http.MethodPost, "/groups", `{"name":"g","platform":"anthropic","owner_payout_multiplier":5}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, svc.createdGroups[0].OwnerPayoutMultiplier)
	require.NotContains(t, body["data"].(map[string]any), "owner_payout_multiplier")

	code, body = ownerPayoutDo(t, r, http.MethodPut, "/groups/7", `{"name":"g","owner_payout_multiplier":5}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, svc.updatedGroups[0].OwnerPayoutMultiplier)
	require.NotContains(t, body["data"].(map[string]any), "owner_payout_multiplier")

	// 越界值同样被静默丢弃，不向所有者暴露该字段存在。
	code, _ = ownerPayoutDo(t, r, http.MethodPut, "/groups/7", `{"name":"g","owner_payout_multiplier":-3}`)
	require.Equal(t, http.StatusOK, code)

	_, list := ownerPayoutDo(t, r, http.MethodGet, "/groups?sort_by=owner_payout_multiplier", "")
	require.Equal(t, "sort_order", svc.lastSortBy, "owners cannot sort by the hidden rate")
	require.NotContains(t, list["data"].(map[string]any)["items"].([]any)[0].(map[string]any), "owner_payout_multiplier")
	_, all := ownerPayoutDo(t, r, http.MethodGet, "/groups/all", "")
	require.NotContains(t, all["data"].([]any)[0].(map[string]any), "owner_payout_multiplier")
	_, one := ownerPayoutDo(t, r, http.MethodGet, "/groups/7", "")
	require.NotContains(t, one["data"].(map[string]any), "owner_payout_multiplier")
}

func TestSanitizeGroupRequestsDropOwnerPayoutMultiplier(t *testing.T) {
	v := 3.0
	create := CreateGroupRequest{Name: "g", Platform: service.PlatformAnthropic, OwnerPayoutMultiplier: &v}
	require.True(t, sanitizeCreateGroupRequestForOwner(&create))
	require.Nil(t, create.OwnerPayoutMultiplier, "owner-created groups get the default 1")
	update := UpdateGroupRequest{Name: "g", OwnerPayoutMultiplier: &v}
	require.True(t, sanitizeUpdateGroupRequestForOwner(&update))
	require.Nil(t, update.OwnerPayoutMultiplier, "owner updates keep the admin-set rate")

	simpleCreate := CreateGroupRequest{Name: "g", OwnerPayoutMultiplier: &v}
	sanitizeCreateGroupRequestForSimpleMode(&simpleCreate)
	require.Nil(t, simpleCreate.OwnerPayoutMultiplier)
	simpleUpdate := UpdateGroupRequest{Name: "g", OwnerPayoutMultiplier: &v}
	sanitizeUpdateGroupRequestForSimpleMode(&simpleUpdate)
	require.Nil(t, simpleUpdate.OwnerPayoutMultiplier)
}
