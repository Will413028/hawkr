# L0 文案草稿（供 Will 核可）

**Will 的決定（2026-10-17）：全照建議（D1–D7）。** 用語結果收進術語表 `docs/開發規範/使用者面用語術語表.md`；proto 與程式註解的三個詞在 L′ 與契約一起改。

基準：`<10e worktree>` HEAD `fe68fab21`。只讀盤點，未改任何 repo 檔。
規則：只在「漂移」或「Q5 用語」需要時改字；其餘文字原樣搬進目錄。測試檔、`gen/`、CodeMeta 生成檔（凍結）不計入「要改的字串」。
受眾 key：`default`（賣家／買家面）、`staff`（admin）。W = website、S = seller-app、B = buyer-app、A = admin。

重新掃描方法（可重跑）：（掃描腳本未入庫） 列出 59 個 proto enum 在四端「含中文字的非測試檔」的引用，再用 （掃描腳本未入庫） 抽出 enum 值到文字的對照；（掃描腳本未入庫） 抽出非註解行的 處置／申訴／補正。結果落在 （掃描腳本未入庫）、`rows.json`、`term.json`。

---

## A. 同一個 enum 值、不同端文字不同

重掃結論：**只有 4 個 enum 在不同端文字不同**（KYCField、KYCFlowStatus、MeasureRequestState、兩個同名的 FoodRegistrationState 之間的賣家／後台差異），另有 RevokeReason 與 MeasureBasis 是「範圍不同」（不是文字衝突）。其餘三端共用的 enum 文字**逐字相同**，不用改：ViolationCategory（7）、ViolationSeverity（4）、RiskGround（8）、MeasureKind（3）、TerminationGround（11）、MeasureState（5，ACTIVE 依 kind 選「停權中」或「合作已終止」）、MeasureRequestKind（兩值，Q5 例外見下）、StoreField（5，W `lib/seller/store-contact.ts:54-58` = S `kyc_fields.dart:163-167`）、SellerRequirement 的 STORE_NAME／LOGO／TOWN_CODE（W `onboardingTasks.ts:57-59` = S `onboarding_tasks.dart:113-115`）。

### A1 KYCField（22 值，10 值不同；12 值相同不列）

分類：**全部是漂移**（同一個人看同一欄位，admin 與賣家端措辭不同；賣家端 W 與 S 完全相同）。
A = `admin/src/app/admin/merchants/[id]/kyc-application.ts`；W = `website/lib/seller/kyc.ts`；S = `seller-app/lib/features/seller/data/kyc_fields.dart`。

| 值 | A | W | S | 提議的單一文字 | 理由 |
|---|---|---|---|---|---|
| REAL_NAME | 姓名 :211 | 真實姓名 :444 | 真實姓名 :73 | **真實姓名** | 隱私頁對賣家寫「真實姓名」（`PrivacyPage.tsx:74`），與「姓名」區分顯示名稱 |
| BIRTH_DATE | 出生年月日 :212 | 出生日期 :445 | 出生日期 :74 | **出生日期** | 隱私頁同寫「出生日期」；較短 |
| BANK_NAME | 銀行名稱 :217 | 銀行代碼 / 金融機構 :460 | 同 W :82 | **銀行名稱** | 兩個值各有名詞才精確（賣家端兩值同文字，`missingItemsOf` 靠文字去重 `kyc.ts:531-543`，要改成依表單欄位去重） |
| BANK_CODE | 銀行代碼 :218 | 銀行代碼 / 金融機構 :464 | 同 W :83 | **銀行代碼** | 同上 |
| PRODUCT_TYPE | 主要商品類型 :220 | 主要產品類型 :469 | 主要產品類型 :85 | **主要商品類型** | 賣家條款「商品」61 次、「產品」5 次（`seller-terms.md`） |
| WEBSITE_URL | 官網 :221 | 官方網站 :472 | 官方網站 :86 | **官方網站** | 較正式，賣家表單同用 |
| ID_CARD_FRONT | 身分證（正面） :222 | 身分證正面照 :474 | 身分證正面照 :87 | **身分證正面照** | 這是上傳影像的欄位，「照」點出類型 |
| ID_CARD_BACK | 身分證（反面） :223 | 身分證反面照 :478 | 身分證反面照 :88 | **身分證反面照** | 同上 |
| BUSINESS_REP_ID_CARD_NO | 負責人身分證字號 :230 | 身分證字號 :503 | 身分證字號 :108 | **負責人身分證字號** | 與個人的 INDIVIDUAL_ID_CARD_NO 區分；隱私頁對賣家也寫「負責人身分證字號」 |
| BUSINESS_COMPANY_DOCUMENTS | 公司文件 :231 | 補充文件 :507 | 補充文件 :112 | **補充文件** | 賣家看到的步驟標題就是「5. 補充文件」（`CertificationsClient.tsx:162`），選填；「公司文件」會讓人以為必填 |

KYCField 不需要 `staff` key（無視角差異）。

### A2 KYCFlowStatus（12 值，含 UNSPECIFIED）

A = `admin/src/app/admin/merchants/store-row.ts:49-88`；W = `website/lib/seller/kyc-flow.ts:147-197`；S = `seller-app/lib/features/seller/data/kyc_flow.dart:83-113`。

