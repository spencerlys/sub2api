package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// scopedAdminUsageLogResponse 仅公开该次请求的用量、计费快照和请求元数据。
// 关联资源只返回 ID；新增通用 DTO 字段不会自动获得受限密钥的读取权限。
func scopedAdminUsageLogResponse(record *service.UsageLog) gin.H {
	if record == nil {
		return nil
	}
	// 在转换前移除关联，避免构造含原始密钥、用户余额等内容的嵌套 DTO。
	flat := *record
	flat.User, flat.APIKey, flat.Group, flat.Subscription, flat.Account = nil, nil, nil, nil, nil
	usage := dto.UsageLogFromServiceAdmin(&flat)
	return gin.H{
		"id":                           usage.ID,
		"user_id":                      usage.UserID,
		"api_key_id":                   usage.APIKeyID,
		"account_id":                   usage.AccountID,
		"request_id":                   usage.RequestID,
		"model":                        usage.Model,
		"service_tier":                 usage.ServiceTier,
		"reasoning_effort":             usage.ReasoningEffort,
		"inbound_endpoint":             usage.InboundEndpoint,
		"upstream_endpoint":            usage.UpstreamEndpoint,
		"group_id":                     usage.GroupID,
		"subscription_id":              usage.SubscriptionID,
		"input_tokens":                 usage.InputTokens,
		"output_tokens":                usage.OutputTokens,
		"cache_creation_tokens":        usage.CacheCreationTokens,
		"cache_read_tokens":            usage.CacheReadTokens,
		"cache_creation_5m_tokens":     usage.CacheCreation5mTokens,
		"cache_creation_1h_tokens":     usage.CacheCreation1hTokens,
		"input_cost":                   usage.InputCost,
		"output_cost":                  usage.OutputCost,
		"cache_creation_cost":          usage.CacheCreationCost,
		"cache_read_cost":              usage.CacheReadCost,
		"total_cost":                   usage.TotalCost,
		"actual_cost":                  usage.ActualCost,
		"rate_multiplier":              usage.RateMultiplier,
		"long_context_billing_applied": usage.LongContextBillingApplied,
		"billing_type":                 usage.BillingType,
		"request_type":                 usage.RequestType,
		"stream":                       usage.Stream,
		"openai_ws_mode":               usage.OpenAIWSMode,
		"native_compaction_v2":         usage.NativeCompactionV2,
		"duration_ms":                  usage.DurationMs,
		"first_token_ms":               usage.FirstTokenMs,
		"image_count":                  usage.ImageCount,
		"image_size":                   usage.ImageSize,
		"image_input_size":             usage.ImageInputSize,
		"image_output_size":            usage.ImageOutputSize,
		"image_input_tokens":           usage.ImageInputTokens,
		"image_input_cost":             usage.ImageInputCost,
		"image_output_tokens":          usage.ImageOutputTokens,
		"image_output_cost":            usage.ImageOutputCost,
		"image_size_source":            usage.ImageSizeSource,
		"image_size_breakdown":         usage.ImageSizeBreakdown,
		"media_type":                   usage.MediaType,
		"user_agent":                   usage.UserAgent,
		"ip_address":                   usage.IPAddress,
		"session_id":                   usage.SessionID,
		"cache_ttl_overridden":         usage.CacheTTLOverridden,
		"billing_mode":                 usage.BillingMode,
		"created_at":                   usage.CreatedAt,
		"upstream_model":               usage.UpstreamModel,
		"upstream_reasoning_effort":    usage.UpstreamReasoningEffort,
		"upstream_response_model":      usage.UpstreamResponseModel,
		"upstream_model_mismatch":      usage.UpstreamModelMismatch,
		"channel_id":                   usage.ChannelID,
		"model_mapping_chain":          usage.ModelMappingChain,
		"upstream_request_id":          usage.UpstreamRequestID,
		"billing_tier":                 usage.BillingTier,
		"account_rate_multiplier":      usage.AccountRateMultiplier,
		"account_stats_cost":           usage.AccountStatsCost,
	}
}

// ownerHiddenUsageFields 是所有者不可见的字段：用户身份（Q3），以及用户计费金额、倍率与实付/赠送拆分，
// 只保留账户维度的用量、官方价与本人收益。
var ownerHiddenUsageFields = []string{
	"user_id", "api_key_id", "subscription_id", "ip_address", "user_agent", "session_id",
	"actual_cost", "rate_multiplier", "account_rate_multiplier", "account_stats_cost",
	"paid_cost", "gift_cost", "gift_account_cost",
}

