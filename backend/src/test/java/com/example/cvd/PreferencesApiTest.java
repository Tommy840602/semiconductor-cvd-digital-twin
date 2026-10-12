package com.example.cvd;
import org.junit.jupiter.api.*;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import java.util.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
class PreferencesApiTest {
 @AfterEach void clear(){SecurityContextHolder.clearContext();}
 @Test void messageCodesRetainOriginalAndBindParameters(){
  assertEquals("sys083",SystemMessages.response("帳號或密碼不正確").get("code"));
  var bound=SystemMessages.response("PM2 故障尚未解除");assertEquals("pmFaultPending",bound.get("code"));assertEquals(Map.of("pm","PM2"),bound.get("params"));assertEquals("PM2 故障尚未解除",bound.get("message"));
  assertEquals("original",SystemMessages.response("external custom text").get("code"));
 }
 @Test void preferenceRouteRequiresSessionAndCsrf() throws Exception {
  var sessions=mock(AccessSessions.class);when(sessions.cookie(any())).thenReturn("session");var filter=new AccessFilter(sessions);
  for(String method:List.of("GET","PATCH")){
   var req=new org.springframework.mock.web.MockHttpServletRequest(method,"/api/auth/preferences");req.setServletPath("/api/auth/preferences");var res=new org.springframework.mock.web.MockHttpServletResponse();filter.doFilter(req,res,(a,b)->fail("anonymous passed"));assertEquals(401,res.getStatus());
  }
  when(sessions.find("session")).thenReturn(new AccessSessions.Session("csrf",new AccessSessions.Identity("id","user","Operator",null,1)));
  var req=new org.springframework.mock.web.MockHttpServletRequest("PATCH","/api/auth/preferences");req.setServletPath("/api/auth/preferences");var res=new org.springframework.mock.web.MockHttpServletResponse();filter.doFilter(req,res,(a,b)->fail("no CSRF passed"));assertEquals(403,res.getStatus());
  req.addHeader("X-CSRF-Token","csrf");filter.doFilter(req,new org.springframework.mock.web.MockHttpServletResponse(),(a,b)->assertEquals("session",a.getAttribute("cvd.session")));
 }
 @Test void ownPreferenceOnlyAndNoAuthorizationMutation(){
  var pg=mock(JdbcTemplate.class);var store=new AccessStore(pg,mock(PlatformTransactionManager.class),"");var api=new PreferencesApi(store);
  for(String role:List.of("Operator","Engineer","Leader")){
   var actor=new AccessSessions.Identity(UUID.randomUUID().toString(),"user",role,"EE",1);SecurityContextHolder.getContext().setAuthentication(new UsernamePasswordAuthenticationToken(actor,null,List.of()));
   when(pg.update("UPDATE app_users SET locale=? WHERE id=?::uuid","ja",actor.id())).thenReturn(1);
   assertEquals("ja",api.save(Map.of("locale","ja")).get("locale"));
   assertThrows(Exception.class,()->api.save(Map.of("locale","en","userId","other")));assertThrows(Exception.class,()->api.save(Map.of("locale","xx")));
  }
  verify(pg,never()).update(contains("auth_version"),any(Object[].class));verify(pg,never()).update(contains("email_version"),any(Object[].class));
 }
}
