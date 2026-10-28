# 單體裡的賣家 KYC 與 PayUni：身分比對、非同步綁定、個資保留

| 項目 | 值 |
|---|---|
| 狀態 | superseded by 2026-10-19-seller-kyc-payuni-consolidated.md（修訂超過 3 段，整合成一份；本檔保留歷史，現況讀新檔） |
| 日期 | 2026-10-02 |
| 影響範圍 | `internal/modules/seller`（KYC、PayUni、刪店清理）、`internal/platform/seal`、`internal/legacy`（PayUni webhook 的 consumer、`merchant.verification.*` 的 relay）、admin、website、seller-app |
| 修訂 | `2026-07-20-seller-kyc-pii-encryption-scope.md` 的加密實作（範圍不變） |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 7c |

## 脈絡

KYC 認證與 PayUni 子商店從 seller-service 搬進 seller 模組。舊實作的盤點（7c 開工時，快照 26ae7fad0）找到的問題多半來自結構：

- Approve 在請求內同步呼叫 PayUni，PayUni 的 profile 更新甚至在 DB transaction 內、持有 advisory lock 時呼叫；審核的 `FOR UPDATE` 不在 transaction 內，鎖沒有作用。
- cert 狀態機混了 PayUni 的整合結果（`forwarded`、`forward_failed`），重試只能「再核准一次」。
- 草稿 `step_data` 是自由格式 JSON：遮蔽函式被它繞過，gateway 另外擋 blob URL。
- `raw_request` 存明文身分證號；webhook 寫入失敗只記 warn、訊息照樣 ack；觀測時間用收到的時間，亂序的舊通知會蓋掉新狀態。
- 沒有任何地方比對身分證號或銀行帳號：被停權的人換一個帳號重新入駐就繞過處分（7a 的決定把這道防線指定給 7c）。
- 關店後的個資只清名稱、電話、email、logo，每天最多 100 筆。

約束：
- 外部：PayUni 子商店 API（MerID 一次性；AGENT04017 表示已綁定，可查詢後認領）；PayUni 是支付機構，金流 KYC 的法定保存由它負責；開立發票的稅務紀錄由 invoice 模組自己保存快照。seller 範圍內沒有找到必須保留個資的法定義務（未經法務確認）。
- 繼承：KYC 的加密範圍（身分證號、銀行帳號加密；姓名、生日明文供後台模糊搜尋），計畫決定第 5 項。仍成立。
- 繼承：停權以帳號為單位、由處分表在讀取時推導（ADR `2026-10-18-seller-governance-consolidated.md`）。仍成立，身分比對用同一個推導。
- 繼承：PayUni webhook 的 inbox 在 Java `payuni-webhook-service`（ADR `2026-09-18-webhook-two-phase-transaction`），到 payment 搬進單體前不變。
- 不再成立：ADR 2026-07-20 的「沿用 `pkg/crypto`、單一 master key」與「密文存 Base64 varchar」——當時是為了與既有服務一致，新 schema 沒有資料要搬。

## 考慮過的方案

**身分比對**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：每個識別欄位各一個 keyed HMAC（blind index），金鑰與加密金鑰分開、帶 key id，先正規化再算（CipherSweet、AWS Database Encryption SDK 的 beacon） | 輪替要以解密後重算遷移；hash 仍是個資，保留要有理由 | ✓ |
| 身分證號加帳號合成一個 hash | 只換其中一項就繞過 | ✗ |
| 確定性加密（AES-SIV） | 密文本身就是比對值，與隨機 nonce 的欄位加密是兩套 | ✗ |
| 第三方身分驗證或風控服務 | 外部帳號與費用；規模不需要 | ✗ |

**比對 hash 與個資清理的衝突**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：資料最小化，保留的資料綁著保留的理由（GDPR Art.5(1)(c)(e)） | | |
| 停權生效時把該帳號的身分 hash 快照到處分擁有的表，處分結束就刪；KYC 列照常清理 | 快照沒有明文可以重算，舊的 HMAC 金鑰要留到這些處分結束 | ✓ |
| hash 留在 KYC 列，有生效處分時清理 job 跳過 | 保留理由在別的表；清理 job 要懂處分 | ✗ |
| hash 永久保留 | 與清理規則矛盾 | ✗ |

**個資保留期**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：目的結束後保留一段處理爭議的期間再刪，有法定保存義務的類別依法定年限 | | |
| 關店後 30 天一律清（與 buyer 同一種 job） | 30 天內個資仍在 | ✓ |
| 依資料類別分級，加 `retention_until`、`legal_hold`（舊 seller-service 設計書 18 §2.9） | 沒有法定年限可填，年限會是編出來的 | ✗（出現法定義務時再採用） |
| 關店當下立即清 | 關店後的售後、申訴、KYC 爭議看不到當事人 | ✗ |
| crypto-shred（每店一把 DEK） | 要 KMS 與金鑰表；規模不需要 | ✗ |

**核准與 PayUni 綁定**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：人工決定與外部整合是兩件事；寫入意圖與排入 job 在同一個 transaction，外部呼叫在 transaction 外、以冪等鍵重試（backend 規則 5） | 核准變成非同步，畫面要顯示合成的流程狀態 | ✓ |
| cert 狀態加 `PENDING_BINDING` | cert 軸繼續混整合結果，重試仍是再核准 | ✗ |
| 先同步呼叫，逾時再轉背景 | 兩套行為，難測 | ✗ |
| 沿用舊的 outbox 與自建 worker | 與 River 重複 | ✗ |

