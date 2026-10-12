## Why

首頁目前有稼動率數字與 3D 即時狀態，但無法看出十台設備在一段時間內何時加工、保養或停機，也無法比較同區間每台的完成與報廢片數。新增一致時間區間的圖表，讓使用者從全廠概況追查單台設備。

## What Changes

- 在既有稼動率區與 3D 之間加入十台設備狀態甘特圖、狀態時間圓環圖、每台完成／報廢片數長條圖。
- 共用今日、最近 24 小時、7 天、30 天與 Asia/Taipei 顯示時間；圓環圖支援全廠或單台，圖表支援滑鼠及觸控查看明細。
- 補足具有來源時間的歷史狀態採樣、互斥分類及可重播的晶圓結案投影。未知時間明確顯示，不以最後狀態補滿缺口。
- 正式晶圓依結案時間計數，每片最多一次；補做不另增產出，Dummy 與正式片分開呈現，不增加正式完成或報廢數。
- 保留目前現代化配色、PM 使用率名稱、稼動率公式及既有 3D／製程導覽；所有圖表操作唯讀。
- 不包含 Cp／Cpk、製程能力表、預測排程、recipe 修改或設備控制功能。

## Capabilities

### New Capabilities

- `homepage-statistics`: 首頁狀態歷史圖表與依結案時間計算的設備產出統計，包含資料品質、歷史相容、用途隔離及互動呈現。

### Modified Capabilities

無。主規格庫目前為空；已完成變更中的 equipment-utilization 與 maintenance-dummy-qualification 為相容依據，本變更不改其既定計算及業務流程。

## Impact

- Vue：首頁、UtilizationPanel、CleanroomOverview 的共同篩選與新增圖表元件。
- Java／Spring：統計查詢、歷史投影與採樣接收；PostgreSQL 保存結案證據，TimescaleDB 保存狀態區間／彙總。
- Go simulator：增加狀態統計契約與需要的結案證據，保持 SECS/GEM → Edge → Sparkplug B → Bridge → Kafka 主鏈路。
- 新增相容遷移、歷史補送／去重、前端可用性與隔離整合驗證。圖表優先採既有 Vue 搭配 SVG，不預設加入圖表套件。
