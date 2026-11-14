# 步驟 8 client 端刪除盤點：seller-app、buyer-app、admin、Dart 套件

- worktree：`hawkr-payment-onboarding`，分支 `payment-onboarding`，快照 `499c5ae8`（`git rev-parse` 核對過）。
- 唯讀盤點；沒有跑 build、測試、生成。所有結論是「讀檔與 grep」等級，標「推論」的是沒有逐行讀完的部分。
- 路徑都相對 worktree 根目錄。website 不在範圍。
- 計畫＝`backend/docs/plans/2026-10-29-payment-onboarding-stripe.md`；草稿＝同名目錄下 `step7-screen-drafts.md`。

## 0. 先講三件最影響範圍的事

1. **seller-app 的 KYC 表單在 7c 已經刪掉 D22 的欄位**：`seller-app/lib/features/seller` 搜 `idCard|bankAccount|bankCode|bankBook|realName|englishName` 是 0 筆（`kyc_fields.dart:55-80` 只剩 PRODUCT_TYPE、WEBSITE_URL 與公司欄位）。步驟 8 在 seller-app 剩下的是 `KYCFlow`（`flow.status`、`can_resubmit`、`identity_locked`）與鎖定文案，沒有表單欄位要刪。
2. **admin 的補充文件（D24 Q2 保留）目前只能經 `RevealKYCSensitive` 看到**：`KYCDocumentView.view_uri` 與 `asset_id` 只在顯示全文的回應裡（`api/hawkr/admin/seller/v1alpha1/seller.proto:367-395`、`714-720`）；admin 的 `DocumentImage`、`documentChoices` 都吃 `reveal`（`KycApplicationCards.tsx:251-300`、`kyc-application.ts:338-352`）。刪顯示全文就等於 admin 看不到保留下來的公司文件，也無法在退回時指定哪一張。計畫沒有寫替代做法（見 3. N2）。
3. **R13 的內容在 repo 裡找不到**：計畫進度表只有「R12 交 Will，R13 延到步驟 8」（計畫 339 行）；`15d13cdc`、`864ef64d` 的 commit 訊息沒有編號；`git grep -E '\bR13\b'` 在計畫目錄 0 筆。**推論**：R13 就是同一格「未驗證或已知缺口」寫的「步驟 8 刪 flow 時要改 `MyKyc.fromApplication`（仍要求 `flow.status` 有效）」（`seller-app/lib/features/seller/data/current_kyc.dart:28-38`），因為它是 7c 唯一標明延到步驟 8 的項目；沒有文字直接把兩者連起來，需要 7c 的審查 ledger 才能確認。

---

## 1. seller-app

### 1.1 表

