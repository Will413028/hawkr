# 金流供應商比較：Stripe Connect 與台灣平台型 PSP（2026-10-29，僅研究）

證據標記：[官方] 從官方文件讀到；[repo] hawkr 自己的文件；[未證實] 搜尋沒找到官方頁，或只有第三方資料。

## 0. 先講結論

1. Stripe 不能把台灣當成平台國。stripe.com/global 的可用國家清單沒有台灣 [官方：https://stripe.com/global]。Connect 跨境入金（連結帳戶在別國）只開給美國、英國、EEA、加拿大、瑞士的平台 [官方：https://docs.stripe.com/connect/cross-border-payouts，頁面為日文版]。不在名單內的國家要找業務，沒有自助路徑。
2. 台灣只出現在 Stripe 的 Global Payouts，當作收款國：TWD、電匯、個人與企業都可，必填 email、姓名或公司名、住址（不含郵政信箱）、電話 [官方：https://docs.stripe.com/global-payouts/recipient-requirements]。但送款平台目前只有 US、GB，其餘是 private preview；需要 Treasury；官方說它適合「已經持有搬錢所需執照」的企業 [官方：https://docs.stripe.com/global-payouts]。Stripe 自己的比較表寫明 Connect 付款走 Stripe 的牌照，Global Payouts 要自己扛執照（如美國 MTL）[官方：https://docs.stripe.com/connect/cross-border-payouts-to-recipients/compare-global-payouts]。
3. Stripe 的付款方式清單沒有台灣在地方式（超商、ATM、LINE Pay、街口）。清單裡只有卡片、Apple Pay、Google Pay、Link 等泛用方式；超商只有日本 konbini、墨西哥 OXXO、巴西 Boleto [官方：https://docs.stripe.com/payments/payment-methods/payment-method-support]。
4. 所以「Stripe Connect test mode 當作品集」可行，但只能模擬金流機制：平台、連結帳戶、分帳、退款、webhook。它不能示範 TWD、台灣 KYC、台灣超商與 ATM。這是 port 設計要照顧的落差。
5. 台灣平台型 PSP 中，只有統一金流 PAYUNi 有我讀得到的公開「平台／子商店 API」（hawkr 的 repo 文件）。綠界、藍新、TapPay 我找不到官方的平台、分潤、子商店 API 文件，要打電話問業務才知道。

## 1. 現行 PayUni 用到的能力（repo）

來源：`backend/docs/第三方整合/PayUni 代理商專用 API 總覽.md`、`PayUni 金流官方 API 總覽.md`、`PayUni 費用一覽.md`、`backend/docs/modules/seller.md` §13–§14。

| 能力 | PayUni 做法 |
|---|---|
| 子商店建立 | `bind_merchant`：代理商（AgentID，4 字母前綴）以 API 註冊合作商店；MerID = 前綴 + 數字。PayUni 自動判斷會員是否已存在，沒有就一併建會員 |
| 子商店修改 | `update_merchant` 只能改聯絡方式、地址、支付設定；核心身分欄位（Name、UBN、MerName 等）建立後不可改 |
| KYC 誰做 | 平台自己先做一輪（seller.md §13：身分證號、負責人身分證號、銀行帳號用 AES-256-GCM 存密文，後台審核核准）；之後送 PayUni，PayUni 另有自己的審核，結果以原碼加推導的 outcome 存（NONE、UNDER_REVIEW、PENDING_INFO、APPROVED、REJECTED）。賣家要兩關都過才能收單 |
| 平台要送什麼 | 姓名或公司名（須與登記證明一致）、UBN（統編或身分證字號）、email、企業負責人資料、商店名稱中英文、網址、地址、郵遞區號、MerType、MCC、交付時間、履約保證類型、客單價、支付方式、費率 |
| 撥款帳戶 | `bind_merchant` 沒有銀行帳號欄位；款項撥到會員的 UNi 帳戶（推斷：銀行帳戶由賣家在 PayUni 會員後台自行設定，不經過平台 API；[repo] 文件只說「撥款至會員 UNi 帳戶」，沒寫驗證方式） |
| 分帳與撥款 | `agent_fee` 多功能收付、`grant` 撥款指示（交易建立時帶 `GrantExport=1`，下單後平台發動，PayUni 隔日撥到子商店 UNi 帳戶；僅正向交易；退款依平台政策） |
| 付款方式 | 信用卡（含分期）、虛擬帳號 ATM、超商代碼（7-11 ibon）、icash Pay、LINE Pay、街口、AFTEE 先買後付、Apple Pay、Google Pay |
| 退款 | 信用卡可發動；超商與 ATM 走「轉帳退款」，收退款手續費；部分退款存在（撥款說明有提到） |
| Webhook | 每筆交易帶 `NotifyURL`（背景通知，只限 80、443 port）；`ReturnURL` 只是前景導回，結果以 NotifyURL 為準；加密為 EncryptInfo 加 HashInfo（SHA256），金鑰為 Hash Key 與 IV |
| 沙盒 | `sandbox-api.payuni.com.tw`，代理商功能需簽約並個別開通 |
| 費率（官網費用一覽，含稅） | 國內卡 2.8%、國外卡 3.5%、7-11 ibon 25 元、ATM 1%（最高 15 元）、icash Pay 2.5%、LINE Pay 0.2%（交易處理費）、街口 2.5%、AFTEE 2.5% |
| 執行模式 | 外部呼叫在 transaction 外，用 River job 排隊，以 `store_id` 唯一鍵去重；`AGENT04017`（MerID 已存在）先查詢再認領，所以重送安全 |

## 2. 比較表

| 維度 | Stripe Connect | PayUni 統一金流（現行） | 綠界 ECPay | 藍新 NewebPay | TapPay | 街口 |
|---|---|---|---|---|---|---|
| 台灣可當平台國 | 否 [官方 stripe.com/global] | 是 | 是（一般特店） | 是（一般特店） | 是 | 是 |
| 子商店 API 建立 | 有（Accounts API）但平台國不含台灣 | 有：`bind_merchant`，需簽代理商合約並個別開通 [repo] | 文件只見選填欄位 `PlatformID`（特約合作平台商代號，「專案合作」用），無子商店管理 API [官方 https://developers.ecpay.com.tw/36991/；首頁無平台商區 https://developers.ecpay.com.tw/] | [未證實] 沒找到官方平台商頁 | [未證實] Portal 手冊目錄有 MARKETPLACE、Create Merchant 章節（https://tappaysdk.com/documents/TapPay Portal User Manual.pdf），內容未讀；日本站提到 sub-account 分帳（https://www.tappaysdk.com/japan-en/service/payments）；台灣站是否同等未證實 | [未證實] 找不到官方開放平台文件 |
| KYC/KYB 誰做 | Stripe 做（hosted 或 embedded onboarding，Stripe 收證件、處理 ToS）[官方 https://docs.stripe.com/connect/service-agreement-types] | 平台先收（hawkr 自己存密文），PayUni 再審 [repo] | [未證實]，一般特店是向綠界申請審核 | [未證實] | [未證實]，Portal 有 KYC verified merchant 設定 | [未證實] |
| 平台要收的資料 | recipient 協議：email、姓名或公司名、台灣另需住址與電話 [官方 recipient-requirements]；API onboarding 時平台自己要保證使用者同意 ToS | 見上表，含 UBN 與負責人身分證字號 | — | — | — | — |
| 撥款帳戶驗證 | Stripe 對銀行帳戶做驗證（細節文件 https://docs.stripe.com/connect/payouts-bank-accounts 未讀）；台灣為電匯 | 沒有公開的戶名比對說明 [repo 無] | — | — | — | — |
| 分帳與託管 | 有：destination charge、separate charges and transfers；可延遲撥款；款項走 Stripe 牌照 | 有：撥款指示 `grant`，次日撥到子商店 UNi 帳戶 | 無公開 API | 無公開 API | 日本站有 sub-account，台灣未證實 | 無 |
| 付款方式（台灣） | 卡片、Apple/Google Pay、Link；無超商、ATM、LINE Pay、街口 | 卡、ATM、超商、icash、LINE Pay、街口、AFTEE、Apple/Google Pay | 超商、ATM、卡、TWQR 等（https://developers.ecpay.com.tw/ 有各付款方式頁） | 常見：卡、ATM、超商（第三方資料，未驗證） | 卡、Apple/Google/Samsung Pay、LINE Pay、街口（https://docs.tappaysdk.com/jko-pay/zh/android/front.html） | 街口自家錢包 |
| 退款含部分退款 | 有 | 有（轉帳退款有手續費） | [未證實] | [未證實] | [未證實] | [未證實] |
| Webhook 與簽章 | Webhook 加簽章驗證（Stripe 標準，本次未再抓文件確認） | NotifyURL，SHA256 HashInfo | `ReturnURL` 背景通知，須回 `1|OK`；`CheckMacValue`（SHA256）[官方 36991] | [未證實] | [未證實] | [未證實] |
| 沙盒不需公司 | test mode 免公司即可 | 測試區存在，但代理商 API 需簽約開通 [repo] | 有「開發者測試後台」vendor-stage.ecpay.com.tw，申請門檻未驗證 | [未證實]（第三方稱可自建測試商店） | 需註冊 Portal（https://docs.tappaysdk.com/ ） | [未證實] |
| 文件品質與英文 | 最佳，有英文與日文，但是機器翻譯夾雜 | 中文，結構清楚，無英文 | 中文，完整 | 中文，PDF 為主 | 中英文 | 無公開 |
| 費率 | Connect 費用見 https://stripe.com/connect/pricing（未讀，不引數字） | 見上表 | 未查 | 未查 | 日本站範例 2%，以合約為準 | 未查 |

## 3. 台灣法規（代收轉付）

- 《電子支付機構管理條例》第 3 條第 6 款定義「代理收付實質交易款項」：接受付款方因實質交易所移轉的款項，經一定條件成就、一定期間屆至或付款方指示後，再移轉給收款方的業務 [來源：第三方轉載 https://6laws.net/6law/laws/電子支付機構管理條例.html；我沒能取得全國法規資料庫的頁面，pcode 試錯，需到 law.moj.gov.tw 核對正式文字]。
- 第 5 條：非電子支付機構不得經營該條例第 4 條第 1 項的業務；僅經營代理收付、且所保管款項總餘額未逾主管機關規定金額、也不收儲值與不辦小額匯兌者可例外；超過金額後 6 個月內須申請許可。第 21 條：專營電子支付機構的代理收付款項須全額交付信託或取得銀行十足履約保證 [同上，第三方轉載]。
- 對 hawkr 的意義（推論）：平台自己收款再轉給賣家，容易落入「代理收付」。所以要讓款項走有牌照的第三方（PSP 或銀行信託），平台不持有款項。PayUni 的撥款指示、Stripe Connect 都屬於這種設計；Stripe Global Payouts 則不是。具體金額門檻與是否適用，需律師或主管機關確認，本報告不下結論。

## 4. 支付 port 的最小共同能力集

設計原則：port 只描述「平台做什麼」，不描述「供應商要什麼欄位」。KYC 資料由供應商側的 adapter 決定怎麼收，平台只存供應商回傳的狀態。

必備能力（兩邊都能實現）：

1. `CreateSubMerchant(storeRef, profile) -> providerRef` 且冪等。Stripe 對應 Accounts API；PayUni 對應 `bind_merchant`（`AGENT04017` 認領語意已經是冪等）。
2. `GetSubMerchantStatus(providerRef) -> {state: PENDING|UNDER_REVIEW|NEEDS_INFO|ACTIVE|REJECTED, missingItems[], remedy}`。把 PayUni 的 outcome 與 Stripe 的 `requirements`、`charges_enabled` 都映射到這個小集合。hawkr 已有 `PayUniRemedy`（SELLER_FIX、CONTACT_SUPPORT、RETRY_LATER），可直接泛化。
3. `UpdateSubMerchant(providerRef, changes)`，其中「不可變欄位」由 adapter 回報。PayUni 建立後核心身分欄位不能改；Stripe 通常可改但會重新驗證。
4. `CreatePayment(order, split[], methods[]) -> {redirectOrToken, providerTxnRef}`，split 是「平台抽成加每個賣家金額」。Stripe 用 destination charge 或 transfer；PayUni 在交易建立帶 `GrantExport=1`。
5. `ReleasePayout(providerTxnRef, store)`：PayUni 要明確發動 `grant`；Stripe 可用延遲 transfer。若某供應商是自動撥款，adapter 回傳 no-op，但 port 要保留這個動作，不然沒法表達「確認收貨後才撥款」。
6. `Refund(providerTxnRef, amount, reason) -> {refundRef, async: bool}`，必須支援部分退款，並且回報「退款是否需要走轉帳退款」（PayUni 超商與 ATM 不是原路退回）。
7. `ParseWebhook(rawBody, headers) -> normalized event`，adapter 內做簽章驗證，統一事件：payment_succeeded、payment_failed、refund_updated、payout_updated、merchant_status_changed。要求事件冪等與可重放。PayUni 的 NotifyURL 還要處理「UNKNOWN 稍後補通知」。
8. `Capabilities() -> {methods, supportsSplit, supportsDelayedPayout, kycOwner, currencies}`，讓上層（UI、驗收、測試）依能力降級，而不是寫死供應商名。

port 刻意不放進去的：

- 銀行帳號驗證細節、證件上傳格式：供應商專屬。
- 供應商特有的付款方式參數（例如 PayUni 超商取號的欄位）：用 `methods[]` 加 opaque 設定，不要擴 port。
- 費率：用 adapter 設定。

## 5. 兩邊最大的分歧：KYC 資料誰持有

| | Stripe Connect | PayUni 類 |
|---|---|---|
| 證件與銀行帳號 | 在 Stripe 的 hosted 或 embedded onboarding 收，平台不碰，只看 `requirements` | 現行 hawkr 自己收並加密存（身分證號、銀行帳號），再轉送 PayUni |
| 平台的合規負擔 | 小：不存敏感個資 | 大：個資加密、揭露稽核、金鑰管理、刪帳（repo 已實作這些） |
| 對 port 的影響 | `StartOnboarding` 回傳一個連結，賣家去供應商頁面填 | `SubmitKyc(data)`，資料由平台提供 |

建議：port 同時留兩種 onboarding 形態，由 `Capabilities().kycOwner` 區分（`PROVIDER_HOSTED` 或 `PLATFORM_COLLECTED`）。用 Stripe 時 hawkr 的 KYC 加密儲存模組應該整個不啟用。這也帶出一個設計問題：作品集階段若以 Stripe 為主，現有 seller 模組的 KYC 欄位（身分證號、銀行帳號）在 Stripe 路徑下是多餘的。這一點要在 port 設計前決定，因為它影響 seller 模組的 §13。

## 6. 實際上線的台灣 PSP 建議

建議：維持 PayUni 當第一個台灣 adapter，因為：

1. 它是唯一有公開代理商 API（建商店、修改商店、多功能收付、撥款指示、撥款查詢）的台灣業者，且 hawkr 已經照它實作到會員綁定與狀態同步，不用重做。
2. 撥款指示對應「確認收貨後才撥款」的託管需求，能與前述法規考量（平台不持有款項）相配。
3. 付款方式涵蓋信用卡、ATM、超商、LINE Pay、街口、icash、AFTEE，與台灣買家習慣相符。

風險與條件：

- 這是我讀 repo 文件的結論；它依賴簽代理商合約與開通，我沒有驗證 PayUni 對作品集或個人是否願意簽。
- 綠界與藍新在網路上沒有可查的平台商 API。若 PayUni 不可行，下一步是用電話或 email 問綠界與藍新是否有「平台商／子特店」方案，而不是假設有。
- TapPay 的 Portal 手冊有 MARKETPLACE 章節，值得請業務給 Portal 手冊與台灣 marketplace 說明，但目前不足以評分。
- 街口目前只是付款方式，不是平台方案。

## 7. 未證實或我沒查的項目

- Stripe webhook 簽章、銀行帳戶驗證（戶名比對或小額驗證）、Connect 費率：沒有抓到文件，所以表中不下結論。
- 綠界、藍新、TapPay、街口的退款、webhook、KYC、費率：官方頁沒抓到，全為 [未證實]。
- 台灣法條的第 5 條金額門檻：第三方轉載未列數字；正式文字與門檻需到全國法規資料庫與金管會規定核對。
- 上述 Stripe 頁面為日文版，英文版文意應相同，但沒有逐句比對。
- `docs.payuni.com.tw` 的公開內容抓不到（只拿到標題），PayUni 事實來自 repo 內文件，不是我親自在官網驗證。

## 來源

- Stripe：https://stripe.com/global ；https://docs.stripe.com/connect/cross-border-payouts ；https://docs.stripe.com/connect/cross-border-payouts-to-recipients/compare-global-payouts ；https://docs.stripe.com/connect/service-agreement-types ；https://docs.stripe.com/global-payouts ；https://docs.stripe.com/global-payouts/recipient-requirements ；https://docs.stripe.com/payments/payment-methods/payment-method-support
- 綠界：https://developers.ecpay.com.tw/ ；https://developers.ecpay.com.tw/36991/
- TapPay：https://docs.tappaysdk.com/ ；https://www.tappaysdk.com/japan-en/service/payments ；https://tappaysdk.com/documents/TapPay Portal User Manual.pdf ；https://docs.tappaysdk.com/jko-pay/zh/android/front.html
- 法規（第三方轉載，需核對）：https://6laws.net/6law/laws/電子支付機構管理條例.html
- Repo：`backend/docs/第三方整合/PayUni 代理商專用 API 總覽.md` ；`PayUni 金流官方 API 總覽.md` ；`PayUni 費用一覽.md` ；`backend/docs/modules/seller.md` §13–§14
