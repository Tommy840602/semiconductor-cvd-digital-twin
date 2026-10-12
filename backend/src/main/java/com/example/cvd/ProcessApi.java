package com.example.cvd;

import java.time.*;
import java.util.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import org.springframework.http.HttpStatus;

@RestController
@RequestMapping("/api/equipment/{equipmentId}")
public class ProcessApi {
 final DataStore db;
 public ProcessApi(DataStore db) {this.db=db;}
 record Metric(String actual,String target,String unit,boolean lock) {}
 static final Map<String,Metric> METRICS=new LinkedHashMap<>();
 static {
  METRICS.put("temperature",new Metric("{temperature}","{targetTemperature}","°C",false));
  METRICS.put("pressure",new Metric("{pressure}","{targetPressure}","mbar",true));
  METRICS.put("flow",new Metric("{flow}","{targetFlow}","sccm",false));
  METRICS.put("power",new Metric("{processTelemetry,heaterPowerPct}",null,"%",false));
  METRICS.put("deviation",new Metric("{processTelemetry,temperatureDeviationC}",null,"°C",false));
  METRICS.put("valve",new Metric("{valveOpeningPct}",null,"%",true));
  METRICS.put("thickness",new Metric("{processTelemetry,thicknessNm}","{processTelemetry,targetThicknessNm}","nm",false));
  METRICS.put("baseline",new Metric("{processTelemetry,baselineThicknessNm}",null,"nm",false));
  METRICS.put("delta",new Metric("{processTelemetry,deltaThicknessNm}",null,"nm",false));
  METRICS.put("rate",new Metric("{processTelemetry,depositionRateNmPerSecond}",null,"nm/s",false));
  METRICS.put("uniformity",new Metric("{processTelemetry,uniformityPct}","{processTelemetry,uniformityLimitPct}","%",false));
  for(String gas:List.of("precursor","reactant","carrier","purge")) METRICS.put(gas,new Metric("{gasChannels,"+gas+",actual}","{gasChannels,"+gas+",target}","sccm",false));
 }
 static String number(String path){return path==null?"NULL::numeric":"(p#>>'"+path+"')::numeric";}
 @GetMapping(value="/telemetry",params="metric")
 public Map<String,Object> metric(@RequestParam String metric,@RequestParam(defaultValue="") String pmId,@RequestParam(defaultValue="") String llId,@RequestParam(defaultValue="") String waferId,@RequestParam(defaultValue="") String processRunId,@RequestParam(required=false) String from,@RequestParam(required=false) String to){
  Metric m=METRICS.get(metric);
  if(m==null || (!pmId.isEmpty()&&!llId.isEmpty()) || (!llId.isEmpty()&&!m.lock()))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"不支援的指標或設備組合");
  String id=llId.isEmpty()?pmId:llId;
  if(!id.matches(llId.isEmpty()?"PM[1-4]":"LL[1-3]"))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"請選擇設備");
  Instant end=to==null?Instant.now():Instant.parse(to),begin=from==null?end.minusSeconds(300):Instant.parse(from);
  Instant cutoff=Instant.now().minus(Duration.ofDays(30));if(begin.isBefore(cutoff))begin=cutoff;
  boolean aggregate=Duration.between(begin,end).getSeconds()>=3600;
  if(!begin.isBefore(end))return Map.of("metric",metric,"unit",m.unit(),"resolutionSeconds",1,"points",List.of());
  String source=llId.isEmpty()?"chambers":"locks";
  String cte="WITH points AS (SELECT time, p->>'id' equipment, nullif(p->>'processRunId','') run, p->>'step' phase, "+number(m.actual())+" actual, "+number(m.target())+" target FROM telemetry CROSS JOIN LATERAL jsonb_array_elements(body#>'{payload,"+source+"}') p WHERE equipment_id=? AND time>=?::timestamptz AND time<?::timestamptz AND p->>'id'=? AND (?='' OR p->>'waferId'=?) AND (?='' OR p->>'processRunId'=?)) ";
  String query=aggregate ? "SELECT time_bucket('1 minute',time) AS time,equipment,run,avg(actual) actual,avg(target) target,min(actual) min,max(actual) max,count(actual) AS count FROM points GROUP BY 1,2,3 ORDER BY 1,2,3 LIMIT 2001" : "SELECT time,equipment,run,phase,actual,target,actual min,actual max,CASE WHEN actual IS NULL THEN 0 ELSE 1 END AS count FROM points ORDER BY time LIMIT 2001";
  var rows=db.ts.queryForList(cte+query,Equipment.current(),begin.toString(),end.toString(),id,waferId,waferId,processRunId,processRunId);
  if(rows.size()>2000)throw new ResponseStatusException(HttpStatus.UNPROCESSABLE_ENTITY,"超過 2000 點，請縮短查詢時間");
  List<Map<String,Object>> points=new ArrayList<>();
  for(var row:rows){var point=new LinkedHashMap<String,Object>();point.put("time",((java.sql.Timestamp)row.get("time")).toInstant().toString());point.put("equipmentId",Equipment.current());point.put("moduleId",row.get("equipment"));point.put("processRunId",row.get("run"));point.put("phase",row.get("phase"));point.put("actual",row.get("actual"));point.put("target",row.get("target"));point.put("min",row.get("min"));point.put("max",row.get("max"));point.put("sampleCount",row.get("count"));point.put("unit",m.unit());points.add(point);}
  return Map.of("metric",metric,"unit",m.unit(),"resolutionSeconds",aggregate?60:1,"points",points);
 }
 @GetMapping("/process-history")
 public List<com.fasterxml.jackson.databind.JsonNode> history(@RequestParam(defaultValue="") String pmId,@RequestParam(defaultValue="") String waferId,@RequestParam(defaultValue="0") int page,@RequestParam(required=false) String from,@RequestParam(required=false) String to){
  Instant end=to==null?Instant.now():Instant.parse(to),begin=from==null?end.minusSeconds(300):Instant.parse(from),cutoff=Instant.now().minus(Duration.ofDays(30));if(begin.isBefore(cutoff))begin=cutoff;
  return db.pg.queryForList("SELECT body::text FROM history WHERE equipment_id=? AND kind IN ('process-phase-changed','process-result') AND time>=?::timestamptz AND time<?::timestamptz AND (?='' OR body#>>'{payload,pm}'=?) AND (?='' OR wafer_id=?) ORDER BY time,(body->>'sequence')::bigint,id LIMIT 100 OFFSET ?",String.class,Equipment.current(),begin.toString(),end.toString(),pmId,pmId,waferId,waferId,Math.max(0,Math.min(100000,page))*100).stream().map(db::parse).toList();
 }
 @ExceptionHandler(java.time.format.DateTimeParseException.class)
 @ResponseStatus(HttpStatus.BAD_REQUEST)
 public Map<String,String> invalid(){return Map.of("message","時間格式不正確");}
}
