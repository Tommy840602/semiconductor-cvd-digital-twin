# OpenSpec Pull Request 工作流程

此專案的功能變更以 `openspec/config.yaml` 為準，使用 `spec-driven` schema。

## 變更流程

1. 先於 `openspec/changes/<change-id>/` 建立或更新 proposal、design、specs 與 tasks（依現有 OpenSpec schema）。
2. 所有文件以繁體中文撰寫；保留 OpenSpec 結構標題及 SHALL/MUST 英文關鍵字。
3. 從最新 `main` 建立 `feat/<change-id>` 或 `fix/<change-id>`；禁止直接提交功能變更至 `main`。
4. 實作應依照 tasks 及驗收規格進行，必要時補齊 Unit、Integration、E2E、Security 測試。
5. PR 說明應列出對應 change-id、需求摘要、風險、回滾方式、測試結果與未完成項目。未執行的測試不得標示通過。
6. Reviewer 檢查 OpenSpec 規格與程式變更一致、無敏感資訊洩露、部署及相容性風險可接受。
7. CI 與必要審查通過後，由專案負責人確認 Merge；後續依 OpenSpec 工作流程同步規格並封存變更。

## 最低驗證項目

- Go：針對改動模組執行 `go test -race ./...`。
- Java：於 `backend/` 執行 `mvn test`。
- Vue：於 `frontend/` 執行 `npm ci && npm run build`。
- Compose：執行 `docker compose -f compose.yaml -f compose.secs.yaml config --quiet`。
- Integration / E2E 僅使用隔離環境，不得對正式資料執行故障注入。

## Merge 保護

建議 GitHub Ruleset 要求 PR、必要審核、CI 通過及禁止直接推送 `main`。此文件本身不會自動建立 Ruleset；需在 GitHub Repository Settings 中配置。
