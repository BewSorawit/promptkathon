CREATE TABLE IF NOT EXISTS stations (
  id BIGINT PRIMARY KEY,
  code TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  province TEXT NOT NULL,
  district TEXT NOT NULL DEFAULT '',
  river TEXT NOT NULL DEFAULT '',
  latitude DOUBLE PRECISION NOT NULL,
  longitude DOUBLE PRECISION NOT NULL,
  bank_msl DOUBLE PRECISION,
  situation_level SMALLINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS observations (
  station_id BIGINT NOT NULL REFERENCES stations(id),
  observed_at TIMESTAMPTZ NOT NULL,
  level_msl DOUBLE PRECISION NOT NULL,
  source TEXT NOT NULL,
  PRIMARY KEY (station_id, observed_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS stations_code_idx ON stations (code) WHERE code <> '';
CREATE INDEX IF NOT EXISTS observations_time_idx ON observations (observed_at DESC);
CREATE TABLE IF NOT EXISTS model_runs (
  id BIGSERIAL PRIMARY KEY,
  station_id BIGINT NOT NULL REFERENCES stations(id),
  trained_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  horizon_hours SMALLINT NOT NULL DEFAULT 6,
  sample_count INTEGER NOT NULL,
  validation_mae DOUBLE PRECISION NOT NULL,
  baseline_mae DOUBLE PRECISION NOT NULL,
  residual_p90 DOUBLE PRECISION NOT NULL,
  coefficients JSONB,
  intercept DOUBLE PRECISION,
  status TEXT NOT NULL CHECK (status IN ('passed', 'rejected'))
);
CREATE TABLE IF NOT EXISTS forecasts (
  station_id BIGINT NOT NULL REFERENCES stations(id),
  origin_at TIMESTAMPTZ NOT NULL,
  target_at TIMESTAMPTZ NOT NULL,
  predicted_msl DOUBLE PRECISION NOT NULL,
  lower_msl DOUBLE PRECISION NOT NULL,
  upper_msl DOUBLE PRECISION NOT NULL,
  model_run_id BIGINT NOT NULL REFERENCES model_runs(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (station_id, origin_at, target_at)
);
CREATE TABLE IF NOT EXISTS source_syncs (
  source TEXT PRIMARY KEY,
  attempted_at TIMESTAMPTZ NOT NULL,
  succeeded_at TIMESTAMPTZ,
  status TEXT NOT NULL CHECK (status IN ('ok', 'error')),
  detail TEXT NOT NULL DEFAULT '',
  received_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS history_syncs (
  station_id BIGINT PRIMARY KEY REFERENCES stations(id),
  attempted_at TIMESTAMPTZ NOT NULL
);