| 值 | A（staff 現況） | W／S（default 現況） | 分類 | default | staff |
|---|---|---|---|---|---|
| UNSPECIFIED | 無 | 無 | — | 無文字（Q4） | 無文字 |
| NOT_STARTED | 無（`flowBadge` 回 undefined） | 待認證 W:153 S:84 | 補缺（staff 要完整） | 待認證 | **尚未申請**（對齊 `food-registration.ts:107` KYCStatus 的「尚未申請」） |
| DRAFT | 填寫中 :51 | 草稿中 W:156 S:85 | **漂移** | **填寫中** | 填寫中 |
| SUBMITTED | 待審核 :52 | 平台審核中 W:158 S:86 | 視角 | 平台審核中 | 待審核 |
| REJECTED | 已退回 :56 | 未通過 W:161 S:87 | 視角（見決定 D2） | 未通過 | 已退回 |
| BINDING | PayUni 綁定中 :60 | PayUni 審核中 W:163 S:88 | 視角（刻意粗細） | PayUni 審核中 | PayUni 綁定中 |
| BIND_FAILED | 待重新綁定 :64 | PayUni 建檔失敗，處理中 W:167 S:89-91 | 視角 | PayUni 建檔失敗，處理中 | 待重新綁定 |
| FORWARDED | PayUni 審核中 :68 | PayUni 審核中 W:171 S:92 | 相同 | PayUni 審核中 | PayUni 審核中 |
| UNDER_REVIEW | PayUni 審核中 :72 | PayUni 審核中 W:175 S:93 | 相同 | PayUni 審核中 | PayUni 審核中 |
| PENDING_INFO | PayUni 待補件 :76 | 需補件 W:179 S:95 | 視角（staff 列表混有平台與 PayUni 的件，要前綴） | 需補件 | PayUni 待補件 |
| APPROVED | 已核准 :80 | 已通過 W:183 S:104 | 視角（見決定 D2） | 已通過 | 已核准 |
| PAYUNI_REJECTED | PayUni 審核未通過 :84 | PayUni 審核未通過 W:194 S:112 | 相同 | PayUni 審核未通過 | PayUni 審核未通過 |

L-brief 把「已核准／已通過、已退回／未通過」歸為漂移。我的重掃看法不同：admin 其他地方也一致用「核准／退回」（`food-registration.ts:42-43` 食品登錄、`:110-111` KYCStatus），賣家端一致用「通過／未通過」（FoodRegSupplement 「未通過：…」`FoodRegSupplement.tsx:~226`），是**審核者動詞對當事人結果**的穩定模式。若硬收斂成一份，admin 的食品登錄與 KYCStatus 會跟同畫面的其他徽章不一致。所以建議用 `staff` key 保留（決定 D2）。

另一個後台內部不一致：admin 的 KYCStatus 把 SUBMITTED 寫成「審核中」（`food-registration.ts:109`），KYCFlowStatus 的 SUBMITTED 寫「待審核」（`store-row.ts:54`）。KYCStatus 只有 admin 一端用文字，不進目錄；順手對齊成「待審核」即可（不屬必要變更）。另 PayUniOutcome 的 admin 卡片（`kyc-application.ts:1021-1029`）與 KYCFlowStatus staff 文字重複（已核准／PayUni 審核未通過／PayUni 待補件／PayUni 審核中），那是 BindStatus×Outcome 的組合，只有 admin 使用，不進目錄。

### A3 MeasureRequestState（5 值）與 MeasureRequestKind

A = `admin/src/app/admin/measures/measure-inbox.ts:70-104`；W = `website/features/seller/measures/measureDisplay.ts:210-285,324`；S = `seller-app/lib/features/seller/data/measure_display.dart:186-214`。分類：**視角**（裁定者對請求人）。

| 值 | A（staff 現況） | W／S（default 現況） | default | staff |
|---|---|---|---|---|
| UNSPECIFIED | 無 | 無 | 無 | 無 |
| PENDING | 待裁定 :83 | 審核中（`{kind}審核中` W:274 S:191） | 審核中 | 待裁定 |
| ACCEPTED | 成立 :84 | 已通過（W:281 S:195） | 已通過 | 成立 |
| REJECTED | 駁回 :85 | 已駁回（W:283 S:199） | 已駁回 | 駁回 |
| CLOSED | 已結案 :86 | 無（W:`measureDisplay.ts:~285` 與 S `requestChip` 回 null，不顯示） | **已結案（新增，補完整）** | 已結案 |

MeasureRequestKind：APPEAL 現況全端「申訴」（A `measure-inbox.ts:75`、W `measureDisplay.ts:212`、S `measure_display.dart:188`），**依 Q5 改成「申覆」**；REMEDIATION 全端「解除請求」不變。兩個 key（default／staff）同字。
組合句（進目錄、用 `select`）：`{kind}審核中／已通過／已駁回`、`已提交{kind}，請等候平台複核。`（W:324 = S:213，相同）、`平台回覆：{reason}`（W:327 = S:216，相同）。admin 的 `requestStateText`（`measure-inbox.ts:96-104`）是 `${kind}${state}`，staff 也用 select 組成「申覆待裁定」「解除請求成立」。

### A4 同名但不同 package 的 FoodRegistrationState

