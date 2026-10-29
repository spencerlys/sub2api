//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 同一客户端请求内的 529 风暴只冷却首个账号，避免一次全局过载把整池打入冷却。
func TestHandle529_RequestScopeMarksOnlyFirstAccount(t *testing.T) {
	accountRepo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	ctx := WithOverload529RequestScope(context.Background())
	first := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	second := &Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth}

	svc.HandleUpstreamError(ctx, first, 529, nil, []byte(`{"error":{"type":"overloaded_error"}}`))
	svc.HandleUpstreamError(ctx, second, 529, nil, []byte(`{"error":{"type":"overloaded_error"}}`))

	require.Equal(t, 1, accountRepo.overloadCalls)
	require.Equal(t, int64(1), accountRepo.lastOverloadID)

	// 新请求拥有独立的配额
	svc.HandleUpstreamError(WithOverload529RequestScope(context.Background()), second, 529, nil, nil)
	require.Equal(t, 2, accountRepo.overloadCalls)
	require.Equal(t, int64(2), accountRepo.lastOverloadID)
}

func TestHandle529_WithoutRequestScopeKeepsMarking(t *testing.T) {
	accountRepo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	svc.HandleUpstreamError(context.Background(), &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}, 529, nil, nil)
	svc.HandleUpstreamError(context.Background(), &Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth}, 529, nil, nil)

	require.Equal(t, 2, accountRepo.overloadCalls)
}

// count_tokens 的 529 不得把账号打入过载冷却。
func TestHandle529_CountTokensNeverMarksOverloaded(t *testing.T) {
	accountRepo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	ctx := withoutOverload529Marking(context.Background())
	svc.HandleUpstreamError(ctx, &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth}, 529, nil, nil)

	require.Equal(t, 0, accountRepo.overloadCalls)
}
