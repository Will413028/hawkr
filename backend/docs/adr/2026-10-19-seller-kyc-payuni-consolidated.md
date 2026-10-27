# 單體裡的賣家 KYC 與 PayUni（整合版）：身分比對、非同步綁定、登記資料快照、入駐同意、個資保留

| 項目 | 值 |
|---|---|
| 狀態 | accepted（整合 `2026-10-02-seller-kyc-payuni-in-monolith.md` 的原決策與 8 次修訂；整合不改任何決策，2026-10-19。舊檔沒寫、照現行程式補上的幾條各附出處；文末 2026-10-20 修訂把關店後的清理改成兩段） |
| 日期 | 2026-10-19 |
| 影響範圍 | `internal/modules/seller`（KYC、PayUni、登記資料查詢、入駐同意、關店與個資清理）、`internal/platform/seal`、`internal/platform/payuni`、`internal/legacy`（PayUni webhook 的 consumer、`merchant.verification.*` 與 `seller.deleted` 的 relay）、admin、website、seller-app |
| 取代 | `2026-10-02-seller-kyc-payuni-in-monolith.md`（本檔取代它，它標 `superseded by` 本檔）。原 ADR 修訂 `2026-07-20-seller-kyc-pii-encryption-scope.md` 的加密實作（範圍不變），本檔承接這個修訂 |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 7c、10a、10b、10c、10e |

為什麼要整合：原 ADR 修訂超過 3 段（`adr/README.md`），讀者要逐段對照前文才知道現況。下面只寫現行決策；括號內的日期是該決策來自哪一次修訂（原決策是 2026-10-02）。前後修訂互相取代的地方，一律以較晚的為準。只描述當時開發資料怎麼搬的 migration 步驟不收（系統未上線，migration 都不回填；見 seller 的 `CARRYOVER.md` 與舊檔）。

## 脈絡

KYC 認證與 PayUni 子商店從 seller-service 搬進 seller 模組。舊實作的盤點（7c 開工時，快照 26ae7fad0）找到的問題多半來自結構：Approve 在請求內同步呼叫 PayUni（profile 更新甚至在 DB transaction 內、持有 advisory lock 時呼叫，審核的 `FOR UPDATE` 不在 transaction 內）；cert 狀態機混了 PayUni 的整合結果，重試只能「再核准一次」；草稿是自由格式 JSON，遮蔽函式被它繞過；`raw_request` 存明文身分證號，webhook 寫入失敗照樣 ack，亂序的舊通知會蓋掉新狀態；沒有任何地方比對身分證號或銀行帳號，被停權的人換帳號就繞過處分；關店後的個資只清一部分、每天最多 100 筆。之後的修訂各自補上：證件影像繞過稽核的通用下載（2026-10-04）、審核員沒有登記資料可以對照（2026-10-07）、PayUni 聯絡資料有兩份副本（2026-10-07）、入駐同意沒有紀錄（2026-10-07）、退回只說哪個欄位不說為什麼（2026-10-12）、登記資料查詢可能永遠停在 PENDING（2026-10-12）、關店不可逆（2026-10-15）、保留期的法定下限（2026-10-19）。

約束：
- 外部：PayUni 子商店 API（MerID 一次性；AGENT04017 表示已綁定，可查詢後認領；`update_merchant` 只能改聯絡方式、地址與支付設定，會員 `Email` 綁定後不能改，代理商文件 §7.3、§8）。PayUni 是支付機構，金流 KYC 的法定保存由它負責；開立發票的稅務紀錄由 invoice 模組自己保存快照。
- 外部：詐欺犯罪危害防制條例第 37 條第 1 項（合理期間保存足以重建使用者與個別交易的資料）與第 2 項（調取資料保存六個月）；6 個月這個值上線前請律師確認（2026-10-19）。P2B 第 4 條第 3 項（撤回終止時恢復並交還生效前的資料）、DSA 第 20 條第 1 項（申訴期至少六個月）（2026-10-15）。
- 繼承：KYC 的加密範圍（身分證號、銀行帳號加密；姓名、生日明文供後台模糊搜尋），計畫決定第 5 項。仍成立。
- 繼承：停權以帳號為單位、由措施在讀取時推導（ADR `2026-10-18-seller-governance-consolidated.md`）。仍成立，身分比對用同一個推導。
- 繼承：PayUni webhook 的 inbox 在 Java `payuni-webhook-service`（ADR `2026-09-18-webhook-two-phase-transaction`），到 payment 搬進單體前不變。
- 不再成立：ADR 2026-07-20 的「沿用 `pkg/crypto`、單一 master key」與「密文存 Base64 varchar」——當時是為了與既有服務一致，新 schema 沒有資料要搬。

## 考慮過的方案

