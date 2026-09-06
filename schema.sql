CREATE TABLE IF NOT EXISTS todos (
  id         BIGSERIAL PRIMARY KEY,
  title      TEXT        NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
