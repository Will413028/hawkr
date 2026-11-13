# 賣家平台審核、防詐身分比對、入駐同意、關店與個資保留（整合版）

| 項目 | 值 |
|---|---|
| 狀態 | accepted（整合並取代 `2026-10-19-seller-kyc-payuni-consolidated.md`；舊檔的核心決定一半被推翻（平台收身分與銀行資料、金流商綁定、流程狀態），一半仍成立（登記資料快照、退回逐項問題、入駐同意、兩段清理），並已有 6 段修訂） |
| 日期 | 2026-11-13 |
| 影響範圍 | `internal/modules/seller`（KYC 申請與審核、登記資料查詢、防詐身分雜湊、入駐同意、關店與個資清理）、`internal/platform/seal`、`internal/legacy`（`seller.deleted` 的 relay）、admin、website、seller-app |
| 取代 | `2026-10-19-seller-kyc-payuni-consolidated.md`（它標 `superseded by` 本檔；它取代的 `2026-10-02-seller-kyc-payuni-in-monolith.md` 與修訂的 `2026-07-20-seller-kyc-pii-encryption-scope.md` 一併被本檔承接，加密範圍不再存在） |
| 相關 | 收款帳戶、payment port、Stripe、開通狀態推導：`2026-11-13-payment-onboarding-provider-port-stripe.md`。治理（措施、停權、終止合作、關聯帳號）：`2026-10-18-seller-governance-consolidated.md`。後台待辦：`2026-10-20-admin-review-queue.md` |

本檔只寫現行決策，不寫改動經過。舊檔沿用的章節名（「身分比對與比對 hash」「登記資料快照」「入駐同意紀錄」「關店與個資保留」）保留，程式註解用它們引用。

## 脈絡

賣家收款開通改走 provider 中立的 payment port（見相關 ADR）之後，平台不再向金流商送身分證號、銀行帳號與證件，也就不必自己收。舊決定裡為「平台自收並轉送金流商」而建的整條管線（密文欄位、顯示全文、金流商綁定、流程狀態）失去存在的理由；平台審核只剩市集政策與統編。為了讓讀者只讀一份就知道現況，本檔重寫仍成立的部分、寫明被推翻的部分。

約束：
- 外部：詐欺犯罪危害防制條例第 37 條第 1 項（合理期間保存足以重建使用者與個別交易的資料）與第 2 項（調取資料保存六個月）；6 個月這個值上線前請律師確認。P2B 第 4 條第 3 項（撤回終止時恢復並交還生效前的資料）、DSA 第 20 條第 1 項（申訴期至少六個月）。
- 外部：金流商的身分驗證與法定的金流 KYC 保存由金流商負責；開立發票的稅務紀錄由 invoice 模組自己保存快照。
- 外部：防詐雜湊保存 5 年的法源未確認（Will 的產品決定，參考金流業「關係結束後 5 年」的慣例，推測），上線前與條款一起核對。
- 繼承：停權以帳號為單位、由措施在讀取時推導（`2026-10-18-seller-governance-consolidated.md`）；身分比對用同一個推導。仍成立。
- 繼承：系統從未上線（根目錄 `AGENTS.md`「上線前不受相容性約束」）：沒有正式資料，migration 不回填、不轉換舊的同意紀錄。已套用的 migration 不得修改（`backend/AGENTS.md` 單體規則 6）。
- 繼承：外部呼叫不在 transaction 內、寫入意圖與排入 job 在同一個 transaction（`backend/AGENTS.md` 單體規則 5）。仍成立，登記資料查詢與核准後建立收款帳戶都照此。
- 不再成立：PayUni 子商店 API 的限制（MerID 一次性、`update_merchant` 只能改聯絡方式、會員 Email 綁定後不能改）：沒有 PayUni 註冊了，聯絡欄位上限、身分欄位鎖與店名鎖都是它的衍生物，一併拿掉。
- 不再成立：ADR 2026-07-20 的加密範圍（身分證號、銀行帳號加密、姓名生日明文）與「沿用 `pkg/crypto`、單一 master key」：沒有要加密的個資。`internal/platform/seal` 只剩 HMAC 金鑰環。
- 不再成立：PayUni webhook 的 inbox 在 Java 舊服務（ADR `2026-09-18-webhook-two-phase-transaction`）：單體不再收 PayUni 的任何事件；外部事件 inbox 由 platform 提供，屬收款帳戶 ADR。

