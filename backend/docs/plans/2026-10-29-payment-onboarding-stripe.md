# 賣家收款開通改走 payment port，作品集接 Stripe Connect

**Goal：** 單體的賣家收款開通（外部帳戶建立、開通連結或內嵌元件、狀態同步、開通閘門、同意書）只經 provider 中立的 payment port；作品集的 adapter 是 Stripe Connect 測試模式（Accounts v2），示範兩種收件方式：Stripe 收件（託管頁面，預設）與平台收件（Stripe 內嵌元件）。平台不再保存身分證號、銀行帳號與證件（D22），防詐改用銀行帳號 fingerprint；共用銀行帳號列入關聯帳號。單體與 `api/hawkr` 不再有 PayUni（允許清單見步驟 8）；付款、退款、撥款與物流的換供應商寫在單體計畫 B2、B3、B8–B10（收款模式依 D20）。驗收見步驟 9 與「完成定義」。

**上層：** 主清單見本機個人脈絡檔；單體遷移的全域約束見 `backend/docs/plans/2026-09-24-go-modular-monolith.md`。

**附件：** `2026-10-29-payment-onboarding-stripe/`：`payuni-inventory.md`（PayUni 盤點，快照 commit `e865c68`，G1–G11，「去向」欄由步驟 8 填）、`psp-compare.md`（Stripe Connect 與台灣金流商比較，官方出處逐項標示）；步驟 1 加入 `baseline-ci.md` 與 `spike/`，步驟 8 加入 `check_inventory.py`。

## 完成定義

- 單體與 `api/hawkr` 的 `git grep -n -i payuni` 只剩步驟 8 允許清單裡的列。
- 開通只經 port；開通閘門依 D10、D20；Stripe 收件與平台收件在測試模式各完成一次，記錄在進度表。
- 平台不存身分證號、銀行帳號與證件（D22）：KYC 申請的表與 request 沒有這些欄位，有 integration 測試與 mutation。
- 防詐身分雜湊與 KYC 個資分開保存（D3），銀行帳號列入關聯帳號（D4），有 integration 測試與 mutation。
- `payuni-inventory.md` 每列有「去向」（`check_inventory.py` 通過）。
- 改到的契約、表、文案與條款的所有消費端在同一個步驟內跟上；三端生成物無 drift。
- CI：與 `baseline-ci.md` 比對，新增失敗的 job 或 step 為 0。
- D1–D22 與 E1 的結論寫進 ADR；`docs/modules/seller.md` 與 ADR 2026-10-19、2026-10-18、2026-09-18 依修訂流程更新。

## 前提（開工時核對過，2026-10-29）

- 會改到或取代的既有部署是否在線：不在線（系統從未上線，根目錄 `AGENTS.md`；舊服務不部署，A3 計畫前提）。不受回滾窗口、相容性與資料搬移的約束；回滾就是 revert。已套用的 migration 仍不得修改（`backend/AGENTS.md` 單體規則 6），表與 view 的改動一律寫新 migration。
- 目標是作品集為主、保留在台灣上線的可能（Will 2026-10-29）。
- 不簽 PayUni 代理商合約，沒有 PayUni 測試帳號（Will 2026-10-29）。單體的 PayUni 程式無法對真實 sandbox 驗證。
- Stripe 不接受台灣的平台帳號；Connect 跨境撥款只開放平台在 US、UK、EEA、CA、CH；Stripe 沒有台灣的超商代碼、ATM、LINE Pay、街口（`psp-compare.md` §1，官方文件，2026-10-29；已確認）。Stripe adapter 只示範機制，不示範台灣上線情境。
- 平台收件的帳戶（`requirement_collection = application`）在 US 測試平台可以只靠 API 完成並啟用（步驟 1 實測，v1；已確認）。新整合用 Accounts v2，平台收件也能用內嵌元件（https://docs.stripe.com/connect/accounts-v2、https://docs.stripe.com/connect/onboarding；2026-11-02 核對）；v2 的 recipient 帳戶在 US 測試平台的實測在步驟 5（未確認）。
- 平台自己收款再轉給賣家可能落入《電子支付機構管理條例》的代理收付（第 3、5 條；第三方轉載，未在全國法規資料庫核對；未確認）。只影響真實上線，不擋本計畫。
- D3 的 5 年保存期是 Will 的產品決定（參考金流業「關係結束後 5 年」的紀錄保存慣例），法源未確認；上線前與條款一起核對。
- PayUni 在單體只有收款開通（盤點 G1–G3、G10），加上 `internal/legacy` 對 Java 舊服務的接縫；買家付款、退款、撥款、webhook、對帳與物流在 Java 舊服務（G4–G9），由 B2、B3、B8–B10 搬（`payuni-inventory.md`；已確認）。
- 過渡期：步驟 8 之後新開通的店家只有 Stripe 帳號；`internal/legacy/sellerserver` 回給 Java 的 `payuni_mer_id` 變成空值，Java 以 PayUni 下單的本機端到端不通，B2 完成後恢復（Will 2026-10-29 接受）。

**驗收基線：** commit `1c0b246` 的 CI：失敗的是 seller-app CI（unit and widget tests）、website e2e（Storybook 的 `product-certification.css` import 與 Playwright 1、2、3、5）、backend monolith（golangci-lint 在未改的檔）、backend migration inventory（16 列缺 disposition）；其餘通過。步驟 1 把每個失敗 job 的失敗 step 與錯誤行存成 `baseline-ci.md`（`gh run view <id> --log-failed`）。這四項不在本階段修；golangci-lint 與 migration inventory 是本階段會改到的 job，驗收以「錯誤行與基線逐行比對，新增為 0」判定，不看 job 的整體結論。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 店家能否曝光、收單由開通閘門推導 | view `seller.store_requirement_flags.kyc_approved`（現行：KYC 核准且 PayUni `outcome = 'APPROVED'`；步驟 4 起：平台核准且 `payment_accounts.transfers_active`）、`model.CapabilitiesOf`、`seller.pub_exposable_*`、discovery 與 `store_card.proto` 的「營業中」、`cmd/hawkr/dev_seed.go` 的種子店家 |
| 外部呼叫不在 transaction 內，一店一個 sync job | 現行：`seller/internal/payuni` 的 River job（唯一鍵）、`TestSyncJobIsUniquePerStoreInRiver`；步驟 4 起：`seller/internal/payment` 的同步 job（唯一鍵 `store_id`） |
| 外部事件先寫 inbox 再處理 | `backend/AGENTS.md` 單體規則 5；單體目前沒有 webhook inbox（PayUni webhook 由舊服務收），步驟 4 新建 `seller.payment_provider_events` |
| KYC 個資只存密文，身分比對只用 HMAC blind index | `seller/internal/kyc`、`kyc_identity_hashes`、`measure_identities`、`CheckKYCKeys`、`TestKYCNeverReturnsOrStoresPlaintext` |
| 關店兩段清理（30 天、184 天） | `seller/internal/purge`、`model.StoreRetireAfter`、`model.StoreRetention`、`payuni_registrations_purged_check` |
| 入駐同意的現行版本只有一個來源 | `internal/model/agreement_gen.go` 的 `CurrentAgreements`（`website/scripts/generate-legal.ts` 從 `website/docs/legal/` 產生）、`agreement_acceptances_kind_check`、已接受的紀錄、同意種類 `PAYUNI_TERMS`（proto enum、DB CHECK、manifest） |
| 後台權限字串在宣告表登記，缺漏時單體不啟動 | `internal/platform/adminperm/adminperm.go`（`seller.payuni_*`）、`internal/httpapi/adminauth.go` 的宣告表 |
| 模組依賴圖的唯一來源 | `internal/arch/modules.go`（seller 與 `platform/payuni`） |
| `pub_*` 與 migration 一致 | `TestPubContractsMatchMigrations`（`cmd/hawkr`） |
| 共用文案與法務生成物無 drift | `make uitext`、`ui-text-generated-diff.yml`、`generate-legal.ts` 的生成物 |
| 審核待辦種類 | `review_queue.proto` 的 `REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY`、seller 的 `review_*` 函式（10 支） |

## 規模

| | 單體（本計畫） | Java、Go 舊服務（不在本計畫） | 產生計數的位置 |
|---|---|---|---|
| 檔案 | 約 160（seller 101、測試 31），另有 `internal/legacy/{sellerrelay,sellerconsumer,sellerserver}` | Java 421、seller-service 151、gateway 40 | `payuni-inventory.md`「計數」；`git -c core.quotepath=off grep -IlP -i "(?<!backend-)pay_?uni"` |
| client | website 107、admin 19、seller-app 89、buyer-app 28、packages 25（含舊服務的呼叫） | | 同上 |
| 對外契約 | `seller/v1alpha1/seller.proto` 的 `KYCFlowStatus` 與 PayUni 欄位（56 筆）、`admin/seller/v1alpha1/seller.proto` 的 4 支 PayUni RPC、`review_queue.proto` 1 個 kind、同意種類 `PAYUNI_TERMS`、`store_card.proto`、`discovery.proto` 的說明 | `order.proto`、舊 `seller.proto` | `git grep -n -i payuni -- api/hawkr` |

