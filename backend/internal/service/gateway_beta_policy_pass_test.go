//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// context1MOpusPassPolicy 放行 opus-4-8 的 context-1m，其余模型过滤。
func context1MOpusPassPolicy() *BetaPolicySettings {
	return &BetaPolicySettings{Rules: []BetaPolicyRule{{
		BetaToken:      claude.BetaContext1M,
		Action:         BetaPolicyActionPass,
		Scope:          BetaPolicyScopeAll,
		ModelWhitelist: []string{"claude-opus-4-8", "claude-opus-4-8-*"},
		FallbackAction: BetaPolicyActionFilter,
	}}}
}

// betaPolicyPassSettingRepoStub 额外容忍 OAuth 构建路径上的其它设置读取（返回未配置）。
type betaPolicyPassSettingRepoStub struct {
	betaPolicySettingRepoStub
}

func (s *betaPolicyPassSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func newGatewayServiceWithBetaPolicy(t *testing.T, settings *BetaPolicySettings) *GatewayService {
	t.Helper()
	values := map[string]string{}
	if settings != nil {
		raw, err := json.Marshal(settings)
		require.NoError(t, err)
		values[SettingKeyBetaPolicySettings] = string(raw)
	}
	return &GatewayService{
		cfg:            &config.Config{},
		settingService: NewSettingService(&betaPolicyPassSettingRepoStub{betaPolicySettingRepoStub{values: values}}, &config.Config{}),
	}
}

func TestEvaluateBetaPolicy_PassSetOnlyForExplicitPass(t *testing.T) {
	oauth := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	for _, tc := range []struct {
		name     string
		settings *BetaPolicySettings
		model    string
		wantPass bool
	}{
		{"whitelisted model", context1MOpusPassPolicy(), "claude-opus-4-8", true},
		{"model outside whitelist", context1MOpusPassPolicy(), "claude-opus-4-7", false},
		{"whitelist miss without fallback is not explicit", &BetaPolicySettings{Rules: []BetaPolicyRule{{
			BetaToken: claude.BetaContext1M, Action: BetaPolicyActionPass, Scope: BetaPolicyScopeAll,
			ModelWhitelist: []string{"claude-opus-4-8"},
		}}}, "claude-opus-4-7", false},
		{"explicit pass fallback", &BetaPolicySettings{Rules: []BetaPolicyRule{{
			BetaToken: claude.BetaContext1M, Action: BetaPolicyActionFilter, Scope: BetaPolicyScopeAll,
			ModelWhitelist: []string{"claude-haiku-*"}, FallbackAction: BetaPolicyActionPass,
		}}}, "claude-opus-4-7", true},
		{"scope mismatch", &BetaPolicySettings{Rules: []BetaPolicyRule{{
			BetaToken: claude.BetaContext1M, Action: BetaPolicyActionPass, Scope: BetaPolicyScopeAPIKey,
		}}}, "claude-opus-4-8", false},
		{"default settings sonnet-5", nil, "claude-sonnet-5", true},
		{"default settings opus", nil, "claude-opus-4-8", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newGatewayServiceWithBetaPolicy(t, tc.settings)
			got := svc.evaluateBetaPolicy(context.Background(), "", oauth, tc.model)
			_, pass := got.passSet[claude.BetaContext1M]
			require.Equal(t, tc.wantPass, pass)
		})
	}
}

