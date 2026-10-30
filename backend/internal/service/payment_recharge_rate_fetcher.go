package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Market-rate sources. Frankfurter publishes the ECB reference rate; the
// open.er-api endpoint is an independent fallback.
const (
	rechargeRateSourceFrankfurter = "frankfurter"
	rechargeRateSourceOpenERAPI   = "open.er-api"

	rechargeRateFrankfurterURL = "https://api.frankfurter.dev/v1/latest?from=USD&to=CNY"
	rechargeRateOpenERAPIURL   = "https://open.er-api.com/v6/latest/USD"

	rechargeRateHTTPTimeout  = 10 * time.Second
	rechargeRateMaxBodyBytes = 256 << 10 // open.er-api returns ~160 currencies
	rechargeRateMaxRedirects = 3
)

// rechargeRateSource is one HTTP endpoint plus the parser for its body.
type rechargeRateSource struct {
	Name  string
	URL   string
	Parse func(body []byte) (decimal.Decimal, error)
}

// HTTPRechargeRateFetcher tries each source in order and returns the first
// rate that parses and lies within the sanity bounds.
//
// It deliberately uses its own plain http.Client instead of the gateway
// urlvalidator / httpclient pool: those enforce security.url_allowlist for
// admin-configurable upstream URLs, while these URLs are compile-time constants.
type HTTPRechargeRateFetcher struct {
	client  *http.Client
	sources []rechargeRateSource
}

// NewHTTPRechargeRateFetcher returns the production fetcher (frankfurter, then open.er-api).
func NewHTTPRechargeRateFetcher() *HTTPRechargeRateFetcher {
	return newHTTPRechargeRateFetcher(rechargeRateFrankfurterURL, rechargeRateOpenERAPIURL)
}

func newHTTPRechargeRateFetcher(primaryURL, fallbackURL string) *HTTPRechargeRateFetcher {
	return &HTTPRechargeRateFetcher{
		client: &http.Client{
			Timeout: rechargeRateHTTPTimeout,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= rechargeRateMaxRedirects {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
		sources: []rechargeRateSource{
			{Name: rechargeRateSourceFrankfurter, URL: primaryURL, Parse: parseFrankfurterRate},
			{Name: rechargeRateSourceOpenERAPI, URL: fallbackURL, Parse: parseOpenERAPIRate},
		},
	}
}

// Fetch returns the CNY-per-USD rate and the name of the source that supplied it.
func (f *HTTPRechargeRateFetcher) Fetch(ctx context.Context) (decimal.Decimal, string, error) {
	var errs []string
	for _, src := range f.sources {
		rate, err := f.fetchOne(ctx, src)
		if err == nil {
			return rate, src.Name, nil
		}
		errs = append(errs, src.Name+": "+err.Error())
		if ctx.Err() != nil {
			break
		}
	}
	return decimal.Zero, "", fmt.Errorf("all recharge rate sources failed: %s", strings.Join(errs, "; "))
}

func (f *HTTPRechargeRateFetcher) fetchOne(ctx context.Context, src rechargeRateSource) (decimal.Decimal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return decimal.Zero, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, rechargeRateMaxBodyBytes))
		return decimal.Zero, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, rechargeRateMaxBodyBytes+1))
	if err != nil {
		return decimal.Zero, fmt.Errorf("read body: %w", err)
	}
	if len(body) > rechargeRateMaxBodyBytes {
		return decimal.Zero, errors.New("response body too large")
	}
	rate, err := src.Parse(body)
	if err != nil {
		return decimal.Zero, err
	}
	if !validRechargeMarketRate(rate) {
		return decimal.Zero, fmt.Errorf("rate %s outside sanity bounds [%s, %s]", rate.String(), minRechargeMarketRate.String(), maxRechargeMarketRate.String())
	}
	return rate, nil
}

func decodeRateJSON(body []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func rateFromNumber(n json.Number) (decimal.Decimal, error) {
	if n == "" {
		return decimal.Zero, errors.New("CNY rate missing")
	}
	d, err := decimal.NewFromString(n.String())
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse CNY rate: %w", err)
	}
	return d, nil
}

// parseFrankfurterRate parses {"amount":1.0,"base":"USD","rates":{"CNY":7.1}}.
func parseFrankfurterRate(body []byte) (decimal.Decimal, error) {
	var payload struct {
		Base  string                 `json:"base"`
		Rates map[string]json.Number `json:"rates"`
	}
	if err := decodeRateJSON(body, &payload); err != nil {
		return decimal.Zero, err
	}
	if payload.Base != "" && !strings.EqualFold(payload.Base, "USD") {
		return decimal.Zero, fmt.Errorf("unexpected base %q", payload.Base)
	}
	return rateFromNumber(payload.Rates["CNY"])
}

// parseOpenERAPIRate parses {"result":"success","base_code":"USD","rates":{"CNY":7.1,...}}.
func parseOpenERAPIRate(body []byte) (decimal.Decimal, error) {
	var payload struct {
		Result   string                 `json:"result"`
		BaseCode string                 `json:"base_code"`
		Rates    map[string]json.Number `json:"rates"`
	}
	if err := decodeRateJSON(body, &payload); err != nil {
		return decimal.Zero, err
	}
	if payload.Result != "" && payload.Result != "success" {
		return decimal.Zero, fmt.Errorf("upstream result %q", payload.Result)
	}
	if payload.BaseCode != "" && !strings.EqualFold(payload.BaseCode, "USD") {
		return decimal.Zero, fmt.Errorf("unexpected base %q", payload.BaseCode)
	}
	return rateFromNumber(payload.Rates["CNY"])
}
