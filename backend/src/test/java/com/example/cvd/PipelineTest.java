package com.example.cvd;

import static org.mockito.Mockito.*;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.*;
import java.util.concurrent.CompletableFuture;
import org.eclipse.paho.client.mqttv3.MqttClient;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.kafka.core.KafkaTemplate;

class PipelineTest {
  @Test
  void snapshotReplayKeepsNewestVersionForEachEquipment() {
    var db = new DataStore(mock(JdbcTemplate.class), new ObjectMapper(), "jdbc:postgresql://localhost/cvd", "cvd", "test");
    var p = spy(new Pipeline(db, mock(KafkaTemplate.class), mock(StringRedisTemplate.class), mock(LiveSocket.class), "tcp://localhost:1883"));
    doNothing().when(p).persistSnapshots(anyList());
    String first = "{\"equipmentId\":\"TEL-CVDTIN-08A-01\",\"version\":9,\"vacuumHold\":true}";
    String second = "{\"equipmentId\":\"TEL-CVDTIN-08A-02\",\"version\":4}";
    String old = "{\"equipmentId\":\"TEL-CVDTIN-08A-01\",\"version\":3,\"vacuumHold\":false}";
    p.snapshots(List.of(first, second, old, first));
    verify(p).persistSnapshots(List.of(first, second));
  }

  @Test
  void mqttCallbackOnlyPersistsWork() throws Exception {
    var pg = mock(JdbcTemplate.class);
    var db =
        new DataStore(pg, new ObjectMapper(), "jdbc:postgresql://localhost/cvd", "cvd", "test");
    var pipeline =
        new Pipeline(
            db,
            mock(KafkaTemplate.class),
            mock(StringRedisTemplate.class),
            mock(LiveSocket.class),
            "tcp://localhost:1883");
    pipeline.mqtt = mock(MqttClient.class);
    pipeline.receive(
        "cvd/CVD-01/events",
        "{\"schemaVersion\":1,\"kind\":\"telemetry\",\"eventId\":\"duplicate-1\",\"sourceTime\":\"2026-09-15T00:00:00Z\"}");
    verifyNoInteractions(pipeline.mqtt);
    verify(pg, never()).update("UPDATE inbox SET sent=false WHERE id=?", "duplicate-1");
  }

  @Test
  void failedKafkaDeliveryDoesNotAcknowledge() {
    var pg = mock(JdbcTemplate.class);
    var db =
        new DataStore(pg, new ObjectMapper(), "jdbc:postgresql://localhost/cvd", "cvd", "test");
    var kafka = mock(KafkaTemplate.class);
    when(kafka.send(anyString(), anyString(), anyString()))
        .thenReturn(CompletableFuture.failedFuture(new RuntimeException("broker offline")));
    when(pg.queryForList(anyString()))
        .thenReturn(List.of(Map.of("id", "event-1", "topic", "cvd.events.v1", "body", "{}", "equipment_id", Equipment.FIRST)));
    var p =
        new Pipeline(
            db,
            kafka,
            mock(StringRedisTemplate.class),
            mock(LiveSocket.class),
            "tcp://localhost:1883");
    p.mqtt = mock(MqttClient.class);
    p.forward();
    verifyNoInteractions(p.mqtt);
    verify(pg, never()).update(org.mockito.ArgumentMatchers.startsWith("UPDATE inbox SET sent=true"), org.mockito.ArgumentMatchers.eq("event-1"));
  }
}