## 考慮過的方案

| 決定 | 基準（業界做法） | 其他方案與判定 |
|---|---|---|
| 平台收哪些身分資料 | 身分由金流商驗證的市集不保存自然人身分與銀行資料，開通交給金流商的託管或內嵌流程（Stripe Connect，https://docs.stripe.com/connect/hosted-onboarding）。✓ 平台只收統編等市集登記資料 | 平台自收身分證號、銀行帳號、證件再轉送金流商（持有最多個資、兩方各驗一次）✗；留本名當市集登記資料（比對基準是賣家自己填的字，沒有驗證）✗ |
| 身分比對的依據 | 每個識別值各一個 keyed HMAC（blind index），金鑰獨立、帶 key id，先正規化再算（CipherSweet，https://ciphersweet.paragonie.com/internals/blind-index；AWS Database Encryption SDK 的 beacon）；金流商自己提供的銀行帳號 fingerprint 當第二個訊號（Stripe API 的 external account `fingerprint`，欄位出處未逐項核）。✓ | 統編加帳號合成一個 hash（只換一項就繞過）✗；第三方身分驗證或風控（外部帳號與費用，規模不需要）✗；fingerprint 命中也自動處分（家人或配偶共用帳戶是常態）✗ |
| 防詐雜湊的保存 | 資料最小化，保留的資料綁著保留的理由（GDPR Art.5(1)(c)(e)）：雜湊自己一張表、自己一個期限，不掛在個資清理上。✓ 5 年（法源未確認，推測自金流業慣例） | 延長申請雜湊的保留（綁著個資清理，保留理由與清理互相牽制）✗；永久保留（無理由）✗；舊三種類（身分證號、負責人身分證號、平台自收銀行帳號）照字面保留（永遠命中不了，後台還顯示標籤）✗；保留列並為它們加比對路徑（為只存在於種子的資料新增機制）✗ |
| 申請的雜湊表 | 同一份索引只存一處。✓ 刪申請的雜湊表與申請列的統編雜湊欄位，關聯查詢只連保存表；草稿「有沒有改」直接比明文統編 | 兩張表都留（同一批雜湊兩處、兩個清理時機）✗ |
| 命中的定義 | 一個規則只有一個定義，讀寫兩端不各寫一份（SSOT；無具名出處，推測）。✓ 規則只在 SQL | 同步時補寫處分快照（兩個寫入點，漏一個就默默少比對）✗；SQL 與 Go 各寫一份、以測試鎖住一致（兩份規則靠測試對齊）✗ |
| 共用銀行帳號的提醒 | 金流商自己做重複帳戶與詐欺偵測，平台另做的比對是加值（出處未核，推測）。✓ 推導的後台待辦，只存「已檢視」 | 只刪欄位、提醒併入上線前的防人頭措施（上線前沒補就沒有人被提醒）✗；同步時寫一筆待辦（待辦是推導的、不儲存）✗；另存提醒列（歷史沒有讀者，手寫開關是待辦 ADR 拒絕的形狀）✗ |
| 後台讀公司補充文件 | 一般審核附件以後台角色權限控管，理由必填加限時授權用在遮蔽的個資欄位（出處未核，推測）。✓ 有 KYC 讀取權限即可讀，每次讀取留紀錄，形狀同佐證的 `evidence_accesses` | 保留顯示全文與 15 分鐘授權只服務公司文件（每次審核都要填理由才看得到必看的文件）✗；連讀取紀錄也不留（被質疑誰看過時無從查）✗ |
| 核准與開通 | 人工決定與外部整合是兩件事；外部帳戶在核准後由獨立的 payment 模組建立，狀態以單一來源推導（Stripe，https://docs.stripe.com/connect/accounts-v2）。✓ | 核准同步呼叫外部服務、cert 狀態混整合結果、流程狀態 `KYCFlow` 把兩條軸合成一個 enum（三端各自推導）✗；管理員重新綁定（缺件由賣家經新的開通連結補）✗ |
| 店家聯絡欄位 | 必填由實際使用它的流程決定；信箱上限 254（RFC 5321 第 4.5.3.1 節）。✓ | 四個欄位規則照舊、只把理由改成平台（兩個沒有流程在讀的欄位擋住送出，上限是別家公司的欄寬）✗；聯絡人姓名與手機整個刪掉（後台聯絡賣家只剩信箱）✗ |
| 入駐同意 | 金流商的服務協議由金流商頁面收（Stripe 託管與內嵌流程收服務協議，https://docs.stripe.com/connect/hosted-onboarding）；平台只記自己的條款，記時間、IP、User-Agent（Stripe `tos_acceptance` 的做法）。✓ | 新增 provider 中立的 `PAYMENT_PROVIDER_TERMS`（沒有代收的需要）✗；每個 provider 一個種類（加 provider 就改 enum 與 CHECK）✗ |
| 個資保留期 | 目的結束後保留一段處理爭議的期間再刪，有法定義務的類別依法定年限；停用與刪除分開（Google Cloud soft deletion、Shopify 終止後保留、eBay 以限制處理代替刪除）。✓ 公開內容與下游資料 30 天，註冊、身分與連線資料 184 天 | 單一保留期 30 天（短於第 37 條的六個月）✗；整個 stint 一起 184 天（下游的聊天、商品刪除跟著晚 5 個月）✗；依資料類別分級加 `retention_until`（沒有法定年限可填）✗；關店當下立即清（售後、申訴、爭議看不到當事人）✗；crypto-shred 每店一把 DEK（要 KMS）✗ |
| 退回的表達 | 伺服器算「還要補什麼」，每項有欄位、原因碼與說明（Stripe `requirements.errors[]`、Sumsub `rejectLabels`、AIP-193 `FieldViolation`，https://google.aip.dev/193）。✓ | 一段理由加 `required_fields`（只回答哪個欄位，兩個 client 各自推導）✗ |
| 審核對照的登記資料 | 送件時自動查詢公司登記、存下結果並標出不一致，審核員看比對結果（Stripe Connect、Middesk、Persona 的 KYB）；查詢是有期限的資源，終態由伺服器負責（AIP-151，https://google.aip.dev/151）。✓ | 審核員開詳情時即時查、不存（核准依據沒有留存）✗；任意統編查詢的 admin RPC ✗；server-streaming（觸發：多位審核員或佇列頁要即時更新多筆）✗ |
| 舊的審核與開通事件 | 通知是獨立模組，依自己的設計消費領域事件（`2026-10-05-notification-module.md`；推測）。✓ 舊 `merchant.verification.*` 停發 | 保留舊事件為沒有部署的 Java 通知服務留接縫（開通結果與審核結果的通知由通知模組搬遷時重做）✗ |

