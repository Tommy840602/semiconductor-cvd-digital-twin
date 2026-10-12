# 多國語言介面

支援繁體中文（zh-TW）、English（en）、日本語（ja）。登入頁與登入後右上角都有語言選單，立即套用，不重新載入頁面。設備 ID、LP／LL／PM、EE／PE、物理單位、recipe 名稱與人員輸入保持原文。

## 偏好與權限

未登入時使用 `localStorage['cvd.locale']`，無設定預設繁中。若本次在登入頁手動選擇語言，登入成功後會先保存至帳號並延續到主頁。未手動選擇或重新整理時，先讀取帳號偏好，再顯示主畫面；帳號設定優先於瀏覽器既存設定。登出保留瀏覽器語言。不同裝置在下次登入或重新整理時取得最新偏好，不做跨裝置推播。

`GET /api/auth/preferences` 回傳 `{locale}`；`PATCH /api/auth/preferences` 僅接受 `{locale: 'zh-TW'|'en'|'ja'}`。Operator、Engineer、Leader 都只能修改自己；沿用登入 Session 與 CSRF。拒絕額外欄位和未知語言。保存失敗保留舊語言並顯示提示。locale 不增加 auth_version／email_version，也不使現有 Session 或待寄通知失效。

切換語言只更新文字與顯示格式。固定的導覽、頁籤、狀態代碼不改，保留機台、PM、篩選條件、recipe／處方草稿，以及 3D 相機、人物位置、模式。3D 走道文字更新會釋放舊 texture／material。語言操作不發送加工控制指令。

## 訊息、日期和數字

`frontend/src/i18n.js` 使用鎖定版本 vue-i18n 11.4.2。`locales/interface.js` 的固定 ID 對應三語文字；修改文案保留 ID。已知 key 缺翻譯回退繁中；未知 key 顯示通用提示，不暴露內部 key。Vue 插值採文字輸出，不使用 v-html。

日期、數字依語言格式化；時間固定 Asia/Taipei，標示 UTC+08:00。datetime-local 查詢值也以台灣時間解讀，不受瀏覽器所在時區影響。製程秒數、物理值、精度與 API 數值不做單位換算。

`contracts/system-messages.json` 定義固定系統訊息代碼與已知舊訊息對照。API 錯誤新增 code／params，保留 message；模擬器的指令結果及系統產生的補做原因新增 reasonCode，保留 reason 與原事件 envelope。未知外部／歷史訊息標示「原始訊息」。人員填寫的原因和排錯紀錄不依內容猜測翻譯；即使恰好等於系統字串，也維持人員原文。

## 郵件

寄送前重新確認原捕捉帳號的部門、角色、地址和版本資格，只從仍符合資格的帳號讀取最新 locale。同一地址只有一筆工作；若同地址帳號語言不同，使用繁中，管理畫面顯示 shared-address-conflict 的原因。

首次實際嘗試前，在取得工作鎖的交易內保存 delivery_locale、locale_reason、template_version、rendered_subject、rendered_body。後續自動／人工重試使用此快照，即使帳號語言或網站網址已變更也不重新渲染。accepted 不重寄。舊未寄工作在首次嘗試前建立快照；舊歷史資料不回填改寫，未知送達結果仍需人工明確確認重試。

維持既有 `EMAIL_ENABLED` 與 mail_connection.enabled 設定。測試使用隔離 PostgreSQL 與本機假 Graph，不能為了驗證語言而開啟正式寄信。

## 驗證與重跑

- `npm --prefix frontend test`：三語 key／參數、回退、所有文案編譯、原文、台灣午夜、數字與待顯示訊息。
- `cd backend && mvn test`：偏好權限／CSRF與既有功能測試。
- 使用明確的拋棄式資料庫 `jdbc:postgresql://localhost:15439/email_test`，設定 `CVD_EMAIL_TEST_DB` 後執行 Maven，才會啟用 EmailDatabaseTest。此測試會刪除該測試資料庫 public schema，禁止指向正式庫。
- `cd simulator && go test -race ./...`；`cd protocol && go test ./...`。使用 Go 1.26。
- 啟動 Vite 18003，設定 `PLAYWRIGHT_MODULE` 為可用 Playwright 路徑，執行 `node tests/integration/multilingual_ui.cjs`。此瀏覽器測試攔截全部 API，使用合成資料、不登入正式帳號、不執行設備控制。驗證三語、桌面／手機、Enter／Esc、草稿、3D 資源及保存失敗。證據輸出 `.runtime/i18n-*.png` 與 `.runtime/i18n-fixture-result.json`。

## 部署與回退

本次 migration 僅新增 app_users.locale、recovery_requests.reason_code 與 notification_deliveries 語言／模板快照欄位。重複啟動安全；不刪除舊欄位或重算歷史。部署前備份 PostgreSQL、記錄帳號／recipe／簽核雜湊與寄信開關，再保留三個舊映像。

2026-09-30 備份位於 `.runtime/backups/i18n-20260930/`，正式備份為 `postgres-base/` 下的 base.tar、pg_wal.tar 與 backup_manifest，使用 PostgreSQL 16 pg_basebackup 一致性備份，並以 pg_verifybackup 驗證。另有 10 台 simulator 狀態 JSON。較慢的邏輯匯出已停止並移除未完成檔案，還原請使用 postgres-base 實體備份。舊映像：`cvd-before-i18n-backend-20260930`、`cvd-before-i18n-frontend-20260930`、`cvd-before-i18n-simulator-20260930`。回退時將三個舊映像標回 `cvd-secs-backend`、`cvd-secs-frontend`、`cvd-secs-simulator`，依序重建對應服務並驗證十台資料恢復。保留新增欄位即可相容舊程式，不必刪資料或還原整庫。郵件在此次部署維持停用；若未來啟用後再回退，先暫停寄送並檢查 sending／unknown 工作，避免舊程式重新渲染重試內容。
