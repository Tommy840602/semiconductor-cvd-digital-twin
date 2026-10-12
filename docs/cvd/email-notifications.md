# Email 通知操作與啟用

本功能已實作告警／待辦 outbox 與個人 Outlook OAuth。部署預設 `EMAIL_ENABLED=false`，不寄外部信件。帳號 Email 預設空白且停用，username 不作收件地址。首次啟用從目前狀態建立基準，不回補舊歷史。

## 通知來源

|來源|通知|收件部門／角色|有效性|
|---|---|---|---|
|Kafka `cvd.events.v1`：`kind=alarm`，payload.pm=PM1–PM4、active=true/false|故障／解除|EE Engineer、EE Leader|同設備同 PM 狀態轉移，依 sourceTime、sequence 拒絕舊事件；一輪各一次|
|設備 snapshot 的 sourceTime、communicationQuality|失聯／恢復|EE Engineer、EE Leader|UI 仍 5 秒過期；持續失聯 60 秒、持續恢復 10 秒才通知|
|recovery_requests 進入 submitted|補做／報廢待審|PE Engineer、PE Leader|案件 revision 與 email_stage 仍相同|
|recovery_requests 進入 awaiting_result|待 PE 結果確認|PE Engineer、PE Leader|同案件階段；包括有結果確認階段的處置分支|
|既有 DispositionApi.releaseView(eq,false) 全機 eligible|整機待放行|EE Leader、PE Leader|全部異常處置符合原放行條件、holdGeneration 相同、沒有 pending release|

SQL trigger 使 history 或案件狀態與 notification_events 同一 PostgreSQL 交易提交；history event ID 唯一鍵、階段版本、delivery(event,address) 唯一鍵提供去重。Kafka consumer 保持關閉 auto commit，失敗會重播。故障歷史事件重播不會再次觸發 INSERT trigger。每秒刷新案件 updated_at 不等於階段改變，email_stage 只在狀態／revision 變動时遞增。

收件設定覆蓋十台機台。每個地址各寄一封，不暴露其他地址。多帳號共用地址合併一份工作；送出前至少一個原始帳號的角色、部門、auth_version、email_version、地址與啟用狀態仍相符才可送出。Operator、未指定部門及空地址不列入。群組通知不授予簽核權，原本自審限制照常。

## 工作與紀錄

Leader 在「帳號管理」設定個別 Email、接收通知，再於下方「Email 通知」管理連接、網站網址及查看事件／寄送嘗試。

- `queued/retry`：排隊或退避；明確限流依 Retry-After，最多 5 次。
- `accepted`：Graph 回覆 202，僅代表 Outlook 接受請求，**不代表收件匣送達**。
- `failed`：永久錯誤或重試達上限。
- `unknown`：逾時／斷線／寄送租約過期，可能已接受。不自動重寄；Leader 必須在畫面明確確認可能重複後重試。
- `cancelled/expired`：案件離開原階段、收件資格版本改變，或告警超過 24 小時。未連接期間也會檢查過期工作。
- 無收件人記錄在事件略過原因，不在之後設定地址時補寄。

租約領取使用 `FOR UPDATE SKIP LOCKED`；worker 崩潰後 sending 工作變成 unknown，而非不斷重送。通知紀錄保留 30 天，未結案 queued/retry/sending/unknown 保留。API 全部沿用工作階段及 CSRF 保護，僅 Leader 可管理。加工與簽核交易中不呼叫 Microsoft。

## 正式 Outlook 啟用（需另行授權）

以下涉及真實 Outlook 連接及外部信件。未取得使用者明確授權前，不執行連接或真實試寄。

