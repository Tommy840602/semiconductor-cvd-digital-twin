package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.Test;
import org.springframework.web.server.ResponseStatusException;
class ProcessApiTest {
 @Test void invalidMetricAndEquipmentRejectedBeforeDatabase(){var api=new ProcessApi(null);
  assertThrows(ResponseStatusException.class,()->api.metric("unknown","PM1","","","",null,null));
  assertThrows(ResponseStatusException.class,()->api.metric("power","","LL1","","",null,null));
  assertThrows(ResponseStatusException.class,()->api.metric("temperature","PM9","","","",null,null));
  assertThrows(ResponseStatusException.class,()->api.metric("pressure","PM1","LL1","","",null,null));
 }
 @Test void reversedIntervalIsEmpty(){var api=new ProcessApi(null);var r=api.metric("temperature","PM1","","","","2099-01-02T00:00:00Z","2099-01-01T00:00:00Z");assertEquals(java.util.List.of(),r.get("points"));}
}
