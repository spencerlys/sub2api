//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// /v1/chat/completions 多轮对话（无 metadata.user_id、无 cache_control）每轮只追加消息：
// 兜底 hash 只取稳定前缀，才能让同一会话持续命中同一账号的 prompt cache。
func TestGenerateSessionHash_ChatCompletionsGrowingConversationKeepsStableHash(t *testing.T) {
	svc := &GatewayService{}
	ctx := &SessionContext{ClientIP: "1.2.3.4", UserAgent: "openai-python/1.0", APIKeyID: 1}
	parse := func(body string) *ParsedRequest {
		parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(body)), "chat_completions")
		require.NoError(t, err)
		parsed.SessionContext = ctx
		return parsed
	}

	round1 := parse(`{"model":"claude-sonnet-4-6","messages":[{"role":"system","content":"Be concise."},{"role":"user","content":"help me with Go"}]}`)
	round2 := parse(`{"model":"claude-sonnet-4-6","messages":[{"role":"system","content":"Be concise."},{"role":"user","content":"help me with Go"},{"role":"assistant","content":"Sure."},{"role":"user","content":"add tests"}]}`)
	other := parse(`{"model":"claude-sonnet-4-6","messages":[{"role":"system","content":"Be concise."},{"role":"user","content":"help me with Rust"}]}`)

	h1 := svc.GenerateSessionHash(round1)
	require.NotEmpty(t, h1)
	require.Equal(t, h1, svc.GenerateSessionHash(round2), "后续轮次只追加消息时 hash 必须保持不变")
	require.NotEqual(t, h1, svc.GenerateSessionHash(other), "不同的首条 user 消息应产生不同 hash")
}

// 原生 /v1/messages（SDK 直连、无 cache_control）同样只取 system + 首条 user 消息。
func TestGenerateSessionHash_MessagesGrowingConversationKeepsStableHash(t *testing.T) {
	svc := &GatewayService{}
	ctx := &SessionContext{ClientIP: "1.2.3.4", UserAgent: "anthropic-sdk", APIKeyID: 1}
	round1 := mustParseSessionHashRequest(t, anthropicSessionBody("System", []any{msg("user", "hello")}, ""), ctx)
	round2 := mustParseSessionHashRequest(t, anthropicSessionBody("System", []any{msg("user", "hello"), msg("assistant", "Hi"), msg("user", "more")}, ""), ctx)
	otherSystem := mustParseSessionHashRequest(t, anthropicSessionBody("Other system", []any{msg("user", "hello")}, ""), ctx)

	h1 := svc.GenerateSessionHash(round1)
	require.NotEmpty(t, h1)
	require.Equal(t, h1, svc.GenerateSessionHash(round2))
	require.NotEqual(t, h1, svc.GenerateSessionHash(otherSystem))
}