## 決策

### 身分比對與比對 hash

- 平台不收身分證號、銀行帳號、證件影像、本名、英文姓名、生日、發證資料與銀行名稱（2026-11-02，Will）。統編是市集登記資料，仍由平台收並計算雜湊。觸發重建：決定接台灣金流商時，依當時金流商的欄位重新設計，舊程式只能從 git 參考。個人賣家沒有統編，不被身分比對；個人賣家的證書持有人比對沒有基準，留給商品模組（B4）決定轉人工或改用統編（2026-11-12，Will）。`TestKYCHasNoPersonalIdentityOrBankColumnsOrFields` 守住 KYC 的表與型別沒有這些欄位。
- 身分比對只有兩種依據（2026-11-02、2026-11-12，Will）：統編，與金流商回報的銀行帳號 fingerprint，各算一個 HMAC，輸入帶識別種類的標籤做 domain separation（統編 `kyc:ubn:v1|`、fingerprint `provider:bankfp:v1|<provider>:<fingerprint>`；明文 fingerprint 不落地）。金鑰環 `KYC_HASH_KEYS`（`seal.MAC`，每把至少 32 bytes、帶 key id，第一把計算、全部可查）必填，沒有加密金鑰。
- 統編命中「別的帳號有生效中的處分」時不能通過認證；fingerprint 命中只列出、不擋、不自動處分、不能據以建立關聯終止（`LinkedIdentity.CountsAsGround`；家人或配偶共用帳戶是常態）（2026-10-29，Will）。送出時先查，核准時在鎖住帳號的 transaction 內再查一次，以後者為準。回應用泛用文案、不透露命中的欄位或帳號，`SubmitKYC` 每個帳號每小時限 5 次（`internal/httpapi/seller` 宣告表的 `limitedTo`）。個人與企業負責人同一個人換角色不能繞過處分，這條隨身分證號一起消失，現在只擋同一個統編。
- 防詐雜湊獨立保存（2026-10-29，Will；2026-11-12 縮減）：`seller.account_identity_hashes`（來源 stint、欄位、金鑰 ID、hash、帳號、建立時間；沒有明文）只有兩種欄位：`TAX_ID`（申請的統編，由 `writeHashes` 在申請寫入的同一個 transaction 寫，統編被改掉時修剪該 stint 已沒有任何申請持有的值）與 `BANK_FINGERPRINT`（payment 同步 job 發 `PayoutAccountChanged`，seller 的訂閱者 `kyc.Fingerprints.RecordBankFingerprints` 在同一個 transaction 寫；fingerprint 是 provider 驗證過的事實，之後消失也不刪列）。不隨 184 天個資清理刪除；來源 stint 目前這次關店滿 `IdentityHashRetention`（5 年，固定 43824 小時，不看關閉原因）後由清理 job 刪，被來函保存或申覆 legal hold 擋下的略過，恢復營業的 stint 不算關閉。身分證號、負責人身分證號、平台自收銀行帳號三個舊種類與列舉值已刪。
- 命中的定義只寫在 SQL（2026-11-13，Will）：「帶身分效果的措施」是 view `identity_effect_measures`（停權，以及非預告的終止合作，含關聯帳號的終止；預告終止不禁止日後重新申請，不帶身分效果）；措施在時點 `at` 的身分來源是函式 `measure_identity_sources(at)`：措施建立時從被處分帳號的保存表複製的快照（`measure_identities`，措施結束或撤銷時刪），加上被處分帳號此刻仍在保存表的 hash；命中是函式 `measure_identity_hits(at)`，以 `(hash, key_id)` 等值連接、種類要對得上。送出與核准的比對、處分頁的關聯帳號 `LinkedAccountHits`、共用銀行帳號待辦都讀這一份，Go 不判斷。快照存在的理由是保存期（雜湊 5 年到期、終止合作永久），不是凍結在處分當下：處分之後才同步進來的 fingerprint 與統編仍然命中。
- 共用銀行帳號的提醒（2026-11-13，Will）：由 seller 的 SQL（`review_shared_bank_account`）從命中推導成後台待辦，不儲存；只存管理員「已檢視」這一對（措施、帳號）的處置 `measure_linked_reviews`。待辦的形狀與期限屬 `2026-10-20-admin-review-queue.md`，處分頁的關聯帳號屬 `2026-10-18-seller-governance-consolidated.md`。
- 金鑰輪替：新增金鑰後 `RehashKYCIdentities` 為缺新金鑰的申請補算統編（取自申請的明文欄位）；快照與已清理 stint 的列沒有明文，不補，舊金鑰留到措施結束、保存列被刪。啟動檢查 `CheckKYCKeys` 要求資料庫參照的每把金鑰（保存表、快照）都在金鑰環裡。
- 保存目的與期間寫進條款 21.3、賣家身分認證告知事項與隱私政策，落在個資法第 11 條第 3 項但書與施行細則第 21 條的「契約約定之保存期限」；業界只對被封禁者保留（Uber、蝦皮），不對所有關閉者。

