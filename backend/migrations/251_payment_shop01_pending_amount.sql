-- SHOP01 attributes an incoming payment to an order purely by its exact
-- amount, so two PENDING orders of the same SHOP01 instance must never share a
-- pay amount. The application serializes allocation per instance with
-- pg_advisory_xact_lock; this partial unique index is the database-level
-- safety net behind that lock.
--
-- Deliberately a plain transactional migration (not an online *_notx build):
-- * additive only; the currently running binary never creates provider_key
--   'shop01' orders, so the predicate matches zero rows and no write it makes
--   can violate the index;
-- * payment_orders is a low-volume table, so the brief SHARE lock of a
--   non-concurrent build is negligible, and a transactional build can never
--   leave behind an INVALID index that IF NOT EXISTS would then silently skip.
CREATE UNIQUE INDEX IF NOT EXISTS paymentorder_shop01_pending_pay_amount_unique
    ON payment_orders (provider_instance_id, (round(pay_amount * 100)::bigint))
    WHERE status = 'PENDING' AND provider_key = 'shop01';
