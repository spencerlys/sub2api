package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupOwnerPayoutMultiplierMigration(t *testing.T) {
	content, err := FS.ReadFile("254_group_owner_payout_multiplier.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ALTER TABLE groups ADD COLUMN IF NOT EXISTS owner_payout_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1")
	require.Contains(t, sql, "CHECK (owner_payout_multiplier >= 0)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS group_owner_payout_multiplier_history")
	require.Contains(t, sql, "group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE")
	require.Contains(t, sql, "multiplier NUMERIC(10,4) NOT NULL CHECK (multiplier >= 0)")
	require.Contains(t, sql, "effective_from TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "created_by BIGINT REFERENCES users(id) ON DELETE SET NULL")
	// 按 (分组, 生效时间) 取请求发生时的倍率。
	require.Contains(t, sql, "ON group_owner_payout_multiplier_history (group_id, effective_from DESC, id DESC)")
	// 每个现有分组一条覆盖全部历史的初始记录（当前值 1），已有数字保持不变；重复执行不重复插入。
	require.Contains(t, sql, "SELECT g.id, g.owner_payout_multiplier, '-infinity'::timestamptz FROM groups g")
	require.Contains(t, sql, "WHERE NOT EXISTS (SELECT 1 FROM group_owner_payout_multiplier_history h WHERE h.group_id = g.id)")
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")
	// 历史不可改写：迁移只追加，不更新已有历史行。
	require.NotContains(t, strings.ToUpper(sql), "UPDATE GROUP_OWNER_PAYOUT_MULTIPLIER_HISTORY")
}
