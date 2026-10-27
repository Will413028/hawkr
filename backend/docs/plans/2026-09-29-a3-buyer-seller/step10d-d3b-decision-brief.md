# 10d D3 追加決定（D3b）：公開的「食品業者登錄字號」從哪裡來

> Will 2026-10-15 決定：選 (F)；F1 送審時機、F2 核對標準照建議；第八節 (F) 的文案與錢包「販售加工食品」列在 KYC 核准前也顯示，同意。

## 摘要（要 Will 決定的）

- **建議 (F)**：把食品業者登錄從 KYC 拆出來，變成獨立的審核資源 `FoodRegistration`。開店前後都可以送審，後台只有一個收件匣，核准的那一筆就是店家字號的唯一來源。KYC 的 `food_business`、`UpdateMyStore` 的 `food_business` 與 `processed` 旗標全部刪除。
- **替代方案是 (C′)**：只留一條賣家自填的路徑，加上格式檢查，前台標成「賣家登錄資訊」，不審核。成本大約是 (F) 的三分之一。平台以法定名稱排版一個沒有核對過的值，義務主體本來就是賣家。
- **(B)（v1 的建議）撤回**：它留下兩條「已核對」的寫入路徑，個人賣家的字號在 KYC 時沒有人看過（HIGH-1），也要另外修「只在 processed 時投影」的 bug。在每一點上都輸給 (F)。
- **基準更正**：沒有任何法規或具名系統要求平台核對食品業者登錄字號。INFORM 與 DSA 第 30 條只管身分與商業登記，不涵蓋行業登錄。支持 (F) 的只有 Amazon「受限品類另走一套核准流程、與身分驗證分開」（第三方整理，非官方）。
- **要同意的**：① (F) 或 (C′)；② (F) 的兩個子決定：送審時機（建議隨時可送，**核准**時要求 KYC 已核准）、個人賣家的核對標準；③ 第八節的文案，以及錢包的「販售加工食品」列在 KYC 核准前也顯示（流程改動）。
- 刀：新增**第 2b 刀**，排在第 2 刀之後、第 4 與第 6 刀之前。會改 `seller.proto` 與 admin `seller.proto`，`maxRESTOperations` 維持 492。10e 第 2c-3 刀（簡報撰寫中）也會改這兩個檔的 `Store` 與 admin 服務，見第九節。

---

## 一、現況（已查證，行號以 `hawkr-10d2` 為準）

| 寫入路徑 | 位置 | 實際的檢查 |
|---|---|---|
| 賣家 `UpdateMyStore`（mask 帶 `food_business` 就整組換掉） | `httpapi/seller/app.go:237-245`；`profile/profile.go:561-563`（`merge`，`:547`）、`UpdateProfile` `:331` | 只檢查字數 ≤50，以及加工旗標要與字號一致（`model/validation.go:101-104`，DB CHECK `schema.sql:97-99`）。需要的能力是 `CapStorefrontWrite`，所以核准後隨時可以改 |
| KYC 核准時投影 | `kyc/admin.go:421-425`、`projectFood` `:443-453`；SQL `kyc.sql:236-240` 寫死 `is_processed_food = true` | **只在申請寫「加工」時投影** |
| KYC 申請的填寫 | `kyc/draft.go:187-200` `applyFood`，不分賣家型態；website `CertificationsClient.tsx:1988-2016`（`SectionOther`，所有型態都會顯示）；seller-app `kyc_wizard_steps.dart:607-630` | 同上，只有字數與一致性 |
| KYC 審核的畫面 | admin `KycApplicationCards.tsx:150-151` 只在 `subject.case === "business"` 時顯示；`kyc-application.ts:722-724` 遇到個人賣家回 `""` | **個人賣家的字號，管理員看不到就投影到店家**（HIGH-1，已確認） |
| devseed | `devseed/devseed.go:160` upsert 時直接寫 `is_processed_food = false, food_business_reg_no = NULL` | v1 說它「不直接寫」，這是錯的 |

