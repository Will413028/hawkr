# CI 本機等價對照（ci-local-map）

快照 commit `e5d5a8d8e`。`.github/workflows/` 共 26 支 workflow。每支一節，每個 job 一列；一個 job 內有多個 step 時，指令欄依 step 順序列出。基線在專用的 detached worktree 跑（見最後一節）。

- 本檔供計畫 `2026-10-24-portfolio-public.md` 的步驟 3、4、5、7 驗收重跑：每一列在改名後的樹上重跑，結果與「基線」欄比較，不得多出失敗。
- 欄位「基線」只寫結論（PASS／FAIL／NOT RUN）；耗時、log 名稱、失敗細節在最後一節。
- 指令都從 repo 根目錄 `$ROOT` 出發，需要換目錄時寫 `cd`。
- 指令裡的 `${N}` 是目前的套件前綴：先 `N=$(ls packages | sed -n 's/_api$//p')`（步驟 3 之前是 `<舊名>`，之後是 `hawkr`）。本檔不寫舊名明文。

## 共同前置

| 代號 | 內容 |
|---|---|
| 依賴 | 根目錄 `CI=true pnpm install --frozen-lockfile`（pnpm 與 Node 看根 `package.json` 的 `packageManager`、`.nvmrc`）。Flutter 的每個 App 與 package 各自 `fvm flutter pub get`（版本看各目錄 `.fvmrc`）。 |
| ENV-BUF | `export PATH=~/.local/bin:$PATH`，再 `buf --version` 對照 `api/.buf-version`（Homebrew 的 buf 較新，會誤報 drift）。需要連 BSR（`buf.build/googleapis/googleapis`），離線時看 wiki `local-runbook.md`「離線 buf drift」。 |
| ENV-DOCKER | `export DOCKER_CONTEXT=desktop-linux DOCKER_HOST=$(docker context inspect desktop-linux -f '{{.Endpoints.docker.Host}}')`；先 `DOCKER_CONTEXT=desktop-linux docker info` 確認 Docker 在跑。`make sqlc` 會先 `rm -rf` 各模組的 `internal/store`，Docker 沒起來會把生成檔刪光。 |
| ENV-WEB | `export API_BASE_URL=http://127.0.0.1:9/api/v1 NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:9/api/v1`（website 預設指向舊公司的 test-api）。 |
| ENV-JAVA | `export JAVA_HOME=/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home`（預設 JDK 在 testCompile 崩 `TypeTag :: UNKNOWN`）。 |
| ENV-GOLINT | CI 固定 golangci-lint `v2.13.2`、Go 看 `backend/.go-version`（1.27.1）。本機 Homebrew 的版本較新（2.14.0）。要得到與 CI 相同的版本：`GOTOOLCHAIN=go1.27.1 GOBIN=<目錄> GOWORK=off go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2`，執行時也帶 `GOTOOLCHAIN=go1.27.1`（用本機預設的 go1.27.2 編譯會在載入標準庫時報「export data version 5 is greater than maximum supported version 4」）。這是從原始碼編的，不是 CI 用的釋出檔。 |
| ENV-REALPATH | `backend-sqlc-generated-diff` 的腳本用 GNU `realpath --relative-to`，macOS 的 `/bin/realpath` 沒有這個旗標：把 `grealpath`（coreutils）以名稱 `realpath` 放進 PATH 最前面。 |
| 不跑 | e2e、Playwright、`storybook:test`、`backend-e2e-verify`、`scripts/characterize.sh`：見「不在本機」一節。 |

Go 的模組有兩種執行方式：單體（`backend/go.mod`，CI 設 `GOWORK=off`）與 `go.work` 的成員（`gateway`、`pkg`、`services/*`）。下列指令已依各 workflow 寫明。

## 對照表

### `_flutter-quality.yml`

可重用 workflow，只由 `buyer-ci.yml`、`seller-ci.yml` 以 `uses:` 呼叫，自己沒有觸發條件。它的三個 job 對到下面 `buyer-ci`、`seller-ci` 各自的列，不另跑。

| job | 呼叫端與輸入 | 本機等價 | 基線 |
|---|---|---|---|
| `static-checks` | buyer-ci、seller-ci（`app`） | 見 `buyer-ci.yml`、`seller-ci.yml` 的 `quality / Static checks` | 隨呼叫端 |
| `tests` | 同上；seller 另帶 `--concurrency=4` | 見呼叫端的 `quality / Unit and widget tests` | 隨呼叫端 |
| `generated-code` | 兩者都帶 `check-generated: true` | 見呼叫端的 `quality / Generated code` | 隨呼叫端 |

### `admin-ci.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `verify` Lint, test, build | `admin/**`、`api/**`、`packages/${N}-{api,twaddr,ui-text}-ts/**`、根 `package.json`／`pnpm-*`／`.nvmrc`／`patches/**` | `cd $ROOT/admin`<br>`pnpm lint`<br>`pnpm test`<br>`pnpm build` | 依賴 | PASS（lint、test 1160 項、build） | `pnpm build` 會改寫 `admin/next-env.d.ts`（追蹤中的檔），驗收後 `git restore admin/next-env.d.ts`。lint 有 27 個 biome warning，不影響結論。 |

