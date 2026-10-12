## Why

使用者確認取消整批 FIFO，讓 LP1、LP2、LP3 同時加工。現有排程只允許最早批次进入 PM，其他 LP 僅能預載。

## What Changes

- **BREAKING** 三批共用四個 PM，依就緒批次輪流派片，取代舊變更的整批 FIFO。
- 單批暫停不阻擋其他批次；PM 故障仍暫停全機新派片，人工處置後恢復。
- 保持 LL 單片、雙手臂互鎖、完成片優先回收及原槽位回收。
- 更新畫面排程文字、檢查點與測試，部署後啟動三個 LP 展示。

## Capabilities

### New Capabilities

- `cvd-concurrent-scheduling`: 多批次輪流派片、資源預留及相容恢復。主規格尚未建立，本能力明確取代 tel-tin-cvd-simulator 中整批 FIFO 的排程要求。

### Modified Capabilities

無。

## Impact

Go engine、排程測試、Vue 排程說明及 Docker 展示；資料庫、通訊契約及製程模型延用現況。
