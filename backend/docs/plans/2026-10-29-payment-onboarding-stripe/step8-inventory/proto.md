# 步驟 8 契約刪除盤點：`api/hawkr/**/*.proto`

快照：worktree `hawkr-payment-onboarding`，分支 `payment-onboarding`，HEAD `499c5ae8`（`git rev-parse` 實測）。唯讀盤點，沒有跑 buf、build、測試。

縮寫：
- **S** = `api/hawkr/seller/v1alpha1/seller.proto`；**A** = `api/hawkr/admin/seller/v1alpha1/seller.proto`；**RQ** = `api/hawkr/admin/reviewqueue/v1alpha1/review_queue.proto`；**M** = `api/hawkr/admin/seller/v1alpha1/measure.proto`。
- **PLAN** = `backend/docs/plans/2026-10-29-payment-onboarding-stripe.md`；L255 是步驟 8 的「範圍」那一行。
- 消費端計數格式 `行數/檔數`：b=`backend/internal`、c=`backend/cmd`、a=`admin/src`、w=`website`、s=`seller-app/lib`、y=`buyer-app/lib`、p=`packages`（手寫部分）。沒列的區域是 0/0。
- 計數是**名稱比對**（實測，指令見第 4 節），不是型別解析：後端的數字包含模組自己的 model、SQL、文案目錄 JSON，不只是 proto 符號的引用；所以是「要動的量級」，不是精確引用數。
- 處置欄的「刪（推論）」表示計畫沒有點名這個符號，是從被點名的刪除推出來的，需要確認。

## 1. 符號表

### 1a. PayUni（D6、D14、PLAN L255）