| path | 內容 | 類別 | 建議處置 | 依賴的 proto／端點 | 受影響的測試 |
|---|---|---|---|---|---|
| `lib/features/seller/data/current_kyc.dart`（＋`current_kyc.g.dart` 重新生成） | `MyKyc`：`status: KYCFlowStatus`（:40）、`identityLocked`（:17、:35、:43、:55、:59）、`fromApplication` 要求 `flow` 有效否則丟 `FormatException`（:28-38）、`notStarted()` 用 `KYC_FLOW_STATUS_NOT_STARTED`（:23）；`AccountKyc.build`／`adopt` 的 FormatException 分支（:79-110） | D15-flow-status ＋ D17-locks | edit：拿掉 `status`、`identityLocked` 與 flow 檢查（`reviewStatus` :49 已經是 `KYCApplication.status`）。`MyKyc.status` 在 lib 沒有讀取者（`git grep -nE "kyc[A-Za-z]*\??\.(value\??\.)?status" -- seller-app/lib` 0 筆） | seller `GetMyKYC`、`KYCApplication.flow`（`KYCFlow`，seller.proto:875-888） | `test/features/seller/data/current_kyc_test.dart`（KYCFlowStatus 28 筆、identityLocked 7 筆）、`seller_status_refresh_test.dart`（7 筆）、`test/support/kyc_fixtures.dart:15-36`；coverage baseline 有列 |
| `lib/features/seller/data/kyc_flow.dart` | `KycFlowCopy`、`_kycFlowTone`（11 個 `KYCFlowStatus`，:40-52）、`isValidKycFlowStatus`（:55）、`kycFlowCopy`（:60-73，讀目錄的 `action`／`bannerTitle`／`bannerDescription`／`gateHeading`／`gateMessage`） | D15-flow-status | delete file（lib 唯一的使用者是 `current_kyc.dart:30` 與 `seller_certification_upload_screen.dart:597`）；前提是 N1 的結論 | `KYCFlowStatus`；`hawkr_ui_text` 的 `SellerKYCFlowStatusUiText` | `test/features/seller/data/kyc_flow_test.dart`（整檔，38 筆）；coverage baseline 285 行 |
| `lib/features/seller/data/kyc_form.dart:403-411` | `kycEditable` 讀 `app.flow.canResubmit`；`kycCanDiscard` 讀 `app.flow.identityLocked` | D15 ＋ D17 | edit：`kycCanDiscard` 拿掉鎖；`kycEditable` 的替代來源 UNDECIDED（N1） | `KYCFlow.can_resubmit`、`identity_locked` | `kyc_form_test.dart`（KYCFlowStatus 6、flow 4；:340 用 `KYC_FIELD_BANK_CODE`、`KYC_FIELD_REAL_NAME` 當「不在表單上的欄位」的例子，enum 值刪掉後要換） |
| `lib/features/seller/data/kyc_wizard.dart:181-184` | `identityLocked` getter | D17-locks | edit：刪 getter | `KYCFlow.identity_locked` | `kyc_wizard_test.dart`（:399-403 `PAYUNI_REJECTED` 情境、:1223-1225 鎖定不能捨棄、KYCFlowStatus 7 筆） |
| `lib/features/seller/presentation/kyc_wizard_steps.dart` | 公司資訊步驟 `locked = c.identityLocked`（:152）讓統編、公司名稱、販售類型 `readOnly`（:164、:203、:255）並關掉自動帶入（:188）；確認頁 `kPayuniLockOnSubmitNotice`（:589-598，key `kyc_lock_on_submit_notice`） | D17-locks | edit：刪 readOnly 分支與送出前鎖定提醒 | 同上 | `seller_certification_upload_screen_test.dart:728-745`；讀原始碼的守門 `protected_gateway_client_inventory_test.dart:24`、`protected_media_architecture_test.dart:52`（只列檔名，檔還在就不受影響） |
| `lib/features/seller/presentation/seller_certification_upload_screen.dart` | 自動查統編的條件含 `!wizard.identityLocked`（:316）；退回提示 `if (!app.hasFlow()) return SizedBox.shrink()`（:593）與 `kycFlowCopy(KYC_FLOW_STATUS_REJECTED)`（:597）；註解 :577 | D17 ＋ D15 | edit；退回提示的標題文字改讀什麼 UNDECIDED（N1） | `KYCFlow`；`KYCFlowStatus.REJECTED` 的目錄文字 | `seller_certification_upload_screen_test.dart`（KYCFlowStatus 7、identityLocked 5、`PAYUNI_TERMS` 4：:61 夾具、:489-511、:664-670） |
| `lib/features/seller/data/store_form.dart` | `supportEmail.hint = 'PayUni 會寄金流相關通知到這個信箱'`（:32）；`lockedByPayuni`（:69-71） | PAYUNI-onboarding（文案）＋ D17 | edit：刪 `lockedByPayuni`；hint 的新文字 UNDECIDED（N7） | 無（店家欄位 `UpdateMyStore`） | `store_editor_test.dart:180-192`（鎖定與 hint 逐字） |
| `lib/features/seller/presentation/profile_form_widgets.dart` | `kPayuniLockedNotice`（:13）、`kPayuniLockOnSubmitNotice`（:17）；`ProfileEditableInfoRow.lockedNotice` 參數與顯示（:68、:85、:94、:143-146） | D17-locks | edit：刪兩個常數；`lockedNotice` 參數沒有別的呼叫者（`git grep lockedNotice -- seller-app/lib` 只有本檔與 `profile_screen.dart:434`），一起刪 | 無 | `profile_payuni_locked_test.dart`（整檔）、`profile_screen_test.dart:64-85、:272-287`；coverage baseline 310 行 |
| `lib/features/seller/presentation/profile_screen.dart:67-68、:434` | `payuniLocked = kyc?.identityLocked`；店名與介紹列的 `lockedNotice` | D17-locks | edit：刪掉；刪完後本檔是否還需要 `currentKycProvider` 要看（:67 是唯一用途，**推論**） | `GetMyKYC.flow.identity_locked` | `profile_screen_test.dart`；`chat_icon_policy_test.dart:42` 只列檔名 |
| `lib/features/seller/data/kyc_store_contact.dart:5`、`presentation/certification_status_view.dart:57` | 註解提到 PayUni、`KYCFlowStatus` | naming-only | edit：改註解 | 無 | 無 |
| `lib/features/seller/data/seller_agreements_repository.dart` | 只用 `KYC_PRIVACY_NOTICE`、`SELLER_TERMS`（:19、:44、:47）；lib 沒有 `PAYUNI_TERMS`（7a 已拿掉） | D11-terms | keep（lib 不用改） | `AgreementKind`、`GetSellerAgreements` | `seller_agreements_repository_test.dart:18-34、:78`（夾具用 `AGREEMENT_KIND_PAYUNI_TERMS`，enum 值刪掉後編譯失敗，要改寫）；`seller_certification_upload_screen_test.dart:61` |
| `lib/features/notification/data/push/seller_status_push.dart`（＋`manager.dart:456`、`router.dart:36、:60`、`presentation/notification_banner.dart:112`） | 推播類型 `PAYUNI_STATUS`、`SELLER_REVIEW_STATUS`、`MERCHANT_VERIFICATION`、`CERTIFICATION_*`、`MERCHANT.VERIFICATION.*`（:9-17）觸發重讀 | D-class stays（Java 推播 `PAYUNI_STATUS`）；`MERCHANT.VERIFICATION.*` 的來源 `sellerrelay` 在步驟 8 刪（計畫 255 行） | keep `PAYUNI_STATUS`；`MERCHANT.VERIFICATION.*` 一支 UNDECIDED（N8） | Java 推播（FCM payload），不是 Connect | `seller_status_push_test.dart:57-76、:102`、`manager_status_signal_test.dart:106` |
| `lib/features/seller/data/onboarding_tasks.dart:5、:90-98` | 物流任務讀 `payuniSubmitStatus`、CodeMeta 的 `PayUniSubmitStatus` | D-class stays（物流 onboarding，計畫 256 行點名） | keep | REST `/api/v1/app/shipping_onboardings` | `onboarding_tasks_test.dart:330-346` 不動 |
| `lib/features/settings/**`（`shipping_sender.dart`、`shipping_settings_screen.dart`、`cvs_shipping_section.dart`、`taqbin_shipping_section.dart`、`store_map_webview_screen.dart`）5 檔 | 物流寄件資料鎖定、7-ELEVEN 電子地圖 | D-class stays | keep | REST `/api/v1/app/shipping_onboardings*`（`shipping_settings_remote_service.dart:15`、`cvs_shipping_mutation.dart:30-32`） | `shipping_sender_test.dart`、`shipping_settings_prefill_test.dart` 不動 |
| `lib/features/order/**`（`payuni_logistics_service.dart`＋`.g.dart`、`logistics_tracking_screen.dart`、`shipment_error_dispatch.dart`、`after_sale_expanded_section.dart` 等） | 出貨、物流查詢、售後 | D-class stays | keep | REST `/api/v1/app/logistics`（`payuni_logistics_service.dart:61`） | `payuni_logistics_service_test.dart`、`progress_parity_test.dart`、`shipping_provider_dispatch_test.dart`、`dead_endpoint_guard_test.dart:50`、`testing_strategy_policy_test.dart:16` 不動 |
| `lib/core/codemeta/code_meta.dart`（52 筆） | CodeMeta 生成檔 | generated（D-class） | keep，照本機脈絡檔的 CodeMeta 規則 | CodeMeta | 無 |
| `lib/features/media/data/media_model.dart:91` | 註解 `docType（id_card, bank_account, contract, document）` | naming-only（D22 的殘留字樣） | edit：改註解；上傳路徑本身留（補充文件與措施佐證還在用，`measures.dart:25`） | 舊 gateway 的 secure upload（REST） | 無 |
| 其餘 36 個 lib 檔（report、chat、product、post、photo_editor、shared 等） | 只在註解引用 `backend-payuni#NNN`、`PayUni #758` | naming-only | keep | — | — |
| `tool/coverage_baseline.json` | `instrumentedFiles` 有 6 個已不存在的檔：`seller/data/bank_book_ocr_service.dart`、`id_card_ocr_service.dart`、`kyc_ocr.dart`、`payuni.dart`、`universal_ocr_service.dart`、`presentation/widgets/payuni_status_body.dart`；還列著 `kyc_flow.dart`（:285） | coverage ratchet | edit：刪檔後照 runbook 的 coverage ratchet 程序重算（`coveredLines`／`instrumentedLines` 會變） | — | `tool/coverage_ratchet.dart`（沒讀它對已不存在的檔怎麼處理，**未驗證**） |
| `README.md:236、:275`、`AGENTS.md:26`、`ARCHITECTURE.md:60`、`docs/design/SELLER_APP_PRODUCT_UX_MASTER_SPEC.md`（11 筆，:93、:113、:302、:390-395、:461、:516、:536） | 文件仍寫「PayUNi 審核狀態面板」「`payuni.dart`」「`flow.payuni`」 | PAYUNI-onboarding（文件） | edit：README 兩列與目錄樹註解跟著程式改；UX master spec 改不改 UNDECIDED（N9） | — | — |
| l10n arb | `seller-app/lib/l10n` 不存在；`git grep -ciE 'payuni|身分證|銀行|存摺' -- '*.arb'` 在 seller-app 0 筆 | — | 無事可做 | — | — |

### 1.2 seller-app 受影響測試彙總（13 檔，`git grep -lE 'identityLocked|KYCFlowStatus|KycFlow|lockedByPayuni|kPayuni|canResubmit|PAYUNI_TERMS|SellerPayUni' -- seller-app/test`）

