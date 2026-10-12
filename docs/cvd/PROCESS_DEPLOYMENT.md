# 製程模型部署與回退

## 驗證環境

使用 `cvd-process-test` 專案、獨立命名資料卷及網路，前端 `127.0.0.1:19000`、API `127.0.0.1:19080`。`.runtime/process-compose.json` 是由正式 compose 配置產生的本機驗收設定；不得改接正式資料卷。

先建置候選映像，執行 `process_queries.py`、`process_pipeline.py`、`process_history.py` 及完整批次／中斷回歸。完整批次需按真實每秒時鐘跑完，不以單元測試代替整條通訊鏈路。

## 升級前備份

本次備份目錄：`backups/cvd-process-20260916/`。

- `equipment-before.json`：升級前 API 狀態。
- `state-before.json`：Go 模型 v2 檢查點。
- `postgres.dump`：PostgreSQL custom-format 全資料庫備份。
- `timescaledb.dump`：TimescaleDB custom-format 全資料庫備份。

備份保存在本機並由 Git 忽略。完成驗收後，先確認沒有新的人工操作或批次，再記錄實際切換時間與映像 ID。如期間有新增批次，應於切換前重新備份。

## 部署

候選驗收通過後才更新 `cvd-twin-simulator`、`cvd-twin-backend`、`cvd-twin-frontend` 映像；保留升級前映像標籤。以 `docker compose up -d --no-build --pull never` 更新服務，資料卷保持原位。

modelVersion 2 載入後升為 3。尚未完成的 legacy PM 加工依原模型完成，新加工才使用 synthetic-tin-v1，不倒推膜厚。部署後核對既有故障、批次、晶圓位置及歷史筆數，依現有人工恢復流程操作，不自動解除故障。

前端有臨時 Tunnel 時，檢查 tunnel 程序及當前網址。只有確認仍有效的指定來源，才恢復 runtime nginx Origin 映射；以外網頁面確認 WebSocket 即時更新。不得把失效網址列為交付網址。

## 回退

先停止新增批次並保存當前檢查點、資料庫及映像 ID。可先回退前端或查詢服務；新增 JSON 欄位向後相容。新版進行中檢查點不得直接交給 v2 模擬器，應排空批次後回退，或使用支援 v3 的相容修正版。

從升級前备份恢復會丟失備份後產生的進度與履歷，必須明確選定恢復時間點。保留原資料卷，先在新的隔離資料庫測試還原，再切換。TimescaleDB 備份應還原至相同擴充版本，依該版本的 pre-restore／post-restore 流程處理；不得直接覆寫唯一資料庫副本。

## 重現隔離測試

標準 Dockerfile 可分別以 `docker build -t cvd-process-candidate-<服務> <服務>` 建置 simulator、gateway、backend、frontend。若 builder registry 拉取受阻，可在安裝 Go 1.24、Java 21、Maven、Node 的主機使用：

```sh
IMAGE_PREFIX=cvd-process-candidate ./scripts/build-local.sh
python3 scripts/prepare-process-validation.py
docker compose -f .runtime/process-compose.json up -d --no-build --pull never
CVD_URL=http://localhost:19080 python3 -u tests/integration/scenarios.py
python3 tests/integration/process_queries.py
python3 tests/integration/process_pipeline.py
python3 tests/integration/process_history.py
python3 tests/integration/process_fault_results.py
python3 tests/integration/process_event_order.py
python3 tests/integration/process_metrics.py
```

`process_recovery.py` 可在隔離正常批次進行中執行；需同樣指定 `CVD_URL=http://localhost:19080`，會依序停止／恢復測試服務並核對重啟檢查點。所有 `process_*.py` 的 Docker 查詢均固定連到 `cvd-process-test`。切勿將這些中斷測試套用到正在展示的 cvd-twin 服務。

測試結束可用 `docker compose -f .runtime/process-compose.json stop` 停止隔離環境，保留驗收資料；不使用 `down -v` 清除資料卷。