| file:line | 符號 | 類別 | 建議處置 | 依據 | 消費端（比對式） |
|---|---|---|---|---|---|
| A:91-95 | rpc `GetPayUniRegistration` | PAYUNI | 刪 | PLAN L255「admin 的 4 支 PayUni RPC」、D6 (L269-270) | b15/5 a7/4（`GetPayUniRegistration\|getPayUniRegistration`）；最多：`backend/internal/httpapi/seller/admin_kyc_integration_test.go`、`admin/src/app/admin/merchants/[id]/use-store-detail.ts` |
| A:97-101 | rpc `RebindPayUni` | D14-rebind | 刪 | D14 (L293)「刪除 `RebindPayUni`」 | b80/19 a2/2；最多：`backend/internal/modules/seller/payuni_integration_test.go`、`.../internal/model/store_action.go` |
| A:103-106 | rpc `ListPayUniObservations` | PAYUNI | 刪 | PLAN L255 | b19/5；最多：`backend/internal/httpapi/seller/admin_kyc_test.go` |
| A:108-110 | rpc `RefreshPayUniStatus` | PAYUNI | 刪 | PLAN L255；D14「保留重新整理狀態」已由 `api/hawkr/admin/payment/v1alpha1/payment.proto` 的 RefreshPaymentAccount 承接（推論，依該檔 L15-26 的註解） | b21/8 a1/1 |
| A:771-777 | `GetPayUniRegistrationRequest`／`Response` | PAYUNI | 刪 | 隨 RPC | 併入 `PayUniRegistration` 的計數 |
| A:779-785 | `RebindPayUniRequest`／`Response` | D14-rebind | 刪 | 隨 RPC | 併入 `RebindPayUni` |
| A:799-809 | `ListPayUniObservationsRequest`／`Response` | PAYUNI | 刪 | 隨 RPC | 併入 |
| A:811-815 | `RefreshPayUniStatusRequest`／`Response` | PAYUNI | 刪 | 隨 RPC | 併入 |
| A:735-769 | message `PayUniRegistration`（現行欄位 18 個：1-8、10-12、15、16、22-26） | PAYUNI | 刪 | PLAN L255 | b183/32 a18/4（`PayUniRegistration`）；最多：`.../seller/internal/payuni/intent.go`、`.../store/payuni.sql.go` |
| A:723-733 | message `PayUniStatusCodes`（7 欄） | PAYUNI | 刪 | 只被 `PayUniRegistration`、`PayUniObservation` 用（A:750、793） | b2/1 |
| A:787-797 | message `PayUniObservation`（6 欄） | PAYUNI | 刪 | 隨 RPC | b100/20（`PayUniObservation`，含 Source） |
| A:358-365 | enum `PayUniObservationSource`（5 值） | PAYUNI | 刪 | 只被 A:790 用 | 併入上一列 |
| A:264-265 | `ListStoresRequest.payuni_bind_status = 7` | PAYUNI | 刪 | 型別 `PayUniBindStatus` 被刪 | 併入 `PayUniBindStatus`：b51/17 a10/4 |
| S:765-773 | enum `PayUniBindStatus`（4 值） | PAYUNI | 刪 | PLAN L255（`payuni_*` 表 drop） | b51/17 a10/4（`PayUniBindStatus\|PAY_UNI_BIND_STATUS\|payuni_bind_status\|payuniBindStatus\|PayuniBindStatus`） |
| S:798-812 | message `PayUniProblem`（欄位 1、3、4、5；2 `remedy` 已 reserved） | PAYUNI | 刪 | 只被 A:756 用 | b54/11（其中 `backend/internal/platform/uitext/catalog/zh-TW.json` 25 行） |
| S:814-833 | enum `PayUniState`（9 值） | PAYUNI | 刪 | D15 (L294) 拆成兩條軸 | b148/17 a4/2；最多：`.../model/payuni_state.go` |
| S:835-846 | message `SellerPayUni`（5 欄） | PAYUNI | 刪 | 同上 | b87/18 |
| S:887 | `KYCFlow.payuni = 5` | PAYUNI | 刪 | 型別被刪 | 併入 `SellerPayUni` |
| S:856-862 | `KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_*`（3–9，7 值） | PAYUNI | 刪 | 註解 S:855「對應 PayUniState」 | b3/2（`RESUBMIT_UNAVAILABLE_REASON_PAYUNI\|KYCResubmitUnavailableReason\.PAYUNI`） |
| S:853-854 | `KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW = 2`（「PayUni 已收下店家，不能取消」） | PAYUNI | UNDECIDED | 計畫沒點名；語意綁 PayUni（後端 `.../model/kyc_resubmit.go:18-19`） | 整個 enum／欄位：b67/11 w3/1 |
| RQ:44-45 | `REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY = 12` | D14-rebind | 刪 | D14 (L293)、PLAN L255 | b21/10 a1/1（`PAYUNI_BIND_RETRY\|PayuniBindRetry\|PayUniBindRetry`）；含 migration `20261021105752_seller_payuni_bind_retry_review.sql`、`db/objects.sql` |
| A:132-137 | `STORE_ACTION_UNAVAILABLE_REASON_PAYUNI_MERCHANT_TERMINAL = 3`、`_PAYUNI_BIND_IN_PROGRESS = 4`、`_PAYUNI_BOUND = 5` | D14-rebind | 刪（推論） | 只描述 RebindPayUni 的前置條件（A:97-100） | b15/7 a3/1（`admin/src/lib/unavailability.ts`） |
| A:138-139 | `STORE_ACTION_UNAVAILABLE_REASON_KYC_NOT_APPROVED = 7` | D14-rebind | 刪（推論） | 註解寫「重新綁定 PayUni 時」；後端只在 REBIND_PAYUNI 用（`.../model/store_action.go:77,144`） | a1/1；後端用自己的常數 `StoreReasonKYCNotApproved`（同檔 L53） |
| A:354-355 | `KYC_REVIEW_ACTION_REBIND = 5` | D14-rebind | 刪（推論） | D14；既有審核紀錄列是否存在屬 DB 盤點 | b1/1 a1/1 |
| S:214-215 | `AGREEMENT_KIND_PAYUNI_TERMS = 3` | D11-terms | 刪 | D11 修訂 (L285)「只刪 `PAYUNI_TERMS`」、PLAN L255 | b22/10 w11/5（`PAYUNI_TERMS\|AgreementKind\.PAYUNI`）；最多：`backend/internal/modules/seller/agreement_integration_test.go`、`website/lib/seller/agreements.unit.test.ts` |
| PayUniRemedy | **proto 裡不存在** | — | 無 proto 動作 | `git grep -n PayUniRemedy -- api/hawkr` 0 筆；S:801-802 已是 `reserved 2; reserved "remedy";`。只剩後端 b13/3 | — |

只改註解的 PayUni 文字（符號留著）：
- A:3（檔頭）、A:71（ApproveKYC「寫入 PayUni 的綁定意圖」）、A:121、A:124、A:155、A:565。
- S:4、S:109、S:112、S:206-207、S:265、S:285、S:312、S:337、S:342、S:344、S:346-347、S:655、S:671-672、S:744、S:874、S:877、S:963、S:1067。
- `api/hawkr/common/v1alpha1/store_card.proto:11`、`:14`；`api/hawkr/discovery/v1alpha1/discovery.proto:15`。
- 其中 S:342-347「店家有 PayUni 註冊之後不能清掉」與 S:285「物流建店的前置條件含 PayUni 審核」是**行為的描述**，不是單純改字，見第 3 節。

### 1b. D15 流程狀態與 D17 鎖