- 刪檔：`data/kyc_flow_test.dart`（隨 N1）、`presentation/profile_payuni_locked_test.dart`。
- 改寫：`current_kyc_test.dart`、`kyc_form_test.dart`、`kyc_wizard_test.dart`、`seller_agreements_repository_test.dart`、`seller_status_refresh_test.dart`、`store_editor_test.dart`、`profile_screen_test.dart`、`seller_certification_upload_screen_test.dart`、`wallet_screen_test.dart`（KYCFlowStatus 3 筆）、`notification/.../seller_status_push_test.dart`（KYCFlowStatus 1 筆）、`test/support/kyc_fixtures.dart`（`identityLocked`、`SellerPayUni? payuni` 參數 :15-36）。

---

## 2. buyer-app

結論：**只有 D 類與只是命名的引用，沒有步驟 8 要刪的東西。**

- 查法：`git grep -cE 'KYCFlow|KYCStatus|KYCField|PayUniRemedy|PayUniBindStatus|PayUniState|SellerPayUni|PAYUNI_TERMS|AgreementKind|identityLocked|idCard|IdCard|bankAccount|bankCode|bankBook|getMyKYC|PAYUNI_BIND|Rebind|Reveal' -- buyer-app/lib buyer-app/test ':!buyer-app/lib/core/codemeta'` 只有 2 個檔命中，都不是 KYC：`cart/presentation/widgets/checkout_payment_sheet.dart:697-701`（ATM 的 `atmBankCode`，買家付款）與 `shared/widgets/buyer_floating_navigation_layout.dart`（`_bottomReveal*` 是動畫名稱）。

| path | 內容 | 類別 | 處置 | 依賴 | 測試 |
|---|---|---|---|---|---|
| `lib/core/payment/pay_token_service.dart`、`lib/config/flavor.dart:123-125`、`features/order/presentation/payment_webview_screen.dart`、`refund_webview_screen.dart`、`features/order/data/order_model.dart:188`、`after_sale_order_lookup.dart:26`、`after_sale_expanded_section.dart:475`、`cart/.../checkout_payment_sheet.dart` | 買家付款、ATM 退款頁、Apple Pay token | D-class stays | keep | 舊 gateway REST（`POST /app/orders/{order_no}/refund_page` 等） | `test/architecture/dead_gateway_endpoint_guardrail_test.dart:29` 不動 |
| `features/cart/presentation/store_map_webview_screen.dart:9`、`features/order/data/logistics_track.dart`、`widgets/order_logistics_section.dart:51`、`address_management_screen.dart:509`、`shared/utils/recipient_name_validator.dart` | 7-ELEVEN 電子地圖、物流軌跡、收件人長度 | D-class stays | keep | REST | 不動 |
| `features/notification/data/push/destination.dart:201`、`presentation/notification_banner.dart:103` | 推播 `PAYUNI_STATUS` | D-class stays | keep | Java 推播 | 不動 |
| `lib/core/codemeta/code_meta.dart`（52 筆）、`*.freezed.dart` | 生成檔 | generated | keep | CodeMeta | — |
| 其餘 16 個 lib 檔與 1 個 test 檔 | 註解的 `backend-payuni#…`、`PayUni #758` | naming-only | keep | — | — |

---

## 3. admin

### 3.1 表

