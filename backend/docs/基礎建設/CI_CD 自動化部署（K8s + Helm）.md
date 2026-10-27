# CI/CD 自動化部署 詳細設計書

> Workflow：`.github/workflows/deploy-k8s.yml`
> 基礎設施：K3s + Helm + GHCR

---

## 1. 整體流程

```
git push (main / dev) ／ workflow_dispatch ／ 每週日排程
        │
        ▼
┌─── GitHub Actions (self-hosted runner) ──────────────────────┐
│                                                               │
│  ⓪ guard-dev-db-values（與下方 go-base 平行）                 │
│  └── 掃 k8s/values：dev 設定不得混入 PlanetScale-only 值      │
│                                                               │
│  ⓪ go-base                                                    │
│  ├── tag = hash(GO_VERSION + 全 monorepo go.mod/sum/work)     │
│  └── GHCR 已有同 tag 就跳過，否則建 Dockerfile.go-base        │
│         │                                                     │
│         ▼                                                     │
│  ① test-and-build（測試／偵測／編譯／推 image 同一個 job）     │
│  ├── 偵測變更的服務（輸出 changed_services / values_only）    │
│  ├── Go test（只測 changed_services 涵蓋的模組 + pkg）        │
│  ├── Java：./mvnw install -pl :svc1,:svc2,... -am -fae -T 1C │
│  │    shared 4 module 編 1 次；service 依 DAG 平行            │
│  │    過濾出 jar 產出成功的服務 → java_succeeded list        │
│  └── buildx build + push GHCR（-P 9 並行）                    │
│       Go：FROM go-base 的多 stage Dockerfile                  │
│       Java：layered jar Dockerfile（打包預編 jar）            │
│         │                                                     │
│         ▼                                                     │
│  ② migrate                                                   │
│  └── atlas migrate apply（Go 服務帶 db/atlas.hcl 者）         │
│         │                                                     │
│         ▼                                                     │
│  ③ deploy                                                    │
│  ├── Helm upgrade（Go 服務平行先部署，Java 服務後序列部署）    │
│  ├── rollout status 驗證                                     │
│  ├── Ingress 歸屬與重寫迴圈檢查                              │
│  └── 對外健康檢查（走真實 CF → origin 路徑）                  │
│                                                               │
│  ④ notify-failure（任一 job failure 才跑，GitHub-hosted）     │
│  └── Discord 告警（提醒本次變更會脫離 diff baseline）          │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
  K3s Cluster (hawkr namespace)
```

**重點變更（2026-06-07）**：
- Java build 從「每個服務的 Dockerfile 各自 mvn install shared 4 次」改為「**單一 mvn reactor 一次編完 shared + 服務 jar**」+「Dockerfile 不再自己編譯，只打包預編好的 jar」。
- shared/ 改動下：~25-35 min → ~5-8 min（**↓75%**）；單一 Java 服務改動：~92s → ~65s（**↓30%**）。
- Maven reactor 內建依 DAG 平行（-T 1C）；containerize 階段 -P 9 真平行（Dockerfile 不再跑 mvn，無 .m2 cache contention）。
- 微服務 runtime 屬性（獨立 pod / Deployment / HPA / image tag）**完全不變**，只動 build 流程。

---

## 2. Branch → 環境對應

| Branch | 環境 | Kubeconfig Secret | Concurrency |
|--------|------|-------------------|-------------|
| `main` | prod | `KUBECONFIG_PROD` | 不取消進行中的部署 |
| `dev` | dev | `KUBECONFIG_DEV` | 不取消進行中的部署 |

兩個環境都是 `cancel-in-progress: false`（concurrency group 依 `github.ref` 分開）：**同分支連續 push 會排隊依序跑，不會互相取消**。