賣家 enum（`api/hawkr/seller/v1alpha1/seller.proto:363`，5 值）與後台 enum（`admin/seller/v1alpha1/seller.proto:755`，6 值，多 WITHDRAWN）是兩個 descriptor，key 不同。
A = `admin/src/lib/food-registration.ts:39-46`；W = `website/features/seller-center/components/FoodRegSupplement.tsx`（內嵌句，「審核中：…」~:205、「未通過：…」~:226）；S = `seller-app/lib/features/wallet/presentation/food_registration_form.dart:109`（「審核中：…」）。分類：視角（同 A2 的動詞模式）。

| 值 | 賣家 enum default（現況） | 後台 enum staff（現況，A） |
|---|---|---|
| UNSPECIFIED | 無 | **現況寫「未知」:40，依 Q4 刪除，改走共用「未知」**（見 D.3） |
| PENDING | 審核中（W／S 內嵌） | 待審核 :41 |
| APPROVED | 已通過（**新增**，賣家端目前不顯示狀態名） | 已核准 :42 |
| REJECTED | 未通過（W 內嵌） | 已退回 :43 |
| WITHDRAWN | （賣家 enum 無此值） | 已撤回 :44 |
| ENDED | 已結束（**新增**） | 已結束 :45 |

### A5 範圍不同、不是文字衝突

- RevokeReason：A `merchants/[id]/measure.ts:298-303` 六值皆有文字；W `measureDisplay.ts:80-83`／S `measure_display.dart:92-93` 只有 FINDING_REVOKED「依據的認定已撤銷」、SOURCE_REVOKED「關聯的終止已撤銷」（兩端與 A 同字），其餘四值賣家端刻意不說明。Q4 要求 enum 完整：default 補 APPEAL_ACCEPTED（**申覆成立**，Q5）、ADMIN_CORRECTION 平台更正、EXTERNAL_DECISION 主管機關或法院的決定、OTHER 其他，取 admin 現有字。「哪幾個值要對賣家顯示」維持 client 規則（`REVOKE_NOTE` 的子集邏輯），不進目錄。
- MeasureBasis：A `measure.ts:104-106` 有 RISK「風險」、VIOLATION「違規認定」、CONTRACT「契約條款」；W／S 只有 CONTRACT（`measureDisplay.ts:56`、`measure_display.dart:60`），標題另由「風險：{類型}」與認定類別組成。default 補 RISK、VIOLATION 兩個字，取 admin。
- 純 admin 的 enum（AppType `app-versions/page.tsx:32-34`、LinkedIdentity、FoodRegistrationEndCause、KYCIssueCode、KYCStatus、AdminRevokeReason…）只有一端用文字，依 Q3 不進目錄。AppType 的 UNSPECIFIED「未指定」（:32）、KYCStatus 的 UNSPECIFIED「尚未申請」（`food-registration.ts:107`）是 Q4 要拿掉的 UNSPECIFIED 文字，請一併處理。
- Gender、AddressLabel：W 與 B 的顯示文字相同（男性／女性／其他：`ProfileClient.tsx:193-195` = `buyer_profile_fields.dart:43-45`；家／公司／其他），但兩端都還用字串 wire 值，不是 proto enum 的對照表；等搬到 proto enum 時再進目錄，這次不動。
- ReportStatus（通報狀態）是 CodeMeta 凍結的舊契約：A `codemeta/generated.ts:630-637`（待處理／已隱藏／已警告…）與 S `report_model.dart:92-97`、B `report_target_type.dart:50-55`（審核中／已處理（已隱藏）…）文字不同，且含「申訴中」；隨舊模組搬遷時處理，不在 L0。
- 伺服器擁有文字的 enum（`*UnavailableReason`、ErrorReason）依 Q4 規則排除；但 W `lib/seller/measures.ts:190-193,236-240`、S `measure_display.dart:100-106,235-238` 與 `measures.dart:28-33` 目前仍有手寫句，含 Q5 要改的字，列在 B。

---

## B. Q5 用語變更清單

規則：對賣家的「處置」→「措施」；賣家對平台決定提出的「申訴」→「申覆」（請求種類）；「補正」（使用者看得到時）→「解除請求」（指請求補正後解除）或「修改」（指資料）；導覽名稱「處置與申訴」→「**措施與申覆**」。「解除請求」本來就是條款用語，不動。「記錄／紀錄」：條款用「紀錄」15 次、「記錄」2 次，建議順手統一成「紀錄」（admin 已是「措施紀錄」）。

計數方式：`rg` 後排除註解行（`//`、`///`、`*`、`/*`）、測試、`gen/`、CodeMeta；單位是「含該詞的程式行」，一行同時含兩詞各算一次。精確重現：`python3 -I scratchpad/term.py`，明細在 （掃描腳本未入庫）。

### B1 摘要

| 範圍 | 處置 | 申訴→申覆 | 補正 | 備註 |
|---|---|---|---|---|
| website（措施相關檔） | 39 | 38 | 1（+2 發票用，見 B7） | 要改 |
| seller-app（措施相關檔） | 27 | 29 | 1 | 要改 |
| admin（措施相關檔） | 0 | 13 | 0 | 3a 新字串 |
| buyer-app | 0 | 0 | 0 | 沒有賣家措施；34 行「申訴」全是售後申訴（保留，見 B7） |
| Go 使用者可見句（seller 模組） | 1 | 14 | 7 | `model.go:120`、`measure/appeal.go`、`measure/remediation.go`、`model/measure.go:724-729`、`measure/revoke.go:32` |
| 合計 | 67 | 94 | 9 | |
| 不計入（需另行決定） | | | | `api/**/*.proto` 註解：處置 5、申訴 64、補正 29；程式註解；測試檔 website 31／seller-app 19／Go 14 個要隨改 |

