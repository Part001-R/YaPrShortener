----------------------------------------------------
---------------------- URL -------------------------
----------------------------------------------------
CREATE TABLE shortener (
    id SERIAL PRIMARY KEY,
    long TEXT UNIQUE NOT NULL,
    short TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по длинной ссылке
CREATE INDEX idx_shortener_long ON shortener(long);

-- Индекс для поиска по короткой ссылке
CREATE INDEX idx_shortener_short ON shortener(short);

---------------------------------------------------
------------------- Metrics -----------------------
---------------------------------------------------

CREATE TABLE gauges (
    id SERIAL PRIMARY KEY,
    name_m TEXT UNIQUE NOT NULL,
    value_m double precision NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по наименованию
CREATE INDEX idx_gauges_name_m ON gauges(name_m);

CREATE TABLE counters (
    id SERIAL PRIMARY KEY,
    name_m TEXT UNIQUE NOT NULL,
    value_m INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по наименованию
CREATE INDEX idx_counters_name_m ON counters(name_m);