**後台看完整的身分證號與銀行帳號**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：預設遮蔽，需要時以獨立動作顯示並留下稽核紀錄（just-in-time access） | 審核員多一次操作 | ✓ |
| 照舊完整顯示、不留紀錄 | 看過的人無從追查 | ✗ |
| 永遠不顯示 | 審核要核對證件，做不到 | ✗ |
| 雙人覆核 | 規模不需要 | ✗ |

## 決策

- **身分比對**：身分證號、企業負責人身分證號、統編、銀行帳號（連同銀行代碼）各算一個 HMAC，輸入帶欄位標籤做 domain separation；HMAC 金鑰環獨立於加密金鑰（啟動時檢查不同）、每筆 hash 帶 key id。身分證號、負責人身分證號、統編命中「別的帳號有生效中的處分」時不能通過認證；銀行帳號命中只在審核畫面標記，不擋（家人或配偶共用帳戶是常態）。送出時先查，核准時在鎖住帳號的 transaction 內再查一次，以後者為準。回應用泛用文案、不透露命中的欄位或帳號，並限流。處分的判斷用 `stint` 的推導，不另寫。
- **比對 hash 的保留**：處分生效時，把該帳號 KYC 的身分 hash 快照到處分擁有的表；處分解除或到期時刪除。KYC 列的 hash 隨 KYC 個資一起清理。
- **個資保留**：關店 30 天後清掉 seller 擁有的全部個資——店家聯絡與地址、KYC 個人欄位與證件檔案的 media 引用、PayUni 聯絡欄位；`payuni_status_history` 的 180 天併入同一條規則。審核的事實（誰、何時、決定）與 PayUni 的狀態碼保留，不含欄位值。
- **加密**：identity 的 `seal`（key id 前綴、可輪替）提到 `internal/platform/seal`，identity 與 seller 共用；AAD 加上「申請 ID 與欄位名」，防止密文在列與欄之間搬動；密文存 `bytea`。加密範圍照 ADR 2026-07-20。
- **核准與綁定**：cert 狀態只記人工決定（草稿、送出、核准、退回、取消），刪除 `forwarded`、`forward_failed`；綁定狀態在 PayUni 註冊列上，那一列就是 intent。核准在同一個 transaction 寫入決定、綁定意圖、排入以 `store_id` 為唯一鍵的 River job，並發出事件；job 在 transaction 外呼叫 PayUni，暫時性錯誤由 River 重試，永久錯誤記為失敗，由管理員「重新綁定」。地址與聯絡欄位的同步走同一個 job。畫面顯示兩條軸合成的流程狀態。`raw_request` 不存。
- **PayUni webhook**：legacy consumer 把通知轉成以 `event_key` 為唯一鍵的 River job，持久化後才 ack；套用時追加觀測，只有 PayUni 的 `ExecuteTime` 比目前狀態新才更新。
- **草稿**：與送出共用同一組具型別的欄位，必填由狀態決定的 CHECK 表達；不收任意 JSON。
- **後台顯示**：身分證號與銀行帳號預設遮蔽，顯示全文是獨立的動作，寫一列 append-only 稽核紀錄（誰、哪筆申請、何時、理由，不含值）；賣家自查的銀行帳號也遮蔽。
- **PayUni 聯絡 email 與電話**：由 KYC 表單收，存在申請上（identity 不再有電話，email 在 Clerk）。

## 後果

- 申請以 `store_id` 為鍵（計畫 7 的共通規則）：重新入駐要重做 KYC；PayUni 端拒絕後，同一個 stint 不能靠重送 KYC 再綁定，只能關店重新入駐（舊行為，UI 文案要說清楚）。
- 停權的人用新的身分證號（例如人頭）仍能入駐；這道防線只擋同一個身分。
- 銀行帳號命中不擋：同一個人只換身分證號、沿用帳戶時，靠審核員看到標記判斷。
- 快照到處分的 hash 期間，對應的舊 HMAC 金鑰不能從金鑰環移除。
- KYC 表單多兩個必填欄位（PayUni 聯絡 email 與電話），後台多一個「顯示全文」動作；入口在步驟 8、10 做。
- 重新評估：法務給出 seller 範圍內的法定保存年限時，改為依資料類別分級（`retention_until`、`legal_hold`）；出現大量換身分重來的濫用時，評估第三方身分驗證。

## 修訂：證件影像與顯示全文共用一次授權（2026-10-04，Will 決定）

本節補充「後台顯示」；原決策段保留歷史。

**脈絡**：身分證與存摺影像上有完整的證號與帳號。原本後台經 gateway 的通用 admin `secure_download` 以連號的 asset ID 讀任何機敏檔：不檢查歸屬、不記是哪位管理員，等於繞過「顯示全文要留稽核」。業界基準是 just-in-time 存取：擁有者授權、綁定用途、限時、每次讀取都留紀錄。