這是刻意取捨——取消進行中的 run 會漏 build：push A 的 run 還沒推 image 就被 push B 取消，而 B 的 diff baseline 是 A，change detection 會認為 A 的改動已在 baseline 內而跳過，結果 A 的 image 從沒上 GHCR、叢集仍跑舊版。真要砍掉舊 run 用 `gh run cancel <id>` 手動處理。

觸發來源三種：
- `push`（`main` / `dev`）
- `workflow_dispatch`：`force_all=true` 強制重新 build 所有服務；`services=<svc1,svc2>` 只針對指定服務重 build + 部署（drift 補救用，免全 build，與 `force_all` 互斥且 `force_all` 優先）
- `schedule`：每週日 UTC 00:00 以 `force_all` 校齊一次，image tag 用 `${SHA}-weekly-${YYYYMMDD}`（新 tag 才會讓 helm 觸發 rollout、k3s 真的重拉 image），補 change detection 的漏網情境

---

## 3. 變更偵測

不是每次 push 都 build 全部 22 個服務（Go 11 + gateway + Java 10）。Workflow 只 build 有變更的服務。

### diff baseline

baseline 是 **`github.event.before`（本次 push 前分支所指的 SHA）**，不是「上次成功 run 的 SHA」。

用「上次成功」會有失敗迴圈：一旦有 run 被 cancel/failed，baseline 就停住不前進，之後每次 push 的 diff 一路累積回上次成功點 → rebuild 全部服務 → 撞 timeout → 又失敗，且該 API 查詢失敗時會誤判成 `force_all`。

代價是**失敗或被取消的 run，它的變更會永久脫離 baseline**（除非之後剛好有 commit 動到同一個服務目錄順便夾帶）。這一點不靠改 baseline 邏輯解決，靠 `notify-failure` 的 Discord 告警把人拉回來手動補（re-run 該 run，或 `gh workflow run deploy-k8s.yml -f services=<svc,...>`），最遲由每週日的 `force_all` 排程兜底。

`github.event.before` 取不到時（新分支首推 / force-push 失聯 / `workflow_dispatch`）退用 `HEAD^`；連 parent 都沒有才 fallback 成 `force_all`。

### 偵測邏輯

```
git diff <github.event.before> <本次 SHA>
        │
        ├── gateway/* 有改 → build gateway
        ├── services/<svc>/* 有改（排除 services/java）→ build 該 Go 服務
        ├── services/java/<svc>/* 有改 → build 該 Java 服務
        ├── services/java/{shared/,pom.xml,.mvn/,mvnw} 或 api/proto/* 有改
        │      → build 所有 Java 服務
        ├── pkg/* 有改（proto 生成檔以外）→ build 所有 Go 服務 + gateway
        ├── .go-version / Dockerfile.go-base / go.work / go.work.sum 有改
        │      → build 所有 Go 服務 + gateway（go-base image 內容變了）
        ├── api/proto/<domain>/* 或 pkg/proto/<domain>/* 有改
        │      → 掃描哪些 Go 服務 import 了該 proto → build 它們
        └── 都沒改 → 跳過 build（deploy job 仍會跑，套用監控資源，但不 helm upgrade 任何服務）
```

### Proto 依賴偵測

Go 服務的 proto 依賴用 `grep` 動態掃描 import（不需要手動維護對照表）：

```bash
grep -rh 'github.com/Will413028/hawkr/go-pkg/proto/' services/account-service --include='*.go'
# → proto/product, proto/payment → 這兩個 proto 有改，account-service 就要 rebuild
```

### values-only 變更

`k8s/values/<svc>.yaml` 或 `k8s/values/overlays/<env>/<svc>.yaml` 有改、但該服務沒有程式碼變更時，會被列進 `values_only_services`：**不 rebuild image，只跑 helm upgrade 套新設定**，image tag 從既有 release 的 `helm get values` 取當前值（推 `image.tag=<新 sha>` 會變成 ImagePullBackOff，因為那個 tag 根本沒 build）。`common.yaml` / `java-common.yaml` 不屬於任何服務，不觸發此路徑。

