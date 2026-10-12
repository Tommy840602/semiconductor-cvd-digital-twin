# 正常製程模型 synthetic-tin-v1

所有數值為展示用合成值，不是真實 TEL 配方。固定 modelSeed=20260915；每個 PM 有 ±2% 固定偏差，每個加工次數／模擬 tick 有 ±1% 流量擾動。`schemaVersion=1` envelope 維持相容，新增 `processTelemetry.telemetryRevision=1`。原欄位 temperature／pressure／flow 保留，flow 為四路實際流量總和。

四階段各先分 1 秒，其餘以 20/20/45/15% 最大餘數法分配，同餘數依階段順序。5 秒為 1/1/2/1；30 秒為 6/6/13/5。完成為瞬時狀態，保存最後一秒快照及完成事件。

| 階段 | precursor | reactant | carrier | purge |
|---|---:|---:|---:|---:|
| 預熱 | 0 | 0 | 30 | 0 |
| 穩定 | 0 | 0 | 30 | 0 |
| 沉積 | 40 | 70 | 30 | 0 |
| 吹掃 | 0 | 0 | 0 | 100 |

氣體單位 sccm；為功能名稱，不指定化學物質。溫度 °C、壓力 mbar、加熱功率和閥门開度 %、膜厚 nm、速率 nm/s。LL 以三秒對數壓力插值連接 1013/0.01 mbar，容量及連鎖不依插值改變。

每次加工由 processRunId（晶圓／attempt／modelRevision）識別；僅沉積時累积五個合成採樣點厚度，平均即 thicknessNm。uniformityPct 為半極差／平均 ×100，未沉積時 null。正常目標 10 nm、允差 ±10%、均勻度上限 5%；品質 pass/review/not-evaluated 不更改 success/scrap/exception。重做保存前次未完成結果並重置累積值；報廢保存 not-evaluated。process-phase-changed 與 process-result 走事件管線，秒級數值走 telemetry。

`/api/telemetry?metric=temperature&pmId=PM1` 回傳 metric、unit、resolutionSeconds、points。指標白名單 temperature/power/deviation/flow/precursor/reactant/carrier/purge/pressure/valve/thickness/rate/uniformity；LL 支援 pressure/valve 及 llId。可加 waferId/processRunId/from/to；UTC [from,to) 限最近30天。短於1小時秒級，否則分鐘聚合。points 包含 time/equipmentId/processRunId/phase/actual/target/min/max/sampleCount/unit。缺失值為 null；超過2000點回422，要求縮短範圍。既有不帶metric請求維持原格式。

`/api/process-history?pmId=PM1&waferId=...&from=...&to=...&page=0` 提供階段／結果事件，每頁100筆。舊模型沒有新欄位時不得補造數據。

檢查點 modelVersion=3。舊版正在加工的 PM 保留 legacy 模型直到該次完成，下一次加工啟用新模型；重啟依既有規則保持暫停。回退不得讓舊版讀取新版進行中檢查點；先排空或保留相容版本。更新前備份檢查點和資料卷；不要清除資料卷作為升級步驟。
