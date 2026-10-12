## Why

設備統計、晶圓量測、缺陷檢查與真空健康資料分散於不同畫面，使用者難以整理成可分享的完整報告。新增依實際資料生成的 AI 圖表與文字總結，並提供 Excel 下載。

## What Changes

- 新增報告生成頁面，選擇單台／全部十台、統計期間與繁中／英文／日文。
- 整合稼動率、PM 使用率、狀態甘特圖／比例、正式完成／報廢片數與獨立 Dummy 統計。
- 納入 KLA 膜厚／均勻度、OpenCV 缺陷及待複核結果、真空 PHM 健康與抽氣表現。
- 使用 OpenAI LLM 生成有來源依據的文字摘要與圖表配置，程式驗證數據並製作圖表。
- 提供非同步生成、進度、報告預覽、失敗重試與包含摘要／圖表／明細的 .xlsx 下載。
- 揭露資料缺口與模擬性質；不加入 Cp／Cpk，不改動設備控制或簽核流程。

## Capabilities

### New Capabilities

- `ai-report-export`: 跨模組資料報告、AI 分析、可追溯預覽與 Excel 匯出。

### Modified Capabilities

無；既有來源功能與業務規則維持不變。

## Impact

前端新增報告入口及工作狀態；Java 後端新增報告工作、資料快照、OpenAI API 整合與 Excel 產生服務；PostgreSQL 保存報告中繼資料，Docker 掛載報告檔案與 API 金鑰設定。沿用既有 RBAC、統計、KLA、視覺與 PHM 查詢來源。