---

## 4. Build & Push

### 4.1 共通設定

- Registry：`ghcr.io/<org>/<repo>/<service>:<image_tag>` + `:latest`
  （`image_tag` 平時 = `github.sha`；排程 run = `${sha}-weekly-${YYYYMMDD}`）
- Cache：不使用 registry cache，layer 重用由 `ci-builder` 的本機 BuildKit cache 負責
  （2026-09-12 移除 `--cache-from` / `--cache-to`；恢復前提是 cache 來源先搬到同區 registry）
- 重試：每個 image build 最多 3 次嘗試，失敗間隔遞增（10s, 20s, 30s）

### 4.2 Go 服務 build

Go 服務的 Dockerfile 是多 stage：builder 階段 `FROM ghcr.io/<org>/<repo>/go-base:${GO_BASE_TAG}`（該 image 的 `/go/pkg/mod` 已含全 monorepo 依賴），最終 stage 為 alpine thin image；另有獨立的 `dev` stage 供本機 Air hot-reload 用，不依賴 go-base。

`go-base` 由 workflow 的 `go-base` job 負責：tag = `hash(GO_VERSION + 全 monorepo go.mod / go.sum / go.work / go.work.sum)`，GHCR 已有同 tag 就直接跳過，只有 Go 版本或依賴異動才重建（定義見 `Dockerfile.go-base`，防退化 lint 見 `scripts/lint-go-base.sh`）。

並行度 `xargs -P 9`。

### 4.3 Java 服務 build（Maven Reactor + Thin Dockerfile）

#### 4.3.1 為何重構

舊架構（每個 Java 服務 Dockerfile 自包含 mvn build）的瓶頸：

| 問題 | 影響 |
|---|---|
| 每個 Java 服務的 Dockerfile 各自編 4 個 shared module | shared 被重複編 N 次，即使 `.m2` cache hit 仍有 N×4 次 JVM 啟動 |
| `--mount=type=cache,sharing=locked` 防 .m2 寫衝突 → 並行度被迫壓到 `-P 2` | 服務幾乎序列化 build |
| shared/ 改動觸發全部 Java 服務 rebuild，舊路徑下每個容器都重編 shared | shared 全改情境 CI 25-35 min |

#### 4.3.2 新架構：Reactor + 只打包的 Dockerfile

**Stage A：Build Java jars (Maven Reactor)**（`test-and-build` job 內的一個 step，單一 mvn 進程，內建依 DAG 平行）

```bash
# -pl 由 changed_services 中的 Java 服務動態組成，例如：
(cd services/java && ./mvnw install \
  -pl :payment-service,:order-service,:product-import-service \
  -am -fae -T 1C -B -Dmaven.test.skip=true)
```

- `-pl <changed>`：指定變更服務
- `-am`：also-make，自動編譯依賴的 shared module
- `-fae`：fail-at-end，**個別服務編譯失敗不中止 reactor，其他服務照常產 jar**（blast radius 等價舊架構）
- `-T 1C`：每核 1 thread，Maven 自動依 DAG 排程平行編
- `-B`：batch mode（CI 無互動）
- `-Dmaven.test.skip=true`：跳過 test 編譯與執行。對齊舊 Dockerfile 行為（`.dockerignore` 排除了 `**/*Test.java`，舊 CI 不會編 test）；Java 測試由獨立的 `java-tests.yml`（PR-time 對 `services/java/**` 跑全 reactor `mvnw test`）負責，不阻擋部署路徑的 build artifact。⚠️ **`java-tests.yml` 只掛 `pull_request`**——直接 push 到 `dev`／`main` 的 Java 變更不會跑到任何 Java 測試。

DAG 排程示意（4 core runner）：

