package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageCostLedgerMigration(t *testing.T) {
	content, err := FS.ReadFile("253_usage_cost_ledger.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS usage_cost_splits")
	require.Contains(t, sql, "usage_log_id BIGINT PRIMARY KEY")
	// 拆分与 usage_logs 成本同精度。
	require.Contains(t, sql, "gift_cost DECIMAL(20,10) NOT NULL")
	require.Contains(t, sql, "paid_cost DECIMAL(20,10) NOT NULL")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS usage_cost_splits_user_created ON usage_cost_splits (user_id, created_at, usage_log_id)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS usage_cost_ledger_users")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS usage_cost_ledger_cursor")
	// 首次部署登记一次全量重建，为历史用量补算拆分。
	require.Contains(t, sql, "INSERT INTO usage_cost_ledger_cursor (id, rebuild_requested_at) VALUES (1, NOW()) ON CONFLICT (id) DO NOTHING")
	// 拆分不得随用量清理级联删除（重建依赖历史拆分）。
	require.NotContains(t, strings.ToUpper(sql), "REFERENCES USAGE_LOGS")
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")
}