### `api-contract.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `buf` buf lint／breaking | `api/**`、兩個 api 套件、根 pnpm 檔、image preset 檔 | `buf lint api`<br>googleapis 版本一致：CI 的 `awk`／`grep` 區塊（見 workflow）原樣貼進 shell 即可<br>`buf breaking api --against ".git#ref=<BASE>,subdir=api" --exclude-imports`，基線用 `<BASE>=e5d5a8d8e~1` | ENV-BUF、BSR 網路 | PASS（三個 step） | CI 的 `<BASE>` 是 PR 的 base 或 push 的 before。驗收時 `<BASE>` 用改名前的 commit。`api/buf.yaml` 設了 `ignore_unstable_packages: true`，`v1alpha1` 的 package 不受 breaking 檢查，所以步驟 3 改 proto package 之後對改名前的 commit 仍是 PASS（步驟 3 驗收量測）；package 改名由 `buf lint` 的目錄對應與四個生成端的 drift 檢查抓。 |
| `dart` Dart 套件（`packages/${N}_api`） | 同上 | 新 worktree 第一次先暖機：<br>`cd $ROOT/packages/${N}_api`<br>`dart pub get; echo '' \| dart run protoc_plugin:protoc_plugin >/dev/null 2>&1; echo '' \| dart run connectrpc:protoc-gen-connect-dart >/dev/null 2>&1`<br>`../../api/generate.sh buf.gen.yaml`，再 `git status --porcelain -- .` 必須為空<br>`fvm dart analyze`<br>`find lib test -name '*.dart' -not -path 'lib/gen/*' -print0 \| xargs -0 fvm dart format --output=none --set-exit-if-changed`<br>`fvm dart test` | ENV-BUF、BSR 網路、`.fvmrc` 的 Flutter | PASS（drift、analyze、format、test 四個 step） | 不要跑 `dart format lib`（會格式化 `lib/gen`）。沒有暖機，第一次生成會 `signal: killed` 並清空 `lib/gen`，要 `git restore -- packages/${N}_api/lib/gen`。 |
| `ts` TS 套件（`packages/${N}-api-ts`） | 同上 | `pnpm --filter @${N}/api gen:api`，再 `git status --porcelain -- packages/${N}-api-ts/src/gen` 必須為空<br>`pnpm --filter @${N}/api exec tsc --noEmit`<br>`pnpm --filter @${N}/api exec biome check .`<br>`pnpm --filter @${N}/api test` | ENV-BUF、BSR 網路、依賴 | PASS（四個 step） | |
| `presets` 圖片 preset 常數 drift | 同上 | `node backend/workers/image-gateway/scripts/gen-client-presets.mjs`，再 `git status --porcelain -- website/lib/media/image-presets.generated.ts admin/src/utils/image-presets.generated.ts packages/${N}_api/lib/image_presets.g.dart` 必須為空 | Node（`.nvmrc`） | PASS | |

### `backend-contract-tests.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `template-contract` notification template↔payload contract | `backend/pkg/mq/**`、`backend/services/notification-service/{db,internal/worker}/**`、Java 的 `mq`／`domain/event`／`service/dto`、`backend/tests/contract/golden/**` | `cd $ROOT/backend/services/notification-service`<br>`make test-integration`（即 `go test -tags=integration ./... -v`，`go.work` 模式） | ENV-DOCKER（testcontainers） | PASS | 指令與 `backend-integration-tests` 的 `services/notification-service` 矩陣項完全相同，兩列共用一次執行結果。 |

### `backend-gateway-json-contract.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `contract` 對外 JSON 與規格一致 | `backend/gateway/**/*.go`、`backend/scripts/gateway-json-contract-*`、`backend/api/openapi/openapi.yaml` | `cd $ROOT/backend`<br>`pip install pyyaml==6.0.2`（或已有 PyYAML）<br>`python3 scripts/gateway-json-contract-check.py --stats`<br>`cd gateway && GOWORK=off go test ./internal/router -run TestEveryRouteIsInOpenAPISpec -count=1` | Python（CI 用 3.13，本機 3.14）、PyYAML | PASS（兩個 step） | |

### `backend-go-checks.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `go-checks` Go 檢查（gofmt／vet／test／tidy） | `backend/**/*.go`、`go.mod`／`go.sum`、`go.work`、`.go-version`，但排除 `backend/cmd/**`、`backend/internal/**`、`backend/go.mod`、`backend/go.sum`（單體另有 workflow） | `cd $ROOT/backend`<br>gofmt：`test -z "$(git ls-files -z '*.go' \| xargs -0 gofmt -l)"`<br>下列三步的模組清單 `MODS="gateway pkg services/ai-service services/crm-service services/im-service services/invoice-service services/notification-service services/pricebook-service services/review-service services/seller-service services/social-service"`（CI 依 diff 挑模組，沒有 base 時測全部；本機一律全部）<br>vet：`for m in $MODS; do (cd $m && go vet -tags integration ./...); done`<br>test：`first=1; for m in $MODS; do extra=; [ $first = 1 ] && extra=--reset && first=0; (cd $m && go test -json ./...) \| python3 scripts/check-test-skips.py --module $m $extra; done; printf '' \| python3 scripts/check-test-skips.py --final`<br>tidy：`for m in $MODS; do (cd $m && go mod tidy); done; git diff --exit-code -- '**/go.mod' '**/go.sum'` | Go（`backend/.go-version`） | PASS（gofmt、vet、test 加未登記 skip 檢查、tidy 四個 step） | 這一列「跑全部模組」是 CI 在動到 `pkg` 時的行為，涵蓋最嚴。`go mod tidy` 會改檔，只在基線 worktree 跑；失敗時 `git restore` 該 worktree。 |