| 決定 | 基準（業界做法） | 其他方案與判定 |
|---|---|---|
| 身分比對 | 每個識別值各一個 keyed HMAC（blind index），金鑰與加密金鑰分開、帶 key id，先正規化再算（CipherSweet、AWS Database Encryption SDK 的 beacon）。代價：輪替要解密後重算；hash 仍是個資，保留要有理由。✓ | 身分證號加帳號合成一個 hash（只換一項就繞過）✗；確定性加密 AES-SIV（與隨機 nonce 的欄位加密兩套）✗；第三方身分驗證或風控（外部帳號與費用，規模不需要）✗ |
| 比對 hash 與個資清理的衝突 | 資料最小化，保留的資料綁著保留的理由（GDPR Art.5(1)(c)(e)）：處分擁有身分快照，處分結束就刪，KYC 列照常清理。代價：快照沒有明文可以重算，舊 HMAC 金鑰要留到這些處分結束。✓ | hash 留在 KYC 列、有生效處分時清理跳過（保留理由在別的表，清理要懂處分）✗；hash 永久保留（與清理矛盾）✗ |
| 個資保留期 | 目的結束後保留一段處理爭議的期間再刪，有法定保存義務的類別依法定年限；停用與刪除分開、刪除走保留期（Google Cloud soft deletion、Shopify 終止後保留、eBay 以限制處理代替刪除）。✓ 關店後 184 天（2026-10-19），終止合作關店另外不早於申覆期限（2026-10-15） | 關店後 30 天（原決策；短於詐欺犯罪危害防制條例的六個月）✗（2026-10-19）；拆成兩段清理（註冊、身分與連線資料 6 個月、其餘 30 天：兩個保留期、兩份法律文字，沒有對應的法定要求）✗（2026-10-19，Q2 (a)）；依資料類別分級加 `retention_until`（沒有法定年限可填，年限是編的）✗；關店當下立即清（售後、申訴、KYC 爭議看不到當事人）✗；crypto-shred 每店一把 DEK（要 KMS，規模不需要）✗ |
| 核准與 PayUni 綁定 | 人工決定與外部整合是兩件事；寫入意圖與排入 job 在同一個 transaction，外部呼叫在 transaction 外、以冪等鍵重試（backend 規則 5）。代價：核准變成非同步，畫面顯示合成的流程狀態。✓ | cert 狀態加 `PENDING_BINDING`（cert 軸繼續混整合結果）✗；先同步呼叫、逾時轉背景（兩套行為）✗；沿用舊 outbox 與自建 worker（與 River 重複）✗ |
| 後台看完整證號與證件 | 預設遮蔽，需要時以獨立動作顯示並留稽核；證件影像 just-in-time：擁有者授權、綁定用途、限時、每次讀取都留紀錄（2026-10-04）。✓ | 照舊完整顯示不留紀錄 ✗；永遠不顯示（審核要核對證件）✗；雙人覆核（規模不需要）✗；通用 admin 機敏下載以 asset ID 讀任何檔（不檢查歸屬、不記管理員）✗（2026-10-04） |
| 審核時對照的登記資料 | 送件時自動查詢公司登記、存下結果並標出不一致，審核員看比對結果（Stripe Connect、Middesk、Persona 的 KYB）。✓（2026-10-07） | 審核員開詳情時即時查詢、不存（每次打外部服務，核准依據沒有留存）✗；以任意統編查詢的 admin RPC（與申請脫鉤的查詢器）✗ |
| 登記資料查詢的終態 | 長時間工作的狀態放在資源上，終態由伺服器負責，client 以 Get 輪詢（AIP-151、Stripe `requirements.pending_verification`）。✓（2026-10-12，選項 (b′)） | 「重新查詢就是出口」（job 遺失留下沒人收得掉的 PENDING）✗；正式的 `Operation` 資源（只有一種工作，與查詢列存兩份）✗；server-streaming（觸發條件：多位審核員或佇列頁要即時更新多筆）✗ |
| PayUni 聯絡資料的擁有者 | 資料以平台為準，平台以 API 更新金流商的帳號，沒有另一個「金流聯絡資料」編輯器（Stripe Custom、Shopify Payments）。✓ 店家（2026-10-07） | KYC 申請收、核准時複製到註冊列（兩份副本：錯誤指向不會送出的欄位、重新核准蓋掉賣家改的值、同一個 email 兩個來源）✗ |
| 退回的表達 | 伺服器算「還要補什麼」，每項有欄位、原因碼與說明（Stripe Connect `requirements.errors[]`、Sumsub `rejectLabels`／`moderationComment`／`clientComment`、AIP-193 `FieldViolation`）。✓（2026-10-12，選項 (c)） | 一段理由加 `required_fields`（只回答哪個欄位；公司文件合併成一個值；兩個 client 各自用時間戳推導）✗ |

## 決策

### 身分比對與比對 hash

