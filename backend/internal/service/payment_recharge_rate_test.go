//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	require.NoError(t, err)
	return d
}

func TestComputeRechargeRate(t *testing.T) {
	cases := []struct {
		name, market, mode, markup string
		effective, multiplier      string
	}{
		{"amount markup", "7.20", "amount", "0.30", "7.5", "0.133333"},
		{"percent markup", "7.20", "percent", "5", "7.56", "0.132275"},
		{"no markup", "7.20", "percent", "0", "7.2", "0.138889"},  // 0.1388888… rounds up
		{"rounds down", "7.30", "amount", "0", "7.3", "0.136986"}, // 0.1369863…
		{"empty mode defaults to percent", "8", "", "25", "10", "0.1"},
		{"exact", "8", "amount", "0", "8", "0.125"},
		{"max amount markup", "5", "amount", "5", "10", "0.1"},
		{"max percent markup", "10", "percent", "100", "20", "0.05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eff, mult, err := ComputeRechargeRate(dec(t, tc.market), tc.mode, dec(t, tc.markup))
			require.NoError(t, err)
			require.True(t, eff.Equal(dec(t, tc.effective)), "effective %s", eff)
			require.True(t, mult.Equal(dec(t, tc.multiplier)), "multiplier %s", mult)
			require.Equal(t, tc.multiplier, formatRechargeMultiplier(mult))
		})
	}
}

func TestComputeRechargeRateRejectsOutOfBounds(t *testing.T) {
	cases := []struct{ name, market, mode, markup string }{
		{"market below 5", "4.99", "percent", "0"},
		{"market above 10", "10.01", "percent", "0"},
		{"market zero", "0", "amount", "0"},
		{"percent over cap", "7.2", "percent", "100.01"},
		{"amount over cap", "7.2", "amount", "5.01"},
		{"negative markup", "7.2", "amount", "-0.1"},
		{"unknown mode", "7.2", "fixed", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ComputeRechargeRate(dec(t, tc.market), tc.mode, dec(t, tc.markup))
			require.Error(t, err)
		})
	}
	require.True(t, validRechargeMarketRate(dec(t, "5")))
	require.True(t, validRechargeMarketRate(dec(t, "10")))
	require.False(t, validRechargeMarketRate(dec(t, "71.5")))
}

// --- fetcher -----------------------------------------------------------------

func rateServer(t *testing.T, status int, body string, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const frankfurterOK = `{"amount":1.0,"base":"USD","date":"2026-10-23","rates":{"CNY":7.1234}}`
const openERAPIOK = `{"result":"success","base_code":"USD","rates":{"USD":1,"CNY":7.2001,"EUR":0.92}}`

func TestRechargeRateFetcherFrankfurterSuccess(t *testing.T) {
	var fallbackHits int32
	primary := rateServer(t, http.StatusOK, frankfurterOK, nil)
	fallback := rateServer(t, http.StatusOK, openERAPIOK, &fallbackHits)

	rate, source, err := newHTTPRechargeRateFetcher(primary.URL, fallback.URL).Fetch(context.Background())
	require.NoError(t, err)
	require.Equal(t, rechargeRateSourceFrankfurter, source)
	require.True(t, rate.Equal(dec(t, "7.1234")), rate.String())
	require.Zero(t, atomic.LoadInt32(&fallbackHits), "fallback must not be called when primary succeeds")
}

func TestRechargeRateFetcherFallsBackWhenPrimaryFails(t *testing.T) {
	for name, primaryBody := range map[string]struct {
		status int
		body   string
	}{
		"http 500":       {http.StatusInternalServerError, `oops`},
		"malformed json": {http.StatusOK, `{"rates":`},
		"missing CNY":    {http.StatusOK, `{"base":"USD","rates":{"EUR":0.9}}`},
		"out of bounds":  {http.StatusOK, `{"base":"USD","rates":{"CNY":71.5}}`},
		"wrong base":     {http.StatusOK, `{"base":"EUR","rates":{"CNY":7.8}}`},
	} {
		t.Run(name, func(t *testing.T) {
			primary := rateServer(t, primaryBody.status, primaryBody.body, nil)
			fallback := rateServer(t, http.StatusOK, openERAPIOK, nil)
			rate, source, err := newHTTPRechargeRateFetcher(primary.URL, fallback.URL).Fetch(context.Background())
			require.NoError(t, err)
			require.Equal(t, rechargeRateSourceOpenERAPI, source)
			require.True(t, rate.Equal(dec(t, "7.2001")), rate.String())
		})
	}
}

func TestRechargeRateFetcherBothFail(t *testing.T) {
	primary := rateServer(t, http.StatusBadGateway, `bad`, nil)
	fallback := rateServer(t, http.StatusOK, `{"result":"error","error-type":"quota-reached"}`, nil)
	_, _, err := newHTTPRechargeRateFetcher(primary.URL, fallback.URL).Fetch(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), rechargeRateSourceFrankfurter)
	require.Contains(t, err.Error(), rechargeRateSourceOpenERAPI)
}