另有 Go 內部字串 `httpapi/seller/declaration.go:88`（豁免理由，開發者可見，含「申訴」「補正」「處分」）與 `devseed.go:298`（dev seed 說明）：非使用者可見，建議一併改以免誤導，不計數。

### B2 導覽名稱與頁面標題「處置與申訴」→「措施與申覆」

| 面 | 位置 | 現況 → 新 |
|---|---|---|
| W 導覽 | `features/seller-center/components/sellerNav.tsx:134` | 處置與申訴 → 措施與申覆 |
| W 頁標題 | `app/(seller)/seller/measures/page.tsx:3`、`[measureId]/page.tsx:3`；`SellerMeasuresPage.tsx:37`、`SellerMeasureDetailPage.tsx:24` | 處置與申訴 → 措施與申覆 |
| W 錯誤頁 | `app/(seller)/seller/measures/error.tsx:19,20,22` | 「處置與申訴載入時出了點狀況」「…或回到處置與申訴。」「回處置與申訴」 → 措施與申覆 |
| W 入口 | `MeasuresAccountEntry.tsx:25`、`TerminationNoticeBanner.tsx:30`、`SellerOnboardingClient.tsx:541`（連結）、`SellerSuspendedNotice.tsx:49,272` | 處置與申訴／查看全部處置與申訴 → 措施與申覆／查看全部措施與申覆 |
| W 常數 | `measureDisplay.ts:394`（`MEASURE_NOT_FOUND_ACTION`）、`SellerMeasureRequestPage.tsx:49` | 回到處置與申訴／請回到處置與申訴查看目前狀態。 → 措施與申覆 |
| S 導覽 | `seller_appeal_screen.dart:68`、`profile_other_options_card.dart:90`（描述 :91「查看平台的處置、期間與申訴進度」→「查看平台的措施、期間與申覆進度」）、`seller_info_screen.dart:224`、`suspension_notice_screen.dart:41`、`measure_display.dart:14`、`seller_measure_request_form_screen.dart:36` | 同上 |
| A 導覽 | `admin/src/components/Layout/Sidebar.tsx:60`、`measures/page.tsx:95`（頁標題） | 措施與申訴 → 措施與申覆 |
| 伺服器句 | `backend/internal/modules/seller/internal/model/model.go:120` | 見 B6 |

### B3 website（措施相關檔，處置 39／申訴 38／補正 1）

代表例（舊 → 新）：
- 頁面標題：`SellerMeasureRequestPage.tsx:64` 提出申訴 → 提出申覆；`app/(seller)/seller/measures/[measureId]/appeal/page.tsx:3` metadata 提出申訴 → 提出申覆。
- 按鈕：`AppealEntry.tsx:9` 提出申訴 → 提出申覆；`SellerMeasureRequestPage.tsx:67,68` 送出申訴／申訴送出中… → 送出申覆／申覆送出中…。
- 表單：`:65,69` 申訴理由／申訴理由最多 N 字 → 申覆理由…；`:66` 請說明此次處置有誤的理由… → 請說明此次措施有誤的理由…；`:48` 申訴已送出，請等候審核 → 申覆已送出，請等候審核。
- 副標與空狀態：`SellerMeasuresPage.tsx:38` 查看平台對你商店的處置與處理進度，符合條件的處置可提出申訴。 → 查看平台對你商店的措施與處理進度，符合條件的措施可提出申覆。；`:56` 目前沒有處置記錄 → 目前沒有措施紀錄；`:69` aria 處置記錄。
- 顯示常數 `measureDisplay.ts`：:131,140 處置記錄、:135 風險處置、:138、:159 處置：{kind}、:212 申訴、:351 …可提出申訴、:366 申訴期限、:389 無法載入處置記錄、:391 找不到這項處置、:393 這項處置不存在…。
- `lib/seller/measures.ts`：:173–176 這項處置已更新…送出申訴／已經提出申訴／已撤銷，不需要申訴／目前不能提出申訴；:180–181 這項處置…請求解除；:191–192 已申訴過／申訴期限已過；:239 申訴審核中。
- 其他：`SellerSuspendedNotice.tsx:48` 到處置與申訴查看處置內容與期間，並可對仍可申訴的處置提出申訴。；:56 …終止生效後 6 個月內申訴成立…；:196 生效中的處置；:341 查看處置；`SellerMeasureDetailPage.tsx:30` 查看全部處置；`SellerOnboardingClient.tsx:417` 你有處置記錄，查看與申訴 → 你有措施紀錄，查看與申覆；`lib/seller/onboarding.ts:23`（伺服器句的 client 抄本，見 B6）。
- 補正（user-facing）：`lib/seller/kyc-flow.ts:137` 認證未通過，請補正資料後重新送審。 → 認證未通過，請修改資料後重新送審。（這是 KycFlowCopy 的 gate 文字，隨 D.1 一起進目錄）

### B4 seller-app（措施相關檔，處置 27／申訴 29／補正 1）