- 第 2 刀（還沒 commit）的 `PublicStore.food_business_registration_number` 讀 `stores.food_business_reg_no`（`httpapi/seller/public.go`），也就是最後一次寫入的值，不管是誰寫的。
- **PayUni 不收食品登錄**：`payuni/sync.go:247-252` 組綁定資料時沒有用到；`model/payuni.go:109` 的 `businessFields` 裡的 `food_business` 是死映射，PayUni 的錯誤碼不可能指向它。
- **現行 bug（與選哪個方案無關，MED-5）**：核准前賣家可以用 `UpdateMyStore` 寫入字號，之後 KYC 以「非加工」核准，因為只在 processed 時投影，店家上的舊值不會被清掉。
- **沒有定義核對標準**：設計書 18 只在 `:330` 說它是公開登記資訊、不加密；runbook 與 admin 都沒有 fadenbook。
- **加工食品的上架門檻只在 client**：website 的 `cert-gate.ts:93`，以及 `ProductFormPage.tsx:118`、`ProductEditDrawer.tsx:108`、`ProductsPage.tsx:135`。seller-app **沒有**上架門檻，`foodRegistrationNumber` 只出現在 `onboarding_tasks.dart:216`、`wallet_account_section.dart:33`、`food_registration_form.dart:24`。Java catalog 也不檢查。

## 二、既有機制為什麼存在

- **放在 KYC**：公司時期的開店表單一次收齊所有資料，食品登錄只是「其他資訊」裡的一欄，從來沒有定義過審核標準。PayUni 不需要它，它也不是 KYC 核准的條件。7c 只把投影搬進核准的 transaction（`CARRYOVER.md:114`，D8），沒有問它該不該在 KYC 裡。
- **放在 `UpdateMyStore`**：舊系統有一支「補登」路由（錯誤碼 9230），服務 KYC 時沒勾、事後才要賣加工食品的賣家。8a、8b 依「一個 aggregate 一個版本」把它併進部分更新（`CARRYOVER.md:34-35`、`:242`）。那次決定處理的是 RPC 的形狀，沒有處理誰可以不經審核寫入；當時字號也不公開。
- **`processed` 旗標**：它只是「有沒有字號」的另一種說法，兩者由 CHECK 綁死成等價，本身沒有獨立的資訊。D3 的附帶發現也指出，法定的登錄範圍不只加工食品（有商業、公司登記的販售業者也包括在內），所以這個旗標的語意本來就不對。
- **D3 改變了前提**：這個值會由平台以法定名稱排版公開，買家會把它當成締約資訊。

## 三、業界基準（只寫來源支持的部分）

