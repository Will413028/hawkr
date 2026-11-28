# 文件

## 分界

- 根目錄 `docs/`：影響整個產品或多個子專案的文件。
  - `docs/adr/`：全部 ADR，一份紀錄（不依影響範圍分兩處）。
  - `docs/conventions/`：四端共用的規範。
  - `docs/plans/`：新開的跨專案計畫。
- 子專案的 `docs/`（`backend/docs/`、`website/docs/`、App 與套件的 `docs/`）：只有該子專案用的文件，例如 backend 的模組規格、單體遷移計畫、只適用後端的規範、runbook。
- 拿不準時問：「刪掉這個子專案，這份文件還有沒有人需要？」有就放根目錄。

## 路徑寫法

- 文件與註解裡的路徑一律從 repo 根目錄起算：`docs/adr/…`、`backend/docs/modules/…`。不寫相對所在子專案的 `docs/…`。
- markdown 連結用 `/` 開頭的 repo 根目錄路徑，或相對所在檔案的路徑。
- 程式在某個目錄執行時讀的路徑字串（執行路徑）不受此限。
- 搬到根目錄的舊 ADR 表頭有「路徑基準｜`backend/`」時，該份 ADR 裡的路徑以 `backend/` 起算；新 ADR 不加這列。
- `scripts/check-doc-paths.py` 檢查指向 `docs/adr/`、`docs/conventions/`、`docs/plans/`（根目錄與 `backend/` 兩處），以及 `backend/docs/modules/` 的引用都存在；相對 backend 寫成 `docs/modules/…` 會失敗，CI 在 repo-hygiene 執行。

## 遺留基準

以下目錄隨單體遷移刪除，保留相對 backend 的舊寫法，不受路徑檢查；刪除時從清單移除。

- `backend/services/`
- `backend/gateway/`
- `backend/pkg/`
- `backend/docs/detailed-design/`
