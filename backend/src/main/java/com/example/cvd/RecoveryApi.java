package com.example.cvd;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import jakarta.annotation.PostConstruct;
import java.nio.file.*;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.http.HttpStatus;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/api/equipment/{equipmentId}/recoveries")
public class RecoveryApi {
 final DataStore db; final AccessStore access; final String keyFile;
 static final Set<String> TERMINAL=Set.of("completed","failed","withdrawn","invalidated");
 public RecoveryApi(DataStore db,AccessStore access,@Value("${CVD_RECOVERY_KEY_FILE:}")String keyFile){this.db=db;this.access=access;this.keyFile=keyFile;}
 @PostConstruct void migrate(){access.tx.executeWithoutResult(s->{
  db.pg.execute("CREATE TABLE IF NOT EXISTS recovery_requests(id text PRIMARY KEY,wafer_id text NOT NULL,owner_id text NOT NULL,revision integer NOT NULL,status text NOT NULL,reason text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now())");
  db.pg.execute("ALTER TABLE recovery_requests ADD COLUMN IF NOT EXISTS reason_code text NOT NULL DEFAULT ''");
  db.pg.execute("CREATE TABLE IF NOT EXISTS recovery_versions(request_id text REFERENCES recovery_requests(id),revision integer,prescription jsonb NOT NULL,author jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(request_id,revision))");
  db.pg.execute("CREATE TABLE IF NOT EXISTS recovery_events(id bigserial PRIMARY KEY,request_id text REFERENCES recovery_requests(id),revision integer NOT NULL,action text NOT NULL,actor jsonb,details jsonb NOT NULL,time timestamptz NOT NULL DEFAULT now())");
  db.pg.execute("CREATE TABLE IF NOT EXISTS recovery_intents(execution_id text PRIMARY KEY,request_id text REFERENCES recovery_requests(id),revision integer NOT NULL,approved_by jsonb NOT NULL,published boolean NOT NULL DEFAULT false,result jsonb,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(request_id,revision))");
  db.pg.execute("ALTER TABLE recovery_requests ADD COLUMN IF NOT EXISTS equipment_id text NOT NULL DEFAULT '"+Equipment.FIRST+"'");
  db.pg.execute("DROP INDEX IF EXISTS recovery_one_active");
  db.pg.execute("DROP INDEX IF EXISTS recovery_one_active_equipment");
  db.pg.execute("ALTER TABLE recovery_requests ALTER COLUMN owner_id DROP NOT NULL");
  db.pg.execute("ALTER TABLE recovery_requests ADD COLUMN IF NOT EXISTS incident_id text");
  db.pg.execute("CREATE UNIQUE INDEX IF NOT EXISTS recovery_incident_unique ON recovery_requests(equipment_id,incident_id) WHERE incident_id IS NOT NULL");
  db.pg.execute("CREATE UNIQUE INDEX IF NOT EXISTS recovery_one_active_equipment ON recovery_requests(equipment_id,wafer_id) WHERE wafer_id<>'' AND status NOT IN ('completed','failed','withdrawn','invalidated')");
 });}
 static void fail(HttpStatus status,String message){throw new ResponseStatusException(status,message);}
 void authorize(String department){var a=AccessSessions.actor();if(!Set.of("Engineer","Leader").contains(a.role())||!department.equals(a.department()))fail(HttpStatus.FORBIDDEN,"需要 "+department+" Engineer／Leader 權限");}
 void currentActor(){var a=AccessSessions.actor();var rows=db.pg.queryForList("SELECT auth_version FROM app_users WHERE id=?::uuid FOR SHARE",a.id());if(rows.isEmpty()||((Number)rows.getFirst().get("auth_version")).longValue()!=a.authVersion())fail(HttpStatus.UNAUTHORIZED,"身分已變更，請重新登入");}
 JsonNode snapshot(){return snapshot(Equipment.current());}
 JsonNode snapshot(String equipment){JsonNode s=db.parse(db.snapshot(equipment));if(!s.path("communicationQuality").asText("good").equals("good"))fail(HttpStatus.CONFLICT,"設備通訊資料不可信");try{if(Duration.between(Instant.parse(s.path("sourceTime").asText()),Instant.now()).abs().toSeconds()>5)fail(HttpStatus.CONFLICT,"設備資料過期，等待通訊恢復");}catch(java.time.format.DateTimeParseException ex){fail(HttpStatus.CONFLICT,"設備資料尚未就緒");}return s;}
 JsonNode chamber(JsonNode s,String pm){for(var p:s.path("chambers"))if(p.path("id").asText().equals(pm))return p;fail(HttpStatus.BAD_REQUEST,"PM 不存在");return null;}
 JsonNode wafer(JsonNode s,String id){for(var l:s.path("lots"))for(var w:l.path("wafers"))if(w.path("id").asText().equals(id))return w;fail(HttpStatus.CONFLICT,"晶圓不存在");return null;}
 void lock(String equipment){db.pg.queryForObject("SELECT pg_advisory_xact_lock(hashtextextended(?,81344022))",Object.class,equipment);}
 ObjectNode prescription(ObjectNode b){
  var s=snapshot();var legacy=s.path("incidents").path(b.path("incidentId").asText());if(legacy.path("legacy").asBoolean()&&legacy.path("returned").asBoolean()&&!legacy.path("released").asBoolean()&&legacy.path("status").asText().equals("awaiting_ee"))return legacyReturnPrescription(s,b,legacy);var pm=chamber(s,b.path("pmId").asText());
  JsonNode incident=s.path("incidents").path(b.path("incidentId").asText());
  if(incident.isMissingNode())for(var i:s.path("incidents"))if(!i.path("released").asBoolean()&&i.path("pmId").equals(pm.path("id"))&&i.path("waferId").equals(pm.path("waferId"))){incident=i;break;}
  if(incident.isMissingNode()||incident.path("released").asBoolean()||!incident.path("status").asText().equals("awaiting_ee")||!incident.path("pmId").equals(pm.path("id")))fail(HttpStatus.CONFLICT,"請選待 EE 排錯的本機異常來源");
  String kind=b.path("dispositionType").asText();boolean empty=incident.path("waferId").asText().isEmpty();
  if(empty?!kind.equals("equipment"):!Set.of("supplement","scrap").contains(kind))fail(HttpStatus.BAD_REQUEST,"處置類型不符");
  ObjectNode n=db.mapper.createObjectNode();
  for(String key:List.of("cause","correctiveAction","verificationMethod","verificationResult")){String value=b.path(key).asText().strip();if(value.length()<3||value.length()>4000)fail(HttpStatus.BAD_REQUEST,"請填完整原因、修復措施、驗證方式與結果（各 3–4000 字）");n.put(key,value);}
  n.put("dispositionType",kind);n.put("incidentId",incident.path("id").asText());n.put("equipmentId",Equipment.current());n.put("pmId",pm.path("id").asText());n.put("waferId",incident.path("waferId").asText());n.put("lotId",incident.path("lotId").asText());n.put("sourceRunId",pm.path("processRunId").asText());
  var t=pm.path("processTelemetry");
  if(!empty){var w=wafer(s,incident.path("waferId").asText());if(!w.path("result").asText().equals("exception")||!w.path("location").equals(pm.path("id")))fail(HttpStatus.CONFLICT,"異常晶圓已離開 PM");n.put("exceptionVersion",w.path("exceptionVersion").asInt());}
  n.set("baseline",t.path("sampleThicknessNm").isArray()?t.path("sampleThicknessNm"):db.mapper.valueToTree(new double[5]));n.put("baselineThicknessNm",t.path("thicknessNm").asDouble());
  double rate=t.path("rootDepositionRateNmPerSecond").asDouble();if(rate<=0&&t.path("stepSeconds").path(2).asInt()>0)rate=t.path("targetThicknessNm").asDouble()/t.path("stepSeconds").path(2).asInt();n.put("rootRate",rate);n.put("modelRevision",t.path("modelRevision").asText());
  JsonNode steps=db.mapper.valueToTree(new int[4]);int total=0;double target=0;
  if(kind.equals("supplement")){
   steps=b.path("stepSeconds");if(!steps.isArray()||steps.size()!=4)fail(HttpStatus.BAD_REQUEST,"需四個固定順序步驟");
   for(var v:steps){if(!v.isIntegralNumber()||!v.canConvertToInt()||v.asInt()<0||v.asInt()>3600)fail(HttpStatus.BAD_REQUEST,"步驟秒數無效");total+=v.asInt();}
   target=b.path("targetThicknessNm").asDouble(Double.NaN);
   if(total<1||total>3600||!Double.isFinite(target)||target<=0||target>100000||rate<=0||!Double.isFinite(rate)||!t.path("sampleThicknessNm").isArray()||t.path("sampleThicknessNm").size()!=5)fail(HttpStatus.BAD_REQUEST,"補做秒數、目標或製程基線無效");
  }
  n.set("stepSeconds",steps);n.put("targetThicknessNm",target);n.put("totalSeconds",total);n.put("estimatedDeltaNm",rate*steps.path(2).asInt());n.put("troubleshooting",n.path("cause").asText()+" / "+n.path("correctiveAction").asText());return n;
 }