- 身分證號（個人與企業負責人用同一個標籤 `ID_CARD`，同一個人；所以一個帳號以個人身分被處分後，同一個人在別的帳號當企業負責人也會命中。舊檔寫的是每個欄位各自的標籤；Will 2026-10-20 決定維持現行程式的行為（`internal/model/kyc.go` 的 `ID_CARD` 標籤與 `model.IdentityInput`、`db/queries/kyc.sql` 的身分比對查詢，從 7c 第一版起）：KYC 的身分比對依證件號碼、不分角色，換角色不能繞過處分的阻擋）、統編、銀行帳號（連同銀行代碼）各算一個 HMAC，輸入帶識別種類的標籤做 domain separation（`model.IdentityInput`）；HMAC 金鑰環獨立於加密金鑰（啟動時 `seal.Distinct` 檢查不同）。
- 身分證號與統編命中「別的帳號有生效中的處分」時不能通過認證；銀行帳號命中只在審核畫面標記，不擋（家人或配偶共用帳戶是常態）。送出時先查，核准時在鎖住帳號的 transaction 內再查一次，以後者為準。回應用泛用文案、不透露命中的欄位或帳號，並限流（每個帳號每小時 5 次；舊檔只寫「限流」，次數取自 `internal/httpapi/seller/declaration.go` 的 `SubmitKYC` 宣告 `limitedTo(5, time.Hour)` 與 seller `AGENTS.md`「已知限制」的身分比對一條）。處分的判斷用 `stint` 的推導，不另寫。
- 身分索引（鑰匙輪替，2026-10-15）：申請與措施快照都存金鑰環每一把金鑰算的 `(key_id, hash)`（`kyc_identity_hashes`、`measure_identities`），關聯帳號的反查以索引等值連接，不解密申請。新增金鑰之後，既有申請缺新金鑰的列由 `RehashKYCIdentities` 在啟動時補算（只解密缺列的申請）；快照沒有明文可以重算，不補，所以舊金鑰留到措施結束。申請上的舊金鑰列只隨該申請的個資清理整列刪除。啟動檢查 `CheckKYCKeys` 要求資料庫參照的每一把金鑰都在金鑰環裡，缺了就不啟動。
- 比對 hash 的保留：限制性措施建立時（立即生效的停權與終止合作，含關聯帳號；預告終止不禁止日後重新申請，不取；舊檔寫「處分生效時、帳號的 hash」，現行做法來自 ADR `2026-10-18-seller-governance-consolidated.md`「終止合作」的身分快照一條與 `queries/kyc.sql` 的 `SnapshotMeasureIdentities`）把該帳號全部 stint 的 KYC 身分 hash 快照到措施擁有的 `measure_identities`，措施結束或撤銷時刪除；KYC 列的 hash 隨 KYC 個資一起清理，快照不受清理影響。第 21.3 條終止的快照保留到終止撤銷為止；清理之後才發現的 21.3 事由，快照取不到舊 stint 已清理的 hash，是接受的缺口（2026-10-15，W8）。保存的目的與期間寫進條款 21.3、賣家身分認證告知事項第 4 項與隱私政策，落在個資法第 11 條第 3 項但書與施行細則第 21 條的「契約約定之保存期限」；業界只對被封禁者保留（Uber、蝦皮），不對所有關閉者。

### 加密

- identity 的 `seal`（key id 前綴、可輪替）提到 `internal/platform/seal`，identity 與 seller 共用；AAD 是「申請 ID 與欄位名」，防止密文在列與欄之間搬動；密文存 `bytea`。加密範圍照 ADR 2026-07-20。

### 申請、草稿與審核

- 申請以 `store_id` 為鍵：重新入駐要重做 KYC。cert 狀態只記人工決定（草稿、送出、核准、退回、取消），沒有 `forwarded`、`forward_failed`。
- 草稿與送出共用同一組具型別的欄位，必填由狀態決定的 CHECK 表達；不收任意 JSON。`raw_request` 不存。
- 申請人不填組織型態（2026-10-07，10b）：`company_type` 沒有送給 PayUni，組織型態由登記機關提供，審核畫面顯示快照裡的值（登記資料快照存 `organization_type`）；契約的 `business.company_type` 是 reserved，申請上的 DB 欄位與比對欄位已刪（10b 只拿掉比對，快照的欄位保留）。型態開始影響要收的文件或 PayUni 的參數時，再以封閉列舉加回。
- 退回帶逐項問題（2026-10-12）：`KYCIssue{field, document_asset_id, code, message}`，`field` 是 `KYCField`，`code` 是封閉的 `KYCIssueCode`（UNREADABLE、MISMATCH、EXPIRED、MISSING、INVALID、OTHER），`message` 必填 1 到 500 字。不做 FINAL 退件，永久拒絕由停權處理。`RejectKYCRequest` 與 `KYCReview` 帶 `issues` 與只給後台的 `internal_note`（選填、至多 1000 字），總理由 `reason` 必填；賣家端的 `KYCRejection` 只有總理由、`issues` 與決定時間。審核紀錄不存欄位值（`kyc_review_issues` 只記欄位路徑、asset ID、原因碼、說明）。PayUni 的問題模型不併入（`PayUniProblem` 的 code 由 PayUni 擁有，是開放集合）。
- 文件類欄位的原因碼規則（2026-10-12，取代「缺少文件只標欄位」的口頭規則）：`UNREADABLE`、`EXPIRED`、`MISMATCH`、`INVALID` 必須帶 `document_asset_id`，`MISSING` 不可以帶，`OTHER` 兩者皆可，非文件類欄位不可以帶；違反回 `invalid_argument`，FieldViolation 指到 `issues[i].document_asset_id`（`model.IssueAssetRequired`／`IssueAssetForbidden`，`validateIssues` 一處）。`document_asset_id` 只能是管理員先前顯示全文後在 `KYCDocumentView.asset_id` 看到的值；遮蔽檢視的文件清單不帶 asset ID 與影像網址。後台的退回對話框照同一條規則限制選項（要求指定文件的原因在還沒顯示全文時說明要先顯示全文；缺少不給選文件）。
- 「哪個賣家型態可以對哪些欄位提出問題」只有 `model.IssuableFields`：`RejectKYC` 的驗證與後台 `KYCApplication.issuable_fields` 共用（2026-10-12）。後台契約沒有證件位置的列舉：申請上的文件、顯示全文的檢視網址與證件讀取紀錄都以文件類欄位（`KYCField`）加順序表示，證件位置只留在模組內部（檔案端點的路徑 `/api/files/admin/…/{slot}/{position}` 與 `kyc_document_accesses` 的鍵，`view_uri` 對 client 是不透明的）。
- 「目前還要補什麼」由伺服器算（2026-10-12）：`KYCApplication.outstanding_issues` 是最近一次退回裡賣家還沒處理的問題，重新送出、核准或取消之後為空；「已處理」在寫入申請的同一個 transaction 判定（欄位的值被改過，或指到的文件被移除或替換），記在 `kyc_review_issues.resolved_at`，讀取不推導。`last_rejection` 只在最近一次審核紀錄是退回、之後沒有重送、核准或取消時才回——賣家處理完每一項、還沒重送時它仍在，直到重送為止（2026-10-12，2b-0）；後台的 `GetKYCApplication` 仍以審核紀錄（`reviews`）看完整的歷史。兩個 client 不自己推導，直接看 `last_rejection` 與 `outstanding_issues`。「值有沒有變」只有寫入路徑一個來源（比明文與正規化後的值，不比密文或 blind index），與「PayUni 已收下之後不能改身分欄位」的檢查共用。

