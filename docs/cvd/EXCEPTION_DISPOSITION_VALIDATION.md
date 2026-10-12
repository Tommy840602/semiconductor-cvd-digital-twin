# cvd-exception-disposition 驗收

2026-09-19；隔離專案 `cvd-disposition-test`，完整10台 SECS/GEM → Edge → Sparkplug B → bridge → Kafka → Java/PG/TS/Redis/Vue。測試不使用正式批次。

| 範圍 | 驗證與證據 |
|---|---|
| 補做成功 | exception_disposition.py：系統建單無 EE 簽名、四排錯欄位、PE核准、精確[0,0,3,1]秒、原膜厚增加、合格PM留片、PE確認、原槽位、Leader放行 |
| 報廢 | 同一單補做被PE否決後追加新版報廢，保留每輪基線及結果；先回槽再PE確認，scrap不變success |
| 空 PM／多單 | 空PM建立設備來源，無虛構晶圓；另一單未結案阻擋全台 |
| 授權 | Operator/錯部門拒絕；換部門仍拒絕自審；參與人升Leader仍不可放行；獨立EE/PE Leader均通過 |
| 版本與重送 | 舊版本／舊結果／跨台拒絕，重複核准僅一execution，重複確認只一授權；Go測試補做／報廢取消競爭 |
| 放行競爭 | Go測試核准快照後新故障拒絕舊generation；已放行ID重播不解除新hold；失敗releaseId不能日後沿用 |
| 斷線重啟 | disposition_outage.py：執行中重啟保留ID／膜厚，需恢复時鐘；gateway斷線後同ID補送、PM留片；其他台持續採樣；新故障拒絕舊PE確認 |
| 新資料遷移 | DispositionMigrationCheck：六種舊狀態、原版本／作者／executionId保留、取消柵欄、不虛構簽名、兩次重跑去重、跨台FK拒絕 |
| 正式checkpoint演練 | TestProductionCheckpointMigrationReadOnly：10台原始backup唯讀Load/Validate；725片位置、Slot、Lot、attempt、膜厚／五點样本／秒數不變 |
| 保留政策 | 新incident鏈與confirm/release命令排除業務30天刪除；40天活動異常仍存在；TS政策仍為30days，1Hz既有通道不變 |
| 畫面 | verify-disposition-ui.mjs：EE四欄位與秒數表單、PE結果、Leader阻擋／紀錄；桌面1440及390px手機無溢出／runtime error；圖片已目視檢查 |
| 3D | verify-cleanroom-blackbox.mjs：10台選取、手動／自動、俯視切換、手機390px、context失敗fallback與retry；觀測60FPS；navigation單元測試保留 |
| 防繞過 | 一般scrap/rework/簽核專用指令拒絕；活動hold的resume/start拒絕；clear/configure/recovery-clock不解hold |
| 傳輸 | Sparkplug指標roundtrip含hold/generation/完整evidence；真實SECS及bridge全流程成功；Go protocol/gateway/bridge既有契約測試 |

原始證據（本機忽略目錄）：`.runtime/disposition-evidence.json`、`disposition-outage-evidence.json`、`disposition-migration.log`、`disposition-production-migration.log`、`disposition-ui-proof.json`、`disposition-3d-proof.json`、各 build/test log。畫面測試只在隔離環境使用測試帳號。

正式備份：`.runtime/backups/cvd-disposition-20260919`；所有資料卷archive、PG/TS logical dumps、映像tag與secrets保存在私人目錄。還原工具會逐檔讀取所有archive資料並驗證SHA256，實際還原兩個logical DB至隔離 `backup_restore`；不把Kafka物理卷展開到正式卷。回退限制參見 EXCEPTION_DISPOSITION.md。

補充實際舊狀態驗收：`disposition_legacy_return.py` 在隔離04號機重建舊直接報廢已回 LP 的暫停批次，通過自動 hold、EE補資料、PE核准純證據檢查、PE確認、獨立Leader放行。晶圓一直留LP1/slot1，scrap與attempt=1不變，baseline未知保持null。部署採 dispositionMigrationVersion=1，01號正式舊草稿關聯其原owner與version，等待真人新簽核。

## 正式部署結果

- PostgreSQL還原帳號3筆、處置單2筆、版本2筆、執行意圖0筆、設備快照10筆、歷史9258筆、命令134筆；TimescaleDB還原1,352,431筆遙測。
- 完整archive checksum/逐檔讀取通過；備份約1.3GiB。
- Redis啟動遇到既有AOF檔尾損壞；原檔已在一致備份，`redis-check-aof --fix` 僅截除無效尾端2550bytes，base RDB checksum正常；製程PG/TS與checkpoint不受此修復影響。
- 新版10台modelVersion6，舊直接回收一次性遷移版本1。725片位置、槽位、Lot、attempt及既有膜厚／秒數對帳不變。01號機650片、05號機75片。
- 01號機既有未放行報廢片維持LP1/scrap，新增hold並關聯原EE草稿，狀態退回補資料；未代替任何人核准。其餘原人工派片暫停及全台重啟時鐘暫停保留。
- 隔離環境使用相同部署模擬器映像重啟已合格待確認晶圓，仍留原PM、膜厚不變、PE確認與派片hold持續。
- 28個服務啟動，27個具健康檢查的服務正常。外網重新建立臨時通道，登入與異常處置桌面／手機畫面驗證正常。

部署／對帳原始證據：`.runtime/disposition-production-proof.json`、`disposition-backup-proof.json`、`disposition-release-images.json`、`disposition-held-restart.json`、`disposition-legacy-return-evidence.json`、`disposition-redis-repair.log`。此記錄不宣稱對真實TEL設備或製程做過認證。
