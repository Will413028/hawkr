# mutation 驗收清單

一份清單對應一個計畫步驟（例如 `a3-10f-3c.json`）。每個 mutant 在 CI 的拋棄式 checkout 裡把一個檔案的 `old` 換成 `new`，再跑指定的測試；測試必須變紅（KILLED），代表這個行為真的被測試鎖住。

mutation 不在開發機跑（機器共用，Go integration 需要資料庫容器）。工作流程是：寫清單、推上去，由 `.github/workflows/mutation.yml` 執行；結果出來之前，計畫的進度標「待 CI」。

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
- `new`：替換文字，可為空字串（刪除）。mutant 必須仍可編譯或載入：Go 的變數刪了之後不能變成未使用（編譯失敗會被誤算成 KILLED），要保留引用。
- `test`：測試選擇器，格式依 stack（下表）。`why` 必填。

## stack 與選擇器

| stack | 選擇器 | 實際執行（工作目錄） |
|---|---|---|
| `go-integration` | `<相對 backend 的套件目錄> <regex>` | `go test -count=1 -tags integration ./<套件>/ -run '<regex>'`（`backend`，`GOWORK=off`） |
| `go` | 同上 | 同上但不帶 integration tag |
| `website` | `<相對 website 的測試檔> [測試名稱]` | `pnpm exec vitest run --project unit <檔> -t '<名稱>'`（`website`） |
| `flutter:buyer-app`、`flutter:seller-app` | `<相對該 App 的測試檔> [測試名稱]` | `bash scripts/ci-flutter-test.sh --no-pub <檔> --plain-name '<名稱>'`（該 App 目錄） |

- Go 的 regex 要錨定（`^TestX$`，子測試 `^TestX$/^sub$`）：前綴比對可能讓被擋下的 mutant 被誤判。
- website 一律只跑 `unit` project（vitest 設定的另一個 project 是 browser）；測試名稱是 `-t` 的子字串或 regex，要唯一對到預期變紅的測試。
- 每個 mutant 先在未改動的樹上跑一次同樣的測試作基準，基準不過就是 `BASELINE-FAILED`（紅的測試證明不了什麼）。

## 結果

| 結果 | 意思 |
|---|---|
| `KILLED` | 測試變紅，符合預期 |
| `SURVIVED` | 測試仍綠：這個行為沒被測試鎖住，補測試或確認 mutant 無意義 |
| `BASELINE-FAILED` | 未改動時測試就不過（選擇器打錯、測試壞了或環境問題） |
| `APPLY-FAILED` | `old` 出現 0 次或多次（程式已變動，清單過期） |
| `TIMEOUT` | 超過 15 分鐘，視為沒被擋下 |

每個選取的 mutant 都 `KILLED` 才算通過；摘要也寫進 job summary。

## 新增一份清單

1. 找出要驗的行為與鎖住它的測試名稱；`old` 取最小但唯一的片段。
2. 在 `mutation/<步驟>.json` 寫 mutant，不要為了讓 mutant 可被擋下去改測試或產品碼。
3. 本機只能做格式檢查（不執行測試）：`python3 scripts/mutation/run.py --validate mutation/<步驟>.json`，確認 `old` 恰好一次、測試檔存在。
4. 推上去；push 與 PR 只跑這次改到的清單，runner 或 workflow 改了則全跑。`workflow_dispatch` 可用 `manifests` 輸入指定清單。
5. runner 的單元測試：`python3 -m unittest scripts/mutation/test_run.py`（repo-hygiene 也會跑）。