 boolean returnedWafer(JsonNode s,JsonNode w){for(var l:s.path("lots"))if(l.path("id").equals(w.path("lotId")))return l.path("lp").equals(w.path("location"))&&Set.of("success","scrap").contains(w.path("result").asText());return false;}
 ObjectNode legacyReturnPrescription(JsonNode s,ObjectNode b,JsonNode i){
  if(!b.path("dispositionType").asText().equals("legacy_return"))fail(HttpStatus.CONFLICT,"已回槽晶圓僅可確認既有回收事實，不得補做或再次報廢");
  var w=wafer(s,i.path("waferId").asText());if(!returnedWafer(s,w))fail(HttpStatus.CONFLICT,"既有回收事實已變更");
  ObjectNode n=db.mapper.createObjectNode();for(String key:List.of("cause","correctiveAction","verificationMethod","verificationResult")){String value=b.path(key).asText().strip();if(value.length()<3||value.length()>4000)fail(HttpStatus.BAD_REQUEST,"請補齊既有異常的四項排錯欄位");n.put(key,value);}
  n.put("equipmentId",Equipment.current());n.put("incidentId",i.path("id").asText());n.put("pmId",i.path("pmId").asText());n.put("waferId",w.path("id").asText());n.put("lotId",w.path("lotId").asText());n.put("exceptionVersion",w.path("exceptionVersion").asInt());n.put("dispositionType","legacy_return");n.put("legacyResult",w.path("result").asText());n.put("sourceRunId",i.path("sourceRunId").asText());n.putNull("baseline");n.putNull("baselineThicknessNm");n.set("stepSeconds",db.mapper.valueToTree(new int[4]));n.put("troubleshooting",n.path("cause").asText());return n;
 }

