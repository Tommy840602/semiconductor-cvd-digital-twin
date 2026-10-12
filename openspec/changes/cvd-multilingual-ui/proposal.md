## Why

目前 CVD 網站的文字直接分散在 Vue 元件、3D 提示與後端訊息中，使用者無法選擇語言，日期格式也不一致。加入繁體中文、英文與日文，讓不同語言的使用者使用同一套加工、簽核與通知流程。

## What Changes

- 登入頁與網站右上角提供「繁體中文 / English / 日本語」選單，即時切換並保留機台、頁面、3D 視角及未提交表單。
- 登入前保存瀏覽器選擇；登入後保存自己的帳號偏好，供跨裝置登入與 Email 使用。
- 翻譯全站導覽、2D／3D 提示、製程、歷史、統計、保養、異常處置、簽核、帳號與通知管理，以及系統產生的告警／錯誤。
- 以固定頁面／狀態／訊息代碼分離邏輯與顯示；使用者備註、設備編碼、recipe 名稱及物理單位保留原值。
- 日期數字顯示跟隨語言，廠區時間與統計分界仍採 Asia/Taipei；翻譯缺漏回退繁中。
- Email 依收件帳號語言產生，維持既有路由、去重、簽核權限與寄送停用設定。

## Capabilities

### New Capabilities

- `multilingual-interface`: 三語介面、偏好保存、固定代碼、日期格式與通知語言。

### Modified Capabilities

無已發布的主規格需修改；既有功能要求保留，通知語言與去重的整合規則寫入本次新增規格。

## Impact

前端 App.vue、各功能 Panel、EquipmentTopology、CleanroomOverview／cleanroom 模組、statusGlow、main.js 與 index.html；後端帳號偏好 API、系統訊息回應及通知模板／寄送紀錄。新增相容的 PostgreSQL 欄位與翻譯資源，沿用 Vue 3 JavaScript、Java 21／Spring 與 Docker。無需改變 SECS/GEM、Sparkplug B 或 Kafka 協定欄位語意，不自動啟用 Outlook 或實寄。
