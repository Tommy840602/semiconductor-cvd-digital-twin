package com.example.cvd;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
/** Disposable restored DB only: verify retention protects active lot and recovery chains. */
public class FleetRetentionCheck {
 public static void main(String[] args){
  String database=args.length==0?"fleet_restore":args[0];if(!database.equals("fleet_restore")&&!database.equals("cvd"))throw new IllegalArgumentException("unsupported check database");
  var pg=new JdbcTemplate(new DriverManagerDataSource("jdbc:postgresql://postgres:5432/"+database,"cvd","cvd-local"));
  var db=new DataStore(pg,new ObjectMapper(),"jdbc:postgresql://timescaledb:5432/"+database,"cvd","cvd-local");
  pg.update("INSERT INTO equipment_snapshot(id,version,body) VALUES('TEL-CVDTIN-08A-10',1,'{\"lots\":[{\"id\":\"retention-active-lot\",\"phase\":\"processing\"}]}') ON CONFLICT(id) DO UPDATE SET body=excluded.body");
  pg.update("INSERT INTO recovery_requests(id,wafer_id,owner_id,revision,status,equipment_id) VALUES('retention-request','retention-wafer','fixture',1,'draft','TEL-CVDTIN-08A-10') ON CONFLICT DO NOTHING");
  pg.update("INSERT INTO history(id,time,lot_id,wafer_id,kind,body,equipment_id) VALUES('retention-active',now()-interval '40 days','retention-active-lot','','test','{}','TEL-CVDTIN-08A-10'),('retention-expired',now()-interval '40 days','','','test','{}','TEL-CVDTIN-08A-10'),('retention-recovery',now()-interval '40 days','','retention-wafer','test','{}','TEL-CVDTIN-08A-10') ON CONFLICT DO NOTHING");
  db.cleanup();
  if(pg.queryForObject("SELECT count(*) FROM history WHERE id IN ('retention-active','retention-recovery')",Integer.class)!=2||pg.queryForObject("SELECT count(*) FROM history WHERE id='retention-expired'",Integer.class)!=0)throw new AssertionError("retention damaged active chain");
  db.ts.update("INSERT INTO telemetry(time,id,body,equipment_id) VALUES(now()-interval '40 days','retention-expired','{}','TEL-CVDTIN-08A-10')");
  Integer job=db.ts.queryForObject("SELECT job_id FROM timescaledb_information.jobs WHERE proc_name='policy_retention' AND hypertable_name='telemetry'",Integer.class);db.ts.execute("CALL run_job("+job+")");
  if(db.ts.queryForObject("SELECT count(*) FROM telemetry WHERE id='retention-expired'",Integer.class)!=0)throw new AssertionError("telemetry retention failed");
  System.out.println("PASS: 30-day telemetry retention and active lot/recovery history protection");
  ((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();
 }
}
