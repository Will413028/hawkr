# 賣家收款開通改走 payment port，作品集接 Stripe Connect

**Goal：** 單體的賣家收款開通（子帳戶建立、KYC 轉送、狀態同步、開通閘門、同意書）只經 provider 中立的 payment port；作品集的 adapter 是 Stripe Connect 測試模式，支援金流商代收（`PROVIDER_HOSTED`）與平台自收（`PLATFORM_COLLECTED`）；防詐身分雜湊與 KYC 個資分開保存；共用銀行帳號列入關聯帳號。單體與 `api/hawkr` 不再有 PayUni（允許清單見步驟 8）；付款、退款、撥款與物流的換供應商寫在單體計畫 B2、B3、B8–B10。驗收見步驟 9 與「完成定義」。

**上層：** 主清單見本機個人脈絡檔；單體遷移的全域約束見 `backend/docs/plans/2026-09-24-go-modular-monolith.md`。

**附件：** `2026-10-29-payment-onboarding-stripe/`：`payuni-inventory.md`（PayUni 盤點，快照 commit `e865c68`，G1–G11，「去向」欄由步驟 8 填）、`psp-compare.md`（Stripe Connect 與台灣金流商比較，官方出處逐項標示）；步驟 1 加入 `baseline-ci.md` 與 `spike/`，步驟 8 加入 `check_inventory.py`。

## 完成定義

- 單體與 `api/hawkr` 的 `git grep -n -i payuni` 只剩步驟 8 允許清單裡的列。
- 開通只經 port；開通閘門依 D10；Stripe 兩種開通模式在測試模式各完成一次，記錄在進度表。
- 防詐身分雜湊與 KYC 個資分開保存（D3），銀行帳號列入關聯帳號（D4），有 integration 測試與 mutation。
- `payuni-inventory.md` 每列有「去向」（`check_inventory.py` 通過）。
- 改到的契約、表、文案與條款的所有消費端在同一個步驟內跟上；三端生成物無 drift。
- CI：與 `baseline-ci.md` 比對，新增失敗的 job 或 step 為 0。
- D1–D12 的結論寫進 ADR；`docs/modules/seller.md` 與 ADR 2026-10-19、2026-10-18 依修訂流程更新。

## 前提（開工時核對過，2026-10-29）

- 會改到或取代的既有部署是否在線：不在線（系統從未上線，根目錄 `AGENTS.md`；舊服務不部署，A3 計畫前提）。不受回滾窗口、相容性與資料搬移的約束；回滾就是 revert。已套用的 migration 仍不得修改（`backend/AGENTS.md` 單體規則 6），表與 view 的改動一律寫新 migration。
- 目標是作品集為主、保留在台灣上線的可能（Will 2026-10-29）。
- 不簽 PayUni 代理商合約，沒有 PayUni 測試帳號（Will 2026-10-29）。單體的 PayUni 程式無法對真實 sandbox 驗證。
- Stripe 不接受台灣的平台帳號；Connect 跨境撥款只開放平台在 US、UK、EEA、CA、CH；Stripe 沒有台灣的超商代碼、ATM、LINE Pay、街口（`psp-compare.md` §1，官方文件，2026-10-29；已確認）。Stripe adapter 只示範機制，不示範台灣上線情境。
- Stripe Custom 帳號（`controller.requirement_collection = application`）在測試模式可以 API 送 KYC，可實作 `PLATFORM_COLLECTED`（推論；未確認，步驟 1 查證）。
- 平台自己收款再轉給賣家可能落入《電子支付機構管理條例》的代理收付（第 3、5 條；第三方轉載，未在全國法規資料庫核對；未確認）。只影響真實上線，不擋本計畫。
- D3 的 5 年保存期是 Will 的產品決定（參考金流業「關係結束後 5 年」的紀錄保存慣例），法源未確認；上線前與條款一起核對。
- PayUni 在單體只有收款開通（盤點 G1–G3、G10），加上 `internal/legacy` 對 Java 舊服務的接縫；買家付款、退款、撥款、webhook、對帳與物流在 Java 舊服務（G4–G9），由 B2、B3、B8–B10 搬（`payuni-inventory.md`；已確認）。
- 過渡期：步驟 6 之後新開通的店家只有 Stripe 帳號；`internal/legacy/sellerserver` 回給 Java 的 `payuni_mer_id` 變成空值，Java 以 PayUni 下單的本機端到端不通，B2 完成後恢復（Will 2026-10-29 接受）。

