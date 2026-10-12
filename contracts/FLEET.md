# 多機台契約

機台識別僅接受 TEL-CVDTIN-08A-01 至 TEL-CVDTIN-08A-10；equipmentId 是完整機台，moduleId 是 PM1／LL1 等局部名稱。

單台資源以 `/api/equipment/{equipmentId}` 為前綴；總覽為 `GET /api/equipment`。未知／缺少機台拒絕，局部目標不可跨台使用。WebSocket 以 equipmentId 查詢參數訂閱單台。全廠摘要由總覽 API 取得。

MQTT 路徑為 `cvd/{equipmentId}/{snapshot|telemetry|events|commands|command-results|acks}`。payload 與 topic 機台必須一致；舊 CVD-01 僅在遷移讀取時對應 01。Kafka 使用完整機台為 key。事件及指令 ID 全域唯一，重送維持原 ID。ACK 只送至來源機台。

Recipe 複製明確指定來源機台／版本、目的台預期版本；複製建立新版本，不連動。補做簽章包含機台，不能換台執行。

未執行的舊補做指令只在 01 路由檢查舊 HMAC 成功後，將 equipmentId 綁入傳輸處方再簽章；資料庫原處方及 commandId 不變。新處方設備端必須比對簽章內機台與自身識別。這個遷移轉換不接受其他機台，也不提供一般控制 API 入口。