## 範圍

- 做：payment port 的開通部分；Stripe Connect adapter（測試模式）的兩種開通；開通閘門改看 port 的能力狀態；同意種類改成 provider 中立；防詐身分雜湊的獨立保存；銀行帳號列入關聯帳號；刪除單體的 PayUni 與 `internal/legacy` 的 PayUni 接縫；B 階段需求。
- 不做：Java 與 Go 舊服務的 PayUni（B 階段搬時刪，避免改兩次）；台灣金流商 adapter（沒有合約與測試帳號）；代理收付的法律意見。
- 延後：
  - live mode 的銀行帳號 fingerprint 範圍核對（D12 的前提；觸發：上線前）。
  - 台灣金流商 adapter（觸發：決定在台灣上線；先確認法規與合約）。
  - 撥款模型與手續費（代理商費反推、ADR `payuni-channel-fee-bearer`）重審（觸發：B2、B10 開工）。
  - 裝置與聯絡方式重疊、風險評分等防人頭措施（觸發：上線前）。
  - 本機付款端到端恢復（觸發：B2 完成）。
  - D3 保存期與告知的法務核對（觸發：上線前）。
  - 賣家端「管理收款帳戶」入口與 `account_update` 連結（觸發：賣家要求改資料，或上線前的設定頁需求；D24 Q5）。
  - 收款帳戶的首次啟用時間與補件期限（`requirements.summary.minimum_deadline`）欄位與期限前提醒（觸發：A8 開通結果通知開工；D13 2026-11-03 修訂）。

## 步驟

- [x] **1. Spike：Stripe 測試模式的事實，與 CI 基線**（被擋於：需要人做的事）
  - 範圍：orchestrator 先把基線 CI 的失敗存成附件 `baseline-ci.md`。之後在 Stripe 測試帳號（US，見 D8）上以附件 `spike/` 的腳本確認並記錄：connected account 可用國別（台灣可否）；Custom（`requirement_collection = application`）能否在測試模式以 API 送 KYC 並啟用；託管與內嵌 onboarding 的帳號連結與回跳；`requirements`（`currently_due`、`errors`）與 `charges_enabled`、`payouts_enabled` 的對應；`account.updated` webhook 的簽章驗證、event id、重送與亂序；外部銀行帳號 `fingerprint` 是否跨 connected account 一致；建立帳號的冪等鍵；證件以 Files API 上傳的用途值。每項附官方文件 URL 與實測輸出；輸出存檔前以 `spike/scrub.py` 遮掉金鑰、`acct_` 與 `evt_` 以外的識別碼與個資。
  - 消費端：無（spike）。
  - 驗收：`spike/README.md` 每個問題有答案、官方出處與遮罩後的實測輸出；`baseline-ci.md` 列出四個失敗 run 的失敗 step 與錯誤行。
  - 停止條件：US 平台無法建立 Custom 或 connected account 時，停下回報給 D7、D8。
  - 需要人做的事：Will 建立 Stripe 測試帳號，把測試金鑰存在本機 `secrets/`（2026-11-02 完成，國別 US）；webhook signing secret 由本機 `stripe listen --api-key` 產生。

- [x] **2. B 階段需求**（被擋於：無；可與步驟 1 並行）
  - 範圍：在單體計畫加需求並指向本計畫：B2（付款經 payment port、Stripe；`internal/legacy/sellerserver` 的 `payuni_mer_id` 由 port 的外部帳戶 ID 取代）、B3（物流改綠界，PayUni 物流退場）、B8–B10（退款、撥款、對帳經 port；撥款模型與手續費重審）。
  - 驗收：`git grep -n "2026-10-29-payment-onboarding-stripe" backend/docs/plans/2026-09-24-go-modular-monolith.md` 在 B2、B3、B8、B9、B10 各至少一筆。

- [x] **3. 防詐身分雜湊的獨立保存與銀行帳號關聯**（被擋於：D3、D4 已決；與 provider 無關；可與步驟 1 並行）
  - 範圍：新增 `seller.account_identity_hashes`（帳號、種類、HMAC、金鑰 ID、來源 stint、建立時間；不存明文），寫入時機與 `kyc_identity_hashes` 相同（同一個 transaction）；`kyc_identity_hashes` 仍是申請的雜湊、照 184 天清，新表不清；新表的列在來源 stint 關閉滿 5 年後由清理 job 刪除；`SnapshotMeasureIdentities` 改從新表取（涵蓋已清理的舊 stint）。`LinkedAccountHits` 納入銀行帳號（`kyc.sql` 的 `mi.kind IN ('ID_CARD','TAX_ID')` 與 `h.field <> 'BANK_ACCOUNT'`），命中種類標成銀行帳號，只列出、不自動終止。本步驟只處理平台自收的身分（現行 KYC）；`PROVIDER_HOSTED` 的訊號依 D12 在步驟 6 處理。依 ADR 修訂流程修訂 `2026-10-19-seller-kyc-payuni-consolidated`（「關店與個資保留」「身分比對與比對 hash」）與 `2026-10-18-seller-governance-consolidated`（關聯帳號）；`docs/modules/seller.md` §13、§23 同步；`website/docs/legal/` 加防詐保存目的的告知並重新生成同意文件。
  - 消費端：`git grep -n -E "SnapshotMeasureIdentities|LinkedAccountHits|kyc_identity_hashes|measure_identities|SharesBankAccount|CheckKYCKeys" -- backend api admin website seller-app`；admin 措施詳情的關聯帳號畫面；`make sqlc` 的生成物。
  - 不能動：KYC 明文只存密文；`CheckKYCKeys`（新表的金鑰 ID 納入啟動檢查）；處分快照保留到措施撤銷；已套用的 migration。
  - 驗收：單體 integration：舊 stint 已過 184 天清理、之後才建立的停權措施，快照含舊 stint 的身分（mutation：快照改回從 `kyc_identity_hashes` 取，測試要失敗）；來源 stint 關閉滿 5 年的列被刪（mutation：清理不刪，測試要失敗）；銀行帳號命中出現在關聯帳號且不自動建立終止（mutation：拿掉銀行帳號條件，測試要失敗）；`CheckKYCKeys` 的測試含新表；`atlas migrate validate`；`TestPubContractsMatchMigrations`；`make lint-monolith` 與基線比對新增為 0；admin 改到的測試。

> 2026-11-02 改寫：原步驟 4（PayUni 改名成中立）、5、6、8 的切法被 D13–D20 與 D2、D7、D9、D10、D11、D17、D19 的修訂取代。原切法會把 PayUni 的資料與狀態機搬進中立契約（開工時的設計審查 8 項照搬都出自這裡）。新的切法是從零建中立的收款帳戶，PayUni 與平台自建的 KYC 個資管線在同一波刪除。步驟 4–8 在同一個分支 `payment-onboarding` 完成，全部完成才進 `main`；分支內每個子步驟一個或多個 commit，每段 CI 綠（D21）。