| path | 內容 | 類別 | 建議處置 | 依賴的 proto | 受影響的測試 |
|---|---|---|---|---|---|
| `src/app/admin/merchants/[id]/PayUniCards.tsx`（199 行） | `PayUniStatusCard`（「金流商店審核」卡：徽章、綁定失敗與錯誤碼、需補資料、金流商家編號、最後同步、「重新綁定」鈕，:39-146）；`PayUniContactCard`（「店家目前的聯絡資料」，:148-199） | PAYUNI-onboarding ＋ D14-rebind | delete file（草稿第 3 節：「PayUni 的兩張卡在步驟 8 刪除」）；聯絡資料卡的內容去向見 N6 | admin `GetPayUniRegistration`、`RebindPayUni`、`PayUniRegistration`、`PayUniBindStatus`、`PayUniProblem` | `tests/merchant-kyc-application.test.ts:1140-1387`（約 9 個測試） |
| `src/app/admin/merchants/[id]/RevealKycDialog.tsx`（110 行） | 顯示全文對話框：理由必填、授權 15 分鐘 | D22-PII（reveal） | delete file，或保留給補充文件用：UNDECIDED（N2） | admin `RevealKYCSensitive` | `tests/admin-governance-modal-a11y.test.ts`（讀原始碼；搜 `reveal` 0 筆，**推論**不受影響） |
| `src/app/admin/merchants/[id]/reveal-state.ts`（90 行） | `RevealState`、`showReveal`／`hideReveal`／`expireReveal`／`scheduleRevealExpiry` | D22-PII（reveal） | 同上，隨 N2 | `RevealKYCSensitiveResponse` | `tests/merchant-reveal-state.test.ts`（整檔，38 筆） |
| `src/app/admin/merchants/[id]/KycApplicationCards.tsx`（706 行） | `KycRevealBar`（:62-100，文字「身分證字號、銀行帳號與證件影像預設遮蔽。」:78）；`DocumentImage`（:251-320，未顯示全文時是「需先顯示全文」）；`KycIdentityCard`（:332-425：身分證正反面影像、姓名、英文姓名、出生年月日、身分證字號（遮蔽／明文）、發證日期、類型、地點）；`KycDocumentsCard`（:428-543：存摺影像＋銀行名稱、代碼、遮蔽帳號 :437-488；補充文件清單 :490-530） | D22-PII：身分證號、負責人身分證號、證件影像、銀行代碼／帳號／存摺、reveal 與遮蔽顯示 | edit：刪 `KycIdentityCard` 整張與 `KycDocumentsCard` 的存摺／銀行區塊（草稿第 3 節：「身分與銀行的 KYC 卡（`KycIdentityCard` 等）…步驟 8 與欄位一起刪」）；`KycRevealBar`、`DocumentImage` 與補充文件區塊隨 N2；姓名、英文姓名、出生日、發證欄位隨 N3 | admin `KYCApplication`：`real_name`(6)、`birth_date`、`id_card_issue_date/type/location`(8-10)、`english_name`(11)、`bank_name`(12)、`bank_code`(13)、`bank_account_masked`(14)、`documents`(22)；`IndividualKYC.id_card_no_masked`、`BusinessKYC.rep_id_card_no_masked`；`KYCDocument`、`KYCDocumentView` | `tests/merchant-kyc-application.test.ts:531-760`（文件欄位四個、遮蔽、顯示全文、到期收回、影像網址，約 12 個測試）；`tests/merchant-kyc-issues.test.ts`（遮蔽欄位夾具，24 筆） |
| `src/app/admin/merchants/[id]/kyc-application.ts`（1020 行） | 文案 `REVEAL_DIALOG_TEXT`（:42）、`DOCUMENT_NEEDS_REVEAL_TEXT`（:44）、`APPROVE_SUCCESS_TEXT`「…系統會在背景向 PayUni 註冊…」（:57-58）、`REBIND_SUCCESS_TEXT`（:62-63）；`PAYUNI_POLL_MIN_MS`、`payUniPollInterval`（:107、:147-163）；`idCardNoField`（:220-224）；`DOCUMENT_FIELDS` 含 `ID_CARD_FRONT`／`ID_CARD_BACK`／`BANK_BOOK`（:230-235）；`documentChoices`（:323-352）；`reviewActionLabel` 的 `KYC_REVIEW_ACTION_REBIND`「重新綁定 PayUni」（:512-513）；`KycReveal`、`revealFrom`、`activeReveal`、`revealRemainingMs`、`revealUntilText`、`sensitiveValue`、`documentViewOf`、`documentSrc`、`documentsOf`（:545-642）；`PayUniState`、`payUniOf`、`rebindPresentation`、`payUniProblemFields`（:906-958） | 混合：PAYUNI-onboarding、D14-rebind、D22-PII | edit：刪 PayUni 段（:906-958）、輪詢、rebind 文案、`idCardNoField`、`DOCUMENT_FIELDS` 的三個 PII 值；reveal 相關隨 N2；`APPROVE_SUCCESS_TEXT` 的新文字與 `KYC_REVIEW_ACTION_REBIND` 的舊紀錄顯示 UNDECIDED（N5、N4） | `RevealKYCSensitiveResponse`、`PayUniRegistration`、`PayUniBindStatus`、`KYCField`、`KYCReviewAction`、`KYCIssue.document_asset_id` | `tests/merchant-kyc-application.test.ts`（185 筆命中；:505 窮舉 `KYCField`、:548 `idCardNoField`、:1438 接線測試以原始碼核對「顯示全文、重新綁定各走自己的方法」）；`tests/merchant-payment-account-card.test.ts:145` 的測試名稱引用 `payUniOf` |
| `src/app/admin/merchants/[id]/page.tsx`（1654 行） | import `PayUniBindStatus`（:22）、`PayUniCards`（:99）、`RevealKycDialog`（:102）、reveal-state（:105-112）；`showRevealDialog`、`secret`（:226-228）；`usePayUniRegistration`（:268）；PENDING→非 PENDING 的效果（:293-294）；核准後 invalidate `getPayUniRegistration`（:467）；`revealMutation`（:481-482）、`rebindMutation`（:489-490）；到期排程（:506-537）；`handleReveal`（:708-723）、`handleRebind`（:746-755）；頁首徽章 `flowBadge(kycSummary.flowStatus)`（:820）；頁首「金流商家編號：merId」（:979-986）；核准鈕上方「核准後系統會向 PayUni 註冊」（:1029）；審核備註「重點核對營業登記證與負責人身分證、統一編號是否一致，並確認所有證件皆在有效期內」（:1199-1205）；`PayUniStatusCard`（:1229-1238）、`PayUniContactCard`（:1240-1243）、`KycRevealBar`（:1246-1252）、`KycIdentityCard`（:1269-1272）、`KycDocumentsCard`（:1316-1319）；退回對話框 `choices={documentChoices(liveReveal)}`（:1549）；`RevealKycDialog`（:1560-1566） | 混合 | edit：刪兩張 PayUni 卡與 rebind、頁首 merId；其餘隨 N2、N3、N5 | admin `GetPayUniRegistration`、`RebindPayUni`、`RevealKYCSensitive`、`KYCSummary.flow_status`(3) | 以原始碼讀本檔的測試：`merchant-kyc-application.test.ts:1438-1510`、`merchant-detail-no-duplicate-keys.test.ts`、`merchant-store-detail.test.ts`、`merchant-payment-account-card.test.ts`、`merchant-onboarding-summary-card.test.ts`、`scroll-to-hash.test.ts`、`admin-governance-modal-a11y.test.ts`（後五個搜 `payuni|reveal|rebind` 0 筆，**推論**只在結構改變時受影響） |
| `src/app/admin/merchants/[id]/use-store-detail.ts:14-15、:23、:48、:69-81` | `usePayUniRegistration`（退避輪詢） | PAYUNI-onboarding | edit：刪 hook 與 import | admin `GetPayUniRegistration` | `merchant-kyc-application.test.ts:1208`、`:1567` |
| `src/app/admin/merchants/[id]/RejectKycDialog.tsx:12、:37` | `choices: DocumentChoice[]`（退回時指定某一張文件） | D22-PII（間接，經 reveal） | 隨 N2：沒有 reveal 就沒有 `asset_id`，UNREADABLE／EXPIRED／MISMATCH／INVALID 四種原因在文件類欄位「必須帶」asset（proto:694-696），補充文件將無法用這四種原因退回 | `KYCIssue.document_asset_id`、`KYCDocumentView.asset_id` | `tests/merchant-kyc-issues.test.ts` |
| `src/app/admin/merchants/store-row.ts` | `PENDING_FLOWS`（:24-31：SUBMITTED、BINDING、BIND_FAILED、FORWARDED、UNDER_REVIEW、PENDING_INFO）、`REJECTED_FLOWS`（:33-36：REJECTED、PAYUNI_REJECTED）、`FLOW_VARIANT`（:48-60，11 值）、`flowBadge`（:66-73，文字 `sellerKYCFlowStatus`）、列的徽章 `flowBadge(item.kycFlowStatus)`（:90） | D15-flow-status | edit；清單與詳情頁首的徽章改讀什麼 UNDECIDED（N1；計畫 7e4 寫「列表頁的 KYC 徽章不動（D31 Q4）」，那是 7e4 的範圍，不是步驟 8 的結論） | admin `StoreListItem.kyc_flow_status`(3)、`KYCSummary.flow_status`(3)；`@hawkr/ui-text` 的 `sellerKYCFlowStatus` | `tests/merchant-store-row.test.ts`（9 筆） |
| `src/app/admin/merchants/page.tsx:378` | 註解「審核、綁定中的 PayUni 狀態也在這顆徽章上」；篩選分頁由 `FILTER_TABS` 產生（:272-278） | D15（註解）＋ D14（分頁） | edit | — | `tests/merchant-store-list.test.ts` |
| `src/lib/store-search.ts:6、:19、:31、:56、:69-70` | 篩選分頁 `rebind`「待重新綁定」→ `payuniBindStatus: PayUniBindStatus.FAILED` | D14-rebind | edit：刪分頁；要不要換成收款帳戶建立失敗的分頁 UNDECIDED（N10） | admin `ListStoresRequest.payuni_bind_status`(7)、`PayUniBindStatus` | `tests/merchant-store-list.test.ts:7、:35` |
| `src/app/admin/queue/queue-item.ts:23` | `QUEUE_KIND_TEXT[PAYUNI_BIND_RETRY] = "重新綁定 PayUni"`（窮舉 Record，enum 值刪掉 `tsc` 會擋）；`paymentAccount` 的連結已在（:68-69） | D14-rebind | edit：刪一列 | `ReviewQueueItemKind.PAYUNI_BIND_RETRY`（review_queue.proto:45） | `tests/review-queue-page.test.ts:185、:226` |
| `src/lib/unavailability.ts:42-44` | `NO_ENTRY` 的 `PAYUNI_MERCHANT_TERMINAL`、`PAYUNI_BIND_IN_PROGRESS`、`PAYUNI_BOUND`（窮舉 Record） | D14-rebind | edit：隨 proto 刪三列（計畫步驟 8 沒有點名 `StoreActionUnavailableReason` 的這三個值，**推論**它們隨 `RebindPayUni` 一起沒有用途） | admin `StoreActionUnavailableReason`（seller.proto:132-138） | `tests/unavailability.test.ts:44-70` |
| `src/lib/kyc-status.ts:11` | 註解「PayUni 的結果另看 PayUni 卡」 | naming-only | edit：改註解 | — | — |
| `src/styles/pages/MerchantCertificationDetail.css` | PayUni 專用：`.payuni-logo-text`（:446）、`.payuni-reason-text`（:479）、`.payuni-sync-meta`（:485）、`.payuni-contact-card`（:541-544，另有 :1290 的註解拿它當間距參考）；身分證區：`.id-card-grid`／`-item`／`-image`／`-placeholder`／`-footer`／`-label`（:607-662）；reveal：`.kyc-reveal-bar`／`-actions`／`-text`（:1141-1160）；文件：`.document-list`／`-item`／`-image`／`-info`／`-number`／`-sub`／`-link`（:669-738）、`.kyc-document-open`（:1204） | 混合 | edit：`.payuni-*` 的唯一使用者是 `PayUniCards.tsx`，刪；`.id-card-*` 的 tsx 使用者只有 `KycApplicationCards.tsx`（`src/app/globals.css` 也有同名規則，要一起看）；`.document-*`、`.kyc-document-open`、`.kyc-reveal-*` 隨 N2；`.payment-label`、`.payment-desc`、`.payment-status-card` 還有 `PaymentAccountCard.tsx`、`OnboardingSummaryCard.tsx` 在用，keep | — | 沒有找到讀這份 CSS 的測試（`git grep -l MerchantCertificationDetail -- admin/tests` 未單獨跑，**未驗證**） |
| `src/app/admin/merchants/[id]/measure.ts:175-190` | `LINKED_IDENTITY_LABEL`：`NATIONAL_ID`「身分證號」、`RESPONSIBLE_PERSON_ID`「負責人身分證號」、`BANK_ACCOUNT`「銀行帳號」 | D22-PII（防詐關聯的顯示） | keep（計畫 255 行：`account_identity_hashes` 已有的列保留到 5 年清理；D12 改用 fingerprint）；enum 值是否改名 UNDECIDED（N11） | admin `LinkedIdentity`（measure.proto:218-226）、`KYCApplication.shares_bank_account_with_sanctioned_account`(26) | — |
| `src/lib/codemeta/generated.ts`（21 筆） | CodeMeta 生成檔（含 `PayuniMerchantVerificationStatus` :117-128） | generated | keep，照 CodeMeta 規則；`PayuniMerchantVerificationStatus` 有沒有 admin 使用者沒查 | CodeMeta | — |
| `src/lib/disputes/detail.ts:4`、`src/lib/posts/adminList.ts:4` | 註解 `backend-payuni` | naming-only | keep | 舊 gateway REST | `admin-post-list-media-contract.test.ts`、`product-detail-contract.test.ts`、`shipping-provider-no-fallback.test.ts`、`admin-bff-no-seller-scope.test.ts:51` 不動 |
| `src/features/invoices/**`、`src/app/admin/invoices/**`（grep 的 `document`／`masked` 命中） | 電子發票文件 | 無關（D-class，舊 REST） | keep | REST | `invoice-adm-contract.test.ts` 不動 |
| `admin/README.md:28`、`admin/PRODUCT.md:56` | README：「認證審核（身分證 / 存摺等證件）、PAYUNi 金流狀態同步」；PRODUCT：`backend-payuni` 是 repo 名 | 文件 | README edit；PRODUCT keep（naming-only） | — | — |

