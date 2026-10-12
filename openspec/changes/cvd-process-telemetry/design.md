## Context

動機見 proposal.md。本變更跨 Go、Spring 與 Vue，涉及資料契約、歷史聚合及檢查點相容性，因此需要設計文件。現有 simulator 以單一 Tick 更新狀態，modelVersion 2 支援單片 LL；製程僅依 remaining 劃分三階段。TimescaleDB 保存遙測 JSON；Api.java 長區間聚合目前僅處理 temperature、pressure、flow。既有設備、FIFO 與人工故障處置以 tel-tin-cvd-simulator 的完整規格為基線。

## Goals / Non-Goals

**Goals:** 讓階段、感測值與合成晶圓結果一致、可重現、可追溯；新舊樣本並存且不誤導；手機也能檢視曲線。

**Non-Goals:** 真實反應化學、TEL 原廠配方、實機控制、新故障模型、自動報廢、登入／外網部署改造及新增資料服務。既有手動故障控制仍須正常運作。

## Decisions

### 階段與時間

採預熱 preheat、穩定 stabilize、沉積 deposition、吹掃 purge，再發出完成事件。完成是結果狀態，不額外占用一個 Tick。沿用 processSeconds 作總加工時間，四階段各至少 1 秒，剩餘秒數按 20%、20%、45%、15% 分配，以固定最大餘數規則保證總和精確；5 秒短配方也不得略過階段。每秒提供 phaseElapsedSeconds、phaseDurationSeconds、phaseProgress、runProgress。階段開始／結束即時記錄，最後樣本與完成事件保存最終結果，避免搬離 PM 後遺失。

相比直接加隨機數，本方案把數據與階段綁定，曲線能解释流程。既有暫停派片不凍結健康 PM；全域暫停凍結模擬時間；PM 故障凍結該製程，解除故障不得自行繼續異常晶圓。

### 合成模型與單位

以下是實作預設，均為展示尺度，不聲稱物理精度：

| 資料 | 模型／單位 |
|---|---|
| 溫度 | 設定／實際 °C、偏差 °C；沿用 400 °C 展示設定，以一階響應靠近設定 |
| 加熱 | heaterPowerPct，依溫差求 0–100% 的合成功率 |
| 四路氣體 | precursor、reactant、carrier、purge，各含 target／actual，單位 sccm；以功能名稱避免暗示特定化學配方 |
| PM 壓力 | 設定／實際 mbar，沿用 2 mbar 展示設定，以有界響應與流量、閥門狀態產生趨勢 |
| LL 壓力 | 在既有三秒抽氣／回填期間，以對數插值連接 1013 與 0.01 mbar；保持原連鎖時序 |
| 真空狀態 | pumpState 列舉、valveOpeningPct 0–100%；狀態對應抽氣／保持／回填，不代替連鎖權威 |
| 膜厚與速率 | thicknessNm、depositionRateNmPerSecond；只有沉積時增長，其他階段速率 0 |
| 均勻度 | uniformityPct，明確定義為模型內合成採樣點的半極差／平均膜厚 ×100%；沉積前無值 |

正常流量以固定版本配方的分階段設定為基礎，非活躍氣路流量為 0。腔體差異限制在設定量的 ±2%，噪聲限制在 ±1%；採 modelSeed、PM ID、processRunId、模擬 Tick 的穩定雜湊產生，不使用牆鐘隨機數。所有輸出保證有限、壓力為正、流量及膜厚非負。零設定不加入負流量噪聲。

預設目標膜厚 10 nm；沉積基準速率由目標膜厚／沉積秒數計算，受合成流量及腔體偏差調整，這是展示模型而非物理沉積方程。品質預設膜厚偏差不超過 ±10%、均勻度不超過 5%，門檻與 modelRevision 存入結果。品質為 pass／review／not-evaluated，獨立於既有 success／scrap／exception，不自動觸發報廢或暫停。

### 狀態、契約與保存

