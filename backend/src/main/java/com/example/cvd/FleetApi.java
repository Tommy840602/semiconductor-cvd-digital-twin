package com.example.cvd;
import java.util.*;
import org.springframework.web.bind.annotation.*;
@RestController
public class FleetApi {
 final DataStore db;
 public FleetApi(DataStore db){this.db=db;}
 @GetMapping("/api/equipment/{equipmentId}/audit") public List<Map<String,Object>> audit(){return db.pg.queryForList("SELECT a.*,c.status execution_status FROM access_audit a JOIN commands c ON c.id=a.command_id WHERE c.equipment_id=? ORDER BY a.id DESC LIMIT 100",Equipment.current());}
 @GetMapping("/api/equipment") public List<Map<String,Object>> list(){
  var snapshots=new HashMap<String,com.fasterxml.jackson.databind.JsonNode>();
  db.pg.queryForList("SELECT s.id,(s.body||jsonb_build_object('equipmentId',s.id,'communicationQuality',coalesce(c.quality,'good')))::text body FROM equipment_snapshot s LEFT JOIN equipment_connectivity c ON c.equipment_id=s.id").forEach(row->snapshots.put(row.get("id").toString(),db.parse(row.get("body").toString())));
  return Equipment.IDS.stream().map(id->{var s=snapshots.getOrDefault(id,db.mapper.createObjectNode());int success=0,scrap=0,active=0;
   for(var l:s.path("lots")){if(l.path("purpose").asText().equals("dummy"))continue;if(!l.path("phase").asText().equals("complete"))active++;for(var w:l.path("wafers")){if(w.path("result").asText().equals("success"))success++;if(w.path("result").asText().equals("scrap"))scrap++;}}
   int processing=0,faults=0;for(var p:s.path("chambers")){if(p.path("fault").asBoolean())faults++;if(p.path("remaining").asInt()>0)processing++;}
   Map<String,Object> m=new LinkedHashMap<>();m.put("equipmentId",id);var maintenance=s.path("maintenance").deepCopy();if(maintenance instanceof com.fasterxml.jackson.databind.node.ObjectNode object&&MaintenanceApi.held(s)&&s.path("maintenance").hasNonNull("run")&&s.path("maintenance").path("run").path("status").asText().equals("passed"))object.put("qualificationStatus",DummyApprovalApi.view(db,id,s).path("status").asText());m.put("maintenance",maintenance);m.put("communicationQuality",s.path("communicationQuality").asText("good"));m.put("sourceTime",s.path("sourceTime").asText());m.put("paused",s.path("paused").asBoolean());m.put("success",success);m.put("scrap",scrap);m.put("activeLots",active);m.put("processing",processing);m.put("faults",faults);m.put("vacuumHold",s.path("vacuumHold").asBoolean());m.put("locks",s.path("locks"));m.put("vacuumFault",java.util.stream.StreamSupport.stream(s.path("locks").spliterator(),false).anyMatch(l->l.path("pump").path("fault").asBoolean()));m.put("vacuumWarning",java.util.stream.StreamSupport.stream(s.path("locks").spliterator(),false).anyMatch(l->l.path("pump").path("warning").asBoolean()));return m;
  }).toList();
 }
}