### `backend-integration-tests.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `changes`（paths-filter，決定矩陣） | `backend/pkg/**`、八個 `backend/services/*-service/**` | 無對應：手動觸發（`workflow_dispatch`）時 `ALL=true`，矩陣是 `pkg` 加八個服務。本機直接跑下一列的全部矩陣項 | 無 | NOT RUN（選取邏輯，無本機意義） | 除 `_flutter-quality.yml`（可重用）外，其餘 25 支都已有 `workflow_dispatch`。 |
| `integration` `<module>` integration（矩陣：`pkg`、`services/{seller,ai,pricebook,crm,notification,social,im,invoice}-service`） | 同上 | 對每個模組：<br>`cd $ROOT/backend/<module>`<br>`make test-integration`（`go test -tags=integration ./... -v`，`go.work` 模式）<br>一次只跑一個模組 | ENV-DOCKER（testcontainers） | FAIL（`services/pricebook-service` 兩項測試失敗；`pkg` 與其餘七個服務 PASS） | 負載高時 testcontainers 啟動逾時是假失敗：加 `-p 2` 重跑，再單獨重跑該套件。`review-service` 沒有 `test-integration` target，不在矩陣內。pricebook 的失敗可重現（單獨重跑同樣兩項）：`TestRollupBatch_CommodityMappedL3GetsPriceWithItsOwnLayerAndUnit`、`TestRollupBatch_OneCommodityFansOutToMultipleL3`，斷言 `expected: "ready"`／`actual: "unavailable"`；測試把交易日寫死為 2026-09-10、2026-09-11，時鐘用 `time.Now()`，資料隨日期變舊而變成 unavailable，是與日期相關的測試，不是改名或負載造成的。 |

### `backend-java-tests.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `reactor-test` full reactor unit tests | `backend/services/java/**`、`backend/scripts/check-surefire-it-includes.py` | `cd $ROOT/backend/services/java`<br>`./mvnw test -B -fae` | ENV-JAVA、Maven 中央庫網路 | PASS（全量跑 1 項時序測試逾時，單獨重跑通過） | 失敗時單獨重跑：`./mvnw -B -pl <服務> -am test -Dtest=<類別> -Dsurefire.failIfNoSpecifiedTests=false`。只在服務目錄跑會吃 `~/.m2` 過期的 shared 模組而編譯失敗。 |
| `integration-test` integration tests (testcontainers) | 同上 | `cd $ROOT/backend && python3 scripts/check-surefire-it-includes.py`<br>`cd $ROOT/backend/services/java && ./mvnw test -B -fae -Dgroups=integration -DexcludedGroups=`<br>執行後計數：`find . -path '*/target/surefire-reports/*.txt' -exec cat {} + \| grep -oE 'Tests run: [0-9]+' \| awk '{s+=$3} END {print s+0}'` 必須 ≥ 100 | ENV-JAVA、ENV-DOCKER | PASS（surefire 檢查通過；integration 全綠，本輪新產生的報告共 229 項測試） | 計數要在乾淨的 `target/` 上算：先跑過 reactor 的話，舊報告會讓總數虛高（基線上虛高為 2862，只用比 reactor 新的報告才是 229）。 |

### `backend-migration-inventory.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `check` 舊服務的去向是否齊全 | `backend/services/**`、`backend/gateway/**`、遷移計畫目錄、`check-migration-inventory.py` | `python3 backend/scripts/check-migration-inventory.py` | Python | PASS | |

### `backend-monolith.yml`

單體（`backend/go.mod`）全部在 `GOWORK=off` 下跑：`cd $ROOT/backend && export GOWORK=off`。

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `lint` golangci-lint（一般／integration） | `backend/{cmd,internal}/**`、`go.mod`／`go.sum`、`.golangci.monolith.yml`、`Makefile` 與跨端生成物（CodeMeta、api） | `GOTOOLCHAIN=go1.27.1 <v2.13.2 的 golangci-lint> run --config=.golangci.monolith.yml ./cmd/... ./internal/...`<br>`GOTOOLCHAIN=go1.27.1 <v2.13.2 的 golangci-lint> run --config=.golangci.monolith.yml --build-tags=integration ./cmd/... ./internal/...` | ENV-GOLINT | FAIL（用 CI 固定的 v2.13.2：一般 22 項、integration tag 35 項） | 另以本機的 2.14.0 跑過一般那一輪：24 項，標註 `本機版本較新，非 CI 結論`。內容為 exhaustive、gocritic、gosec、ineffassign、staticcheck、unused，integration tag 另有 errcheck、errorlint，分布在 `seller`、`identity/applerevoke`、`social`、`httpapi/*` 的檔。可重現：兩個 linter 版本都報、第二輪重跑結果相同。這不是改名造成的，步驟 3 之後只比較「項數與位置不增加」。 |
| `check` gofmt／vet／test／tidy | 同上 | gofmt：`test -z "$(gofmt -l cmd internal)"`<br>vet：`go vet ./... && go vet -tags integration ./...`<br>test：`go test -race ./...`<br>integration：`go test -race -timeout 15m -tags integration ./...`<br>Connect drift：`make connect`，再 `git status --porcelain -- internal/httpapi/gen internal/legacy/gen` 必須為空<br>sqlc drift 與 query 隔離：`make sqlc`，再 `git status --porcelain -- internal/modules` 必須為空；接著 `docker run --rm -v "$PWD/internal/arch/testdata/sqlc-foreign-schema:/work" -w /work "$(sed -n 's/^IMAGE="\(.*\)"$/\1/p' scripts/gen-sqlc.sh)" generate` 的輸出必須含 `schema "seller" does not exist`<br>jet drift：`make jet`，再 `git status --porcelain -- internal/modules` 必須為空<br>CodeMeta drift：`"./docs/代碼管理（CodeMeta）/scripts/generate.sh" --go --go-media --web --admin --flutter`，再 `git status --porcelain -- internal/platform/codemeta internal/modules/media pkg/media ../website/lib/codemeta/generated.ts ../admin/src/lib/codemeta/generated.ts ../buyer-app/lib/core/codemeta/code_meta.dart ../seller-app/lib/core/codemeta/code_meta.dart` 必須為空<br>tidy drift：`go mod tidy`，再 `git status --porcelain -- go.mod go.sum` 必須為空 | ENV-BUF、ENV-DOCKER（sqlc image 由 Docker 拉取） | PASS（九個 step 都過；integration 見備註） | integration 在機器負載高時有兩個套件（`cmd/${N}`、`internal/modules/seller`）撞 15 分鐘逾時，其中 seller 還發生測試資料庫容器連線被拒；加 `-p 2` 重跑 `cmd/${N}` 通過，`seller` 以 `-p 1 -timeout 40m` 單獨重跑通過（1072 秒）。其餘 108 個套件第一輪就通過。逾時不是失敗，但驗收重跑時要預留時間並避免與其他重測試同時跑。 |

