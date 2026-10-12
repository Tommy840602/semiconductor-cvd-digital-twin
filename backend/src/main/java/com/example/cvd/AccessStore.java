package com.example.cvd;

import jakarta.annotation.PostConstruct;
import java.nio.file.*;
import java.util.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.transaction.PlatformTransactionManager;

@Component
public class AccessStore {
  final JdbcTemplate pg;
  final TransactionTemplate tx;
  final BCryptPasswordEncoder passwords = new BCryptPasswordEncoder(12);
  final String secretFile;
  public AccessStore(JdbcTemplate pg, PlatformTransactionManager manager,
      @Value("${CVD_BOOTSTRAP_PASSWORD_FILE:}") String file) {
    this.pg=pg; this.tx=new TransactionTemplate(manager); this.secretFile=file;
  }
  @PostConstruct void initialize() {
    tx.executeWithoutResult(status -> {
      pg.execute("SELECT pg_advisory_xact_lock(81344021)");
      pg.execute("CREATE TABLE IF NOT EXISTS app_users(id uuid PRIMARY KEY, username text UNIQUE NOT NULL, password_hash text NOT NULL, role text NOT NULL CHECK(role IN ('Operator','Engineer','Leader')), created_at timestamptz NOT NULL DEFAULT now(), created_by uuid)");
      pg.execute("CREATE TABLE IF NOT EXISTS auth_bootstrap(id integer PRIMARY KEY, completed_at timestamptz NOT NULL DEFAULT now())");
      pg.execute("CREATE TABLE IF NOT EXISTS access_audit(id bigserial PRIMARY KEY, time timestamptz NOT NULL DEFAULT now(), user_id text, username text, role text, action text NOT NULL, target text, command_id text, outcome text NOT NULL)");
      pg.execute("ALTER TABLE app_users ADD COLUMN IF NOT EXISTS department text CHECK(department IN ('EE','PE'))");
      pg.execute("ALTER TABLE app_users ADD COLUMN IF NOT EXISTS locale text NOT NULL DEFAULT 'zh-TW' CHECK(locale IN ('zh-TW','en','ja'))");
      pg.execute("ALTER TABLE app_users ADD COLUMN IF NOT EXISTS auth_version bigint NOT NULL DEFAULT 1");
      pg.execute("ALTER TABLE access_audit ADD COLUMN IF NOT EXISTS department text");
      if (pg.queryForObject("SELECT count(*) FROM auth_bootstrap WHERE id=1",Long.class)>0) return;
      if (pg.queryForObject("SELECT count(*) FROM app_users WHERE username='cvd.leader'",Long.class)>0)
        throw new IllegalStateException("Bootstrap username already exists; manual reconciliation required");
      String password;
      try { password=Files.readString(Path.of(secretFile)).stripTrailing(); }
      catch(Exception e) { throw new IllegalStateException("Bootstrap password file required"); }
      if (!validPassword(password)) throw new IllegalStateException("Bootstrap password must be 12+ characters and at most 72 UTF-8 bytes");
      pg.update("INSERT INTO app_users(id,username,password_hash,role) VALUES(?,'cvd.leader',?,'Leader')",UUID.randomUUID(),passwords.encode(password));
      pg.update("INSERT INTO auth_bootstrap(id) VALUES(1)");
    });
  }
  static boolean validPassword(String p) {return p!=null && p.length()>=12 && p.getBytes(java.nio.charset.StandardCharsets.UTF_8).length<=72;}
  static boolean validUsername(String s) {return s!=null && s.matches("[A-Za-z0-9][A-Za-z0-9._-]{2,63}");}
  Map<String,Object> user(String name) {
    var rows=pg.queryForList("SELECT id::text,username,password_hash,role,department,auth_version FROM app_users WHERE username=?",name);
    return rows.isEmpty()?null:rows.getFirst();
  }
  void audit(AccessSessions.Identity u,String action,String target,String id,String outcome) {
    pg.update("INSERT INTO access_audit(user_id,username,role,action,target,command_id,outcome,department) VALUES(?,?,?,?,?,?,?,?)",
      u==null?null:u.id(),u==null?null:u.username(),u==null?null:u.role(),safe(action),safe(target),safe(id),outcome,u==null?null:u.department());
  }
  static String safe(String s) {return s==null?"":s.substring(0,Math.min(s.length(),128));}
}
