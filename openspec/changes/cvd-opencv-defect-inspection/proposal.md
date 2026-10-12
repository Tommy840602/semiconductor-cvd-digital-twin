## Why

現有 KLA 產生合成膜厚與缺陷座標，尚無影像分析證據。加入 OpenCV，讓每片完成量測的晶圓能展示缺陷分布辨識與表面影像檢查，並區分公開資料重播、模擬影像與辨識結果。

## What Changes

- 優先查核並使用 Kaggle 資料；缺少可取得、可使用且標註適合的表面影像時，以可重現的模擬影像補齊。
- KLA 量測完成後自動建立兩種視覺檢查，獨立保存狀態與版本，不改寫既有量測結果。
- OpenCV 實際分析像素；標註僅供評估，不直接當作辨識輸出。
- 網站提供原圖、標記圖、分布類型、缺陷詳情、歷史與來源標示；異常僅站內告警。

## Capabilities

### New Capabilities
- `inspection-dataset-provenance`：資料來源、授權查核、模擬補齊、標註隔離及可重現評估。
- `opencv-defect-inspection`：自動雙類檢查、可靠執行、結果與權限、歷史呈現及告警。

### Modified Capabilities
無。主規格庫目前為空；本變更新增 KLA 完成後的獨立檢查，不修改既有 KLA 完成條件、膜厚與處置行為。

## Impact

新增 Docker 視覺工作服務與影像持久儲存，擴充 Java 工作排程、資料庫、API、Kafka 事件及 Vue 三語晶圓履歷。既有 Go 設備／閘道器、KLA SECS/GEM 回傳與十台 CVD 排程維持原職責。資料下載、程式實作與部署屬後續 apply 範圍，本次僅建立規劃。
