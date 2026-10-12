package com.example.cvd;

import com.fasterxml.jackson.databind.JsonNode;
import jakarta.annotation.PostConstruct;
import java.sql.Timestamp;
import java.time.*;
import java.util.*;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;

/** Compact, idempotent wall-clock evidence; summaries are updated atomically with seconds. */
@Component
public class UtilizationStore {
 final DataStore db;
 public UtilizationStore(DataStore db){this.db=db;}
 @PostConstruct void migrate(){
  for(String ddl:List.of(
   "CREATE TABLE IF NOT EXISTS utilization_second(time timestamptz NOT NULL,equipment_id text NOT NULL,mask smallint NOT NULL CHECK(mask BETWEEN -1 AND 15),PRIMARY KEY(equipment_id,time))",
   "SELECT create_hypertable('utilization_second',by_range('time'),if_not_exists=>true)",
   "SELECT add_retention_policy('utilization_second',INTERVAL '30 days',if_not_exists=>true)",
   "CREATE TABLE IF NOT EXISTS utilization_minute(time timestamptz NOT NULL,equipment_id text NOT NULL,known integer NOT NULL,busy integer NOT NULL,pm integer NOT NULL,PRIMARY KEY(equipment_id,time))",
   "SELECT create_hypertable('utilization_minute',by_range('time'),if_not_exists=>true)",
   "SELECT add_retention_policy('utilization_minute',INTERVAL '30 days',if_not_exists=>true)"))db.ts.execute(ddl);
 }
 record Sample(Instant start, int mask){}
 static Sample sample(JsonNode event){
  var u=event.path("payload").path("utilization");
  if(u.path("version").asInt()!=1||!"good".equals(event.path("quality").asText())||event.path("sourceEpoch").asText().isBlank()||event.path("sequence").asLong()<=0)return null;
  try{
   Instant start=Instant.parse(u.path("intervalStart").asText()),end=Instant.parse(u.path("intervalEnd").asText()),source=Instant.parse(event.path("sourceTime").asText());
   if(start.getNano()!=0||end.getNano()!=0||!end.equals(start.plusSeconds(1))||source.isBefore(end)||source.isAfter(end.plusSeconds(1)))return null;
   if(!u.path("pm").isObject()||u.path("pm").size()!=4)return null;
   int mask=0;for(int i=1;i<=4;i++){var p=u.path("pm").path("PM"+i);if(!p.isBoolean())return null;if(p.asBoolean())mask|=1<<(i-1);}
   return new Sample(start,mask);
  }catch(java.time.format.DateTimeParseException e){return null;}
 }
 void ingest(JsonNode event){
  Sample s=sample(event);if(s==null)return;
  Instant now=Instant.now();if(s.start().isBefore(now.minus(Duration.ofDays(30)))||s.start().plusSeconds(1).isAfter(now))return;
  String id=Equipment.legacy(event.path("equipmentId").asText());
  new TransactionTemplate(new DataSourceTransactionManager(db.ts.getDataSource())).executeWithoutResult(tx->{
   db.ts.queryForList("SELECT pg_advisory_xact_lock(hashtext(?))","utilization/"+id);
   var old=db.ts.queryForList("SELECT mask FROM utilization_second WHERE equipment_id=? AND time=?",Integer.class,id,Timestamp.from(s.start()));
   Integer previous=old.isEmpty()?null:old.getFirst();
   int next=previous==null?s.mask():previous==s.mask()?previous:-1;
   if(previous!=null&&previous==next)return;
   db.ts.update("INSERT INTO utilization_second(time,equipment_id,mask) VALUES(?,?,?) ON CONFLICT(equipment_id,time) DO UPDATE SET mask=excluded.mask",Timestamp.from(s.start()),id,next);
   int known=(next<0?0:1)-(previous==null||previous<0?0:1);
   int busy=(next>0?1:0)-(previous!=null&&previous>0?1:0);
   int pm=(next<0?0:Integer.bitCount(next))-(previous==null||previous<0?0:Integer.bitCount(previous));
   db.ts.update("INSERT INTO utilization_minute(time,equipment_id,known,busy,pm) VALUES(date_trunc('minute',?::timestamptz),?,?,?,?) ON CONFLICT(equipment_id,time) DO UPDATE SET known=utilization_minute.known+excluded.known,busy=utilization_minute.busy+excluded.busy,pm=utilization_minute.pm+excluded.pm",s.start().toString(),id,known,busy,pm);
  });
 }
 // One statement gives all ten tools a consistent database snapshot. Only two boundary minutes use raw rows.
 static final String TOTALS="""
 WITH limits AS (SELECT ?::timestamptz a,?::timestamptz b),
 bounds AS (SELECT a,b, CASE WHEN a=date_trunc('minute',a) THEN a ELSE date_trunc('minute',a)+interval '1 minute' END lo,date_trunc('minute',b) hi FROM limits),
 totals AS (
 SELECT equipment_id,known,busy,pm FROM utilization_minute,bounds WHERE time>=lo AND time<hi
 UNION ALL
 SELECT equipment_id,CASE WHEN mask>=0 THEN 1 ELSE 0 END,CASE WHEN mask>0 THEN 1 ELSE 0 END,
 CASE WHEN mask>=0 THEN bit_count(mask::int::bit(4))::int ELSE 0 END
 FROM utilization_second,bounds WHERE time>=a AND time<b AND (time<lo OR time>=hi)
 ) SELECT equipment_id,sum(known) known,sum(busy) busy,sum(pm) pm FROM totals GROUP BY equipment_id
 """;
 List<Map<String,Object>> totals(Instant start,Instant end){return db.ts.queryForList(TOTALS,start.toString(),end.toString());}
}
