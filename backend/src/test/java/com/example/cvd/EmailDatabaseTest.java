package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.*;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.oauth2.jwt.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpServer;
import java.net.*;
import java.nio.file.*;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;

/** Only an explicitly supplied disposable PostgreSQL instance; never production. */
@EnabledIfEnvironmentVariable(named="CVD_EMAIL_TEST_DB",matches="jdbc:postgresql://localhost:15439/email_test")
class EmailDatabaseTest {
 JdbcTemplate pg;DataStore db;AccessStore access;NotificationStore store;EmailConfig config;DispositionApi disposition;NotificationWorker worker;OutlookMail mail;HttpServer server;
 AtomicInteger responseCode=new AtomicInteger(202),sendCount=new AtomicInteger(),tokenCount=new AtomicInteger();String lastBody;Path key,secret;AccessSessions.Identity leader;
 @BeforeEach void setup() throws Exception {
  var ds=new DriverManagerDataSource(System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test");pg=new JdbcTemplate(ds);pg.execute("DROP SCHEMA public CASCADE; CREATE SCHEMA public");
  pg.execute("CREATE TABLE auth_bootstrap(id integer primary key); INSERT INTO auth_bootstrap VALUES(1)");access=new AccessStore(pg,new DataSourceTransactionManager(ds),"");access.initialize();
  pg.execute("CREATE TABLE history(id text primary key,time timestamptz,kind text,body jsonb,equipment_id text); CREATE TABLE recovery_requests(id text primary key,equipment_id text,revision bigint,status text,updated_at timestamptz default now()); CREATE TABLE equipment_snapshot(id text primary key,body jsonb); CREATE TABLE equipment_connectivity(equipment_id text primary key,quality text,source_time timestamptz); CREATE TABLE equipment_releases(equipment_id text,generation bigint,status text)");
  db=new DataStore(pg,new ObjectMapper(),System.getenv("CVD_EMAIL_TEST_DB"),"cvd","email-test");store=new NotificationStore(access,db);store.initialize();
  key=Files.createTempFile("email-key", ".txt");secret=Files.createTempFile("email-secret", ".txt");Files.writeString(key,Base64.getEncoder().encodeToString(new byte[32]));Files.writeString(secret,"fake-secret");
  config=new EmailConfig(new MockEnvironment().withProperty("EMAIL_ENABLED","true").withProperty("OUTLOOK_CLIENT_ID","client-test").withProperty("OAUTH_REDIRECT_URI","http://localhost:18000/oauth/outlook").withProperty("EMAIL_TOKEN_KEY_FILE",key.toString()).withProperty("OUTLOOK_CLIENT_SECRET_FILE",secret.toString()));
  server=HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
  server.createContext("/send",ex->{sendCount.incrementAndGet();lastBody=new String(ex.getRequestBody().readAllBytes(),java.nio.charset.StandardCharsets.UTF_8);ex.getResponseHeaders().add("Retry-After","120");ex.sendResponseHeaders(responseCode.get(),-1);ex.close();});
  server.createContext("/token",ex->{tokenCount.incrementAndGet();ex.getRequestBody().readAllBytes();byte[] body="{\"access_token\":\"fake-token\",\"refresh_token\":\"fake-refresh\",\"scope\":\"Mail.Send\",\"expires_in\":3600}".getBytes();ex.sendResponseHeaders(200,body.length);ex.getResponseBody().write(body);ex.close();});server.start();
  String root="http://127.0.0.1:"+server.getAddress().getPort();mail=new OutlookMail(store,config,root+"/token",root+"/send",token->{throw new JwtException("untrusted");});
  disposition=spy(new DispositionApi(mock(RecoveryApi.class),db,access));doReturn(Map.of("eligible",false)).when(disposition).releaseView(anyString(),eq(false));
  worker=new NotificationWorker(store,config,mail,disposition);
  leader=new AccessSessions.Identity(UUID.randomUUID().toString(),"test.leader","Leader","EE",1);pg.update("INSERT INTO app_users(id,username,password_hash,role,department) VALUES(?::uuid,?,'unused','Leader','EE')",leader.id(),leader.username());actor(leader);
  pg.update("UPDATE mail_connection SET started_at=now()-interval '1 hour',enabled=true,status='connected',public_base_url='https://example.com',tokens=?",mail.encrypt("{\"access_token\":\"fake-token\",\"refresh_token\":\"fake-refresh\",\"expires_at\":"+Instant.now().plusSeconds(3600).getEpochSecond()+"}"));
 }
 @AfterEach void close() throws Exception {SecurityContextHolder.clearContext();if(server!=null)server.stop(0);if(db!=null)((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();if(key!=null)Files.deleteIfExists(key);if(secret!=null)Files.deleteIfExists(secret);}
 void actor(AccessSessions.Identity user){SecurityContextHolder.getContext().setAuthentication(new UsernamePasswordAuthenticationToken(user,null,List.of()));}
 UUID user(String name,String role,String dept,String address){UUID id=UUID.randomUUID();pg.update("INSERT INTO app_users(id,username,password_hash,role,department) VALUES(?,?,'unused',?,?)",id,name,role,dept);store.email(id,address,!address.isBlank());return id;}
 void alarm(String id,boolean active,Instant time,long sequence){pg.update("INSERT INTO history VALUES(?,?::timestamptz,'alarm',?::jsonb,?) ON CONFLICT DO NOTHING",id,time.toString(),"{\"sequence\":"+sequence+",\"payload\":{\"pm\":\"PM2\",\"active\":"+active+"}}",Equipment.FIRST);}
 long count(String table){return pg.queryForObject("SELECT count(*) FROM "+table,Long.class);}
 void pending(String id,String type){store.event(id,type,Equipment.FIRST,"PM2",0,Instant.now());store.expand();}
 @Test void localeDefaultsMigrationAndVersionIsolation() throws Exception {
  assertEquals("zh-TW",new PreferencesApi(access).get().get("locale"));
  long auth=pg.queryForObject("SELECT auth_version FROM app_users WHERE id=?::uuid",Long.class,leader.id());
  long email=pg.queryForObject("SELECT email_version FROM app_users WHERE id=?::uuid",Long.class,leader.id());
  new PreferencesApi(access).save(Map.of("locale","ja"));access.initialize();store.initialize();
  assertEquals("ja",new PreferencesApi(access).get().get("locale"));
  assertEquals(auth,pg.queryForObject("SELECT auth_version FROM app_users WHERE id=?::uuid",Long.class,leader.id()));
  assertEquals(email,pg.queryForObject("SELECT email_version FROM app_users WHERE id=?::uuid",Long.class,leader.id()));
  assertThrows(Exception.class,()->pg.update("UPDATE app_users SET locale='xx'"));
 }
 @Test void firstSendUsesLatestLocaleAndRetriesKeepSnapshot(){
  UUID ee=user("ee.locale","Engineer","EE","locale@outlook.com");pending("locale","fault");
  pg.update("UPDATE app_users SET locale='ja' WHERE id=?",ee);responseCode.set(429);worker.deliver();
  assertEquals("ja",pg.queryForObject("SELECT delivery_locale FROM notification_deliveries",String.class));
  assertTrue(lastBody.contains("CVD シミュレーション"));String first=lastBody;
  pg.update("UPDATE app_users SET locale='en' WHERE id=?",ee);pg.update("UPDATE mail_connection SET public_base_url='https://changed.example.com'");
  pg.update("UPDATE notification_deliveries SET available_at=now()");responseCode.set(400);worker.deliver();assertEquals(first,lastBody);
  UUID id=pg.queryForObject("SELECT id FROM notification_deliveries",UUID.class);new NotificationApi(store,mail,config,worker).retry(id,new NotificationApi.Retry(true));
  responseCode.set(202);worker.deliver();assertEquals(first,lastBody);worker.deliver();assertEquals(3,sendCount.get());
 }
 @Test void sharedAddressConflictAndOnlyStillEligibleCapturedRecipients(){
  UUID one=user("one.locale","Engineer","EE","shared@outlook.com"),two=user("two.locale","Leader","EE","shared@outlook.com");
  pg.update("UPDATE app_users SET locale='en' WHERE id=?",one);pg.update("UPDATE app_users SET locale='ja' WHERE id=?",two);
  pending("shared","fault");worker.deliver();assertEquals(1,sendCount.get());assertEquals("zh-TW",pg.queryForObject("SELECT delivery_locale FROM notification_deliveries",String.class));assertEquals("shared-address-conflict",pg.queryForObject("SELECT locale_reason FROM notification_deliveries",String.class));
  pending("second","fault");pg.update("UPDATE app_users SET email_enabled=false WHERE id=?",two);worker.deliver();assertEquals("en",pg.queryForObject("SELECT delivery_locale FROM notification_deliveries WHERE event_id='second'",String.class));assertTrue(lastBody.contains("CVD simulation platform"));
 }
 @Test void migrationRoutingReplayAndRollback() throws Exception {
  assertEquals("",pg.queryForObject("SELECT email FROM app_users WHERE id=?::uuid",String.class,leader.id()));assertFalse(pg.queryForObject("SELECT email_enabled FROM app_users WHERE id=?::uuid",Boolean.class,leader.id()));
  user("ee.one","Engineer","EE","SAME@outlook.com");user("ee.two","Leader","EE","same@outlook.com");user("pe.one","Engineer","PE","pe@outlook.com");user("operator","Operator","EE","operator@outlook.com");user("no.dept","Engineer",null,"none@outlook.com");
  Instant time=Instant.now();alarm("fault1",true,time,10);alarm("fault1",true,time,10);alarm("repeat",true,time.plusMillis(1),11);alarm("old",false,time.minusSeconds(1),9);assertEquals(1,count("notification_events"));
  assertThrows(RuntimeException.class,()->access.tx.executeWithoutResult(tx->{alarm("rolledback",false,time.plusMillis(2),12);throw new RuntimeException();}));assertEquals(1,count("notification_events"));assertEquals(0,pg.queryForObject("SELECT count(*) FROM history WHERE id='rolledback'",Long.class));
  alarm("clear",false,time.plusMillis(3),13);alarm("fault2",true,time.plusMillis(4),14);store.expand();store.expand();assertEquals(3,count("notification_deliveries"));assertEquals("same@outlook.com",pg.queryForObject("SELECT DISTINCT address FROM notification_deliveries",String.class));
  store.initialize();assertEquals(3,count("notification_deliveries"));assertEquals(6,count("app_users"));
 }
 @Test void phasesScrapAndStaleRecipients(){
  UUID pe=user("pe","Engineer","PE","pe@outlook.com");
  pg.update("INSERT INTO recovery_requests(id,equipment_id,revision,status) VALUES('scrap-case',?,1,'draft')",Equipment.FIRST);
  pg.update("UPDATE recovery_requests SET status='submitted',updated_at=now() WHERE id='scrap-case'");store.expand();
  // A periodic reason/timestamp update must not invalidate the same stage.
  pg.update("UPDATE recovery_requests SET updated_at=now() WHERE id='scrap-case'");worker.deliver();assertEquals(1,sendCount.get());assertTrue(lastBody.contains("scrap-case"));
  pg.update("UPDATE recovery_requests SET status='awaiting_result',updated_at=now() WHERE id='scrap-case'");store.expand();store.email(pe,"new@outlook.com",true);worker.deliver();assertEquals(1,sendCount.get());assertEquals(1,pg.queryForObject("SELECT count(*) FROM notification_deliveries WHERE status='cancelled'",Long.class));
  pg.update("UPDATE recovery_requests SET status='submitted',revision=2 WHERE id='scrap-case'");store.expand();pg.update("UPDATE recovery_requests SET status='resolved' WHERE id='scrap-case'");worker.deliver();assertEquals(1,sendCount.get());
 }
 @Test void graphRetryUnknownAndLeases() throws Exception {
  user("ee","Engineer","EE","ee@outlook.com");pending("a","fault");responseCode.set(429);worker.deliver();assertEquals("retry",pg.queryForObject("SELECT status FROM notification_deliveries",String.class));assertTrue(pg.queryForObject("SELECT available_at>now()+interval '100 seconds' FROM notification_deliveries",Boolean.class));
  pg.update("UPDATE notification_deliveries SET available_at=now()");responseCode.set(202);worker.deliver();assertEquals("accepted",pg.queryForObject("SELECT status FROM notification_deliveries",String.class));worker.deliver();assertEquals(2,sendCount.get());assertEquals(1,db.parse(lastBody).path("message").path("toRecipients").size());
  pending("b","fault");responseCode.set(400);worker.deliver();assertEquals(1,pg.queryForObject("SELECT count(*) FROM notification_deliveries WHERE status='failed'",Long.class));
  pending("c","fault");var pool=Executors.newFixedThreadPool(2);var jobs=pool.invokeAll(List.of(worker::claim,worker::claim));assertEquals(1,jobs.stream().filter(f->{try{return f.get()!=null;}catch(Exception e){throw new RuntimeException(e);}}).count());pool.shutdownNow();
  pg.update("UPDATE notification_deliveries SET lease_until=now()-interval '1 second' WHERE status='sending'");worker.deliver();assertEquals(1,pg.queryForObject("SELECT count(*) FROM notification_deliveries WHERE status='unknown'",Long.class));assertEquals(3,sendCount.get());
  var api=new NotificationApi(store,mail,config,worker);UUID id=pg.queryForObject("SELECT id FROM notification_deliveries WHERE status='unknown'",UUID.class);assertThrows(Exception.class,()->api.retry(id,new NotificationApi.Retry(false)));api.retry(id,new NotificationApi.Retry(true));responseCode.set(202);worker.deliver();assertEquals(4,sendCount.get());
  responseCode.set(401);assertEquals("retry",mail.send("x@outlook.com","test","test").status());assertEquals(1,tokenCount.get());assertEquals("needs_reauth",pg.queryForObject("SELECT status FROM mail_connection",String.class));
 }
 @Test void encryptionOauthStateAndAuthority(){
  String encrypted=mail.encrypt("private-token");assertFalse(encrypted.contains("private-token"));assertEquals("private-token",mail.decrypt(encrypted));assertThrows(Exception.class,()->mail.decrypt(encrypted.substring(0,encrypted.length()-2)+"xx"));
  String uri=mail.begin("original-session");assertTrue(uri.contains("code_challenge_method=S256"));assertFalse(uri.contains("fake-secret"));String state=Arrays.stream(URI.create(uri).getRawQuery().split("&")).filter(s->s.startsWith("state=")).findFirst().orElseThrow().substring(6);
  assertThrows(Exception.class,()->mail.complete("different-session",state,"code"));assertEquals(0,tokenCount.get());assertEquals(1,count("mail_oauth"));assertThrows(Exception.class,()->mail.complete("original-session",state,"code"));assertEquals(0,count("mail_oauth"));assertEquals(1,tokenCount.get());assertThrows(Exception.class,()->mail.complete("original-session",state,"code"));assertEquals(1,tokenCount.get());
 }
 @Test void outageBaselineAndRecovery(){
  Instant now=Instant.now();worker.connectivity(Equipment.FIRST,now);worker.connectivity(Equipment.FIRST,now.plusSeconds(120));assertEquals(0,count("notification_events"));
  snapshot(now.plusSeconds(121));worker.connectivity(Equipment.FIRST,now.plusSeconds(121));snapshot(now.plusSeconds(132));worker.connectivity(Equipment.FIRST,now.plusSeconds(132));
  worker.connectivity(Equipment.FIRST,now.plusSeconds(140));snapshot(now.plusSeconds(150));worker.connectivity(Equipment.FIRST,now.plusSeconds(150));assertEquals(0,count("notification_events"));
  worker.connectivity(Equipment.FIRST,now.plusSeconds(160));worker.connectivity(Equipment.FIRST,now.plusSeconds(221));worker.connectivity(Equipment.FIRST,now.plusSeconds(240));assertEquals(1,count("notification_events"));snapshot(now.plusSeconds(241));worker.connectivity(Equipment.FIRST,now.plusSeconds(241));snapshot(now.plusSeconds(252));worker.connectivity(Equipment.FIRST,now.plusSeconds(252));assertEquals(2,count("notification_events"));
 }
 @Test void expiryRoleChangeNoRecipientAndAttemptLimit(){
  pending("nobody","fault");assertEquals("skipped",pg.queryForObject("SELECT status FROM notification_events WHERE id='nobody'",String.class));
  UUID ee=user("ee","Engineer","EE","ee@outlook.com");pending("old","fault");pg.update("UPDATE notification_events SET source_time=now()-interval '25 hours' WHERE id='old'");worker.sweep();assertEquals("expired",pg.queryForObject("SELECT status FROM notification_deliveries",String.class));
  pending("changed","fault");pg.update("UPDATE app_users SET department='PE',auth_version=auth_version+1 WHERE id=?",ee);worker.deliver();assertEquals(0,sendCount.get());assertEquals(1,pg.queryForObject("SELECT count(*) FROM notification_deliveries WHERE status='cancelled'",Long.class));
  pg.update("UPDATE app_users SET department='EE' WHERE id=?",ee);pending("limited","fault");responseCode.set(429);for(int i=0;i<6;i++){pg.update("UPDATE notification_deliveries SET available_at=now()");worker.deliver();}assertEquals(5,sendCount.get());assertEquals("failed",pg.queryForObject("SELECT status FROM notification_deliveries WHERE event_id='limited'",String.class));
 }
 @Test void releaseGenerationAndPendingFence(){
  user("eeleader","Leader","EE","eeleader@outlook.com");user("peleader","Leader","PE","peleader@outlook.com");user("peengineer","Engineer","PE","peengineer@outlook.com");
  doReturn(Map.of("eligible",true,"generation",4L)).when(disposition).releaseView(Equipment.FIRST,false);worker.release(Equipment.FIRST,true);store.expand();assertEquals(0,count("notification_deliveries"));
  doReturn(Map.of("eligible",true,"generation",5L)).when(disposition).releaseView(Equipment.FIRST,false);worker.release(Equipment.FIRST,false);worker.release(Equipment.FIRST,false);store.expand();assertEquals(2,count("notification_deliveries"));
  pg.update("INSERT INTO equipment_releases VALUES(?,5,'pending')",Equipment.FIRST);worker.sweep();assertEquals(2,pg.queryForObject("SELECT count(*) FROM notification_deliveries WHERE status='cancelled'",Long.class));
 }
 @Test void signedOidcSuccessNonceRejectionAndPkce() throws Exception {
  var generator=java.security.KeyPairGenerator.getInstance("RSA");generator.initialize(2048);var pair=generator.generateKeyPair();
  var decoder=NimbusJwtDecoder.withPublicKey((java.security.interfaces.RSAPublicKey)pair.getPublic()).build();decoder.setJwtValidator(JwtValidators.createDefaultWithIssuer(OutlookMail.ISSUER));
  String root="http://127.0.0.1:"+server.getAddress().getPort();mail=new OutlookMail(store,config,root+"/token",root+"/send",decoder);
  String url=mail.begin("session"),state=Arrays.stream(URI.create(url).getRawQuery().split("&")).filter(v->v.startsWith("state=")).findFirst().orElseThrow().substring(6);
  var oauth=pg.queryForMap("SELECT * FROM mail_oauth");String verifier=mail.decrypt(oauth.get("verifier").toString());assertTrue(url.contains("code_challenge="+OutlookMail.hash(verifier)));
  var claims=new com.nimbusds.jwt.JWTClaimsSet.Builder().issuer(OutlookMail.ISSUER).subject("personal-user").audience("client-test").expirationTime(Date.from(Instant.now().plusSeconds(3600))).issueTime(new Date()).claim("nonce",oauth.get("nonce")).claim("preferred_username","sender@outlook.com").build();
  var jwt=new com.nimbusds.jwt.SignedJWT(new com.nimbusds.jose.JWSHeader(com.nimbusds.jose.JWSAlgorithm.RS256),claims);jwt.sign(new com.nimbusds.jose.crypto.RSASSASigner(pair.getPrivate()));
  server.removeContext("/token");server.createContext("/token",ex->{String body=new String(ex.getRequestBody().readAllBytes());assertTrue(body.contains("code_verifier="+verifier));byte[] response=db.mapper.writeValueAsBytes(Map.of("access_token","fake-token","refresh_token","fake-refresh","scope","Mail.Send","expires_in",3600,"id_token",jwt.serialize()));ex.sendResponseHeaders(200,response.length);ex.getResponseBody().write(response);ex.close();});
  mail.complete("session",state,"code");assertEquals("connected",pg.queryForObject("SELECT status FROM mail_connection",String.class));assertEquals("sender@outlook.com",pg.queryForObject("SELECT sender FROM mail_connection",String.class));assertEquals(0,sendCount.get());assertFalse(pg.queryForObject("SELECT tokens FROM mail_connection",String.class).contains("fake-token"));
  String next=mail.begin("session"),nextState=Arrays.stream(URI.create(next).getRawQuery().split("&")).filter(v->v.startsWith("state=")).findFirst().orElseThrow().substring(6);
  server.removeContext("/token");server.createContext("/token",ex->{ex.getRequestBody().readAllBytes();byte[] response=db.mapper.writeValueAsBytes(Map.of("access_token","fake-token","refresh_token","fake-refresh","scope","Mail.Send","expires_in",3600,"id_token",jwt.serialize()));ex.sendResponseHeaders(200,response.length);ex.getResponseBody().write(response);ex.close();});
  assertThrows(Exception.class,()->mail.complete("session",nextState,"code"));assertEquals(0,count("mail_oauth"));
 }
 @Test void networkAmbiguityAndSerializedRefresh() throws Exception {
  pg.update("UPDATE mail_connection SET tokens=?",mail.encrypt("{\"access_token\":\"expired\",\"refresh_token\":\"fake-refresh\",\"expires_at\":0}"));
  var pool=Executors.newFixedThreadPool(2);var tokens=pool.invokeAll(List.of(()->mail.accessToken(false),()->mail.accessToken(false)));assertEquals("fake-token",tokens.get(0).get());assertEquals("fake-token",tokens.get(1).get());assertEquals(1,tokenCount.get());pool.shutdownNow();
  server.removeContext("/send");server.createContext("/send",ex->{sendCount.incrementAndGet();ex.getRequestBody().readAllBytes();ex.close();});
  user("ee","Engineer","EE","ee@outlook.com");pending("uncertain","fault");worker.deliver();assertEquals("unknown",pg.queryForObject("SELECT status FROM notification_deliveries",String.class));int count=sendCount.get();worker.deliver();assertEquals(count,sendCount.get());
 }
 void snapshot(Instant now){pg.update("INSERT INTO equipment_snapshot VALUES(?,?::jsonb) ON CONFLICT(id) DO UPDATE SET body=excluded.body",Equipment.FIRST,"{\"sourceTime\":\""+now+"\",\"communicationQuality\":\"good\"}");}
}
