package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentShop01PendingAmountMigration(t *testing.T) {
	content, err := FS.ReadFile("251_payment_shop01_pending_amount.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	// SHOP01 按精确金额归属订单：同一实例的两笔 PENDING 订单不得同额（按分比较）。
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS paymentorder_shop01_pending_pay_amount_unique")
	require.Contains(t, sql, "ON payment_orders (provider_instance_id, (round(pay_amount * 100)::bigint))")
	require.Contains(t, sql, "WHERE status = 'PENDING' AND provider_key = 'shop01'")
	// 普通事务迁移：不得混入并发索引（那是 _notx.sql 的职责）。
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")
}
