## Context

參見 proposal.md。現有十台具備獨立 state／spool 卷、equipmentId 路由、Kafka 分區鍵及資料歸屬；Java Pipeline 同時負責 MQTT 接收、Kafka 發布與消費。歷史、帳號與補做處方已有正式資料。本變更跨設備、閘道、橋接、控制與部署，因此需要此設計。

## Goals / Non-Goals

**Goals:** 以真實 HSMS 封包與 Sparkplug B 編碼驗證十台完整資料／控制鏈路，既有加工引擎不因通訊替換改變。端到端保留來源時間、機台、命令、晶圓、配方版本與補做意圖。

**Non-Goals:** 不宣稱 TEL 原廠相容或完整 SEMI 認證；不導入 MES、GEM300 全套 carrier／job 管理或 EDA；不連接真實設備。Sparkplug 與 Kafka 是本平台選型，不描述為 SEMI 強制架構。

## Decisions

### 1. 設備與 Host 邊界

十組 Go simulator／gateway 延用獨立容器及卷。Simulator 作為 HSMS passive equipment，Gateway 作為 active GEM Host；機台身分由受控端點映射加設備查詢雙重驗證，不直接把 HSMS session ID 當全域 equipmentId。定義連線、Select、通信建立、Online Local／Remote 為不同狀態；只有允許的 Remote 狀態可下控制。設備安全聯鎖與加工決策仍在 simulator。

實作前凍結版本化「模擬 GEM profile」：HSMS Select／Deselect／Linktest／Separate、交易關聯、T3／T5／T6／T7／T8；SECS-II 型別及長度限制；通信建立、狀態變數查詢、trace、事件報告定義／連結／啟用、alarm、remote command、process program 上下載。候選訊息族包括 S1、S2F23、S2F33–38、S2F41/42、S5、S6F1/2、S6F11/12、S7，具體格式與支援清單須在實作契約驗證階段依可用標準資料確認。未支援訊息明確拒絕，不以成功回覆掩蓋未實作功能。

SVID／DVID／CEID／ALID／RCMD／PPID、資料型別、單位、可寫條件與錯誤回覆全部納入資料字典。補做處方、控制識別碼與事件重送為明確命名的模擬擴充，不假稱標準 GEM 已定義。替代方案是保留 OPC adapter 並行；本次採完整取代，避免正式雙重派令。

### 2. Sparkplug 拓樸與字典

以 Sparkplug 3.0 規範及 spBv1.0 namespace／Sparkplug B Protobuf 為基準；沿用 MQTT Broker，合規責任主要位於端點與 Host。

每台 Gateway 為一個 Edge Node，例如 `edge-TEL-CVDTIN-08A-01`，對應一個 Device `TEL-CVDTIN-08A-01`，群組使用固定 `cvd-fab`。PM／LL／LP 為裝置內具型別與單位的 metric 路徑，不拆成可跨台搬送的設備。topic 形如 `spBv1.0/cvd-fab/DDATA/edge-TEL-CVDTIN-08A-01/TEL-CVDTIN-08A-01`。

實作 NBIRTH／NDEATH、DBIRTH／DDEATH、NDATA／DDATA、NCMD／DCMD、Host STATE；遵循序號、bdSeq、metric alias、LWT、rebirth 規則。Birth 建立字典，資料不可只把原 JSON 放入單一字串冒充指標化。複雜事件及簽核 envelope 使用有版本且明確標示的 Bytes metric。離線重播依規範標記 historical，保留原 sourceTime；歷史資料不可覆蓋最新有效快照。

### 3. 獨立 Bridge 與 Data Platform

新增獨立 Go Bridge 服務作為 Sparkplug Host Application，具固定 Host ID、持久卷與 MQTT session。負責 Birth 字典、別名展開、節點世代、品質狀態、解碼與 Kafka 正規化；Java 不再直接消費舊 MQTT 資料 topic。相比無狀態原文轉送，這能避免遺失 Birth 後錯解 alias 或把死亡節點顯示為正常。

Bridge 輸出仍以完整 equipmentId 為 Kafka key。內部 envelope 保存 eventId、schemaVersion、equipmentId、moduleId、sourceTime、ingestTime、quality、epoch／來源版本、commandId／executionId（適用時）。同機序列在來源與平台驗證；Sparkplug 8-bit seq 不作業務唯一 ID。

