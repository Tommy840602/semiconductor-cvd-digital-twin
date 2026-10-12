## Purpose

提供可在本機以容器啟動的完整展示環境，包含前後端、模擬設備、通訊及各資料服务，並讓資料與模擬狀態在服務重啟後維持可追蹤，以支持正常、故障及斷線情境驗證。

## ADDED Requirements

### Requirement: 獨立資料服務

系統 SHALL 以 Docker Compose 提供 Vue、Java 21 Spring、兩個 Go 服務、MQTT、Kafka、PostgreSQL、TimescaleDB 與 Redis；PostgreSQL 及 TimescaleDB 須使用不同服務與持久化資料卷。

#### Scenario: 啟動完整展示
- **WHEN** 依部署說明啟動環境
- **THEN** 各服務就緒後可操作批次並查詢歷史，兩個資料庫可獨立停止及重啟。

### Requirement: 重新啟動保留狀態

系統 SHALL 保存歷史、批次檢查點與必要指令去重資料；模擬器重啟後不得自動重做已完成動作，並等待人工恢復派片。

#### Scenario: 進行中重啟
- **WHEN** 批次尚未完成而模擬器重新啟動
- **THEN** 晶圓位置及數量恢復一致，歷史保留且派片暫停。

### Requirement: 展示情境可驗證

系統 SHALL 提供正常批次、PM 故障重做、報廢回收及通訊中斷恢復的展示說明與結果判定。

#### Scenario: 驗證故障批次
- **WHEN** 依說明注入 PM 故障並報廢處置
- **THEN** 可觀察暫停、正常 PM 回片、人工恢復及含報廢結案。

