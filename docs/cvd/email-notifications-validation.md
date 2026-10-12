# cvd-email-notifications 驗證紀錄

2026-09-25，部署完成，實際 Outlook 尚未連接，寄信維持停用。

## 自動與隔離驗證

- Maven：35 tests，0 failures，0 errors，0 skipped；包含 9 個 PostgreSQL/fake Graph 整合案例與 4 個 Email 基本測試，及既有加工、權限、簽核等回歸測試。
- 前端：17 tests 通過；Vite production build 成功（既有 3D chunk 大小提示仍存在）。
- PostgreSQL：獨立 `email_test` 資料庫；migration 重入、帳號預設停用、交易回滾、告警重播／亂序、共享地址去重、無收件人、案件階段重驗、Email／部門異動取消、24 小時過期、故障輪次、失聯／恢復基準、整機 generation 與 pending release 柵欄。
- 假 Graph：202、400、401、429、網路接受結果不確定、最多 5 次、Retry-After、租約失聯 unknown、明確人工重試、刷新 token 併發序列化。
- OAuth：state 與工作階段綁定、單次消耗、PKCE、AES-GCM、測試 RSA 簽署 ID token、issuer/audience/nonce 驗證與錯誤 nonce 拒絕；連接成功沒有寄出測試信。
- 以正式 PostgreSQL schema 複本啟動隔離 Spring、Redis、TimescaleDB，**未連接正式 Kafka**。API 驗證 Leader Email 寫入／版本、CSRF 拒絕、Engineer 管理拒絕、環境停用禁止 UI 開啟寄送。
- Playwright：桌面／390px 手機通知狀態、略過原因、unknown 重試確認、基底網址更新；完整 App 登入、Email 儲存、通知停用與登出。
- 測試僅使用本機 fake Graph；無真實 Outlook 授權或外部信件。臨時測試容器已移除。

## 正式部署

只重建 backend、frontend，其他資料服務及十台設備保留。部署前 PG custom-format dump 已完成，pg_restore 列表可讀，檔案位於 `.runtime/backups/email-20260925/postgres.dump`（約 333 MiB）。

回退映像：`cvd-backend-before-email-20260925`、`cvd-frontend-before-email-20260925`。

部署前後重要資料筆數相同：帳號 4、recipe_versions 10、recovery_requests 2、disposition_confirmations 1、equipment_releases 1。

正式 Playwright 驗證登入後由通知設備連結定位 `TEL-CVDTIN-08A-08`、10 台設備清單、帳號管理 Email 通知頁、登出；公開 tunnel 回應 HTTP 200，無 browser pageerror。

正式資料庫 mail_connection：`status=disconnected`、`enabled=false`、`started_at=NULL`、`tokens=NULL`；notification_deliveries=0。環境 `EMAIL_ENABLED=false`。不宣稱已實寄或送達。

操作與後續授權設定見 [email-notifications.md](email-notifications.md)。