```
t=0   common-core / java-proto / observability / payuni-client  ← 4 shared 平行（4 thread）
t=15  payment / order / cart / shipping                          ← 服務 batch 1
t=60  coupon / aftersales / product / media                      ← 服務 batch 2
t=105 payuni-webhook / product-import                            ← 收尾
```

完成後過濾出 `services/java/*/target/*.jar` 存在的服務為 `java_succeeded`。任一服務沒產出 jar，`Java build status gate` step 會讓 `test-and-build` 轉紅，deploy 的 needs gate 因此擋下——避免「部分編譯失敗」時成功的那幾個被半推半就部署出去。

**Stage B：containerize**（Dockerfile 只負責打包，與 Go 服務同一個 `Build and Push` step 並行）

每個 Java 服務的 `Dockerfile` 不再自己跑 mvn，而是把 Stage A 產出的 fat jar 用 `layertools` 拆成 4 層再逐層 COPY（dependencies / spring-boot-loader / snapshot-dependencies / application）。deps 不變時 GHCR 不重推那幾層，每次通常只推 ~15 MB 的 application 層（整包 fat jar 是每次 ~100 MB）：

```dockerfile
# === Stage 1: 從 fat jar 拆出 layered 目錄 ===
FROM eclipse-temurin:21-jre-alpine AS extractor
WORKDIR /extract
COPY services/java/<service-name>/target/*.jar app.jar
RUN java -Djarmode=layertools -jar app.jar extract && rm app.jar

# === Stage 2: runtime ===
FROM eclipse-temurin:21-jre-alpine

LABEL service="<service-name>"

RUN addgroup -S hawkr && adduser -S hawkr -G hawkr
WORKDIR /app

# 順序：deps（rarely change）→ loader → snapshot → application（changes every PR）
# 用 --chown 一次設好權限，避免 RUN chown 產生額外 ~100 MB layer
COPY --chown=hawkr:hawkr --from=extractor /extract/dependencies/ ./
COPY --chown=hawkr:hawkr --from=extractor /extract/spring-boot-loader/ ./
COPY --chown=hawkr:hawkr --from=extractor /extract/snapshot-dependencies/ ./
COPY --chown=hawkr:hawkr --from=extractor /extract/application/ ./
USER hawkr

ENV JAVA_OPTS="-Xms256m -Xmx512m -XX:+UseG1GC -XX:MaxGCPauseMillis=100"
ENV SERVER_PORT=<port>
ENV GRPC_PORT=<grpc-port>

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${SERVER_PORT}/actuator/health || exit 1

EXPOSE ${SERVER_PORT} ${GRPC_PORT}
ENTRYPOINT ["sh", "-c", "exec java ${JAVA_OPTS} org.springframework.boot.loader.launch.JarLauncher"]
```

⚠️ 兩件容易踩到的事：① build context 是 repo root（`docker build -f services/java/<svc>/Dockerfile .`），jar 路徑才對得上；② `ENTRYPOINT` 走 `JarLauncher` 而不是 `java -jar`，因為 jar 已被拆散、目錄裡沒有那個 fat jar 檔了。

並行度：`xargs -P 9`。

#### 4.3.3 Build Failure Blast Radius

| 失敗類型 | 舊架構 | 新架構（含 `-fae`） |
|---|---|---|
| `common-core` / shared 編譯錯 | 每個 Java 服務的 docker build 全 fail | reactor 失敗 → 全部服務無 jar → 全部 image 不 build |
| 單一服務 src 編譯錯 | 該服務 docker build fail；其餘 push 成功 | 該服務無 jar；其餘 jar 照常產出 → 其餘 image push 成功 |
| 多服務同時編譯錯 | 各自 docker build fail | mvn log 集中列出所有失敗 module；其他成功者照常產出 |

**結論：blast radius 與舊架構等價**，但 debug log 更集中（單一 mvn 輸出）。