### 後台顯示全文與證件影像

- 身分證號與銀行帳號預設遮蔽；賣家自查的銀行帳號也遮蔽。`RevealKYCSensitive` 是唯一的授權動作（2026-10-04）：理由必填，寫一列 append-only 稽核（誰、哪筆申請、何時、理由，不含值），同一列同時是 15 分鐘的證件檢視授權，涵蓋欄位全文與證件影像。
- 證件影像只經 seller 擁有的端點，以（申請、證件位置 slot、順序）為鍵、不以 asset ID（`internal/httpapi/seller/admin_document.go` 的路由 `{slot}/{position}`、`kyc_document_accesses.slot`；slot 不等於 `KYCField`：個人與負責人的身分證共用一個 slot，對 client 只露出不透明的 `view_uri`，見上一節）；只服務持有這筆申請未到期授權的那位管理員；每次讀取先寫一列存取紀錄（含實際送出的 asset ID）再讀，寫不進去就不送。KYC 證件只接受機敏上傳的資產。
- 通用的 admin 機敏下載與預覽整條刪除（gateway 只剩 App 的機敏上傳與讀取）；其他機敏檔（發票稅籍文件）由各自的擁有者授權。誰可以顯示全文由後台的逐方法權限決定（`seller.kyc_reveals.create`，`platform/adminperm`；角色與權限的對應見 ADR `2026-10-04-admin-roles.md`，目前只有 OWNER 角色，擁有全部權限）。

### 登記資料快照（2026-10-07，2026-10-12 修訂期限）

- 只有公司型（有統編）的申請有快照。`SubmitKYC` 在同一個 transaction 排入 River job；job 在 transaction 外呼叫 `gcis` client，把查詢時間與結構化的登記資料（統編、名稱、地址、設立日期、登記現況）存成該申請的一列查詢，並逐欄記下與申請內容的不一致。查無或失敗記下結果，不擋送件。不一致與「現況不是核准設立」只標示、不自動退件；觸發改成自動規則的條件是送件量超過人工逐筆審核能處理的量。
- 每次查詢一列（`seller.kyc_registration_checks`，送件與後台重新查詢各新增一列），寫入結果後不再改；重新查詢以新的一列取代目前顯示的那一列，舊列保留，審核紀錄才能引用它當依據。只在審核中可以重新查詢。
- 核准與退回以當下的快照為依據：`kyc_reviews` 記下依據的查詢（沒有快照記「無」）；後台 KYC 檢視的 etag 由 KYC `version` 與快照序號組成（AIP-154），快照在審核員看過之後變了，用舊 etag 核准或退回回 `aborted`；賣家端的 etag 不受影響。沒有快照或結果是不可用時允許審核，畫面標示。決定之後，已在執行的查詢照樣寫入結果，不改變已記下的依據；決定之後才開始的 job 放棄查詢、記成 `CANCELLED`（舊檔原決策寫「決定之後才回來的結果照樣寫入」，它自己的 2026-10-12 修訂以「與認領時放棄的語意一致」改成這條；見 seller `AGENTS.md` 的登記資料快照一條與 `CARRYOVER.md`「查詢停在 PENDING 沒有出口」列）。
- 比對：名稱與地址明細以 NFKC、台／臺、空白正規化後比較（組織型態只存在快照、不比對，10b）；地址先拆出鄉鎮市區代碼，拆不開記「無法比對」，不算不一致；設立日期任一方缺漏記「無法比對」。「查詢當天是否在停業期間」由 `model.Registration.SuspendedAt` 一處算（台北日期，含起迄兩天），`KYCRegistration.suspended_at_query` 帶結果。
- 查詢是有期限的資源（2026-10-12）：建立時寫入 `deadline_at`（`requested_at` 加 15 分鐘，NOT NULL）。期限是互動情境的決定，不由 River 退避公式反推；River 的重試照舊、不釘住預設值（上限 8 次只防失控；預設退避下 15 分鐘內實際只跑到前幾次嘗試）。job 執行時已過期限就把查詢記成 `UNAVAILABLE`，不呼叫資料來源；結果只在查詢仍是 PENDING 時寫入。periodic job `seller.kyc_registry_sweep`（啟動即執行、每 1 分鐘）把 `deadline_at` 不晚於「現在加一個掃描間隔」還在 PENDING 的查詢收成終態（仍屬這一輪審核、沒被取代的記 `UNAVAILABLE`；申請已退回、核准、取消、重送過或被取代的記 `CANCELLED`），所以終態一定在期限之前；再把閒置超過 3 分鐘的 PENDING 補排 job。兩支掃描查詢走部分索引 `kyc_registration_checks_pending (deadline_at) WHERE outcome = 'PENDING'`。`seller.kyc_registry_check` 以 `check_id` 為 River 唯一鍵（`ByState` 涵蓋還活著的狀態：available、pending、scheduled、running、retryable），補排不與活著的 job 重複，被丟棄的才補得回來。
- 契約：`KYCApplication.pending_registration_check`（`KYCRegistrationCheck`，含 `request_time`、`deadline_time`；沒有進行中的查詢時不設），`RequeryKYCRegistrationResponse` 回這筆查詢。client 依 `deadline_time` 以退避輪詢到終態或期限（頁籤隱藏時不輪詢），查詢中可以停用「重新查詢」，畫面寫「最晚 HH:mm 會有結果」；超過期限仍沒有終態是伺服器的錯誤，畫面改提示重新查詢。
- 快照與 KYC 個資同時清除：它的用途是存續期間的審核依據。

