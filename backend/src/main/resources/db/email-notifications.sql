ALTER TABLE app_users ADD COLUMN IF NOT EXISTS email text NOT NULL DEFAULT '';
ALTER TABLE app_users ADD COLUMN IF NOT EXISTS email_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE app_users ADD COLUMN IF NOT EXISTS email_version bigint NOT NULL DEFAULT 1;
ALTER TABLE recovery_requests ADD COLUMN IF NOT EXISTS email_stage bigint NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS mail_connection (
 id integer PRIMARY KEY CHECK(id=1), status text NOT NULL DEFAULT 'disconnected', tokens text,
 sender text NOT NULL DEFAULT '', enabled boolean NOT NULL DEFAULT false,
 public_base_url text NOT NULL DEFAULT '', started_at timestamptz, updated_at timestamptz NOT NULL DEFAULT now());
INSERT INTO mail_connection(id) VALUES(1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS mail_oauth (
 state_hash text PRIMARY KEY, session_hash text NOT NULL, actor_id uuid NOT NULL,
 verifier text NOT NULL, nonce text NOT NULL, expires_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS notification_events (
 id text PRIMARY KEY, type text NOT NULL, equipment_id text NOT NULL, subject_id text NOT NULL DEFAULT '',
 revision bigint NOT NULL DEFAULT 0, stage bigint NOT NULL DEFAULT 0, source_time timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 status text NOT NULL DEFAULT 'pending', reason text NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS notification_deliveries (
 id uuid PRIMARY KEY, event_id text NOT NULL REFERENCES notification_events(id) ON DELETE CASCADE,
 address text NOT NULL, recipients jsonb NOT NULL, status text NOT NULL DEFAULT 'queued', reason text NOT NULL DEFAULT '',
 attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(), lease_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(event_id,address));
CREATE INDEX IF NOT EXISTS notification_pending ON notification_deliveries(available_at) WHERE status IN ('queued','retry');
CREATE TABLE IF NOT EXISTS notification_attempts (
 id bigserial PRIMARY KEY, delivery_id uuid NOT NULL REFERENCES notification_deliveries(id) ON DELETE CASCADE,
 time timestamptz NOT NULL DEFAULT now(), result text NOT NULL, reason text NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS notification_incidents (
 equipment_id text NOT NULL, subject_id text NOT NULL, active boolean NOT NULL DEFAULT false,
 round bigint NOT NULL DEFAULT 0, source_time timestamptz NOT NULL DEFAULT '-infinity', sequence bigint NOT NULL DEFAULT 0,
 since timestamptz NOT NULL DEFAULT now(), notified boolean NOT NULL DEFAULT false, PRIMARY KEY(equipment_id,subject_id));
CREATE OR REPLACE FUNCTION cvd_email_alarm() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s notification_incidents%ROWTYPE; pm text; a boolean; seq bigint; started timestamptz;
BEGIN
 IF NEW.kind <> 'alarm' THEN RETURN NEW; END IF;
 SELECT started_at INTO started FROM mail_connection WHERE id=1;
 IF started IS NULL OR NEW.time < started THEN RETURN NEW; END IF;
 pm := NEW.body->'payload'->>'pm';
 IF pm IS NULL OR pm NOT IN ('PM1','PM2','PM3','PM4') OR jsonb_typeof(NEW.body->'payload'->'active') <> 'boolean' THEN RETURN NEW; END IF;
 a := (NEW.body->'payload'->>'active')::boolean; seq := COALESCE((NEW.body->>'sequence')::bigint,0);
 INSERT INTO notification_incidents(equipment_id,subject_id) VALUES(NEW.equipment_id,pm) ON CONFLICT DO NOTHING;
 SELECT * INTO s FROM notification_incidents WHERE equipment_id=NEW.equipment_id AND subject_id=pm FOR UPDATE;
 IF NEW.time < s.source_time OR (NEW.time=s.source_time AND seq<=s.sequence) THEN RETURN NEW; END IF;
 UPDATE notification_incidents SET active=a,source_time=NEW.time,sequence=seq,round=round+CASE WHEN a AND NOT s.active THEN 1 ELSE 0 END,
 notified=CASE WHEN a AND NOT s.active THEN true WHEN NOT a THEN false ELSE notified END
 WHERE equipment_id=NEW.equipment_id AND subject_id=pm;
 IF a<>s.active AND (a OR s.notified) THEN
  INSERT INTO notification_events(id,type,equipment_id,subject_id,source_time)
  VALUES('alarm:'||NEW.id,CASE WHEN a THEN 'fault' ELSE 'fault-cleared' END,NEW.equipment_id,pm,NEW.time) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS cvd_email_alarm_trigger ON history;
CREATE TRIGGER cvd_email_alarm_trigger AFTER INSERT ON history FOR EACH ROW EXECUTE FUNCTION cvd_email_alarm();
CREATE OR REPLACE FUNCTION cvd_email_case() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE started timestamptz;
BEGIN
 SELECT started_at INTO started FROM mail_connection WHERE id=1;
 IF TG_OP='UPDATE' AND (OLD.status<>NEW.status OR OLD.revision<>NEW.revision) THEN NEW.email_stage:=OLD.email_stage+1; END IF;
 IF started IS NULL THEN RETURN NEW; END IF;
 IF NEW.status NOT IN ('submitted','awaiting_result') THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE' AND OLD.status=NEW.status AND OLD.revision=NEW.revision THEN RETURN NEW; END IF;
 INSERT INTO notification_events(id,type,equipment_id,subject_id,revision,stage,source_time)
 VALUES('case:'||NEW.id||':'||NEW.revision||':'||NEW.status||':'||NEW.email_stage,
 CASE WHEN NEW.status='submitted' THEN 'review' ELSE 'result' END,NEW.equipment_id,NEW.id,NEW.revision,NEW.email_stage,now())
 ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS cvd_email_case_trigger ON recovery_requests;
CREATE TRIGGER cvd_email_case_trigger BEFORE INSERT OR UPDATE ON recovery_requests FOR EACH ROW EXECUTE FUNCTION cvd_email_case();

ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS delivery_locale text CHECK(delivery_locale IN ('zh-TW','en','ja'));
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS locale_reason text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS template_version integer;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS rendered_subject text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS rendered_body text;