admin 沒有用到的舊 RPC（不需要刪 client 程式）：`ListKYCReveals`、`ListKYCDocumentAccesses`、`ListPayUniObservations`、`RefreshPayUniStatus`（`git grep -niE 'listKYCReveals|listKYCDocumentAccesses|listPayUniObservations|refreshPayUniStatus' -- admin/src` 只有 `PayUniCards.tsx:22` 的註解「不做 UI」）。所以「存取稽核」在 admin 沒有畫面：4 支 PayUni RPC 裡 client 實際呼叫的只有 `GetPayUniRegistration` 與 `RebindPayUni`。

### 3.2 admin 受影響測試彙總

- 整檔刪或大改：`tests/merchant-reveal-state.test.ts`（隨 N2）。
- 大量刪改：`tests/merchant-kyc-application.test.ts`（1140-1387 的 PayUni 段刪；531-760 的遮蔽與顯示全文段隨 N2、N3；1438 的接線測試改）。
- 小改：`merchant-kyc-issues.test.ts`、`merchant-store-row.test.ts`、`merchant-store-list.test.ts`、`review-queue-page.test.ts`、`unavailability.test.ts`、`merchant-payment-account-card.test.ts:145`（只是測試名稱）。

---

## 4. Dart 套件

| path | 內容 | 類別 | 處置 | 依賴 | 測試 |
|---|---|---|---|---|---|
| `packages/hawkr_api/lib/gen/hawkr/seller/v1alpha1/*.dart`（`seller.pb.dart` 76、`seller.pbenum.dart` 84、`seller.pbjson.dart` 34、connect client／spec 各 2） | 生成碼 | generated | regenerates（`../../api/generate.sh buf.gen.yaml`） | `api/hawkr/seller` | — |
| `packages/hawkr_api/lib/gen/hawkr/common/v1alpha1/store_card.pbenum.dart:23、:28`、`discovery/*.dart` | proto 註解寫「身分認證與 PayUni 審核通過」 | generated（來源是 `api/hawkr/common`、`discovery` 的註解） | regenerates；proto 註解要改才會消失（屬 api 的盤點，不在本檔範圍） | — | — |
| `packages/hawkr_api/test/field_mask_test.dart:46-58` | 手寫測試拿 `KYCApplication` 的 `individual.id_card_no`、`bank_book.asset_id`、`birth_date.year` 當 field mask 路徑的例子 | D22-PII（測試夾具） | edit：欄位刪掉後 `fieldMask` 會因路徑不存在而改變結果，換成留下來的欄位（例如 `business.tax_id`、`business.company_address.*`） | seller `KYCApplication`、`IndividualKYC.id_card_no` | 本檔 |
| `packages/hawkr_api/lib/*.dart`（`account`、`dates`、`errors`、`field_mask`、`go_text`、`images`、`submit`、`taipei_time`、`testing`、`transport`） | 手寫執行期 | — | keep：`git grep -liE 'payuni|KYCFlow|idCard|bank|identityLocked|KYCField|Rebind|Reveal' -- packages/hawkr_api ':!packages/hawkr_api/lib/gen'` 只有上面那個測試 | — | — |
| `packages/hawkr_ui_text/lib/src/ui_text.g.dart`（24 筆）、`test/golden.g.json`（43）、`test/golden_calls.g.dart`（7）、`tool/enum_manifest.g.json:214-222`（`PayUniBindStatus`、`PayUniState`） | 生成物：`sellerKYCFlowStatus*`（Action、BannerTitle、BannerDescription、GateHeading、GateMessage）、`SellerPayUniState`、`sellerSellerPayUniFixFieldsHeading`、`sellerSellerPayUniReasonHeading`、`sellerKYCField` | generated | regenerates（backend `make uitext`，來源 `backend/internal/platform/uitext/catalog/`） | 目錄 | `test/golden_test.dart` 跟著生成物 |
| `packages/hawkr_ui_text/test/ui_text_test.dart:115-119` | 手寫測試：`KYCFlowStatus.KYC_FLOW_STATUS_APPROVED.action(...)`、`SUBMITTED.action(...)` | D15-flow-status | edit：隨 N1（`KYCFlowStatus` 的目錄鍵整組刪的話，這兩個斷言換別的 enum） | `SellerKYCFlowStatusUiText` | 本檔 |
| `packages/hawkr_ui_text/tool/check_enum_text.dart`、`lib/hawkr_ui_text.dart`、`test/check_enum_text_test.dart` | 手寫守門工具與 `UiLocale` | — | keep（同一條 grep 沒有命中） | — | — |

