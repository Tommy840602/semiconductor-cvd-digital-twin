## Why

目前十台 CVD 使用 OPC UA、自訂 MQTT JSON 與內嵌於 Java 後端的橋接，未呈現半導體設備的 SECS/GEM 主機通訊。改為 SECS/GEM 與 Sparkplug B 資料鏈路，同時保留已驗證的加工、配方及簽核行為。

## What Changes

- **BREAKING**：十台 Go 模擬設備以 SECS-II／GEM over HSMS-SS 取代 OPC UA 主鏈路；定義可驗證的模擬通訊子集，並非 TEL 原廠協定或完整 GEM 認證。
- Go Edge Gateway 扮演 GEM Host 與 Sparkplug Edge Node；每台維持獨立連線、持久化、身分及故障範圍。
- **BREAKING**：自訂 MQTT 主鏈路改為 Sparkplug B topic、Protobuf payload、Birth／Death、重建及命令語意。
- 獨立 Sparkplug-aware MQTT–Kafka Bridge，將指標、事件、連線品質與控制結果送至 Kafka，再由 Java Data Platform 消費。
- 控制回程保留 RBAC、EE／PE 簽核、不可變 recipe、設備狀態檢查、過期控制拒絕及端到端去重。
- 提供十台 Docker 部署、原資料遷移、隔離驗收、容量量測及回滾方案；不新增正式測試批次。

## Capabilities

### New Capabilities

- `cvd-secs-gem-interface`：模擬設備與 Host 通訊、資料字典、狀態及控制接受／完成區分。
- `cvd-sparkplug-data-plane`：Sparkplug Edge、Broker、Bridge 與 Kafka 的資料、品質及重建行為。
- `cvd-protocol-control-migration`：雙向控制、既有業務不變、資料遷移與部署驗收。

### Modified Capabilities

無。`openspec/specs/` 尚無已同步的規格；既有行為依已完成變更與程式保留，本次新規格描述通訊替換及相容性要求。

## Impact

涉及 simulator、gateway、Java Pipeline、部署 Compose、通訊契約、網站連線品質顯示與整合測試。新增 HSMS／SECS 編解碼及 Sparkplug Protobuf 依賴或實作、獨立 Bridge 服務。Vue 3、Java 21、PostgreSQL、TimescaleDB、Redis、Kafka 與 MQTT Broker 延用；既有機台 ID、資料及帳號不可重建或清空。
