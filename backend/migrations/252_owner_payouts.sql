-- AI 账号所有者（role=owner）结算打款记录，金额为 USD。
-- 软删除（deleted_at）保留审计痕迹；所有者用户不可被物理删除而丢失打款记录（不级联）。
CREATE TABLE IF NOT EXISTS owner_payouts (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id),
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    paid_at TIMESTAMPTZ NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS ownerpayout_owner_user_id_paid_at ON owner_payouts (owner_user_id, paid_at);
CREATE INDEX IF NOT EXISTS ownerpayout_deleted_at ON owner_payouts (deleted_at);

COMMENT ON TABLE owner_payouts IS 'AI 账号所有者结算打款记录（USD，软删除）';
