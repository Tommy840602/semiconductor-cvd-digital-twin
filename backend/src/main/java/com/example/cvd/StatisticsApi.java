package com.example.cvd;
import java.time.*;
import java.time.temporal.ChronoUnit;
import java.util.*;
import org.springframework.web.bind.annotation.*;

@RestController
public class StatisticsApi {
 final StatisticsStore store;final UtilizationApi utilization;
 StatisticsApi(StatisticsStore store,UtilizationApi utilization){this.store=store;this.utilization=utilization;}
 static LinkedHashMap<String,Long> empty(){var m=new LinkedHashMap<String,Long>();for(String s:StatisticsStore.STATES)m.put(s,0L);return m;}
 static List<Map<String,Object>> coalesce(Collection<Map<String,Object>> buckets){
  var out=new ArrayList<Map<String,Object>>();for(var b:buckets){var prev=out.isEmpty()?null:out.getLast();if(prev!=null&&prev.get("end").equals(b.get("start"))&&prev.get("state").equals(b.get("state"))&&prev.get("flags").equals(b.get("flags"))){prev.put("end",b.get("end"));@SuppressWarnings("unchecked")var dest=(Map<String,Long>)prev.get("states");@SuppressWarnings("unchecked")var source=(Map<String,Long>)b.get("states");source.forEach((k,n)->dest.merge(k,n,Long::sum));}else out.add(b);}return out;
 }
 static long number(Map<String,Object> r,String k){return ((Number)r.getOrDefault(k,0L)).longValue();}
 @GetMapping("/api/equipment/statistics") public Map<String,Object> get(@RequestParam(defaultValue="today")String period,@RequestParam(defaultValue="")String equipmentId){if(!equipmentId.isBlank())Equipment.require(equipmentId);return report(period,Instant.now().truncatedTo(ChronoUnit.SECONDS));}
 Map<String,Object> report(String period,Instant end){
  Instant start=UtilizationApi.start(period,end);int resolution=period.equals("7d")?300:period.equals("30d")?1800:60;long duration=Duration.between(start,end).getSeconds();
  var receipts=store.receipts();var rows=store.buckets(start,end,resolution);var counts=store.closures(start,end);var conflicts=store.conflictingTools();
  var equipment=new ArrayList<Map<String,Object>>();var fleetStates=empty();long totalStateKnown=0,totalProductionKnown=0;
  long first=Math.floorDiv(start.getEpochSecond(),resolution)*resolution;
  for(String id:Equipment.IDS){
   var buckets=new LinkedHashMap<Long,Map<String,Object>>();long productionKnown=0;
   for(long t=first;t<end.getEpochSecond();t+=resolution){long a=Math.max(t,start.getEpochSecond()),b=Math.min(t+resolution,end.getEpochSecond());var states=empty();states.put("unknown",b-a);var bucket=new LinkedHashMap<String,Object>();bucket.put("start",Instant.ofEpochSecond(a).toString());bucket.put("end",Instant.ofEpochSecond(b).toString());bucket.put("states",states);bucket.put("flags",new TreeSet<Integer>());buckets.put(t,bucket);}
   for(var r:rows){if(!id.equals(r.get("equipment_id")))continue;long t=((java.sql.Timestamp)r.get("bucket")).toInstant().getEpochSecond();var b=buckets.get(t);if(b==null)continue;
    @SuppressWarnings("unchecked") var states=(Map<String,Long>)b.get("states");String state=(String)r.get("state");long samples=number(r,"samples");
    if(!state.equals("unknown")){states.merge(state,samples,Long::sum);states.merge("unknown",-samples,Long::sum);}
    @SuppressWarnings("unchecked") var flags=(Set<Integer>)b.get("flags");flags.add(((Number)r.get("flags")).intValue());
    long delivered=receipts.getOrDefault(id+"/"+r.get("era"),0L);if(!r.get("era").equals("")&&delivered>=number(r,"counter"))productionKnown+=number(r,"valid");
   }
   var states=empty();for(var b:buckets.values()){@SuppressWarnings("unchecked") var values=(Map<String,Long>)b.get("states");values.forEach((s,n)->states.merge(s,n,Long::sum));var present=values.entrySet().stream().filter(x->x.getValue()>0).map(Map.Entry::getKey).toList();b.put("state",present.size()==1?present.getFirst():"mixed");}
   var production=new LinkedHashMap<String,Object>();production.put("completed",0L);production.put("scrapped",0L);production.put("dummyCompleted",0L);production.put("dummyScrapped",0L);long conflictCount=conflicts.contains(id)?1:0;
   for(var r:counts){if(!id.equals(r.get("equipment_id")))continue;String key=r.get("outcome").equals("success")?"completed":"scrapped";if(r.get("purpose").equals("dummy"))key=key.equals("completed")?"dummyCompleted":"dummyScrapped";production.put(key,number(r,"amount"));conflictCount+=number(r,"conflicts");}
   if(conflictCount>0)productionKnown=0;
   production.put("knownSeconds",productionKnown);production.put("coverage",duration>0?100.0*productionKnown/duration:0);production.put("quality",duration>0&&productionKnown==duration?"complete":"partial");production.put("conflicts",conflictCount);
   long known=duration-states.get("unknown");var tool=new LinkedHashMap<String,Object>();tool.put("equipmentId",id);tool.put("states",states);tool.put("timeline",coalesce(buckets.values()));tool.put("coverage",duration>0?100.0*known/duration:0);tool.put("production",production);equipment.add(tool);states.forEach((s,n)->fleetStates.merge(s,n,Long::sum));totalStateKnown+=known;totalProductionKnown+=productionKnown;
  }
  return Map.of("period",period,"start",start.toString(),"end",end.toString(),"computedAt",Instant.now().toString(),"timezone","Asia/Taipei","resolutionSeconds",resolution,"equipment",equipment,"fleet",Map.of("states",fleetStates,"durationSeconds",duration*10,"coverage",duration>0?100.0*totalStateKnown/(duration*10):0,"productionCoverage",duration>0?100.0*totalProductionKnown/(duration*10):0),"utilization",utilization.report(period,end));
 }
}