1. 在 Microsoft 應用註冊建立允許**個人 Microsoft 帳號**的應用。以 **Web** 平台註冊精確 redirect URI：`https://<公開網站>/oauth/outlook`。使用 delegated `Mail.Send`，以及 openid/profile/email/offline_access；不需 Mail.Read。
2. 建立 client secret，將值存到儲存庫外、限擁有者可讀的檔案。另以 `openssl rand -base64 32` 產生 token 加密金鑰檔，權限 600。妥善保存；遺失金鑰需重新授權。不要提交或輸出 secret/token，不在網頁輸入 Outlook 密碼。
3. `.env` 設定 `OUTLOOK_CLIENT_ID`、`OAUTH_REDIRECT_URI`、`PUBLIC_BASE_URL`，以及外部檔案路徑 `OUTLOOK_CLIENT_SECRET_PATH`、`EMAIL_TOKEN_KEY_PATH`。加入 overlay：`COMPOSE_FILE=compose.yaml:compose.secs.yaml:compose.email.yaml`。初次仍維持 `EMAIL_ENABLED=false`。
4. 重建 backend 使 Docker secrets 掛載，Leader 點「連接／重新授權 Outlook」自行登入 Microsoft。授權綁定原 Leader 工作階段，10 分鐘有效；用 state、PKCE、已驗證簽章的 ID token、issuer/audience/nonce 驗證。返回頁立即移除 URL 參數，再以 same-origin CSRF POST 完成；原有 SameSite=Strict 登入 Cookie 不變。若登入過期，重新登入後重新發起連接。
5. 授權完成**不自動寄測試信**。先確認通知網站網址、指定測試帳號的 Email／角色／部門及啟用設定。經明確授權後設定 `EMAIL_ENABLED=true` 並重建 backend，再由 Leader 啟用通知。
6. 以授權的測試事件對指定收件人驗證，先停用其他測試不涵蓋的收件設定。確認寄送紀錄 accepted，並由信箱持有人確認實際收到。不可只以 accepted 宣稱送達。無額外任意地址試寄 API，避免繞過固定收件資格。
7. 正式連接及真實試寄是尚待執行的上線條件。若應用註冊政策不允許個人帳號，須先解決 Microsoft 應用設定；不改用儲存密碼的 SMTP。

`EMAIL_OFFLINE_SECONDS`（預設60）及 `EMAIL_RECOVERY_SECONDS`（預設10）可調整。通知設定的網站基底網址僅接受 HTTPS origin；localhost/127.0.0.1 開發測試可 HTTP。OAuth callback 固定 `/oauth/outlook`，不從 Host header 推導。若使用臨時 tunnel，網址變更要更新公開網址、Microsoft 註冊 callback 與 OAUTH_REDIRECT_URI；舊信件不能自動更換連結。

OAuth callback nginx 關閉 access log，回應 no-store/no-referrer。授權及 Graph 錯誤只保存通用分類／HTTP 狀態，不存原始錯誤與 token。AES-256-GCM 加密 token，刷新由 mail_connection row lock 序列化並原子替換。

## 回退

1. 先在管理頁暫停通知或設定 `EMAIL_ENABLED=false` 並重建 backend。
2. 保留通知表與稽核資料，不 drop 欄位／表、不清除資料 volumes。
3. 將本次部署前標記的 backend/frontend 備份映像恢復到 compose 使用的映像，僅重建這兩個 service。新增欄位預設值可與舊版本共存。
4. 回退前若曾啟用通知，佇列及 unknown 紀錄仍保留；再次升級重新驗證有效性，不能盲目重寄。

單機所有 Docker／主機停機時，這套通知也停止。獨立外部可用性監測不在本功能範圍。

## 隔離驗證

`EmailNotificationsTest` 驗證權限、CSRF、固定矩陣、格式與連結；`EmailDatabaseTest` 僅接受 `jdbc:postgresql://localhost:15439/email_test`，會清空該**專用測試資料庫**。測試郵件使用本機 fake Graph，不連接 Microsoft。

```sh
docker run -d --name cvd-email-test-pg -e POSTGRES_PASSWORD=email-test -e POSTGRES_USER=cvd -e POSTGRES_DB=email_test -p 127.0.0.1:15439:5432 postgres:16-alpine
CVD_EMAIL_TEST_DB=jdbc:postgresql://localhost:15439/email_test mvn -f backend/pom.xml test
npm --prefix frontend test
npm --prefix frontend run build
```

官方依據：[sendMail](https://learn.microsoft.com/en-us/graph/api/user-sendmail?view=graph-rest-1.0)、[authorization code + PKCE](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow)。

UI fixture 位於 `frontend/test-fixtures/email-notifications.html`，Vite 建置不打包此檔；以 dev server 18003 搭配 `PLAYWRIGHT_MODULE` 指向已安裝 Playwright 執行 `node scripts/verify-email-ui.cjs`。