與 website 逐項對應，另列 Dart 才有的：`measure_display.dart:118,121,123,125,141`（處置記錄／風險處置／處置：）、`:102,104` 已申訴過／申訴期限已過、`:188` `subject = … '申訴'`、`:213` 已提交申訴，請等候平台複核。、`:238` 申訴審核中、`:338` 申訴期限、`:359` …可提出申訴；`measures.dart:28-33` 六句 notice；`seller_appeal_screen.dart:26` 目前沒有處置記錄、`:222` **處置時間：$decided**（見 B8 問題 2）、`:270` 提出申訴；`seller_measure_request_form_screen.dart:41-65` 申訴表單七句；`seller_info_screen.dart:237` 你有處置紀錄，查看與申訴；`suspension_notice_screen.dart:35,40`；`kyc_flow.dart:80` 補正資料 → 修改資料。

### B5 admin（3a 的字串，申訴 13、處置 0）

- 導覽／標題：`Sidebar.tsx:60`、`measures/page.tsx:95` 措施與申訴 → 措施與申覆。
- 動作：`measure-actions.ts:327` 裁定申訴 → 裁定申覆；`measures/[id]/page.tsx:249` 裁定申訴 → 裁定申覆。
- 清單：`measure-inbox.ts:36` 沒有待裁定的申訴或解除請求 → 沒有待裁定的申覆或解除請求；`:75` `REQUEST_KIND_LABEL[APPEAL]` 申訴 → 申覆。
- 詳情：`MeasureSections.tsx:212` 申訴期限 → 申覆期限；`ActiveMeasuresCard.tsx:67` 申訴期限：- ；`measure.ts:290` 申訴期限：{…}、`:298` RevokeReason.APPEAL_ACCEPTED 申訴成立 → 申覆成立、`:556`（…撤銷或申訴成立…）、`:559`（賣家可對這筆單獨申訴）、`:578`（賣家可提出申訴）。
- 「解除請求」本來就是條款用語，admin 已統一，不動。

### B6 伺服器使用者可見句（Go，處置 1／申訴 14／補正 7）

| 位置 | 現況 → 新 |
|---|---|
| `modules/seller/internal/model/model.go:120`（ApplyStore 被終止擋下） | 「此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，請至「處置與申訴」提出申訴。」 → 「此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。」依 D-L2：伺服器句不寫客戶端導覽名稱（導向由 `unavailable_reasons` 的 enum 決定）。同一句的另外三份抄本：W `lib/seller/onboarding.ts:23`、S `seller_onboarding.dart:28`、`api/…/error_reason.proto:36,39`、`seller.proto:467-470` 註解；L 之後 client 讀 `LocalizedMessage`，W／S 抄本與 `STORE_OPENING_REFUSAL_TEXT` 整組應刪 |
| `measure/appeal.go:22,34` 這筆措施目前不能申訴；`:28` 這筆措施已申訴過；`:30` 終止合作的申訴期限已過；`:32` …有待處理的補正請求，審核完成後才能申訴；`:46` reasonLabel 申訴理由；`:170` 只有接受申訴時可以一併撤銷認定；`:189,201` 沒有待裁定的申訴 | 申訴 → 申覆（共 9 行）；`:32` 補正請求 → 解除請求 |
| `measure/remediation.go:20,35` 補正請求目前不能提出；`:26` 補正請求目前在冷卻期內，不能再提出；`:28` 只有依據風險類型的措施可以請求補正後解除；`:85,91` 沒有可以裁定的補正 | 補正請求 → 解除請求；`:28` → 只有依據風險類型的措施可以請求解除；`:85,91` → 沒有可以裁定的解除請求 |
| `model/measure.go:724,726` 賣家提出過申訴，或有待處理的解除請求，…；`:729` 申訴待裁定，請先裁定 | 申訴 → 申覆（admin 看到的 `MeasureActionUnavailability` 原因） |
| `measure/revoke.go:32` 撤銷原因不合法；接受申訴請以裁定申訴撤銷 | 接受申覆請以裁定申覆撤銷 |

### B7 需 Will 判斷的模糊情形（建議值標在括號）