### 核准與 PayUni 綁定

- 綁定狀態在 PayUni 註冊列上，那一列就是 intent。核准在同一個 transaction 寫入決定、綁定意圖、排入以 `store_id` 為唯一鍵的 River job，並發出事件；job 在 transaction 外呼叫 PayUni，暫時性錯誤由 River 重試，永久錯誤記為 FAILED，由管理員「重新綁定」（`RebindPayUni`）。地址與聯絡欄位的同步走同一個 job。畫面顯示兩條軸合成的流程狀態（`model.DeriveKYCFlow`）。`payuni_sweep` 保證 `bind_status` 到終態（BOUND 或 FAILED）；admin 只對 `bind_status=PENDING` 做退避輪詢（沒有次數上限，間隔上限 60 秒）（2026-10-12）。
- PayUni 拒絕之後：MerID 在 PayUni 端不是終止狀態時，已核准的申請可以回到草稿修正後重送（`Editable`、`CanResubmit`；有 MerID 之後身分欄位仍鎖住，因為 `update_merchant` 改不了它們）；MerID 已是終止狀態（`needs_reonboarding`）時只能關店重新入駐。這一條取代舊檔「後果」的「PayUni 拒絕只能關店重新入駐」：出處是 `CARRYOVER.md`「`deriveOverallStatus`／`deriveCanResubmit`／`deriveApprovalStatus`」與「PayUni 會員層級拒絕之後重新送出再核准」兩列（7c 審查）、計畫書 10b「實作中的決定」E4（Will 2026-10-07：`SubmitKYC` 直接接受 `flow.can_resubmit` 為真的申請）與 `step10b-decision-brief.md` K4（Will 照做：`needs_reonboarding` 只代表 MerID 終止）。
- PayUni webhook：legacy consumer 把通知轉成以 `event_key` 為唯一鍵的 River job，持久化後才 ack；套用時追加觀測，只有 PayUni 的 `ExecuteTime` 比目前狀態新才更新。

### PayUni 聯絡資料的擁有者是店家（2026-10-07，10c 第 0a 刀）

- 店家是唯一來源：`stores.contact_name`（至多 20 字，PayUni 的 `ContactName`）、`contact_mobile`（`09` 開頭共 10 碼）、`support_email`（綁定時同時是會員的 `Email`，企業的 `MBCMail` 也用它）；`contact_phone` 是商店電話（`MerTel`）。KYC 申請與註冊列沒有聯絡副本，沒有 `UpdateMyPayUniProfile`。`ContactTel` 不複製，綁定與同步時讀註冊所指申請的 `company_phone`。
- 聯絡資料與地址走同一條路：`UpdateProfile` 改了其中任何一欄，在同一個 transaction 登記同步 intent、排入同一個 job；核准不重設聯絡資料。`ApplyStore` 不帶這三欄，由 `UpdateMyStore`（KYC 頁與店家設定）寫入。綁定之後只送 `MerEmail`（會員 `Email` 改不了）。
- 送出 KYC 時檢查店家的綁定資料（地址、聯絡人姓名、聯絡人手機、客服信箱；Stripe `requirements` 的做法，註定綁定失敗的申請不送進人工審核）：申請自己的缺項先回 `invalid_argument`，申請齊全之後店家資料還缺回 `failed_precondition`，缺哪些由 `KYCApplication.missing_store_fields` 回報（同一份規則）。綁定時的 `INCOMPLETE_DATA` 保留為防線，只指向店家現在實際缺的欄位。
- 有 PayUni 註冊之後，PayUni 必填的店家欄位不能清掉（`invalid_argument`，欄位是 `store.*`）；同步時必填值是空的就不呼叫 PayUni、記 `INCOMPLETE_DATA`。
- 店家聯絡欄位的長度上限就是 PayUni 的上限（聯絡人姓名 20 字、客服信箱 50 字，代理商文件 §7.3），接上第二家金流商時重新評估；店家存只有數字的電話，送給 PayUni 時在 `platform/payuni` 轉成「區碼-電話號碼」（§7.3.3）。
- `PayUniProblem.store_fields`（`StoreField`：地址、聯絡人姓名、聯絡人手機、客服信箱、商店電話）：聯絡類的錯誤碼指向店家欄位，`INCOMPLETE_DATA` 指向綁定需要的店家欄位；`MerTel` 的三個碼（`AGENT04008`–`04010`）賣家可以自己修正。

### 入駐同意紀錄（2026-10-07，10c 第 0b 刀）