### 申請、草稿與審核

- 申請以 `store_id` 為鍵，重新入駐要重做；`status` 只記人工決定（`DRAFT`、`SUBMITTED`、`APPROVED`、`REJECTED`、`CANCELLED`），沒有 `forwarded` 類的整合狀態。沒有申請以欄位不帶值表示，不另造狀態值（2026-11-12，Will）；`KYCFlow` 與 `KYCFlowStatus` 整個刪除。可否編輯只看申請狀態（`model.CanEditKYC`）：草稿與被退回的可編輯與送出，審核中只能取消，已核准的不能編輯、不能重送、不能取消；核准之後改登記資料另議（觸發：賣家要求）。
- 草稿與送出共用同一組具型別的欄位，必填由 `requirements` 與 CHECK `kyc_applications_complete_check` 表達；不收任意 JSON。申請只有市集登記資料（商品類型、網站、公司名稱、統編、公司登記日期、公司地址與電話）與公司補充文件；申請人不填組織型態（登記機關提供，存在快照）。送出另要求帳號有聯絡信箱（`CONTACT_EMAIL_REQUIRED`，來源屬收款帳戶 ADR）。後台審核佇列的列名是店名，搜尋涵蓋店名、公司名稱與統編。
- 退回帶逐項問題（2026-10-12，Will）：`KYCIssue{field, document_asset_id, code, message}`，`code` 是封閉的 `KYCIssueCode`（UNREADABLE、MISMATCH、EXPIRED、MISSING、INVALID、OTHER），`message` 必填 1 到 500 字，另有只給後台的 `internal_note`（至多 1000 字），總理由 `reason` 必填。不做 FINAL 退件，永久拒絕由停權處理。文件類欄位的原因碼規則與「哪個賣家型態可以對哪些欄位提問」（`model.IssuableFields`）各只有一處；`document_asset_id` 必須是該申請在那個欄位目前的文件。審核紀錄不存欄位值（`kyc_review_issues`）。
- 「目前還要補什麼」由伺服器算（2026-10-12，Will）：`outstanding_issues` 是最近一次退回裡賣家還沒處理的問題，已處理在寫入申請的同一個 transaction 判定（欄位值被改過，或指到的文件被移除或替換，記在 `kyc_review_issues.resolved_at`），讀取不推導；`last_rejection` 在重送、核准或取消之前都在。兩個 client 不自己推導。
- 審核的鎖順序：店家列（帳號全部的 stint）先於申請列，鎖內重新確認版本與狀態再寫決定、審核紀錄與事件；etag 的版本檢查先於狀態檢查。核准還要求店家營業中、申請人帳號沒有停權、身分沒有命中生效中的處分。事件（`KYCSubmitted`、`KYCApproved`、`KYCRejected`、`KYCCancelled`）在寫入的 transaction 內發出，`EventID` 是審核紀錄的 UUIDv7。

