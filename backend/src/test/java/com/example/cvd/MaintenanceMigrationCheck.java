package com.example.cvd;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.time.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.*;
import org.springframework.web.server.ResponseStatusException;
public class MaintenanceMigrationCheck {
 static void check(boolean ok,String why){if(!ok)throw new AssertionError(why);}
 public static void main(String[] args){
  var source=new DriverManagerDataSource("jdbc:postgresql://postgres:5432/maintenance_check","cvd","cvd-local");var pg=new JdbcTemplate(source);var mapper=new ObjectMapper();
  var db=new DataStore(pg,mapper,"jdbc:postgresql://timescaledb:5432/maintenance_check","cvd","cvd-local");db.migrate();
  var access=new AccessStore(pg,new DataSourceTransactionManager(source),"");var recovery=new RecoveryApi(db,access,"/run/secrets/recovery_key");recovery.migrate();new DispositionApi(recovery,db,access).migrate();new RecipeApi(db,access).migrate();
  var api=new MaintenanceApi(db);api.migrate();api.migrate();
  var event=mapper.createObjectNode();event.put("equipmentId",Equipment.FIRST);event.put("eventId","m1");event.put("kind","maintenance-test");event.put("sequence",2);event.put("sourceTime",Instant.now().minus(Duration.ofDays(40)).toString());var m=event.putObject("payload").putObject("maintenance");m.put("id","cycle1");m.put("phase","qualifying");m.put("generation",1);
  api.observe(event);api.observe(event);check(pg.queryForObject("SELECT count(*) FROM maintenance_events",Integer.class)==1,"duplicate event");
  var old=event.deepCopy();old.put("eventId","m0");old.put("sequence",1);((ObjectNode)old.path("payload").path("maintenance")).put("phase","waiting");api.observe(old);check(pg.queryForObject("SELECT phase FROM maintenance_cycles WHERE id='cycle1'",String.class).equals("qualifying"),"out-of-order overwrite");
  pg.update("INSERT INTO history(id,time,kind,body,lot_id,equipment_id) VALUES('e',now()-interval '40 days','process-result','{}','cycle1-carrier-test-dummy',?)",Equipment.FIRST);
  pg.update("INSERT INTO commands(id,body,status,equipment_id,created_at) VALUES('c','{\"parameters\":{\"maintenanceId\":\"cycle1\"},\"type\":\"qualification-start\"}','succeeded',?,now()-interval '40 days')",Equipment.FIRST);
  pg.update("INSERT INTO equipment_snapshot(id,version,body) VALUES(?,1,'{\"lots\":[{\"id\":\"formal-parked\",\"phase\":\"processing\",\"purpose\":\"production\",\"parked\":true}]}')",Equipment.IDS.get(1));
  pg.update("INSERT INTO history(id,time,kind,body,lot_id,equipment_id) VALUES('formal',now()-interval '40 days','process-result','{}','formal-parked',?)",Equipment.IDS.get(1));
  db.cleanup();api.cleanup();check(pg.queryForObject("SELECT count(*) FROM history",Integer.class)==2,"active old wafer evidence deleted");check(pg.queryForObject("SELECT count(*) FROM commands",Integer.class)==1,"active old command deleted");check(pg.queryForObject("SELECT count(*) FROM maintenance_events",Integer.class)==2,"active cycle deleted");
  pg.update("UPDATE maintenance_cycles SET phase='released'");api.cleanup();db.cleanup();check(pg.queryForObject("SELECT count(*) FROM maintenance_cycles",Integer.class)==0,"released expired cycle retained");check(pg.queryForObject("SELECT count(*) FROM commands",Integer.class)==0,"released command retained");
  var state=mapper.createObjectNode();state.put("maintenanceModelVersion",1);state.put("sourceTime",Instant.now().toString());state.put("recipeVersion","r1");state.set("maintenance",m);pg.update("INSERT INTO equipment_snapshot(id,version,body) VALUES(?,1,?::jsonb)",Equipment.FIRST,state.toString());
  pg.update("INSERT INTO recipe_versions(id,equipment_id,revision,settings,actor) VALUES('r1',?,1,'{}','{}')",Equipment.FIRST);pg.update("INSERT INTO recipe_heads VALUES(?,'r1')",Equipment.FIRST);
  var c=mapper.createObjectNode();c.put("type","qualification-start");c.put("equipmentId",Equipment.FIRST);var params=c.putObject("parameters");params.put("maintenanceId","cycle1");params.put("expectedGeneration",1);
  access.tx.executeWithoutResult(tx->MaintenanceApi.guard(db,c));params.put("expectedGeneration",0);try{MaintenanceApi.guard(db,c);throw new AssertionError("stale generation accepted");}catch(ResponseStatusException expected){}params.put("expectedGeneration",1);
  pg.update("INSERT INTO commands(id,equipment_id,status,body) VALUES('pending',?,'received','{\"type\":\"configure\"}')",Equipment.FIRST);try{MaintenanceApi.guard(db,c);throw new AssertionError("pending recipe accepted");}catch(ResponseStatusException expected){}
  check(!MaintenanceApi.fresh(mapper.createObjectNode().put("sourceTime","invalid")),"invalid timestamp accepted");
  System.out.println("PASS idempotent migration, event replay/order, 40-day active retention, released cleanup, stale generation and pending recipe fences");((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();
 }
}
