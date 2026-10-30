package service

import (
	"context"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// MaxOwnerPayoutMultiplier 号主分成倍率上限（防误输入；存储为 NUMERIC(10,4)）。
const MaxOwnerPayoutMultiplier = 10.0

var ErrOwnerPayoutMultiplierInvalid = infraerrors.BadRequest("OWNER_PAYOUT_MULTIPLIER_INVALID", "owner_payout_multiplier must be between 0 and 10")

// ValidateOwnerPayoutMultiplier 号主分成倍率须在 [0, MaxOwnerPayoutMultiplier] 内；0 表示该分组不向号主分成。
func ValidateOwnerPayoutMultiplier(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > MaxOwnerPayoutMultiplier {
		return ErrOwnerPayoutMultiplierInvalid
	}
	return nil
}

type operatorUserIDKey struct{}

// WithOperatorUserID 记录发起本次管理操作的用户，供审计类历史（如号主分成倍率历史的 created_by）使用。
func WithOperatorUserID(ctx context.Context, userID int64) context.Context {
	if userID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, operatorUserIDKey{}, userID)
}

// OperatorUserIDFromContext 返回 WithOperatorUserID 记录的用户；未记录（如管理密钥、后台任务）时为 nil。
func OperatorUserIDFromContext(ctx context.Context) *int64 {
	if id, ok := ctx.Value(operatorUserIDKey{}).(int64); ok && id > 0 {
		return &id
	}
	return nil
}
