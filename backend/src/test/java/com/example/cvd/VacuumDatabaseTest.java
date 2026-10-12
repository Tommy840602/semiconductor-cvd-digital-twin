package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.web.context.request.*;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.*;
import java.time.*;
import java.util.*;
@EnabledIfEnvironmentVariable(named="CVD_EMAIL_TEST_DB",matches="jdbc:postgresql://localhost:15439/email_test")
class VacuumDatabaseTest {
 DataStore db;JdbcTemplate pg;VacuumApi api;ObjectMapper mapper=new ObjectMapper();
 @BeforeEach void setup(){pg=new JdbcTemplate(new DriverManagerDataSource(System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test"));pg.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public;CREATE TABLE equipment_snapshot(id text PRIMARY KEY,body jsonb);CREATE TABLE equipment_connectivity(equipment_id text PRIMARY KEY,quality text);CREATE TABLE telemetry(time timestamptz,id text,body jsonb,equipment_id text)");db=new DataStore(pg,mapper,System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test");api=new VacuumApi(db);api.init();var req=new MockHttpServletRequest("GET","/api/equipment/"+Equipment.FIRST+"/vacuum");RequestContextHolder.setRequestAttributes(new ServletRequestAttributes(req));pg.update("INSERT INTO equipment_snapshot VALUES(?,?::jsonb)",Equipment.FIRST,"{\"sourceTime\":\""+Instant.now()+"\",\"locks\":[]}");}
 @AfterEach void close(){RequestContextHolder.resetRequestAttributes();((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();}
 String event(String id,String tool,String state){var n=mapper.createObjectNode().put("eventId",id).put("equipmentId",tool).put("sourceTime",Instant.now().minusSeconds(1).toString()).put("sequence",1).put("kind","vacuum-result");n.putObject("payload").put("pumpId","LL1").putObject("pump").putObject("cycle").put("status",state);return n.toString();}
 @Test void independentEvidenceAndTrend(){String good=event("1",Equipment.FIRST,"passed");api.ingest(good);api.ingest(good);api.ingest(event("2",Equipment.FIRST,"failed"));api.ingest(event("3",Equipment.FIRST,"interrupted"));api.ingest(event("4",Equipment.IDS.get(1),"passed"));String sample="{\"payload\":{\"locks\":[{\"id\":\"LL1\",\"pressure\":12,\"pump\":{\"health\":60}}]}}";pg.update("INSERT INTO telemetry VALUES(now()-interval '1 second','t1',?::jsonb,?)",sample,Equipment.FIRST);var report=api.view("LL1","","");var counts=(Map<?,?>)report.get("counts");assertEquals(1L,counts.get("passed"));assertEquals(1L,counts.get("failed"));assertEquals(1L,counts.get("interrupted"));assertEquals(1,((List<?>)report.get("trend")).size());assertEquals(true,report.get("fresh"));assertEquals(4,pg.queryForObject("SELECT count(*) FROM vacuum_events",Integer.class));pg.execute("DELETE FROM telemetry");assertEquals(4,pg.queryForObject("SELECT count(*) FROM vacuum_events",Integer.class));}
 @Test void noDataAndStale(){var r=api.view("LL2","","");assertEquals(0L,((Map<?,?>)r.get("counts")).get("passed"));assertTrue(((List<?>)r.get("trend")).isEmpty());pg.execute("UPDATE equipment_snapshot SET body=jsonb_set(body,'{sourceTime}',to_jsonb((now()-interval '1 hour')::text))");assertEquals(false,api.view("LL1","","").get("fresh"));assertThrows(Exception.class,()->api.view("LL4","",""));assertThrows(Exception.class,()->api.view("LL1",Instant.now().minusSeconds(31*86400).toString(),Instant.now().toString()));}
}