**決策**：
- `RevealKYCSensitive` 是唯一的授權動作：理由必填，同一列稽核同時是 15 分鐘的證件檢視授權，涵蓋欄位全文與證件影像。
- 證件影像只經 seller 擁有的端點，以（申請、證件位置）為鍵、不以 asset ID；只服務持有這筆申請、未到期授權的那位管理員；每次讀取先寫一列存取紀錄（含實際送出的 asset ID）再讀，寫不進去就不送。遮蔽的檢視不帶影像網址與 asset ID。
- KYC 證件只接受機敏上傳的資產。
- 通用的 admin 機敏下載與預覽整條刪除，其餘使用者（發票稅籍文件）改由各自的擁有者授權（計畫 8f 之後的子步驟）。
- 後台的最小權限（誰可以顯示全文）由後台角色處理（計畫 8g 之前的子步驟）。

## 修訂：審核時對照的登記資料由送件時的查詢快照提供（2026-10-07，Will 決定）

本節補充「審核」；原決策段保留歷史。

**脈絡**：GCIS 查詢只給賣家帶入表單（`SellerAppService.LookupBusinessRegistration`），審核員沒有登記資料可以對照，統編正確性只有格式與檢查碼把關（seller `CARRYOVER.md` 的 GCIS 列）。業界基準（Stripe Connect、Middesk、Persona 的 KYB）是送件時自動查詢公司登記、存下結果並標出不一致，審核員看比對結果，不自己查。

| 方案 | 代價 | 判定 |
|---|---|---|
| 送件時由 job 查詢並存快照，審核畫面並列、標出不一致，可重新查詢 | 一張快照表、一個 River job | 採用（基準） |
| 審核員開詳情時即時查詢、不存 | 每次開詳情打一次外部服務；核准依據沒有留存；「只有一位審核員」是現況，不是約束 | ✗ |
| 以任意統編查詢的 admin RPC | 與申請脫鉤，是不受限的查詢器 | ✗ |

**決策**：
- `SubmitKYC` 在同一個 transaction 排入 River job；job 在 transaction 外呼叫既有的 `gcis` client，把查詢時間與登記資料（統編、名稱、地址、設立日期、組織型態）存成該申請的快照，並逐欄記下與申請內容的不一致。GCIS 查無或失敗時記下結果，由 River 重試，不擋送件。
- 後台以申請讀取快照，審核畫面在申請內容旁並列；「重新查詢」以同一個 job 覆寫快照（快照過期或送件時查詢失敗時用）。
- 核准與退回時，以當下的快照作為審核依據（「審核員以核准時的登記資料為準」）。

**後果**：不一致只標示、不自動退件；觸發改成自動規則的條件是 KYC 送件量超過人工逐筆審核能處理的量。

**補充（2026-10-07，Will 決定，步驟 10a 開工時）**：
- 審核依據：後台 KYC 檢視的 etag 由 KYC `version` 與快照序號組成（AIP-154：etag 代表回傳的整個資源），快照在審核員看過之後變了，用舊 etag 核准或退回回 `aborted`；賣家端的 etag 不受影響。`kyc_reviews` 記下依據的快照（查詢時間與結果；沒有快照記「無」）。
- 沒有快照或結果是不可用時允許審核，畫面標示；只在審核中可重新查詢；決定之後才回來的結果照樣寫入，不改變已記下的依據。
- 比對：名稱與地址明細以 NFKC、台／臺、空白正規化後比較；地址先拆出鄉鎮市區代碼，拆不開時記「無法比對」，不算不一致；組織型態在 GCIS 沒有值時記「無法比對」，兩邊都有值而不同時是不一致；設立日期任一方缺漏記「無法比對」。
- 快照存結構化的登記現況（KYB 的主要訊號；8b 不回現況是因為當時用途是填表、沒有樣本）。值域以 GCIS 公開 API 的真實樣本決定（Will 同意取樣）；現況不是核准設立時只標示，不自動退件。
- 快照的儲存：每次查詢一列（`seller.kyc_registration_checks`，送件與後台重新查詢各新增一列），寫入結果後不再改；「重新查詢覆寫快照」的實作是新的一列取代目前顯示的那一列，舊列保留，審核紀錄才能引用它當依據。
- 只有公司型（有統編）的申請有快照；快照與 KYC 個資同時清除（關店 30 天）：它的用途是存續期間的審核依據。

**補充（2026-10-07，Will 決定，步驟 10b 開工時）**：申請人不再填組織型態。`company_type` 沒有送給 PayUni、送件不要求，唯一的用途是和登記資料比對；組織型態由登記機關提供（KYB 服務的做法），審核畫面顯示快照裡的組織型態。契約的 `business.company_type` 改成 reserved，DB 欄位與比對欄位一併刪除；上面「組織型態在 GCIS 沒有值時記『無法比對』」那條不再適用。型態開始影響要收的文件或 PayUni 的參數時，再以封閉列舉加回申請內容。

## 修訂：PayUni 聯絡資料的擁有者從 KYC 申請與註冊列改成店家（2026-10-07，Will 決定，步驟 10c 第 0a 刀）

本節補充上面「PayUni 聯絡 email 與電話」；原決策段保留歷史。

