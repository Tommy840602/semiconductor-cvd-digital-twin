## Context

Pipeline.history 已接收 Kafka 事件並以 eventId 去重寫入 PostgreSQL；Api.alarms 查詢 alarm、data-gap 與 dead_letter。帳號目前有角色與部門但無 Email。簽核通知需銜接實際提交成功的狀態轉移，不能將每個 command-result 都視為待辦。

## Goals / Non-Goals

**Goals:** 非同步、持久、可追蹤地寄送通知；寄信服務失效不能阻塞加工與簽核。

**Non-Goals:** 不提供 Email 直接核准、收件匣讀取、每人機台分工、值班排程或外部主機可用性監測；不回補首次啟用前全部歷史信件。

## Decisions

### 1. 個人 Outlook 與 OAuth

採 Microsoft Graph v1.0 的 /me/sendMail，使用委派 Mail.Send，搭配 authorization code 與 PKCE、state、OIDC 身分驗證及 offline_access。Microsoft 應用必須允許個人 Microsoft 帳號，redirect URI 必須與註冊相符。設定單一寄件帳號；授權流程綁定當次 Leader 工作階段，callback 再驗證權限。token 僅在後端使用，加密保存於 PostgreSQL，金鑰與應用憑證從 Docker secrets 讀取，不進前端、Git 或日誌；更新 token 原子替換並避免並行更新衝突。OAuth 連接不會自動寄測試信。

相比保存密碼的 SMTP，本方案不接觸 Outlook 密碼。Mail.Send 不包含讀信，故紀錄只顯示服務接受，不推斷收件匣送達。參考：https://learn.microsoft.com/en-us/graph/api/user-sendmail?view=graph-rest-1.0 （2026-09-25 查閱；個人帳號支援委派 Mail.Send，202 表示接受而非已送達）。

### 2. 資料模型與權限

增加帳號 email、email_enabled、email_version；既有帳號初始為空且停用。Leader 管理設定、OAuth 連接與通知紀錄，所有寫入沿用 CSRF、稽核及後端角色檢查。通知路由是固定矩陣，不新增可任意擴大收件群組的規則編輯器。既有自審限制仍由案件 API 決定，群組通知不授予核准權限。

增加 notification_events、notification_deliveries、notification_attempts、notification_incidents、mail_connection。工作唯一鍵包含事件／案件階段版本及正規化收件地址；記錄收件帳號及版本以便發送前重新檢查。若多個合格帳號共用地址，任一仍符合原資格才可送出同地址工作，不重送已接受工作。

### 3. 事件與 outbox

在同一 PostgreSQL transaction 寫入事件／案件狀態與通知來源工作。Kafka 位移於成功提交後才確認；避免只靠既有 history 的 ON CONFLICT 而漏掉中途失敗的通知。獨立背景工作解析收件人並以資料庫租約領取寄送工作；不在設備事件消費交易中呼叫 Microsoft。測試重播、事件亂序、程序在提交前後終止、租約回收。

PM 故障按設備、PM 與故障輪次識別；解除使用來源狀態轉移。資料失聯使用後端持續檢查品質與來源時間，預設持續 60 秒才通知、有效資料持續 10 秒後恢復，兩者為可設定操作參數；不同於 UI 5 秒過期提示。第一次啟用以目前狀態建立基準，不寄舊歷史；啟用後的中斷會持久追蹤。已通知的失聯輪次只寄一次恢復。故障與失聯是獨立事件，不互相覆蓋。

簽核使用案件版本與實際階段：提案待 PE、補做完成待 PE 結果確認、整機待 Leader 放行。必須盤點現有狀態映射並測試報廢分支；通知只是待辦提示，不改報廢／留片／放行規則。發送前檢查該階段是否仍有效，否則取消。

### 4. 重試與追蹤

每封郵件單獨收件以避免暴露群組地址。服務回應 202 記為 accepted；不顯示 delivered。預設最多 5 次，對明確可重試錯誤採退避並尊重 Retry-After；401 先受控更新 token，再失敗則標記需重新授權；永久錯誤停止；已傳輸後逾時或 worker 失聯留下的 sending 工作標記 unknown，由 Leader 在清楚提示可能重複後選擇重試。外部 sendMail 沒有本方案可依賴的恰好一次保證。

授權未就緒時不丟失工作，但排隊待辦須重驗有效性。超過 24 小時的告警待寄工作標記 expired，避免連接恢復後大量寄送陳舊告警；保留紀錄。通知紀錄預設保留 30 天，未結案工作不被清除。

### 5. 內容與連結

使用伺服端固定模板、純文字或經跳脫的 HTML，不嵌入 token、配方或完整遙測。所有時間包含台灣時區標記。設定獨立 PUBLIC_BASE_URL 與 OAUTH_REDIRECT_URI，不從未信任的 Host header 組合；只接受核准的 HTTPS 網址（本機開發 callback 另按 Microsoft 規則設定）。登入後定位機台及案件，返回路徑只接受站內路徑。臨時網址改變需同步管理與應用 callback 設定；已寄舊連結不能改寫。

## Risks / Trade-offs

- 個人帳號授權可能撤銷或寄信限流 → 可見連接狀態、有限重試、暫停與重新授權。
- Outlook 已接受但本機未寫入結果 → unknown 狀態及明確人工重試，不宣稱零重複。
- 全廠停機時本機寄信也停止 → 明確排除外部監測，恢復後處理仍有效工作。
- 錯誤地址或權限異動 → Leader 設定稽核、格式驗證與送出前重驗；格式驗證不代表擁有該信箱。

## Migration Plan

先新增相容資料表及可選欄位，預設 EMAIL_ENABLED=false，使用 fake Graph 與隔離資料完成驗證，再部署。寄件帳號、Microsoft 應用 ID／secret、加密金鑰、callback 與收件地址為上線設定，不在規劃文件內填入。外部連接及指定收件人的實際試寄需使用者明確授權；未完成時可交付停用狀態的實作，不宣稱實寄驗證成功。回退先停用 worker，再還原應用映像，保留通知表與紀錄供後續處理。