- [x] **4. 中立的收款帳戶：port、資料、閘門、fake adapter、新契約**（被擋於：D13、D16、D19、D20、D21；**4–8 必須連著做**）
  - 範圍：
    - port（seller 模組 `internal/payment`）：`EnsureAccount`（冪等鍵是建立中列寫入時產生並保存的 UUID（D19 的 F2 修訂）；先在本地寫「建立中」的列，再在 transaction 外呼叫）、`CreateOnboardingSession`（託管連結或內嵌元件的 session，依 D2 的兩軸）、`FetchAccount`（帳戶快照：capability 狀態、requirements 原始 JSON、`disabled_reason`）、`ParseEvent`（驗簽並取出 event id 與帳戶 ID）。能力描述宣告 `requirement_collection`（`provider`／`platform`）與 `onboarding_surface`（`hosted`／`embedded`）。不做 `SubmitKYC`。in-memory fake adapter 供測試與本機。
    - 資料：新表 `seller.payment_accounts`（`store_id` 主鍵、provider、外部帳戶 ID（建立中可為空）、`requirement_collection`、狀態快取（D13 的 6 值）、`transfers_active`、`payouts_active`、requirements 原始 JSON、`disabled_reason`、`fetched_at`），一店一列、關店再開沿用（D19）；不存個資，不隨 184 天 KYC 清理刪除，隨店家列的最終清理刪除（`seller/internal/purge` 跟上，取代 `payuni_registrations_purged_check`）；新表 `seller.payment_provider_events`（provider、event id 唯一、收到時間、處理時間、帳戶 ID）作為 webhook inbox（單體規則 5）。不從 `payuni_*` 改名。
    - 同步：一店一個 River job（唯一鍵 `store_id`）：`FetchAccount` 在 transaction 外，結果在 transaction 內整列覆寫快取；inbox 的事件與管理員「重新整理狀態」都只排這個 job；輪詢補漏是 River periodic job（每 15 分鐘），排 `UNDER_REVIEW`、`ACTION_REQUIRED` 的帳戶，以及 `DISABLED` 但原因碼不是終局（`rejected*`、`unsupported*`）的帳戶（5a 之後 restricted 加 `contact_stripe`／`no_resolution` 也是 `DISABLED`，Stripe 可以解除，掉了 webhook 時只有輪詢補得到；終局的不會再啟用，不輪詢），再加上 inbox 有逾時未處理事件的任何狀態的帳戶。沒有觀測歷史表、沒有事件時間排序（重讀即最新）。
    - 時點（D16）：平台審核核准的同一個 transaction 排入 `EnsureAccount` job；建立失敗由 River 退避重試，用完進審核待辦的新種類 `PAYMENT_ACCOUNT_CREATE_FAILED`（管理員只能「重試建立」）。
    - 閘門（D10 修訂、D20）：view `seller.store_requirement_flags` 改成平台核准且 `payment_accounts.transfers_active`；依賴它的 view 重建；`model.CapabilitiesOf` 跟上。
    - 契約（新增，不改名）：seller 的「收款帳戶」讀取與開通 session RPC；admin 的「讀帳戶」「重新整理狀態」「重試建立」RPC；`PaymentAccountStatus` enum（D13）；審核待辦 kind `PAYMENT_ACCOUNT_CREATE_FAILED`。PayUni 的舊 RPC 與 enum 留到步驟 8 刪（分支內並存，不進 `main`）。`make connect`、TS 與 Dart 生成、`make uitext`。
    - 本機種子：`cmd/hawkr/dev_seed.go` 以 fake provider 寫 `transfers_active = true` 的帳戶，種子店家仍可曝光。
  - 消費端：本步驟只新增；閘門的消費端 `git grep -n -E "store_requirement_flags|kyc_approved|CapabilitiesOf|pub_exposable" -- backend api/hawkr admin/src website packages ':!**/gen/**'`（含 `exposure_rule_dataset_integration_test.go` 等以 PayUni 條件建資料的測試）。`kyc_approved` 在閘門改寫後不再只代表 KYC，改名為 `onboarding_complete`（view、admin proto、生成物同步）。
  - 不能動：已套用的 migration；外部呼叫不在 transaction 內；一店一個 sync job；PayUni 的舊路徑在本步驟仍可編譯（步驟 8 才刪）。
  - 驗收：`atlas migrate validate`、`make sqlc` 無 drift、`TestPubContractsMatchMigrations`；unit 與 `go vet -tags integration`；integration（CI）：核准後排入 `EnsureAccount`，重複核准不建第二個帳戶（mutation：每次呼叫產生新的冪等鍵、不用列上保存的，測試要失敗）；同一店兩個同步請求只有一個 job（mutation：拿掉唯一鍵，測試要失敗）；曝光測試 `TestExposure*`（以 `rg -n "func Test.*Expos" backend/internal/modules/seller` 定位）在 `transfers_active = false` 時不可曝光（mutation：閘門拿掉 `transfers_active`，測試要失敗）；同一 event id 第二次寫 inbox 不排 job（mutation：拿掉唯一約束，測試要失敗）；輪詢只排三種狀態的帳戶（mutation：條件拿掉 `ENABLED` 的排除，測試要失敗）。`buf lint`；三端生成物與 uitext 無 drift；`make dev-seed` 後種子店家可曝光；`make lint-monolith`、migration inventory 與基線比對新增為 0。
  - 完成後的狀態：只有 fake adapter；新舊契約在分支內並存。

- [x] **5a. 收款帳戶狀態改依 v2（D13 2026-11-03 修訂）**（被擋於：步驟 4，D13）
  - 範圍：`internal/payment` 的 `Snapshot` 改成中立的轉帳能力快照（狀態、行動方、原因碼）加 `TransfersActive`、`PayoutsActive`、`Requirements`；`StatusOf` 照 D13 修訂的對應；新 migration 刪 `details_submitted`、`requirements_due`、`disabled_reason`，加 `status_reason`，狀態 CHECK 改 5 值；proto（seller、admin seller、admin reviewqueue）刪 `ONBOARDING`、`disabled_reason` 改 `status_reason`；uitext、三端生成物；fake adapter；種子與測試。
  - 消費端：`git grep -n -E "ONBOARDING|DetailsSubmitted|RequirementsDue|DisabledReason|details_submitted|requirements_due|disabled_reason|PaymentOnboarding" -- backend api admin/src website/src seller-app/lib packages ':!**/gen/**' ':!**/*.g.dart' ':!backend/docs/plans/**'`（排除與收款無關的同名字）；`backend/docs/modules/seller.md`、seller 模組 `AGENTS.md`。
  - 不能動：已套用的 migration；步驟 4 的不變量；閘門只看 `transfers_active`。
  - 驗收：`StatusOf` 表格測試涵蓋對應表每一列與「多筆取最嚴重」「未知值」（mutation：`provide_info` 的判斷拿掉，ACTION_REQUIRED 的列要失敗；未知值改回 DISABLED，對應列要失敗）；`atlas migrate validate`、`make sqlc`、`make connect`、TS／Dart 生成、`make uitext` 無 drift；`buf lint`；gofmt、`go vet`（含 `-tags integration`）、unit、`make lint-monolith`；admin、website `tsc`；integration 在 CI。

- [x] **5b. Stripe adapter（Accounts v2）與 webhook 入口**（被擋於：5a，D7、D8、E1；sandbox 金鑰）
  - 範圍：`internal/platform/stripe`，只做開通需要的 API，用 Accounts v2（`/v2/core/accounts`，configuration `recipient`，capability `stripe_transfers`），D8 的 US 測試帳號。`EnsureAccount` 以列上保存的 UUID 當 `Idempotency-Key`（v2 期限 30 天）；本地「建立中」的列已有外部 ID 時不再建立。`CreateOnboardingSession`：Stripe 收件用 Account Link（託管），平台收件（D2 修訂）用 Account Session（內嵌元件）。`ParseEvent`：thin events 驗簽，只收平台自己的 `v2.core.account*`（`.created`、`[...].updated`、`[configuration.recipient].capability_status_updated`，`related_object.id` 是帳戶 ID；連線帳戶的 `v1.*` thin events 不訂閱；`spike/README.md` Q10）；Stripe 收件用 `dashboard=express`、平台收件用 `dashboard=none`（兩者 `fees_collector`/`losses_collector` 都是 `application`，Q9），handler 只寫 inbox 並排同步 job。入口是 `internal/httpapi` 的 `POST /webhooks/stripe`，不經身分驗證（簽章就是驗證），掛在 Connect 與 session 中介層之外（開工時以 `rg -n "Handle\(" backend/internal/httpapi/*.go` 找到路由組裝處並在驗收列出）；Stripe 直接打單體，不經 website 反代；本機以 `stripe listen --forward-to localhost:8080/webhooks/stripe` 轉送。缺金鑰時開通 RPC 回 `failed_precondition`，serve 照常啟動。adapter 把 v2 回應整理成 5a 的中立快照，對應表見 D13 2026-11-03 修訂。限流與 429 退避在 adapter 內，不上提到 port。
  - 設定與事件處理的決定（Will，2026-11-03）：`PAYMENT_PROVIDER=stripe` 讀 `STRIPE_SECRET_KEY`、`STRIPE_WEBHOOK_SECRET`、`STRIPE_REQUIREMENT_COLLECTION=provider|platform`（預設 `provider`：Stripe 收件，Account Link 加 `dashboard=express`；`platform`：平台收件，Account Session 加 `dashboard=none`；其他值啟動失敗）；缺 `STRIPE_SECRET_KEY` 與沒有 provider 相同（警示 log，serve 照常啟動，開通 RPC 回 `failed_precondition`），有金鑰卻缺 `STRIPE_WEBHOOK_SECRET` 啟動失敗。驗過簽章但不處理的事件（`v1.*`、`v2.core.account_person.*`）：port 回 `ErrIgnoredEvent`，`ReceiveEvent` 回 nil、不寫 inbox 不排 job，入口回 2xx（https://docs.stripe.com/webhooks）；簽章或格式不合回 400。
  - 實測（orchestrator 在本機跑，結果補進 `spike/README.md`）：v2 建帳戶與冪等重放；thin event 的實際型別名與簽章；Stripe 收件時平台能否讀到外部銀行帳號的 `fingerprint`（D12，讀不到就停下回報）；平台改店名後是否需要同步給 Stripe（D17：不同步，只確認 Stripe 端不擋）。
  - 消費端：步驟 4 的 port；`git grep -n -i "webhook" -- backend/internal/httpapi website/next.config.* website/src/middleware*`。
  - 不能動：步驟 4 的不變量。
  - 驗收：contract 測試以實測錄下、遮罩過的回應重放（CI 不打網路）；webhook mutation：簽章錯誤要拒絕、同一 event id 第二次不再排 job；本機手動開通（client 畫面在步驟 7，本步驟用 `make dev-token` 加 `buf curl` 呼叫 seller 的開通 session RPC 取得 Account Link，在瀏覽器走完）：`payment_accounts.transfers_active = true`、狀態 `ENABLED`、該店家在 discovery 可曝光，三項輸出貼進進度表。內嵌元件需要網頁，平台收件模式的手動開通移到步驟 7 的驗收。
  - 停止條件：v2 的 recipient 帳戶在 US 測試平台建不起來，或 fingerprint 讀不到，停下回報。

