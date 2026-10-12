# 完整異常處置契約 v1

本契約取代 RECOVERY.md 中「合格自動卸片」及一般 scrap/resume 的活動異常語義。

| 入口／狀態 | 舊行為 | 新行為 |
|---|---|---|
| fault（含空 PM） | 暫停派片 | checkpoint 先保存 incident、holdGeneration、dispatch hold，再發布事件 |
| wafer-supplement | EE→PE，固定 executionId | 保留，另綁 incidentId、排錯版本；成功後 awaiting_result 留 PM |
| 補做品質失敗／再故障 | exception | 保留膜厚，原單新版本，generation 遞增，舊確認失效 |
| scrap | Engineer 直接報廢 | 拒絕；僅簽章 wafer-scrap 核准意圖可執行 |
| disposition-confirm | 無 | PE 結果簽章綁 executionId/resultVersion；補做授權原 LL→LP；報廢須已回槽 |
| disposition-reject | 無 | 保留膜厚／hold；不得把已回槽報廢片重新補做 |
| equipment-release | 無 | Leader 簽章綁 releaseId、generation、完整 incident/evidence 集合；設備原子重驗 |
| resume/start | 一般控制 | 有 disposition hold 拒絕；日常無異常流程保留 |
| clear/configure | 清故障／更新 recipe | 可執行但不得清除 disposition hold |
| recovery-clock | 恢復時鐘、保持派片停 | 保留；不等同放行 |
| 重啟 | 暫停時鐘 | 保留所有 incident、generation、結果及去重 ID |

傳輸維持 CVD_CONTROL/ENVELOPE、SVID1000 狀態、SVID1001/CEID1 事件、Sparkplug CVD/State 與 CVD/Event、Kafka cvd.events.v1。新業務指令同時具有平台 envelope 簽章及 type+payload 的核准簽章。equipmentId 在每層比對；SECS transaction ID、Sparkplug seq 不作業務去重。

處置確認及放行均採固定 commandId。設備 checkpoint 中 release/result 證據才代表實際完成；ACK 或 Kafka 發布成功不能替代回槽／放行事實。所有業務來源、版本、簽名與執行證據保存 PostgreSQL，不受 TimescaleDB 30 天政策清除。

升級補充：dispositionMigrationVersion=1 識別舊版直接報廢／重做後已回槽但批次仍暫停的活動異常。`legacy-return-check` 僅在設備標示 Legacy+Returned 的來源使用，EE補齊排錯、PE核准後核對既有槽位／terminal結果，再由PE確認、Leader放行。此意圖不加工、不重新報廢、不搬回PM；缺少的舊膜厚資料標示未知而非補零。原命令／作者／歷史保留。
