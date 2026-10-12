package com.example.cvd;

import jakarta.servlet.*;
import jakarta.servlet.http.*;
import java.io.IOException;
import java.util.List;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

public class AccessFilter extends OncePerRequestFilter {
  final AccessSessions sessions;
  AccessFilter(AccessSessions sessions){this.sessions=sessions;}
  static void reject(HttpServletResponse res,int status,String message)throws IOException {
    res.setStatus(status);res.setContentType("application/json;charset=UTF-8");res.getWriter().write(new com.fasterxml.jackson.databind.ObjectMapper().writeValueAsString(SystemMessages.response(message)));
  }
  @Override protected void doFilterInternal(HttpServletRequest req,HttpServletResponse res,FilterChain chain)throws ServletException,IOException {
    res.setHeader("Cache-Control","no-store");
    String path=req.getServletPath();
    if(path.equals("/actuator/health")){chain.doFilter(req,res);return;}
    AccessSessions.Session s;
    try{s=sessions.find(sessions.cookie(req));}catch(Exception e){reject(res,503,"登入服務暫時無法使用");return;}
    boolean publicPath=path.equals("/api/auth/csrf")||path.equals("/api/auth/login");
    if(!publicPath&&(s==null||s.user()==null)){reject(res,401,"請重新登入");return;}
    if(!List.of("GET","HEAD","OPTIONS").contains(req.getMethod()) && (s==null||!AccessSessions.same(s.csrf(),req.getHeader("X-CSRF-Token")))) {
      reject(res,403,"請重新整理後再操作");return;
    }
    if(s!=null&&s.user()!=null){
      req.setAttribute("cvd.session",sessions.cookie(req));
      SecurityContextHolder.getContext().setAuthentication(new UsernamePasswordAuthenticationToken(s.user(),null,List.of(new SimpleGrantedAuthority("ROLE_"+s.user().role()))));
    }
    chain.doFilter(req,res);
  }
}
