package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 账号健康统计窗口：只开放固定档位，最长 7 天，避免任意时间范围拖垮聚合查询。
var accountHealthRanges = map[string]time.Duration{
	"1h":  time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

const AccountHealthDefaultRange = "24h"

// 账号错误分类；SQL 中的映射见 repository.accountHealthErrorCategorySQL。
const (
	AccountHealthErrorRateLimit   = "rate_limit"
	AccountHealthErrorOverloaded  = "overloaded"
	AccountHealthErrorAuth        = "auth"
	AccountHealthErrorUpstream5xx = "upstream_5xx"
	AccountHealthErrorTimeout     = "timeout"
	AccountHealthErrorOther       = "other"
)

var ErrAccountHealthInvalidRange = infraerrors.BadRequest("ACCOUNT_HEALTH_INVALID_RANGE", "range must be one of 1h, 24h, 7d")

// AccountHealthErrorCounts 按分类统计的账号错误数。
type AccountHealthErrorCounts struct {
	RateLimit   int64 `json:"rate_limit"`
	Overloaded  int64 `json:"overloaded"`
	Auth        int64 `json:"auth"`
	Upstream5xx int64 `json:"upstream_5xx"`
	Timeout     int64 `json:"timeout"`
	Other       int64 `json:"other"`
}

// AccountHealthUsage 来自 usage_logs 的成功请求聚合（单账户）。
type AccountHealthUsage struct {
	RequestsOK  int64
	TokensIn    int64
	TokensOut   int64
	TokensCache int64
	Cost        float64
	// OfficialCost 官方价（total_cost）；AccountCost 账户成本；Earnings 按精确账本的应付（未区分账户是否有所有者）；
	// PendingRequests 账本尚未拆分的请求数。
	OfficialCost    float64
	AccountCost     float64
	Earnings        float64
	PendingRequests int64
	TTFTP50Ms       *int
	TTFTP95Ms       *int
	DurationP50Ms   *int
	LastSuccessAt   *time.Time
}

// AccountHealthErrors 来自 ops_error_logs 的账号侧错误聚合（单账户）。
type AccountHealthErrors struct {
	Total             int64
	ByCategory        AccountHealthErrorCounts
	LastErrorAt       *time.Time
	LastErrorCategory string
	LastErrorStatus   *int
}

// AccountHealthRepository 账号健康所需的只读查询。
// ListAccountHealthAccounts 必须在所有者请求中只返回本人账户；ownerUserID 仅供管理员筛选。
type AccountHealthRepository interface {
	ListAccountHealthAccounts(ctx context.Context, ownerUserID *int64) ([]Account, error)
	AggregateAccountHealthUsage(ctx context.Context, accountIDs []int64, start, end time.Time) (map[int64]*AccountHealthUsage, error)
	AggregateAccountHealthErrors(ctx context.Context, accountIDs []int64, start, end time.Time) (map[int64]*AccountHealthErrors, error)
	// ListAccountOwnerLabels 所有者的邮箱与用户名（仅管理员请求使用）。
	ListAccountOwnerLabels(ctx context.Context, ownerIDs []int64) (map[int64]OwnerIdentity, error)
}

// AccountHealthItem 单个账户的健康概况。只含账户自身状态与聚合指标，
// 不含用户、API Key、IP 或请求内容。
type AccountHealthItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Schedulable bool   `json:"schedulable"`

	RateLimitResetAt       *time.Time `json:"rate_limit_reset_at"`
	OverloadUntil          *time.Time `json:"overload_until"`
	TempUnschedulableUntil *time.Time `json:"temp_unschedulable_until"`
	RateLimited            bool       `json:"rate_limited"`
	Overloaded             bool       `json:"overloaded"`
	TempUnschedulable      bool       `json:"temp_unschedulable"`
	// ErrorMessage 为账户上记录的上游错误文本，不含用户数据。
	ErrorMessage string `json:"error_message"`

	RequestsOK  int64                    `json:"requests_ok"`
	ErrorsTotal int64                    `json:"errors_total"`
	Errors      AccountHealthErrorCounts `json:"errors"`
	ErrorRate   float64                  `json:"error_rate"`

	TTFTP50Ms     *int `json:"ttft_p50_ms"`
	TTFTP95Ms     *int `json:"ttft_p95_ms"`
	DurationP50Ms *int `json:"duration_p50_ms"`

	TokensIn    int64 `json:"tokens_in"`
	TokensOut   int64 `json:"tokens_out"`
	TokensCache int64 `json:"tokens_cache"`
	// Cost 用户计费（actual_cost 之和），只给管理员；所有者响应中不出现。
	Cost *float64 `json:"cost,omitempty"`
	// OfficialCost 官方价；Earnings 应付所有者（精确账本，账户无所有者时为 0）；PendingRequests 尚未拆分的请求数。
	OfficialCost    float64 `json:"official_cost"`
	Earnings        float64 `json:"earnings"`
	PendingRequests int64   `json:"pending_requests"`
	// AccountCost 账户成本（官方价 × 账户倍率），只给管理员。
	AccountCost *float64 `json:"account_cost,omitempty"`
	// OwnerUserID 账户所有者；OwnerEmail/OwnerUsername 只给管理员。
	OwnerUserID   *int64 `json:"owner_user_id"`
	OwnerEmail    string `json:"owner_email,omitempty"`
	OwnerUsername string `json:"owner_username,omitempty"`

	LastSuccessAt     *time.Time `json:"last_success_at"`
	LastErrorAt       *time.Time `json:"last_error_at"`
	LastErrorCategory string     `json:"last_error_category"`
	LastErrorStatus   *int       `json:"last_error_status"`
}