func TestRechargeRateFetcherRejectsOversizedBody(t *testing.T) {
	huge := `{"base":"USD","rates":{"CNY":7.1},"pad":"` + strings.Repeat("x", rechargeRateMaxBodyBytes) + `"}`
	primary := rateServer(t, http.StatusOK, huge, nil)
	fallback := rateServer(t, http.StatusInternalServerError, ``, nil)
	_, _, err := newHTTPRechargeRateFetcher(primary.URL, fallback.URL).Fetch(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "too large")
}

func TestRechargeRateFetcherClientHasTimeout(t *testing.T) {
	f := NewHTTPRechargeRateFetcher()
	require.Equal(t, 10*time.Second, f.client.Timeout)
	require.Equal(t, rechargeRateFrankfurterURL, f.sources[0].URL)
	require.Equal(t, rechargeRateOpenERAPIURL, f.sources[1].URL)
}

// --- service write path --------------------------------------------------------

type fakeRechargeRateFetcher struct {
	rate   string
	source string
	err    error
	calls  int32
}

func (f *fakeRechargeRateFetcher) Fetch(context.Context) (decimal.Decimal, string, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return decimal.Zero, "", f.err
	}
	d, _ := decimal.NewFromString(f.rate)
	return d, f.source, nil
}

var rechargeRateTestNow = time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)

func newTestRechargeRateService(repo SettingRepository, fetcher RechargeRateFetcher) *BalanceRechargeRateService {
	svc := NewBalanceRechargeRateService(repo, fetcher)
	svc.now = func() time.Time { return rechargeRateTestNow }
	return svc
}

func TestRechargeRateRefreshWritesMultiplierWhenAutoEnabled(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingBalanceRechargeRateAutoEnabled: "true",
		SettingBalanceRechargeRateMarkupMode:  "amount",
		SettingBalanceRechargeRateMarkupValue: "0.3",
		SettingBalanceRechargeMult:            "1",
	}}
	svc := newTestRechargeRateService(repo, &fakeRechargeRateFetcher{rate: "7.20", source: "frankfurter"})

	st, err := svc.Refresh(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		SettingBalanceRechargeRateMarket:    "7.2",
		SettingBalanceRechargeRateFetchedAt: "2026-10-24T08:00:00Z",
		SettingBalanceRechargeRateSource:    "frankfurter",
		SettingBalanceRechargeMult:          "0.133333",
	}, repo.updates)
	require.Equal(t, 7.2, st.MarketRate)
	require.Equal(t, 7.5, st.EffectiveRate)
	require.Equal(t, 0.133333, st.ComputedMultiplier)
	require.Equal(t, 0.133333, st.BalanceRechargeMultiplier)

	// The crediting path reads the multiplier from the payment config unchanged.
	cfg := (&PaymentConfigService{settingRepo: repo}).parsePaymentConfig(repo.values)
	require.Equal(t, 0.133333, cfg.BalanceRechargeMultiplier)
	require.Equal(t, 1.33, calculateCreditedBalance(10, cfg.BalanceRechargeMultiplier))
}

func TestRechargeRateRefreshLeavesManualMultiplierWhenAutoDisabled(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingBalanceRechargeMult: "0.14",
	}}
	svc := newTestRechargeRateService(repo, &fakeRechargeRateFetcher{rate: "7.20", source: "frankfurter"})

	st, err := svc.Refresh(context.Background())
	require.NoError(t, err)
	_, wroteMultiplier := repo.updates[SettingBalanceRechargeMult]
	require.False(t, wroteMultiplier, "manual multiplier must stay untouched when auto is off")
	require.Equal(t, "0.14", repo.values[SettingBalanceRechargeMult])
	require.Equal(t, "7.2", repo.values[SettingBalanceRechargeRateMarket], "market rate is stored as a preview")
	require.Equal(t, 0.14, st.BalanceRechargeMultiplier)
	require.Equal(t, 0.138889, st.ComputedMultiplier)
}

func TestRechargeRateRefreshFailureWritesNothing(t *testing.T) {
	for name, fetcher := range map[string]*fakeRechargeRateFetcher{
		"fetch error":   {err: errors.New("network down")},
		"absurd rate":   {rate: "0.0001", source: "frankfurter"},
		"rate too high": {rate: "715", source: "frankfurter"},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &paymentConfigSettingRepoStub{values: map[string]string{
				SettingBalanceRechargeRateAutoEnabled: "true",
				SettingBalanceRechargeRateMarket:      "7.1",
				SettingBalanceRechargeRateFetchedAt:   "2026-10-20T00:00:00Z",
				SettingBalanceRechargeMult:            "0.140845",
			}}
			svc := newTestRechargeRateService(repo, fetcher)
			_, err := svc.Refresh(context.Background())
			require.Error(t, err)
			require.Nil(t, repo.updates, "nothing may be written on failure")
			require.Equal(t, "0.140845", repo.values[SettingBalanceRechargeMult])
			require.Equal(t, "7.1", repo.values[SettingBalanceRechargeRateMarket])
		})
	}
}