### 後台讀公司補充文件（2026-11-12，Will）

- 補充文件是市集審核用的公司證明（工廠登記證、保險、檢驗報告、授權書等），不是證件影像。有後台的 KYC 讀取權限（`adminperm.SellerKYCDocumentsGet`）就能讀，`GetKYCApplication` 直接帶每張的檢視網址與 `asset_id`；文件只以（申請、位置）定址，不以 asset ID。每次讀取先寫一列 `kyc_document_accesses`（誰、位置、實際送出的 asset ID、時間；不含內容）再讀，寫不進去就不送（fail closed），紀錄的形狀同 `evidence_accesses`、不掛授權。保留紀錄的理由：上傳內容不受限制，授權書可能帶個人姓名與簽名。
- 刪除：顯示全文 `RevealKYCSensitive`、`ListKYCReveals`、`kyc_reveals` 表、15 分鐘限時授權與 admin 的理由對話框。觸發重評：補充文件的規格改成收自然人證件。通用的 admin 機敏下載整條已刪（2026-10-04 的決定），機敏檔由各自的擁有者授權。

### 登記資料快照（2026-10-07、2026-10-12，Will）

- 只有公司型（有統編）的申請有快照。`SubmitKYC` 在同一個 transaction 排入 River job `seller.kyc_registry_check`；job 在 transaction 外呼叫 `gcis` client，把查詢時間與結構化的登記資料（統編、名稱、地址、設立日期、登記現況）存成該申請的一列 `kyc_registration_checks`，並逐欄記下與申請內容的不一致（NFKC、台／臺、空白正規化；地址先拆鄉鎮市區代碼，拆不開記「無法比對」）。查無或失敗記下結果，不擋送件；不一致與「現況不是核准設立」只標示、不自動退件（觸發改自動規則：送件量超過人工逐筆審核能處理的量）。
- 每次查詢一列，寫入結果後不再改；後台重新查詢（只在審核中）以新的一列取代目前顯示的那一列，舊列保留。核准與退回在 `kyc_reviews` 記下依據的查詢；後台 etag 由申請版本加快照序號組成（AIP-154），審核員看過之後快照變了，用舊 etag 核准或退回回 `aborted`。決定之後才開始的 job 放棄、記成 `CANCELLED`。
- 查詢是有期限的資源：建立時寫入 `deadline_at`（15 分鐘，互動情境的決定，不由 River 退避反推），job 執行時已過期限記 `UNAVAILABLE`、不呼叫資料來源；periodic job `seller.kyc_registry_sweep`（每 1 分鐘）把到期的 PENDING 收成終態並補排閒置超過 3 分鐘的 job，所以終態一定在期限之前。契約 `pending_registration_check` 帶 `deadline_time`，client 依它以退避輪詢。快照與 KYC 個資同時清除。

