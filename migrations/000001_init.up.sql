CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE servers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    host       VARCHAR(255) NOT NULL UNIQUE,
    status     VARCHAR(20) NOT NULL DEFAULT 'normal',
    last_seen  TIMESTAMPTZ
);

CREATE TABLE metrics (
    time       TIMESTAMPTZ NOT NULL,
    server_id  UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    cpu        DOUBLE PRECISION NOT NULL,
    memory     DOUBLE PRECISION NOT NULL,
    disk       DOUBLE PRECISION NOT NULL,
    net_in     BIGINT NOT NULL DEFAULT 0,
    net_out    BIGINT NOT NULL DEFAULT 0
);

SELECT create_hypertable('metrics', 'time');

CREATE TABLE alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id   UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    level       VARCHAR(20) NOT NULL,
    metric      VARCHAR(50) NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE thresholds (
    server_id       UUID PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    cpu_warning     DOUBLE PRECISION NOT NULL DEFAULT 75,
    cpu_critical    DOUBLE PRECISION NOT NULL DEFAULT 90,
    mem_warning     DOUBLE PRECISION NOT NULL DEFAULT 80,
    mem_critical    DOUBLE PRECISION NOT NULL DEFAULT 95,
    disk_warning    DOUBLE PRECISION NOT NULL DEFAULT 80,
    disk_critical   DOUBLE PRECISION NOT NULL DEFAULT 90
);