### `backend-proto-generated-diff.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `proto-generated-diff` proto 生成檔 regenerate + diff | `backend/api/proto/**`、`backend/pkg/proto/**`、Java proto 副本、`gen-*-proto.sh`、`scripts/proto/**` | `cd $ROOT/backend`<br>`IMAGE=$(bash scripts/proto/image-tag.sh); MODULE=$(sed -n 's/^MODULE="\(.*\)"$/\1/p' scripts/gen-go-proto.sh \| head -1)`<br>`docker image inspect "$IMAGE" >/dev/null 2>&1 \|\| docker build -t "$IMAGE" scripts/proto`<br>`docker run --rm -v "$PWD:/work:ro" -w /work -e MODULE="$MODULE" "$IMAGE" sh -c 'set -eu; mkdir -p /out; find api/proto -name "*.proto" \| sort \| while read -r f; do protoc --proto_path=api/proto --go_out=/out --go_opt=module="$MODULE" --go-grpc_out=/out --go-grpc_opt=module="$MODULE" "$f"; done; diff -ru /work/pkg/proto /out/proto'`<br>`diff -r -x .DS_Store api/proto services/java/shared/java-proto/src/main/proto` | ENV-DOCKER（image 內容定址，已有則不重建） | PASS（49 個 `.pb.go` 一致；Java 副本一致） | image tag 是 `scripts/proto/Dockerfile` 的雜湊，改名後 Dockerfile 若變，tag 會變而重建。 |

### `backend-sqlc-generated-diff.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `sqlc-generated-diff` sqlc 生成檔 regenerate + diff（舊服務） | `backend/services/*/db/{queries,schema,sqlc.yaml}`、`internal/**/gen/**`、`gen-sqlc.sh` | 在 `$ROOT/backend` 把 workflow 的「逐 service 重新生成並與 repo 對拍」那個 step 的 shell 原樣執行（`IMAGE=$(sed -n 's/^IMAGE="\(.*\)"$/\1/p' scripts/gen-sqlc.sh \| head -1)`；`docker pull` 若 image 不存在）；為了不寫進 `/tmp`，把腳本裡 `mktemp -d "/tmp/sqlc-check-…"` 的前綴換成暫存目錄 | ENV-DOCKER、ENV-REALPATH | PASS（九個服務都一致） | 單體模組的 sqlc 是 `backend-monolith` 的 `make sqlc`，不在這一列。 |

### `buyer-ci.yml`

呼叫 `_flutter-quality.yml`，`app: buyer-app`、`check-generated: true`。以下所有指令在 `$ROOT/buyer-app` 執行；`packages/` 與 `buyer-app` 都要先 `fvm flutter pub get`。觸發 paths：`buyer-app/**`、`packages/${N}_{api,auth,app_update,twaddr,ui_text}/**`、兩個 workflow 檔。

| job | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|
| `quality / Static checks` | `fvm dart format --output=none --set-exit-if-changed lib test integration_test tool`<br>`fvm flutter analyze`<br>`fvm dart run custom_lint`<br>`bash scripts/deadcode.sh`<br>`(cd ../packages/${N}_ui_text && fvm flutter pub get && fvm dart run tool/check_enum_text.dart ../../buyer-app/lib)` | `.fvmrc`（3.41.2） | PASS（五個 step） | CI 的 step 都是 `continue-on-error`，最後一個 step 彙整；本機任一步失敗就算失敗。 |
| `quality / Unit and widget tests` | `python3 scripts/test_ci_flutter_test.py`<br>`fvm flutter test --no-pub --exclude-tags hermetic-integration`<br>`fvm flutter test --no-pub --tags hermetic-integration`<br>coverage ratchet 不在本機跑（見備註） | 同上 | PASS（全量 1 項時序測試失敗，單獨重跑通過；hermetic 通過） | CI 還有 `--coverage`、解析 base 分支的 floor、`dart run tool/coverage_ratchet.dart`：只在 CI 跑，本機標 `NOT RUN（CI 專屬）`。本機不用 `scripts/ci-flutter-test.sh`（要 `RUNNER_TEMP`）。 |
| `quality / Generated code` | `fvm dart run build_runner build --delete-conflicting-outputs`<br>`git diff --exit-code -- .` 與 `git ls-files --others --exclude-standard -- .` 為空 | 同上；乾淨的 worktree 沒有 build cache | FAIL（`lib/core/session/session_provider.g.dart` 的 `_$sessionHash` 與重新生成不同） | 清掉 `.dart_tool/build` 後重跑一次結果相同（可重現）。原因：`session_provider.dart` 在 `83c98ca2a` 之後被改過，`.g.dart` 上次更新是更早的 `ccb63b085`，沒有重新生成。有 build cache 時 build_runner 會跳過全部輸入而假通過，驗收前要清 `.dart_tool/build`。 |

