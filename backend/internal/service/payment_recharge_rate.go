package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// Automatic balance recharge rate.
//
// Customers pay in CNY, but their balance is denominated in USD. The credited
// amount is "paid CNY × BALANCE_RECHARGE_MULTIPLIER". When auto mode is on, the
// multiplier is derived from the market USD/CNY rate plus an admin markup and
// written back into BALANCE_RECHARGE_MULTIPLIER, so every existing crediting
// path keeps reading one setting and needs no change.
const (
	// SettingBalanceRechargeRateAutoEnabled toggles the automatic multiplier. Off
	// (default) keeps the manual BALANCE_RECHARGE_MULTIPLIER untouched.
	SettingBalanceRechargeRateAutoEnabled = "BALANCE_RECHARGE_RATE_AUTO_ENABLED"
	// SettingBalanceRechargeRateMarkupMode is "amount" (+x CNY per USD) or "percent" (+x%).
	SettingBalanceRechargeRateMarkupMode = "BALANCE_RECHARGE_RATE_MARKUP_MODE"
	// SettingBalanceRechargeRateMarkupValue is the markup (>= 0) interpreted per mode.
	SettingBalanceRechargeRateMarkupValue = "BALANCE_RECHARGE_RATE_MARKUP_VALUE"

	// Read-only state written by the rate job / refresh endpoint.
	SettingBalanceRechargeRateMarket    = "BALANCE_RECHARGE_RATE_MARKET" // CNY per 1 USD
	SettingBalanceRechargeRateFetchedAt = "BALANCE_RECHARGE_RATE_FETCHED_AT"
	SettingBalanceRechargeRateSource    = "BALANCE_RECHARGE_RATE_SOURCE"
)

const (
	RechargeRateMarkupModeAmount  = "amount"
	RechargeRateMarkupModePercent = "percent"
	defaultRechargeRateMarkupMode = RechargeRateMarkupModePercent
)

var (
	// Markup caps: 100% doubles the price; 5 CNY per USD is ~70% at today's rate.
	maxRechargeRateMarkupPercent = decimal.NewFromInt(100)
	maxRechargeRateMarkupAmount  = decimal.NewFromInt(5)

	// Sanity bounds for a fetched USD/CNY market rate. Anything outside is
	// treated as a bad upstream answer and never written.
	minRechargeMarketRate = decimal.NewFromInt(5)
	maxRechargeMarketRate = decimal.NewFromInt(10)
)

const rechargeMultiplierPlaces = 6

var errRechargeRateNoMarket = errors.New("no valid market rate")

// NormalizeRechargeRateMarkupMode returns the canonical mode; empty means the default.
func NormalizeRechargeRateMarkupMode(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return defaultRechargeRateMarkupMode, true
	case RechargeRateMarkupModeAmount:
		return RechargeRateMarkupModeAmount, true
	case RechargeRateMarkupModePercent:
		return RechargeRateMarkupModePercent, true
	default:
		return "", false
	}
}

// validRechargeMarketRate reports whether rate is inside the accepted sanity bounds.
func validRechargeMarketRate(rate decimal.Decimal) bool {
	return rate.GreaterThanOrEqual(minRechargeMarketRate) && rate.LessThanOrEqual(maxRechargeMarketRate)
}

func validateRechargeRateMarkup(mode string, value decimal.Decimal) error {
	if value.IsNegative() {
		return fmt.Errorf("markup must be >= 0")
	}
	limit := maxRechargeRateMarkupPercent
	if mode == RechargeRateMarkupModeAmount {
		limit = maxRechargeRateMarkupAmount
	}
	if value.GreaterThan(limit) {
		return fmt.Errorf("markup must be <= %s for mode %s", limit.String(), mode)
	}
	return nil
}

