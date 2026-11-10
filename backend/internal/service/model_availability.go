package service

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/tidwall/gjson"
)

// 按账号可用模型路由（Anthropic）。
//
// 每个账号保存一份"本地模型列表"（上游 GET /v1/models 的结果，由 ModelAvailabilitySyncService
// 每天在随机时刻同步一次）。请求时只看这份列表，不访问上游：
//
//   - 账号有列表，且请求的是官方 Claude 模型 ID，但列表里没有它 → 该账号不参与调度；
//   - 账号没有列表（从未同步成功）或请求的是别名/自定义名 → 保持原行为，不过滤；
//   - 真实请求仍被上游以"无权使用该模型"拒绝（列表有误）→ 该模型在此账号上暂停到下次同步，
//     并从列表中移除；账号本身不再因此被标记为 error（见 isAnthropicModelEntitlementRejection）。

const (
	UpstreamModelsExtraKey           = "upstream_models"
	UpstreamModelsSyncedAtExtraKey   = "upstream_models_synced_at"
	UpstreamModelsNextSyncAtExtraKey = "upstream_models_next_sync_at"
	UpstreamModelsLastErrorExtraKey  = "upstream_models_last_error"

	// upstreamModelUnavailableReason 写入 model_rate_limits 的原因，用来区分真正的限流。
	upstreamModelUnavailableReason = "upstream_model_unavailable"
	// upstreamModelUnavailableFallback 账号没有下次同步时间时，模型暂停的时长。
	upstreamModelUnavailableFallback = 24 * time.Hour
	// upstreamModelUnavailableMin 暂停时长下限，避免下次同步恰好临近时立刻重试同一账号。
	upstreamModelUnavailableMin = 30 * time.Minute
)

// upstreamModelsExtraKeys 只由同步任务与真实失败写入；管理员/所有者编辑账号时既不能注入也不能清掉。
var upstreamModelsExtraKeys = []string{
	UpstreamModelsExtraKey,
	UpstreamModelsSyncedAtExtraKey,
	UpstreamModelsNextSyncAtExtraKey,
	UpstreamModelsLastErrorExtraKey,
}

// officialClaudeModelIDPattern 官方 Claude 模型 ID 的形态：claude-<家族>-<主版本>[-<次版本>][-<日期>]，
// 以及更早的 claude-<n>[-<n>]-<家族>[-<日期>]。只有这类 ID 才按列表过滤，别名与自定义名不受影响。
var officialClaudeModelIDPattern = regexp.MustCompile(
	`^claude-(?:(?:opus|sonnet|haiku|fable|mythos)-\d+(?:-\d+)?|\d+(?:-\d+)?-(?:opus|sonnet|haiku))(?:-\d{8})?$`,
)

// modelAvailabilityEligible 只有能拉取上游模型列表的 Anthropic 账号才参与（Bedrock/服务账号除外）。
func modelAvailabilityEligible(a *Account) bool {
	return a != nil && a.Platform == PlatformAnthropic && !a.IsBedrock() && a.Type != AccountTypeServiceAccount
}

// UpstreamModelList 返回账号保存的上游模型列表；ok=false 表示从未同步成功。
func (a *Account) UpstreamModelList() (models []string, ok bool) {
	if a == nil || a.Extra == nil {
		return nil, false
	}
	switch raw := a.Extra[UpstreamModelsExtraKey].(type) {
	case []string:
		models = raw
	case []any:
		models = make([]string, 0, len(raw))
		for _, v := range raw {
			if s, isStr := v.(string); isStr && strings.TrimSpace(s) != "" {
				models = append(models, strings.TrimSpace(s))
			}
		}
	default:
		return nil, false
	}
	return models, len(models) > 0
}

func extraTime(extra map[string]any, key string) *time.Time {
	if extra == nil {
		return nil
	}
	raw, _ := extra[key].(string)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

// UpstreamModelsNextSyncAt 下次同步时间；nil 表示尚未排期。
func (a *Account) UpstreamModelsNextSyncAt() *time.Time {
	if a == nil {
		return nil
	}
	return extraTime(a.Extra, UpstreamModelsNextSyncAtExtraKey)
}

// availabilityModelID 把请求模型归一化成上游列表里的写法：去掉 Claude Code 的 [1m] 选择器，
// 套用账号 model_mapping，统一小写。
func availabilityModelID(a *Account, requestedModel string) string {
	model := normalizeClaudeCodeLongContextModel(strings.TrimSpace(requestedModel))
	if model == "" {
		return ""
	}
	if a != nil {
		if mapped := strings.TrimSpace(a.GetMappedModel(model)); mapped != "" {
			model = normalizeClaudeCodeLongContextModel(mapped)
		}
	}
	return strings.ToLower(model)
}

// isOfficialClaudeModelID 是否为官方 Claude 模型 ID（内置模型表或符合官方命名形态）。
func isOfficialClaudeModelID(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	for _, m := range claude.DefaultModels {
		if strings.EqualFold(m.ID, model) {
			return true
		}
	}
	return officialClaudeModelIDPattern.MatchString(model)
}

// modelListContains 比较时兼容短名与带日期的长名（claude-haiku-4-5 ↔ claude-haiku-4-5-20251001）。
func modelListContains(list []string, model string) bool {
	candidates := []string{model, strings.ToLower(claude.NormalizeModelID(model)), strings.ToLower(claude.DenormalizeModelID(model))}
	for _, item := range list {
		item = strings.ToLower(strings.TrimSpace(item))
		for _, c := range candidates {
			if c != "" && item == c {
				return true
			}
		}
	}
	return false
}

// UpstreamModelAvailable 请求时的本地检查：只有账号确有列表、请求的是官方模型 ID 且列表里没有它时才返回 false。
func (a *Account) UpstreamModelAvailable(requestedModel string) bool {
	if !modelAvailabilityEligible(a) {
		return true
	}
	list, ok := a.UpstreamModelList()
	if !ok {
		return true
	}
	model := availabilityModelID(a, requestedModel)
	if model == "" || !isOfficialClaudeModelID(model) {
		return true
	}
	return modelListContains(list, model)
}

// modelAvailabilityRoutingEnabled 网关是否启用按账号可用模型过滤（未加载配置时视为关闭）。
func (s *GatewayService) modelAvailabilityRoutingEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.ModelAvailabilityRoutingEnabled
}