### `commit-messages.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `check` Conventional Commits | 不限（所有 PR 與 push to main） | `python3 scripts/check-commit-msg.py --range <BASE>..<HEAD>`；基線的範圍是 CI 在 push 時會檢查的 `origin/main..e5d5a8d8e`（20 個 commit），實跑 `e5d5a8d8e~20..e5d5a8d8e` | git 歷史（CI 用 `fetch-depth: 0`） | PASS | 新 repo 只有單一初始 commit：步驟 8 之後 CI 檢查的就是那一個，訊息要符合規則（type 在清單內、subject 不以句號結尾）。 |

### `dart-packages.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `package` `<package>`：analyze、format、test（矩陣：`${N}_app_update`、`${N}_auth`、`${N}_twaddr`、`${N}_ui_text`） | `packages/${N}_*/**` | 對每個 package：<br>`cd $ROOT/packages/<package>`<br>`fvm flutter pub get`<br>`fvm dart analyze`<br>`dirs=(lib test); [ -d tool ] && dirs+=(tool); fvm dart format --output=none --set-exit-if-changed "${dirs[@]}"`<br>`fvm dart test`（`${N}_auth` 用 `fvm flutter test`） | `.fvmrc` | PASS（四個 package 各自 analyze、format、test 都過） | `${N}_api` 不在這支，在 `api-contract.yml` 的 `dart`。 |

### `ios-lockfile-check.yml`

只有 `workflow_dispatch`。

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `changes`（paths-filter） | 手動觸發時 `ALL=true` | 無對應 | 無 | NOT RUN（選取邏輯） | |
| `ios-lockfile-check` `<app>`（矩陣：`buyer-app`、`seller-app`；`macos-15`） | 手動 | 對每個 App：<br>`cd $ROOT/<app> && fvm flutter pub get`<br>`cd ios && bundle install --quiet && bundle exec pod install --deployment` | macOS、Ruby 3.2、Bundler、CocoaPods 的 spec repo 網路 | NOT RUN（只手動跑，基線不跑；`AGENTS.local.md` 指定，且要下載 CocoaPods 規格庫，不是便宜的離線檢查） | 步驟 8 在新 repo 以 `gh workflow run` 手動觸發一次。 |

### `legacy-endpoints.yml`（步驟 7a 已刪 workflow；腳本與本機 hook 在步驟 9 移除）

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `check` 舊公司端點（D9） | 不限 | `python3 scripts/check-legacy-endpoints.py` | Python | PASS（259 個既有檔仍有 996 個命中，單體路徑 0） | 守門的語意是「命中只能減少」：步驟 2 清掉命中之後要 `--update` 重寫 baseline，而且那是所有刪改的最後一步。 |

### `legal-generated-diff.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `legal-generated-diff` 賣家入駐文件生成物 | `website/docs/legal/**`、`website/scripts/generate-legal.ts`、`legal-markdown.ts`、`website/features/legal/documents/**`、`agreement_gen.go` | `cd $ROOT/website && pnpm run gen:legal`<br>`cd $ROOT && git diff --exit-code -- website/features/legal/documents backend/internal/modules/seller/internal/model/agreement_gen.go`<br>`test -z "$(git ls-files --others --exclude-standard -- website/features/legal/documents backend/internal/modules/seller/internal/model)"` | 依賴 | PASS | 步驟 4 改寫法律頁後必須重新生成並提交。 |

### `repo-hygiene.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `check` workflow 與 shell 靜態檢查 | 不限 | `actionlint -color -shellcheck=`（本機 Homebrew 的 actionlint 是 1.7.12，與 CI 的 `rhysd/actionlint:1.7.12` 同版，不必起 Docker）<br>`bash scripts/check-fullwidth-var-expansion.sh` | `actionlint` | PASS（兩個 step） | 步驟 7 改 workflow 後這一列會重跑。 |

### `seller-ci.yml`

呼叫 `_flutter-quality.yml`，`app: seller-app`、`check-generated: true`、`test-args: --concurrency=4`。以下指令在 `$ROOT/seller-app` 執行。觸發 paths 同 buyer-ci（換成 `seller-app/**`）。

| job | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|
| `quality / Static checks` | `fvm dart format --output=none --set-exit-if-changed lib test integration_test tool`<br>`fvm flutter analyze`<br>`fvm dart run custom_lint`<br>`bash scripts/deadcode.sh`<br>`(cd ../packages/${N}_ui_text && fvm flutter pub get && fvm dart run tool/check_enum_text.dart ../../seller-app/lib)` | `.fvmrc`（3.41.2） | PASS（五個 step） | |
| `quality / Unit and widget tests` | `python3 scripts/test_ci_flutter_test.py`<br>`fvm flutter test --no-pub --exclude-tags hermetic-integration --concurrency=4`<br>`fvm flutter test --no-pub --tags hermetic-integration`<br>coverage ratchet 不在本機跑 | 同上 | PASS（unit／widget 與 hermetic 都過） | 同 buyer：ratchet 標 `NOT RUN（CI 專屬）`。 |
| `quality / Generated code` | `fvm dart run build_runner build --delete-conflicting-outputs`<br>`git diff --exit-code -- .` 與 `git ls-files --others --exclude-standard -- .` 為空 | 同上；`--delete-conflicting-outputs` 不能省（舊版在非互動環境 NPE） | PASS | 在負載高時要 31 分鐘。 |

### `ts-packages.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `package` `<package>`：lint、test（矩陣：`@${N}/twaddr`、`@${N}/ui-text`） | 三個 `${N}-*-ts` 套件、根 pnpm 檔 | 對每個套件：<br>`pnpm --filter <package> lint`<br>`pnpm --filter <package> test` | 依賴 | PASS（兩個套件各自 lint、test 都過） | 套件名在步驟 3 會從 `@${N}/*` 改成 `@hawkr/*`，矩陣與 filter 要同步。 |

