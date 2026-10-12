package com.example.cvd;

import com.fasterxml.jackson.databind.JsonNode;
import jakarta.annotation.PostConstruct;
import java.time.*;
import java.sql.Timestamp;
import java.util.*;
import org.springframework.stereotype.Component;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

@Component
public class StatisticsStore {
 final DataStore db;
 static final List<String> STATES=List.of("processing","idle","maintenance","dummy","fault","paused","unknown");
 StatisticsStore(DataStore db){this.db=db;}
 @PostConstruct void migrate(){
  db.pg.execute("CREATE TABLE IF NOT EXISTS statistics_backfill_attempt(equipment_id text NOT NULL,wafer_id text NOT NULL,attempted_at timestamptz NOT NULL,PRIMARY KEY(equipment_id,wafer_id))");
  db.pg.execute("CREATE TABLE IF NOT EXISTS statistics_closures(equipment_id text NOT NULL,wafer_id text NOT NULL,closed_at timestamptz NOT NULL,purpose text NOT NULL,outcome text NOT NULL,body jsonb NOT NULL,conflict boolean NOT NULL DEFAULT false,PRIMARY KEY(equipment_id,wafer_id))");
  db.pg.execute("CREATE INDEX IF NOT EXISTS statistics_closures_time ON statistics_closures(equipment_id,closed_at)");
  db.pg.execute("CREATE TABLE IF NOT EXISTS statistics_receipts(equipment_id text NOT NULL,era text NOT NULL,seq bigint NOT NULL,wafer_id text NOT NULL,signature text NOT NULL,PRIMARY KEY(equipment_id,era,seq))");
  db.pg.execute("CREATE TABLE IF NOT EXISTS statistics_eras(equipment_id text NOT NULL,era text NOT NULL,contiguous bigint NOT NULL DEFAULT 0,poisoned boolean NOT NULL DEFAULT false,PRIMARY KEY(equipment_id,era))");
  db.pg.execute("CREATE INDEX IF NOT EXISTS statistics_history_lookup ON history(equipment_id,wafer_id,kind,time) WHERE kind IN ('movement-complete','process-result','disposition-confirm','disposition-returned')");
  db.ts.execute("CREATE TABLE IF NOT EXISTS statistics_second(time timestamptz NOT NULL,equipment_id text NOT NULL,state text NOT NULL,flags integer NOT NULL,era text NOT NULL,counter bigint NOT NULL,valid boolean NOT NULL,PRIMARY KEY(equipment_id,time))");
  db.ts.execute("CREATE TABLE IF NOT EXISTS statistics_minute(time timestamptz NOT NULL,equipment_id text NOT NULL,state text NOT NULL,flags integer NOT NULL,era text NOT NULL,samples integer NOT NULL,counter bigint NOT NULL,valid integer NOT NULL,PRIMARY KEY(equipment_id,time,state,flags,era))");
  for(String table:List.of("statistics_second","statistics_minute")){
   db.ts.execute("SELECT create_hypertable('"+table+"',by_range('time'),if_not_exists=>true)");db.ts.execute("SELECT add_retention_policy('"+table+"',INTERVAL '30 days',if_not_exists=>true)");
  }
 }
 @org.springframework.scheduling.annotation.Scheduled(fixedDelay=3600000) void cleanup(){
  db.pg.update("UPDATE statistics_closures c SET body='{}' WHERE closed_at<now()-interval '30 days' AND body<>'{}'::jsonb AND NOT EXISTS(SELECT 1 FROM equipment_snapshot s,jsonb_array_elements(s.body->'lots') l WHERE s.id=c.equipment_id AND l->>'id'=c.body->>'lotId' AND l->>'phase'<>'complete')");
  db.pg.update("DELETE FROM statistics_backfill_attempt WHERE attempted_at<now()-interval '30 days'");
 }
 record Sample(Instant time,String state,int flags,String era,long counter){}
 static Sample sample(JsonNode e){
  var interval=UtilizationStore.sample(e);var s=e.path("payload").path("statistics");
  if(interval==null||s.path("version").asInt()!=1||!STATES.contains(s.path("state").asText())||!s.path("flags").isIntegralNumber()||s.path("flags").asInt()<0||s.path("flags").asInt()>66111||s.path("era").asText().isBlank()||!s.path("closureCount").isIntegralNumber()||s.path("closureCount").asLong()<0)return null;
  return new Sample(interval.start(),s.path("state").asText(),s.path("flags").asInt(),s.path("era").asText(),s.path("closureCount").asLong());
 }
 void ingest(JsonNode e){
  var s=sample(e);if(s==null||s.time.isBefore(Instant.now().minus(Duration.ofDays(30)))||s.time.plusSeconds(1).isAfter(Instant.now()))return;
  String id=Equipment.legacy(e.path("equipmentId").asText());var at=Timestamp.from(s.time);
  new TransactionTemplate(new DataSourceTransactionManager(db.ts.getDataSource())).executeWithoutResult(tx->{
   db.ts.queryForList("SELECT pg_advisory_xact_lock(hashtext(?))","statistics/"+id);
   var rows=db.ts.queryForList("SELECT * FROM statistics_second WHERE equipment_id=? AND time=?",id,at);
   String state=s.state;int flags=s.flags;String era=s.era;long counter=s.counter;boolean valid=true;
   if(!rows.isEmpty()){
    var old=rows.getFirst();if(old.get("state").equals(state)&&((Number)old.get("flags")).intValue()==flags&&old.get("era").equals(era)&&((Number)old.get("counter")).longValue()==counter)return;
    if(Boolean.FALSE.equals(old.get("valid")))return;
    state="unknown";flags=0;era="";counter=0;valid=false;
   }
   db.ts.update("INSERT INTO statistics_second VALUES(?,?,?,?,?,?,?) ON CONFLICT(equipment_id,time) DO UPDATE SET state=excluded.state,flags=excluded.flags,era=excluded.era,counter=excluded.counter,valid=excluded.valid",at,id,state,flags,era,counter,valid);
   db.ts.update("DELETE FROM statistics_minute WHERE equipment_id=? AND time=date_trunc('minute',?::timestamptz)",id,s.time.toString());
   db.ts.update("INSERT INTO statistics_minute SELECT date_trunc('minute',time),equipment_id,state,flags,era,count(*)::int,max(counter),count(*) FILTER(WHERE valid)::int FROM statistics_second WHERE equipment_id=? AND time>=date_trunc('minute',?::timestamptz) AND time<date_trunc('minute',?::timestamptz)+interval '1 minute' GROUP BY 1,2,3,4,5",id,s.time.toString(),s.time.toString());
  });
 }
 static boolean closureValid(JsonNode e){
  try{
   var p=e.path("payload");if(p.path("version").asInt()!=1||e.path("waferId").asText().isBlank()||!List.of("production","dummy").contains(p.path("purpose").asText())||!List.of("success","scrap").contains(p.path("result").asText()))return false;
   Instant at=Instant.parse(p.path("closedAt").asText()),returned=Instant.parse(p.path("returnedAt").asText());
   if(p.path("incidentId").asText().isBlank())return p.path("result").asText().equals("success")&&at.equals(returned)&&!at.isAfter(Instant.parse(e.path("sourceTime").asText()));
   Instant confirmed=Instant.parse(p.path("confirmedAt").asText());return !p.path("confirmationId").asText().isBlank()&&at.equals(returned.isAfter(confirmed)?returned:confirmed)&&!at.isAfter(Instant.parse(e.path("sourceTime").asText()));
  }catch(RuntimeException ex){return false;}
 }
 void closed(JsonNode e){
  if(!e.path("kind").asText().equals("wafer-closed")||!closureValid(e))return;
  String id=Equipment.legacy(e.path("equipmentId").asText()),wafer=e.path("waferId").asText();var p=e.path("payload");String era=p.path("era").asText();long seq=p.path("closureSequence").asLong();
  if(era.isBlank()||seq<1)return;
  new TransactionTemplate(new DataSourceTransactionManager(db.pg.getDataSource())).executeWithoutResult(tx->{
   db.pg.update("INSERT INTO statistics_eras(equipment_id,era) VALUES(?,?) ON CONFLICT DO NOTHING",id,era);
   var row=db.pg.queryForMap("SELECT * FROM statistics_eras WHERE equipment_id=? AND era=? FOR UPDATE",id,era);
   String signature=wafer+"/"+p.path("closedAt").asText()+"/"+p.path("purpose").asText()+"/"+p.path("result").asText();
   db.pg.update("INSERT INTO statistics_receipts VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING",id,era,seq,wafer,signature);
   boolean conflict=!signature.equals(db.pg.queryForObject("SELECT signature FROM statistics_receipts WHERE equipment_id=? AND era=? AND seq=?",String.class,id,era,seq));
   project(e);
   if(conflict)db.pg.update("UPDATE statistics_closures SET conflict=true WHERE equipment_id=? AND (wafer_id=? OR wafer_id=(SELECT wafer_id FROM statistics_receipts WHERE equipment_id=? AND era=? AND seq=?))",id,wafer,id,era,seq);
   conflict|=Boolean.TRUE.equals(db.pg.queryForObject("SELECT conflict FROM statistics_closures WHERE equipment_id=? AND wafer_id=?",Boolean.class,id,wafer));
   long contiguous=((Number)row.get("contiguous")).longValue();
   while(Boolean.TRUE.equals(db.pg.queryForObject("SELECT EXISTS(SELECT 1 FROM statistics_receipts WHERE equipment_id=? AND era=? AND seq=?)",Boolean.class,id,era,contiguous+1)))contiguous++;
   db.pg.update("UPDATE statistics_eras SET contiguous=?,poisoned=poisoned OR ? WHERE equipment_id=? AND era=?",contiguous,conflict,id,era);
  });
 }
 void project(JsonNode e){
  var p=e.path("payload");db.pg.update("INSERT INTO statistics_closures(equipment_id,wafer_id,closed_at,purpose,outcome,body) VALUES(?,?,?::timestamptz,?,?,?::jsonb) ON CONFLICT(equipment_id,wafer_id) DO UPDATE SET conflict=statistics_closures.conflict OR statistics_closures.closed_at<>excluded.closed_at OR statistics_closures.purpose<>excluded.purpose OR statistics_closures.outcome<>excluded.outcome",Equipment.legacy(e.path("equipmentId").asText()),e.path("waferId").asText(),p.path("closedAt").asText(),p.path("purpose").asText(),p.path("result").asText(),e.toString());
 }
 static final String BUCKETS="""
 WITH bounds AS (SELECT ?::timestamptz a,?::timestamptz b), points AS (
 SELECT time,equipment_id,state,flags,era,samples,counter,valid FROM statistics_minute,bounds WHERE time>=CASE WHEN a=date_trunc('minute',a) THEN a ELSE date_trunc('minute',a)+interval '1 minute' END AND time<date_trunc('minute',b)
 UNION ALL
 SELECT time,equipment_id,state,flags,era,1,counter,CASE WHEN valid THEN 1 ELSE 0 END FROM statistics_second,bounds WHERE time>=a AND time<b AND (time<CASE WHEN a=date_trunc('minute',a) THEN a ELSE date_trunc('minute',a)+interval '1 minute' END OR time>=date_trunc('minute',b))
 ) SELECT time_bucket(?::interval,time) bucket,equipment_id,state,flags,era,sum(samples) samples,max(counter) counter,sum(valid) valid FROM points GROUP BY 1,2,3,4,5 ORDER BY 1,2
 """;
 List<Map<String,Object>> buckets(Instant a,Instant b,int resolution){return db.ts.queryForList(BUCKETS,a.toString(),b.toString(),resolution+" seconds");}
 List<Map<String,Object>> closures(Instant a,Instant b){return db.pg.queryForList("SELECT equipment_id,purpose,outcome,count(*) FILTER(WHERE NOT conflict) amount,count(*) FILTER(WHERE conflict) conflicts FROM statistics_closures WHERE closed_at>=?::timestamptz AND closed_at<?::timestamptz GROUP BY 1,2,3",a.toString(),b.toString());}
 Set<String> conflictingTools(){return new HashSet<>(db.pg.queryForList("SELECT DISTINCT equipment_id FROM statistics_closures WHERE conflict",String.class));}
 Map<String,Long> receipts(){var out=new HashMap<String,Long>();for(var r:db.pg.queryForList("SELECT * FROM statistics_eras"))out.put(r.get("equipment_id")+"/"+r.get("era"),Boolean.TRUE.equals(r.get("poisoned"))?-1L:((Number)r.get("contiguous")).longValue());return out;}
}
