# OpenCV 視覺檢查

KLA 新完成量測會自動建立分布與表面兩項檢查。晶圓履歷的 KLA 詳情下方，可切換原圖、標記圖、缺陷列表與檢查版本；Engineer／Leader 可重新分析同一原圖及確認告警。告警頁亦提供未確認視覺告警入口。

## 資料與限制

來源及查核紀錄見 [datasets.json](vision/datasets.json)，授權見 [ATTRIBUTION.md](../../vision/catalog/ATTRIBUTION.md)。每次送檢由 180 張 Kaggle 分布樣本與 40 張合成表面樣本中，按量測 ID 確定選取；每組一張公開 map、一張模擬 surface，執行比例為 50%／50%。重分析凍結同一張圖。不是目前 CVD 晶圓的實測，不以此證明製程品質。

公開樣本是 JPEG 壓縮的 32×32 分布圖，不含原始 die 網格與比例尺；顯示異常像素區域，不宣稱壞 die 數。合成 surface 是簡化背景及形狀，沒有真實 SEM 光學效應。膜厚由原 KLA 模擬提供。

[評估報告](vision/evaluation.json) 分列公開與合成，採來源 ID 固定分組。規則基線對公開資料誤判明顯，未做真實 TiN 品質認證；合成資料的高分不能推論真實泛化能力。uncertain 是無法分類，failed 是技術失敗，兩者都不是合格。不自動停機、報廢、簽核或 Email。

## 部署

1. 備份 PostgreSQL、.env 及現有 image ID。增加 `compose.vision.yaml` 至根目錄 COMPOSE_FILE；現有 secs/metrology 配置保持。
2. 以安全亂數建立 `.runtime/secrets/vision-token`（至少 32 字元、0600），backend 與 worker 共用。不要使用登入密碼作為此 secret。
3. 先保持 `VISION_ENABLED=false` 建置 `docker compose build backend frontend vision`；catalog 為唯讀綁定，vision-images 為持久 volume。worker 無對外 port。
4. 設 `VISION_ENABLED=true` 後從根目錄執行 `docker compose up -d backend frontend vision`。初次啟用界線保存在 vision_config；只有之後 KLA 完成才送檢，無舊資料補造。
5. 查看 `/actuator/health`、worker 健康與 `vision_jobs` 狀態。原圖讀取需要網站登入；內部領取介面使用獨立 Bearer secret。

圖片每張上限 2 MB／1024×1024，worker 256 MB、0.5 CPU，常駐隔離分析程序單次逾時 20 秒，租約 30 秒，最多 3 次。工作透過資料庫 outbox 重送，不丟棄排隊資料；磁碟須監控，實際容量證據見驗收紀錄。固定目錄原始素材去重，衍生結果保留 30 天及未結案告警例外。

## 匯入與恢復

只接受有界 ZIP 內的 JPG／PNG。使用 `vision/import_catalog.py`，在 `--network none --read-only --memory=256m --cap-drop ALL --security-opt no-new-privileges` 的無憑證容器中匯入，僅掛來源 ZIP 與輸出目錄。執行服務不載入 pickle，原始大型 PKL 未匯入。evaluation-labels.json 只作離線評估，不掛入 worker。

worker 重啟後重新加入 Kafka；未完成租約到期重送。技術重試沿用 ID；手動重分析建立新版本。停用 `VISION_ENABLED` 可停止新派送與領取，保留 DB／volume；回退後重新啟用可續作，勿執行刪除 volume 的指令。

## 驗證指令

- Python：`python -m unittest discover -s vision/tests -v`
- 分類評估：`python vision/evaluate.py vision/catalog vision/evaluation-labels.json`
- Java：`mvn -f backend/pom.xml test`；真實資料庫測試需指向專用測試 DB，測試會清空其 public schema。
- 前端：`npm --prefix frontend test`、`npm --prefix frontend run build`
- 隔離端到端：`tests/integration/vision_pipeline.py`，僅對 cvd-vision-proof 專案注入十台 KLA 完成事件，勿對正式資料庫造資料。

## 驗收紀錄

2026-10-02 隔離測試：十台來源、20 項分析全部完成，0.5 CPU 工作服務下端到端 28.86 秒，沒有新增設備控制指令；這是當次主機負載下的觀察值，不是延遲保證。原始素材目錄約 4.1 MB，最初十組衍生圖約 712 KB，圖片按內容雜湊去重。後端與 worker 重啟後，原排隊兩工作完成；[驗收資料](vision/validation.json)。

備份範圍為 PostgreSQL 結構與帳號／配方／KLA 關鍵表，已在專用 PostgreSQL 還原並核對帳號與 recipe_heads 雜湊及配方外鍵。全歷史／inbox 匯出因時間成本中止，不宣稱完成全庫還原演練；本次不修改或刪除既有歷史資料。

正式部署驗收（2026-10-02）：使用者同意後，在 01 號新增 25 片展示批次並送出 start／resume 兩項 CVD 指令。25 片均成功加工、回槽並完成 KLA，每片 49 點量測；25 組 OpenCV 共 50 項（分布／表面）全部完成，無技術失敗。21 組產生站內品質告警；來源仍為公開資料重播與模擬影像，不能當成實際晶圓品質結論。原有 47 批均保留且仍 complete，其餘九台保持暫停。結果 API、原圖／標記圖、兩種檢查 Tab、未登入影像拒絕、桌面與手機畫面均驗證通過，無 JavaScript 錯誤。

Bridge 修正：`bridge/outbox.go` 使用不含 payload 的排序索引，重啟時由持久檔案重建，每批最多讀取 250 筆內容。跨設備按來源時間排序，避免早編號設備長期占滿批次；仍在 Kafka 與必要的 Gateway 確認成功後才刪除檔案。已通過 Go race 測試，涵蓋重啟、重試、較舊設備資料優先及傳送期間檔案替換不誤刪。積壓由約 14,500 筆／1.6 GB 追完至零，驗收時十台設備最大來源時間延遲低於 1 秒；31 項服務運行，所有健康探針均通過。

部署續作演練範圍：短暫撤下 vision worker，保留已有 36 項完成結果及撤下期間排隊的 4 項工作。恢復 worker 後 4 項全數完成，原有與排隊工作輸入雜湊一致。此演練是工作服務撤下／恢復，未切回舊版 backend／frontend image；該回退 image 已保留，操作方式按上列部署步驟。完整歷史資料庫還原亦未演練，驗收不宣稱這兩項已執行。

最新驗收證據見 [validation.json](vision/validation.json) 的 productionDeployment；原始本機證據為 `.runtime/vision-demo-final-proof.json`、`.runtime/vision-demo-ui-proof.json`、`.runtime/vision-demo-rollback-proof.json` 與 `.runtime/vision-demo-detail.png`／`.runtime/vision-demo-mobile.png`。