- [x] **5c. 收款聯絡信箱（D23）**（被擋於：5b，D23）
  - 範圍：KYC 申請加「收款聯絡信箱」（草稿可空，送出時必填，格式與長度同 `stores.support_email` 的規則但上限 254）；proto `KYCApplication`／`SaveKYCDraftRequest` 加欄位、`SubmitKYC` 缺值回 invalid_argument（field violation）；核准的 transaction 把它複製到 `payment_accounts.contact_email`（新 migration，NOT NULL）；`EnsureInput.ContactEmail`，Stripe `CreateAccount` 送 `contact_email`，fake adapter 同步；integration、unit 跟上。seller-app 的表單欄位（預設帶入登入信箱、可改）在步驟 7 做。
  - 消費端：`rg -n "SaveKYCDraft|SubmitKYC|InsertPaymentAccount|EnsureInput|CreateAccount\(" backend/internal seller-app/lib api --glob '!**/gen/**'`。
  - 驗收：CI 等價檢查；mutation：`CreateAccount` 不送 `contact_email` 時 client 測試失敗；送出缺信箱的申請時測試失敗；核准不複製時 integration 失敗。
- [x] **6. 防詐訊號改用 fingerprint**（被擋於：步驟 5，D12）
  - 範圍：同步 job 讀到外部銀行帳號的 `fingerprint` 時，以 HMAC 寫入步驟 3 的 `account_identity_hashes`（新 field `BANK_FINGERPRINT`，來源是 provider；不存明文 fingerprint）；`LinkedAccountHits` 涵蓋它。本步驟只加 fingerprint 這條訊號；平台收的身分證號、銀行帳號與證件（D22）在步驟 7 的畫面不再送出之後，於步驟 8 與 proto 欄位、client 一起刪（避免中間 CI 編譯失敗）。
  - 消費端：`git grep -n -E "account_identity_hashes|LinkedAccountHits|SnapshotMeasureIdentities" -- backend/internal backend/cmd api/hawkr admin/src`。
  - 不能動：防詐雜湊只存 HMAC；處分快照保留到措施撤銷；已套用的 migration。
  - 驗收：integration：兩店綁同一 fingerprint 出現在關聯帳號且不自動終止（mutation：不寫 fingerprint，測試要失敗）；`atlas migrate validate`；與基線比對新增為 0。

- [ ] **7. client 的流程與畫面**（被擋於：步驟 4、5；D24）：畫面依附件 `step7-screen-drafts.md`（Will 2026-11-03 同意）。拆成四個子步驟，7b–7d 在 7a 之後各自做；每個子步驟的 CI 都要綠（D21）。7a 到步驟 8 之間的預期行為：核准仍會排 PayUni 綁定（`kyc/admin.go` 的 `payuni.RequestBind`），因為身分欄位不再必填，新核准的店家會停在 `BINDING` 或變成 `BIND_FAILED` 並產生審核待辦；這只發生在未合併的分支（D21），步驟 8 刪掉綁定。7b–7d 的畫面不得依賴 `KYCFlowStatus`：平台審核讀 `KYCApplication.status`，收款讀 `PaymentAccount.status`。
- [x] **7a. 後端的送出規則與文案，兩端停送 PayUni 同意**（被擋於：D24）
  - 範圍：
    - 修改 `backend/internal/modules/seller/internal/kyc/kyc.go` 的 `requirements`：刪掉 `real_name`、`english_name`、`bank_name`、`bank_code`、`bank_account`、`id_card_front`、`id_card_back`、`bank_book`、`individual.id_card_no`、`business.rep_id_card_no`。
    - 新增 migration，重新定義 `kyc_applications_complete_check`（現行定義在 `db/migrations/20261102234040_payout_contact_email_required.sql`，`kyc.go` 的註解稱它為保底）：送出後只要求統編、公司資料與 `payout_contact_email`，與新的 `requirements` 一致。同步更新 `schema.sql`。只改 Go、不改 CHECK，送出會被 DB 擋下。
    - 修改 `model/agreement.go` 的 `SubmitKYCAgreements`：拿掉 `PAYUNI_TERMS`。`AgreementKinds()`、`seller-agreements.json` 與生成檔留給步驟 8（D11）：`GetSellerAgreements` 要求每個種類都有現行版本，單獨拿掉會讓所有 client 讀不到同意書。
    - 兩端停送 `PAYUNI_TERMS` 並拿掉它的勾選：修改 `website/features/seller-center/profile/certifications/CertificationsClient.tsx`（同意清單）與 `seller-app/lib/features/seller/data/seller_agreements_repository.dart`（送出的種類），以及兩端的確認頁勾選與對應測試：website 的 `payuniAccepted`、`agreementsAccepted`；seller-app 的 `payuniChecked`、`onPayuniChanged`（screen 與 `kyc_wizard_steps.dart`）。送出後店名鎖定的提示（`kPayuniLockOnSubmitNotice`）保留：後端的 `IdentityLocked` 還在，步驟 8 與 D17 一起刪。這一步要在 7a 一起做，因為 `CheckAgreements` 會拒絕多出來的種類。
    - 修改共用文案目錄 `backend/internal/platform/uitext/catalog/*.json`：
      - 新增 `seller.KYCStatus`（DRAFT、SUBMITTED、APPROVED、REJECTED）的徽章文字。沒有申請，或申請已取消，在畫面上都是「未送出」（client 依 `GetMyKYC` 沒有申請判斷）；CANCELLED 只出現在取消的回應，不另外顯示。
      - 新增 `seller.PaymentAccountStatus` 的徽章、說明與動作（依附件第 2 節的表）。
      - 修改 `SELLER_REQUIREMENT_ONBOARDING_COMPLETE` 的閘門提示（`zh-TW.json` 約 795 行），兩個語系都改成平台審核與收款帳戶都完成。
      - 然後 `make uitext`。開通任務卡的兩項標題不是契約值，留在 client（7b、7c）。
    - 條款：修改 `website/docs/legal/seller-kyc-notice.md`（依附件第 4 節），版本行升到 7a 的日期，並跑 `gen:legal`。內容改變就要換新版本；系統還沒上線，舊的同意紀錄不轉換。manifest 的 `PAYUNI_TERMS` 不動。
    - 不動：proto 欄位、密文欄位、證件檔、enum。
  - 消費端：
    - 後端：`rg -n -i "SubmitKYCAgreements|AgreementPayUniTerms|CheckAgreements|missingFields|complete_check" backend/internal backend/cmd`；寫死必填清單的測試：`rg -n '"(real_name|english_name|bank_name|bank_code|bank_account|id_card_front|id_card_back|bank_book)"|individual\.id_card_no|rep_id_card_no' backend --glob '*_test.go'`，逐檔判斷。
    - client：`rg -n "PAYUNI_TERMS|payuniTerms|payuniChecked|onPayuniChanged|payuniAccepted|AgreementKind\.[A-Za-z_]*PAYUNI" website/lib website/features seller-app/lib seller-app/test admin/src --glob '!**/gen/**' --glob '!**/*.pb*.dart'`。
    - 目錄的消費端：兩端的 `check-enum-text` 與 golden。
  - 不能動：統編、公司資料與店家欄位（`MissingStoreFields`）的必填規則；`payout_contact_email` 的必填；`GetSellerAgreements` 的回應；已存在的申請資料。
  - 驗收：
    - CI 等價檢查：monolith（含 `atlas migrate validate`）、`gen:legal --check`、ui-text 生成比對、兩端 `check-enum-text`、website `test:unit` 與 `pnpm test`、seller-app 改到的測試。
    - integration：不帶身分、銀行、證件與 `PAYUNI_TERMS` 的個人與公司申請可以送出並核准；帶了 `PAYUNI_TERMS` 的送出被拒。
    - mutation：把 `real_name` 加回 `requirements`、把 `PAYUNI_TERMS` 加回 `SubmitKYCAgreements`、CHECK 改回原定義（integration），各自讓測試失敗。