即時遙測維持每台每秒採樣，TimescaleDB 保存 30 天；PostgreSQL 保存事件／簽核／配方，Redis 保存最新狀態。Kafka replay、Bridge 重啟與別名字典失效須先取得有效 Birth；在此之前隔離資料並顯示未知品質，不猜測別名。

### 4. 控制回程與可靠性

Vue → Java 授權／簽核／交易 outbox → Kafka command topic → Bridge → Sparkplug DCMD → Gateway → SECS/GEM equipment。設備 ACK 表示接受或拒絕，完成結果由設備事件回報，再循資料鏈路返回平台；UI 區分待送、已接受、執行中、完成、失敗與結果待確認。

命令包含 equipmentId、不可變 commandId、期限、目標、期望版本及授權上下文。EE 提案、PE 核准所建立的 executionId 與簽章處方一路不變。MQTT 不是另一個繞過簽核的控制入口；Broker 使用獨立服務身分與 topic ACL，Gateway 驗證可信控制 envelope，補做仍驗證原簽章。

設備持久去重是執行不重複的最終防線。Bridge／Gateway 採持久佇列、有限重試與容量告警；Sparkplug／MQTT QoS 不單獨保證 exactly-once。重要事件以穩定 eventId 與應用層交付確認擴充，Kafka durable ACK 後才可清除對應待送資料。普通遙測與重要事件的重送／丟棄策略分別定義，不能靜默丟失一秒採樣。逾時且結果未知時查詢原 commandId，不產生新身分重做。取消與核准競態維持既有簽核規則。

### 5. 保持業務與資料相容

保持十台編碼、每台 3 LP／3 單片 LL／4 PM、ATM／VAC、三批輪流派片及晶圓不跨台。保留每台不可變 recipe 版本與複製來源；進行中晶圓使用啟動時版本。PPID 對應不可變 recipe 版本，下載成功不代表正在加工的晶圓已改配方。

既有歷史原文不改寫；以既有歸屬欄與新增協定 metadata 兼容查詢。舊簽章不可原地改寫。Web API 盡量相容，只增加通訊／品質狀態與協定標示，保留現代配色與設備布局。

## Risks / Trade-offs

- [無 TEL 字典或完整授權標準文本] → 以明示模擬 profile 驗收，依賴選型先驗證實際封包、授權與維護性；不得以自己兩端互通作為標準認證。
- [Birth／alias 世代錯配] → 世代隔離、rebirth、未知品質與重連測試。
- [重送造成補做重複或 recipe 跨台] → 持久 command／execution 去重、版本與機台驗證、故障注入。
- [中斷控制連線被誤當製程停止] → 通訊狀態與製程狀態分開，故障策略明確；恢復連線不自動派新批次。
- [磁碟不足] → 延用既有 30 天策略，重新實測十台流量與緩衝增長；容量不足需明確報告，不能降低採樣或刪活動簽核鏈以通過驗收。

## Migration Plan

1. 在隔離 Compose 驗證新鏈路，保留正式舊鏈路正常運作；不得建立正式測試 lot。
2. 維護切換前停新命令、完成或逐一記錄在途結果，保留 unknown 狀態；備份兩個 DB、全部 simulator／gateway 卷、Bridge／broker／Kafka 狀態、映像及配置，記錄帳號雜湊、晶圓／膜厚／配方／簽核及佇列對帳。
3. 舊 outbox／spool 不直接丟棄：先排空已可確認項目，未確認項目以保留原 ID 的轉接規則交付；無法判定的命令不自動換 ID 重送。新舊 publisher 不同時對同台發出控制。
4. 啟動新 Bridge、十組 HSMS／Sparkplug 端點及平台，完成 Birth 與對帳；全部設備保持暫停，不清空目前已有業務資料。
5. 驗證十台健康、歷史／帳號／簽核不變、每秒採樣及網站；成功後停止正式 OPC UA 與舊 JSON 主鏈路。
6. 未開放新操作即失敗時，停止新版並還原成套 DB、卷、broker／Kafka 狀態及舊映像。若已接受新操作，先匯出與對帳，禁止回滾直接丟失新資料。隔離環境先實際演練還原。

## Open Questions

Go HSMS／SECS 與 Sparkplug 套件的版本及授權，在實作第一階段以封包、斷線與編解碼測試選定；不改變本設計的角色分工或可觀察契約。

參考：SEMI 通訊標準導讀 https://www.semi.org/en/standards-watch-2022-Sept/intro-to-semi-communication-standards ；Eclipse Sparkplug 規範 https://sparkplug.eclipse.org/specification/ 。
