-- =====================================================================
--  KirpichMusic — миграция 005
--  Добавляет хранение аватаров и шапок профиля.
--  Файлы лежат на диске в media/{avatars,banners}/...; в БД только пути и метаданные.
-- =====================================================================

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS avatar_path  TEXT,
    ADD COLUMN IF NOT EXISTS avatar_mime  TEXT,
    ADD COLUMN IF NOT EXISTS avatar_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS banner_path  TEXT,
    ADD COLUMN IF NOT EXISTS banner_mime  TEXT,
    ADD COLUMN IF NOT EXISTS banner_bytes BIGINT;

COMMIT;
