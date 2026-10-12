# AI 報告與 Excel 匯出

登入後從側欄「AI 報告」選擇全部十台／單台、今日／24 小時／7 天／30 天及繁中／英文／日文，按「生成 AI 報告」。完成後可查看摘要、圖表、統計明細與資料品質，下載 .xlsx。第一版只顯示自己建立的報告，Operator、Engineer、Leader 皆可使用。

## OpenAI 設定

此功能呼叫 OpenAI API，不能直接使用網站 ChatGPT 的登入憑證。後端必須設定 API 金鑰及帳號可用、支援 Responses API 結構化輸出的模型。

建議用 Docker 持久化 volume 內的金鑰檔案。先在主機建立內容只有金鑰的檔案（不要提交 Git），再執行：

```sh
docker compose exec -T backend mkdir -p /data/secrets
docker compose cp /安全路徑/openai-api-key backend:/data/secrets/openai-api-key
docker compose exec -T backend chmod 600 /data/secrets/openai-api-key
```

在本機 `.env` 設定：

```dotenv
OPENAI_API_KEY_FILE=/data/secrets/openai-api-key
CVD_REPORT_MODEL=你的可用模型ID
```

然後只重建後端容器載入設定：

```sh
docker compose up -d --no-deps backend
```

也支援 `OPENAI_API_KEY` 環境變數，但不要將金鑰填入前端或對話、寫入版本控制或貼出容器環境內容。當金鑰與模型未完整設定時，「生成」停用並顯示設定不足；不生成假的 AI 摘要。設定 flag 只檢查存在性，實際憑證／模型可用性由 API 呼叫確認。API 設定錯誤會顯示失敗。

## 資料與圖表

- 固定 Asia/Taipei 的 [start,end) 範圍；完成報告使用不可變快照，晚到資料只影響重新生成的報告。
- 正式片數依原統計的結案規則，Dummy 獨立列出。完成片數不等於正式品質良率。
- KLA 按量測完成時間採截止前最新有效 source-key 版本；OpenCV 按檢查完成時間採 measurement-id 最新有效版本，map／surface 兩種檢查獨立列出。各模組分母不同，不將其比例混成生產良率。
- PHM 趨勢採歷史抽氣事件觀測，標示解析度，不是每秒連續監測。末端健康／剩餘模擬次數取區間內最近歷史 telemetry；缺值不填現在值。抽氣達標率分母為通過＋失敗，中斷獨立列出。
- 保留未知時間、產出完整性、未完成量測／檢查、資料來源與擷取時間。待處理件數依截止時未完成證據推定，來源到期清理可能低估。跨 PostgreSQL 與 TimescaleDB 擷取不是全域原子交易。
- AI 使用彙整指標與固定資料集引用，程式驗證引用、數字占位符與允許圖表類型。拒絕不合法輸出，摘要仍需要人工判讀，不能視為設備故障根因診斷。
- 甘特混合區段在 Excel 保留各狀態秒數；KLA 趨勢按時間桶平均膜厚，PHM 按事件數加權平均健康；視覺分類圖為候選類別出現次數，多候選可重複出現在不同類別，不是已確認缺陷良率。
- 不包含 Cp／Cpk，不改 recipe、派片、告警確認或簽核。

Excel 八分頁為摘要、設備統計、狀態時間軸、KLA 量測、視覺缺陷、真空 PHM、資料品質、報告資訊。圖表使用嵌入圖片，來源數值可再分析；第一版不提供可直接編輯的 Excel 原生圖表。預覽只顯示前 100 筆量測／檢查，Excel 含完整明細。

## 限制與維運

報告保存在原 `backend-data` volume 的 `/data/reports/<UUID>/`；PostgreSQL `ai_reports` 保存建立者、範圍、狀態、固定快照、摘要與雜湊。檔案保存 30 天，到期下載回傳 410；到期快照與檔案清除不影響來源資料。

同時最多 2 個背景工作，每人最多 1 個未完成工作；冪等 requestId 不重複呼叫。API 暫時錯誤／連線逾時最多 2 次重試（總共 3 次）。重啟將未完成工作標示 interrupted，使用者手動重試。最多 100,000 明細列、50 MB Excel；超限失敗，不靜默刪除資料。前端只在報告頁可見且有工作進行中時每 5 秒輪詢。

API：POST／GET `/api/reports`、GET `/api/reports/config`、GET `/api/reports/{id}`、GET `/api/reports/{id}/download`、GET `/api/reports/{id}/charts/{dataset}`。沿用 Redis session 與 CSRF；所有讀取／下載須驗證建立者，檔案路徑不接受外部任意路徑。

Docker 圖表使用隨程式附帶的 Noto Sans CJK 字型與 SIL 授權，不依賴執行時下載。

部署只更新前端與後端，不重啟模擬器或刪除資料 volume。回滾使用部署前的前端／後端 image，保留 `ai_reports` 與 `/data/reports`。需要停止背景工作時停止後端，不能刪除整個 backend-data（還有其他持久化內容）。

## 驗證

`ReportTest` 驗證結構化請求、API 失敗／拒絕／格式／逾時、數值與引用、三語工作簿、公式樣式文字、圖表圖片與檔案限制。`ReportDatabaseTest` 僅在 `CVD_REPORT_TEST_DB=jdbc:postgresql://localhost:15441/report_test` 時執行，會重建該隔離資料庫的 public schema，禁止指向正式資料庫。

`ReportProductionReadCheck` 必須明確設定 `CVD_REPORT_READONLY_PROBE=true`，使用 localhost 15442／15443 的暫時代理與 server-enforced read-only PostgreSQL session，只讀正式來源，沒有 migrate、簽核或控制寫入。十台 30 天實測：收集約 15.8 秒、快照約 2 MB、3,844 明細列、使用 heap 約 81 MB；此值取決於當時資料量，不是效能承諾。

瀏覽器整合測試 `tests/integration/ai_report_ui.cjs` 在本地開發預覽 18009 上以隔離回應驗證完整預覽／下載、三語、日夜、手機與離頁停止輪詢；不把模擬 AI 回應部署至正式服務。

本次環境未設定 OpenAI API 金鑰／模型，真實外部 API 報告尚未驗證。測試 stub 僅驗證協定與端到端程式流程，不能宣稱實際 AI 模型輸出品質已驗證。