**驗收基線：** commit `1c0b246` 的 CI：失敗的是 seller-app CI（unit and widget tests）、website e2e（Storybook 的 `product-certification.css` import 與 Playwright 1、2、3、5）、backend monolith（golangci-lint 在未改的檔）、backend migration inventory（16 列缺 disposition）；其餘通過。步驟 1 把每個失敗 job 的失敗 step 與錯誤行存成 `baseline-ci.md`（`gh run view <id> --log-failed`）。這四項不在本階段修；golangci-lint 與 migration inventory 是本階段會改到的 job，驗收以「錯誤行與基線逐行比對，新增為 0」判定，不看 job 的整體結論。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 店家能否曝光、收單由開通閘門推導 | view `seller.store_requirement_flags.kyc_approved`（KYC 核准且 PayUni `outcome = 'APPROVED'`）、`model.CapabilitiesOf`、`seller.pub_exposable_*`、discovery 與 `store_card.proto` 的「營業中」、`cmd/hawkr/dev_seed.go` 的種子店家 |
| 外部呼叫不在 transaction 內，一店一個 sync job，先追加觀測再套用 | `seller/internal/payuni` 的 River job（唯一鍵）、`payuni_registrations.intent_seq/synced_seq`、`payuni_observations`、`TestSyncJobIsUniquePerStoreInRiver` |
| 外部事件先寫 inbox 再處理 | `backend/AGENTS.md` 單體規則 5；現行 PayUni webhook 的入口 |
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
| 對外契約 | `admin/seller/v1alpha1/seller.proto` 的 4 支 PayUni RPC、`review_queue.proto` 1 個 kind、同意種類 `PAYUNI_TERMS`、`store_card.proto`、`discovery.proto` 的說明 | `order.proto`、舊 `seller.proto` | `git grep -n -i payuni -- api/hawkr` |

## 範圍

- 做：payment port 的開通部分；Stripe Connect adapter（測試模式）的兩種開通；開通閘門改看 port 的能力狀態；同意種類改成 provider 中立；防詐身分雜湊的獨立保存；銀行帳號列入關聯帳號；刪除單體的 PayUni 與 `internal/legacy` 的 PayUni 接縫；B 階段需求。
- 不做：Java 與 Go 舊服務的 PayUni（B 階段搬時刪，避免改兩次）；台灣金流商 adapter（沒有合約與測試帳號）；代理收付的法律意見。
- 延後：
  - 台灣金流商 adapter（觸發：決定在台灣上線；先確認法規與合約）。
  - 撥款模型與手續費（代理商費反推、ADR `payuni-channel-fee-bearer`）重審（觸發：B2、B10 開工）。
  - 裝置與聯絡方式重疊、風險評分等防人頭措施（觸發：上線前）。
  - 本機付款端到端恢復（觸發：B2 完成）。
  - D3 保存期與告知的法務核對（觸發：上線前）。

## 步驟

- [ ] **1. Spike：Stripe 測試模式的事實，與 CI 基線**（被擋於：需要人做的事）
  - 範圍：orchestrator 先把基線 CI 的失敗存成附件 `baseline-ci.md`。之後在 Stripe 測試帳號（暫定國別 SG，見 D8，可推翻）上以附件 `spike/` 的腳本確認並記錄：connected account 可用國別（台灣可否）；Custom（`requirement_collection = application`）能否在測試模式以 API 送 KYC 並啟用；託管與內嵌 onboarding 的帳號連結與回跳；`requirements`（`currently_due`、`errors`）與 `charges_enabled`、`payouts_enabled` 的對應；`account.updated` webhook 的簽章驗證、event id、重送與亂序；外部銀行帳號 `fingerprint` 是否跨 connected account 一致；建立帳號的冪等鍵；證件以 Files API 上傳的用途值。每項附官方文件 URL 與實測輸出；輸出存檔前以 `spike/scrub.py` 遮掉金鑰、`acct_` 與 `evt_` 以外的識別碼與個資。
  - 消費端：無（spike）。
  - 驗收：`spike/README.md` 每個問題有答案、官方出處與遮罩後的實測輸出；`baseline-ci.md` 列出四個失敗 run 的失敗 step 與錯誤行。
  - 停止條件：SG 帳號無法建立 Custom 或 connected account 時，換 D8 的其他國別重試一次，仍不行就停下回報給 D7、D8。
  - 需要人做的事：Will 建立 Stripe 測試帳號（國別 SG），把測試金鑰與 webhook signing secret 存在本機 `secrets/` 並回覆 done；不貼出金鑰。

