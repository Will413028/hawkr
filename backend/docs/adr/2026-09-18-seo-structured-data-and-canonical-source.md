# SEO structured data 缺資料省略欄位、canonical 由 route param 重建

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | `hawkr-website` `lib/seo/jsonld.ts`、`lib/seo/urls.ts` |

## 脈絡

網站的每個公開頁面都會輸出兩樣搜尋引擎會直接採信的東西：JSON-LD structured data，以及 `<link rel="canonical">`。兩者都有「便宜但有毒」的作法，一旦選錯，後果不是當下的 bug 而是搜尋引擎端的長期降權——而降權沒有任何 CI 或測試看得見。

## 考慮過的方案

### 一、JSON-LD 遇到缺資料的商品／賣家

| 方案 | 代價 | 判定 |
|---|---|---|
| 捏造預設值（例如 rating 一律給 5.0、price 給 0） | structured data 與頁面實際內容不符。Google 對「假資料」的處置是 rich result 資格降權乃至人工懲罰，而且是站台層級 | 否決 |
| **省略該欄位** | 該頁拿不到對應的 rich result 樣式 | **採用** |

### 二、canonical / OG URL 的來源

| 方案 | 代價 | 判定 |
|---|---|---|
| 從 request URL 取 | query string 會一起進 canonical。`utm_*` 等追蹤參數讓同一頁產生無限多個 canonical，搜尋引擎判定為重複內容，權重被打散 | 否決 |
| **從 route param 重建** | 每新增一種路由都要在 `urls.ts` 補一個 builder，不能自動涵蓋 | **採用** |

## 決策

1. JSON-LD 缺資料時**省略該欄位**，不以預設值填補。
2. canonical 與 OG URL 一律**從 route param 重建**，`lib/seo/urls.ts` 是唯一來源；**不得**從 request URL 取。

## 後果

- **接受**：資料不全的商品拿不到 rich result；新增路由時 `urls.ts` 必須同步補 builder，漏補會讓該頁沒有 canonical。
- **重新評估條件**：搜尋引擎改變對 structured data 完整性的評分方式，或框架提供了能安全剝除追蹤參數的 request URL 取得方式。

現行規格（各 schema 的產生規則、`urls.ts` 的責任邊界）見
`docs/詳細設計書/48-SEO 優化與現況（SEO Optimization）詳細設計書.md` §4、§7.3。
