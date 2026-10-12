# 完整異常處置操作與部署

異常時停止該機台新派片；正常在途晶圓仍可安全完成。資料與簽核按機台獨立。

1. EE 在「異常處置」認領系統建單，填原因、修復措施、驗證方式、驗證結果。晶圓方案可選補做或報廢；空 PM 故障選設備排錯。
2. 補做僅執行所選固定步驟（1–3600 整數秒、總計最多3600秒），沿用原膜厚及沉積速率。提交後不可改寫；需退回／撤回或失敗後另建版本。
3. 不同帳號 PE 核准即觸發執行。PM 故障或時鐘暫停時保留等待原因；核准不解除故障。
4. 補做合格後在原 PM 留片，由 PE 確認結果才授權原 LL／LP 槽位回收。不合格或 PE 否決保留膜厚，由原 EE 修訂；報廢需先完成回槽再由 PE 確認。
5. 所有處置完成後，未參與此次任何單的 EE／PE Leader 核准整台放行。設備原子重驗 generation、完整集合、確認及回收證據後自動恢復。新異常使舊核准失效。

無合格 Leader 時明示阻擋；不以換部門或同人代簽避開身份限制。「恢复時鐘」只讓已核准處置與安全回收推進，派片仍鎖定。

## 儲存與契約

- PostgreSQL：disposition_incidents、recovery_requests/versions/events/intents、disposition_confirmations、equipment_releases；新處置證據不隨30天遙測清除。
- Go checkpoint modelVersion 6：incidents、dispositionHold、holdGeneration、recoveries、authorizations、releases；Redis 不是派片鎖來源。
- 傳輸：參見 contracts/DISPOSITION.md、SECS_GEM_PROFILE.md、SPARKPLUG.md。
- 事件與所有命令使用固定 ID，重送查原結果；發布或 ACK 不代表已回收／放行。

## 升級／維護順序

1. 先驗證隔離環境。停止 frontend/backend/bridge/gateways/simulators，保留原卷。
2. `scripts/secs-backup.py <新的備份目錄>` 保留實際部署 image tag、完整 secrets/bindings、PG/TS logical dump、所有卷 archive、10台原始 checkpoint、SHA256 manifest。腳本需 PG/TS 處於可 dump 狀態，完成後正式服務保持停止。
3. 驗證全部 archive checksum 與可解壓完整性；在隔離資料庫還原 logical dump，對帳業務計數。對10台 checkpoint 執行新版 Load/Validate 對帳位置、膜厚、attempt、executionId。
4. 先啟動資料服務與新版相容 backend，再部署新版模擬器／gateway／bridge／frontend。持久化 modelVersion6 後才能對外開放。
5. 觀察待審、待結果確認、派片 hold、1Hz採樣及命令／事件交付。重啟保留人工時鐘暫停；不可為升級便利替使用者核准處置。

## 舊資料遷移

- 舊草稿／送審：保留 ID、版本及人員，退回補齊新排錯欄位。
- 舊核准未開始：建立取消柵欄，設備確認後才能修訂重審；遲到的舊核准不啟動處置。
- 執行中：保留 executionId、膜厚、秒數進度，完成後原 PM 留片。
- 已回槽但未放行的活動週期：保存原回槽事實，補 PE 結果確認及 Leader 放行；不搬回 PM、不重做。
- 真正舊歷史不生成新簽名；自動異常來源 owner 為空，由 EE 首次認領。

## 回退

新留片／確認／放行意圖存在後，不可單獨換回 modelVersion5 模擬器：舊版會自動卸片或允許 resume。優先相容修復。只有尚未產生新業務事件時才可恢復同一一致備份的所有資料卷與原 images；若已有新事件，保留現場與新稽核，先對帳再制定恢復方案。禁止只還原 PG、只回退 frontend 或清空 Redis 來假裝解除異常。

### 舊直接報廢已回槽的活動批次

部署前對帳發現01號機有舊 `scrap` 已回 LP、批次仍暫停而沒有 recovery execution 的案例。額外的一次性 dispositionMigrationVersion=1 遷移將其納入 hold。EE補齊排錯並選「確認既有回收事實」，PE核准只核對原槽位與報廢結果，再做結果確認及不同Leader放行；完全不重新加工或搬送。未取得的舊膜厚不填造為零，舊版本與晶圓履歷仍可查詢。
