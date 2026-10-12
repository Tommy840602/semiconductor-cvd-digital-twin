package com.example.cvd;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import jakarta.annotation.PostConstruct;
import java.time.Instant;
import java.util.*;
import org.springframework.http.HttpStatus;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/api/equipment/{equipmentId}/maintenance")
public class MaintenanceApi {
 final DataStore db;
 public MaintenanceApi(DataStore db){this.db=db;}
 @PostConstruct void migrate(){
  db.pg.execute("CREATE TABLE IF NOT EXISTS maintenance_events(id text PRIMARY KEY,equipment_id text NOT NULL,maintenance_id text NOT NULL,time timestamptz NOT NULL,body jsonb NOT NULL)");
  db.pg.execute("CREATE INDEX IF NOT EXISTS maintenance_events_cycle ON maintenance_events(equipment_id,maintenance_id,time)");
  db.pg.execute("CREATE TABLE IF NOT EXISTS maintenance_cycles(id text PRIMARY KEY,equipment_id text NOT NULL,phase text NOT NULL,source_time timestamptz NOT NULL,sequence bigint NOT NULL,body jsonb NOT NULL)");
 }
 void observe(JsonNode event){
  if(!event.path("kind").asText().startsWith("maintenance-"))return;
  var m=event.path("payload").path("maintenance");String id=m.path("id").asText();if(id.isBlank())return;
  String tool=Equipment.legacy(event.path("equipmentId").asText());
  var tx=new org.springframework.transaction.support.TransactionTemplate(new org.springframework.jdbc.datasource.DataSourceTransactionManager(db.pg.getDataSource()));
  tx.executeWithoutResult(s->{
   db.pg.update("INSERT INTO maintenance_events VALUES(?,?,?,?::timestamptz,?::jsonb) ON CONFLICT DO NOTHING",event.path("eventId").asText(),tool,id,event.path("sourceTime").asText(),event.toString());
   db.pg.update("INSERT INTO maintenance_cycles VALUES(?,?,?,?::timestamptz,?,?::jsonb) ON CONFLICT(id) DO UPDATE SET phase=excluded.phase,source_time=excluded.source_time,sequence=excluded.sequence,body=excluded.body WHERE (maintenance_cycles.source_time,maintenance_cycles.sequence)<(excluded.source_time,excluded.sequence)",id,tool,m.path("phase").asText(),event.path("sourceTime").asText(),event.path("sequence").asLong(),m.toString());
  });
 }
 @GetMapping public Map<String,Object> view(){
  String tool=Equipment.current();var s=db.parse(db.snapshot(tool));return Map.of("current",s.path("maintenance"),"modelVersion",s.path("maintenanceModelVersion").asInt(),"history",db.pg.queryForList("SELECT body::text FROM maintenance_cycles WHERE equipment_id=? ORDER BY source_time DESC LIMIT 50",String.class,tool).stream().map(db::parse).toList());
 }
 @GetMapping("/{id}/events") public List<Map<String,Object>> events(@PathVariable String id){
  return db.pg.queryForList("SELECT e.time,e.body::text body,c.body->'actor' actor FROM maintenance_events e LEFT JOIN commands c ON c.id=e.body#>>'{payload,maintenance,lastCommandId}' AND c.equipment_id=e.equipment_id WHERE e.equipment_id=? AND e.maintenance_id=? ORDER BY e.time,(e.body->>'sequence')::bigint LIMIT 1000",Equipment.current(),id);
 }
 static boolean fresh(JsonNode state){try{return !Instant.parse(state.path("sourceTime").asText()).isBefore(Instant.now().minusSeconds(5));}catch(RuntimeException e){return false;}}
 static boolean held(JsonNode state){return state.hasNonNull("maintenance")&&!state.path("maintenance").path("phase").asText().equals("released");}
 static void guard(DataStore db,ObjectNode c){
  String kind=c.path("type").asText(),tool=c.path("equipmentId").asText();var state=db.parse(db.snapshot(tool));
  if(kind.equals("start")&&held(state))throw new ResponseStatusException(HttpStatus.CONFLICT,"保養期間禁止正式新批次");
  boolean maintenance=kind.startsWith("vacuum-")||kind.startsWith("maintenance-")||kind.equals("qualification-start")||(kind.equals("resume")&&held(state));
  if(!maintenance)return;
  DummyApprovalApi.lock(db,tool);
  if(state.path("maintenanceModelVersion").asInt()!=1||!"good".equals(state.path("communicationQuality").asText())||!fresh(state))throw new ResponseStatusException(HttpStatus.CONFLICT,"設備保養能力尚未就緒或資料過期");
  if(kind.equals("vacuum-recover")||kind.equals("vacuum-service-channel"))return;
  String head=db.pg.queryForObject("SELECT version_id FROM recipe_heads WHERE equipment_id=? FOR UPDATE",String.class,tool);
  if(kind.equals("qualification-start")||kind.equals("resume")){
   if(!Objects.equals(head,state.path("recipeVersion").asText())||Boolean.TRUE.equals(db.pg.queryForObject("SELECT EXISTS(SELECT 1 FROM commands WHERE equipment_id=? AND body->>'type'='configure' AND status IN ('received','accepted'))",Boolean.class,tool)))throw new ResponseStatusException(HttpStatus.CONFLICT,"等待配方下發完成後再驗證");
  }
  if(state.hasNonNull("maintenance")){
   var m=state.path("maintenance");var p=c.path("parameters");if(!m.path("id").asText().equals(p.path("maintenanceId").asText())||!p.path("expectedGeneration").isIntegralNumber()||m.path("generation").asLong()!=p.path("expectedGeneration").asLong())throw new ResponseStatusException(HttpStatus.CONFLICT,"保養週期已變更，請重新載入");
  }
  if(Set.of("qualification-start","maintenance-carrier-load").contains(kind)&&held(state)){
   var v=DummyApprovalApi.view(db,tool,state);
   if(v.path("status").asText().equals("rejected"))((ObjectNode)c.path("parameters")).put("peRetest",true);
  }
  DummyApprovalApi.guard(db,c,state);
 }
 @Scheduled(fixedDelay=3600000) void cleanup(){
  var tx=new org.springframework.transaction.support.TransactionTemplate(new org.springframework.jdbc.datasource.DataSourceTransactionManager(db.pg.getDataSource()));
  tx.executeWithoutResult(s->{
   db.pg.update("DELETE FROM maintenance_events e USING maintenance_cycles c WHERE e.maintenance_id=c.id AND c.phase='released' AND c.source_time<now()-interval '30 days'");
   db.pg.update("DELETE FROM maintenance_cycles WHERE phase='released' AND source_time<now()-interval '30 days'");
  });
 }
}
