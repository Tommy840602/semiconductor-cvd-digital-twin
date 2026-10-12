# TiN CVD 數位孿生

Vue 3 JavaScript + Java 21 Spring Boot + Go SECS/GEM 模擬器／Sparkplug B 閘道器／獨立 MQTT–Kafka Bridge，搭配 Kafka、PostgreSQL、TimescaleDB、Redis。十台 TEL-CVDTIN-08A-01～10 各自獨立加工。鏈路為 SECS/GEM → Edge Gateway → MQTT（Sparkplug B）→ Bridge → Kafka → Data Platform。依指定拓樸提供功能模擬，不代表 TEL 原廠設備或配方。

新鏈路部署與回滾見 [SECS/GEM 部署](docs/cvd/SECS_GEM_DEPLOYMENT.md)，驗收與磁碟估算見 [驗收紀錄](docs/cvd/SECS_GEM_VALIDATION.md)。

## 啟動

安裝 Docker Desktop／Docker Compose 後，先依 [RBAC 部署說明](docs/cvd/RBAC_DEPLOYMENT.md) 建立初始 Leader 密碼秘密檔案，再於專案根目錄執行：

```sh
cp .env.example .env  # 僅首次建立，既有部署不要覆寫
python3 scripts/init-recovery-key.py
python3 scripts/prepare-secs-validation.py --production
docker compose -f compose.yaml -f compose.secs.yaml up -d --build
```

開啟 [儀表板](http://localhost:18000)，以 `cvd.leader` 和部署時指定密碼登入。Leader 的「帳號管理」可新增 Operator／Engineer。網站及 API 均需登入；API 為 [http://localhost:18080/api/equipment](http://localhost:18080/api/equipment)。初始沒有批次，選 LP 及 LL 後按「啟動新批次」。每批自動建立 25 片。網站及 API 僅綁定本機，資料服務不對主機開放連接埠。

### 映像拉取受阻時

可改用主機 Java 21、Maven、Go 1.26 以上、Node 20 以上建置，再以快取的 Alpine／Nginx 包裝執行映像：

```sh
GO_BIN=/path/to/go IMAGE_PREFIX=cvd-secs scripts/build-local.sh
docker compose up -d --no-build --pull never
```

此路徑仍將全部服務運行在 Docker，程式碼與標準 Dockerfile 相同。資料服務映像需預先存在；TimescaleDB 固定 2.28.2-pg16 的 digest。Java runtime 使用 Temurin 21.0.12.1。首次主機建置仍需存取套件來源及 Adoptium。

## 操作

- 3 LP 可選任一空 LL；LL 容量為 1 片，25 片批次位於 LP 載具。所有 PM 均為單片平行腔體。
- 三個 LP 可同時加工，依就緒批次輪流派片，共用四個 PM；完成片優先回收。
- PM 故障會停止新派片，其他正常 PM 完成加工及回片。
- 異常片由 EE 提出單片補做，PE 核准直接觸發；只執行所選步驟、保留已生長膜厚。合格回原槽後人工恢復派片；不合格留置。舊「完整重做」停用；報廢回收仍可使用。
- Leader 在帳號管理指定 EE／PE 部門，變更後需重新登入。詳見 [補做流程與部署](docs/cvd/RECOVERY_DEPLOYMENT.md)。
- 每片成功或報廢後經 LL 單片回填並送回原 LP 槽位，25 片均返回後完成批次。成功及報廢分開統計。
- Engineer／Leader 可分別修改預熱、穩定化、沉積、吹掃秒數，每步至少 1 整數秒，總和最多 3600 秒；僅在無進行中批次時接受。總時間自動加總，舊版總秒數 API 仍相容。
- 「受控停止」停止新派片，讓現有製程完成；不會強行移除晶圓。
- 點 LL 槽位查看晶圓履歷；「告警與操作」查看執行結果。接收不代表完成，逾時顯示結果未知。

## 資料與恢復

PostgreSQL 保存事件履歷、命令及批次快照；TimescaleDB 保存秒級數值；Redis 保存登入 session 並快取最新狀態；Kafka 保存 7 天重播資料。歷史查詢限最近 30 天，清理保留活動批次必要資料。機台資料斷線時標示最後更新時間。

設備與閘道器有持久化緩衝，事件透過 Kafka 確認後才釋放設備端佇列。超過 10 分鐘未確認時暫停模擬，待恢復後人工解除。重啟模擬器會保留晶圓位置並暫停，請人工確認恢復。

```sh
docker compose ps
docker compose logs --tail 100 backend gateway simulator
docker compose stop
docker compose start
```

停止不刪除資料。十台完整一致性備份、離線 state 遷移與全部卷還原使用 [部署文件](docs/cvd/SECS_GEM_DEPLOYMENT.md) 的工具；不要只停止 01 而讓其他設備繼續寫入。

僅在明確要清空本展示的全部資料時執行 `docker compose down -v`，再重新啟動；一般停止請勿加 `-v`。本次驗證不會清理其他專案的容器或資料。

## 驗證

```sh
(cd protocol && go test -race ./...)
(cd simulator && go test -race ./...)
(cd gateway && go test -race ./...)
(cd bridge && go test -race ./...)
(cd backend && mvn test)
(cd frontend && npm ci && npm run build)
docker compose -f compose.yaml -f compose.secs.yaml config --quiet
python3 scripts/prepare-secs-validation.py
docker compose -f .runtime/secs-compose.json up -d
```

整合測試只指向隔離專案的 21000／21080；不要對正式資料執行建立批次或故障注入測試。測試入口為 `tests/integration/fleet.py`、`fleet_recovery.py`、`secs_cancel.py`、`secs_transport.py`、`fleet_metrics.py`；設定 `CVD_TEST_API=http://127.0.0.1:21000/api`、`CVD_TEST_PASSWORD_FILE=.runtime/secrets/secs/leader-password`，量測另設 `CVD_TEST_COMPOSE=.runtime/secs-compose.json`。詳細結果見驗收紀錄。

協定契約見 [SECS/GEM profile](contracts/SECS_GEM_PROFILE.md)、[Sparkplug B](contracts/SPARKPLUG.md)。舊 OPC UA 分支僅供維護回滾，正式 Compose 使用 `CVD_PROTOCOL=secs`，不與新主鏈路同時派令。

整台預防保養、FOUP 暫存、四片 Dummy 驗證與人工復機，見 [保養操作說明](docs/cvd/MAINTENANCE.md) 與 [驗證紀錄](docs/cvd/MAINTENANCE-VALIDATION.md)。保養中不得以舊版 simulator 直接讀取新 checkpoint 回退。

首頁狀態時間軸、圓環圖與正式／Dummy 結案片數，見 [統計圖表操作與口徑](docs/cvd/HOMEPAGE_STATISTICS.md)。三圖與既有稼動率共用區間，不包含 Cp／Cpk。

多國語言使用、通知語言及部署回退：[MULTILINGUAL.md](docs/cvd/MULTILINGUAL.md)。

[KLA 模擬量測與 wafer map](docs/cvd/METROLOGY.md)

真空泵浦 PHM：[操作與回退](docs/cvd/VACUUM_PHM.md)、[模擬契約](contracts/VACUUM_PHM.md)。

- [OpenCV 視覺檢查與資料來源](docs/cvd/OPENCV_INSPECTION.md)