| file:line | 符號 | 類別 | 建議處置 | 依據 | 消費端 |
|---|---|---|---|---|---|
| S:752-759、S:762 | `KYC_FLOW_STATUS_BINDING = 5`、`BIND_FAILED = 6`、`FORWARDED = 7`、`UNDER_REVIEW = 8`、`PENDING_INFO = 9`、`PAYUNI_REJECTED = 11` | D15-flow | 刪 | PLAN L255、D15 (L294) | b59/5 a12/1 s6/1（`KYC_FLOW_STATUS_(BINDING\|…)\|KYCFlowStatus\.(BINDING\|…)`）；`uitext/catalog/zh-TW.json` 54 行、`admin/src/app/admin/merchants/store-row.ts`、`seller-app/lib/features/seller/data/kyc_flow.dart` |
| S:760-761 | `KYC_FLOW_STATUS_APPROVED = 10`（註解「PayUni 核准，可以收款」） | D15-flow | 留、改註解；或隨整個 enum 處置（UNDECIDED，見 2-5） | D15 的平台審核軸含 APPROVED | b11/1 a1/1 s1/1 p1/1 |
| S:745-763 | enum `KYCFlowStatus` 整體 | D15-flow | UNDECIDED | 刪完剩 NOT_STARTED、DRAFT、SUBMITTED、REJECTED、APPROVED | b219/22 c1/1 a29/1 w5/2 s23/4 p11/2（`KYCFlowStatus\|KYC_FLOW_STATUS\|kyc_flow_status\|kycFlowStatus\|KycFlowStatus`） |
| S:881 | `KYCFlow.identity_locked = 3` | D17-lock | 刪 | D17 (L296)、PLAN L255「`CheckNameUnlocked`／`IdentityLocked`（D17）」 | b32/14 w16/8 s16/8（`-i identity_?locked`）；`seller-app/lib/features/seller/data/current_kyc.dart` 7 行 |
| S:878、S:884 | `KYCFlow.can_resubmit = 2`、`resubmit_unavailabilities = 4` | D15-flow | UNDECIDED | 見 2-5 | `-i can_?resubmit`：b55/16 w6/3 s2/1 |
| A:231 | `StoreListItem.kyc_flow_status = 3` | D15-flow | UNDECIDED（隨 enum） | — | 併入 `KYCFlowStatus` |
| A:243 | `KYCSummary.flow_status = 3` | D15-flow | UNDECIDED（隨 enum） | — | 併入 |
| A:566 | admin `KYCApplication.flow = 23` | D15-flow | UNDECIDED（隨 `KYCFlow`） | — | `KYCFlow([^S]\|$)`：b73/23 w2/1 |
| A:768 | `PayUniRegistration.flow_status = 26` | PAYUNI | 刪（隨 message） | — | — |

### 1c. D22 個資管線

