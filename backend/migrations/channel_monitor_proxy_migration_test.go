package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorProxyMigration(t *testing.T) {
	content, err := FS.ReadFile("250_channel_monitor_proxy.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	// proxy_id 可空，代理删除置空（监控保留，回退到关联账号代理 / 直连）。
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS proxy_id BIGINT NULL REFERENCES proxies(id) ON DELETE SET NULL")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_channel_monitors_proxy_id ON channel_monitors (proxy_id)")
	// 普通事务迁移：不得混入并发索引（那是 _notx.sql 的职责）。
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")
}