**脈絡**：原決策把 PayUni 要的聯絡 email 與手機放在 KYC 申請上，核准時再複製到 `payuni_registrations`（`contact_name`、`contact_mobile`、`contact_tel`、`mer_email`，`mer_tel` 沒有初值），賣家用 `UpdateMyPayUniProfile` 改的是註冊列上的副本。這些資料都不是 PayUni 特有的：店家本來就有聯絡電話與地址，地址早就是店家唯一來源、改了由 `seller.payuni_sync` 送出，只有聯絡資料另存一份。兩份副本已經造成三個矛盾：
- PayUni 回聯絡類錯誤時，`problem.fields` 指向 KYC 的 `contact_email`、`contact_phone`，但送給 PayUni 的是註冊列上的副本，賣家照著改申請，PayUni 那邊不會變。
- 重新核准時副本重設成這次申請的值，靜默蓋掉賣家用 `UpdateMyPayUniProfile` 改過的內容。
- 綁定時 `Email` 讀 KYC、`MerEmail` 讀副本，同一個 email 有兩個來源。

業界基準（Stripe Custom 與 platform-controlled 的帳號、Shopify Payments）：資料以平台為準，平台以 API 更新金流商的帳號；沒有另一個「金流聯絡資料」的編輯器，改聯絡方式也不觸發重新審核。「沿用也是決策」漏掉的是：原本只重新評估了機制（冪等表、advisory lock），沒有問這份資料該不該獨立存在。

**決策**：
- 店家是唯一來源：`stores` 加 `contact_name`（至多 20 字，PayUni 的 `ContactName`）、`contact_mobile`（`09` 開頭 10 碼）、`support_email`（綁定時同時是會員的 `Email`）；既有的 `contact_phone` 就是商店電話（`MerTel`）。`KYCApplication.contact_email`、`contact_phone` 與 `payuni_registrations` 的聯絡副本欄位（含只為它存在的 `version`）刪除，`UpdateMyPayUniProfile` 刪除。`ContactTel` 不複製，綁定與同步時讀註冊所指申請的 `company_phone`；企業的 `MBCMail` 同樣用客服信箱。
- 聯絡資料與地址走同一條路：`UpdateProfile` 改了其中任何一欄，在同一個 transaction 登記同步 intent、排入同一個 job；核准不再重設聯絡資料。`ApplyStore` 不帶這三欄，由 `UpdateMyStore`（KYC 頁與店家設定）寫入。
- 送出 KYC 時檢查店家的綁定資料（地址、聯絡人姓名、聯絡人手機、客服信箱）：基準是 Stripe 的 `requirements`，註定綁定失敗的申請不該送進人工審核。店家的狀態不在請求裡，所以缺項不是欄位違規：申請自己的缺項先回 `invalid_argument`（BadRequest 是申請的欄位路徑），申請齊全之後店家資料還缺回 `failed_precondition`（API 設計規範的錯誤表），缺哪些由 `KYCApplication.missing_store_fields` 回報（同一份規則）。綁定時的 `INCOMPLETE_DATA` 保留為防線，問題只指向店家現在實際缺的欄位。
- 有 PayUni 註冊之後，PayUni 必填的店家欄位不能清掉（`invalid_argument`，欄位是 `store.*`）：清掉會讓 PayUni 留著舊值、同步卻成功；同步時必填值是空的就不呼叫 PayUni、記 `INCOMPLETE_DATA`。
- 店家聯絡欄位的長度上限就是 PayUni 的上限（聯絡人姓名 20 字、客服信箱 50 字，代理商文件 §7.3）：PayUni 是這些欄位唯一的去處；接上第二家金流商時重新評估。送給 PayUni 的電話在 `platform/payuni` 轉成「區碼-電話號碼」（§7.3.3），店家存的是只有數字的電話。
- `PayUniProblem` 加 `store_fields`（`StoreField` 列舉：地址、聯絡人姓名、聯絡人手機、客服信箱、商店電話）：聯絡類的錯誤碼指向店家欄位，`INCOMPLETE_DATA` 指向綁定需要的店家欄位；`MerTel` 的三個碼（`AGENT04008`–`04010`）從「聯絡客服」改成賣家可自己修正，因為商店電話現在是賣家的店家欄位、會送給 PayUni。
- 個資清理：關店滿 30 天，店家的三個聯絡欄位與其他個資一起清成 NULL（`stores_pii_purged_check`）；註冊列只剩 PayUni 的自由文字 `reason`。
- migration 不回填舊資料：系統還沒上線，migration 在上線前合併成 baseline（計畫 A3b），D1 刪 `company_type` 也沒有搬值；本機資料由 dev seed 提供。migration 只加欄位與 CHECK、刪舊欄位，並清掉退回紀錄裡的舊欄位名。

**後果**：App 在店家設定就有聯絡資料，不再有「金流聯絡資料」這個獨立功能；換金流商或接第二家時只需新增同步端。`update_merchant` 仍只能改聯絡方式、地址與支付設定（代理商文件 §8），會員 `Email` 綁定之後不能改，所以之後只送 `MerEmail`。

## 修訂：10c 補充：入駐同意紀錄（2026-10-07，Will 決定，步驟 10c 第 0b 刀）

