package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// AI 账号所有者结算（精确账本口径，见 usage_cost_ledger.go）：
//   - 账户成本 = COALESCE(account_stats_cost, total_cost) × COALESCE(account_rate_multiplier, 1)（官方价 × 账户倍率）；
//   - 每次请求的所有者收益 = 账户成本 × paid_cost / actual_cost（actual_cost > 0 时，否则为 0），
//     paid_cost 为该请求由实付余额（在线支付、线下实收）承担的部分；赠送部分对应的账户成本只作参考，不计入应付；
//   - 账本任务尚未拆分的请求计为待结算（pending），不估算、不计入收益；
//   - 未结 = 历史累计收益 − 已打款（未删除的 owner_payouts）。
//
// 报表只读；所有者只能读取本人汇总（GetMySettlement），且不含用户计费、实付/赠送拆分、倍率与用户身份。

var (
	ErrSettlementInvalidDate  = infraerrors.BadRequest("SETTLEMENT_INVALID_DATE", "start and end must be YYYY-MM-DD")
	ErrSettlementInvalidRange = infraerrors.BadRequest("SETTLEMENT_INVALID_RANGE", "range must be one of today, yesterday, this_month, last_month, last_7d, last_30d, all, custom; custom end must not be before start")
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

// OwnerSettlementUsage 某所有者账户上的用量聚合（按账户当前所有者归属）：期间值与历史累计收益。
type OwnerSettlementUsage struct {
	OwnerUserID        int64
	Requests           int64
	OfficialCost       float64
	UserBilled         float64
	PaidCost           float64
	GiftCost           float64
	AccountCost        float64
	Earnings           float64
	GiftAccountCost    float64
	PendingRequests    int64
	PendingAccountCost float64
	EarningsAllTime    float64
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

// FinanceConsumption 期间全部用量按精确账本拆分的消耗。
type FinanceConsumption struct {
	Requests          int64   `json:"requests"`
	UserBilled        float64 `json:"user_billed"`
	PaidCost          float64 `json:"paid_cost"`
	GiftCost          float64 `json:"gift_cost"`
	PendingRequests   int64   `json:"pending_requests"`
	PendingUserBilled float64 `json:"pending_user_billed"`
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
	// AggregateOwnerSettlement 按所有者聚合 [start,end) 内的用量与精确拆分，另含历史累计收益。
	AggregateOwnerSettlement(ctx context.Context, start, end time.Time, ownerUserID *int64) ([]OwnerSettlementUsage, error)
	// CountActiveOwnerAccounts 各所有者当前启用且可调度的账户数。
	CountActiveOwnerAccounts(ctx context.Context, ownerUserID *int64) (map[int64]int64, error)
	SumOwnerPayouts(ctx context.Context, ownerUserID *int64) (map[int64]float64, error)
	ListOwnerPayouts(ctx context.Context, filter OwnerPayoutFilter, params pagination.PaginationParams) ([]OwnerPayout, int64, error)
	CreateOwnerPayout(ctx context.Context, p *OwnerPayout) error
	DeleteOwnerPayout(ctx context.Context, id int64) error
	AggregateFinancePeriod(ctx context.Context, start, end time.Time) (*FinancePeriodRaw, error)
	AggregateFinanceConsumption(ctx context.Context, start, end time.Time) (*FinanceConsumption, error)
	GetBalanceRecord(ctx context.Context, id int64) (*RedeemCode, error)
	UpdateBalanceRecordNotes(ctx context.Context, id int64, notes string) error
}

// OwnerSettlementAmounts 结算金额（USD）。
type OwnerSettlementAmounts struct {
	ActiveAccounts int64   `json:"active_accounts"`
	Requests       int64   `json:"requests"`
	OfficialCost   float64 `json:"official_cost"`
	// UserBilled 应收用户（actual_cost），拆分为 PaidCost（实付）+ GiftCost（赠送）+ 待结算部分。
	UserBilled  float64 `json:"user_billed"`
	PaidCost    float64 `json:"paid_cost"`
	GiftCost    float64 `json:"gift_cost"`
	AccountCost float64 `json:"account_cost"`
	// Earnings 应付所有者；GiftAccountCost 为赠送资助的账户成本，仅供参考，不计入应付。
	Earnings           float64 `json:"earnings"`
	GiftAccountCost    float64 `json:"gift_account_cost"`
	PendingRequests    int64   `json:"pending_requests"`
	PendingAccountCost float64 `json:"pending_account_cost"`
	EarningsAllTime    float64 `json:"earnings_all_time"`
	PaidTotal          float64 `json:"paid_total"`
	Unpaid             float64 `json:"unpaid"`
}

func (a *OwnerSettlementAmounts) add(b OwnerSettlementAmounts) {
	a.ActiveAccounts += b.ActiveAccounts
	a.Requests += b.Requests
	a.OfficialCost += b.OfficialCost
	a.UserBilled += b.UserBilled
	a.PaidCost += b.PaidCost
	a.GiftCost += b.GiftCost
	a.AccountCost += b.AccountCost
	a.Earnings += b.Earnings
	a.GiftAccountCost += b.GiftAccountCost
	a.PendingRequests += b.PendingRequests
	a.PendingAccountCost += b.PendingAccountCost
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

// ReportWindow 报表区间回显。
type ReportWindow struct {
	Range     string    `json:"range"`
	Timezone  string    `json:"timezone"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func reportWindow(p ReportPeriod) ReportWindow {
	return ReportWindow{Range: p.Range, Timezone: p.TimezoneName(), StartTime: p.Start, EndTime: p.End}
}

type OwnerSettlementReport struct {
	ReportWindow
	Ledger *CostLedgerStatus      `json:"ledger"`
	Items  []OwnerSettlementRow   `json:"items"`
	Totals OwnerSettlementAmounts `json:"totals"`
}

// OwnerSettlementSelf 所有者本人视角：只含官方价成本与收益，不含用户计费、实付/赠送拆分、账户倍率与用户身份。
type OwnerSettlementSelf struct {
	ReportWindow
	Requests        int64             `json:"requests"`
	OfficialCost    float64           `json:"official_cost"`
	Earnings        float64           `json:"earnings"`
	PendingRequests int64             `json:"pending_requests"`
	EarningsAllTime float64           `json:"earnings_all_time"`
	PaidTotal       float64           `json:"paid_total"`
	Unpaid          float64           `json:"unpaid"`
	Ledger          *CostLedgerStatus `json:"ledger"`
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

// FinanceSummary 期间入账按来源汇总（入账时间口径：redeem_codes.used_at / promo_code_usages.used_at / 返利转入时间），
// 以及期间用量按精确账本拆分的消耗与账本进度。
type FinanceSummary struct {
	ReportWindow
	Online      FinanceOnlineSummary     `json:"online"`
	Offline     FinanceCountUSD          `json:"offline"`
	Gift        FinanceGiftSummary       `json:"gift"`
	Corrections FinanceCorrectionSummary `json:"corrections"`
	// Deductions 非更正来源的管理员扣减（负数调整），不计入收入，单独列示便于对账。
	Deductions  FinanceCountUSD    `json:"deductions"`
	Consumption FinanceConsumption `json:"consumption"`
	Ledger      *CostLedgerStatus  `json:"ledger"`
}

// BalanceRecordSource 余额调整记录的来源更新结果。
type BalanceRecordSource struct {
	ID     int64  `json:"id"`
	Source string `json:"balance_source"`
	Notes  string `json:"notes"`
}

type OwnerSettlementService struct {
	repo   OwnerSettlementRepository
	ledger *UsageCostLedgerService
	now    func() time.Time
}

func NewOwnerSettlementService(repo OwnerSettlementRepository, ledger *UsageCostLedgerService) *OwnerSettlementService {
	return &OwnerSettlementService{repo: repo, ledger: ledger, now: time.Now}
}

// ParsePeriod 按站点时区解析报表区间。
func (s *OwnerSettlementService) ParsePeriod(rangeKey, startRaw, endRaw string) (ReportPeriod, error) {
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	return ParseReportPeriod(rangeKey, startRaw, endRaw, ReportLocation(), now())
}

func (s *OwnerSettlementService) ledgerStatus(ctx context.Context) (*CostLedgerStatus, error) {
	if s.ledger == nil {
		return &CostLedgerStatus{LagSeconds: -1}, nil
	}
	return s.ledger.Status(ctx)
}

// GetOwnerSettlements 管理员结算报表：每个所有者一行，加合计行。所有者上下文强制限定为本人。
func (s *OwnerSettlementService) GetOwnerSettlements(ctx context.Context, period ReportPeriod, ownerUserID *int64) (*OwnerSettlementReport, error) {
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		ownerUserID = &id
	}
	owners, err := s.repo.ListSettlementOwners(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.AggregateOwnerSettlement(ctx, period.Start, period.End, ownerUserID)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.CountActiveOwnerAccounts(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	payouts, err := s.repo.SumOwnerPayouts(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	ledger, err := s.ledgerStatus(ctx)
	if err != nil {
		return nil, err
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
		r.Requests += u.Requests
		r.OfficialCost += u.OfficialCost
		r.UserBilled += u.UserBilled
		r.PaidCost += u.PaidCost
		r.GiftCost += u.GiftCost
		r.AccountCost += u.AccountCost
		r.Earnings += u.Earnings
		r.GiftAccountCost += u.GiftAccountCost
		r.PendingRequests += u.PendingRequests
		r.PendingAccountCost += u.PendingAccountCost
		r.EarningsAllTime += u.EarningsAllTime
	}
	for id, n := range active {
		ensure(id).ActiveAccounts = n
	}
	for id, paid := range payouts {
		ensure(id).PaidTotal = paid
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	report := &OwnerSettlementReport{ReportWindow: reportWindow(period), Ledger: ledger, Items: make([]OwnerSettlementRow, 0, len(order))}
	for _, id := range order {
		r := rows[id]
		r.Unpaid = r.EarningsAllTime - r.PaidTotal
		report.Items = append(report.Items, *r)
		report.Totals.add(r.OwnerSettlementAmounts)
	}
	return report, nil
}

// GetMySettlement 所有者本人汇总。
func (s *OwnerSettlementService) GetMySettlement(ctx context.Context, ownerUserID int64, period ReportPeriod) (*OwnerSettlementSelf, error) {
	if ownerUserID <= 0 {
		return nil, ErrSettlementOwnerOnly
	}
	report, err := s.GetOwnerSettlements(ctx, period, &ownerUserID)
	if err != nil {
		return nil, err
	}
	me := &OwnerSettlementSelf{ReportWindow: report.ReportWindow}
	if report.Ledger != nil {
		// 错误文本可能含用户 ID，所有者只看进度。
		l := *report.Ledger
		l.LastError = ""
		me.Ledger = &l
	}
	for _, r := range report.Items {
		if r.OwnerUserID != ownerUserID {
			continue
		}
		me.Requests = r.Requests
		me.OfficialCost = r.OfficialCost
		me.Earnings = r.Earnings
		me.PendingRequests = r.PendingRequests
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

// GetFinanceSummary 期间入账按来源汇总，另含期间消耗的实付/赠送拆分与账本进度。
func (s *OwnerSettlementService) GetFinanceSummary(ctx context.Context, period ReportPeriod) (*FinanceSummary, error) {
	raw, err := s.repo.AggregateFinancePeriod(ctx, period.Start, period.End)
	if err != nil {
		return nil, err
	}
	consumption, err := s.repo.AggregateFinanceConsumption(ctx, period.Start, period.End)
	if err != nil {
		return nil, err
	}
	ledger, err := s.ledgerStatus(ctx)
	if err != nil {
		return nil, err
	}
	out := &FinanceSummary{ReportWindow: reportWindow(period), Ledger: ledger}
	if consumption != nil {
		out.Consumption = *consumption
	}
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
