# 機台稼動率驗證紀錄

## 自動化測試

- Go 模擬器、協定、Gateway、Bridge 測試通過；新增採樣測例涵蓋四階段、暫停、故障、留片、補做、延遲採樣與重啟。
- Java 19 項測試通過，含公式、台灣日界、四個區間、舊資料及十台固定分母。
- 前端 9 項測試通過且 production build 成功。
- 隔離資料庫驗證重送冪等、亂序、同秒矛盾永久未知、跨分鐘裁切、重啟與相容遷移。
- 隔離完整 SECS/GEM → Sparkplug B → Kafka → TimescaleDB 管線確認十台採樣、模擬暫停遮罩為零、健康加工可繼續、故障腔體排除、Bridge 中斷後指定缺口秒數補送且無重複。
- Leader、Engineer、Operator 均能唯讀查詢，未登入 401、非法區間 400；查詢前後 commands 筆數相同。

## 效能

以 432,000 筆分鐘彙總表示十台各 30 天，與獨立產生的 25,920,000 個設備秒的計算逐台比對一致。首次應用查詢 84 ms，連續十次平均 94 ms。這是首次／重複查詢量測，未清除作業系統磁碟快取，不宣稱實體磁碟完全冷啟動。

查詢計畫與完整數據保存於 .runtime/utilization-database-proof.json。查詢不解析歷史 telemetry JSON，只讀分鐘彙總與兩個邊界分鐘的秒資料。

## 介面與回退

1440px 桌面與 390px 手機實測十台卡片、四種區間、失敗重試與快速切換；手機 scrollWidth=390，無水平溢出，無 JavaScript 執行錯誤。自動巡走 Enter 進入正確設備、Esc 返回 3D 仍正常。

在 cvd-disposition-test 隔離環境回退至既有 disposition 映像再升級，晶圓、配方、暫停、派片暫停與異常流程資料保持一致。

## 重現入口

- scripts/check-utilization-database.sh：僅操作隔離 utilization_check 資料庫。
- tests/integration/utilization.py：僅操作 cvd-disposition-test 的測試機台。
- scripts/verify-utilization-ui.mjs：登入、唯讀查詢與頁面導覽。
- scripts/verify-utilization-production.py：對照備份檢查正式環境業務狀態及十台採樣，不發出設備指令。

## 展示環境部署

部署前完整保存 27 個 volume 備份及十台檢查點，SHA-256 與所有 tar 檔案讀取驗證通過；備份位置為 .runtime/backups/cvd-utilization-20260919-complete。

更新後已確認十台均開始利用率採樣，1,125 片晶圓的批次與狀態、recipe、派片暫停、異常事件及補做紀錄與備份一致，模擬時鐘維持暫停。所有業務控制未由部署流程自動執行。部署證據為 .runtime/utilization-production-proof.json，映像清單為 .runtime/utilization-release-images.json。隔離測試環境已停止，保留資料供重現。

外網 HTTPS 實測登入、四區間切換、失敗重試、390px 手機版與 Enter／Esc 導覽通過，十台卡片完整且無執行錯誤；證據為 .runtime/utilization-public-ui.log。