- [ ] **7b. website**（被擋於：7a）
  - 範圍：附件第 1、2 節。
    - 修改 `CertificationsClient.tsx` 與 `lib/seller/kyc.ts`：表單刪減，並加收款聯絡信箱。
    - 改成兩區狀態頁：平台審核讀 `KYCApplication.status`，收款讀 `GetMyPaymentAccount`。
    - 新增收款帳戶區元件：HOSTED 導頁並處理 `?payment=return|refresh`；EMBEDDED 用 `ConnectAccountOnboarding`（`@stripe/connect-js@3.4.6`、`@stripe/react-connect-js@3.4.4`，`NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY`）。
    - 修改 `website/lib/security/csp.ts` 與它的測試：依 Stripe Connect 內嵌元件文件放行 `script-src`、`frame-src`、`connect-src`，附文件 URL。
    - 新增給 seller-app 回跳的免登入頁 `website/app/[locale]/(public)/seller-app/payment-return/page.tsx`：`?state=done|expired`，內容是「請回到 hawkr 賣家 App」。加一條測試，證明未登入也不會被導去登入頁。
    - 開通任務卡（`dashboard/onboardingTasks.ts`）拆成兩項，標題是 client 自己的文字；錢包與儀表板的 PayUni 狀態改看收款帳戶。
    - 後端（D25）：`internal/platform/stripe` 的 `CreateAccountSession` 帶 `disable_stripe_user_authentication=true`，contract 測試斷言請求本文。
  - 消費端：`rg -n "kycFlow|KYCFlowStatus|PayuniStatusBody|payuniStateTitle|kycInPayuniStage" website --glob '!**/gen/**' --glob '!node_modules'`。
  - 不能動：退回提示與逐項問題的行為；`.seller-cert__status` 的 tone 樣式；不寫 `PaymentAccountStatus` 或 `KYCStatus` 的本地對照表（文案只讀目錄）。
  - 驗收：
    - `test:unit`、`pnpm test`、lint、`tsc`、build（帶 `API_BASE_URL` 覆寫）。
    - e2e 與 Storybook 只在 CI 跑，與基線比對。
    - mutation：EMBEDDED 時改走導頁、return 時不重讀狀態、CSP 拿掉 Stripe 來源，Account Session 不帶 `disable_stripe_user_authentication`，各自讓測試失敗。
    - 本機 sandbox 手動驗收：單體設 `STRIPE_REQUIREMENT_COLLECTION=platform`，建一家新店，送出並核准，然後在 website 用內嵌元件完成開通。判定同步驟 5：`transfers_active = true`、`ENABLED`、可曝光。店 1009 是金流商收件，不能用。
  - 停止條件：內嵌元件不能用 v2 帳戶與 `2026-09-30.preview` 初始化時（5b 只驗過 v1 的 `account_sessions`），停下回報，不改 API 版本或 SDK 版本。2026-11-03 實測可以初始化並進入表單（`spike/README.md` Q15），走完開通仍是本步驟的手動驗收。
  - 需要人做的事：Will 在本機環境放 sandbox 的公開金鑰（2026-11-03 完成，`NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY` 放在本機個人脈絡檔記錄的位置）。
- [ ] **7c. seller-app**（被擋於：7a、7b 的回跳頁）
  - 範圍：
    - 修改 `data/kyc_fields.dart`、`data/kyc_form.dart`、`presentation/kyc_wizard_steps.dart`：精靈刪減（個人兩步），營運資訊步驟加收款聯絡信箱（預設登入信箱、可改）。
    - 修改 `seller_certification_upload_screen.dart` 的 `_LockedStatusView`：改成兩區，平台審核讀 `KYCApplication.status`，收款讀 `GetMyPaymentAccount`。
    - 新增收款帳戶的 repository 與 provider。
    - HOSTED：以 `launchUrl(externalApplication)` 開外部瀏覽器，`return_url` 與 `refresh_url` 指向 7b 的回跳頁。新增 `--dart-define=HAWKR_WEB_ORIGIN`，沿用 `config/flavor.dart` 的 `resolveGatewayHost` 做法：dev 預設 `http://localhost:3000`，stage 與 prod 缺值時丟 `StateError`。同步更新 `config/dart-defines.example.json`。
    - 回到前景時的重讀：修改 `data/seller_status_refresh.dart`，把 `GetMyPaymentAccount` 併進同一輪重讀。認證畫面在 `AppLifecycleState.resumed` 時觸發，從外部瀏覽器回來帶 `fresh: true`，跳過 30 秒節流。
    - EMBEDDED：顯示「請到網頁版完成」。
    - 開通任務卡（`data/onboarding_tasks.dart`）拆成兩項；錢包閘門（`wallet_screen.dart`）改看收款帳戶。
  - 消費端：`rg -n "kycFlowCopy|KYCFlowStatus|PayuniStatusBody|payoutContactEmail|buildOnboardingTasks|seller_status_refresh|SellerStatusRefresh" seller-app/lib seller-app/test --glob '!**/*.pb*.dart'`。
  - 不能動：不新增 deep link 或依賴（D24）；不寫 `PaymentAccountStatus` 或 `KYCStatus` 的本地對照表。
  - 驗收：
    - Flutter 兩段驗證：本機跑 analyze、custom_lint、改到的測試、讀原始碼的守門測試與 `deadcode.sh`；有 `part` 生成檔改動時跑 build_runner 比對；完整測試與 coverage 待 CI。
    - mutation：信箱不帶預設值、resumed 時不呼叫 `GetMyPaymentAccount`、從瀏覽器回來沒帶 `fresh: true`，各自讓測試失敗。
- [ ] **7d. admin 收款帳戶卡**（被擋於：7a）
  - 範圍：附件第 3 節。
    - 新增 `admin/src/app/admin/merchants/[id]/PaymentAccountCard.tsx` 與它的測試：`GetPaymentAccount`、`RefreshPaymentAccount`、`RetryPaymentAccountCreate`；not_found 時顯示尚未建立。
    - 修改 `page.tsx`：把卡片放在 `PayUniStatusCard` 的位置，錨點 `#payment-account`。
    - 修改 `queue/queue-item.ts`：收款帳戶待辦的連結帶這個錨點。
    - PayUni 卡與身分、銀行的 KYC 卡留給步驟 8。
  - 消費端：`rg -n "paymentAccount|PayUniStatusCard" admin/src`。
  - 驗收：
    - `pnpm test`、lint、`tsc`。
    - mutation：沒有 `create_failed_time` 時仍顯示重試按鈕、not_found 當成錯誤，各自讓測試失敗。

- [ ] **8. 刪除 PayUni、舊契約與 KYC 個資管線的程式**（被擋於：步驟 4–7，D6、D22）
  - 範圍：D22 的 KYC 個資管線：刪身分證號、銀行帳號、證件的 proto 欄位、request 驗證、密文欄位與證件檔參照（新 migration）、`kyc_identity_hashes` 的 `ID_CARD_NO`、`REP_ID_CARD_NO`、`BANK_ACCOUNT` 種類、client 的對應欄位；`account_identity_hashes` 已有的列保留到 5 年清理；`CheckKYCKeys` 只檢查仍在用的金鑰；清理程式（`seller/internal/purge`）跟上。統編是市集登記資料（D9），仍由平台收並計算雜湊。PayUni：刪 `internal/platform/payuni`、`seller/internal/payuni`、`payuni_*` 表（新 migration drop）、`CheckNameUnlocked`／`IdentityLocked`（D17）、admin 的 4 支 PayUni RPC、`PAYUNI_BIND_RETRY`、`PayUniRemedy`、`KYCFlowStatus` 的 PayUni 值與 `BINDING`／`BIND_FAILED`／`FORWARDED`／`UNDER_REVIEW`／`PENDING_INFO`（D15）、`PAYUNI_TERMS`（D11：CHECK 改掉、不轉換舊紀錄）、`seller.payuni_*` 權限、`modules.go` 的 `platform/payuni`、`cmd/hawkr` 的 `readPayUni` 與 `PAYUNI_*`、`internal/legacy/sellerrelay` 的 `merchant.verification.*` 與 `sellerconsumer` 的 PayUni 審核通知（D18）、`sellerserver` 的 `payuni_mer_id` 回空值（Stripe 的 `acct_` 不是 PayUni 商店代號；B2 以 port 的外部帳戶 ID 取代這個接縫）；CodeMeta 照本機脈絡檔規則；client 刪掉舊 RPC 與 enum 的使用；CARRYOVER 聯絡欄位長度一列改成平台自己的上限並結案；`backend/docs/第三方整合/` 的 PayUni 文件檔頭改標「Java 舊服務的現行規格，B 階段刪除」；建立附件 `check_inventory.py` 並填完 `payuni-inventory.md` 的去向。
  - 消費端：附件 `step4-inventory.md` 的 A、B、C、E 類（以 `git grep -l -i payuni -- backend/internal backend/cmd api/hawkr admin/src website seller-app/lib packages ':!**/gen/**'` 重跑）；D 類（買家付款、訂單、物流、Java 推播 `PAYUNI_STATUS`、物流的 `PayUniSubmitStatus`）留給 B 階段。
  - 允許清單：`internal/legacy/**` 中 Java 契約生成的欄位名（B2 刪）、`backend/docs/plans/**`、指向 B 階段的文件、`backend/docs/第三方整合/` 的 PayUni 文件、buyer 與 content 模組 `CARRYOVER.md` 指向 B 階段的列、D 類檔；其餘單體與 `api/hawkr` 0 筆。
  - 驗收：`git grep -n -i payuni -- backend/internal backend/cmd api/hawkr` 逐筆屬於允許清單；`check_inventory.py` 通過，mutation：一列去向改回 TBD，腳本要失敗；`buf lint`、`buf breaking`（`v1alpha1` 由 `api/buf.yaml` 的 `ignore_unstable_packages` 略過，預期通過）；KYC 申請的表與 request 沒有身分證號、銀行帳號欄位（`TestKYCNeverReturnsOrStoresPlaintext` 改寫成「沒有這些欄位」，mutation：加回一個欄位，測試要失敗）；單體 build、unit、integration（CI）；`make sqlc`、`atlas migrate validate`；三端生成物無 drift；與基線比對新增為 0。之後開 `payment-onboarding` → `main` 的 PR。

