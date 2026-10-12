## Why

目前 LL 抽氣採固定時間，無法展示真空系統逐漸退化、提前預警與保養成效。加入可重現的模擬 PHM，讓使用者能從壓力與抽氣耗時追蹤泵浦健康，並接回既有保養及 Dummy 驗證流程。

## What Changes

- TEL-CVDTIN-08A-01～10 各有 LL1～LL3 獨立模擬泵浦，共 30 組；累計使用與退化互不影響。
- 抽氣效率隨使用下降，產生壓力曲線、耗時、達標率、健康分數與模擬剩餘抽氣次數；保留模型版本及估算依據。
- 退化預警不中斷加工；實際抽氣逾時／無法達標時鎖住該 LL 搬送，保留晶圓位置，整台暫停派片，其他設備不受影響。
- 無可用回收通道時，Engineer／Leader 可在全台停派、空 LL 隔離後先修復並測試一組泵浦，只供回收；之後仍須完整保養、Dummy 與人工復機。
- 整台進入保養，可選一或多組泵浦；僅重設所選泵浦退化，保留歷史與終身使用次數。
- 所選 LL 抽氣測試通過後，才進行既有四個 PM Dummy 驗證；最後由 Engineer／Leader 手動恢復加工，既有異常處置放行仍須完成。
- 提供 PHM 檢視與預警、歷史曲線及保養證據；所有壽命數值明確標示為模擬估算。

## Capabilities

### New Capabilities
- `cvd-vacuum-phm`: 獨立泵浦退化模擬、健康監測、預警、抽氣失敗聯鎖及資料歷史。
- `cvd-vacuum-maintenance`: 選擇泵浦保養、抽氣測試及既有 Dummy／人工恢復流程整合。

### Modified Capabilities
無。主規格庫尚無已同步規格；本次新增規格引用既有 maintenance-dummy-qualification 與 exception-disposition change 的行為，不改寫其他 change。

## Impact

影響 Go simulator 的 LL 狀態機與 checkpoint、SECS/GEM 至 Sparkplug B／Kafka 的加法式資料契約、Java API 與權限驗證、PostgreSQL 事件紀錄、TimescaleDB 一秒採樣，以及 Vue 多語言 PHM 與保養介面。沿用既有 Docker 與傳輸架構。

不包含真實泵浦壽命校準、機器學習、PM 真空泵、加熱／供氣退化、KLA 缺陷因果模型、自動保養或自動恢復加工；不假設實際 TEL 泵浦配置等同本模擬。
