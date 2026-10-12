package com.example.cvd;
import jakarta.servlet.http.HttpServletRequest;
import java.util.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/api")
public class NotificationApi {
 final NotificationStore store;final OutlookMail mail;final EmailConfig config;final NotificationWorker worker;
 public NotificationApi(NotificationStore store,OutlookMail mail,EmailConfig config,NotificationWorker worker){this.store=store;this.mail=mail;this.config=config;this.worker=worker;}
 record Email(String email,boolean enabled){}
 @PostMapping("/users/{id}/email") public Map<String,Boolean> email(@PathVariable UUID id,@RequestBody Email body){store.email(id,body.email(),body.enabled());return Map.of("ok",true);}
 @GetMapping("/notifications") public Map<String,Object> status(){NotificationStore.leader();var connection=store.access.pg.queryForMap("SELECT status,sender,enabled,public_base_url,updated_at FROM mail_connection WHERE id=1");return Map.of("connection",connection,"allowed",config.allowed,"configured",config.configured(),"redirectUri",config.redirect,"offlineSeconds",config.offlineSeconds,"recoverySeconds",config.recoverySeconds,"events",store.access.pg.queryForList("SELECT * FROM notification_events ORDER BY created_at DESC LIMIT 100"),"deliveries",store.access.pg.queryForList("SELECT d.id,d.delivery_locale,d.locale_reason,d.template_version,d.address,d.status,d.reason,d.attempts,d.updated_at,e.type,e.equipment_id,e.subject_id FROM notification_deliveries d JOIN notification_events e ON e.id=d.event_id ORDER BY d.created_at DESC LIMIT 100"),"attempts",store.access.pg.queryForList("SELECT * FROM notification_attempts ORDER BY id DESC LIMIT 100"));}
 record Settings(boolean enabled,String publicBaseUrl){}
 @PostMapping("/notifications/settings") public Map<String,Boolean> settings(@RequestBody Settings body){
  var actor=NotificationStore.leader();String base;try{base=EmailConfig.url(body.publicBaseUrl(),false);}catch(IllegalArgumentException e){throw new ResponseStatusException(HttpStatus.BAD_REQUEST,e.getMessage());}
  if(body.enabled()&&!config.allowed)throw new ResponseStatusException(HttpStatus.CONFLICT,"部署環境 EMAIL_ENABLED=false，寄信保持停用");
  store.access.tx.executeWithoutResult(tx->{var old=store.access.pg.queryForMap("SELECT * FROM mail_connection WHERE id=1 FOR UPDATE");if(body.enabled()&&old.get("started_at")==null){worker.baseline();store.access.pg.update("UPDATE mail_connection SET started_at=now() WHERE id=1");}store.access.pg.update("UPDATE mail_connection SET enabled=?,public_base_url=?,updated_at=now() WHERE id=1",body.enabled(),base);store.access.audit(actor,"email-settings","mail","",body.enabled()?"enabled":"paused");});return Map.of("ok",true);
 }
 @PostMapping("/notifications/connect") public Map<String,String> connect(HttpServletRequest request){return Map.of("url",mail.begin((String)request.getAttribute("cvd.session")));}
 record Callback(String state,String code){}
 @PostMapping("/notifications/callback") public Map<String,Boolean> callback(@RequestBody Callback body,HttpServletRequest request){mail.complete((String)request.getAttribute("cvd.session"),body.state(),body.code());return Map.of("ok",true);}
 @PostMapping("/notifications/disconnect") public Map<String,Boolean> disconnect(){var actor=NotificationStore.leader();store.access.tx.executeWithoutResult(tx->{store.access.pg.queryForMap("SELECT id FROM mail_connection WHERE id=1 FOR UPDATE");store.access.pg.update("UPDATE mail_connection SET status='disconnected',tokens=null,sender='',enabled=false,updated_at=now() WHERE id=1");store.access.pg.update("DELETE FROM mail_oauth");store.access.audit(actor,"outlook-disconnect","mail","","disconnected");});return Map.of("ok",true);}
 record Retry(boolean acceptDuplicateRisk){}
 @PostMapping("/notifications/{id}/retry") public Map<String,Boolean> retry(@PathVariable UUID id,@RequestBody Retry body){var actor=NotificationStore.leader();if(!body.acceptDuplicateRisk())throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"重試可能造成重複郵件，請明確確認");store.access.tx.executeWithoutResult(tx->{if(store.access.pg.update("UPDATE notification_deliveries SET status='queued',attempts=0,available_at=now(),reason='Leader 確認可能重複後重試',updated_at=now() WHERE id=? AND status IN ('unknown','failed')",id)==0)throw new ResponseStatusException(HttpStatus.CONFLICT,"此工作不可重試");store.access.audit(actor,"email-retry",id.toString(),"","duplicate-risk-confirmed");});return Map.of("ok",true);}
}
