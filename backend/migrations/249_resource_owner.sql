-- AI 账号所有者（role=owner）只能管理自己创建的账户、代理与分组。
-- NULL 表示归管理员所有（含全部存量数据）；所有者被物理删除时资源回归管理员。
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE proxies
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_owner_user_id ON accounts (owner_user_id);
CREATE INDEX IF NOT EXISTS idx_proxies_owner_user_id ON proxies (owner_user_id);
CREATE INDEX IF NOT EXISTS idx_groups_owner_user_id ON groups (owner_user_id);

COMMENT ON COLUMN accounts.owner_user_id IS 'AI 账号所有者用户 ID；NULL 表示管理员所有';
COMMENT ON COLUMN proxies.owner_user_id IS 'AI 账号所有者用户 ID；NULL 表示管理员所有';
COMMENT ON COLUMN groups.owner_user_id IS 'AI 账号所有者用户 ID；NULL 表示管理员所有';
