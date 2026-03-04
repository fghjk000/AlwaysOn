CREATE TABLE health_check_configs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name            VARCHAR(255) NOT NULL,
  type            VARCHAR(10)  NOT NULL CHECK (type IN ('http', 'tcp')),
  target          VARCHAR(512) NOT NULL,
  expected_status INT          NOT NULL DEFAULT 200,
  interval_sec    INT          NOT NULL DEFAULT 30,
  enabled         BOOLEAN      NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