- [ ] **9. 收尾**：完成定義逐項核對；獨立設計審查（design-review skill）；`check_inventory.py` 留在附件（一次性）；延後項附觸發條件搬回主清單；A8 的需求清單加「賣家收款開通結果的通知」（D18）；ADR（D1–D22）寫進 `backend/docs/adr/`，ADR 2026-10-19、2026-10-18、2026-09-18 依修訂流程更新；需要人做的事列給 Will。驗收：完成定義每項附證據；CI 與 `baseline-ci.md` 比對新增為 0。

## 決定

- **D1 作品集的金流 adapter**（已決 2026-10-29，Will；擋住：全部）：Stripe Connect 測試模式。不簽 PayUni（測試帳號要簽約）；作品集要展示國際標準。
- **D2 開通模式**（已決 2026-10-29，Will；擋住：4–7）：port 支援 `PROVIDER_HOSTED` 與 `PLATFORM_COLLECTED`。（2026-11-02 修訂，Will，E2：「誰收件」與「用什麼介面」是兩軸；Stripe 收件用託管頁面為預設，平台收件改用 Stripe 內嵌元件，不再自建 KYC 表單，見 D22）
- **D3 防詐身分雜湊的保存**（已決 2026-10-29，Will；擋住：3）：與 KYC 個資分開保存 5 年（來源 stint 關閉起算）；明文仍 184 天清。法源未確認，見前提。
- **D4 共用銀行帳號**（已決 2026-10-29，Will；擋住：3）：列入關聯帳號，只標記、不自動終止。
- **D5 本計畫的範圍**（已決 2026-10-29，Will）：只做單體的收款開通；付款、退款、撥款、物流寫成 B2、B3、B8–B10 的需求。
- **D6 單體的 PayUni adapter**（已決 2026-11-02，Will，選 A；擋住：4、8；步驟 1 確認 Custom 在測試模式可用）
  - A：刪除（git 可找回）。必要條件不成立：沒有測試帳號，無法驗證；`PLATFORM_COLLECTED` 由 Stripe Custom 示範。（建議，前提是步驟 1 確認 Custom 可用）
  - B：保留在 port 後面當第二個 adapter，只有 fake 測試——保留台灣上線的起點，但無法對真實服務驗證，會隨 port 演進而失真。步驟 8 改走 B 分支。
- **D7 Stripe 的預設開通模式**（已決 2026-11-02，Will，選 A；`PLATFORM_COLLECTED` 以 US 的 Custom 帳號示範：SG 卡在 `proof_of_liveness`，TW 只能開 transfers；擋住：4–6）（2026-11-02 修訂，Will：平台收件的示範改用 Accounts v2 加內嵌元件，見 D2、E1）
  - A：預設 `PROVIDER_HOSTED`（Stripe 託管或內嵌 onboarding，Stripe 收證件與銀行帳號），`PLATFORM_COLLECTED` 用 Custom 示範。平台持有的 PII 最少。（建議）
  - B：預設 `PLATFORM_COLLECTED`——沿用現行 KYC 畫面最多，但平台持有最多 PII。
- **D8 Stripe 測試平台帳號的國別**（已決 2026-11-02，Will，選 C：測試帳號建成 US，沿用不改；擋住：5）
  - A：SG（亞洲、英文介面、可示範本地幣；平台與 connected account 同國）。（建議，暫定）
  - B：JP——同為亞洲，但 KYC 欄位與文件以日文為主。
  - C：US 平台加跨境撥款——示範跨境，但離台灣情境更遠，且台灣能否當 connected account 未確認。
- **D9 平台審核與金流商驗證的分工**（已決 2026-10-29，Will，選 A；擋住：4、6）（2026-11-02 修訂，Will：送給金流商的欄位清單作廢，平台不收身分與銀行資料，見 D22；平台審核只管市集政策與統編）
  - A：平台審核只管市集政策（店家資訊、經營項目、登記資料比對），身分與銀行帳號由金流商驗證。`PLATFORM_COLLECTED` 送給金流商的欄位清單：本人姓名、身分證號、出生日期、住址、電話、email；公司型另加公司名稱、統編、登記地址、負責人；銀行代碼與帳號；證件正反面；服務協議接受（時間、IP）。平台仍以身分證號、統編、銀行帳號計算防詐雜湊。（建議）
  - B：平台審核繼續核對身分證件（現行）——兩方各驗一次；`PROVIDER_HOSTED` 時平台沒有證件可審。
- **D10 開通閘門**（已決 2026-10-29，Will，選 A；擋住：4）（2026-11-02 修訂，Will：能力改看 `stripe_transfers`，見 D20；選項 A 正文的 `charges_enabled` 作廢）
  - A：平台審核通過且 port 回報收款能力啟用（Stripe 的 `charges_enabled`；與 ADR `2026-09-18-seller-capability-authorization` 的 capability 加 requirements 同一模型）。（建議）
  - B：只看平台審核——店家可能在金流商未開通時曝光、無法收款。
- **D11 同意種類 `PAYUNI_TERMS`**（已決 2026-10-29，Will，選 A；擋住：4、7）（2026-11-02 修訂，Will：不轉換已接受的 `PAYUNI_TERMS` 紀錄（未上線，只有本機種子）；託管與內嵌流程由 Stripe 頁面收服務協議，adapter 不送 `tos_acceptance`；不新增 `PAYMENT_PROVIDER_TERMS`，只刪 `PAYUNI_TERMS`，入駐只記平台自己的條款；平台代收同意的需要在接台灣金流商時再加）
  - A：改成 provider 中立的 `PAYMENT_PROVIDER_TERMS`，記錄 provider 與版本；已接受的 `PAYUNI_TERMS` 紀錄轉成 provider = PayUni；Stripe 的服務協議接受（`tos_acceptance`）由 adapter 送出。（建議）
  - B：每個 provider 一個同意種類——新增 provider 就要改 enum 與 CHECK。
- **D12 `PROVIDER_HOSTED` 時的防詐訊號**（已決 2026-11-02，Will，選 A；步驟 1 在測試模式確認 fingerprint 跨帳號一致，live mode 的範圍上線前核對；擋住：6）（2026-11-02 修訂，Will：D22 之後兩種收件方式都只用 fingerprint；Stripe 收件時平台讀不讀得到 fingerprint 由步驟 5 實測）
  - A：平台沒有身分證號，只用金流商提供的銀行帳號 `fingerprint` 做關聯標記；身分層級的防重複開通交給金流商。（建議，前提是步驟 1 確認 fingerprint 跨帳號一致）
  - B：`PROVIDER_HOSTED` 也要求平台另收身分證號計算雜湊——兩邊重複收 PII，違反 D7 選 A 的目的。

