package com.example.cvd;

import com.fasterxml.jackson.databind.*;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;
import org.eclipse.paho.client.mqttv3.*;
import org.eclipse.paho.client.mqttv3.persist.MqttDefaultFilePersistence;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class Pipeline {
  final DataStore db;
  @org.springframework.beans.factory.annotation.Autowired UtilizationStore utilization;
  @org.springframework.beans.factory.annotation.Autowired StatisticsStore statistics;
  @org.springframework.beans.factory.annotation.Autowired MaintenanceApi maintenance;
  final KafkaTemplate<String, String> kafka;
  final StringRedisTemplate redis;
  final LiveSocket socket;
  final String mqttUrl;
  MqttClient mqtt;
  boolean secs(){return "secs".equals(System.getenv("CVD_PROTOCOL"));}
  String signControl(String body) throws Exception {
    byte[] key=java.nio.file.Files.readAllBytes(java.nio.file.Path.of(System.getenv().getOrDefault("CVD_CONTROL_KEY_FILE","/run/secrets/control_key")));
    if(key.length<32)throw new IllegalStateException("control key too short");
    String version="cvd-secs-sparkplug-v1";
    var mac=javax.crypto.Mac.getInstance("HmacSHA256");mac.init(new javax.crypto.spec.SecretKeySpec(key,"HmacSHA256"));mac.update((version+"\n").getBytes(StandardCharsets.UTF_8));
    byte[] payload=body.getBytes(StandardCharsets.UTF_8);
    return db.mapper.writeValueAsString(Map.of("schemaVersion",version,"payload",Base64.getEncoder().encodeToString(payload),"signature",HexFormat.of().formatHex(mac.doFinal(payload))));
  }
  record Incoming(String topic,String body,int id,int qos,long generation){}
  final java.util.concurrent.ArrayBlockingQueue<Incoming> incoming=new java.util.concurrent.ArrayBlockingQueue<>(5000);
  volatile long generation=0;
  final java.util.concurrent.ConcurrentHashMap<String,String> acknowledgements=new java.util.concurrent.ConcurrentHashMap<>();

  public Pipeline(
      DataStore db,
      KafkaTemplate<String, String> kafka,
      StringRedisTemplate redis,
      LiveSocket socket,
      @Value("${MQTT_URL:tcp://localhost:1883}") String mqttUrl) {
    this.db = db;
    this.kafka = kafka;
    this.redis = redis;
    this.socket = socket;
    this.mqttUrl = mqttUrl;
  }

  @Scheduled(fixedDelay = 3000)
  synchronized void connect() {
    if(secs())return;
    try {
      if (mqtt != null && mqtt.isConnected()) return;
      if (mqtt == null) {
        mqtt =
            new MqttClient(
                mqttUrl,
                "cvd-backend",
                new MqttDefaultFilePersistence(
                    System.getenv().getOrDefault("MQTT_DATA_DIR", ".runtime/mqtt")));
        mqtt.setCallback(
            new MqttCallback() {
              public void connectionLost(Throwable cause) {}

              public void deliveryComplete(IMqttDeliveryToken t) {}

              public void messageArrived(String topic, MqttMessage m) throws Exception {
                if(!incoming.offer(new Incoming(topic,new String(m.getPayload(),StandardCharsets.UTF_8),m.getId(),m.getQos(),generation)))throw new MqttException(MqttException.REASON_CODE_CLIENT_EXCEPTION);
              }
            });
      }
      var o = new MqttConnectOptions();
      o.setCleanSession(false);
      o.setMaxInflight(1000);
      o.setConnectionTimeout(3);
      mqtt.setManualAcks(true);generation++;
      mqtt.connect(o);
      mqtt.subscribe(
          new String[] {
            "cvd/+/telemetry",
            "cvd/+/events",
            "cvd/+/command-results",
            "cvd/+/snapshot"
          },
          new int[] {1, 1, 1, 1});
    } catch (Exception e) {
      System.err.println("MQTT reconnect: " + e.getMessage());
    }
  }

  @Scheduled(fixedDelay=50)
  void ingest(){
    var batch=new ArrayList<Incoming>();incoming.drainTo(batch,250);if(batch.isEmpty())return;
    try{
      new org.springframework.transaction.support.TransactionTemplate(new org.springframework.jdbc.datasource.DataSourceTransactionManager(db.pg.getDataSource())).executeWithoutResult(tx->{for(var m:batch)receive(m.topic(),m.body());});
      for(var m:batch)if(m.generation()==generation&&mqtt!=null&&mqtt.isConnected())mqtt.messageArrivedComplete(m.id(),m.qos());
    }catch(Exception ex){synchronized(this){try{if(mqtt!=null)mqtt.disconnectForcibly(0,0);}catch(Exception ignored){}incoming.clear();}System.err.println("Inbound retry: "+ex.getMessage());}
  }
  void receive(String topic, String body) {
    JsonNode j;
    String equipment;
    boolean snapshot = topic.endsWith("/snapshot");
    try {
      j = db.parse(body);
      equipment=Equipment.legacy(topic.split("/")[1]);
      String source=j.path("equipmentId").asText();
      if(source.isEmpty() && topic.startsWith("cvd/CVD-01/")) source="CVD-01";
      if(!equipment.equals(Equipment.legacy(source))) throw new IllegalArgumentException("equipment mismatch");
      if (snapshot) {
        if (!j.has("version") || !j.has("epoch"))
          throw new IllegalArgumentException("invalid snapshot");
      } else {
        if (j.path("schemaVersion").asInt() != 1
            || j.path("eventId").asText().isBlank()
            || j.path("kind").asText().isBlank())
          throw new IllegalArgumentException("invalid event envelope");
        Instant.parse(j.path("sourceTime").asText());
      }
    } catch (RuntimeException ex) {
      var q = db.mapper.createObjectNode();
      q.put("kind", "invalid-message");
      q.put("sourceTime", Instant.now().toString());
      q.put("topic", topic);
      q.put("payload", ex.getMessage());
      q.put("raw", body);
      db.pg.update(
          "INSERT INTO dead_letter(id,body,equipment_id) VALUES(?,?::jsonb,?)",
          UUID.randomUUID().toString(),
          q.toString(),topic.split("/").length>1&&Equipment.IDS.contains(topic.split("/")[1])?topic.split("/")[1]:Equipment.FIRST);
      return;
    }
    if (!snapshot && j instanceof com.fasterxml.jackson.databind.node.ObjectNode o) {
      o.put("ingestedAt", Instant.now().toString());
      body = o.toString();
    }
    String id =
        snapshot
            ? "snapshot-" + j.path("epoch").asText() + "-" + j.path("version").asLong()
            : j.path("eventId").asText();
    if (id.isBlank()) throw new IllegalArgumentException("eventId required");
    String stream = snapshot ? "snapshot" : topic.substring(topic.lastIndexOf('/') + 1);
    db.pg.update(
        "INSERT INTO inbox(id,topic,body,equipment_id) VALUES(?,?,?::jsonb,?) ON CONFLICT DO NOTHING",
        id,
        "cvd." + stream + ".v1",
        body,equipment);
    // Never publish synchronously from the MQTT callback: replay through the outbox.
    if (!snapshot && Boolean.TRUE.equals(db.pg.queryForObject("SELECT sent FROM inbox WHERE id=?",Boolean.class,id))) acknowledgements.put(id,equipment);
  }

  String routed(String body,String equipment) {
    var n=(com.fasterxml.jackson.databind.node.ObjectNode)db.parse(body);n.put("equipmentId",Equipment.require(equipment));
    if(Equipment.FIRST.equals(equipment)&&Set.of("wafer-supplement","cancel-supplement").contains(n.path("type").asText())){
      var params=n.path("parameters");String raw=params.path("payload").asText();var prescription=(com.fasterxml.jackson.databind.node.ObjectNode)db.parse(raw);
      if(!prescription.has("equipmentId"))try{
        String key=java.nio.file.Files.readString(java.nio.file.Path.of(System.getenv("CVD_RECOVERY_KEY_FILE"))).strip();
        var mac=javax.crypto.Mac.getInstance("HmacSHA256");mac.init(new javax.crypto.spec.SecretKeySpec(key.getBytes(StandardCharsets.UTF_8),"HmacSHA256"));
        String type=n.path("type").asText();byte[] signature=HexFormat.of().parseHex(params.path("signature").asText());
        if(!java.security.MessageDigest.isEqual(signature,mac.doFinal((type+"\n"+raw).getBytes(StandardCharsets.UTF_8))))throw new IllegalArgumentException("legacy signature mismatch");
        prescription.put("equipmentId",equipment);String migrated=prescription.toString();((com.fasterxml.jackson.databind.node.ObjectNode)params).put("payload",migrated).put("signature",HexFormat.of().formatHex(mac.doFinal((type+"\n"+migrated).getBytes(StandardCharsets.UTF_8))));
      }catch(Exception ex){throw new IllegalArgumentException("cannot bind legacy approval",ex);}
    }
    return n.toString();
  }
  @Scheduled(fixedDelay = 100)
  void forward() {
    var rows=db.pg.queryForList("SELECT id,topic,equipment_id,body::text body FROM inbox WHERE NOT sent ORDER BY created_at,id LIMIT 500");
    var pending=new ArrayList<java.util.concurrent.CompletableFuture<?>>();
    for(var row:rows)pending.add(kafka.send(row.get("topic").toString(),row.get("equipment_id").toString(),routed(row.get("body").toString(),row.get("equipment_id").toString())));
    for(int i=0;i<rows.size();i++){
      var row=rows.get(i);try{
        pending.get(i).get(5,java.util.concurrent.TimeUnit.SECONDS);
        String id=row.get("id").toString();
        if(!id.startsWith("snapshot-"))acknowledgements.put(id,row.get("equipment_id").toString());
        db.pg.update("UPDATE inbox SET sent=true,body=CASE WHEN topic IN ('cvd.snapshot.v1','cvd.telemetry.v1') THEN '{}'::jsonb ELSE body END WHERE id=?",id);
      }catch(Exception e){System.err.println("Forward retry: "+e.getMessage());break;}
    }
  }

  @Scheduled(fixedDelay = 100)
  void acknowledge(){
    if(mqtt==null||!mqtt.isConnected())return;
    int count=0;for(var entry:acknowledgements.entrySet()){
      if(count++>=100)break;
      try{mqtt.getTopic("cvd/"+entry.getValue()+"/acks").publish(entry.getKey().getBytes(StandardCharsets.UTF_8),1,false);acknowledgements.remove(entry.getKey(),entry.getValue());}
      catch(MqttException e){break;}
    }
  }
  @Scheduled(fixedDelay = 500)
  void sendCommands() {
    if (!secs() && (mqtt == null || !mqtt.isConnected())) return;
    for (var row :
        db.pg.queryForList(
            "SELECT id,equipment_id,body::text body FROM commands WHERE status IN ('received','accepted') AND (last_sent IS NULL"
                + " OR last_sent<now()-interval '3 seconds') ORDER BY created_at")) {
      try {
        if(secs()){kafka.send("cvd.commands.v1",row.get("equipment_id").toString(),signControl(routed(row.get("body").toString(),row.get("equipment_id").toString()))).get(5,java.util.concurrent.TimeUnit.SECONDS);}else mqtt.publish(
            "cvd/"+row.get("equipment_id")+"/commands",
            routed(row.get("body").toString(),row.get("equipment_id").toString()).getBytes(StandardCharsets.UTF_8),
            1,
            false);
        db.pg.update("UPDATE commands SET sent=true,last_sent=now() WHERE id=?", row.get("id"));
      } catch (Exception e) {
        break;
      }
    }
    db.pg.update(
        "UPDATE commands SET timed_out=true WHERE status IN ('received','accepted') AND created_at<now()-interval"
            + " '30 seconds'");
  }

  @KafkaListener(
      topics = {"cvd.events.v1", "cvd.command-results.v1"},
      groupId = "cvd-history")
  void history(String body) {
    var j = db.parse(body);
    maintenance.observe(j);
    statistics.closed(j);
    db.pg.update(
        "INSERT INTO history(id,time,lot_id,wafer_id,kind,body,equipment_id)"
            + " VALUES(?,?::timestamptz,?,?,?,?::jsonb,?) ON CONFLICT DO NOTHING",
        j.path("eventId").asText(),
        j.path("sourceTime").asText(),
        j.path("lotId").asText(),
        j.path("waferId").asText(),
        j.path("kind").asText(),
        body,Equipment.legacy(j.path("equipmentId").asText()));
    if (j.path("kind").asText().equals("command-result")) {
      var r = j.path("payload");
      db.pg.update(
          "UPDATE commands SET status=?,result=?::jsonb WHERE id=? AND equipment_id=?",
          r.path("status").asText(),
          r.toString(),
          r.path("commandId").asText(),Equipment.legacy(j.path("equipmentId").asText()));
    }
  }

  @KafkaListener(topics = "cvd.command-accepted.v1", groupId = "cvd-command-accepted")
  void accepted(String body){var j=db.parse(body);db.pg.update("UPDATE commands SET status='accepted' WHERE id=? AND equipment_id=? AND status='received'",j.path("commandId").asText(),Equipment.require(j.path("equipmentId").asText()));}

  @KafkaListener(topics = "cvd.connectivity.v1", groupId = "cvd-connectivity", batch = "true")
  void connectivity(List<String> bodies) {
    var latest = new LinkedHashMap<String,JsonNode>();
    for (String body : bodies) {
      var next=db.parse(body);String id=Equipment.require(next.path("equipmentId").asText());
      var old=latest.get(id);
      if(old==null || Instant.parse(next.path("sourceTime").asText()).isAfter(Instant.parse(old.path("sourceTime").asText())))latest.put(id,next);
    }
    if(latest.isEmpty())return;
    String batch="["+String.join(",",latest.values().stream().map(JsonNode::toString).toList())+"]";
    var changed=db.pg.queryForList("INSERT INTO equipment_connectivity(equipment_id,quality,source_time) SELECT v->>'equipmentId',v->>'quality',(v->>'sourceTime')::timestamptz FROM jsonb_array_elements(?::jsonb) v ON CONFLICT(equipment_id) DO UPDATE SET quality=excluded.quality,source_time=excluded.source_time WHERE equipment_connectivity.source_time<excluded.source_time RETURNING equipment_id",String.class,batch);
    for(String id:changed)if(socket.subscribed(id))socket.broadcast(db.snapshot(id));
  }

  @KafkaListener(topics = "cvd.telemetry.v1", groupId = "cvd-timeseries")
  void telemetry(String body) {
    var j = db.parse(body);
    utilization.ingest(j);
    statistics.ingest(j);
    db.ts.update(
        "INSERT INTO telemetry(time,id,body,equipment_id) SELECT ?::timestamptz,?,?::jsonb,? WHERE"
            + " ?::timestamptz>=now()-interval '30 days' ON CONFLICT DO NOTHING",
        j.path("sourceTime").asText(),
        j.path("eventId").asText(),
        body,Equipment.legacy(j.path("equipmentId").asText()),
        j.path("sourceTime").asText());
  }

  @KafkaListener(topics = "cvd.snapshot.v1", groupId = "cvd-snapshot", batch = "true",
      properties = {"max.partition.fetch.bytes=16777216", "fetch.max.bytes=33554432"})
  void snapshots(List<String> bodies) {
    // Snapshots are replaceable views; telemetry and audit events retain their own consumers.
    var latest = new LinkedHashMap<String, JsonNode>();
    for (String body : bodies) {
      var next = db.parse(body);
      String equipment = Equipment.legacy(next.path("equipmentId").asText());
      var previous = latest.get(equipment);
      if (previous == null || next.path("version").asLong() > previous.path("version").asLong())
        latest.put(equipment, next);
    }
    persistSnapshots(latest.values().stream().map(next -> {
      ((com.fasterxml.jackson.databind.node.ObjectNode) next).put("equipmentId", Equipment.legacy(next.path("equipmentId").asText()));
      return next.toString();
    }).toList());
  }

  void persistSnapshots(List<String> bodies) {
    if (bodies.isEmpty()) return;
    String batch = "[" + String.join(",", bodies) + "]";
    var changed = db.pg.queryForList("WITH changed AS (INSERT INTO equipment_snapshot(id,version,body) "
        + "SELECT v->>'equipmentId',(v->>'version')::bigint,v FROM jsonb_array_elements(?::jsonb) v "
        + "ON CONFLICT(id) DO UPDATE SET version=excluded.version,body=excluded.body,updated_at=now() "
        + "WHERE equipment_snapshot.version<excluded.version RETURNING id,body) "
        + "SELECT s.id,(s.body||jsonb_build_object('communicationQuality',coalesce(c.quality,'good')))::text body "
        + "FROM changed s LEFT JOIN equipment_connectivity c ON c.equipment_id=s.id", batch);
    try {
      redis.executePipelined((org.springframework.data.redis.core.RedisCallback<Object>) connection -> {
        for (var row : changed) {
          String id = row.get("id").toString(), body = row.get("body").toString();
          connection.stringCommands().set(("cvd:"+id+":latest").getBytes(StandardCharsets.UTF_8), body.getBytes(StandardCharsets.UTF_8));
          connection.stringCommands().setEx(("cvd:"+id+":heartbeat").getBytes(StandardCharsets.UTF_8), 5, db.parse(body).path("sourceTime").asText().getBytes(StandardCharsets.UTF_8));
        }
        return null;
      });
    } catch (Exception e) {
      System.err.println("Redis unavailable; using persisted snapshot");
    }
    for (var row : changed) socket.broadcast(row.get("body").toString());
  }
}