- [ ] **2. B 階段需求**（被擋於：無；可與步驟 1 並行）
  - 範圍：在單體計畫加需求並指向本計畫：B2（付款經 payment port、Stripe；`internal/legacy/sellerserver` 的 `payuni_mer_id` 由 port 的外部帳戶 ID 取代）、B3（物流改綠界，PayUni 物流退場）、B8–B10（退款、撥款、對帳經 port；撥款模型與手續費重審）。
  - 驗收：`git grep -n "2026-10-29-payment-onboarding-stripe" backend/docs/plans/2026-09-24-go-modular-monolith.md` 在 B2、B3、B8、B9、B10 各至少一筆。

- [ ] **3. 防詐身分雜湊的獨立保存與銀行帳號關聯**（被擋於：D3、D4 已決；與 provider 無關；可與步驟 1 並行）
  - 範圍：新增 `seller.account_identity_hashes`（帳號、種類、HMAC、金鑰 ID、來源 stint、建立時間；不存明文），寫入時機與 `kyc_identity_hashes` 相同（同一個 transaction）；`kyc_identity_hashes` 仍是申請的雜湊、照 184 天清，新表不清；新表的列在來源 stint 關閉滿 5 年後由清理 job 刪除；`SnapshotMeasureIdentities` 改從新表取（涵蓋已清理的舊 stint）。`LinkedAccountHits` 納入銀行帳號（`kyc.sql` 的 `mi.kind IN ('ID_CARD','TAX_ID')` 與 `h.field <> 'BANK_ACCOUNT'`），命中種類標成銀行帳號，只列出、不自動終止。本步驟只處理平台自收的身分（現行 KYC）；`PROVIDER_HOSTED` 的訊號依 D12 在步驟 6 處理。依 ADR 修訂流程修訂 `2026-10-19-seller-kyc-payuni-consolidated`（「關店與個資保留」「身分比對與比對 hash」）與 `2026-10-18-seller-governance-consolidated`（關聯帳號）；`docs/modules/seller.md` §13、§23 同步；`website/docs/legal/` 加防詐保存目的的告知並重新生成同意文件。
  - 消費端：`git grep -n -E "SnapshotMeasureIdentities|LinkedAccountHits|kyc_identity_hashes|measure_identities|SharesBankAccount|CheckKYCKeys" -- backend api admin website seller-app`；admin 措施詳情的關聯帳號畫面；`make sqlc` 的生成物。
  - 不能動：KYC 明文只存密文；`CheckKYCKeys`（新表的金鑰 ID 納入啟動檢查）；處分快照保留到措施撤銷；已套用的 migration。
  - 驗收：單體 integration：舊 stint 已過 184 天清理、之後才建立的停權措施，快照含舊 stint 的身分（mutation：快照改回從 `kyc_identity_hashes` 取，測試要失敗）；來源 stint 關閉滿 5 年的列被刪（mutation：清理不刪，測試要失敗）；銀行帳號命中出現在關聯帳號且不自動建立終止（mutation：拿掉銀行帳號條件，測試要失敗）；`CheckKYCKeys` 的測試含新表；`atlas migrate validate`；`TestPubContractsMatchMigrations`；`make lint-monolith` 與基線比對新增為 0；admin 改到的測試。