// usageSplitFields 成本账本的逐行拆分；未拆分（待结算）时金额为 null、split_pending=true，不估算。
type usageSplitFields struct {
	PaidCost        *float64 `json:"paid_cost"`
	GiftCost        *float64 `json:"gift_cost"`
	Earnings        *float64 `json:"earnings"`
	GiftAccountCost *float64 `json:"gift_account_cost"`
	SplitPending    bool     `json:"split_pending"`
}

func splitFieldsFor(id int64, splits map[int64]service.UsageRowSettlement) usageSplitFields {
	v, ok := splits[id]
	if !ok {
		return usageSplitFields{SplitPending: true}
	}
	paid, gift := v.PaidCost, v.GiftCost
	return usageSplitFields{PaidCost: &paid, GiftCost: &gift, Earnings: v.Earnings, GiftAccountCost: v.GiftAccountCost}
}

// adminUsageLogWithSplit 管理员完整用量行 + 拆分。
type adminUsageLogWithSplit struct {
	dto.AdminUsageLog
	usageSplitFields
}

func adminUsageLogResponses(c *gin.Context, records []service.UsageLog, splits map[int64]service.UsageRowSettlement) any {
	owner := adminaccess.OwnerFromContext(c.Request.Context()) != nil
	if owner || adminaccess.FromContext(c.Request.Context()) != nil {
		out := make([]gin.H, 0, len(records))
		for i := range records {
			item := scopedAdminUsageLogResponse(&records[i])
			split := splitFieldsFor(records[i].ID, splits)
			item["paid_cost"], item["gift_cost"] = split.PaidCost, split.GiftCost
			item["earnings"], item["gift_account_cost"] = split.Earnings, split.GiftAccountCost
			item["split_pending"] = split.SplitPending
			if owner {
				for _, key := range ownerHiddenUsageFields {
					delete(item, key)
				}
			}
			out = append(out, item)
		}
		return out
	}
	out := make([]adminUsageLogWithSplit, 0, len(records))
	for i := range records {
		out = append(out, adminUsageLogWithSplit{
			AdminUsageLog:    *dto.UsageLogFromServiceAdmin(&records[i]),
			usageSplitFields: splitFieldsFor(records[i].ID, splits),
		})
	}
	return out
}

// ownerEndpointStat 所有者可见的端点统计：不含用户计费金额。
type ownerEndpointStat struct {
	Endpoint    string  `json:"endpoint"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
}

func ownerEndpointStats(in []usagestats.EndpointStat) []ownerEndpointStat {
	if len(in) == 0 {
		return nil
	}
	out := make([]ownerEndpointStat, 0, len(in))
	for _, e := range in {
		out = append(out, ownerEndpointStat{Endpoint: e.Endpoint, Requests: e.Requests, TotalTokens: e.TotalTokens, Cost: e.Cost})
	}
	return out
}

// ownerUsageStatsResponse 所有者的用量统计：官方价、本人收益（不含赠送）与待结算；
// 不含用户计费（actual_cost）、账户成本与实付/赠送拆分。
func ownerUsageStatsResponse(stats *usagestats.UsageStats) gin.H {
	if stats == nil {
		stats = &usagestats.UsageStats{}
	}
	var earnings float64
	if stats.TotalEarnings != nil {
		earnings = *stats.TotalEarnings
	}
	var pending int64
	if stats.PendingRequests != nil {
		pending = *stats.PendingRequests
	}
	out := gin.H{
		"total_requests":              stats.TotalRequests,
		"total_input_tokens":          stats.TotalInputTokens,
		"total_output_tokens":         stats.TotalOutputTokens,
		"total_cache_tokens":          stats.TotalCacheTokens,
		"total_cache_creation_tokens": stats.TotalCacheCreationTokens,
		"total_cache_read_tokens":     stats.TotalCacheReadTokens,
		"total_tokens":                stats.TotalTokens,
		"total_cost":                  stats.TotalCost,
		"total_earnings":              earnings,
		"pending_requests":            pending,
		"average_duration_ms":         stats.AverageDurationMs,
	}
	if eps := ownerEndpointStats(stats.Endpoints); eps != nil {
		out["endpoints"] = eps
	}
	if eps := ownerEndpointStats(stats.UpstreamEndpoints); eps != nil {
		out["upstream_endpoints"] = eps
	}
	if eps := ownerEndpointStats(stats.EndpointPaths); eps != nil {
		out["endpoint_paths"] = eps
	}
	return out
}