### 核准與收款開通分開（2026-11-02，Will）

- 核准只寫人工決定、審核紀錄與事件；同一個 transaction 呼叫 payment 的 `RequestAccount` 寫「建立中」的收款帳戶列並排入建立 job，外部呼叫不在 transaction 內。收款帳戶、provider、開通狀態與閘門副本 `payout_capabilities` 的決策見 `2026-11-13-payment-onboarding-provider-port-stripe.md`，本檔不重複。
- 刪除：PayUni 註冊列與綁定 job、重新綁定、webhook 的觀測與 `payuni_sweep`、`KYCFlow`／`KYCFlowStatus` 與 `can_resubmit`、身分欄位鎖與店名鎖（`IdentityLocked`、`CheckNameUnlocked`）、`PAYUNI_BIND_RETRY` 待辦與管理員的 PayUni RPC。店名是平台自己的欄位，不同步給金流商。
- 舊事件 `merchant.verification.*` 與舊的 PayUni 審核通知停發（`internal/legacy/sellerrelay` 只轉店家生命週期、停權、資料與曝光五種事件）；開通結果與平台審核結果（核准、退回）的通知，留給通知模組搬遷時依新的設計重做。物流用的商店代號與舊契約的 `payuni_mer_id` 回空字串，本機的物流與付款端到端在對應模組搬進單體前不通（`TestMerchantIDFieldsAreEmpty`）。

### 店家聯絡欄位（2026-11-12，Will）

- 店家是聯絡資料的唯一來源：`stores.contact_name`、`contact_mobile`、`support_email`、`contact_phone`，申請上沒有聯絡副本。地址與客服信箱送出前必填，申請在審核中或已核准時不能清掉（物流要用）；聯絡人姓名與手機選填、可清掉。上限是平台自己的：客服信箱 254、聯絡人姓名 50 字（手機格式不變）。送出時申請齊全之後店家資料還缺回 `failed_precondition`，缺哪些由 `missing_store_fields` 回報。
- 舊決定「聯絡資料與地址走同一條路同步給 PayUni」整段作廢：沒有同步端；收款通知寄到帳號的聯絡信箱，不是客服信箱。

