# 機台利用率採樣 v1

telemetry 的 payload.utilization 為選用擴充。v1 含 version=1、intervalStart、intervalEnd（UTC 整秒，半開區間，恰一秒）、pm={PM1:boolean,PM2:boolean,PM3:boolean,PM4:boolean}。設備 ID、sourceEpoch、sequence、sourceTime、quality 沿用事件 envelope。sourceTime 必須對應 intervalEnd 所在秒。

旗標表示該 tick 實際執行加工步驟，包含核准補做；模擬暫停仍取樣但四 PM 為 false。首次 tick、重啟首次 tick 或非連續秒不宣稱已知區間。等待留片、故障或待機不是加工。未知不能編碼為 false，舊樣本缺少整個 utilization 時保持未知。

SECS/GEM 既有事件報告保存完整事件 JSON；Edge 以 Sparkplug B CVD/Event Bytes 原樣傳送，Bridge 保留 payload 並送至 cvd.telemetry.v1。無需變更既有指令或標準服務語意。

範例：{"version":1,"intervalStart":"2026-09-19T01:00:00Z","intervalEnd":"2026-09-19T01:00:01Z","pm":{"PM1":true,"PM2":false,"PM3":false,"PM4":false}}。舊版 payload 只有 chambers/locks 不構成利用率證據。

來源區間而非接收時間決定統計位置。相同設備與秒、相同遮罩的重送冪等；同秒矛盾遮罩永遠標未知，重播不得恢復成任選值。資料超過 30 天不納入。
