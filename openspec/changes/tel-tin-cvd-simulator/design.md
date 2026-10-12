## Context

動機見 proposal.md。此為多服務新建設計，需明確定義狀態擁有者、事件一致性與恢復流程，因此建立本設計文件。使用者確認的設備及操作規則見各能力規格；下列未經逐項確認的技術細節屬本提案設計預設，可於實作前修訂。

## Goals / Non-Goals

**Goals:** 以實際 OPC UA 與 MQTT 通訊展示雙向控制，維持晶圓位置及數量一致，提供可追溯的批次履歷與可恢復資料管線。

**Non-Goals:** 不接實機，不提供 TEL 原廠配方或精確物理沉積模型；不納入 MES、SECS/GEM、多租戶、高可用叢集及 3D 模型。模擬設定不代表可用於真實製程的參數。

## Decisions

### 服務與資料流

採 Vue 3 JavaScript、Java 21 Spring Boot，以及兩個 Go 服務。Go simulator 是設備狀態唯一權威；Go gateway 只做 OPC UA client、格式轉換與 MQTT 通訊，避免兩處各自計算排程。相較直接生成 MQTT 假資料，此方案能驗證 OPC UA 訂閱與控制。

```text
Go Simulator --OPC UA--> Go Gateway --MQTT--> Spring Ingestion --Kafka--+
                                                                      |
                      +-----------------------------------------------+
                      v                 v                 v
                 TimescaleDB       PostgreSQL       Redis / WebSocket
                                                          |
                                                          v
                                                         Vue

Vue --HTTP--> Spring --MQTT--> Go Gateway --OPC UA--> Go Simulator
```

第一版 Spring 可為單一部署，內含接入、查詢、控制及不同 Kafka consumer group；不必先拆成多個微服務。MQTT Broker 預設 Mosquitto，Kafka 採本機單節點模式。確切版本及 Go OPC UA library 在實作時依 Java 21、server/client 支援與映像相容性驗證並固定，本文不宣稱已完成版本驗證。

### 模擬與排程

3 LP 可達任一 LL，LL 容量固定為 1 片，每個 LP 載具容納一批 25 片；兩組手臂分別服務大氣側及真空側。手臂預設一次搬一片，PM 一次加工一片。任一晶圓包含 home LP、home LL、home slot 及唯一 current location；原 LP 槽位保留；LL 單槽可重複供該批次進片與回片使用，搬送中保留目的槽位。

模擬循環以單一序列化狀態更新執行，兩手臂可以有重疊時間區間，但不能同時占用同一資源。LL 大氣側與真空側門互斥；抽氣、回填期間不可搬送。位置轉移以完成事件原子更新，在途晶圓歸屬手臂。相較獨立 goroutine 任意更新設備，此方式較易保證守恆及重現測試。

FIFO 預設以後端接受批次啟動的持久化序號定義。後續批次可裝片及抽氣，但上一批全部結案且回到原 LP 槽位前不可進 PM；每片獨立回填與卸片。回片優先於派片。模擬配方以可設定階段與合成數值描述，數值隨步驟漸變，不使用原廠真實配方。

### 故障恢復

PM 故障將批次切為異常暫停：停止新派片，正常 PM 繼續加工與回片。清除告警不改變晶圓結果、不恢復派片。操作員選擇重做時僅授權故障晶圓在原 PM 重做，不解除整批派片暫停；報廢則在搬送條件成立後回原槽。異常晶圓結案回槽且故障條件解除後才接受恢復派片。每片結案回 LL 後獨立回填、送回原 LP 槽位；25 片全數回 LP 才結案。

### 控制契約

HTTP POST /api/commands 接受 commandId、targetId、lotId、type、parameters、expiresAt、expectedStateVersion；type 包含批次啟動、暫停派片、受控停止、故障注入／解除、重做、報廢回收及恢復。模擬配方設定僅允許在尚未開始加工時變更，原先泛用轉速項目改成模擬階段設定。

PostgreSQL 交易保存命令與 outbox，發布到 cvd/{equipmentId}/commands，QoS 1、retain=false。Go simulator 以持久化 commandId 判斷重複、檢查期限與狀態版本；拒絕過期或不符合連鎖的指令。Redis 去重僅為加速，不能取代執行端判斷。回覆以同 commandId 關聯 received、executing、succeeded、rejected；逾時顯示結果未知，不自動重發新的控制指令，晚到結果需更新並保留逾時紀錄。

### 遙測與事件契約

MQTT topics 分為 cvd/{equipmentId}/telemetry、events、command-results。共用欄位為 schemaVersion、eventId、equipmentId、sourceEpoch、sequence、sourceTime、ingestedAt、quality、lotId、waferId、payload；不適用的關聯欄位可空。重新啟動以新 epoch 區隔序號，狀態快照帶版本。

Kafka topics 預設 cvd.telemetry.v1、cvd.events.v1、cvd.command-results.v1，以 equipmentId 分區以保持同設備來源順序；不同 topic 不假設全域順序，以版本及識別碼合併。各儲存 consumer 成功落盤後才提交 offset；重播依 eventId 去重，快取拒絕較舊狀態版本。異常格式送錯誤佇列並告警。

