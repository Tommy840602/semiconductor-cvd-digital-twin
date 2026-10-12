# cvd-maintenance-dummy-qualification 驗證

2026-09-20，隔離 Docker project `cvd-disposition-test`，HTTP 22000／22080；正式環境測試只讀。

- Go 全套：三 LP 全占用排空；正式 75 片逐欄位比較不變；載入即有四片；固定四 PM；單片 LL 與搬送不變式；單 PM 品質失敗與全輪重測；recipe 失效、generation 拒絕；中途重啟保留參照與鎖；原空 LP 復原；指令重播；普通 pause 不能鎖死 Dummy；異常放行不越過保養鎖；Dummy 計入 utilization，保養不計 busy。
- Java 既有 19 測試通過；獨立 `maintenance_check` 資料庫演練重複遷移、事件去重／亂序、40 天未完成證據及暫存正式 FOUP 歷史保留、已結案過期清理、錯誤 generation、待下發 recipe 拒絕。
- `tests/integration/maintenance_qualification.py` 完整 SECS／Sparkplug／Kafka：Engineer 操作、Operator 403、未登入 401；三正式批次 → 換 LP2 載具 → 四 PM → Bridge 中斷／simulator 重啟 → 人工恢復時鐘 → 四 PM 合格 → 原 FOUP 全欄位比較一致 → 手動 resume。正式片數／批次數不增加。
- 第二輪注入 Dummy 故障：EE 排錯報廢提案、PE 核准及確認、Leader 放行；保養鎖仍在；當輪失敗；四片全新輪次通過並換回。其餘九台 lot 資料一致。
- 桌面與 390px 手機實際瀏覽器：四 PM 結果、配方／歷史、首頁 badge、Engineer 有操作、Operator 無操作、無橫向溢出、無 JavaScript 執行錯誤；Esc 回 3D。既有 Enter／Esc 導覽另跑回歸。
- 回退檢查：十份舊 checkpoint 通過；插入活躍保養後拒絕舊程式回退；含 Dummy checkpoint 亦拒絕。

證據位於 `.runtime/maintenance-*.log`、`maintenance-evidence.json`、`maintenance-ui-proof.json`、`maintenance-operator-proof.json` 與 screenshots。測試專用帳號、命令與批次均在隔離環境。

## 正式部署

完整備份：`.runtime/backups/cvd-maintenance-20260920`，含 27 個資料卷、兩個資料庫 dump、原映像標籤及十份 checkpoint；manifest 保存 archive SHA-256。新版 28 個服務正常運作。

只讀部署比對：十台共 1,625 片晶圓的批次、槽位、結果、搬送位置、剩餘製程秒數與異常鎖一致，僅新增用途／載具相容欄位。十台模擬時鐘保持暫停，沒有建立保養週期或送出加工操作。證據：`.runtime/maintenance-deploy-proof.json`。

舊 Cloudflare 臨時通道回報 Tunnel not found，已更新新通道與後端允許來源。最新網址保存在 `.runtime/maintenance-public-url`；外網瀏覽器登入及「申請保養」入口已驗證，沒有執行業務命令。證據：`.runtime/maintenance-production-ui.json`。