- [ ] **4. payment port、provider 中立的資料與契約，以及所有消費端的改名**（被擋於：步驟 1，D6、D7、D9、D10、D11；**4、5、7 必須連著做**）
  - 範圍：
    - port：在 seller 模組定義（建立子帳戶、送 KYC 或取得開通連結、讀狀態與缺的條件、解析 webhook、能力描述含 `onboarding_mode`），以 in-memory fake adapter 做測試。
    - 資料：新 migration 把 `payuni_registrations`、`payuni_observations` 改成 provider 中立（provider、外部帳戶 ID、狀態、缺的條件、觀測），重建 view `seller.store_requirement_flags`（閘門依 D10）與依賴它的 view；`make sqlc`、`atlas.sum`。
    - 行為：sync job、`intent_seq/synced_seq` 與「先追加觀測再套用」改成走 port。
    - 契約：admin 的 `GetPayUniRegistration`、`RebindPayUni`、`ListPayUniObservations`、`RefreshPayUniStatus` 改名為 provider 中立；`REVIEW_QUEUE_ITEM_KIND_PAYUNI_BIND_RETRY` 改名；`PayUniRemedy` 泛化；`PAYUNI_TERMS` 依 D11（新 migration 改 `agreement_acceptances_kind_check` 並轉換已接受的紀錄）；`store_card.proto`、`discovery.proto` 的說明改寫；三端重新生成。
    - 其他消費端同一步跟上：`adminperm.go` 的 `seller.payuni_*` 與 `adminauth.go` 宣告表；`internal/arch/modules.go`；`cmd/hawkr` 的 `readPayUni` 與 `PAYUNI_*` 設定、`docker-compose-local.yml`；`internal/platform/codemeta/code_meta.go` 的 PayUni 審核狀態（CodeMeta 凍結，只刪單體用不到的；照本機脈絡檔的 CodeMeta 規則跑 `generate.sh`）；`internal/legacy/sellerrelay` 的 `merchant.verification.*` 事件、`sellerconsumer`（Java 的審核通知）、`sellerserver` 的 `payuni_mer_id`（改回 port 的外部帳戶 ID，PayUni 帳戶不存在時回空值）；client 的機械改名（RPC、enum、權限字串、文案 key），讓四個 client 編得過、測試通過；共用文案目錄 `make uitext`。
    - 本機種子：`cmd/hawkr/dev_seed.go` 以 fake provider 寫入「收款能力啟用」，讓種子店家仍能曝光；`dev_seed_integration_test.go` 跟上。
  - 消費端：`git grep -n -i "payuni" -- backend/internal backend/cmd api/hawkr admin/src website seller-app/lib buyer-app/lib packages docker-compose-local.yml ':!**/gen/**'`；`git grep -n -E "seller\.payuni_|PAYUNI_TERMS|PAYUNI_BIND_RETRY|PayUniRemedy" -- .`。
  - 不能動：同一店家同時只有一個外部呼叫 job；外部呼叫不在 transaction 內；KYC 個資只存密文；已套用的 migration。
  - 驗收：單體 unit 與 integration（fake adapter）；mutation：閘門拿掉 provider 能力條件，曝光測試要失敗；`buf lint`（`buf breaking` 預期失敗，未上線，commit 訊息列出 breaking 項）；三端生成物 drift；uitext 與法務生成物 drift；啟動檢查（`go run ./cmd/hawkr serve` 在本機以 fake provider 起得來）；`make dev-seed` 後種子店家可曝光；四個 client 改到的測試與 CI 等價檢查；`make lint-monolith` 與 golangci-lint、migration inventory 和基線比對新增為 0。
  - 完成後的狀態：只有 fake adapter；與步驟 5、7 連著做。

- [ ] **5. Stripe adapter：金流商代收（`PROVIDER_HOSTED`）**（被擋於：步驟 4，D7、D8）
  - 範圍：`internal/platform/stripe`（只做開通需要的 API）；建立 connected account（冪等鍵取自 store_id 與 stint）、帳號連結；`account.updated` webhook：單體 httpapi 的入口驗簽後先寫 inbox（單體規則 5），以 event id 去重，處理時以 API 重新讀取帳號的目前狀態（不信任 payload 的順序）；入口不經身分驗證（簽章就是驗證），在 `internal/httpapi` 的豁免清單登記；website 同源反代與本機以 `stripe listen` 轉送；狀態輪詢補漏；`requirements` 對應 port 的缺的條件；設定由環境變數提供，缺金鑰時開通 API 回 `failed_precondition`，serve 照常啟動。
  - 消費端：步驟 4 的 port；`git grep -n -i "webhook" -- backend/internal/httpapi website/next.config.* website/src/middleware*`。
  - 不能動：步驟 4 的不變量。
  - 驗收：adapter 的 contract 測試以步驟 1 錄下、遮罩過的回應重放（CI 不打網路）；webhook mutation：簽章錯誤要拒絕、同一 event id 第二次不再處理；本機手動開通的通過判定：測試帳號完成 onboarding 後，port 狀態為啟用、`charges_enabled = true`、該店家在 discovery 可曝光，三項的輸出貼進進度表（orchestrator 在本機跑）。
  - 需要人做的事：無（用步驟 1 的金鑰）。

