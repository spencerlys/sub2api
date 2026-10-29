package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	ownerResourceAccount = "account"
	ownerResourceGroup   = "group"
	ownerResourceProxy   = "proxy"
	// 单次请求可引用的资源上限，避免以超大 ID 列表放大归属查询。
	ownerMaxReferencedIDs = 1000
	ownerMaxBodyBytes     = 4 << 20
)

// ownerPathResources 按路径前缀确定 :id 指向的资源；所有者目录中其他带 :id 的路由一律拒绝。
var ownerPathResources = []struct {
	prefix string
	kind   string
}{
	{"/api/v1/admin/accounts/:id", ownerResourceAccount},
	{"/api/v1/admin/proxies/:id", ownerResourceProxy},
	{"/api/v1/admin/groups/:id", ownerResourceGroup},
}

// ownerBodyReferences 列出请求体中引用其他资源的顶层字段，均须属于当前所有者。
// 分组的兜底分组、复制来源等字段由分组处理器直接丢弃，不在此放行。
var ownerBodyReferences = map[string]string{
	"proxy_id":        ownerResourceProxy,
	"backup_proxy_id": ownerResourceProxy,
	"group_ids":       ownerResourceGroup,
	"account_id":      ownerResourceAccount,
	"account_ids":     ownerResourceAccount,
}

// OwnerResourceScope 在 AI 账号所有者进入原处理器前校验路径、查询与请求体引用的资源归属。
// 管理员与管理密钥直接放行；路由准入已由认证中间件按所有者目录完成。
func (h *AccountHandler) OwnerResourceScope(c *gin.Context) {
	owner := adminaccess.OwnerFromContext(c.Request.Context())
	if owner == nil {
		return
	}
	ctx := c.Request.Context()
	path := c.FullPath()

	if raw := c.Param("id"); raw != "" {
		kind := ""
		for _, item := range ownerPathResources {
			if path == item.prefix || strings.HasPrefix(path, item.prefix+"/") {
				kind = item.kind
				break
			}
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if kind == "" || err != nil || id < 0 {
			ownerScopeAbort(c, http.StatusForbidden, "此操作未开放给账号所有者")
			return
		}
		// 新建分组时前端以 0 读取平台默认模型候选，不涉及任何已有资源。
		allowZero := kind == ownerResourceGroup && path == "/api/v1/admin/groups/:id/model-allowlist-candidates" && c.Request.Method == http.MethodGet
		if !(id == 0 && allowZero) && !h.ownerOwns(ctx, owner.OwnerUserID, kind, []int64{id}) {
			ownerScopeAbort(c, http.StatusNotFound, "资源不存在")
			return
		}
	}

	query := c.Request.URL.Query()
	// 所有者不得按用户或 API Key 检索，避免借筛选结果推断用户身份。
	if query.Has("user_id") || query.Has("api_key_id") {
		ownerScopeAbort(c, http.StatusForbidden, "账号所有者不能按用户或 API Key 筛选")
		return
	}
	for key, kind := range map[string]string{"account_id": ownerResourceAccount, "group_id": ownerResourceGroup, "proxy_id": ownerResourceProxy} {
		raw := strings.TrimSpace(query.Get(key))
		if raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 || !h.ownerOwns(ctx, owner.OwnerUserID, kind, []int64{id}) {
			ownerScopeAbort(c, http.StatusNotFound, "资源不存在")
			return
		}
	}

	if c.Request.Body == nil || c.Request.Method == http.MethodGet {
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, ownerMaxBodyBytes+1))
	if err != nil || len(body) > ownerMaxBodyBytes {
		ownerScopeAbort(c, http.StatusBadRequest, "请求体无效")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return
	}
	refs, status, message := ownerBodyReferenceIDs(body)
	if status != 0 {
		ownerScopeAbort(c, status, message)
		return
	}
	for kind, ids := range refs {
		if !h.ownerOwns(ctx, owner.OwnerUserID, kind, ids) {
			ownerScopeAbort(c, http.StatusNotFound, "引用的资源不存在")
			return
		}
	}
}

// ownerBodyReferenceIDs 解析请求体中的资源引用；0 或 null 表示清除引用，不需校验。
func ownerBodyReferenceIDs(body []byte) (map[string][]int64, int, string) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return nil, http.StatusBadRequest, "请求格式无效"
	}
	refs := map[string][]int64{}
	add := func(kind string, id int64) {
		if id > 0 {
			refs[kind] = append(refs[kind], id)
		}
	}
	for key, kind := range ownerBodyReferences {
		raw, ok := fields[key]
		if !ok || string(bytes.TrimSpace(raw)) == "null" {
			continue
		}
		if strings.HasSuffix(key, "_ids") {
			var ids []int64
			if json.Unmarshal(raw, &ids) != nil {
				return nil, http.StatusBadRequest, "资源 ID 列表无效"
			}
			for _, id := range ids {
				add(kind, id)
			}
			continue
		}
		var id int64
		if json.Unmarshal(raw, &id) != nil {
			return nil, http.StatusBadRequest, "资源 ID 无效"
		}
		add(kind, id)
	}
	// 分组模型路由指向账户 ID，只能路由到本人账户。
	if raw, ok := fields["model_routing"]; ok && string(bytes.TrimSpace(raw)) != "null" {
		var routing map[string][]int64
		if json.Unmarshal(raw, &routing) != nil {
			return nil, http.StatusBadRequest, "模型路由无效"
		}
		for _, ids := range routing {
			for _, id := range ids {
				add(ownerResourceAccount, id)
			}
		}
	}
	for _, ids := range refs {
		if len(ids) > ownerMaxReferencedIDs {
			return nil, http.StatusBadRequest, "引用的资源过多"
		}
	}
	return refs, 0, ""
}

// ownerOwns 判断全部资源均登记为该所有者；任一不存在或归属他人即视为不存在。
func (h *AccountHandler) ownerOwns(ctx context.Context, ownerID int64, kind string, ids []int64) bool {
	owned := func(got *int64) bool { return got != nil && *got == ownerID }
	unique := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 {
			return false
		}
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if kind == ownerResourceAccount {
		accounts, err := h.adminService.GetAccountsByIDs(ctx, unique)
		if err != nil {
			return false
		}
		found := map[int64]bool{}
		for _, a := range accounts {
			if a != nil && owned(a.OwnerUserID) {
				found[a.ID] = true
			}
		}
		return len(found) == len(unique)
	}
	for _, id := range unique {
		switch kind {
		case ownerResourceProxy:
			p, err := h.adminService.GetProxy(ctx, id)
			if err != nil || p == nil || !owned(p.OwnerUserID) {
				return false
			}
		case ownerResourceGroup:
			g, err := h.adminService.GetGroup(ctx, id)
			if err != nil || g == nil || !owned(g.OwnerUserID) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func ownerScopeAbort(c *gin.Context, status int, message string) {
	c.Abort()
	response.Error(c, status, message)
}

// isOwnerRequest 供处理器裁剪所有者可写字段与输出。
func isOwnerRequest(c *gin.Context) bool {
	return adminaccess.OwnerFromContext(c.Request.Context()) != nil
}
