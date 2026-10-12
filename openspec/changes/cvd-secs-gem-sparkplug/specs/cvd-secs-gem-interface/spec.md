## Purpose

定義十台 CVD 模擬設備與主機透過 HSMS、SECS-II 與 GEM 交換資料和控制的可驗證契約，讓通訊故障、設備模式、指令接受與加工結果具有清楚且相互獨立的語意。

## ADDED Requirements

### Requirement: 明示模擬通訊 Profile

系統 SHALL 公布版本化支援訊息、型別、變數、事件、告警、遠端命令與配方映射；所有 TEL 未提供的定義 MUST 標示為模擬，不宣稱完整 GEM 或 TEL 相容。

#### Scenario: 不支援的訊息
- **WHEN** Host 發送未支援訊息或不合法型別
- **THEN** 設備回覆明確協定錯誤或依 profile 終止交易，且不變更製程。

### Requirement: 十台独立通信與設備識別

每台 SHALL 具有獨立 HSMS session、交易關聯及斷線重連；端點與設備身分不符 MUST 拒絕資料和控制。

#### Scenario: 機台错配
- **WHEN** 01 的 Gateway 連到宣告 02 的設備
- **THEN** 連線標示身分不符，01 和 02 均不因該控制改變。

#### Scenario: 單台斷線
- **WHEN** 01 的 HSMS 連線中斷或交易逾時
- **THEN** 01 顯示通信異常，其他九台持續更新；01 重連不得自動派新批次。

### Requirement: HSMS 與 GEM 狀態分離

系統 SHALL 區分 TCP、HSMS Selected、通信建立、Online Local／Remote 與加工狀態，並驗證 profile 定義的計時器、Linktest 及異常封包處理。

#### Scenario: Local 控制拒絕
- **WHEN** 設備處於 Local 或通信未建立時收到遠端加工命令
- **THEN** 回覆拒絕及原因，晶圓與配方不改變。

#### Scenario: 截斷與過大封包
- **WHEN** 收到分段、黏包、截斷或超長封包
- **THEN** 合法分段可重組；非法資料受長度與時間限制拒絕，不造成其他設備失去服務。

### Requirement: 資料與製程控制行為

設備 SHALL 支援 profile 的狀態查詢、每秒採樣、事件報告、告警、配方傳輸及遠端控制，並分開回報命令接受與最終結果。

#### Scenario: 接受但未完成
- **WHEN** 設備接受開始或補做命令
- **THEN** 平台先顯示已接受，只有具相同命令或執行身分的完成事件才能標為完成。

#### Scenario: 配方版本固定
- **WHEN** 新配方下載至設備而某晶圓仍加工中
- **THEN** 後續新加工可使用新版本，現有晶圓的步驟秒數與版本保持不變。
