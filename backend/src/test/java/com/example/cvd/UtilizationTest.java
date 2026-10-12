package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.*;
import java.util.*;
import org.junit.jupiter.api.Test;
class UtilizationTest {
 @Test void formulas(){
  assertEquals(100.0,UtilizationApi.metric(3600,3600,3600,3600).get("utilization"));
  assertEquals(25.0,UtilizationApi.metric(3600,3600,3600,3600).get("pmUtilization"));
  var half=UtilizationApi.metric(3600,3600,1800,7200);assertEquals(50.0,half.get("utilization"));assertEquals(50.0,half.get("pmUtilization"));
  var gap=UtilizationApi.metric(36000,32400,32400,32400);assertEquals(90.0,gap.get("coverage"));assertNull(gap.get("utilization"));
  assertEquals("no-data",UtilizationApi.metric(0,0,0,0).get("quality"));
 }
 @Test void ranges(){
  Instant end=Instant.parse("2026-09-19T02:00:00Z");assertEquals(Instant.parse("2026-09-18T16:00:00Z"),UtilizationApi.start("today",end));
  for(var p:Map.of("24h",1,"7d",7,"30d",30).entrySet())assertEquals(end.minus(Duration.ofDays(p.getValue())),UtilizationApi.start(p.getKey(),end));
  assertThrows(org.springframework.web.server.ResponseStatusException.class,()->UtilizationApi.start("bad",end));
 }
 @Test void validatesNewAndOldSamples()throws Exception{
  var mapper=new ObjectMapper();var j=mapper.readTree("""
  {"quality":"good","sourceEpoch":"a","sequence":1,"sourceTime":"2026-09-19T00:00:01.123Z","payload":{"utilization":{"version":1,"intervalStart":"2026-09-19T00:00:00Z","intervalEnd":"2026-09-19T00:00:01Z","pm":{"PM1":true,"PM2":false,"PM3":false,"PM4":false}}}}
  """);assertEquals(1,UtilizationStore.sample(j).mask());
  ((com.fasterxml.jackson.databind.node.ObjectNode)j.path("payload").path("utilization").path("pm")).put("PM2","false");assertNull(UtilizationStore.sample(j));assertNull(UtilizationStore.sample(mapper.readTree("{\"payload\":{\"chambers\":[]}}")));
 }
 @Test void fleetDoesNotDropMissingTool(){
  var store=mock(UtilizationStore.class);Instant end=Instant.parse("2026-09-19T02:00:00Z");var rows=new ArrayList<Map<String,Object>>();
  for(String id:Equipment.IDS.subList(0,9))rows.add(Map.of("equipment_id",id,"known",3600L*24,"busy",3600L*24,"pm",3600L*24));
  when(store.totals(any(),any())).thenReturn(rows);var r=new UtilizationApi(store).report("24h",end);
  var fleet=(Map<?,?>)r.get("fleet");assertEquals(90.0,fleet.get("coverage"));assertNull(fleet.get("utilization"));assertEquals(10,((List<?>)r.get("equipment")).size());
 }
}
