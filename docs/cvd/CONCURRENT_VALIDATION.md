# 三批並行派片驗證

2026-09-16，變更 `cvd-concurrent-lots`。使用者決定取代整批 FIFO：LP1、LP2、LP3 同時運作，就緒批次輪流派片，共享四個 PM；LL 維持單片容量。

## 已驗證

- Go race 測試通過：三批 75 片、5／30 秒製程、四 PM 使用、就緒批次輪替、單批暫停不阻擋其他批次、全域故障停止派片、人工處置恢復。
- 檢查點：保存游標後重啟繼續保持相同進度；v3 升級 v4 保留晶圓，舊過量預載可回退 LP 解除堵塞。
- Vue 建置通過，瀏覽器顯示「多批輪流派片」、三個 LP 使用中及 LL 單片容量；批次資訊仍點方塊才顯示。PM 視窗批次資訊隨當前晶圓更新。

## 部署備份

`backups/cvd-concurrent-20260916/` 保存正式環境 v3 state、API 快照、PostgreSQL 與 TimescaleDB 完整 dump。舊 simulator/frontend 映像保留標籤 `pre-concurrent-20260916`。本次不修改資料庫 schema 或資料保留策略。

## 完成結果

- 隔離 Docker：VERIFY-daf1b3、VERIFY-0860a8、VERIFY-dd82e8，三批共 75 片全部成功回到原 LP，LL 含搬入預留始終最多一片，實際觀察三批同時占用 PM。快速端到端驗證使用 5 秒模擬製程；30 秒情境另由 Go 測試及正式展示驗證。
- 歷史核對：含本次 75 次，共 199 次完整製程的四階段採樣、五筆階段事件順序及最終膜厚均通過。
- 正式升級：v3 → v4 後全部既有 lots/wafer 資料與備份逐項相等，保持暫停，經恢復指令才繼續。
- 正式展示：啟動 DEMO-PARALLEL-9e594e（LP1）、DEMO-PARALLEL-08506c（LP2）、DEMO-PARALLEL-5ebd7b（LP3），每批 25 片、30 秒製程；API 確認三批同時在 PM，外網瀏覽器確認四 PM 加工、三 LP 使用中及即時連線。
- 外網地址沿用 https://enhanced-del-stylish-settled.trycloudflare.com/ 。隔離測試服務停止，資料卷保留。

