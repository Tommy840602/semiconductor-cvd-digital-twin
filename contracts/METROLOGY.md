# 模擬 KLA 量測契約 v1

此為專案自訂 SECS/GEM／Sparkplug 應用擴充，不是 KLA 原廠格式。站名 `KLA-SIM-01`，設備類型 `metrology`；`contract.ValidEquipment` 仍只接受十台 CVD，通訊 `ValidDevice` 另接受量測站。

## 來源與命令

來源必須有 `equipmentId,waferId,lotId,slot,pm,processRunId,recipeVersion,thicknessNm,sampleThicknessNm,targetThicknessNm,thicknessTolerancePct,uniformityLimitPct`；可附 `purpose,executionId`。slot 1–25、PM1–PM4、五點膜厚，數值有限且非負，target > 0。

返 LP 的 `movement-complete.payload` 加入 `processRunId,result,slot,purpose`；平台僅配對同一設備／晶圓／版本的 `process-result.complete=true` 及 `to=LP[123],result=success`。不回填升級前缺失欄位。新欄位不改既有搬送與加工語意。

簽名命令沿用 HMAC v1 envelope，payload 為 `equipmentId=KLA-SIM-01,commandId,expiresAt,type=measure,parameters={measurementId,source}`。commandId MUST 等於 measurementId；重送同 ID 與來源為技術重試，不新增版本，異來源拒絕。站端依序持久化接受及完成事件。

## 量測事件

`equipmentId,eventId,sequence,kind=measurement-state,sourceTime,payload`；payload 為 `measurementId,source,state,at,result?`。sequence 單調，重啟保留；queued → measuring → completed／awaiting-remeasure。结果僅於 completed 存在。Bridge 路由 `metrology.events.v1`、`metrology.snapshot.v1`、`metrology.connectivity.v1`、`metrology.command-accepted.v1`，不進 CVD 統計。

## 結果

`modelVersion=synthetic-kla-v1,seed,synthetic=true,source,diameterMm=300,edgeExclusionMm=3,coordinates=center-x-right-y-up-notch-down-mm`。

- `points` 固定 49 個 `{x,y,thicknessNm}`，半徑不超過147 mm；中心加8／16／24點三圈。
- `defects` 為0–1000個 `{defectId,type,x,y,sizeUm,severity,path?}`；type 為particle／scratch／other；scratch 有至少兩點的路徑，各點皆在有效圓內。此缺陷模型不代表真實量測。
- `meanNm,minNm,maxNm,uniformityPct`，均勻度=(max-min)/2/mean×100，平均零時null。
- `thicknessQuality,defectQuality,quality` 為pass／review，分別依來源膜厚門檻、均勻度門檻及`defectCountLimit=10,defectSizeLimitUm=1`判斷；任一測點膜厚超規則review。
- 缺少任一圖、數值非有限、越界、重複缺陷ID、摘要或判定不符均不得公布完成。未知／失敗不當作零。

兩類圖一次驗證及交易發布；每個 measurementId 一筆品質告警旗標，確認不刪結果。圖於完成30天後可清理，但進行中同來源工作與未確認告警保護結果。資料不寫入每秒 telemetry。
