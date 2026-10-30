package service

import (
	"regexp"
	"strings"
	"time"
)

// 管理员余额调整（redeem_codes.type=admin_balance）的资金来源。不改表结构，
// 以机器可读前缀 "[src:<source>] " 写在 notes 开头，其余备注原样保留。
//   - gift：赠送（默认；无前缀的旧记录同样视为赠送）；
//   - offline：线下实收；
//   - correction：更正。财务汇总中单列，不算收入；成本账本中正数按赠送计入，负数先冲减赠送
//     （见 usage_cost_ledger.go 的负数规则）。
//
// SQL 侧解析见 repository.balanceSourceSQL，两处规则必须一致。
const (
	BalanceSourceGift       = "gift"
	BalanceSourceOffline    = "offline"
	BalanceSourceCorrection = "correction"
)

var balanceSourcePrefixRe = regexp.MustCompile(`(?i)^\s*\[src:([a-z]+)\]\s*`)

// NormalizeBalanceSource 校验并归一化来源；空值或未知值返回 false。
func NormalizeBalanceSource(raw string) (string, bool) {
	switch s := strings.ToLower(strings.TrimSpace(raw)); s {
	case BalanceSourceGift, BalanceSourceOffline, BalanceSourceCorrection:
		return s, true
	default:
		return "", false
	}
}

// ParseBalanceSourceNotes 解析备注前缀，返回来源、去掉前缀后的备注，以及是否带有效前缀。
// 无前缀或前缀值未知时按旧数据处理：来源 gift，备注原样返回。
func ParseBalanceSourceNotes(notes string) (source, rest string, explicit bool) {
	m := balanceSourcePrefixRe.FindStringSubmatchIndex(notes)
	if m == nil {
		return BalanceSourceGift, notes, false
	}
	src, ok := NormalizeBalanceSource(notes[m[2]:m[3]])
	if !ok {
		return BalanceSourceGift, notes, false
	}
	return src, notes[m[1]:], true
}

// FormatBalanceSourceNotes 以指定来源重写备注前缀（替换已有前缀，不叠加）；来源非法时备注不变。
func FormatBalanceSourceNotes(source, notes string) string {
	src, ok := NormalizeBalanceSource(source)
	if !ok {
		return notes
	}
	_, rest, _ := ParseBalanceSourceNotes(notes)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "[src:" + src + "]"
	}
	return "[src:" + src + "] " + rest
}

// 入账种类：决定一笔入账进赠送桶还是实付桶（见 LedgerBuckets.ApplyCredit）。
const (
	CreditKindPayment    = "payment"     // 在线支付到账（Amount 含 Bonus）
	CreditKindAdmin      = "admin"       // 管理员调整，按 Source 区分
	CreditKindRedeemCode = "redeem_code" // 发放的余额兑换码
	CreditKindPromo      = "promo"       // 优惠码赠送
	CreditKindAffiliate  = "affiliate"   // 推广返利转入余额
)

// 注册赠送等自动发放的余额以管理员调整记录（[src:gift]）留痕，余额本身的发放逻辑不变；
// 成本账本据此把这部分余额计入赠送桶。
const (
	SignupBonusNotes         = "[src:gift] signup bonus"
	FirstBindBonusNotes      = "[src:gift] first bind bonus"
	AdminInitialBalanceNotes = "[src:gift] initial balance"
)

// newGiftBalanceRecord 构造一条已使用的赠送余额调整记录（redeem_codes.type=admin_balance）。
func newGiftBalanceRecord(userID int64, amount float64, notes string) (*RedeemCode, error) {
	code, err := GenerateRedeemCode()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &RedeemCode{
		Code:   code,
		Type:   AdjustmentTypeAdminBalance,
		Value:  amount,
		Status: StatusUsed,
		UsedBy: &userID,
		UsedAt: &now,
		Notes:  notes,
	}, nil
}
