-- 号主分成倍率（按分组，仅管理员可设）：应付号主 = 账户成本 × 号主分成倍率 × paid_cost / actual_cost；
-- 内部使用（赠送）账户成本 = 账户成本 × 号主分成倍率 − 应付。口径见 internal/repository/owner_settlement_repo.go。
-- 默认 1 时结果与此前完全一致。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS owner_payout_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1 CHECK (owner_payout_multiplier >= 0);

-- 倍率历史（只追加）：每次请求按其 created_at 取所属分组 effective_from 不晚于该时间的最新一条；
-- 修改设置只追加一条“自此生效”的记录，不改写历史，之前的请求继续按旧值结算。
CREATE TABLE IF NOT EXISTS group_owner_payout_multiplier_history (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    multiplier NUMERIC(10,4) NOT NULL CHECK (multiplier >= 0),
    effective_from TIMESTAMPTZ NOT NULL,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS group_owner_payout_multiplier_history_group_effective
    ON group_owner_payout_multiplier_history (group_id, effective_from DESC, id DESC);

-- 每个现有分组一条覆盖全部历史的初始记录（当前值，即 1）。
INSERT INTO group_owner_payout_multiplier_history (group_id, multiplier, effective_from)
SELECT g.id, g.owner_payout_multiplier, '-infinity'::timestamptz FROM groups g
WHERE NOT EXISTS (SELECT 1 FROM group_owner_payout_multiplier_history h WHERE h.group_id = g.id);

COMMENT ON COLUMN groups.owner_payout_multiplier IS '号主分成倍率：应付号主 = 账户成本 × 本倍率 × 实付占比';
COMMENT ON TABLE group_owner_payout_multiplier_history IS '分组号主分成倍率历史（只追加，按请求时间取值）';
