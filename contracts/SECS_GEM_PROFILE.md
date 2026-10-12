# CVD 模擬 GEM Profile v1

這是 TEL-CVDTIN-08A-01～10 的模擬通訊契約，不是 TEL 字典或完整 SEMI 認證。線路採 SECS-II over HSMS-SS，設備 passive／Host active；session ID=1 僅局部有效，設備 SVID 1 必須符合受控端點映射。

計時器 T3=5s、T5=1s、T6=5s、T7=10s、T8=5s；啟用嚴格交易回覆匹配。HSMS 支援 Select／Deselect／Linktest／Separate 與斷線重新選擇，GEM 通信及 Online 狀態須重新建立，不能把 TCP 存活當作 Remote。

## 訊息契約

| 訊息 | 用途／限制 |
|---|---|
| S1F1/2 | model=CVD-SIM、revision=1.0 |
| S1F13/14 | 建立通信，COMMACK；尚未建立通信的其他請求拒絕 |
| S1F15/16、S1F17/18 | Offline／Online，與本地 Local／Remote 選擇分開 |
| S1F3/4 | 按 SVID 順序回值，未知 SVID 回空 List |
| S2F23/24 | trace；DSPER=000001、REPGSZ=1，TOTSMP=0 取消 |
| S6F1/2 | trace ID、sample number、UTC 來源時間與型別化值 |
| S2F33/34、35/36、37/38 | 定義 report、連結 CEID、啟用／停用；非法連結不改既有定義 |
| S6F11/12 | 已啟用 CEID 的報告；交易 ACK 不取代應用層 durable ACK |
| S5F1/2 | PM alarm set／clear；ALID 1～4 對應 PM1～4 |
| S2F41/42 | RCMD 接受／拒絕；接受不是晶圓加工完成 |
| S7F3/4、S7F5/6 | 以不可變 PPID 下載／上傳 process program；相同 PPID 異內容拒絕 |
| S9F5、S9F7 | 未支援功能／非法資料，攜帶原訊息 header |

## 模擬資料字典

| ID | 類型 | 意義 |
|---|---|---|
| SVID 1 | ASCII | 全域 equipmentId |
| SVID 2、3 | ASCII | model、revision |
| SVID 4 | U1 | 控制狀態：Offline=1、Local=4、Remote=5 |
| SVID 101～104 | F8 | PM1～4 溫度，°C |
| SVID 1000 | Binary | 版本化完整狀態 JSON，模擬擴充；供保留既有晶圓布局與履歷欄位 |
| SVID 1001 | Binary | 尚未 durable ACK 的事件陣列，模擬擴充；事件包含穩定 eventId／sequence／sourceTime |
| CEID 1 | 報告 | 持久事件佇列變更，內含製程／搬送／指令結果／補做狀態 |

`CVD_CONTROL` RCMD 使用 Binary `ENVELOPE` CP，攜帶經平台簽章的原始命令；保留 commandId、equipmentId、expiresAt、參數與既有補做簽章。只有 Online Remote 允許執行。`CVD_ACK` RCMD 的 U8 `SEQUENCE` CP 是持久事件交付確認，不啟動製程。此二者都是模擬擴充。

PPID 為設備 recipeVersion，PPBODY 是平台簽章的 configure envelope；設備驗證後保存其版本化 recipe 參數，S7F6 回傳已保存的參數供比對；不可變版本的保存與目前設備選用分開，下載不追改正在加工的晶圓。GEM 交易 system bytes／Sparkplug seq 均不是業務去重鍵。

## 依賴與驗證

- github.com/arloliu/go-secs/v2 v2.4.1，Apache-2.0；Go 1.26。HSMS、SECS-II 與 GEM builder 由此固定版本提供。
- Eclipse Tahu sparkplug_b.proto，commit 5736e404889d4b95910613040a99ba79589ffb13，EPL-2.0；保留來源授權註記，Go 產碼使用 google.golang.org/protobuf v1.36.11（BSD-3-Clause）。
- `protocol/gemlink/device_test.go` 使用手工 TCP framing 與 SECS bytes 作獨立 peer；`protocol/sparkplug/wire_test.go` 使用手工 Protobuf bytes；不宣稱已執行 SEMI 或 Sparkplug 認證。

## 異常處置擴充（modelVersion 6）

`CVD_CONTROL` 明確支援 `wafer-scrap`、`equipment-check`、`disposition-confirm`、`disposition-reject`、`equipment-release`。仍使用 Binary ENVELOPE，設備另驗核准 HMAC；不可改用一般 resume 代替 release。`wafer-supplement` 新核准需 incidentId/dispositionType。取消沿用 cancel-supplement 並共用 executionId 取消柵欄。

SVID1000 增加 dispositionHold、holdGeneration、incidents、recoveries、releases、authorizations。SVID1001／CEID1 增加 disposition-exception、disposition-executed、disposition-confirm、disposition-reject、disposition-returned、equipment-released。完整欄位見 DISPOSITION.md 及 Go 的 Incident／DispositionAction／ReleaseEvidence；它們保留設備與結果版本，不以 trace sample number 替代。

`legacy-return-check` 為升級時已回槽活動異常的證據核對，仍用同一 CVD_CONTROL 雙簽章。設備核對 legacyResult、exceptionVersion、原LP及 Legacy/Returned 標記；不產生搬送／沉積。`disposition-legacy-return` 事件與 dispositionMigrationVersion=1 保留這項一次性遷移。
