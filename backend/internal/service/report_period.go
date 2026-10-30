package service

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// 财务与结算报表的统计区间。日界一律按站点时区（timezone.Location()，默认 Asia/Shanghai）计算，
// 不使用浏览器时区，保证所有者与管理员看到同一口径。
const (
	ReportRangeToday     = "today"
	ReportRangeYesterday = "yesterday"
	ReportRangeThisMonth = "this_month"
	ReportRangeLastMonth = "last_month"
	ReportRangeLast7d    = "last_7d"
	ReportRangeLast30d   = "last_30d"
	ReportRangeAll       = "all"
	ReportRangeCustom    = "custom"
)

// ReportPeriod 半开区间 [Start, End)。
type ReportPeriod struct {
	Range    string
	Start    time.Time
	End      time.Time
	Location *time.Location
}

// TimezoneName 区间所用时区名。
func (p ReportPeriod) TimezoneName() string {
	if p.Location == nil {
		return "UTC"
	}
	return p.Location.String()
}

// ReportLocation 报表时区：站点时区。
func ReportLocation() *time.Location {
	return timezone.Location()
}

// ParseReportPeriod 解析报表区间：
//   - today / yesterday / this_month（本月 1 日至今天）/ last_month / last_7d、last_30d（含今天在内的最近 7、30 个自然日）；
//   - all：自 Unix 纪元至今天结束；
//   - custom：start、end 为 YYYY-MM-DD（end 含当天），缺省分别为本月 1 日与今天，长度不限；
//   - range 为空时：带 start/end 视为 custom，否则为 this_month。
func ParseReportPeriod(rangeKey, startRaw, endRaw string, loc *time.Location, now time.Time) (ReportPeriod, error) {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	key := strings.ToLower(strings.TrimSpace(rangeKey))
	if key == "" {
		key = ReportRangeThisMonth
		if strings.TrimSpace(startRaw) != "" || strings.TrimSpace(endRaw) != "" {
			key = ReportRangeCustom
		}
	}
	p := ReportPeriod{Range: key, Location: loc}
	switch key {
	case ReportRangeToday:
		p.Start, p.End = today, tomorrow
	case ReportRangeYesterday:
		p.Start, p.End = today.AddDate(0, 0, -1), today
	case ReportRangeThisMonth:
		p.Start, p.End = monthStart, tomorrow
	case ReportRangeLastMonth:
		p.Start, p.End = monthStart.AddDate(0, -1, 0), monthStart
	case ReportRangeLast7d:
		p.Start, p.End = today.AddDate(0, 0, -6), tomorrow
	case ReportRangeLast30d:
		p.Start, p.End = today.AddDate(0, 0, -29), tomorrow
	case ReportRangeAll:
		p.Start, p.End = time.Unix(0, 0).In(loc), tomorrow
	case ReportRangeCustom:
		p.Start, p.End = monthStart, tomorrow
		if s := strings.TrimSpace(startRaw); s != "" {
			t, err := time.ParseInLocation("2006-01-02", s, loc)
			if err != nil {
				return ReportPeriod{}, ErrSettlementInvalidDate
			}
			p.Start = t
		}
		if s := strings.TrimSpace(endRaw); s != "" {
			t, err := time.ParseInLocation("2006-01-02", s, loc)
			if err != nil {
				return ReportPeriod{}, ErrSettlementInvalidDate
			}
			p.End = t.AddDate(0, 0, 1)
		}
		if !p.End.After(p.Start) {
			return ReportPeriod{}, ErrSettlementInvalidRange
		}
	default:
		return ReportPeriod{}, ErrSettlementInvalidRange
	}
	return p, nil
}
