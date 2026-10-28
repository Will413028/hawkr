# Hawkr 本機腳本

這個目錄放本機用的腳本：資料庫初始化、一次性維運工具，以及 CI gate 的檢查腳本。
本檔說明開發者手動會用到的幾支；其餘腳本（CI gate、smoke test、一次性工具）的用途與參數寫在各自檔頭的註解／docstring 裡。

## 快速開始

新機器 `git clone` 後，本地開發環境一律用專案根目錄的 `./dev.sh`：

```bash
cd backend-payuni
cp .env.template .env    # 填入實際值
./dev.sh up              # 啟動基礎設施與服務，並建立／補建所有資料庫
./dev.sh seed            # 套用 Go 服務的 Atlas migration + 匯入測試資料
```

`./dev.sh up` 會在 postgres 容器內執行本目錄的 `init-databases.sh`（冪等：只建缺少的資料庫，不動既有資料）。
Java 服務的 schema 由各服務啟動時的 Flyway 自動套用。

⚠️ `./dev.sh seed` 會 **DROP 並重建**所有 Go 服務的資料庫。

## 前置需求

### 1. Docker（OrbStack）

本地容器只跑在 OrbStack，由 `./dev.sh` 以 `docker-compose-local.yml` 啟動。

### 2. Atlas CLI

