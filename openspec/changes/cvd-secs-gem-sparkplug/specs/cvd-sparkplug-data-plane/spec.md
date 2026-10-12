## Purpose

定義 Sparkplug B 設備資料如何經 MQTT、獨立橋接服務與 Kafka 進入資料平台，涵蓋可辨識的機台與指標、連線生命週期、重播品質及來源時間，使網站和歷史查詢不誤用失效資料。

## ADDED Requirements

### Requirement: Sparkplug 訊息與拓樸

系統 SHALL 使用 Sparkplug B Protobuf、spBv1.0 namespace、十個獨立 Edge Node 及各自對應的 CVD Device；Birth SHALL 公布可解析的指標型別與身分，不能只以自訂 JSON 替代指標。

#### Scenario: 正常上線
- **WHEN** 任一 Edge 與 Device 上線
- **THEN** Host 取得有效 Birth 字典後可解析該機台 PM／LL／LP 指標及 sourceTime。

### Requirement: 生命週期與失效品質

系統 SHALL 遵循 Birth／Death、Host STATE、序號、世代與 rebirth 行為；未知字典或死亡來源 MUST 不得顯示為有效即時值。

#### Scenario: Bridge 重啟缺字典
- **WHEN** 收到尚無有效 Birth 的 alias 資料
- **THEN** Bridge 要求重建且隔離未解析資料，網站顯示未知或失聯，不能猜測 alias。

#### Scenario: Gateway 突然死亡
- **WHEN** Gateway 非正常斷線並觸發死亡通知
- **THEN** 相應機台值標示失效，其餘機台品質保持獨立。

### Requirement: 順序去重與歷史重播

資料 SHALL 保留穩定 eventId、equipmentId、來源時間、品質和世代；Kafka SHALL 依機台作分區鍵，歷史重播不得使最新狀態倒退。

#### Scenario: 重複與舊資料
- **WHEN** 相同事件重送或舊世代歷史資料晚到
- **THEN** 事件不重複入帳，合法歷史仍可查詢，但最新設備快照不被舊值覆蓋。

### Requirement: 持久交付與取樣保留

系統 SHALL 維持十台各每秒一次遙測及最近 30 天保留，重要事件與未確認命令 SHALL 持久保存；佇列滿或資料缺口 MUST 可觀察，不得靜默遺失。

#### Scenario: Broker 或 Kafka 中斷
- **WHEN** 資料生成期間下游暫時不可用
- **THEN** 待送資料保持原身分與來源時間，恢復後可對帳交付；積壓與容量不足有告警。

#### Scenario: 過期清理
- **WHEN** 執行 30 天保留清理
- **THEN** 過期遙測依策略移除，活動晶圓、未交付控制及必要簽核資料鏈不被破壞。