type AccountHealthReport struct {
	Range     string              `json:"range"`
	StartTime time.Time           `json:"start_time"`
	EndTime   time.Time           `json:"end_time"`
	Items     []AccountHealthItem `json:"items"`
}

type AccountHealthService struct {
	repo AccountHealthRepository
	now  func() time.Time
}

func NewAccountHealthService(repo AccountHealthRepository) *AccountHealthService {
	return &AccountHealthService{repo: repo, now: time.Now}
}

// ParseAccountHealthRange 校验统计窗口；空值取默认 24h。
func ParseAccountHealthRange(raw string) (string, time.Duration, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		key = AccountHealthDefaultRange
	}
	d, ok := accountHealthRanges[key]
	if !ok {
		return "", 0, ErrAccountHealthInvalidRange
	}
	return key, d, nil
}

// AccountHealthErrorRate = 错误数 / (成功数 + 错误数)；无流量时为 0。
func AccountHealthErrorRate(ok, errs int64) float64 {
	if ok+errs <= 0 {
		return 0
	}
	return float64(errs) / float64(ok+errs)
}

// GetAccountHealth 汇总范围内账户的健康指标；成功与错误各一次分组聚合，不按账户逐个查询。
// 所有者请求一律限定为本人账户，忽略传入的 ownerUserID。
func (s *AccountHealthService) GetAccountHealth(ctx context.Context, rawRange string, ownerUserID *int64) (*AccountHealthReport, error) {
	key, window, err := ParseAccountHealthRange(rawRange)
	if err != nil {
		return nil, err
	}
	ownerRequest := adminaccess.OwnerUserID(ctx) > 0
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		ownerUserID = &id
	}
	end := s.now().UTC()
	start := end.Add(-window)

	accounts, err := s.repo.ListAccountHealthAccounts(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	report := &AccountHealthReport{Range: key, StartTime: start, EndTime: end, Items: make([]AccountHealthItem, 0, len(accounts))}
	if len(accounts) == 0 {
		return report, nil
	}
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	usage, err := s.repo.AggregateAccountHealthUsage(ctx, ids, start, end)
	if err != nil {
		return nil, err
	}
	errs, err := s.repo.AggregateAccountHealthErrors(ctx, ids, start, end)
	if err != nil {
		return nil, err
	}
	labels := map[int64]OwnerIdentity{}
	if !ownerRequest {
		seen := map[int64]struct{}{}
		ownerIDs := []int64{}
		for i := range accounts {
			if id := accounts[i].OwnerUserID; id != nil {
				if _, ok := seen[*id]; !ok {
					seen[*id] = struct{}{}
					ownerIDs = append(ownerIDs, *id)
				}
			}
		}
		if len(ownerIDs) > 0 {
			if labels, err = s.repo.ListAccountOwnerLabels(ctx, ownerIDs); err != nil {
				return nil, err
			}
		}
	}

	for i := range accounts {
		a := &accounts[i]
		item := AccountHealthItem{
			ID:                     a.ID,
			Name:                   a.Name,
			Platform:               a.Platform,
			Type:                   a.Type,
			Status:                 a.Status,
			Schedulable:            a.Schedulable,
			RateLimitResetAt:       a.RateLimitResetAt,
			OverloadUntil:          a.OverloadUntil,
			TempUnschedulableUntil: a.TempUnschedulableUntil,
			RateLimited:            a.RateLimitResetAt != nil && a.RateLimitResetAt.After(end),
			Overloaded:             a.OverloadUntil != nil && a.OverloadUntil.After(end),
			TempUnschedulable:      a.TempUnschedulableUntil != nil && a.TempUnschedulableUntil.After(end),
			ErrorMessage:           a.ErrorMessage,
			OwnerUserID:            a.OwnerUserID,
		}
		if !ownerRequest {
			zero, zeroAccount := 0.0, 0.0
			item.Cost, item.AccountCost = &zero, &zeroAccount
			if a.OwnerUserID != nil {
				l := labels[*a.OwnerUserID]
				item.OwnerEmail, item.OwnerUsername = l.Email, l.Username
			}
		}
		if u := usage[a.ID]; u != nil {
			item.RequestsOK = u.RequestsOK
			item.TokensIn, item.TokensOut, item.TokensCache = u.TokensIn, u.TokensOut, u.TokensCache
			item.OfficialCost = u.OfficialCost
			item.PendingRequests = u.PendingRequests
			if a.OwnerUserID != nil {
				item.Earnings = u.Earnings
			}
			if !ownerRequest {
				cost, accountCost := u.Cost, u.AccountCost
				item.Cost, item.AccountCost = &cost, &accountCost
			}
			item.TTFTP50Ms, item.TTFTP95Ms, item.DurationP50Ms = u.TTFTP50Ms, u.TTFTP95Ms, u.DurationP50Ms
			item.LastSuccessAt = u.LastSuccessAt
		}
		if e := errs[a.ID]; e != nil {
			item.ErrorsTotal = e.Total
			item.Errors = e.ByCategory
			item.LastErrorAt = e.LastErrorAt
			item.LastErrorCategory = e.LastErrorCategory
			item.LastErrorStatus = e.LastErrorStatus
		}
		item.ErrorRate = AccountHealthErrorRate(item.RequestsOK, item.ErrorsTotal)
		report.Items = append(report.Items, item)
	}
	return report, nil
}
