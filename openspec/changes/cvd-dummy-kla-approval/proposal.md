## Why

目前四腔體 Dummy 驗證以設備模擬製程數據判定，恢復加工沒有強制等待 KLA 量測及 PE 確認。增加保養復機的量測與人工確認關卡，避免僅因設備加工完成就恢復正式派片。

## What Changes

- 本輪四片 Dummy 各完成 KLA 量測，保留 wafer map 與膜厚／均勻度證據。
- 僅 PE 部門 Engineer／Leader 可確認本輪四腔體測試合格或退回重測。
- 確認合格仍維持派片暫停，由 Engineer／Leader 手動恢復加工。
- 將結果及確認綁定設備、保養週期、驗證輪次、配方版本及量測版本，變更後失效。
- 展示等待量測、等待 PE 確認及可手動恢復狀態，保留既有保養、換載具及異常處置互鎖。

## Capabilities

### New Capabilities

- `dummy-kla-approval`: 保養 Dummy 的 KLA 證據、PE 確認與手動復機互鎖。與既有變更中的 maintenance-dummy-qualification 整合；目前主規格目錄沒有已同步能力。

### Modified Capabilities

無。

## Impact

影響 Go 設備保養狀態機、Java MaintenanceApi／MetrologyApi 與權限稽核、PostgreSQL 證據及確認紀錄、Vue 保養介面與首頁／3D 狀態展示。沿用既有 KLA 模擬及資料流，不新增真實設備連線；不更改一般正式晶圓僅標示異常的量測政策、異常處置的 Leader 放行流程或統計分類。