- [ ] **6. Stripe adapter：平台自收（`PLATFORM_COLLECTED`）與 `PROVIDER_HOSTED` 的防詐訊號**（被擋於：步驟 5，D7、D9、D12；步驟 1 確認 Custom 可用）
  - 範圍：依 D9 的欄位清單把平台收的 KYC 對應到 Stripe 的 `individual`、`company`、`external_account`、`tos_acceptance`；證件以 Files API 上傳；平台審核核准後才送（ADR 2026-10-19「核准與綁定」的時點）。條款（`website/docs/legal/` 的 KYC 告知）加「證件與銀行帳號提供給金流商」。依 D12 為 `PROVIDER_HOSTED` 的店家寫入銀行帳號 fingerprint 到步驟 3 的表（種類另列），關聯帳號比對涵蓋它。
  - 驗收：contract 測試（重放）；mutation：缺必填欄位時 port 回報缺的條件且不送出；送出的欄位集合等於 D9 的清單（測試比對 request 欄位，多一個就失敗）；fingerprint 命中出現在關聯帳號（mutation：不寫 fingerprint，測試要失敗）；本機以 Custom 測試帳號完成一次開通，判定同步驟 5。
  - 停止條件：步驟 1 查出測試模式不支援 Custom 時，本步驟改依 D7 的結論處理。

- [ ] **7. client 的流程與畫面**（被擋於：步驟 4、5；步驟 6 的平台自收畫面待步驟 6）
  - 範圍：website 與 seller-app 的開通流程（`PROVIDER_HOSTED` 導到 Stripe 連結並處理回跳與重新取得連結；`PLATFORM_COLLECTED` 沿用現行表單）、收款狀態頁、admin 的帳戶與觀測畫面；provider 中立的文案；條款與同意書。版面與流程的改動先給 Will 看（根目錄 UI 改動規則）。
  - 驗收：各 client 改到的測試與 CI 等價檢查（Flutter 兩段驗證、website `test:unit`、admin `pnpm test`）；e2e 與 Storybook 只在 CI 跑，與基線比對。
  - 需要人做的事：Will 同意開通流程的畫面改動。

- [ ] **8. 刪除單體的 PayUni**（被擋於：步驟 4–7，D6）
  - 範圍（D6 選 A）：刪除 `internal/platform/payuni`、`seller/internal/payuni`、PayUni 的 config 與單體文件；`backend/docs/第三方整合/` 的 PayUni 文件檔頭改標「Java 舊服務的現行規格，B 階段刪除」；建立附件 `check_inventory.py`（盤點每列的「去向」不得為空或 TBD），並填完 `payuni-inventory.md` 的去向。D6 選 B 時：本步驟改成把 PayUni adapter 接到 port 後面（只有 fake 測試），允許清單加上 adapter 的目錄。
  - 允許清單：`internal/legacy/**` 中 Java 契約生成的欄位名（例：`payuni_mer_id` 的 proto 欄位，B2 刪）、`backend/docs/plans/**`、指向 B 階段的文件；其餘單體與 `api/hawkr` 0 筆。
  - 驗收：`git grep -n -i payuni -- backend/internal backend/cmd api/hawkr` 的結果逐筆屬於允許清單；`python3 <附件>/check_inventory.py` 通過，mutation：把一列的去向改回 TBD，腳本要失敗；單體 build、unit、integration；`make sqlc`、`atlas migrate validate`；與基線比對新增為 0。

- [ ] **9. 收尾**：完成定義逐項核對；獨立設計審查（design-review skill）；`check_inventory.py` 留在附件（一次性）；延後項附觸發條件搬回主清單；ADR（D1–D12）寫進 `backend/docs/adr/`；需要人做的事列給 Will。驗收：完成定義每項附證據；CI 與 `baseline-ci.md` 比對新增為 0。

## 決定

- **D1 作品集的金流 adapter**（已決 2026-10-29，Will；擋住：全部）：Stripe Connect 測試模式。不簽 PayUni（測試帳號要簽約）；作品集要展示國際標準。
- **D2 開通模式**（已決 2026-10-29，Will；擋住：4–7）：port 支援 `PROVIDER_HOSTED` 與 `PLATFORM_COLLECTED`。
- **D3 防詐身分雜湊的保存**（已決 2026-10-29，Will；擋住：3）：與 KYC 個資分開保存 5 年（來源 stint 關閉起算）；明文仍 184 天清。法源未確認，見前提。
- **D4 共用銀行帳號**（已決 2026-10-29，Will；擋住：3）：列入關聯帳號，只標記、不自動終止。
- **D5 本計畫的範圍**（已決 2026-10-29，Will）：只做單體的收款開通；付款、退款、撥款、物流寫成 B2、B3、B8–B10 的需求。
- **D6 單體的 PayUni adapter**（未決；擋住：4、8；需要的事實：步驟 1）
  - A：刪除（git 可找回）。必要條件不成立：沒有測試帳號，無法驗證；`PLATFORM_COLLECTED` 由 Stripe Custom 示範。（建議，前提是步驟 1 確認 Custom 可用）
  - B：保留在 port 後面當第二個 adapter，只有 fake 測試——保留台灣上線的起點，但無法對真實服務驗證，會隨 port 演進而失真。步驟 8 改走 B 分支。