---

## 5. 使用者看得到的改變（草稿已同意的範圍之外要另外取得同意的）

草稿的涵蓋範圍：第 1 節（表單刪減）、第 2 節（兩區狀態頁，取代 `_LockedStatusView`）、第 3 節（admin 收款帳戶卡；「PayUni 的兩張卡在步驟 8 刪除」「身分與銀行的 KYC 卡（`KycIdentityCard` 等）步驟 7 不動，步驟 8 與欄位一起刪」）、第 4 節（條款）。

### admin 商家認證詳情頁（`admin/src/app/admin/merchants/[id]/page.tsx`）

| # | 會變的畫面 | 草稿有沒有涵蓋 |
|---|---|---|
| A1 | 刪「金流商店審核」卡（`PayUniStatusCard`）與「重新綁定」鈕 | 有：第 3 節「PayUni 的兩張卡在步驟 8 刪除」；D14 |
| A2 | 刪「店家目前的聯絡資料」卡（`PayUniContactCard`：聯絡人姓名、聯絡手機、聯絡電話、客服信箱、地址） | 刪卡有涵蓋（同上一句）；但刪掉之後聯絡人姓名與聯絡手機在這頁還有沒有地方顯示，草稿沒寫（`KycBasicInfoCard` 只有電子信箱與聯絡電話，`KycApplicationCards.tsx:131-134`）→ N6 |
| A3 | 刪身分資訊卡（`KycIdentityCard`：證件正反面、身分證字號） | 有：第 3 節最後一點 |
| A4 | 同一張卡上的姓名、英文姓名、出生年月日、發證日期／類型／地點也一起消失 | 「`KycIdentityCard` 等…與欄位一起刪」可以讀成涵蓋整張卡；但 D22 與步驟 8 的範圍只寫「身分證號、銀行帳號、證件」，沒有點名這六個欄位 → N3 |
| A5 | 檢附文件卡：刪存摺影像與「銀行名稱（代碼）遮蔽帳號」一列 | 有（D22；「`KycIdentityCard` 等」的「等」，**推論**） |
| A6 | 「顯示全文」列（鎖頭圖示、「身分證字號、銀行帳號與證件影像預設遮蔽。」、「顯示全文／重新顯示全文／收起」）、理由對話框、15 分鐘到期 | **沒有**。草稿沒有提到顯示全文；而保留的補充文件要靠它才看得到 → N2 |
| A7 | 補充文件的縮圖、「查看原圖」連結、放大預覽 | **沒有**。草稿第 1 節只說賣家端保留補充文件，沒說 admin 怎麼看 → N2 |
| A8 | 退回對話框裡「指定是哪一張文件」的選項（來自顯示全文） | **沒有** → N2 |
| A9 | 頁首的「金流商家編號：HAWK…」 | 沒有明寫；是 PayUni 註冊的欄位，註冊刪掉後沒有資料來源。屬於跟隨契約的刪除（**推論**），收款帳戶卡已顯示外部帳戶 |
| A10 | 核准鈕上方的小字「核准後系統會向 PayUni 註冊」（:1029）與核准成功提示（`APPROVE_SUCCESS_TEXT`） | **沒有**。要換成什麼字、或整句刪掉，是 UX 文案 → N5 |
| A11 | 審核備註「重點核對營業登記證與負責人身分證、統一編號是否一致，並確認所有證件皆在有效期內」（:1199-1205） | **沒有**。負責人身分證不再收，這句要改 → N5 |
| A12 | 頁首狀態徽章（`flowBadge(kycSummary.flowStatus)`，:820）不再有「綁定中、綁定失敗、已轉送、PayUni 審核中、待補件、PayUni 拒絕」 | 部分：D15 把狀態拆成兩條，但草稿只畫了賣家端；admin 頁首與清單徽章改讀什麼沒寫 → N1 |
| A13 | 審核紀錄裡動作「重新綁定 PayUni」的舊紀錄 | **沒有** → N4 |

### admin 其他頁

| # | 會變的畫面 | 草稿有沒有涵蓋 |
|---|---|---|
| A14 | 商家清單的篩選分頁「待重新綁定」消失（`store-search.ts:31`） | **沒有** → N10 |
| A15 | 商家清單每列的狀態徽章（`store-row.ts:90`） | 同 A12 → N1 |
| A16 | 審核待辦的種類篩選少一項「重新綁定 PayUni」（`queue-item.ts:23`） | 有：D14；草稿第 3 節已把連結改到 `#payment-account` |

### seller-app

| # | 會變的畫面 | 草稿有沒有涵蓋 |
|---|---|---|
| S1 | 確認送出頁的小字「送出後商店名稱與商店介紹將無法修改，請先確認無誤」消失（`kyc_wizard_steps.dart:589-598`） | 草稿沒有提到這句；D17 決定刪鎖，鎖不存在後這句是錯的，屬於跟隨決定的刪除（**推論**） |
| S2 | 設定頁店名與商店介紹列的「資料已送至 PayUni，店名與商店介紹無法修改，如需變更請聯繫 PayUni」消失，兩列恢復可編輯（`profile_screen.dart:434`） | 同上（D17） |
| S3 | 公司資訊步驟的統編、公司名稱、販售類型不再因鎖定變成唯讀，自動帶入也不再被關掉（`kyc_wizard_steps.dart:164-255`） | 同上（D17） |
| S4 | 客服電子信箱欄位的提示「PayUni 會寄金流相關通知到這個信箱」（`store_form.dart:32`） | **沒有** → N7 |
| S5 | 被鎖定時不能「捨棄草稿」的限制消失（`kyc_form.dart:408-411`） | 同 S1（D17） |
| S6 | 退回提示的標題文字（目前取 `KYCFlowStatus.REJECTED` 的目錄文字，`seller_certification_upload_screen.dart:597`） | 草稿第 2 節的平台審核表列了 REJECTED「需修改」；來源鍵要換 → N1（文字相同就不是可見改變） |

buyer-app：沒有可見改變。

---

## 6. 計畫沒有決定的事（只列選項與證據，不做決定）

- **N1 `KYCFlow` 與 `KYCFlowStatus` 的去留。** 步驟 8 寫「刪 `KYCFlowStatus` 的 PayUni 值與 BINDING／BIND_FAILED／FORWARDED／UNDER_REVIEW／PENDING_INFO」（計畫 255 行），照字面會留下 NOT_STARTED、DRAFT、SUBMITTED、REJECTED、APPROVED 五個值，和 `KYCStatus` 重疊；進度表另寫「步驟 8 刪 flow 時」（339 行），讀起來是整個 `KYCFlow` 刪掉。`KYCFlow` 還有 `can_resubmit`、`resubmit_unavailabilities`、`identity_locked`、`payuni`（seller.proto:875-888）。
  - 選項 A：整個 `KYCFlow` 與 `KYCFlowStatus` 刪掉，client 一律讀 `KYCApplication.status`；`can_resubmit` 要有新的家（直接放在 `KYCApplication`，或 client 由狀態推導）。
  - 選項 B：留五個值的 `KYCFlowStatus` 與 `can_resubmit`，只刪 `identity_locked`、`payuni`。
  - 牽動：`seller-app/lib/features/seller/data/kyc_form.dart:403-404`（`kycEditable` 讀 `flow.canResubmit`）、`current_kyc.dart:28-38`、`kyc_flow.dart` 整檔、`admin/src/app/admin/merchants/store-row.ts:24-90`（清單徽章）、`[id]/page.tsx:820`（頁首徽章）、admin proto `StoreListItem.kyc_flow_status`、`KYCSummary.flow_status`、目錄的 `sellerKYCFlowStatus*` 六組鍵、`KYCResubmitUnavailableReason` 的 `PAYUNI_*` 值（seller.proto:856-862）。
