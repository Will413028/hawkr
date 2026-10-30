# seller 模組

## 職責

店家業務身分與 profile 的擁有者：開店、KYC 與 PayUni、措施與停權、治理（通報、案件、申訴）、關店與清理、商家名單、商品瀏覽數、刪帳參與者。行為規格的唯一來源是 `docs/modules/seller.md`。

## 擁有的資料

schema `seller`（表與約束在 `db/schema.sql`，view 與函式在 `db/objects.sql`），依子領域：店家（`stores`、`store_closures`、`store_exposures`、`store_blocks`）；措施與停權（`measures`、`measure_*`、`violations`、`suspension_periods`）；治理（`reports`、`report_*`、`cases`、`case_inquiries`、`governance_notes`、`evidence_accesses`）；KYC 與 PayUni（`kyc_*`、`account_identity_hashes`、`payuni_*`）；`prospects`、`prospect_*`；瀏覽（`product_view_events`、`product_views_daily`）；媒體同步 `media_ref_sync`、`retired_assets`。讀取模型：無（規格 §1）。

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
- 動作可用性各只有一處來源（`Can*`、`*UnavailableReasons`、`BlockFor`），API 的 `can_*` 共用。守門：`TestCanFlagsMatchWhetherTheActionSucceeds`。規格：§5–§8。
- 能力只由 `model.CapabilitiesOf` 推導；停權不擋關店與刪帳。守門：`TestActingStoreFailsClosed`。規格：§10。
- 事件與快取失效在寫入的 transaction 內；儲存代碼 UPPER_SNAKE 加 CHECK 等於 `Values()`。守門：`TestEventFailuresRollBackTheWholeCommand`、`TestChecksMatchGo`。規格：§18、§20。
- 子領域的不變量在 `internal/<子領域>/AGENTS.md`（碰該目錄時才載入）：closing、exposure、governance、kyc、measure、mediaref、payuni、profile、prospect、purge、stint、suspension、views。單一子領域的規則放那裡，跨子領域的才放這裡。

## 不負責

- Connect 方法、能力宣告與限流、粉絲數與評價補資料：`internal/httpapi/seller`。
- 刪帳編排、買家側未結訂單：`accountdeletion`；店家側查詢是本模組的 `Orders` port。
- 通知送達：notification；vendor portal、推薦排行：`agent`、`discovery`。

## 決策紀錄

從 seller-service 沿用的每個機制、當時的決定與理由，以及與 seller-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/` 的 `2026-10-18-seller-governance-consolidated.md`、`2026-10-19-seller-kyc-payuni-consolidated.md`、`2026-10-03-public-store-identity-and-visibility.md`、`2026-10-04-prospect-contact-points-and-matching.md`。