MQTT 接入至 Kafka 之間採本機持久化接收佇列，gateway 保留尚未收到接入確認的事件並可重送，避免將 broker 的 QoS 確認誤當 Kafka 落盤確認。展示預設斷線緩衝上限 10 分鐘，超限明確記錄缺口並暫停模擬推進；不可靜默丟棄履歷。Kafka 保留預設 7 天作短期重播，不替代 30 天歷史資料庫。

### 資料儲存與保留

PostgreSQL 與 TimescaleDB 使用不同容器、帳號及 volume；TimescaleDB 雖基於 PostgreSQL，本案不合併實例。PostgreSQL 儲存 lot、wafer、movement、process-run、alarm、command、operator-action 等；TimescaleDB 儲存每秒各 PM／LL 適用數值，按 sourceTime 分區、以時間與 eventId 去重。只在設備提供資料時記錄有效量測，斷線不以舊數值補成新數值。

歷史查詢使用 UTC 時戳及 [from,to) 範圍，介面轉本地時間。預設所有歷史事件及量測皆保留 30 天；設備設定與尚未結案批次的必要狀態不因清理刪除。超過 30 天的資料由查詢邊界立即排除，實體分區清理由排程執行；遲到且已超出期限的數值不重新納入查詢。刪除順序須保留活動批次參照完整性。長區間採分鐘聚合、單片詳情保留秒級查詢，聚合結果亦遵守 30 天界線。

Redis 保存最新狀態、心跳 TTL 與短期查詢快取；重建時先載入權威快照並驗證版本。Redis 不作排程鎖或機台狀態權威。兩個資料庫不使用分散式交易；歷史介面揭示資料更新時間，允許短暫最終一致。

### 儀表板與 API

主頁採 2D 拓樸：保留現代化青綠配色，中央 TM 真空側手臂、上方與左右四 PM、下方三 LL、再下方大氣側手臂與最底部三 LP；上方批次與產量指標，批次 FIFO 資訊改由點擊設備方塊查看，另提供告警及控制結果。LL 顯示 1 格槽位；點 LP 方塊可查看載具的 25 格槽位，區分未加工、在途、加工中、成功、異常與報廢。側欄顯示設備數值、品質、時間、步驟及等待原因。

查詢 API 提供 /api/equipment、/api/lots、/api/wafers/{id}/history、/api/telemetry、/api/alarms 及 /api/commands/{id}，列表分頁，遙測限制時間及點數。WebSocket 傳快照與帶版本增量；重新連線或序號缺口先重取快照，不從畫面動畫推算實際位置。按鈕依狀態顯示可用性，但所有檢查仍由執行端實施。

### Docker 與狀態恢复

Compose 服務為 frontend、backend、simulator、gateway、mqtt、kafka、postgres、timescaledb、redis。預設僅前端與 API 對本機開放，其餘使用內部網路。命名 volume 保存各資料服務、simulator 檢查點／指令帳本與 gateway 緩衝；設定由環境檔提供，不將帳密寫入程式。

健康檢查與重試處理啟動次序，schema migration 完成後才提供就緒狀態。simulator 重啟從一致檢查點恢復並暫停派片，確認快照後人工恢復，不自行重播搬送動作。這比每次重啟清空批次更能保留履歷一致性。

## Risks / Trade-offs

- [展示資源需求較高] → 提供單節點 Compose 與資源使用說明，先量測再訂部署資源門檻。
- [FIFO 批次因異常阻塞] → 依確認規則維持阻塞，顯示原因並提供重做／報廢，不自動跳批。
- [事件重複、亂序及跨庫延遲] → eventId、狀態版本、獨立 consumer offset 與可見更新時間。
- [模擬拓樸被誤認為原廠設備] → 介面及文件標註情境模擬，配方使用合成資料。
- [一秒採樣漏掉瞬間狀態] → 搬送與控制事件即時記錄，與定時採樣分流。

## Migration Plan

新建服務與資料卷，執行結構遷移、載入設備拓樸及三個展示批次，先驗證正常循環，再驗證故障及斷線。此規劃不恢復工作區既有刪除檔案。回退以停止新服務及切回先前映像為主，保留 volume；結構變更採可向前相容遷移，破壞性變更前另做備份。清空展示資料需明確操作，不隨容器重啟執行。

## Open Questions

- 示範配方各階段名稱、持續時間、合成數值範圍與畫面配色，可於實作時以可設定預設值落地，不改變本文件的設備與控制規則。

### 參考圖配置調整

搬送以權威快照 from/to 驅動，動畫沿來源、共用手臂中心、目的地呈現；不由動畫決定晶圓結果。資料過期或模擬器暫停時停止動畫。四 PM 仍均為平行 TiN 製程。

### 批次資訊按需顯示

設備總覽移除常駐 Lot 佇列、LP 批次識別及含 Lot 前綴的晶圓識別。點擊 LP、LL、PM 方塊或 LL 槽位後，以可關閉對話框顯示對應批次及晶圓資訊；履歷查詢維持可用。

### 單片 LL 排程與升級

每批仍綁定選定的 LL，但單槽逐片重用。回片優先；無空 PM 時不再裝新片，避免占住回片位置。暫停時 LL 內未加工片可退回 LP，以便健康 PM 回片。後批最多預裝一片等待。舊檢查點升級前備份，禁止搬送中升級；舊 LL 庫存移回原 LP 槽位並逐片記錄 capacity-migration，不更改加工結果；故障及暫停保留。
