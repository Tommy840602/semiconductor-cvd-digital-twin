## Purpose

以完整機台為單位彙總阻擋生產的異常處置，要求未參與該次處置的 EE 或 PE Leader 核准，再由設備重驗後恢復派片，防止局部結案、舊核准或訊息重送解除新的異常限制。

## ADDED Requirements

### Requirement: 整台放行資格

系統 SHALL 要求同台所有阻擋異常完成PE結果確認及必要回收、故障解除且互鎖與可信資料滿足，才可核准整台放行；介面須逐項列出阻擋原因。

#### Scenario: 其他PM未結案
- **WHEN** PM1已確認但PM2仍待PE或未回收到位
- **THEN** 整台不可放行，不以PM1結案恢復任一批派片

#### Scenario: 資料過期
- **WHEN** 设备狀態失聯或執行結果未知
- **THEN** 拒絕放行並提示等待設備確認

### Requirement: 不同帳號Leader

系統 SHALL 只允許 EE／PE Leader 核准放行，且其帳號不得是本次集合任何處置的提案人、PE審核人或結果確認人；角色或部門變更不得消除既有參與紀錄。

#### Scenario: 參與過任一處置
- **WHEN** PE Leader審核過其中一張單後嘗試整台放行
- **THEN** 拒絕，需其他合格Leader

#### Scenario: 不同部門主管
- **WHEN** 未參與本次任何處置的EE Leader或PE Leader核准
- **THEN** 在其餘條件滿足時允許，Engineer與未指定部門Leader仍拒絕

### Requirement: 核准自動恢復

系統 SHALL 在Leader核准後自动觸發設備重驗；全部条件通过才解除此次整台異常派片鎖並恢復符合既有條件的批次派片，收到設備證據後才顯示實際放行完成。

#### Scenario: 正常放行
- **WHEN** 整台所有條件滿足且Leader核准
- **THEN** 設備重驗後自動恢復，紀錄核准與實際恢復時間，不需再按resume

#### Scenario: 指令結果未知
- **WHEN** 核准後ACK遺失
- **THEN** 顯示待設備確認並查詢原放行識別，不宣稱已恢復

### Requirement: 新異常使核准失效

系統 SHALL 將核准綁定當次機台異常集合與結果版本；在實際恢復前新增異常、集合變動或必要條件失效時，舊核准失效，需重新取得Leader核准。

#### Scenario: 核准與新異常競爭
- **WHEN** Leader核准後、設備恢復前新增異常
- **THEN** 保持派片鎖並拒絕舊核准，不能自動等新異常處理完再沿用

#### Scenario: 放行後新異常與重播
- **WHEN** 已恢復後再發生異常且舊放行命令重送
- **THEN** 新異常重新鎖派片，舊命令不解除新鎖

### Requirement: 不可繞過與每台隔離

系統 SHALL 在有異常放行週期時禁止一般resume、clear、recovery-clock、start、recipe變更與重啟绕過派片鎖。恢復設備時鐘只允許既有核准處置及安全回收推進，不等於整台放行。十台放行 SHALL 獨立。

#### Scenario: 恢復時鐘
- **WHEN** 工程人員為已核准補做恢復時鐘
- **THEN** 補做可依互鎖推進，仍不得新派片

#### Scenario: 跨台核准
- **WHEN** 把01的放行識別送往02
- **THEN** 拒絕，01異常也不阻擋02的獨立運作

### Requirement: 遷移不偽造簽核

系統 SHALL 保留舊歷史與既有執行識別，為活動處置明確標示待補確認或重審，不得假造PE確認或Leader放行，不得重新執行已完成處置。

#### Scenario: 舊補做執行中升級
- **WHEN** 原核准補做尚未結束
- **THEN** 保留其執行進度，完成後依新留片規則等待PE

#### Scenario: 舊紀錄已回收
- **WHEN** 舊補做已完成且回LP、但活動異常尚未放行
- **THEN** 保留回收事實，補齊PE結果確認與Leader放行，不搬回PM或重做
