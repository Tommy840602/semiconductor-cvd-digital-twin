package com.example.cvd;

import jakarta.annotation.PostConstruct;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;
import org.springframework.context.annotation.DependsOn;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Component;
import org.springframework.http.HttpStatus;
import org.springframework.web.server.ResponseStatusException;

@Component
@DependsOn("dispositionApi")
public class NotificationStore {
 final AccessStore access; final DataStore db;
 public NotificationStore(AccessStore access, DataStore db){this.access=access;this.db=db;}
 @PostConstruct void initialize() throws Exception {
  String sql=new ClassPathResource("db/email-notifications.sql").getContentAsString(StandardCharsets.UTF_8);
  access.tx.executeWithoutResult(tx->{access.pg.execute("SELECT pg_advisory_xact_lock(81344022)");access.pg.execute(sql);});
 }
 static AccessSessions.Identity leader(){var actor=AccessSessions.actor();if(actor==null||!"Leader".equals(actor.role()))throw new ResponseStatusException(HttpStatus.FORBIDDEN,"僅 Leader 可管理通知");return actor;}
 static String address(String value){
  String s=value==null?"":value.strip().toLowerCase(Locale.ROOT);
  if(!s.isEmpty()&&(s.length()>254||!s.matches("[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+")))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"Email 格式不正確");
  return s;
 }
 static boolean eligible(String type,String role,String department){
  if(!Set.of("Engineer","Leader").contains(role==null?"":role))return false;
  return switch(type){case "fault","fault-cleared","offline","online" -> "EE".equals(department);case "review","result" -> "PE".equals(department);case "release" -> "Leader".equals(role)&&Set.of("EE","PE").contains(department==null?"":department);default -> false;};
 }
 void email(UUID id,String email,boolean enabled){
  var actor=leader();String normalized=address(email);
  if(enabled&&normalized.isEmpty())throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"啟用通知需填寫 Email");
  access.tx.executeWithoutResult(tx->{
   var rows=access.pg.queryForList("SELECT username FROM app_users WHERE id=? FOR UPDATE",id);
   if(rows.isEmpty())throw new ResponseStatusException(HttpStatus.NOT_FOUND,"帳號不存在");
   access.pg.update("UPDATE app_users SET email=?,email_enabled=?,email_version=email_version+1 WHERE id=?",normalized,enabled,id);
   access.audit(actor,"email-settings",rows.getFirst().get("username").toString(),"",enabled?"enabled":"disabled");
  });
 }
 void event(String id,String type,String equipment,String subject,long revision,Instant time){
  access.pg.update("INSERT INTO notification_events(id,type,equipment_id,subject_id,revision,source_time) VALUES(?,?,?,?,?,?::timestamptz) ON CONFLICT DO NOTHING",id,type,Equipment.require(equipment),subject,revision,time.toString());
 }
 void expand(){access.tx.executeWithoutResult(tx->{
  for(var event:access.pg.queryForList("SELECT * FROM notification_events WHERE status='pending' ORDER BY created_at LIMIT 100 FOR UPDATE SKIP LOCKED")){
   var grouped=new LinkedHashMap<String,List<Map<String,Object>>>();
   for(var user:access.pg.queryForList("SELECT id::text,email,email_version,auth_version,role,department FROM app_users WHERE email_enabled AND email<>''")){
    if(!eligible(event.get("type").toString(),(String)user.get("role"),(String)user.get("department")))continue;
    grouped.computeIfAbsent((String)user.get("email"),k->new ArrayList<>()).add(Map.of("id",user.get("id"),"emailVersion",user.get("email_version"),"authVersion",user.get("auth_version"),"role",user.get("role"),"department",user.get("department")));
   }
   for(var entry:grouped.entrySet())access.pg.update("INSERT INTO notification_deliveries(id,event_id,address,recipients) VALUES(?,?,?,?::jsonb) ON CONFLICT DO NOTHING",UUID.randomUUID(),event.get("id"),entry.getKey(),db.mapper.valueToTree(entry.getValue()).toString());
   access.pg.update("UPDATE notification_events SET status=?,reason=? WHERE id=?",grouped.isEmpty()?"skipped":"expanded",grouped.isEmpty()?"無符合部門、角色與收件設定的帳號":"",event.get("id"));
  }
 });}
 boolean recipientValid(Map<String,Object> job){
  for(var captured:db.parse(job.get("recipients").toString())){
   var rows=access.pg.queryForList("SELECT * FROM app_users WHERE id=?::uuid AND email_enabled AND email=? AND email_version=? AND auth_version=?",captured.path("id").asText(),job.get("address"),captured.path("emailVersion").asLong(),captured.path("authVersion").asLong());
   if(!rows.isEmpty()){var u=rows.getFirst();if(Objects.equals(u.get("role"),captured.path("role").asText())&&Objects.equals(u.get("department"),captured.path("department").asText())&&eligible(job.get("type").toString(),(String)u.get("role"),(String)u.get("department")))return true;}
  }return false;
 }
 Set<String> recipientLocales(Map<String,Object> job){
  var locales=new HashSet<String>();
  for(var captured:db.parse(job.get("recipients").toString())){
   var rows=access.pg.queryForList("SELECT locale,role,department FROM app_users WHERE id=?::uuid AND email_enabled AND email=? AND email_version=? AND auth_version=? FOR SHARE",captured.path("id").asText(),job.get("address"),captured.path("emailVersion").asLong(),captured.path("authVersion").asLong());
   if(!rows.isEmpty()){var u=rows.getFirst();if(Objects.equals(u.get("role"),captured.path("role").asText())&&Objects.equals(u.get("department"),captured.path("department").asText())&&eligible(job.get("type").toString(),(String)u.get("role"),(String)u.get("department")))locales.add((String)u.get("locale"));}
  }
  return locales;
 }
 void outcome(UUID id,String status,String reason,int delay){
  access.pg.update("UPDATE notification_deliveries SET status=?,reason=?,available_at=now()+make_interval(secs=>?),lease_until=null,updated_at=now() WHERE id=?",status,reason,delay,id);
  access.pg.update("INSERT INTO notification_attempts(delivery_id,result,reason) VALUES(?,?,?)",id,status,reason);
 }
}
