package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	ownerResourceAccount = service.OwnerResourceAccount
	ownerResourceGroup   = service.OwnerResourceGroup
	ownerResourceProxy   = service.OwnerResourceProxy
	// 单次请求每类可引用的资源上限；归属按类批量一次查询，上限仅防超大 ID 列表。
	ownerMaxReferencedIDs = 200
	ownerMaxBodyBytes     = 4 << 20
)

// ownerOutboundAccountPaths 会立即以账户凭证或代理发起出站请求的接口，执行前复查出站地址。
var ownerOutboundAccountPaths = map[string]bool{
	"/api/v1/admin/accounts/:id/test":                 true,
	"/api/v1/admin/accounts/:id/models":               true,
	"/api/v1/admin/accounts/:id/models/sync-upstream": true,
	"/api/v1/admin/accounts/:id/refresh":              true,
	"/api/v1/admin/accounts/:id/recover-state":        true,
}

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
	// unchanged 记录被编辑资源当前已有的引用，请求体原样回传时不再要求归属。
	unchanged := map[string][]int64{}

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
		switch {
		case kind == ownerResourceAccount && path == "/api/v1/admin/accounts/:id" && c.Request.Method == http.MethodPut:
			// 编辑表单总会回传当前分组与代理；管理员为该账户绑定的分组/代理属于“保持不变”的引用。
			account, err := h.adminService.GetAccount(ctx, id)
			if err != nil || account == nil {
				ownerScopeAbort(c, http.StatusNotFound, "资源不存在")
				return
			}
			unchanged[ownerResourceGroup] = account.GroupIDs
			if account.ProxyID != nil {
				unchanged[ownerResourceProxy] = []int64{*account.ProxyID}
			}
		case kind == ownerResourceAccount && ownerOutboundAccountPaths[path]:
			// 发起出站前复查地址，覆盖规则生效前保存的账户。
			account, err := h.adminService.GetAccount(ctx, id)
			if err != nil || account == nil {
				ownerScopeAbort(c, http.StatusNotFound, "资源不存在")
				return
			}
			if err := service.ValidateOwnerAccountOutbound(ctx, account); err != nil {
				c.Abort()
				response.ErrorFrom(c, err)
				return
			}
		case kind == ownerResourceProxy && path == "/api/v1/admin/proxies/:id" && c.Request.Method == http.MethodPut:
			p, err := h.adminService.GetProxy(ctx, id)
			if err != nil || p == nil {
				ownerScopeAbort(c, http.StatusNotFound, "资源不存在")
				return
			}
			if p.BackupProxyID != nil {
				unchanged[ownerResourceProxy] = []int64{*p.BackupProxyID}
			}
		}
	}

	query := c.Request.URL.Query()
	// 所有者不得按用户或 API Key 检索，避免借筛选结果推断用户身份。
	if query.Has("user_id") || query.Has("api_key_id") {
		ownerScopeAbort(c, http.StatusForbidden, "账号所有者不能按用户或 API Key 筛选")
		return
	}
	for key, kind := range map[string]string{"account_id": ownerResourceAccount, "group_id": ownerResourceGroup, "proxy_id": ownerResourceProxy, "group": ownerResourceGroup} {
		raw := strings.TrimSpace(query.Get(key))
		// 账户列表的 group 筛选另有 "ungrouped" 与 0（全部）取值，不涉及具体分组。
		if raw == "" || (key == "group" && (raw == "ungrouped" || raw == "0")) {
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
		ids = slices.DeleteFunc(ids, func(id int64) bool { return slices.Contains(unchanged[kind], id) })
		if len(ids) > 0 && !h.ownerOwns(ctx, owner.OwnerUserID, kind, ids) {
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
	// encoding/json 绑定结构体时字段名大小写不敏感（含 Unicode 折叠，如 "group_idſ"），
	// 而下方按精确键名取值；非规范写法的引用字段一律拒绝，否则可绕过归属校验。
	for key := range fields {
		for ref := range ownerBodyReferences {
			if key != ref && strings.EqualFold(key, ref) {
				return nil, http.StatusBadRequest, "请求字段名无效"
			}
		}
		if key != "model_routing" && strings.EqualFold(key, "model_routing") {
			return nil, http.StatusBadRequest, "请求字段名无效"
		}
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
// 每类资源一次批量查询，只读取归属列。
func (h *AccountHandler) ownerOwns(ctx context.Context, ownerID int64, kind string, ids []int64) bool {
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return false
		}
		if !slices.Contains(unique, id) {
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 || len(unique) > ownerMaxReferencedIDs {
		return false
	}
	owners, err := h.adminService.ResourceOwnerIDs(ctx, kind, unique)
	if err != nil {
		return false
	}
	for _, id := range unique {
		if got := owners[id]; got == nil || *got != ownerID {
			return false
		}
	}
	return true
}

// ownerForeignGroupIDs 返回这些账户所绑分组中不属于当前所有者的分组（例如管理员把所有者账户
// 加入的共享分组），响应中只以引用输出。非所有者请求返回 nil；归属查询失败时全部视为他人分组。
func (h *AccountHandler) ownerForeignGroupIDs(ctx context.Context, accounts ...*dto.Account) map[int64]struct{} {
	ownerID := adminaccess.OwnerUserID(ctx)
	if ownerID <= 0 {
		return nil
	}
	var ids []int64
	for _, account := range accounts {
		if account == nil {
			continue
		}
		for _, group := range account.Groups {
			if group != nil {
				ids = append(ids, group.ID)
			}
		}
		for _, accountGroup := range account.AccountGroups {
			ids = append(ids, accountGroup.GroupID)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	foreign := make(map[int64]struct{})
	if len(ids) == 0 {
		return foreign
	}
	owners, err := h.adminService.ResourceOwnerIDs(ctx, ownerResourceGroup, ids)
	for _, id := range ids {
		if owner := owners[id]; err != nil || owner == nil || *owner != ownerID {
			foreign[id] = struct{}{}
		}
	}
	return foreign
}

func ownerScopeAbort(c *gin.Context, status int, message string) {
	c.Abort()
	response.Error(c, status, message)
}

// isOwnerRequest 供处理器裁剪所有者可写字段与输出。
func isOwnerRequest(c *gin.Context) bool {
	return adminaccess.OwnerFromContext(c.Request.Context()) != nil
}
