# 商品匯入精靈 — 真實 Shopee 樣本 CSV 與重跑指引

> 配套檔：[`product_import_e2e_sample.csv`](./product_import_e2e_sample.csv)
>
> 本檔是 **raw**：蝦皮 `.product-detail` 整段 outerHTML 直接塞入 CSV description 欄位，零處理。E2E 已驗證 **5/5 commit ACTIVE 成功**。含 `<h2>` / `<h3>` / `<a>` / `<picture><source><img>` / 蝦皮 CSS class，最完整的 §12 HTML→blocks 轉換壓力測試樣本。
>
> 早期曾有預處理過的 v1（剝 CSS class / 剝 picture wrapper / 平展 `<br>`），但因 raw 已能 5/5 完整上架且 §12 涵蓋面更廣，v1 不再保留。

## 用途

本檔提供一份**從蝦皮（shopee.tw）抓取的真實生鮮商家商品**轉換而來的合規 CSV，作為：

- 後端工程師 dev 環境 smoke 測試的真實基準資料
- 賣家匯入流程的真實樣本（覆蓋 §12 HTML→blocks 轉換的多種情境）
- E2E 自動化驗證的 fixture
- 跨環境 SSRF 行為的對照（蝦皮 CDN `down-tw.img.susercontent.com` 已驗證 ✅ 通過 SSRF 白名單）

## 5 個真實蝦皮商品來源

| product_key | 蝦皮原商品 | 蝦皮 URL（商品 ID） | dev 分類對應 |
|---|---|---|---|
| `SHOPEE1` | 盛花園 台中梨山高麗菜 1.2~1.5kg | `i.418518320.24522696766` | 528 包白 ✅ |
| `SHOPEE2` | 快樂農夫 三地門 40 年老欉 土芒果青 金煌 | `i.40439027.9721656099` | 745 芒果青 ✅ |
| `SHOPEE3` | 屏東九如 楊媽媽 A 級有籽檸檬 | `i.127581581.11516583944` | 758 檸檬 ✅ |
| `SHOPEE4` | 示範農場 TGAP 產銷履歷 無籽檸檬 | `i.33342483.16099099358` | 761 無子檸檬 ✅ |
| `SHOPEE5` | 彰化溪州 在地產地直送 珍珠芭樂 | `i.16235049.138142288` | 829 珍珠芭 ✅ |

> ⚠️ 早期版本曾用奇異果（綠之果物 Zespri）做 SHOPEE1，但 dev 分類樹沒對應 leaf；改用高麗菜（對應 528 包白 leaf）後 5/5 commit ACTIVE 全通。

> ⚠️ category_id **必須是 L3 leaf**（無 children 的 node）。L2 parent（如 184 酪梨 / 174 番石榴）會被 `CATEGORY_INVALID NOT_LEAF` 擋下。確認 leaf 用：`curl /api/v1/public/categories | jq '.data[] | select(.id=="X")'` 然後檢查有無其他項目 parentId=X。

## 涵蓋的 §12 HTML 轉換場景（真實蝦皮描述風格）

詳細設計書 [40-商品匯入精靈服務（Product Import Service）詳細設計書.md](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md) §12

蝦皮後台給賣家的編輯器只支援「純文字段落 + 圖片插入」，沒有富文本 `<h>` / `<ul>` / `<strong>` 標籤。所以本樣本**真實反映蝦皮賣家描述的典型結構**：

| 商品 | HTML 結構 | 對 §12 的測試重點 |
|---|---|---|
| SHOPEE1 高麗菜 | `<h2>`(商品規格) + `<a>` 分類麵包屑 + 多 `<p>` 描述 | h2 階梯 / `<a>` 超連結 warning / 描述 60 blocks |
| SHOPEE2 芒果青 | `<h2>` + 大量 `<p>` + emoji + `<a>` | `<a>` warning / 描述 38 blocks |
| SHOPEE3 檸檬 | `<h2>` + 多 emoji + 多警示符號 + `<a>` | 特殊 unicode / 描述 68 blocks |
| SHOPEE4 TGAP 檸檬 | `<h2>` + `<p>` 履歷說明 + `<a>` | 履歷說明 / 描述 37 blocks |
| SHOPEE5 珍珠芭樂 | `<h2>` + 編號清單純文字 + 警示 emoji + `<a>` | 編號清單純文字 / 描述 85 blocks（最高） |

