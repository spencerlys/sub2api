package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// AI 账号所有者结算：
//   - 账户成本 = COALESCE(account_stats_cost, total_cost) × COALESCE(account_rate_multiplier, 1)（官方价 × 账户倍率）；
//   - 所有者收益 = Σ 本人账户上的账户成本 × 该请求用户的实付占比（PaidShare，按截至当前的全部正向入账计算），
//     赠送额度不为所有者产生收益，线下实收视同实付；
//   - 未结 = 历史累计收益 − 已打款（未删除的 owner_payouts）。
//
// 报表只读；所有者只能读取本人汇总（GetMySettlement），且不含倍率、实付占比与用户身份。

const settlementMaxRangeDays = 366

var (
	ErrSettlementInvalidDate  = infraerrors.BadRequest("SETTLEMENT_INVALID_DATE", "start and end must be YYYY-MM-DD")
	ErrSettlementInvalidRange = infraerrors.BadRequest("SETTLEMENT_INVALID_RANGE", "end must not be before start and the range must not exceed 366 days")
	ErrOwnerPayoutNotFound    = infraerrors.NotFound("OWNER_PAYOUT_NOT_FOUND", "owner payout not found")
	ErrOwnerPayoutInvalid     = infraerrors.BadRequest("OWNER_PAYOUT_INVALID", "amount must be greater than 0")
	ErrOwnerPayoutNotOwner    = infraerrors.BadRequest("OWNER_PAYOUT_NOT_OWNER", "payee is not an AI account owner")
	ErrBalanceSourceInvalid   = infraerrors.BadRequest("BALANCE_SOURCE_INVALID", "source must be one of gift, offline, correction")
	ErrBalanceRecordNotAdjust = infraerrors.BadRequest("BALANCE_RECORD_NOT_ADJUSTMENT", "only admin balance adjustments have a source")
	ErrBalanceRecordNotFound  = infraerrors.NotFound("BALANCE_RECORD_NOT_FOUND", "balance record not found")
	ErrSettlementOwnerOnly    = infraerrors.Forbidden("SETTLEMENT_OWNER_ONLY", "only AI account owners have a personal settlement")
)

// OwnerIdentity 结算对象（role=owner，或名下有账户/打款记录的用户）。
type OwnerIdentity struct {
	UserID   int64
	Email    string
	Username string
}

// OwnerUserUsage 某所有者账户上某用户的用量聚合：期间值与历史累计账户成本。
type OwnerUserUsage struct {
	OwnerUserID        int64
	UserID             int64
	Requests           int64
	OfficialCost       float64
	UserBilled         float64
	AccountCost        float64
	AccountCostAllTime float64
}

// UserCreditRow 某用户的一类入账汇总。
type UserCreditRow struct {
	UserID int64
	Credit BalanceCredit
}

// FinanceCreditBucket 期间入账按种类与来源的汇总；Positive/Negative 分别为正负金额之和。
type FinanceCreditBucket struct {
	Kind     string
	Source   string
	Count    int64
	Positive float64
	NegCount int64
	Negative float64
	Bonus    float64
}

type FinanceCurrencyAmount struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

// FinancePeriodRaw 财务汇总的原始聚合，分类规则在服务层统一应用。
type FinancePeriodRaw struct {
	OnlineOrders int64
	PayAmounts   []FinanceCurrencyAmount
	Credits      []FinanceCreditBucket
}