- 文件與時點：開店（`ApplyStore`）同意賣家契約 `SELLER_TERMS`；送出 KYC（`SubmitKYC`）確認個資蒐集告知 `KYC_PRIVACY_NOTICE`、同意 PayUni 服務條款 `PAYUNI_TERMS`（食品業者登錄的送出另要求 `FOOD_REGISTRATION_NOTICE`，綁在登錄上；出處：`step10d-2b-decisions.md` DC「告知與保存期間」，不在舊檔）。每個操作要求的種類固定，多一種、少一種、重複都是 `invalid_argument`。
- 條文與版本只有一個來源：`website/docs/legal/` 的正本（版本行寫生效日）加 manifest `seller-agreements.json`（沒有正本的 PAYUNI_TERMS 把版本寫在 manifest）。產生器 `website/scripts/generate-legal.ts`（`pnpm gen:legal`）輸出 web 的 TS 投影與 Go 的 `model.CurrentAgreements`（`agreement_gen.go`）；CI `legal-generated-diff.yml` 重新生成後比對。不記內容雜湊：版本加 repo 的正本就能還原條文。填入營運主體、聯絡信箱等佔位符就是條文改變，版本要遞增。
- 伺服器以版本核對：client 送回它顯示給賣家的版本，不同回 `failed_precondition`（`AGREEMENT_OUTDATED`）；web 送自己 bundle 裡的版本，部署有時間差時由它擋下，請賣家重新整理。App 讀 `GetSellerAgreements`（免登入）取得現行版本與網址。
- 每次同意事件一列（`seller.agreement_acceptances`，沒有 `ON CONFLICT`）：開店那一份每家店一列（部分唯一索引）；KYC 的兩份每次送出各一列，以 `submission_review_id` 綁定那次送出的 SUBMIT 審核紀錄（CHECK 要求 KYC 的列一定帶、其他的一定不帶），換了申請主體再送出因此留下新的一組同意（Stripe 在稅籍變更時也要求重新同意）。寫入與開店、送出在同一個 transaction。
- 記錄的欄位：店家、種類、版本、帳號、時間、來源 IP（`inet`，與 `httpapi` 判定限流身分同一套邏輯）、User-Agent（上限 512 字）。不記 `client`（後端沒有可信的來源；Stripe 的 `tos_acceptance` 只記 date、ip、user_agent）。
- 寫入路徑只有 INSERT、店家個資清理把 IP 與 User-Agent 清成 NULL（`agreement_acceptances_purged_check`）、食品登錄審核紀錄期滿刪除時 CASCADE 刪掉綁在它上面的告知同意（`agreement_acceptances.food_registration_id` 的 `ON DELETE CASCADE`，出處同上 DC），以及 dev seed 的重設（守門測試以 query 名稱限定）。

### 關店與個資保留（2026-10-15，2026-10-19 改保留期）

- 關店可逆，不可逆的點在清理（2026-10-15；2026-10-20 起是第一段清理〔退場〕，見文末修訂）：關店只記為什麼關的、結束預告終止、移除 SELLER、發 `StoreClosed`；logo 與封面連同 media 的引用留在店家列，到清理才釋放並記成退場（`stores_closed_images_check`：已清理的店沒有圖片）。relay 不把 `StoreClosed` 轉成 `seller.deleted`；清理的同一個 transaction 發 `StoreRetired`，relay 轉成 `seller.deleted`（product、social、im、notification、pricebook 的刪除動作因此晚到清理時）。
- 關店紀錄只放 `store_closures` 一張表（每次關店一列，`reopened_at` 為 NULL 的是目前這一次；稽核永久保留），`stores` 不存關店欄位。`closed_by` 是 `ACCOUNT_OWNER`、`TERMINATION`、`ACCOUNT_DELETION`，終止另記 `closed_by_measure_id`（理由與作者在措施裡）；沒有 `ADMIN`、`closed_by_admin_id`、`close_reason`。
- 保留期（2026-10-19，Will 核准設計書 Q2 (a)、Q4 (a)）：`model.StoreRetention` 是 4416 小時（184 天，任何連續六個曆月的最大長度，所以不論哪天關店都不短於「六個月」；用固定時數，因為 Go 與 SQL 的固定時數處處相同，timestamptz 加日數依 session 時區計算）。整個 stint 的個資一起延到保留期，不拆兩段清理（2026-10-20 修訂推翻這一點，改成兩段，見文末）。保留期的時數只從這一個常數來。
- 保留期依關閉原因（2026-10-15，目的不同：爭議處理對資料最小化）：`TERMINATION` 關閉的是 `greatest(關閉後 StoreRetention, 帳號上未撤銷終止中最晚的申覆期限)`（申覆期限是終止生效後 `AppealWindow`，同為 4416 小時，不短於條款與 DSA 的六個月；已撤銷的終止不延長）。兩個常數同值，造成關店的終止在關店時已生效，它自己的申覆期限不晚於關店加保留期，所以第二項只在帳號上有生效晚於關店的另一筆未撤銷終止時起作用（關店之後才發現的 21.3 事由、關聯帳號的終止）（2026-10-19）。其他關閉是關閉後 `StoreRetention`。保住條款 21.6「終止後三十日內請求資料」。
- 判斷式只有 `RetireDueStores` 一支查詢（`now > retire_after`、未清理、沒有 hold），清理與恢復營業（`closing.Reopen`）共用，保留期與申覆期間的時數由 Go 常數傳入。`stores_pii_purged_check` 不檢查時間，資料庫只保證「已清理 ⇒ 已關閉、個人欄位是 NULL」。
- legal hold（2026-10-15）：帳號上未撤銷的終止合作措施有待處理的申覆（終止合作沒有補正，ADR `2026-10-18-seller-governance-consolidated.md`「終止合作」），或帳號上未撤銷的關聯終止其來源措施有待處理的申覆時，帳號全部已關閉的 stint 都不清理（比照 eBay 的 restrict the processing 與 GDPR 17(3)(e)）；裁定後的下一輪清理照常。主管機關來函的保存（`hold_until`，ADR `2026-10-18-seller-governance-consolidated.md` 的 2026-10-19 修訂）只擋清理，不改變業務結果：恢復營業只問保留期是否已過。
- 清理的範圍：店家聯絡與地址、店名、說明、`prize`、`website_url`，KYC 申請全部狀態的個人欄位（密文、blind index 與 `kyc_identity_hashes`、姓名、生日、證件資訊、銀行、公司資料、負責人證號）與證件（media 引用經 `seller.media_sync` 解除），登記資料查詢，PayUni 註冊與觀測的自由文字 `reason`（`payuni_status_history` 的 180 天併入這條規則，沒有另外的期限與 job；關店之後才到的通知不把 `reason` 寫回），食品業者登錄的字號、退回與撤銷的理由、內部註記（仍核准中的先結束為 `STORE_RETIRED`，待審的撤回），同意紀錄的 IP 與 User-Agent。保留：審核的事實（誰、何時、決定）、PayUni 的狀態碼與 outcome、`seller_type`／`product_type`、措施的身分快照、管理員的稽核文字。食品登錄的審核紀錄（不含字號）在關店滿 2 個日曆年（台北時區）後整列刪除，同樣受來函的保存擋下（2 個日曆年出自 `step10d-2b-decisions.md` DC 與 `CARRYOVER.md`「食品業者登錄字號隨店家個資清理保留」列，不在舊檔）。
- 清理是每小時的 River periodic job `seller.purge_pii`（啟動即執行），每批 100 個 stint 一個 transaction，迴圈到取不滿；重跑是 no-op（排程與批次出自 `CARRYOVER.md`「刪店 PII 清理」列〔7c 階段 3〕，舊檔只寫「每天最多 100 筆」是要修的缺陷）。
- 清理後撤銷（2026-10-15）：保留期之後撤銷終止，只撤銷紀錄與禁止入駐（快照刪除，帳號可以開新店，新的 `store_id`）；不恢復店家，因為商品、貼文、聊天與 KYC 都已清掉。