⚠️ image 層的 blast radius 等價，**deploy 層不等價**：只要有任一 Java 服務編譯失敗，`Java build status gate` 就讓整個 job 轉紅，本次 run **所有服務**（含編成功的 Java 與全部 Go）都不會部署。

#### 4.3.4 變更偵測（與舊架構一致）

`changed_services` 偵測邏輯在 `deploy-k8s.yml` 的「偵測變更的服務」step，完整規則見 §3：

```
services/java/{shared/,pom.xml,.mvn/,mvnw} 或 api/proto/* 改 → 全部 Java 服務標記
services/java/<svc>/* 改 → 該 Java 服務標記
services/<svc>/* 改 → Go 服務標記（排除 services/java）
pkg/*（proto 以外）/ .go-version / Dockerfile.go-base / go.work* 改 → 全部 Go 服務 + gateway
api/proto/<domain>/* 改 → 看哪些 Go 服務 import 該 proto 標記
```

差別在 Stage A 不再讓每個服務各自在 docker build 內跑一次 mvn，而是把列表組成 `mvn -pl` 參數一次 reactor build。

---

## 5. Helm 部署

### 5.1 Values 結構

```
k8s/
├── charts/hawkr-service/    # 共用 Helm chart（所有服務共用同一個 chart）
├── values/
│   ├── gateway.yaml              # 每個服務的 base values
│   ├── order-service.yaml
│   ├── ...
│   └── overlays/
│       ├── dev/
│       │   ├── common.yaml       # dev 環境共用覆蓋（如 replicas: 1）
│       │   └── java-common.yaml  # dev Java 服務覆蓋（如 JVM 記憶體）
│       ├── prod/
│       └── local/                # 本地 compose 用，CI 不讀
```

### 5.2 Values 疊加順序

```
helm upgrade --install <service> charts/hawkr-service -n hawkr \
  -f values/<service>.yaml              # ① 服務 base values
  -f values/overlays/<env>/common.yaml  # ② 環境共用覆蓋
  -f values/overlays/<env>/java-common.yaml  # ③ Java 額外覆蓋（僅 Java 服務）
  -f values/overlays/<env>/<service>.yaml    # ④ 服務+環境專屬覆蓋（如有）
  --set image.tag=<image_tag>           # ⑤ image tag
  --wait --atomic --timeout <3m|6m>     # ⑥ 等 readiness，失敗自動回滾
```

後面的 values 覆蓋前面的，最精確的設定放最後。服務是 Go 還是 Java，由 `values/<service>.yaml` 內有沒有 `serviceType: java` 決定（同時決定要不要疊 `java-common.yaml`、以及 timeout 取 3m 還是 6m）。

### 5.3 部署順序

```
第一批：Go 服務（啟動快，2-5 秒，各自獨立 release）
  → 全部平行 helm upgrade --wait --atomic --timeout 3m
  → 全部收斂才進第二批

第二批：Java 服務（啟動慢，~120 秒）
  → 逐一 helm upgrade --wait --atomic --timeout 6m
  → Go 服務已就緒，不會搶 CPU
```

### 5.4 自動 Rollback

```
helm upgrade 失敗（CrashLoopBackOff / 起不來 / 逾時）
  → --atomic 自動回滾到前一版，壞版本永不取代好版本

被 cancel 的 workflow 留下 pending-upgrade / pending-install / pending-rollback
  → 下次部署前自動偵測，從 helm history 找最後一個 deployed/superseded
     的 revision 明確回滾過去
```

⚠️ 清 pending 狀態時**不能用 `helm rollback <svc> 0`**（= 回到「目前指標的前一版」）：若卡住的 `pending-rollback` 本身就是「回滾失敗版本又失敗」留下的，目前指標可能仍停在那個失敗版本上，`0` 等於原地踏步。

### 5.5 部署後驗證

`deploy` job 最後三步都掛 `if: always()`（helm 那步失敗時反而最需要看這些，預設的 `success()` 會讓它們在最該執行時被 skip）：

