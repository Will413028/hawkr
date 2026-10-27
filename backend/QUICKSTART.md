# Hawkr Backend 快速開始指南

## 🚀 新機器 git clone 後的設定

### 1. 安裝前置工具

```bash
# macOS
brew install postgresql atlas helm kubectl sqlc

# 安裝 k3s（Linux VM，需 root）
cd k8s && bash scripts/install-k3s-master.sh

# macOS 本地開發可用 Docker Desktop 內建 Kubernetes
# Docker Desktop → Settings → Kubernetes → Enable Kubernetes
```

### 2. Git hook

git hook 由 monorepo 根目錄的 pre-commit 管理：clone 之後在根目錄執行一次 `pre-commit install`（見根目錄的 `AGENTS.md`）。backend 的檢查在 CI（`.github/workflows/backend-*.yml`）。

### 3. 設定 kubeconfig

```bash
# 從 VM 下載 kubeconfig
scp root@<VM_IP>:/root/kubeconfig-hawkr.yaml ~/.kube/hawkr.yaml

# 設定環境變數
export KUBECONFIG=~/.kube/hawkr.yaml

# 驗證連線
kubectl get nodes
```

### 4. 初始化 K8s 基礎設施

```bash
cd k8s

# 一鍵初始化（建立 namespace + NetworkPolicy + 監控元件）
make init ENV=dev

# 建立 Secrets（從 .env 檔案注入）
cp ../.env.template ../.env  # 填入實際值
make secrets ENV_FILE=../.env
```

外部服務（PostgreSQL / Redis / RabbitMQ）透過 K8s Endpoints 指向 Infra VM，
設定檔在 `k8s/external-services/` 下（selector-less Service + 手填 Endpoints，
**PostgreSQL 不在叢集內**）。如需修改 IP，編輯對應 yaml 後重新 apply：

```bash
kubectl apply -f external-services/postgresql.yaml   # 或 redis.yaml / rabbitmq.yaml
```

### 5. 建立資料庫

```bash
cd backend-payuni

# 本地（localhost:5432）
DB_PASSWORD=<password> make db-init

# 遠端（Infra VM 上的 PG，強制 sslmode=require）
DB_HOST=<infra-host> DB_PASSWORD=<password> ./scripts/init-databases.sh prod
```

只建空資料庫，**schema 不用手動套**：Java 服務啟動時由 Flyway 自動 migrate；
Go 服務由 CI/CD `deploy-k8s.yml` 的 `migrate` job 跑 `atlas migrate apply`。

### 6. 部署服務

```bash
cd k8s

# 部署單一服務
make deploy SVC=gateway TAG=latest

# 部署所有服務
make deploy-all TAG=latest

# 先導部署（gateway + account + buyer）
make deploy-pilot TAG=latest
```

### 7. 驗證

```bash
# 查看所有服務狀態
make status

# 查看 Pod logs
kubectl logs -f deploy/gateway -n hawkr
```

---

## 📋 常用命令速查

### K8s 操作

```bash
cd k8s

# 部署
make deploy SVC=<service> TAG=<tag>   # 部署單一服務
make deploy-all TAG=<tag>             # 部署所有服務

# 偵錯
make status                           # Helm releases + Pod 狀態
make template SVC=<service>           # 渲染 Helm 模板（debug 用）
make template-all                     # 驗證所有模板

# 回滾
make rollback SVC=<service>           # 回滾到上一版

# 基礎設施
make init ENV=<env>                   # namespace + NetworkPolicy + 監控元件
make secrets ENV_FILE=<path>          # 建立/更新 Secrets
kubectl apply -f external-services/postgresql.yaml   # 更新外部服務 Endpoints
```

### 資料庫相關

```bash
# 建立所有資料庫（僅 CREATE DATABASE，不含 schema）
make db-init               # 本地環境，等同 ./scripts/init-databases.sh local

# 遠端環境
DB_HOST=<host> DB_PASSWORD=<pwd> ./scripts/init-databases.sh prod

# 🔄 本地完全重設（清掉 postgres data volume，重新 up 時 init script 會重建）
docker compose -f docker-compose-local.yml down -v
./dev.sh up

# 重建 Go 服務 schema + 載入測試資料（drop/recreate 各 DB 後套完整 migration 歷史）
./dev.sh seed
```

> schema 不靠手動命令：Java 走 Flyway（服務啟動時 migrate），Go 走 Atlas（CI/CD 的 `migrate` job）。
> ⚠️ `./dev.sh down` 不接受 `-v`（參數不會被轉給 docker compose），清 volume 一律用上面的 `docker compose ... down -v`。

### Go 服務開發

