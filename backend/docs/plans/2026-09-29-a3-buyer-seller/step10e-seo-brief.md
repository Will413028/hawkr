# 決策簡報：店家頁 SEO 該怎麼做（10e 第 1 刀觸發）

> 快照：HEAD `ad7d55822` 加上未提交的第 1 刀改動。

**Will 2026-10-09：選 C。**

## 結論

**選 C：店家頁的 title／description／OG 由平台從店家「看得到的資料」在 render 時生成，刪掉 `seo_title`／`seo_description` 這組隱藏欄位、後台 SEO 頁與覆蓋率卡片。** 不留後台手寫覆寫，也暫不開放賣家覆寫；要加覆寫的觸發是「上線開索引後，Search Console 顯示店家頁有可歸因於標題的問題」，屆時加在賣家編輯店家的地方（Etsy 模式），不是後台。

10e 第 1 刀：**SEO 頁直接刪除，不再改寫**（目前未提交的 SEO 頁改寫作廢）。後端、契約與 website 的移除是另一個 commit（建議 10e 第 1b 刀），不擋第 1 刀的其他項目。

---

## 1. 事實

**誰能改**
- 只有管理員：`SellerAdminService.UpdateStore`，mask 只限 SEO 兩欄（`api/hawkr/admin/seller/v1alpha1/seller.proto:38-39,288-290`）→ 模組唯一的寫入者 `UpdateSEO`（`backend/internal/modules/seller/internal/profile/admin.go:20-61`、`db/queries/stores.sql:77-84`）。
- 賣家改不了：`UpdateProfileInput` 沒有 SEO（seller `AGENTS.md:45`、`CARRYOVER.md:29` 的 8a 決定），`SellerAppService` 的契約也沒有 SEO 欄位（`rg -n -i seo api/hawkr/seller/v1alpha1/seller.proto` 只命中 `PublicStore:973-976`）。B7「維持只有 admin」的觸發條件就是「上線前的 SEO 規劃」（`plans/…/step10-decision-brief.md:153`、計畫 `:154`）——也就是現在。
- 沒有任何自動生成寫進這兩欄；devseed 把它們設成 NULL（`devseed.go:155`）。系統從未上線，沒有任何手寫值需要保留。

**website 怎麼出 meta**（店面頁仍讀舊的 `/public/sellers/{id}`，`website/lib/api/cached.ts:277-287`；10d 才改 `GetStore`）
- title：`sellerSeoTitle`——有 `seoTitle` 就原樣用（連品牌都不加），否則 `{店名} | Hawkr`（`website/lib/seo/metadata.ts:162-170`）。
- description：`seoDescription || description || "{店名} - 在 Hawkr 看今天有什麼"`（`website/features/sellers/storefront/metadata.ts:34`）。**`description` 最多 2000 字（`model/validation.go:47`）且原樣輸出、不截斷**；OG description 另有一套 fallback（含 location、評分，`:39-41`）。
- JSON-LD 用 `name`、`description`、logo（`app/(public)/sellers/[id]/(storefront)/page.tsx:41-55`）。
- **商品頁已經是「平台生成」**：`productSeoTitle`（商品｜店家｜品牌，`metadata.ts:188-199`）、`productSeoDescription`（描述夠長就截斷摘要，否則用產地、分類、店家組句，`lib/seo/product-summary.ts:117-139`）。商品沒有任何手寫 SEO 欄位。店家頁是全站唯一的例外。
- sitemap 尚未收錄店家（`website/app/sitemap.ts:7`）；prod 索引要手動開 `NEXT_PUBLIC_SEO_INDEXABLE`（設計書 48 `:269,296`）。

**後台 SEO 頁在量什麼**（`admin/src/app/admin/seo/`）
- 載入 `ListStores` 第一頁 100 筆（`page.tsx:31,55-58`；契約上限 100，`admin/…/seller.proto:234`），「覆蓋率」是這一批的統計（`page.tsx:70`）。
- 「SEO 殘缺」= 沒有簡介或沒有圖（`coverage.ts:14-16`）——這是**店面內容完整度**，不是 SEO。而 logo 本來就是公開曝光的必要條件（`SellerRequirement` LOGO，`api/hawkr/seller/v1alpha1/seller.proto:259`），所以對公開中的店，圖片那半永遠成立；清單又含尚未公開的店（`ListStores` 預設是營運中＋等待刪除，`:239`），它們的店面頁本來就 404，SEO 無意義。
- 「已填 override」= 有沒有手寫兩欄之一（`coverage.ts:9-11`）。
- 第 1 刀未提交的改寫（`git diff HEAD --stat`：`page.tsx` +170/−61、`coverage.ts`、新增 `seo-update.ts`、`admin/tests/seo-update.test.ts`）把它搬到 Connect、加 etag 與 `/70` `/180` 計數器（D2）。

