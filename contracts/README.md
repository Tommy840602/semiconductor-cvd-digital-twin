> 正式鏈路已改為 [SECS/GEM](SECS_GEM_PROFILE.md) 與 [Sparkplug B](SPARKPLUG.md)。下方應用事件／控制資料結構仍沿用；OPC UA 傳輸說明僅作舊版回滾參考。

# CVD 通訊契約 v1

情境設備 ID 為 `CVD-01`。OPC UA namespace URI 為 `urn:cvd:model`，client 必須動態解析 index。

| OPC UA 節點 | 內容 |
|---|---|
| `Snapshot` | 唯讀 String：設備狀態 JSON，支援資料變更訂閱 |
| `Events` | 唯讀 String：尚未確認事件的 JSON 陣列，支援訂閱 |
| `Command` | Objects 下的方法；一個 String JSON 輸入，一個 String JSON 結果 |
| `Ack` | Objects 下的方法；一個 Int64 連續事件序號，僅刪除已持久交付的前綴 |

```json
{"commandId":"example-001","type":"start","lotId":"LOT-001","targetId":"","parameters":{"lp":"LP1","ll":"LL2"},"expiresAt":"2026-09-15T12:00:00Z","expectedStateVersion":0}
```

`expectedStateVersion=0` 表示不要求精確版本；執行端仍檢查所有操作狀態。重複 commandId 返回原結果，無論重送參數為何都不再次執行。

```json
{"commandId":"example-001","status":"succeeded","reason":""}
```

命令 type：`start`、`pause`、`stop`、`resume`、`fault`、`clear`、`recovery-clock`、`scrap`、`configure`。`configure` 目前設定 `processSeconds`（5–3600）；這是合成配方加工時間。`stop` 為停止新派片並讓現有加工完成回片，可人工恢復。初次啟動無批次，使用者自行投片；批次各 25 片。未處置故障晶圓不得恢復。

MQTT 路徑前綴 `cvd/CVD-01/`：`telemetry`、`events`、`command-results`、`snapshot`、`commands`、`acks`。QoS 1，控制及狀態不 retained。acks 的 payload 是 eventId，後端僅在 Kafka 已確認後發出；重複訊息重送同一 ACK。

```json
{"schemaVersion":1,"eventId":"epoch-21","equipmentId":"CVD-01","sourceEpoch":"epoch","sequence":21,"sourceTime":"2026-09-15T12:00:00Z","quality":"good","lotId":"LOT-001","waferId":"LOT-001-W01","kind":"movement-complete","payload":{"from":"LL2","to":"PM1","robot":"VAC"}}
```

Kafka topics 為 `cvd.telemetry.v1`、`cvd.events.v1`、`cvd.command-results.v1`、`cvd.snapshot.v1`。以設備 ID 分區。不同 topic 不假設全域順序；使用 eventId 去重及快照 version 比較。

快照包含 version、epoch、sourceTime、paused、dispatchPaused、processSeconds、lots、locks、chambers、atm、vac。每片 wafer 的 id、lotId、slot、location、result、attempts 是履歷關聯鍵；成功／報廢與位置獨立。

HTTP 控制先回傳持久化接收狀態，不代表完成；`GET /api/commands/{id}` 取得執行結果。超過等待時間會顯示結果未知，晚到結果仍可更新。WebSocket `/ws` 傳送完整版本快照，連線建立時前端另以 `GET /api/equipment` 同步。

本機展示使用內部網路 OPC UA None／anonymous 與 MQTT anonymous，未開放設備協定端口至主機。非實機部署配置。

## 單片 LL（modelVersion 2）

每個 LL 容量 1 片；lot.wafers[].slot 是原 LP 載具槽位（1–25），不是 LL 槽位。lot.ll 為該批選定的往返 LL。LL.lotId 表示批次使用權，實際在槽晶圓由 wafer.location 判斷，另計入手臂目的地預留。每片單獨抽氣／回填；批次全數結案回 LP 才完成。capacity-migration 是容量改版的檢查點位置調整事件，不是物理搬送事件。

## 登入與角色

HTTP `/api/**` 與 WebSocket `/ws` 需要登入 session（僅 `/api/auth/csrf`、`/api/auth/login` 公開）。先 GET `/api/auth/csrf` 取得 token 與 Cookie，再 POST `/api/auth/login` 傳 username/password 及 `X-CSRF-Token`。登入會輪替 Cookie／token，之後所有 POST 使用新 token；GET `/api/auth/me` 取得角色，POST `/api/auth/logout` 使 session 與即時連線失效。

Operator 可 start/pause/stop；Engineer、Leader 另可 configure/fault/clear/recovery-clock/scrap/resume。POST `/api/users` 僅 Leader 可使用，role 限 Operator／Engineer。匿名 401、權限不足 403、重複帳號 409、登入限流 429、登入儲存服務不可用 503。

後端覆寫控制命令的 actor（id、username、role），不信任客戶端身分欄位；同一 commandId 的原操作者不變。GET `/api/audit` 提供最近 100 筆操作稽核，透過 command_id 關聯 execution_status。既有事件無身分時維持未記錄。

## 分步 Recipe

`configure` 可傳 `parameters.stepSeconds`，完整包含 `preheat`、`stabilize`、`deposition`、`purge` 四個整數秒數；每步 1–3600，總和最多 3600。總時間由系統計算；仍須 Engineer／Leader 且沒有進行中批次。非法參數整組拒絕，不部分套用。舊 `parameters.processSeconds`（5–3600 整數）仍可使用，會恢復原比例分配。

快照 `recipeSteps` 依上述順序保存四步設定；未設定時沿用既有比例。每次加工的 `processTelemetry.stepSeconds` 與 process-result 另保存當次四步秒數，重啟可延續，歷史不隨之後設定而變動。

單片補做與 EE／PE 簽核、部門 API、取消確認、膜厚延續及 checkpoint v5 見 [RECOVERY.md](RECOVERY.md)。舊 rework 已停用。
