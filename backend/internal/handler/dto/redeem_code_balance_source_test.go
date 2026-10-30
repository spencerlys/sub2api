//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 来源前缀是内部记账标记：用户只看到去掉前缀的备注；管理员另得 balance_source 字段。
func TestRedeemCodeBalanceSourcePrefixIsStripped(t *testing.T) {
	rc := &service.RedeemCode{ID: 1, Type: service.AdjustmentTypeAdminBalance, Value: 5, Notes: "[src:offline] bank transfer"}

	user := RedeemCodeFromService(rc)
	require.NotNil(t, user.Notes)
	require.Equal(t, "bank transfer", *user.Notes)
	raw, err := json.Marshal(user)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "src:")
	require.NotContains(t, string(raw), "balance_source")

	admin := RedeemCodeFromServiceAdmin(rc)
	require.Equal(t, "bank transfer", admin.Notes)
	require.Equal(t, service.BalanceSourceOffline, admin.BalanceSource)

	legacy := RedeemCodeFromServiceAdmin(&service.RedeemCode{Type: service.AdjustmentTypeAdminBalance, Notes: "old"})
	require.Equal(t, service.BalanceSourceGift, legacy.BalanceSource)
	require.Equal(t, "old", legacy.Notes)

	onlyPrefix := RedeemCodeFromService(&service.RedeemCode{Type: service.AdjustmentTypeAdminBalance, Notes: "[src:gift]"})
	require.Nil(t, onlyPrefix.Notes, "a bare prefix leaves no user-visible note")

	other := RedeemCodeFromServiceAdmin(&service.RedeemCode{Type: service.RedeemTypeBalance, Notes: "[src:offline] x"})
	require.Empty(t, other.BalanceSource, "only admin_balance records carry a source")
	require.Equal(t, "[src:offline] x", other.Notes)
}
