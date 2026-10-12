## Why

現有網站的設備資料及控制 API 尚無登入與角色驗證。導入 RBAC，讓日常操作、製程設定與帳號管理依職責執行，並追溯操作者。

## What Changes

- **BREAKING**：設備、歷史、控制 API 與即時連線改為登入後使用。
- 建立 Operator < Engineer < Leader 權限繼承；Operator 可查看資料、啟動批次、暫停與受控停止。
- Engineer 與 Leader 可修改 Recipe、注入／解除故障、重做、報廢與異常恢復。
- Leader 可建立 Operator／Engineer 帳號與初始密碼，不提供新增 Leader 或自行升權功能。
- 首次初始化建立使用者指定的 cvd.leader，使用指定初始密碼；重啟不覆寫。
- 帳號持久化、登入／登出、後端授權及操作稽核；前端依權限呈現控制功能。

## Capabilities

### New Capabilities

- `cvd-access-control`: 登入、階層角色、帳號建立、首次初始化與操作者稽核。

### Modified Capabilities

無；目前主規格庫尚無能力規格。本變更補充既有模擬功能的存取條件，不改變排程與製程規則。

## Impact

Vue 登入與帳號管理頁、控制按鈕；Spring HTTP／WebSocket 存取與指令入口；PostgreSQL 帳號及稽核資料；Redis 登入 session；Docker 初始帳密設定與外網反向代理；整合測試需先登入。Go、MQTT、Kafka 的製程與派片邏輯沿用。
