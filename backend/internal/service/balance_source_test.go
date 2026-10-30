//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBalanceSource(t *testing.T) {
	for raw, want := range map[string]string{
		"gift": BalanceSourceGift, " Offline ": BalanceSourceOffline, "CORRECTION": BalanceSourceCorrection,
	} {
		got, ok := NormalizeBalanceSource(raw)
		require.True(t, ok, raw)
		require.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"", "paid", "online", "gift;drop"} {
		_, ok := NormalizeBalanceSource(raw)
		require.False(t, ok, raw)
	}
}

func TestFormatBalanceSourceNotes(t *testing.T) {
	require.Equal(t, "[src:offline] bank transfer", FormatBalanceSourceNotes(BalanceSourceOffline, "bank transfer"))
	require.Equal(t, "[src:gift]", FormatBalanceSourceNotes(BalanceSourceGift, ""))
	require.Equal(t, "[src:gift]", FormatBalanceSourceNotes(BalanceSourceGift, "   "))
	// 已有前缀时替换而不是叠加，其余备注原样保留。
	require.Equal(t, "[src:correction] fix  typo", FormatBalanceSourceNotes(BalanceSourceCorrection, "[src:offline] fix  typo"))
	require.Equal(t, "[src:offline] x", FormatBalanceSourceNotes(BalanceSourceOffline, "  [SRC:Gift]x"))
	// 未知来源不写前缀，备注保持原样。
	require.Equal(t, "plain", FormatBalanceSourceNotes("bogus", "plain"))
}

func TestParseBalanceSourceNotes(t *testing.T) {
	for _, tc := range []struct {
		notes, source, rest string
		explicit            bool
	}{
		{"", BalanceSourceGift, "", false},
		{"legacy note", BalanceSourceGift, "legacy note", false},
		{"[src:offline] bank transfer", BalanceSourceOffline, "bank transfer", true},
		{"[src:offline]bank transfer", BalanceSourceOffline, "bank transfer", true},
		{"  [SRC:Correction]  fix ", BalanceSourceCorrection, "fix ", true},
		{"[src:gift]", BalanceSourceGift, "", true},
		// 未知或格式不符的前缀按旧数据处理：来源 gift，备注不剥离。
		{"[src:paid] x", BalanceSourceGift, "[src:paid] x", false},
		{"note [src:offline]", BalanceSourceGift, "note [src:offline]", false},
		{"[src:offline", BalanceSourceGift, "[src:offline", false},
	} {
		source, rest, explicit := ParseBalanceSourceNotes(tc.notes)
		require.Equal(t, tc.source, source, tc.notes)
		require.Equal(t, tc.rest, rest, tc.notes)
		require.Equal(t, tc.explicit, explicit, tc.notes)
	}
}

func TestPaidShare(t *testing.T) {
	require.InDelta(t, 0, PaidShare(0, 50), 1e-12, "gift only")
	require.InDelta(t, 1, PaidShare(80, 0), 1e-12, "offline/paid only")
	require.InDelta(t, 0.75, PaidShare(30, 10), 1e-12, "mixed")
	require.InDelta(t, 0, PaidShare(0, 0), 1e-12, "zero denominator")
	require.InDelta(t, 0, PaidShare(-5, 5), 1e-12, "negative inputs never yield a share")
	require.InDelta(t, 1, PaidShare(5, -5), 1e-12, "negative gift clamps to zero")
}

// 按用户汇总入账：更正与负数调整不计入任何一方。
func TestUserCreditTotalsAccumulate(t *testing.T) {
	var c UserCreditTotals
	c.Add(BalanceCredit{Kind: CreditKindPayment, Amount: 100, Bonus: 20}) // 实付 80 + 赠送 20
	c.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: 50, Source: "offline"})
	c.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: 10, Source: ""}) // 旧数据无前缀按 gift
	c.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: 7, Source: "bogus"})
	c.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: 30, Source: "CORRECTION"})
	c.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: -40, Source: "offline"})
	c.Add(BalanceCredit{Kind: CreditKindRedeemCode, Amount: 5})
	c.Add(BalanceCredit{Kind: CreditKindPromo, Amount: 3})
	c.Add(BalanceCredit{Kind: CreditKindAffiliate, Amount: 2})
	c.Add(BalanceCredit{Kind: CreditKindPayment, Amount: 10, Bonus: 15}) // 赠送不能超过到账
	require.InDelta(t, 80+50, c.Paid, 1e-9)
	require.InDelta(t, 20+10+7+5+3+2+10, c.Gift, 1e-9)
	require.InDelta(t, 130.0/(130+57), c.PaidShare(), 1e-12)

	var giftOnly UserCreditTotals
	giftOnly.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: 10, Source: "gift"})
	require.Zero(t, giftOnly.PaidShare())

	var corrOnly UserCreditTotals
	corrOnly.Add(BalanceCredit{Kind: CreditKindAdmin, Amount: 10, Source: "correction"})
	require.Zero(t, corrOnly.Paid+corrOnly.Gift)
	require.Zero(t, corrOnly.PaidShare())
}
