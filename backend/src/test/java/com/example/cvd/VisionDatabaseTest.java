package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.*;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.*;
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.CompletableFuture;

@EnabledIfEnvironmentVariable(named="CVD_EMAIL_TEST_DB",matches="jdbc:postgresql://localhost:15439/email_test")
class VisionDatabaseTest {
 JdbcTemplate pg;DataStore db;VisionApi api;AccessStore access;KafkaTemplate<String,String> kafka;ObjectMapper mapper=new ObjectMapper();AccessSessions.Identity leader;Path catalog,images;
 @BeforeEach void setup()throws Exception{
  var ds=new DriverManagerDataSource(System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test");pg=new JdbcTemplate(ds);pg.execute("DROP SCHEMA public CASCADE;CREATE SCHEMA public;CREATE TABLE auth_bootstrap(id integer PRIMARY KEY);INSERT INTO auth_bootstrap VALUES(1)");access=new AccessStore(pg,new DataSourceTransactionManager(ds),"");access.initialize();db=new DataStore(pg,mapper,System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test");kafka=mock(KafkaTemplate.class);
  var metrology=new MetrologyApi(db,mock(Pipeline.class),access,kafka,false);metrology.init();catalog=Path.of("../vision/catalog").toAbsolutePath();images=Files.createTempDirectory("vision-test");api=new VisionApi(db,access,kafka,metrology,true,catalog.toString(),images.toString());api.init();
  leader=new AccessSessions.Identity(UUID.randomUUID().toString(),"fixture","Leader","EE",1);pg.update("INSERT INTO app_users(id,username,password_hash,role,department) VALUES(?::uuid,'fixture','unused','Leader','EE')",leader.id());actor(leader);
 }
 @AfterEach void clean()throws Exception{SecurityContextHolder.clearContext();if(images==null)return;try(var files=Files.list(images)){for(Path p:files.toList())Files.delete(p);}Files.delete(images);}
 void actor(AccessSessions.Identity a){SecurityContextHolder.getContext().setAuthentication(new UsernamePasswordAuthenticationToken(a,null,List.of()));}
 String source(String eq,String wafer){String id=UUID.randomUUID().toString();var s=mapper.createObjectNode().put("equipmentId",eq).put("waferId",wafer).put("pm","PM1").put("processRunId","run-"+wafer);pg.update("INSERT INTO metrology_runs(id,source_key,revision,equipment_id,wafer_id,source,state,completed_at) VALUES(?,?,1,?,?,?::jsonb,'completed',now())",id,id,eq,wafer,s.toString());return id;}
 JsonNode group(String measurement){return api.detail(api.list(measurement).getFirst().path("id").asText());}
 ObjectNode finish(JsonNode job,boolean review)throws Exception{String id=job.path("id").asText();var claim=api.claim(id);JsonNode in=claim.path("input");byte[] image=api.workerInput(id,claim.path("lease").asText());String hash=VisionApi.sha(image);Files.write(images.resolve(hash+".png"),image);
  var r=mapper.createObjectNode().put("kind",in.path("kind").asText()).put("inputHash",in.path("hash").asText()).put("overlayHash",hash).put("width",in.path("width").asInt()).put("height",in.path("height").asInt()).put("algorithmVersion","opencv-rules-v1").put("opencvVersion","4.12.0").put("quality",review?"review":"pass");r.putObject("parameters");r.putArray("candidates").add(review?"scratch":"none");r.putArray("defects");var e=mapper.createObjectNode().put("jobId",id).put("lease",claim.path("lease").asText()).put("state","completed");e.set("result",r);api.result(e.toString());return e;}
 @Test void tenToolsBoundaryAndReplay()throws Exception{
  for(String eq:Equipment.IDS)source(eq,eq);String old=source(Equipment.FIRST,"old");pg.update("UPDATE metrology_runs SET completed_at='2020-01-01' WHERE id=?",old);api.enqueue();api.enqueue();assertEquals(10,pg.queryForObject("SELECT count(*) FROM vision_runs",Integer.class));assertEquals(20,pg.queryForObject("SELECT count(*) FROM vision_jobs",Integer.class));assertTrue(api.list(old).isEmpty());assertEquals(11,pg.queryForObject("SELECT count(*) FROM metrology_runs WHERE state='completed'",Integer.class));verifyNoInteractions(kafka);
 }
 @Test void partialFailureLeaseReplayAndAlert()throws Exception{
  String mid=source(Equipment.FIRST,"w");api.enqueue();var g=group(mid);var first=g.path("jobs").get(0);var event=finish(first,true);api.result(event.toString());assertFalse(group(mid).path("alarm").asBoolean());var second=g.path("jobs").get(1);api.claim(second.path("id").asText());pg.update("UPDATE vision_jobs SET lease_until=now()-interval '1 minute',attempts=3 WHERE id=?",second.path("id").asText());api.dispatch();assertTrue(group(mid).path("alarm").asBoolean());assertEquals(1,api.alarms(Equipment.FIRST).size());assertEquals("failed",db.parse(pg.queryForObject("SELECT to_jsonb(j)::text FROM vision_jobs j WHERE id=?",String.class,second.path("id").asText())).path("state").asText());verifyNoInteractions(kafka);
 }
 @Test void frozenReanalysisRoleIdempotencyRetention()throws Exception{
  String mid=source(Equipment.FIRST,"w");api.enqueue();var g=group(mid);for(var j:g.path("jobs"))finish(j,true);String id=g.path("id").asText();var request=Map.of("requestId",UUID.randomUUID().toString());String next=api.reanalyze(id,request).get("id");assertEquals(next,api.reanalyze(id,request).get("id"));assertEquals(2,api.list(mid).size());var nextG=api.detail(next);for(var j:nextG.path("jobs")){assertTrue(g.path("jobs").findValues("input").contains(j.path("input")));finish(j,false);}
  actor(new AccessSessions.Identity(leader.id(),"fixture","Operator","EE",1));assertThrows(Exception.class,()->api.reanalyze(id,request));assertThrows(Exception.class,()->api.acknowledge(id));actor(leader);
  pg.update("UPDATE vision_runs SET completed_at=now()-interval '31 days' WHERE id=?",id);api.cleanup();assertFalse(api.detail(id).path("expired").asBoolean());api.acknowledge(id);api.cleanup();assertTrue(api.detail(id).path("expired").asBoolean());assertThrows(Exception.class,()->api.reanalyze(id,Map.of("requestId",UUID.randomUUID().toString())));assertThrows(Exception.class,()->api.image(g.path("jobs").get(0).path("id").asText(),"original"));assertFalse(api.detail(next).path("expired").asBoolean());
 }
 @Test void retriesAndMalformedResults()throws Exception{
  String mid=source(Equipment.FIRST,"w");api.enqueue();var g=group(mid);String id=g.path("jobs").get(0).path("id").asText();var c=api.claim(id);assertThrows(Exception.class,()->api.claim(id));pg.update("UPDATE vision_jobs SET lease_until=now()-interval '1 minute' WHERE id=?",id);when(kafka.send(anyString(),anyString(),anyString())).thenReturn(CompletableFuture.completedFuture(null));api.dispatch();var c2=api.claim(id);assertNotEquals(c.path("lease"),c2.path("lease"));api.result(mapper.createObjectNode().put("jobId",id).put("lease",c.path("lease").asText()).toString());assertEquals("running",pg.queryForObject("SELECT state FROM vision_jobs WHERE id=?",String.class,id));api.result(mapper.createObjectNode().put("jobId",id).put("lease",c2.path("lease").asText()).put("state","completed").toString());assertEquals("failed",pg.queryForObject("SELECT state FROM vision_jobs WHERE id=?",String.class,id));assertThrows(Exception.class,()->VisionApi.verifyImage(catalog,"../secret"));
 }
}