1. **驗證 Rollout 狀態**（另需 `changed_services` 非空）：逐服務 `kubectl rollout status --timeout=300s`；失敗者自動 dump deployment events、pod events、previous container logs（crash trace）與當前 logs
2. **Ingress 歸屬與重寫迴圈檢查**（`k8s/scripts/check-ingress-ownership.sh`）：Ingress 是唯一一種「壞掉但每個既有檢查都綠」的資源——兩張 Ingress 搶同一個 host 時總有一張贏，pod Ready、helm 成功、HTTP 也通，沒有任何地方會轉紅。這步改用症狀判斷（resourceVersion 持續變動、同 host 多張 Ingress）
3. **對外健康檢查**：從 runner 打該環境的 `GATEWAY_URL`（讀 `k8s/values/overlays/<env>/gateway.yaml`，不硬編第二份）走完整 CF → origin → gateway 路徑打 `/api/v1/health`，重試 6 次 × 10 秒。它抓得到叢集內檢查全部看不見的東西：TLS 終結壞掉、ingress controller cache、CF 設定漂移、DNS 指錯

---

## 6. GitHub Secrets

| Secret | 說明 |
|--------|------|
| `KUBECONFIG_PROD` | Prod K3s kubeconfig（base64） |
| `KUBECONFIG_DEV` | Dev K3s kubeconfig（base64） |
| `GHCR_PAT` | GitHub Container Registry Token |
| `DB_PASSWORD_DEV` | dev migrate 用的 DB 密碼（prod 不走 repo secret，見下） |
| `PRICEBOOK_MODEL_R2_*` | pricebook-service 取 model.json 用的 R2 憑證（`ACCOUNT_ID` / `ACCESS_KEY_ID` / `SECRET_ACCESS_KEY` / `BUCKET_NAME`） |
| `ASTRA_DISCORD_WEBHOOK_URL` | `notify-failure` 的 Discord webhook（未設定時只印 warning，不讓 workflow 失敗） |

Helm values 中的敏感資料（DB 密碼、JWT Secret 等）透過 K8s Secret 管理，不在 CI/CD 中處理。

**prod migrate 的連線資訊刻意不另存一份**：`DB_HOST` 從 `k8s/values/overlays/prod/common.yaml` 讀、`DB_USER` / `DB_PASSWORD` 用 `KUBECONFIG_PROD` 現場向叢集的 `hawkr-db` secret 撈。理由是兩邊分開硬編一定會漂移——2026-07-12 PlanetScale 遷移漏改 `atlas.hcl`，CI 對著已退場的舊 DB 空轉回報假綠燈；`DB_PASSWORD_PROD` 這份 repo secret 也曾在轉密碼時只改 kubectl 沒同步而炸掉。

### 6.1 GitHub Environment

`migrate` job 在 `main` 分支時帶 `environment: production`。⚠️ **這不構成審批停頓**——該 environment 的 `protection_rules` 是空的（`gh api repos/<org>/<repo>/environments` 可查），prod migration 目前是**全自動、無人工 approve** 就直接 apply 的。要真的加閘門，得先去 repo Settings → Environments 設 required reviewers。

---

## 7. 新增服務

### 7.1 Go 服務

1. 建立 `services/<name>/Dockerfile`（builder stage `FROM go-base`，見 §4.2）
2. 把模組加進 `go.work` 的 `use (...)`
3. 建立 `k8s/values/<name>.yaml`（複製現有服務修改 port、env）
4. 如需環境覆蓋，建立 `k8s/values/overlays/<env>/<name>.yaml`
5. Push — workflow 自動偵測新 Dockerfile 並 build + deploy

不需要修改 workflow 本身。

### 7.2 Java 服務

額外步驟（除 7.1 的 values 相關步驟外）：

