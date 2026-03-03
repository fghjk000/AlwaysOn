CREATE TABLE IF NOT EXISTS alert_cooldowns (
    key        TEXT PRIMARY KEY,
    alerted_at TIMESTAMPTZ NOT NULL
);
