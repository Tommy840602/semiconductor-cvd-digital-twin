package com.example.cvd;

import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.time.*;
import java.util.*;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

@RestController
@RequestMapping("/api/equipment/{equipmentId}")
public class Api {
  final DataStore db;
  final AccessStore access;
  final org.springframework.data.redis.core.StringRedisTemplate redis;

  public Api(DataStore db, org.springframework.data.redis.core.StringRedisTemplate redis, AccessStore access) {
    this.db = db;
    this.access = access;
    this.redis = redis;
  }

  @GetMapping
  public JsonNode equipment() {
    String authoritative = db.snapshot();
    try {
      String cached = redis.opsForValue().get("cvd:"+Equipment.current()+":latest");
      if (cached != null
          && db.parse(cached).path("version").asLong()
              == db.parse(authoritative).path("version").asLong()
          && db.parse(cached).path("equipmentId").equals(db.parse(authoritative).path("equipmentId"))
          && db.parse(cached).path("communicationQuality").equals(db.parse(authoritative).path("communicationQuality"))) return db.parse(cached);
      redis.opsForValue().set("cvd:"+Equipment.current()+":latest", authoritative);
    } catch (Exception ignored) {
    }
    return db.parse(authoritative);
  }

  @GetMapping("/lots")
  public JsonNode lots() {
    return equipment().path("lots");
  }

