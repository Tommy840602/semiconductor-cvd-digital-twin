## Why

現有簽核只涵蓋補做核准，報廢與恢復派片仍可由一般工程操作直接觸發，補做合格也會在 PE 確認前回收。需要把排錯、處置決策、實際執行、結果確認與整台放行串成可追溯的異常處置流程。

## What Changes

- 異常建立處置紀錄並暫停該台派片；EE 提交排錯原因、修復與驗證結果，提出補做或報廢方案。
- PE 核准指定版本後直接觸發處置，保留現有互鎖、基線檢查與去重契約。
- **BREAKING** 補做完成後留在原 PM，PE 確認通過才允許搬回原 LP；不通過保留膜厚，由 EE 重提方案。
- **BREAKING** 報廢改為 EE 提案、PE 核准後回收，再由 PE 確認回收結果，停用直接報廢繞過入口。
- 整台所有阻擋異常完成 PE 確認與晶圓回收後，才可由 EE／PE Leader 核准放行。
- **BREAKING** Leader 必須與此次放行涵蓋所有處置的提案、PE 審核、結果確認帳號不同；核准後重驗条件並自動恢復整台派片，新異常使舊放行核准失效。
- 獨立保存核准處置、確認結果、核准放行及設備實際完成紀錄。

## Capabilities

### New Capabilities

- `cvd-exception-disposition`: 異常建單、排錯、補做／報廢處方、PE 審核、留片與結果確認資料鏈。
- `cvd-equipment-release`: 整台阻擋條件彙總、不同帳號 Leader 放行、設備最終重驗與自動恢復派片。

### Modified Capabilities

無已同步的主規格可修改，目前主規格庫為空。本變更擴充既有 cvd-wafer-recovery-approval 與 cvd-multi-equipment 變更；對活動異常處置，合格即回收、直接報廢與一般 resume 規則由本變更取代。實作／後續規格同步須明確調和這些舊契約，不能讓兩套放行行為並存。

## Impact

涉及 Vue 處置與控制畫面、Java RecoveryApi／控制授權／交易發布、PostgreSQL 處置及放行紀錄、Go 模擬器的製程完成／回收／派片狀態機，以及 SECS/GEM 和 Sparkplug B 指令事件映射。維持十台獨立、晶圓不跨機台、1Hz／30天遙測與現有 recipe 隔離。需要對舊活動處置做相容遷移及斷線重送驗證，不改寫歷史核准事實。