// ComputeRechargeRate returns the effective CNY per USD (market + markup) and the
// balance multiplier (USD per ¥1) = 1 / effective, rounded to 6 decimals.
// All arithmetic is decimal; no float accumulation.
func ComputeRechargeRate(market decimal.Decimal, mode string, markup decimal.Decimal) (effective, multiplier decimal.Decimal, err error) {
	if !validRechargeMarketRate(market) {
		return decimal.Zero, decimal.Zero, errRechargeRateNoMarket
	}
	mode, ok := NormalizeRechargeRateMarkupMode(mode)
	if !ok {
		return decimal.Zero, decimal.Zero, fmt.Errorf("invalid markup mode")
	}
	if err := validateRechargeRateMarkup(mode, markup); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	if mode == RechargeRateMarkupModeAmount {
		effective = market.Add(markup)
	} else {
		effective = market.Mul(decimal.NewFromInt(1).Add(markup.Div(decimal.NewFromInt(100))))
	}
	multiplier = decimal.NewFromInt(1).DivRound(effective, rechargeMultiplierPlaces)
	if !multiplier.IsPositive() {
		return decimal.Zero, decimal.Zero, fmt.Errorf("computed multiplier is not positive")
	}
	return effective, multiplier, nil
}

func parseRechargeDecimal(raw string) (decimal.Decimal, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, false
	}
	return d, true
}

// rechargeRateSettingKeys are the settings needed to derive the auto multiplier.
var rechargeRateSettingKeys = []string{
	SettingBalanceRechargeRateAutoEnabled,
	SettingBalanceRechargeRateMarkupMode,
	SettingBalanceRechargeRateMarkupValue,
	SettingBalanceRechargeRateMarket,
	SettingBalanceRechargeRateFetchedAt,
	SettingBalanceRechargeRateSource,
	SettingBalanceRechargeMult,
}

// BalanceRechargeRateState is the admin-visible auto-rate state.
type BalanceRechargeRateState struct {
	AutoEnabled bool    `json:"auto_enabled"`
	MarkupMode  string  `json:"markup_mode"`
	MarkupValue float64 `json:"markup_value"`
	// MarketRate is CNY per 1 USD; 0 means no valid rate fetched yet.
	MarketRate float64 `json:"market_rate"`
	FetchedAt  string  `json:"fetched_at"`
	Source     string  `json:"source"`
	// EffectiveRate is CNY per 1 USD after markup; 0 when MarketRate is unset.
	EffectiveRate float64 `json:"effective_rate"`
	// ComputedMultiplier is 1/EffectiveRate (6 dp); 0 when MarketRate is unset.
	ComputedMultiplier float64 `json:"computed_multiplier"`
	// BalanceRechargeMultiplier is the multiplier currently used for crediting.
	BalanceRechargeMultiplier float64 `json:"balance_recharge_multiplier"`
}

// rechargeRateInputs is the parsed decimal view of the stored settings.
type rechargeRateInputs struct {
	autoEnabled bool
	mode        string
	markup      decimal.Decimal
	market      decimal.Decimal
	hasMarket   bool
	fetchedAt   string
	source      string
	multiplier  float64
}

func parseRechargeRateInputs(vals map[string]string) rechargeRateInputs {
	in := rechargeRateInputs{
		autoEnabled: vals[SettingBalanceRechargeRateAutoEnabled] == "true",
		fetchedAt:   vals[SettingBalanceRechargeRateFetchedAt],
		source:      vals[SettingBalanceRechargeRateSource],
		multiplier:  normalizeBalanceRechargeMultiplier(pcParseFloat(vals[SettingBalanceRechargeMult], defaultBalanceRechargeMultiplier)),
	}
	in.mode, _ = NormalizeRechargeRateMarkupMode(vals[SettingBalanceRechargeRateMarkupMode])
	if in.mode == "" {
		in.mode = defaultRechargeRateMarkupMode
	}
	if markup, ok := parseRechargeDecimal(vals[SettingBalanceRechargeRateMarkupValue]); ok && validateRechargeRateMarkup(in.mode, markup) == nil {
		in.markup = markup
	}
	if market, ok := parseRechargeDecimal(vals[SettingBalanceRechargeRateMarket]); ok && validRechargeMarketRate(market) {
		in.market = market
		in.hasMarket = true
	}
	return in
}