| 來源 | 實際範圍 | 對本題的意義 |
|---|---|---|
| 美國 INFORM（15 U.S.C. §45f，[uscode](https://uscode.house.gov/us/usc/119-118/t15/s45f/)，2026-10-15 讀過原文） | 只適用高交易量賣家（24 個月內任一連續 12 個月有 200 筆以上、且營收 5,000 美元以上）。驗證的是銀行帳戶、身分或商業文件、稅號、email 與電話。資訊變更要在「收到變更通知後 10 天內」驗證；每年至少通知一次，賣家要電子確認 | **不涵蓋行業執照或登錄號碼**。變更期間新值不必等驗證通過才生效，所以它不支持「通過前沿用舊值」 |
| 歐盟 DSA 第 30 條（[條文整理站](https://www.eu-digital-services-act.com/Digital_Services_Act_Article_30.html)，非 EUR-Lex 官方，EUR-Lex 本次抓不到內容） | 1(d) 是交易者所在的 trade register 與登記號碼；1(e) 是賣家自我聲明只提供合法商品；第 2 項是盡力評估資訊可靠，賣家對正確性負責；第 3 項是有跡象顯示資訊不正確時，要求更正、不更正就暫停服務 | 管的是身分與商業登記，本 repo 已經用 GCIS 做（`kyc/registrycheck.go`）。**沒有提到行業執照**，也**沒有例行的變更重新驗證**。「食品業者登錄算不算 1(d) 的 similar public register」沒有查到解釋，不採用 |
| Amazon 受限品類（Grocery & Gourmet Food） | 品類要另外申請核准，與 Seller Identity Verification 分開；核准時可能要求營業執照、食品安全證明，以及「與產品相關的政府機關許可」 | **只有第三方整理**（[litcommerce](https://litcommerce.com/blog/selling-groceries-on-amazon/)、[bebolddigital](https://www.bebolddigital.com/blog/how-to-sell-food-on-amazon)、Seller Central 論壇），沒有讀到 Amazon 官方頁。前台會不會顯示這個許可號碼，沒有查到 |
| 蝦皮、momo、PChome | 沒有查到它們收集或核對賣家登錄字號的做法 | 臺北市 2020 年查核 5 家、4 家未揭露（[foodnext](https://www.foodnext.net/news/newsnow/paper/5098431122)），台灣市場的常態是賣家自己寫在商品說明 |

**結論**：
- 沒有法規、也沒有具名系統要求平台核對食品業者登錄字號。法定的義務主體是賣家（食安法 §8 第 3 項；〈通訊交易食品定型化契約應記載事項〉第三點）。
- 唯一可以參照的設計模式是 Amazon：**品類資格與身分驗證分開，各自審核**。這支持「拆出 KYC」，但支持不了「一定要審」。
- 「新值通過前沿用舊值」是我們自己的設計選擇，不是基準。

## 四、選項

上架門檻在 B 之前只在 client（所有選項都一樣能用 API 繞過），所以不列為任何一個選項的差異。

| 選項 | 內容 | 取捨 |
|---|---|---|
| (A) 維持現狀 | 兩條路徑都能寫，公開最後一次寫入的值 | 不用改。平台以法定名稱排版任意字串；個人賣家的字號沒有人看過；processed 投影的 bug 還在 |
| (B) v1 的建議 | 保留 KYC 的欄位，另外新增「核准後變更要審」的請求 | 已核對的值有兩條寫入路徑、兩個後台審核畫面、同一條核對標準寫兩處、申請與店家各存一份。另外還要修 HIGH-1 與 processed 投影。**每一點都輸給 (F)，撤回** |
| **(C′) 誠實版的不審** | 只留 `UpdateMyStore` 一條路徑，**同時刪掉 KYC 的欄位與 `processed`**。字號做格式檢查（觀察到的 `A-\d{9}-\d{5}-\d`，**沒有官方規格**）。前台標成「賣家登錄資訊」，買家可以用既有的檢舉入口（`reports`，DSA 第 16 條的對應）回報 | 成本最低，約 (F) 的三分之一。單一路徑，HIGH-1 與 processed 投影的 bug 一起消失。責任落在法定主體，標示也誠實。代價：平台仍然排版一個沒有核對過的值，冒用別家字號要等檢舉才會發現；格式規則沒有官方規格，可能誤擋合法的字號 |
| (D) 自動比對官方資料庫 | — | 沒有可程式化的 API，現在做不到 |
| (E) 核准後要改就整份 KYC 重送；或在 KYC `APPROVED` 底下加補件子狀態 | — | KYC 的 `APPROVED` 綁著 PayUni 綁定與曝光。為了一個 PayUni 根本不收的欄位重開身分審核，代價過大 |
| **(F) 獨立的審核資源** | `FoodRegistration` 自己一張表、自己一套狀態，開店前後都能送審，後台一個收件匣。核准的那一筆是店家字號的唯一來源。KYC 的 `food_business`、`UpdateMyStore` 的 `food_business`、`processed` 全部刪除 | 對應 Amazon「品類資格與身分分開」的模式。單一寫入路徑，HIGH-1 與 processed 投影的 bug 都是刪掉程式碼而消失，不是修補。字號也不再綁在加工旗標上，D3 附帶發現的「登錄範圍不只加工食品」以後不必再改模型。成本：一張表、賣家三支指令、後台三支方法、一個收件匣、兩端 UI 的狀態，以及要有人力審核 |

## 五、約束帳

| 約束 | 性質 | 來源 |
|---|---|---|
| 揭露義務的主體是賣家；平台本身的顯示或核對義務沒有查到明文 | REAL（法規）。**這條推向 (C′)** | 食安法 §8 第 3 項；應記載事項第三點（消保法 §17） |
| 沒有逐家登錄資料的 API，核對只能人工查 fadenbook | REAL（缺資料；fadenbook 本次又逾時，查詢鍵與結果顯示的欄位都**未驗證**） | 衛福部 103-12-01 公告開放查詢 |
| 個人型 KYC 沒有商業登記資料（只有 real_name 與身分證），沒辦法比對統編 | REAL（沒有商號的個人賣家根本沒有這份資料） | `KycApplicationCards.tsx:150`、KYC 個人子訊息 |
| 加工食品的上架門檻不在伺服器 | REAL，到 B 為止（catalog 在 Java）。所有選項都一樣，不影響選擇 | 主計畫 B |
| 審核結果送到賣家的通知 | REAL，到 A8 為止：模組在 transaction 內發事件，送達由 A8 的 notification 負責（`measure/measure.go:4` 是同一個模式） | 主計畫 A8 |
| 食品登錄放在 KYC | SELF-IMPOSED（公司時期的表單結構；PayUni 不收，也不是核准條件） | `payuni/sync.go:247-252` |
| 食品登錄併在 `UpdateMyStore` | SELF-IMPOSED（8a、8b 的 RPC 形狀；CARRYOVER 自己的原則就是「有領域語意的狀態轉移另開指令」） | `CARRYOVER.md:34`、`:242` |
| `processed` 旗標 | SELF-IMPOSED（與字號等價，沒有獨立的資訊） | `schema.sql:97-99` |
| 兩端「儲存後立即套用、不需重新送審」的文案與 10c 的補件流程 | SELF-IMPOSED（10c 的選擇） | `FoodRegSupplement.tsx:125`、`food_registration_form.dart:110` |
| 審核的人力 | SELF-IMPOSED（營運成本）。這是 (F) 與 (C′) 之間真正的取捨，由 Will 判斷 | — |
| 10e 第 2c-3 刀同時在改 `seller.proto` 與 admin `seller.proto` | SELF-IMPOSED（排程）；處理方式見第九節 | `hawkr-10e` 的 `step10e-2c3-brief.md` |
| 既有資料、client 相容性 | 不是約束（從未上線） | 根目錄 `AGENTS.md` |

## 六、建議：(F)

**理由**：唯一可參照的模式（Amazon 把品類資格與身分分開審）指向 (F)。推離它的 REAL 約束只有兩條：「只能人工核對」與「個人賣家沒有商業登記資料」，兩條都只是調整核對方式，不會讓人選 (B)。(C′) 是正當的替代方案，前提是 Will 判斷審核的人力不值得；約束帳第一列確實支持它。

**子決定 F1：什麼時候可以送審**。建議：店家建立之後**隨時可以送**；**核准**時要求該帳號的 KYC 已是 `APPROVED`，否則回 FailedPrecondition。理由是核對標準要比對的是**已驗證過的**主體，核准前的 KYC 主體還可能改。收件匣預設先列 KYC 已核准、可以裁定的件。
- 另一個做法是「KYC 核准後才能送」：少一個前提檢查，但想賣加工食品的賣家要排兩次隊。

**子決定 F2：核對標準**（寫進設計書 18 與 runbook）：
- 在 fadenbook 查得到這個字號，而且登錄狀態有效。
- 企業賣家：登錄的業者統編或名稱，要與 KYC 的 `business.tax_id`、`company_name` 相符。
- 個人賣家：查詢結果有顯示業者名稱或負責人時，要與 KYC 的 `real_name` 相符。查詢結果不顯示的話，只核對字號有效，並在裁定紀錄寫明「僅核對有效」。fadenbook 到底顯示哪些欄位，**未驗證**，上線前要人工查一次再定稿。
- 不符就退回，理由必填。

**根本修正**：單一資源、單一寫入點。每家店最多一筆 `PENDING`、最多一筆 `APPROVED`，各用 partial UNIQUE 保證。公開、後台、legacy 的讀取一律讀 `APPROVED` 的那一筆。店家表上不再有食品欄位，所以不會有第二份。

**狀態**：`PENDING`、`APPROVED`、`REJECTED`、`WITHDRAWN`、`ENDED`。舊的核准在新的被核准時，或賣家清除時，變成 `ENDED`，並以 `end_cause` 區分（`SUPERSEDED`、`CLEARED`，沿用 `measure_requests.close_cause` 的做法）。
- 新值核准之前，舊的核准繼續有效。這是我們的選擇，不是基準：審核期間店面不中斷，也不會公開未核對的值。
- 賣家清除（不再賣加工食品）立即生效，因為撤回一項主張不會誤導買家。

**賣家的三支指令**（v2 回應 LOW「Withdraw 沒有論證」）：
- `SubmitFoodRegistration`：已有待審時，取代它（舊的變 `WITHDRAWN`）。
- `CancelFoodRegistrationRequest`：只撤回待審，保留目前核准的那一筆。這是它存在的理由：送錯了更換，又不想清掉現有的登錄。只靠 Submit 與 Clear 做不到。
- `ClearFoodRegistration`：結束核准的那一筆，同時撤回待審。

三支都需要 `CapStorefrontWrite`。

**關店**：不撤回待審。10e 第 2c-3 刀的 W7(b) 會讓關店可以在 30 天內重開同一家店，待審的件留著，重開後照常有效。收件匣只列 `ACTIVE` 的店，裁定已關閉的店回 FailedPrecondition。這樣 2b 不必碰 `closing`，避開 2c-3 正在重寫的鎖順序。

## 七、改動清單（第 2b 刀，同一刀內做完）

**契約**
- `api/hawkr/seller/v1alpha1/seller.proto`：
  - `Store.food_business = 22` 改成 reserved。新增 OUTPUT_ONLY 的 `FoodRegistration food_registration`（核准的字號與核准時間），以及 `FoodRegistrationRequest food_registration_request`（最近一筆未結束的請求：狀態、字號、送出時間、裁定時間、退回理由）。
  - `KYCApplication.food_business = 15` 改成 reserved；`KYC_FIELD_FOOD_BUSINESS = 11` 改成 reserved。刪除 `FoodBusiness` 訊息。
  - 新增第六節的三支 RPC。
  - `PublicStore.food_business_registration_number` 的註解改成「平台核對過的登錄字號」。
- `api/hawkr/admin/seller/v1alpha1/seller.proto`：
  - 後台 `Store.food_business = 25` 換成核准的那一筆與待審的請求。
  - KYC 申請的 `food_business = 16` 改成 reserved。
  - 新增 `ListFoodRegistrationRequests`、`GetFoodRegistrationRequest`、`DecideFoodRegistration`（approve 或 reject；reject 理由必填，內部註記選填，沿用 `auditreason`）。
- legacy `backend/api/proto/seller/seller.proto`（`SellerMeta`）不改，`IsProcessedFood` 改由「有沒有核准的字號」推導。
- `buf breaking` 會報破壞，這是預期的。之後 `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 都要重新生成。

**單體（seller 模組）**
- 新的 migration：
  - 建 `seller.food_registrations`：UUIDv7、`store_id` 外鍵、字號（trim 後 1–50 字；不加格式 regex，因為沒有官方規格，交給人工核對）、狀態、送出者、裁定者、理由、內部註記、`end_cause`；依狀態的 CHECK；兩個 partial UNIQUE。
  - DROP `stores.is_processed_food`、`food_business_reg_no` 與 `stores_food_registration_check`。
  - DROP `kyc_applications` 的兩欄與 `kyc_applications_food_check`。
  - 從兩個 KYC 欄位 CHECK（`schema.sql:794`，以及 `kyc_review_issues_field_check`）移除 `'food_business'`。本機已經有的列，比照 10e W2 (a) 以 RAISE 列出。
  - 店家上既有的字號**不搬**，因為它們按定義都沒有核對過；以 NOTICE 列出受影響的 store_id。
- 新的 `internal/foodreg`：Submit、Cancel、Clear、List、Get、Decide。
  - 核准與清除時，在同一個 transaction 內發 `StoreProfileChanged` 並呼叫 `stint.Invalidate`。
  - 裁定時發 `FoodRegistrationDecided`，送達交給 A8。
  - 加三個權限：`seller.food_registrations.list`、`.get`、`.decide`（`adminperm`、`adminauth` 的角色）。
- 刪除：
  - `kyc/draft.go:169,187-200`、`kyc/diff.go:93-94`、`kyc/convert.go:28,93,111` 的食品欄位、`kyc/admin.go:421-425` 與 `projectFood :441-453`。
  - SQL：`ProjectFoodRegistration`（`kyc.sql:236-`），以及 `kyc.sql:25,35,66-67` 的欄位。
  - `profile.go:488,561-563,633`、`stores.sql:83-84`。
  - `model/validation.go:101-104`、`model/model.go:36,180-196`、`model/kyc.go:249-250,417,673`、`model/payuni.go:109` 的 `food_business`。
  - `seller.go:302-303`。
- 讀取改成讀核准的那一筆：
  - 第 2 刀的 `public.go`（`deref(s.FoodBusinessRegNo)`）。
  - `httpapi/seller/app.go:91,117,237-245`、`admin.go:83`、`admin_kyc.go:351`、`kyc.go:28,158-160,255`。
  - `stint/stint.go:208-209`、`legacy/sellerserver/server.go:142,167-168`。
  - `GetFDAExport` 的後台 `Store`（`CARRYOVER.md:292`）。
- `devseed/devseed.go:112,160`：刪掉兩欄。另外種一家有核准字號、一家有待審的店，給 UI 開發用；這是 fixture 直接寫表的例外，明寫在註解裡。
- `purge.sql:13` 的註解：登錄是公開資訊，清理時保留。
- 範例路徑：`httpapi/rpc/updatemaskfields_test.go`、`packages/hawkr-api-ts/tests/field-mask.test.ts`、`packages/hawkr_api/test/field_mask_test.dart` 都拿 `food_business` 當例子，要換成別的路徑。

**admin**
- 新增收件匣頁「食品業者登錄審核」與單筆頁：
  - 顯示送出的字號、目前核准的字號、KYC 狀態與主體（企業：公司名稱、統編；個人：real_name），附 fadenbook 連結。
  - 核准；退回時理由必填。
  - 側欄與權限一起加。
- `merchants/[id]`：
  - 刪除 `KycApplicationCards.tsx:19,122,150-151` 與 `kyc-application.ts:221,721-724`。HIGH-1 的「個人賣家看不到」因為 KYC 不再有這個欄位而消失。
  - 店家頁加上核准的字號與待審的連結。目前 admin UI 沒有任何地方讀店家的 `foodBusiness`（`rg -n -i "foodBusiness|食品業者" admin/src`，排除 gen，只命中 KYC 那兩個檔）。

**website**
- `FoodRegSupplement.tsx`：改寫成「未登錄、審核中、未通過、已登錄」四個狀態，對應送出、撤回、清除；連同 `:3-12` 的檔頭註解。
- `WalletClient.tsx:314,411`：「販售加工食品」列在 KYC 核准前也顯示（流程改動，要同意）。
- 資料層：
  - `lib/seller/repo.ts:62,169-190`：`updateFoodBusinessRegistration` 換成三支指令。
  - `lib/seller/store.ts:135`：讀核准的那一筆。
  - `cert-gate.ts:93` 的語意不變。
  - `lib/seller/kyc.ts:53-54,77-78,141-142,202-203,347-353,491,670`：刪除。
- KYC 表單：刪除 `CertificationsClient.tsx:13,119,198-199,211-212,1988-2016` 的食品區塊（KYC 預設值也不再從店家帶入）。
- 文案（見第八節）：
  - `onboardingTasks.ts:229-245`，連同 `foodHref`：一律導到錢包的食品列。
  - `ProductFormSections.tsx:150-152`、`ProductsPage.tsx:59-60` 的 `FOOD_REG_REQUIRED_HINT`。
  - 註解：`ShelfStatusControl.tsx:200`、`ProductsPage.tsx:129-132`。
- 測試：`FoodRegSupplement`、`onboardingTasks`、`CertificationsClient`、`WalletClient`、`kyc`、`repo`、`store` 的 unit test；`tests/e2e/helpers/mock-gateway.ts:186,241`、`seller-dashboard-onboarding-actions.spec.ts`、`seller-profile-contract.test.ts`、`seller-e2e-fixtures.test.ts`。

**seller-app**
- `food_registration_form.dart`：改寫成同樣的四個狀態，`:110` 的文案也要換。
- `wallet_account_section.dart:14-33,91-92`：核准前也顯示。
- `store_form.dart:160-227`：移除 `foodBusiness`。
- `my_store.dart:229-233`：讀核准的那一筆，再加上請求的狀態。
- `onboarding_tasks.dart:216-257`：`walletSection` 一律指向食品列。
- 刪除 `kyc_form.dart`、`kyc_fields.dart:86`、`kyc_wizard.dart:285,300,339-343`、`kyc_wizard_steps.dart:529,607-630` 的食品部分。
- 測試：`kyc_form`、`kyc_wizard`、`onboarding_tasks`、`store_editor`、`onboarding_progress_card`、`wallet_screen`，以及 `my_store_fixtures`。
- buyer-app：不用改（第 4 刀讀 `PublicStore`）。

**後端測試**
- 送出時已有待審，舊的被取代，而且只剩一筆待審（並行送出）。
- 核准前 `PublicStore` 不變；核准後換成新值，同時有 `StoreProfileChanged` 與快取失效。
- KYC 不是 `APPROVED` 時核准回 FailedPrecondition。
- 清除立即生效，同時撤回待審。
- 撤回只動待審，核准的那一筆保留。
- 已關閉的店不在收件匣，裁定回 FailedPrecondition。
- `UpdateMyStore` 帶 `food_business` 回 InvalidArgument（未知路徑）。
- KYC 契約已經沒有這個欄位。
- 三個權限各自擋得住。
- **Mutation**：拿掉 partial UNIQUE，並行送出的測試要失敗。

**文件**
- `CARRYOVER.md` 沿用盤點表：`:34-35`、`:114`、`:242`、`:292`、`:348` 改指本決定；新增一列「食品登錄放在 KYC → 不成立」。
- 設計書 18：`:129`、`:330`、`:448`、`:818`，並寫入核對標準。
- runbook：審核步驟與 SLO（建議比照補正，7 個日曆天）。
- seller `AGENTS.md`：寫入點與權限。
- 主計畫 10d 的刀表加 2b。

## 八、文案（要同意）

**(F)**
- **賣家端說明**，取代「資料儲存後會立即套用，不需要重新送審。」：
  「上架加工食品前須完成食品業者登錄。送出後平台會核對食藥署的登錄資料，通過後才會顯示在店面與商品頁，並開放上架加工食品。審核期間沿用目前的設定。」
- **未登錄**：按鈕「送出審核」。
- **審核中**：「審核中：{字號}（{日期} 送出）」，按鈕「撤回」；實名認證還沒通過時，加一句「平台會在實名認證通過後進行審核。」
- **未通過**：「未通過：{理由}。請確認字號與食藥署登錄資料一致後重新送出。」
- **已登錄**：「已登錄：{字號}」，按鈕「更換字號」「清除」。
- **清除確認**：「清除後，店面與商品頁不再顯示食品業者登錄字號，也無法上架加工食品類商品。」
- **上架提示**（`ProductFormSections.tsx:152`）：「加工食品類商品需先通過食品業者登錄審核才能上架。請到『錢包與認證』送出食品業者登錄字號。」；`FOOD_REG_REQUIRED_HINT` 改成「加工食品需先通過食品業者登錄審核才能上架」。
- **新手任務**（`onboardingTasks.ts`）：只留一句「若販售加工食品，請在錢包與認證送出食品業者登錄字號審核；一般生鮮商品可略過。」，不再分三種 KYC 狀態。
- **後台**：收件匣標題「食品業者登錄審核」，單筆頁有「在食藥署查詢」的連結。
- **前台**：維持 D3 已同意的一列「食品業者登錄字號　{字號}」，不加「已核對」字樣（平台不保證正確）。

**(C′)（如果選它）**
- 前台改成「食品業者登錄字號（賣家登錄資訊）　{字號}」。
- 賣家端：「此字號由你提供並顯示於店面與商品頁，請確認與食藥署登錄資料一致；不實登錄須自負法律責任。」

## 九、刀、契約與 10e

- **第 2b 刀**：排在第 2 刀（D3 欄位，在 `hawkr-10d2`，還沒 commit）之後、第 4 刀與第 6 刀（顯示）之前，讓前台第一次顯示時，值就已經只來自核對過的路徑。
  - 不併進第 2 刀：2b 會讓兩端的補件 UI 與 KYC 表單同時改，第 2 刀只動契約與單體，每一刀都要各自完整。
- **`maxRESTOperations` 維持 492**：2b 只新增 Connect 方法，不增也不刪 REST 路由。舊的補登路由早就不在 gateway：`rg -n "food-business-registration" backend/gateway/internal/router backend/api/openapi/openapi.yaml` 沒有命中。
- **10e**：
  - `local/10e` 已經合進 main（`git merge-base --is-ancestor local/10e main` 成立）。
  - 但 `hawkr-10e` 還有撰寫中的 `step10e-2c3-brief.md`，它要改 `seller.proto` 的 `Store`（`StoreCloser`、`Closure`）、admin `seller.proto`（刪 `CloseStore`）與 `closing` 的鎖順序。
  - 2b 與 2c-3 改的是同一個訊息的不同欄位：proto 本身可以手動合併，生成檔由後落地的一方重新生成。
  - 2b 刻意不碰 `closing`（第六節「關店」）。
  - 2c-2 那一段（`Measure`、`RequestRemediation`）2b 不碰。
- **回滾**：整刀 revert，本機 DB 重建（上線前沒有資料遷移的負擔）。

## 十、終點與觸發

- **食藥署釋出逐家登錄資料的 API 或開放資料**：人工核對改成自動比對，並對核准過的字號做週期複查（登錄可能被廢止）。觸發條件就是該資料集出現。
- **B 搬 catalog**：加工食品的上架門檻移到伺服器，讀同一筆核准的登錄；seller-app 目前沒有門檻的問題同時消失。
- **A8 notification**：送達 `FoodRegistrationDecided`。
- **律師回覆 D3 附帶發現的登錄範圍**：如果有商業或公司登記的生鮮販售業者也必須登錄，就在開店流程依型態要求送審。(F) 的模型不用改。

## 十一、審查意見的處理

| 項目 | 處理 |
|---|---|
| HIGH-1 個人賣家 | 採納並已核對。(F) 下 KYC 不再有這個欄位，所以顯示問題消失；核對標準改成依實際有的資料比對（F2），約束帳加一列 REAL |
| HIGH-2 (F) | 採納，改成建議 |
| MED-1 基準講過頭 | 採納。INFORM 讀過原文，DSA 第 30 條讀過條文整理站，範圍照第三節更正；「沿用舊值」改標為我們的選擇 |
| MED-2 上架門檻 | 採納，從選項的比較中拿掉 |
| MED-3 (C′) | 採納，寫成正當的替代方案，並承認約束帳第一列推向它 |
| MED-4 漏列 | 採納，第七節逐檔列出；另外補了 review 沒提到的：KYC 表單從店家帶入預設值（`CertificationsClient.tsx:198`、`kyc_wizard.dart:339`）、`legacy/sellerserver`、`stint`、`GetFDAExport`、field-mask 的範例測試 |
| MED-5 現行 bug | 採納，在第一節獨立列出；(F) 與 (C′) 都會把這段程式刪掉 |
| devseed | 採納，改成明寫的 fixture 例外 |
| LOW 三支改兩支 | 不採納，保留 Cancel，理由寫在第六節 |

## 十二、未驗證

- fadenbook 本站兩次逾時：查詢鍵、有沒有 CAPTCHA、結果顯示哪些欄位（影響 F2 對個人賣家的比對）、能不能用字號建立直接連結，都沒有確認。
- 有沒有逐家登錄資料的 API：沒有查到，不等於確定不存在。
- Amazon 的品類核准只有第三方整理；DSA 只讀到條文整理站，不是 EUR-Lex 官方。
- 蝦皮、momo、PChome 的做法沒有查到。
- 字號格式 `A-\d{9}-\d{5}-\d` 只是觀察，沒有官方規格（影響 (C′) 的格式檢查）。
- 個人賣家（沒有商號）能不能、需不需要登錄，沒有查證。
