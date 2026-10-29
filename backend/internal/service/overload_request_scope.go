package service

import (
	"context"
	"sync/atomic"
)

// overload529ScopeKey 标识单个客户端请求的 529 过载处理作用域。
type overload529ScopeKey struct{}

// overload529Scope 记录本请求是否已经因 529 冷却过账号。
// Anthropic 侧的全局过载会让 failover 换到的每个账号都返回 529，若逐个冷却，
// 一次过载突发就会把整个账号池打入冷却；因此同一请求内只冷却首个账号。
type overload529Scope struct {
	marked   atomic.Bool
	disabled bool
}

// WithOverload529RequestScope 为一次客户端请求（含其全部 failover 尝试）建立 529 作用域。
func WithOverload529RequestScope(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(overload529ScopeKey{}).(*overload529Scope); ok {
		return ctx
	}
	return context.WithValue(ctx, overload529ScopeKey{}, &overload529Scope{})
}

// withoutOverload529Marking 用于 count_tokens 等辅助请求：其 529 不冷却账号。
func withoutOverload529Marking(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, overload529ScopeKey{}, &overload529Scope{disabled: true})
}

// claimOverload529Mark 返回本次 529 是否应冷却账号：无作用域时保持原语义（总是冷却），
// 有作用域时仅首次返回 true。
func claimOverload529Mark(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	scope, ok := ctx.Value(overload529ScopeKey{}).(*overload529Scope)
	if !ok || scope == nil {
		return true
	}
	if scope.disabled {
		return false
	}
	return scope.marked.CompareAndSwap(false, true)
}