```bash
# 建置所有 Go 服務
make build-all

# 測試所有 Go 服務
make test-all

# 檢查程式碼
make vet-all

# 單一服務
cd services/seller-service
go build ./...
go test ./...
go test ./internal/service/ -run TestCertificationUsecase_SubmitForReview
```

### Protobuf

```bash
# 產生 proto code
make proto-gen

# 檢查 proto 檔案
make proto-lint
```

### 新增 Migration（禁止手動建立）

```bash
cd services/seller-service

# 1. 編輯唯一真實來源
vim db/schema/schema.sql

# 2. 自動產生 migration（只連一次性 scratch container，不連共用環境）
make migrate-diff          # 互動提示輸入名稱；等同 cd db && atlas migrate diff <name> --env dev

# 3. 校驗雜湊（漏跑會擋 CI）
cd db && atlas migrate hash

# 4. 驗證 migration 目錄完整性
atlas migrate validate --dir "file://migrations"
```

> atlas.hcl 只有 `dev` / `prod` 兩個 env（沒有 `local`、沒有 `stage`）。
> **禁止手動對 dev / prod 跑 `atlas migrate apply`** —— 遠端一律由 CI/CD 的 `migrate` job 套用。

---

## 🔧 常見問題

### Q: pull 新程式碼後，本地 schema 沒跟上？

```bash
# Java 服務：Flyway 在啟動時 migrate，重啟後看 log
./dev.sh restart {service}

# Go 服務：drop + recreate 專屬 DB 並套用完整 migration 歷史
./dev.sh seed
```

### Q: media-service 啟動失敗，出現 Flyway checksum mismatch？

**症狀：** `kubectl logs deploy/media-service -n hawkr` 顯示：
```
Migration checksum mismatch for migration version 1
-> Applied to database : -705590836
-> Resolved locally    : 86485880
```

**原因：** 已執行的 migration 檔案被修改過（V1 凍結，違反 Flyway 規則）

**解決方法：** 用 Flyway 官方 image 對該 DB 跑 `repair`（只改 `flyway_schema_history` 的 checksum，
不執行任何 DDL、不碰業務表），再 rollout 重啟。PG 在 Infra VM 上，不在叢集內，所以直接連它：
```bash
docker run --rm \
  -v $(pwd)/services/java/media-service/src/main/resources/db/migration:/flyway/sql:ro \
  flyway/flyway:10 \
  -url=jdbc:postgresql://${INFRA_HOST}:5432/hawkr_media \
  -user=dbadmin -password=${DB_PASSWORD} -baselineOnMigrate=true repair
kubectl rollout restart deploy/media-service -n hawkr
```

### Q: 想重新開始資料庫？

```bash
# 本地：清 volume 後重新 up，init script 會重建全部 DB
docker compose -f docker-compose-local.yml down -v
./dev.sh up
```

### Q: 某個 Go 服務的 migration 落後或失敗？

```bash
cd services/seller-service/db
atlas migrate status --env dev      # 看落後哪幾支、錯在哪支
```

遠端的補套用一律走 CI/CD（重跑 `deploy-k8s.yml`），不要手動 `atlas migrate apply`。

### Q: Pod 一直 CrashLoopBackOff？

```bash
# 查看 Pod logs
kubectl logs <pod-name> -n hawkr --previous

# 常見原因：
# 1. 缺少 Secret（檢查 k8s/values/<service>.yaml 的 secretRef）
# 2. 資料庫未建立（make db-init）
# 3. 依賴服務未啟動（檢查 gRPC 連線的 host/port）
# 4. RabbitMQ queue 屬性衝突（需到 RabbitMQ 管理介面刪除舊 queue）
```

### Q: 如何更新 Secrets？

```bash
# 編輯 .env 後重新注入
cd k8s
make secrets ENV_FILE=../.env

# 重啟受影響的服務以載入新 Secret
kubectl rollout restart deploy/<service> -n hawkr
```

---

## 🎯 下一步

- 完整文件總覽：[docs/index.md](docs/index.md)
- 開發規範（最高優先級）：[docs/開發規範/API 設計規範.md](docs/開發規範/API%20設計規範.md)、[docs/開發規範/開發工作流程.md](docs/開發規範/開發工作流程.md)
- 本機腳本（smoke test、資料同步等）：[scripts/README.md](scripts/README.md)
- Gateway swagger：契約 SSOT 是 handler 註解；生成物 `gateway/docs/` **不入版控**，本地要看就 `cd gateway && make swagger`（唯一消費者是 Apidog，由 CI 同步）。見 [docs/開發規範/Gateway Swagger 註解規範.md](docs/開發規範/Gateway%20Swagger%20註解規範.md)