1. **消費者申訴／售後申訴（保留「申訴」，英文 customer complaint）**：發票申訴（W `features/invoices/i18n.ts` 21 行、`lib/invoices/complaintStatus.ts` 5、`orderInvoiceState.ts` 2、seller-center invoices appeals 約 35 行、`SellerSuspendedNotice.tsx:87-88` 買家發票申訴、A `invoices/complaints/**` 約 12 行、`Sidebar.tsx:147`）；售後申訴（W `AfterSaleDetailClient.tsx` 18、`AfterSaleDecisionDrawer.tsx` 5、`lib/after-sales/status.ts`；S `appeal_deadline_card.dart`、`after_sale_expanded_section.dart`；B 34 行；A `disputes/**`）。條款第十條標題就是「取消、退換貨、退款及申訴」，所以保留。**風險**：同一個英文 appeal 會同時指「售後申訴（買家對賣家的處理結果向平台申訴）」與「措施申覆」；英文建議售後用 `dispute`、措施用 `appeal`（見 C）。
2. **seller-app 的「處置時間：{decided}」**（`seller_appeal_screen.dart:222`）：指措施的決定時間，建議「措施時間」，網頁沒有對應字串。
3. **「處置」在非治理意義**（建議不動，Q5 只退場「對賣家的平台措施」意義）：A `anomalies/page.tsx:371,372,1187`（Admin 處置／賣家處置，異常處理）、`invoices/page.tsx:276`、`invoices/issuances/new/page.tsx:504`（例外處置）、W `DelegatedOrderDetailPanel.tsx:288`（前往平台異常處置）。
4. **補正在發票領域**（W `TaxExtensionSection.tsx:77`、`selfIssueTerms.ts:47`「平台的資料補正期限，不是法定開票寬限期」）：不是解除請求。賣家條款 `seller-terms.md:50`（第 3 項）正文也用「補正方式」。建議：UI 改「補件期限」（對應 KYC 的補件），條款正文不在 L0 動。若要保留法律一致，也可不改；請 Will 選。
5. **法律文件頁**（W `TermsPage.tsx:241,260`、`UserPolicyPage.tsx:392,395,421`、`ReturnPolicyPage.tsx`）：買家／一般使用者的「被處置者…提出申訴」，與賣家條款（已寫申覆）不同文件；`TermsPage.tsx:241` 同一句已混用「被處置者…事證申覆」。屬法律審閱範圍，建議不在 L0 動，另案。
6. **舊 CodeMeta 通報狀態「申訴中」**（A／S／B／W 的 `codemeta` 生成檔、S `report_model.dart:93`、B `report_target_type.dart:51`）：凍結，隨模組搬遷。
7. **proto／文件註解**（api 64＋29＋5 行）：不是 UI 字串，但與契約同源；建議同一個提交一起改，或在 L0 明說延後。
8. **紀錄／記錄**：要不要順手統一成「紀錄」（條款用語）。

---

## C. 英文術語、品牌名、英文搜尋提示

### C1 術語表（zh → en）

| zh | 草案（i18n brief I7） | 我的提議 | 備註 |
|---|---|---|---|
| 措施 | restriction | **enforcement action**（導覽：Enforcement and appeals）；若 Will 偏好 DSA 用語則維持 restriction | 警告不是「限制」，restriction 蓋不住 MeasureKind.WARNING（INFERENCE，未查證各平台實際用語）。 |
| 申覆 | appeal | **appeal** | DSA 第 20 條 complaint-handling 即對平台決定提出異議（i18n brief 已查）。 |
| 解除請求 | request to lift | **request to lift** | 或 lift request；按鈕「Request to lift」。 |
| 消費者申訴 | customer complaint | **customer complaint**（發票）；售後 **dispute** | 避開與 appeal 撞名（B7-1）。 |
| 認定 | finding | **finding** | |
| 警告 | warning | **warning** | |
| 停權 | （未列） | **suspension** | |
| 終止合作 | termination | **termination**（UI：Partnership terminated，對應「合作已終止」） | |
| 預告終止 | （未列） | **notice of termination** | 對應 TerminationGround.NOTICE |
| 事由 | （未列） | **ground** | |
| 裁定／駁回／成立 | （未列） | decide／rejected／upheld（staff 才用） | admin 無英文，僅供對照 |
| 補件 | （未列） | **provide additional information** / "More information needed" | KYC 的 PENDING_INFO |
| 商家認證 | （未列） | **seller verification** | 條款稱「身分及資格審核」 |
| 平台 | （未列） | **Hawkr**（句中）或 the platform | |
| 代行 | 待定 | 見 C2 | |

### C2 「代行」英文候選

現況用法（W／S 共約 300 行含此詞，多半在註解與後台代行工作區）：「代行工作區」「代行已到期」「結束代行」「代行紀錄」「代行通知」，指平台人員（ADM）在賣家授權範圍內替賣家操作。

| 候選 | 優點 | 缺點 |
|---|---|---|
| A. **delegated operation / delegated session**（紀錄：delegation log） | 與程式內部命名一致（`delegated` 前綴）；IAM 領域慣用；精確 | 對賣家稍技術性 |
| B. **acting on your behalf**（句型：「Hawkr staff acted on your behalf」） | 賣家一看就懂；法律上也清楚 | 不是名詞，標籤與按鈕難用 |
| C. managed by Hawkr | 簡短、有品牌感 | 暗示長期代管，不是限時、可稽核的授權工作階段 |
| D. assisted operation（草案） | 簡單 | 讀起來像一般客服協助，看不出是「代為執行」 |

**建議**：名詞標籤用 A（「Delegated session」「Delegation log」），面向賣家的整句用 B（「This was done on your behalf by Hawkr staff.」）。

### C3 品牌英文名（現況盤點）

- 中文名：**Hawkr**（`website/lib/seo/metadata.ts:7` `SITE_NAME`、`:8` 標題「Hawkr — 在地職人的市集」；`website/app/manifest.ts:5-6`；iOS 顯示名 `buyer-app/ios/Flutter/*.xcconfig:6` 與 `seller-app/ios/Flutter/*.xcconfig:6`＝「Hawkr」「Hawkr 賣家」，dev／stage 加後綴）。
- 英文：**Hawkr**（一個字）：iOS bundle `io.github.will413028.hawkr.seller`（`seller-app/pubspec.yaml:270-286`）、`pubspec.yaml:4` "Hawkr Seller App"、admin 頂欄「Hawkr Admin」（`admin/src/components/Layout/Topbar.tsx:33`）、字型 "Hawkr Rounded"、**使用者可見**的 `website/features/account/screens/settings/DeleteAccountSection.tsx:23`（「移除 Hawkr 的授權」）。
- 英文：**Hawkr**（兩個字）：只有 `buyer-app/lib/shared/widgets/top_navigation.dart:312`（頂欄預設標題）。
- 套件名 `hawkr_*`、媒體網域 `*.example.com`（`seller-app/lib/config/media_image.dart:124-125`；是否仍屬舊公司我沒有查，AGENTS 要求不連舊公司網域，請 Will 確認）。
- App Store／Google Play 名稱與商店頁：repo 內沒有商店中繼資料（`buyer-app/android`、`seller-app/android` 不存在，iOS 只有 xcconfig），無法盤點。商標與網域可用性我沒有查（無網路）。