| file:line | 符號 | 類別 | 建議處置 | 依據 | 消費端 |
|---|---|---|---|---|---|
| S:893 | `IndividualKYC.id_card_no = 1` | D22-身分證號 | 刪 | PLAN L255、D22 (L301) | `-i id_?card_?no`（含負責人與 masked）：b258/55 c5/2 a10/2 w1/1 p10/4；最多：`.../seller/internal/store/kyc.sql.go`、`.../seller/db/schema.sql` |
| S:894 | `IndividualKYC.id_card_no_masked = 2` | D22-遮蔽值 | 刪 | 同上 | `-i (id_?card_?no\|bank_?account)_?masked`：b36/11 a3/1 |
| S:908 | `BusinessKYC.rep_id_card_no = 8` | D22-負責人身分證號 | 刪 | 同上 | `-i rep_?id_?card_?no`：b115/41 a5/2 w1/1 p4/2 |
| S:909 | `BusinessKYC.rep_id_card_no_masked = 9` | D22-遮蔽值 | 刪 | 同上 | 併入 |
| S:944 | `KYCApplication.bank_code = 11` | D22-銀行 | 刪 | PLAN L255「銀行帳號」；7a 已從必填拿掉（PLAN L139） | `-i bank_?code`：b108/34 c2/1 a1/1 w14/4 y50/7（website 與 buyer-app 的命中是訂單付款的銀行代碼，D 類，不屬本步驟） |
| S:945 | `KYCApplication.bank_account = 12` | D22-銀行 | 刪 | 同上 | `-i bank_?account`：b296/65 c4/2 a5/3 w1/1 s3/2 |
| S:946 | `KYCApplication.bank_account_masked = 13` | D22-遮蔽值 | 刪 | 同上 | 併入 masked |
| S:942 | `KYCApplication.bank_name = 10` | D22-銀行 | 刪（推論）→ 列入第 3 節 | 計畫只寫「銀行帳號」；7a 必填清單有刪它（PLAN L139）；畫面稿「刪掉：銀行帳戶」（`step7-screen-drafts.md:18`） | `-i bank_?name`：b78/32 c2/1 a1/1 w5/2（website 是訂單） |
| S:949-950 | `KYCApplication.id_card_front = 19`、`id_card_back = 20` | D22-證件 | 刪 | PLAN L255「證件」 | `-i id_?card_?(front\|back)`：b143/28 c1/1 a8/2 w3/2 |
| S:951 | `KYCApplication.bank_book = 21` | D22-證件 | 刪 | 同上 | `-i bank_?book`：b94/25 a8/2 w3/2 p2/2 |
| S:681-683 | `KYC_FIELD_BANK_NAME = 7`、`BANK_CODE = 8`、`BANK_ACCOUNT = 9` | D22-KYCField | 刪（BANK_NAME 是推論） | 與欄位路徑一對一（S:670） | 下列 8 個值合計：b36/4 a15/2 w6/2（`KYC_FIELD_(BANK_\|ID_CARD_\|INDIVIDUAL_ID_CARD_NO\|BUSINESS_REP_ID_CARD_NO)\|KYCField\.(…)`；含 ID_CARD_ISSUE_*） |
| S:691-694 | `KYC_FIELD_ID_CARD_FRONT = 15`、`ID_CARD_BACK = 16`、`BANK_BOOK = 17`、`INDIVIDUAL_ID_CARD_NO = 18` | D22-KYCField | 刪 | 同上 | 同上；`admin/src/app/admin/merchants/[id]/KycApplicationCards.tsx` 10 行 |
| S:705 | `KYC_FIELD_BUSINESS_REP_ID_CARD_NO = 26` | D22-KYCField | 刪 | 同上 | 同上 |
| S:706 | `KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS = 27` | keep | 留 | D24 Q2 (L303) | b4/3 a6/2 w7/2 s4/2（`COMPANY_DOCUMENTS`） |
| S:911 | `BusinessKYC.company_documents = 10` | keep | 留 | D24 Q2 | `-i company_?documents`：b86/20 a6/2 w53/4 s42/5 p1/1 |
| S:904、S:699 | `BusinessKYC.tax_id = 3`、`KYC_FIELD_BUSINESS_TAX_ID = 21` | keep | 留 | D22「統編…仍由平台收」(L301) | 未計 |
| S:936-941 | `real_name = 4`、`birth_date = 5`、`id_card_issue_date = 6`、`id_card_issue_type = 7`、`id_card_issue_location = 8`、`english_name = 9` | D22-身分（未點名） | UNDECIDED | 見第 3 節 #2 | `-i real_?name` b242/54 c3/2 a1/1 w3/2；`-i id_?card_?issue` b122/23 c2/1 a3/1；`-i english_?name` b102/38 c2/1 a1/1 w2/1；`-i birth_?date` b157/45（含 buyer 模組）a1/1 |
| S:675-680 | `KYC_FIELD_REAL_NAME = 1` … `ENGLISH_NAME = 6` | D22-身分（未點名） | UNDECIDED | 隨上一列 | 併入 `KYCField`：b154/26 a51/3 w76/4 s64/4 |
| S:777-784 | seller `KYCDocument`（asset_id、download_uri、staged_token） | keep | 留、改註解（「證件含個資」） | 公司文件仍用（S:911） | `KYCDocument\|KycDocument`：b292/33 a9/3 w2/1 s7/2 |
| S:734-738 | `KYCIssue.document_asset_id = 2` | keep | 留、改註解（列了 id_card_front 等） | 公司文件仍用 | `-i document_?asset_?id`：b66/16 a12/2 w6/3 s2/1 |
| S:716-721 | `KYC_ISSUE_CODE_UNREADABLE = 1`、`EXPIRED = 3` | D22-證件（未點名） | UNDECIDED | 見第 3 節 #7 | b4/2 a6/1 w2/1 |
| A:414 | admin `IndividualKYC.id_card_no_masked = 1` | D22-遮蔽值 | 刪 | PLAN L255 | 併入 masked |
| A:426 | admin `BusinessKYC.rep_id_card_no_masked = 8` | D22-遮蔽值 | 刪 | 同上 | 併入 |
| A:556-557 | admin `KYCApplication.bank_code = 13`、`bank_account_masked = 14` | D22-銀行 | 刪 | 同上 | 併入 |
| A:555 | admin `KYCApplication.bank_name = 12` | D22-銀行 | 刪（推論） | 同 S:942 | 併入 |
| A:549-554 | admin `KYCApplication.real_name = 6` … `english_name = 11` | D22-身分（未點名） | UNDECIDED | 第 3 節 #2；後台卡片還在顯示（`admin/src/app/admin/merchants/[id]/KycApplicationCards.tsx:396-416`） | 同上 |
| A:614 | `KYCApplicationSummary.real_name = 6`（審核佇列；A:632 的 query 也比對姓名） | D22-身分（未點名） | UNDECIDED | 同上 | 同上 |
| A:571 | `KYCApplication.shares_bank_account_with_sanctioned_account = 26` | D22-銀行（未點名） | UNDECIDED | 第 3 節 #6 | `-i shares_?bank_?account`：b12/5，client 0 |
| A:85-89 | rpc `RevealKYCSensitive` | D22-全文 | UNDECIDED | 第 3 節 #1 | `-i RevealKYCSensitive`：b32/9 c1/1 a7/3 |
| A:715-717 | `RevealKYCSensitiveResponse.id_card_no = 1`、`rep_id_card_no = 2`、`bank_account = 3` | D22-全文 | 刪 | PLAN L255（三個值都不再存） | 併入 |
| A:719-720 | `RevealKYCSensitiveResponse.expire_time = 4`、`documents = 5` | D22-證件授權 | UNDECIDED | 第 3 節 #1 | 併入 |
| A:707-711 | `RevealKYCSensitiveRequest` | D22-全文 | UNDECIDED | 隨 RPC | 併入 |
| A:60-63、A:649-659、A:528-535 | rpc `ListKYCReveals`、Request／Response、message `KYCReveal` | D22-稽核 | UNDECIDED | 第 3 節 #1 | `-i ListKYCReveals` b32/11，client 0；`KYCReveal\|KycReveal\|kyc_reveal` b110/18 a20/4 |
| A:65-69、A:661-671、A:398-410 | rpc `ListKYCDocumentAccesses`、Request／Response、message `KYCDocumentAccess` | D22-稽核 | UNDECIDED | 第 3 節 #1 | `-i ListKYCDocumentAccesses` b29/11，client 0；`KYCDocumentAccess…` b104/17 |
| A:382-395 | message `KYCDocumentView` | D22-證件授權 | UNDECIDED | 第 3 節 #1 | `KYCDocumentView\|view_uri\|viewUri\|ViewUri`：b15/7 a16/5 |
| A:369-378、A:564 | admin `KYCDocument`、`KYCApplication.documents = 22` | keep | 留、改註解（A:374 列了三個要刪的欄位） | 公司文件仍在 | 併入 `KYCDocument` |
| A:591 | `KYCApplication.issuable_fields = 34` | keep | 留（內容由後端規則縮小） | — | `-i issuable_?fields`：b18/7 a3/1 |
| M:218-219、M:222-223 | `LINKED_IDENTITY_NATIONAL_ID = 1`、`LINKED_IDENTITY_RESPONSIBLE_PERSON_ID = 3` | D22-雜湊 | UNDECIDED | 第 3 節 #3 | `LINKED_IDENTITY_(NATIONAL_ID\|RESPONSIBLE_PERSON_ID\|BANK_ACCOUNT)\|LinkedIdentity\.(…)`：a3/1（`admin/src/app/admin/merchants/[id]/measure.ts`）；後端用字串常數（`.../model/measure.go:486-493`） |
| M:224-225 | `LINKED_IDENTITY_BANK_ACCOUNT = 4` | keep | 留、改註解（「連同銀行代碼」） | fingerprint 進快照時就是 BANK_ACCOUNT（`backend/internal/modules/seller/db/queries/kyc.sql:245-248`） | 同上 |
| M:220-221 | `LINKED_IDENTITY_UBN = 2` | keep | 留 | D22 統編保留 | — |
| A:339-342 | enum `KYCSellerType` 的註解（「身分證號是賣家本人的」） | keep | 改註解 | — | b35/16 |
| S:213 | `AGREEMENT_KIND_KYC_PRIVACY_NOTICE = 2` | keep | 留 | 畫面稿留下「`KYC_PRIVACY_NOTICE` 同意勾選」（`step7-screen-drafts.md:26`） | b17/10 w19/9 s2/1 |
| S:313-322、S:965 | enum `StoreField`、`KYCApplication.missing_store_fields = 31` | keep | 留、改註解（寫「PayUni 綁定需要的店家資料」） | 7a「不能動…店家欄位（`MissingStoreFields`）的必填規則」(PLAN L154)；L255「CARRYOVER 聯絡欄位長度…改成平台自己的上限」 | `-i missing_?store_?fields` b36/11 w10/4 s6/2 |

