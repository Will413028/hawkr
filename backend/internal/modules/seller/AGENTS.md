# seller 模組

## 職責

店家業務身分與 profile 的擁有者：開店、KYC 與 PayUni、措施與停權、治理（通報、案件、申訴）、關店與清理、商家名單、商品瀏覽數、刪帳參與者。行為規格的唯一來源是 `docs/modules/seller.md`。

## 擁有的資料

schema `seller`（`db/schema.sql`，view 與函式在 `db/objects.sql`）：`stores`、`store_closures`、`store_exposures`、`store_blocks`；`measures`、`measure_*`、`violations`、`suspension_periods`、`reports`、`report_*`、`cases`、`case_inquiries`、`governance_notes`、`evidence_accesses`；`kyc_*`、`payuni_*`；`prospects`、`prospect_*`；`product_view_events`、`product_views_daily`；媒體同步 `media_ref_sync` 與退場資產 `retired_assets`。讀取模型：無（規格 §1）。

## 發布的讀取介面

形狀在 `db/pub.sql`，定義在 `db/objects.sql`；消費端只能呼叫函式，改形狀要同一次變更更新 `pub.sql` 與消費端。守門：`TestPubContractsMatchMigrations`（cmd/hawkr）。語意與各函式的守門測試見規格 §21。

- `seller.pub_exposable_stores(at timestamptz) RETURNS SETOF seller.pub_exposable_store (store_id bigint)`：`at` 這一刻可公開曝光的店家。
- `seller.pub_review_items(at timestamptz) RETURNS SETOF seller.pub_review_item (kind, item_id, 對象的鍵, store_id, store_name, start_at, due_at)`：`at` 這一刻的全部待辦（10 種），給 `reviewqueue`。
- `seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown (measure_id, content_kind, content_id)`：生效中的內容下架對象，一列一個（措施，內容），不收 `at`；給 social 每日對帳核對複本。守門：`TestPubContentTakedownsListsOnlyActiveTakedownTargets`。

## 模組內部結構

- 根套件只保留 `Module`、`New`、`Migrations`、轉呼叫方法、型別別名與事件。
- `internal/` 的每個子領域（`onboarding`、`measure`、`closing`、`kyc`、`payuni`、`prospect` 等）管什麼見規格 §1.2。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加子領域時同步修改。各子領域在自己套件宣告最小的 `Deps`，由根套件組裝。

## 不變量

守門測試在本模組；規格唯一來源是 `docs/modules/seller.md`。
- 一帳號最多一個目前 stint，SELLER 與它成對授予、移除。守門：`stores_current_account`、`TestApplyRejectsSecondStoreAndReonboardsWithNewStint`。規格：§2。
- 鎖順序：全域順序見 `backend/AGENTS.md` 單體規則 4；本模組內：帳號全部 stint、通報／案件／詢問／措施列（依 id 升序）、公告列。守門：`TestConcurrentSourceRevokeAndLinkedCreateNeverDeadlock`。規格：§18。
- 停權不存，推導點只有 `stint.Derive`；措施「生效中」的各處 SQL 定義與 `StateOf` 在邊界一致。守門：`TestEffectiveMeasuresAndStateAgreeAtTheBoundaries`。規格：§5、§17。
- 措施版本只經 `InsertMeasure`／`AdvanceMeasureRevision`；停權期間只由 `suspension.Recompute` 寫，先鎖再讀。守門：`TestRecomputeReadsMeasuresAfterLockingTheStores`。規格：§5。
- 動作可用性各只有一處來源（`Can*`、`*UnavailableReasons`、`BlockFor`），API 的 `can_*` 共用。守門：`TestCanFlagsMatchWhetherTheActionSucceeds`。規格：§5–§8。
- 能力只由 `model.CapabilitiesOf` 推導；停權不擋關店與刪帳。守門：`TestActingStoreFailsClosed`。規格：§10。
- 關店只走 `closing.close()`，紀錄只在 `store_closures`，自己關店先查未結訂單。守門：`TestEveryClosePathRecordsWhoClosedTheStore`。規格：§11。
- 兩段清理只在期滿且無保存條件時做，候選鎖住後重判。守門：`TestPurgeRechecksTheAppealHoldAfterLockingTheCandidates`。規格：§11。
- 可曝光公告只經 `exposure.Announce`，先鎖再算。守門：`TestAnnouncementLocksBeforeItComputes`。規格：§12。
- media 引用只由 `seller.media_sync` 寫，transaction 外只 `Finalize`，退場資產不能寫回。守門：`TestImagesLinkByStoreIDAndUseVersion`。規格：§9。
- 寫入以版本樂觀鎖，拒絕都在 token 轉正之前。守門：`TestRejectedProfileUpdateDoesNotConsumeTheStagedToken`。規格：§3。
- 通報只指向公開過的店，重複送出併成補充；佐證與證件先寫讀取紀錄才讀。守門：`TestEvidenceReadIsRecordedBeforeTheBytesAreRead`。規格：§7、§9。
- KYC 個資只存密文，型別不帶明文，顯示全文先稽核。守門：`TestKYCNeverReturnsOrStoresPlaintext`。規格：§13。
- PayUni 外部呼叫不在 transaction 內，一店一個 sync job。守門：`TestSyncJobIsUniquePerStoreInRiver`。規格：§14。
- 商家名單統編在未封存業者間唯一。守門：`TestTaxIDIsUniqueAmongLiveProspects`。規格：§15。
- 瀏覽數以 `store_id` 與台北日計，請求路徑不呼叫商品服務。守門：`TestRollupBucketsByStoreAndTaipeiDayOfOccurrence`。規格：§16。
- 事件與快取失效在寫入的 transaction 內；儲存代碼 UPPER_SNAKE 加 CHECK 等於 `Values()`。守門：`TestEventFailuresRollBackTheWholeCommand`、`TestChecksMatchGo`。規格：§18、§20。

## 不負責

- Connect 方法、能力宣告與限流、粉絲數與評價補資料：`internal/httpapi/seller`。
- 刪帳編排、買家側未結訂單：`accountdeletion`；店家側查詢是本模組的 `Orders` port。
- 通知送達：notification；vendor portal、推薦排行：`agent`、`discovery`。

## 決策紀錄

從 seller-service 沿用的每個機制、當時的決定與理由，以及與 seller-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/` 的 `2026-10-18-seller-governance-consolidated.md`、`2026-10-19-seller-kyc-payuni-consolidated.md`、`2026-10-03-public-store-identity-and-visibility.md`、`2026-10-04-prospect-contact-points-and-matching.md`。
