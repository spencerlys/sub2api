package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 本文件汇总 Anthropic 网关三个入口（/v1/messages、/v1/chat/completions、/v1/responses）
// 在选号之后、转发成功之后共用的账号侧簿记，避免各入口各自实现而漏项。

// acquireSelectedAccountSlot 获取选号结果对应的账号并发槽位：调度器已抢到槽位时直接返回；
// 否则按等待计划计入账号等待队列并等待。等待队列已满返回 *WaitQueueFullError，
// 等待超时返回 *ConcurrencyError（调用方可经 FailoverState.RecordAccountWaitFailure 换号重选）。
func acquireSelectedAccountSlot(c *gin.Context, helper *ConcurrencyHelper, selection *service.AccountSelectionResult, reqStream bool, streamStarted *bool, reqLog *zap.Logger) (func(), error) {
	if selection.Acquired {
		return selection.ReleaseFunc, nil
	}
	accountID := selection.Account.ID
	ctx := c.Request.Context()
	canWait, err := helper.IncrementAccountWaitCount(ctx, accountID, selection.WaitPlan.MaxWaiting)
	if err != nil {
		reqLog.Warn("gateway.account_wait_counter_increment_failed", zap.Int64("account_id", accountID), zap.Error(err))
	} else if !canWait {
		reqLog.Info("gateway.account_wait_queue_full",
			zap.Int64("account_id", accountID),
			zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
		)
		return nil, &WaitQueueFullError{SlotType: "account"}
	} else {
		// 拿到槽位或放弃等待后都不再占用等待队列
		defer helper.DecrementAccountWaitCount(ctx, accountID)
	}
	return helper.AcquireAccountSlotWithWaitTimeout(
		c,
		accountID,
		selection.WaitPlan.MaxConcurrency,
		selection.WaitPlan.Timeout,
		reqStream,
		streamStarted,
	)
}

// incrementAccountRPMAfterForward 转发成功后递增账号 RPM 计数。
// TOCTOU 竞态是已知且可接受的设计权衡，与 WindowCost 一致的 soft-limit 模式。
func (h *GatewayHandler) incrementAccountRPMAfterForward(ctx context.Context, account *service.Account, reqLog *zap.Logger) {
	if !account.IsAnthropicOAuthOrSetupToken() || account.GetBaseRPM() <= 0 {
		return
	}
	if err := h.gatewayService.IncrementAccountRPM(ctx, account.ID); err != nil {
		reqLog.Warn("gateway.rpm_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
}

// releaseAccountSessions 立即释放本请求在 failover 链上注册过的会话槽。
// 客户端可能已断开、请求 ctx 已取消，用独立 ctx 执行释放。
func (h *GatewayHandler) releaseAccountSessions(accounts map[int64]*service.Account, sessionKey string) {
	for _, acc := range accounts {
		h.gatewayService.ReleaseAccountSession(context.Background(), acc, sessionKey)
	}
}

// accountSlotReleaseContext 返回仅在 handler 返回时才取消的 ctx，作为账号槽位的兜底释放时机。
// 不能用请求 ctx：客户端断开后流式转发仍在排空上游计费，槽位必须持有到 Forward 返回。
func accountSlotReleaseContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(context.WithoutCancel(c.Request.Context()))
}