`./dev.sh seed` 與 Go 服務的 migration 需要 [Atlas](https://atlasgo.io/)：

```bash
# macOS
brew install ariga/tap/atlas

# Linux
curl -sSf https://atlasgo.sh | sh
```

### 3. PostgreSQL Client（選用）

只有在 host 上直接跑 `init-databases.sh` 或 `psql` 時才需要。要進本機資料庫，用容器內的 psql 就不必裝：

```bash
docker exec -it hawkr-postgres psql -U dbadmin -d postgres
```

## 腳本說明

### `init-databases.sh` - 建立資料庫

建立所有服務需要的 PostgreSQL 資料庫，並補上必要的 extension（`hawkr_product` 的 `pg_trgm`）。

**使用方式:**
```bash
./scripts/init-databases.sh [local|prod]     # 預設 local
```

**功能:**
- 檢查資料庫是否已存在（避免重複建立），對既有資料庫完全不寫入
- 依腳本內 `DATABASES` 陣列建立資料庫 —— **該陣列是資料庫清單的唯一真實來源**，新增服務時改那裡
- `local` 連 `localhost:5432`；`prod` 的 host 沒有預設值，必須以 `DB_HOST` 明確指定，且強制 `sslmode=require`
- 需要 `DB_PASSWORD` 環境變數；被 postgres 容器當 entrypoint 執行時會自動沿用 `POSTGRES_USER` / `POSTGRES_PASSWORD`

**範例:**
```bash
# 在 host 上對本機環境執行（需本機已安裝 psql）
DB_PASSWORD=<本機 postgres 密碼> ./scripts/init-databases.sh local
```

平常不需要手動跑：`./dev.sh up` 每次啟動都會在容器內重跑一次同一份腳本。

## Makefile 命令（資料庫相關）

在 `backend-payuni` 目錄下：

| 命令 | 說明 |
|------|------|
| `make db-init` | 建立所有資料庫（`make db-init-local` 為同一目標；內部呼叫 `./scripts/init-databases.sh local`） |
| `make atlas-validate-all` | 離線驗證所有 Go 服務的 migration 目錄完整性（`atlas.sum` 與檔案對得上），不需連 DB |
| `make atlas-validate-staged` | 同上，只驗有 staged `db/` 變更的服務（pre-commit 用） |

## 環境說明

### Local 環境

- **資料庫位置:** localhost:5432（`docker-compose-local.yml` 的 `hawkr-postgres` 容器）
- **啟動方式:** `./dev.sh up`
- **帳號密碼:** 見 `docker-compose-local.yml`

### Dev / Prod

遠端資料庫的 migration 一律由 CI/CD 的 migrate job 套用，禁止 SSH 手跑 DDL，也禁止從本機執行 `atlas migrate apply`。

## 目前支援的服務

以下 Go 服務使用 Atlas migration（有 `db/atlas.hcl` 配置）：

| 服務 | 資料庫 |
|------|--------|
| ai-service | hawkr_ai |
| crm-service | hawkr_crm |
| im-service | hawkr_im |
| invoice-service | hawkr_invoice |
| notification-service | hawkr_notification |
| review-service | hawkr_review |
| seller-service | hawkr_seller |
| social-service | hawkr_social |

現況以 `ls services/*/db/atlas.hcl` 為準。Java 服務不使用 Atlas，schema 由各服務的 Flyway 在啟動時套用。

## 常見問題

### Q: 服務起不來，說 database does not exist？

**A:** 多半是這台機器的 postgres volume 建立得比該服務早，init script 不會再跑一次。
執行 `./dev.sh up`（會在容器內重跑 `init-databases.sh` 補建），再 `./dev.sh seed` 套用 schema。

### Q: `./dev.sh seed` 失敗？

**A:** 依序檢查：

1. **Atlas CLI 是否安裝？** `atlas version`
2. **postgres 容器是否在跑？** `docker ps | grep hawkr-postgres`
3. **migration 目錄是否完整？** 在專案根目錄跑 `make atlas-validate-all`

### Q: 某個 Go 服務的 migration 想單獨套到本地？

**A:**

```bash
cd services/seller-service/db
atlas migrate apply --dir "file://migrations" \
  --url "postgres://dbadmin:<密碼>@localhost:5432/hawkr_seller?sslmode=disable"
```

不要用各服務 Makefile 的 `make migrate-apply`：它的 `--env dev` 連的是**共用的遠端 dev 資料庫**。

## 新增 Migration

**重要:** Go 服務的 migration 一律用 Atlas 產生，**禁止手寫、禁止手改**。`db/schema/schema.sql` 是唯一真實來源。

### 步驟：

1. **修改 schema 檔案**
   ```bash
   vim services/seller-service/db/schema/schema.sql
   ```

2. **自動產生 migration**（互動輸入 migration 名稱）
   ```bash
   cd services/seller-service && make migrate-diff
   ```

3. **重算 checksum**（漏了 CI 直接紅）
   ```bash
   cd db && atlas migrate hash
   ```

4. **套用到本地並測試**（回專案根目錄執行）
   ```bash
   cd ../../..
   ./dev.sh seed
   (cd services/seller-service && go test ./...)
   ```

5. **提交**（新的 migration 檔與 `atlas.sum` 一起，逐檔精確 add）
   ```bash
   git add services/seller-service/db/migrations/<timestamp>_<name>.sql \
           services/seller-service/db/migrations/atlas.sum
   ```

遠端由 CI/CD 在部署前的 migrate job 套用，不需要手動 apply。

## 團隊協作建議

### 新成員加入流程

1. Clone 專案
   ```bash
   git clone <repository-url>
   cd backend-payuni
   ```

2. 準備環境變數
   ```bash
   cp .env.template .env    # 填入實際值
   ```

3. 一鍵啟動（含建立資料庫）
   ```bash
   ./dev.sh up
   ```

4. 匯入 schema 與測試資料
   ```bash
   ./dev.sh seed
   ./dev.sh status          # 確認全部 healthy
   ```

### 日常開發

- **Pull 最新程式碼後:**
  ```bash
  git pull
  ./dev.sh seed      # Go 服務：套用新的 Atlas migration
  ./dev.sh rebuild   # Java 服務：重建有變更的服務（啟動時 Flyway 自動 migrate）
  ```

- **本地資料要整組清掉:**
  ```bash
  docker compose -f docker-compose-local.yml down -v   # ./dev.sh down 不轉傳 -v
  ./dev.sh up
  ./dev.sh seed
  ```

## 支援

如有問題請聯繫後端團隊或在專案 issue 中提出。