Go 為唯一數據來源；前端只繪圖。保留既有 temperature、pressure、flow 欄位及意義，flow 對應四路 actual 的總量；增加巢狀 processTelemetry 與 gasChannels，使用 telemetryRevision 區分欄位集合。事件 envelope 延用 schemaVersion 1、eventId、sourceTime、sequence 與既有去重策略。每次加工用 waferId、attempt 與配方版本構成獨立 processRunId；重做清零本次膜厚，原次結果保留。

事件包含 process-phase-changed 與 process-result；process-result 在成功完成時保存模型結果，在人工報廢時記錄未完成與 not-evaluated，不補造成功膜厚。PM 空閒時 processRunId／waferId 無值，過去晶圓結果只在履歷或明確的「上次結果」區域顯示。

繼續透過 OPC UA Snapshot／Events、Go gateway、MQTT、Kafka 傳送，TimescaleDB 每秒保存數值，PostgreSQL 保存階段／結果事件，Redis 保存最新狀態。優先延用 JSON 儲存而非新增大量固定欄位表，降低遷移風險；以契約 fixture 驗證 gateway 沒有遺失新欄位。模型版本、階段計時、seed 與累積膜厚納入檢查點，重啟保持暫停，不重複產生結果。

### 查詢與視覺呈現

沿用 /api/telemetry，擴充既有 PM／wafer／時間篩選並增加受白名單限制的 metric 及 LL 篩選，保持既有無 metric 請求相容。新曲線使用一致的 time、equipmentId、processRunId、actual、target、unit、sampleCount 格式；秒級回傳原值，長區間按分鐘與設備／processRunId 分組，數值取平均並提供 min／max，列舉狀態不取平均。階段界線從事件查詢，不能以平均進度重建。

最近 30 天、UTC [from,to)、分頁及點數上限延用既有規則。新增欄位在舊樣本中回傳 null／無資料，不能補 0；斷線曲線留缺口，不連線冒充持續採樣。超過點數限制時明示聚合粒度或需縮短時間，不靜默截掉範圍。

保留現代青綠配色與放射狀拓樸。總覽保留狀態摘要；點 PM 的詳情內以溫度／氣體／真空／膜厚分組顯示設定值和實際值，使用對應單位及座標軸，不把不同單位混畫在同一尺度。手機以分頁或單列卡片呈現；PM 及晶圓歷史共用指標選擇。LL 點擊詳情可看壓力曲線。Lot 與晶圓識別仍在點擊後顯示。

## Risks / Trade-offs

- [合成曲線被誤認實測] → 詳情與結果標示模擬值，保存 modelRevision 與判定門檻。
- [短配方顯示不清楚] → 每階段至少 1 秒，支援既有總時間設定，保持事件歷史可追蹤。
- [欄位增加使每秒快照與查詢变大] → 僅保存當前數值，歷史走查詢 API，量測 payload 大小與查詢時間。
- [新舊資料混查誤判零值] → nullable 指標及 sampleCount；缺資料畫缺口。
- [中斷／重做重複計膜厚] → run ID、固定 seed、檢查點、事件 ID 去重與恢復測試。

## Migration Plan

1. 先加入契約 fixture 與向後相容查詢，再發布新版前端，舊樣本顯示未提供。
2. 備份 simulator 檢查點與資料庫；記錄現有故障／暫停。modelVersion 2 舊製程若尚在 PM 加工，保持 legacy 模型完成該次，新增模型從下一次加工開始；不倒推過去膜厚或偷偷重跑。新檢查點版本由實作明確編號。
3. 更新 Go simulator 與 gateway，驗證既有通訊鏈路及秒級存儲。重啟後保留人工恢復流程，部署驗收不得擅自清除現有故障。
4. 以隔離測試環境跑完整新模型正常批次與回歸情境，確認後再更新展示環境。
5. 回退前停止新增批次，保留新檢查點與資料卷；優先回退前端及查詢。新版進行中檢查點不得直接交給舊 simulator；需排空批次或使用相容模型版本。只有明確接受回復時間點時才從備份恢復，避免遺失升級後履歷。
