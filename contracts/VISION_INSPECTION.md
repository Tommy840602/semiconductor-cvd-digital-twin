# 視覺檢查契約 v1

KLA 完成後建立 vision_runs（measurement_id + revision 唯一）與 map/surface 工作。cvd 原始結果不覆寫。啟用界線以資料庫 vision_config.enabled_at 為準，停用不刪界線或工作。

Kafka `vision.jobs.v1`：`{jobId}`。工作為持久 outbox，未領取每 15 秒重送，單批最多 20。

內部 `POST /api/vision/worker/jobs/{id}/claim` 使用獨立 Bearer secret，回傳 jobId、lease、kind、input。租約 30 秒，最多 3 次；逾時重領後舊 lease 的結果忽略。`GET .../{id}/input` 另帶 X-Vision-Lease，提供雜湊驗證的 PNG。執行期不接受 pickle。

Kafka `vision.results.v1`：`{jobId,lease,state,result}`。state 為 completed 或 failed；完成結果包含 inputHash、overlayHash、algorithmVersion、opencvVersion、parameters、kind、width、height、coordinates、quality、candidates、mixed、features、defects。圖片另存持久 volume，先原子寫入並 fsync，再送事件；Java 確認雜湊與結果合法後提交。工作 ID、lease 及狀態使重送不重複發布。

quality：pass（未檢出）、review（需人工查看）、uncertain（無法確定）；技術失敗不屬品質合格。defects：id、type、x/y、width/height、areaPx、sizePx，皆為影像像素，不可越界。sizeUm 僅在 input.calibration 有有限正數 µm/px 時存在。無全片轉換不建立物理全片座標。

input：hash、kind、width、height、mode（dataset-replay/synthetic）、datasetId、datasetVersion、sampleId、sourceUrl、license、attribution、coordinates、calibration，以及原圖 originalHash 或模擬 seed。標註只存在離線 evaluation-labels.json，不掛入執行服務；分析函式只接圖片、kind 與 calibration。

登入查詢：GET /api/vision/runs?measurementId=、GET /runs/{id}、GET /jobs/{id}/image/{original|overlay}、GET /alarms?equipmentId=。所有設備均沿用目前全廠角色授權；不存在新增跨租戶資料範圍。

Engineer/Leader：POST /runs/{id}/reanalyze（requestId UUID）以及 /acknowledge，要求登入及 CSRF。重分析沿用凍結輸入，另建版本；重複請求沿用已建 ID。兩工作均終止才彙整每版本一筆站內告警，不建立設備命令、簽核或 Email。

結果與衍生圖保留 30 天；進行中／未確認告警引用延長，原始公共素材為共用唯讀目錄。刪除衍生圖先查所有有效引用並留 24 小時寫入寬限。過期圖片 API 回傳 410。
