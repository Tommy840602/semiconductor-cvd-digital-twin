package com.example.cvd;

import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.net.*;
import java.net.http.*;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.time.*;
import java.util.*;
import javax.crypto.*;
import javax.crypto.spec.*;
import org.springframework.stereotype.Component;
import org.springframework.security.oauth2.jwt.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.server.ResponseStatusException;

@Component
public class OutlookMail {
 static final String TENANT="9188040d-6c67-4c5b-b112-36a304b66dad";
 static final String ISSUER="https://login.microsoftonline.com/"+TENANT+"/v2.0";
 static final String AUTH="https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize";
 static final String TOKEN="https://login.microsoftonline.com/consumers/oauth2/v2.0/token";
 static final String SCOPES="openid profile email offline_access https://graph.microsoft.com/Mail.Send";
 final NotificationStore store;final EmailConfig config;final HttpClient http;final String tokenUrl,sendUrl;final JwtDecoder decoder;
 @org.springframework.beans.factory.annotation.Autowired
 public OutlookMail(NotificationStore store,EmailConfig config){this(store,config,TOKEN,"https://graph.microsoft.com/v1.0/me/sendMail",null);}
 OutlookMail(NotificationStore store,EmailConfig config,String tokenUrl,String sendUrl,JwtDecoder decoder){
  this.store=store;this.config=config;this.tokenUrl=tokenUrl;this.sendUrl=sendUrl;this.http=HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).followRedirects(HttpClient.Redirect.NEVER).build();
  if(decoder==null){var d=NimbusJwtDecoder.withJwkSetUri("https://login.microsoftonline.com/consumers/discovery/v2.0/keys").build();d.setJwtValidator(JwtValidators.createDefaultWithIssuer(ISSUER));this.decoder=d;}else this.decoder=decoder;
 }
 static String random(){byte[] b=new byte[32];new SecureRandom().nextBytes(b);return Base64.getUrlEncoder().withoutPadding().encodeToString(b);}
 static String hash(String text){try{return Base64.getUrlEncoder().withoutPadding().encodeToString(MessageDigest.getInstance("SHA-256").digest(text.getBytes(StandardCharsets.UTF_8)));}catch(Exception e){throw new IllegalStateException();}}
 static String form(Map<String,String> fields){return fields.entrySet().stream().map(e->URLEncoder.encode(e.getKey(),StandardCharsets.UTF_8)+"="+URLEncoder.encode(e.getValue(),StandardCharsets.UTF_8)).collect(java.util.stream.Collectors.joining("&"));}
 String encrypt(String text){try{byte[] iv=new byte[12];new SecureRandom().nextBytes(iv);var c=Cipher.getInstance("AES/GCM/NoPadding");c.init(Cipher.ENCRYPT_MODE,new SecretKeySpec(config.key(),"AES"),new GCMParameterSpec(128,iv));c.updateAAD("cvd-outlook-v1".getBytes(StandardCharsets.UTF_8));return Base64.getEncoder().encodeToString(iv)+"."+Base64.getEncoder().encodeToString(c.doFinal(text.getBytes(StandardCharsets.UTF_8)));}catch(Exception e){throw new IllegalStateException("Email token 加密失敗");}}
 String decrypt(String text){try{String[] parts=text.split("\\.");var c=Cipher.getInstance("AES/GCM/NoPadding");c.init(Cipher.DECRYPT_MODE,new SecretKeySpec(config.key(),"AES"),new GCMParameterSpec(128,Base64.getDecoder().decode(parts[0])));c.updateAAD("cvd-outlook-v1".getBytes(StandardCharsets.UTF_8));return new String(c.doFinal(Base64.getDecoder().decode(parts[1])),StandardCharsets.UTF_8);}catch(Exception e){throw new IllegalStateException("Email token 無法解密，請重新授權");}}
 String begin(String session){
  var actor=NotificationStore.leader();if(!config.configured())throw new ResponseStatusException(HttpStatus.CONFLICT,"尚未設定 Microsoft 應用與 secrets");
  String redirect=EmailConfig.url(config.redirect,true);config.key();config.secret();
  String state=random(),verifier=random(),nonce=random();
  store.access.tx.executeWithoutResult(tx->{store.access.pg.update("DELETE FROM mail_oauth WHERE expires_at<now() OR actor_id=?::uuid",actor.id());store.access.pg.update("INSERT INTO mail_oauth(state_hash,session_hash,actor_id,verifier,nonce,expires_at) VALUES(?,?,?::uuid,?,?,now()+interval '10 minutes')",hash(state),hash(session),actor.id(),encrypt(verifier),nonce);store.access.audit(actor,"outlook-connect","mail","","started");});
  return AUTH+"?"+form(Map.of("client_id",config.clientId,"response_type","code","redirect_uri",redirect,"response_mode","query","scope",SCOPES,"state",state,"nonce",nonce,"code_challenge",hash(verifier),"code_challenge_method","S256","prompt","select_account"));
 }
 void complete(String session,String state,String code){
  var actor=NotificationStore.leader();if(state==null||code==null||state.length()>128||code.length()>12000)throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"無效授權回覆");
  var rows=store.access.pg.queryForList("DELETE FROM mail_oauth WHERE state_hash=? AND session_hash=? AND actor_id=?::uuid AND expires_at>now() RETURNING verifier,nonce",hash(state),hash(session),actor.id());
  if(rows.isEmpty())throw new ResponseStatusException(HttpStatus.CONFLICT,"授權已過期或不是原 Leader 工作階段，請重新連接");
  try{
   var fields=new HashMap<>(Map.of("grant_type","authorization_code","code",code,"redirect_uri",EmailConfig.url(config.redirect,true),"code_verifier",decrypt(rows.getFirst().get("verifier").toString())));
   var tokens=token(fields);Jwt jwt=decoder.decode(tokens.path("id_token").asText());
   if(jwt.getExpiresAt()==null||jwt.getIssuedAt()==null||jwt.getIssuedAt().isAfter(Instant.now().plusSeconds(60))||(jwt.getAudience().size()>1&&!config.clientId.equals(jwt.getClaimAsString("azp")))||!jwt.getAudience().contains(config.clientId)||!Objects.equals(jwt.getClaimAsString("nonce"),rows.getFirst().get("nonce"))||!ISSUER.equals(jwt.getIssuer().toString())||jwt.getSubject()==null||tokens.path("refresh_token").asText().isBlank())throw new IllegalStateException();
   tokens.remove("id_token");
   String sender=Optional.ofNullable(jwt.getClaimAsString("preferred_username")).orElse(Optional.ofNullable(jwt.getClaimAsString("email")).orElse(jwt.getSubject()));
   store.access.tx.executeWithoutResult(tx->{store.access.pg.queryForMap("SELECT id FROM mail_connection WHERE id=1 FOR UPDATE");
    var current=store.access.pg.queryForMap("SELECT role,auth_version FROM app_users WHERE id=?::uuid FOR SHARE",actor.id());if(!"Leader".equals(current.get("role"))||((Number)current.get("auth_version")).longValue()!=actor.authVersion())throw new IllegalStateException();
    store.access.pg.update("UPDATE mail_connection SET tokens=?,status='connected',sender=?,updated_at=now() WHERE id=1",encrypt(tokens.toString()),AccessStore.safe(sender));store.access.audit(actor,"outlook-connect","mail","","connected");});
  }catch(Exception e){store.access.audit(actor,"outlook-connect","mail","","failed");throw new ResponseStatusException(HttpStatus.BAD_GATEWAY,"Outlook 授權未完成，請檢查應用設定並重新連接");}
 }
 ObjectNode token(Map<String,String> fields) throws Exception {
  fields.put("client_id",config.clientId);fields.put("client_secret",config.secret());fields.put("scope",SCOPES);
  var response=http.send(HttpRequest.newBuilder(URI.create(tokenUrl)).timeout(Duration.ofSeconds(8)).header("Content-Type","application/x-www-form-urlencoded").POST(HttpRequest.BodyPublishers.ofString(form(fields))).build(),HttpResponse.BodyHandlers.ofString());
  if(response.statusCode()!=200)throw new IllegalStateException("token unavailable");
  var t=(ObjectNode)store.db.parse(response.body());
  if(t.path("access_token").asText().isBlank()||!Arrays.stream(t.path("scope").asText().split(" ")).anyMatch(s->s.equalsIgnoreCase("Mail.Send")||s.equalsIgnoreCase("https://graph.microsoft.com/Mail.Send")))throw new IllegalStateException("Mail.Send required");
  t.put("expires_at",Instant.now().plusSeconds(Math.max(0,t.path("expires_in").asLong())).getEpochSecond());t.remove("id_token");
  // Initial authorization needs the signed ID token for audience/nonce verification.
  var raw=store.db.parse(response.body());if(raw.has("id_token"))t.set("id_token",raw.get("id_token"));return t;
 }
 String accessToken(boolean force){return store.access.tx.execute(tx->{
  var connection=store.access.pg.queryForMap("SELECT * FROM mail_connection WHERE id=1 FOR UPDATE");if(!"connected".equals(connection.get("status"))||connection.get("tokens")==null)return null;
  try{var t=store.db.parse(decrypt(connection.get("tokens").toString()));if(!force&&t.path("expires_at").asLong()>Instant.now().plusSeconds(60).getEpochSecond())return t.path("access_token").asText();
   var next=token(new HashMap<>(Map.of("grant_type","refresh_token","refresh_token",t.path("refresh_token").asText())));if(next.path("refresh_token").asText().isBlank())next.set("refresh_token",t.get("refresh_token"));next.remove("id_token");store.access.pg.update("UPDATE mail_connection SET tokens=?,updated_at=now() WHERE id=1",encrypt(next.toString()));return next.path("access_token").asText();
  }catch(Exception e){store.access.pg.update("UPDATE mail_connection SET status='needs_reauth',updated_at=now() WHERE id=1");return null;}
 });}
 record Result(String status,String reason,int delay){}
 Result send(String address,String subject,String text){
  String token=accessToken(false);if(token==null)return new Result("retry","等待 Outlook 重新授權",60);
  for(int attempt=0;attempt<2;attempt++){
   try{
    String payload=store.db.mapper.writeValueAsString(Map.of("message",Map.of("subject",subject,"body",Map.of("contentType","Text","content",text),"toRecipients",List.of(Map.of("emailAddress",Map.of("address",address)))),"saveToSentItems",true));
    var response=http.send(HttpRequest.newBuilder(URI.create(sendUrl)).timeout(Duration.ofSeconds(10)).header("Authorization","Bearer "+token).header("Content-Type","application/json").POST(HttpRequest.BodyPublishers.ofString(payload)).build(),HttpResponse.BodyHandlers.discarding());
    int status=response.statusCode();if(status==202)return new Result("accepted","Outlook 已接受；不代表收件匣已送達",0);
    if(status==401&&attempt==0){token=accessToken(true);if(token!=null)continue;}
    if(status==401){store.access.pg.update("UPDATE mail_connection SET status='needs_reauth' WHERE id=1");return new Result("retry","等待 Outlook 重新授權",60);}
    if(status==429||status==503)return new Result("retry","Graph HTTP "+status,retryAfter(response.headers().firstValue("Retry-After").orElse("60")));
    if(status>=500)return new Result("unknown","Graph 伺服器錯誤，接受結果不確定",0);
    return new Result("failed","Graph HTTP "+status,0);
   }catch(Exception e){if(e instanceof InterruptedException)Thread.currentThread().interrupt();return new Result("unknown","網路中斷或逾時，接受結果不確定",0);}
  }return new Result("failed","授權失效",0);
 }
 static int retryAfter(String value){try{return Math.max(1,Math.min(604800,Integer.parseInt(value)));}catch(Exception e){try{return (int)Math.max(1,Math.min(604800,Duration.between(Instant.now(),ZonedDateTime.parse(value,java.time.format.DateTimeFormatter.RFC_1123_DATE_TIME).toInstant()).getSeconds()));}catch(Exception ignored){return 60;}}}
}
