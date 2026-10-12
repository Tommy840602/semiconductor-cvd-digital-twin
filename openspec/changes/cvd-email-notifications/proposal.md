## Why

目前設備告警與異常簽核待辦需登入網站查看，EE／PE 人員離開畫面後容易錯過處置時機。新增依帳號部門與角色分派的 Email 通知，涵蓋十台設備。

## What Changes

- 帳號增加收件 Email，由 Leader 管理；依 EE／PE 與 Engineer／Leader 分派通知，Operator 預設不收信。
- 包含 PM 故障、設備失聯、解除通知，以及補做／報廢待審、待結果確認、待整機放行。
- 使用一個個人 Outlook 寄件帳號，透過 OAuth 委派授權連接 Microsoft Graph，不儲存 Outlook 密碼。
- 加入持久寄信佇列、事件去重、有限重試、寄送紀錄及授權失效提示。
- 信件附事件摘要與登入網站連結，核准仍在網站依現有權限執行。

## Capabilities

### New Capabilities

- `email-notifications`: 個人 Outlook 授權、收件分派、通知生命週期及寄送追蹤。

### Modified Capabilities

無；目前主規格庫沒有已登錄能力。

## Impact

Java 後端事件消費、簽核狀態轉移、PostgreSQL migrations、帳號管理與通知管理前端、Docker secrets／環境設定。十台設備全部納入，不加入個人負責機台設定。整台主機或 Docker 停機的外部監測不在本次範圍；本機完全停止時不能即時寄信。本次規劃不註冊 Microsoft 應用、不授權信箱、不實際寄信。
