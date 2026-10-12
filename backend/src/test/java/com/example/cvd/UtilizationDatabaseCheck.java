package com.example.cvd;
import com.fasterxml.jackson.databind.*;
import java.time.*;
import java.time.temporal.ChronoUnit;
import java.util.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
/** Disposable utilization_check databases only. */
public class UtilizationDatabaseCheck {
 static void check(boolean c,String m){if(!c)throw new AssertionError(m);}
 public static void main(String[] args)throws Exception{
  var pg=new JdbcTemplate(new DriverManagerDataSource("jdbc:postgresql://postgres:5432/utilization_check","cvd","cvd-local"));
  var mapper=new ObjectMapper();var db=new DataStore(pg,mapper,"jdbc:postgresql://timescaledb:5432/utilization_check","cvd","cvd-local");db.migrate();var store=new UtilizationStore(db);store.migrate();
  Instant start=Instant.now().minusSeconds(600).truncatedTo(ChronoUnit.MINUTES).plusSeconds(17);String id=Equipment.FIRST;
  java.util.function.BiFunction<Integer,Integer,JsonNode> event=(second,mask)->{
   var j=mapper.createObjectNode();j.put("equipmentId",id);j.put("quality","good");j.put("sourceEpoch","test");j.put("sequence",second+1);j.put("sourceTime",start.plusSeconds(second+1).toString());
   var u=j.putObject("payload").putObject("utilization");u.put("version",1);u.put("intervalStart",start.plusSeconds(second).toString());u.put("intervalEnd",start.plusSeconds(second+1).toString());var pm=u.putObject("pm");for(int n=1;n<=4;n++)pm.put("PM"+n,(mask&(1<<(n-1)))!=0);return j;
  };
  for(int i=119;i>=0;i--)store.ingest(event.apply(i,i<60?1:0));for(int i=0;i<120;i++)store.ingest(event.apply(i,i<60?1:0));
  var r=store.totals(start,start.plusSeconds(120)).getFirst();check(((Number)r.get("known")).intValue()==120,"coverage");check(((Number)r.get("busy")).intValue()==60,"busy");check(((Number)r.get("pm")).intValue()==60,"pm");
  var bounded=store.totals(start.plusSeconds(12),start.plusSeconds(92)).getFirst();check(((Number)bounded.get("known")).intValue()==80,"boundary double count");check(((Number)bounded.get("busy")).intValue()==48,"boundary active");
  store.ingest(event.apply(30,3));store.ingest(event.apply(30,1));r=store.totals(start,start.plusSeconds(120)).getFirst();check(((Number)r.get("known")).intValue()==119,"conflict not sticky");check(((Number)r.get("busy")).intValue()==59,"conflict busy");
  new UtilizationStore(db).ingest(event.apply(31,1));check(((Number)store.totals(start,start.plusSeconds(120)).getFirst().get("known")).intValue()==119,"restart duplicate");
  db.ts.update("INSERT INTO telemetry(time,id,body,equipment_id) VALUES(now(),'legacy','{\"payload\":{\"chambers\":[]}}',?)",id);store.ingest(mapper.readTree("{\"payload\":{\"chambers\":[]}}"));store.migrate();check(db.ts.queryForObject("SELECT count(*) FROM telemetry WHERE id='legacy'",Integer.class)==1,"legacy deleted");
  db.ts.execute("TRUNCATE utilization_second,utilization_minute");
  Instant end=Instant.now().truncatedTo(ChronoUnit.MINUTES),begin=end.minus(30,ChronoUnit.DAYS);
  db.ts.update("INSERT INTO utilization_minute(time,equipment_id,known,busy,pm) SELECT ?::timestamptz+n*interval '1 minute','TEL-CVDTIN-08A-'||lpad(tool::text,2,'0'),60,CASE WHEN ((n/60+tool)%5)>0 THEN 60 ELSE 0 END,((n/60+tool)%5)*60 FROM generate_series(0,43199) n CROSS JOIN generate_series(1,10) tool",begin.toString());
  db.ts.execute("ANALYZE utilization_minute");long t=System.nanoTime();var totals=store.totals(begin,end);long cold=(System.nanoTime()-t)/1000000;
  var expected=db.ts.queryForList("SELECT tool,sum(CASE WHEN ((n/3600+tool)%5)>0 THEN 1 ELSE 0 END) busy,sum((n/3600+tool)%5) pm FROM generate_series(0,2591999) n CROSS JOIN generate_series(1,10) tool GROUP BY tool");
  for(var row:expected){String tool="TEL-CVDTIN-08A-%02d".formatted(((Number)row.get("tool")).intValue());var actual=totals.stream().filter(x->tool.equals(x.get("equipment_id"))).findFirst().orElseThrow();check(((Number)actual.get("known")).longValue()==2592000,"30d coverage");for(String key:List.of("busy","pm"))check(((Number)actual.get(key)).longValue()==((Number)row.get(key)).longValue(),"raw/summary mismatch");}
  t=System.nanoTime();for(int i=0;i<10;i++)store.totals(begin,end);long warm=(System.nanoTime()-t)/10000000;
  var plan=db.ts.queryForList("EXPLAIN (ANALYZE,BUFFERS) "+UtilizationStore.TOTALS,begin.toString(),end.toString());
  System.out.println(mapper.writeValueAsString(Map.of("databaseChecks","passed","equivalentToolSeconds",25920000,"minuteRows",432000,"firstQueryMs",cold,"warmMeanMs",warm,"plan",plan)));
 }
}