type OwnerPayout struct {
	ID          int64     `json:"id"`
	OwnerUserID int64     `json:"owner_user_id"`
	OwnerEmail  string    `json:"owner_email"`
	Amount      float64   `json:"amount"`
	PaidAt      time.Time `json:"paid_at"`
	Notes       string    `json:"notes"`
	CreatedBy   *int64    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type OwnerPayoutFilter struct {
	OwnerUserID *int64
	Start       *time.Time
	End         *time.Time
}

type OwnerPayoutInput struct {
	OwnerUserID int64
	Amount      float64
	PaidAt      *time.Time
	Notes       string
}

// OwnerSettlementRepository 结算与财务报表所需的数据访问。
type OwnerSettlementRepository interface {
	// ListSettlementOwners ownerUserID 非空时只返回该用户（且其须为结算对象）。
	ListSettlementOwners(ctx context.Context, ownerUserID *int64) ([]OwnerIdentity, error)
	AggregateOwnerUserUsage(ctx context.Context, start, end time.Time, ownerUserID *int64) ([]OwnerUserUsage, error)
	ListUserCredits(ctx context.Context, userIDs []int64) ([]UserCreditRow, error)
	SumOwnerPayouts(ctx context.Context, ownerUserID *int64) (map[int64]float64, error)
	ListOwnerPayouts(ctx context.Context, filter OwnerPayoutFilter, params pagination.PaginationParams) ([]OwnerPayout, int64, error)
	CreateOwnerPayout(ctx context.Context, p *OwnerPayout) error
	DeleteOwnerPayout(ctx context.Context, id int64) error
	AggregateFinancePeriod(ctx context.Context, start, end time.Time) (*FinancePeriodRaw, error)
	GetBalanceRecord(ctx context.Context, id int64) (*RedeemCode, error)
	UpdateBalanceRecordNotes(ctx context.Context, id int64, notes string) error
}

// OwnerSettlementAmounts 结算金额（USD）。
type OwnerSettlementAmounts struct {
	Requests        int64   `json:"requests"`
	OfficialCost    float64 `json:"official_cost"`
	UserBilled      float64 `json:"user_billed"`
	AccountCost     float64 `json:"account_cost"`
	Earnings        float64 `json:"earnings"`
	EarningsAllTime float64 `json:"earnings_all_time"`
	PaidTotal       float64 `json:"paid_total"`
	Unpaid          float64 `json:"unpaid"`
}

func (a *OwnerSettlementAmounts) add(b OwnerSettlementAmounts) {
	a.Requests += b.Requests
	a.OfficialCost += b.OfficialCost
	a.UserBilled += b.UserBilled
	a.AccountCost += b.AccountCost
	a.Earnings += b.Earnings
	a.EarningsAllTime += b.EarningsAllTime
	a.PaidTotal += b.PaidTotal
	a.Unpaid += b.Unpaid
}

// OwnerSettlementRow 管理员视角的单个所有者结算行。
type OwnerSettlementRow struct {
	OwnerUserID int64  `json:"owner_user_id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	OwnerSettlementAmounts
}

type OwnerSettlementReport struct {
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Items     []OwnerSettlementRow   `json:"items"`
	Totals    OwnerSettlementAmounts `json:"totals"`
}

// OwnerSettlementSelf 所有者本人视角：只含官方价成本与收益，不含用户计费、账户倍率与用户身份。
type OwnerSettlementSelf struct {
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Requests        int64     `json:"requests"`
	OfficialCost    float64   `json:"official_cost"`
	Earnings        float64   `json:"earnings"`
	EarningsAllTime float64   `json:"earnings_all_time"`
	PaidTotal       float64   `json:"paid_total"`
	Unpaid          float64   `json:"unpaid"`
}

type FinanceOnlineSummary struct {
	Orders      int64                   `json:"orders"`
	PayAmounts  []FinanceCurrencyAmount `json:"pay_amounts"`
	CreditedUSD float64                 `json:"credited_usd"`
	BonusUSD    float64                 `json:"bonus_usd"`
}

type FinanceCountUSD struct {
	Count int64   `json:"count"`
	USD   float64 `json:"usd"`
}

type FinanceGiftSummary struct {
	TotalUSD        float64 `json:"total_usd"`
	AdminGiftUSD    float64 `json:"admin_gift_usd"`
	PaymentBonusUSD float64 `json:"payment_bonus_usd"`
	PromoUSD        float64 `json:"promo_usd"`
	RedeemCodeUSD   float64 `json:"redeem_code_usd"`
	AffiliateUSD    float64 `json:"affiliate_usd"`
}

type FinanceCorrectionSummary struct {
	Count  int64   `json:"count"`
	NetUSD float64 `json:"net_usd"`
}

// FinanceSummary 期间入账按来源汇总（入账时间口径：redeem_codes.used_at / promo_code_usages.used_at / 返利转入时间）。
type FinanceSummary struct {
	StartTime   time.Time                `json:"start_time"`
	EndTime     time.Time                `json:"end_time"`
	Online      FinanceOnlineSummary     `json:"online"`
	Offline     FinanceCountUSD          `json:"offline"`
	Gift        FinanceGiftSummary       `json:"gift"`
	Corrections FinanceCorrectionSummary `json:"corrections"`
	// Deductions 非更正来源的管理员扣减（负数调整），不计入收入，单独列示便于对账。
	Deductions FinanceCountUSD `json:"deductions"`
}

// BalanceRecordSource 余额调整记录的来源更新结果。
type BalanceRecordSource struct {
	ID     int64  `json:"id"`
	Source string `json:"balance_source"`
	Notes  string `json:"notes"`
}

type OwnerSettlementService struct {
	repo OwnerSettlementRepository
	now  func() time.Time
}

func NewOwnerSettlementService(repo OwnerSettlementRepository) *OwnerSettlementService {
	return &OwnerSettlementService{repo: repo, now: time.Now}
}

// SettlementLocation 按请求时区解析；空或非法时退回服务器时区。
func SettlementLocation(userTZ string) *time.Location {
	if userTZ = strings.TrimSpace(userTZ); userTZ != "" {
		if loc, err := time.LoadLocation(userTZ); err == nil {
			return loc
		}
	}
	return timezone.Location()
}

// ParseSettlementPeriod 解析 YYYY-MM-DD 日期区间（end 含当天），返回 [start, end)。
// 缺省为本月 1 日至今天；最长 366 天。
func ParseSettlementPeriod(startRaw, endRaw string, loc *time.Location, now time.Time) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if s := strings.TrimSpace(startRaw); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, loc)
		if err != nil {
			return time.Time{}, time.Time{}, ErrSettlementInvalidDate
		}
		start = t
	}
	if s := strings.TrimSpace(endRaw); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, loc)
		if err != nil {
			return time.Time{}, time.Time{}, ErrSettlementInvalidDate
		}
		endDay = t
	}
	end := endDay.AddDate(0, 0, 1)
	if !end.After(start) || end.After(start.AddDate(0, 0, settlementMaxRangeDays)) {
		return time.Time{}, time.Time{}, ErrSettlementInvalidRange
	}
	return start, end, nil
}

// GetOwnerSettlements 管理员结算报表：每个所有者一行，加合计行。所有者上下文强制限定为本人。
func (s *OwnerSettlementService) GetOwnerSettlements(ctx context.Context, start, end time.Time, ownerUserID *int64) (*OwnerSettlementReport, error) {
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		ownerUserID = &id
	}
	owners, err := s.repo.ListSettlementOwners(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.AggregateOwnerUserUsage(ctx, start, end, ownerUserID)
	if err != nil {
		return nil, err
	}
	payouts, err := s.repo.SumOwnerPayouts(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}

	userSet := make(map[int64]struct{}, len(usage))
	userIDs := make([]int64, 0, len(usage))
	for _, u := range usage {
		if _, ok := userSet[u.UserID]; !ok {
			userSet[u.UserID] = struct{}{}
			userIDs = append(userIDs, u.UserID)
		}
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	shares := make(map[int64]float64, len(userIDs))
	if len(userIDs) > 0 {
		credits, err := s.repo.ListUserCredits(ctx, userIDs)
		if err != nil {
			return nil, err
		}
		totals := make(map[int64]*UserCreditTotals, len(userIDs))
		for _, c := range credits {
			t := totals[c.UserID]
			if t == nil {
				t = &UserCreditTotals{}
				totals[c.UserID] = t
			}
			t.Add(c.Credit)
		}
		for id, t := range totals {
			shares[id] = t.PaidShare()
		}
	}

	rows := make(map[int64]*OwnerSettlementRow, len(owners))
	order := make([]int64, 0, len(owners))
	ensure := func(id int64) *OwnerSettlementRow {
		if r := rows[id]; r != nil {
			return r
		}
		r := &OwnerSettlementRow{OwnerUserID: id}
		rows[id] = r
		order = append(order, id)
		return r
	}
	for _, o := range owners {
		r := ensure(o.UserID)
		r.Email, r.Username = o.Email, o.Username
	}
	for _, u := range usage {
		r := ensure(u.OwnerUserID)
		share := shares[u.UserID]
		r.Requests += u.Requests
		r.OfficialCost += u.OfficialCost
		r.UserBilled += u.UserBilled
		r.AccountCost += u.AccountCost
		r.Earnings += u.AccountCost * share
		r.EarningsAllTime += u.AccountCostAllTime * share
	}
	for id, paid := range payouts {
		ensure(id).PaidTotal = paid
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	report := &OwnerSettlementReport{StartTime: start, EndTime: end, Items: make([]OwnerSettlementRow, 0, len(order))}
	for _, id := range order {
		r := rows[id]
		r.Unpaid = r.EarningsAllTime - r.PaidTotal
		report.Items = append(report.Items, *r)
		report.Totals.add(r.OwnerSettlementAmounts)
	}
	return report, nil
}

// GetMySettlement 所有者本人汇总。
func (s *OwnerSettlementService) GetMySettlement(ctx context.Context, ownerUserID int64, start, end time.Time) (*OwnerSettlementSelf, error) {
	if ownerUserID <= 0 {
		return nil, ErrSettlementOwnerOnly
	}
	report, err := s.GetOwnerSettlements(ctx, start, end, &ownerUserID)
	if err != nil {
		return nil, err
	}
	me := &OwnerSettlementSelf{StartTime: start, EndTime: end}
	for _, r := range report.Items {
		if r.OwnerUserID != ownerUserID {
			continue
		}
		me.Requests = r.Requests
		me.OfficialCost = r.OfficialCost
		me.Earnings = r.Earnings
		me.EarningsAllTime = r.EarningsAllTime
		me.PaidTotal = r.PaidTotal
		me.Unpaid = r.Unpaid
	}
	return me, nil
}

func (s *OwnerSettlementService) ListOwnerPayouts(ctx context.Context, filter OwnerPayoutFilter, params pagination.PaginationParams) ([]OwnerPayout, int64, error) {
	return s.repo.ListOwnerPayouts(ctx, filter, params)
}

// CreateOwnerPayout 记录一笔打款；收款人必须是结算对象。createdBy 为 0 表示未知（如管理密钥）。
func (s *OwnerSettlementService) CreateOwnerPayout(ctx context.Context, in OwnerPayoutInput, createdBy int64) (*OwnerPayout, error) {
	if in.Amount <= 0 || in.OwnerUserID <= 0 {
		return nil, ErrOwnerPayoutInvalid
	}
	owners, err := s.repo.ListSettlementOwners(ctx, &in.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if len(owners) == 0 {
		return nil, ErrOwnerPayoutNotOwner
	}
	p := &OwnerPayout{
		OwnerUserID: in.OwnerUserID,
		OwnerEmail:  owners[0].Email,
		Amount:      in.Amount,
		PaidAt:      s.now(),
		Notes:       strings.TrimSpace(in.Notes),
	}
	if in.PaidAt != nil && !in.PaidAt.IsZero() {
		p.PaidAt = *in.PaidAt
	}
	if createdBy > 0 {
		p.CreatedBy = &createdBy
	}
	if err := s.repo.CreateOwnerPayout(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *OwnerSettlementService) DeleteOwnerPayout(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrOwnerPayoutNotFound
	}
	return s.repo.DeleteOwnerPayout(ctx, id)
}

// GetFinanceSummary 期间入账按来源汇总。
func (s *OwnerSettlementService) GetFinanceSummary(ctx context.Context, start, end time.Time) (*FinanceSummary, error) {
	raw, err := s.repo.AggregateFinancePeriod(ctx, start, end)
	if err != nil {
		return nil, err
	}
	out := &FinanceSummary{StartTime: start, EndTime: end}
	out.Online.PayAmounts = []FinanceCurrencyAmount{}
	if raw == nil {
		return out, nil
	}
	out.Online.Orders = raw.OnlineOrders
	if raw.PayAmounts != nil {
		out.Online.PayAmounts = raw.PayAmounts
	}
	for _, b := range raw.Credits {
		switch b.Kind {
		case CreditKindPayment:
			bonus := b.Bonus
			if bonus > b.Positive {
				bonus = b.Positive
			}
			out.Online.CreditedUSD += b.Positive - bonus
			out.Online.BonusUSD += bonus
			out.Gift.PaymentBonusUSD += bonus
		case CreditKindAdmin:
			src, ok := NormalizeBalanceSource(b.Source)
			if !ok {
				src = BalanceSourceGift
			}
			switch src {
			case BalanceSourceCorrection:
				out.Corrections.Count += b.Count + b.NegCount
				out.Corrections.NetUSD += b.Positive + b.Negative
				continue
			case BalanceSourceOffline:
				out.Offline.Count += b.Count
				out.Offline.USD += b.Positive
			default:
				out.Gift.AdminGiftUSD += b.Positive
			}
			out.Deductions.Count += b.NegCount
			out.Deductions.USD += b.Negative
		case CreditKindPromo:
			out.Gift.PromoUSD += b.Positive
		case CreditKindRedeemCode:
			out.Gift.RedeemCodeUSD += b.Positive
		case CreditKindAffiliate:
			out.Gift.AffiliateUSD += b.Positive
		}
	}
	g := &out.Gift
	g.TotalUSD = g.AdminGiftUSD + g.PaymentBonusUSD + g.PromoUSD + g.RedeemCodeUSD + g.AffiliateUSD
	return out, nil
}

// UpdateBalanceRecordSource 改写某用户一条管理员余额调整记录的来源前缀，其余备注保留。
func (s *OwnerSettlementService) UpdateBalanceRecordSource(ctx context.Context, userID, recordID int64, source string) (*BalanceRecordSource, error) {
	src, ok := NormalizeBalanceSource(source)
	if !ok {
		return nil, ErrBalanceSourceInvalid
	}
	rec, err := s.repo.GetBalanceRecord(ctx, recordID)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			return nil, ErrBalanceRecordNotFound
		}
		return nil, err
	}
	if rec == nil || rec.UsedBy == nil || *rec.UsedBy != userID {
		return nil, ErrBalanceRecordNotFound
	}
	if rec.Type != AdjustmentTypeAdminBalance {
		return nil, ErrBalanceRecordNotAdjust
	}
	notes := FormatBalanceSourceNotes(src, rec.Notes)
	if err := s.repo.UpdateBalanceRecordNotes(ctx, rec.ID, notes); err != nil {
		return nil, err
	}
	_, rest, _ := ParseBalanceSourceNotes(notes)
	return &BalanceRecordSource{ID: rec.ID, Source: src, Notes: rest}, nil
}
