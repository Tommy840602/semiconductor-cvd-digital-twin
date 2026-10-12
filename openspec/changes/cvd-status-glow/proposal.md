## Why

目前首頁、2D 設備配置與 3D 機台主要使用靜態色彩提示狀態，使用者不容易快速辨識加工、暫停、故障與資料過期。加入一致的明暗發光效果，讓十台設備及個別 PM 的即時狀態更容易辨認。

## What Changes

- 加工綠色每 2 秒呼吸、暫停黃色每 2 秒閃爍、故障橘紅色每 1 秒閃爍、資料過期灰藍色每 3 秒閃爍。
- 首頁即時狀態標記、2D 設備外框、3D 機台底座光圈與狀態燈套用一致規則；3D 保留模型原色。
- 保留資料過期、故障、保養、暫停、加工的既有優先序；待機及保養維持固定色。PM 依自身狀態呈現。
- 保留文字辨識、選取效果與原操作；減少動態效果偏好下改用固定發光。

## Capabilities

### New Capabilities

- `equipment-status-glow`: 定義首頁、2D、3D 即時狀態的配色、週期、優先序、設備隔離及動態效果生命週期。

### Modified Capabilities

無。主規格庫目前沒有已登錄能力；本次以獨立視覺能力銜接既有 3D 與製程功能。

## Impact

前端 CleanroomOverview.vue、EquipmentTopology.vue、App.vue、cleanroom/navigation.js、cleanroom/scene.js 與相關樣式、測試。沿用既有狀態資料，不更動後端 API、加工邏輯、稼動率、權限、資料庫或 GLB 資產；不要求新增圖形套件。
