## 1. 協定契約與依賴驗證

- [x] 1.1 凍結模擬 GEM profile、支援訊息及錯誤回覆，標示標準與自訂部分；以資料字典和合法／非法封包 fixture 審查完成度。
- [x] 1.2 選定 Go HSMS／SECS 與 Sparkplug 編解碼依賴及版本，记录授權；以獨立 fixture 或外部實作驗證編解碼，不只測自己兩端互通。
- [x] 1.3 定義設備／節點／指標映射、Kafka envelope、控制簽章、ACK 及去重契約；驗證十台命名唯一、錯配拒絕及版本相容測試。

## 2. SECS/GEM 設備與 Host

- [x] 2.1 在 simulator 加入 HSMS session 與 SECS 型別處理；驗證 Select、Linktest、Separate、計時器、分段／黏包及超長封包。
- [x] 2.2 實作 GEM 通信與 Local／Remote 狀態機、身分確認；驗證未建立通信和 Local 時控制拒絕、重連不自動派片。
- [x] 2.3 實作狀態查詢、每秒 trace、事件及 alarm 報告；驗證字典型別／單位、來源時間與原加工引擎一致。
- [x] 2.4 實作配方傳輸、遠端命令及補做模擬擴充；驗證接受／完成分離、版本固定、膜厚保留及持久去重。
- [x] 2.5 將 Go Gateway 主鏈路改為 GEM Host；以兩台相同 PM ID 驗證端點隔離、拒絕錯配及單台逾時重連。

## 3. Sparkplug 與獨立 Bridge

- [x] 3.1 在 Gateway 實作 Sparkplug Edge／Device Birth、Death、資料與命令；以規範 fixture 驗證 Protobuf、topic、seq、bdSeq 與 alias。
- [x] 3.2 新增獨立 Go Bridge 與 Host STATE／rebirth／字典管理；驗證缺 Birth、重啟、世代更換與失聯品質不誤判。
- [x] 3.3 實作 MQTT 到 Kafka 正規化與持久交付；驗證按機台 key、歷史標記、重播去重及最新狀態不倒退。
- [x] 3.4 實作 Kafka command 到 Sparkplug DCMD 回程、可信 envelope 及 Broker 身分／ACL；驗證偽造、過期、錯機台和未授權發布拒絕。
- [x] 3.5 實作 Gateway／Bridge 積壓、容量告警與重要事件 ACK；用 Broker／Kafka 中斷及程序崩潰驗證原 ID 重送、已執行控制不重做。

## 4. 平台與既有業務相容

- [x] 4.1 將 Java Pipeline 改為消費新 Kafka envelope 並保留既有歷史讀取；驗證來源時間、品質、機台篩選及 Redis 重建。
- [x] 4.2 將控制 outbox 接至新回程並補足待確認狀態；驗證接受不等於完成、逾時查詢原 commandId 與 API 相容。
- [x] 4.3 串接不可變配方及 EE／PE 補做簽核；驗證 Operator 拒絕、跨台 recipe 複製、重複核准、撤銷競態及補做只執行指定秒數。
- [x] 4.4 更新 Vue 協定標示與通信／資料品質；以桌面和手機驗證十台切換、失聯顯示、PM hover 及既有布局不退化。

## 5. 遷移與整體驗收

- [x] 5.1 更新隔離 Compose 的十組 HSMS／Sparkplug 端點及 Bridge；驗證健康檢查、独立卷和單台重啟不影響他台。
- [x] 5.2 在隔離環境完成十台各三批共 750 片全鏈路加工與單台故障；逐台對帳晶圓、歷史、配方及控制，確認無跨台污染。
- [x] 5.3 進行 PE 補做期間斷線／回覆遺失／重播測試；確認 executionId 單次执行、原膜厚保留、取消競態及完成事件可追溯。
- [x] 5.4 實測十台每秒採樣的延遲、CPU、記憶體、Kafka lag、緩衝與磁碟成長，交付 30 天容量估算並驗證清理保護活動資料。
- [x] 5.5 交付原 outbox／spool／簽章轉接及備份／回滾流程；在完整副本演練遷移兩次與實際還原，對帳原始資料、帳號雜湊和在途意圖。
- [x] 5.6 正式維護切換並停用舊 OPC UA／JSON 主鏈路；驗證十台健康且暫停、原晶圓與簽核不變、外網登入與資料品質正常，保留可還原備份和驗收紀錄。
