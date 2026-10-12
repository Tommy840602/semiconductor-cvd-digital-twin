# cvd-multilingual-ui 驗證紀錄

日期：2026-09-30。前端使用 Vue 3／vue-i18n 11.4.2；後端 Java 21。此紀錄對應 OpenSpec `cvd-multilingual-ui`。

## 已完成驗證

| 範圍 | 結果／證據 |
|---|---|
| 前端單元測試 | 24 項通過；三語 key／插值一致、字典編譯、缺 key 回退、固定台灣午夜、原文及持續顯示訊息 |
| Java 與隔離資料庫 | 41 項通過，0 skipped；自己的偏好、三角色、CSRF、未知 locale、額外 userId 拒絕、不改 auth／email version、migration 重跑 |
| 假 Graph | 寄送前最新偏好、共用地址衝突、單地址去重、retry／manual retry 保留內容、accepted 不重送 |
| Go 模擬器 | `go test -race ./...` 通過；既有派片、保養、Dummy、排錯、留片與放行回歸；PE 人員原因即使等於固定系統文字也不標示系統代碼 |
| SECS/GEM／Sparkplug | protocol 三套件測試通過；新增欄位不變更 envelope 或控制語意 |
| 桌面／手機 | Chromium 1440×1100、390×844；繁中、英文、日文，無頁面錯誤、無橫向整頁溢出 |
| 3D | 手動模式及人物位置保留；Enter 進入／Esc 返回正常；語言切換前後 resources 都為 55 materials、12 textures、55 geometries |
| 狀態與草稿 | PM2、recipe 19 秒草稿、含 `<b>` 的中文排錯文字保留；設備、統計頁籤與資料不重設；語言保存失敗仍維持舊語言 |
| 操作邊界 | 測試全部設備控制寫入數 0；Session activity 與自己的 locale 不屬於設備控制 |

可重跑的合成瀏覽器測試：`tests/integration/multilingual_ui.cjs`。本機證據：`.runtime/i18n-fixture-result.json`、`.runtime/i18n-front-tests.log`、`.runtime/i18n-backend-tests.log`、`.runtime/i18n-go-tests.log`、`.runtime/i18n-protocol-tests.log`。截圖位於 `.runtime/i18n-full-process-*.png`、`.runtime/i18n-{utilization,timeline,distribution,production}-*.png`、`.runtime/i18n-mobile-*.png`。

## 規格對照

語言資源／偏好由 i18n.js、LanguageSelect.vue、PreferencesApi 實現。頁面仍沿用既有元件實例；3D 只更新文字 sprite。系統訊息由固定 catalog、code／params 與來源明確的 reasonCode 處理，人員欄位不做字串猜測。日期固定 Asia/Taipei，查詢秒數和物理單位維持原值。EmailTemplates 與 NotificationWorker 在首次寄送 claim 交易內保存語言與內容。

資料與部署操作依 [MULTILINGUAL.md](MULTILINGUAL.md)。正式環境不執行故障注入、批次控制或實際寄信測試。

補充：Gateway 與 MQTT–Kafka Bridge 測試均通過。原生必填／格式／數值範圍驗證訊息也隨語言更新，瀏覽器測試確認英文必填提示切換日文後立即更新。

## 正式 Docker 部署結果

2026-09-30 已部署 cvd-i18n-backend／frontend／simulator；部署使用既有 cvd-secs 映像標籤。SECS/GEM 重連期間曾短暫過期，重啟 Bridge／Gateway 並完成補送後，十台資料延遲恢復約 1–2 秒，後端與所有通訊／模擬器服務 healthy。

正式瀏覽器驗證通過：匿名日文偏好在登入時由帳號覆蓋、英文偏好在另一裝置登入生效、另一裝置改日文後重新整理取得新值、登出保留語言；缺 CSRF=403、非法語言及額外 userId=400、未登入讀偏好=401。無瀏覽器錯誤、無加工控制寫入，測試後還原原帳號語言。臨時 HTTPS 網址回應 200。

部署前後資料雜湊相同：4 帳號、10 recipe 版本、2 處置單、3 處方版本、1 結果確認、1 整機放行。寄信部署開關與連線開關皆 false、連線 disconnected、delivery 0；未實寄郵件。PostgreSQL 實體備份已通過 pg_verifybackup，並保存在工作區；十台模擬器狀態檔也已備份。

正式證據：`.runtime/i18n-production-proof.json`、`.runtime/i18n-production-en.png`、`.runtime/i18n-production-mobile-ja.png`、`.runtime/i18n-backup-verify.log`、`.runtime/backups/i18n-20260930/preservation-{before,after}.txt`。

## 登入頁語言延續修正（2026-09-30）

`tests/integration/login_locale.cjs` 以攔截 API 的瀏覽器測試驗證：登入頁明確選英文／日文後保存至帳號、主頁與重新整理保持選擇；未操作選單時仍以帳號偏好優先。前端 24 項測試及正式建置通過。
