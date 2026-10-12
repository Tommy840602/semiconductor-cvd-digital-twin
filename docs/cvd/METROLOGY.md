# 模擬 KLA wafer map

## 功能與界線

十台 CVD 加工完成且返 LP 後，每個加工版本建立一筆送測工作。一台 KLA-SIM-01 共用 Go 模擬站逐片量測，網站晶圓履歷／告警與操作頁提供膜厚及缺陷 Tab、圖例、點選及鍵盤詳情、版本比較。三語介面保留晶圓與設備 ID。

膜厚 49 點、缺陷分類與位置皆為合成展示，不是真實 KLA 儀器輸出。合成模型及設定見 `contracts/METROLOGY.md`。超規只生成量測紀錄的站內告警；不自動報廢、暫停 CVD、改 recipe、建立簽核或寄信。CVD 成功不表示量測合格。

量測搬送是獨立物流狀態，不改 CVD LP／LL／PM 的現有位置模型。尚未量測、無完整結果及過期分別呈現。Engineer／Leader 可重測與確認告警；Operator 唯讀。技術重送沿用原 ID，人工重測新增版本。

## 部署準備

1. 備份 PostgreSQL 與所有 CVD 檢查點，另保留原 image tag；確認備份可驗證，再開始更新。不要清空任何 volume。
2. 建置 simulator、gateway、bridge、backend、frontend 與 `metrology/Dockerfile`。Go 版本 1.26；Java 21。量測站 build context 為 repo root。
3. 使用 `scripts/prepare-metrology-auth.py --directory .runtime/secrets/secs-production` 增加獨立 MQTT 密碼與 ACL；先備份該私人目錄。工具保留既有帳號密碼，不輸出 secret。重新載入 MQTT 設定後才啟動量測 gateway。
4. 在既有 compose 清單附加 `compose.metrology.yaml`；預設 `METROLOGY_ENABLED=false`。測試環境應使用獨立 project、volume、secrets，覆寫 `kla_mqtt` 路徑及不公開服務埠。
5. 先在隔離環境確認 Kafka／MQTT／HSMS 全鏈路、重連與資料庫持久化。再部署正式服務並設定 `METROLOGY_ENABLED=true`。首次啟用時記錄資料庫啟用時間；升級前晶圓不回填。
6. 驗證來源加工與返 LP 事件欄位均已由新版 simulator 提供，並確認十台 CVD／40 PM 統計不包含 KLA。

## 回退與恢復

設 `METROLOGY_ENABLED=false` 並重啟 backend，停止新工作建立及派送；已發出工作仍可回報原結果。保留 metrology-data、metrology-gateway-data、資料庫表、Kafka 訊息與 bridge spool。回退 UI／其他服務 image 時不移除 migration 表。重新啟用後讀取既有工作及啟用界線，使用同 measurementId 續傳；不要用刪檔方式重置站端序號。

完整 map 預設保留30天；未確認告警與進行中同來源工作保護結果。清理後保留識別及 expired 標記。重測過期或來源無效紀錄會被拒絕。

## 驗證狀態（2026-10-02）

Docker 原卡在停止狀態，已恢復；正式功能已部署並啟用。十台 CVD 及 KLA 站即時資料均正常，網站與臨時公開網址回應 HTTP 200。正式驗證未新增批次或下達加工指令，既有已完成晶圓不補造 map。

- 前端24項測試、正式建置、三語／手機／圖表與版本比較瀏覽器驗證通過。
- 後端50項測試零失敗、零跳過，包含隔離 PostgreSQL migration、乱序／重複證據、留片／報廢／dummy、失敗結果、版本冪等、權限及清理保護。
- Go 模型、站端 checkpoint／HSMS round-trip、protocol、gateway、bridge 與既有 simulator 測試通過；Bridge 驗證1000缺陷不截斷。
- 隔離 `cvd-kla-validation` 執行兩輪各10來源晶圓；第二輪 MQTT 中斷後續傳，皆完成49點膜厚與缺陷圖。過程含重複及亂序事件、站端重啟、人工重測冪等。查證加工 commands、recovery_requests、notification_events 均為0。
- 匿名查詢401、缺CSRF拒絕403、Operator唯讀與禁止重測通過。功能停派／恢復演練保留所有不可變量測結果。
- 正式 PostgreSQL 備份經 pg_verifybackup 驗證；保存十台檢查點及回退 image。部署前後帳號、recipe、補做與簽核資料雜湊一致，Email 維持停用。

備份位於 `.runtime/backups/kla-20261002/`；回退 image 為 `cvd-before-kla-{backend,frontend,simulator,gateway,bridge}-20261002`。`.env` 已加入 `compose.metrology.yaml` 與 `METROLOGY_ENABLED=true`。恢復舊環境設定前先停止新派送並保存現有量測資料。

驗證腳本：`scripts/prepare-kla-validation.py`、`scripts/verify-kla-stack.py`、`scripts/verify-kla-access.py`、`scripts/verify-kla-switch.py`。腳本限定隔離 project／連接埠，不對正式資料送合成事件。瀏覽器 mock 測試為 `tests/integration/metrology_ui.cjs`，需 Vite 18003 及 `PLAYWRIGHT_MODULE`。

證據在 `.runtime/kla-*.log`、`.runtime/kla-stack-proof.json`、`.runtime/kla-production-proof.json`、`.runtime/kla-mobile.png` 與 `.runtime/kla-production.png`。初次停派演練等待時間太短曾逾時，後以包含完整冷啟動時間的 `verify-kla-switch.py` 重驗；以 `kla-switch-test.log` 為準。
