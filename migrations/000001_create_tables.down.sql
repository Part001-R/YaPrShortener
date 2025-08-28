-------------- shortener ---------------- 
DROP INDEX IF EXISTS idx_shortener_long;
DROP INDEX IF EXISTS idx_shortener_short;
DROP TABLE IF EXISTS shortener;

--------------- metrics -----------------
DROP INDEX IF EXISTS idx_gauges_name_m;
DROP TABLE IF EXISTS gauges;

DROP INDEX IF EXISTS idx_counters_name_m;
DROP TABLE IF EXISTS counters;