- **N2 admin 怎麼看保留下來的補充文件。** 現況 `view_uri` 與 `asset_id` 只在 `RevealKYCSensitive` 的回應（proto:367-395、:714-720）；步驟 8 刪「證件檔參照」與個資全文，但沒說 `RevealKYCSensitive`、`ListKYCReveals`、`ListKYCDocumentAccesses`、`KYCDocumentView` 與 `/api/files/admin/seller/kyc_applications/…` 的去留。
  - 選項 A：保留顯示全文機制，只把它縮成「看補充文件」（理由必填、15 分鐘、稽核照舊）；文字「身分證字號、銀行帳號與證件影像預設遮蔽」要改。
  - 選項 B：補充文件不是個資，`GetKYCApplication` 直接回 `view_uri` 與 `asset_id`，刪掉 reveal 對話框、`reveal-state.ts` 與稽核 RPC。
  - 選項 C：admin 不顯示補充文件影像（那 D24 Q2 保留它的理由「市集審核用」就不成立）。
  - 牽動：`KycApplicationCards.tsx:62-100、:251-320、:490-530`、`RevealKycDialog.tsx`、`reveal-state.ts`、`kyc-application.ts:323-352、:545-642`、`RejectKycDialog.tsx:37`、`page.tsx:226-228、:481-537、:708-723、:1246-1252、:1549-1566`、`tests/merchant-reveal-state.test.ts`。
- **N3 D22 沒點名的身分欄位：`real_name`、`english_name`、`birth_date`、`id_card_issue_date/type/location`、`bank_name`。** 7a 已把 `real_name`、`english_name`、`bank_name` 從送出必填拿掉（計畫 7a 範圍）；seller-app 表單已不送；admin 仍顯示（`KycApplicationCards.tsx:396-417、:443`），proto 欄位仍在（admin seller.proto:549-555，`KYCApplicationSummary.real_name` :614；`KYCField` 的 `REAL_NAME`、`BIRTH_DATE`、`ID_CARD_ISSUE_*`、`ENGLISH_NAME`、`BANK_NAME` 值 seller.proto:675-681）。
  - 選項 A：全部在步驟 8 一起刪（平台完全不存自然人資料）。
  - 選項 B：`real_name`（企業是負責人）留作市集登記資料，其餘刪。
  - 牽動：admin 的 `KycIdentityCard` 是整張刪還是留一部分；`issuable_fields` 的內容；`packages/hawkr_api/test/field_mask_test.dart:54` 的 `birth_date.year`。
- **N4 `KYC_REVIEW_ACTION_REBIND` 的舊審核紀錄。** `kyc-application.ts:512-513` 顯示「重新綁定 PayUni」；D11 對 `PAYUNI_TERMS` 的處理是「不轉換舊紀錄」，但審核紀錄沒有對應的決定。選項：刪 enum 值並用 migration 清掉舊列（未上線，只有本機種子）；或保留值只供顯示。
- **N5 admin 的兩句審核文案與核准成功提示。** `page.tsx:1029`「核准後系統會向 PayUni 註冊」、`:1199-1205` 的審核備註（提到負責人身分證與證件有效期）、`kyc-application.ts:57-58` 的 `APPROVE_SUCCESS_TEXT`。選項：改成中立說法（「核准後系統會建立收款帳戶，賣家再到金流商完成設定」之類）、或刪掉；字要 Will 同意（repo 規則：UX 文案先取得同意）。
- **N6 「店家目前的聯絡資料」卡刪掉後的聯絡欄位。** `PayUniCards.tsx:153-166` 顯示聯絡人姓名、聯絡手機、聯絡電話、客服信箱、地址；刪卡後前兩項在詳情頁沒有別處顯示（`KycBasicInfoCard` 只有信箱與電話，`KycApplicationCards.tsx:131-134`；`KycOtherInfoCard` 有送貨地址 :562）。選項：併進基本資料卡；或直接刪（admin 還能從店家資料頁看，沒有查證）。
- **N7 seller-app 客服信箱的提示文字。** `store_form.dart:32`「PayUni 會寄金流相關通知到這個信箱」。D23 的修訂把收款聯絡信箱改成帳號層級的聯絡信箱，客服信箱不再給金流商用。選項：刪 hint；或改成描述它現在的用途。`store_editor_test.dart:192` 逐字比對。
- **N8 seller-app 推播類型 `MERCHANT.VERIFICATION.*`、`MERCHANT_VERIFICATION`、`CERTIFICATION_*`、`SELLER_REVIEW_STATUS`。** `seller_status_push.dart:9-17`。步驟 8 刪 `sellerrelay` 的 `merchant.verification.*` 與 `sellerconsumer` 的 PayUni 審核通知（D18），D 類只點名 Java 推播 `PAYUNI_STATUS`。選項：只留 `PAYUNI_STATUS`；或整組留到 B 階段（A8 的通知需求清單會加「賣家收款開通結果的通知」，計畫步驟 9）。哪些類型 Java 還會發，要看 backend 的盤點。
- **N9 `seller-app/docs/design/SELLER_APP_PRODUCT_UX_MASTER_SPEC.md` 的 PayUni 段落**（:93、:113、:390-395、:461、:516、:536）。步驟 8 的允許清單只管單體與 `api/hawkr`；client 文件要不要同步沒寫。README（:236、:275）描述的 `payuni.dart` 與「PayUNi 審核狀態面板」已經不存在，是事實錯誤。
- **N10 admin 商家清單的「待重新綁定」分頁。** `store-search.ts:19、:31、:69-70`。選項：直接刪（建立失敗的店家從審核待辦 `PAYMENT_ACCOUNT_CREATE_FAILED` 進來）；或換成「收款帳戶建立失敗」分頁（要 `ListStoresRequest` 新增篩選欄位，是新機制）。
- **N11 `LinkedIdentity` 的 `NATIONAL_ID`、`RESPONSIBLE_PERSON_ID`、`BANK_ACCOUNT` 與 `shares_bank_account_with_sanctioned_account`。** 步驟 8 刪 `kyc_identity_hashes` 的三個種類，但 `account_identity_hashes` 的舊列保留到 5 年清理；admin `measure.ts:180-183` 的標籤要不要留、`BANK_ACCOUNT` 是否改指 fingerprint（步驟 6 已做的範圍，沒查證）沒有寫在步驟 8。
- **N12 admin `StoreActionUnavailableReason` 的 `PAYUNI_MERCHANT_TERMINAL`、`PAYUNI_BIND_IN_PROGRESS`、`PAYUNI_BOUND`**（seller.proto:132-137，`unavailability.ts:42-44`）。步驟 8 的清單沒有點名；**推論**隨 `RebindPayUni` 一起刪。另有「註冊所根據的申請不是已核准」一值（proto:138 之後）同樣只服務重新綁定。
- **N13 seller-app `tool/coverage_baseline.json` 的 6 個已不存在的檔。** 是 7c 留下的，不是步驟 8 造成的；重算基準會同時帶進這個修正。要不要和步驟 8 的刪除放同一個 commit 沒有規定。
- **N14 R13 是什麼**：見第 0 節第 3 點，需要 7c 的審查 ledger 確認。

