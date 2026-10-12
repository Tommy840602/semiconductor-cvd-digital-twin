# 登入與 RBAC 部署

角色由後端強制驗證：Operator 可查看資料與啟動／暫停／停止批次；Engineer、Leader 另可修改 Recipe、故障注入／解除、報廢、異常恢復；單片補做另需 EE 提案與 PE 核准；Leader 可建立 Operator／Engineer。

## 首次啟動

以未追蹤的秘密檔案提供初始主管密碼，設定 `CVD_LEADER_PASSWORD_FILE` 指向該檔案，檔案權限設為 0600。預設位置 `.runtime/secrets/leader-password`。帳號固定 `cvd.leader`；密碼至少 12 字元，最多 72 UTF-8 bytes。不要把密碼寫入映像、命令輸出或版本化設定。現有部署已使用使用者指定的初始密碼。

PostgreSQL 新增 app_users、auth_bootstrap、access_audit，不改寫模擬狀態。交易與資料庫鎖保證初始化唯一。重啟不讀取／重設既有密碼；未有完成標記但同名帳號存在時停止初始化，須人工核對，避免升權覆寫。

## 登入與代理

設定 `CVD_PUBLIC_ORIGIN` 為完整外網 HTTPS 網址；WebSocket 不使用任意來源。Nginx 覆寫 X-CVD-Scheme，臨時 trycloudflare.com 主機使用 Secure Cookie，本機使用 HTTP 開發 Cookie；自訂 HTTPS 網域部署時須將其加入 nginx 的 scheme map。後端 18080 僅綁定 loopback，外部不得直接繞過 Nginx。

HttpOnly、SameSite=Strict session cookie 對應 Redis 隨機 session；CSRF token 由 GET /api/auth/csrf 取得，所有 POST 帶 X-CSRF-Token，登入後 token 與 session 一起更新。閒置期限 30 分鐘，只由登入及前端使用者點擊／鍵盤活動呼叫 /api/auth/activity 更新；背景輪詢不延長期限。登出與過期會關閉 WebSocket，Redis 不可用時拒絕存取。

MQTT、OPC UA、資料庫及 Redis 不映射對外埠；前端 18000 與後端 18080 綁定 127.0.0.1，外網沿用 Cloudflare tunnel。此權限模型服務於網站帳號，內部服務流量留在 Docker 網路。

## 驗證與回退

隔離測試使用 `.runtime/rbac-compose.json`，從 `.runtime/secrets/test-leader-password` 讀取測試秘密。執行 `python3 tests/integration/rbac.py` 檢查三角色矩陣。既有 scenarios.py 改為先登入，可透過 CVD_URL、CVD_USERNAME、CVD_PASSWORD_FILE 設定環境。

正式備份位於 `backups/cvd-rbac-20260916`，包括模擬狀態、設備快照、PostgreSQL 與 TimescaleDB dump；舊後端、前端映像標籤為 `pre-rbac-20260916`。回退至無權限版本前先停止公開 tunnel；保留新增帳號表與部署後歷史，不能直接覆寫唯一資料庫。一般回退只更新前後端映像，不重啟或回寫模擬器。
