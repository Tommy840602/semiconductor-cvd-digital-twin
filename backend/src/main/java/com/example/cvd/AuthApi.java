package com.example.cvd;
import jakarta.servlet.http.*;
import java.time.Duration;
import java.util.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import org.springframework.dao.DuplicateKeyException;

@RestController
@RequestMapping("/api")
public class AuthApi {
 final AccessStore store;final AccessSessions sessions;final LiveSocket socket;
 final String dummy;
 public AuthApi(AccessStore store,AccessSessions sessions,LiveSocket socket){this.store=store;this.sessions=sessions;this.socket=socket;dummy=store.passwords.encode(UUID.randomUUID().toString());}
 record Login(String username,String password) {}
 record NewUser(String username,String password,String role,String department) {}
 @GetMapping("/auth/csrf") public Map<String,String> csrf(HttpServletRequest req,HttpServletResponse res){
  var s=sessions.find(sessions.cookie(req));if(s==null)s=sessions.create(null,req,res);return Map.of("token",s.csrf());
 }
 @PostMapping("/auth/login") public Map<String,Object> login(@RequestBody Login body,HttpServletRequest req,HttpServletResponse res) {
  String name=body.username()==null?"":body.username();
  // Per-name and global fixed windows also cover unknown accounts; no trust in client-supplied IP headers.
  for(String key:List.of("cvd:login:global","cvd:login:name:"+Integer.toHexString(name.hashCode()))){
   long count=sessions.redis.opsForValue().increment(key);if(count==1)sessions.redis.expire(key,Duration.ofMinutes(5));
   if(count>(key.endsWith("global")?100:10))throw new ResponseStatusException(HttpStatus.TOO_MANY_REQUESTS,"登入嘗試過多，請稍後重試");
  }
  var u=AccessStore.validUsername(name)?store.user(name):null;
  String pass=body.password()==null?"":body.password();
  boolean valid=pass.getBytes(java.nio.charset.StandardCharsets.UTF_8).length<=72 && store.passwords.matches(pass,u==null?dummy:(String)u.get("password_hash"));
  if(u==null||!valid)throw new ResponseStatusException(HttpStatus.UNAUTHORIZED,"帳號或密碼不正確");
  String old=sessions.cookie(req);sessions.remove(old);socket.revoke(old);
  var identity=new AccessSessions.Identity((String)u.get("id"),(String)u.get("username"),(String)u.get("role"),(String)u.get("department"),((Number)u.get("auth_version")).longValue());
  var s=sessions.create(identity,req,res);return Map.of("user",identity,"csrf",s.csrf());
 }
 @GetMapping("/auth/me") public AccessSessions.Identity me(){return AccessSessions.actor();}
 @PostMapping("/auth/activity") public Map<String,Boolean> activity(HttpServletRequest req){sessions.touch(sessions.cookie(req));return Map.of("ok",true);}
 @PostMapping("/auth/logout") public Map<String,Boolean> logout(HttpServletRequest req,HttpServletResponse res){
  String id=sessions.cookie(req);sessions.remove(id);socket.revoke(id);
  res.addHeader("Set-Cookie","CVD_SESSION=; Path=/; Max-Age=0; HttpOnly; SameSite=Strict");return Map.of("ok",true);
 }
 @PostMapping("/users") public Map<String,Object> create(@RequestBody NewUser body) {
  var actor=AccessSessions.actor();String target=AccessStore.validUsername(body.username())?body.username():"invalid-username";
  if(!actor.role().equals("Leader")){store.audit(actor,"user-create",target,"","denied");throw new ResponseStatusException(HttpStatus.FORBIDDEN,"僅 Leader 可建立帳號");}
  if(!AccessStore.validUsername(body.username())||!AccessStore.validPassword(body.password())||!Set.of("Operator","Engineer").contains(body.role()==null?"":body.role())){
   store.audit(actor,"user-create",target,"","rejected");throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"帳號需 3–64 位英數字或 ._-；密碼至少 12 字元且最多 72 bytes；角色限 Operator 或 Engineer");
  }
  validDepartment(body.department());
  String id=UUID.randomUUID().toString();
  try{store.tx.executeWithoutResult(status->{
   store.pg.update("INSERT INTO app_users(id,username,password_hash,role,created_by,department) VALUES(?::uuid,?,?,?,?::uuid,?)",id,body.username(),store.passwords.encode(body.password()),body.role(),actor.id(),body.department());
   store.audit(actor,"user-create",target,"","accepted");
  });}catch(DuplicateKeyException e){store.audit(actor,"user-create",target,"","duplicate");throw new ResponseStatusException(HttpStatus.CONFLICT,"帳號已存在");}
  return Map.of("id",id,"username",body.username(),"role",body.role());
 }
 static void validDepartment(String d){if(d!=null&&!Set.of("EE","PE").contains(d))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"部門限 EE 或 PE");}
 @GetMapping("/users") public List<Map<String,Object>> users(){
  if(!AccessSessions.actor().role().equals("Leader"))throw new ResponseStatusException(HttpStatus.FORBIDDEN,"僅 Leader 可管理帳號");
  return store.pg.queryForList("SELECT id::text,username,role,department,created_at,email,email_enabled,email_version FROM app_users ORDER BY created_at");
 }
 record Department(String department){}
 @PostMapping("/users/{id}/department") public Map<String,Boolean> department(@PathVariable UUID id,@RequestBody Department body){
  var actor=AccessSessions.actor();if(!actor.role().equals("Leader"))throw new ResponseStatusException(HttpStatus.FORBIDDEN,"僅 Leader 可指派部門");validDepartment(body.department());
  store.tx.executeWithoutResult(status->{
   var rows=store.pg.queryForList("SELECT username,department FROM app_users WHERE id=? FOR UPDATE",id);
   if(rows.isEmpty())throw new ResponseStatusException(HttpStatus.NOT_FOUND,"帳號不存在");
   var old=rows.getFirst();if(Objects.equals(old.get("department"),body.department()))return;
   store.pg.update("UPDATE app_users SET department=?,auth_version=auth_version+1 WHERE id=?",body.department(),id);
   store.audit(actor,"department-change",(String)old.get("username"),"",String.valueOf(old.get("department"))+" -> "+body.department());
  });return Map.of("ok",true);
 }
 @GetMapping("/audit") public List<Map<String,Object>> audit(){return store.pg.queryForList("SELECT a.*,c.status execution_status FROM access_audit a LEFT JOIN commands c ON c.id=a.command_id ORDER BY a.id DESC LIMIT 100");}
}