func TestRechargeRateRefreshIfDue(t *testing.T) {
	t.Run("auto disabled never fetches", func(t *testing.T) {
		fetcher := &fakeRechargeRateFetcher{rate: "7.2", source: "frankfurter"}
		svc := newTestRechargeRateService(&paymentConfigSettingRepoStub{values: map[string]string{}}, fetcher)
		_, fetched, err := svc.refreshIfDue(context.Background())
		require.NoError(t, err)
		require.False(t, fetched)
		require.Zero(t, fetcher.calls)
	})
	t.Run("fresh rate is not refetched", func(t *testing.T) {
		fetcher := &fakeRechargeRateFetcher{rate: "7.2", source: "frankfurter"}
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingBalanceRechargeRateAutoEnabled: "true",
			SettingBalanceRechargeRateMarket:      "7.1",
			SettingBalanceRechargeRateFetchedAt:   rechargeRateTestNow.Add(-23 * time.Hour).Format(time.RFC3339),
		}}
		_, fetched, err := newTestRechargeRateService(repo, fetcher).refreshIfDue(context.Background())
		require.NoError(t, err)
		require.False(t, fetched)
		require.Zero(t, fetcher.calls)
	})
	t.Run("stale or missing rate is fetched", func(t *testing.T) {
		for _, fetchedAt := range []string{"", rechargeRateTestNow.Add(-24 * time.Hour).Format(time.RFC3339), "garbage"} {
			fetcher := &fakeRechargeRateFetcher{rate: "7.2", source: "frankfurter"}
			repo := &paymentConfigSettingRepoStub{values: map[string]string{
				SettingBalanceRechargeRateAutoEnabled: "true",
				SettingBalanceRechargeRateMarket:      "7.1",
				SettingBalanceRechargeRateFetchedAt:   fetchedAt,
			}}
			_, fetched, err := newTestRechargeRateService(repo, fetcher).refreshIfDue(context.Background())
			require.NoError(t, err)
			require.True(t, fetched, "fetchedAt=%q", fetchedAt)
			require.Equal(t, "0.138889", repo.values[SettingBalanceRechargeMult])
		}
	})
}

type rechargeRateLockStub struct {
	acquire  bool
	acquired int32
	released int32
}

func (l *rechargeRateLockStub) TryAcquireLeaderLock(_ context.Context, key, _ string, ttl time.Duration) (bool, error) {
	if key != balanceRechargeRateLeaderLockKey || ttl != balanceRechargeRateLeaderLockTTL {
		return false, errors.New("unexpected lock key/ttl")
	}
	atomic.AddInt32(&l.acquired, 1)
	return l.acquire, nil
}

func (l *rechargeRateLockStub) ReleaseLeaderLock(context.Context, string, string) error {
	atomic.AddInt32(&l.released, 1)
	return nil
}

func TestRechargeRateRunOnceHonorsLeaderLock(t *testing.T) {
	values := func() map[string]string {
		return map[string]string{SettingBalanceRechargeRateAutoEnabled: "true"}
	}

	follower := &fakeRechargeRateFetcher{rate: "7.2", source: "frankfurter"}
	svc := newTestRechargeRateService(&paymentConfigSettingRepoStub{values: values()}, follower)
	svc.SetLeaderLock(&rechargeRateLockStub{acquire: false}, nil)
	svc.runOnce()
	require.Zero(t, follower.calls, "non-leader must not fetch")

	leaderLock := &rechargeRateLockStub{acquire: true}
	leader := &fakeRechargeRateFetcher{rate: "7.2", source: "frankfurter"}
	repo := &paymentConfigSettingRepoStub{values: values()}
	svc = newTestRechargeRateService(repo, leader)
	svc.SetLeaderLock(leaderLock, nil)
	svc.runOnce()
	require.EqualValues(t, 1, leader.calls)
	require.EqualValues(t, 1, leaderLock.released)
	require.Equal(t, "0.138889", repo.values[SettingBalanceRechargeMult])
}

func TestRechargeRateStartStop(t *testing.T) {
	fetcher := &fakeRechargeRateFetcher{rate: "7.2", source: "frankfurter"}
	repo := &paymentConfigSettingRepoStub{values: map[string]string{SettingBalanceRechargeRateAutoEnabled: "true"}}
	svc := newTestRechargeRateService(repo, fetcher)
	svc.Start()
	require.Eventually(t, func() bool { return atomic.LoadInt32(&fetcher.calls) == 1 }, 2*time.Second, 10*time.Millisecond,
		"startup run must fetch when auto is enabled")
	svc.Stop()
}