本節補充上面「個資保留」；來源是決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10c-decision-brief.md` D2。

**決策**：
- 三份文件與時點：開店（`ApplyStore`）同意賣家契約 `SELLER_TERMS`；送出 KYC（`SubmitKYC`）確認已閱讀個資蒐集告知 `KYC_PRIVACY_NOTICE`、同意 PayUni 服務條款 `PAYUNI_TERMS`。每個操作要求的種類固定，多一種、少一種、重複都是 `invalid_argument`。
- 條文與版本只有一個來源：`website/docs/legal/` 的正本（`seller-terms.md`、`seller-kyc-notice.md`，版本行寫生效日）加 manifest `seller-agreements.json`（每種文件的網址；沒有正本的 PAYUNI_TERMS 把版本寫在 manifest）。產生器 `website/scripts/generate-legal.ts`（`pnpm gen:legal`）輸出 web 的 TS 投影（`website/features/legal/documents/*.gen.ts`）與 Go 的 `model.CurrentAgreements`（`agreement_gen.go`）；CI `legal-generated-diff.yml` 重新生成後比對，有差異就失敗。不記內容雜湊：版本加 repo 的正本就能還原當時的條文。
- 伺服器以版本核對：client 送回它顯示給賣家的版本，不同回 `failed_precondition`（`AGREEMENT_OUTDATED`）。web 送的是自己 bundle 裡的版本（也就是賣家實際看到的那一版）；部署有時間差時由 `AGREEMENT_OUTDATED` 擋下，web 取消勾選並請賣家重新整理頁面，不會記錯版本。App 讀 `GetSellerAgreements`（免登入）取得現行版本與網址。
- 填入營運主體、聯絡信箱等佔位符就是條文改變，版本要遞增。
- 每次同意事件一列（`seller.agreement_acceptances`，主鍵是 identity，沒有 `ON CONFLICT`）：開店那一份每家店一列（部分唯一索引）；KYC 的兩份每次送出各一列，以 `submission_review_id` 綁定那次送出的 SUBMIT 審核紀錄（CHECK 要求 KYC 的列一定帶、開店的列一定不帶）。被退回後換了申請主體（個人改公司、換統編）再送出，因此留下新的一組同意；Stripe 在稅籍變更時也要求重新同意。寫入與開店、送出在同一個 transaction。
- 記錄的欄位：店家、種類、版本、帳號、時間、來源 IP（`inet`，單一來源是 `httpapi` 判定限流身分的同一套 `CLIENT_IP_HEADER` 邏輯）、User-Agent（上限 512 字，儲存成本的上限）。不記 `client`（App 或 web）：後端沒有可信的來源，User-Agent 已足夠（Stripe 的 `tos_acceptance` 只記 date、ip、user_agent）。
- 保存：IP 與 User-Agent 是欄位值，依本 ADR 的個資保留只保留審核的事實，所以關店滿 30 天的清理把它們清成 NULL 並記下 `pii_purged_at`（`agreement_acceptances_purged_check`）；種類、版本、時間、帳號與送出保留。除了這一條與 dev seed 的重設，這張表只有 INSERT（守門測試以 query 名稱限定）。

**已知限制**：PAYUNI_TERMS 記的是我們 manifest 的日期，條文在 PayUni 的外部網址，無法還原賣家同意當時的內容。觸發條件是簽代理商契約時：依契約取得條款正本存進 `website/docs/legal/`，manifest 改用 `doc`，版本改用正本的日期。

**重新評估的觸發**：系統上線之前，或文件版本第一次在有真實賣家之後改版。那時加 `pending_agreements` 與 `AcceptSellerAgreements`，處理已入駐的賣家在版本更新後重新同意、以及換主體的情況（遷移計畫階段 C 的待辦）。

## 修訂：10e 第 2b-0 刀：退回改成逐項問題（2026-10-12，Will 決定）

本節取代 7c 以來「退回 = 一段理由加 `required_fields`（要補的欄位名）」的模型；來源是決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10e-decision-brief.md` 的第 2b 刀設計簡報題 2（Will 選 (c)）。

**脈絡**：舊模型只回答「哪個欄位」，沒有回答「為什麼」，公司文件合併成一個值（`BUSINESS_COMPANY_DOCUMENTS`），賣家改順序就指不到那一張；「這次退回還算不算數」由 website 與 seller-app 各自用時間戳推導，同一套邏輯寫了兩份。業界基準：Stripe Connect 的 `requirements`（`currently_due` 由伺服器算，`errors[]` 每項有 requirement、code、reason）、Sumsub 的 `rejectLabels`（原因碼）與 `moderationComment`（給申請人）／`clientComment`（內部）、AIP-193 的 `FieldViolation`。

**決策**：
- 退回帶逐項問題 `KYCIssue{field, document_asset_id, code, message}`：`field` 是 `KYCField`；文件類欄位可以用 `document_asset_id` 指到某一張（換順序不會指錯），非文件類的欄位不可以帶；`code` 是封閉的 `KYCIssueCode`（UNREADABLE、MISMATCH、EXPIRED、MISSING、INVALID、OTHER），`message` 必填 1 到 500 字。不做 FINAL 退件，永久拒絕由停權處理。
- `RejectKYCRequest` 與 `KYCReview` 的 `required_fields` 換成 `issues`，另加 `internal_note`（選填、至多 1000 字，只給後台）；總理由 `reason` 照舊必填。賣家端的 `KYCRejection` 只有總理由與 `issues`，沒有 `internal_note`。審核紀錄仍不存欄位值（`kyc_review_issues` 只記欄位路徑、asset ID、原因碼、說明）。
- 「目前還要補什麼」由伺服器算（Stripe 的做法）：`KYCApplication.outstanding_issues` 是最近一次退回的問題裡賣家還沒處理的，重新送出、核准或取消之後為空。「已處理」在寫入申請的同一個 transaction 判定（欄位的值被改過，或指到的文件被移除或替換），記在 `kyc_review_issues.resolved_at`，讀取不推導。`KYCApplication.last_rejection` 只在最近一次審核紀錄是退回、之後賣家沒有重送、核准或取消時才回（賣家處理完每一項、還沒重送時仍在，直到重送為止），後台的 `GetKYCApplication` 仍用 `reviews` 看完整紀錄。兩個 client 刪除自己的 `platformRejectionOf` 與時間戳推導，直接看有沒有 `last_rejection`，逐項標記看 `outstanding_issues`。「值有沒有變」只有寫入路徑一個來源（比明文與正規化後的值，不比密文或 blind index，也不存欄位值的雜湊），與「PayUni 已收下之後不能改身分欄位」的檢查共用。
- PayUni 的問題模型不併入（`PayUniProblem` 的 code 由 PayUni 擁有，是對方定義的開放集合）。
- migration 不回填：舊的 `required_fields` 欄位刪除，沒有搬成問題（系統還沒上線）。

- 退回的文件問題以顯示全文回應（`KYCDocumentView.asset_id`）中的 asset ID 指定：管理員必須先顯示全文（寫稽核、限時）才能指定某一張；規則是：遮蔽檢視的文件清單（`KYCApplication.documents`）不帶 asset ID 與影像網址；退回紀錄（`reviews[].issues`、`outstanding_issues`）裡的 `document_asset_id` 只回顯管理員先前在顯示全文後指定的值；讀圖一律以（申請、位置）加有效的顯示全文授權，與上面「證件圖片」一節一致。「缺少文件」只標欄位、不帶 asset。

**後果**：後台的退回對話框（第 2b 刀）直接逐欄、逐文件選原因碼並寫說明。

## 修訂：10e 第 2b 刀：登記資料查詢是有期限的資源（2026-10-12，Will 決定）

本節取代 10a 第 4 刀「PENDING 的出口只有三個，不需要掃描 job，重新查詢就是出口」；來源是決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10e-decision-brief.md` 的第 2b 刀設計簡報題 1（Will 選 (b′)）。

**脈絡**：舊機制的前提是「重新查詢就是出口」，但兩份後台實作都在查詢中停用「重新查詢」、並以猜的上限輪詢，job 遺失（程序當掉、River 丟棄）留下的 PENDING 就成了沒有人收得掉的死結；前提已經失效。業界基準：AIP-151（長時間工作的狀態放在資源上，終態由伺服器負責，client 以 Get 輪詢到終態）、Stripe 的 `requirements.pending_verification`（非同步驗證的狀態在資源上）。選項 (b)（正式的 `Operation` 資源）與 (c)（server-streaming）不採用：這裡只有一種工作、只有一位審核員，Operation 與查詢列是同一件事存兩份；改用 (c) 的觸發條件是同時有多位審核員、或佇列頁要即時更新多筆。

**決策**：
- 查詢是有期限的資源：建立查詢時寫入 `deadline_at`（`requested_at` 加 15 分鐘；migration 加欄位，NOT NULL，既有列回填 `requested_at` 加 15 分鐘），後台的 `pending_registration_check.deadline_time` 讀這個欄位。期限不由 River 預設退避的公式反推（那組數字沒有被釘住，推導只是自我比對），而是一個決定：互動情境（審核員按下去等結果）業界要幾分鐘內有結果；GCIS 沒有 SLA，但有掃描 job 保證終態、審核員隨時可以重新查詢，所以逾時收成 `UNAVAILABLE` 讓人重查，比等一個可能一個多小時才放棄的重試鏈合理。River 的重試照舊、不釘住預設值：時間上的終態由 `deadline_at` 與掃描保證，重試次數上限（8 次）只防失控；預設退避下 15 分鐘期限內實際只會跑到前幾次嘗試（約 5 次），逾期由 `deadline_at` 與掃描收掉。
- job 執行時已過期限（重試拖過期限，或 job 很晚才醒來）就直接把查詢記成 `UNAVAILABLE`，不再呼叫資料來源。新增 periodic job `seller.kyc_registry_sweep`（照 `payuni_sweep`，啟動即執行、每 1 分鐘）：把 `deadline_at` 不晚於「現在加一個掃描間隔」還在 PENDING 的查詢收成終態（仍屬審核中這一輪、沒有被更新的取代的記 `UNAVAILABLE`；申請已退回、核准、取消、重送過、或被取代的記 `CANCELLED`，與認領時放棄的語意一致），所以終態一定在期限之前；再把閒置超過 3 分鐘的 PENDING 補排 job。兩支掃描查詢走部分索引 `kyc_registration_checks_pending (deadline_at) WHERE outcome = 'PENDING'`（先例 `payuni_registrations_unsynced`）。
- `seller.kyc_registry_check` 以 `check_id` 為 River 唯一鍵（`ByState` 為排隊、執行、等重試，不含 completed 與 discarded），所以補排不會與還活著的 job 重複，被丟棄的 job 才補得回來。寫入結果仍只在查詢是 PENDING 時發生，期限之後才醒來的 job 不覆寫已收成的終態。
- 契約：`KYCApplication.registration_check_pending`（bool）換成 `pending_registration_check`（`KYCRegistrationCheck`，加 `request_time`、`deadline_time`；沒有進行中的查詢時不設），`RequeryKYCRegistrationResponse` 回這筆查詢。client 依 `deadline_time` 以退避輪詢到終態或期限（頁籤隱藏時不輪詢），查詢中可以停用「重新查詢」（伺服器保證有出口），畫面寫「最晚 HH:mm 會有結果」。超過期限仍沒有終態是伺服器的錯誤，畫面改提示重新查詢，不繼續等。
- PayUni 不改：`payuni_sweep` 已經保證 `bind_status` 到終態（BOUND 或 FAILED）；admin 只對 `bind_status=PENDING` 做同樣的退避輪詢（沒有次數上限，間隔上限 60 秒）。

**後果**：後台的 PENDING 不再需要人去按重新查詢才收得掉；多了一個 1 分鐘的掃描 job 與兩個 query（`ExpireKYCRegistrationChecks`、`KYCRegistrationChecksToResync`）。

**文件類欄位的原因碼規則（同刀的 design review，取代 2b-0 修訂裡「缺少文件只標欄位、不帶 asset」的口頭規則）**：`RejectKYC` 的 `KYCIssue` 在文件類欄位上依原因碼：`UNREADABLE`、`EXPIRED`、`MISMATCH`、`INVALID` 必須帶 `document_asset_id`（問題出在某一張文件上，不指定就不知道是哪一張）；`MISSING` 不可以帶（缺的是整個位置，沒有那一張）；`OTHER` 兩者皆可。違反回 `invalid_argument`，FieldViolation 指到 `issues[i].document_asset_id`（`model.IssueAssetRequired`／`IssueAssetForbidden`，`validateIssues` 一處）。後台的退回對話框照同一條規則限制選項（要求先選某一張文件的原因在沒有顯示全文時說明要先顯示全文；缺少不給選文件）。

**單一規則來源（同刀的 design review）**：「哪個賣家型態可以對哪些欄位提出問題」只有 `model.IssuableFields`：`RejectKYC` 驗證 `KYCIssue.field` 與後台 `KYCApplication.issuable_fields` 共用，後台不再寫死欄位表。後台契約不再有證件位置的列舉 `KYCDocumentSlot`：申請上的文件、顯示全文的檢視網址與證件讀取紀錄都以文件類欄位（`KYCField`）加順序表示；證件位置只留在模組內部（檔案端點的路徑 `/api/files/admin/…/{slot}/{position}` 與 `kyc_document_accesses` 的鍵不變，view_uri 本來就是不透明的）。「查詢當天是否在停業期間」由 `model.Registration.SuspendedAt` 一處算（以台北日期，含起迄兩天），`KYCRegistration.suspended_at_query` 帶結果，後台不再自己比對日期。

## 修訂：10e 第 2c-3 刀：關店可逆，個資保留依關閉原因（2026-10-15，Will 決定「全照這版做」）

本節修訂上面的「個資保留」與「比對 hash 的保留」；來源是決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10e-2c3-brief.md`（W7、W8、R6）。基準：停用與刪除分開、刪除走保留期（Google Cloud 的 soft deletion、Shopify 終止後保留 2 年、eBay 以限制處理代替刪除）；P2B 第 4 條第 3 項要求撤回終止時恢復並交還生效前的資料；DSA 第 20 條第 1 項申訴期至少六個月。

- **關店可逆，不可逆的點移到清理**：關店（`close()`）只記為什麼關的、結束預告終止、移除 SELLER、發 `StoreClosed`；logo 與封面連同 media 的引用留在店家列，到清理才釋放並記成退場（`stores_closed_images_check` 改成「已清理的店沒有圖片」）；relay 不再把 `StoreClosed` 轉成 `seller.deleted`，清理的同一個 transaction 發新事件 `StoreRetired`，relay 轉成 `seller.deleted`（product、social、im、notification、pricebook 的刪除動作因此晚到清理時）。
- **保留期依關閉原因**（目的不同：爭議處理對資料最小化）：`closed_by = TERMINATION`（平台終止）是 `greatest(關閉後 30 天, 帳號上未撤銷的終止中最晚的申覆期限)`——申覆期限是終止生效後 184 天（任何連續六個曆月的最大長度，不短於條款的「六個月」），已撤銷的終止不延長，第一項只在訂單拖過申覆期間才起作用，保住條款 21.6「終止後三十日內請求資料」；`ACCOUNT_OWNER`、`ACCOUNT_DELETION` 維持關閉後 30 天。判斷式只有 `RetireDueStores` 一支查詢（`now > retire_after`、未清理、沒有 hold），清理與恢復營業（`closing.Reopen`）共用，兩個保留期的時數都由 Go 常數傳入。`stores_pii_purged_check` 不再檢查「關閉滿 30 天」，資料庫只保證「已清理 ⇒ 已關閉」。
- **legal hold**：帳號上有終止合作措施的待處理申覆，或帳號上的關聯終止（`LINKED_ACCOUNT`）其來源措施有待處理申覆時，帳號全部已關閉的 stint 都不清理（比照 eBay 的 restrict the processing 與 GDPR 17(3)(e)）；申覆裁定後的下一輪清理照常。
- **關店紀錄**：關店的紀錄只放 `store_closures` 一張表（每次關店一列，`reopened_at` 為 NULL 的是目前這一次；稽核永久保留，主鍵帶關閉時間），`stores` 不存關店欄位。`closed_by` 是 `ACCOUNT_OWNER`、`TERMINATION`、`ACCOUNT_DELETION`，終止另記 `closed_by_measure_id`（造成關店的措施，關店當時的事實；理由與作者在措施裡，單一來源）；刪除 `ADMIN`、`closed_by_admin_id`、`close_reason`。
- **身分比對索引（鑰匙輪替）**：申請與措施快照都存金鑰環每一把金鑰算的 `(key_id, hash)`（`kyc_identity_hashes`、`measure_identities`），關聯帳號的反查以索引等值連接，不解密申請；新增金鑰之後，既有申請缺新金鑰的列由 `RehashKYCIdentities` 在啟動時補算（只解密缺列的申請）。快照沒有明文可以重算，所以舊金鑰仍留到措施結束；申請上的舊金鑰列不會被刪，只隨該申請的個資清理整列刪除；啟動檢查 `CheckKYCKeys` 要求資料庫參照的每一把金鑰（含 `kyc_identity_hashes`、`measure_identities` 的 `key_id`）都在金鑰環裡，缺了就不啟動，所以舊金鑰要等所有參照它的列都清掉才能從金鑰環拿掉。
- **比對 hash（W8）**：第 21.3 條終止的身分快照存在措施擁有的 `measure_identities`（本來就不受個資清理影響），保留到終止撤銷為止；清理後才發現的第 21.3 條事由，快照取不到，是接受的缺口。保存的目的與期間寫進條款 21.3、賣家身分認證告知事項第 4 項與隱私政策，落在個資法第 11 條第 3 項但書與施行細則第 21 條的「契約約定之保存期限」；業界只對被封禁者保留（Uber、蝦皮），不對所有關閉者。
- **清理後撤銷**：申覆期間後撤銷終止，只撤銷紀錄與禁止入駐（快照刪除，帳號可以開新店，新的 `store_id`）；不恢復店家，因為商品、貼文、聊天個資與 KYC 都已清掉，名為恢復做不到。

## 修訂：10e 第 3c-b-4 刀：關店後的個資保留改為 6 個月（2026-10-19，Will 核准設計書 Q2 (a)、Q4 (a)）

本節修訂上面「個資保留」與 2026-10-15 修訂的 30 天。來源是設計書 `docs/plans/2026-09-29-a3-buyer-seller/step10e-3cb-design.md`（B21、Q2、Q4、G12）與 rb R5。基準：詐欺犯罪危害防制條例第 37 條第 1 項要求採可行之技術、於合理期間保存足以重建使用者與個別交易的資料（用戶註冊及確認身分程序所得資料、連線紀錄、交易紀錄），第 2 項的調取資料要保存六個月；Will 照建議採關店後 6 個月，上線前請律師確認（REAL：現行法；6 個月這個值待律師確認）。

- **`StoreRetention` 從 720 小時（30 天）改成 4416 小時（184 天）**：6 個曆月最長是 184 天，所以不論哪一天關店都不短於「六個月」（同 `AppealWindow` 的選法）；用固定時數而不是月份，因為 Go 與 SQL 的固定時數才處處相同（timestamptz 加日數依 session 時區計算）。保留期的時數只從 `model.StoreRetention` 這一個常數來，`RetireDueStores` 的 `retention_hours` 由清理（purge）與恢復營業（closing）兩個呼叫方傳入，沒有第二份。
- **範圍（Q2 (a)）**：整個 stint 的個資一起延到 6 個月，不拆成兩段清理：店家聯絡與地址、店名與說明、KYC 個人欄位與證件、PayUni 自由文字、食品登錄字號與理由、同意紀錄的 IP 與 User-Agent，清理的範圍不變，只是晚到 6 個月。理由：拆成「註冊、身分與連線資料保存 6 個月、其餘 30 天」要兩段清理、兩個保留期與兩份法律文字，卻沒有對應的法定要求，多出的一段只是為了少保存幾個欄位 150 天。
- **終止合作關店**：`greatest(關閉後保留期, 申覆期限)` 的式子不變；兩個常數現在同值（4416 小時）：造成關店的終止在關店之前已生效，所以它自己的申覆期限一定不晚於關店加保留期；申覆期限那一項只在帳號上有生效時間晚於關店的另一筆未撤銷終止時才起作用（關店之後才發現的 21.3 事由、關聯帳號的終止）。保存來函（`hold_until`）、申覆的 legal hold 與恢復營業的規則不變：恢復只問保留期是否已過。
- **身分比對 hash（W8）**不變：措施擁有的快照不受清理影響，保留到終止撤銷為止。
- **還沒做、列為上線的阻擋條件**（計畫書 `2026-09-29-a3-buyer-seller.md` 的 I-legal 項）：(1) 四份法律文字（隱私權政策、賣家身分認證告知事項、食品業者登錄告知事項、賣家合作條款附件）仍寫「關店後 30 天」，依 rb §4 法律文字在 I-legal 改，本刀不改；現在程式保存 6 個月而公告寫 30 天，系統未上線所以沒有讀者受影響；(2) identity（註冊與登入資料、連線紀錄）與買家的保存期（Q4 (a)）是獨立的上線前步驟，本刀不碰：帳號刪除現在立刻清 identity 資料；(3) 新資料類別（詢問的說明與回覆、主管機關對轉報的回覆、來函的摘要與對象）的保存期（設計 B31）。

## 修訂：pricebook 刪除（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D1；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 6）

relay 轉 `seller.deleted` 的消費者不再包含 pricebook（`pricebook.seller.deleted` queue 已從 topology 刪除）。
