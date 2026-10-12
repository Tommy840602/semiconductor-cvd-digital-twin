package com.example.cvd;

import com.fasterxml.jackson.databind.JsonNode;
import jakarta.annotation.PostConstruct;
import java.time.*;
import java.util.*;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import org.springframework.http.HttpStatus;

@RestController
@RequestMapping("/api/equipment/{equipmentId}/vacuum")
public class VacuumApi {
 final DataStore db;
 public VacuumApi(DataStore db){this.db=db;}
 @PostConstruct void init(){
  db.pg.execute("CREATE TABLE IF NOT EXISTS vacuum_events(id text PRIMARY KEY,equipment_id text NOT NULL,pump_id text NOT NULL,time timestamptz NOT NULL,sequence bigint NOT NULL,kind text NOT NULL,body jsonb NOT NULL)");
  db.pg.execute("CREATE INDEX IF NOT EXISTS vacuum_events_lookup ON vacuum_events(equipment_id,pump_id,time DESC)");
 }
 @KafkaListener(topics="cvd.events.v1",groupId="cvd-vacuum-evidence") void ingest(String raw){var n=db.parse(raw);String kind=n.path("kind").asText();if(!kind.startsWith("vacuum-")&&!kind.startsWith("maintenance-"))return;String eq=Equipment.require(n.path("equipmentId").asText());String id=n.path("eventId").asText();if(id.isBlank())return;db.pg.update("INSERT INTO vacuum_events VALUES(?,?,?,?::timestamptz,?,?,?::jsonb) ON CONFLICT DO NOTHING",id,eq,n.path("payload").path("pumpId").asText(),n.path("sourceTime").asText(),n.path("sequence").asLong(),kind,raw);}
 static void pump(String id){if(!Set.of("LL1","LL2","LL3").contains(id))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"invalid pumpId");}
 static Instant[] range(String from,String to){try{Instant end=to.isBlank()?Instant.now():Instant.parse(to),start=from.isBlank()?end.minus(Duration.ofHours(1)):Instant.parse(from);if(!start.isBefore(end)||Duration.between(start,end).compareTo(Duration.ofDays(30))>0)throw new IllegalArgumentException();return new Instant[]{start,end};}catch(Exception e){throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"invalid range (maximum 30 days)");}}
 @GetMapping public Map<String,Object> view(@RequestParam(defaultValue="LL1") String pumpId,@RequestParam(defaultValue="") String from,@RequestParam(defaultValue="") String to){pump(pumpId);var r=range(from,to);String eq=Equipment.current();JsonNode state=db.parse(db.snapshot(eq));
 var events=db.pg.queryForList("SELECT body::text FROM vacuum_events WHERE equipment_id=? AND (pump_id=? OR kind LIKE 'maintenance-%') AND time>=?::timestamptz AND time<?::timestamptz ORDER BY time DESC,sequence DESC LIMIT 100",String.class,eq,pumpId,r[0].toString(),r[1].toString()).stream().map(db::parse).toList();
 var counts=db.pg.queryForMap("SELECT count(*) FILTER(WHERE body#>>'{payload,pump,cycle,status}'='passed') passed,count(*) FILTER(WHERE body#>>'{payload,pump,cycle,status}'='failed') failed,count(*) FILTER(WHERE body#>>'{payload,pump,cycle,status}'='interrupted') interrupted FROM vacuum_events WHERE equipment_id=? AND pump_id=? AND kind='vacuum-result' AND time>=?::timestamptz AND time<?::timestamptz",eq,pumpId,r[0].toString(),r[1].toString());
 // At most 720 buckets, retaining pressure extrema rather than downloading 30 days of seconds.
 long bucket=Math.max(1,(Duration.between(r[0],r[1]).getSeconds()+719)/720);
 var trend=db.ts.queryForList("SELECT floor(extract(epoch from time)/?)*? AS epoch,min((p->>'pressure')::double precision) pressure_min,max((p->>'pressure')::double precision) pressure_max,avg((p#>>'{pump,health}')::double precision) health FROM telemetry CROSS JOIN LATERAL jsonb_array_elements(body#>'{payload,locks}') p WHERE equipment_id=? AND p->>'id'=? AND jsonb_exists(p,'pump') AND time>=?::timestamptz AND time<?::timestamptz GROUP BY 1 ORDER BY 1",bucket,bucket,eq,pumpId,r[0].toString(),r[1].toString());
 return Map.of("locks",state.path("locks"),"sourceTime",state.path("sourceTime"),"fresh",MaintenanceApi.fresh(state)&&state.path("communicationQuality").asText("good").equals("good"),"vacuumHold",state.path("vacuumHold").asBoolean(),"counts",counts,"events",events,"trend",trend,"bucketSeconds",bucket);
 }
}
