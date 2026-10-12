## Why

目前展示與控制流程固定於 CVD-01，無法呈現十台設備各自加工與故障的情境。擴充為多機台需讓模擬、控制、履歷及補做簽核皆以機台識別隔離，避免同名 PM 或晶圓造成誤操作。

## What Changes

- 建立 TEL-CVDTIN-08A-01 至 TEL-CVDTIN-08A-10，每台各有 3 LP、3 個單片 LL、4 PM、ATM 與 VAC；各自輪流派片且不跨機搬送。
- 新增十台總覽與單台詳情切換，沿用現代化配色、2D 配置及手臂動畫。
- 每台獨立 recipe，Engineer／Leader 可跨台複製特定版本並個別調整，記錄來源與操作者。
- 共用帳號與基礎服務，以機台隔離事件、指令、快取、資料查詢及 EE／PE 補做簽核。
- 現有 CVD-01 對應 01，保留批次、歷史、簽核及製程狀態；02 至 10 空機待命。
- **BREAKING**：單台操作與查詢介面改為明確指定機台，MQTT 路由及訊息識別改為機台分流；前後端、模擬器與閘道器須配套升級。

## Capabilities

### New Capabilities

- `cvd-equipment-fleet`: 十台獨立運作、總覽、資料與控制隔離及既有資料遷移。
- `cvd-equipment-recipes`: 每台 recipe 與跨台版本複製、授權及追溯。

### Modified Capabilities

無。主規格目錄目前為空；本變更新增多機台契約，並延續既有變更中的輪流派片、權限、製程數據與補做規則。

## Impact

涉及 Vue 機台選擇與 API／WebSocket、Java 控制及歷史與簽核服務、Go 模擬器與 OPC UA 閘道、MQTT 路由、Kafka 分區鍵、PostgreSQL／TimescaleDB 索引、Redis 機台鍵與 Docker Compose 部署。採樣仍為每秒一次、歷史保留最近 30 天；需驗證十台負載及磁碟需求。
