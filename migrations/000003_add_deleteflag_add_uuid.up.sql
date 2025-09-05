-- +goose Up
-- Добавление полей deleteFlag и uuid в таблицу shortener
ALTER TABLE shortener
ADD COLUMN deleteFlag BOOLEAN DEFAULT FALSE,  -- Устанавливаем значение по умолчанию в FALSE
ADD COLUMN uuid TEXT;                -- Добавляем поле uuid и делаем его обязательным

-- +goose Down
-- Удаление полей deleteFlag и uuid из таблицы shortener
ALTER TABLE shortener
DROP COLUMN deleteFlag,
DROP COLUMN uuid;
