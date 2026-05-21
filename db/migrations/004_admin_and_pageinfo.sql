-- 004_admin_and_pageinfo.sql
-- 1) is_admin флаг для доступа к /admin/uptime и потенциальным админ-API.
-- 2) Если в БД ещё нет ни одного админа — первый зарегистрированный
--    юзер автоматически назначается админом (для бутстрапа dev/демо).
--    В проде админа стоит назначать вручную:
--        UPDATE users SET is_admin = TRUE WHERE username = 'твой_логин';
--
-- Идемпотентен: безопасно прогонять повторно.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Промоутим первого юзера, если ни один не помечен как админ.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM users WHERE is_admin = TRUE) THEN
        UPDATE users
        SET is_admin = TRUE
        WHERE id = (SELECT id FROM users ORDER BY id ASC LIMIT 1);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_username_lower ON users (lower(username));

COMMIT;