  @PostMapping("/commands")
  public Map<String, Object> command(@RequestBody ObjectNode c) {
    String equipment=Equipment.current();
    if(c.has("equipmentId")&&!equipment.equals(c.path("equipmentId").asText()))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"機台識別不符");
    c.put("equipmentId",equipment);
    String id = c.path("commandId").asText();
    if (id.isBlank() || id.length() > 128)
      throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "commandId required");
    if (c.path("type").asText().isBlank())
      throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "type required");
    var existing=db.pg.queryForList("SELECT equipment_id FROM commands WHERE id=?",String.class,id);
    if(!existing.isEmpty()&&!equipment.equals(existing.getFirst()))throw new ResponseStatusException(HttpStatus.CONFLICT,"指令屬於另一機台");
    String lot=c.path("lotId").asText();
    if(!lot.isBlank()){
      boolean found=false;for(var l:equipment().path("lots"))if(l.path("id").asText().equals(lot))found=true;
      if(!c.path("type").asText().equals("start")&&!found)throw new ResponseStatusException(HttpStatus.NOT_FOUND,"本機批次不存在");
      if(c.path("type").asText().equals("start")&&!lot.startsWith(equipment+"-"))throw new ResponseStatusException(HttpStatus.BAD_REQUEST,"新批次需包含機台前綴");
    }
    var actor = AccessSessions.actor();
    String kind=c.path("type").asText();
    if(Set.of("start","resume").contains(kind)&&equipment().path("dispositionHold").asBoolean())throw new ResponseStatusException(HttpStatus.CONFLICT,"需先完成異常處置並由 Leader 核准整台放行");
    boolean basic=Set.of("start","pause","stop").contains(kind);
    boolean advanced=Set.of("vacuum-service-channel","vacuum-recover","vacuum-test","fault","clear","recovery-clock","resume","maintenance-request","maintenance-end","maintenance-carrier-load","qualification-start","maintenance-carrier-restore").contains(kind);
    if (!basic && !(advanced && Set.of("Engineer","Leader").contains(actor.role()))) {
      access.audit(actor,"command:"+kind,c.path("targetId").asText(),id,"denied");
      throw new ResponseStatusException(HttpStatus.FORBIDDEN,"無此操作權限");
    }
    if(!existing.isEmpty())return command(id);
    if (!c.has("expiresAt")) c.put("expiresAt", Instant.now().plusSeconds(30).toString());
    c.remove(List.of("actor","role","userId","username"));
    c.set("actor",db.mapper.valueToTree(actor));
    access.tx.executeWithoutResult(status -> {
      MaintenanceApi.guard(db,c);
      int inserted=db.pg.update(
        "INSERT INTO commands(id,body,status,equipment_id) VALUES(?,?::jsonb,'received',?) ON CONFLICT DO NOTHING",id,c.toString(),equipment);
      access.audit(actor,"command:"+kind,c.path("targetId").asText(),id,inserted==1?"accepted":"duplicate");
    });
    return command(id);
  }

  @GetMapping("/commands/{id}")
  public Map<String, Object> command(@PathVariable String id) {
    var r =
        db.pg.queryForList(
            "SELECT id,body->>'type' type,body->>'targetId' target_id,body->>'lotId'"
                + " lot_id,body->'actor'->>'username' actor,body->'actor'->>'role' actor_role,status,result::text result,timed_out,created_at FROM commands WHERE"
                + " id=? AND equipment_id=?",
            id,Equipment.current());
    if (r.isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
    return r.getFirst();
  }

  @GetMapping("/commands")
  public List<Map<String, Object>> commands() {
    return db.pg.queryForList(
        "SELECT id,body->>'type' type,body->>'targetId' target_id,body->>'lotId'"
            + " lot_id,body->'actor'->>'username' actor,body->'actor'->>'role' actor_role,status,result::text result,timed_out,created_at FROM commands WHERE equipment_id=? ORDER BY"
            + " created_at DESC LIMIT 100",Equipment.current());
  }

  @GetMapping({"/history", "/wafers/{wafer}/history"})
  public List<JsonNode> history(
      @PathVariable(required = false) String wafer,
      @RequestParam(defaultValue = "") String lotId,
      @RequestParam(defaultValue = "0") int page,
      @RequestParam(required = false) String from,
      @RequestParam(required = false) String to) {
    Instant end = to == null ? Instant.now() : Instant.parse(to);
    Instant begin = from == null ? Instant.now().minus(Duration.ofDays(30)) : Instant.parse(from);
    Instant cutoff = Instant.now().minus(Duration.ofDays(30));
    if (begin.isBefore(cutoff)) begin = cutoff;
    return db
        .pg
        .queryForList(
            "SELECT (body || jsonb_build_object('actor',(SELECT c.body->'actor' FROM commands c WHERE c.id=history.body#>>'{payload,commandId}' AND c.equipment_id=history.equipment_id)))::text FROM history WHERE equipment_id=? AND time>=?::timestamptz AND time<?::timestamptz AND"
                + " (?='' OR lot_id=?) AND (?='' OR wafer_id=?) ORDER BY time,id LIMIT 100 OFFSET"
                + " ?",
            String.class,
            Equipment.current(),begin.toString(),
            end.toString(),
            lotId,
            lotId,
            wafer == null ? "" : wafer,
            wafer == null ? "" : wafer,
            Math.min(Math.max(0, page), 100000) * 100)
        .stream()
        .map(db::parse)
        .toList();
  }

  @GetMapping("/alarms")
  public List<JsonNode> alarms() {
    return db
        .pg
        .queryForList(
            "SELECT body::text FROM (SELECT time,body FROM history WHERE equipment_id=? AND kind IN"
                + " ('alarm','data-gap') UNION ALL SELECT created_at,body FROM dead_letter WHERE equipment_id=?) a WHERE"
                + " time>=now()-interval '30 days' ORDER BY time DESC LIMIT 100",
            String.class,Equipment.current(),Equipment.current())
        .stream()
        .map(db::parse)
        .toList();
  }

  @GetMapping(value="/telemetry", params="!metric")
  public List<JsonNode> telemetry(
      @RequestParam(required = false) String from,
      @RequestParam(required = false) String to,
      @RequestParam(defaultValue = "") String waferId,
      @RequestParam(defaultValue = "") String pmId) {
    Instant end = to == null ? Instant.now() : Instant.parse(to);
    Instant begin = from == null ? end.minusSeconds(300) : Instant.parse(from);
    Instant cutoff = Instant.now().minus(Duration.ofDays(30));
    if (begin.isBefore(cutoff)) begin = cutoff;
    if (!begin.isBefore(end)) return List.of();
    boolean aggregate = Duration.between(begin, end).toHours() >= 1;
    if (aggregate) {
      String sql =
          "SELECT"
              + " jsonb_build_object('sourceTime',bucket,'payload',jsonb_build_object('chambers',jsonb_agg(jsonb_build_object('id',pm,'temperature',temperature,'pressure',pressure,'flow',flow))))::text"
              + " FROM (SELECT time_bucket('1 minute',time) bucket,p->>'id'"
              + " pm,avg((p->>'temperature')::numeric) temperature,avg((p->>'pressure')::numeric)"
              + " pressure,avg((p->>'flow')::numeric) flow FROM telemetry CROSS JOIN LATERAL"
              + " jsonb_array_elements(body#>'{payload,chambers}') p WHERE equipment_id=? AND time>=?::timestamptz AND"
              + " time<?::timestamptz AND (?='' OR p->>'waferId'=?) AND (?='' OR p->>'id'=?) GROUP"
              + " BY bucket,pm) x GROUP BY bucket ORDER BY bucket DESC LIMIT 2000";
      return db
          .ts
          .queryForList(
              sql, String.class, Equipment.current(),begin.toString(), end.toString(), waferId, waferId, pmId, pmId)
          .stream()
          .map(db::parse)
          .toList();
    }
    return db
        .ts
        .queryForList(
            "SELECT body::text FROM telemetry WHERE equipment_id=? AND time>=?::timestamptz AND time<?::timestamptz"
                + " AND EXISTS (SELECT 1 FROM jsonb_array_elements(body#>'{payload,chambers}') p"
                + " WHERE (?='' OR p->>'waferId'=?) AND (?='' OR p->>'id'=?)) ORDER BY time DESC"
                + " LIMIT 2000",
            String.class,
            Equipment.current(),begin.toString(),
            end.toString(),
            waferId,
            waferId,
            pmId,
            pmId)
        .stream()
        .map(db::parse)
        .toList();
  }

  @ExceptionHandler({IllegalArgumentException.class, java.time.format.DateTimeParseException.class})
  @ResponseStatus(HttpStatus.BAD_REQUEST)
  public Map<String, String> invalid(Exception e) {
    return Map.of("message", "查詢時間或參數格式不正確");
  }
}
