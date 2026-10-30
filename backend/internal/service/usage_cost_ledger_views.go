package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// UsageRowSettlement 单次请求的精确拆分视图（用量明细用）。未拆分（待结算）的请求没有该视图。
type UsageRowSettlement struct {
	PaidCost float64
	GiftCost float64
	// Earnings 应付所有者 = 账户成本 × paid_cost / actual_cost；GiftAccountCost = 账户成本 − Earnings（仅参考）。
	// 账户无所有者时均为 nil。
	Earnings        *float64
	GiftAccountCost *float64
}

// UsageSplitStats 与用量统计同一筛选条件下的拆分汇总。Earnings 与 GiftAccountCost 只计有所有者的账户。
type UsageSplitStats struct {
	PaidCost        float64
	GiftCost        float64
	Earnings        float64
	GiftAccountCost float64
	PendingRequests int64
}

// UsageCostSplitReader 用量明细与统计读取拆分所需的查询（由账本仓储实现）。
type UsageCostSplitReader interface {
	GetUsageSplits(ctx context.Context, ids []int64) (map[int64]UsageRowSettlement, error)
	UsageSplitStats(ctx context.Context, filters usagestats.UsageLogFilters) (*UsageSplitStats, error)
}

// UsageRowSettlements 按用量 id 读取拆分；账本不可用时返回空结果（所有行均视为待结算）。
func (s *UsageCostLedgerService) UsageRowSettlements(ctx context.Context, ids []int64) (map[int64]UsageRowSettlement, error) {
	if s == nil || s.repo == nil || len(ids) == 0 {
		return map[int64]UsageRowSettlement{}, nil
	}
	reader, ok := s.repo.(UsageCostSplitReader)
	if !ok {
		return map[int64]UsageRowSettlement{}, nil
	}
	return reader.GetUsageSplits(ctx, ids)
}

// AttachSplitStats 把拆分汇总写入用量统计（新增字段，原有字段不变）。账本不可用时不写入。
func (s *UsageCostLedgerService) AttachSplitStats(ctx context.Context, stats *usagestats.UsageStats, filters usagestats.UsageLogFilters) error {
	if s == nil || s.repo == nil || stats == nil {
		return nil
	}
	reader, ok := s.repo.(UsageCostSplitReader)
	if !ok {
		return nil
	}
	split, err := reader.UsageSplitStats(ctx, filters)
	if err != nil || split == nil {
		return err
	}
	stats.TotalPaidCost = &split.PaidCost
	stats.TotalGiftCost = &split.GiftCost
	stats.TotalEarnings = &split.Earnings
	stats.TotalGiftAccountCost = &split.GiftAccountCost
	stats.PendingRequests = &split.PendingRequests
	return nil
}