需要整段改寫的註解（D22）：A:53-55（GetKYCApplication）、A:85-88、A:367-368、A:380-381、A:537-538、A:713；S:103、S:110-111、S:730、S:734-737、S:775-776、S:890、S:897、S:916-924。

## 2. 刪除後變空或失去意義的 message

1. **S:891-895 `IndividualKYC`（賣家端）**：兩個欄位都刪 → 空 message。它同時是 `oneof subject` 的一支（S:952-955），賣家型態由「設定哪一支」表示（S:916-920）。UNDECIDED：留空 message 當型態標記，或把 oneof 換成型態 enum。計畫沒寫。
2. **A:413-415 admin `IndividualKYC`**：唯一欄位刪掉 → 空，同上（A:560-563）。
3. **A:714-721 `RevealKYCSensitiveResponse`**：刪 1–3 後只剩 `expire_time` 與 `documents`，而 `documents` 只會有公司文件。RPC 名稱與註解（A:85「顯示…身分證號、負責人身分證號與銀行帳號全文」）都不再成立。UNDECIDED。
4. **A:528-535 `KYCReveal`＋`ListKYCReveals`、A:398-410 `KYCDocumentAccess`＋`ListKYCDocumentAccesses`、A:382-395 `KYCDocumentView`**：稽核的對象只剩公司文件。兩支 List RPC 目前 client 沒有呼叫（admin/src 0 筆，實測）。UNDECIDED。
5. **S:875-888 `KYCFlow`**：刪 `identity_locked`、`payuni` 後剩 `status`、`can_resubmit`、`resubmit_unavailabilities`；`KYCFlowStatus` 刪完剩 5 值，與 `KYCStatus`（S:656-668）只差 NOT_STARTED 與 CANCELLED。D26 (L305) 已讓 client 不再由 `GetMyKYC` 推導狀態。UNDECIDED：留精簡後的 `KYCFlow`、併回 `KYCStatus`，或只留 `can_resubmit`。
6. **S:849-863 `KYCResubmitUnavailableReason`**：刪 7 個 PayUni 值後剩 UNSPECIFIED、UNDER_REVIEW_CANCELABLE，加上語意綁 PayUni 的 UNDER_REVIEW（S:853-854）。`KYCResubmitUnavailability`（S:867-872）跟著只剩一兩種原因。UNDECIDED。
7. **A:126-149 `StoreActionUnavailableReason`**：刪 3、4、5、7 後剩 KYC_NOT_SUBMITTED、KYC_NOT_BUSINESS、STORE_CLOSED、STORE_PENDING_DELETION、ACCOUNT_SUSPENDED；仍被 approve／reject／requery／delegation／create_measure 用（A:236、333、596-604），不會變空。註解 A:124 的「沒有入口」清單要改。
8. **A:346-356 `KYCReviewAction`**：刪 REBIND 後剩 4 值，不變空。
9. **A:571 `shares_bank_account_with_sanctioned_account`**：申請上不再有銀行帳號；fingerprint 要到核准後建立收款帳戶才有（D16，L295）。審核當下這個旗標能不能為真是後端的事（推論：首次申請時恆為 false）。UNDECIDED。
10. **S:716-721 `KYCIssueCode` 的 UNREADABLE、EXPIRED**：註解寫的是圖片與證件的問題；公司文件仍是文件類，所以還用得到（推論）。UNDECIDED，傾向留。

