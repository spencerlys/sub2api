-- Migration: 250_channel_monitor_proxy
-- 渠道监控探活走代理：部署环境出网只允许经由已配置的代理，直连探活恒失败。
--   proxy_id 显式指定探活（含 origin HEAD ping）使用的代理；
--   NULL 时回退到关联账号的代理，账号也无代理则直连（存量行为）。
-- 代理删除时置空，监控保留并按上述回退规则继续运行。
ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS proxy_id BIGINT NULL REFERENCES proxies(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_channel_monitors_proxy_id ON channel_monitors (proxy_id);

COMMENT ON COLUMN channel_monitors.proxy_id IS
    '探活请求使用的代理 ID；NULL 表示回退到关联账号代理，否则直连';