**這個機制為什麼存在**：公司時期設計書 48 的目標就是「賣家 SEO override 全鏈路」admin → gateway → seller-service → revalidate（`設計書 48:18,29,89,121`），小編在後台手寫。沒有 ADR 記錄為什麼是後台而不是賣家或自動生成；8a／8e 是照搬（CARRYOVER:29），B7 明說留到 SEO 規劃再決定。

**能自動產生 meta 的資料**（公開店一定有的打 ✓）：店名 ✓、鄉鎮市區代碼 ✓（`town_code`，縣市是前 5 碼）、logo ✓（以上三個是曝光條件 `seller.proto:257-261`）；店家介紹、得獎紀錄、封面（選填，`PublicStore:956-972`）；評價數與平均、粉絲數（`StoreStats`，讀得到才設）；在售商品與其分類、產地（商品模組）。

## 2. 業界基準

| 系統 | 做法 | 來源 |
|---|---|---|
| Google Search Central | 每頁 title 要不同、避免樣板重複；大型資料庫型網站**鼓勵程式化生成** description；snippet 主要取自頁面內容，meta 只在更準時才用 | [snippet](https://developers.google.com/search/docs/appearance/snippet)、[title link](https://developers.google.com/search/docs/appearance/title-link) |
| Shopify（店家擁有整站） | title 空白時用「商店名＋內容標題」的樣板；覆寫在**該資源自己的編輯頁**（Search engine listing），沒有獨立的覆蓋率頁 | [Shopify Help](https://help.shopify.com/en/manual/promoting-marketing/seo/adding-keywords) |
| Etsy（市集，最接近我們） | 店面頁 title 由平台組（含店名），賣家可填 **Shop title**——它同時顯示在店面上，是看得到的內容，不是隱藏欄位；在賣家的 Shop Manager 編輯 | [Etsy community](https://community.etsy.com/t5/New-To-Etsy/Shop-Improvement-and-Search-Engine-Optimization-SEO/td-p/97201121)、[Semrush](https://www.semrush.com/blog/etsy-seo) |
| 多賣家套件（WCFM 等） | 賣家頁 title 自動填店名；手寫 meta 欄位多半只給平台自己的頁 | [WCFM forum](https://wclovers.com/?p=60525)、[Webkul](https://webkul.com/blog/adding-metadata-to-custom-seller-domain-multivendor-marketplace-for-shopify/) |
| 「缺 meta」的報表 | 是**稽核工具**的職責（Ahrefs Site Audit「Meta description tag missing or empty」、Search Console），掃的是實際輸出的 HTML，不是資料庫欄位 | [Ahrefs](https://help.ahrefs.com/en/articles/2630975-meta-description-tag-missing-or-empty-error-in-site-audit) |

未查證：Amazon、蝦皮、momo 店面頁有無賣家可填的 meta 欄位（官方文件找不到，不拿來當論據）。找不到任何市集是由**平台營運人員逐店手寫** meta 的例子。

## 3. 選項

| | 做法 | 取捨 |
|---|---|---|
| **A** 保留後台手寫＋修覆蓋率 | 後端加 SEO 統計聚合（或 ListStores 篩選 `has_seo_override`）、清單 keyset 分頁 | 修好的是統計，不是 SEO：每家店靠小編手寫，不隨店名、介紹、所在地更新而更新（手寫值會過期）；沒寫的店仍然是 2000 字未截斷的 description；要新增契約欄位與查詢來維護一個沒有業界對應物的功能 |
| **B** 平台生成預設＋可選覆寫（Shopify／Etsy 型） | website 用純函式從店家資料生成；覆寫欄位移到賣家編輯店家的地方（`UpdateMyStore`），後台頁刪除 | 覆寫是隱藏欄位，賣家多一個要懂的東西；上線前沒有搜尋資料證明需要；保留的欄位、驗證、快取失效鏈都要維護 |
| **C** 只有平台生成，沒有隱藏欄位（**建議**） | 同 B 的生成，但刪掉 `seo_title`／`seo_description`；賣家要改善搜尋結果就改看得到的店名與介紹（Etsy 的 shop title 也是看得到的內容） | 最少機制、與商品頁一致（商品本來就沒有手寫 SEO）；失去「單店客製標題」——上線前沒有使用者，沒有實際損失；之後要加回只是加一個欄位 |

C 生成規則的草案（實作時定細節）：title `{店名}｜{縣市}{鄉鎮} | Hawkr`（重複片段去除，照 `productSeoTitle` 的寫法）；description 照 `productSeoDescription`：介紹夠長就截斷摘要，否則用所在地、主要分類、評價數組句；OG 與 meta 同一個值（消除現在兩套 fallback）。

## 4. 約束帳

| 把我們推離基準的約束 | 類別 | 來源 |
|---|---|---|
| 設計書 48 定義了 admin override 全鏈路 | SELF-IMPOSED（公司時期規範；AGENTS.md「上線前不受相容性約束」） | `設計書 48:29,89` |
| B7「SEO 維持只有 admin」 | SELF-IMPOSED（計畫內的暫定，觸發條件就是現在） | 計畫 `:154`、`step10-decision-brief.md:153` |
| 10e 第 1 刀範圍只有 admin | SELF-IMPOSED（計畫切法）；但它決定「後端與 website 的移除另一個 commit」 | `step10e-decision-brief.md` 切分表 |
| 店面頁還讀舊 REST `/public/sellers/{id}`，gateway DTO 與 legacy `sellerInfo` 帶 seo 欄位 | REAL 但只影響順序：10d 才改 `GetStore`；移除欄位時 legacy 轉換（`internal/legacy/sellerserver/server.go:136-137`）與 gateway DTO（`gateway/internal/handler/seller.go:56-57,115-116,171`）要一起改，在我們的邊界內 | 計畫 `:255` |
| 沒有搜尋資料（未上線、店家不在 sitemap、prod 未開索引） | REAL（缺資料）——它是「現在不做覆寫」的理由，也是加覆寫的觸發點 | `sitemap.ts:7`、設計書 48 `:296` |
| 沒有既有手寫值要遷移 | REAL（有利）：drop column 零資料成本 | 從未上線；`devseed.go:155` |

沒有任何 REAL 約束支持 A 或保留後台手寫。

## 5. 建議與步驟

**建議 C。** 終點：店家頁 meta 全由 website 純函式從公開資料生成，與商品頁同一套規則；全站 meta 品質由上線後的 Search Console／稽核工具看實際 HTML。**觸發**：開索引並收錄店家 sitemap 後，若 Search Console 顯示店家頁標題被大量改寫或點擊率異常，才在賣家的店家編輯加「搜尋結果標題」（B，Etsy 型），不放後台。

| 步驟 | 內容 | 擋不擋 |
|---|---|---|
| **10e 第 1 刀（現在）** | 刪 `admin/src/app/admin/seo/`（`page.tsx`、`coverage.ts`、未提交的 `seo-update.ts`）、`admin/tests/seo-update.test.ts`、Sidebar「SEO管理」（`Sidebar.tsx:157`）、`MerchantManagement.css:424` 提到 SEO 頁共用 `.merchant-management` 的註解（樣式本身其他頁還在用，只改註解）；`admin-removed-seller-routes.test.ts` 視需要加 `/admin/seo`。gateway `PATCH …/seo` 已刪，不變 | 不擋；比改寫還小 |
| **10e 第 1b 刀（緊接著，同一步驟內，不留沒有呼叫者的 RPC）** | 契約：刪 `SellerAdminService.UpdateStore`（只有 SEO）與 `Store.seo_*`，`PublicStore` 10、11 `reserved`；模組：刪 `UpdateSEO`、`NormalizeSEO`、`Profile.SEO*`、查詢，migration drop 兩欄與 CHECK、改 `stores_pii_purged_check`；legacy `sellerInfo` 與 gateway DTO 不再填；website 的 `sellerSeoTitle` 去掉覆寫分支、description 改用新的生成函式並截斷、刪 `tests/seo/seller-seo-override.spec.ts`；設計書 48 標為已被取代，CARRYOVER:29 補沿用盤點，寫 ADR（取代 B7） | 擋 10e 收尾，不擋第 2 刀之後 |
| **10d（website 公開讀取）** | 店面頁改讀 `GetStore` 時，把生成函式的輸入換成 `PublicStore`＋`StoreStats`（縣市、評價），補 sitemap 收錄店家 | 不擋 10e |
| （可選，需 Will 同意的 UI） | 「沒有店家介紹」原本是覆蓋率卡片想抓的訊號，正確位置是賣家端 B8 的清單卡加一項「建議填寫」 | 不擋 |

**Rollback**：1b 是單一 commit，revert 即恢復欄位與 RPC；沒有資料遺失（欄位全為 NULL）。

**要 Will 決定的只有一件**：C（不留覆寫）或 B（覆寫改由賣家在店家編輯填）。
