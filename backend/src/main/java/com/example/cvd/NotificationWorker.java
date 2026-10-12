package com.example.cvd;

import java.time.*;
import java.time.format.DateTimeFormatter;
import java.util.*;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class NotificationWorker {
 final NotificationStore store;final EmailConfig config;final OutlookMail mail;final DispositionApi disposition;
 public NotificationWorker(NotificationStore store,EmailConfig config,OutlookMail mail,DispositionApi disposition){this.store=store;this.config=config;this.mail=mail;this.disposition=disposition;}
 @jakarta.annotation.PostConstruct void initialize(){if(!config.publicBase.isBlank())store.access.pg.update("UPDATE mail_connection SET public_base_url=? WHERE id=1 AND public_base_url=''",EmailConfig.url(config.publicBase,false));}
 @Scheduled(fixedDelay=2000) void monitor(){
  if(!config.allowed)return;
  try{if(store.access.pg.queryForObject("SELECT started_at IS NOT NULL FROM mail_connection WHERE id=1",Boolean.class)){
   for(String eq:Equipment.IDS){connectivity(eq,Instant.now());release(eq,false);}
   store.expand();sweep();
  }}catch(Exception e){System.err.println("Email monitor 暫時無法處理；保留佇列重試");}
 }
 void baseline(){
  for(String eq:Equipment.IDS){
   var snapshot=store.db.parse(store.db.snapshot(eq));
   for(var pm:snapshot.path("chambers"))if(Set.of("PM1","PM2","PM3","PM4").contains(pm.path("id").asText()))store.access.pg.update("INSERT INTO notification_incidents(equipment_id,subject_id,active,source_time) VALUES(?,?,?,now()) ON CONFLICT DO NOTHING",eq,pm.path("id").asText(),pm.path("fault").asBoolean());
   connectivity(eq,Instant.now());
   boolean alreadyReady=snapshot.path("dispositionHold").asBoolean()&&!snapshot.path("paused").asBoolean();
   for(var pm:snapshot.path("chambers"))if(pm.path("fault").asBoolean())alreadyReady=false;
   for(var incident:snapshot.path("incidents"))if(!incident.path("released").asBoolean()&&(!incident.path("status").asText().equals("resolved")||!incident.path("returned").asBoolean()||incident.path("confirmationId").asText().isBlank()))alreadyReady=false;
   for(var recovery:snapshot.path("recoveries"))if(Set.of("approved_waiting","executing").contains(recovery.path("status").asText()))alreadyReady=false;
   if(alreadyReady){String id="release:"+eq+":"+snapshot.path("holdGeneration").asLong();store.event(id,"release",eq,"",snapshot.path("holdGeneration").asLong(),Instant.now());store.access.pg.update("UPDATE notification_events SET status='skipped',reason='首次啟用前的待辦基準' WHERE id=?",id);}
   else release(eq,true);
  }
 }
 static boolean fresh(com.fasterxml.jackson.databind.JsonNode snapshot,Instant now){try{return "good".equals(snapshot.path("communicationQuality").asText("good"))&&Duration.between(Instant.parse(snapshot.path("sourceTime").asText()),now).abs().compareTo(Duration.ofSeconds(5))<=0;}catch(Exception e){return false;}}
 void connectivity(String eq,Instant now){store.access.tx.executeWithoutResult(tx->{
  boolean bad=!fresh(store.db.parse(store.db.snapshot(eq)),now);
  int inserted=store.access.pg.update("INSERT INTO notification_incidents(equipment_id,subject_id,active,since) VALUES(?,'connection',?,?::timestamptz) ON CONFLICT DO NOTHING",eq,bad,now.toString());
  if(inserted>0){ // An already stale installation is a baseline, not a new outage.
   if(bad)store.access.pg.update("UPDATE notification_incidents SET round=-1 WHERE equipment_id=? AND subject_id='connection'",eq);
   return;
  }
  var row=store.access.pg.queryForMap("SELECT * FROM notification_incidents WHERE equipment_id=? AND subject_id='connection' FOR UPDATE",eq);
  boolean active=(Boolean)row.get("active"),notified=(Boolean)row.get("notified");long round=((Number)row.get("round")).longValue();Instant since=((java.sql.Timestamp)row.get("since")).toInstant();
  if(bad!=active){store.access.pg.update("UPDATE notification_incidents SET active=?,since=?::timestamptz WHERE equipment_id=? AND subject_id='connection'",bad,now.toString(),eq);since=now;}
  long duration=Duration.between(since,now).getSeconds();
  if(bad&&!notified&&round>=0&&duration>=config.offlineSeconds){round++;store.event("offline:"+eq+":"+round,"offline",eq,"",round,now);store.access.pg.update("UPDATE notification_incidents SET notified=true,round=? WHERE equipment_id=? AND subject_id='connection'",round,eq);}
  if(!bad&&duration>=config.recoverySeconds){if(notified)store.event("online:"+eq+":"+round,"online",eq,"",round,now);store.access.pg.update("UPDATE notification_incidents SET notified=false,round=greatest(round,0) WHERE equipment_id=? AND subject_id='connection'",eq);}
 });}
 void release(String eq,boolean baseline){
  try{store.access.tx.executeWithoutResult(tx->{
   disposition.recovery.lock(eq);var view=disposition.releaseView(eq,false);
   if(!Boolean.TRUE.equals(view.get("eligible")))return;
   long generation=((Number)view.get("generation")).longValue();String id="release:"+eq+":"+generation;
   store.event(id,"release",eq,"",generation,Instant.now());
   if(baseline)store.access.pg.update("UPDATE notification_events SET status='skipped',reason='首次啟用前的待辦基準' WHERE id=?",id);
  });}catch(org.springframework.web.server.ResponseStatusException ignored){/* stale snapshots cannot establish release readiness */}
 }
 String invalid(Map<String,Object> job){
  String type=job.get("type").toString(),eq=job.get("equipment_id").toString();long revision=((Number)job.get("revision")).longValue();
  if(Set.of("fault","fault-cleared","offline","online").contains(type)&&((java.sql.Timestamp)job.get("source_time")).toInstant().isBefore(Instant.now().minus(Duration.ofHours(24))))return "expired";
  if(Set.of("review","result").contains(type)){
   String expected=type.equals("review")?"submitted":"awaiting_result";
   if(store.access.pg.queryForObject("SELECT count(*) FROM recovery_requests WHERE id=? AND equipment_id=? AND revision=? AND status=? AND email_stage=?",Long.class,job.get("subject_id"),eq,revision,expected,job.get("stage"))==0)return "cancelled";
  }
  if(type.equals("release")){
   try{var view=disposition.releaseView(eq,false);if(!Boolean.TRUE.equals(view.get("eligible"))||((Number)view.get("generation")).longValue()!=revision)return "cancelled";}catch(Exception e){return "waiting";}
   if(store.access.pg.queryForObject("SELECT count(*) FROM equipment_releases WHERE equipment_id=? AND generation=? AND status='pending'",Long.class,eq,revision)>0)return "cancelled";
  }
  if(!store.recipientValid(job))return "cancelled";return "";
 }
 @Scheduled(fixedDelay=1000) void deliver(){
  try{
   store.access.tx.executeWithoutResult(tx->{for(var row:store.access.pg.queryForList("SELECT id FROM notification_deliveries WHERE status='sending' AND lease_until<now() FOR UPDATE SKIP LOCKED"))store.outcome((UUID)row.get("id"),"unknown","寄送程序失聯，需人工確認是否重試",0);});
   if(!config.allowed||!store.access.pg.queryForObject("SELECT enabled AND status='connected' FROM mail_connection WHERE id=1",Boolean.class))return;
   var job=claim();if(job==null)return;
   UUID id=(UUID)job.get("id");var content=Map.of("subject",job.get("rendered_subject").toString(),"text",job.get("rendered_body").toString());
   var result=mail.send(job.get("address").toString(),content.get("subject"),content.get("text"));
   store.access.tx.executeWithoutResult(tx->{String status=result.status();int attempts=((Number)job.get("attempts")).intValue();if(status.equals("retry")&&attempts>=5)status="failed";store.outcome(id,status,result.reason(),Math.max(result.delay(),(int)Math.pow(2,attempts)*15));});
  }catch(Exception e){System.err.println("Email worker 暫時無法處理；租約保留供恢復檢查");}
 }
 void sweep(){store.access.tx.executeWithoutResult(tx->{for(var job:store.access.pg.queryForList("SELECT d.*,e.type,e.equipment_id,e.subject_id,e.revision,e.stage,e.source_time FROM notification_deliveries d JOIN notification_events e ON e.id=d.event_id WHERE d.status IN ('queued','retry') ORDER BY d.created_at LIMIT 500 FOR UPDATE OF d SKIP LOCKED")){String reason=invalid(job);if(reason.equals("expired")||reason.equals("cancelled"))store.outcome((UUID)job.get("id"),reason,reason.equals("expired")?"告警已超過 24 小時":"待辦已變更或收件資格／地址版本失效",0);}});}
 Map<String,Object> claim(){return store.access.tx.execute(tx->{
  var rows=store.access.pg.queryForList("SELECT d.*,e.type,e.equipment_id,e.subject_id,e.revision,e.stage,e.source_time FROM notification_deliveries d JOIN notification_events e ON e.id=d.event_id WHERE d.status IN ('queued','retry') AND d.available_at<=now() ORDER BY d.created_at LIMIT 1 FOR UPDATE OF d SKIP LOCKED");
  if(rows.isEmpty())return null;var job=rows.getFirst();UUID id=(UUID)job.get("id");String invalid=invalid(job);
  if(!invalid.isEmpty()){if(invalid.equals("waiting"))store.access.pg.update("UPDATE notification_deliveries SET available_at=now()+interval '30 seconds' WHERE id=?",id);else store.outcome(id,invalid,invalid.equals("expired")?"告警已超過 24 小時":"待辦已變更或收件資格／地址版本失效",0);return null;}
  if(job.get("rendered_subject")==null){
   var locales=store.recipientLocales(job);
   if(locales.isEmpty()){store.outcome(id,"cancelled","待辦已變更或收件資格／地址版本失效",0);return null;}
   String locale=locales.size()==1?locales.iterator().next():"zh-TW";
   String reason=locales.size()==1?"recipient-preference":"shared-address-conflict";
   var content=EmailTemplates.render(job,store.access.pg.queryForObject("SELECT public_base_url FROM mail_connection WHERE id=1",String.class),locale);
   store.access.pg.update("UPDATE notification_deliveries SET delivery_locale=?,locale_reason=?,template_version=?,rendered_subject=?,rendered_body=? WHERE id=?",locale,reason,EmailTemplates.VERSION,content.get("subject"),content.get("text"),id);
   job.put("rendered_subject",content.get("subject"));job.put("rendered_body",content.get("text"));
  }
  store.access.pg.update("UPDATE notification_deliveries SET status='sending',attempts=attempts+1,lease_until=now()+interval '2 minutes',updated_at=now() WHERE id=?",id);job.put("attempts",((Number)job.get("attempts")).intValue()+1);return job;
 });}
 static Map<String,String> content(Map<String,Object> job,String base){
  return EmailTemplates.render(job,base,"zh-TW");
 }
 @Scheduled(fixedDelay=3600000) void cleanup(){store.access.pg.update("DELETE FROM mail_oauth WHERE expires_at<now()");store.access.pg.update("DELETE FROM notification_events e WHERE e.created_at<now()-interval '30 days' AND e.status<>'pending' AND NOT EXISTS(SELECT 1 FROM notification_deliveries d WHERE d.event_id=e.id AND d.status IN ('queued','retry','sending','unknown'))");}
}