## 3. 計畫沒有決定的事（不在這裡決定）

1. **Reveal 與兩支稽核 RPC 的去留**（`RevealKYCSensitive`、`ListKYCReveals`、`ListKYCDocumentAccesses`、`KYCDocumentView`、`KYCReveal`、`KYCDocumentAccess`）。證據：PLAN L255 只寫「刪身分證號、銀行帳號、證件的 proto 欄位…密文欄位與證件檔參照」，沒有提 Reveal；D24 Q2 (L303) 說公司補充文件是「市集審核用的公司證明，不是 D22 的證件」。選項：(a) 全刪，公司文件改成一般的後台讀取；(b) 留授權與稽核機制，只服務公司文件，RPC 改名、回應拿掉三個全文欄位；(c) 留現名。影響：`KYCIssue.document_asset_id` 的註解說 asset_id 只在顯示全文的回應裡帶（A:392-393、A:694-695），選 (a) 時退回流程要另給 asset_id。統編本來就不遮蔽（A:423、A:617），不需要 Reveal。
2. **`real_name`、`birth_date`、`id_card_issue_date/type/location`、`english_name`、`bank_name`**（S:936-942、A:549-555、A:614，及對應的 `KYC_FIELD_*` 1–7）。步驟 8 的用字是「身分證號、銀行帳號、證件」(L255)，D22 是「不再收身分證號、銀行帳號與證件」(L301)，都沒有點名這些。旁證：7a 從必填拿掉 `real_name`、`english_name`、`bank_name`（L139）；畫面稿「刪掉的內容（D22）：身分證明、銀行帳戶、身分證正反面、存摺、英文名稱」，留下的清單沒有姓名與生日（`step7-screen-drafts.md:18-26`）；D9 修訂「平台不收身分與銀行資料」(L279)。seller-app 與 website 的表單已不送（`-i real_?name` s0、w 只剩測試，實測）；後台卡片還在顯示（`KycApplicationCards.tsx:396-416`），審核佇列用 `real_name` 當列的名稱與搜尋（A:607、A:614、A:632）。選項：(a) 全刪，個人型申請在佇列改顯示店名；(b) 留 `real_name` 當聯絡／負責人姓名，刪其餘；(c) 全留。
3. **`LINKED_IDENTITY_NATIONAL_ID`、`RESPONSIBLE_PERSON_ID`**（M:218-223）。L255 說刪 `kyc_identity_hashes` 的三個種類，但「`account_identity_hashes` 已有的列保留到 5 年清理」；現行 CHECK 仍含 `ID_CARD_NO`、`REP_ID_CARD_NO`（`backend/internal/modules/seller/db/schema.sql:1071`）。舊列若還會命中，enum 值就還有輸出。選項：(a) 留值（舊列仍會命中）；(b) 刪值並讓後端不再回這兩類命中；(c) 未上線、只有本機種子，連舊列一起清（與 L255「保留」的文字衝突，要 Will 決定）。
4. **欄位與 enum 編號的處理**。規範的相關行只有一句：「版本：上線前一律 `v1alpha1`（AIP-185），`buf breaking` 不檢查，契約可以直接改。上線時一次改名成 `v1`，之後 `WIRE_JSON` 的 breaking 才生效（`api/buf.yaml`）。」（`backend/docs/開發規範/API 設計規範（Connect）.md:17`）；`api/buf.yaml:24-26` 同義（`ignore_unstable_packages: true`）。規範裡沒有 reserve 或重編號的規則（`git grep -n -i -E 'reserved|編號'` 該檔 0 筆，實測）。現行做法是刪掉就留 `reserved N; reserved "name";`：S 檔 39 行、A 檔 38 行 `reserved`，本分支 7a′ 刪 `payout_contact_email` 也是這樣（S:928-930、S:708-710）。選項：(a) 照現行做法 reserve；(b) 直接刪、不 reserve；(c) 整個 message 重編號。根 `AGENTS.md`「上線前不受相容性約束」只說相容性不是約束，沒說選哪個。
5. **`KYCFlow`／`KYCFlowStatus` 留不留**（第 2 節 5、6），連同 admin 的 `StoreListItem.kyc_flow_status`（A:231）、`KYCSummary.flow_status`（A:243）、`KYCApplication.flow`（A:566）。D15 只說拆成兩條軸，L255 只說刪值。消費端量：共用文案目錄 `zh-TW.json` 124 行、`admin/.../store-row.ts` 29 行、`seller-app/.../kyc_flow.dart` 18 行。
6. **`shares_bank_account_with_sanctioned_account`**（A:571）。D4 (L267)「共用銀行帳號列入關聯帳號，只標記」；步驟 6 (L131) 把訊號改成 fingerprint、放在 `LinkedAccountHits`。申請層的這個旗標計畫沒提。選項：刪；或改由 fingerprint 推導並改名。
7. **只適用文件的退回原因碼**：`KYC_ISSUE_CODE_UNREADABLE`、`EXPIRED`（S:716-721）與 `KYCIssue.document_asset_id` 的規則（S:734-738、A:693-697）。公司文件還在，所以看起來仍需要（推論）；計畫沒寫。
8. **`IndividualKYC` 變空**（第 2 節 1、2）：oneof 留空 message，或改成型態 enum（admin 已有 `KYCSellerType`，A:337-343）。
9. **`KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW = 2`**（S:853-854）與 `STORE_ACTION_UNAVAILABLE_REASON_KYC_NOT_APPROVED = 7`、`KYC_REVIEW_ACTION_REBIND = 5`：都是從 D14 推出來的刪除，計畫沒有逐一點名。REBIND 另有既有審核紀錄列的問題（D11 對 `PAYUNI_TERMS` 的處理是「不轉換舊紀錄」，L285；REBIND 沒有對應的話）。
10. **註解描述的行為**：S:342-347「店家有 PayUni 註冊之後不能清掉」聯絡欄位；S:285「物流建店的前置條件是商家認證（含 PayUni 審核）核准」；S:963「PayUni 綁定需要的店家資料」。PayUni 註冊刪掉後規則的依附對象不在了；改成什麼條件（有收款帳戶之後不能清？完全不擋？）計畫沒寫。D17 只談店名與身分欄位鎖。
11. **`KYC_PRIVACY_NOTICE` 的內容**：種類留著，但告知書的內容描述的蒐集項目少了（法務文件，不在 proto）；D11 說「入駐只記平台自己的條款」(L285)。只提示，屬別的盤點。