// --- admin save path -----------------------------------------------------------

func TestUpdatePaymentConfigRechargeRate(t *testing.T) {
	ctx := context.Background()
	boolPtr := func(v bool) *bool { return &v }
	strPtr := func(v string) *string { return &v }
	f64Ptr := func(v float64) *float64 { return &v }

	t.Run("auto disabled leaves manual multiplier as entered", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingBalanceRechargeRateMarket: "7.2",
		}}
		svc := &PaymentConfigService{settingRepo: repo}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{
			BalanceRechargeMultiplier: f64Ptr(0.1389),
		}))
		require.Equal(t, map[string]string{SettingBalanceRechargeMult: "0.1389"}, repo.updates)
	})

	t.Run("enabling auto recomputes from stored market without fetching", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingBalanceRechargeRateMarket: "7.20",
			SettingBalanceRechargeMult:       "1",
		}}
		svc := &PaymentConfigService{settingRepo: repo}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{
			BalanceRechargeMultiplier:      f64Ptr(1), // stale manual value from the form is ignored
			BalanceRechargeRateAutoEnabled: boolPtr(true),
			BalanceRechargeRateMarkupMode:  strPtr("percent"),
			BalanceRechargeRateMarkupValue: f64Ptr(5),
		}))
		require.Equal(t, "true", repo.updates[SettingBalanceRechargeRateAutoEnabled])
		require.Equal(t, "percent", repo.updates[SettingBalanceRechargeRateMarkupMode])
		require.Equal(t, "5", repo.updates[SettingBalanceRechargeRateMarkupValue])
		require.Equal(t, "0.132275", repo.updates[SettingBalanceRechargeMult])

		cfg, err := svc.GetPaymentConfig(ctx)
		require.NoError(t, err)
		require.True(t, cfg.BalanceRechargeRateAutoEnabled)
		require.Equal(t, 0.132275, cfg.BalanceRechargeMultiplier)
		require.Equal(t, 7.56, cfg.BalanceRechargeRateEffective)
		require.Equal(t, 7.2, cfg.BalanceRechargeRateMarket)
	})

	t.Run("markup change while auto on recomputes", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingBalanceRechargeRateAutoEnabled: "true",
			SettingBalanceRechargeRateMarkupMode:  "percent",
			SettingBalanceRechargeRateMarkupValue: "5",
			SettingBalanceRechargeRateMarket:      "7.2",
			SettingBalanceRechargeMult:            "0.132275",
		}}
		svc := &PaymentConfigService{settingRepo: repo}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{
			BalanceRechargeRateMarkupMode:  strPtr("amount"),
			BalanceRechargeRateMarkupValue: f64Ptr(0.3),
		}))
		require.Equal(t, "0.133333", repo.updates[SettingBalanceRechargeMult])
	})

	t.Run("auto on without market rate keeps current multiplier", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{SettingBalanceRechargeMult: "0.14"}}
		svc := &PaymentConfigService{settingRepo: repo}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{
			BalanceRechargeMultiplier:      f64Ptr(1),
			BalanceRechargeRateAutoEnabled: boolPtr(true),
		}))
		_, wrote := repo.updates[SettingBalanceRechargeMult]
		require.False(t, wrote)
		require.Equal(t, "0.14", repo.values[SettingBalanceRechargeMult])
	})

	t.Run("unrelated save with auto on refreshes multiplier from stored state", func(t *testing.T) {
		enabled := true
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingBalanceRechargeRateAutoEnabled: "true",
			SettingBalanceRechargeRateMarket:      "8",
			SettingBalanceRechargeMult:            "0.5",
		}}
		svc := &PaymentConfigService{settingRepo: repo}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{Enabled: &enabled}))
		require.Equal(t, "0.125", repo.updates[SettingBalanceRechargeMult])
	})

	t.Run("rejects invalid markup", func(t *testing.T) {
		for name, req := range map[string]UpdatePaymentConfigRequest{
			"bad mode":         {BalanceRechargeRateMarkupMode: strPtr("fixed")},
			"negative":         {BalanceRechargeRateMarkupValue: f64Ptr(-1)},
			"percent over 100": {BalanceRechargeRateMarkupMode: strPtr("percent"), BalanceRechargeRateMarkupValue: f64Ptr(101)},
			"amount over 5":    {BalanceRechargeRateMarkupMode: strPtr("amount"), BalanceRechargeRateMarkupValue: f64Ptr(5.5)},
		} {
			repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
			err := (&PaymentConfigService{settingRepo: repo}).UpdatePaymentConfig(ctx, req)
			require.Error(t, err, name)
			require.Nil(t, repo.updates, name)
		}
	})
}