### `twaddr-generated-diff.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `twaddr-generated-diff` twaddr 生成物 | `backend/internal/platform/twaddr/**`、TS 與 Dart 的生成檔 | `cd $ROOT/backend/internal/platform/twaddr && go run generate.go`<br>`cd $ROOT && git diff --exit-code -- backend/internal/platform/twaddr packages/${N}-twaddr-ts/src/twaddr.gen.ts packages/${N}_twaddr/lib` | Go | PASS | |

### `ui-text-generated-diff.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `ui-text-generated-diff` ui-text 生成物 | `api/**`、`backend/cmd/uitext-gen/**`、`backend/internal/platform/uitext/**`、模組的 `internal/text/**`、兩個 ui-text 套件 | `cd $ROOT && make -C backend uitext`<br>`paths=(backend/internal/platform/uitext 'backend/internal/modules/*/internal/text' packages/${N}-ui-text-ts/src packages/${N}-ui-text-ts/tests packages/${N}_ui_text/lib packages/${N}_ui_text/tool packages/${N}_ui_text/test)`<br>`git diff --exit-code -- "${paths[@]}"`；`git ls-files --others --exclude-standard -- "${paths[@]}"` 必須為空 | ENV-BUF、Go | PASS | 來源檔（catalog JSON）會被寫成正規形式，所以改 catalog 後一定要跑這一列。 |

### `ui-text-guard.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `cross-client` 跨端：enum 文字表 | `website/**`、`admin/**`、兩支 App 的 `lib/**`、各套件、`scripts/check-enum-text-cross.sh`、根 pnpm 檔 | `bash scripts/check-enum-text-cross.sh` | 依賴、Flutter（`packages/${N}_ui_text/.fvmrc`） | PASS | |

### `website-ci.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `verify` Lint, test, build | `website/**`、`api/**`、三個 TS 套件、`backend/gateway/internal/proxy/testdata/agui-frames/**`、兩支 App 的 `lib/core/theme/**`、根 pnpm 檔 | `cd $ROOT/website`，先 ENV-WEB，再依序：<br>`pnpm run lint`<br>`pnpm run design:check --apps-root ..`<br>`pnpm run test:unit`<br>`pnpm test`（vitest 的 `unit` project）<br>`pnpm run build`<br>`pnpm run test:build-output` | 依賴、ENV-WEB | PASS（六個 step；`pnpm test` 全量時 2 項 5 秒逾時，單獨重跑該 2 檔通過） | `pnpm run build` 不帶 ENV-WEB 會連到舊公司的 test-api。這一列不含 Playwright 與 Storybook 測試。 |

### `workspace-ci.yml`

| job | 觸發 paths | 本機等價指令 | 前置 | 基線 | 備註 |
|---|---|---|---|---|---|
| `deps` audit 與 override 檢查 | 根 pnpm 檔、`patches/**`、`scripts/check-dependency-overrides.ts`；另有每週一 03:17 的排程 | `pnpm run check:deps`（即 `pnpm audit --prod && node scripts/check-dependency-overrides.ts`，前一步失敗時後一步不會跑，所以要分開跑）<br>`pnpm audit --prod`<br>`node scripts/check-dependency-overrides.ts` | 依賴、npm registry 網路 | FAIL（`pnpm audit --prod`：16 個已知弱點，3 low／7 moderate／3 high／3 critical；override 檢查 PASS） | 重跑一次結果相同（可重現）。弱點在 `admin>@sentry/nextjs>next`（多筆 Next.js 公告，含 critical）與 `website>@ag-ui/core>zod`。這一列依賴 registry 當下的公告資料庫，隨日期變動，不是快照 commit 的屬性；驗收時比較的是「沒有新增的套件或公告」，不是 0。 |

## 不在本機（D5，交 CI）

2026-10-24 量測：`grep -rlE 'playwright|test:e2e|storybook:test' .github/workflows` 的結果是 0 個檔，也就是目前 CI 沒有任何 workflow 跑這些。依 D5，步驟 7 在新 repo 新增 website 與 admin 的 Playwright workflow；A7-2v 把 backend 驗收搬進 CI。以下項目本機一律不跑，在驗收時標 `不在本機（D5，交 CI）`，直到新 repo 的 CI 通過。

| 項目 | 指令 | 現況 |
|---|---|---|
| website Playwright e2e | `pnpm run test:e2e`、`test:e2e:hermetic`、`test:e2e:live`、`test:e2e:contract`、`test:e2e:ui` | 不在本機（D5，交 CI）；目前 CI 也沒有 |
| website 可靠度與 SEO | `pnpm run test:reliability`、`pnpm run test:seo`（`build:seo` 前置） | 不在本機（D5，交 CI） |
| Storybook 測試 | `pnpm run storybook:test`（`vitest run --config vitest.storybook.config.ts`）、`VITEST_STORYBOOK=true` | 不在本機（D5，交 CI） |
| `playwright install` | 瀏覽器下載，共用快取鎖會卡住別的 repo | 不在本機（D5，交 CI） |
| backend 整套環境驗證 | `backend-e2e-verify` skill | 不在本機（D5，交 CI），由 A7-2v 搬進 CI |
| 特性測試 | `backend/scripts/characterize.sh` | 不在本機（D5，交 CI），同上 |

## 驗收基線（e5d5a8d8e）