- **D13 收款帳戶的狀態模型**（已決 2026-11-02，Will，選 A；擋住：4）：中立狀態從帳戶快照推導，不自創語意：`NOT_CREATED`（本地無外部 ID）、`ONBOARDING`（尚未送出資料）、`UNDER_REVIEW`（已送出、`stripe_transfers` 非 active、無待補條件）、`ACTION_REQUIRED`（`currently_due` 或 `past_due` 非空）、`ENABLED`（`stripe_transfers` active）、`DISABLED`（`disabled_reason` 屬拒絕或平台暫停）。另存 `transfers_active`、`payouts_active`、requirements 原始 JSON、`disabled_reason`。B（只存原始 JSON，client 自己判讀）未採。業界：Stripe capability 加 requirements（https://docs.stripe.com/connect/accounts-v2）。（2026-11-03 修訂，Will：改依 Accounts v2 重新設計，取代上面的 v1 對應。業界基線是 Stripe 的 v2 做法：開通完成看 `stripe_transfers.status == active`，其餘看 `status_details[].resolution` 決定誰要行動，`requirements.entries` 只用來顯示要補什麼（https://docs.stripe.com/connect/marketplace/tasks/onboard）。狀態 5 值，由 `stripe_transfers` 推導（`status_details` 多筆時取最嚴重）：無外部 ID → `NOT_CREATED`；`active` → `ENABLED`；任何一筆 `resolution = provide_info`（含 `pending`）→ `ACTION_REQUIRED`；`pending` → `UNDER_REVIEW`；`rejected`、`unsupported`、`restricted` 加 `contact_stripe`／`no_resolution` → `DISABLED` 並存原因碼；不認得的值與 `restricted` 但 `status_details` 為空 → `UNDER_REVIEW` 並記警示 log。`ONBOARDING` 刪除：v2 沒有「已送出」訊號，新帳戶一開始就是 `restricted`＋`requirements_past_due`＋`provide_info`、原因全是 `routine_onboarding`（`spike/outputs/fixtures/retrieve_provider_new.json`），賣家動作與補件相同；賣家端文案不得把這時的 past_due 寫成「逾期停用」。快取存推導後的狀態（理由：補漏輪詢 `payment.sql` 以狀態篩選、後台列表）、`status_reason`（provider 原因碼，只給後台）、`transfers_active`、`payouts_active`、requirements 原始 JSON；刪除 `details_submitted`、`requirements_due`、`disabled_reason`。不另存 capability 的 status／resolution（由回應現算）。`payouts` 受限不進閘門，只顯示。未採：以 requirements 計數重造 v1 的「已送出」；以 `routine_onboarding` 推 ONBOARDING；以賣家是否開過連結判斷。`rejected`、`unsupported` 優先於 `provide_info`（終態，補件解不開）；`unsupported` 的原因碼是 `unsupported_country`／`unsupported_entity_type`（https://docs.stripe.com/changelog/basil/2025-06-30/adds-unsupported-capability-status），本平台只建 US individual，出現即平台設定錯，仍記 DISABLED 並存原因碼給後台；`2026-09-30` 之前的版本以 `restricted`＋`restricted_other` 表示被拒（https://docs.stripe.com/changelog/endive/2026-09-30/rejected-accounts-v2-capabilities）。建立外部帳戶後、第一次同步前的列寫 `ACTION_REQUIRED`）
- **D14 管理員重新綁定**（已決 2026-11-02，Will，選 A；擋住：4、8）：刪除 `RebindPayUni`、`PAYUNI_BIND_RETRY`、`PayUniRemedy`；缺件由賣家經新的開通連結補。保留「重新整理狀態」；建立失敗用完重試才進審核待辦 `PAYMENT_ACCOUNT_CREATE_FAILED`。
- **D15 賣家端狀態**（已決 2026-11-02，Will，選 A；擋住：7、8）：拆成平台審核（NOT_STARTED、DRAFT、SUBMITTED、REJECTED、APPROVED）與收款帳戶（D13）兩條。
- **D16 何時建立外部帳戶**（已決 2026-11-02，Will，選 A；擋住：4）：平台審核核准後才建立並產生開通連結或 session。
- **D17 店名與身分欄位鎖**（已決 2026-11-02，Will，選 A；擋住：8）：刪除；店名是平台自己的欄位，不同步給 Stripe（Stripe 收件時平台在建立 Account Link 後不能寫入身分資料，https://docs.stripe.com/connect/hosted-onboarding）。
- **D18 舊事件**（已決 2026-11-02，Will，選 A；擋住：8）：停發 `merchant.verification.*`、刪 `sellerconsumer` 的 PayUni 審核通知；開通結果通知寫進 A8 的需求，A8 完成時恢復。
- **D19 關店再開**（已決 2026-11-02，Will，選 A；擋住：4、5）：一店一個外部帳戶，關店再開沿用；冪等鍵是建立中列寫入時產生並保存的 UUID；同一店重試沿用（v2 期限 30 天，https://docs.stripe.com/api-v2-overview#idempotency；Stripe 建議每個邏輯請求用隨機 V4 UUID，https://docs.stripe.com/api/idempotent_requests），先寫本地「建立中」的列。
- **D20 收款模式與閘門的能力**（已決 2026-11-02，Will；擋住：4；限定 B2）：市集用 destination charges（平台收款再轉給賣家），閘門看 `stripe_transfers`（D10 修訂）。Etsy、Shopify 同型是推論，未核出處；台灣上線前核對代理收付法規（前提段）。
- **D21 分支**（已決 2026-11-02，Will）：步驟 4–8 在 `payment-onboarding` 分支完成才進 `main`，不在 `main` 留新舊契約並存或 shim。
- **D22 平台自建 KYC 個資管線**（已決 2026-11-02，Will；擋住：6–8）：退場並在本波刪除。平台不再收身分證號、銀行帳號與證件；統編是市集登記資料，仍由平台收並算雜湊（2026-11-02 Will 確認）；防詐改用 fingerprint（D12）。觸發重建：決定接台灣金流商。
- **D23 收款聯絡信箱**（已決 2026-11-03，Will）：Stripe v2 有 `configuration.recipient` 的帳戶必須帶 `contact_email`（sandbox 實測，`spike/outputs/q13_contact_email.txt`，兩種 dashboard 都是 400）。賣家在送出 KYC 申請時確認（預設帶入登入信箱、可改，送出時必填），核准時複製到收款帳戶列並用於建立外部帳戶；D16「核准時建立」不變。未採：直接用登入信箱（LINE 可能沒有信箱，且賣家無法改）；延到賣家第一次開始收款才建立（要改 D16 與步驟 4 的建立 job、輪詢、後台重試）。延後：建立後修改信箱並同步到 Stripe（觸發：賣家要求改信箱，或上線前的設定頁需求）。
- **D24 步驟 7 的畫面與流程**（已決 2026-11-03，Will，全部採建議；擋住：7）：附件 `step7-screen-drafts.md` 第 5 節。Q1 送出規則的身分、銀行、證件與 `PAYUNI_TERMS` 在 7a 先拿掉，欄位與 enum 照舊在步驟 8 刪（原計畫把規則也排在 8，表單刪減後送出會一律失敗）。Q2 公司的補充文件保留（市集審核用的公司證明，不是 D22 的證件）。Q3 seller-app 回跳到 website 的靜態頁、回到前景時重讀，不加 deep link。Q4 website 做內嵌元件，新增 `@stripe/connect-js@3.4.6`、`@stripe/react-connect-js@3.4.4`（Will 2026-11-03 同意；兩者 2026-06-23 發佈，3.5.0 未滿兩週；3.4 能否開通 v2 帳戶由 7b 的 sandbox 驗收確認）。Q5 啟用後的「管理收款帳戶」入口不做（要改 proto 加 `account_update` 連結；觸發：賣家要求改資料，或上線前的設定頁需求）。Q6 admin 徽章與賣家共用目錄文字，另顯示原始 `status_reason`。
- **D25 內嵌元件的 Stripe 使用者驗證**（已決 2026-11-03，Will，選 A；擋住：7b）：平台收件的 Account Session 帶 `components[account_onboarding][features][disable_stripe_user_authentication]=true`。依據：預設會先彈出 Stripe 登入視窗，平台負責補件的帳戶可以用這個參數關掉，代價是平台承擔負餘額（https://docs.stripe.com/connect/get-started-connect-embedded-components?platform=web 「User authentication」）；本平台的帳戶本來就是 `losses_collector=application`，賣家已以 Google／LINE 登入，平台收件的目的就是賣家留在平台頁面完成（`spike/README.md` Q15）。未採 B：維持預設，賣家每次開通都要另外登入或註冊 Stripe。Stripe 收件（託管頁面）不受影響。
- **E1 Stripe API 版本**（已決 2026-11-02，Will）：Accounts v2（https://docs.stripe.com/connect/accounts-v2）；只有台灣只收轉帳的帳號需要 v1，D8 用 US 碰不到。webhook 用 thin events，收到後一律重讀帳戶（https://docs.stripe.com/webhooks）。（2026-11-03 修訂，Will：釘 `Stripe-Version: 2026-09-30.preview`，capability 有 `rejected` 狀態，D13 的 DISABLED 直接用它；這一版在一般測試模式不能建 v2 Account Link（`spike/README.md` Q11、Q12），測試與錄製改在 Stripe sandbox。未採：`2026-04-22.preview` 沿用測試模式，DISABLED 要從 status_details 推斷、升級時重做）

