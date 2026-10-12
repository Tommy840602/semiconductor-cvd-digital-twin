package com.example.cvd;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.time.Instant;
import java.util.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.*;

/** Run only in cvd-disposition-test against a disposable migration_check DB. */
public class DispositionMigrationCheck {
 static void check(boolean value,String reason){if(!value)throw new AssertionError(reason);}
 public static void main(String[] args){
  var source=new DriverManagerDataSource("jdbc:postgresql://postgres:5432/migration_check","cvd","cvd-local");var pg=new JdbcTemplate(source);var mapper=new ObjectMapper();
  var db=new DataStore(pg,mapper,"jdbc:postgresql://timescaledb:5432/migration_check","cvd","cvd-local");db.migrate();
  var access=new AccessStore(pg,new DataSourceTransactionManager(source),"");var recovery=new RecoveryApi(db,access,"/run/secrets/recovery_key");recovery.migrate();var disposition=new DispositionApi(recovery,db,access);disposition.migrate();
  String equipment=Equipment.FIRST;ObjectNode snapshot=mapper.createObjectNode();snapshot.put("equipmentId",equipment);snapshot.put("modelVersion",6);snapshot.put("sourceTime",Instant.now().toString());snapshot.put("communicationQuality","good");var incidents=snapshot.putObject("incidents");var recoveries=snapshot.putObject("recoveries");
  String[] states={"draft","submitted","approved_waiting","executing","completed","completed"};
  for(int n=0;n<states.length;n++){
   String id="migration-"+n,wafer="legacy-wafer-"+n,execution="legacy-execution-"+n;
   pg.update("INSERT INTO recovery_requests(id,wafer_id,owner_id,revision,status,equipment_id) VALUES(?,?,?,1,?,?)",id,wafer,"original-ee",states[n],equipment);
   ObjectNode p=mapper.createObjectNode();p.put("requestId",id);p.put("revision",1);p.put("waferId",wafer);p.put("equipmentId",equipment);p.put("pmId","PM1");p.put("executionId",execution);
   pg.update("INSERT INTO recovery_versions VALUES(?,1,?::jsonb,'{\"id\":\"original-ee\"}'::jsonb,now())",id,p.toString());
   if(n>=2)pg.update("INSERT INTO recovery_intents(execution_id,request_id,revision,approved_by,published) VALUES(?,?,1,'{\"id\":\"original-pe\"}',true)",execution,id);
   if(n==5)continue; // True historical record: no current incident, no invented signatures.
   var i=incidents.putObject("incident-"+n);i.put("id","incident-"+n);i.put("equipmentId",equipment);i.put("pmId","PM1");i.put("waferId",wafer);i.put("executionId",n>=3?execution:"");i.put("status",n==3?"executing":n==4?"awaiting_result":"awaiting_ee");i.put("legacy",true);i.put("returned",n==4);i.put("released",false);
   if(n>=2){var r=recoveries.putObject(execution);r.put("status",n==2?"cancelled":n==3?"executing":"awaiting_result");r.put("requestId",id);}
  }
  pg.update("INSERT INTO equipment_snapshot(id,version,body) VALUES(?,1,?::jsonb)",equipment,snapshot.toString());
  disposition.observe();
  for(int n=0;n<5;n++){var row=pg.queryForMap("SELECT * FROM recovery_requests WHERE id=?","migration-"+n);check(row.get("incident_id").equals("incident-"+n),"incident association");check(row.get("owner_id").equals("original-ee"),"owner changed");}
  check(pg.queryForObject("SELECT status FROM recovery_requests WHERE id='migration-0'",String.class).equals("returned"),"draft not returned");
  check(pg.queryForObject("SELECT status FROM recovery_requests WHERE id='migration-1'",String.class).equals("returned"),"submitted not returned");
  check(pg.queryForObject("SELECT status FROM recovery_requests WHERE id='migration-2'",String.class).equals("cancelling"),"missing device cancellation fence");
  check(pg.queryForObject("SELECT status FROM recovery_requests WHERE id='migration-3'",String.class).equals("executing"),"execution lost");
  check(pg.queryForObject("SELECT status FROM recovery_requests WHERE id='migration-4'",String.class).equals("awaiting_result"),"returned fact lost");
  check(pg.queryForObject("SELECT incident_id IS NULL FROM recovery_requests WHERE id='migration-5'",Boolean.class),"historical incident invented");
  String versions=pg.queryForObject("SELECT string_agg(prescription::text,'|' ORDER BY request_id) FROM recovery_versions",String.class);long events=pg.queryForObject("SELECT count(*) FROM recovery_events",Long.class);
  for(int n=0;n<2;n++){recovery.migrate();disposition.migrate();disposition.observe();}
  check(versions.equals(pg.queryForObject("SELECT string_agg(prescription::text,'|' ORDER BY request_id) FROM recovery_versions",String.class)),"immutable versions changed");
  check(events==pg.queryForObject("SELECT count(*) FROM recovery_events",Long.class),"duplicate migration events");
  check(pg.queryForObject("SELECT count(*) FROM disposition_confirmations",Integer.class)==0&&pg.queryForObject("SELECT count(*) FROM equipment_releases",Integer.class)==0,"fabricated signatures");
  try {pg.update("UPDATE recovery_requests SET equipment_id=? WHERE id='migration-0'",Equipment.IDS.get(1));throw new AssertionError("cross-tool FK missing");}catch(org.springframework.dao.DataIntegrityViolationException expected){}
  // Age a new failed incident chain beyond the telemetry policy; audit survives.
  pg.update("UPDATE recovery_requests SET status='failed',updated_at=now()-interval '40 days' WHERE id='migration-0'");db.cleanup();check(pg.queryForObject("SELECT count(*) FROM recovery_requests WHERE id='migration-0'",Integer.class)==1,"active incident expired");
  check(pg.queryForObject("SELECT count(*) FROM recovery_versions",Integer.class)==6,"version lost");
  System.out.println("PASS: six legacy states, immutable IDs/versions/authors, device cancellation, idempotent migrations, cross-equipment FK, 40-day active evidence");
  ((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();
 }
}
