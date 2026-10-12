# CVD 展示驗收紀錄

驗證日期：2026-09-15（Asia/Taipei）。環境：macOS ARM64、Docker Desktop，Java 21；全部九個服務以 Docker 執行。

## 已通過

- Go 模擬器 9 組頂層測試（故障測試含重做及報廢子情境），包含晶圓守恆、容量／連鎖、FIFO、四 PM 並行、指令期限及版本、故障回片、檢查點與緩衝超限。
- Go OPC UA 獨立客戶端實際讀取快照與呼叫控制成功；gateway 的正式執行鏈路使用訂閱。
- Java 3 個測試：跨語言事件契約、MQTT callback 不同步發布、Kafka 失敗不確認事件。前後端與兩個 Go 模組均成功建置。
- 三批 75 片端到端完成：FIFO、不混批、最多四 PM、全數回原 LP。
- PM 故障後正常 PM 完成回片；異常片重做與報廢兩條流程皆通過，人工恢復後整批結案，履歷保留。
- MQTT、Kafka、PostgreSQL、TimescaleDB 各停止 7 秒再啟動，資料鏈路恢復；Redis 快取清除後重建。
- 搬送中重啟模擬器，25 片位置保留且停派；人工恢復後整批完成。
- 30 天歷史查詢邊界、TimescaleDB retention job 實際清除過期分區，以及 PostgreSQL 保留活動批次必要紀錄。
- 重複投遞不增加歷史事件數，PM 分別分鐘彙整及分頁不重複，指令接收交易回滾不留下待執行紀錄。
- 瀏覽器檢查設備配置、25 格 LL 槽位、即時／過期狀態、歷史表格與控制結果。含報廢批次明確標示，操作名稱及設備識別可見。

- 不合法訊息進入獨立錯誤佇列並顯示告警，正常遙測持續接收。

## 修正與驗證範圍

中斷測試發現 MQTT 回呼內同步 publish 可在重連積壓訊息時互相等待；已改成持久化佇列交由背景發布，加入回歸測試並重跑 MQTT 恢復情境。

Retention 測試最初以 CALL 內含子查詢呼叫 job，PostgreSQL 不接受該語法；已改成先讀取 job ID，再執行 CALL，過期分區實際清除成功。此為測試指令修正。

標準 Docker builder 映像拉取在本機沒有完成；驗收使用 `scripts/build-local.sh` 產生相同程式的執行映像，再以 Compose 啟動。已驗證該建置路徑，沒有宣稱標準多階段 Docker build 已完成。

資源快照（約五批展示資料）：Java 後端約 311 MiB、Kafka 約 455 MiB、兩個資料庫合計約 198 MiB、其他服務合計約 70 MiB。這是單次觀察，非長期容量或壓力測試；未進行連續 30 天運行測試，保存期限採邊界資料驗證。

## 重現入口

- `tests/integration/scenarios.py`：正常／重做／報廢。
- `tests/integration/resilience.py`：中斷、重啟與保存期限。
- `tests/integration/storage.py`：重複事件、彙整與分頁。
- `README.md`：建置、啟動、操作、停止及備份說明。

## 參考圖配置驗證（2026-09-15）

保留青綠配色，改為中央 TM 與四周 PM、下方三 LL／EFEM／三 LP。Vue production build、前端 Docker 映像建置及 OpenSpec strict validate 通過。瀏覽器確認 PM3 點選切換詳情、LL1 槽位連至晶圓履歷。建立 LAYOUT-7d7927（LP3 → LL1）驗證大氣側跨站路徑及真空側 LL1 → PM2 路徑；SVG 動畫持續 0.85 秒，畫面確認在途晶圓與手臂。本次為前端配置驗證，未重跑整套後端耐故障測試。

### 批次資訊按需顯示

前端建置與 OpenSpec strict validation 通過。瀏覽器確認 LP1 點擊開啟空狀態，啟動模擬批次後視窗顯示關聯 Lot、FIFO、進度，Escape 可關閉；關閉後設備總覽不顯示該 Lot 識別。

## 單片 LL 改版（modelVersion 2）

Go race 測試通過：三批 FIFO／四 PM 並行、單片容量（含在途預留）、逐片抽氣回填、故障清除仍保留異常片、暫停時退回待加工片避免回片死鎖、原 LP 槽位守恆及舊狀態遷移。前端 production build、Docker 前端與模擬器映像建置、OpenSpec strict validation 通過。瀏覽器確認 LL1～LL3 各 1 格，LP1 對話框 25 格。部署前備份 backups/ll-single-capacity/state-before.json；保留 PM2 故障與暫停，25 片舊 LL 庫存移回 LP1，透過 API 分頁確認 25 筆 capacity-migration 已入歷史。完整長時間整合腳本已調整容量與時間界線，本次未重新跑完全部端到端情境，既有故障狀態未為測試而解除。