## 4. 計數與指令

決定明確（含標「推論」者另計）的刪除量，實測自上表逐列加總：

- **RPC**：刪 4（PayUni）；UNDECIDED 3（`RevealKYCSensitive`、`ListKYCReveals`、`ListKYCDocumentAccesses`）。
- **message**：刪 13（A：`PayUniStatusCodes`、`PayUniRegistration`、`PayUniObservation`、4 組 Request／Response 共 8；S：`PayUniProblem`、`SellerPayUni`）；UNDECIDED 12（`RevealKYCSensitiveRequest/Response`、`ListKYCReveals*` 2、`ListKYCDocumentAccesses*` 2、`KYCReveal`、`KYCDocumentAccess`、`KYCDocumentView`、`KYCFlow`、`KYCResubmitUnavailability`、兩個 `IndividualKYC` 算 1 組）。
- **整個 enum**：刪 3（`PayUniObservationSource` 5 值、`PayUniBindStatus` 4 值、`PayUniState` 9 值，共 18 值）；UNDECIDED 2（`KYCFlowStatus`、`KYCResubmitUnavailableReason`）。
- **保留的 enum 裡要刪的值**：明確 22（`KYCFlowStatus` 6、`KYCResubmitUnavailableReason` PAYUNI_* 7、`REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY` 1、`AGREEMENT_KIND_PAYUNI_TERMS` 1、`KYCField` 7：BANK_CODE、BANK_ACCOUNT、ID_CARD_FRONT、ID_CARD_BACK、BANK_BOOK、INDIVIDUAL_ID_CARD_NO、BUSINESS_REP_ID_CARD_NO）；推論 6（`STORE_ACTION_UNAVAILABLE_REASON` 3、4、5、7；`KYC_REVIEW_ACTION_REBIND`；`KYC_FIELD_BANK_NAME`）；UNDECIDED 11（`KYC_FIELD_` 1–6、`LINKED_IDENTITY_` 1 與 3、`KYC_ISSUE_CODE_` 1 與 3、`KYC_RESUBMIT_UNAVAILABLE_REASON_UNDER_REVIEW`）。
- **保留的 message 裡要刪的欄位**：明確 20（S `KYCApplication` 6：11、12、13、19、20、21；S `IndividualKYC` 2；S `BusinessKYC` 2：8、9；S `KYCFlow` 2：3、5；A `KYCApplication` 2：13、14；A `IndividualKYC` 1；A `BusinessKYC` 1：8；A `RevealKYCSensitiveResponse` 3：1、2、3；A `ListStoresRequest` 1：7）；推論 2（`bank_name`：S 10、A 12）；UNDECIDED 至少 19（S 與 A 的 `real_name`…`english_name` 各 6、`KYCApplicationSummary.real_name`、A `shares_bank_account…`、`KYCFlow` 的 2 與 4、A 的三個 flow 欄位視 enum 而定）。
- **只改註解**：PayUni 字樣 119 行分佈在 5 個檔（`git grep -c -i payuni -- api/hawkr`：RQ 2、A 59、`store_card.proto` 2、`discovery.proto` 1、S 55）；刪完符號後要剩 0（L257-258 的驗收）。

