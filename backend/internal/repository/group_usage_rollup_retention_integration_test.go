//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 保留期清理只删过期数据：发布水位不能后退（否则下一次同步会在持有状态行锁时重建整个保留窗口），
// 但剩余各日的日桶、以及 /admin/groups 的汇总必须与 usage_logs 逐日重算的结果完全一致。
func TestGroupUsageRollupRetentionCleanupKeepsWatermarkAndTotals(t *testing.T) {
	for _, partitioned := range []bool{false, true} {
		name := "ordinary"
		if partitioned {
			name = "partitioned"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			useGroupUsageRepositoryTestTimezone(t, "Asia/Shanghai")
			schema := createGroupUsageRollupTriggerTestSchema(t, ctx, partitioned)
			tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
			defer func() { _ = tx.Rollback() }()

			if partitioned {
				// 分区按 UTC 月切；07-31 23:00Z 的那一行属于 7 月分区，却落在上海的 08-01。
				_, err := tx.ExecContext(ctx, `
					CREATE TABLE usage_logs_202607 PARTITION OF usage_logs
						FOR VALUES FROM ('2026-07-01 00:00:00+00') TO ('2026-08-01 00:00:00+00');
					CREATE TABLE usage_logs_202608 PARTITION OF usage_logs
						FOR VALUES FROM ('2026-08-01 00:00:00+00') TO ('2026-09-01 00:00:00+00');
				`)
				require.NoError(t, err)
			}

			_, err := tx.ExecContext(ctx, `
				INSERT INTO groups (id) VALUES (10), (11);
				INSERT INTO users (id) VALUES (1);
				INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at) VALUES
					(1, 1, 10, 100, TIMESTAMPTZ '2026-07-15 12:00:00+08'),
					(2, 1, 10, 200, TIMESTAMPTZ '2026-08-01 07:00:00+08'),
					(3, 1, 11, 2, TIMESTAMPTZ '2026-08-01 10:00:00+08'),
					(9, 1, NULL, 1000, TIMESTAMPTZ '2026-08-01 11:00:00+08'),
					(4, 1, 10, 4, TIMESTAMPTZ '2026-08-02 08:00:00+08'),
					(5, 1, 10, 8, TIMESTAMPTZ '2026-08-02 20:00:00+08'),
					(6, 1, 10, 16, TIMESTAMPTZ '2026-08-03 12:00:00+08'),
					(7, 1, 11, 32, TIMESTAMPTZ '2026-08-03 13:00:00+08'),
					(8, 1, 10, 64, TIMESTAMPTZ '2026-08-05 09:00:00+08');
			`)
			require.NoError(t, err)

			now := time.Date(2026, 8, 5, 2, 0, 0, 0, time.UTC) // 上海 08-05 10:00
			todayStart := time.Date(2026, 8, 4, 16, 0, 0, 0, time.UTC)
			repo := newDashboardAggregationRepositoryWithSQL(tx)
			repo.clock = func() time.Time { return now }
			require.NoError(t, repo.SyncGroupUsageRollups(ctx, todayStart))
			requireGroupUsageRollupClosedBefore(t, ctx, tx, "2026-08-05")
			// 哨兵日桶：任何从保留起点开始的重建都会把它删掉。金额为 0，不影响下面的逐日对账。
			_, err = tx.ExecContext(ctx, `
				INSERT INTO usage_group_daily_rollups (bucket_date, group_id, actual_cost, computed_at)
				VALUES (DATE '2026-08-04', 11, 0, NOW())
			`)
			require.NoError(t, err)

			// 截止点落在上海 08-02 的中午：08-02 是被部分删除的边界日。
			cutoff := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
			require.NoError(t, repo.CleanupUsageLogs(ctx, cutoff))

			requireGroupUsageRollupClosedBefore(t, ctx, tx, "2026-08-05")
			var sentinelKept bool
			require.NoError(t, tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM usage_group_daily_rollups
					WHERE bucket_date = DATE '2026-08-04' AND group_id = 11
				)
			`).Scan(&sentinelKept))
			require.True(t, sentinelKept, "清理过期数据不应触发保留窗口内日桶的重建")
			var retainedFrom time.Time
			require.NoError(t, tx.QueryRowContext(ctx, `
				SELECT retained_from FROM usage_group_rollup_state WHERE id = 1
			`).Scan(&retainedFrom))
			require.True(t, retainedFrom.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)), "retained_from=%s", retainedFrom)

			var staleBuckets int
			require.NoError(t, tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM usage_group_daily_rollups WHERE bucket_date < DATE '2026-08-02'
			`).Scan(&staleBuckets))
			require.Zero(t, staleBuckets)

			// 每个已发布日桶都必须等于从剩余 usage_logs 逐日重算的金额，反之亦然。
			var mismatched int
			require.NoError(t, tx.QueryRowContext(ctx, `
				WITH expected AS (
					SELECT (created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
						group_id,
						SUM(actual_cost) AS actual_cost
					FROM usage_logs
					WHERE group_id IS NOT NULL
						AND created_at < TIMESTAMPTZ '2026-08-05 00:00:00+08'
					GROUP BY 1, 2
				)
				SELECT COUNT(*)
				FROM expected
				FULL JOIN usage_group_daily_rollups rollup
					ON rollup.bucket_date = expected.bucket_date
					AND rollup.group_id = expected.group_id
				WHERE COALESCE(rollup.actual_cost, 0) <> COALESCE(expected.actual_cost, 0)
			`).Scan(&mismatched))
			require.Zero(t, mismatched)

			var boundaryCost float64
			require.NoError(t, tx.QueryRowContext(ctx, `
				SELECT actual_cost FROM usage_group_daily_rollups
				WHERE bucket_date = DATE '2026-08-02' AND group_id = 10
			`).Scan(&boundaryCost))
			require.InDelta(t, 8, boundaryCost, 0.0000001)

			usageRepo := newUsageLogRepositoryWithSQL(nil, tx)
			result, err := usageRepo.GetAllGroupUsageSummary(ctx, todayStart)
			require.NoError(t, err)
			require.Len(t, result, 2)
			require.Equal(t, int64(10), result[0].GroupID)
			require.InDelta(t, 88, result[0].TotalCost, 0.0000001)
			require.InDelta(t, 64, result[0].TodayCost, 0.0000001)
			require.Equal(t, int64(11), result[1].GroupID)
			require.InDelta(t, 32, result[1].TotalCost, 0.0000001)

			// 清理之后的同步不应再有任何需要重建的内容。
			require.NoError(t, repo.SyncGroupUsageRollups(ctx, todayStart))
			requireGroupUsageRollupClosedBefore(t, ctx, tx, "2026-08-05")
		})
	}
}

func requireGroupUsageRollupClosedBefore(t *testing.T, ctx context.Context, q sqlQueryer, want string) {
	t.Helper()
	var closedBefore string
	require.NoError(t, scanSingleRow(ctx, q, `
		SELECT closed_before::text FROM usage_group_rollup_state WHERE id = 1
	`, nil, &closedBefore))
	require.Equal(t, want, closedBefore)
}
