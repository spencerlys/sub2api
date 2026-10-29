package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 管理端 DTO 暴露 owner_user_id 以区分所有者资源；管理员资源省略该字段，用户侧分组/代理 DTO 不暴露。
func TestAdminDTOsExposeOwnerUserID(t *testing.T) {
	owner := int64(7)
	decode := func(v any) map[string]any {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		out := map[string]any{}
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}

	require.Equal(t, float64(7), decode(AccountFromService(&service.Account{ID: 1, OwnerUserID: &owner}))["owner_user_id"])
	require.Equal(t, float64(7), decode(GroupFromServiceAdmin(&service.Group{ID: 1, OwnerUserID: &owner}))["owner_user_id"])
	require.Equal(t, float64(7), decode(ProxyFromServiceAdmin(&service.Proxy{ID: 1, OwnerUserID: &owner}))["owner_user_id"])
	require.Equal(t, float64(7), decode(ProxyWithAccountCountFromServiceAdmin(&service.ProxyWithAccountCount{Proxy: service.Proxy{ID: 1, OwnerUserID: &owner}}))["owner_user_id"])

	require.NotContains(t, decode(AccountFromService(&service.Account{ID: 1})), "owner_user_id")
	require.NotContains(t, decode(GroupFromServiceAdmin(&service.Group{ID: 1})), "owner_user_id")
	require.NotContains(t, decode(ProxyFromServiceAdmin(&service.Proxy{ID: 1})), "owner_user_id")
	require.NotContains(t, decode(GroupFromService(&service.Group{ID: 1, OwnerUserID: &owner})), "owner_user_id")
	require.NotContains(t, decode(ProxyFromService(&service.Proxy{ID: 1, OwnerUserID: &owner})), "owner_user_id")
}
