package com.example.cvd;

import java.util.*;
import java.util.stream.IntStream;
import org.springframework.http.HttpStatus;
import org.springframework.web.context.request.*;
import org.springframework.web.server.ResponseStatusException;

/** Physical tool identity; module identifiers such as PM1 are local to this identity. */
public final class Equipment {
 public static final String FIRST="TEL-CVDTIN-08A-01";
 public static final List<String> IDS=IntStream.rangeClosed(1,10).mapToObj(i->"TEL-CVDTIN-08A-%02d".formatted(i)).toList();
 public static String require(String id){if(!IDS.contains(id))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"請指定有效機台");return id;}
 public static String legacy(String id){return "CVD-01".equals(id)?FIRST:require(id);}
 public static String current(){
  var a=RequestContextHolder.getRequestAttributes();
  if(a instanceof ServletRequestAttributes s){String p=s.getRequest().getRequestURI();String prefix="/api/equipment/";if(p.startsWith(prefix))return require(p.substring(prefix.length()).split("/",2)[0]);}
  throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"必須指定機台");
 }
 private Equipment(){}
}
