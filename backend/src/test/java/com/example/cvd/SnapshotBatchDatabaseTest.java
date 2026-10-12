package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import java.util.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.kafka.core.KafkaTemplate;
@EnabledIfEnvironmentVariable(named="CVD_EMAIL_TEST_DB",matches="jdbc:postgresql://localhost:15439/email_test")
class SnapshotBatchDatabaseTest {
 @Test void bulkSnapshotKeepsMonotonicVersionsAndConnectivity() {
  var pg=new JdbcTemplate(new DriverManagerDataSource(System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test"));
  pg.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public; CREATE TABLE equipment_snapshot(id text PRIMARY KEY,version bigint,body jsonb,updated_at timestamptz default now()); CREATE TABLE equipment_connectivity(equipment_id text PRIMARY KEY,quality text,source_time timestamptz)");
  var db=new DataStore(pg,new ObjectMapper(),System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test");
  try {
   var socket=mock(LiveSocket.class);var p=new Pipeline(db,mock(KafkaTemplate.class),mock(StringRedisTemplate.class),socket,"tcp://unused");
   String a="{\"equipmentId\":\"TEL-CVDTIN-08A-01\",\"version\":9,\"vacuumHold\":true}",b="{\"equipmentId\":\"TEL-CVDTIN-08A-02\",\"version\":4}";
   pg.update("INSERT INTO equipment_connectivity(equipment_id,quality) VALUES(?,?)",Equipment.FIRST,"offline");
   p.snapshots(List.of(a,b,a.replace("9","3").replace("true","false")));
   assertEquals(2,pg.queryForObject("SELECT count(*) FROM equipment_snapshot",Integer.class));
   assertEquals(true,db.parse(db.snapshot(Equipment.FIRST)).path("vacuumHold").asBoolean());
   verify(socket).broadcast(argThat(s->db.parse(s).path("equipmentId").asText().equals(Equipment.FIRST)&&db.parse(s).path("communicationQuality").asText().equals("offline")));
   p.snapshots(List.of(a.replace("9","2")));assertEquals(9L,pg.queryForObject("SELECT version FROM equipment_snapshot WHERE id=?",Long.class,Equipment.FIRST));
   verify(socket,times(2)).broadcast(anyString());
   pg.execute("DELETE FROM equipment_connectivity");
   String fresh="{\"equipmentId\":\"TEL-CVDTIN-08A-01\",\"quality\":\"good\",\"sourceTime\":\"2026-10-02T04:00:00Z\"}";
   String old=fresh.replace("good","unknown").replace("04:00:00","03:00:00");
   p.connectivity(List.of(fresh,old));p.connectivity(List.of(old));
   assertEquals("good",pg.queryForObject("SELECT quality FROM equipment_connectivity WHERE equipment_id=?",String.class,Equipment.FIRST));

  } finally {((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();}
 }
}