1. 在 **aggregator pom（`services/java/pom.xml`）** 的 `<modules>` 新增 `<module><name>-service</module>`
   （路徑相對於該檔，不要寫成 `services/<name>-service`）
   - 若漏加，reactor 編不到該服務 → CI Stage A 不會產 jar → image 不 build
2. `services/java/<name>-service/Dockerfile` 用 §4.3.2 的 layered 範本，不要再寫 mvn build stage
3. Push — workflow 自動偵測並走 reactor 流程

---

## 8. 常用操作

```bash
# 手動觸發部署（GitHub UI 或 CLI）
gh workflow run deploy-k8s.yml --ref dev

# 強制 rebuild 所有服務
gh workflow run deploy-k8s.yml --ref dev -f force_all=true

# 只重 build + 部署指定服務（deploy drift 補救；--ref 必須與目標環境對齊）
gh workflow run deploy-k8s.yml --ref dev -f services=buyer-service,account-service

# 查看部署狀態
gh run list --workflow=deploy-k8s.yml --limit=5

# 手動 rollback 特定服務
helm rollback <service> -n hawkr

# 查看 K8s 狀態
kubectl get pods -n hawkr
kubectl rollout status deployment/<service> -n hawkr
```

---

## 9. 本地開發 Build 流程

### 9.1 Go 服務

維持 Air hot-reload，改完自動生效，無需重 build。

### 9.2 Java 服務

統一走 `./dev.sh rebuild <java-service>`：

```
./dev.sh rebuild payment-service
        │
        ├── (cd services/java && ./mvnw install -pl :payment-service -am -T 1C -Dmaven.test.skip=true)
        │       ↑ 主機跑 mvnw，產出 services/java/payment-service/target/*.jar
        │
        ├── docker compose build payment-service
        │       ↑ Dockerfile 不編譯，只把上一步的 jar 拆層打包（見 §4.3.2）
        │
        └── docker compose up -d --force-recreate payment-service
```

**前置需求**：

| 項目 | 說明 |
|---|---|
| JDK 21 | macOS：`brew install --cask temurin@21`；驗證 `java -version` |
| Maven | **不需要手動裝**，使用 `services/java/mvnw` 自動下載 Maven 3.9.x |
| 本地 `.m2` | 第一次 `(cd services/java && ./mvnw install)` 會下載依賴（~5 min），後續快取 |

### 9.3 為何不延用「Docker 內 mvn」做本地開發

- 主機跑 mvnw：增量編譯快（30-60s vs 容器內 90-120s）
- IDE 整合：IntelliJ 直接讀 reactor pom，code completion / refactor 跨 service 可用
- CI 與本地 build 路徑一致（同一個 reactor + 同一份 Dockerfile）

---

## 10. Root Aggregator pom.xml

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>

    <groupId>io.github.will413028.hawkr</groupId>
    <artifactId>hawkr-backend-aggregator</artifactId>
    <version>1.0.0</version>
    <packaging>pom</packaging>
    <name>Hawkr Backend (Aggregator)</name>
    <description>Maven reactor 聚合 pom，用於跨服務一次性編譯。各服務仍各自繼承 spring-boot-starter-parent。</description>

    <modules>
        <!-- shared modules（4 個） -->
        <module>shared/common-core</module>
        <module>shared/java-proto</module>
        <module>shared/observability-common</module>
        <module>shared/payuni-java-client</module>

        <!-- microservices（10 個） -->
        <module>payment-service</module>
        <module>media-service</module>
        <module>product-service</module>
        <module>order-service</module>
        <module>cart-service</module>
        <module>coupon-service</module>
        <module>shipping-service</module>
        <module>aftersales-service</module>
        <module>payuni-webhook-service</module>
        <module>product-import-service</module>
    </modules>