### 入駐同意紀錄（2026-10-07，Will；2026-11-02 縮減種類）

- 種類只有 `SELLER_TERMS`（開店 `ApplyStore`）與 `KYC_PRIVACY_NOTICE`（送出 KYC `SubmitKYC`）；多一種、少一種、重複都是 `invalid_argument`。金流商的服務協議由金流商頁面收，平台不記；舊的 `PAYUNI_TERMS` 刪除、不轉換舊紀錄（未上線）。接台灣金流商而平台要代收同意時再加種類。
- 條文與版本只有一個來源：`website/docs/legal/` 的正本加 manifest `seller-agreements.json`，產生器 `website/scripts/generate-legal.ts` 輸出 web 的 TS 投影與 Go 的 `model.CurrentAgreements`（`agreement_gen.go`），CI `legal-generated-diff.yml` 比對。不記內容雜湊：版本加 repo 正本就能還原條文；填入營運主體等佔位符就是條文改變，版本要遞增。
- 伺服器以版本核對：client 送回它顯示的版本，不同回 `failed_precondition`（`AGREEMENT_OUTDATED`）；App 讀免登入的 `GetSellerAgreements`。
- 每次同意事件一列 `seller.agreement_acceptances`（沒有 `ON CONFLICT`）：開店那一份每家店一列（部分唯一索引）；KYC 的告知每次送出各一列，以 `submission_review_id` 綁定那次送出的 SUBMIT 審核紀錄（CHECK 要求 KYC 的列一定帶、其他一定不帶），與開店、送出在同一個 transaction。欄位：店家、種類、版本、帳號、時間、來源 IP（`inet`）、User-Agent（上限 512 字）；不記 `client`。
- 寫入路徑只有 INSERT、第二段清理把 IP 與 User-Agent 清成 NULL（`agreement_acceptances_purged_check`），以及 dev seed 的重設。

### 關店與個資保留（2026-10-15，2026-10-19、2026-10-20 改保留期，Will）

- 關店可逆，不可逆的點在清理：關店只記為什麼關的（`store_closures`，每次關店一列，`reopened_at` 為 NULL 的是目前這次，稽核永久保留；`closed_by` 是 `ACCOUNT_OWNER`、`TERMINATION`、`ACCOUNT_DELETION`，終止另記 `closed_by_measure_id`）、結束預告終止、移除 SELLER、發 `StoreClosed`；所有關店走同一條路徑 `closing.close()`。logo 與封面留在店家列到第一段才釋放。relay 不轉 `StoreClosed`，第一段的 `StoreRetired` 才轉成 `seller.deleted`（下游的刪除因此晚到第一段）。
- 兩段清理，不是一個保留期（2026-10-20，Will）：**第一段（退場）**在關店後 `model.StoreRetireAfter`（720 小時，30 天）清公開內容（說明、獎項、網站）、釋放 logo 與封面、發 `StoreRetired`，記 `stores.retired_at`；退場之後不能恢復營業。**第二段（個資清理）**在已退場、關店後 `model.StoreRetention`（4416 小時，184 天，任何連續六個曆月的最大長度，所以不論哪天關店都不短於第 37 條的六個月；用固定時數，因 timestamptz 加日數依 session 時區）清店名、店家聯絡與地址、KYC 申請的個人欄位與證件（media 引用經 `seller.media_sync` 解除）、登記資料查詢、同意紀錄的 IP 與 User-Agent，記 `pii_purged_at`；收款帳戶由 payment 擁有，清理的 transaction 只呼叫 `ForgetOwners` 排入它的刪除。保留：審核的事實（誰、何時、決定）、`seller_type`／`product_type`、措施的身分快照、防詐保存表（5 年）、管理員的稽核文字。沒有密文、證件影像與金流商註冊列要清。
- `TERMINATION` 關閉的，兩段都取 `greatest(關店後的時數, 帳號上未撤銷終止中最晚的申覆期限)`（申覆期限是終止生效後 `AppealWindow`，4416 小時，不短於條款與 DSA 的六個月；已撤銷的終止不延長）。保住條款 21.6「終止後三十日內請求資料」。時間條件只有 `RetireDueStores` 一支查詢（兩段與恢復營業共用，`second_stage` 選段），時數由 Go 常數傳入；資料庫只保證結構（`stores_retired_check`、`stores_closed_images_check`、`stores_pii_purged_check`、`kyc_applications_purged_check`）。
- legal hold（2026-10-15，2026-10-19）：帳號上未撤銷的終止合作有待處理的申覆（或關聯終止的來源措施有待處理的申覆）時兩段都不清、也不恢復營業（eBay 的 restrict the processing、GDPR 17(3)(e)）；主管機關來函的保存（`hold_until`）擋兩段，不影響恢復營業。兩種保存只有 `StoreHolds` 一支查詢定義；清理在鎖住候選之後再判斷時間並問保存，寫入保存的一方先鎖店家列（競態證明見治理 ADR）。
- 清理是每小時的 River periodic job（`seller.purge_pii`，啟動即執行），每批一個 transaction、迴圈到取不滿，重跑是 no-op；第三項是刪過期的防詐保存列。清理後撤銷終止只撤銷紀錄與禁止入駐（快照刪除，可開新店，新的 `store_id`），不恢復店家。

