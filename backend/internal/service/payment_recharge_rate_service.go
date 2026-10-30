package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	// rechargeRateRefreshInterval is how stale the market rate may get.
	rechargeRateRefreshInterval = 24 * time.Hour
	// rechargeRateCheckInterval is how often the job checks staleness. A failed
	// fetch therefore retries within the hour instead of waiting a full day,
	// and turning auto on at runtime picks up a rate without a restart.
	rechargeRateCheckInterval = time.Hour
	rechargeRateRunTimeout    = 45 * time.Second

	balanceRechargeRateLeaderLockKey = "payment:recharge_rate:leader"
	// TTL exceeds the run timeout so the lock never expires mid-run.
	balanceRechargeRateLeaderLockTTL = 2 * time.Minute
)

// RechargeRateFetcher fetches the USD→CNY market rate (CNY per 1 USD).
type RechargeRateFetcher interface {
	Fetch(ctx context.Context) (rate decimal.Decimal, source string, err error)
}

// BalanceRechargeRateService keeps BALANCE_RECHARGE_MULTIPLIER in sync with the
// market USD/CNY rate when auto mode is on. It runs at startup and then refreshes
// whenever the stored rate is older than 24h, gated by a leader lock so only one
// instance fetches.
type BalanceRechargeRateService struct {
	settingRepo   SettingRepository
	fetcher       RechargeRateFetcher
	now           func() time.Time
	checkInterval time.Duration

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewBalanceRechargeRateService creates the service; call Start to run the job.
func NewBalanceRechargeRateService(settingRepo SettingRepository, fetcher RechargeRateFetcher) *BalanceRechargeRateService {
	return &BalanceRechargeRateService{
		settingRepo:   settingRepo,
		fetcher:       fetcher,
		now:           time.Now,
		checkInterval: rechargeRateCheckInterval,
		instanceID:    uuid.NewString(),
		stopCh:        make(chan struct{}),
	}
}

// SetLeaderLock injects the leader-lock cache and DB (same pattern as
// PaymentOrderExpiryService). Both nil = ungated (single instance / tests).
func (s *BalanceRechargeRateService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// Start runs one check immediately, then one per checkInterval.
func (s *BalanceRechargeRateService) Start() {
	if s == nil || s.settingRepo == nil || s.fetcher == nil || s.checkInterval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.checkInterval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop stops the job and waits for an in-flight run.
func (s *BalanceRechargeRateService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *BalanceRechargeRateService) runOnce() {
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, balanceRechargeRateLeaderLockKey, s.instanceID, balanceRechargeRateLeaderLockTTL)
	lockCancel()
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), rechargeRateRunTimeout)
	defer cancel()
	if _, _, err := s.refreshIfDue(ctx); err != nil {
		slog.Warn("[BalanceRechargeRate] scheduled refresh failed; keeping previous rate and multiplier", "error", err)
	}
}

// refreshIfDue fetches only when auto mode is on and the stored rate is missing
// or at least rechargeRateRefreshInterval old. It reports whether it fetched.
func (s *BalanceRechargeRateService) refreshIfDue(ctx context.Context) (*BalanceRechargeRateState, bool, error) {
	vals, err := s.settingRepo.GetMultiple(ctx, rechargeRateSettingKeys)
	if err != nil {
		return nil, false, fmt.Errorf("get recharge rate settings: %w", err)
	}
	in := parseRechargeRateInputs(vals)
	if !in.autoEnabled {
		return in.state(), false, nil
	}
	if in.hasMarket {
		if fetchedAt, err := time.Parse(time.RFC3339, in.fetchedAt); err == nil && s.now().Sub(fetchedAt) < rechargeRateRefreshInterval {
			return in.state(), false, nil
		}
	}
	st, err := s.Refresh(ctx)
	return st, err == nil, err
}

// Refresh fetches the market rate now, stores it, and — when auto mode is on —
// writes the derived multiplier. On any fetch error nothing is written and the
// previous rate and multiplier stay in effect. With auto mode off the market
// rate is still stored (as a preview) but the manual multiplier is untouched.
func (s *BalanceRechargeRateService) Refresh(ctx context.Context) (*BalanceRechargeRateState, error) {
	if s == nil || s.settingRepo == nil || s.fetcher == nil {
		return nil, infraerrors.ServiceUnavailable("RECHARGE_RATE_UNAVAILABLE", "recharge rate service is not configured")
	}
	rate, source, err := s.fetcher.Fetch(ctx)
	if err == nil && !validRechargeMarketRate(rate) {
		err = fmt.Errorf("rate %s outside sanity bounds", rate.String())
	}
	if err != nil {
		slog.Warn("[BalanceRechargeRate] fetch failed; keeping previous rate and multiplier", "error", err)
		return nil, infraerrors.ServiceUnavailable("RECHARGE_RATE_FETCH_FAILED", "failed to fetch the USD/CNY market rate; previous rate kept").WithCause(err)
	}

	vals, err := s.settingRepo.GetMultiple(ctx, rechargeRateSettingKeys)
	if err != nil {
		return nil, fmt.Errorf("get recharge rate settings: %w", err)
	}
	in := parseRechargeRateInputs(vals)
	in.market, in.hasMarket = rate, true
	in.fetchedAt = s.now().UTC().Format(time.RFC3339)
	in.source = source

	updates := map[string]string{
		SettingBalanceRechargeRateMarket:    rate.String(),
		SettingBalanceRechargeRateFetchedAt: in.fetchedAt,
		SettingBalanceRechargeRateSource:    source,
	}
	if in.autoEnabled {
		_, multiplier, err := ComputeRechargeRate(in.market, in.mode, in.markup)
		if err != nil {
			slog.Warn("[BalanceRechargeRate] cannot derive multiplier; keeping previous", "error", err)
			return nil, infraerrors.ServiceUnavailable("RECHARGE_RATE_COMPUTE_FAILED", "failed to derive the recharge multiplier").WithCause(err)
		}
		updates[SettingBalanceRechargeMult] = formatRechargeMultiplier(multiplier)
		in.multiplier = multiplier.InexactFloat64()
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, fmt.Errorf("store recharge rate: %w", err)
	}
	st := in.state()
	slog.Info("[BalanceRechargeRate] market rate updated",
		"source", source, "market", rate.String(), "auto_enabled", st.AutoEnabled,
		"multiplier", st.BalanceRechargeMultiplier)
	return st, nil
}

// GetState returns the stored auto-rate state without fetching.
func (s *BalanceRechargeRateService) GetState(ctx context.Context) (*BalanceRechargeRateState, error) {
	vals, err := s.settingRepo.GetMultiple(ctx, rechargeRateSettingKeys)
	if err != nil {
		return nil, fmt.Errorf("get recharge rate settings: %w", err)
	}
	return parseRechargeRateInputs(vals).state(), nil
}
