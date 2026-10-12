package com.example.cvd;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.*;
import java.util.*;
/** Executed only inside the isolated validation network against restored databases. */
public class FleetMigrationCheck {
 static String digest(JdbcTemplate db,String table,String column){return db.queryForObject("SELECT count(*)::text||':'||coalesce(sum(hashtext("+column+"::text)::bigint),0)::text FROM "+table,String.class);}
 public static void main(String[] args){
  String database=args.length==0?"fleet_restore":args[0];if(!database.equals("fleet_restore")&&!database.equals("cvd"))throw new IllegalArgumentException("unsupported check database");
  var ds=new DriverManagerDataSource("jdbc:postgresql://postgres:5432/"+database,"cvd","cvd-local");var pg=new JdbcTemplate(ds);
  var db=new DataStore(pg,new ObjectMapper(),"jdbc:postgresql://timescaledb:5432/"+database,"cvd","cvd-local");
  var access=new AccessStore(pg,new DataSourceTransactionManager(ds),"");
  var tables=Map.of("history","body","commands","body","inbox","body","equipment_snapshot","body","recovery_versions","prescription","app_users","row_to_json(app_users)","recipe_versions","row_to_json(recipe_versions)","recipe_heads","version_id","recovery_intents","row_to_json(recovery_intents)");var before=new HashMap<String,String>();tables.forEach((t,c)->before.put(t,digest(pg,t,c)));String telemetry=digest(db.ts,"telemetry","body");
  for(int i=0;i<2;i++){db.migrate();new RecoveryApi(db,access,"").migrate();new RecipeApi(db,access).migrate();}
  tables.forEach((t,c)->{if(!before.get(t).equals(digest(pg,t,c)))throw new AssertionError("Changed original data: "+t);});
  if(!telemetry.equals(digest(db.ts,"telemetry","body")))throw new AssertionError("Changed telemetry");
  if(pg.queryForObject("SELECT count(*) FROM equipment_snapshot WHERE id='CVD-01'",Integer.class)!=0)throw new AssertionError("Unmigrated identity");
  System.out.println("PASS: restored PostgreSQL and TimescaleDB; two migrations preserve counts, payloads, passwords and recovery versions");
  ((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();
 }
}