- **D7 Stripe 的預設開通模式**（未決；擋住：4–6；需要的事實：步驟 1）
  - A：預設 `PROVIDER_HOSTED`（Stripe 託管或內嵌 onboarding，Stripe 收證件與銀行帳號），`PLATFORM_COLLECTED` 用 Custom 示範。平台持有的 PII 最少。（建議）
  - B：預設 `PLATFORM_COLLECTED`——沿用現行 KYC 畫面最多，但平台持有最多 PII。
- **D8 Stripe 測試平台帳號的國別**（未決；步驟 1 以 A 暫定開工，查證後確認；擋住：5）
  - A：SG（亞洲、英文介面、可示範本地幣；平台與 connected account 同國）。（建議，暫定）
  - B：JP——同為亞洲，但 KYC 欄位與文件以日文為主。
  - C：US 平台加跨境撥款——示範跨境，但離台灣情境更遠，且台灣能否當 connected account 未確認。
- **D9 平台審核與金流商驗證的分工**（未決；擋住：4、6）
  - A：平台審核只管市集政策（店家資訊、經營項目、登記資料比對），身分與銀行帳號由金流商驗證。`PLATFORM_COLLECTED` 送給金流商的欄位清單：本人姓名、身分證號、出生日期、住址、電話、email；公司型另加公司名稱、統編、登記地址、負責人；銀行代碼與帳號；證件正反面；服務協議接受（時間、IP）。平台仍以身分證號、統編、銀行帳號計算防詐雜湊。（建議）
  - B：平台審核繼續核對身分證件（現行）——兩方各驗一次；`PROVIDER_HOSTED` 時平台沒有證件可審。
- **D10 開通閘門**（未決；擋住：4）
  - A：平台審核通過且 port 回報收款能力啟用（Stripe 的 `charges_enabled`；與 ADR `2026-09-18-seller-capability-authorization` 的 capability 加 requirements 同一模型）。（建議）
  - B：只看平台審核——店家可能在金流商未開通時曝光、無法收款。
- **D11 同意種類 `PAYUNI_TERMS`**（未決；擋住：4、7）
  - A：改成 provider 中立的 `PAYMENT_PROVIDER_TERMS`，記錄 provider 與版本；已接受的 `PAYUNI_TERMS` 紀錄轉成 provider = PayUni；Stripe 的服務協議接受（`tos_acceptance`）由 adapter 送出。（建議）
  - B：每個 provider 一個同意種類——新增 provider 就要改 enum 與 CHECK。
- **D12 `PROVIDER_HOSTED` 時的防詐訊號**（未決；擋住：6；需要的事實：步驟 1）
  - A：平台沒有身分證號，只用金流商提供的銀行帳號 `fingerprint` 做關聯標記；身分層級的防重複開通交給金流商。（建議，前提是步驟 1 確認 fingerprint 跨帳號一致）
  - B：`PROVIDER_HOSTED` 也要求平台另收身分證號計算雜湊——兩邊重複收 PII，違反 D7 選 A 的目的。

拍板順序：D1–D5 已決；D9、D10、D11 現在可以拍；D8 以 A 暫定讓步驟 1 開工；D6、D7、D12 與 D8 的確認在步驟 1 之後交。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 Spike 與 CI 基線 | 未開始 | | | |
| 2 B 階段需求 | 未開始 | | | |
| 3 防詐雜湊與銀行帳號關聯 | 未開始 | | | |
| 4 payment port、中立契約與改名 | 未開始 | | | |
| 5 Stripe 金流商代收 | 未開始 | | | |
| 6 Stripe 平台自收與防詐訊號 | 未開始 | | | |
| 7 client 流程與畫面 | 未開始 | | | |
| 8 刪除單體 PayUni | 未開始 | | | |
| 9 收尾 | 未開始 | | | |
