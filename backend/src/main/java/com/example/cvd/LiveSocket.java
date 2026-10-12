package com.example.cvd;

import java.util.concurrent.CopyOnWriteArraySet;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.*;
import org.springframework.web.socket.handler.TextWebSocketHandler;

@Component
public class LiveSocket extends TextWebSocketHandler {
  final AccessSessions access;
  public LiveSocket(AccessSessions access) { this.access=access; }
  public void revoke(String id) {
    if(id==null)return;
    for(var s:sessions) if(id.equals(s.getAttributes().get("cvd.session"))) {
      try { s.close(CloseStatus.POLICY_VIOLATION); } catch(Exception ignored) {} sessions.remove(s);
    }
  }
  @org.springframework.scheduling.annotation.Scheduled(fixedDelay=1000)
  public void expire() {
    for(var s:sessions) if(!valid(s)) revoke((String)s.getAttributes().get("cvd.session"));
  }
  boolean valid(WebSocketSession s) {
    try { var user=access.find((String)s.getAttributes().get("cvd.session")); return user!=null && user.user()!=null; }
    catch(Exception e) { return false; }
  }
  final CopyOnWriteArraySet<WebSocketSession> sessions = new CopyOnWriteArraySet<>();

  @Override
  public void afterConnectionEstablished(WebSocketSession s) {
    sessions.add(s);
  }

  @Override
  public void afterConnectionClosed(WebSocketSession s, CloseStatus status) {
    sessions.remove(s);
  }

  boolean subscribed(String equipment) {
    return sessions.stream().anyMatch(s -> equipment.equals(s.getAttributes().get("equipmentId")));
  }

  public void broadcast(String body) {
    if (sessions.isEmpty()) return;
    String equipment;try{equipment=new com.fasterxml.jackson.databind.ObjectMapper().readTree(body).path("equipmentId").asText();}catch(Exception e){return;}
    for (var s : sessions) {
      if(!equipment.equals(s.getAttributes().get("equipmentId")))continue;
      try {
        synchronized (s) {
          if(!valid(s)) { s.close(CloseStatus.POLICY_VIOLATION); sessions.remove(s); continue; }
          s.sendMessage(new TextMessage(body));
        }
      } catch (Exception e) {
        sessions.remove(s);
        try {
          s.close();
        } catch (Exception ignored) {
        }
      }
    }
  }
}
