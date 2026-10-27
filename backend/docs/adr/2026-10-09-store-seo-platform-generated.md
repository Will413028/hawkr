# 店家頁的 SEO 由平台從可見資料生成，不留手寫欄位

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-09，選項 C） |
| 日期 | 2026-10-09 |
| 影響範圍 | seller 模組（`stores.seo_title`、`seo_description`、`UpdateSEO`）、`api/` 的 `SellerAdminService.UpdateStore`（刪除）與 `PublicStore`、`Store` 的 SEO 欄位（reserved）、`internal/legacy/sellerserver`、gateway 的賣家 DTO、website 的店家頁 metadata、admin（SEO 頁已於 10e 第 1 刀刪除） |
| 取代 | 設計書 48 的「賣家 SEO override 全鏈路」（admin → gateway → seller-service → revalidate）；`docs/plans/2026-09-29-a3-buyer-seller/step10-decision-brief.md` 的 B7（SEO 維持只有 admin） |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 10e；決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10e-seo-brief.md` |

## 脈絡

店家頁的 title、description 原本有兩個來源：管理員在後台手寫的 `seo_title`（70 字）、`seo_description`（180 字），或退路——title 用 `{店名} | Hawkr`，description 用店家介紹原文（最多 2000 字，不截斷）。同一個頁面的 og:description 另有一套退路。商品頁早已由純函式生成（`productSeoTitle`、`productSeoDescription`），沒有任何手寫欄位；店家頁是全站唯一的例外。

這個機制是設計書 48 照搬公司時期的做法，沒有 ADR 記錄為什麼由小編在後台逐店手寫。B7 把它留到「上線前的 SEO 規劃」，也就是這一刀。

約束：

- 外部（缺資料）：系統未上線，店家不在 sitemap，prod 沒有開索引（`NEXT_PUBLIC_SEO_INDEXABLE`），沒有任何搜尋資料能證明店家需要自訂標題。
- 沒有既有值要保留：系統從未上線，沒有任何手寫值（devseed 設成 NULL），移除欄位沒有資料成本。
- 自訂：設計書 48 與 B7 是公司時期規範與計畫內的暫定，上線前不受相容性約束。
- 順序：店面頁在 10d 之前仍讀舊 REST `/public/sellers/{id}`；舊 `SellerInfo`、gateway DTO 是舊契約的欄位，在 10d 與 gateway 退場前只能不填。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：平台從頁面上看得到的資料生成 title／description（Google 對大型資料庫型網站鼓勵程式化生成 description；Shopify 的標題空白時用樣板；Etsy 的店面標題由平台組，賣家能填的 shop title 同時顯示在店面上，是可見內容） | 失去單店客製標題 | 採用 |
| A 保留後台手寫並修覆蓋率 | 每家店靠小編手寫，不隨店名、介紹、所在地更新；沒寫的店仍輸出 2000 字未截斷的 description；要新增統計與清單查詢維護一個沒有業界對應物的功能 | 否決 |
| B 平台生成預設加賣家可選覆寫（Etsy／Shopify 型） | 隱藏欄位、驗證與快取失效鏈都要維護；上線前沒有搜尋資料證明需要 | 暫不做（見下方觸發） |
| C 只有平台生成，沒有隱藏欄位 | 同基準 | **採用** |

沒有任何 REAL 約束支持 A 或保留後台手寫；B 的推遲理由是缺資料。

## 決策

- **D1 生成規則**：店家頁的 `<title>`、`<meta description>`、og:title、og:description 由 website 的純函式（`lib/seo/store-summary.ts`）從店家看得到的資料生成，meta 與 og 是同一個值。title 是 `{店名} | {縣市}{鄉鎮市區} | Hawkr`，分隔符與商品頁相同，空片段略去、相鄰相同的片段只出現一次；description 在店家介紹夠長（10 字，以 code point 計）時取介紹的摘要，否則由所在地與評分組句，缺的欄位省略，不補空字串或樣板。商品頁與店家頁共用 `website/lib/seo/summary-rules.ts`（門檻、長度上限、截斷、title 片段去重各一份），「與商品頁相同」由程式碼保證，不靠兩份各自的實作 剛好一致。
- **D1a 為什麼是 160**：與商品頁相同的顯示截斷目標，不是 Google 的限制（Google 沒有公布字數上限，摘要依裝置與查詢改寫）。上線後隨 D4 的 Search Console 觸發一併重新評估。
- **D2 刪除手寫欄位**：`stores.seo_title`、`seo_description` 與它們的 CHECK 刪除；seller 模組的 `UpdateSEO`、`UpdateSEOInput`、`NormalizeSEO` 刪除；`SellerAdminService.UpdateStore` 只有 SEO 一種用途，整支刪除，連同權限 `seller.stores.update`；`Store`（後台）與 `PublicStore` 的 SEO 欄位號碼與名稱 reserved。
- **D3 舊契約不填**：`internal/legacy/sellerserver` 的 `SellerInfo` 與 gateway 的 DTO 不再輸出 SEO 欄位；舊 proto（`backend/api/proto/seller/seller.proto`）仍有這些欄位，舊 `seller-service` 還在使用，留到 gateway 與舊服務退場時一起刪。
- **D4 覆寫的終點與觸發**：不開放覆寫。賣家要改善搜尋結果，就改看得到的店名與介紹。觸發條件：上線並開索引、店家進 sitemap 之後，Search Console 顯示店家頁的標題被大量改寫或點擊率異常，且可歸因於標題，才在**賣家編輯店家處**（`UpdateMyStore`）加「搜尋結果標題」（B，Etsy 型），不放回後台。

## 後果

- 失去單店客製標題；上線前沒有使用者，沒有實際損失，之後要加回是加一個欄位。
- 「缺 meta」的稽核交給 Search Console 與稽核工具（它們掃的是實際輸出的 HTML）；後台的 SEO 覆蓋率卡片不再存在。「沒有店家介紹」的提示已在賣家端 B8 的開店進度卡：「建議完成」含「商店介紹」（Will 2026-10-10；website `website/features/seller-center/dashboard/onboardingTasks.ts:276-278`、seller-app `seller-app/lib/features/seller/data/onboarding_tasks.dart:260-262`）。
- 10d 店面頁改讀 `GetStore` 時，生成函式的輸入換成 `PublicStore`（`town_code` 經 `@hawkr/twaddr/lookup` 轉縣市與鄉鎮市區名稱）與 `StoreStats`（評價數與平均），並補店家的 sitemap。
- Rollback：單一 commit，revert 即恢復欄位與 RPC；欄位全為 NULL，沒有資料遺失。