 void event(String id,int rev,String action,Object actor,Object details){db.pg.update("INSERT INTO recovery_events(request_id,revision,action,actor,details) VALUES(?,?,?,?::jsonb,?::jsonb)",id,rev,action,db.mapper.valueToTree(actor).toString(),db.mapper.valueToTree(details).toString());}
 void state(String id,int rev,String status,String reason){state(id,rev,status,reason,SystemMessages.code(reason));}
 void state(String id,int rev,String status,String reason,String code){db.pg.update("UPDATE recovery_requests SET status=?,reason=?,reason_code=?,updated_at=now() WHERE id=?",status,reason,code,id);}
 @GetMapping public List<Map<String,Object>> list(@RequestParam(defaultValue="")String waferId){return db.pg.queryForList("SELECT r.*,v.prescription::text prescription FROM recovery_requests r JOIN recovery_versions v ON v.request_id=r.id AND v.revision=r.revision WHERE r.equipment_id=? AND (?='' OR r.wafer_id=?) ORDER BY r.created_at DESC LIMIT 200",Equipment.current(),waferId,waferId).stream().map(this::decode).toList();}
 Map<String,Object> decode(Map<String,Object> m){var n=new LinkedHashMap<>(m);for(String k:List.of("prescription","author","actor","details","result","approved_by","evidence"))if(n.get(k) instanceof String s)n.put(k,db.parse(s));return n;}
 @GetMapping("/{id}") public Map<String,Object> detail(@PathVariable String id){var rows=db.pg.queryForList("SELECT * FROM recovery_requests WHERE id=? AND equipment_id=?",id,Equipment.current());if(rows.isEmpty())fail(HttpStatus.NOT_FOUND,"處置單不存在");var n=new LinkedHashMap<>(rows.getFirst());n.put("versions",db.pg.queryForList("SELECT revision,prescription::text,author::text,created_at FROM recovery_versions WHERE request_id=? ORDER BY revision",id).stream().map(this::decode).toList());n.put("events",db.pg.queryForList("SELECT revision,action,actor::text,details::text,time FROM recovery_events WHERE request_id=? ORDER BY id",id).stream().map(this::decode).toList());n.put("intents",db.pg.queryForList("SELECT execution_id,revision,published,approved_by::text,result::text FROM recovery_intents WHERE request_id=? ORDER BY revision",id).stream().map(this::decode).toList());if(n.get("incident_id")!=null){var incidents=db.pg.queryForList("SELECT source::text,observed::text FROM disposition_incidents WHERE equipment_id=? AND id=?",Equipment.current(),n.get("incident_id"));if(!incidents.isEmpty()){n.put("incidentSource",db.parse((String)incidents.getFirst().get("source")));n.put("incident",db.parse((String)incidents.getFirst().get("observed")));}n.put("confirmations",db.pg.queryForList("SELECT id,revision,result_version,execution_id,decision,actor::text,evidence::text,reason,created_at FROM disposition_confirmations WHERE request_id=? ORDER BY created_at",id).stream().map(this::decode).toList());}return n;}
 @PostMapping public Map<String,Object> create(@RequestBody ObjectNode body){authorize("EE");final String[] selected={null};access.tx.executeWithoutResult(tx->{lock(Equipment.current());currentActor();var p=prescription(body);var rows=db.pg.queryForList("SELECT * FROM recovery_requests WHERE equipment_id=? AND incident_id=? FOR UPDATE",Equipment.current(),p.path("incidentId").asText());if(rows.isEmpty())fail(HttpStatus.CONFLICT,"異常建單同步中，請稍後重試");var r=rows.getFirst();String id=(String)r.get("id");if(r.get("owner_id")!=null)fail(HttpStatus.CONFLICT,"異常已有提案人，請由原提案人修訂");selected[0]=id;int rev=((Number)r.get("revision")).intValue()+1;db.pg.update("UPDATE recovery_requests SET owner_id=?,revision=?,status='draft',reason='',reason_code='',updated_at=now() WHERE id=?",AccessSessions.actor().id(),rev,id);version(id,rev,p);event(id,rev,"draft",AccessSessions.actor(),p);});return detail(selected[0]);}

