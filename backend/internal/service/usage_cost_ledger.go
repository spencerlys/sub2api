package service

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// 精确的“赠送优先”成本账本：每个用户有赠送桶与实付桶。
//
// 入账（LedgerCredit）：
//   - 在线支付：到账金额减 bonus 进实付桶，bonus 进赠送桶；
//   - 管理员调整：[src:offline] 进实付桶；无前缀、[src:gift]、未知前缀、[src:correction] 的正数进赠送桶
//     （更正不是新到账的现金，不能为所有者产生收益）；
//   - 发放的兑换码、优惠码、返利转入、注册赠送等其余正向入账进赠送桶。
//
// 负数规则（扣减与负数更正）：[src:offline] 的负数视为冲销线下实收，先减实付桶再减赠送桶；
// 其余负数（无前缀、gift、correction 以及其他种类）先减赠送桶再减实付桶。两个桶都不低于 0，超出部分忽略。
//
// 扣费（LedgerDebit，usage_logs.actual_cost）：按 (created_at, id) 顺序先扣赠送桶再扣实付桶；
// 两桶皆空时剩余部分记为实付（透支）。同一时刻入账先于扣费。
// 订阅计费的请求不扣余额，不动两个桶：用户为该分组购买过（已完成、实付 > 0 的）订阅订单时整笔记为实付，否则记为赠送。
//
// 金额使用十进制精确计算，gift_cost + paid_cost 恒等于 actual_cost。

// LedgerCredit 一笔余额入账事件。Key 在同一用户内唯一，用于同一时刻的稳定排序（如 "rc:12"）。
type LedgerCredit struct {
	Key    string
	At     time.Time
	Kind   string
	Source string
	Amount decimal.Decimal
	Bonus  decimal.Decimal
}

// LedgerDebit 一次请求的扣费事件。
type LedgerDebit struct {
	UsageLogID       int64
	At               time.Time
	ActualCost       decimal.Decimal
	BillingType      int8
	SubscriptionPaid bool
	// Fixed 非空时沿用已有拆分（原始用量已被清理的订阅请求），不重新计算。
	Fixed *CostSplit
}

// CostSplit 一次请求的实付/赠送拆分结果。
type CostSplit struct {
	UsageLogID  int64
	UserID      int64
	At          time.Time
	ActualCost  decimal.Decimal
	GiftCost    decimal.Decimal
	PaidCost    decimal.Decimal
	BillingType int8
}

// LedgerBuckets 用户剩余的赠送与实付额度。
type LedgerBuckets struct {
	Gift decimal.Decimal
	Paid decimal.Decimal
}

// classifyCredit 返回正向入账对赠送桶与实付桶的增量。
func classifyCredit(c LedgerCredit) (gift, paid decimal.Decimal) {
	switch c.Kind {
	case CreditKindPayment:
		bonus := decimal.Max(c.Bonus, decimal.Zero)
		bonus = decimal.Min(bonus, c.Amount)
		return bonus, c.Amount.Sub(bonus)
	case CreditKindAdmin:
		if src, ok := NormalizeBalanceSource(c.Source); ok && src == BalanceSourceOffline {
			return decimal.Zero, c.Amount
		}
		return c.Amount, decimal.Zero
	default:
		return c.Amount, decimal.Zero
	}
}

func takeFrom(bucket *decimal.Decimal, want decimal.Decimal) decimal.Decimal {
	take := decimal.Min(*bucket, want)
	if take.IsNegative() {
		take = decimal.Zero
	}
	*bucket = bucket.Sub(take)
	return take
}

// ApplyCredit 计入一笔入账（规则见文件头注释）。
func (b *LedgerBuckets) ApplyCredit(c LedgerCredit) {
	switch {
	case c.Amount.IsPositive():
		gift, paid := classifyCredit(c)
		b.Gift = b.Gift.Add(gift)
		b.Paid = b.Paid.Add(paid)
	case c.Amount.IsNegative():
		rest := c.Amount.Neg()
		if src, ok := NormalizeBalanceSource(c.Source); c.Kind == CreditKindAdmin && ok && src == BalanceSourceOffline {
			rest = rest.Sub(takeFrom(&b.Paid, rest))
			takeFrom(&b.Gift, rest)
			return
		}
		rest = rest.Sub(takeFrom(&b.Gift, rest))
		takeFrom(&b.Paid, rest)
	}
}

