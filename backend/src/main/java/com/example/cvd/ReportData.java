package com.example.cvd;

import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.*;
import java.time.*;
import java.util.*;
import java.security.MessageDigest;
import org.springframework.stereotype.Component;
import org.springframework.jdbc.core.JdbcTemplate;

/** Read-only adapter: no commands, acknowledgement or qualification writes. */
@Component
public class ReportData {
 final DataStore db; final StatisticsApi statistics;
 static final int MAX_ROWS=100000;
 final ThreadLocal<Integer> rowBudget=ThreadLocal.withInitial(()->MAX_ROWS);
 public ReportData(DataStore db,StatisticsApi statistics){this.db=db;this.statistics=statistics;}
 static List<String> equipment(String id){return id.equals("all")?Equipment.IDS:List.of(Equipment.require(id));}
 static Instant start(String period,Instant end){return UtilizationApi.start(period,end);}
 static String hash(JsonNode node){try{return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(node.toString().getBytes(java.nio.charset.StandardCharsets.UTF_8)));}catch(Exception e){throw new IllegalStateException(e);}}
 List<JsonNode> rows(JdbcTemplate jdbc,String sql,Object... args){var q=new JdbcTemplate(jdbc.getDataSource());q.setQueryTimeout(45);q.setMaxRows(rowBudget.get()+1);q.setFetchSize(500);var result=q.queryForList(sql,String.class,args);if(result.size()>rowBudget.get())throw new ReportFailure("limit-exceeded");rowBudget.set(rowBudget.get()-result.size());return result.stream().map(db::parse).toList();}
 ObjectNode collect(String id,String period,Instant end,String language){
  rowBudget.set(MAX_ROWS);try{return collectSnapshot(id,period,end,language);}finally{rowBudget.remove();}
 }
 ObjectNode collectSnapshot(String id,String period,Instant end,String language){
  var ids=equipment(id);Instant begin=start(period,end);var s=db.mapper.createObjectNode();s.put("schemaVersion",1).put("language",language).put("start",begin.toString()).put("end",end.toString()).put("timezone","Asia/Taipei").put("simulated",true);
  var sources=s.putArray("sources");var stats=db.mapper.valueToTree(statistics.report(period,end));var tools=s.putArray("equipment");for(var tool:stats.path("equipment"))if(ids.contains(tool.path("equipmentId").asText())){var copy=(ObjectNode)tool.deepCopy();for(var u:stats.path("utilization").path("equipment"))if(u.path("equipmentId").equals(tool.path("equipmentId")))copy.set("utilization",u);tools.add(copy);}
  int timelineRows=0;for(var tool:tools)timelineRows+=tool.path("timeline").size();rowBudget.set(MAX_ROWS-timelineRows);if(rowBudget.get()<0)throw new ReportFailure("limit-exceeded");
  s.set("resolutionSeconds",stats.path("resolutionSeconds"));sources.addObject().put("source","statistics-v1/utilization-v1").put("capturedAt",Instant.now().toString());
  var kla=s.putArray("kla");var vision=s.putArray("vision");var phm=s.putArray("phm");var pending=s.putArray("pending");long bucket=Math.max(60,(Duration.between(begin,end).getSeconds()+359)/360);s.put("phmResolutionSeconds",bucket).put("phmTrendSource","vacuum-event-observations");
  for(String eq:ids){
   var measures=rows(db.pg,"""
    SELECT jsonb_build_object('id',id,'equipmentId',equipment_id,'waferId',wafer_id,'sourceKey',source_key,'revision',revision,'completedAt',completed_at,'pm',source->>'pm','purpose',source->>'purpose','processRunId',source->>'processRunId','meanNm',result->'meanNm','minNm',result->'minNm','maxNm',result->'maxNm','uniformityPct',result->'uniformityPct','quality',result->>'quality','thicknessQuality',result->>'thicknessQuality','defectQuality',result->>'defectQuality','modelVersion',result->>'modelVersion','synthetic',result->'synthetic')::text
    FROM (SELECT DISTINCT ON(source_key) * FROM metrology_runs WHERE equipment_id=? AND completed_at<?::timestamptz AND result IS NOT NULL AND NOT expired ORDER BY source_key,revision DESC) m
    WHERE completed_at>=?::timestamptz ORDER BY completed_at,id
    """,eq,end.toString(),begin.toString());measures.forEach(kla::add);
   rows(db.pg,"""
    WITH latest AS (SELECT DISTINCT ON(measurement_id) * FROM vision_runs WHERE equipment_id=? AND completed_at<?::timestamptz AND NOT expired ORDER BY measurement_id,revision DESC)
    SELECT jsonb_build_object('id',j.id,'runId',v.id,'measurementId',v.measurement_id,'equipmentId',v.equipment_id,'waferId',v.wafer_id,'revision',v.revision,'completedAt',v.completed_at,'kind',j.kind,'purpose',v.source->>'purpose','mode',j.input->>'mode','datasetId',j.input->>'datasetId','datasetVersion',j.input->>'datasetVersion','quality',j.result->>'quality','algorithmVersion',j.result->>'algorithmVersion','candidates',j.result->'candidates','defectCount',jsonb_array_length(coalesce(j.result->'defects','[]'::jsonb)))::text
    FROM latest v JOIN vision_jobs j ON j.run_id=v.id WHERE v.completed_at>=?::timestamptz AND j.state='completed' AND j.result IS NOT NULL ORDER BY v.completed_at,j.id
    """,eq,end.toString(),begin.toString()).forEach(vision::add);
   var p=pending.addObject().put("equipmentId",eq);
   p.put("kla",db.pg.queryForObject("SELECT count(*) FROM metrology_runs WHERE equipment_id=? AND created_at>=?::timestamptz AND created_at<?::timestamptz AND (completed_at IS NULL OR completed_at>=?::timestamptz)",Long.class,eq,begin.toString(),end.toString(),end.toString()));
   p.put("vision",db.pg.queryForObject("SELECT count(*) FROM vision_runs WHERE equipment_id=? AND created_at>=?::timestamptz AND created_at<?::timestamptz AND (completed_at IS NULL OR completed_at>=?::timestamptz)",Long.class,eq,begin.toString(),end.toString(),end.toString()));
   var trends=rows(db.pg,"""
    WITH points AS (SELECT pump_id,floor(extract(epoch from time)/?)*? epoch,body#>'{payload,pump}' p
    FROM vacuum_events WHERE equipment_id=? AND body#>'{payload,pump}' IS NOT NULL AND pump_id IN ('LL1','LL2','LL3') AND time>=?::timestamptz AND time<?::timestamptz)
    SELECT jsonb_build_object('pumpId',pump_id,'epoch',epoch,'health',avg((p->>'health')::double precision),'pressureMin',min((p#>>'{cycle,pressure}')::double precision),'pressureMax',max((p#>>'{cycle,pressure}')::double precision),'samples',count(*))::text FROM points GROUP BY pump_id,epoch ORDER BY epoch,pump_id
    """,bucket,bucket,eq,begin.toString(),end.toString());
   var last=rows(db.ts,"""
    SELECT jsonb_build_object('pumpId',p->>'id','time',time,'health',p#>'{pump,health}','remainingCycles',p#>'{pump,remainingCycles}')::text
    FROM (SELECT time,body FROM telemetry WHERE equipment_id=? AND time>=?::timestamptz AND time<?::timestamptz ORDER BY time DESC LIMIT 1) t CROSS JOIN LATERAL jsonb_array_elements(body#>'{payload,locks}') p WHERE jsonb_exists(p,'pump')
    """,eq,begin.toString(),end.toString());
   var pumps=phm.addObject().put("equipmentId",eq).putArray("pumps");
   for(String pump:List.of("LL1","LL2","LL3")){
    var item=pumps.addObject().put("pumpId",pump);var counts=db.pg.queryForMap("SELECT count(*) FILTER(WHERE body#>>'{payload,pump,cycle,status}'='passed') passed,count(*) FILTER(WHERE body#>>'{payload,pump,cycle,status}'='failed') failed,count(*) FILTER(WHERE body#>>'{payload,pump,cycle,status}'='interrupted') interrupted FROM vacuum_events WHERE equipment_id=? AND pump_id=? AND kind='vacuum-result' AND time>=?::timestamptz AND time<?::timestamptz",eq,pump,begin.toString(),end.toString());item.set("counts",db.mapper.valueToTree(counts));long total=((Number)counts.get("passed")).longValue()+((Number)counts.get("failed")).longValue();if(total>0)item.put("passRate",100.0*((Number)counts.get("passed")).longValue()/total);else item.putNull("passRate");
    var trend=item.putArray("trend");for(var point:trends)if(point.path("pumpId").asText().equals(pump))trend.add(point);
    item.set("lastHistorical",last.stream().filter(x->x.path("pumpId").asText().equals(pump)).findFirst().orElse(db.mapper.nullNode()));
   }
  }
  sources.addObject().put("source","kla/vision revisions").put("capturedAt",Instant.now().toString());sources.addObject().put("source","vacuum-events (event trends)/telemetry (last observation)").put("capturedAt",Instant.now().toString());
  long count=kla.size()+vision.size();for(var eq:tools)count+=eq.path("timeline").size();for(var eq:phm)for(var pump:eq.path("pumps"))count+=pump.path("trend").size();if(count>MAX_ROWS)throw new ReportFailure("limit-exceeded");s.put("detailRows",count);
  buildMetrics(s);return s;
 }
 static void buildMetrics(ObjectNode s){
  var m=s.putObject("metrics");for(var eq:s.path("equipment")){String id=eq.path("equipmentId").asText();metric(m,id+".utilization",eq.path("utilization").path("utilization"),"%",id);metric(m,id+".pmUtilization",eq.path("utilization").path("pmUtilization"),"%",id);metric(m,id+".coverage",eq.path("coverage"),"%",id);for(String key:List.of("completed","scrapped","dummyCompleted","dummyScrapped"))metric(m,id+"."+key,eq.path("production").path(key),"wafer",id);}
  metric(m,"kla.count",JsonNodeFactory.instance.numberNode(s.path("kla").size()),"measurement","");metric(m,"vision.count",JsonNodeFactory.instance.numberNode(s.path("vision").size()),"inspection","");
  metric(m,"kla.meanNm",mean(s.path("kla"),"meanNm",s),"nm","");metric(m,"kla.uniformityPct",mean(s.path("kla"),"uniformityPct",s),"%","");
  long defects=0,review=0;for(var v:s.path("vision")){defects+=v.path("defectCount").asLong();if(v.path("quality").asText().equals("review"))review++;}metric(m,"vision.defects",JsonNodeFactory.instance.numberNode(defects),"region","");metric(m,"vision.review",JsonNodeFactory.instance.numberNode(review),"inspection","");
  for(var eq:s.path("phm"))for(var p:eq.path("pumps")){String id=eq.path("equipmentId").asText(),prefix=id+"."+p.path("pumpId").asText();metric(m,prefix+".health",p.path("lastHistorical").path("health"),"score",id);metric(m,prefix+".remainingCycles",p.path("lastHistorical").path("remainingCycles"),"simulated-cycle",id);metric(m,prefix+".passRate",p.path("passRate"),"%",id);}
 }
 static JsonNode mean(JsonNode rows,String key,ObjectNode s){double total=0;int n=0;for(var r:rows)if(r.path(key).isNumber()){total+=r.path(key).asDouble();n++;}return n==0?s.nullNode():JsonNodeFactory.instance.numberNode(total/n);}
 static void metric(ObjectNode m,String key,JsonNode value,String unit,String equipment){var x=m.putObject(key);x.set("value",value.isNumber()?value:x.nullNode());x.put("unit",unit).put("equipment",equipment);}
}
class ReportFailure extends RuntimeException {final String code;ReportFailure(String code){super(code);this.code=code;}}
