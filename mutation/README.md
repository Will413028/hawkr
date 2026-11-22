# mutation 驗收清單

一份清單對應一個計畫步驟（例如 `a3-10f-3c.json`）。每個 mutant 在 CI 的拋棄式 checkout 裡把一個檔案的 `old` 換成 `new`，再跑指定的測試；測試必須變紅（KILLED），代表這個行為真的被測試鎖住。

mutation 不在開發機跑（機器共用，Go integration 需要資料庫容器）。工作流程是：寫清單、推上去，由 `.github/workflows/mutation.yml` 執行；結果出來之前，計畫的進度標「待 CI」。

清單是該步驟的一次性證據，不是回歸防線：CI 只在清單或 runner 變動時跑，之後測試被改弱不會被發現。步驟結案後清單可以刪；要長期守住的行為寫成一般測試或守門檢查。

## 格式

```json
{"mutants": [
  {"id": "short-kebab", "stack": "website", "file": "website/features/report/report-logic.ts",
   "old": "exact text", "new": "replacement",
   "test": "features/report/my-reports.unit.test.ts hides the supplement button once the report has an outcome",
   "why": "一句話：這個 mutant 拿掉了什麼行為"}
]}
```

- `id`：清單內唯一。
- `file`：從 repo 根目錄起算的路徑。
- `old`：必須在 `file` 裡剛好出現一次，否則 `APPLY-FAILED`。可含換行與縮排（JSON 的 `\n`、`\t`），逐字比對。
- `new`：替換文字，可為空字串（刪除）。mutant 必須仍可編譯或載入：Go 的變數刪了之後不能變成未使用，要保留引用（否則 `BUILD-FAILED`）。
- `test`：測試選擇器，格式依 stack（下表）。`why` 必填。

## stack 與選擇器

| stack | 選擇器 | 實際執行（工作目錄） |
|---|---|---|
| `go-integration` | `<相對 backend 的套件目錄> <regex>` | `go test -count=1 -vet=off -tags integration ./<套件>/ -run '<regex>'`（`backend`，`GOWORK=off`） |
| `go` | 同上 | 同上但不帶 integration tag |
| `go:<模組目錄>`、`go-integration:<模組目錄>` | 同上，套件目錄改為相對該模組 | 同上，工作目錄改為 `<模組目錄>`（相對 repo 根目錄，例如 `backend/services/invoice-service`，`GOWORK=off`）；目錄必須存在且有 `go.mod`（`--validate` 檢查） |
| `website` | `<相對 website 的測試檔> [測試名稱]` | `pnpm exec <website/package.json 的 test:file script> <檔> -t '<名稱>'`（`website`；`test:file` 目前是 `vitest run --project unit`，`pnpm test` 也呼叫它。script 必須恰為 `vitest run --project unit`（或 `--project=unit`），前後不得有其他 token） |
| `admin` | `<相對 admin 的測試檔> [測試名稱]` | `<admin/package.json 的 test:file script> [--test-name-pattern '<名稱>'] <檔>`（`admin`；`test:file` 目前是 `node --import tsx --import ./tests/setup/env.mjs --test`，`pnpm test` 也呼叫它，runner 讀 script 後直接以 node 執行；script 必須恰為 `node`、零或多組 `--import <specifier>`、`--test`，specifier 不得以 `-` 開頭或含 glob、shell 字元、空白）。`--test-name-pattern` 必須在檔案之前，放後面 node 會忽略並整檔照跑 |
| `flutter:buyer-app`、`flutter:seller-app`（只接受這兩個） | `<相對該 App 的測試檔> [測試名稱]` | `bash scripts/ci-flutter-test.sh --no-pub <檔> --plain-name '<名稱>'`（該 App 目錄） |

- Go 的 regex 要錨定（`^TestX$`，子測試 `^TestX$/^sub$`）：前綴比對可能讓被擋下的 mutant 被誤判。
- admin 與 website 的 `test:file` script 必須是單純的 argv：不得含 `&&`、`;`、`|`、重導向、`$` 展開或 `VAR=value` 前綴，不是字串或不完全符合上述文法都是清單錯誤；文法刻意嚴格，兩支 script 要新增旗標時必須同步修改 runner 的文法（`--validate` 會在改動 package.json 的 PR 就擋下）。
- admin 的名稱是 regex，沒對到任何測試時 node 仍以 0 結束（只顯示檔案層級通過），結果是 `SURVIVED` 而不是錯誤，所以名稱要打對、並唯一對到預期變紅的測試。
- website 一律只跑 `unit` project（vitest 設定的另一個 project 是 browser）；測試名稱是 `-t` 的子字串或 regex，要唯一對到預期變紅的測試。
- 每個 mutant 先在未改動的樹上跑一次同樣的測試作基準，基準不過就是 `BASELINE-FAILED`（紅的測試證明不了什麼）。

