package com.example.cvd;

import static org.junit.jupiter.api.Assertions.*;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.*;
import org.junit.jupiter.api.Test;

class ContractTest {
 @Test void processFixturePreservesNewAndLegacyFields() throws Exception {
 var m=new ObjectMapper();var e=m.readTree(Files.readString(Path.of("../contracts/process-event.json")));var p=e.path("payload").path("chambers").get(0);
 assertEquals("synthetic-tin-v1",p.path("processTelemetry").path("modelRevision").asText());assertEquals(40,p.path("gasChannels").path("precursor").path("actual").asInt());assertEquals(395,p.path("temperature").asInt());assertEquals(e,m.readTree(m.writeValueAsString(e)));
 }

  @Test
  void envelopeRoundTripsAcrossLanguages() throws Exception {
    var m = new ObjectMapper();
    var e = m.readTree(Files.readString(Path.of("../contracts/example-event.json")));
    assertEquals("L1-W01", e.path("waferId").asText());
    assertEquals(1, e.path("sequence").asLong());
    assertEquals(e, m.readTree(m.writeValueAsString(e)));
  }
}
