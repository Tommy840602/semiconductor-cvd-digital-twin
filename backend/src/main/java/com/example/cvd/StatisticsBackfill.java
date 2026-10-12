package com.example.cvd;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import jakarta.annotation.PostConstruct;
import java.time.*;
import java.util.*;
import org.springframework.stereotype.Component;
import org.springframework.scheduling.annotation.Scheduled;

/** Bounded replay of evidence; never establishes historical coverage from a snapshot. */
@Component
class StatisticsBackfill {
 final DataStore db;final StatisticsStore store;
 StatisticsBackfill(DataStore db,StatisticsStore store){this.db=db;this.store=store;}
 @PostConstruct void migrate(){db.pg.execute("CREATE TABLE IF NOT EXISTS statistics_backfill_attempt(equipment_id text NOT NULL,wafer_id text NOT NULL,attempted_at timestamptz NOT NULL,PRIMARY KEY(equipment_id,wafer_id))");}
 @Scheduled(initialDelay=15000,fixedDelay=5000) void run(){
  var candidates=db.pg.queryForList("SELECT s.id equipment_id,l->>'id' lot_id,l->>'lp' lp,l->>'purpose' purpose,w::text wafer,s.body->'incidents' incidents,s.body->>'statisticsEra' era FROM equipment_snapshot s CROSS JOIN LATERAL jsonb_array_elements(s.body->'lots') l CROSS JOIN LATERAL jsonb_array_elements(l->'wafers') w LEFT JOIN statistics_closures c ON c.equipment_id=s.id AND c.wafer_id=w->>'id' LEFT JOIN statistics_backfill_attempt a ON a.equipment_id=s.id AND a.wafer_id=w->>'id' WHERE (c.wafer_id IS NULL OR (w->>'closedAt' IS NOT NULL AND NOT EXISTS(SELECT 1 FROM statistics_receipts receipt WHERE receipt.equipment_id=s.id AND receipt.era=s.body->>'statisticsEra' AND receipt.seq=(w->>'closureSequence')::bigint))) AND w->>'result' IN ('success','scrap') AND w->>'location'=l->>'lp' AND (a.attempted_at IS NULL OR a.attempted_at<now()-interval '1 hour') ORDER BY a.attempted_at NULLS FIRST,s.id,w->>'id' LIMIT 40");
  for(var row:candidates){String id=(String)row.get("equipment_id");var w=db.parse((String)row.get("wafer"));String wafer=w.path("id").asText();
   // Native evidence is authoritative; wait for its event rather than duplicate it as legacy.
   if(w.hasNonNull("closedAt")){var e=nativeEvidence(row);if(e!=null)store.closed(e);}else{var e=reconstruct(row);if(e!=null&&StatisticsStore.closureValid(e))store.project(e);}
   db.pg.update("INSERT INTO statistics_backfill_attempt VALUES(?,?,now()) ON CONFLICT(equipment_id,wafer_id) DO UPDATE SET attempted_at=excluded.attempted_at",id,wafer);
  }
 }
 JsonNode nativeEvidence(Map<String,Object> row){
  var w=db.parse((String)row.get("wafer"));if(w.path("closureSequence").asLong()<1||row.get("era")==null)return null;
  ObjectNode e=db.mapper.createObjectNode();e.put("equipmentId",(String)row.get("equipment_id"));e.put("waferId",w.path("id").asText());e.put("lotId",(String)row.get("lot_id"));e.put("kind","wafer-closed");e.put("sourceTime",w.path("closedAt").asText());var p=e.putObject("payload");p.put("version",1);p.put("era",(String)row.get("era"));p.put("closureSequence",w.path("closureSequence").asLong());p.put("purpose",w.path("purpose").asText());p.put("result",w.path("result").asText());p.set("closedAt",w.path("closedAt"));p.set("returnedAt",w.path("lastReturnAt"));p.put("incidentId","");
  for(var i:db.parse(Objects.toString(row.get("incidents"),"{}"))){if(i.path("waferId").equals(w.path("id"))){if(!i.path("status").asText().equals("resolved"))return null;p.set("incidentId",i.path("id"));p.set("confirmationId",i.path("confirmationId"));p.set("confirmedAt",i.path("confirmedAt"));if(!w.hasNonNull("lastReturnAt"))p.set("returnedAt",i.path("returnedAt"));break;}}
  return e;
 }
 JsonNode reconstruct(Map<String,Object> row){
  String id=(String)row.get("equipment_id"),lp=(String)row.get("lp");var w=db.parse((String)row.get("wafer"));String wafer=w.path("id").asText(),purpose=(String)row.get("purpose");
  if(!List.of("production","dummy").contains(purpose==null?"":purpose))return null;
  var returns=db.pg.queryForList("SELECT body::text FROM history WHERE equipment_id=? AND wafer_id=? AND kind='movement-complete' AND body#>>'{payload,to}'=? ORDER BY time DESC LIMIT 1",String.class,id,wafer,lp);if(returns.isEmpty())return null;
  var returned=db.parse(returns.getFirst());Instant at=Instant.parse(returned.path("sourceTime").asText());String returnTime=at.toString();JsonNode incident=null;
  var incidents=db.parse(Objects.toString(row.get("incidents"),"{}"));for(var i:incidents){if(i.path("waferId").asText().equals(wafer)){incident=i;break;}}
  String confirmation="",confirmed="",incidentId="";
  if(incident!=null){
   if(!incident.path("status").asText().equals("resolved")||!incident.path("returned").asBoolean()||incident.path("confirmationId").asText().isBlank())return null;
   confirmation=incident.path("confirmationId").asText();incidentId=incident.path("id").asText();
   var confirmations=db.pg.queryForList("SELECT body::text FROM history WHERE equipment_id=? AND wafer_id=? AND kind='disposition-confirm' AND body#>>'{payload,incident,confirmationId}'=? ORDER BY time LIMIT 1",String.class,id,wafer,confirmation);if(confirmations.isEmpty())return null;
   confirmed=db.parse(confirmations.getFirst()).path("sourceTime").asText();Instant ct=Instant.parse(confirmed);if(ct.isAfter(at))at=ct;
  }else{
   if(w.path("attempts").asInt()!=1||!w.path("result").asText().equals("success"))return null;
   if(!Boolean.TRUE.equals(db.pg.queryForObject("SELECT EXISTS(SELECT 1 FROM history WHERE equipment_id=? AND wafer_id=? AND kind='process-result' AND body#>>'{payload,complete}'='true' AND time<=?::timestamptz)",Boolean.class,id,wafer,returnTime)))return null;
  }
  ObjectNode e=db.mapper.createObjectNode();e.put("equipmentId",id);e.put("waferId",wafer);e.put("lotId",(String)row.get("lot_id"));e.put("kind","wafer-closed");e.put("sourceTime",at.toString());e.put("eventId","historical/"+id+"/"+wafer);var p=e.putObject("payload");p.put("version",1);p.put("purpose",purpose);p.put("result",w.path("result").asText());p.put("closedAt",at.toString());p.put("returnedAt",returnTime);p.put("incidentId",incidentId);p.put("confirmationId",confirmation);if(!confirmed.isEmpty())p.put("confirmedAt",confirmed);p.put("backfilled",true);return e;
 }
}
