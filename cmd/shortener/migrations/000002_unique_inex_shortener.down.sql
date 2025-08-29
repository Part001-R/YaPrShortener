----------------------------------------------------
---------------------- URL -------------------------
----------------------------------------------------
-- Удаление уникального индекса на поле long
DROP INDEX IF EXISTS idx_shortener_long;

-- Восстановление уникальности на полях long и short
ALTER TABLE shortener
ADD CONSTRAINT shortener_long_key UNIQUE (long),
ADD CONSTRAINT shortener_short_key UNIQUE (short);

-- Восстановление индекса idx_shortener_short
CREATE INDEX idx_shortener_short ON shortener(short);