// modelEntitlementPhrases 明确的"无权使用该模型"措辞（已小写，"_"/"-" 视为空格），任何状态码都算。
var modelEntitlementPhrases = []string{
	"does not have access",
	"do not have access",
	"don't have access",
	"no access to",
	"not available on your",
	"not available for your",
	"not allowed to use",
	"not entitled",
	"not enabled for your",
	"upgrade your plan",
}

// modelUnavailablePhrases 较宽泛的措辞，只对 403/404 生效：400 常是请求本身的问题
// （如 "thinking is not supported for model X"），按它暂停会让一个坏请求在逐个换号时把整组账号的该模型停掉。
var modelUnavailablePhrases = []string{
	"not available",
	"unavailable",
	"not supported",
	"not found",
	"not permitted",
	"not enabled",
}

// isAnthropicModelEntitlementRejection 判断 400/403/404 是否是"该账号不能用这个模型"，而不是账号级封禁。
// 必须同时满足：错误信息提到模型（"model" 或请求的模型 ID），并且含有无权/不可用一类的措辞。
// 只回显模型名的普通参数错误（如 max_tokens 超限）与不提模型的组织级 403
// （"OAuth authentication is currently not allowed for this organization"）都不算。
func isAnthropicModelEntitlementRejection(statusCode int, body []byte, models ...string) bool {
	switch statusCode {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound:
	default:
		return false
	}
	msg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	if msg == "" {
		msg = gjson.GetBytes(body, "message").String()
	}
	normalized := normalizeModelNotFoundBody([]byte(msg))
	if normalized == "" {
		return false
	}
	mentionsModel := strings.Contains(normalized, "model")
	for _, m := range models {
		m = normalizeModelNotFoundBody([]byte(normalizeClaudeCodeLongContextModel(strings.TrimSpace(m))))
		if m != "" && strings.Contains(normalized, m) {
			mentionsModel = true
		}
	}
	if !mentionsModel {
		return false
	}
	// Anthropic 对不存在/不可用模型的 404：{"type":"not_found_error","message":"model: claude-x"}
	if statusCode == http.StatusNotFound && gjson.GetBytes(body, "error.type").String() == "not_found_error" {
		return true
	}
	phrased := strings.NewReplacer("’", "'").Replace(normalized)
	for _, p := range modelEntitlementPhrases {
		if strings.Contains(phrased, p) {
			return true
		}
	}
	if statusCode == http.StatusBadRequest {
		return false
	}
	for _, p := range modelUnavailablePhrases {
		if strings.Contains(phrased, p) {
			return true
		}
	}
	return false
}

// upstreamModelUnavailableUntil 模型在该账号上暂停到下次同步（至少 30 分钟）；没有排期时按 24 小时。
func upstreamModelUnavailableUntil(a *Account, now time.Time) time.Time {
	next := a.UpstreamModelsNextSyncAt()
	if next == nil {
		return now.Add(upstreamModelUnavailableFallback)
	}
	if next.After(now.Add(upstreamModelUnavailableMin)) {
		return *next
	}
	return now.Add(upstreamModelUnavailableMin)
}

// markAnthropicModelUnavailable 真实请求被拒、本地列表有误：该模型在此账号上暂停到下次同步
// （model_rate_limits，reason=upstream_model_unavailable，调度已据此排除），并从本地列表移除。
// 账号状态不变，其它模型照常调度。
func (s *RateLimitService) markAnthropicModelUnavailable(ctx context.Context, account *Account, requestedModel string, statusCode int) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}
	now := time.Now()
	resetAt := upstreamModelUnavailableUntil(account, now)
	scope := modelRateLimitKeyForUpstreamModelNotFound(ctx, account, requestedModel)
	if scope != "" {
		if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, scope, resetAt, upstreamModelUnavailableReason); err != nil {
			slog.Warn("upstream_model_unavailable_set_model_rate_limit_failed", "account_id", account.ID, "model", scope, "error", err)
		}
	}
	if list, ok := account.UpstreamModelList(); ok {
		if trimmed, changed := upstreamModelListWithout(list, availabilityModelID(account, requestedModel)); changed {
			if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{UpstreamModelsExtraKey: trimmed}); err != nil {
				slog.Warn("upstream_model_unavailable_update_list_failed", "account_id", account.ID, "model", scope, "error", err)
			}
		}
	}
	slog.Info("upstream_model_unavailable", "account_id", account.ID, "model", scope, "status_code", statusCode, "until", resetAt)
}

// upstreamModelListWithout 去掉列表中与 model 等价的项；返回新列表与是否有变化。
func upstreamModelListWithout(list []string, model string) ([]string, bool) {
	out := make([]string, 0, len(list))
	changed := false
	for _, item := range list {
		if modelListContains([]string{item}, model) {
			changed = true
			continue
		}
		out = append(out, item)
	}
	return out, changed
}