</project>
```

> aggregator pom 位於 `services/java/pom.xml`，`<module>` 路徑皆相對於該檔。

⚠️ **要盤點「有哪些服務」時一律現查 `ls services/java/`（Java）＋ `go.work` 的 `use (...)`（Go），不要背誦或沿用任何文件裡的清單**——本檔與各處註解的服務數都可能落後於實際新增。

特性：
- **純 aggregator**，沒有 `<dependencyManagement>` / `<build>`，**不影響服務 pom 的繼承鏈**（services 仍 `<parent>spring-boot-starter-parent</parent>`）
- 新增 Java 服務需在 `<modules>` 補一行
- 不在 service pom 內被引用，個別服務獨立 build 仍可用（`cd services/java/payment-service && ../../mvnw install`）

---

## 11. Rollback（部署後回退）

**部署當下失敗**（CrashLoopBackOff / readiness 不過 / 逾時）不需要人工回退——`helm upgrade --wait --atomic` 已自動回滾到前一版，見 §5.4。本節講的是**部署成功、但新版本行為有問題**時怎麼回去。

### 11.1 立即止血：helm rollback 到前一個 revision

image tag 是以 `--set image.tag=<sha>` 寫進 release values 的（`deploy` job 的 `helm upgrade`），因此回退 revision ＝ 連 image 一起回到舊 sha，不需要重 build。

```bash
# 1. 找最後一個狀態為 deployed 的 revision
#    不要用 `helm rollback <service> 0`，理由見 §5.4
helm history <service> -n hawkr

# 2. 回退（Java 服務啟動慢，timeout 放寬）
helm rollback <service> <revision> -n hawkr --wait --timeout 6m

# 3. 確認 pod 與實際生效的 image tag
kubectl rollout status deployment/<service> -n hawkr
helm get values <service> -n hawkr -o json | jq -r '.image.tag'
```

`kubeconfig` 依環境取用（dev / prod 分別對應 `KUBECONFIG_DEV` / `KUBECONFIG_PROD`，見 §6）。

### 11.2 正式回退：revert commit 走 CI/CD

`helm rollback` 只改叢集狀態、不改版控——下一次偵測到該服務有變更的 deploy 會把問題版本再推上去。止血後必須把修正走正規路徑落回版控：

1. `git revert <bad-commit>` → PR base `dev` → merge
2. CI 依變更偵測自動重 build + 部署；若該服務這次沒被偵測到（例如只有 values 改動）用
   `gh workflow run deploy-k8s.yml --ref <dev|main> -f services=<service>`（`--ref` 必須與目標環境對齊）

⚠️ 舊 image 不需要自己重建：image tag 以 commit sha 為基底、一直留在 GHCR，revert 後的新 build 只是換一個新 sha。

### 11.3 rollback 不會回退的東西

- **DB schema**：Java 走 Flyway（服務啟動時執行）、Go 走 Atlas（`migrate` job，是 `deploy` job 的 needs 前置），兩者都是 append-only forward-only。回退程式碼**不會**回退已套用的 migration。若要回退的版本跨越了 migration，先確認舊程式碼能在新 schema 上跑（append-only 的加欄位通常可以，改欄位語意／改型別則不行）。
- **已發出的 MQ 事件**與**已寫入的業務資料**：回退程式碼不會撤銷它們。
- **ConfigMap / Secret 的非 Helm 來源改動**：由 `k8s/values/**` 以外的路徑（例如手動 `kubectl patch`）改出來的值不在 release revision 內，rollback 不會還原。

---

## 12. 效能對比

| 情境 | 舊架構 | 新架構 | 改善 |
|---|---|---|---|
| 只改 1 個 Java 服務 src | ~92s | ~65s | ↓30% |
| 改 shared/ → 9 服務全 rebuild | 25-35 min | 5-8 min | ↓75% |
| 改 proto → 5 個服務受影響 | 10-15 min | 3-5 min | ↓70% |
| Go 服務 build（不受新流程影響） | 同 | 同 | — |

實測數據以實際 CI run 為準（部署後追蹤 1 週驗證）。