## 後果與重新評估

- 停權的人用別的統編（例如人頭）仍能入駐；這道防線只擋同一個統編。個人賣家沒有統編、完全不被身分比對，fingerprint 命中只列出。接受：身分層級的防重複開通交給金流商。出現大量換身分重來的濫用時，評估第三方身分驗證，或上線前的裝置與聯絡方式重疊等防人頭措施。
- 平台審核不再核對任何自然人證件：個人賣家的審核只看市集政策。個人賣家證書持有人沒有比對基準（`MatchCertHolder` 先比本名、現已無本名），商品模組（B4）搬進單體時決定轉人工或改用統編比對。
- 刪掉的雜湊與本名無法重算；未上線、只有種子資料，不構成損失。決定接台灣金流商時，平台自收的管線要依當時金流商的欄位重新設計（git 可找回舊程式）。
- 防詐保存的保存期法源未確認（5 年，推測自金流業慣例）；6 個月這個值請律師確認；上線前核對條款 21.3、告知事項與隱私政策的保存目的與期間。四份法律文字仍寫「關店後 30 天」處要改成兩段（公開內容與下游資料 30 天，註冊與身分資料 6 個月）。
- live mode 的銀行帳號 fingerprint 範圍（同一個銀行帳號在不同帳戶是否一致）只在測試模式實測；上線前核對，範圍不同則共用銀行帳號的比對要重評。
- 快照到措施結束之前，對應的舊 HMAC 金鑰不能從金鑰環移除（啟動檢查擋下）；保存表的舊金鑰列要留到來源 stint 關店滿 5 年。
- 保留期、申覆期間與來函保存期同值（4416 小時）只靠設計決定，沒有程式強制（model 的 unit test 釘住數值），三者法律依據不同所以是三個常數。主管機關來函的涵蓋狀態只看第二段，已退場而個資還在的店顯示「保存中」，下游資料已刪不另標（觸發：機關第一次要求調取聊天或商品資料而店家已退場）。
- 已核准後不能改登記資料，也沒有 `pending_agreements`：已入駐的賣家在條款版本更新後重新同意、換主體的處理，在系統上線之前或文件版本第一次在有真實賣家之後改版時加。同時有多位審核員或佇列頁要即時更新多筆時，登記資料查詢改用 server-streaming。
- 重新評估：法務給出 seller 範圍內其他類別的法定保存年限時，改為依資料類別分級（`retention_until`）；補充文件的規格改成收自然人證件時，加回限時授權。
