package com.example.cvd;
import org.springframework.context.annotation.*;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.AnonymousAuthenticationFilter;
@Configuration
public class SecurityConfig {
 @Bean SecurityFilterChain access(HttpSecurity http,AccessSessions sessions)throws Exception {
  return http.csrf(c->c.disable()) // Redis-bound CSRF tokens, including login/logout, checked by AccessFilter.
   .sessionManagement(s->s.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
   .requestCache(c->c.disable()).formLogin(c->c.disable()).httpBasic(c->c.disable()).logout(c->c.disable())
   .addFilterBefore(new AccessFilter(sessions),AnonymousAuthenticationFilter.class)
   .authorizeHttpRequests(a->a.requestMatchers("/error","/actuator/health","/api/auth/csrf","/api/auth/login").permitAll().anyRequest().authenticated())
   .exceptionHandling(e->e.authenticationEntryPoint((req,res,x)->AccessFilter.reject(res,401,"請重新登入")))
   .build();
 }
}
