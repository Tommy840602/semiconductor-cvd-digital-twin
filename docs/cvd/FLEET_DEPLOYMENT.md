# 十台 CVD 部署與復原

變更：cvd-multi-equipment。機台編碼 TEL-CVDTIN-08A-01 至 10；原機台與所有既有資料歸屬 01。十台共用基礎服務，各自使用 simulator／gateway 容器與持久卷。Compose 的 simulator、gateway 保留為 01 服務名，02 至 10 加數字後綴。

## 部署順序

1. 先完成隔離環境驗收，不向正式設備建立測試批次。保留舊映像標籤及舊部署設定。
2. 維護期間停止 frontend、backend、gateway、simulator，防止新操作及持續推進。停止後備份 PostgreSQL、TimescaleDB、simulator 與 gateway 卷；原 Kafka／MQTT／Redis 卷保留。記錄資料筆數、schema、帳號雜湊摘要及全部晶圓／簽核狀態。資料庫備份含敏感帳號資料，置於忽略版本控制的受限目錄。
3. 使用 `scripts/fleet-state-migration.py <原 state.json> <輸出目錄>` 產生十份狀態；可重跑，不覆寫既有輸出。01 保留晶圓、加工與簽核並暫停，其他九台空機且使用原 recipe 獨立副本。把各檔案放入对应機台的 `/data/state.json`，不可覆寫已有多機台業務狀態。
4. 啟動新版後端執行可重跑 schema 遷移，再啟動十組模擬器／閘道與前端。舊事件原文不改寫，歸屬欄統一對應 01；舊 CVD-01 積壓經遷移路由正規化，原唯一 ID 保留。
5. 驗證十台新鮮度、01 前後晶圓與簽核對帳、新九台無批次，以及登入、查詢及 recipe 複製。01 保持暫停交由操作者人工恢復。

## 回滾

未開放操作前驗收失敗：停止新版寫入，保留失敗現場，恢復成套的遷移前 PostgreSQL、TimescaleDB、simulator／gateway 卷及映像。不要讓舊服務直接使用新資料庫或新狀態檔。新部署已接受業務資料時，先匯出並對帳後決定恢復點，禁止直接丟棄新操作。

備份還原演練在隔離資料庫執行；`pg_restore --list` 僅能驗證可讀，不能取代實際還原與筆數／內容對帳。

## 例行運作

recipe 顯示保存版本與設備確認結果；保存成功但未收到設備確認時，不宣稱設備已套用。01 原晶圓的舊加工版本不補造；新加工記錄實際 recipeVersion。機台遷移後的舊 CVD-01 歷史原文以歸屬欄查詢。

全廠摘要不顯示常駐 Lot 清單；單台詳情依現有按需查詢。單機重啟只暫停該台，其他台繼續運作。每秒採樣與 30 天保留不因十台擴充降低，容量量測見 FLEET_VALIDATION.md。

投遞成功後，新增 inbox 快照／遙測僅保留去重與路由欄，清除已交付 payload；正式數據仍位於設備快照與 TimescaleDB，避免把完整批次快照再保存 30 天造成十倍磁碟浪費。未送達的 outbox payload 必須保留，重複來源事件只重送 ACK，不重複送 Kafka。

## 本次切換保存點

正式維護備份位於 `backups/cvd-fleet-cutover-20260917-052003/`（受限且不入版本控制）：PostgreSQL／TimescaleDB custom dump、原 state／spool、完整 simulator／gateway data tar、遷移後十台 state、前後對帳摘要及含舊映像標籤的 rollback-compose.json。

回滾時以該 compose 配置啟動舊映像。先停止全部新 simulator／gateway／backend／frontend；以備份還原兩個資料庫（TimescaleDB 要執行 timescaledb_pre_restore／timescaledb_post_restore），復原 01 的兩個 data 卷，再啟動舊系統。不要刪除新九台卷；保留作為失敗現場。`rollback-compose.json` 含環境設定，須按秘密檔管理。

可重跑的隔離驗收入口：`scripts/prepare-fleet-validation.py`、`tests/integration/fleet.py`、`fleet_isolation.py`、`fleet_recovery.py`、`fleet_metrics.py`、`fleet-ui.mjs`。整批測試要求乾淨的隔離資料，禁止對正式環境執行 fixture reset。