**實測結果**：5/5 commit ACTIVE 成功，total blocks 從 37 ~ 85 分布。HTML_PARSE_WARNING 觸發如下：

| Warning | 觸發 case |
|---|---|
| `超連結未保留，僅保留文字` | `<a>` 從蝦皮商品規格的分類麵包屑（4-5 條 per item）→ §12 拿掉 href 保留文字 |
| `表格已展平為純文字段落` | 早期 v1 手工 CSV 用 `<table>` 時觸發；本 raw 樣本未含 `<table>` |

## 環境依賴

| 項目 | 值（dev） |
|---|---|
| 賣家 | `alexischen@example.com`（accountId=2683382637） |
| 賣家已開通 shipping | 黑貓 3001、7-11 常溫 3002+3201、Uber 3003 |
| 圖片 URL allowlist | **`down-tw.img.susercontent.com` ✅ 已驗證 SSRF 通過** / `placehold.co` ✅ / `images.unsplash.com` ✅ / `picsum.photos` ❌ cross-domain redirect / `cdn.example.com` ❌ internal |
| Gateway | `https://test-api.example.com` |
| Web | `https://web-dev.example.com` |
| 每日上傳額度 | **10 jobs / 帳號 / 天** — `PRODUCT_IMPORT_RATE_LIMITED` (HTTP 429) |

換賣家帳號跑時記得：

1. 對應 `accountId` 的 shipping methods 重新確認（`GET /api/v1/app/shipping_methods`）— 若未開通對應 method 會被 downgrade
2. 重跑同樣資料一定要改 `product_key` + 改檔名（**雙重 dedup**：filename 鎖 + content hash dedup）
3. hashtags 欄位**禁止 `#` 前綴 / 空白 / emoji / 標點**（commit 時 product-service 會拒）— 本樣本已調為純文字
4. `target_status=ACTIVE` 要 commit 才會公開上架；若只想測流程不污染商城，改 `DRAFT`

## 重跑步驟

### A. API 直打（最快驗證 §12 轉換 + SSRF）

```bash
TOKEN=<your token>  # via .claude/skills/ui-verify/session-inject.md
curl -X POST "https://test-api.example.com/api/v1/app/product_import_jobs" \
  -H "Cookie: __Host-hk_session=$TOKEN" \
  -H "Origin: https://web-dev.example.com" \
  -H "X-Requested-With: XMLHttpRequest" \
  -F "file=@docs/補充資料/product_import_e2e_sample.csv" \
  -F "filename=shopee-$(date +%s).csv" \
  -F "idempotency_key=manual-$(date +%s)" \
  -F "target_status=ACTIVE"

# 預期：status=9102 (CREATED)
# 10-15 秒後 GET /product_import_jobs/{jobId}
#   → status=9104 (READY), counters.itemsReady=5, counters.assetsUploaded>=15
# 查單筆 item 詳情，descriptionJson.blocks[] 多為 text block（無 fontSize 因為純 <p>），偶有 inline image block（SHOPEE1）
```

### B. UI 全流程（透過 web wizard）

打開 `https://web-dev.example.com/seller-center/products/import` →「新增匯入任務」→ 上傳 → 等 READY → 預覽 → Commit。

## 預期結果（API 直打路徑）

