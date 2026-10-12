package com.example.cvd;

import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.servlet.http.*;
import java.security.*;
import java.time.Duration;
import java.util.*;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.http.ResponseCookie;
import org.springframework.stereotype.Component;

@Component
public class AccessSessions {
  public record Identity(String id,String username,String role,String department,long authVersion) {}
  public record Session(String csrf,Identity user) {}
  final AccessStore store; final StringRedisTemplate redis; final ObjectMapper json;
  final SecureRandom random=new SecureRandom();
  static final String COOKIE="CVD_SESSION", PREFIX="cvd:session:";
  public AccessSessions(StringRedisTemplate redis,ObjectMapper json,AccessStore store) {this.redis=redis;this.json=json;this.store=store;}
  String token() {byte[] b=new byte[32];random.nextBytes(b);return Base64.getUrlEncoder().withoutPadding().encodeToString(b);}
  String cookie(HttpServletRequest req) {
    if(req.getCookies()!=null) for(var c:req.getCookies()) if(c.getName().equals(COOKIE)&&c.getValue().matches("[A-Za-z0-9_-]{43}"))return c.getValue();
    return null;
  }
  Session find(String id) {
    if(id==null)return null;
    String value=redis.opsForValue().get(PREFIX+id);
    if(value==null)return null;
    try{
      var s=json.readValue(value,Session.class);
      if(s.user()!=null && store.pg.queryForObject("SELECT count(*) FROM app_users WHERE id=?::uuid AND auth_version=?",Long.class,s.user().id(),s.user().authVersion())!=1){remove(id);return null;}
      return s;
    }catch(com.fasterxml.jackson.core.JsonProcessingException e){return null;}
  }
  Session create(Identity user,HttpServletRequest req,HttpServletResponse res) {
    String id=token();var session=new Session(token(),user);
    try{redis.opsForValue().set(PREFIX+id,json.writeValueAsString(session),Duration.ofMinutes(user==null?10:30));}
    catch(com.fasterxml.jackson.core.JsonProcessingException e){throw new IllegalStateException(e);}
    // Nginx overwrites this header; the backend is only exposed on loopback for development.
    boolean secure=req.isSecure()||"https".equals(req.getHeader("X-CVD-Scheme"));
    res.addHeader("Set-Cookie",ResponseCookie.from(COOKIE,id).httpOnly(true).secure(secure).sameSite("Strict").path("/").build().toString());
    return session;
  }
  void remove(String id) {if(id!=null)redis.delete(PREFIX+id);}
  void touch(String id) {if(id!=null)redis.expire(PREFIX+id,Duration.ofMinutes(30));}
  static boolean same(String a,String b) {return a!=null&&b!=null&&MessageDigest.isEqual(a.getBytes(java.nio.charset.StandardCharsets.UTF_8),b.getBytes(java.nio.charset.StandardCharsets.UTF_8));}
  static Identity actor() {return (Identity)org.springframework.security.core.context.SecurityContextHolder.getContext().getAuthentication().getPrincipal();}
}
