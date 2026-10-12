# SECS/GEM 與 Sparkplug 切換

目前程式保留明確的協定選擇開關，正式切換採 CVD_PROTOCOL=secs；驗收前舊環境維持原映像。新鏈路：十台 Go 設備／HSMS → Go Gateway／Sparkplug B → MQTT → 獨立 Go Bridge → Kafka → Java 平台。Bridge 啟動先建立命令 topic，並監測分區變更，避免 consumer 先於 topic 啟動而一直沒有指派。

## 驗收與秘密設定

`scripts/prepare-secs-validation.py` 只建立 `cvd-secs-test` 設定，網站 21000、API 21080；秘密位於 `.runtime/secrets/secs/`。它產生 Broker 密碼雜湊與逐帳號 ACL：各 Gateway 只能發布自己的資料並接收自己的命令，Bridge 只能接收設備資料及發布控制，禁止匿名。

control key 與既有 recovery key 分離。Base64 包裝原始 JSON 後簽章，避免跨 Java／Go 的字串轉義改變內容。S7 配方下載也需有效控制簽章；只保存已核准的參數，回讀比對後才下套用指令。不得把隔離測試帳號或控制密鑰複製至正式環境。

## 維護與遷移

1. 完成隔離 750 片、單台斷線、命令重送、PE 補做、桌面手機、資源與保留策略驗收。記錄原始資料與測試命令。
2. 正式環境停止新控制及推進，保存全部設備 state、gateway spool、兩個資料庫完整備份及 Broker／Kafka／Redis／後端持久卷，保留舊映像和完整 Compose／秘密檔路徑。備份原檔不可公開。
3. 對帳每台晶圓位置、膜厚、recipeVersion、命令 ID、eventId、sequence、帳號雜湊及簽核處方。既有 signed recovery 原文不改写，舊 spool Pending／Done／Acked 直接沿用，只有新傳輸包裝改變；新 Bridge 收到旧 CVD-01 只可對應 01。
4. 未完成 Java inbox 繼續按原 ID 送 Kafka；設備／Gateway 未交付事件透過新 Bridge 重送，資料庫以原 eventId 去重。命令未知結果先沿原 commandId 查詢／重送，不另造命令。旧、新控制 publisher 不同時運作。
5. 啟動新鏈路，確認 Birth、十台品質及帳號／簽核／晶圓對帳；全部機台暫停等待操作。確認無正式 OPC UA 連線和舊 MQTT JSON 訂閱。
6. 若未開放新操作便失敗，停止新服務並以成套備份還原 DB、所有卷、Kafka／MQTT 状態及映像。若已接受新操作，先保存差異與對帳，不直接還原而丟失新資料。

備份必須先在副本實際還原。備份／還原所需空間不足時暫停切換；長期容量缺口須按量測結果明確報告，不得降低一秒採樣或 30 天保留。驗證期間的已完成隔離資料卷，只有在確定可刪除後才清除。

## 可執行工具

- `python3 scripts/prepare-secs-validation.py --production`：只產生 `.runtime/secs-production-compose.json` 與獨立正式控制密鑰／Broker 憑證，不啟動任何服務。沿用正式 recovery key、Leader 密碼來源和原卷名稱。
- `python3 scripts/secs-backup.py '<新的備份目錄>'`：進入維護、停止推進及控制，保存資料庫 dump、全部 Docker 持久卷、舊映像標籤與獨立回滾 Compose，完成後保持正式環境停止。
- `python3 scripts/secs-restore-rehearsal.py '<備份目錄>'`：驗證 SHA-256 後將全部卷還原到新 `cvd-secs-restore` 專案。Kafka 副本放在備份目錄的 `restore-kafka/`，其餘為隔離卷；不自動啟動。
- `python3 scripts/secs-state-migration.py '<原 state 目錄>' '<新 state 目錄>'`：檢查十台 ID 並只設定 `paused=true`。重跑必須與既有目的檔完全相同，否則拒絕。

備份／還原目的地若已存在，工具會拒絕覆寫。正式資料卷不會由這些工具刪除。外接磁碟副本僅供備份與演練，正式 Kafka 仍留在原 Docker 卷。


正式預設使用 `.env` 的 `COMPOSE_FILE=compose.yaml:compose.secs.yaml`。之後的 `secs-backup.py` 會讀取此實際啟用設定，連同 Bridge 卷與控制密鑰一起備份；回滾則明確指定備份內的 `compose.json`，不受目前 `.env` 影響。還原舊系統前必須停止新 Bridge 與所有應用，避免兩套 publisher 同時運作。
