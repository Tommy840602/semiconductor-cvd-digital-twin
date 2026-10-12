package com.example.cvd;

import com.fasterxml.jackson.databind.*;
import jakarta.annotation.PostConstruct;
import java.util.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class DataStore {
  final JdbcTemplate pg, ts;
  final ObjectMapper mapper;

  public DataStore(
      JdbcTemplate pg,
      ObjectMapper mapper,
      @Value("${TIMESCALE_URL:jdbc:postgresql://localhost:5433/cvd}") String url,
      @Value("${POSTGRES_USER:cvd}") String user,
      @Value("${POSTGRES_PASSWORD:cvd-local}") String password) {
    this.pg = pg;
    this.mapper = mapper;
    var config=new com.zaxxer.hikari.HikariConfig();config.setJdbcUrl(url);config.setUsername(user);config.setPassword(password);config.setMaximumPoolSize(4);config.setMinimumIdle(0);config.setInitializationFailTimeout(-1);config.setPoolName("timescale");
    this.ts = new JdbcTemplate(new com.zaxxer.hikari.HikariDataSource(config));
  }

  @PostConstruct
  void migrate() {
    pg.execute("CREATE TABLE IF NOT EXISTS maintenance_cycles(id text PRIMARY KEY,equipment_id text NOT NULL,phase text NOT NULL,source_time timestamptz NOT NULL,sequence bigint NOT NULL,body jsonb NOT NULL)");
    pg.execute("CREATE TABLE IF NOT EXISTS equipment_connectivity(equipment_id text PRIMARY KEY,quality text NOT NULL,source_time timestamptz NOT NULL)");
    pg.execute(
        "CREATE TABLE IF NOT EXISTS schema_version(version integer PRIMARY KEY, installed_at"
            + " timestamptz DEFAULT now())");
    pg.execute(
        "CREATE TABLE IF NOT EXISTS inbox(id text PRIMARY KEY, topic text NOT NULL, body jsonb NOT"
            + " NULL, sent boolean DEFAULT false, created_at timestamptz DEFAULT now())");
    pg.execute("CREATE INDEX IF NOT EXISTS inbox_pending_delivery ON inbox(created_at,id) WHERE NOT sent");
    pg.execute(
        "CREATE TABLE IF NOT EXISTS commands(id text PRIMARY KEY, body jsonb NOT NULL, status text"
            + " NOT NULL, result jsonb, sent boolean DEFAULT false, created_at timestamptz DEFAULT"
            + " now(), timed_out boolean DEFAULT false)");
    pg.execute("ALTER TABLE commands ADD COLUMN IF NOT EXISTS last_sent timestamptz");
    pg.execute(
        "CREATE TABLE IF NOT EXISTS history(id text PRIMARY KEY, time timestamptz NOT NULL, lot_id"
            + " text, wafer_id text, kind text NOT NULL, body jsonb NOT NULL)");
    pg.execute("CREATE INDEX IF NOT EXISTS history_lookup ON history(lot_id,wafer_id,time)");
    pg.execute(
        "CREATE TABLE IF NOT EXISTS equipment_snapshot(id text PRIMARY KEY, version bigint NOT"
            + " NULL, body jsonb NOT NULL, updated_at timestamptz DEFAULT now())");
    pg.execute("ALTER TABLE equipment_snapshot ALTER COLUMN body SET COMPRESSION lz4");
    pg.execute(
        "CREATE TABLE IF NOT EXISTS dead_letter(id text PRIMARY KEY, created_at timestamptz DEFAULT"
            + " now(), body jsonb NOT NULL)");
    pg.execute("INSERT INTO schema_version(version) VALUES(1) ON CONFLICT DO NOTHING");
    ts.execute("CREATE EXTENSION IF NOT EXISTS timescaledb");
    ts.execute(
        "CREATE TABLE IF NOT EXISTS telemetry(time timestamptz NOT NULL, id text NOT NULL, body"
            + " jsonb NOT NULL, PRIMARY KEY(time,id))");
    ts.execute("SELECT create_hypertable('telemetry',by_range('time'),if_not_exists=>true)");
    ts.execute("SELECT add_retention_policy('telemetry',INTERVAL '30 days',if_not_exists=>true)");
    for(String table:List.of("inbox","commands","history","dead_letter")) {
      pg.execute("ALTER TABLE "+table+" ADD COLUMN IF NOT EXISTS equipment_id text NOT NULL DEFAULT '"+Equipment.FIRST+"'");
      pg.execute("CREATE INDEX IF NOT EXISTS "+table+"_equipment ON "+table+"(equipment_id)");
    }
    pg.update("UPDATE equipment_snapshot SET id=? WHERE id='CVD-01' AND NOT EXISTS (SELECT 1 FROM equipment_snapshot WHERE id=?)",Equipment.FIRST,Equipment.FIRST);
    ts.execute("ALTER TABLE telemetry ADD COLUMN IF NOT EXISTS equipment_id text NOT NULL DEFAULT '"+Equipment.FIRST+"'");
    ts.execute("CREATE INDEX IF NOT EXISTS telemetry_equipment_time ON telemetry(equipment_id,time DESC)");

  }

  JsonNode parse(String s) {
    try {
      return mapper.readTree(s);
    } catch (Exception e) {
      throw new IllegalArgumentException("Invalid JSON", e);
    }
  }

  String snapshot() { return snapshot(Equipment.current()); }
  String snapshot(String equipment) {
    Equipment.require(equipment);
    var rows =
        pg.queryForList(
            "SELECT (s.body || jsonb_build_object('equipmentId',s.id,'communicationQuality',coalesce(c.quality,'good')))::text FROM equipment_snapshot s LEFT JOIN equipment_connectivity c ON c.equipment_id=s.id WHERE s.id=?", String.class, equipment);
    return rows.isEmpty() ? "{}" : rows.getFirst();
  }

  @Scheduled(fixedDelay = 3600000)
  void cleanup() {
    // Recovery chains expire as a unit, only after every request for that wafer is terminal.
    // During startup the recovery migration may not yet have run.
    if (Boolean.TRUE.equals(pg.queryForObject("SELECT to_regclass('recovery_requests') IS NOT NULL",Boolean.class))) {
        // The transaction and request row locks serialize with approval/revision.
      new org.springframework.transaction.support.TransactionTemplate(new org.springframework.jdbc.datasource.DataSourceTransactionManager(pg.getDataSource())).executeWithoutResult(status -> {
        pg.execute("SELECT pg_advisory_xact_lock(81344022)");
        var expired=pg.queryForList("SELECT id FROM recovery_requests r WHERE incident_id IS NULL AND status IN ('completed','failed','withdrawn','invalidated') AND updated_at<now()-interval '30 days' AND NOT EXISTS (SELECT 1 FROM recovery_requests other WHERE other.wafer_id=r.wafer_id AND (other.status NOT IN ('completed','failed','withdrawn','invalidated') OR other.updated_at>=now()-interval '30 days')) AND NOT EXISTS (SELECT 1 FROM history h WHERE h.wafer_id=r.wafer_id AND h.time>=now()-interval '30 days') FOR UPDATE");
        for(var row:expired){String id=(String)row.get("id");pg.update("DELETE FROM recovery_events WHERE request_id=?",id);pg.update("DELETE FROM recovery_intents WHERE request_id=?",id);pg.update("DELETE FROM recovery_versions WHERE request_id=?",id);pg.update("DELETE FROM recovery_requests WHERE id=?",id);}
      });
    } else return;
    pg.update("DELETE FROM dead_letter WHERE created_at<now()-interval '30 days'");
    pg.update("DELETE FROM inbox WHERE sent AND created_at<now()-interval '30 days'");
    pg.update(
        "DELETE FROM history WHERE time<now()-interval '30 days' AND NOT EXISTS (SELECT 1 FROM maintenance_cycles m WHERE m.phase<>'released' AND (history.body#>>'{payload,maintenance,id}'=m.id OR history.lot_id LIKE m.id||'-%')) AND NOT EXISTS (SELECT 1 FROM recovery_requests r WHERE r.wafer_id=history.wafer_id) AND NOT EXISTS (SELECT 1 FROM"
            + " equipment_snapshot s, jsonb_array_elements(s.body->'lots') l WHERE"
            + " l->>'id'=history.lot_id AND l->>'phase'<>'complete' AND (l->>'purpose' IS DISTINCT FROM 'dummy' OR coalesce((l->>'parked')::boolean,false)=false))");
    pg.update(
        "DELETE FROM commands WHERE created_at<now()-interval '30 days' AND status IN"
            + " ('succeeded','rejected') AND NOT EXISTS (SELECT 1 FROM maintenance_cycles m WHERE m.id IN (commands.body#>>'{parameters,maintenanceId}',commands.equipment_id||'-maintenance-'||commands.id) AND m.phase<>'released') AND commands.body->>'type' NOT IN ('disposition-confirm','disposition-reject','equipment-release') AND NOT EXISTS (SELECT 1 FROM recovery_intents i WHERE commands.id IN (i.execution_id||'-execute',i.execution_id||'-cancel'))");
  }
}
