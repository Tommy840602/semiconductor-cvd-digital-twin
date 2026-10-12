package com.example.cvd;

import java.util.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/api/auth/preferences")
public class PreferencesApi {
 final AccessStore store;
 public PreferencesApi(AccessStore store){this.store=store;}
 static String validate(String locale){if(locale==null||!Set.of("zh-TW","en","ja").contains(locale))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"不支援的語言");return locale;}
 @GetMapping public Map<String,String> get(){return Map.of("locale",store.pg.queryForObject("SELECT locale FROM app_users WHERE id=?::uuid",String.class,AccessSessions.actor().id()));}
 @PatchMapping public Map<String,String> save(@RequestBody Map<String,Object> body){
  if(body.size()!=1||!(body.get("locale") instanceof String))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"僅可修改自己的語言偏好");
  String locale=validate((String)body.get("locale"));var actor=AccessSessions.actor();
  store.tx.executeWithoutResult(tx->{if(store.pg.update("UPDATE app_users SET locale=? WHERE id=?::uuid",locale,actor.id())!=1)throw new ResponseStatusException(HttpStatus.UNAUTHORIZED,"請重新登入");store.audit(actor,"locale-change",actor.id(),"",locale);});return Map.of("locale",locale);
 }
}