| 候選 | 優點 | 缺點 |
|---|---|---|
| **Hawkr**（建議） | repo 內已是主要寫法（bundle id、字型、admin、DeleteAccountSection）；單字好做 wordmark | 與買家 App 頂欄的 Hawkr 不一致，要改那一處 |
| Hawkr | 英文可讀性高 | 只有一處在用；兩字與 bundle id 不一致 |
| Hawkr 加中文名（雙名 lockup） | 英文介面保留中文名辨識（台灣市場）、商店搜尋兩邊都命中 | 標題變長；英文使用者看不懂中文部分 |
| 純 Hawkr（英文介面也用中文） | 不用定英文名 | 與英文介面不一致；標題模板 `%s \| Hawkr` 對英文頁沒意義 |

**建議**：英文介面品牌名 `Hawkr`；標題模板 `%s | Hawkr`；預設標題 `Hawkr — A marketplace for local artisans`（對應「在地職人的市集」，要 Will 核可）；商店名稱用「Hawkr」。

### C4 英文搜尋提示

搜尋框現況：W `components/ui/Dropdown.tsx:588`「搜尋」、B `search_screen.dart:737`「搜尋食材或「今天煮什麼？」」、`search_results_screen.dart:566`「搜尋商品」、`category_screen.dart:697`「搜尋番茄、雞蛋、虱目魚…」。這類是單一畫面文案，**不進目錄**，各端自己的 i18n。

| 方案 | 英文 | 備註 |
|---|---|---|
| **A（建議）** | placeholder：`Search in Chinese, e.g. 番茄`；零結果時 helper：`No results. Search currently works with Chinese keywords only.` | 範例用中文字，使用者看得出該輸入什麼 |
| B | placeholder：`Search (Chinese keywords only)` | 短，但沒有範例 |
| C | 不提示 | 英文使用者會以為搜尋壞了；i18n brief 建議兩處都要 |

「請用中文搜尋」中文版也要核可：對應英文版是 A；中文介面不需要。

---

## D. 非 enum 的共用文字（Q3 進目錄）

### D.1 KycFlowCopy 文字欄位（W `website/lib/seller/kyc-flow.ts` = S `seller-app/lib/features/seller/data/kyc_flow.dart`，逐字相同，沒有漂移）

| 組 | 欄位 | 文字（現況＝提議，除標 ⚠） | W 行 | S 行 |
|---|---|---|---|---|
| 認證前（NOT_STARTED、DRAFT） | action | 開始認證 | 142 | 44 |
| | banner.title／description | 尚未開始商家認證／完成認證後才能正式收款、提領與配送設定。 | 78-79 | 46-47 |
| | gate.heading／message | 尚未通過商家認證／請先完成商家實名 / 認證流程，才能{subject}。 | 72-73 | 40,38 |
| 平台審核（SUBMITTED） | action | 查看進度 | 84 | 54 |
| | banner | 平台審核中／審核通過後將自動送出至 PayUni 進行金流審核。 | 87-88 | 56-57 |
| | gate | 商家認證審核中／你的商家認證平台審核中，通過後將自動送出至 PayUni 進行金流審核，完成後就能{subject}。 | 92-94 | 58-60 |
| PayUni 審核（BINDING、BIND_FAILED、FORWARDED、UNDER_REVIEW） | action | 查看進度 | 99 | 65 |
| | banner | PayUni 審核中／平台審核已通過，PayUni 正在審查金流資料。 | 102-103 | 67-68 |
| | gate | 商家認證審核中／你的商家認證已通過平台審核，PayUni 金流審核中，通過後就能{subject}。 | 107-109 | 69-70 |
| 需補件（PENDING_INFO） | action | 前往補件 | 114 | 96 |
| | banner | 需要補件／依下方提示更新資料後重新送出。 | 117-118 | 98-99 |
| | gate | 商家認證需要補件／認證需要補件。完成補件後才能{subject}。 | 122-123 | 100-101 |
| 未通過（REJECTED、PAYUNI_REJECTED） | action | 前往補件 | 128 | 75 |
| | banner | 認證未通過／請查看退件原因並編輯後重新送出。 | 131-132 | 77-78 |
| | gate | 商家認證需要補件／⚠ 認證未通過，請**補正**資料後重新送審。 → **認證未通過，請修改資料後重新送審。** | 136-137 | 79-80 |
| 已通過（APPROVED） | action | 無 | 184 | 105 |
| | banner | 商家認證已通過／可開始正式收款、提領與配送設定。 | 187-188 | 107-108 |
| | gate | 同「認證前」 | 191 | 109-110 |

