-- 精确的“赠送优先”成本账本（规则见 internal/service/usage_cost_ledger.go）。
-- 后台任务离线计算，不在请求计费路径上；金额与 usage_logs 成本同精度（USD）。

-- 每次请求的实付/赠送拆分：gift_cost + paid_cost = actual_cost。
-- 不对 usage_logs 建外键：用量保留期清理后拆分仍保留，全量重建时作为历史扣费继续参与计算。
CREATE TABLE IF NOT EXISTS usage_cost_splits (
    usage_log_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    actual_cost DECIMAL(20,10) NOT NULL,
    gift_cost DECIMAL(20,10) NOT NULL,
    paid_cost DECIMAL(20,10) NOT NULL,
    billing_type SMALLINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS usage_cost_splits_user_created ON usage_cost_splits (user_id, created_at, usage_log_id);

-- 每个用户的剩余赠送/实付额度；processed_until 之前（含）的全部事件均已计入。
-- credit_fp 为截至 processed_until 的入账指纹，用于发现历史入账的增删改（届时该用户从头重建）。
CREATE TABLE IF NOT EXISTS usage_cost_ledger_users (
    user_id BIGINT PRIMARY KEY,
    gift_remaining DECIMAL(20,10) NOT NULL DEFAULT 0,
    paid_remaining DECIMAL(20,10) NOT NULL DEFAULT 0,
    processed_until TIMESTAMPTZ NOT NULL,
    credit_fp TEXT NOT NULL DEFAULT '',
    last_rebuild_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 全局任务进度（单行）。rebuild_requested_at 晚于 last_rebuild_at 时下一轮执行全量重建；
-- 首次部署即登记一次，为历史用量补算拆分。
CREATE TABLE IF NOT EXISTS usage_cost_ledger_cursor (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_usage_id BIGINT NOT NULL DEFAULT 0,
    processed_until TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    rebuild_requested_at TIMESTAMPTZ,
    last_rebuild_at TIMESTAMPTZ
);

INSERT INTO usage_cost_ledger_cursor (id, rebuild_requested_at) VALUES (1, NOW()) ON CONFLICT (id) DO NOTHING;

COMMENT ON TABLE usage_cost_splits IS '每次请求 actual_cost 的赠送/实付拆分（赠送优先，后台任务计算）';
COMMENT ON TABLE usage_cost_ledger_users IS '成本账本：用户剩余赠送/实付额度与处理水位';
COMMENT ON TABLE usage_cost_ledger_cursor IS '成本账本任务进度（单行）';
