## Why

現有 CVD 模擬已有晶圓加工結果及五點合成膜厚，但缺乏加工後獨立量測流程與可追溯 wafer map。新增共用模擬 KLA 量測站，讓十台 CVD 每片完成晶圓都有膜厚與缺陷圖，可於網站判讀異常。

## What Changes

- 一台 Go 模擬 KLA 站接收十台 CVD 已完成且回到 LP 的晶圓，模擬送測，依到站順序逐片處理。
- 每次量測回傳膜厚測點與缺陷座標，保存來源設備、PM、晶圓、加工版本及量測版本；重測新增紀錄。
- 沿用 SECS/GEM → Edge Gateway → MQTT Sparkplug B → Bridge → Kafka → Data Platform，提供可恢復、去重的量測資料回傳。
- 晶圓履歷提供膜厚／缺陷 Tab、摘要、測點詳情與前後量測比較，支援既有三語與手機畫面。
- 超規僅標示異常並產生站內告警，不自動報廢、暫停 CVD、建立簽核案件或寄送 Email。量測失敗標示待重測，不產生假的完成圖。
- 所有量測明確標示模擬；不宣稱特定 KLA 型號、原廠資料格式或真實量測精度。

## Capabilities

### New Capabilities

- `kla-metrology-flow`：共用量測站、自動送測、排隊、模擬量測與可靠回傳。
- `wafer-map-history`：兩類 wafer map、版本履歷、查詢、比較及異常站內告警。

### Modified Capabilities

無。現有 `openspec/specs` 尚無規格；相容性依已完成變更及程式保留。

## Impact

涉及 Go 模擬站與 gateway、協定契約、Bridge 路由、Java 消費與查詢、PostgreSQL、Vue 晶圓履歷及 Docker Compose。保留既有 CVD 加工、補做簽核、保養、稼動率與成功／報廢統計語意；不新增實體 AMHS／FOUP 搬運或 KLA 3D 模型。
