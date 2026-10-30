package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerPayoutsMigration(t *testing.T) {
	content, err := FS.ReadFile("252_owner_payouts.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS owner_payouts")
	require.Contains(t, sql, "owner_user_id BIGINT NOT NULL REFERENCES users(id)")
	// 金额与 usage_logs 成本同精度（USD）。
	require.Contains(t, sql, "amount DECIMAL(20,8) NOT NULL CHECK (amount > 0)")
	require.Contains(t, sql, "paid_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "deleted_at TIMESTAMPTZ")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS ownerpayout_owner_user_id_paid_at ON owner_payouts (owner_user_id, paid_at)")
	// 打款记录是财务审计数据：所有者用户被物理删除时不得级联删除。
	require.NotContains(t, strings.ToUpper(sql), "ON DELETE CASCADE")
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")
}
