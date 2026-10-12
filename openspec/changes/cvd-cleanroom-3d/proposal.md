## Why

目前十台設備總覽無法呈現走道空間與人員巡走。以使用者提供的 GLB 建立第 8 走道 3D 總覽，讓使用者在空間中辨識設備狀態並進入既有製程畫面。

## What Changes

- 總覽改成 3D 無塵室，保留現代化配色；01～05、06～10 分列走道兩側，Load Port 朝中央。
- 十台模型綁定 TEL-CVDTIN-08A-01～10 與現有即時摘要，點擊進入原製程畫面。
- 加入無塵衣人物，自動巡走與手動操控可切換且保留位置；提供第三人稱跟隨與全場俯視。
- 電腦 WASD／方向鍵與手機虛擬搖桿，限制可走區域並處理避障。
- 增加模型載入、離線提示、資源釋放及行動裝置效能驗證。

## Capabilities

### New Capabilities

- `cleanroom-3d-overview`: 第 8 走道配置、模型與設備綁定、即時狀態、進入既有製程及降級顯示。
- `cleanroom-person-navigation`: 自動巡走、手動控制、視角切換與碰撞限制。

### Modified Capabilities

無。主規格庫目前為空；本變更新增總覽呈現契約，沿用既有 multi-equipment 變更的設備隔離與單機功能。

## Impact

主要涉及 Vue App 總覽、新增 Three.js 場景與模型資源、人員控制及前端驗證。沿用既有設備摘要與詳情介面，不變更 SECS/GEM、Sparkplug B、Kafka 資料鏈、資料庫 schema、recipe、RBAC 或 EE／PE 簽核。Docker 前端映像將包含模型資源，不新增運算服務。
