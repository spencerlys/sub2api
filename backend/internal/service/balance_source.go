package service

import (
	"regexp"
	"strings"
)

// 管理员余额调整（redeem_codes.type=admin_balance）的资金来源。不改表结构，
// 以机器可读前缀 "[src:<source>] " 写在 notes 开头，其余备注原样保留。
//   - gift：赠送（默认；无前缀的旧记录同样视为赠送）；
//   - offline：线下实收；
//   - correction：更正，既不算实付也不算赠送。
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

// PaidShare = paid / (paid + gift)，分母为 0 时为 0；负数输入按 0 处理。
func PaidShare(paid, gift float64) float64 {
	if paid < 0 {
		paid = 0
	}
	if gift < 0 {
		gift = 0
	}
	if paid+gift <= 0 {
		return 0
	}
	return paid / (paid + gift)
}

// 入账种类：决定一笔正向入账算实付还是赠送。
const (
	CreditKindPayment    = "payment"     // 在线支付到账（Amount 含 Bonus）
	CreditKindAdmin      = "admin"       // 管理员调整，按 Source 区分
	CreditKindRedeemCode = "redeem_code" // 发放的余额兑换码
	CreditKindPromo      = "promo"       // 优惠码赠送
	CreditKindAffiliate  = "affiliate"   // 推广返利转入余额
)

// BalanceCredit 一笔（或同类汇总的）余额入账。
type BalanceCredit struct {
	Kind   string
	Amount float64
	// Bonus 仅对在线支付有效：到账金额中免费赠送的部分（payment_orders.bonus_amount）。
	Bonus float64
	// Source 仅对管理员调整有效：备注前缀中的原始来源值，未知或为空按 gift。
	Source string
}

// UserCreditTotals 用户累计实付与赠送入账。
type UserCreditTotals struct {
	Paid float64
	Gift float64
}

// Add 计入一笔入账；负数与更正不计入任何一方。
func (t *UserCreditTotals) Add(c BalanceCredit) {
	if c.Amount <= 0 {
		return
	}
	switch c.Kind {
	case CreditKindPayment:
		bonus := c.Bonus
		if bonus < 0 {
			bonus = 0
		}
		if bonus > c.Amount {
			bonus = c.Amount
		}
		t.Paid += c.Amount - bonus
		t.Gift += bonus
	case CreditKindAdmin:
		src, ok := NormalizeBalanceSource(c.Source)
		if !ok {
			src = BalanceSourceGift
		}
		switch src {
		case BalanceSourceOffline:
			t.Paid += c.Amount
		case BalanceSourceGift:
			t.Gift += c.Amount
		}
	default:
		t.Gift += c.Amount
	}
}

// PaidShare 用户的实付占比。
func (t UserCreditTotals) PaidShare() float64 {
	return PaidShare(t.Paid, t.Gift)
}
