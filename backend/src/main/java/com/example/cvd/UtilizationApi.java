package com.example.cvd;

import java.time.*;
import java.time.temporal.ChronoUnit;
import java.util.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

@RestController
public class UtilizationApi {
 final UtilizationStore store;
 public UtilizationApi(UtilizationStore store){this.store=store;}
 static Instant start(String period,Instant end){return switch(period){
  case "today" -> end.atZone(ZoneId.of("Asia/Taipei")).toLocalDate().atStartOfDay(ZoneId.of("Asia/Taipei")).toInstant();
  case "24h" -> end.minus(1,ChronoUnit.DAYS);case "7d" -> end.minus(7,ChronoUnit.DAYS);case "30d" -> end.minus(30,ChronoUnit.DAYS);
  default -> throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"無效統計區間");};}
 static Map<String,Object> metric(long duration,long known,long busy,long pm){
  var m=new LinkedHashMap<String,Object>();boolean complete=duration>0&&known==duration;
  m.put("durationSeconds",duration);m.put("knownSeconds",known);m.put("busySeconds",busy);m.put("pmBusySeconds",pm);
  m.put("coverage",duration>0?100.0*known/duration:0.0);
  m.put("quality",known==0?"no-data":complete?"complete":"partial");
  m.put("utilization",complete?100.0*busy/duration:null);m.put("pmUtilization",complete?25.0*pm/duration:null);return m;
 }
 @GetMapping("/api/equipment/utilization")
 public Map<String,Object> get(@RequestParam(defaultValue="today") String period){return report(period,Instant.now().truncatedTo(ChronoUnit.SECONDS));}
 Map<String,Object> report(String period,Instant end){
  Instant start=start(period,end);long duration=Duration.between(start,end).getSeconds();
  var rows=store.totals(start,end);var equipment=new ArrayList<Map<String,Object>>();long known=0,busy=0,pm=0;
  for(String id:Equipment.IDS){
   var r=rows.stream().filter(x->id.equals(x.get("equipment_id"))).findFirst().orElse(Map.of());
   long k=((Number)r.getOrDefault("known",0)).longValue(),b=((Number)r.getOrDefault("busy",0)).longValue(),p=((Number)r.getOrDefault("pm",0)).longValue();
   var m=metric(duration,k,b,p);m.put("equipmentId",id);equipment.add(m);known+=k;busy+=b;pm+=p;
  }
  return Map.of("period",period,"start",start.toString(),"end",end.toString(),"timezone","Asia/Taipei","computedAt",Instant.now().toString(),"fleet",metric(duration*10,known,busy,pm),"equipment",equipment);
 }
}
