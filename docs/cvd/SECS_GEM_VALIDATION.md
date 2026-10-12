# SECS/GEM · Sparkplug B 驗收紀錄

驗收日期：2026-09-17。這是模擬 GEM profile，不代表 TEL 原廠 SECS mapping 或完整 SEMI／Sparkplug 認證。

## 已驗證

- 獨立手工 HSMS／SECS 封包：Select、Linktest、Deselect、Separate、分段／黏包、Local 拒絕控制、非法功能 S9F5、T3／T7／T8 與超長封包。
- 官方 Tahu schema 與獨立 Protobuf fixture；十台 topic、alias 世代、bdSeq、seq gap、缺 Birth／rebirth 與過期 Death 隔離。
- 外層控制 HMAC 驗證、過期與錯機台拒絕；S7 配方不可變、下載不自行套用。Broker MQTT v5 negative PUBACK 驗證跨台資料發布、Gateway 控制發布、Bridge 設備資料發布均被拒絕。
- 隔離環境十台各三批，共 750 片，逐台對帳回原 LP；01 為 74 成功／1 報廢，其餘各 75 成功。此批驗收沒有使用正式資料。
- 跨台 recipe 複製及版本衝突、Operator 拒絕修改、EE 提案／PE 核准、錯機台請求、重複核准只有一個執行意圖。
- 補做執行中中斷 Gateway 6 秒，其他設備繼續更新；恢復後原 executionId 完成一次，膜厚由 1.0043665424906012 增至 5.037324660596531。僅執行核准的 deposition 8 秒與 purge 1 秒。
- 核准／撤回競態與完成指令重播；已完成補做不再執行。未發布意圖撤回由 API 整合測試驗證，已發布取消柵欄由 Go 確定性測試驗證。
- 桌面十台卡片、機台切換與快速切換、recipe 複製目的地、PM2 實際滑鼠 hover、失聯／重連顯示；390px 手機無橫向溢出。
- PostgreSQL／TimescaleDB 隔離副本實際還原，重跑 schema 遷移兩次保留資料計數、內容、帳號密碼雜湊及簽核版本。
- 30 天 TimescaleDB 清理測試，以及 PostgreSQL 過期歷史清理保護活動批次和補做資料鏈。

## 驗收中修正的問題

1. Kafka command consumer 早於 topic 建立時沒有分區：Bridge 先建立命令 topic，並監測分區變更。
2. 待機時缺少製程可選欄位導致反覆 rebirth：Birth 保留完整字典，暫缺值使用 null。
3. 人工暫停停止歷史採樣：暫停仍每秒採樣，製程時鐘不推進；超過緩衝上限必須保存 data-gap。
4. 斷線期間大量重送增加磁碟與 Broker 負擔：每台每秒最多補送 10 筆、同 ID 間隔 15 秒，Bridge 不反覆重寫已有的持久事件。
5. Redis 相同版本的快照可能缺少／保留舊通訊品質：API 比對 equipmentId 與 communicationQuality，Pipeline 快取含品質的權威快照。
6. ExFAT 還原副本產生 AppleDouble `._*` 副檔干擾 Kafka 檔名解析：還原工具只清除隔離 Kafka 副本內這些作業系統副檔，原備份不改動。

## 證據位置

測試腳本在 `tests/integration/`、`protocol/`、`simulator/`、`gateway/`、`bridge/` 和 `backend/src/test/`。執行輸出與畫面在本機忽略目錄 `.runtime/secs-*.json`、`.runtime/secs-*.log`、`.runtime/secs-desktop.png`、`.runtime/secs-mobile.png`。

正式切換、完整卷還原、舊映像回滾及新鏈路演練均已完成，詳見下方簽結。

## 穩定負載與容量

最新候選版在 65.37 秒內新增 650 筆採樣，約全廠 9.94 筆／秒；最大來源時間年齡 4.42 秒。這是一般 Docker 排程下的短測，不是硬即時保證。壓力測試出現的排程延遲會記錄 `data-gap` 與缺少秒數，不補造不存在的樣本。

當次資源快照：Java 約 338 MiB／13.6% CPU，Bridge 約 9.9 MiB／15.0%，單台 simulator 約 7.4 MiB／1.3%，單台 Gateway 約 7.8 MiB／0.5%，Kafka 約 1,004 MiB／18.4%。Docker CPU 百分比以單核心 100% 計；主機同時仍有其他服務。

Kafka lag：command、accepted、connectivity、history 為 0，snapshot 為 1，telemetry 為 2。斷線驗收曾累積 428 筆 Bridge outbox，恢復後剩 6 筆在途；176 筆歷史補送，十台均完成固定時間窗對帳且 eventId 無重複。

TimescaleDB 短測增長 860,160 bytes／65.37 秒，線性推算 30 天約 **31.8 GiB**。Bridge 使用 Kafka Snappy 壓縮；另一個 30.78 秒短測增長約 32,345 bytes／秒，7 天 Kafka 約 **18.2 GiB**。兩者加上 PostgreSQL、索引、WAL、重送緩衝與安全餘量，現況至少應預留約 **80 GiB**；長期滿載需另做持續量測，不能把暫停／低活動短測當作最高需求。

本機內建磁碟驗收時約剩 12 GiB，**不足以上長期容量估算**。備份與完整還原的 Kafka 副本使用 My Book 外接磁碟，正式資料卷仍留原位；本次沒有刪除活動資料、降低採樣頻率或缩短 30 天策略來節省空間。後續持續展示前須補足正式儲存容量。


## 正式切換簽結

2026-09-17 完成。完整備份位於 `/Volumes/My Book/Codex-CVD-Backups/cvd-secs-cutover-20260917-1925/`：26 個原始持久卷的壓縮檔、逐檔 SHA-256 manifest、PostgreSQL／TimescaleDB dump、原設備 state、秘密檔案與固定舊映像的回滾 Compose。原備份不改寫；回滾使用 `compose.json` 內的備份映像標籤。

1. 十台 state 轉接重跑兩次完全一致，只設定 paused；保持 epoch、晶圓、膜厚、配方、指令與簽章原文。
2. 所有卷实际還原到 `cvd-secs-restore`。正式 DB 副本重跑新 schema 遷移兩次，歷史、遙測、帳號、recipe 版本／來源、簽核版本及在途意圖校驗一致。
3. 還原副本以舊 OPC UA 映像成功啟動，再於同一副本切換新 SECS/GEM／Sparkplug；兩次均通過 10 台、25 批、625 片晶圓、3 帳號對帳，全部暫停。
4. 正式切換先只啟動資料與 API，對帳成功後才開放前端。28 個正式服務均運行，27 個有健康檢查的服務全部健康；21 個 simulator／gateway／backend 服務使用 SECS 模式，Gateway 無 OPC endpoint，設備只監聽 HSMS 5000 和健康檢查 8081。
5. 正式網站與原臨時外網網址通過登入、十台切換、PM hover、配方目的地及手機 390px 檢查。沒有建立正式測試批次、修改正式 recipe 或恢復加工。

證據：`.runtime/secs-production-copy-migration.log`、`secs-rollback-proof.json`、`secs-restored-new-proof.json`、`secs-production-proof.json`、`secs-production-web-proof.json`、`secs-production-ui.json`、`secs-production-health.json`。本次建立的還原容器／內部測試卷於驗證後清除，外接備份、舊映像與其他既有測試資料保留。
