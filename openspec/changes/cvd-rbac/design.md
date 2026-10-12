## Context

參見 proposal.md 的動機。Spring 目前未使用安全驗證套件，所有控制由 /api/commands 接收並持久化後送出；即時畫面使用 /ws。已有 PostgreSQL、Redis 與 HTTPS 臨時網址，適合沿用既有服務。此變更跨前後端、資料模型與部署，因此需要本設計。

## Goals / Non-Goals

**Goals:** 在指令持久化與發布前完成授權；登入身分可追溯；不干擾既有加工及歷史資料。

**Non-Goals:** 自助註冊、SSO、新增 Leader、角色修改、刪除帳號及密碼重設流程。Recipe 權限涵蓋既有 configure 能力，不在此次擴充配方編輯欄位。

## Decisions

- 採 Spring Security 與伺服器 session，Redis 保存 session；相較自訂 JWT 可直接撤銷登出狀態，沿用已有 Redis。建議閒置期限 30 分鐘；背景資料輪詢不延长人工使用期限，具體活動更新方式在實作中統一。Redis 不可用時拒絕受保護請求，不降級匿名。
- 使用同源 HttpOnly、SameSite Cookie，外網 HTTPS 使用 Secure；本機 HTTP 使用明確開發設定。所有狀態修改包含登入／登出採 CSRF 防護。僅登入、CSRF 取得與必要的最小健康檢查公開。代理只信任受控來源設定，不任意接受客戶端偽造的轉送標頭。
- /api/auth/login、/me、/logout 作為登入介面；/api/users 由 Leader 建立帳號。帳號唯一且角色採明確允許值；密碼存 BCrypt 雜湊，不回傳或記錄明文。建議密碼至少 12 字元，指定初始密碼符合此規則。錯誤帳密回應一致，登入失敗採速率限制。
- PostgreSQL 使用者表保存 ID、username、password_hash、role、created_at、created_by。首次部署透過 Docker secret 或未追蹤的環境設定提供指定初始帳密：帳號 cvd.leader，密碼沿用使用者本次指定值，不將明文寫入版本化文件或映像。初始化以持久化標記與交易避免重啟或多實例重複建立，既有同名帳號衝突時明確失敗，不覆寫或自動升權。
- 角色採明確動作白名單：三角色均可讀取及 start/pause/stop；Engineer、Leader 另可 configure/fault/clear/rework/scrap/resume。未知命令一律拒絕。Leader 只能建立 Operator、Engineer。後端在 commands 寫入前檢查，不只隱藏按鈕；不信任請求提供的 actor 或 role。
- /ws 握手檢查登入與來源，session 登出／過期後停止推播並關閉連線。前端遇到 401 清除設備顯示及返回登入，403 顯示權限不足。
- 新命令由伺服器附加使用者 ID、username、role；稽核保存命令 ID、操作者、動作、目標、時間及接受／拒絕結果，執行結果透過 commandId 關聯。帳號建立另記錄不含密碼的稽核。既有歷史維持原樣並標示未記錄操作者，不捏造身分。相同 commandId 重送不改寫原操作者，也不略過授權。
- 網站顯示登入帳號與角色；Operator 隱藏 Recipe 及異常處置，Leader 顯示帳號建立頁。登出不停止已開始的模擬製程。

## Risks / Trade-offs

- [公開展示轉為需登入] → 同時更新前端、API、WebSocket 與測試登入方式，確認外網 Cookie／代理設定。
- [內部訊息入口繞過網站授權] → MQTT、OPC UA 維持內部 Docker 網路；檢查宿主機對外埠，只公開網站所需入口。RBAC 保護網站使用者，不將內部服務視為一般使用者入口。
- [安全設定錯誤造成無法登入] → 隔離驗證初始化、重新啟動、失敗登入、權限拒絕及 session 失效，再部署。
- [已接受的指令與登出競爭] → 授權以接受時為準，登出後不接受新指令，已接受命令仍按原狀態機完成。

## Migration Plan

1. 備份正式 PostgreSQL、模擬 checkpoint 與目前映像；隔離環境驗證新增資料表與角色矩陣。
2. 透過部署秘密設定注入指定初始 Leader 帳密，新增表與首次初始化不得改寫既有資料。
3. 更新後端及前端，重新建立登入 session；驗證本機及外網登入、權限、帳號建立、WebSocket、既有製程連續性。
4. 回退保留新資料表與帳號資料；舊無驗證版本若需啟用，先停用公開入口，避免將權限功能移除後直接暴露外網。不得用旧備份覆寫部署後產生的歷史。
