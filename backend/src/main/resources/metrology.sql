CREATE TABLE IF NOT EXISTS metrology_config(id integer PRIMARY KEY CHECK(id=1), enabled_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS metrology_evidence(equipment_id text NOT NULL,wafer_id text NOT NULL,run_id text NOT NULL,completed jsonb,returned jsonb,PRIMARY KEY(equipment_id,wafer_id,run_id));
CREATE TABLE IF NOT EXISTS metrology_runs(id text PRIMARY KEY,source_key text NOT NULL,revision integer NOT NULL,previous_id text REFERENCES metrology_runs(id),equipment_id text NOT NULL,wafer_id text NOT NULL,source jsonb NOT NULL,state text NOT NULL DEFAULT 'waiting-transfer',created_at timestamptz NOT NULL DEFAULT now(),arrival_at timestamptz NOT NULL DEFAULT now()+interval '2 seconds',queue_seq bigserial UNIQUE,last_sent timestamptz,source_sequence bigint NOT NULL DEFAULT 0,result jsonb,completed_at timestamptz,alarm boolean NOT NULL DEFAULT false,acknowledged boolean NOT NULL DEFAULT false,expired boolean NOT NULL DEFAULT false,request_key text UNIQUE,UNIQUE(source_key,revision));
CREATE INDEX IF NOT EXISTS metrology_queue ON metrology_runs(state,arrival_at,queue_seq);
CREATE INDEX IF NOT EXISTS metrology_wafer ON metrology_runs(equipment_id,wafer_id,created_at DESC);
CREATE TABLE IF NOT EXISTS metrology_inbox(id text PRIMARY KEY,received_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS metrology_station(id text PRIMARY KEY,body jsonb NOT NULL,received_at timestamptz NOT NULL DEFAULT now());

ALTER TABLE metrology_runs ADD COLUMN IF NOT EXISTS failure_reason text;