## 後果與重新評估

- 停權的人用新的身分證號（例如人頭）仍能入駐；這道防線只擋同一個身分。銀行帳號命中不擋：只換身分證號、沿用帳戶時，靠審核員看到標記判斷。出現大量換身分重來的濫用時，評估第三方身分驗證。
- 快照到措施的 hash 期間，對應的舊 HMAC 金鑰不能從金鑰環移除（啟動檢查擋下）。停權措施建立時，帳號已被清理的舊 stint 的 KYC hash 取不到，要完全涵蓋就得在清理之前停權。
- 後台的 PENDING 查詢不需要人去按重新查詢才收得掉；代價是一個 1 分鐘的掃描 job 與兩個 query。
- App 在店家設定就有聯絡資料，沒有獨立的「金流聯絡資料」；換金流商或接第二家時只需新增同步端。
- 保留期、終止的申覆期間與來函的保存期同值（4416 小時）只靠設計決定，沒有程式強制（model 的 unit test 釘住數值與不短於 184 天）；三者的法律依據不同（契約與 DSA 的申覆期、第 37 條第 1 項的保存、第 37 條第 2 項的調取保存），所以分成三個常數。與關店同時收文的來函，保存期不比保留期更久。
- PAYUNI_TERMS 記的是 manifest 的日期，條文在 PayUni 的外部網址，無法還原賣家同意當時的內容；簽代理商契約時依契約取得條款正本存進 `website/docs/legal/`，manifest 改用 `doc`。
- 上線前的阻擋條件（計畫書 `2026-09-29-a3-buyer-seller.md` 的 I-legal 項）：四份法律文字（隱私權政策、賣家身分認證告知事項、食品業者登錄告知事項、賣家合作條款附件）仍寫「關店後 30 天」，要改成現行保留期；identity（註冊與登入資料、連線紀錄）與買家的保存期（Q4 (a)）是獨立的步驟，帳號刪除現在立刻清 identity 資料；新資料類別（詢問的說明與回覆、主管機關對轉報的回覆、來函的摘要與對象）的保存期（設計 B31）。6 個月這個值請律師確認。
- 重新評估：法務給出 seller 範圍內其他類別的法定保存年限時，改為依資料類別分級（`retention_until`）；系統上線之前或文件版本第一次在有真實賣家之後改版時，加 `pending_agreements` 與 `AcceptSellerAgreements`，處理已入駐的賣家在版本更新後重新同意、以及換主體的情況（遷移計畫階段 C 的待辦）；同時有多位審核員、或佇列頁要即時更新多筆時，登記資料查詢改用 server-streaming。

## 修訂：關店後的清理改成兩段（2026-10-20，Will 2026-10-19 改選設計書 Q2 (b)；A3b 步驟 10e 第 3c-b 刀修正輪，設計 `plans/2026-09-29-a3-buyer-seller/step10e-3cb-design.md` 修正輪的修訂）

上文「關店與個資保留」的保留期（2026-10-19，Q2 (a)：整個 stint 一起延到 184 天）保留歷史。脈絡：3c-b 的 design-review 指出 Q2 (a) 的分析「多保存的只有店家說明與圖片」不成立——清理的同一個 transaction 發 `StoreRetired`，relay 轉成 `seller.deleted`，im 遮蔽聊天個資（`services/im-service/internal/worker/seller_deleted.go` 的 `RedactDeletedAccount`）、product、social、ai 的下游刪除都跟著晚了 5 個月。Will 依完整的範圍重新選 (b)。

