package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.*;import java.sql.Timestamp;import java.util.*;
import org.junit.jupiter.api.Test;
class StatisticsTest {
 @Test void sampleVersionAndMalformedEvidence()throws Exception{
  var m=new ObjectMapper();var e=m.readTree("""
  {"quality":"good","sourceEpoch":"test","sequence":1,"sourceTime":"2026-09-20T00:00:01Z","payload":{"utilization":{"version":1,"intervalStart":"2026-09-20T00:00:00Z","intervalEnd":"2026-09-20T00:00:01Z","pm":{"PM1":false,"PM2":false,"PM3":false,"PM4":false}},"statistics":{"version":1,"state":"paused","flags":66048,"era":"era","closureCount":0}}}
  """);assertNotNull(StatisticsStore.sample(e));assertNull(StatisticsStore.sample(m.readTree("{}")));((com.fasterxml.jackson.databind.node.ObjectNode)e.path("payload").path("statistics")).put("state","nonsense");assertNull(StatisticsStore.sample(e));
 }
 @Test void closureRequiresLatestEvidence()throws Exception{
  var m=new ObjectMapper();var e=m.readTree("""
  {"waferId":"w","sourceTime":"2026-09-20T00:00:03Z","payload":{"version":1,"purpose":"production","result":"scrap","closedAt":"2026-09-20T00:00:02Z","returnedAt":"2026-09-20T00:00:01Z","confirmedAt":"2026-09-20T00:00:02Z","incidentId":"i","confirmationId":"pe"}}
  """);assertTrue(StatisticsStore.closureValid(e));((com.fasterxml.jackson.databind.node.ObjectNode)e.path("payload")).put("closedAt","2026-09-20T00:00:01Z");assertFalse(StatisticsStore.closureValid(e));
 }
 @Test void exactUnknownDenominatorAndIndependentClosureCoverage(){
  var store=mock(StatisticsStore.class);var util=mock(UtilizationApi.class);Instant end=Instant.parse("2026-09-20T00:02:00Z");when(util.report(any(),any())).thenReturn(Map.of());when(store.receipts()).thenReturn(Map.of());when(store.conflictingTools()).thenReturn(Set.of());when(store.closures(any(),any())).thenReturn(List.of());
  when(store.buckets(any(),any(),anyInt())).thenReturn(List.of(Map.of("bucket",Timestamp.from(end.minusSeconds(60)),"equipment_id",Equipment.FIRST,"state","unknown","flags",0,"era","era","samples",60L,"counter",0L,"valid",60L)));
  var report=new StatisticsApi(store,util).report("24h",end);var all=(List<Map<String,Object>>)report.get("equipment");assertEquals(10,all.size());var one=all.getFirst();var states=(Map<String,Long>)one.get("states");assertEquals(86400L,states.get("unknown"));assertEquals(60L,((Map<?,?>)one.get("production")).get("knownSeconds"));assertTrue(((Collection<?>)one.get("timeline")).size()<=1441);
  var fleet=(Map<String,Object>)report.get("fleet");assertEquals(864000L,((Map<String,Long>)fleet.get("states")).get("unknown"));
 }
}
