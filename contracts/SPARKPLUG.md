# CVD Sparkplug 與控制契約 v1

Group=cvd-fab、Host=cvd-data-platform、Edge=edge-{equipmentId}、Device={equipmentId}。僅接受十台完整編碼；Node 和 Device 必須一致。使用 Sparkplug 3.0 lifecycle、spBv1.0 topic 和 Sparkplug B Protobuf。固定拓樸避免從任意 topic 建立可信設備。

資料包括 PM／LL／LP 指標的 name、datatype、timestamp、單位；Birth 完整定義 alias。DBIRTH 前、seq 缺口或世代失配不可把 alias 猜成有效即時資料；先 rebirth。NDEATH／DDEATH 令所屬機台品質失效，historical 資料只回填歷史，不覆蓋新版快照。Birth、資料與死亡 QoS／retain／序號遵循 Sparkplug 規範，應用交付保證另行處理。

Kafka 延用 cvd.snapshot.v1、cvd.telemetry.v1、cvd.events.v1、cvd.command-results.v1，控制新增 cvd.commands.v1，key=equipmentId。正規化資料保留原事件 envelope，加入 protocol、ingestTime、來源世代與 quality；原 eventId、epoch、sequence、sourceTime 不改寫。schemaVersion 是業務版本，不以 transport seq 去重。

平台命令 envelope：schemaVersion=cvd-secs-sparkplug-v1、payload=原命令 JSON UTF-8 bytes 的 Base64、signature=hex(HMAC-SHA256)。簽章輸入為版本加換行，再接 Base64 解碼後原始 JSON 的確切 UTF-8 bytes；不得對已簽章內容重新排序。平台與 Gateway／設備使用獨立 control key，與 PE recovery key 分離。拒絕未簽章、未知版本、過期或錯機台；Kafka／MQTT 重送始終沿用原 commandId／executionId。

命令結果含 accepted／rejected 與後續業務結果。設備 durable commit 前不得回成功；重要事件等 Kafka durable ACK 後才送回應用確認並清 spool。Gateway ACK 只可確認該機台連續已交付 sequence，不能以較新序號跳過未交付事件。斷線、容量不足或未知結果應可見，不能靜默改成成功。

### 補送與容量

Gateway 每秒最多補送 10 筆重要事件，同一 eventId 至少間隔 15 秒才重試；正常每秒新採樣不受影響。Bridge 在 Kafka 尚未確認前保存該 eventId 的首份持久副本，收到重送不反覆寫盤；Kafka ACK 後才通知 Gateway，再由 Gateway 以連續 sequence 向設備 ACK。

Gateway `/metrics` 顯示 pendingRecords，超過 5,000 筆輸出告警且健康檢查失敗；Bridge `/metrics` 提供 pendingRecords、lastKafkaAck、deliveryFailures，超過 10,000 筆或 15 秒沒有 Kafka ACK 會顯示不健康。以上閾值不刪除重要事件。设备既有 10 分鐘緩衝保護會暫停並保存 data-gap 告警，停止增加普通採樣，以免無限填滿磁碟。

一般人工暫停仍每秒記錄採樣，製程秒數、搬送與膜厚不推進。只有超過上述緩衝上限才以可追蹤的資料缺口取代後續採樣。

### 異常處置指標與事件

DBIRTH/DDATA 新增 `Equipment/dispositionHold` (Boolean)、`Equipment/holdGeneration` (Double，與既有 JSON 整數指標相同)、`Equipment/dispatchPaused` (Boolean)。`CVD/State` Bytes 保留完整 incidents/recoveries/releases/authorizations。`CVD/Event` Bytes 中 disposition-* 與 equipment-released 經 bridge 的持久事件路由發送 `cvd.events.v1`，key=equipmentId；簽核指令經 DCMD 的既有簽章控制通道，不能用 metric 數值變更直接放行。