拍板順序：D1–D22、E1 全部已決（2026-11-02）。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 Spike 與 CI 基線 | 完成 | `baseline-ci.md`：四個失敗 run 的失敗 step 與錯誤行（`27bc439`）。`spike/README.md` Q1–Q8 各有答案、官方 URL 與遮罩後輸出；`spike/` grep `sk_|whsec_|rk_` 為 0；測試帳號全刪（`GET /v1/accounts` 0 筆）。結論：Custom（`requirement_collection=application`）在 US 與 TW（recipient、只開 transfers）可只靠 API 完成並啟用；SG 卡在 `proof_of_liveness`（無 API 參數）；TW 不能開 `card_payments`；同一測試銀行帳號在兩個 connected account 的 `fingerprint` 相同。前置：Will 在 Dashboard 確認 ongoing seller compliance（2026-11-02）。 | 文件上的重試期、冪等 key 期限、事件不保證順序未在本次重抓文件核對；webhook 重試未實測（接收端一律回 2xx）；live mode 的 fingerprint 範圍未查（D12 拍板前核對）；託管 onboarding 未以瀏覽器走完（步驟 5 的本機手動驗收做）。 | `d690766`、`d601aa0`、`27bc439` |
| 2 B 階段需求 | 完成 | `git grep -c "2026-10-29-payment-onboarding-stripe" backend/docs/plans/2026-09-24-go-modular-monolith.md` = 5（B2、B3、B8、B9、B10 各一） | 無 | 見 git log |
| 3 防詐雜湊與銀行帳號關聯 | 完成 | 本機：`atlas migrate diff` 已同步、`atlas migrate validate`、sqlc、`make connect`、TS 與 Dart 生成、`make uitext` 無 drift；gofmt、`go vet`（含 `-tags integration`）、unit（seller、httpapi、arch、cmd）、`make lint-monolith` 0 issues、`GOWORK=off go mod tidy -diff` 無差異；admin `merchant-termination` 與 `merchant-create-termination-dialog` 35 個測試通過。法務告知：`seller-kyc-notice.md` 第 6 點加防詐比對碼保存說明並 `gen:legal`（`612a5b3`）。CI：backend monolith run 41318569031（`b2dcbb0`，含 `ab3eb7d`）兩個 job 綠，integration 的 seller 套件 ok。mutation 在 CI 診斷 run 41366066358 各自執行：未改 6 個測試全過；快照改回 `kyc_identity_hashes`、5 年清理不刪、拿掉銀行帳號條件、`CheckKYCKeys` 拿掉新表，四個對應測試各自失敗。 | `TestPubContractsMatchMigrations` 本步驟沒改 `pub_*`，隨 CI 的 integration 一起跑過。決定：不回填（系統未上線）；新表 `field` 與 `kyc_identity_hashes.field` 同一組值；銀行帳號命中不能建立關聯終止（`LinkedIdentity.CountsAsGround`），防詐列在草稿欄位改掉時同步修剪。 | `ab3eb7d`、`612a5b3` |
| 4 中立收款帳戶與新契約 | 完成 | 本機（orchestrator 重跑）：`atlas migrate validate`、gofmt、`go vet`（含 `-tags integration`）、`go test -race ./...`、`make lint-monolith` 0 issues；實作者：sqlc、`make connect`、TS、Dart、`make uitext` 重跑無 drift，`buf lint`、`buf breaking` 通過，admin 與 website `tsc --noEmit`。CI（PR #284，`51b6268`）全部 job 綠，含 integration：冪等建立、sync job 唯一、`transfers_active=false` 不可曝光、inbox 重複 event id 不排 job、輪詢三種狀態、未處理事件補排、最終清理刪帳戶。獨立 review：MED 2、LOW 1 已修。 | mutation 由實作者寫進測試，未另跑反轉核對；`make dev-seed` 後可曝光由 `dev_seed_integration_test.go` 涵蓋，未手動跑；閘門缺項文案「商家認證已通過」留給步驟 7。 | `e4b249e`、`2cc53ca`、`38b3d4d`、`6c75e7e`、`de744f2`、`51b6268` |
| 5a 收款帳戶狀態改依 v2 | 完成 | 見 5b 列的本機重跑；CI（PR #284，`0be5295`）56 個 check 全綠，integration step 的 `modules/seller`、`httpapi/seller`、`cmd/hawkr` ok。 | mutation 由實作者執行，orchestrator 未反轉核對。 | `72837d1` |
| 5b Stripe adapter 與 webhook | 完成 | orchestrator 重跑（`0be5295`）：gofmt、`go vet`（含 integration tag）、`go test ./...`、`make lint-monolith` 0 issues、`atlas migrate validate`、migration inventory；code review（medium）F1–F9 全修。sandbox（hawkr-dev，2026-11-03）：fixtures 以 `2026-09-30.preview` 重錄（`0de99a88`，含真的 Account Link 成功回應）；v1 `account_sessions` 帶 `2026-09-30.endive` 回 200；`contact_email` 兩種 dashboard 都必填（Q13，催生 D23／5c）。本機手動開通（store 1009，`acct_1UXVfaID6WkY4CJw`，Stripe 收件、託管頁面）：admin `RetryPaymentAccountCreate` → worker 建帳（帶 contact_email，Stripe 頁面自動帶入）→ seller `CreatePaymentOnboardingSession` 回 HOSTED 連結 → 走完開通 → `payment_accounts`：`ENABLED transfers=true payouts=true`；`store_exposures.exposable = t`（version 3，開通前被閘門關掉、ENABLED 後打開）；外部銀行帳號（last4 2227）有 `fingerprint`（16 字元）。`stripe listen` 轉送的 thin events 驗簽後回 204。 | 開通當下本機網路位址改變，單體連不上 DB，送出後的 webhook 全部處理失敗（`stripe listen` 不重送）；重啟後由輪詢補上 ENABLED，webhook 驅動的 ENABLED 沒有在這次觀察到（建帳階段的事件有 204）。integration 的 mutation 未反轉核對；in-progress 409 錯誤碼仍是推測；平台收件（內嵌元件）的手動開通在步驟 7。 | `c9c7d53`、`f34d705`、`78a894f`、`555b5b5`、`f297597`、`0be5295`、`0de99a88` |
| 5c 收款聯絡信箱（D23） | 完成 | 本機：gofmt、`go vet`（含 integration tag）、`go test ./...`、`make lint-monolith` 0 issues、`atlas migrate validate`、`buf lint`/`breaking`、三端生成；mutation：不送 `contact_email`、送出不檢查信箱、`NormalizeEmail` 改回，各自測試失敗。code review 2 項（重新核准沿用舊信箱、帶引號信箱造成 500）已修。CI（PR #284，`a2d79742`）56 個 check 全綠，含 integration（複製信箱、重新核准換信箱、complete_check）。sandbox：建立的帳戶帶 `seller-1009@example.com`。 | 核准不複製信箱的 mutation 只靠 integration，未反轉核對。seller-app 表單欄位在步驟 7。 | `fb4e0f43`、`97e2202d`、`d28c6131`、`b3ebbe81`、`a2d79742` |
| 6 fingerprint 防詐訊號 | 完成 | orchestrator 重跑：gofmt、`go vet`（含 integration tag）、seller／stripe／httpapi／cmd 的 unit、`make lint-monolith` 0 issues、`atlas migrate validate`；unit mutation 反轉核對：adapter 不填 `BankFingerprints`、client 不帶 `starting_after`，各自測試失敗。CI（PR #284，`018a0399`）56 個 check 全綠（第一輪 integration 因 CI 的 Postgres 容器斷線失敗，重跑綠），含 `TestSharedProviderBankFingerprintIsListedButNeverTerminates` 等 5 個 fingerprint 測試與索引計畫測試。integration mutation 在 CI 診斷 run 41568080611 反轉：Sync 不寫 fingerprint、拿掉 UNION 分支、快照不把 `BANK_FINGERPRINT` 對成 `BANK_ACCOUNT`，三個各自失敗、baseline 通過。sandbox：hosted 開通的帳戶（5b 的 `acct_1UXVfaID6WkY4CJw`）v1 `external_accounts` 回 fingerprint，錄成 contract fixture（`spike/README.md` Q14）。獨立 review：MED 2（fingerprint 連接缺 `(hash, key_id)` 索引、列數測試只有一把金鑰）已修；LOW 2 駁回（Sync 要 DB 無法 unit、`sync.go` 已保證 provider 一致；列表 4xx 取消 job 與 RetrieveAccount 相同處理）。 | 設計：寫入器是只握 MAC 的 `kyc.Fingerprints`（kyc 依賴 payment，避免循環）；命中種類沿用 `BANK_ACCOUNT`（不改 proto、admin、`measure_identities`）；fingerprint 消失不刪列，只隨 5 年到期；HMAC 輪替後由下一次同步補新金鑰的列。fingerprint 命中保留 5 年，身分證號與統編命中仍隨 184 天個資清理（步驟 8 後只剩統編）。平台收的身分證號、銀行帳號、證件照計畫在步驟 8 刪。 | `0c6ba56f`、`018a0399`、本列的 commit |
| 7 client 流程與畫面 | 進行中（畫面與 D24 已同意；7a 完成，7b–7d 未開始） | 7a：orchestrator 重跑 gofmt、`go vet`（含 integration tag）、seller／uitext／httpapi unit、`make lint-monolith` 0 issues、`atlas migrate validate`、`make uitext` 無 drift、`gen:legal --check`、website `test:unit`（2133 過）與 `pnpm test`（2869 過）、admin `pnpm test`（1104 過）、seller-app `analyze`、`hawkr_ui_text` 測試。實作者的 unit mutation：`real_name` 加回 `requirements`、`PAYUNI_TERMS` 加回 `SubmitKYCAgreements`，各自失敗。獨立 diff 審查（`d9956edc...c9125b8e`）blocking 4（integration 測試沿用舊規則）已修；CI 再抓到 1 個漏改的斷言（`5394fca2`）與修正輪的範圍外誤改（`5f738e81` 還原）。CI（PR #284，`5f738e81`）57 個 check 全綠，integration 的 `modules/seller`、`httpapi/seller`、`cmd/hawkr` ok（前兩輪 `modules/seller`、`closing`、`devseed` 的 testcontainers `No such container` 屬 CI 偶發，第三輪起正常）。 | CHECK 改回原定義的 mutation 只靠 integration 的負面案例，未在 CI 反轉核對；`deadcode.sh` 本機在 `dependency_validator` 失敗（實作者稱主 checkout 同樣失敗，orchestrator 未核對），以 CI 的 seller-app hygiene 綠為準；`/code-review` 在主 checkout 執行而審到 `dev` 的 diff，改用獨立審查者。 | `0f6fc3df`、`febcffc1`、`c9125b8e`、`6eec5b64`、`5394fca2`、`5f738e81` |
| 8 刪 PayUni、舊契約、KYC 管線 | 未開始 | | | |
| 9 收尾 | 未開始 | | | |
