-- +goose Up
-- Создание таблицы shortener
CREATE TABLE shortener (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    long TEXT UNIQUE NOT NULL,
    short TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по длинной ссылке
CREATE INDEX idx_shortener_long ON shortener(long);

-- Индекс для поиска по короткой ссылке
CREATE INDEX idx_shortener_short ON shortener(short);

-- Создание таблицы gauges
CREATE TABLE gauges (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name_m TEXT UNIQUE NOT NULL,
    value_m double precision NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по наименованию
CREATE INDEX idx_gauges_name_m ON gauges(name_m);

-- Создание таблицы counters
CREATE TABLE counters (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name_m TEXT UNIQUE NOT NULL,
    value_m INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по наименованию
CREATE INDEX idx_counters_name_m ON counters(name_m);

-- +goose Down
-- Удаление таблицы shortener
DROP TABLE IF EXISTS shortener;

-- Удаление таблицы gauges
DROP TABLE IF EXISTS gauges;

-- Удаление таблицы counters
DROP TABLE IF EXISTS counters;
