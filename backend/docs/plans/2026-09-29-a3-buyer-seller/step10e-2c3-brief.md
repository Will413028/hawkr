# 10e 第 2c-3 刀（終止措施 TERMINATION）：決策簡報（定案版）

輸入：兩份唯讀盤點（backend、client，未進版控）、一份獨立審查（未進版控）與一份業界基準查證（未進版控）；SSOT `step10e-2c-design.md`（下稱 D：§0 G4、G15、§1、§5、§6、§8、§9 W1=(c)、W2=(a)）、`step10e-measure-brief.md`（下稱 MB）、`step10e-2c2-brief.md`（下稱 B2）、條款 `website/docs/legal/seller-terms.md` 第 17、18、21 條（版本 2026-10-15）。worktree `hawkr-10e`，branch `local/10e`，HEAD `c300b405f`。路徑縮寫：`S`=`backend/internal/modules/seller`、`L`=`backend/internal/legacy`、`SV`=`backend/services`。

定位（沿用 MB:6、B2:7）：DSA、P2B 是**參考基準**，不是台灣的法遵義務，但「台灣沒有 P2B」只是少了外部執法者，**不是**推離基準的限制；條款是 Will 的草稿，未經律師審閱。系統未上線，「和既有程式一致」「改動比較小」都只算 SELF-IMPOSED。外部來源在 2026-10-15 查過；只有二手來源的標 **SECONDARY**，找不到一手來源的標 **NOT FOUND**，兩者都不作為依據。

本版是 Will 2026-10-15 的決定（「全照這版做」）：v2 的 W7–W11、R1–R6 定案，另加申覆期間六個月、依關閉原因的保留期、關聯帳號的連帶撤銷與通知期 30 日。被排除的選項只留一句理由。

---

## 0. 決定總表

### Will 2026-10-15 的決定

| # | 題 | 決定（Will 2026-10-15） | 業界基準 | 排除的選項 |
|---|---|---|---|---|
| **W7** | 終止後的關店、恢復與不可逆點（T1） | **(b1) 所有關店都可逆**；不可逆的下游（relay 的 `seller.deleted`、logo／封面釋放）移到保留期滿的個資清理，由新事件 `StoreRetired` 觸發。**平台終止的申覆期間是終止生效後 6 個月**；由 `TERMINATION` 關閉的 stint，保留期就是申覆期間（期滿且沒有處理中的申覆才清理），賣家自己關店與其他關閉維持 30 天。帳號上有 `TERMINATION` 的待處理申覆時清理暫停（legal hold）。申覆期間內撤銷或接受申覆＝恢復同一個店（store_id、店號、商品、KYC、資料）；期滿後不受理終止的申覆，撤銷只撤銷紀錄與禁止入駐。兩個保留期的目的不同（爭議處理 vs 資料最小化），不是特判 | 停用與刪除分開、刪除走保留期（Google Cloud、Shopify 終止後保留 2 年、eBay 以限制處理代替刪除）；P2B 4(3) 撤回時恢復並交還生效前資料；DSA 20(1) 申訴期「at least six months」 | (a) 延後關店：不可逆點仍綁在關店，只是延後；(b2) 只延後終止的關店：為單一案例加特判；(c) G15 原案：關店後的「恢復」做不到；(d) 不關店：保留期無上限 |
| **W8** | 清理後的身分雜湊（T5i） | 21.3 終止的身分雜湊存在措施的身分快照（本來就不清理），**保留到終止撤銷為止**；清理後才發現的 21.3 事由是接受的缺口。條款 21.3、賣家身分認證告知事項與隱私政策寫明目的與保留期（個資法 §11 III 但書、施行細則 §21 的「契約約定之保存期限」「其他不能刪除之正當事由」） | Uber 只對因詐欺或不安全行為被封禁者在刪除後保留資料以防再進入；蝦皮台灣：曾有違規紀錄者刪除後可能拒絕新帳號申請。雜湊化保存：NOT FOUND | (ii) 所有關閉的 stint 保留雜湊 N 年：超出目的；(iii) 只保留曾有措施的 stint：清理前已由快照涵蓋，多一份儲存 |
| **W9** | 同身分的其他已開店帳號（T5h） | 系統在措施詳情自動列出同身分的其他帳號；管理員對每個帳號**另建一筆**措施（事由「關聯帳號」，`source_measure_id` 指向來源措施）；來源措施撤銷時，以它為來源的措施在**同一個 transaction** 撤銷（與 G3 連帶撤銷同一族機制）；每筆關聯措施可各自申覆 | Google Ads：關聯帳號可被停權、每個帳號分別申訴，「your account will be automatically reinstated once those other accounts are successfully appealed」；eBay、Etsy 同樣把關聯帳號納入 | 自動處分：每個決定要自己的理由（DSA 17），且會誤傷共用統編；只寫 runbook：不留紀錄、撤銷時不會連動 |
| **W10** | 條款 21.2–21.4（T6） | 21.2 預告 **14 → 30 日**；21.3 第 1 款保留 7 日改善期並適用 R5，其他 21.3 事由立即；21.4 的恢復期間由「關店後 30 日」改成「申覆期間（終止生效後六個月）內」；21.3 加身分雜湊的目的與保留、關聯帳號；內文改寫、不移號 | Amazon BSA §3（便利終止 30 日、重大違約 7 日 cure、詐欺立即）；P2B 4(2) 30 日、4(4) 例外 | v2 的 W7=(a) 版 21.4（不採用 (a)） |
| **W11** | client 文案與流程（T7） | 照 v2，數字改為「30 天後生效」「終止生效後 6 個月內」，admin 措施詳情加關聯帳號區塊 | Google MC：狀態與到期日；Merchant API 每個動作帶可用性 | — |

### 對已定 SSOT 的修訂（Will 2026-10-15 核可）

| # | 修訂 | 原文 | 理由 |
|---|---|---|---|
| R1 | G15「已關閉時重新開啟同一個店，KYC 已清理則回到需補 KYC」→ **申覆期間內撤銷或接受申覆，恢復同一個店；期滿後只撤銷紀錄與禁止入駐**。D §6 2c-3 驗收、D §7「改判…定案為重開同一個店」同步 | D:32、:98、:400、:418 | 清理後商品、貼文、聊天個資、KYC 都已刪，「同一個店」做不到誠實的恢復；保留期與申覆期間對齊後，清理前就是申覆期間 |
| R2 | G4 不新增 `TerminationScheduled`；`MeasureCreated` 加 `StartsAt` | D:21 | 沒有 Measure* 事件的訂閱者（`cmd/hawkr/seller_events.go`）；開始時間是所有措施的屬性 |
| R3 | D §1「`effective_at` 只給終止」、§5 `Measure.effective_time` → **所有措施**加不可變的 `starts_at`，契約沿用既有 `MeasurePeriod.start_time` | D:51、:65、:352 | 生效判斷只有一個式子（T2c） |
| R4 | `MeasureBasis` 加 `CONTRACT`（只配 TERMINATION） | D §3 enum | 21.2 無過錯、21.3 部分事由與關聯帳號不是 RISK（T2a） |
| R5 | 21.3 第 1 款「重大違約」要引用一筆**至少 7 天前**、未撤銷的認定 | G15「21.3 立即」 | 條款本身要求「經通知後七日內未改善」（T5c） |
| R6 | ADR「個資保留」（`backend/docs/adr/2026-10-02-seller-kyc-payuni-in-monolith.md`）與 seller `AGENTS.md`：保留期依關閉原因（`TERMINATION` 為申覆期間，其他 30 天）、legal hold、`seller.deleted` 與 logo／封面釋放改在清理時（`StoreRetired`）、關店紀錄移到歷史表、21.3 身分快照保留到撤銷 | `relay.go:316-320`、`closing.go:242-281`、`purge.sql`、seller `AGENTS.md`:13、:31、:45、:48、:66 | T1；實作項，本簡報不改這兩份 |

### 不需決定（基準直接定案，理由在各節）
T2 模型（`TerminationGround` 必填、終止永不為 RISK、`starts_at`、申覆期限常數、「待關閉」不存、`source_measure_id`）；T3 操作表、`StateOf` 順序（ENDED 先於 SCHEDULED）與申覆不可用原因；T4 身分快照逐措施在建立時取（21.2 不取）、21.2 在**單一關店路徑**結束；關店改用 `LockAccountStores` 鎖順序；T5 各邊界；devseed 改讀 `EffectiveMeasures`；`closed_by=TERMINATION`、系統作者用 `system_cause`。

---

## 外部基準（本簡報查過的）

一手來源逐字查過的列「已查」；只在論壇、部落格或搜尋摘要看到的標 **SECONDARY**，不作為依據。

