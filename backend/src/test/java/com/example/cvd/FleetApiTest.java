package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import java.util.*;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import com.fasterxml.jackson.databind.ObjectMapper;
class FleetApiTest {
 @Test void bulkReadPreservesEquipmentOrderAndMissingTools() {
  var pg=mock(JdbcTemplate.class);var db=new DataStore(pg,new ObjectMapper(),"jdbc:postgresql://localhost/cvd","cvd","test");
  try {
   when(pg.queryForList(anyString())).thenReturn(List.of(Map.of("id",Equipment.IDS.get(1),"body","{\"communicationQuality\":\"offline\",\"vacuumHold\":true,\"locks\":[{\"id\":\"LL1\",\"pump\":{\"fault\":true}}]}")));
   var list=new FleetApi(db).list();assertEquals(10,list.size());assertEquals(Equipment.FIRST,list.getFirst().get("equipmentId"));assertEquals(true,list.get(1).get("vacuumFault"));assertEquals("offline",list.get(1).get("communicationQuality"));assertEquals(false,list.getFirst().get("vacuumHold"));
   verify(pg,times(1)).queryForList(anyString());
  } finally {((com.zaxxer.hikari.HikariDataSource)db.ts.getDataSource()).close();}
 }
}
