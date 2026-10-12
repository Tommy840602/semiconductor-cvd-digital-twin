package com.example.cvd;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import com.fasterxml.jackson.databind.*;
import java.time.*;
import java.nio.file.*;
import static org.junit.jupiter.api.Assertions.*;
/** Explicit opt-in probe with server-enforced read-only sessions; never migrates or writes production. */
@EnabledIfEnvironmentVariable(named="CVD_REPORT_READONLY_PROBE",matches="true")
class ReportProductionReadCheck {
 @Test void tenToolsThirtyDays()throws Exception{String options="?options=-c%20default_transaction_read_only%3Don",pass=System.getenv("CVD_PROBE_DB_PASSWORD");var pg=new JdbcTemplate(new DriverManagerDataSource("jdbc:postgresql://localhost:15442/cvd"+options,"cvd",pass));var json=new ObjectMapper();var db=new DataStore(pg,json,"jdbc:postgresql://localhost:15443/cvd"+options,"cvd",pass);assertEquals("on",pg.queryForObject("SHOW default_transaction_read_only",String.class));assertEquals("on",db.ts.queryForObject("SHOW default_transaction_read_only",String.class));try{var statistics=new StatisticsApi(new StatisticsStore(db),new UtilizationApi(new UtilizationStore(db)));var data=new ReportData(db,statistics);Instant begin=Instant.now();var snapshot=data.collect("all","30d",Instant.now(),"zh-TW");double seconds=Duration.between(begin,Instant.now()).toMillis()/1000.0;assertEquals(10,snapshot.path("equipment").size());assertTrue(snapshot.path("detailRows").asInt()<=ReportData.MAX_ROWS);assertEquals(10,snapshot.path("phm").size());var proof=json.createObjectNode().put("readOnly",true).put("scope","all").put("period","30d").put("collectionSeconds",seconds).put("snapshotBytes",snapshot.toString().length()).put("detailRows",snapshot.path("detailRows").asInt()).put("klaRows",snapshot.path("kla").size()).put("visionRows",snapshot.path("vision").size()).put("usedHeapMB",(Runtime.getRuntime().totalMemory()-Runtime.getRuntime().freeMemory())/1024/1024);Path output=Path.of("../.runtime/report-production-read-proof.json");Files.writeString(output,proof.toPrettyString());System.out.println("REPORT_PROBE "+proof);Files.writeString(Path.of("target/report-production-snapshot.json"),snapshot.toString());}finally{((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();}}
}
