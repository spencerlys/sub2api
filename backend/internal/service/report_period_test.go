//go:build unit

package service

import (
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestParseReportPeriodPresetsUseSiteTimezone(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	// 2026-10-15 01:30 UTC+8 = 2026-10-14 17:30 UTC：日界必须按站点时区。
	now := time.Date(2026, 10, 14, 17, 30, 0, 0, time.UTC)
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, loc) }

	cases := map[string][2]time.Time{
		ReportRangeToday:     {d(2026, 10, 15), d(2026, 10, 16)},
		ReportRangeYesterday: {d(2026, 10, 14), d(2026, 10, 15)},
		ReportRangeThisMonth: {d(2026, 10, 1), d(2026, 10, 16)},
		ReportRangeLastMonth: {d(2026, 9, 1), d(2026, 10, 1)},
		ReportRangeLast7d:    {d(2026, 10, 9), d(2026, 10, 16)},
		ReportRangeLast30d:   {d(2026, 9, 16), d(2026, 10, 16)},
		"":                   {d(2026, 10, 1), d(2026, 10, 16)},
	}
	for key, want := range cases {
		p, err := ParseReportPeriod(key, "", "", loc, now)
		require.NoError(t, err, key)
		require.True(t, want[0].Equal(p.Start), "%s start: %s", key, p.Start)
		require.True(t, want[1].Equal(p.End), "%s end: %s", key, p.End)
	}

	all, err := ParseReportPeriod("all", "", "", loc, now)
	require.NoError(t, err)
	require.Equal(t, ReportRangeAll, all.Range)
	require.True(t, all.Start.Equal(time.Unix(0, 0)))
	require.True(t, all.End.Equal(d(2026, 10, 16)))
	require.Equal(t, "UTC+8", all.TimezoneName())
}

func TestParseReportPeriodCustom(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 10, 15, 13, 0, 0, 0, loc)

	p, err := ParseReportPeriod("custom", "2020-01-01", "2026-09-30", loc, now)
	require.NoError(t, err, "custom ranges have no length cap")
	require.Equal(t, time.Date(2020, 1, 1, 0, 0, 0, 0, loc), p.Start)
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, loc), p.End, "end date is inclusive")

	p, err = ParseReportPeriod("", "2026-09-01", "2026-09-01", loc, now)
	require.NoError(t, err, "start/end without range means custom")
	require.Equal(t, ReportRangeCustom, p.Range)
	require.Equal(t, 24*time.Hour, p.End.Sub(p.Start))

	_, err = ParseReportPeriod("custom", "2026-09-02", "2026-09-01", loc, now)
	require.Equal(t, 400, infraerrors.Code(err))
	_, err = ParseReportPeriod("custom", "2026/09/01", "", loc, now)
	require.Equal(t, 400, infraerrors.Code(err))
	_, err = ParseReportPeriod("forever", "", "", loc, now)
	require.Equal(t, 400, infraerrors.Code(err))
}
