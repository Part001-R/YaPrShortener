-- +goose Up
-- Удаление уникальных ограничений на поля long и short
ALTER TABLE shortener
DROP CONSTRAINT IF EXISTS shortener_long_key,  -- Удаление уникального ограничения на long
DROP CONSTRAINT IF EXISTS shortener_short_key;  -- Удаление уникального ограничения на short

-- Удаление индекса idx_shortener_long, если он существует
DROP INDEX IF EXISTS idx_shortener_long;

-- Создание нового уникального индекса на поле long
CREATE UNIQUE INDEX idx_shortener_long ON shortener(long);

-- +goose Down
-- Восстановление уникальных ограничений на поля long и short
ALTER TABLE shortener
ADD CONSTRAINT shortener_long_key UNIQUE (long),
ADD CONSTRAINT shortener_short_key UNIQUE (short);

-- Восстановление индекса idx_shortener_long
CREATE INDEX idx_shortener_long ON shortener(long);