 void version(String id,int rev,ObjectNode p){p.put("requestId",id);p.put("revision",rev);db.pg.update("INSERT INTO recovery_versions(request_id,revision,prescription,author) VALUES(?,?,?::jsonb,?::jsonb)",id,rev,p.toString(),db.mapper.valueToTree(AccessSessions.actor()).toString());}
 @PostMapping("/{id}/{action}") public Map<String,Object> action(@PathVariable String id,@PathVariable String action,@RequestBody ObjectNode body){
  if(Set.of("approve","return").contains(action))authorize("PE");else authorize("EE");
  try{access.tx.executeWithoutResult(s->{lock(Equipment.current());currentActor();var rows=db.pg.queryForList("SELECT * FROM recovery_requests WHERE id=? AND equipment_id=? FOR UPDATE",id,Equipment.current());if(rows.isEmpty())fail(HttpStatus.NOT_FOUND,"處置單不存在");var r=rows.getFirst();int rev=((Number)r.get("revision")).intValue();String status=(String)r.get("status");var actor=AccessSessions.actor();
   if(body.path("revision").asInt()!=rev)fail(HttpStatus.CONFLICT,"版本已更新，請重新載入");
   boolean review=Set.of("approve","return").contains(action);
   if(review&&Objects.equals(r.get("owner_id"),actor.id()))fail(HttpStatus.FORBIDDEN,"提案人不可自審，即使已更換部門");
   if(!review&&!Objects.equals(r.get("owner_id"),actor.id()))fail(HttpStatus.FORBIDDEN,"僅原提案人可修改或撤回");
   ObjectNode p=(ObjectNode)db.parse(db.pg.queryForObject("SELECT prescription::text FROM recovery_versions WHERE request_id=? AND revision=?",String.class,id,rev));
   p.put("equipmentId",Equipment.current());
   switch(action){
    case "save", "revise" -> {
     if(!(action.equals("save")&&status.equals("draft"))&&!(action.equals("revise")&&Set.of("returned","withdrawn","failed","invalidated").contains(status)))fail(HttpStatus.CONFLICT,"需退回或設備確認撤回後才可修訂");
     var next=prescription(body);if(!next.path("waferId").asText().equals(r.get("wafer_id"))||!next.path("incidentId").asText().equals(r.get("incident_id")))fail(HttpStatus.CONFLICT,"修訂不可更換晶圓");
     // Every edit appends a version: even drafts retain an audit trail.
     event(id,rev,"superseded",actor,Map.of("nextRevision",rev+1));version(id,rev+1,next);db.pg.update("UPDATE recovery_requests SET revision=? WHERE id=?",rev+1,id);state(id,rev+1,"draft","");event(id,rev+1,"draft",actor,next);
    }
    case "submit" -> {if(!status.equals("draft"))fail(HttpStatus.CONFLICT,"僅草稿可送審");String block=baselineBlock(snapshot(),p);if(block!=null)fail(HttpStatus.CONFLICT,block);state(id,rev,"submitted","");event(id,rev,"submitted",actor,Map.of());}
    case "return" -> {if(!status.equals("submitted"))fail(HttpStatus.CONFLICT,"非待審版本");String reason=body.path("reason").asText().strip();if(reason.isEmpty()||reason.length()>4000)fail(HttpStatus.BAD_REQUEST,"請填退回說明");state(id,rev,"returned",reason,"");event(id,rev,"returned",actor,Map.of("reason",reason));}
    case "approve" -> {
     if(Set.of("approved_waiting","executing","awaiting_result","returning","resolved","completed","failed").contains(status))return;
     if(!status.equals("submitted"))fail(HttpStatus.CONFLICT,"非待審版本");String block=baselineBlock(snapshot(),p);if(block!=null)fail(HttpStatus.CONFLICT,block);
     String execution=UUID.randomUUID().toString();db.pg.update("INSERT INTO recovery_intents(execution_id,request_id,revision,approved_by) VALUES(?,?,?,?::jsonb)",execution,id,rev,db.mapper.valueToTree(actor).toString());state(id,rev,"approved_waiting","等待互鎖檢查");event(id,rev,"approved",actor,Map.of("executionId",execution));
    }
    case "withdraw" -> {
     if(Set.of("draft","submitted","returned").contains(status)){state(id,rev,"withdrawn","");event(id,rev,"withdrawn",actor,Map.of());}
     else if(status.equals("approved_waiting")){
      var intent=db.pg.queryForMap("SELECT * FROM recovery_intents WHERE request_id=? AND revision=?",id,rev);
      if(!(Boolean)intent.get("published")){state(id,rev,"withdrawn","");event(id,rev,"withdrawn",actor,Map.of("reason","尚未發布，交易已撤銷意圖"));}
      else{p.put("executionId",(String)intent.get("execution_id"));queue(p,"cancel-supplement",actor);state(id,rev,"cancelling","等待設備確認尚未開始；確認前不能修訂");event(id,rev,"cancel-requested",actor,Map.of());}
     }else if(!status.equals("withdrawn")&&!status.equals("cancelling"))fail(HttpStatus.CONFLICT,"已開始或結束，無法撤回");
    }
    default -> fail(HttpStatus.BAD_REQUEST,"未知動作");
   }
  });}catch(DuplicateKeyException e){fail(HttpStatus.CONFLICT,"同片已有活動處置單或版本衝突");}return detail(id);
 }
 String baselineBlock(JsonNode s,JsonNode p){
  if(!p.has("verificationResult")||!p.has("incidentId"))return "舊版本需補齊排錯欄位並重新送審";
  var incident=s.path("incidents").path(p.path("incidentId").asText());if(incident.isMissingNode()||incident.path("released").asBoolean()||!incident.path("status").asText().equals("awaiting_ee"))return "異常已變動";
  if(p.path("dispositionType").asText().equals("legacy_return")){var w=wafer(s,p.path("waferId").asText());return incident.path("legacy").asBoolean()&&incident.path("returned").asBoolean()&&returnedWafer(s,w)&&w.path("result").equals(p.path("legacyResult"))&&w.path("exceptionVersion").asInt()==p.path("exceptionVersion").asInt()?null:"既有回收事實已變更";}
  if(p.path("dispositionType").asText().equals("equipment"))return incident.path("waferId").asText().isEmpty()?null:"設備來源不符";
  if(p.has("equipmentId")&&!p.path("equipmentId").equals(s.path("equipmentId")))return "機台不符";
  try{var pm=chamber(s,p.path("pmId").asText());var w=wafer(s,p.path("waferId").asText());var t=pm.path("processTelemetry");
   if(!pm.path("waferId").equals(p.path("waferId"))||!w.path("result").asText().equals("exception")||!pm.path("processRunId").equals(p.path("sourceRunId"))||w.path("exceptionVersion").asInt()!=p.path("exceptionVersion").asInt()||(p.path("dispositionType").asText().equals("supplement")&&!t.path("sampleThicknessNm").equals(p.path("baseline"))))return "來源晶圓／膜厚基線已變動，需重新提案";
   return null;
  }catch(ResponseStatusException e){return "來源晶圓已離開或不存在";}
 }
 void queue(ObjectNode p,String type,Object actor){
  try{String raw=p.toString();String key=Files.readString(Path.of(keyFile)).strip();if(key.length()<32)throw new IllegalStateException("Recovery key too short");var mac=Mac.getInstance("HmacSHA256");mac.init(new SecretKeySpec(key.getBytes(StandardCharsets.UTF_8),"HmacSHA256"));String signature=HexFormat.of().formatHex(mac.doFinal((type+"\n"+raw).getBytes(StandardCharsets.UTF_8)));
   ObjectNode c=db.mapper.createObjectNode();String commandId=p.has("authorizationId")?p.path("authorizationId").asText():p.has("releaseId")?p.path("releaseId").asText():p.path("executionId").asText()+(type.equals("cancel-supplement")?"-cancel":"-execute");c.put("equipmentId",p.path("equipmentId").asText());c.put("commandId",commandId);c.put("type",type);c.put("targetId",p.path("pmId").asText());c.put("lotId",p.path("lotId").asText());c.put("expiresAt",Instant.now().plus(Duration.ofDays(3650)).toString());c.set("actor",db.mapper.valueToTree(actor));c.putObject("parameters").put("payload",raw).put("signature",signature);db.pg.update("INSERT INTO commands(id,body,status,equipment_id) VALUES(?,?::jsonb,'received',?) ON CONFLICT DO NOTHING",commandId,c.toString(),p.path("equipmentId").asText());
  }catch(Exception e){throw new IllegalStateException("無法建立已簽章補做指令",e);}
 }
 @Scheduled(fixedDelay=1000) void reconcile(){
  for(var row:db.pg.queryForList("SELECT id FROM recovery_requests WHERE status IN ('approved_waiting','executing','cancelling','awaiting_result','returning')")){
   try{access.tx.executeWithoutResult(tx->{
    String id=(String)row.get("id");var r=db.pg.queryForMap("SELECT * FROM recovery_requests WHERE id=? FOR UPDATE",id);String status=(String)r.get("status");if(!Set.of("approved_waiting","executing","cancelling","awaiting_result","returning").contains(status))return;
    int rev=((Number)r.get("revision")).intValue();var intent=db.pg.queryForMap("SELECT *,approved_by::text approver FROM recovery_intents WHERE request_id=? AND revision=?",id,rev);String execution=(String)intent.get("execution_id");ObjectNode p=(ObjectNode)db.parse(db.pg.queryForObject("SELECT prescription::text FROM recovery_versions WHERE request_id=? AND revision=?",String.class,id,rev));p.put("executionId",execution);p.put("equipmentId",(String)r.get("equipment_id"));
    JsonNode s;try{s=snapshot((String)r.get("equipment_id"));}catch(ResponseStatusException e){state(id,rev,status,"結果待確認：等待設備通訊恢復，保留原 executionId");return;}
    var device=s.path("recoveries").path(execution);
    if(!device.isMissingNode()){
     String ds=device.path("status").asText();String deviceReason=ds.equals("executing")&&s.path("paused").asBoolean()?"補做時鐘暫停：等待工程確認恢復":device.path("reason").asText();String next=ds.equals("cancelled")?"withdrawn":ds;
     if(Set.of("approved_waiting","executing","completed","failed","withdrawn","invalidated","awaiting_result","returning","resolved").contains(next)){
      if(Set.of("completed","failed").contains(next) && db.pg.queryForObject("SELECT count(*) FROM recovery_events WHERE request_id=? AND revision=? AND action='executing'",Long.class,id,rev)==0)event(id,rev,"executing",null,Map.of("startedAt",device.path("startedAt").asText(),"executionId",execution,"observedAfterCompletion",true));
      if(!status.equals(next)&&!(status.equals("cancelling")&&next.equals("approved_waiting"))){event(id,rev,next,null,device);state(id,rev,next,deviceReason,(ds.equals("executing")&&s.path("paused").asBoolean()?SystemMessages.code(deviceReason):device.path("reasonCode").asText()));}
      else if(!status.equals("cancelling"))state(id,rev,status,deviceReason,(ds.equals("executing")&&s.path("paused").asBoolean()?SystemMessages.code(deviceReason):device.path("reasonCode").asText()));
      db.pg.update("UPDATE recovery_intents SET result=?::jsonb WHERE execution_id=?",device.toString(),execution);return;
     }
    }
    if((Boolean)intent.get("published")){
     var cmds=db.pg.queryForList("SELECT status,result::text result FROM commands WHERE id=?",execution+"-execute");
     if(!cmds.isEmpty()&&"rejected".equals(cmds.getFirst().get("status"))&&!status.equals("cancelling")){state(id,rev,"invalidated","設備拒絕核准綁定："+cmds.getFirst().get("result"));event(id,rev,"invalidated",null,cmds.getFirst());}
     else state(id,rev,status,status.equals("cancelling")?"等待設備取消確認":"結果待確認：已發布，查詢原 executionId");return;
    }
    String block=baselineBlock(s,p);if(block!=null){state(id,rev,"invalidated",block);event(id,rev,"invalidated",null,Map.of("reason",block));return;}
    var pm=p.path("dispositionType").asText().equals("legacy_return")?db.mapper.createObjectNode():chamber(s,p.path("pmId").asText());String waiting=pm.path("fault").asBoolean()?"等待解除 PM 故障":s.path("paused").asBoolean()?"等待工程確認恢復補做時鐘":"";
    state(id,rev,status,waiting);if(!waiting.isEmpty())return;
    queue(p,p.path("dispositionType").asText().equals("scrap")?"wafer-scrap":p.path("dispositionType").asText().equals("equipment")?"equipment-check":p.path("dispositionType").asText().equals("legacy_return")?"legacy-return-check":"wafer-supplement",db.parse((String)intent.get("approver")));db.pg.update("UPDATE recovery_intents SET published=true WHERE execution_id=?",execution);event(id,rev,"published",null,Map.of("executionId",execution));
   });}catch(Exception e){org.slf4j.LoggerFactory.getLogger(getClass()).warn("Recovery reconciliation deferred for {}: {}",row.get("id"),e.getClass().getSimpleName());}
  }
 }
}