func TestBuildUpstreamRequest_BetaPolicyPassContext1M(t *testing.T) {
	const custom = "custom-beta-2026-01-01"
	for _, tc := range []struct {
		name        string
		settings    *BetaPolicySettings
		model       string
		mimic       bool
		clientBeta  string
		longContext bool
		want1M      bool
	}{
		{"mimic keeps whitelisted client beta", context1MOpusPassPolicy(), "claude-opus-4-8", true, claude.BetaContext1M + "," + custom, false, true},
		{"mimic drops beta outside whitelist", context1MOpusPassPolicy(), "claude-opus-4-7", true, claude.BetaContext1M, false, false},
		{"mimic drops beta without a rule", &BetaPolicySettings{}, "claude-opus-4-8", true, claude.BetaContext1M, false, false},
		{"mimic default settings non-whitelisted model", nil, "claude-opus-4-8", true, claude.BetaContext1M, false, false},
		{"mimic default settings sonnet-5", nil, "claude-sonnet-5", true, claude.BetaContext1M, false, true},
		{"mimic [1m] suffix whitelisted", context1MOpusPassPolicy(), "claude-opus-4-8", true, "", true, true},
		{"mimic [1m] suffix default settings", nil, "claude-opus-4-8", true, "", true, false},
		{"passthrough [1m] suffix whitelisted", context1MOpusPassPolicy(), "claude-opus-4-8", false, "", true, true},
		{"passthrough [1m] suffix without a rule", &BetaPolicySettings{}, "claude-opus-4-8", false, "", true, false},
		{"passthrough client beta outside whitelist", context1MOpusPassPolicy(), "claude-opus-4-7", false, claude.BetaContext1M, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			if tc.clientBeta != "" {
				c.Request.Header.Set("anthropic-beta", tc.clientBeta)
			}
			if tc.longContext {
				c.Set(longContextRequestedKey, true)
			}
			account := &Account{ID: 702, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
				Credentials: map[string]any{"access_token": "test-token"}, Status: StatusActive, Schedulable: true}
			body := []byte(`{"model":"` + tc.model + `","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`)
			svc := newGatewayServiceWithBetaPolicy(t, tc.settings)

			req, _, err := svc.buildUpstreamRequest(context.Background(), c, account, body,
				"test-token", "oauth", tc.model, false, tc.mimic)
			require.NoError(t, err)
			header := getHeaderRaw(req.Header, "anthropic-beta")
			require.Equal(t, tc.want1M, containsBetaToken(header, claude.BetaContext1M), header)
			require.True(t, containsBetaToken(header, claude.BetaOAuth), header)
			if tc.mimic {
				require.False(t, containsBetaToken(header, custom), "mimic must not forward unknown client betas")
				require.Equal(t, claude.BetaClaudeCode, parseAnthropicBetaHeader(header)[0], "mimic betas keep their order")
			}
		})
	}
}

// count_tokens 与 /v1/messages 对 beta 策略 pass / [1m] 后缀 / filter 优先级的处理必须一致。
func TestBuildCountTokensRequest_BetaPolicyPassMatchesMessages(t *testing.T) {
	for _, tc := range []struct {
		name        string
		settings    *BetaPolicySettings
		model       string
		tokenType   string
		mimic       bool
		clientBeta  string
		longContext bool
		want1M      bool
	}{
		{"mimic [1m] suffix whitelisted", context1MOpusPassPolicy(), "claude-opus-4-8", "oauth", true, "", true, true},
		{"mimic [1m] suffix default settings", nil, "claude-opus-4-8", "oauth", true, "", true, false},
		{"mimic filter fallback wins over client beta", context1MOpusPassPolicy(), "claude-opus-4-7", "oauth", true, claude.BetaContext1M, false, false},
		{"mimic [1m] suffix outside whitelist", context1MOpusPassPolicy(), "claude-opus-4-7", "oauth", true, "", true, false},
		{"passthrough [1m] suffix whitelisted", context1MOpusPassPolicy(), "claude-opus-4-8", "oauth", false, "", true, true},
		{"passthrough [1m] suffix without a rule", &BetaPolicySettings{}, "claude-opus-4-8", "oauth", false, "", true, false},
		{"passthrough client beta outside whitelist", context1MOpusPassPolicy(), "claude-opus-4-7", "oauth", false, claude.BetaContext1M, false, false},
		{"apikey [1m] suffix whitelisted", context1MOpusPassPolicy(), "claude-opus-4-8", "apikey", false, "", true, true},
		{"apikey [1m] suffix default settings", nil, "claude-opus-4-8", "apikey", false, "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			accountType := AccountTypeOAuth
			if tc.tokenType == "apikey" {
				accountType = AccountTypeAPIKey
			}
			account := &Account{ID: 704, Platform: PlatformAnthropic, Type: accountType,
				Credentials: map[string]any{"access_token": "test-token", "api_key": "test-key"}, Status: StatusActive, Schedulable: true}
			body := []byte(`{"model":"` + tc.model + `","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`)
			newContext := func(path string) *gin.Context {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, path, nil)
				if tc.clientBeta != "" {
					c.Request.Header.Set("anthropic-beta", tc.clientBeta)
				}
				if tc.longContext {
					c.Set(longContextRequestedKey, true)
				}
				return c
			}
			svc := newGatewayServiceWithBetaPolicy(t, tc.settings)

			ctReq, _, err := svc.buildCountTokensRequest(context.Background(), newContext("/v1/messages/count_tokens"), account, body,
				"test-token", tc.tokenType, tc.model, tc.mimic)
			require.NoError(t, err)
			ctHeader := getHeaderRaw(ctReq.Header, "anthropic-beta")
			require.Equal(t, tc.want1M, containsBetaToken(ctHeader, claude.BetaContext1M), ctHeader)

			msgReq, _, err := svc.buildUpstreamRequest(context.Background(), newContext("/v1/messages"), account, body,
				"test-token", tc.tokenType, tc.model, false, tc.mimic)
			require.NoError(t, err)
			msgHeader := getHeaderRaw(msgReq.Header, "anthropic-beta")
			require.Equal(t, containsBetaToken(msgHeader, claude.BetaContext1M), containsBetaToken(ctHeader, claude.BetaContext1M),
				"count_tokens=%q messages=%q", ctHeader, msgHeader)
		})
	}
}

