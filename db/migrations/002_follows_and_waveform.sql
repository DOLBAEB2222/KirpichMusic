-- =====================================================================
--  KirpichMusic — миграция 002
--  Добавляет:
--    1) tracks.waveform_peaks    JSONB — массив нормализованных пиков (0..1)
--                                для отрисовки реальной формы волны.
--    2) follows                  — таблица подписок (follower → followee).
--    3) materialised view top_uploaders — топ юзеров по сумме plays_count
--                                их треков (для блока "Рекомендуемые").
--                                Регулярный REFRESH делает приложение.
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
--  1) Waveform peaks
-- ---------------------------------------------------------------------
ALTER TABLE tracks
    ADD COLUMN IF NOT EXISTS waveform_peaks JSONB;

-- Размер JSON ограничиваем CHECK'ом: ~2 KiB достаточно для 120 float'ов.
ALTER TABLE tracks
    ADD CONSTRAINT tracks_waveform_size
    CHECK (waveform_peaks IS NULL OR pg_column_size(waveform_peaks) <= 4096);

-- ---------------------------------------------------------------------
--  2) Подписки
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS follows (
    follower_id  BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id  BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    PRIMARY KEY (follower_id, followee_id),
    -- Нельзя подписаться на самого себя.
    CONSTRAINT follows_no_self CHECK (follower_id <> followee_id)
);

-- "Сколько подписчиков у юзера X" — частый запрос (профиль, recommended).
CREATE INDEX IF NOT EXISTS idx_follows_followee ON follows (followee_id);

-- ---------------------------------------------------------------------
--  3) Топ юзеров по суммарным прослушиваниям
--      Materialised view, чтобы блок "Рекомендуемые" был быстрым.
--      Refresh-сейчас не нужен в реальном времени; refresh раз в N минут.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS top_uploaders AS
SELECT
    u.id                                AS user_id,
    u.username,
    COALESCE(SUM(t.plays_count), 0)::BIGINT AS total_plays,
    COUNT(t.id)::BIGINT                  AS tracks_count,
    (SELECT COUNT(*) FROM follows f WHERE f.followee_id = u.id)::BIGINT AS followers_count
FROM users u
LEFT JOIN tracks t
       ON t.uploader_id = u.id
      AND t.is_published = TRUE
WHERE u.is_active = TRUE
GROUP BY u.id, u.username
ORDER BY total_plays DESC, u.id ASC;

-- UNIQUE-индекс позволяет REFRESH MATERIALIZED VIEW CONCURRENTLY (без блокировки SELECT'ов).
CREATE UNIQUE INDEX IF NOT EXISTS idx_top_uploaders_user
    ON top_uploaders (user_id);

CREATE INDEX IF NOT EXISTS idx_top_uploaders_plays
    ON top_uploaders (total_plays DESC);

COMMIT;