要重新生成的生成物（刪 proto 後）：
- backend：`make connect` → `backend/internal/httpapi/gen/hawkr/**`（`backend/buf.gen.connect.yaml` 的 out）。
- TS：`pnpm --filter @hawkr/api gen:api` → `packages/hawkr-api-ts/src/gen/**`。
- Dart：`../../api/generate.sh buf.gen.yaml` → `packages/hawkr_api/lib/gen/**`（只有賣家／買家端的 proto；admin 的 proto 沒有 Dart 生成物，`git ls-files` 實測）。
- 另外會連動（不是 buf 生成，但依 enum 值）：共用文案目錄 `backend/internal/platform/uitext/catalog/zh-TW.json` 有 `KYCFlowStatus`、`PayUniState`、`KYC_FIELD_*` 的條目，`make uitext` 會重生 `packages/hawkr-ui-text-ts/src/**/*.gen.*`、`packages/hawkr_ui_text/lib/src/ui_text.g.dart`、`enum_manifest`（`git grep -l` 實測有命中）。

指令（都在 worktree 根目錄執行）：

```sh
# 起點
git grep -n -i payuni -- api/hawkr
git grep -c -i payuni -- api/hawkr
git grep -n -E 'id_card|rep_id|bank_|KYCDocument|Reveal|DocumentAccess|identity_locked|KYCFlow|KYC_FIELD_|AgreementKind|AGREEMENT_KIND|LINKED_IDENTITY_|issuable_fields|KYCIssue|national_id|masked|locked' -- api/hawkr
git grep -n PayUniRemedy -- api/hawkr            # 0 筆
git grep -n -i -E 'reserved|編號' -- 'backend/docs/開發規範/API 設計規範（Connect）.md'   # 0 筆
git grep -c -E '^ *reserved ' -- api/hawkr/seller/v1alpha1/seller.proto api/hawkr/admin/seller/v1alpha1/seller.proto

# 每個符號、每個區域的 行數/檔數（$FL 是空或 -i，$PAT 是表中括號裡的比對式）
EX=(':!**/gen/**' ':!**/*.pb.go' ':!**/*.g.dart' ':!**/*.gen.*' ':!**/*.g.json')
for a in backend/internal backend/cmd admin/src website seller-app/lib buyer-app/lib packages; do
  git grep $FL -c -E "$PAT" -- "$a" "${EX[@]}"     # 加總最後一欄＝行數，列數＝檔數
done
# 最多的檔
git grep $FL -c -E "$PAT" -- backend/internal backend/cmd admin/src website seller-app/lib buyer-app/lib packages "${EX[@]}" | sort -t: -k2 -nr | head -4
# 全域量級：-i payuni → b2585/147 c51/8 a147/15 w362/108 s220/61 y110/33 p0/0（含 D 類，買家付款、訂單、物流留給 B 階段，PLAN L256）
```

限制：
- 計數的排除式沒有排掉 `*.freezed.dart`（buyer-app 的 `bank_code` 50 行多半是它）與測試檔；`backend/internal` 的數字包含 `CARRYOVER.md`、SQL、migration、文案 JSON。
- `-i id_?card_?no` 這類式子同時命中 snake_case、camelCase 與 Go 的 `IDCardNo` 嗎？Go 的縮寫若寫成 `IDCardNo` 會命中（不分大小寫），寫成 `IDNumber` 之類則不會；沒有逐一核對後端的命名。
- 沒有看 `backend/api/proto/**`（舊 gRPC 契約）、`backend/api/openapi`、`api/testdata`（對 kyc／payuni／id_card／bank 0 筆，實測）、CodeMeta。
- 沒有跑 `buf lint`／`buf breaking`；「刪掉後仍被引用」只靠閱讀兩個 seller.proto 的相關區段與 grep，沒有編譯驗證。A 檔全文讀過（1-824）；S 檔讀了 60-360、640-1140，其餘只用 grep（KYC／PayUni 在其他區段的命中只有 S:4、S:50、S:486 的註解）。