---

## 7. 數量與指令

| app | delete（檔） | edit（檔） | keep：D 類 | naming-only | generated |
|---|---|---|---|---|---|
| seller-app lib | 1（`kyc_flow.dart`，隨 N1） | 9（`current_kyc`、`kyc_form`、`kyc_wizard`、`kyc_wizard_steps`、`seller_certification_upload_screen`、`store_form`、`profile_form_widgets`、`profile_screen`、`media_model` 註解）＋2 個只改註解（`kyc_store_contact`、`certification_status_view`） | 17（含 `code_meta.dart` 與通知 4 檔） | 36 | `current_kyc.g.dart` 重新生成 |
| seller-app test／tool／docs | 2（`kyc_flow_test.dart`、`profile_payuni_locked_test.dart`） | test 11、`tool/coverage_baseline.json` 1、docs 3–4（README、AGENTS、ARCHITECTURE；UX spec 隨 N9） | 20（24 個命中檔扣掉 naming-only 4；其中只有上面 13 檔與步驟 8 有關） | 4 | — |
| buyer-app | 0 | 0 | lib 17、test 1 | lib 16、test 1 | `code_meta.dart`、`*.freezed.dart` 算在 D 類 |
| admin src | 1 確定（`PayUniCards.tsx`）＋2 隨 N2（`RevealKycDialog.tsx`、`reveal-state.ts`） | 11（`KycApplicationCards.tsx`、`kyc-application.ts`、`page.tsx`、`use-store-detail.ts`、`RejectKycDialog.tsx`、`store-row.ts`、`merchants/page.tsx`、`store-search.ts`、`queue-item.ts`、`unavailability.ts`、CSS）＋1 註解（`kyc-status.ts`） | 1（`codemeta/generated.ts`）＋發票相關不計 | 2（`disputes/detail.ts`、`posts/adminList.ts`） | — |
| admin tests／docs | 0–1（`merchant-reveal-state.test.ts` 隨 N2） | 7（`merchant-kyc-application`、`merchant-kyc-issues`、`merchant-store-row`、`merchant-store-list`、`review-queue-page`、`unavailability`、`merchant-payment-account-card`）＋README 1 | 4 | 2 | — |
| Dart 套件 | 0 | 2 手寫測試（`hawkr_api/test/field_mask_test.dart`、`hawkr_ui_text/test/ui_text_test.dart`） | — | — | `hawkr_api/lib/gen/**`（9 個命中檔）、`hawkr_ui_text` 的 4 個 `.g.*` |

指令（都在 worktree 根目錄執行）：

```
# 起點清單（seller-app 93、buyer-app 35、admin 27、hawkr_ui_text 4、hawkr_api 9 檔）
git grep -l -i payuni -- seller-app | wc -l    # 其餘四個路徑同
# seller-app lib：features/seller 內 10 檔、之外 53 檔
git grep -l -i payuni -- seller-app/lib/features/seller | wc -l
git grep -l -i payuni -- seller-app/lib ':!seller-app/lib/features/seller' | wc -l
# 步驟 8 的符號（lib 10 檔、test 13 檔）
git grep -l -E 'identityLocked|KYCFlowStatus|KycFlow|lockedByPayuni|kPayuni|canResubmit|hasFlow\(\)' -- seller-app/lib ':!*.g.dart'
git grep -l -E 'identityLocked|KYCFlowStatus|KycFlow|lockedByPayuni|kPayuni|canResubmit|PAYUNI_TERMS|SellerPayUni' -- seller-app/test
# naming-only：一個檔的每個 payuni 命中都符合 backend-payuni|PayUni #NNN|payuni_sale_price
#   seller-app/lib 63 中 36、buyer-app/lib 33 中 16、admin/src 15 中 2、seller-app/test 24 中 4、buyer-app/test 2 中 1、admin/tests 10 中 2
# buyer-app 沒有 KYC／PayUni 開通的符號（只命中 2 個無關的檔）
git grep -c -E 'KYCFlow|KYCStatus|KYCField|PayUniRemedy|PayUniBindStatus|PayUniState|SellerPayUni|PAYUNI_TERMS|AgreementKind|identityLocked|idCard|IdCard|bankAccount|bankCode|bankBook|getMyKYC|PAYUNI_BIND|Rebind|Reveal' -- buyer-app/lib buyer-app/test ':!buyer-app/lib/core/codemeta'
# admin 的符號（16 檔）
git grep -l -E 'PayUni|payuni|Reveal|reveal|KYCFlowStatus|flowStatus|idCard|bankAccount|BANK_BOOK|ID_CARD|PAYUNI' -- admin/src ':!admin/src/lib/codemeta' ':!admin/src/features/invoices' ':!admin/src/app/admin/invoices'
# admin 實際呼叫的舊 RPC
git grep -n -i -E 'listKYCReveals|listKYCDocumentAccesses|listPayUniObservations|refreshPayUniStatus|getPayUniRegistration|rebindPayUni|revealKYCSensitive' -- admin/src
# 套件的手寫部分
git grep -l -i -E 'payuni|KYCFlow|idCard|bank|identityLocked|KYCField|Rebind|Reveal' -- packages/hawkr_ui_text packages/hawkr_api ':!packages/hawkr_api/lib/gen'
# coverage baseline 裡已不存在的檔（6 個）
cd seller-app && for f in $(python3 -c "import json;print(' '.join(json.load(open('tool/coverage_baseline.json'))['instrumentedFiles']))"); do [ -e $f ] || echo STALE $f; done
```

## 8. 沒有涵蓋到的

- 沒有逐行讀完：`admin/.../page.tsx`（只讀命中行與 976-1034、1188-1252）、`kyc-application.ts`（只讀大綱與 205-243、305-353、500-518）、`RejectKycDialog.tsx`、`RevealKycDialog.tsx`、`reveal-state.ts`、`PayUniCards.tsx`（只讀文字行）、seller-app 的各測試檔內容（只看命中行）。
- 沒有查：`admin/src/app/globals.css` 裡與 `MerchantCertificationDetail.css` 同名的 `.id-card-*`、`.document-*`、`.payment-*` 規則是否重複定義；`PayuniMerchantVerificationStatus`（CodeMeta）在 admin 有沒有使用者；`tool/coverage_ratchet.dart` 對已不存在的檔的行為；admin 的 e2e 或 Playwright 測試（如果有）。
- seller-app 與 buyer-app 的 D 類檔只依「打哪個後端」分類（REST 路徑的字串或 CodeMeta），沒有逐檔讀。
- website、backend、`api/` 的 proto 本身、`packages/hawkr-api-ts`、`packages/hawkr-ui-text-ts` 不在範圍。
- `seller.payuni_*` 權限、admin BFF 的路由設定沒有查（`admin/src` 搜不到權限字串，沒有另外驗證）。