func TestComputeFinalCountTokensAnthropicBeta_PassBetasRespectDropSet(t *testing.T) {
	svc := &GatewayService{cfg: &config.Config{}}
	pass := []string{claude.BetaContext1M}
	drop := map[string]struct{}{claude.BetaContext1M: {}}

	got, ok := svc.computeFinalCountTokensAnthropicBeta("apikey", false, "claude-opus-4-8", http.Header{}, []byte(`{}`), nil, pass)
	require.True(t, ok)
	require.Equal(t, claude.BetaContext1M, got)
	got, ok = svc.computeFinalCountTokensAnthropicBeta("apikey", false, "claude-opus-4-8", http.Header{}, []byte(`{}`), drop, pass)
	require.False(t, ok)
	require.Empty(t, got)

	got, ok = svc.computeFinalCountTokensAnthropicBeta("oauth", false, "claude-opus-4-8", http.Header{}, []byte(`{}`), nil, pass)
	require.True(t, ok)
	require.True(t, containsBetaToken(got, claude.BetaTokenCounting), got)
	require.True(t, containsBetaToken(got, claude.BetaContext1M), got)
	got, _ = svc.computeFinalCountTokensAnthropicBeta("oauth", true, "claude-opus-4-8", http.Header{}, []byte(`{}`), drop, pass)
	require.False(t, containsBetaToken(got, claude.BetaContext1M), got)
}

func TestParseGatewayRequest_LongContextSuffixMarksRequest(t *testing.T) {
	parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"claude-opus-4-8[1m]","messages":[]}`)), domain.PlatformAnthropic)
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", parsed.Model)
	require.True(t, parsed.LongContextRequested)

	parsed, err = ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"claude-opus-4-8","messages":[]}`)), domain.PlatformAnthropic)
	require.NoError(t, err)
	require.False(t, parsed.LongContextRequested)
}

func TestPolicyPassBetas(t *testing.T) {
	pass := map[string]struct{}{claude.BetaContext1M: {}}
	require.Nil(t, policyPassBetas(claude.BetaContext1M, true, nil))
	require.Equal(t, []string{claude.BetaContext1M}, policyPassBetas(" other , "+claude.BetaContext1M, true, pass))
	require.Equal(t, []string{claude.BetaContext1M}, policyPassBetas("", true, pass))
	require.Empty(t, policyPassBetas("other", false, pass))
}

// output_config.format（GA 结构化输出）不依赖任何 beta：OAuth mimic / API-key 路径都必须原样保留。
func TestBuildUpstreamRequest_KeepsOutputConfigFormat(t *testing.T) {
	const format = `{"type":"json_schema","schema":{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}}`
	for _, tokenType := range []string{"oauth", "apikey"} {
		t.Run(tokenType, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			accountType := AccountTypeOAuth
			if tokenType == "apikey" {
				accountType = AccountTypeAPIKey
			}
			account := &Account{ID: 703, Platform: PlatformAnthropic, Type: accountType,
				Credentials: map[string]any{"access_token": "test-token", "api_key": "test-key"}, Status: StatusActive, Schedulable: true}
			body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":1024,"output_config":{"effort":"high","format":` + format + `},"messages":[{"role":"user","content":"Return JSON"}]}`)
			svc := newGatewayServiceWithBetaPolicy(t, nil)

			req, _, err := svc.buildUpstreamRequest(context.Background(), c, account, body,
				"test-token", tokenType, "claude-sonnet-4-6", false, tokenType == "oauth")
			require.NoError(t, err)
			out := readUpstreamBodyForTest(t, req)
			require.JSONEq(t, format, gjson.GetBytes(out, "output_config.format").Raw)
		})
	}
}
