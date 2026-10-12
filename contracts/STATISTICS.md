# 首頁統計契約 v1

`telemetry.payload.statistics` 與同事件 `utilization.intervalStart/intervalEnd` 共用一秒真實區間，欄位 version=1、state、flags、era、closureCount。state 為 processing／idle／maintenance／dummy／fault／paused／unknown；flags 低四位為 PM1～4 故障，16 為派片暫停、32 為異常 hold、bits 6～9 為保養階段（無、draining、maintenance、waiting、qualifying、failed、passed、ready、released），65536 為時鐘暫停。指令或狀態切換穿過採樣邊界時該段 state=unknown，不沿用新快照回填。

`wafer-closed` 事件帶原 equipmentId／waferId／lotId，payload 有 version=1、purpose、result、closedAt、returnedAt、confirmedAt、confirmationId、incidentId、era 與 closureSequence。正常完成須回原槽；異常處置還須 PE 確認。closedAt 取所有必要證據中最晚來源時間，Leader 放行不另計數。era 與單調 closureSequence 持久保存；晶圓 checkpoint 保存 closedAt、lastReturnAt、closureSequence，支援來源證據重建。未能確定的舊快照不生成新結案。

來源事件沿既有 SECS/GEM、Sparkplug B、Kafka 傳遞，沒有圖表專用設備連線。後端以設備／秒去重狀態；結案以設備／晶圓唯一鍵及 era／sequence 收據去重。收據未連續到該區間宣告的 closureCount，產出完整性不能宣稱完整。狀態涵蓋率與結案涵蓋率獨立。

唯讀 `GET /api/equipment/statistics?period=today|24h|7d|30d` 回傳 start、end、computedAt、timezone、resolutionSeconds、equipment、fleet、utilization。後者沿用原雙指標並共用截止時間。equipment 包含 states 秒數、timeline 桶、狀態 coverage 及 production（completed、scrapped、dummyCompleted、dummyScrapped、knownSeconds、coverage、quality、conflicts）。桶包含 start、end、states、flags、state；混合桶 state=mixed。所有分類加總涵蓋完整區間，未知在分母內。

查詢範圍半開 [start,end)，上限 30 天；今日以台灣午夜起算。概覽解析度 60／300／1800 秒，包含邊界每台最多 2,017 桶。頁面合併相鄰同狀態／旗標，保留合併後各分類秒數。快照內既有累計 success／scrap 不作區間統計。

最近 30 天可查詢詳細結案；更舊結案僅留去重識別、時間與結果墓碑，活動批次保留證據。歷史回填用既有回槽及必要簽核事件，標示 backfilled，不藉此宣稱舊區間採集完整。Cp／Cpk 不在此契約範圍。