## 結果

| 結果 | 意思 |
|---|---|
| `KILLED` | 測試變紅，符合預期 |
| `SURVIVED` | 測試仍綠：這個行為沒被測試鎖住，補測試或確認 mutant 無意義 |
| `BASELINE-FAILED` | 未改動時測試就不過（選擇器打錯、測試壞了或環境問題） |
| `APPLY-FAILED` | `old` 出現 0 次或多次（程式已變動，清單過期） |
| `BUILD-FAILED` | mutant 本身壞了（編譯、轉換失敗），不是測試擋下的，job 失敗；修 `new` |
| `TIMEOUT` | 超過 15 分鐘（含建置檢查），視為沒被擋下；逾時會殺掉整個行程樹 |
| `ERROR` | 指令無法啟動（工具缺失等），job 失敗 |

mutant 套用後先檢查還能建置：Go 跑 `go test -c -o /dev/null [-tags integration] ./<套件>/`；Flutter 只對 App 目錄內的檔案跑 `dart analyze --no-fatal-warnings <檔>`（只有 error 算失敗），`packages/` 下的檔案跳過這步，靠測試載入階段的錯誤判斷。建置檢查本身真的失敗才是 `BUILD-FAILED`，逾時是 `TIMEOUT`，無法啟動是 `ERROR`。website 與 admin 沒有便宜的逐 mutant 編譯，改看測試輸出。admin（tsx／esbuild）失敗且含 `TransformError`、`Transform failed` 或 `SyntaxError` 同樣是 `BUILD-FAILED`。website 看 vitest 輸出：失敗且含 `Transform failed`、`SyntaxError` 或 `Failed to parse source` 才是 `BUILD-FAILED`；Flutter 輸出含 `Compilation failed` 或編譯器的 `<檔>:<行>:<欄>: Error:` 行同理。其他沒有這些標記的失敗（含沒有測試摘要）算測試擋下（`KILLED`）。Go 測試帶 `-vet=off`（vet 失敗不該被算成 KILLED，建置仍由 `go test -c` 檢查），`-timeout` 設成 20 分鐘，比 runner 的 15 分鐘長；Flutter 用 `ci-flutter-test.sh` 預設的逐測試逾時。

mutant 的目標是 `*.sql` 檔，且同目錄有 `atlas.sum` 時，套用後 runner 會依 Atlas 的演算法重算並改寫該 `atlas.sum`（逐檔累加的 SHA-256，第一行是總雜湊），還原時一併還原。必要條件：整合測試建庫前先驗證 `atlas.sum`，不重算的話改 migration 的 mutant 一律先死在 checksum 檢查，migration 定義的約束（NOT NULL、CHECK、索引）就沒辦法被 mutation 驗證。這條規則只看「同目錄有 `atlas.sum`」，不針對特定清單。不呼叫 Atlas CLI：repo 內每個 `atlas.sum` 都由 Atlas 產生，`test_run.py` 逐位元組重現全部，Atlas 改格式時重新產生的檔會讓這個測試失敗；mutation job 因此不必另裝 Atlas。

每個選取的 mutant 都 `KILLED` 才算通過；摘要也寫進 job summary。

## 新增一份清單

1. 找出要驗的行為與鎖住它的測試名稱；`old` 取最小但唯一的片段。
2. 在 `mutation/<步驟>.json` 寫 mutant，不要為了讓 mutant 可被擋下去改測試或產品碼。
3. 本機只能做格式檢查（不執行測試）：`python3 scripts/mutation/run.py --validate mutation/<步驟>.json`，確認 `old` 恰好一次、測試檔存在。
4. 推上去；push 與 PR 只跑這次改到的清單，runner 或 workflow 改了則全跑。`workflow_dispatch` 可用 `manifests` 輸入指定清單。
5. 清單過期（`old` 不再恰好一次、測試檔不見）由 repo-hygiene 的 `--validate` 步驟抓：repo-hygiene 沒有 path 條件，每次 push、PR 與每週排程都跑，所以改了被 mutate 的程式也會看到。
6. runner 的單元測試：`python3 -m unittest scripts/mutation/test_run.py`（repo-hygiene 也會跑）。
