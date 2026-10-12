package com.example.cvd;

import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.ObjectNode;
import jakarta.annotation.PostConstruct;
import java.time.*;
import java.util.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/api/equipment/{equipmentId}/recipes")
public class RecipeApi {
 final DataStore db; final AccessStore access;
 public RecipeApi(DataStore db,AccessStore access){this.db=db;this.access=access;}
 @PostConstruct void migrate(){
  db.pg.execute("CREATE TABLE IF NOT EXISTS recipe_versions(id text PRIMARY KEY,equipment_id text NOT NULL,revision integer NOT NULL,settings jsonb NOT NULL,source_equipment text,source_version text,actor jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(equipment_id,revision))");
  db.pg.execute("CREATE TABLE IF NOT EXISTS recipe_heads(equipment_id text PRIMARY KEY,version_id text NOT NULL REFERENCES recipe_versions(id))");
 }
 void init(){if(db.pg.queryForObject("SELECT count(*) FROM recipe_heads",Integer.class)==10)return;access.tx.executeWithoutResult(tx->{db.pg.execute("SELECT pg_advisory_xact_lock(81344023)");
  var original=db.parse(db.snapshot(Equipment.FIRST));
  if(!original.has("processSeconds"))throw new ResponseStatusException(HttpStatus.CONFLICT,"等待原機台設定同步");
  ObjectNode settings=db.mapper.createObjectNode();var steps=original.path("recipeSteps");
  if(!steps.isArray()){int total=original.path("processSeconds").asInt(30),remaining=total-4;double[] weights={.2,.2,.45,.15};int[] values=new int[4];int used=0;for(int i=0;i<4;i++){values[i]=1+(int)Math.floor(remaining*weights[i]);used+=values[i];}var order=new ArrayList<>(List.of(0,1,2,3));order.sort((a,b)->Double.compare(remaining*weights[b]%1,remaining*weights[a]%1));for(int i=0;i<total-used;i++)values[order.get(i)]++;steps=db.mapper.valueToTree(values);}
  var obj=settings.putObject("stepSeconds");int i=0;for(String key:List.of("preheat","stabilize","deposition","purge"))obj.set(key,steps.get(i++));
  for(String id:Equipment.IDS){String v=id+"-recipe-1";db.pg.update("INSERT INTO recipe_versions(id,equipment_id,revision,settings,source_equipment,source_version,actor) VALUES(?,?,1,?::jsonb,?,?,?::jsonb) ON CONFLICT DO NOTHING",v,id,settings.toString(),id.equals(Equipment.FIRST)?null:Equipment.FIRST,id.equals(Equipment.FIRST)?null:Equipment.FIRST+"-recipe-1","{\"username\":\"migration\"}");db.pg.update("INSERT INTO recipe_heads VALUES(?,?) ON CONFLICT DO NOTHING",id,v);}
 });}
 @org.springframework.scheduling.annotation.Scheduled(fixedDelay=5000) void initializeDevices(){
  try{init();}catch(ResponseStatusException e){return;}
  for(String id:Equipment.IDS){var snapshot=db.parse(db.snapshot(id));if(!snapshot.has("sourceTime")||!snapshot.path("recipeVersion").asText().isBlank())continue;
   var row=db.pg.queryForMap("SELECT v.id,v.settings::text settings FROM recipe_heads h JOIN recipe_versions v ON v.id=h.version_id WHERE h.equipment_id=?",id);
   var command=db.mapper.createObjectNode();String cid=id+"-initialize-recipe";command.put("commandId",cid);command.put("equipmentId",id);command.put("type","configure");command.put("expiresAt",Instant.now().plus(Duration.ofDays(3650)).toString());var params=(ObjectNode)db.parse((String)row.get("settings"));params.put("recipeVersion",(String)row.get("id"));command.set("parameters",params);
   db.pg.update("INSERT INTO commands(id,equipment_id,body,status) VALUES(?,?,?::jsonb,'received') ON CONFLICT DO NOTHING",cid,id,command.toString());
  }
 }
 @GetMapping public Map<String,Object> list(){init();String id=Equipment.current();return Map.of("equipmentId",id,"currentVersion",db.pg.queryForObject("SELECT version_id FROM recipe_heads WHERE equipment_id=?",String.class,id),"versions",db.pg.queryForList("SELECT id,equipment_id,revision,source_equipment,source_version,created_at,settings::text settings,actor::text actor FROM recipe_versions WHERE equipment_id=? ORDER BY revision DESC",id).stream().map(r->{r.put("settings",db.parse((String)r.get("settings")));r.put("actor",db.parse((String)r.get("actor")));return r;}).toList());}
 @PostMapping public Map<String,Object> save(@RequestBody ObjectNode body){
  String id=Equipment.current();var actor=AccessSessions.actor();if(!Set.of("Engineer","Leader").contains(actor.role())){access.audit(actor,"recipe",id,"","denied");throw new ResponseStatusException(HttpStatus.FORBIDDEN,"僅 Engineer／Leader 可修改 recipe");}init();
  String command=UUID.randomUUID().toString();
  try {access.tx.executeWithoutResult(tx->{
   String old=db.pg.queryForObject("SELECT version_id FROM recipe_heads WHERE equipment_id=? FOR UPDATE",String.class,id);
   if(db.parse(db.snapshot(id)).path("maintenance").path("phase").asText().equals("qualifying") || Boolean.TRUE.equals(db.pg.queryForObject("SELECT EXISTS(SELECT 1 FROM commands WHERE equipment_id=? AND body->>'type'='qualification-start' AND status IN ('received','accepted'))",Boolean.class,id)))throw new ResponseStatusException(HttpStatus.CONFLICT,"Dummy 驗證執行中或待執行，不可修改 recipe");
   if(!old.equals(body.path("expectedVersion").asText()))throw new ResponseStatusException(HttpStatus.CONFLICT,"Recipe 已更新，請重新載入");
   String source=body.path("sourceEquipment").asText(),version=body.path("sourceVersion").asText();JsonNode settings=body.path("settings");
   if(!source.isBlank()){Equipment.require(source);if(source.equals(id))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"請選擇另一機台");var rows=db.pg.queryForList("SELECT settings::text FROM recipe_versions WHERE equipment_id=? AND id=?",String.class,source,version);if(rows.isEmpty())throw new ResponseStatusException(HttpStatus.NOT_FOUND,"來源版本不存在");settings=db.parse(rows.getFirst());}
   validate(settings);int rev=db.pg.queryForObject("SELECT revision FROM recipe_versions WHERE id=?",Integer.class,old)+1;String next=id+"-recipe-"+rev;
   db.pg.update("INSERT INTO recipe_versions(id,equipment_id,revision,settings,source_equipment,source_version,actor) VALUES(?,?,?,?::jsonb,?,?,?::jsonb)",next,id,rev,settings.toString(),source.isBlank()?null:source,version.isBlank()?null:version,db.mapper.valueToTree(actor).toString());
   db.pg.update("UPDATE recipe_heads SET version_id=? WHERE equipment_id=?",next,id);
   ObjectNode c=db.mapper.createObjectNode();c.put("equipmentId",id);c.put("commandId",command);c.put("type","configure");c.put("expiresAt",Instant.now().plus(Duration.ofDays(3650)).toString());c.set("actor",db.mapper.valueToTree(actor));ObjectNode params=(ObjectNode)settings.deepCopy();params.put("recipeVersion",next);c.set("parameters",params);
   db.pg.update("INSERT INTO commands(id,equipment_id,body,status) VALUES(?,?,?::jsonb,'received')",command,id,c.toString());access.audit(actor,"recipe",id,command,"accepted");
  });}catch(ResponseStatusException ex){access.audit(actor,"recipe",id,command,"rejected");throw ex;}return Map.of("commandId",command,"recipe",list());
 }
 static void validate(JsonNode settings){var steps=settings.path("stepSeconds");int total=0;if(steps.size()!=4)throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"需要四個步驟");for(String key:List.of("preheat","stabilize","deposition","purge")){var v=steps.path(key);if(!v.isIntegralNumber()||!v.canConvertToInt()||v.asInt()<1||v.asInt()>3600)throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"各步驟需 1–3600 整數秒");total+=v.asInt();}if(total>3600)throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"總時間不得超過 3600 秒");}
}
