// Package adminaccess 定义管理密钥的显式授权，不依赖 HTTP 或数据库实现。
package adminaccess

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type Rule struct {
	Permissions      []string `json:"permissions"`
	GroupIDs         []int64  `json:"group_ids"`
	AllGroups        bool     `json:"all_groups"`
	IncludeUngrouped bool     `json:"include_ungrouped"`
	AccountTypes     []string `json:"account_types"`
	Platforms        []string `json:"platforms"`
}

type Policy struct {
	Rules             []Rule   `json:"rules"`
	GlobalPermissions []string `json:"global_permissions"`
	// owner 不参与序列化，只能由 OwnerPolicy 构造，存储的管理密钥策略无法获得所有者目录。
	owner bool
}

// Principal 是受限管理密钥或 AI 账号所有者（OwnerUserID 非零）的身份。
type Principal struct {
	KeyID       string
	KeyName     string
	CreatorID   int64
	OwnerUserID int64
	Policy      Policy
}

type Account struct {
	ID       int64
	GroupIDs []int64
	Type     string
	Platform string
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// FromContext 只返回受限管理密钥身份；所有者身份见 OwnerFromContext，
// 避免所有者落入管理密钥专用的脱敏处理器。
func FromContext(ctx context.Context) *Principal {
	principal, _ := ctx.Value(principalContextKey{}).(*Principal)
	if principal == nil || principal.OwnerUserID != 0 {
		return nil
	}
	return principal
}

// OwnerFromContext 返回 AI 账号所有者身份；管理员与管理密钥均返回 nil。
func OwnerFromContext(ctx context.Context) *Principal {
	principal, _ := ctx.Value(principalContextKey{}).(*Principal)
	if principal == nil || principal.OwnerUserID <= 0 {
		return nil
	}
	return principal
}

// OwnerUserID 返回当前所有者用户 ID，非所有者返回 0。
func OwnerUserID(ctx context.Context) int64 {
	if principal := OwnerFromContext(ctx); principal != nil {
		return principal.OwnerUserID
	}
	return 0
}

// ownerPermissions 在包初始化时由静态目录计算一次，之后只读。
var ownerPermissions = func() []string {
	permissions := []string{}
	for _, definition := range definitions {
		if definition.Scope == "owner" {
			permissions = append(permissions, definition.ID)
		}
	}
	return slices.Clip(permissions)
}()

// OwnerPolicy 授予全部所有者目录权限；资源归属由处理器与仓储按 OwnerUserID 再次限定。
// 返回的权限切片与其他调用方共享，只读；容量已截断，追加不会改写共享数据。
func OwnerPolicy() Policy {
	return Policy{GlobalPermissions: ownerPermissions, owner: true}
}

// Matches 保持分组、类型和平台在同一规则中匹配，不拆开合并授权条件。
func (r Rule) Matches(a Account) bool {
	if !slices.Contains(r.AccountTypes, a.Type) || (len(r.Platforms) > 0 && !slices.Contains(r.Platforms, a.Platform)) {
		return false
	}
	if len(a.GroupIDs) == 0 {
		return r.IncludeUngrouped
	}
	if r.AllGroups {
		return true
	}
	for _, id := range a.GroupIDs {
		if id > 0 && slices.Contains(r.GroupIDs, id) {
			return true
		}
	}
	return false
}

func (p Policy) RulesFor(permission string) []Rule {
	rules := make([]Rule, 0)
	for _, r := range p.Rules {
		if slices.Contains(r.Permissions, permission) {
			rules = append(rules, r)
		}
	}
	return rules
}

func (p Policy) AllowsAccount(permission string, a Account) bool {
	for _, r := range p.RulesFor(permission) {
		if r.Matches(a) {
			return true
		}
	}
	return false
}

// AllowsGroup 只有当前账户及目标分组在同一授权规则内，才能调整该关联。
func (p Policy) AllowsGroup(permission string, a Account, groupID int64) bool {
	if groupID <= 0 {
		return false
	}
	for _, r := range p.RulesFor(permission) {
		if r.Matches(a) && (r.AllGroups || slices.Contains(r.GroupIDs, groupID)) {
			return true
		}
	}
	return false
}

// AllowsGroupConfig 只授权显式勾选的分组配置动作；账户类型不适用于整个分组的配置。
// 分组与平台仍须命中同一规则，不能从不同规则拼接授权。
func (p Policy) AllowsGroupConfig(permission string, id int64, platform string) bool {
	if id <= 0 {
		return false
	}
	for _, r := range p.RulesFor(permission) {
		if (r.AllGroups || slices.Contains(r.GroupIDs, id)) && (len(r.Platforms) == 0 || slices.Contains(r.Platforms, platform)) {
			return true
		}
	}
	return false
}

func ValidatePolicy(p Policy) error {
	if len(p.Rules) > 32 || len(p.GlobalPermissions) > 128 {
		return fmt.Errorf("授权规则数量超过限制")
	}
	catalog := make(map[string]string)
	for _, permission := range PermissionCatalog() {
		catalog[permission.ID] = permission.Scope
	}
	for _, permission := range p.GlobalPermissions {
		if catalog[permission] != "global" {
			return fmt.Errorf("无效的全站权限：%s", permission)
		}
	}
	for _, r := range p.Rules {
		if len(r.Permissions) > 64 || len(r.GroupIDs) > 1000 || len(r.AccountTypes) > 32 || len(r.Platforms) > 64 {
			return fmt.Errorf("授权范围超过限制")
		}
		for _, permission := range r.Permissions {
			if catalog[permission] != "account" && catalog[permission] != "group" {
				return fmt.Errorf("无效的资源权限：%s", permission)
			}
		}
		for _, id := range r.GroupIDs {
			if id <= 0 {
				return fmt.Errorf("分组 ID 必须是正整数")
			}
		}
		for _, kind := range r.AccountTypes {
			if !slices.Contains([]string{"oauth", "setup-token", "apikey", "upstream", "bedrock", "service_account"}, kind) {
				return fmt.Errorf("无效的账户类型：%s", kind)
			}
		}
		for _, platform := range r.Platforms {
			if strings.TrimSpace(platform) != platform || platform == "" || len(platform) > 64 || platform == "*" {
				return fmt.Errorf("无效的平台")
			}
		}
	}
	return nil
}