func (in rechargeRateInputs) state() *BalanceRechargeRateState {
	st := &BalanceRechargeRateState{
		AutoEnabled:               in.autoEnabled,
		MarkupMode:                in.mode,
		MarkupValue:               in.markup.InexactFloat64(),
		FetchedAt:                 in.fetchedAt,
		Source:                    in.source,
		BalanceRechargeMultiplier: in.multiplier,
	}
	if in.hasMarket {
		st.MarketRate = in.market.InexactFloat64()
		if effective, multiplier, err := ComputeRechargeRate(in.market, in.mode, in.markup); err == nil {
			st.EffectiveRate = effective.Round(rechargeMultiplierPlaces).InexactFloat64()
			st.ComputedMultiplier = multiplier.InexactFloat64()
		}
	}
	return st
}

// formatRechargeMultiplier renders a multiplier for storage (6 dp, no trailing zeros).
func formatRechargeMultiplier(m decimal.Decimal) string {
	return m.Round(rechargeMultiplierPlaces).String()
}

// applyRechargeRateToConfig fills the auto-rate fields of cfg from raw settings.
func applyRechargeRateToConfig(cfg *PaymentConfig, vals map[string]string) {
	st := parseRechargeRateInputs(vals).state()
	cfg.BalanceRechargeRateAutoEnabled = st.AutoEnabled
	cfg.BalanceRechargeRateMarkupMode = st.MarkupMode
	cfg.BalanceRechargeRateMarkupValue = st.MarkupValue
	cfg.BalanceRechargeRateMarket = st.MarketRate
	cfg.BalanceRechargeRateFetchedAt = st.FetchedAt
	cfg.BalanceRechargeRateSource = st.Source
	cfg.BalanceRechargeRateEffective = st.EffectiveRate
}

// resolveRechargeRateUpdate validates the auto-rate part of a config update and
// adds the resulting writes to m. When auto mode ends up enabled, the multiplier
// is recomputed from the stored market rate and the (possibly new) markup, with
// no network fetch, and any manual multiplier in the request is ignored.
func (s *PaymentConfigService) resolveRechargeRateUpdate(ctx context.Context, req UpdatePaymentConfigRequest, m map[string]string) error {
	touched := req.BalanceRechargeRateAutoEnabled != nil || req.BalanceRechargeRateMarkupMode != nil || req.BalanceRechargeRateMarkupValue != nil
	vals, err := s.settingRepo.GetMultiple(ctx, rechargeRateSettingKeys)
	if err != nil {
		return fmt.Errorf("get recharge rate settings: %w", err)
	}
	in := parseRechargeRateInputs(vals)
	if !touched && !in.autoEnabled {
		return nil // auto off and untouched: manual multiplier path is unchanged
	}

	if req.BalanceRechargeRateMarkupMode != nil {
		mode, ok := NormalizeRechargeRateMarkupMode(*req.BalanceRechargeRateMarkupMode)
		if !ok {
			return badRechargeRateMarkup("markup mode must be amount or percent")
		}
		in.mode = mode
		m[SettingBalanceRechargeRateMarkupMode] = mode
	}
	if req.BalanceRechargeRateMarkupValue != nil {
		v := *req.BalanceRechargeRateMarkupValue
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return badRechargeRateMarkup("markup value is not a finite number")
		}
		in.markup = decimal.NewFromFloat(v).Round(4)
		m[SettingBalanceRechargeRateMarkupValue] = in.markup.String()
	}
	if err := validateRechargeRateMarkup(in.mode, in.markup); err != nil {
		return badRechargeRateMarkup(err.Error())
	}
	if req.BalanceRechargeRateAutoEnabled != nil {
		in.autoEnabled = *req.BalanceRechargeRateAutoEnabled
		m[SettingBalanceRechargeRateAutoEnabled] = formatBoolOrEmpty(req.BalanceRechargeRateAutoEnabled)
	}
	if !in.autoEnabled {
		return nil
	}
	// Auto mode owns the multiplier: a manual value sent alongside is ignored.
	delete(m, SettingBalanceRechargeMult)
	if !in.hasMarket {
		return nil // keep the current multiplier until the first successful fetch
	}
	_, multiplier, err := ComputeRechargeRate(in.market, in.mode, in.markup)
	if err != nil {
		return badRechargeRateMarkup(err.Error())
	}
	m[SettingBalanceRechargeMult] = formatRechargeMultiplier(multiplier)
	return nil
}

func badRechargeRateMarkup(msg string) error {
	return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_RATE_MARKUP", msg)
}
