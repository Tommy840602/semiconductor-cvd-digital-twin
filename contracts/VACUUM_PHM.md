# 真空 PHM 模擬契約

模型 `synthetic-vacuum-v1`。這是展示模型，不代表 TEL 實體泵浦配置或真實壽命。

每台三個 LL 各一組泵浦，以 equipmentId / LL ID 隔離。每次有效抽氣啟動一次增加 lifetimeCycles 與 cyclesSinceService；wear=min(1,cyclesSinceService×0.001)，效率=max(0.1,1−0.9×wear)。健康分數=100×(1−wear)。wear≥0.6 產生一次持續性預警。

由 1013 mbar 起算，P(t)=1013×exp(−ln(1013/0.01)×效率×t/3)。t 是模擬秒。0.01 mbar 達標，12 秒逾時；邊界先判定達標，允許 1e−10 相對浮點容差。暫停時採樣繼續、t 不增加。剩餘次數為在目前模型下，後续可於 12 秒內達標的抽氣次數，不是日曆時間或經驗統計 RUL。

`locks[].pump` 包含 modelVersion、enabledAt、lifetimeCycles、cyclesSinceService、serviceGeneration、wear、health、remainingCycles、warning、fault、faultGeneration、recovering、cycle。cycle 凍結 efficiency，包含 id、purpose（production/dummy/test）、status、elapsedSeconds、startTime/endTime 及 pressures。

使用既有 `CVD/Snapshot` 與 `CVD/Event` Sparkplug bytes metrics，經 SVID/CEID JSON extension 原樣傳輸；不改既有 namespace。vacuum-* 事件進 cvd.events.v1，telemetry 的 payload.locks 進 cvd.telemetry.v1。每事件沿用 eventId、equipmentId、sourceEpoch、sequence、sourceTime，payload 有 pumpId、pump 快照、pressure、commandId。KLA-SIM-01 仍使用 metrology.*，不可混入 30 組泵浦。

新增命令 vacuum-service-channel（無片／大氣／隔離且全台停派下修復並測試回收通道；後續仍需整台保養）、vacuum-recover（targetId、cycleId、faultGeneration）、vacuum-test（maintenanceId、expectedGeneration），使用既有簽章、期限與命令冪等。maintenance-request 可帶 selectedPumpIds；省略表示既有一般保養。明確空陣列、重複或跨台 ID 拒絕。

GET /api/equipment/{equipmentId}/vacuum?pumpId=LL1&from=ISO&to=ISO 提供最新資料、counts、最多 720 桶趨勢及最近 100 筆範圍事件。from/to 最大 30 天。pressure_min/max 為各桶壓力極值、health 為平均分數。採樣保留 30 天，週期與服務證據独立保存。達標率分母僅 passed+failed，零分母顯示無資料。
