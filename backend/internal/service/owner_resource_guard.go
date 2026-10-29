package service

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

// AI 账号所有者可引用的资源种类，与处理器层的路径/字段登记一致。
const (
	OwnerResourceAccount = "account"
	OwnerResourceGroup   = "group"
	OwnerResourceProxy   = "proxy"
)

// ownerOutboundResolveTimeout 限定所有者出站地址校验的 DNS 解析耗时。
const ownerOutboundResolveTimeout = 5 * time.Second

var (
	ErrOwnerResourceNotFound      = infraerrors.NotFound("OWNER_RESOURCE_NOT_FOUND", "引用的资源不存在")
	ErrOwnerResourceLookup        = infraerrors.InternalServer("OWNER_RESOURCE_LOOKUP_UNAVAILABLE", "resource owner lookup unavailable")
	ErrOwnerOutboundHostForbidden = infraerrors.BadRequest("OWNER_OUTBOUND_HOST_FORBIDDEN", "地址不能指向内网、回环、链路本地或无法解析的主机")
	ErrOwnerBillingProbeManaged   = infraerrors.Forbidden("OWNER_BILLING_PROBE_MANAGED", "上游计费探测由管理员维护，不能改变账户类型使其失效")
	ErrOwnerSharedGroupManaged    = infraerrors.Forbidden("OWNER_SHARED_GROUP_MANAGED", "该账号已被管理员加入共享分组，请联系管理员移出后再操作")
)

// ResourceOwnerReader 批量读取资源归属（id → owner_user_id，nil 为管理员所有）。
// 每类资源一次查询，不存在或已删除的 id 不出现在结果中，且不附加所有者列表范围。
type ResourceOwnerReader interface {
	ResourceOwnerIDs(ctx context.Context, kind string, ids []int64) (map[int64]*int64, error)
}

// ResourceOwnerIDs 由生产仓储批量实现；仓储缺少该能力时拒绝（fail closed），不逐个回退查询。
func (s *adminServiceImpl) ResourceOwnerIDs(ctx context.Context, kind string, ids []int64) (map[int64]*int64, error) {
	reader, ok := s.accountRepo.(ResourceOwnerReader)
	if !ok {
		return nil, ErrOwnerResourceLookup
	}
	return reader.ResourceOwnerIDs(ctx, kind, ids)
}

// ensureOwnerReferences 在所有者请求中校验新引用的资源均归本人所有；unchanged 为当前已存在、
// 本次保持不变的引用（例如管理员绑定的分组/代理），不再要求归属。管理员与管理密钥直接放行。
// 与处理器层 OwnerResourceScope 语义一致，保证不依赖中间件的 JSON 字段清单。
func (s *adminServiceImpl) ensureOwnerReferences(ctx context.Context, kind string, ids []int64, unchanged ...int64) error {
	ownerID := adminaccess.OwnerUserID(ctx)
	if ownerID <= 0 {
		return nil
	}
	check := make([]int64, 0, len(ids))
	for _, id := range normalizeInt64IDs(ids) {
		if !slices.Contains(unchanged, id) {
			check = append(check, id)
		}
	}
	if len(check) == 0 {
		return nil
	}
	owners, err := s.ResourceOwnerIDs(ctx, kind, check)
	if err != nil {
		return err
	}
	for _, id := range check {
		if owner := owners[id]; owner == nil || *owner != ownerID {
			return ErrOwnerResourceNotFound
		}
	}
	return nil
}

// ensureOwnerKeepsSharedGroups 拒绝所有者把账户移出不属于自己的分组（管理员加入的共享池）：
// 撤出会让其他所有者的账户突然承接这部分流量，须由管理员操作。remaining 为 nil 表示删除账户。
// 已删除的分组不在归属结果中，不阻止。管理员与管理密钥直接放行。
func (s *adminServiceImpl) ensureOwnerKeepsSharedGroups(ctx context.Context, previous, remaining []int64) error {
	ownerID := adminaccess.OwnerUserID(ctx)
	if ownerID <= 0 {
		return nil
	}
	var removed []int64
	for _, id := range normalizeInt64IDs(previous) {
		if !slices.Contains(remaining, id) {
			removed = append(removed, id)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	owners, err := s.ResourceOwnerIDs(ctx, OwnerResourceGroup, removed)
	if err != nil {
		return err
	}
	for _, id := range removed {
		if owner, exists := owners[id]; exists && (owner == nil || *owner != ownerID) {
			return ErrOwnerSharedGroupManaged
		}
	}
	return nil
}

// checkOwnerOutboundHost 拒绝所有者把出站地址指向内网：localhost、回环/私网/链路本地/未指定等
// 字面量 IP、云元数据主机名，以及解析结果含上述地址或无法解析的域名（fail closed）。
// 管理员与管理密钥不受影响。
func checkOwnerOutboundHost(ctx context.Context, host string) error {
	if adminaccess.OwnerUserID(ctx) <= 0 {
		return nil
	}
	host = strings.TrimSuffix(strings.Trim(strings.TrimSpace(host), "[]"), ".")
	if host == "" || urlvalidator.IsBlockedHost(host) {
		return ErrOwnerOutboundHostForbidden
	}
	lookupCtx, cancel := context.WithTimeout(ctx, ownerOutboundResolveTimeout)
	defer cancel()
	if blocked, err := isPrivateOrLoopbackHost(lookupCtx, host); err != nil || blocked {
		return ErrOwnerOutboundHostForbidden
	}
	return nil
}

// ownerAccountOutboundURLs 收集账户中会被服务端直接请求的地址：凭证 base_url、
// 分协议 api_base_urls 与 extra.custom_base_url。
func ownerAccountOutboundURLs(credentials, extra map[string]any) []string {
	urls := []string{}
	add := func(v any) {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			urls = append(urls, strings.TrimSpace(s))
		}
	}
	add(credentials["base_url"])
	if byProtocol, ok := credentials["api_base_urls"].(map[string]any); ok {
		for _, v := range byProtocol {
			add(v)
		}
	}
	add(extra["custom_base_url"])
	return urls
}

// checkOwnerAccountOutbound 校验所有者账户新增的出站地址；previous 中已存在的地址不重复解析，
// 避免无关编辑因 DNS 抖动失败。
func checkOwnerAccountOutbound(ctx context.Context, credentials, extra map[string]any, previous []string) error {
	if adminaccess.OwnerUserID(ctx) <= 0 {
		return nil
	}
	for _, raw := range ownerAccountOutboundURLs(credentials, extra) {
		if slices.Contains(previous, raw) {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			return ErrOwnerOutboundHostForbidden
		}
		if err := checkOwnerOutboundHost(ctx, parsed.Hostname()); err != nil {
			return err
		}
	}
	return nil
}

// ValidateOwnerAccountOutbound 供测试/同步等即时出站接口在发起请求前复查账户地址与所用代理，
// 覆盖规则生效前已保存的资源。管理员与管理密钥直接放行。
func ValidateOwnerAccountOutbound(ctx context.Context, account *Account) error {
	if adminaccess.OwnerUserID(ctx) <= 0 || account == nil {
		return nil
	}
	if err := checkOwnerAccountOutbound(ctx, account.Credentials, account.Extra, nil); err != nil {
		return err
	}
	if account.Proxy != nil {
		return checkOwnerOutboundHost(ctx, account.Proxy.Host)
	}
	return nil
}
