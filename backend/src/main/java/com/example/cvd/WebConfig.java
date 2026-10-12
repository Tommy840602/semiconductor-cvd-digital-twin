package com.example.cvd;

import org.springframework.context.annotation.Configuration;
import org.springframework.web.socket.config.annotation.*;

@Configuration
@EnableWebSocket
public class WebConfig implements WebSocketConfigurer {
  final LiveSocket socket;
  final String publicOrigin;

  public WebConfig(LiveSocket s, @org.springframework.beans.factory.annotation.Value("${CVD_PUBLIC_ORIGIN:}") String origin) {
    this.publicOrigin=origin;
    socket = s;
  }

  public void registerWebSocketHandlers(WebSocketHandlerRegistry r) {
    r.addHandler(socket, "/ws")
        .addInterceptors(new org.springframework.web.socket.server.HandshakeInterceptor() {
          public boolean beforeHandshake(org.springframework.http.server.ServerHttpRequest req, org.springframework.http.server.ServerHttpResponse res, org.springframework.web.socket.WebSocketHandler h, java.util.Map<String,Object> attrs) {
            if(req instanceof org.springframework.http.server.ServletServerHttpRequest servlet) {
              Object id=servlet.getServletRequest().getAttribute("cvd.session");
              if(id!=null){ String equipment=servlet.getServletRequest().getParameter("equipmentId");if(!Equipment.IDS.contains(equipment))return false;attrs.put("equipmentId",equipment); attrs.put("cvd.session",id); return true; }
            }
            return false;
          }
          public void afterHandshake(org.springframework.http.server.ServerHttpRequest req, org.springframework.http.server.ServerHttpResponse res, org.springframework.web.socket.WebSocketHandler h, Exception e) {}
        })
        .setAllowedOriginPatterns(publicOrigin.isBlank() ? new String[]{"http://localhost:*", "http://127.0.0.1:*"} : new String[]{"http://localhost:*", "http://127.0.0.1:*", publicOrigin});
  }
}