```
POST /product_import_jobs              → status=9102 (CREATED), validationErrors=[]
~5-10s（5 product × 3 image = 15 assets 下載）
GET  /product_import_jobs/{jobId}      → status=9104 (READY)
  counters.itemsTotal=5 / itemsReady>=2 / assetsUploaded=15/15
  ※ 首次跑同 product_name 在 dev 環境會撞 DUPLICATE_NAME_VS_EXISTING（之前 commit 過同名），需 PUT allow_duplicate_name=true 化解
GET  /items/{id}（任一）                 → brief.warningCount=4-5 (HTML_PARSE_WARNING 超連結)
  descriptionJson.blocks 37 ~ 85 個 text block（依商品而異）
PUT  /items/{id} body={"expectedVersion":"0","allowDuplicateNameSet":true,"allowDuplicateName":true} → 化解 blocking
POST /product_import_jobs/{jobId}/commit → status=9105 (COMMITTING), queuedCount=5
~5-10s
GET  /product_import_jobs/{jobId}       → status=9106 (COMMITTED)
  5 個商品落地 product table，status='4004' (ACTIVE)
GET  /api/v1/public/products/{formalProductId} → 完整 PDP 資料含 description.blocks[]
```

**實測結果**（2026-06-18 dev）：5/5 commit ACTIVE 成功，formal product 全 4004 公開可訪問。

## 已知 commit 失敗誘因（從本次 E2E 學到）

| commitFailedReason | 怎麼避開 |
|---|---|
| `hashtag 含不允許字元（僅接受中文/英文/數字，不可含空白、#、emoji 或標點）` | hashtags 欄位**禁止**用 `#` 前綴、空白、emoji；本 CSV 已調為純文字 |
| 各 SKU 配送方式設定的 method 賣家未開通 | 跑前 `GET /shipping_methods` 確認，sku_shipping_* 只用已開通的 method |
| `product_image_1 必填` | 每 row 至少 `product_image_1` 一張圖；同一 product_key 的多 SKU row 都要重複填 image |

## 給後續維護者的教訓

1. **CSV template 與 product-service 驗證規則不一致**（HIGH）：`hawkr-website/public/templates/hawkr_import_template.csv` 範例 hashtags 是 `#當季#在地小農`，但 commit 時被 product-service 擋。template 跟驗證規則必須對齊
2. **hashtag 分隔符未定義**（HIGH）：去 `#` 後變單一 hashtag 整段（`Zespri紐西蘭黃金奇異果` 整個一個 tag），無法拆多個。需明定分隔符（逗號 / 空白）
3. **`<br>` 平展為空白、`<blockquote>` 靜默降級、inline `<strong>`/`<em>`/`<span style="font-size">` 未實作**（MED）：§12.3 inline run 拆解部分未實作；建議補 warning enum
4. **每日上傳額度 10 jobs / 帳號**（LOW）：除錯時連續多次測試會撞 429，要分散跨日或多帳號
5. **dedup 機制**：相同 (filename + content hash) 即使前一個 job 已 cancelled 也會 dedup hit 回舊 jobId。換檔名 + 改 product_key 才能跑新 job
6. **dev 分類樹缺奇異果 leaf**：實務上 dev 應補上完整水果 leaf 才能對應蝦皮所有商品

## 對應 issue / PR

- Issue 觸發點：hawkr-website 前端對接 product-import-service（#115 / #120 已合入）
- 對應後端設計書：[40-商品匯入精靈服務（Product Import Service）詳細設計書](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md)
- 本 sample 由 2026-06-18 dev E2E 驗證產生，搭配真實蝦皮商品作 reference

## 蝦皮資料抓取方法（給後人參考）

本樣本透過 macOS Safari + AppleScript `do JavaScript` 即時抓取，**不是**爬蟲：

1. Safari 開啟 → 設定 → 進階 → 顯示開發選單 → 開發 → 允許從 Apple Event 執行 JavaScript
2. 在 Safari 開啟蝦皮商品頁面，等 SPA 渲染完成
3. AppleScript：`tell application "Safari" to do JavaScript "document.querySelector('.product-detail').outerHTML" in current tab of front window`
4. 解析 `.product-detail` 內 `商品描述` section 的 `<p>` 內容 + `<picture><img src>`

蝦皮商品描述的 selector：`.product-detail` → `section` → `h2:商品描述` → `div.QN2lPu`（每段）+ `picture > img`（內嵌圖片）。

> ⚠️ 此方法只用於**內部測試資料生成**，不應作為生產用蝦皮爬蟲；蝦皮 ToS 禁止自動化大量抓取。
