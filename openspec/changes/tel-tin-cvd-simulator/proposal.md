## Why

建立可操作、可追溯的 TEL TiN CVD 情境數位孿生展示，讓使用者觀察批次、晶圓、搬送與腔體之間的連動，並驗證跨 OPC UA、MQTT 與 Kafka 的完整資料流程。此專案是依指定拓樸建立的功能模擬，不代表特定 TEL 機型或原廠製程控制。

## What Changes

- 建立 Vue 3 JavaScript 儀表板與 Java 21／Spring 後端，支援即時設備視圖、控制、告警與歷史查詢。
- 以 Go 分別建立 OPC UA 設備模擬器與 OPC UA 至 MQTT 閘道器。
- 模擬 3 LP、3 個各容納 1 片的 LL、兩組手臂與 4 個單片平行 PM，採整批 FIFO。
- 實作故障停止派片、正常腔體完成回片、異常晶圓重做或報廢，以及人工恢復。
- 導入 Kafka、獨立 PostgreSQL、獨立 TimescaleDB 與 Redis；秒級採樣保留 30 天。
- 提供 Docker Compose 本機展示部署與可重現情境。

## Capabilities

### New Capabilities

- `cvd-simulation`: 設備拓樸、晶圓守恆、批次排程與搬送連鎖。
- `cvd-control`: 控制指令、故障處置與人工恢復。
- `equipment-data-pipeline`: OPC UA、MQTT、Kafka 與資料一致性。
- `cvd-dashboard-history`: 即時視圖、歷史查詢、快取與保存。
- `container-deployment`: Docker 服務、持久化與展示驗證。

### Modified Capabilities

無；目前沒有既有能力規格。

## Impact

新增前端、Java 後端、兩個 Go 服務、資料模型、通訊契約及容器部署。依賴 MQTT Broker、Kafka、PostgreSQL、TimescaleDB、Redis；本次僅建立規劃文件，不變動既有已刪除檔案、不部署服務或實作程式。
