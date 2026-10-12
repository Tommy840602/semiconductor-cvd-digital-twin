package com.example.cvd;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import jakarta.annotation.PostConstruct;
import java.util.*;
import java.security.MessageDigest;
import java.nio.charset.StandardCharsets;
import org.springframework.context.annotation.DependsOn;
import org.springframework.http.HttpStatus;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

/** Human decisions are immutable PG facts; device snapshots prove their effects. */
@RestController
@DependsOn("recoveryApi")
@RequestMapping("/api/equipment/{equipmentId}")
public class DispositionApi {
 final RecoveryApi recovery;final DataStore db;final AccessStore access;
 public DispositionApi(RecoveryApi recovery,DataStore db,AccessStore access){this.recovery=recovery;this.db=db;this.access=access;}
 @PostConstruct void migrate(){access.tx.executeWithoutResult(tx->{
  db.pg.execute("CREATE TABLE IF NOT EXISTS disposition_equipment(id text PRIMARY KEY)");
  for(String id:Equipment.IDS)db.pg.update("INSERT INTO disposition_equipment VALUES(?) ON CONFLICT DO NOTHING",id);
  db.pg.execute("CREATE TABLE IF NOT EXISTS disposition_incidents(equipment_id text REFERENCES disposition_equipment(id),id text,source jsonb NOT NULL,observed jsonb NOT NULL,created_at timestamptz DEFAULT now(),PRIMARY KEY(equipment_id,id))");
  db.pg.execute("CREATE TABLE IF NOT EXISTS disposition_confirmations(id text PRIMARY KEY,request_id text REFERENCES recovery_requests(id),revision integer NOT NULL,result_version integer NOT NULL,execution_id text NOT NULL,decision text NOT NULL CHECK(decision IN ('confirm','reject')),actor jsonb NOT NULL,evidence jsonb NOT NULL,reason text NOT NULL,created_at timestamptz DEFAULT now(),UNIQUE(request_id,revision,result_version,decision))");
  db.pg.execute("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='confirmation_version_fk') THEN ALTER TABLE disposition_confirmations ADD CONSTRAINT confirmation_version_fk FOREIGN KEY(request_id,revision) REFERENCES recovery_versions(request_id,revision); END IF; END $$");
  db.pg.execute("CREATE TABLE IF NOT EXISTS equipment_releases(id text PRIMARY KEY,equipment_id text REFERENCES disposition_equipment(id),generation bigint NOT NULL,evidence jsonb NOT NULL,digest text NOT NULL,actor jsonb NOT NULL,status text NOT NULL,device_result jsonb,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now())");
  db.pg.execute("CREATE UNIQUE INDEX IF NOT EXISTS release_one_pending ON equipment_releases(equipment_id) WHERE status='pending'");
  db.pg.execute("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='recovery_incident_fk') THEN ALTER TABLE recovery_requests ADD CONSTRAINT recovery_incident_fk FOREIGN KEY(equipment_id,incident_id) REFERENCES disposition_incidents(equipment_id,id); END IF; END $$");
 });}
 @Scheduled(fixedDelay=1000) void observe(){for(String equipment:Equipment.IDS){try{var s=recovery.snapshot(equipment);if(s.path("modelVersion").asInt()<6)continue;access.tx.executeWithoutResult(tx->{recovery.lock(equipment);for(var i:s.path("incidents")){String id=i.path("id").asText();if(id.isBlank())continue;
   db.pg.update("INSERT INTO disposition_incidents(equipment_id,id,source,observed) VALUES(?,?,?::jsonb,?::jsonb) ON CONFLICT(equipment_id,id) DO UPDATE SET observed=excluded.observed",equipment,id,i.toString(),i.toString());
   var rows=db.pg.queryForList("SELECT id FROM recovery_requests WHERE equipment_id=? AND incident_id=?",equipment,id);
   if(rows.isEmpty()){
    // Preserve pre-upgrade request and version IDs. Never attribute a system event to EE.
    var legacy=db.pg.queryForList("SELECT id,revision,status FROM recovery_requests WHERE equipment_id=? AND wafer_id=? AND incident_id IS NULL ORDER BY created_at DESC LIMIT 1",equipment,i.path("waferId").asText());
    if(!legacy.isEmpty()&&!i.path("waferId").asText().isEmpty()){
     String old=(String)legacy.getFirst().get("id");db.pg.update("UPDATE recovery_requests SET incident_id=? WHERE id=?",id,old);
     String status=(String)legacy.getFirst().get("status");
     if(Set.of("draft","submitted").contains(status))db.pg.update("UPDATE recovery_requests SET status='returned',reason='升級：需補齊排錯欄位並重新送審',reason_code='upgradeTroubleshoot' WHERE id=?",old);
     if(status.equals("approved_waiting")){
      var intents=db.pg.queryForList("SELECT execution_id FROM recovery_intents WHERE request_id=? AND revision=?",old,legacy.getFirst().get("revision"));
      if(!intents.isEmpty()){
       String execution=(String)intents.getFirst().get("execution_id");var device=s.path("recoveries").path(execution);
       if(device.path("status").asText().equals("executing"))db.pg.update("UPDATE recovery_requests SET status='executing' WHERE id=?",old);
       else {
        ObjectNode p=(ObjectNode)db.parse(db.pg.queryForObject("SELECT prescription::text FROM recovery_versions WHERE request_id=? AND revision=?",String.class,old,legacy.getFirst().get("revision")));p.put("equipmentId",equipment);p.put("executionId",execution);
        recovery.queue(p,"cancel-supplement",null);db.pg.update("UPDATE recovery_requests SET status='cancelling',reason='升級：等待設備取消柵欄，確認後修訂重審',reason_code='upgradeCancel' WHERE id=?",old);
       }
      }
     }
     if(Set.of("executing","completed","failed").contains(status)&&!i.path("executionId").asText().isEmpty())db.pg.update("UPDATE recovery_requests SET status=?,reason='升級：保留原執行與回收事實，等待新流程確認',reason_code='upgradeEvidence' WHERE id=?",i.path("status").asText().equals("awaiting_ee")?"failed":i.path("status").asText(),old);
     if(i.path("legacy").asBoolean()&&i.path("returned").asBoolean()&&i.path("executionId").asText().isEmpty())db.pg.update("UPDATE recovery_requests SET status='returned',reason='舊版已回槽，請補齊排錯資料並確認既有回收事實',reason_code='legacyReturned' WHERE id=?",old);
     recovery.event(old,((Number)legacy.getFirst().get("revision")).intValue(),"migration",null,i);
    } else {
     String request=UUID.randomUUID().toString();db.pg.update("INSERT INTO recovery_requests(id,wafer_id,owner_id,revision,status,equipment_id,incident_id) VALUES(?,?,NULL,0,'awaiting_ee',?,?)",request,i.path("waferId").asText(),equipment,id);
     ObjectNode source=i.deepCopy();source.put("incidentId",id);source.put("dispositionType",i.path("legacy").asBoolean()&&i.path("returned").asBoolean()?"legacy_return":i.path("waferId").asText().isEmpty()?"equipment":"supplement");source.set("stepSeconds",db.mapper.valueToTree(new int[4]));
     db.pg.update("INSERT INTO recovery_versions(request_id,revision,prescription,author) VALUES(?,0,?::jsonb,'{}'::jsonb)",request,source.toString());recovery.event(request,0,"exception",null,i);
    }
   }
   if(i.path("status").asText().equals("awaiting_ee"))db.pg.update("UPDATE recovery_requests SET status='failed',reason='處置結果失效，保留晶圓待 EE 新方案',reason_code='resultInvalid',updated_at=now() WHERE equipment_id=? AND incident_id=? AND status IN ('awaiting_result','returning','resolved')",equipment,id);
   if(i.path("released").asBoolean())db.pg.update("UPDATE recovery_requests SET status='completed',reason='整台已經 Leader 核准並由設備確認放行',reason_code='releaseComplete',updated_at=now() WHERE equipment_id=? AND incident_id=? AND status='resolved'",equipment,id);
  }
  for(var r:db.pg.queryForList("SELECT id FROM equipment_releases WHERE equipment_id=? AND status='pending'",equipment)){
   String id=(String)r.get("id");var result=s.path("releases").path(id);
   if(!result.isMissingNode())db.pg.update("UPDATE equipment_releases SET status=?,device_result=?::jsonb,updated_at=now() WHERE id=?",result.path("status").asText(),result.toString(),id);
   else {var commands=db.pg.queryForList("SELECT status,result::text FROM commands WHERE id=?",id);if(!commands.isEmpty()&&"rejected".equals(commands.getFirst().get("status")))db.pg.update("UPDATE equipment_releases SET status='invalidated',device_result=?::jsonb,updated_at=now() WHERE id=?",commands.getFirst().get("result"),id);}
  }
 });}catch(Exception ex){org.slf4j.LoggerFactory.getLogger(getClass()).debug("Disposition observation waiting for {}: {}",equipment,ex.getClass().getSimpleName());}}}
 @PostMapping("/dispositions/{id}/{decision}") public Map<String,Object> result(@PathVariable String id,@PathVariable String decision,@RequestBody ObjectNode body){
  recovery.authorize("PE");if(!Set.of("confirm","reject").contains(decision))RecoveryApi.fail(HttpStatus.BAD_REQUEST,"未知結果動作");
  access.tx.executeWithoutResult(tx->{String equipment=Equipment.current();recovery.lock(equipment);recovery.currentActor();var rows=db.pg.queryForList("SELECT * FROM recovery_requests WHERE id=? AND equipment_id=? FOR UPDATE",id,equipment);if(rows.isEmpty())RecoveryApi.fail(HttpStatus.NOT_FOUND,"處置不存在");var r=rows.getFirst();var actor=AccessSessions.actor();if(Objects.equals(actor.id(),r.get("owner_id")))RecoveryApi.fail(HttpStatus.FORBIDDEN,"提案人不可確認自己的結果");int rev=((Number)r.get("revision")).intValue();if(rev!=body.path("revision").asInt())RecoveryApi.fail(HttpStatus.CONFLICT,"版本已變更");
   var s=recovery.snapshot();var i=s.path("incidents").path((String)r.get("incident_id"));String execution=i.path("executionId").asText();int version=i.path("resultVersion").asInt();
   if(version!=body.path("resultVersion").asInt()||!execution.equals(body.path("executionId").asText()))RecoveryApi.fail(HttpStatus.CONFLICT,"結果版本已變更");
   var old=db.pg.queryForList("SELECT id FROM disposition_confirmations WHERE request_id=? AND revision=? AND result_version=? AND decision=?",id,rev,version,decision);if(!old.isEmpty())return;
   if(!i.path("status").asText().equals("awaiting_result")||i.path("released").asBoolean()||execution.isEmpty())RecoveryApi.fail(HttpStatus.CONFLICT,"尚非待確認結果");
   var p=i.path("legacy").asBoolean()&&i.path("pmId").asText().isEmpty()?db.mapper.createObjectNode():recovery.chamber(s,i.path("pmId").asText());if(p.path("fault").asBoolean())RecoveryApi.fail(HttpStatus.CONFLICT,"請先解除 PM 故障");
   var device=s.path("recoveries").path(execution);if(device.isMissingNode())RecoveryApi.fail(HttpStatus.CONFLICT,"執行结果未知");
   String kind=device.path("dispositionType").asText("supplement");
   if(decision.equals("confirm")){
    if(kind.equals("supplement")&&!device.path("result").path("quality").asText().equals("pass"))RecoveryApi.fail(HttpStatus.CONFLICT,"補做品質不合格，不能強制通過");
    if(Set.of("scrap","legacy_return").contains(kind)&&!i.path("returned").asBoolean())RecoveryApi.fail(HttpStatus.CONFLICT,"報廢晶圓尚未回原槽位");
   }
   String reason=body.path("reason").asText().strip();if(reason.length()>4000||(decision.equals("reject")&&reason.length()<3))RecoveryApi.fail(HttpStatus.BAD_REQUEST,"否決需至少三字說明");
   String authorization=UUID.randomUUID().toString();ObjectNode payload=db.mapper.createObjectNode();payload.put("equipmentId",equipment);payload.put("incidentId",i.path("id").asText());payload.put("executionId",execution);payload.put("resultVersion",version);payload.put("authorizationId",authorization);payload.put("reason",reason);
   db.pg.update("INSERT INTO disposition_confirmations(id,request_id,revision,result_version,execution_id,decision,actor,evidence,reason) VALUES(?,?,?,?,?,?,?::jsonb,?::jsonb,?)",authorization,id,rev,version,execution,decision,db.mapper.valueToTree(actor).toString(),device.toString(),reason);
   recovery.queue(payload,"disposition-"+decision,actor);recovery.event(id,rev,"result-"+decision,actor,payload);
  });return recovery.detail(id);
 }
 String statusLabel(String value){return Map.of("awaiting_ee","待 EE 排錯","approved_waiting","等待核准處置執行","executing","處置執行中","awaiting_result","待 PE 結果確認","returning","等待回原槽位","resolved","已完成").getOrDefault(value,value);}
 Map<String,Object> releaseView(String equipment,boolean requireActor){
  var s=recovery.snapshot(equipment);List<String> blocks=new ArrayList<>();List<Map<String,Object>> items=new ArrayList<>();List<JsonNode> evidence=new ArrayList<>();Set<String> participants=new HashSet<>();
  if(!s.path("dispositionHold").asBoolean())blocks.add("沒有待放行的異常週期");
  if(s.path("paused").asBoolean())blocks.add("設備時鐘暫停，請先確認恢復時鐘（維持派片鎖）");
  for(var p:s.path("chambers"))if(p.path("fault").asBoolean())blocks.add(p.path("id").asText()+" 故障尚未解除");
  for(var i:s.path("incidents"))if(!i.path("released").asBoolean()){
   String incident=i.path("id").asText();var rows=db.pg.queryForList("SELECT id,status,owner_id FROM recovery_requests WHERE equipment_id=? AND incident_id=?",equipment,incident);
   String request=rows.isEmpty()?"":(String)rows.getFirst().get("id");
   if(!i.path("status").asText().equals("resolved")||!i.path("returned").asBoolean()||i.path("confirmationId").asText().isEmpty())blocks.add(i.path("pmId").asText("已回槽晶圓")+"："+statusLabel(i.path("status").asText()));
   if(i.path("status").asText().equals("resolved")&&(rows.isEmpty()||!"resolved".equals(rows.getFirst().get("status"))))blocks.add(i.path("pmId").asText()+" 處置證據尚待同步確認");
   if(!request.isEmpty())for(var actor:db.pg.queryForList("SELECT DISTINCT actor->>'id' id FROM recovery_events WHERE request_id=? AND action IN ('draft','submitted','approved','result-confirm','result-reject') AND actor->>'id' IS NOT NULL",request))participants.add((String)actor.get("id"));
   ObjectNode e=db.mapper.createObjectNode();e.put("incidentId",incident);e.put("executionId",i.path("executionId").asText());e.put("resultVersion",i.path("resultVersion").asInt());e.put("confirmationId",i.path("confirmationId").asText());evidence.add(e);
   items.add(Map.of("requestId",request,"incident",i));
  }
  evidence.sort(Comparator.comparing(x->x.path("incidentId").asText()));
  if(requireActor){var actor=AccessSessions.actor();if(!actor.role().equals("Leader")||!("EE".equals(actor.department())||"PE".equals(actor.department())))blocks.add("需要 EE／PE Leader");if(participants.contains(actor.id()))blocks.add("你參與過本次處置，需其他帳號 Leader 放行");}
  for(var r:s.path("recoveries"))if(Set.of("approved_waiting","executing").contains(r.path("status").asText()))blocks.add("仍有活動處置 "+r.path("executionId").asText());
  Map<String,Object> out=new LinkedHashMap<>();out.put("equipmentId",equipment);out.put("generation",s.path("holdGeneration").asLong());out.put("evidence",evidence);out.put("items",items);out.put("participants",participants);out.put("blocks",blocks);out.put("blockMessages",blocks.stream().map(SystemMessages::response).toList());out.put("eligible",blocks.isEmpty());
  out.put("releases",db.pg.queryForList("SELECT id,generation,digest,actor::text,status,device_result::text,created_at,updated_at FROM equipment_releases WHERE equipment_id=? ORDER BY created_at DESC LIMIT 30",equipment));return out;
 }
 @GetMapping("/release") public Map<String,Object> release(){return releaseView(Equipment.current(),true);}
 @PostMapping("/release") public Map<String,Object> approveRelease(@RequestBody ObjectNode body){
  String equipment=Equipment.current();access.tx.executeWithoutResult(tx->{recovery.lock(equipment);recovery.currentActor();var view=releaseView(equipment,true);if(!Boolean.TRUE.equals(view.get("eligible")))RecoveryApi.fail(HttpStatus.CONFLICT,String.join("；",(List<String>)view.get("blocks")));
   if(body.path("generation").asLong(-1)!=((Number)view.get("generation")).longValue())RecoveryApi.fail(HttpStatus.CONFLICT,"異常集合已變更");
   var pending=db.pg.queryForList("SELECT id FROM equipment_releases WHERE equipment_id=? AND status='pending'",equipment);if(!pending.isEmpty())return;
   String id=UUID.randomUUID().toString();ObjectNode p=db.mapper.createObjectNode();p.put("equipmentId",equipment);p.put("generation",((Number)view.get("generation")).longValue());p.put("releaseId",id);p.set("evidence",db.mapper.valueToTree(view.get("evidence")));
   String digest;try{digest=HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(p.path("evidence").toString().getBytes(StandardCharsets.UTF_8)));}catch(Exception ex){throw new IllegalStateException(ex);}
   db.pg.update("INSERT INTO equipment_releases(id,equipment_id,generation,evidence,digest,actor,status) VALUES(?,?,?,?::jsonb,?,?::jsonb,'pending')",id,equipment,view.get("generation"),p.path("evidence").toString(),digest,db.mapper.valueToTree(AccessSessions.actor()).toString());
   recovery.queue(p,"equipment-release",AccessSessions.actor());
  });return release();
 }
}
