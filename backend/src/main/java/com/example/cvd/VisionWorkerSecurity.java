package com.example.cvd;

import java.nio.file.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.*;
import org.springframework.core.annotation.Order;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.AnonymousAuthenticationFilter;
import org.springframework.web.filter.OncePerRequestFilter;
import jakarta.servlet.*;
import jakarta.servlet.http.*;
import java.io.IOException;

@Configuration
public class VisionWorkerSecurity {
 @Bean @Order(1) SecurityFilterChain visionWorker(HttpSecurity http,@Value("${VISION_TOKEN_FILE:}") String file)throws Exception {
  String token=file.isBlank()?"":Files.readString(Path.of(file)).trim();
  return http.securityMatcher("/api/vision/worker/**").csrf(c->c.disable()).sessionManagement(s->s.sessionCreationPolicy(org.springframework.security.config.http.SessionCreationPolicy.STATELESS))
   .addFilterBefore(new OncePerRequestFilter(){
    protected void doFilterInternal(HttpServletRequest req,HttpServletResponse res,FilterChain chain)throws IOException,ServletException {
     res.setHeader("Cache-Control","no-store");
     if(token.length()<32||!AccessSessions.same("Bearer "+token,req.getHeader("Authorization"))){res.sendError(401);return;}
     chain.doFilter(req,res);
    }
   },AnonymousAuthenticationFilter.class).authorizeHttpRequests(a->a.anyRequest().permitAll()).build();
 }
}