- **第一段（退場）**：關店後 `model.StoreRetireAfter` = 720 小時（30 天，回到 3c-b-4 之前的值）。`TERMINATION` 關閉的是 `greatest(關店後 30 天, 帳號上未撤銷終止中最晚的申覆期限)`（同上文的式子）。清公開內容（說明、獎項、網站）、釋放 logo 與封面，發 `StoreRetired`（relay 轉成 `seller.deleted`，下游刪除），記下 `stores.retired_at`；待審的食品業者登錄撤回、仍核准中的結束（`STORE_RETIRED`）。退場之後不能恢復營業（下游已刪除），恢復營業問的「保留期是否已過」就是第一段的時間條件。
- **第二段（個資清理）**：已退場、關店後 `model.StoreRetention` = 4416 小時（184 天，詐欺犯罪危害防制條例第 37 條第 1 項的六個月；`TERMINATION` 關閉同一個 `greatest` 式子），清註冊、身分與連線資料：店名、店家聯絡與地址、KYC 個人欄位與證件、登記資料查詢、PayUni 的自由文字、食品業者登錄的字號與理由、同意紀錄的 IP 與 User-Agent，記下 `pii_purged_at`。店名歸第二段：它是送交金流審核的身分欄位（PayUni 綁定後鎖住），不只是公開內容。
- **保存條件擋哪一段**：申覆的 legal hold 擋兩段（爭議還沒結束，店家應該還能恢復，資料也要留給爭議，GDPR 17(3)(e)）；主管機關來函的保存擋兩段（第 37 條第 2 項的調取保存涵蓋聊天、商品這類會被第一段的下游刪除的資料）與食品登錄審核紀錄的刪除，不影響恢復營業。兩種保存只有 `StoreHolds` 一支查詢定義；兩段的時間條件只有 `RetireDueStores` 一支（`second_stage` 選段）。每一段都在鎖住候選之後再判斷時間並問保存（同上文與 governance ADR 的競態證明）。
- 資料庫保證：`stores_retired_check`（已退場 ⇒ 已關閉、公開內容是 NULL）、`stores_closed_images_check`（已退場的店沒有圖片）、`stores_pii_purged_check`（已清理 ⇒ 已退場、註冊資料是 NULL）。既有已清理的列在 migration 裡以 `retired_at = pii_purged_at` 補上（當時兩段是同一個 transaction）。
- 後果：兩個保留期、兩份法律文字的期間（I-legal 那一刀改四份法律文字時要寫兩段：公開內容與下游資料關店後 30 天，註冊與身分資料 6 個月）；主管機關來函的涵蓋狀態（`AuthorityStoreCoverage`）只看第二段（個資是否已清），已退場而個資還在的店顯示「保存中」或「保存期限已過」，不另標示下游資料已刪（觸發：機關第一次要求調取聊天或商品資料而店家已退場時，加一個涵蓋狀態）。

## 修訂：PayUni 拒絕與綁定失敗後賣家能不能重新送出（2026-10-21，Will 同日核可；A3b 步驟 10e 第 5a 刀，設計 `plans/2026-09-29-a3-buyer-seller/step10e-5a-design.md` F5 (a)、F17、F8）

上文「核准與 PayUni 綁定」的「PayUni 拒絕之後」一條保留歷史。改的是「能不能重送」的規則，與它的表達方式：

- **「能不能重送」只有一個判斷**：`KYCFlow.can_resubmit` 在 PayUni 階段（申請已核准）等於 `SellerPayUniOf` 的推導結果是 `FIXABLE`；`SaveKYCDraft` 與 `SubmitKYC` 能不能做是同一個條件。取代 `Editable` 與 PayUni 階段各自的 `!NeedsReonboarding`。
- **PayUni 拒絕而且補救方式是聯絡客服（`MEMBER_99`：會員已關閉或停權）時不能自行重送**（原本只排除商家層級的終止狀態）：賣家改申請也改不了 PayUni 的結果，客服處理。其餘非終止的拒絕（`MEMBER_8`、`MEMBER_92`）仍可回草稿重送，有 MerID 之後身分欄位仍鎖住。
- **綁定失敗而且 PayUni 指出的是申請欄位時賣家可以重送**（原本一律不行）：綁定失敗時 PayUni 沒有收下這家店，身分欄位本來就解鎖（`IdentityLockedBy` 在 FAILED 為假），重新核准時 FAILED 的註冊回到 PENDING 重新綁定，整條路本來就通，只差 `CanResubmit`。只指出店家欄位的（賣家改店家設定、客服重新綁定）、暫時性失敗（等平台重新送出）與其他碼（聯絡客服）仍不能自行重送。
- **綁定失敗的重新綁定與申請狀態**：管理員 `RebindPayUni` 在註冊的申請不是已核准（賣家把它改回草稿）時回 `failed_precondition`，避免綁定 job 把還沒審核的草稿送給 PayUni。
- **不能重送時帶原因**：`KYCFlow.resubmit_unavailabilities`（原因 enum 加伺服器句子，與兩個 RPC 的 `failed_precondition` 同一個 Go 函式 `model.ResubmitUnavailableReasons`），client 只顯示。
- **綁定重試用完進後台待辦**（`PAYUNI_BIND_RETRY`，期限 24 小時，上線前的起始值）：「平台會協助重新送出」由待辦成立，管理員重新綁定後待辦消失。

## 修訂：食品業者登錄刪除（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D3；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 4）

示範品類改成一般電商。食品業者登錄（`food_registrations` 與它的審核、公開字號、2 年保存）整個刪除：入駐同意紀錄少了 `FOOD_REGISTRATION_NOTICE` 與 `food_registration_id`（每次同意事件仍一列，依種類綁定來源），關店後的兩段清理少了退場時撤回待審、結束核准中的登錄與個資清理時清字號，也不再有關店後 2 年刪除審核紀錄；admin 對店家的動作少了核准與退回登錄。KYC、PayUni、兩段清理的時間條件與保存條件（`StoreHolds`）不變。
