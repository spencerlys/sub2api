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