基線在 `<baseline worktree>`（detached `e5d5a8d8e`，`git rev-parse HEAD` 驗證）執行，依賴在 worktree 內安裝，沒有從主 checkout 連結。跑完 worktree 的 `git status` 乾淨（`admin/next-env.d.ts` 與重新生成而漂移的 `.g.dart` 都已 `git restore`）。

### 結論

共 39 個表格列：3 列是 `_flutter-quality.yml` 的轉介（隨呼叫端，不獨立計算），其餘 36 列：PASS 29、FAIL 4、NOT RUN 3。另有一個 step 為 NOT RUN：兩支 App 的 coverage ratchet（CI 專屬，`AGENTS.local.md`）。

### 既有失敗（4 列）

| 列 | 失敗內容 | 如何確認可重現 | 判斷 |
|---|---|---|---|
| `backend-monolith / lint` | 用 CI 固定的 golangci-lint v2.13.2、Go 1.27.1 跑：一般 22 項、integration tag 35 項（exhaustive、gocritic、gosec、ineffassign、staticcheck、unused；integration tag 另有 errcheck、errorlint） | 本機 2.14.0 也報（一般 24 項，標註 `本機版本較新，非 CI 結論`）；v2.13.2 另跑兩輪結果相同 | 在本機以釘定版本重現。CI 在這個 commit 是否已紅無法從本機得知（領先 origin 的 20 個 commit 從未推上去跑 CI）。 |
| `backend-integration-tests / integration`，`services/pricebook-service` | `internal/usecase` 兩項 rollup 測試 `expected "ready"`、`actual "unavailable"` | 整個矩陣跑一次、`internal/usecase` 單獨 `-count=1 -p 1` 再跑一次，結果相同 | 日期相關：測試的交易日寫死在 2026-09，時鐘是現在。不是快照 commit 本身的問題，而是時間過去才壞。改名階段不處理，但驗收要把它當既有失敗。 |
| `buyer-ci / Generated code` | `buyer-app/lib/core/session/session_provider.g.dart` 的 `_$sessionHash` 與重新生成不同 | 第一次生成後失敗；清掉 `.dart_tool/build` 再跑一次，結果相同 | `session_provider.dart` 在 `83c98ca2a` 改過而沒有重新生成 `.g.dart`（上次更新是 `ccb63b085`）。步驟 3 之後重新生成會順便修好，要記得這個 diff 是既有的。 |
| `workspace-ci / deps` | `pnpm audit --prod`：16 個弱點（3 low、7 moderate、3 high、3 critical），主要是 `admin>@sentry/nextjs>next` 與 `website>@ag-ui/core>zod` | 兩次執行相同（各 13 秒）；`node scripts/check-dependency-overrides.ts` 另外跑，PASS | 與 registry 當下的公告有關，會隨日期變。步驟 2 移除 admin 的 Sentry 後，`@sentry/nextjs` 這條路徑會消失，但 `next` 本身的弱點還在；驗收只比較「沒有新增」。 |

### NOT RUN

| 列 | 原因 |
|---|---|
| `backend-integration-tests / changes` | paths-filter 的選取邏輯，手動觸發時為全部，沒有本機意義 |
| `ios-lockfile-check / changes` | 同上 |
| `ios-lockfile-check / ios-lockfile-check` | 只手動跑，基線不跑（`AGENTS.local.md`；要下載 CocoaPods 規格庫） |
| coverage ratchet（`buyer-ci`、`seller-ci` 的 tests 列內的一個 step） | CI 專屬（`AGENTS.local.md`）；`scripts/ci-flutter-test.sh` 要 `RUNNER_TEMP` 且本機直接退出 |
| e2e、Playwright、`storybook:test`、`backend-e2e-verify`、`characterize.sh` | 不在本機（D5，交 CI）；CI 目前也沒有，見上一節 |

### 負載造成的假失敗（已重跑，記為 PASS）

只記重跑後通過的失敗；沒有重現的不當作失敗。

| 列 | 第一輪 | 重跑 |
|---|---|---|
| `website-ci / verify` 的 `pnpm test` | `RfqDraftFlow.audience-change`、`RfqDraftFlow.publication-execute` 各一項 5 秒逾時（共 3146 項） | 單獨重跑這兩個檔，22 項通過 |
| `buyer-ci / Unit and widget tests` | `test/core/session/session_provider_test.dart` 一項「冷啟動暫時失敗會自動退避重試」（共 1431 項通過、1 項失敗） | 單獨重跑該檔，16 項通過 |
| `backend-java-tests / reactor-test` | `cart-service` 的 `SellerGrpcClientBatchMetaTest.success_doesNotRetry`（耗時 7.4 秒），其餘模組都沒被跳過 | 以 `-pl cart-service -am -Dtest=` 單獨重跑，4 項通過 |
| `backend-monolith / check` 的 integration | `cmd/${N}` 與 `internal/modules/seller` 撞 15 分鐘逾時（其餘 108 個套件通過）；加 `-p 2` 重跑時 seller 的測試資料庫容器連線被拒 | `cmd/${N}` 在 `-p 2` 通過；`seller` 以 `-p 1 -timeout 40m` 單獨重跑通過（1072 秒） |

### 驗收重跑的注意事項

