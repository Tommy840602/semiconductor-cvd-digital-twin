# 保養與 Dummy 驗證 v1

快照 maintenanceModelVersion=1 表示設備支援保養互鎖。maintenance 包含 id、generation、phase、selectedLP、parkedLotId、carrierId、loaded、loadedLotId、run、runs、blockers；Lot 增加 purpose、carrierId、parked，Wafer 增加 purpose 及 Dummy 的 targetPM。舊用途缺失預設 production。

指令 maintenance-request、maintenance-end、maintenance-carrier-load、qualification-start、maintenance-carrier-restore 使用既有簽署控制 envelope。parameters 包含 maintenanceId、expectedGeneration；load 另含 lp。首次申請無週期時省略週期；後續申請必須帶前週期版本。保養期間 resume 也必須帶相同 fence。只有 Engineer／Leader 可提交，Operator 唯讀。所有操作可用固定 commandId 重查、重送，不另建執行。

phase：draining → maintenance → waiting → qualifying → passed／failed → ready → released。passed 必須換回正式 FOUP 才成為 ready。配方變更及故障增加 generation，舊輪次不能用於復機。每輪四片各固定一 PM；每片正常搬送至原測試載具，不跨設備。

maintenance-* 事件的 payload.maintenance 為不可變完整週期證據，sourceEpoch／sequence／sourceTime 與其他事件相同。SECS 事件報告、Sparkplug CVD/Event Bytes、Kafka 保持原資料。快照 CVD/State 保存載具與晶圓位置；不得靠 Redis 恢復派片鎖。

Dummy 品質失敗不冒充設備故障；設備故障則仍須原 EE／PE／Leader 處置。maintenance hold 與 disposition hold 均解除才可派正式片。Dummy 成功／報廢不增加正式生產 KPI，但實際加工時間依既有 utilization 定義累計。