只有 W 有而 S 沒有的 banner `ctaLabel`（開始認證／查看進度／前往補件／查看原因／前往錢包，W:80,89,104,119,133,189）與 `tone` 屬按鈕文字與語氣，依 L-brief Q3 留在 client。未通過的 action「前往補件」與標題「認證未通過」語意稍不合（退件是修改而非補件），**我沒有改**（超出 Q5 範圍），列為可選。

### D.2 「合作已終止」（MeasureState ACTIVE × MeasureKind TERMINATION，select）

現況六處：W `measureDisplay.ts:207`（狀態徽章）、W `SellerSuspendedNotice.tsx:54`（`TERMINATION_TITLE`）、S `measure_display.dart:150`、S `suspension_notice_screen.dart:33`、A `merchants/[id]/measure.ts:88`、A `ActiveMeasuresCard.tsx:55`（卡片標題寫「合作已終止」，非終止時寫「賣家停權中」）。文字全端相同。提議：`MeasureState.ACTIVE` default＝`{kind, select, TERMINATION{合作已終止} other{停權中}}`；staff 同。`ActiveMeasuresCard.tsx:55` 的「賣家停權中」是 admin 卡片標題，與狀態徽章「停權中」略不同，屬 admin 版面文案，留在 client。

### D.3 共用「未知」（Q4）

現況：admin `food-registration.ts:40` UNSPECIFIED「未知」；admin `app-versions/page.tsx:32`「未指定」；admin `food-registration.ts:107`「尚未申請」；`kycFieldLabel` 等回 `-`（`kyc-application.ts:~236`、`requestStateText` 回 `-`）。提議：目錄只有一個通用 key `common.unknown`＝**未知**（en：Unknown）；所有 enum 的 UNSPECIFIED 與不認得的值走它；各端移除上述 UNSPECIFIED 專屬文字與 `-` 後備。注意：KYCStatus UNSPECIFIED「尚未申請」是契約把它當業務值（`admin/seller/v1alpha1/seller.proto:832`），L 內改為 `optional`（L-brief Q4），顯示改由欄位不存在表達。

### D.4 Connect 預設錯誤文字（I7 Q3 增補，`common.error.<CODE>`）

TS `packages/hawkr-api-ts/src/connect/errors.ts:95-125`（文字在 :102,104,108,110,112,114,116,119,123）與 Dart `packages/hawkr_api/lib/errors.dart:201-209` 逐字相同；admin 與 website 共用 TS 版。提議全部**原樣搬入**（不為 L0 改字）：

| Code | 現況＝提議 | 英文（供 I7 用，未核可） |
|---|---|---|
| INVALID_ARGUMENT、OUT_OF_RANGE | 參數錯誤 | Invalid request |
| NOT_FOUND | 資源不存在 | Not found |
| ALREADY_EXISTS、FAILED_PRECONDITION、ABORTED | 資源衝突 | Conflict |
| PERMISSION_DENIED | 無權操作 | You do not have permission |
| UNAUTHENTICATED | 未登入 | Please sign in |
| RESOURCE_EXHAUSTED | 操作過於頻繁，請稍後再試 | Too many requests. Try again later |
| UNIMPLEMENTED | 功能暫不支援 | Not supported yet |
| UNAVAILABLE、DEADLINE_EXCEEDED | 服務暫時不可用 | Service temporarily unavailable |
| UNKNOWN、INTERNAL、DATA_LOSS | 系統錯誤 | Something went wrong |
| CANCELED | 無（不顯示） | — |

可選改善（我沒有納入）：FAILED_PRECONDITION 與 ALREADY_EXISTS 顯示「資源衝突」對使用者不夠有用，但伺服器通常會附 `LocalizedMessage`，預設文字只是後備。

### D.5 其他共用文字（記錄，不新增變更）

- 開店被擋原因四句（伺服器擁有）：Go `model.go:119-127`、W `lib/seller/onboarding.ts:21-27`、S `seller_onboarding.dart:26-40`，三處各一份。L 之後以 `LocalizedMessage` 為準，W／S 抄本刪除；唯一需改字的是第一句（B6）。
- `AppealUnavailableReason`／`RemediationUnavailableReason` 的 client 手寫句（W `lib/seller/measures.ts:190-193,236-240`、S `measure_display.dart:100-106,235-238`）是伺服器擁有文字，L 之後由 `LocalizedMessage` 取代；若 L 沒有同時處理，需依 B3／B4 先改字。

---

## 給 Will 的決定（共 7 個）

- D1 KYCField 十個值的單一文字（A1，建議照表）。
- D2 KYCFlowStatus 的「已核准／已退回」與「已通過／未通過」用 `staff` key 保留（建議），或收斂成一份（L-brief 原看法）。
- D3 DRAFT 用「填寫中」（建議）或「草稿中」。
- D4 新增的補完整文字：MeasureRequestState.CLOSED「已結案」、FoodRegistrationState（賣家）APPROVED「已通過」／ENDED「已結束」、MeasureBasis RISK「風險」／VIOLATION「違規認定」、RevokeReason 四值、NOT_STARTED 的 staff「尚未申請」。
- D5 Q5 落字：導覽「措施與申覆」；「補正」→「解除請求」或「修改」；紀錄／記錄統一；B7 的 1、3、4、5 保留範圍。
- D6 英文：措施用 enforcement action 或 restriction；代行用 delegated session（標籤）加 on your behalf（整句）；售後申訴英文用 dispute。
- D7 品牌英文名 Hawkr、預設標題英文、英文搜尋提示方案 A。
