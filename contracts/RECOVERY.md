# 單片補做契約（checkpoint modelVersion 5）

角色與部門獨立。`/api/auth/me` 回傳 `id,username,role,department,authVersion`。既有帳號 department=null；EE Engineer／Leader 可提出自己的處置，PE Engineer／Leader 可審核別人的處置。Leader 不跨部門取得補做權限。既有全機 Recipe 權限維持 Engineer／Leader。

`POST /api/users` 增加可選 `department: "EE" | "PE" | null`。Leader 以 `GET /api/users` 列出帳號、`POST /api/users/{uuid}/department` 傳 `{ "department": "EE" }` 指派部門；不改角色，增加 authVersion 使舊 session／WebSocket 失效。

## 處置 API

所有 POST 需要登入 Cookie 與 X-CSRF-Token。401 身分失效、403 權限、自審／他人修改、400 無效欄位、409 基線／版本／狀態衝突、503 資料服務不可用。

| 方法與路徑 | 行為 |
|---|---|
| GET `/api/recoveries?waferId=...` | 最近 200 張，可依晶圓篩選 |
| GET `/api/recoveries/{id}` | 所有版本、簽核事件、執行意圖與設備結果 |
| POST `/api/recoveries` | EE 建立單片草稿；設備產生基線，不採用客戶端基線 |
| POST `/{id}/save` | 草稿編輯也追加新版本，舊版不覆写 |
| POST `/{id}/submit` | 凍結送審版本 |
| POST `/{id}/return` | PE 退回，必須 reason |
| POST `/{id}/approve` | PE 核准，交易同時新增唯一 executionId |
| POST `/{id}/withdraw` | 原 EE 撤回；已發布要等待設備確認取消 |
| POST `/{id}/revise` | 退回／撤回後建立新版本，重新送審 |

上述 `/{id}` 動作均以 `/api/recoveries` 為前綴，body 必須帶目前 `revision`。`save/revise` 另傳完整處方欄位：

```json
{"revision":1,"pmId":"PM1","stepSeconds":[0,0,3,2],"targetThicknessNm":10,"troubleshooting":"已排除異常原因，補做沉積與吹掃"}
```

步驟依 `[preheat,stabilize,deposition,purge]` 排列，0 表示不選，至少一個為 1–3600 整數，總和 1–3600 秒。目標正數、上限 100000 nm；排錯說明 3–4000 字元。目標不改寫原始沉積速率，也不截斷核准秒數；低於現況不會去膜。每片只能有一張活動單。

狀態：draft → submitted → approved_waiting → executing → completed／failed；另有 returned、withdrawn、invalidated。已發布撤回時為 cancelling，取得設備未開始確認才變 withdrawn，否則顯示已開始的實際結果。取消中的版本不能修訂。

## 內部執行

一般 `/api/commands` 永遠拒絕 `wafer-supplement`、`cancel-supplement`、`rework`。一般工程權限新增 `recovery-clock`，只恢復時鐘，保留 dispatchPaused 與原批次暫停。核准不清除 PM 故障／全域暫停。

後端發布器從 PostgreSQL 唯一意圖產生 `executionId-execute` 或 `executionId-cancel` 指令，使用相同 executionId 重送。parameters 的 `payload` 是原始 JSON 字串，`signature` 為 HMAC-SHA256 十六進位，簽章資料為 `type + "\n" + payload` 的 UTF-8 bytes。兩服務從 Docker `recovery_key` secret 讀取同一金鑰；空白前後去除。設備拒絕無簽章／竄改的核准內容。

payload 綁定 requestId、revision、executionId、waferId、pmId、sourceRunId、exceptionVersion、baseline（五點）、rootRate、stepSeconds、targetThicknessNm。設備再次確認位置、exception、來源加工、五點基線、原始速率、故障與時鐘。基線失效不能轉用另一片；臨時互鎖保留等待。取消 tombstone 同樣寫入 checkpoint，晚到指令不會重新開始。

快照 `recoveries[executionId]` 保存狀態、原因、startedAt／finishedAt 與終態 result。重啟不清除，已開始進度連同膜厚一次持久化。重送已處理 commandId 只重新發出既有 command-result，不再次套用控制。每次實際補做最多增加一次 attempts。

## 製程與歷史

processTelemetry／process-result 新增 `sourceProcessRunId,recoveryRequestId,recoveryRevision,executionId,rootDepositionRateNmPerSecond,baselineThicknessNm,deltaThicknessNm`。保留既有五點 `sampleThicknessNm`。只有所選沉積追加膜厚，其他步驟為零增量。original target/original deposition seconds 的 rootRate 在多輪補做保持不變。

GET `/api/telemetry?metric=baseline|delta|thickness&pmId=PM1&waferId=...` 查詢每秒膜厚；仍保留 30 天。recovery-started／recovery-completed／recovery-failed 事件串連來源與版本。1 秒完成的處方仍保留 startedAt，即使後端首次看到的快照已完成。

PostgreSQL 的 recovery_requests、recovery_versions、recovery_events、recovery_intents 保存完整鏈。活動處置晶圓的原始 history 不清除；終態鏈至少保留 30 天，且同片仍有活動／近期處置或近期 history 時保留整鏈。過期時先刪除關聯事件／意圖／版本，最後刪主單；歷史身分是當時快照。
