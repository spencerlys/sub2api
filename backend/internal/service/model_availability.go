package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// Anthropic 账号"无权使用某个模型"的被动处理（不向上游发任何额外请求）。
//
// 真实请求被上游以"本账号不能用这个模型"拒绝时（400/403/404 且错误信息指向模型），
// 只在该账号上把这个模型暂停 24 小时（model_rate_limits，调度已据此排除），并让网关换号重试；
// 账号本身不因此被标记为 error。组织级封禁（如 OAuth 被禁用、不提模型的 403）仍按原逻辑停用账号。

const (
	// upstreamModelUnavailableReason 写入 model_rate_limits 的原因，用来区分真正的限流。
	upstreamModelUnavailableReason = "upstream_model_unavailable"
	// upstreamModelUnavailablePause 模型在该账号上的暂停时长；到期后由下一次真实请求重新验证。
	upstreamModelUnavailablePause = 24 * time.Hour
)

// modelAvailabilityEligible 仅 Anthropic 直连账号（Bedrock/服务账号除外）走模型级暂停。
func modelAvailabilityEligible(a *Account) bool {
	return a != nil && a.Platform == PlatformAnthropic && !a.IsBedrock() && a.Type != AccountTypeServiceAccount
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

// markAnthropicModelUnavailable 真实请求被拒：该模型在此账号上暂停 24 小时
// （model_rate_limits，reason=upstream_model_unavailable，调度已据此排除）。账号状态不变，其它模型照常调度。
func (s *RateLimitService) markAnthropicModelUnavailable(ctx context.Context, account *Account, requestedModel string, statusCode int) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}
	resetAt := time.Now().Add(upstreamModelUnavailablePause)
	scope := modelRateLimitKeyForUpstreamModelNotFound(ctx, account, requestedModel)
	if scope != "" {
		if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, scope, resetAt, upstreamModelUnavailableReason); err != nil {
			slog.Warn("upstream_model_unavailable_set_model_rate_limit_failed", "account_id", account.ID, "model", scope, "error", err)
		}
	}
	slog.Info("upstream_model_unavailable", "account_id", account.ID, "model", scope, "status_code", statusCode, "until", resetAt)
}
