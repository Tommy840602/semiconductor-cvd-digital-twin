package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import org.junit.jupiter.api.*;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.mock.web.*;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import java.util.*;
import java.time.*;

class EmailNotificationsTest {
 @AfterEach void clear(){SecurityContextHolder.clearContext();}
 @Test void matrixAndAddresses(){for(String eq:Equipment.IDS){assertNotNull(Equipment.require(eq));for(String type:List.of("fault","fault-cleared","offline","online")){assertTrue(NotificationStore.eligible(type,"Engineer","EE"));assertTrue(NotificationStore.eligible(type,"Leader","EE"));assertFalse(NotificationStore.eligible(type,"Engineer","PE"));}for(String type:List.of("review","result")){assertTrue(NotificationStore.eligible(type,"Engineer","PE"));assertFalse(NotificationStore.eligible(type,"Leader","EE"));}assertTrue(NotificationStore.eligible("release","Leader","EE"));assertTrue(NotificationStore.eligible("release","Leader","PE"));assertFalse(NotificationStore.eligible("release","Engineer","PE"));for(String type:List.of("fault","review","result","release")){assertFalse(NotificationStore.eligible(type,"Operator","EE"));assertFalse(NotificationStore.eligible(type,"Leader",null));}}
 assertEquals("abc+test@outlook.com",NotificationStore.address(" ABC+test@Outlook.COM "));for(String s:List.of("x\r\nBcc:a@x.com","not-mail","a@","a b@x.com","a..b@x.com"))assertThrows(Exception.class,()->NotificationStore.address(s));}
 @Test void originAndTemplates(){assertEquals("https://example.com",EmailConfig.url("https://example.com/",false));for(String bad:List.of("https://evil.com@site.com","javascript:alert(1)","http://site.com","https://site.com/?x=a","https://site.com/path"))assertThrows(IllegalArgumentException.class,()->EmailConfig.url(bad,false));assertEquals("http://localhost:18000/oauth/outlook",EmailConfig.url("http://localhost:18000/oauth/outlook",true));
 var body=NotificationWorker.content(Map.of("type","review","equipment_id",Equipment.IDS.get(7),"subject_id","case_<b>","source_time",java.sql.Timestamp.from(Instant.parse("2026-09-25T00:00:00Z"))),"https://example.com");assertTrue(body.get("text").contains("08:00:00 UTC+08:00"));assertTrue(body.get("text").contains("case=case_%3Cb%3E"));assertFalse(body.get("text").contains("token"));}
 @Test void emailApiUsesExistingSessionCsrf() throws Exception {
  var sessions=mock(AccessSessions.class);var identity=new AccessSessions.Identity(UUID.randomUUID().toString(),"leader","Leader","EE",1);when(sessions.cookie(any())).thenReturn("session");when(sessions.find("session")).thenReturn(new AccessSessions.Session("csrf-value",identity));
  var filter=new AccessFilter(sessions);var request=new MockHttpServletRequest("POST","/api/notifications/settings");request.setServletPath("/api/notifications/settings");var response=new MockHttpServletResponse();filter.doFilter(request,response,(a,b)->fail("missing CSRF passed"));assertEquals(403,response.getStatus());
  request.addHeader("X-CSRF-Token","csrf-value");filter.doFilter(request,new MockHttpServletResponse(),(a,b)->assertEquals("session",a.getAttribute("cvd.session")));
  SecurityContextHolder.getContext().setAuthentication(new UsernamePasswordAuthenticationToken(new AccessSessions.Identity("x","operator","Operator","EE",1),null,List.of()));assertThrows(Exception.class,NotificationStore::leader);
 }
 @Test void safeDefaults(){assertFalse(new EmailConfig(new MockEnvironment()).allowed);assertFalse(new EmailConfig(new MockEnvironment()).configured());assertEquals(120,OutlookMail.retryAfter("120"));}
}