// ApplyDebit 扣一次请求费用并返回拆分（UserID 由调用方填写）。
func (b *LedgerBuckets) ApplyDebit(d LedgerDebit) CostSplit {
	if d.Fixed != nil {
		return *d.Fixed
	}
	s := CostSplit{UsageLogID: d.UsageLogID, At: d.At, ActualCost: d.ActualCost, BillingType: d.BillingType, GiftCost: decimal.Zero, PaidCost: decimal.Zero}
	if !d.ActualCost.IsPositive() {
		return s
	}
	if d.BillingType == BillingTypeSubscription {
		if d.SubscriptionPaid {
			s.PaidCost = d.ActualCost
		} else {
			s.GiftCost = d.ActualCost
		}
		return s
	}
	s.GiftCost = takeFrom(&b.Gift, d.ActualCost)
	s.PaidCost = d.ActualCost.Sub(s.GiftCost)
	takeFrom(&b.Paid, s.PaidCost)
	return s
}

// SortLedgerCredits 按 (At, Key) 排序。
func SortLedgerCredits(credits []LedgerCredit) {
	sort.SliceStable(credits, func(i, j int) bool {
		if !credits[i].At.Equal(credits[j].At) {
			return credits[i].At.Before(credits[j].At)
		}
		return credits[i].Key < credits[j].Key
	})
}

// SortLedgerDebits 按 (At, UsageLogID) 排序。
func SortLedgerDebits(debits []LedgerDebit) {
	sort.SliceStable(debits, func(i, j int) bool {
		if !debits[i].At.Equal(debits[j].At) {
			return debits[i].At.Before(debits[j].At)
		}
		return debits[i].UsageLogID < debits[j].UsageLogID
	})
}

// LedgerMerger 按时间合并入账与扣费：同一时刻入账先于扣费。
// credits 须已排序；扣费可分批（每批内及批与批之间按 (At, ID) 递增）喂入。
type LedgerMerger struct {
	Buckets LedgerBuckets
	credits []LedgerCredit
	next    int
}

func NewLedgerMerger(start LedgerBuckets, sortedCredits []LedgerCredit) *LedgerMerger {
	return &LedgerMerger{Buckets: start, credits: sortedCredits}
}

// Debit 先计入所有不晚于该扣费时间的入账，再扣费。
func (m *LedgerMerger) Debit(d LedgerDebit) CostSplit {
	for m.next < len(m.credits) && !m.credits[m.next].At.After(d.At) {
		m.Buckets.ApplyCredit(m.credits[m.next])
		m.next++
	}
	return m.Buckets.ApplyDebit(d)
}

// Finish 计入剩余入账并返回最终余额。
func (m *LedgerMerger) Finish() LedgerBuckets {
	for ; m.next < len(m.credits); m.next++ {
		m.Buckets.ApplyCredit(m.credits[m.next])
	}
	return m.Buckets
}

// RunLedger 从 start 余额开始处理一批事件（会排序输入的副本），返回最终余额与按扣费顺序的拆分。
func RunLedger(start LedgerBuckets, credits []LedgerCredit, debits []LedgerDebit) (LedgerBuckets, []CostSplit) {
	cs := append([]LedgerCredit(nil), credits...)
	ds := append([]LedgerDebit(nil), debits...)
	SortLedgerCredits(cs)
	SortLedgerDebits(ds)
	m := NewLedgerMerger(start, cs)
	splits := make([]CostSplit, 0, len(ds))
	for _, d := range ds {
		splits = append(splits, m.Debit(d))
	}
	return m.Finish(), splits
}
