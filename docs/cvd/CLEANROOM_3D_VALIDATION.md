# 第 8 走道 3D 驗收與回退

## 交付範圍

入口為兩側各五台 GLB 設備的 3D 場景，LP 朝中央走道。桌面指向或手機點選取得摘要，再按「進入製程畫面」開啟原單機畫面。人物提供自動巡走、手動操控、第三人稱及全場俯視；巡走是前端展示，不控制設備。

## 驗證方式與結果

2026-09-18，macOS 主機上的 Chrome headless，1440×1000 桌面與 390×844 手機尺寸／觸控模擬：

- `npm --prefix frontend test`：六項測試通過，涵蓋十台身分、兩列排列、斜向等速、邊界、障礙、長幀時間、路徑與資料品質。
- `npm --prefix frontend run build`：通過。Three.js 場景採延後載入，主程式約136 KB，場景約612 KB（gzip 約158 KB）；Vite 仍提醒場景 chunk 超過500 KB，不影響建置。
- 完整巡走實測56秒，經過路徑索引0～9全部十個設備前停留點。
- 實際鍵盤W與CDP觸控搖桿可移動；切換模式無瞬移，俯視不接收手動位移。
- 01～10 全數進入正確原製程畫面，反覆返回人物位置及模式保持；3D網格指向02取得02摘要。
- 初次及十次重建場景後均為64 geometries、11 textures；穩態俯視約74 draw calls、112350 triangles。機台共用geometry與instancing，避免2460個機台零件各自發出draw call。
- 單獨執行的已部署前端桌面／手機尺寸皆約60 FPS；390px無橫向溢出。曾並行啟動兩個 headless Chrome 時觀察到非作用中視窗降頻，因此正式穩態量測採單一瀏覽器。尚未在實體手機GPU上量測，不能將模擬結果視為所有手機的效能保證。
- 模型請求阻擋與 WebGL context loss 均顯示十台設備清單，重試可恢復3D。
- 瀏覽器測試監聽POST，除登入／活動等auth端點外沒有業務異動請求；既有recipe及EE／PE權限條件未修改。部署後Leader可進入原recipe編輯入口；本次未在生產重新建立帳號或執行補做簽核測試。
- Docker前端部署後模型HTTP200；外網同一模型224116 bytes且SHA-256與來源一致。

瀏覽器驗證腳本：`scripts/verify-cleanroom-ui.mjs`，使用 Node 26 的原生 WebSocket、獨立Chrome測試profile與 `.runtime/secrets/leader-password`。先啟動 frontend Vite 18001，從專案根目錄執行腳本；它只登入、查詢、操作瀏覽器展示與導覽，不發出設備控制命令。完整結果保存 `.runtime/cleanroom-proof.json`，截圖保存 `.runtime/cleanroom-{desktop,mobile,follow}.png`。以 `CVD_QUICK=1` 可略過完整巡走重測，其結果另存 quick-proof。

## 規格對照

| 規格 | 證據 |
| --- | --- |
| 走道與設備配置 | 模型視覺檢查、包圍盒與方向紀錄、唯一ID測試 |
| 設備狀態隔離 | 狀態測試、瀏覽器資料替身故障／陳舊標記檢查（不注入設備故障） |
| 製程畫面導覽 | 3D指向02、逐台01～10選台、部署後07詳情 |
| 總覽返回 | 十次重建位置相等、geometry／texture數穩定 |
| 載入與降級 | GLB阻擋、context loss、重試 |
| 模式切換與巡走 | 完整巡走、模式切換位置相等 |
| 跨裝置手動控制 | 鍵盤事件、touchStart／touchEnd、正規化向量測試 |
| 第三人稱與俯視 | 截圖及俯視期間位置不變 |
| 移動邊界與避障 | 長時間持續輸入、長幀、障礙滑動測試；目前場景一名人物 |
| 展示與業務隔離 | 非auth POST為零；visibility／blur清輸入與停動畫、卸載釋放資源 |

## 部署與回退

只更新 `frontend` 容器，沒有重建後端、資料庫或模擬器，也沒有初始化批次／recipe／帳號。

已部署映像：`cvd-secs-frontend:cleanroom-3d`，同時標記為 Compose 使用的 `cvd-secs-frontend`。

回退映像：`cvd-secs-frontend:before-cleanroom-3d`。在專案根目錄依序執行：

```sh
docker image tag cvd-secs-frontend:before-cleanroom-3d cvd-secs-frontend
docker compose up -d --no-deps frontend
```

重新套用3D時改用 `cvd-secs-frontend:cleanroom-3d`。本機建置以已安裝Node及快取nginx映像完成，生產Dockerfile仍保留標準npm ci建置流程。
