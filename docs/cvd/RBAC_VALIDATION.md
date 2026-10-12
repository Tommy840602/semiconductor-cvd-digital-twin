# RBAC 驗證紀錄

2026-09-16；變更 `cvd-rbac`。本次新增登入與角色控管，保留現有製程、三批輪流派片與歷史模型。

## 已通過

- Java 9 個測試，包括首次初始化衝突、不讀取秘密覆寫既有帳號、密碼長度與雜湊截斷邊界；Vue 正式建置通過。
- 隔離 Docker 三角色 HTTP 測試：匿名 401、登入 CSRF、錯誤帳密、失敗速率限制、Operator 所有越權命令被拒絕且命令表不增加；Engineer／Leader 及 Operator 日常命令依矩陣接受。
- 帳號建立：只有 Leader 可建立 Operator／Engineer；同名 409、非法／Leader 角色拒絕，API 與稽核無密碼。資料庫只保存雜湊。
- 稽核：記錄真實登入操作者；偽造 actor 無效；commandId 重送不覆寫原操作者，舊歷史可讀。
- WebSocket：匿名及跨來源拒絕，合法登入可收資料；登出／Redis session 到期後關閉連線。背景讀取不延長期限。
- 瀏覽器：登入前無設備資料；Operator 隱藏配方、異常與帳號管理；Engineer 顯示配方及異常處置；隔離 Leader 測試 fixture 從網站建立 Operator，建立後可登入。隔離 fixture 不部署至正式資料庫。
- 加工中的已授權 configure 仍被設備互鎖拒絕。登入後製程查詢通過缺值、時間邊界、分鐘聚合、晶圓隔離、資料上限及 30 天期限測試；階段事件順序通過。

## 完成結果

- 登入後三批 75 片全流程通過，全部成功返回原 LP；含既有測試共 274 次完成製程的四階段採樣、五筆階段事件及最終膜厚核對通過。
- Redis 停止期間受保護 API 回覆 503，既有 WebSocket 關閉；恢復後 session 可繼續使用。後端重啟後全部帳號／密碼雜湊摘要相同。
- 正式環境已部署，初始化只建立一個 cvd.leader，使用指定初始密碼。正式 Operator／Engineer 臨時驗證帳號已完成建立、登入與越權拒絕，登出後刪除測試資料；正式保留初始 Leader。
- 正式後端重啟後，帳號、登入 session 及部署前全部批次／晶圓保持一致。未重新啟動或改寫模擬器。
- 外網 HTTPS 驗證 Secure／HttpOnly Cookie、登入、合法即時 WebSocket、登出通過；瀏覽器確認匿名登入頁、指定 Leader 登入及小螢幕帳號／登出按鈕。測試後畫面已登出。
- Docker Compose 驗證通過，只將網站／後端埠綁定 loopback；版本化原始碼與文件未包含指定初始明文密碼。備份與回退參見 RBAC_DEPLOYMENT.md。
- 隔離測試服務停止，資料卷保留；本次不封存 OpenSpec 變更。