- 本機的 Go 是 1.27.2，CI 是 1.27.1（`backend/.go-version`）；除了釘定版本的 lint，其餘 Go 檢查用本機的 1.27.2。Python 3.14.7（CI 3.13）、Node 26.10.0、pnpm 11.2.2、Flutter 3.41.2、buf 1.50.0、actionlint 1.7.12、JDK temurin-21。
- `backend-monolith / lint` 的釘定版本要自己編（ENV-GOLINT）；用預設工具鏈編會得到「export data version」的假錯誤，不是真問題。
- 生成物 drift 類的列會改寫追蹤中的檔，只能在可丟棄的 worktree 跑，跑完 `git status` 要乾淨。已知會留下痕跡的：`admin` 的 `pnpm build` 改 `next-env.d.ts`；`buyer-app` 的 build_runner。
- Flutter 的 build_runner 在有 `.dart_tool/build` 時會假通過（全部輸入 skipped）；驗收 generated-code 前先刪該目錄。
- 一次只跑一個重測試（Flutter 完整測試、整合測試、Maven）。這台機器負載高時，各列耗時加總約 7 小時（部分列有重疊執行），大部分是 Flutter 完整測試與 Java。

### 各列耗時

耗時是該列所有 step 與重跑的牆鐘秒數加總。基線期間這台機器的 load average 約 30 到 130（多個 session 共用），耗時偏長，只能當相對量級，不是 CI 的耗時。log 都在 `scratchpad/baseline/<id>.log`（不進版控）。

| 列（workflow / job） | 結論 | 耗時 | log id |
|---|---|---|---|
| admin-ci / verify | PASS | 12 分 31 秒 | admin-lint、admin-test、admin-build |
| api-contract / buf | PASS | 1 分 28 秒 | api-contract-buf-lint、-googleapis-pin、-buf-breaking |
| api-contract / dart | PASS | 16 秒 | api-contract-dart-* |
| api-contract / ts | PASS | 12 秒 | api-contract-ts-* |
| api-contract / presets | PASS | 0 秒 | api-contract-presets |
| backend-contract-tests / template-contract | PASS | 1 分 25 秒 | backend-integration-notification-service（共用） |
| backend-gateway-json-contract / contract | PASS | 1 分 20 秒 | gateway-json-contract-py、gateway-route-coverage |
| backend-go-checks / go-checks | PASS | 28 分 04 秒 | go-checks-* |
| backend-integration-tests / integration（矩陣九項） | FAIL | 14 分 32 秒 | backend-integration-<模組>（pricebook 另有 -rerun） |
| backend-java-tests / reactor-test | PASS | 44 分 00 秒 | java-reactor-test、-rerun-cart |
| backend-java-tests / integration-test | PASS | 29 分 42 秒 | java-it-surefire-includes、java-integration-test |
| backend-migration-inventory / check | PASS | 1 秒 | migration-inventory |
| backend-monolith / lint | FAIL | 19 分 14 秒 | monolith-lint（v2.13.2）、monolith-lint-local-v2.14.0、monolith-lint-attempt1-toolchain-mismatch |
| backend-monolith / check | PASS | 58 分 23 秒 | monolith-*（integration 另有 -rerun-p2、-rerun-seller） |
| backend-proto-generated-diff / proto-generated-diff | PASS | 21 秒 | backend-proto-generated-diff |
| backend-sqlc-generated-diff / sqlc-generated-diff | PASS | 2 分 05 秒 | backend-sqlc-generated-diff |
| buyer-ci / Static checks | PASS | 5 分 35 秒 | flutter-buyer-app-static |
| buyer-ci / Unit and widget tests | PASS | 79 分 23 秒 | flutter-buyer-app-test、-hermetic、-test-rerun-session |
| buyer-ci / Generated code | FAIL | 7 分 36 秒 | flutter-buyer-app-generated、-generated-rerun |
| commit-messages / check | PASS | 0 秒 | commit-messages |
| dart-packages / package（四項） | PASS | 2 分 19 秒 | dart-packages-* |
| legacy-endpoints / check | PASS | 15 秒 | legacy-endpoints |
| legal-generated-diff / legal-generated-diff | PASS | 1 秒 | legal-generated-diff |
| repo-hygiene / check | PASS | 3 秒 | repo-hygiene-* |
| seller-ci / Static checks | PASS | 7 分 23 秒 | flutter-seller-app-static |
| seller-ci / Unit and widget tests | PASS | 62 分 24 秒 | flutter-seller-app-test、-hermetic |
| seller-ci / Generated code | PASS | 31 分 22 秒 | flutter-seller-app-generated |
| ts-packages / package（兩項） | PASS | 1 分 24 秒 | ts-packages-* |
| twaddr-generated-diff / twaddr-generated-diff | PASS | 2 秒 | twaddr-generated-diff |
| ui-text-generated-diff / ui-text-generated-diff | PASS | 3 秒 | ui-text-generated-diff |
| ui-text-guard / cross-client | PASS | 1 分 44 秒 | ui-text-guard-cross-client |
| website-ci / verify | PASS | 16 分 14 秒 | website-*（test 另有 -rerun-2files） |
| workspace-ci / deps | FAIL | 40 秒 | workspace-ci-deps、workspace-ci-overrides |
| 依賴安裝與 `flutter pub get`（前置，不算列） | 完成 | 3 分 11 秒 | setup-* |


## 涵蓋檢查

在 repo 根目錄執行（2026-10-24，快照 `e5d5a8d8e`）：

```
ls .github/workflows | wc -l
diff <(ls .github/workflows | sort) <(grep -oE '^### `[^`]+\.yml`' backend/docs/plans/2026-10-24-portfolio-public/ci-local-map.md | tr -d '#` ' | sort) && echo "diff: empty"
grep -cE '^### `[^`]+\.yml`' backend/docs/plans/2026-10-24-portfolio-public/ci-local-map.md
grep -rlE 'playwright|test:e2e|storybook:test' .github/workflows | wc -l
```

輸出：

```
      26
diff: empty
26
       0
```

`ls` 的 26 支與本檔的 26 個 workflow 小節逐一相同（diff 為空）；CI 內沒有任何 e2e 或 Playwright 的引用（0）。
