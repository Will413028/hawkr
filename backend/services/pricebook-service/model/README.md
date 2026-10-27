# model/ — 模型版本升級 Runbook

`registry.json` 是一份 git-committed 的指標檔，指向 R2 上真正的
`model.json`（154MB，不進 git，見 `../.gitignore`）。CI 在
`docker buildx build` 前用 `cmd/pricebook-model-fetch` 把它下載＋checksum
驗證＋bake 進 image；跑起來的服務只讀本機 `model/model.json`（`config.go`
的 `MODEL_ARTIFACT_PATH`），從不連 R2。

## registry.json schema

見 `internal/modelregistry/registry.go` 的 `Registry` struct（唯一權威定義）：

| 欄位 | 說明 |
|---|---|
| `model_version` | 版本字串，同時是 R2 key 的一段與本機 cache 檔名 |
| `r2_object_key` | R2 bucket 內的完整 key（見下方命名慣例） |
| `sha256` | model.json 內容的 sha256，下載後強制核對，不符直接失敗 |
| `trained_at` | 訓練時間（RFC3339） |
| `dataset_digest` | 訓練資料集摘要，來自 training pipeline |
| `series_count` | 序列數，僅供人讀 log 用 |

## R2 key 命名慣例

```
pricebook-models/<model_version>/model.json
```

## 升版步驟

1. 用 `hawkr_price_ai` 的 training pipeline 產出新的 `model.json`。
2. 算 sha256：`shasum -a 256 model.json`。
3. 把新檔上傳到 R2 的 `pricebook-models/<新 model_version>/model.json`
   （需要下方 4 個 secret 對應的 R2 存取權限）。
4. 更新本目錄 `registry.json` 的 6 個欄位（`model_version` /
   `r2_object_key` / `sha256` / `trained_at` / `dataset_digest` /
   `series_count`）。
5. 開一個只動 `registry.json` 的 PR。CI 的 "Fetch pricebook model.json"
   步驟會自動下載、驗證、bake 進新 image；不需要額外碰 Dockerfile 或 k8s
   values。

## CI 需要的 4 個 GitHub secrets

`.github/workflows/deploy-k8s.yml` 的 Fetch 步驟讀取：

- `PRICEBOOK_MODEL_R2_ACCOUNT_ID`
- `PRICEBOOK_MODEL_R2_ACCESS_KEY_ID`
- `PRICEBOOK_MODEL_R2_SECRET_ACCESS_KEY`
- `PRICEBOOK_MODEL_R2_BUCKET_NAME`

## 本機建置注意

`docker build --target prod` 需要本機真的存在
`services/pricebook-service/model/model.json`（gitignored，CI 才會產出）。
全新 checkout 若要在本機建 prod target，得先用真的 R2 credentials 跑一次
`go run ./cmd/pricebook-model-fetch/ --registry model/registry.json
--cache-dir <dir> --out model/model.json`，或放一個假檔做純語法檢查
（不是真的可跑，僅供 `docker build` 語法驗證）。
