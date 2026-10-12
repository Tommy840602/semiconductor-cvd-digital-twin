# 製程數據擴充驗證（2026-09-16）

## 已完成

- Go `go test -race ./...` 通過，含原有 FIFO、單片 LL、故障處置測試及新增四階段／5秒配方、合成值界線、25片結果、完成樣本保存、重啟可重現、legacy 製程保持、LL 壓力曲線及重做清零／前次結果保存。
- Java Maven test／package 通過，新舊契約 fixture、指標／設備白名單與反向時間區間測試通過。
- Vue production build 通過，新增 PM／LL 多指標詳情、歷史查詢、null／缺口切線及分鐘聚合檢視。
- Go linux/arm64 二進位與 Java jar 已準備於本機 runtime 映像建置目錄。

## 實際容器驗證進度

Docker 經官方強制停止及重新啟動後恢復。已建置 cvd-process-candidate 前端、後端與模擬器映像，在 cvd-process-test 獨立網路／資料卷啟動九服務；既有 cvd-twin 展示服務也已恢復，歷史資料未刪除。

- `tests/integration/process_queries.py` 通過：秒級值、null 樣本數、[from,to)、分鐘 avg/min/max/count、run／wafer 隔離、非法指標與設備、30 天截止與 2000 點拒絕截斷。
- `tests/integration/process_pipeline.py` 通過：實際 OPC UA gateway 發布 MQTT 樣本與 TimescaleDB 保存資料逐欄一致（僅排除後端新增 ingestedAt），相同訊息重播兩次仍只有一列。一次活動製程樣本為 3757 bytes。
- 瀏覽器已顯示即時四階段、功能氣路、膜厚與階段標記；修正 PM 換片後詳情追蹤與關閉視窗停止查詢。
- 三批 75 片完整流程已通過，包含整批 FIFO、四 PM 並行、LL 單片守恆及返回原 LP 槽位。另對 75 次完成製程核對四階段秒級樣本、五個階段事件與最終膜厚，全部一致；故障重做／報廢測試也已通過：兩批各 25 片，重做批次 25 片成功、報廢批次 24 片成功及 1 片報廢，全數返回原 LP。
- Gateway、MQTT、Kafka、PostgreSQL、TimescaleDB 停止／恢復後持續落盤。測試曾在 TimescaleDB 尚未就緒時提早查詢，已修正等待邏輯並重測通過，見 `.runtime/process-recovery-final.log`。
- 模擬器由持久檢查點重新啟動，逐欄比較 processTelemetry 完全一致，等待三秒仍不前進；明確 resume 後繼續，事件無重複。
- 390×844 手機尺寸驗證 PM／LL 詳情、膜厚指標、日期輸入、空區間、分鐘聚合表與關閉後隱藏 Lot。
- 在 723 筆驗收樣本上查詢最近 30 天：分鐘聚合 41 點，10 次請求中位 22.73ms、最大 36.21ms。平均 JSON 4225 bytes、最大 6103 bytes；這不是 30 天全量負載測試。

## 驗收完成

所有 17 項任務已完成，完整批次、故障處置、通訊恢復、手機畫面、查詢與展示部署均已驗證。

若臨時 Cloudflare Tunnel 仍在使用，更新 frontend 容器後須恢復 `.runtime/tunnel/nginx.conf` 的指定來源映射並測試 WebSocket；不可默認舊臨時網址仍有效。不得為通過測試而清除展示資料卷或現有故障狀態。

## 既有展示服務恢復

發現正式後端 MQTT persistence 中 `s-21818.msg` 為 46 bytes 全零檔案，後端因此持續無法連線。已先備份至 `backups/cvd-process-20260916/mqtt-persistence/`，再將原檔移至容器資料卷 `/data/mqtt-quarantine-20260916/` 保留；未刪除資料卷。後端重新啟動後，外網 API sourceTime 恢復至當前時間。當時所有舊批次均已結案，模擬器維持重啟後暫停狀態。

## 完整回歸結果

`scenarios.py` 全部通過，共三批正常 75 片，加上重做／報廢兩批 50 片。最終 124 片成功、1 片報廢。`process_history.py` 核對全部 124 次完成加工之四階段樣本、五個階段事件、最終膜厚均一致。`process_fault_results.py` 通過：兩次未完成結果均為 not-evaluated，重做用新的 run，報廢無成功加工結果，run 無重複結果。

短配方同秒事件排序回歸已在舊候選查詢重現：UUID 排序會把 complete 排在 purge 之前；修正改依 source sequence 排序，另以 `process_event_order.py` 驗證。

## 展示部署完成

正式九服務已更新並正常運作；模型由 v2 升級至 v3，部署前批次 ID 清單完全保留。短配方同秒事件排序修正已於實際 PostgreSQL/API 通過，查詢回歸亦再度通過。

已啟動 `DEMO-PROCESS-59082b`（LP1／LL1，25 片）。這批沿用原先 5 秒配方；進行中調整至 30 秒的要求被原有安全規則拒絕，沒有強制繞過。外網頁面已確認「即時連線」、四階段標記與 10.05 nm 等合成結果，包含同秒吹掃→完成正確次序。

展示網址： https://enhanced-del-stylish-settled.trycloudflare.com/ 。此為仍在執行的臨時 Tunnel。nginx 指定來源映射已恢復並通過設定檢查。隔離測試服務於驗收後停止，資料卷保留；正式展示與 Tunnel 繼續運作。

恢復 MQTT 後的最新部署前備份位於 `backups/cvd-process-20260916/pre-deploy/`；升級前映像保留 `pre-process-20260916` 標籤。未提交 Git、未歸檔 OpenSpec。
