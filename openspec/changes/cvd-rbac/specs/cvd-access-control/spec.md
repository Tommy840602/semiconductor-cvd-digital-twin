## Purpose

為 CVD 模擬儀表板建立依職責分工的登入與角色權限，涵蓋設備讀取、批次控制、製程設定、異常處置及帳號建立，並讓各項操作可追溯至實際登入者，同時保存既有製程與歷史紀錄。

## ADDED Requirements

### Requirement: 登入與連線存取
系統 SHALL 要求登入才能讀取設備、歷史、指令及即時資料，並提供登入與登出；登出或過期後 SHALL 拒絕新操作並停止該 session 的即時資料傳輸。登入失敗 SHALL 不揭露帳號是否存在，密碼不得以明文持久化或出現在日誌與 API 回應。

#### Scenario: 未登入讀取
- **WHEN** 未登入使用者呼叫受保護 API 或開啟即時連線
- **THEN** API 回覆 401，連線被拒絕，網站提示登入

#### Scenario: 登出與過期
- **WHEN** 使用者登出或 session 過期
- **THEN** 原 session 無法繼續使用 API 或接收即時資料，已開始的製程仍繼續

### Requirement: 角色與命令授權
系統 SHALL 實施 Operator < Engineer < Leader 的權限繼承；全部角色可查看資料與 start/pause/stop，只有 Engineer、Leader 可 configure/fault/clear/rework/scrap/resume。未知命令 SHALL 拒絕；前端與後端 SHALL 一致實施權限，後端拒絕發生於任何指令寫入或發布前。

#### Scenario: Operator 日常操作
- **WHEN** Operator 啟動批次、暫停或受控停止
- **THEN** 系統允許通過角色檢查，仍遵守原有設備狀態限制

#### Scenario: Operator 嘗試越權
- **WHEN** Operator 直接呼叫修改 Recipe 或任一異常處置命令，包括偽造 role 或 actor
- **THEN** 回覆 403，不寫入待執行命令且不改變設備或 Recipe

#### Scenario: 工程與主管操作
- **WHEN** Engineer 或 Leader 修改 Recipe 或執行異常處置
- **THEN** 通過角色檢查，但加工中不可修改設定等既有互鎖仍適用

### Requirement: Leader 建立帳號
系統 SHALL 僅允許 Leader 建立 Operator 或 Engineer 帳號並設定初始密碼，帳號名稱 MUST 唯一；建立成功的帳號 SHALL 能登入對應角色，系統 SHALL 不提供新增 Leader 或透過建立請求變更既有角色。

#### Scenario: 建立一般帳號
- **WHEN** Leader 提交有效且未使用的帳號、密碼及 Operator 或 Engineer 角色
- **THEN** 帳號成功保存且可登入，回應不含密碼或密碼雜湊

#### Scenario: 非法角色與重複帳號
- **WHEN** 提交 Leader／未知角色或已存在帳號
- **THEN** 拒絕建立，不覆寫既有帳號或權限

#### Scenario: 非主管越權建立
- **WHEN** Operator 或 Engineer 直接呼叫帳號建立 API
- **THEN** 回覆 403 且不建立帳號

### Requirement: 初始主管與持久化
系統 SHALL 在首次初始化建立 cvd.leader，角色為 Leader，初始密碼使用使用者指定的部署秘密值。重啟 SHALL 保留既有帳號、密碼與角色，初始化不得覆寫同名帳號。

#### Scenario: 首次啟動與再次啟動
- **WHEN** 空白帳號資料庫首次啟動後再重新啟動
- **THEN** 指定初始帳密可登入，且只存在一個初始化主管；後續建立的帳號保持不變

### Requirement: 操作身分與稽核
系統 SHALL 由登入 session 決定操作者，記錄新命令及帳號建立的操作者、時間、目標與接受／拒絕結果，命令執行結果可透過 commandId 追蹤；稽核不得包含密碼。舊歷史 SHALL 保留且不補造操作者。

#### Scenario: 操作追溯與重送
- **WHEN** 登入使用者送出或重送命令
- **THEN** 系統依當次權限檢查，原命令的操作者不被重送內容改寫，結果可關聯原命令

#### Scenario: 舊資料可讀
- **WHEN** 已登入使用者查看升級前的晶圓歷史
- **THEN** 原紀錄保持完整，操作者顯示未記錄
