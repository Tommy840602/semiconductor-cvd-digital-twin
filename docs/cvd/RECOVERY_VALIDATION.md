# 單片補做簽核驗收（2026-09-16）

變更 `cvd-wafer-recovery-approval`，前端 Vue 3、Spring Java 21、Go 模擬器。正式採用 checkpoint modelVersion 5；未修改 gateway 或設備布局配色。

## 結果

- Go 全套測試通過：15 種非空步驟組合、1／10／3600 秒、非法總量、五點膜厚保留、無沉積零增量、沉積越久增量越大、原始速率多輪不變、簽章／錯 PM／錯基線拒絕、取消 tombstone、重送、故障與重啟。
- Spring 11 個測試通過，包括 18 組部門／角色授權組合；Vue 正式 build、Compose config 與 OpenSpec validate 通過。
- 全新隔離專案 `cvd-recovery-acceptance` 執行 `wafer_recovery.py` 完整通過；保留前一測試專案的資料，未清除其他專案 volume。
- `recovery_resilience.py` 通過核准/outbox 交易回滾、通訊中斷恢復、1 秒製程實際開始紀錄、ACK 遺失重送、已發布取消設備確認、晚到指令禁止執行、開始後修改／撤回拒絕、backend 重啟、核准／撤回競爭、活動歷史保護、終態整鏈過期及報廢失效。
- RBAC 整合回歸通過登入、CSRF、三角色命令矩陣、帳號建立、操作稽核、WebSocket 登出／過期、限流及舊歷史。舊 rework 與偽造補做指令被拒絕。
- 瀏覽器驗證 EE／PE 對應操作、退回修訂、秒數 1→2 的版本比較、2.5 秒禁止儲存、核准即執行提示、狀態自動更新、累積膜厚與簽核時間線。Leader 指派既有 Engineer 部門，角色未改動；正式帳號部門不自動指定。

## 膜厚證據

完整原始鏈見 [recovery-approved-flow.json](evidence/recovery-approved-flow.json)。

| 指標 | 數值 |
|---|---:|
| 補做前平均膜厚 | 1.012580431 nm |
| 所選步驟 | 沉積 3 秒、吹掃 1 秒 |
| 精確已執行時間 | 4 秒 |
| 追加膜厚 | 1.511671337 nm |
| 累積膜厚 | 2.524251768 nm |
| 品質 | pass，回原 LL／原 LP 原槽 |

來源目標及補做目標不同；未按新目標或新秒數重新正規化。合格後派片仍暫停。另驗證僅預熱／吹掃為零增量、目標 100 nm 的不合格留置，以及再故障保存新增膜厚。

ACK 測試曾發現舊 command-result 已移除後無新回報；已修正為重送只重新回覆持久化原結果，並在全新環境重新通過，未再次加工。測試也區分 backend 健康與新鮮設備快照，等待恢復後才可核准。

## 正式部署

備份：`backups/cvd-wafer-recovery-20260916/`，含 PostgreSQL／TimescaleDB dump、state.json、gateway spool、帳號雜湊摘要及原歷史筆數。三個舊映像保留：

- `cvd-twin-backend:pre-recovery-20260916`
- `cvd-twin-frontend:pre-recovery-20260916`
- `cvd-twin-simulator:pre-recovery-20260916`

部署後各服務正常，維持既有 17 批、425 片、3 個帳號。逐片核對槽位、位置、結果、attempts、批次暫停狀態；配方及密碼雜湊相同，歷史筆數未減少。既有三帳號 department 均 null；沒有建立正式測試帳號。全域時鐘已恢復至部署前運行狀態。機讀結果見 [recovery-production.json](evidence/recovery-production.json)。

外網仍使用原臨時展示網址。升级前登入會失效，請重新登入，再由 Leader 在「帳號管理」建立／指定不同的 EE、PE 工程師。簽章金鑰保留於未追蹤秘密檔案，回退限制見 [RECOVERY_DEPLOYMENT.md](RECOVERY_DEPLOYMENT.md)。
