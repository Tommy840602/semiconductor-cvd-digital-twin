# 單片補做簽核部署與操作

使用 Leader「帳號管理」建立兩個不同的 Engineer 帳號，分別指定 EE、PE。既有帳號不預設部門；Leader 可指派自己的部門，但仍不可核准自己提出的處置。調整部門後重新登入，角色保持不變。

1. 異常片仍在 PM 時，EE 開啟「單片補做簽核」，選擇 PM、填排錯說明、目標膜厚及各勾選步驟的整數秒數，儲存後送審。
2. PE 查看原膜厚、預估增量、前後版本差異，可退回修改，或按「核准並執行」。核准後無額外 EE 執行按鈕。
3. 有故障／通訊中斷／全域時鐘暫停時顯示等待原因。工程人員解除故障，重啟後使用「確認恢復補做時鐘」，條件就緒便自動開始。此操作保留批次派片暫停。
4. 合格才經原 LL 回原 LP 原槽，再由工程人員人工恢復批次。不合格或再故障保留膜厚、留在 PM，需新一輪 EE／PE 處置；舊完整重做已停用。報廢仍沿用原工程權限。
5. 已發布撤回會先等待設備確認；若已開始，不能改秒數或撤回。仍可使用故障處置停止模擬製程。

## 安裝／升級

先備份 PostgreSQL、TimescaleDB、模擬 checkpoint 與舊映像。新增簽核資料表與 nullable department，不覆寫既有帳號、密碼、配方、批次或歷史。authVersion=1 使升級前舊 session 失效。

```sh
python3 scripts/init-recovery-key.py
docker compose up -d --build
```

簽章秘密預設 `.runtime/secrets/recovery-key`，0600、不納入版本控制。Compose 同時掛載 backend／simulator。使用 `CVD_RECOVERY_KEY_FILE` 自訂主機檔案位置；既有金鑰不可任意重建，否則尚未交付的核准指令簽章無法驗證。初始 Leader 密碼另依 RBAC 文件建立。

資料庫、後端、設備分開使用各自的持久化存放區。只更新 backend／simulator／frontend 映像；gateway 與通訊 topic 維持相容。模擬器升級至 checkpoint modelVersion 5，保留五點膜厚和既有步驟；缺少可還原樣本／速率的舊异常不捏造資料，阻止新提案。

## 驗收

只在隔離 Docker 專案執行。`wafer_recovery.py` 預期初始無活動批次，會建立 EE／PE／Operator 測試帳號及三批晶圓；resilience 測試重啟／暫停隔離服務，不指向正式環境。

```sh
CVD_COMPOSE=.runtime/recovery-acceptance-compose.json python3 tests/integration/wafer_recovery.py
CVD_COMPOSE=.runtime/recovery-acceptance-compose.json python3 tests/integration/recovery_resilience.py
```

網站/API 預設 localhost:19000/19080；測試秘密位於 `.runtime/secrets/test-leader-password`。測試記錄及膜厚證據見 RECOVERY_VALIDATION.md。

## 回退

有 approved_waiting、cancelling 或 executing 時不得直接降版到不理解補做的設備。先保留現版與 checkpoint，完成／確認取消在途意圖；必要時採相容修復映像。不得還原舊 checkpoint 抹去已消費 executionId，否則可能重複生長。保留部門與簽核表，不以舊 dump 覆蓋新歷史。

本次升級備份及回退映像標籤記錄在 RECOVERY_VALIDATION.md。回退舊後端還可能重新開放 rework，故不適合保留公開展示服務的直接降版；若必須降版，先關閉外網展示與控制入口。
