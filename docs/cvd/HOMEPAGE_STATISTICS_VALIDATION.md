# 首頁統計驗證 — 2026-09-20

本次只在 `cvd-disposition-test` 執行批次、故障、簽核及保養控制。正式環境僅備份、部署及讀取核對。

## 驗證範圍

- Go 全套測試：來源狀態優先序、Dummy 完成當秒搬離 PM 的分類、正常回槽、兩輪補做待確認、報廢與 PE 確認前後、重啟序號、Leader 放行不重計、舊終態不假造時間。
- Java 22 項測試與獨立資料庫：重複遷移、重送與亂序收據、衝突秒未知、矛盾結案序號排除、半開區間、可靠舊事件回填、原生 Dummy 證據重建、已存在投影但缺收據修復、暫存正式 FOUP 保留及結案 payload 清理後舊重播。
- 30 天 × 10 台的 432,000 筆分鐘事實：首次完整基準查詢 2,278 ms，合併後 JSON 7,977 bytes；各分類秒數合計等於分母，每台不超過 2,017 桶。這是固定狀態資料的量測，非所有負載的保證。額外壓力執行曾造成隔離通訊延遲，收據與來源缺口保留不完整標記；補送恢復後續驗證。
- 前端 13 項單元測試；真實 Chrome 桌面及 390px 手機：三圖／十列、單台切換、30 秒更新保留選台、四區間、快速切換、失敗重試、甘特圖鍵盤與觸控、無整頁橫向溢出、無執行錯誤；既有 Enter 進製程與 Esc 返回 3D 通過。
- SECS/GEM → Sparkplug B → Kafka：正式正常片、補做 PE 確認回槽、報廢回收確認；查詢與 SQL 逐台／逐區間一致。驗收快照 07：正式完成 9、報廢 2；05：正式 0、Dummy 完成 4。數字是隔離測試結果，不是正式設備產量。
- Operator／Engineer／Leader 皆可讀；未登入 401、非法參數 400，查詢前後 commands 筆數不變。
- 05 在原生來源升級、Bridge 斷線及重啟後仍維持 ready／派片鎖與四片 Dummy 結案，時鐘暫停；其餘設備隔離比對不受測試機台影響。

原始證據：`.runtime/statistics-evidence.json`、`statistics-transport-proof.json`、`statistics-ui-proof.json`、`statistics-migration-extra.log`、`statistics-go.log`、`statistics-java.log`、`statistics-frontend-tests.log` 及桌面／手機 PNG。

## 部署核對

備份目錄：`.runtime/backups/cvd-statistics-20260920`。完成後的 manifest 保存全部卷 SHA-256、兩個資料庫 dump、原映像與十份 checkpoint。舊映像及備份保留供整組回退；新增表不改動舊業務表結構。

正式部署只讀核對十台共 1,729 片晶圓一致。部署後逐台比較批次、槽位、剩餘加工秒數、配方、異常與保養狀態，詳細結果保存 `.runtime/statistics-deploy-proof.json`。部署與檢查不發送恢復加工、保養或簽核命令；重啟使模擬時鐘保持暫停。

相容演練：十份停止後 checkpoint 與備份逐一相等，新版 Load 不改批次、PM 製程、recipe、保養或簽核；另以舊映像在無網路的暫存目錄載入保養中、已放行及加工中的三種備份狀態，保持暫停且業務資料一致。這是 checkpoint 回退演練，未對正式資料庫執行還原。證據：`.runtime/statistics-upgrade-check.log`、`statistics-rollback-proof.json`。

正式外網網址完成登入、四區間查詢、手機觸控及 Enter／Esc 驗證；10 列與 10 台長條圖正常，390px 無橫向溢出、JavaScript 錯誤 0。可靠舊資料以背景批次回填，未補齊前仍明示不完整。證據：`.runtime/statistics-production-ui-proof.json`、`statistics-production-api.json`。
