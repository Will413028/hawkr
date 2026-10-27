# 作品集轉 public：改名 Hawkr、單一初始 commit 的新 repo，文件清洗後公開，名字類檢查只在本機

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-23、2026-10-24） |
| 日期 | 2026-10-24 |
| 影響範圍 | 整個 monorepo：四個 client、單體、legacy 服務、`api/`、`packages/`、`.github/workflows/`、`scripts/`、文件 |
| 詳細論證 | `docs/plans/2026-10-24-portfolio-public.md`（規模、步驟、決定 D1–D7；本 ADR 不複製） |
| 相關 | ADR `2026-09-26-external-contract-protobuf-connect.md`（`v1alpha1` 改 `v1` 不在本次做） |

## 脈絡

需求（Will 2026-10-23）：repo 公開當作品集，同時解決 private repo 的 Actions 額度（每月 2,000 分鐘；public repo 用標準 runner 免費）。舊品牌名與 logo 有商標，文案與素材不在老闆同意的範圍內（同意範圍只寫「程式碼」，Will 2026-10-18）。

約束：

- 外部：舊品牌名與 logo 有商標。
- 外部：GitHub 不能把 issue 從 private repo 移轉到 public repo。
- 外部：public repo 收 fork PR，fork 拿不到 repository secret。
- 外部：沒有自己的網域，暫不申請（Will 2026-10-23）。
- 繼承：系統從未上線，改名不受相容性約束（根目錄 `AGENTS.md`）。
- 繼承：legacy 服務預定在主計畫階段 B、C 刪除，但單體還靠它們跑沒搬完的模組。現在仍成立。

## 考慮過的方案與決策

| 題目 | 決策 | 沒選的方案與理由 |
|---|---|---|
| 歷史 | 開新 repo，只有單一初始 commit；舊 repo 改名封存為 private | 在原 repo 重寫歷史：無法保證舊 commit、fork 與快取都清乾淨 |
| 名字 | Hawkr，中英文介面都用 | 和農產品有關的名字：作品集示範的是一般電商 |
| D1 公司時期的設計書、規範與供應商文件 | 公開，清洗品牌、人名與供應商的特店資訊 | 只公開 2026-09 之後自己寫的文件：Will 選擇保留完整的設計紀錄 |
| D2 來源不明的圖片 | 品牌圖用 `brand/hawkr/`，示範圖改成程式產生的佔位 SVG | Unsplash：要逐張判斷授權；AI 生成：授權依工具條款 |
| D3 issue | 在新 repo 重建，並替換舊名與舊路徑 | 留在封存：當初搬 issue 就是為了不漏 |
| D4 反向 DNS 前綴 | `io.github.will413028.hawkr` | `dev.hawkr`：沒有這個網域，被別人註冊後就成了冒用 |
| D5 e2e | 在本階段新增 website 與 admin 的 Playwright workflow；視覺基準第一次在 CI 產生 | 延後：現況是 e2e 在本機與 CI 都沒有跑 |
| D6 兩字母縮寫 | 改成 `hk-`／`hk_` | 保留：會讓人看出舊名 |
| D7 名字類檢查（品牌、法人、同事） | 只在本機 pre-commit／pre-push 執行，讀不進版控的明文清單；CI 只跑不含名字的檢查、gitleaks 與 secret scanning | 公開未加鹽的雜湊：可以用字典反查；HMAC 加 repository secret：fork PR 會跳過，還要多管一個 secret |

## 後果

- 之後的開發在新 repo。計畫與 ADR 裡的 commit hash 指的是私人封存。
- 外部 PR 不受名字類檢查；只有 Will 一個開發者時可以接受。
- 重新評估的條件：
  - 有第二個長期貢獻者時，重評 D7。
  - 申請網域時，改掉 D4 的前綴。

## 修訂：縮寫的同族寫法與撞名的資料值（2026-10-24，Will）

步驟 1 的盤點發現，D6 列的 `<縮寫>-`／`<縮寫>_` 之外，同一個縮寫還用在 UI 元件與頁面前綴、Pascal 類別前綴、日誌類別、camelCase 識別碼，以及大寫的資料值（代行工作階段編號、header 格式、訂單與出貨單編號前綴、測試編號）。決策（計畫 D8）：全部改成 `hk` 系列，資料值的格式前綴連同產生碼、驗證正則、CHECK、種子與 fixture 一起改，migration 改寫後重算 `atlas.sum`。沒選的方案是只改程式識別碼、保留資料值：掃描要為資料值開例外清單，不能直接當「必須是 0」的關卡，而上線前也沒有相容性約束要保留舊格式。
