package com.example.cvd;
import java.util.*;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
final class SystemMessages {
 private static final Map<String,String> LEGACY;
 static {try(var input=SystemMessages.class.getResourceAsStream("/system-messages.json")){LEGACY=new ObjectMapper().readValue(input,new TypeReference<Map<String,String>>(){});}catch(Exception e){throw new ExceptionInInitializerError(e);}}
 static String code(String message){return LEGACY.getOrDefault(message,"");}
 static Map<String,Object> response(String message){
  String[][] patterns={{"^(PM[1-4]) 故障尚未解除$","pmFaultPending","pm"},{"^(PM[1-4]) 處置證據尚待同步確認$","evidencePending","pm"},{"^仍有活動處置 ([A-Za-z0-9_-]{1,128})$","activeDisposition","id"},{"^需要 (EE|PE) Engineer／Leader 權限$","permissionRequired","department"}};
  for(var entry:patterns){var match=java.util.regex.Pattern.compile(entry[0]).matcher(message);if(match.matches())return Map.of("message",message,"code",entry[1],"params",Map.of(entry[2],match.group(1)));}
  return Map.of("message",message,"code",LEGACY.getOrDefault(message,"original"),"params",Map.of());
 }
}
