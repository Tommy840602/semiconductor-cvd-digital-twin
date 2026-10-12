## Why

目前無法將整台機台掛為預防保養狀態，也無法用四腔體 Dummy wafer 測試證據限制復機。需要讓保養、驗證及正式加工恢復有明確且可追溯的分界。

## What Changes

- Engineer／Leader 可申請及結束保養；申請後停止正式派片，在製晶圓完成並回槽後進入保養。
- 結束保養回到待機且保留正式派片暫停，使用四片專用 Dummy wafer，每個 PM 各一片，獨立記錄測試結果。
- 四個 PM 都通過本次保養的測試、測試片完成回收且互鎖允許後，由 Engineer／Leader 手動恢復加工，不自動派片。
- 首頁及 3D 顯示保養／待驗證狀態。保養時間計入稼動率分母，保留「PM 使用率」名稱。
- 正式晶圓成功數不包含 Dummy，保養鎖與既有異常處置鎖各自管理，任何一方未解除均不得正式派片。

- 支援正式 FOUP 暫存：全部晶圓回槽後保留批次及槽位紀錄，騰出一個 LP 換上四片 Dummy 載具，測試後換回原正式 FOUP。

## Capabilities

### New Capabilities

- `maintenance-dummy-qualification`: 整機保養、專用測試片、四腔體驗證、手動復機、狀態展示與稽核。

### Modified Capabilities

無；主規格庫尚無規格，本次能力獨立新增並沿用既有設備隔離與異常處置契約。

## Impact

Go 模擬器與搬送排程、SECS/GEM／Sparkplug B 事件與指令、Java 權限及歷史 API、PostgreSQL 保養與驗證紀錄、TimescaleDB 利用率狀態、Vue 首頁與 3D 摘要。沿用十台獨立設備、三 LP、單片 LL、雙搬送手臂及四 PM 的實體限制。