| 來源 | 要點 | 驗證 |
|---|---|---|
| P2B（EU 2019/1150）Art.4 | (2) 終止全部服務「at least 30 days prior to the termination taking effect, with a statement of reasons … on a durable medium」。(3) 撤回時「reinstate the business user without undue delay, including providing the business user with any access to personal or other data, or both, that resulted from its use of the relevant online intermediation services prior to the restriction, suspension or termination having taken effect」——恢復涵蓋終止，也涵蓋生效前資料的存取。(4) 免 30 日：(a) 法令義務、(b)「exercises a right of termination under an imperative reason pursuant to national law」、(c)「can demonstrate that the business user concerned has repeatedly infringed」；仍要「without undue delay, with a statement of reasons」。(5) 理由指出事實與條款依據 | 已查 [legislation.gov.uk EU 原始版](https://www.legislation.gov.uk/eur/2019/1150/article/4/adopted) |
| DSA（EU 2022/2065） | 17(1)(d) 停權或終止帳號要給 statement of reasons；20(1) 申訴管道「for a period of at least six months following the decision」，涵蓋「(c) decisions whether or not to suspend or terminate the recipients' account」；20(4)「reverse its decision … without undue delay」；20(6)「not solely on the basis of automated means」；23(1) 先警告只綁在頻繁提供明顯違法內容的暫停 | 已查（全文鏡像 [eu-digital-services-act.com](https://www.eu-digital-services-act.com/)、[springlex Art.20](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-20/)）；EUR-Lex 原文因 WAF 未讀 |
| Amazon Business Solutions Agreement §3 | 「We may terminate … for convenience with 30 days' advance notice」；立即終止限「(a) you have materially breached the Agreement and failed to cure within 7 days of a cure notice …; (b) … deceptive or fraudulent, or illegal activity; or (c) … harmed …」；終止後仍要履行終止前成立的交易 | 已查 [NA 版 PDF](https://images-na.ssl-images-amazon.com/images/G/01/rainier/help/legal/BSA_NA_English_redlined.pdf)、[EU 版 PDF](https://m.media-amazon.com/images/G/48/AmazonServices/BSA_English_Updated_Tracked.pdf) |
| Amazon 停用後 reactivate、90 天申訴窗、Account Health 頁 | — | **SECONDARY**（賣家論壇、wiseppc 部落格）；Amazon 何時刪除停用賣家的資料：NOT FOUND |
| Shopify 隱私政策 | 「If you are a merchant … and you close the store, stop paying your subscription fees, or we terminate your account, we retain store information for two years before we begin the deletion process.」 | 已查 [shopify.com/legal/privacy](https://www.shopify.com/legal/privacy) |
| eBay | 停權可 reinstate；隱私通知「we restrict the processing of your data instead of deleting it … in particular … for the assertion of or defense against legal claims」（legal hold 的形狀），並為「detect and prevent fraudulent or illegal activity」保留；Multiple accounts policy「If we apply restrictions or limits to one account, then similar restrictions or limits may be applied to the member's other linked accounts」 | 已查 [4190](https://www.ebay.com/help/default/default/account-holds-restrictions-suspensions?id=4190)、[隱私通知](https://www.ebay.com/help/policies/member-behaviour-policies/user-privacy-notice-privacy-policy?id=4260)、[4232](https://www.ebay.co.uk/help/policies/identity-policies/multiple-accounts-policy?id=4232) |
| Etsy | ToS §7「terminate or suspend your account (and any accounts Etsy determines are related to your account)」 | 已查（Wayback 快照 [terms-of-use](https://web.archive.org/web/2026id_/https://www.etsy.com/legal/terms-of-use)）；關店後能否重開、6 個月申訴期：**SECONDARY** |
| Google Cloud 資料刪除 | soft deletion 是「internal staging and recovery period」；帳號關閉後「internal recovery period up to 30 days」 | 已查 [cloud.google.com/docs/security/deletion](https://cloud.google.com/docs/security/deletion) |
| Google Merchant Center | 停權後「attempting to use the Shopping system again by creating new accounts」屬規避；「Having a suspended account linked to your Merchant Center account is not allowed」；多數違規先警告並給 7 或 28 天 | 已查 [6150118](https://support.google.com/merchants/answer/6150118?hl=en)、[2797483](https://support.google.com/merchants/answer/2797483)、[13693195](https://support.google.com/merchants/answer/13693195?hl=en) |
| Google Ads 停權 | 「Accounts related to the suspended account may be suspended」；「submit a separate appeal for each suspended account」；「If your account was suspended because other accounts verified using the same identity documents were suspended, your account will be automatically reinstated once those other accounts are successfully appealed.」 | 已查 [2375414](https://support.google.com/adspolicy/answer/2375414)、[15938075](https://support.google.com/adspolicy/answer/15938075?hl=en) |
| YouTube 終止 | 終止後不得建立其他頻道；可申訴 | MB 已查 [2802168](https://support.google.com/youtube/answer/2802168?hl=en) |
| Uber 隱私聲明 §II.F | 「if you are banned from Uber's services because of serious fraudulent or unsafe behavior, Uber will retain your data after an account deletion request to prevent you from re-obtaining access」；其他情形一般 90 天內刪除 | 已查 [uber.com privacy-notice](https://www.uber.com/legal/en/document/?name=privacy-notice&country=united-states&lang=en)；封禁資料的保留期限未訂明 |
| 蝦皮台灣 | 說明頁「若您的帳號曾有任何違反蝦皮使用規範的紀錄，則帳號刪除後，可能因歷史交易紀錄拒絕您的新帳號申請」；臺北地院 114 年度訴字第 953 號（2025-07-30）：「已屢次警告並給予改正之機會」，永久停權合法 | 已查 [80367](https://help.shopee.tw/portal/4/article/80367)、[判決](https://judgment.judicial.gov.tw/FILES/TPDV/114%2c%e8%a8%b4%2c953%2c20250730%2c1.pdf)；凍結 60 天未申訴即永久停權：**SECONDARY** |
| momo 客服糾紛處理原則 | 「平台有權逕自關店至改善後再重新開店」——關店是可恢復的狀態 | 已查 [rules.momo.com.tw 00008](https://rules.momo.com.tw/standard/00008/) |
| Stripe、PayPal | 關閉帳號後為防詐保留個資（未點名防重新註冊） | 已查 [stripe.com/privacy](https://stripe.com/privacy)、[PayPal](https://www.paypal.com/us/legalhub/paypal/privacy-full) |
| GDPR Art.17(3)(e)、EDPB Guidelines 1/2024 ¶100–105 | 刪除權例外「for the establishment, exercise or defence of legal claims」；防詐可為正當利益，但要具體說明防哪種詐欺、需要哪些資料，並受 storage limitation 限制 | 已查 [gdpr-info.eu](https://gdpr-info.eu/art-17-gdpr/)、[EDPB](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202401_legitimateinterest_en.pdf) |
| 個資法 §11 III、施行細則 §21 | 目的消失應刪除，「但因執行職務或業務所必須……不在此限」；細則 §21：「有法令規定或契約約定之保存期限」「有理由足認刪除將侵害當事人值得保護之利益」「其他不能刪除之正當事由」 | 已查（[6laws.net 鏡像](https://6laws.net/6law/laws/個人資料保護法.html)、[法務部函釋](https://laws.gov.taipei/law/Interpretation/Content/FE272509)）；全國法規資料庫原文因連線失敗未讀 |

**共通形狀**：(1) 限制（停用）與刪除分開，刪除走保留期；終止後保留的理由是爭議處理與法律主張（eBay、GDPR 17(3)(e)），保留期可以依目的不同（Shopify 終止後 2 年、一般帳號關閉 30 天）。(2) P2B 4(3) 對終止也要求恢復並交還資料，這只有「刪除不綁在生效或關店上」才做得到。(3) 終止是終局決定，不是條件型限制（沒有補正、複審）。(4) 為禁止重新入駐而保留身分資料，業界只對被封禁者保留（Uber、蝦皮），且要寫明目的。(5) 關聯帳號各自處分、各自申訴，來源申訴成功時連動恢復（Google Ads）。(6) 便利終止 30 日、重大違約先 7 日改善、詐欺立即（Amazon BSA、P2B 4(4)）。

---

## T1. 終止後的關店、恢復，以及不可逆點在哪裡（W7）

### 業界基準
停用可逆、刪除在保留期後（共通形狀 1、2）。P2B 4(3)：撤回時恢復，含生效前資料的存取；DSA 20(1) 申訴管道至少 6 個月、20(4) 不遲延地撤回。

### 現有機制為什麼存在
- G15 要求生效後撤回也恢復（P2B 4(3)），「關閉是否可逆」當時列為盤點（D §8）。
- `closing.close`（`S/internal/closing/closing.go:242-281`，已核對）是為「賣家離開」設計，一次做完：`stores` 設 CLOSED、釋放 logo 與封面（media 7 天後 GC）、移除 SELLER、發 `StoreClosed`、公告不可曝光。**不可逆點是 relay 的對應**：`StoreClosed` → `seller.deleted`（`L/sellerrelay/relay.go:316-320`，**在單體內**）。消費者（在邊界外）：product `offShelfAllByMerchant` 全部 ACTIVE 商品改 INACTIVE 並記 `seller_deleted_at`；social 軟刪貼文並釋放圖片；im `RedactDeletedAccount` 遮蔽聊天個資；notification 清收件匣個資、軟刪該 incarnation 的收件匣、停用 device token；pricebook 刪該帳號的 market watch（`SV/*/internal/worker/seller_deleted.go`，已核對五個）。
- 個資清理（`S/internal/purge`，ADR「個資保留」）：關店滿 30 天清店家與 KYC 個資（含 blind index，`db/queries/purge.sql`），措施的身分快照保留。
- **停權**的下游大致可逆：product 只讀 `seller.exposure.changed`；coupon 停用／還原模板；order 收到 `seller.suspended` 批次取消未付款單（不還原）；im 斷開 WS（重連即可）。

### 決定（W7 = (b1)）
- 生效＝停權；未結訂單完成就關店（`closed_by=TERMINATION`、`closed_by_measure_id`）。
- `StoreClosed` 不再對應 `seller.deleted`。**所有**關店的不可逆下游都在個資清理時：清理的同一個 transaction 釋放 logo／封面、發新事件 `StoreRetired` → relay 發 `seller.deleted`。
- **申覆期間**：終止措施的 `appeal_deadline = starts_at + 4416 小時`（184 天，T2c）。期間內（含 21.2 的預告期）可申覆；期滿後 `can_appeal=false`、原因 `APPEAL_WINDOW_ENDED`（T3）。已在期間內提出的申覆照常裁定。
- **保留期依關閉原因**（目的不同：`TERMINATION` 的保留是為處理爭議，其他關閉是資料最小化）：
  - `closed_by=TERMINATION`：`retire_after = greatest(closed_at + 30 天, 帳號上未撤銷的終止中最晚的 appeal_deadline)`（Will 2026-10-16 修訂：已撤銷的終止不延長，造成關店的那一筆撤銷後也不再延長）。第一項只在訂單拖過申覆期間時起作用，保住 21.6「終止後三十日內請求資料」。
  - 其他（`ACCOUNT_OWNER`、`ACCOUNT_DELETION`）：`retire_after = closed_at + 30 天`（不變）。
- **legal hold**：帳號上有 `TERMINATION` 的待處理申覆，或帳號上的關聯終止（T5h）其來源措施有待處理申覆時，該帳號所有已關閉 stint 的清理暫停。比照 eBay「restrict the processing … instead of deleting」與 GDPR 17(3)(e)。
- **單一判斷式**：`RetireDue(stint, now) = 已關閉 ∧ 未清理 ∧ now > retire_after ∧ ¬hold`。清理只清 `RetireDue` 的 stint；`Reopen` 只在 `¬RetireDue ∧ 未清理` 時進行。兩者用同一個 SQL 片段與 Go 函式，所以「恢復或只撤銷紀錄」只由資料決定，不受清理 job 的延遲影響。
- 申覆期間內撤銷或接受申覆＝**重開同一個店**；之後＝撤銷紀錄與禁止入駐，帳號可重新開店（新 store_id）。

### 排除的選項（各一句）
- (a) 延後關店：不可逆點仍綁在關店，有待處理申覆時店家無限期卡在停權，也沒有解決其他關店的同一個問題。
- (b2) 只有 `closed_by=TERMINATION` 延後 `seller.deleted`：為單一案例加特判（root `AGENTS.md`「不留技術債」）；本決定的差異是**保留期長短**依目的不同，延後機制本身對所有關店一致。
- (c) G15 原案：商品已 INACTIVE、貼文與聊天個資已刪、KYC 已清，名為恢復，做不到。
- (d) 不關店：個資保留期無上限，違反 ADR「個資保留」。

### 限制帳本

| 限制 | 推離基準的方向 | 性質 | 來源 |
|---|---|---|---|
| `seller.deleted` 的消費者行為不可逆（五個服務） | 不推離：它決定「刪除做什麼」，不決定「何時發」 | REAL（product、social、im、notification、pricebook 的遷移步驟前在邊界外） | 上列 `seller_deleted.go`、`SellerProductOffShelfService.java:53-72` |
| relay 在關店時發 `seller.deleted` | 推向 (a)／(c) | **SELF-IMPOSED**（單體內的對應，改掉） | `relay.go:316-320` |
| `close()` 釋放圖片、`stores_closed_images_check`、`stores_closure_check`、關店紀錄「永久保留」 | 推向 (a) | SELF-IMPOSED（改；關店紀錄改存歷史表仍永久保留） | `closing.go`、`schema.sql:59-77`、seller `AGENTS.md`:45 |
| 個資清理 30 天（含 blind index） | 不推離：它就是一般關閉的保留期邊界 | SELF-IMPOSED（ADR，自己的資料最小化政策；`TERMINATION` 改用申覆期間，其他不改） | `purge.sql`、`purge.go` |
| 條款 17.4「申覆不設期限」 | 只涵蓋第 17 條的措施；終止的申覆期間由 21.2 另訂 6 個月 | SSOT（條款草稿） | `seller-terms.md:176` |
| 是否有法規要求「賣家角色關閉」立即刪除聊天個資 | 若有，推向 (b2) 或 (a) | **未查證**。im 註解引用的 Apple 5.1.1(v) 指 App 內刪除**帳號**，走 `account.deleted`，不受影響 | `SV/im-service/internal/worker/seller_deleted.go:13-24` |
| 改動範圍大 | 推向 (a) | SELF-IMPOSED | — |

沒有 REAL 限制推離基準。

### 機制
- 「待關閉」不存：生效中的 TERMINATION ∧ stint 未關閉（T2d）。
- job `seller.termination_effect`（以 measure_id 唯一，`ScheduledAt = starts_at`）：
  1. transaction：`LockAccountStores` → 重讀措施；REVOKED 或 ENDED → 結束；否則 `Recompute(max(starts_at, now))`（第一次發 `StoreSuspended`）。
  2. transaction 外 `Orders.HasOpenStoreOrders`（fail-closed）；有未結訂單 → 結束這次（`order.completed` 加速、每小時掃描補）。
  3. 關店 transaction：identity 帳號鎖 → `LockAccountStores` → **重讀措施並重新判斷**（ENFORCED、`StateOf == ACTIVE`）、stint 仍是步驟 2 查過的那一個、不是 PENDING_DELETION → `close(closed_by=TERMINATION)`。任一不成立就不關。
- 帳號在等待刪除：job 視為終局、不做事，由刪帳以 `ACCOUNT_DELETION` 關店。
- job 唯一性：River v0.47 的 `ByState` 必須含 `running`（`river@v0.47.0/insert_opts.go:244-283`），執行中被去重的加速排入由每小時掃描補上，延遲 ≤ 1 小時，方向是「晚關」，安全。
- 安全網：每小時掃描「生效中的 TERMINATION 且 stint 未關閉」（precedent `accountdeletion.reconcile`）。
- relay：`OnStoreClosed` 不再轉 `seller.deleted`；新增 `OnStoreRetired` → `seller.deleted`。`StoreReopened` 不轉送（legacy 從未收到 `seller.deleted`，曝光與解除停權已有事件）。
- 清理：`PurgeCandidates`／`PurgeStores` 改用 `RetireDue`（join 目前的關店紀錄取關閉原因與時間、帳號上未撤銷的終止取 `appeal_deadline`、join 待處理申覆取 hold）；清理時釋放 logo／封面並發 `StoreRetired`；`stores_closed_images_check` 改成「已清理的店沒有圖片」；`stores_pii_purged_check` 的「關閉嚴格超過 30 天」改由 `RetireDue` 在程式與查詢中保證（DB 只保留「已清理 ⇒ 已關閉」）。
- 重開：`closing.Reopen`（在撤銷、接受申覆、來源撤銷連帶撤銷的同一個 transaction）：條件由帳號上全部的終止推導（Will 2026-10-16 經 G1「推導、不存第二個事實」）：帳號最近的 stint 由 `TERMINATION` 關閉、造成關店的那一筆（`closed_by_measure_id`，關店當時的事實）在撤銷之後已撤銷（Will 2026-10-16 修訂：預告終止先關店、之後在同一個已關閉的店建立並撤銷 21.3，店仍因預告終止而關閉）、`¬RetireDue`（其 `retire_after` 與 legal hold 看帳號上全部未撤銷的終止：最晚的申覆期限、任何一筆的待處理申覆）、未清理、同一個 transaction 撤銷之後帳號沒有 ACTIVE 的終止（預定生效的預告終止不算）、帳號不在等待刪除；動作是 CLOSED → ACTIVE、結束目前的關店紀錄（`store_closures` 是關店的唯一一份紀錄，目前這一次 `reopened_at IS NULL`；歷史永久保留）、重新授予 SELLER、公告可曝光、`Recompute`、發 `StoreReopened`。帳號已開新店（21.2 允許）時只撤銷，不重開。清理與重開都鎖 `LockAccountStores`，互相串行。

### 範圍影響
- **所有**關店的五個 legacy 下游延後到清理：一般關店晚 30 天，`TERMINATION` 關店晚約 6 個月。期間商品留在 product（已因不可曝光而隱藏）、聊天與通知個資晚遮蔽、device token 晚停用。刪帳另有 `account.deleted`，不受影響。
- 其他 `StoreClosed` 的消費者：單體內只有 relay（`rg StoreClosed`，已核對）。
- 開工前要確認的見「還沒有驗證的」。

### 終局與觸發
本身就是終局形狀。之後的觸發：product、social、im、notification、pricebook 搬進單體時，`seller.deleted` 的對應改成模組事件的訂閱者（遷移步驟的一部分）；賣家自己關店後 30 天內「重開」的產品需求出現時，`Reopen` 開放給 `ACCOUNT_OWNER`。

---

## T2. 模型形狀（不需決定；R3、R4 已核可）

### 2a. 依據與終止事由
- 基準：DSA 17(3)(e)、P2B 4(5)：決定指出條款依據；措施與認定分開（MB D1）；終止不配複審與補正（共通形狀 3）。
- 定案：`basis` 加 `CONTRACT`。`TerminationGround`＝`NOTICE`（21.2）、`MATERIAL_BREACH`、`CEASED_BUSINESS`、`DISSOLUTION`、`BANKRUPTCY`、`LICENSE_REVOKED`、`FOOD_SAFETY_INCIDENT`、`FRAUD`、`PLATFORM_SECURITY`、`OTHER_SERIOUS`、`LINKED_ACCOUNT`（W9）。`NOTICE`、`LINKED_ACCOUNT` ⇒ CONTRACT；`MATERIAL_BREACH` ⇒ VIOLATION；其他 ⇒ 有認定 VIOLATION、沒有 CONTRACT。終止永不為 RISK，所以 `review_by`、補正、End 自動排除（`RemediationUnavailableReasons` 的 `NOT_RISK`，`model/measure.go:456-470`）。
- 排除：basis 為 NULL（認定連結、升級、連帶撤銷要另寫）；食安、詐欺走 RISK（帶來 `review_by`、補正、End，全錯）。
- DB：`measures_kind_check` 加 TERMINATION；`measures_basis_value_check` 加 CONTRACT；`(kind='TERMINATION') = (termination_ground IS NOT NULL)`；`basis <> 'CONTRACT' OR kind = 'TERMINATION'`；`termination_ground NOT IN ('NOTICE','LINKED_ACCOUNT') OR basis = 'CONTRACT'`；`termination_ground <> 'MATERIAL_BREACH' OR basis = 'VIOLATION'`；`(source_measure_id IS NOT NULL) = (termination_ground IS NOT DISTINCT FROM 'LINKED_ACCOUNT')`。終止建立時 `ends_at` NULL，API 不接受終止的 `end_time`；只有系統版本（21.2 關店）寫入。

### 2b. `review_by`
非 RISK ⇒ 沒有，由既有 `(basis='RISK') = (review_by IS NOT NULL)` 保證。

### 2c. 開始時間與申覆期限（R3）
- 基準：SQL:2011 application-time period 每列都有 `valid_from`／`valid_to`，判斷式一個。
- 定案：所有措施加不可變的 `starts_at NOT NULL`；非終止與非 NOTICE 的終止 = `created_at`，21.2 = `created_at + 720 小時`（30 日）。生效：`status='ENFORCED' AND kind IN ('SUSPENSION','TERMINATION') AND starts_at <= @now AND (ends_at IS NULL OR ends_at > @now)`。契約用既有 `MeasurePeriod.start_time`（`seller.proto:1029-1034`），不加 `effective_time`。
- CHECK：`starts_at >= created_at`；`kind = 'TERMINATION' OR starts_at = created_at`；`termination_ground IS DISTINCT FROM 'NOTICE' OR starts_at = created_at + interval '720 hours'`；`termination_ground IS NULL OR termination_ground = 'NOTICE' OR starts_at = created_at`。不用 `'30 days'`：timestamptz 加日數依 session TimeZone 計算，Go 用 `30*24*time.Hour`，兩邊用同一個常數。部分索引 `measures_enforced_suspension`、`measures_active_timed`、`measures_active_open` 改 `kind IN (...)`、`starts_at` 取代 `created_at`。
- 申覆期限：`appeal_deadline = starts_at + 4416 小時`（184 天），只對 TERMINATION，推導、不儲存；契約輸出 OUTPUT_ONLY 的 `Measure.appeal_deadline_time`。184 天是任何連續六個曆月的最大長度，所以不論起算日都不短於條款的「六個月」與 DSA 20(1) 的「at least six months」；用固定時數是為了不依賴時區，與 720 小時同一個理由。DSA 從「decision」起算，本設計從生效起算，21.2 多出預告期，只會更長。

### 2d. 「待關閉」不存
推導（生效中的 TERMINATION ∧ stint 未關閉），存它就是 G1 刪掉的影子狀態。client 看 `suspended=true`＋一筆 ACTIVE 的 TERMINATION。

---

## T3. 操作與狀態推導（不需決定）

| 操作 | SCHEDULED | ACTIVE | 理由 |
|---|---|---|---|
| 申訴 | ✔ | ✔（含已關店、21.2 已結束），**到 `appeal_deadline` 為止** | 預告期就是申訴期；期滿不受理（21.2） |
| 撤銷 | ✔（無待處理申訴） | ✔ | 唯一的解除路徑；`¬RetireDue` 時＝重開 |
| 裁定申訴 | ✔ | ✔（期滿前提出的，期滿後照常裁定） | 不變 |
| 改理由 | ✔（凍結規則照舊，4A-3） | ✔ | — |
| 延長／縮短／End／複審／補正 | ✘ | ✘ | 終止沒有可調的 `ends_at`、`starts_at` 不可變（要延後就撤銷重建）；非 RISK |

實作：`CanUpdateStatement` 的 state 集合加 SCHEDULED；`CanExtend`、`CanShorten` 加 `kind == SUSPENSION`；`update.go checkedEnds` 對終止拒絕 `end_time`。

**申覆的可用性由伺服器推導**（比照 2c-2 的 `remediation_unavailable_reasons`）：新增 `AppealUnavailableReason` 與 `Measure.appeal_unavailable_reasons`（依序、結構性原因在前）：`REVOKED`（措施已撤銷）、`ALREADY_APPEALED`（同一措施已申覆過）、`APPEAL_WINDOW_ENDED`（kind=TERMINATION ∧ now ≥ `appeal_deadline`）、`REQUEST_PENDING`（有待處理請求）。`can_appeal == (appeal_unavailable_reasons 為空)`，`CanAppeal()` 與 `SubmitAppeal` 的前置條件共用同一個函式（`model/measure.go:449-454`），期滿後送出回 FailedPrecondition。client 只呈現原因與 `appeal_deadline_time`。

**`StateOf`（ENDED 先於 SCHEDULED）**：
```
REVOKED → REVOKED
WARNING → ISSUED
kind ∉ {SUSPENSION, TERMINATION} → panic
ends_at != nil && ends_at <= now → ENDED      （21.2 在預告期內因關店結束：ends_at < starts_at）
starts_at > now → SCHEDULED
否則 → ACTIVE
```
SQL 同構：ACTIVE `starts_at <= @now AND (ends_at IS NULL OR ends_at > @now)`；SCHEDULED `starts_at > @now AND (ends_at IS NULL OR ends_at > @now)`；ENDED `ends_at <= @now`；三者都 `kind IN (...)`（`measures.sql:291-383` 的 state 分支）。handler 的 `period` 輸出擴及 TERMINATION（`H/seller/measures.go:150`、`governance_measures.go:85`）。

**第四份寫法刪除**：devseed `lifecycleStatesSQL` 的措施分支（`S/internal/devseed/devseed.go:192-206`）改成呼叫 `EffectiveMeasures`（帳號＝種子帳號 ∪ 種子店家的帳號），寫法剩兩支 SQL＋`StateOf`，由既有兩個資料集測試鎖住。

---

## T4. W1 的落實（不需決定）

- **21.3**：措施永不結束（只能撤銷）；既有 `stint.Derive` 擋同帳號開店（`onboarding.go:94-110`），`measure_identities` 擋其他帳號以同身分證號／統編送 KYC（`kyc/identity.go:49-66`）。不新增檢查。代價：永遠進行中的 `suspension_periods`、HMAC 金鑰永留、`measures_active_open` 與每小時對帳隨 21.3 的累計成長（違反該索引註解「只隨仍有效力的措施成長」的精神，`schema.sql:359-361`）——接受；觸發：21.3 累計數使對帳掃描超過 1 秒或 1 萬筆時，改成依「禁止入駐」另建索引。
- **21.2 在單一關店路徑結束**：規則寫在 `closing.close()`（所有關店者：`ACCOUNT_OWNER`、`TERMINATION`、`ACCOUNT_DELETION`）：同一個 transaction 對帳號上 ENFORCED、`termination_ground='NOTICE'`、尚未結束（SCHEDULED 或 ACTIVE）的措施寫一版 `ends_at = 關店時間`，發 `MeasureEnded`，再 `Recompute`。不寫的話，`stint.Derive` 會永遠擋住 21.2 的重新申請，違反 W1=(c)。
- **鎖順序**：所有關店路徑改成 identity 帳號鎖 → `LockAccountStores`（全部 stint、store_id 順序），取代 `LockStore(current)`（`closing.go:161-189`、`CloseForDeletion`、`BeginDeletion`）。措施寫入只鎖 `LockAccountStores`（`stint.go:35-50`），若關店先鎖目前 stint 再寫措施，與措施寫入的升序鎖會互鎖。關聯措施的連帶撤銷跨帳號時，依 account_id 升序鎖各帳號的 `LockAccountStores`（T5h）。seller `AGENTS.md` 的「鎖順序」一句同步。
- **系統作者**：`measure_revisions.admin_id` 可為 NULL，加 `system_cause`（值 `STORE_CLOSED`），CHECK `(system_cause IS NULL) = (admin_id IS NOT NULL)`，比照 `measure_requests.close_cause`（`schema.sql:416-445`）。
- **身分快照逐措施、在建立時取**：`RecordIdentities` 對「建立即生效且有禁止效果」的措施在建立的 transaction 呼叫：SUSPENSION（既有）與非 NOTICE 的 TERMINATION（含 `LINKED_ACCOUNT`）。21.2 無過錯、W1=(c) 不禁止重新入駐，**不取快照**。不靠 `Recompute`（它只看停權的轉換，帳號已因其他措施停權時不會「發現」新措施，seller `AGENTS.md`:56）。快照保留到措施撤銷（W8）。

---

## T5. 邊界情形

### 5a. 預告期內或生效後賣家自行關店
允許（停權中可關，`TestSuspendedSellerCanCloseStore`）。21.2 由 T4 的單一規則結束；21.3 維持 ACTIVE，`closed_by=ACCOUNT_OWNER`，保留期 30 天（不是 `TERMINATION` 關閉），之後撤銷只撤銷紀錄；有待處理申覆時照 hold 暫停。CHECK 放寬成 `ends_at > created_at`。

### 5b. 店已關閉才發現 21.3 事由
允許對「沒有目前 stint」的帳號建立非 NOTICE 的終止，`store_id` 取最近的 stint，鎖用 `LockAccountStores`；效果只剩禁止入駐。NOTICE 維持只能建在開著的店。清理前 blind index 仍在，快照照取；清理後取不到，是 W8 接受的缺口。

### 5c. 21.3 第 1 款（R5）
`MATERIAL_BREACH` 必須是 VIOLATION，且至少一筆引用的認定**未撤銷**、其 `origin_measure` 建立於 ≤ now − 7 天。錯誤 `invalid_argument` 指 `violation.cited_finding_ids`。runbook：先前措施的 `statement_to_seller` 要寫出改善要求。無法改善或法律要求立即的情形走其他事由。基準：Amazon BSA §3 (a)、蝦皮判決、P2B 4(4)(c)。

### 5d. `closed_by`
新值 `TERMINATION`＋`closed_by_measure_id`（FK）；`CloseStore` 刪除後刪 `ADMIN`、`closed_by_admin_id`、`close_reason`（理由與作者在措施裡，單一來源）。既有 `ADMIN` 列只在本機：比照 W2 (a) migration RAISE 列出 store_id。proto `StoreCloser` 加 `TERMINATION`、刪 `ADMIN`；`Closure` 改 `{closer, measure_id}`。

### 5e. 事件（R2）
不加 `TerminationScheduled`；`MeasureCreated` 加 `StartsAt`。生效的停權由 `Recompute` 發 `StoreSuspended`；關店 `StoreClosed`；21.2 結束 `MeasureEnded`；清理 `StoreRetired`；重開 `StoreReopened`；關聯措施的連帶撤銷照常發 `MeasureRevoked`。觸發：A8 需要「終止生效」的獨立通知時再看。

### 5f. legacy G17
`EffectiveMeasure` 沒有 Kind（`model/measure.go:237-242`），VIOLATION 依據的終止會被分成 `SUSPENDED_GOVERNANCE`。照 G17：兩支 `EffectiveMeasures*` 回 `kind`、`termination_ground`；`Classify` 有 TERMINATION 或 RISK → 9603；reasonLabel 優先用終止事由。

### 5g. 帳號在等待刪除
終止 job 不做事（T1）；刪帳以 `ACCOUNT_DELETION` 關店，21.2 由單一規則結束，21.3 維持（快照繼續擋其他帳號）。

### 5h. 同身分的其他帳號（W9）
- 基準：Google Ads 同身分證件的關聯帳號各自停權、各自申訴，來源申訴成功時自動恢復；eBay、Etsy 把關聯帳號納入處分。
- 偵測：admin 的措施詳情輸出 OUTPUT_ONLY 的 `linked_accounts`：反查本措施的 `measure_identities` 對其他帳號 `kyc_applications` 的 hash，每列 `{account_id, store_id, store_code, matched_identity (NATIONAL_ID｜UBN｜RESPONSIBLE_PERSON_ID), linked_measure_id}`（`linked_measure_id` 是以本措施為來源、仍 ENFORCED 的那一筆）。只對有快照的措施列（SUSPENSION 與非 NOTICE 的 TERMINATION）。
- 模型：關聯措施是 `kind=TERMINATION`、`termination_ground=LINKED_ACCOUNT`、`basis=CONTRACT`、`source_measure_id` 指向來源，立即生效、取身分快照。建立時驗證：來源是 ENFORCED、非 NOTICE、非 `LINKED_ACCOUNT`（不成鏈）的 TERMINATION，且在**另一個**帳號；目標帳號的 KYC hash 確實命中來源的快照（伺服器再算一次，不信任 client）；同一個來源對同一個帳號只能有一筆 ENFORCED。理由由管理員寫（DSA 17）。
- 連帶撤銷：來源措施被撤銷（`RevokeMeasure`、`DecideAppeal(ACCEPTED)`、G3 的 `FINDING_REVOKED`）時，同一個 transaction 撤銷 `source_measure_id = 來源` 且 ENFORCED 的措施，原因 `SOURCE_REVOKED`，待處理請求以 CLOSED 結案，各自走 `Reopen`。關聯措施自己被撤銷或申覆成立，不影響來源。
- 鎖：連帶撤銷依 account_id 升序鎖來源帳號與各關聯帳號的 `LockAccountStores`；建立關聯措施時同樣先鎖來源帳號再鎖目標帳號（升序）。
- 限制帳本：無 REAL 限制。

### 5i. 清理後的身分雜湊（W8）
- 21.3（含 `LINKED_ACCOUNT`）的快照在建立時取（T4），存在 `measure_identities`，不受個資清理影響，保留到措施撤銷；撤銷時刪除（既有）。
- 清理後才發現的 21.3 事由：快照取不到，接受缺口（seller `AGENTS.md`:144 的既有結論）。業界只對被封禁者保留（Uber、蝦皮），不對所有關閉者。
- 目的與保留期要寫進條款 21.3（T6）、賣家身分認證告知事項與隱私政策（直接定案表），落在個資法 §11 III 但書與施行細則 §21 的「契約約定之保存期限」。

---

## T6. 條款第 21 條（W10，定案文字）

基準：P2B 4(2)(3)(4)(5)、11(3)；DSA 17、20；Amazon BSA §3。照 B2 W5 先例改寫內文、不移號。17.4 的「前述措施」只涵蓋第 17 條的措施，終止的申覆規則寫進 21.2，21.3 引用它。

> 2. 乙方沒有未結訂單時，得隨時於平台關閉店家以終止本契約；有未結訂單時，於該等訂單處理完成後終止。甲方得以書面或可保存的電子方式附具理由通知乙方，於通知後三十日終止本契約。乙方對甲方的終止得依第十八條第四項申覆，申覆期間自通知時起至終止生效後六個月止，同一終止以申覆一次為限；預告期間乙方的權利不受影響。依本項終止者，乙方得重新申請開店，並應重新完成身分及資格審核。
>
> 3. 一方重大違約，經通知後七日內未改善；或有停業、解散、破產、撤銷必要許可、重大食安或詐欺事件、危害平台安全及其他足以嚴重影響履約的情形，他方得立即終止。依事件性質無法改善或法律要求立即處理者，無須先定改善期。甲方依本項終止者，應以可保存方式告知事由及主要理由，乙方得依前項申覆；乙方不得以同一帳號，或以終止時甲方仍保有的同一身分證明文件或統一編號，重新申請開店，但終止經撤銷者不在此限。為執行前段限制，甲方於終止撤銷前保存足以辨識該身分證明文件或統一編號的不可逆雜湊值，不作其他用途，終止撤銷時刪除。乙方以同一身分證明文件或統一編號經營的其他店家，甲方得依本項分別終止並分別告知理由，乙方得分別申覆；原終止經撤銷者，該等終止一併撤銷。
>
> 4. 契約終止後，乙方不得成立新訂單；終止前已成立的訂單、退貨、退款、發票、對帳及客服案件仍應完成。甲方終止者，乙方店家自終止生效時起暫停營業，於前述訂單處理完成後關閉。終止於申覆期間（終止生效後六個月）內經撤銷，或乙方於該期間內提出的申覆經處理後撤銷者，甲方應儘速恢復同一店家及其資料；甲方於申覆期間屆滿且無處理中的申覆後，始刪除該店家的資料，其後始撤銷者，甲方應撤銷終止的紀錄及前項的限制。

說明：21.3「仍保有的」對應 W8 的缺口；「不可逆雜湊值……終止撤銷時刪除」是個資法 §11 III 但書與施行細則 §21 要的契約約定保存期限。21.4 的保留期比 21.6「終止後三十日內請求資料」長，兩者一致（T1 的 `greatest(…, closed_at + 30 天)` 保住訂單拖過申覆期間的情形）。版本行改成 commit 當日、不要求重新同意（G20）；與 17.4 改版同日 commit 時 `AGREEMENT_OUTDATED` 分不出兩次，上線前無影響。18.4 不改。條款未經律師審閱。

---

## T7. client 呈現（W11）

可用性與原因由伺服器給，client 只呈現；標籤放 proto enum 註解，三端同字。

- 標籤：`MEASURE_KIND_TERMINATION`「終止合作」；`MEASURE_STATE_SCHEDULED`「預定生效」；`MEASURE_BASIS_CONTRACT`「契約條款」；`TerminationGround`：「預告終止（第 21.2 條）」「重大違約未改善」「停業」「解散」「破產」「必要許可遭撤銷」「重大食安事件」「詐欺事件」「危害平台安全」「其他嚴重影響履約的情形」「關聯帳號」；`AppealUnavailableReason`：「已撤銷」「已申訴過」「申訴期限已過」「有審核中的請求」；`RevokeReason.SOURCE_REVOKED`「關聯的終止已撤銷」。
- admin（`admin/src/app/admin/merchants/[id]/`）：刪「關閉店家」與 `CloseStoreDialog`、`canCloseStore`、`CLOSE_STORE_*`；同位置「終止合作」（`actions.canCreateMeasure`）開 `CreateTerminationDialog`（骨架沿用 `CreateRiskMeasureDialog`：request_id、理由、內部註記）；欄位：事由、給賣家的理由、內部註記；「重大違約未改善」時顯示「依據的認定」複選。說明依事由：預告「建立後 30 天生效（{日期時間}）。期間店家照常營業，賣家可提出申訴。」；21.3「立即生效：店家暫停營業。」；共同「未結訂單完成後店家會關閉。終止生效後 6 個月內撤銷或申訴成立，可恢復同一家店；之後只能撤銷紀錄。」`KIND_LABEL` 改完整 Record；`measureStateBadge` 加 SCHEDULED；`ActiveMeasuresCard` 上方列「預定終止：{日期時間}」（`ListMeasures(account_id, state=SCHEDULED)`）；`closerLabel` 加「終止合作（措施 {id}）」；措施詳情顯示「申訴期限：{日期時間}」（終止）。
- admin 關聯帳號區塊（W9）：措施詳情加「相同身分的其他帳號」，每列：帳號、店號（連到該店家頁）、命中的身分類別（身分證號／統編／負責人身分證號）、狀態（「已建立關聯終止（措施 {id}）」或按鈕「建立關聯終止」）。按鈕開 `CreateTerminationDialog`，事由固定「關聯帳號」、來源措施帶入且不可改，說明「來源措施撤銷時，這筆會一併撤銷。賣家可對這筆單獨申訴。」關聯措施的詳情顯示「來源措施 {id}」連結。沒有命中時不顯示區塊。
- website 賣家：預告橫幅「平台已通知將於 {YYYY-MM-DD HH:mm} 終止合作。在此之前店家照常營業；如有異議，可提出申訴。」（`useSellerMeasures(SCHEDULED)`）；SCHEDULED chip；`measurePeriodText`「終止生效：{start}」，21.2 已結束加「合作已於 {end} 結束」；公告頁標題「合作已終止」，副標「店家已暫停營業，已成立的訂單仍需完成；訂單完成後店家會關閉。終止生效後 6 個月內申訴成立，可恢復同一家店。」；申訴入口一律顯示，不能用時反灰並附原因，終止顯示「申訴期限：{YYYY-MM-DD HH:mm}」；帳號頁在有措施時顯示「處置與申訴」；開店被拒的 21.3 文案「此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，請至「處置與申訴」提出申訴。」；`measureDisplay.ts` 加完整性測試。
- seller-app：首頁預告提示卡；`measure_display.dart` 加終止、SCHEDULED、`LINKED_ACCOUNT` 與申訴不可用原因，`measureNeedsAttention` 加 SCHEDULED；公告頁同 website；開店頁的「處置與申訴」入口。

---

## 依 SSOT 直接定案的實作項

| 項目 | 依據 |
|---|---|
| `MeasureKind.TERMINATION = 3`、`MeasureState.SCHEDULED = 5`、`TerminationGround`（含 `LINKED_ACCOUNT`）、`MeasureBasis.CONTRACT`、`AppealUnavailableReason`、`Measure.appeal_deadline_time`、`Measure.appeal_unavailable_reasons`、`source_measure_id`、`RevokeReason.SOURCE_REVOKED`、admin 的 `linked_accounts`（seller、admin proto） | D §3.1、§5；T2、T3、T5h |
| 申訴不暫停排程；生效前接受申訴＝REVOKED，排程依 state 自然失效 | G15 |
| job `seller.termination_effect` 以 measure_id 唯一、重跑無副作用 | G15、D §6 |
| 刪 `CloseStore`、`CloseByAdmin`、權限 `seller.stores.close`（adminperm、adminauth）；`identity.proto:32` 註解改指「建立終止措施」 | G5、G15 |
| 等未結訂單沿用 `HasOpenStoreOrders`（transaction 外、fail-closed）與「事件加速＋週期掃描」；不沿用 `PENDING_DELETION` | 盤點 §3 |
| 21.4「不得成立新訂單」：**已受保護**——結帳 `requireSellerAcceptingOrders`（`SV/java/order-service/.../CheckoutService.java:1329-1333`）在 `can_accept_orders=false`（含停權）時拒絕；只留一條 e2e 斷言 | 審查 LOW |
| 停權推導的所有消費者自動涵蓋終止 | G1、G16 |
| `maxRESTOperations` 不變（492） | 盤點 §9 |
| 條款 `website/docs/legal/seller-terms.md` 21.2–21.4 照 T6 | W10 |
| 賣家身分認證告知事項 `website/docs/legal/seller-kyc-notice.md` 第 4 項改為：「**利用期間**：自蒐集起至您關閉商店後 30 天；期滿刪除。商店因我們終止合作而關閉者，保存至終止生效後 6 個月（申覆期間），期間內有處理中的申覆者，保存至申覆處理完成；期滿刪除。審核紀錄（審核人、時間、結果）不含上列欄位內容，另行保存。您的帳號受停權處分期間，或因賣家合作條款第 21 條第 3 項終止合作而在終止撤銷前，為防止以同一身分重新開店，我們保存足以辨識同一身分證字號或統一編號的不可逆雜湊值，不作其他用途，處分結束或終止撤銷即刪除。」版本行改 commit 當日、不要求重新同意（G20） | W7、W8；個資法 §8、§11 III 但書、施行細則 §21 |
| 隱私政策 `website/features/legal/pages/PrivacyPage.tsx`「二、資料用途與保存期限」（手寫，不在 `website/docs/legal/`；第四節由 `sellerKycNotice.gen` 嵌入上一列）：「賣家的店家聯絡資料與身分認證資料保存至關閉商店後 30 天。」改為「賣家的店家聯絡資料與身分認證資料保存至關閉商店後 30 天；因我們終止合作而關閉的商店，保存至終止生效後 6 個月，有處理中的申覆時保存至處理完成。因賣家合作條款第 21 條第 3 項終止合作者，為防止以同一身分重新開店，我們在終止撤銷前保存身分證字號或統一編號的不可逆雜湊值。」 | W7、W8 |
| ADR「個資保留」修訂（R6）：保留期依關閉原因、legal hold、`StoreRetired` 與 `seller.deleted` 的時點、關店紀錄歷史表、21.3 快照保留到撤銷；seller `AGENTS.md` 同步（:13 關店、:14 `CloseByAdmin`、:31 關店紀錄、:43 鎖順序、:45 `closed_by`、:48 清理條件、:66 圖片釋放、:140 relay 對應） | R6 |
| 其他文件：gov ADR 修訂（終止、關聯措施）、設計書 32、runbook（事由選用、7 天、清理後無快照、關聯帳號的建立與連帶撤銷、申覆期間）、`CARRYOVER.md` 沿用盤點（關店不可逆、`closed_by`、relay 對應、30 天保留）、計畫 10e 段 | AGENTS「沿用也是決策」 |
| 生成：`make connect`、`pnpm --filter @hawkr/api gen:api`、`../../api/generate.sh buf.gen.yaml`、`pnpm gen:legal` | root `AGENTS.md` |

---

## 2c-3 驗收測試（取代 D §6 的 2c-3 三條）

**狀態與生效**
- 21.2：`SCHEDULED`、`period.start_time = create_time + 720h`、`suspended=false`；到時 ACTIVE、`StoreSuspended` 一次。`starts_at == now`、±1µs 時 `StateOf` 與兩支 SQL 一致。**mutation**：常數改回 336h → `period.start_time` 斷言與 CHECK（`starts_at = created_at + interval '720 hours'`）失敗；SQL `starts_at <= @now` 改 `<`；`StateOf` 拿掉 SCHEDULED 分支。
- 資料集加入：SCHEDULED、ACTIVE、21.3 永久、撤銷、**21.2 於預告期內結束（`ends_at < starts_at`）→ ENDED、不在 SCHEDULED 清單**。**mutation**：`StateOf` 把 SCHEDULED 判斷移到 ENDED 前；SCHEDULED SQL 拿掉 `ends_at` 條件。
- devseed 不再有自己的措施 SQL：`rg "m.kind = 'SUSPENSION'" S/internal/devseed` 無結果；種子檢查對有生效中終止的種子帳號回錯誤。
- 未知 kind → `StateOf` panic。DB：TERMINATION＋RISK、CONTRACT＋SUSPENSION、NOTICE 的 `starts_at ≠ created_at + 720h`、21.3 的 `starts_at ≠ created_at`、`LINKED_ACCOUNT` 沒有 `source_measure_id`、非 `LINKED_ACCOUNT` 有 `source_measure_id` 都被 CHECK 擋。

**操作**（表格驅動 `can_X == 方法成功`）：延長、縮短、End、複審、補正失敗；申訴、撤銷、改理由成功；補正原因首項 `NOT_RISK`。**mutation**：`CanExtend` 拿掉 kind 條件。

**申覆期間**
- `appeal_deadline_time = period.start_time + 4416h`（21.2 從生效起算，不是從建立）；`now = deadline − 1µs` 可申訴，`now = deadline` 時 `can_appeal=false`、原因含 `APPEAL_WINDOW_ENDED`、`SubmitAppeal` 回 FailedPrecondition；SUSPENSION 沒有 deadline、不受影響。原因順序：已撤銷且期滿 → `[REVOKED, APPEAL_WINDOW_ENDED]`。**mutation**：起算改成 `created_at` → 21.2 的邊界測試失敗；`<` 改 `<=` → 邊界失敗；`CanAppeal` 不呼叫原因函式（自己再寫一次）→ 表格驅動 `can_appeal == 原因為空 == SubmitAppeal 成功` 失敗。
- 期滿前提出、期滿後裁定 ACCEPTED → 恢復同一個店（hold 擋住清理）。

**保留期與 legal hold**
- 依關閉原因：`ACCOUNT_OWNER` 關閉滿 30 天＋1µs 清理；`TERMINATION` 關閉滿 30 天但未過 `appeal_deadline` **不清理**，過了才清理；訂單拖到 deadline 之後才關店者，關閉滿 30 天才清理。**mutation**：`RetireDue` 對 `TERMINATION` 也用 30 天 → 第二條失敗；拿掉 `greatest` 的第二項 → 第三條失敗。
- hold：帳號上的 TERMINATION 有待處理申覆時，該帳號**所有**已關閉 stint（含 `ACCOUNT_OWNER` 關閉的舊 stint）過期也不清理、不發 `StoreRetired`；裁定後下一輪清理；關聯措施的來源有待處理申覆時，關聯帳號的 stint 也不清理。**mutation**：拿掉 hold 條件；hold 只看關店的那筆措施（不看帳號）→ 舊 stint 那條失敗；hold 不看來源措施 → 關聯那條失敗。
- `Reopen` 與清理用同一個 `RetireDue`：`now > retire_after`、清理 job 尚未執行時撤銷 → 不重開、只撤銷紀錄。**mutation**：`Reopen` 改成只看「未清理」→ 失敗。

**關店與恢復（T1）**
- 生效、無未結訂單 → 關店，`closed_by=TERMINATION`、`closed_by_measure_id` 正確；有未結訂單 → 不關；查詢失敗不關。
- **競態**：訂單檢查後、關店 transaction 前撤銷措施 → 不關。**mutation**：拿掉 transaction 內的重讀。
- 帳號 PENDING_DELETION → job 不做事；刪帳關店 `ACCOUNT_DELETION`。**mutation**：job 不判斷等待刪除 → 回 FailedPrecondition 重試（測試斷言 job 成功結束）。
- 申覆期間內撤銷／接受申訴 → 同一個 store_id ACTIVE、store_code 不變、商品與 KYC 欄位仍在、SELLER 重新授予、`store_closures` 有一列、可曝光恢復（無其他停權時）、沒有 `seller.deleted`。**mutation**：`Reopen` 不重新授予 SELLER；`Reopen` 的條件改成「關閉 ≤ 30 天」→ 第 31 天到 deadline 之間撤銷的案例失敗。
- 帳號已開新店時撤銷 → REVOKED、不重開。
- relay：`StoreClosed` 不產生 `seller.deleted`；`StoreRetired` 產生。**mutation**：恢復舊對應。
- 申覆期間後撤銷 21.3 → REVOKED、快照刪除、帳號可開新店（新 store_id）。
- job 重複執行、job 遺失（只靠掃描）結果相同。

**關聯帳號（W9）**
- 措施詳情的 `linked_accounts` 列出 KYC hash 命中快照的其他帳號，不列本帳號、不列未命中者。
- 建立關聯措施：來源是 NOTICE、`LINKED_ACCOUNT`、REVOKED 或同帳號 → invalid_argument；目標未命中快照 → invalid_argument；同來源同帳號第二筆 → AlreadyExists。
- 來源 `RevokeMeasure`、`DecideAppeal(ACCEPTED)`、G3 `FINDING_REVOKED` 三條路徑 → 同一個 transaction 內所有 ENFORCED 的關聯措施 REVOKED（`SOURCE_REVOKED`）、待處理請求 CLOSED、申覆期間內的關聯店家重開。**mutation**：連帶撤銷移到 transaction 之後（以事件或 job）→ 斷言「commit 時兩者同時 REVOKED」的測試失敗；只在 `RevokeMeasure` 觸發 → 另兩條失敗。
- 關聯措施自己申覆成立 → 只撤銷它，來源不變。**mutation**：反向連動。
- 鎖：來源與關聯帳號並發撤銷與建立 200 次無 deadlock（壓力測試，不當 mutation 斷言）。

**W1、單一關店路徑、鎖**
- 21.3 關店後：同帳號開店 FailedPrecondition；他帳號同身分證號／統編送 KYC 被擋；措施 ACTIVE。
- 21.2 結束於三種關店：賣家在**預告期**關店、賣家在**生效後**關店、**刪帳**關店 → 措施 ENDED（`system_cause=STORE_CLOSED`、`MeasureEnded`），帳號（未刪者）可開新店。**mutation**：只在 termination job 寫 `ends_at`（不在 `close()`）→ 前兩者失敗。
- 身分快照：21.3 建立在**已因其他措施停權**的帳號上 → 有快照；`LINKED_ACCOUNT` 有快照；個資清理後快照仍在、撤銷後刪除。**mutation**：把 `RecordIdentities` 改成跟著 `Recompute` 的轉換；清理順手刪快照。21.2 從頭到尾沒有快照。
- 鎖：並發的「建立措施」與「關店」在同一個有兩個 stint 的帳號上重複 200 次，無 deadlock 錯誤（壓力測試；改回 `LockStore(current)` 時預期出現 `40P01`，非確定性，不當 mutation 斷言）。

**邊界**
- 沒有目前 stint 的帳號可建 21.3（只禁止入駐）；NOTICE 回 NotFound。
- `MATERIAL_BREACH`：引用的認定不足 7 天或已撤銷 → invalid_argument。**mutation**：拿掉「未撤銷」條件；7 天改 0。
- legacy：VIOLATION 依據的終止 → 9603、reasonLabel 為終止事由。**mutation**：`Classify` 拿掉 kind 判斷。
- 既有 `closed_by='ADMIN'` 列 → migration RAISE。

**契約與 client**：`CloseStore` 不在契約、`buf breaking` 為預期的破壞、`TestEveryServiceInTheContractIsMounted`、adminperm 無 `seller.stores.close`；admin `KIND_LABEL` 完整、終止對話框重送 request_id 不變（**mutation**：每次重產）、關聯帳號區塊只在有命中時出現且來源不可改；website `measureDisplay` 完整性（含 `AppealUnavailableReason`）、橫幅只在有 SCHEDULED 時出現、預告文案的日期等於 `period.start_time`（**mutation**：用 `create_time + 30 天` 自算 → mock 中兩者刻意不同時失敗）、期滿的申訴入口反灰並顯示原因、mock 以 fromJson 驗 wire；seller-app `measureNeedsAttention` 對已申訴的 SCHEDULED 為 true；`legal-generated-diff`（條款與 KYC 告知事項）；compose（backend-e2e-verify）：21.3 建立、關店、申覆期間內撤銷，各抓 `seller.suspended`／`unsuspended` 的 wire payload 與順序，並證明關店到清理之前沒有 `seller.deleted`；清理時有 `seller.deleted`；結帳對終止中的店回 9241。

---

## 還沒有驗證的

- 開工前確認的四項已有結論：social 依賴 10d D2（social 依曝光事件過濾）；在那之前，已關閉店家的貼文在清理前仍公開，到清理時才軟刪。notification 的 device token 依設計保留啟用，賣家關店期間仍要收到措施與申覆通知（停用延到清理）。review 以狀態判斷，不受影響。PayUni 子商店的註冊在關店後保留，`Reopen` 時重新排入綁定。
- 是否有法規要求賣家角色關閉時立即刪除聊天個資（T1 帳本）。
- 21.2 在關店的同一 transaction 結束時，`seller.unsuspended` 的順序對 coupon 的影響（既有「關店後措施到期」路徑已有同樣順序）。
- 2025-11 個資法修正是否改動 §11：未驗證；全國法規資料庫原文、DSA 的 EUR-Lex 原文未讀（用鏡像）。
- Amazon 的停用與 reactivate 流程、申訴窗、資料刪除時點；Etsy 關店後重開與 6 個月申訴期；蝦皮凍結 60 天未申訴即永久停權：只有 SECONDARY。防詐雜湊保留的業界做法、各平台關聯帳號是否人工複核：NOT FOUND。
- A8 通知範本是否只靠 `MeasureCreated.StartsAt` 足夠：通知模組未盤點。
- 條款、KYC 告知事項與隱私政策的文字未經律師審閱。

---

## 審查意見的處理

| 審查意見 | 處理 | 依據／落點 |
|---|---|---|
| HIGH `StateOf` 順序與 T5a 矛盾 | 採納 | T3：ENDED 先於 SCHEDULED；SCHEDULED SQL 含 `ends_at`；資料集與 mutation |
| HIGH 關店 transaction 不重讀措施 | 採納（待處理申訴不擋關店，重讀 REVOKED／ENDED 仍必要） | T1 機制步驟 3；競態測試 |
| MED 21.2 只在兩條路徑結束 | 採納 | T4：規則放在 `close()`，涵蓋三種關店者；測試 |
| MED 鎖順序 | 採納（已核對：措施寫入只鎖 `LockAccountStores`，`closeCurrent` 鎖帳號＋`LockStore(current)`） | T4 鎖順序 |
| MED 不可逆點在單體的 relay，不是 REAL；漏 notification | 採納；另補漏列的 pricebook | T1 重做、帳本 |
| MED「台灣沒有 P2B」不是限制；4(3) 的資料存取 | 採納 | 定位段、外部基準、T1 帳本刪除該列 |
| MED T5b 與 T1 對清理的標記矛盾 | 採納，改為 W8（已定案） | T5i |
| MED 同身分已開店的其他帳號 | 採納，改為 W9（已定案） | T5h；Google MC 6150118、Google Ads 2375414 已查 |
| MED 17.4 不涵蓋終止 | 採納 | T6：申覆規則（含六個月）寫進 21.2，21.3 引用 |
| MED devseed 第四份寫法無測試 | 採納（刪除該寫法，改讀 `EffectiveMeasures`） | T3 |
| MED 缺測試（競態、ACTIVE 21.2＋自行關店／刪帳、PENDING_DELETION、已停權帳號的快照） | 採納；快照規則改成建立時逐措施，21.2 不取 | T4、T5g、驗收 |
| LOW 7 天 SLO 引用錯（那是補正的 SLO） | 採納；保留期改由申覆期間（6 個月）與既有 30 天決定，不需要自訂數字 | T1 |
| LOW `author` 欄與 `close_cause` 先例不一致 | 採納 | T4 `system_cause` |
| LOW job 去重的 lost wakeup | **部分駁回**：建議的修法「`ByState` 排除 running」在 River v0.47 不可行（`insert_opts.go:244-283` 要求含 running）；問題成立，由每小時掃描補，延遲 ≤ 1 小時且方向安全 | T1 機制 |
| LOW 結帳已受保護 | 採納 | 直接定案表 |
| LOW 停權下游不是全可逆 | 採納（未付款單取消不還原、im 斷線） | T1 現有機制 |
| LOW P2B 4(4) 引用不精確 | 採納 | 外部基準 |
| LOW 標籤沿用 W1 | 採納（改 W7，並在 R1 明列推翻 G15 與 D §7） | §0 |
| LOW SSOT 修訂被標為不需決定 | 採納 | §0 R1–R6 |
| LOW `interval '14 days'` | 採納（固定時數；通知期改 30 日後為 720 小時，申覆期限 4416 小時） | T2c |
| LOW 永久 21.3 讓索引與對帳成長 | 採納，接受並設觸發 | T4 |
| LOW `MATERIAL_BREACH` 未要求認定有效 | 採納 | T5c |

## 本文重新核對的程式事實
- `closing.close` 的副作用與 `closeCurrent` 的鎖（`S/internal/closing/closing.go:156-281`）；`CloseForDeletion`、`BeginDeletion` 只 `LockStore`（:58-105）；`CapAccountClose` 需 `RequirementNotPendingDeletion`（`model/capability.go:77`）。
- 措施寫入的鎖：`stint.LockAccountOf` → `LockAccountStores`（`stint.go:35-50`、`measure/measure.go:253-263`、`create.go:197`）。
- `StoreClosed` 在單體內只有 relay 訂閱（`cmd/hawkr/seller_events.go:28-41`）；`seller.deleted` 的消費者：product、social、im、notification、pricebook（`SV/*/internal/worker/seller_deleted.go`）。
- `RecordIdentities` 只在 `create.go:248` 的 SUSPENSION 分支；`Recompute` 持續停權中不發事件（seller `AGENTS.md`:56）。
- 清理保留措施的身分快照、清除 KYC blind index（`purge.go` 套件說明、`db/queries/purge.sql`）；`stores_closed_images_check`、`stores_pii_purged_check`（`schema.sql:61-68`）。
- `measure_requests.close_cause` 的 CHECK（`schema.sql:416-445`）；`measures_active_open` 的註解（:359-363）。
- devseed `lifecycleStatesSQL`（`devseed.go:192-206`）不在資料集測試（`suspensions.sql` 註解只列三種寫法）。
- River `UniqueOpts.ByState` 必含 running（`river@v0.47.0/insert_opts.go:244-283`）；payuni 的 `openStates` 含 running（`payuni/jobs.go:18-23`）。
- 結帳 `requireSellerAcceptingOrders`（`CheckoutService.java:1329-1333`）。
- 條款 17.4「前述措施」、18.4、21.6（`seller-terms.md:173-209`）。
- 申訴的可用性目前只有 `can_appeal`（`seller.proto:1073`、`model/measure.go:449-454`），沒有原因清單；補正的原因清單是 2c-2 的 `RemediationUnavailableReason`（`seller.proto:996-1009`）。
- 賣家身分認證告知事項第 4 項已寫「停權處分期間保存不可逆雜湊值，處分結束即刪除」（`website/docs/legal/seller-kyc-notice.md`）；隱私政策的保存期限段是手寫的（`website/features/legal/pages/PrivacyPage.tsx:99-103`），第四節嵌入告知事項的生成檔。
- 沿用 v1 已核對：`StateOf` 未知 kind 落到 ACTIVE（`model/measure.go:115-126`）、`measures_basis_check`（`schema.sql:332-335`）、`measure_revisions.admin_id NOT NULL`（:384）、`EffectiveMeasure` 無 Kind（`model/measure.go:237-242`）、`MeasurePeriod.start_time`（`seller.proto:1029-1034`）、`onboarding.go:94-110`。
