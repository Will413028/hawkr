-- invoice-service schema（Atlas declarative；唯一手改處）
-- SSOT：docs/詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md §5.1（表定義）＋ §5.2.1（排程候選索引）＋ §5.3.3（序號 SEQUENCE）
-- 本檔逐字照抄設計書；新增 jsonb 欄位時必須同步在 db/sqlc.yaml 加 override（§5.4.2，守護者 I-44）
--
-- ⚠️ **partial index 的述詞必須寫成 PostgreSQL 正規化後的形式**（`((col)::text = ANY
--    ((ARRAY[...])::text[]))`），不能寫成看起來比較好讀的 `col IN (...)`。
--    2026-08-30 實測：寫成 `IN (...)` 會讓 `atlas migrate diff` **永不收斂**——每次
--    執行都產出同一批 13 條假的 DROP+CREATE INDEX，且第二次、第三次跑照樣產出。
--    後果是「任何未來的 schema 變更都夾帶 13 行噪音」，而 `atlas migrate validate`
--    是綠的、CI 也不會發現。取得正規化文字的方法：跑一次 diff，拿它產出的 CREATE
--    語句逐字貼回本檔。

-- ---------------------------------------------------------------------------
-- §5.1.1 invoice_issuers｜開票主體
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_issuers" (
  "id"              bigint NOT NULL,                        -- TSID
  "issuer_type"     varchar(16) NOT NULL,                   -- CodeMeta InvoiceIssuerType：SELLER / PLATFORM
  "issuer_ref_id"   bigint NOT NULL,                        -- SELLER ⇒ sellers.store_id（非 account_id）；PLATFORM ⇒ 0
  "display_name"    varchar(120) NOT NULL,                  -- 開票時的銷售人名稱（法定名稱），非店名
  "status"          varchar(16) NOT NULL DEFAULT 'active',  -- active / retired
  "retired_at"      timestamptz,
  "version"         bigint NOT NULL DEFAULT 0,              -- 樂觀鎖：每條寫入路徑逐句 version = version + 1
  "created_at"      timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"      timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_issuers_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invoice_issuers_type"   CHECK ("issuer_type" IN ('SELLER', 'PLATFORM')),
  CONSTRAINT "chk_invoice_issuers_status" CHECK ("status" IN ('active', 'retired')),
  CONSTRAINT "chk_invoice_issuers_retired" CHECK ("status" <> 'retired' OR "retired_at" IS NOT NULL),
  CONSTRAINT "chk_invoice_issuers_ref" CHECK (("issuer_type" = 'PLATFORM' AND "issuer_ref_id" = 0) OR ("issuer_type" = 'SELLER' AND "issuer_ref_id" > 0)),
  CONSTRAINT "chk_invoice_issuers_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invoice_issuers_ref" ON "invoice_issuers" ("issuer_type", "issuer_ref_id");  -- 一個 store_id 只會有一列 issuer

-- ---------------------------------------------------------------------------
-- §5.1.2 seller_tax_profiles｜賣家稅務資格與治理
-- ---------------------------------------------------------------------------
CREATE TABLE "seller_tax_profiles" (
  "id"                        bigint NOT NULL,                       -- TSID
  "issuer_id"                 bigint NOT NULL,
  "review_status"             varchar(40) NOT NULL DEFAULT 'DRAFT',  -- CodeMeta SellerTaxReviewStatus，10 值見 chk_stp_review_status
  "claimed_obligation"        varchar(16) NOT NULL DEFAULT 'UNKNOWN',-- CodeMeta SellerTaxObligation：REQUIRED / EXEMPT / UNKNOWN（賣家自述，非判定依據）
  "entity_tax_type"           varchar(32),                           -- CodeMeta SellerTaxEntityType，Phase 1 五值（U-1）
  "legal_name"                varchar(120),                          -- 發票／收據上的銷售人名稱
  "tax_id"                    varchar(8),                            -- 統一編號；EXEMPT 賣家為 NULL
  "responsible_person"        varchar(60),
  "business_address"          varchar(255),                           -- 三段串接後的完整地址（AMEGO 送件與畫面單行顯示用）
  -- 🔴 city／district 必須**分開存**（#3031）：wire 上是三個欄位（58A §S02 第 3 列逐字
  --    「三個欄位，前端串接；DTO 沒有單一的地址欄」），而寫入端 joinAddress 折成一段之後
  --    **拆不回去** ⇒ 賣家回到 S02 重新編輯時 city／district 兩格是空的，ADM 審核頁也
  --    看不到行政區。business_address 保留：它是既有讀者（profileComplete 守衛）的來源。
  "address_city"              varchar(32),
  "address_district"          varchar(32),
  "default_tax_category"      smallint NOT NULL DEFAULT 1,           -- 1 應稅 / 2 零稅率 / 3 免稅，對齊 AMEGO ProductItem.TaxType。Phase 1 唯一稅別來源
  "evidence_object_keys"      jsonb NOT NULL DEFAULT '[]',           -- media objectKey 陣列
  "attestation_version"       varchar(32),                           -- 自行開票責任條款版本
  "attested_at"               timestamptz,
  -- 🔴 條款是**自然人的意思表示**（§6.1.7）：只有版本與時間、沒有人，爭議時舉不出
  --    簽署主體。actor 原本只記在 invoice_status_history，而讀取路徑（端點 21）
  --    只讀 profile ⇒ accepted_by_account_id 恆為空（#3031）。與 attestation_version／
  --    attested_at 同一組三元組，一起反正規化在本表。
  "attested_by_account_id"    bigint,
  "submitted_at"              timestamptz,                           -- 60 天期限的起算點：送出審核申請的時刻（單一時鐘）
  "pending_valid_until"       timestamptz,
  "extension_count"           smallint NOT NULL DEFAULT 0,           -- 延長次數（屬性，不設 EXTENDED 狀態）
  "pending_order_limit_count"  integer NOT NULL DEFAULT 50,          -- 待審期間接單上限，先到者停接新單
  "pending_order_limit_amount" bigint  NOT NULL DEFAULT 100000,
  "pending_order_count"        integer NOT NULL DEFAULT 0,           -- 口徑＝已付款訂單，只在建立 invoice_documents 的同交易內累加
  "pending_order_amount"       bigint  NOT NULL DEFAULT 0,
  "review_note"               text,                                  -- entity_tax_type 選 OTHER 時必須註明實際型態
  -- review_note 最後一次**真的改變**的時點（#3089 第 11 項）。
  --
  -- 🔴 不能用整列的 updated_at 代替：BumpSellerTaxProfilePendingOrderCounters
  -- 會為**每一筆為該賣家建 document 的已付款訂單**推進 updated_at ⇒ 拿它當
  -- 「這則審核備註是什麼時候寫的」，數字會隨接單量一直往前跳，而備註一個字都沒改。
  -- （#3051 是同一個形狀：credential_updated_at 被健康檢查每分鐘改寫。）
  "review_note_at"            timestamptz,
  "approved_at"               timestamptz,                           -- 核定時點；approval_valid_until = approved_at + 365 天的左運算元
  "cooldown_until"            timestamptz,                           -- 駁回／逾期後可重申的 7 天冷卻
  "approval_valid_until"      timestamptz,                           -- 屆滿不改狀態，只餵 A03「屆期待複審」分頁與 idx_stp_recheck_due
  "recheck_reason"            text,
  -- §6.1.9 S05 免用統一發票的收據代製授權（端點 28／29）。四欄一組：
  --   receipt_authorized_at / receipt_consent_version 是**賣家**按下同意時寫的；
  --   receipt_approval_basis_* 是 **ADM** 在端點 40 核定時指名的依據文件（不另外上傳，
  --   必須是該賣家自己在 S02 上傳的稅籍文件之一 ⇒ media 的授權述詞才成立）。
  -- 🔴 **不加「只有 APPROVED_INVOICE_EXEMPT 才能有 receipt_authorized_at」的 CHECK**：
  --    review_status 之後會被複審推翻（T-C04），而那條 CHECK 會讓**之後每一次**對該列的
  --    更新都撞 23514（同 chk_ipa_cutover_requires_verified 綁 verified_at 而不綁 status
  --    的理由）。資格守衛寫在授權那一句 CAS 的 WHERE，rows=0 回 9335。
  "receipt_authorized_at"     timestamptz,                          -- 賣家同意平台代製收據的時點
  "receipt_consent_version"   varchar(32),                          -- 同意的條款版本；開立時快照進 invoice_documents.receipt_authorization_version
  "receipt_approval_basis_key" text,                                -- §25：只存 objectKey，不存完整 URL
  "receipt_approval_basis_asset_id" bigint,                         -- 給 Gateway 組 approvalBasisDownloadPath（key 才是 SSOT）
  "violation_window_days"     smallint NOT NULL DEFAULT 90,          -- 90 天內 3 次判定違規 ⇒ 建議停權 14 天
  "violation_threshold"       smallint NOT NULL DEFAULT 3,
  "suspension_days"           smallint NOT NULL DEFAULT 14,          -- 建議值，本服務不倒數（停權權威＝sellers.status）
  "version"                   bigint NOT NULL DEFAULT 0,
  "created_at"                timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"                timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "seller_tax_profiles_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_seller_tax_profiles_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_stp_review_status" CHECK ("review_status" IN ('DRAFT','SUBMITTED','UNDER_REVIEW','SUPPLEMENT_REQUIRED','SELF_ISSUE_PENDING','APPROVED_INVOICE_REQUIRED','APPROVED_INVOICE_EXEMPT','REJECTED','EXPIRED','REVOKED')),
  CONSTRAINT "chk_stp_obligation" CHECK ("claimed_obligation" IN ('REQUIRED','EXEMPT','UNKNOWN')),
  CONSTRAINT "chk_stp_tax_category" CHECK ("default_tax_category" IN (1, 2, 3)),
  CONSTRAINT "chk_stp_tax_id" CHECK ("tax_id" IS NULL OR "tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_stp_required_needs_taxid" CHECK ("review_status" <> 'APPROVED_INVOICE_REQUIRED' OR ("tax_id" IS NOT NULL AND "legal_name" IS NOT NULL)),
  CONSTRAINT "chk_stp_exempt_no_taxid" CHECK ("review_status" <> 'APPROVED_INVOICE_EXEMPT' OR "tax_id" IS NULL),
  CONSTRAINT "chk_stp_extension" CHECK ("extension_count" BETWEEN 0 AND 1),
  CONSTRAINT "chk_stp_extension_needs_deadline" CHECK ("extension_count" = 0 OR "pending_valid_until" IS NOT NULL),
  CONSTRAINT "chk_stp_pending_window" CHECK ("review_status" NOT IN ('SELF_ISSUE_PENDING', 'SUPPLEMENT_REQUIRED') OR ("submitted_at" IS NOT NULL AND "pending_valid_until" IS NOT NULL)),
  CONSTRAINT "chk_stp_valid_until_needs_approved" CHECK ("approval_valid_until" IS NULL OR ("approved_at" IS NOT NULL AND "approval_valid_until" > "approved_at")),
  CONSTRAINT "chk_stp_counters_nonneg" CHECK ("pending_order_count" >= 0 AND "pending_order_amount" >= 0 AND "version" >= 0),
  -- 授權與條款版本同進同出：只有時點沒有版本 ⇒ 事後不知道賣家同意的是哪一版條款，
  -- 而那正是爭議時唯一能舉證的東西；只有版本沒有時點 ⇒ 看起來像已授權但從未發生。
  CONSTRAINT "chk_stp_receipt_auth_pair" CHECK (("receipt_authorized_at" IS NULL) = ("receipt_consent_version" IS NULL)),
  -- 依據文件的 key 與 asset id 同進同出：只有 asset id 沒有 key，§25 的「objectKey 是 SSOT」
  -- 就斷了；只有 key 沒有 asset id，Gateway 組不出下載路徑而前端會拿到一個點下去 404 的連結。
  CONSTRAINT "chk_stp_receipt_basis_pair" CHECK (("receipt_approval_basis_key" IS NULL) = ("receipt_approval_basis_asset_id" IS NULL)),
  -- 條款版本不得是空字串：NULL 與 '' 在 chk_stp_receipt_auth_pair 底下判定相反，
  -- 而 Go 的零值是後者。
  CONSTRAINT "chk_stp_receipt_consent_nonblank" CHECK ("receipt_consent_version" IS NULL OR length("receipt_consent_version") >= 1),
  -- §6.1.7 裁決 C 的不變式，三條一組（同 receipt_* 那一組的形狀）：
  --   ① 簽署時點與版本同進同出——只有時點沒有版本 ⇒ 事後不知道賣家同意的是哪一版，
  --      而那正是「審核期間發票由賣家自行開立、責任在賣家」唯一能舉證的東西；
  --      只有版本沒有時點 ⇒ 看起來像已簽但從未發生。
  --   ② 版本不得是空字串——Go 的零值是 ''，而 '' 在 ① 底下算「非空」，
  --      不擋的話一次漏傳參數就寫出一列「簽了一份沒有名字的條款」。
  --   ③ 🔴 **送審成功 ⇒ 條款必已簽**。這是裁決 C 的核心承諾，先前只有 usecase 那一道
  --      9325 守衛（`Submit` 進交易前判 accepted_terms），任何繞過該 usecase 的寫入路徑
  --      ——新 query、維運 SQL、未來的 ADM 代送——都能寫出「已送審但沒有簽署記錄」的列，
  --      而它不會報錯、不會有告警，只會在爭議發生時才發現沒有憑據。
  --      值域是 {SUBMITTED, SELF_ISSUE_PENDING}：前者是 T02 的落點，後者是「自行開票
  --      責任生效中」的那個狀態（domain.SellerTaxProfile.SelfIssueEffective 的唯一判準）
  --      ——生效中卻查不到簽署記錄，正是本條要防的最壞情況。
  --      ⚠️ **刻意不含 UNDER_REVIEW／APPROVED_*／REJECTED／EXPIRED／REVOKED**：那些狀態
  --      不承載自行開票的法律主張，而 ADM 可以從 EXPIRED／REVOKED 直接把列拉回
  --      UNDER_REVIEW（T13／T15／T16），把它們納進來等於讓一條與稅務審核無關的
  --      ADM 動作被一張 60 天前的簽署記錄擋住。同一理由寫在 receipt_authorized_at
  --      那一段的紅字裡（不綁 status 而綁那一句 CAS 的 WHERE）。
  CONSTRAINT "chk_stp_attestation_pair" CHECK (("attested_at" IS NULL) = ("attestation_version" IS NULL)),
  CONSTRAINT "chk_stp_attestation_version_nonblank" CHECK ("attestation_version" IS NULL OR length("attestation_version") >= 1),
  CONSTRAINT "chk_stp_attested_when_committed" CHECK ("review_status" NOT IN ('SUBMITTED', 'SELF_ISSUE_PENDING') OR ("attestation_version" IS NOT NULL AND "attested_at" IS NOT NULL))
);

CREATE UNIQUE INDEX "uq_seller_tax_profiles_issuer" ON "seller_tax_profiles" ("issuer_id");  -- 一 issuer 一列
-- A06 ADM 稅務審核佇列
CREATE INDEX "idx_stp_review_queue" ON "seller_tax_profiles" ("review_status", "created_at") WHERE ((review_status)::text = ANY ((ARRAY['SUBMITTED'::character varying, 'UNDER_REVIEW'::character varying, 'SUPPLEMENT_REQUIRED'::character varying, 'SELF_ISSUE_PENDING'::character varying])::text[]));
-- 60 天到期掃描（T10）。SUPPLEMENT_REQUIRED 必須在述詞內，否則補件中的賣家永不逾期；
-- 因採 extension_count 屬性而非 EXTENDED 狀態，本述詞天然涵蓋被延長過的賣家（I-34）
CREATE INDEX "idx_stp_pending_expiry" ON "seller_tax_profiles" ("pending_valid_until") WHERE ((review_status)::text = ANY ((ARRAY['SELF_ISSUE_PENDING'::character varying, 'SUPPLEMENT_REQUIRED'::character varying])::text[]));
-- A03「屆期待複審」分頁 ＋ INV.gov 每日推 invoice_seller_approval_recheck_due 指標。
-- DB 就是 SSOT：不需要 worker 改狀態、不需要事件、更不需要「已通知」旗標（那是 《多副本排程搶佔規範》禁止的永久旗標）
CREATE INDEX "idx_stp_recheck_due" ON "seller_tax_profiles" ("approval_valid_until") WHERE ((review_status)::text = ANY ((ARRAY['APPROVED_INVOICE_REQUIRED'::character varying, 'APPROVED_INVOICE_EXEMPT'::character varying])::text[]));

-- ---------------------------------------------------------------------------
-- §5.1.3 invoice_provider_accounts｜provider 憑證與帳戶狀態
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_provider_accounts" (
  "id"                       bigint NOT NULL,                                  -- TSID
  "issuer_id"                bigint NOT NULL,
  "provider"                 varchar(16) NOT NULL DEFAULT 'AMEGO',             -- 兩個唯一索引的鍵，不進任何 DTO
  "provider_tax_id"          varchar(8) NOT NULL,                              -- 光貿帳戶綁定的統編（＝AMEGO invoice_merchant_id）
  "credential_ciphertext"    text NOT NULL,                                    -- AES-256-GCM base64。用 text 不用 varchar(n)
  "credential_key_version"   smallint NOT NULL DEFAULT 1,                      -- 對應 INVOICE_ENCRYPTION_MASTER_KEY / _PREV
  "credential_fingerprint"   varchar(64) NOT NULL,                             -- HMAC-SHA256 hex，canonical 形式見下方
  "credential_last4"         varchar(4) NOT NULL,                              -- UI 遮罩顯示
  "status"                   varchar(32) NOT NULL DEFAULT 'CREDENTIAL_SUBMITTED', -- CodeMeta InvoiceProviderAccountStatus 落 DB 的 11 值，見 chk_ipa_status
  "status_reason"            text,
  "verified_at"              timestamptz,
  "automation_cutover_at"    timestamptz,                                      -- 自動開立切點：此時間之後付款的訂單才走 AMEGO_AUTO；之前標 LEGACY_MANUAL
  "last_health_checked_at"   timestamptz,                                      -- 事實時間戳：最後一次真的打了 AMEGO
  "health_check_next_at"     timestamptz NOT NULL DEFAULT NOW(),                 -- 健康檢查的租約欄（《多副本排程搶佔規範》）。不可拿 last_health_checked_at 充當
  "breaker_opened_at"        timestamptz,                                      -- DEGRADED ＝ circuit breaker open/half-open 的持久化投影
  "breaker_probe_success"    smallint NOT NULL DEFAULT 0,
  "pending_ciphertext"       text,                                             -- 輪替：先驗證再覆蓋，失敗保留原 key ⇒ 新舊密文需同時存在一段時間
  "pending_fingerprint"      varchar(64),
  "pending_last4"            varchar(4),
  "rotation_started_at"      timestamptz,
  -- 憑證專用時間戳（#3051）。S03 的「AppKey 更新於」原本下發整列的 updated_at，
  -- 而健康檢查輪詢每分鐘改寫那一欄（實查同一列 updated_at == last_health_checked_at、
  -- version 已被推到 225，憑證卻只設定過一次）⇒ 畫面在告訴賣家一件沒有發生的事。
  --
  -- 🔴 它是賣家唯一能察覺「我的 AppKey 被換過嗎」的訊號：AppKey write-only，平台與
  -- 賣家都看不到全文 ⇒ 這個時間永遠在跳，等於這個訊號永遠是雜訊。
  -- 只有 PutCredentials／Rotate 落地時才寫，健康檢查一律不碰。
  "credential_updated_at"    timestamptz,
  "version"                  bigint NOT NULL DEFAULT 0,
  "created_at"               timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"               timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_provider_accounts_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_ipa_issuer" FOREIGN KEY ("issuer_id") REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_ipa_provider" CHECK ("provider" IN ('AMEGO')),
  -- CodeMeta 另有 NOT_STARTED / CONFIG_ERROR 兩個投影值，只出現在 API 回應與 ADM 篩選器，不落 DB ⇒ 本 CHECK 是該 enum 的真子集
  CONSTRAINT "chk_ipa_status" CHECK ("status" IN ('NOT_CONFIGURED','CREDENTIAL_SUBMITTED','CREDENTIAL_VERIFYING','ACTIVE','INVALID_CREDENTIAL','PROVIDER_REJECTED','PROVIDER_UNAVAILABLE','ROTATION_PENDING','DEGRADED','SUSPENDED','REVOKED')),
  CONSTRAINT "chk_ipa_tax_id" CHECK ("provider_tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_ipa_cutover_requires_verified" CHECK ("automation_cutover_at" IS NULL OR "verified_at" IS NOT NULL),
  CONSTRAINT "chk_ipa_active_verified" CHECK ("status" <> 'ACTIVE' OR "verified_at" IS NOT NULL),
  CONSTRAINT "chk_ipa_rotation" CHECK (("status" = 'ROTATION_PENDING') = ("pending_ciphertext" IS NOT NULL)),
  CONSTRAINT "chk_ipa_rotation_triplet" CHECK (("pending_ciphertext" IS NULL AND "pending_fingerprint" IS NULL AND "pending_last4" IS NULL) OR ("pending_ciphertext" IS NOT NULL AND "pending_fingerprint" IS NOT NULL AND "pending_last4" IS NOT NULL)),
  CONSTRAINT "chk_ipa_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_ipa_issuer_provider" ON "invoice_provider_accounts" ("issuer_id", "provider");
CREATE UNIQUE INDEX "uq_ipa_fingerprint" ON "invoice_provider_accounts" ("provider", "credential_fingerprint") WHERE ((status)::text = ANY ((ARRAY['CREDENTIAL_SUBMITTED'::character varying, 'CREDENTIAL_VERIFYING'::character varying, 'ACTIVE'::character varying, 'INVALID_CREDENTIAL'::character varying, 'PROVIDER_REJECTED'::character varying, 'PROVIDER_UNAVAILABLE'::character varying, 'ROTATION_PENDING'::character varying, 'DEGRADED'::character varying, 'SUSPENDED'::character varying])::text[]));
-- provider 健康檢查排程（租約欄是 health_check_next_at，不是 last_health_checked_at）
-- INV.hc 候選索引。三個狀態、兩種語意：ACTIVE／DEGRADED 是無終態巡檢，
-- PROVIDER_UNAVAILABLE 是 §7.3.4 P08 的自動重驗（上界 24h，錨在 breaker_opened_at，
-- 由 query 的述詞負責——partial index 不能含 now()，那不是 IMMUTABLE）。
-- ⚠️ 這裡的狀態集合必須與 FetchDueHealthChecks／ClaimHealthCheck 逐字一致；
--    少列一個狀態不會壞掉、只會讓那批列走全表掃描，而那是無聲的。
CREATE INDEX "idx_ipa_health" ON "invoice_provider_accounts" ("health_check_next_at") WHERE ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'DEGRADED'::character varying, 'PROVIDER_UNAVAILABLE'::character varying])::text[]));

-- ---------------------------------------------------------------------------
-- §5.1.4 buyer_invoice_preferences｜買家常用發票資料（append-only）
-- ---------------------------------------------------------------------------
CREATE TABLE "buyer_invoice_preferences" (
  "id"                 bigint NOT NULL,                    -- TSID（對外暴露，結帳與 B08 都用它）
  "buyer_id"           bigint NOT NULL,                    -- buyers.buyer_id，非 account_id
  "preference_kind"    varchar(20) NOT NULL,               -- CodeMeta InvoiceCarrierType 4 值（UPPER_SNAKE）。對外 JSON 名是 carrierType
  "carrier_type"       varchar(6),                         -- AMEGO CarrierType 原值 '3J0002' / 'CQ0001'；TAX_ID/DONATION 為 NULL。**不出 API**
  "carrier_value_ciphertext" text,                         -- 載具顯碼密文（AES-256-GCM base64）。AMEGO CarrierId1 上限 64 字元
  "carrier_value_hash" varchar(64),                        -- HMAC-SHA256 hex，供同買家去重與 B08 列表，避免解密全表掃描
  "carrier_value_mask" varchar(16),                        -- UI 遮罩顯示（例：/ABC***9）
  "tax_id"             varchar(8),                         -- 公司統編（公開營業登記資訊，明文）
  "tax_title"          varchar(60),                        -- 買方公司抬頭。AMEGO BuyerName 上限 60 字元（61 → 3040123）
  "donation_code"      varchar(7),                         -- 捐贈碼，3–7 碼純數字
  "label"              varchar(30),                        -- 買家自訂備註名
  "is_default"         boolean NOT NULL DEFAULT false,
  "superseded_at"      timestamptz,
  "superseded_by_id"   bigint,
  "deleted_at"         timestamptz,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"         timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "buyer_invoice_preferences_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_bip_superseded" FOREIGN KEY ("superseded_by_id") REFERENCES "buyer_invoice_preferences" ("id"),
  CONSTRAINT "chk_bip_kind" CHECK ("preference_kind" IN ('MOBILE_BARCODE','CITIZEN_CERT','TAX_ID','DONATION')),
  CONSTRAINT "chk_bip_shape" CHECK (
    ("preference_kind" = 'MOBILE_BARCODE' AND COALESCE("carrier_type", '') = '3J0002' AND "carrier_value_ciphertext" IS NOT NULL AND "tax_id" IS NULL AND "donation_code" IS NULL)
 OR ("preference_kind" = 'CITIZEN_CERT'   AND COALESCE("carrier_type", '') = 'CQ0001' AND "carrier_value_ciphertext" IS NOT NULL AND "tax_id" IS NULL AND "donation_code" IS NULL)
 OR ("preference_kind" = 'TAX_ID'         AND "carrier_type" IS NULL AND "carrier_value_ciphertext" IS NULL AND "tax_id" IS NOT NULL AND "tax_title" IS NOT NULL AND "donation_code" IS NULL)
 OR ("preference_kind" = 'DONATION'       AND "carrier_type" IS NULL AND "carrier_value_ciphertext" IS NULL AND "tax_id" IS NULL AND "donation_code" IS NOT NULL)),
  CONSTRAINT "chk_bip_tax_id"          CHECK ("tax_id" IS NULL OR "tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_bip_donation_code"   CHECK ("donation_code" IS NULL OR "donation_code" ~ '^[0-9]{3,7}$'),
  CONSTRAINT "chk_bip_hash_pair"       CHECK (("carrier_value_ciphertext" IS NULL) = ("carrier_value_hash" IS NULL)),
  CONSTRAINT "chk_bip_superseded_pair" CHECK (("superseded_at" IS NULL) = ("superseded_by_id" IS NULL)),
  CONSTRAINT "chk_bip_default_alive"   CHECK (NOT "is_default" OR ("superseded_at" IS NULL AND "deleted_at" IS NULL))
);

CREATE UNIQUE INDEX "uq_bip_default" ON "buyer_invoice_preferences" ("buyer_id")
  WHERE "is_default" AND "deleted_at" IS NULL AND "superseded_at" IS NULL;
CREATE UNIQUE INDEX "uq_bip_carrier_dedup" ON "buyer_invoice_preferences" ("buyer_id", "carrier_type", "carrier_value_hash")
  WHERE "carrier_value_hash" IS NOT NULL AND "deleted_at" IS NULL AND "superseded_at" IS NULL;
CREATE UNIQUE INDEX "uq_bip_taxid_dedup" ON "buyer_invoice_preferences" ("buyer_id", "tax_id")
  WHERE "tax_id" IS NOT NULL AND "deleted_at" IS NULL AND "superseded_at" IS NULL;
-- B08「我的發票資料」列表：上面三條 partial unique 沒有一條服務得了它
CREATE INDEX "idx_bip_buyer" ON "buyer_invoice_preferences" ("buyer_id", "created_at" DESC)
  WHERE "deleted_at" IS NULL AND "superseded_at" IS NULL;

-- ---------------------------------------------------------------------------
-- §5.1.5 invoice_documents｜發票／收據文件
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_documents" (
  "id"                    bigint NOT NULL,                            -- TSID
  "issuer_id"             bigint NOT NULL,
  "issuer_name_snapshot"  varchar(120) NOT NULL,                      -- 開票當下自 seller_tax_profiles 複製，之後不隨主檔變動
  "issuer_tax_id_snapshot" varchar(8),                                -- EXEMPT 賣家的收據無統編
  "source_type"           varchar(16) NOT NULL,                       -- ORDER / MANUAL
  "source_ref"            varchar(64),                                -- ORDER ⇒ orders.order_no；MANUAL ⇒ NULL
  "trigger_type"          varchar(16) NOT NULL,                       -- AUTO / ADM_MANUAL
  "recipient_type"        varchar(16) NOT NULL,                       -- BUYER / SELLER / EXTERNAL
  "recipient_ref_id"      bigint,                                     -- BUYER ⇒ buyers.buyer_id；SELLER ⇒ sellers.store_id；EXTERNAL ⇒ NULL
  "recipient_name"        varchar(60) NOT NULL,                       -- AMEGO BuyerName，上限 60 字元。取值規則見下方取值表
  "recipient_tax_id"      varchar(10) NOT NULL DEFAULT '0000000000',  -- B2C 固定 10 個 0；B2B 為 8 碼統編
  "recipient_email_ciphertext" text,                                  -- AMEGO BuyerEmailAddress（上限 312 字元）
  "recipient_email_hash"  varchar(64),
  "recipient_phone_ciphertext" text,
  "carrier_type"          varchar(6),                                 -- AMEGO 原值 '3J0002' / 'CQ0001' / 'amego'
  "carrier_id1_ciphertext" text,
  "carrier_id2_ciphertext" text,
  "carrier_value_hash"    varchar(64),
  "donation_code"         varchar(7),
  "document_kind"         varchar(16) NOT NULL,                       -- B2C / B2B / RECEIPT（恆三值）
  "fulfillment_mode"      varchar(24) NOT NULL,                       -- AMEGO_AUTO / SELLER_SELF_ISSUE / PLATFORM_RECEIPT / LEGACY_MANUAL
  "status"                varchar(24) NOT NULL DEFAULT 'PENDING',     -- CodeMeta InvoiceDocumentStatus，11 值見 chk_invdoc_status
  "not_applicable_reason" varchar(32),                                -- status=NOT_APPLICABLE 時必填。CodeMeta InvoiceNotApplicableReason（5 值）
  "manual_reason_code"    varchar(32),                                -- status=AWAITING_MANUAL 時必填。CodeMeta InvoiceManualReason，ADM 工作台直接顯示它
  "cancel_reason_code"    varchar(48),                                -- CodeMeta InvoiceCancelReason（4 值）
  "cancel_reason"         text,
  "snapshot_version"      integer NOT NULL DEFAULT 1,                 -- 內容軸，來自 order_invoice_snapshots；唯一寫入者是 order-service（I-50）
  "issue_attempt_seq"     integer NOT NULL DEFAULT 1,                 -- 實體軸：同一 snapshot_version 下第幾張實體發票（作廢重開 +1）
  "provider_account_version" bigint,                                  -- 開票當下的 invoice_provider_accounts.version 快照；對外 wire 名是 cutoverVersion
  "superseded_by_id"      bigint,                                     -- 作廢重開鏈：舊文件指向新文件
  "original_document_id"  bigint,                                     -- 重開文件指回鏈頭
  "sales_amount"          bigint NOT NULL,                            -- 應稅銷售額合計。B2C＝含稅；B2B＝已扣除 tax_amount 的未稅
  "free_tax_sales_amount" bigint NOT NULL DEFAULT 0,
  "zero_tax_sales_amount" bigint NOT NULL DEFAULT 0,
  "tax_amount"            bigint NOT NULL DEFAULT 0,                  -- B2C 一律 0
  "total_amount"          bigint NOT NULL,
  "tax_type"              smallint NOT NULL DEFAULT 1,                -- 文件級課稅別 1/2/3/9（9 僅 B2C C0401 可用），由實際輸出的行推導（§5A.6.4）
  "tax_rate_bps"          integer NOT NULL DEFAULT 500,               -- 5% = 500 bps；送 AMEGO 時轉字串 "0.05"
  "allowance_total_amount" bigint NOT NULL DEFAULT 0,                 -- 折讓／作廢累計（反正規化，與 invoice_adjustments 寫入同交易維護）
  "voided_at"             timestamptz,
  "provider"              varchar(16),                                -- AMEGO；RECEIPT 為 NULL
  "provider_request_no"   varchar(40),                                -- ＝AMEGO OrderId，可重建（§5.3.2）
  "provider_invoice_no"   varchar(10),                                -- AMEGO invoice_number
  "provider_random_code"  varchar(4),                                 -- AMEGO random_number
  "provider_issued_at"    timestamptz,                                -- AMEGO invoice_time（Unix ts → UTC）
  "external_invoice_no"   varchar(10),                                -- 賣家自行開立並回報的號碼（status=RESOLVED_EXTERNALLY 專用）
  "external_invoice_date" date,                                       -- 那張發票上印的日期（台灣曆日，UTC+8 日界）
  "external_reported_at"  timestamptz,                                -- 平台收到回報的時刻（UTC）；端點 62／56 寫同一欄
  "provider_upload_status" smallint,                                  -- AMEGO invoice_status：1/2/3/31/32/91/99，另實測會回未列舉的 -1
  "provider_upload_checked_at" timestamptz,                           -- 事實時間戳：最後一次真的查了 AMEGO 的時刻
  "upload_recheck_next_at" timestamptz,                               -- 上傳輪詢租約欄（《多副本排程搶佔規範》）。NULL ＝ 已離開候選集合
  "upload_recheck_attempts" smallint NOT NULL DEFAULT 0,
  "receipt_no"            varchar(20),                                -- 平台代製收據編號（§5.3.3）
  "receipt_authorization_version" varchar(32),                        -- 賣家授權代製的條款版本
  "pdf_object_key"        varchar(512),                               -- 只存 objectKey，禁 URL／presigned／JWT
  "pdf_fetch_next_at"     timestamptz,                                -- PDF 抓取租約欄（《多副本排程搶佔規範》）。NULL ＝ 已離開候選集合。與上傳輪詢是兩組獨立租約
  "pdf_fetch_attempts"    smallint NOT NULL DEFAULT 0,
  "invoice_date_taipei"   date,                                       -- 台灣曆日，AMEGO invoice_date 的來源
  "tax_period_code"       char(5),                                    -- 民國年 3 碼 + 期別 2 碼，例 '11504'
  "issue_not_before"      timestamptz NOT NULL,                       -- 可開立的最早時點，取值見下方取值表
  "paid_at"               timestamptz,                                -- 該筆訂單的收款時點（COALESCE(payment.provider_paid_at, pay_time)）。賣家 S06 對帳用；ADM 手動開票與作廢重開的新票沒有對應的收款事件故為 NULL
  "statutory_deadline_at" timestamptz,                                -- 法定存證期限（自然日算式，見下方取值表）。權威值，invoice_operations 同名欄是它的副本
  "source_snapshot"       jsonb,                                      -- 開票判定依據的白名單快照（已遮罩）
  -- §6.1.10 端點 32／49／71 的重寄通知窗口：同一 document_id 每 10 分鐘 1 次，
  -- **三個入口共用同一個計數器**（key ＝ document_id，不是 document_id × actor）。
  -- 🔴 落在本表而不是 Redis：本服務的 fx 圖上沒有任何 redis client，而為了一個
  --    計數器接進整個 redis 相依，還要付《硬上限依賴接入治理規範》的三件前置
  --    （用量＋上限兩個 series、兩條告警、預算數字）——那與它守護的東西不成比例。
  --    窗口述詞是一句 `notification_resent_at <= now - 10min`，DB 本來就在交易裡。
  -- ⚠️ 重寄**不遞增 version**：它不是狀態轉移，沒有人拿它做樂觀鎖。跟著 +1 的話，
  --    ADM 開著 A06 頁面時只要有人按了重寄，他的重試／取消就會被回 9343，而畫面上
  --    什麼都沒變。
  "notification_resent_at"    timestamptz,                             -- 最後一次重寄的時點（窗口起算）
  "notification_resend_count" integer NOT NULL DEFAULT 0,              -- 累計重寄次數；同時是 notification_ref_id 的去重種子
  "version"               bigint NOT NULL DEFAULT 0,
  "created_at"            timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"            timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_documents_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invdoc_issuer"     FOREIGN KEY ("issuer_id")            REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "fk_invdoc_superseded" FOREIGN KEY ("superseded_by_id")     REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invdoc_original"   FOREIGN KEY ("original_document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "chk_invdoc_source"    CHECK ("source_type" IN ('ORDER','MANUAL')),
  CONSTRAINT "chk_invdoc_trigger"   CHECK ("trigger_type" IN ('AUTO','ADM_MANUAL')),
  CONSTRAINT "chk_invdoc_recipient" CHECK ("recipient_type" IN ('BUYER','SELLER','EXTERNAL')),
  CONSTRAINT "chk_invdoc_kind"      CHECK ("document_kind" IN ('B2C','B2B','RECEIPT')),
  CONSTRAINT "chk_invdoc_mode"      CHECK ("fulfillment_mode" IN ('AMEGO_AUTO','SELLER_SELF_ISSUE','PLATFORM_RECEIPT','LEGACY_MANUAL')),
  CONSTRAINT "chk_invdoc_status"    CHECK ("status" IN ('PENDING','ISSUING','ISSUED','ISSUE_FAILED','AWAITING_MANUAL','VOIDING','VOIDED','SUPERSEDED','CANCELLED_BEFORE_ISSUE','NOT_APPLICABLE','RESOLVED_EXTERNALLY')),
  CONSTRAINT "chk_invdoc_tax_type"  CHECK ("tax_type" IN (1,2,3,9)),
  CONSTRAINT "chk_invdoc_source_ref" CHECK (("source_type" = 'ORDER') = ("source_ref" IS NOT NULL)),
  -- AMEGO 官方算式：TotalAmount = Sales + FreeTax + ZeroTax + Tax
  CONSTRAINT "chk_invdoc_total" CHECK ("total_amount" = "sales_amount" + "free_tax_sales_amount" + "zero_tax_sales_amount" + "tax_amount"),
  CONSTRAINT "chk_invdoc_amounts_nonneg" CHECK ("sales_amount" >= 0 AND "free_tax_sales_amount" >= 0 AND "zero_tax_sales_amount" >= 0 AND "tax_amount" >= 0 AND "total_amount" >= 0),
  CONSTRAINT "chk_invdoc_b2c" CHECK ("document_kind" <> 'B2C' OR ("tax_amount" = 0 AND "recipient_tax_id" = '0000000000')),
  -- B2B 必為 8 碼統編；不可左補零成 10 碼（會被 3040122 拒）
  CONSTRAINT "chk_invdoc_b2b" CHECK ("document_kind" <> 'B2B' OR "recipient_tax_id" ~ '^[0-9]{8}$'),
  CONSTRAINT "chk_invdoc_taxid_shape" CHECK ("recipient_tax_id" ~ '^([0-9]{8}|0000000000)$'),
  -- 統編／載具／捐贈碼三選一。AMEGO 不擋（三種組合實測皆 code:0），只能我方擋
  CONSTRAINT "chk_invdoc_exclusive" CHECK (
    (CASE WHEN "recipient_tax_id" <> '0000000000' THEN 1 ELSE 0 END)
  + (CASE WHEN "carrier_type" IS NOT NULL THEN 1 ELSE 0 END)
  + (CASE WHEN "donation_code" IS NOT NULL THEN 1 ELSE 0 END) <= 1),
  CONSTRAINT "chk_invdoc_carrier_pair" CHECK (("carrier_type" IS NULL) = ("carrier_id1_ciphertext" IS NULL)),
  CONSTRAINT "chk_invdoc_donation" CHECK ("donation_code" IS NULL OR "donation_code" ~ '^[0-9]{3,7}$'),
  CONSTRAINT "chk_invdoc_issued_has_no" CHECK ("fulfillment_mode" <> 'AMEGO_AUTO' OR "status" <> 'ISSUED' OR "provider_invoice_no" IS NOT NULL),
  CONSTRAINT "chk_invdoc_receipt" CHECK (("document_kind" = 'RECEIPT') = ("receipt_no" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_receipt_no_provider" CHECK ("document_kind" <> 'RECEIPT' OR ("provider_invoice_no" IS NULL AND "provider" IS NULL)),
  -- RECEIPT 的稅額必須為 0，理由與 chk_invdoc_b2c 同源但**先前只有應用層守著**：
  -- 收據不是統一發票（出據人依法免用），沒有可分列的營業稅額；而 BuildHeader 對
  -- 非 B2B 一律填 tax_amount = 0。B2C 有 CHECK、RECEIPT 沒有，是 40 條 chk_invdoc*
  -- 裡唯一的空缺 —— 而少了它，任何繞過 BuildHeader 的寫入路徑（維運 SQL、日後新增的
  -- usecase）都能在收據上寫出一個稅額，然後那個數字會直接印進 PDF 的欄位表。
  -- ⚠️ 不併進 chk_invdoc_b2c：那一條同時管 recipient_tax_id，而 RECEIPT 的買受人統編
  --    規則由 resolveRecipient 決定、與 B2C 不必然相同。兩件事分開守才改得動。
  CONSTRAINT "chk_invdoc_receipt_tax" CHECK ("document_kind" <> 'RECEIPT' OR "tax_amount" = 0),
  -- 賣家自行開票／cutover 前訂單不得顯示為「平台已開立」，也不得被平台作廢
  CONSTRAINT "chk_invdoc_self_issue" CHECK ("fulfillment_mode" NOT IN ('SELLER_SELF_ISSUE', 'LEGACY_MANUAL') OR "status" IN ('NOT_APPLICABLE', 'RESOLVED_EXTERNALLY', 'CANCELLED_BEFORE_ISSUE')),
  -- 外部三欄同進同出：只寫號碼會當場 23514
  CONSTRAINT "chk_invdoc_external_no" CHECK (
    ("status" = 'RESOLVED_EXTERNALLY') = ("external_invoice_no" IS NOT NULL)
    AND ("status" = 'RESOLVED_EXTERNALLY') = ("external_invoice_date" IS NOT NULL)
    AND ("status" = 'RESOLVED_EXTERNALLY') = ("external_reported_at" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_external_reported_after" CHECK ("external_reported_at" IS NULL OR "external_reported_at" >= "issue_not_before"),
  CONSTRAINT "chk_invdoc_external_no_shape" CHECK ("external_invoice_no" IS NULL OR "external_invoice_no" ~ '^[A-Z]{2}[0-9]{8}$'),
  CONSTRAINT "chk_invdoc_external_excl" CHECK ("external_invoice_no" IS NULL OR "provider_invoice_no" IS NULL),
  CONSTRAINT "chk_invdoc_kind_mode" CHECK (("document_kind" = 'RECEIPT') = ("fulfillment_mode" = 'PLATFORM_RECEIPT')),
  CONSTRAINT "chk_invdoc_na_reason" CHECK (("status" = 'NOT_APPLICABLE') = ("not_applicable_reason" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_manual_reason" CHECK ("status" <> 'AWAITING_MANUAL' OR "manual_reason_code" IS NOT NULL),
  CONSTRAINT "chk_invdoc_voided_pair" CHECK (("status" = 'VOIDED') = ("voided_at" IS NOT NULL)),
  -- 時點與次數同進同出：只有次數沒有時點 ⇒ 窗口述詞永遠放行（等於沒有限流）；
  -- 只有時點沒有次數 ⇒ notification_ref_id 的去重種子恆為 0，第二次重寄會被
  -- notification-service 依 reference_id 靜默吞掉（而按下去的人看到的是成功）。
  CONSTRAINT "chk_invdoc_resend_pair" CHECK (("notification_resent_at" IS NULL) = ("notification_resend_count" = 0)),
  CONSTRAINT "chk_invdoc_resend_count_nonneg" CHECK ("notification_resend_count" >= 0),
  CONSTRAINT "chk_invdoc_allowance_cap" CHECK ("allowance_total_amount" >= 0 AND "allowance_total_amount" <= "total_amount"),
  CONSTRAINT "chk_invdoc_request_no_len" CHECK ("provider_request_no" IS NULL OR length("provider_request_no") <= 40),
  CONSTRAINT "chk_invdoc_period" CHECK ("tax_period_code" IS NULL OR "tax_period_code" ~ '^[0-9]{5}$'),
  CONSTRAINT "chk_invdoc_versions" CHECK ("snapshot_version" >= 1 AND "issue_attempt_seq" >= 1 AND "version" >= 0),
  CONSTRAINT "chk_invdoc_deadline" CHECK ("statutory_deadline_at" IS NULL OR "statutory_deadline_at" > "issue_not_before"),
  CONSTRAINT "chk_invdoc_deadline_presence" CHECK (("document_kind" IN ('B2C','B2B')) = ("statutory_deadline_at" IS NOT NULL)),
  CONSTRAINT "chk_invdoc_pdf_attempts" CHECK ("pdf_fetch_attempts" >= 0),
  -- 還掛著輪詢租約 ⇒ 狀態必須還在在途的正面集合內（99／91／-1 都該已把租約欄清成 NULL）
  CONSTRAINT "chk_invdoc_upload_recheck" CHECK ("upload_recheck_attempts" >= 0 AND ("upload_recheck_next_at" IS NULL OR "provider_upload_status" IS NULL OR "provider_upload_status" IN (1, 2, 3, 31, 32)))
);

CREATE UNIQUE INDEX "uq_invdoc_order_identity" ON "invoice_documents" ("issuer_id", "source_ref", "document_kind", "snapshot_version", "issue_attempt_seq") WHERE "source_type" = 'ORDER';
CREATE UNIQUE INDEX "uq_invdoc_provider_request_no" ON "invoice_documents" ("provider_request_no") WHERE "provider_request_no" IS NOT NULL;
CREATE UNIQUE INDEX "uq_invdoc_provider_invoice_no" ON "invoice_documents" ("provider", "provider_invoice_no") WHERE "provider_invoice_no" IS NOT NULL;
CREATE UNIQUE INDEX "uq_invdoc_receipt_no" ON "invoice_documents" ("receipt_no") WHERE "receipt_no" IS NOT NULL;

-- 業務查詢索引
CREATE INDEX "idx_invdoc_recipient" ON "invoice_documents" ("recipient_type", "recipient_ref_id", "status", "created_at" DESC);  -- 買家發票中心列表（端點 68）＋ 賣家「進項」分頁（端點 72，recipient_type='SELLER'）。**加了 status**：兩支列表都以狀態篩選，缺它會退化成全掃該人所有文件
CREATE INDEX "idx_invdoc_issuer_status" ON "invoice_documents" ("issuer_id", "status", "created_at" DESC);             -- S02 賣家列表 ＋ ADM 依賣家＋狀態篩選
CREATE INDEX "idx_invdoc_source_ref" ON "invoice_documents" ("source_ref") WHERE "source_type" = 'ORDER';              -- 訂單詳情反查發票、退款事件用 order_no 找文件

COMMENT ON TABLE  invoice_documents IS
  '稅務文件（發票／收據）。法定保存 5 年，列為 PII 清理例外：帳號刪除不得連帶刪除本表任何一列，只關閉對外查詢（授權層）。本服務不存在任何 DELETE FROM invoice_documents 的路徑（不變式 I-40）。';
COMMENT ON COLUMN invoice_documents.recipient_name IS
  'AMEGO BuyerName。永不寫入自然人真實姓名：B2C 恆為常數「消費者」、B2B 為公司抬頭、RECEIPT／MANUAL 為 ADM 或賣家手填抬頭。因此本欄是明文且不進遮罩清單。';
COMMENT ON COLUMN invoice_documents.source_snapshot IS
  '開票判定依據的白名單快照（已遮罩）。禁存收件人姓名／電話／地址／買家 email／載具值。';
COMMENT ON COLUMN invoice_documents.statutory_deadline_at IS
  '法定存證期限（自然日）：自 invoice_date_taipei 之翌日起算，B2C 2 日、B2B 7 日，末日 23:59:59 Asia/Taipei，不因假日延長；RECEIPT 無法定期限故為 NULL。禁止以 +48h 計算。本欄是權威值，invoice_operations 上的同名欄是它的反正規化副本（I-52）。';
COMMENT ON COLUMN invoice_documents.external_invoice_no IS
  '賣家自行開立並回報的發票號碼（status=RESOLVED_EXTERNALLY 專用）。與 provider_invoice_no 互斥且永不互相 fallback：平台不冒充開立人。';

-- ---------------------------------------------------------------------------
-- §5.1.6 invoice_document_lines｜品項行
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_document_lines" (
  "id"              bigserial NOT NULL,
  "document_id"     bigint NOT NULL,
  "line_seq"        integer NOT NULL,                    -- 1-based，決定送 AMEGO 的 ProductItem 順序
  "line_type"       varchar(20) NOT NULL,                -- CodeMeta InvoiceLineType：PRODUCT / SHIPPING / DISCOUNT
  "order_item_id"   bigint,                              -- order_items.id（TSID），SHIPPING/DISCOUNT 為 NULL
  "sku_id"          bigint,
  "description"     varchar(256) NOT NULL,               -- AMEGO ProductItem.Description 上限 256 字（257 → 3040139）
  "unit"            varchar(6),                          -- AMEGO ProductItem.Unit 上限 6 字。Phase 1 恆為 NULL（U-2）
  "quantity"        integer NOT NULL,
  "unit_price"      bigint NOT NULL,                     -- 含稅單價（元）。DISCOUNT 行為負值
  "net_amount"      bigint NOT NULL,                     -- ＝ unit_price × quantity，送 AMEGO 的 ProductItem.Amount
  "tax_category"    smallint NOT NULL DEFAULT 1,         -- AMEGO ProductItem.TaxType：1 應稅 / 2 零稅率 / 3 免稅
  "tax_rate_bps"    integer NOT NULL DEFAULT 500,
  "remark"          varchar(40),                         -- 開立端上限 120 字、查詢端只回 40 字 ⇒ 取保守值 40
  "allocated_discount"  bigint NOT NULL DEFAULT 0,
  "allowanceable_base"  bigint NOT NULL DEFAULT 0,       -- 折讓可用基數 ＝ net_amount − allocated_discount。DISCOUNT 行恆 0
  "allowanced_amount"   bigint NOT NULL DEFAULT 0,       -- 累計已折讓（含稅），每次部分折讓在同交易內 += 本次分攤額
  "created_at"      timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_document_lines_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invline_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id") ON DELETE CASCADE,
  CONSTRAINT "chk_invline_type" CHECK ("line_type" IN ('PRODUCT','SHIPPING','DISCOUNT')),
  CONSTRAINT "chk_invline_tax_category" CHECK ("tax_category" IN (1,2,3)),
  CONSTRAINT "chk_invline_qty" CHECK ("quantity" > 0),
  CONSTRAINT "chk_invline_amount" CHECK ("net_amount" = "unit_price" * "quantity"),
  CONSTRAINT "chk_invline_sign" CHECK (("line_type" = 'DISCOUNT' AND "net_amount" < 0) OR ("line_type" <> 'DISCOUNT' AND "net_amount" >= 0)),
  CONSTRAINT "chk_invline_seq" CHECK ("line_seq" >= 1),
  CONSTRAINT "chk_invline_seq_max" CHECK ("line_seq" <= 9999),        -- AMEGO ProductItem 最多 9999 筆
  CONSTRAINT "chk_invline_allowance_shape" CHECK (
    ("line_type" = 'DISCOUNT' AND "allocated_discount" = 0 AND "allowanceable_base" = 0 AND "allowanced_amount" = 0)
    OR ("line_type" <> 'DISCOUNT' AND "allocated_discount" >= 0 AND "allowanceable_base" = "net_amount" - "allocated_discount")),
  CONSTRAINT "chk_invline_allowanced_cap" CHECK ("allowanced_amount" >= 0 AND "allowanced_amount" <= "allowanceable_base")
);

CREATE UNIQUE INDEX "uq_invline_doc_seq" ON "invoice_document_lines" ("document_id", "line_seq");

-- ---------------------------------------------------------------------------
-- §5.1.7 invoice_adjustments ＋ invoice_adjustment_lines｜作廢與折讓
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_adjustments" (
  "id"                  bigint NOT NULL,                          -- TSID
  "document_id"         bigint NOT NULL,                          -- 原發票（AMEGO 硬約束：一張折讓單只對一張發票）
  "adjustment_type"     varchar(16) NOT NULL,                     -- VOID / ALLOWANCE
  "status"              varchar(32) NOT NULL DEFAULT 'PENDING',   -- CodeMeta InvoiceAdjustmentStatus，9 值見 chk_invadj_status
  "trigger_kind"        varchar(24) NOT NULL,                     -- ORDER_REFUND / BUYER_CORRECTION / ADM_MANUAL，也是冪等鍵的一段
  "refund_no"           varchar(32),                              -- ORDER_REFUND ⇒ 權威退款單號（＝financial_logs.external_txn_id）
  "correction_request_id" bigint,                                 -- BUYER_CORRECTION ⇒ invoice_correction_requests.id
  "allowance_number"    varchar(16),                              -- 我方自產、送出前先落庫（AMEGO 不回傳單號，§5.3.3）
  "allowance_date_taipei" date,
  "net_amount_ex_tax"   bigint,                                   -- AMEGO TotalAmount（未稅合計）
  "tax_amount"          bigint,                                   -- AMEGO TaxAmount（整數）
  "gross_amount"        bigint,                                   -- ＝ net_amount_ex_tax + tax_amount，用於與退款金額對帳
  "void_reason_code"    varchar(32),                              -- AMEGO f0501 CancelReason 對應
  "void_reason_detail"  text,
  "blocked_reason_code" varchar(32),                              -- 「送不出去」的理由碼，值域與 invoice_documents.manual_reason_code 同源
  "cancel_reason_code"  varchar(48),                              -- 「不做了」的理由碼（§7.3.7 G05／G07）
  "provider_confirmed_at"    timestamptz,
  "provider_upload_status"   smallint,                            -- AMEGO allowance_status / invoice_status
  "provider_upload_checked_at" timestamptz,
  "pdf_object_key"      varchar(512),
  "version"             bigint NOT NULL DEFAULT 0,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_adjustments_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invadj_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "chk_invadj_type"   CHECK ("adjustment_type" IN ('VOID','ALLOWANCE')),
  CONSTRAINT "chk_invadj_status" CHECK ("status" IN ('PENDING','BLOCKED_UNKNOWN_DOCUMENT','BLOCKED_PENDING_CORRECTION','SENDING','WAITING_CONFIRMATION','COMPLETED','FAILED_PERMANENT','FAILED_UNKNOWN','CANCELLED')),
  CONSTRAINT "chk_invadj_blocked_reason" CHECK ("status" <> 'FAILED_PERMANENT' OR "blocked_reason_code" IS NOT NULL),
  CONSTRAINT "chk_invadj_trigger" CHECK ("trigger_kind" IN ('ORDER_REFUND','BUYER_CORRECTION','ADM_MANUAL')),
  CONSTRAINT "chk_invadj_refund_pair" CHECK (("trigger_kind" = 'ORDER_REFUND') = ("refund_no" IS NOT NULL)),
  CONSTRAINT "chk_invadj_correction_pair" CHECK (("trigger_kind" = 'BUYER_CORRECTION') = ("correction_request_id" IS NOT NULL)),
  CONSTRAINT "chk_invadj_allowance_shape" CHECK (
    ("adjustment_type" = 'ALLOWANCE' AND "allowance_number" IS NOT NULL AND "allowance_date_taipei" IS NOT NULL AND "net_amount_ex_tax" IS NOT NULL AND "tax_amount" IS NOT NULL AND "gross_amount" IS NOT NULL)
    OR ("adjustment_type" = 'VOID' AND "allowance_number" IS NULL AND "net_amount_ex_tax" IS NULL AND "tax_amount" IS NULL AND "gross_amount" IS NULL)),
  CONSTRAINT "chk_invadj_allowance_sum" CHECK ("gross_amount" IS NULL OR "gross_amount" = "net_amount_ex_tax" + "tax_amount"),
  CONSTRAINT "chk_invadj_allowance_positive" CHECK ("gross_amount" IS NULL OR ("gross_amount" > 0 AND "net_amount_ex_tax" >= 0 AND "tax_amount" >= 0)),
  CONSTRAINT "chk_invadj_allowance_no_len" CHECK ("allowance_number" IS NULL OR length("allowance_number") <= 16),   -- AMEGO AllowanceNumber 上限 16 字
  CONSTRAINT "chk_invadj_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invadj_allowance_refund" ON "invoice_adjustments" ("document_id", "refund_no")
  WHERE "adjustment_type" = 'ALLOWANCE' AND "refund_no" IS NOT NULL AND "status" <> 'CANCELLED';
CREATE UNIQUE INDEX "uq_invadj_void_once" ON "invoice_adjustments" ("document_id")
  WHERE "adjustment_type" = 'VOID' AND "status" <> 'CANCELLED';
CREATE UNIQUE INDEX "uq_invadj_allowance_number" ON "invoice_adjustments" ("allowance_number") WHERE "allowance_number" IS NOT NULL;

CREATE TABLE "invoice_adjustment_lines" (
  "id"                 bigserial NOT NULL,
  "adjustment_id"      bigint NOT NULL,
  "line_seq"           integer NOT NULL,
  "source_line_id"     bigint,                              -- 對應 invoice_document_lines.id（allocation 依據）
  "original_invoice_no"     varchar(10) NOT NULL,           -- AMEGO g0401 要求逐行回填原發票資訊
  "original_invoice_date"   date NOT NULL,
  "original_description"    varchar(256) NOT NULL,
  "quantity"           integer NOT NULL,
  "unit_price_ex_tax"  bigint NOT NULL,                     -- 未稅單價（金額語意與 invoice_document_lines 相反）
  "net_amount_ex_tax"  bigint NOT NULL,                     -- ＝ unit_price_ex_tax × quantity
  "tax_amount"         bigint NOT NULL,                     -- AMEGO ProductItem.Tax，必須整數（4040139）
  "tax_category"       smallint NOT NULL DEFAULT 1,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_adjustment_lines_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invadjline_adjustment" FOREIGN KEY ("adjustment_id") REFERENCES "invoice_adjustments" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_invadjline_source" FOREIGN KEY ("source_line_id") REFERENCES "invoice_document_lines" ("id"),
  CONSTRAINT "chk_invadjline_amount" CHECK ("net_amount_ex_tax" = "unit_price_ex_tax" * "quantity"),
  CONSTRAINT "chk_invadjline_qty" CHECK ("quantity" > 0),
  CONSTRAINT "chk_invadjline_tax_category" CHECK ("tax_category" IN (1,2,3)),
  CONSTRAINT "chk_invadjline_nonneg" CHECK ("net_amount_ex_tax" >= 0 AND "tax_amount" >= 0),
  CONSTRAINT "chk_invadjline_seq" CHECK ("line_seq" BETWEEN 1 AND 9999)
);

CREATE UNIQUE INDEX "uq_invadjline_seq" ON "invoice_adjustment_lines" ("adjustment_id", "line_seq");
CREATE INDEX "idx_invadj_document" ON "invoice_adjustments" ("document_id", "created_at");  -- B05 折讓連回原發票、原發票反查折讓
-- ADM 異常工作台。兩個 BLOCKED_* 必須在述詞內，漏掉等於把卡住的退款折讓藏起來
CREATE INDEX "idx_invadj_status" ON "invoice_adjustments" ("status", "created_at")
  WHERE ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'BLOCKED_UNKNOWN_DOCUMENT'::character varying, 'BLOCKED_PENDING_CORRECTION'::character varying, 'SENDING'::character varying, 'WAITING_CONFIRMATION'::character varying, 'FAILED_UNKNOWN'::character varying])::text[]));
CREATE INDEX "idx_invadjline_source" ON "invoice_adjustment_lines" ("source_line_id") WHERE "source_line_id" IS NOT NULL;  -- 「這一行被折讓了多少」的反查

-- ---------------------------------------------------------------------------
-- §5.1.8 invoice_operations｜對 provider 的一次操作（＝排程 job 表）
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_operations" (
  "id"                  bigint NOT NULL,                        -- TSID（ADM 異常工作台會顯示它）
  "document_id"         bigint NOT NULL,
  "adjustment_id"       bigint,                                 -- VOID/ALLOWANCE/VOID_ALLOWANCE 指向它；ISSUE/QUERY/REISSUE 為 NULL
  "operation_type"      varchar(16) NOT NULL,                   -- CodeMeta InvoiceOperationType，6 值見 chk_invop_type
  "operation_key"       varchar(96) NOT NULL,                   -- 冪等鍵（§5.3.2）。全域唯一，重試不換鍵
  "provider_request_no" varchar(40) NOT NULL,                   -- 送給 AMEGO 的商家單號（＝document.provider_request_no）
  "status"              varchar(28) NOT NULL DEFAULT 'READY',   -- CodeMeta InvoiceOperationStatus，20 值見 chk_invop_status
  "request_snapshot_hash" varchar(64) NOT NULL,                 -- 送出內容的雜湊：不一樣代表有人改了快照
  "attempt_count"       integer NOT NULL DEFAULT 0,
  "max_attempts"        integer NOT NULL DEFAULT 13,
  "next_attempt_at"     timestamptz NOT NULL DEFAULT NOW(),       -- 同時是「下次執行時間」與「租約」；claim 把它推到未來即取得所有權
  "held_until"          timestamptz,                            -- HELD 的到期時間（買家送出付款後修改 ⇒ now + 60s）
  "recheck_list_page"   smallint NOT NULL DEFAULT 0,            -- invoice_list 的分頁游標（一次 claim 只做一次外部呼叫）
  "statutory_deadline_at" timestamptz,                          -- 反正規化副本，權威值在 invoice_documents（I-52）
  -- 🔴 **刻意不使用，不要接上去**（#3026 收尾稽查，2026-09-03）。
  -- 每一次 AMEGO 往返的 HTTP status 權威落點是 `invoice_provider_audit_logs.http_status`，
  -- 那張表**已經 3488/3488 全部有值**（dev 實測）且它的存在目的就是對外舉證。
  -- 本欄從建表至今零寫入、零消費者（proto 無對應欄位），是同一份事實的第二個落點
  -- ⇒ 接上去只會製造「兩個都說是 HTTP status、而其中一個會漂移」的問題。
  -- 保留欄位而不 DROP：丟一個可空欄位的收益低於一次破壞性 migration 的 rollout 窗口
  -- （migrate 在 deploy 之前跑，舊 pod 的 SELECT 會在那段時間炸）。
  "last_http_status"    integer,
  "provider_code"       varchar(16),                            -- AMEGO code（實測可能是 String，例 '3040171'）
  "provider_message"    text,
  "unknown_result"      boolean NOT NULL DEFAULT false,         -- 結果不確定（A4 受控人工出口的入口條件）
  "unknown_since"       timestamptz,
  "cancel_requested_by"     bigint,                             -- ADM 取消，雙人確認
  "cancel_requested_at"     timestamptz,
  "cancel_confirmed_by"     bigint,
  "cancel_confirmed_at"     timestamptz,
  "cancel_evidence_kind"       varchar(48),                     -- CodeMeta InvoiceCancelEvidenceKind（4 值），§6.4
  "cancel_evidence_object_key" text,                            -- §25：只存 objectKey，不存完整 URL
  "cancel_reason_code"      varchar(48),                        -- CodeMeta InvoiceCancelReason（4 值）
  "cancel_reason"           text,
  "started_at"          timestamptz,
  "completed_at"        timestamptz,
  "last_error"          text,
  "version"             bigint NOT NULL DEFAULT 0,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_operations_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invop_document"   FOREIGN KEY ("document_id")   REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invop_adjustment" FOREIGN KEY ("adjustment_id") REFERENCES "invoice_adjustments" ("id"),
  CONSTRAINT "chk_invop_type" CHECK ("operation_type" IN ('ISSUE','QUERY','VOID','ALLOWANCE','REISSUE','VOID_ALLOWANCE')),
  CONSTRAINT "chk_invop_status" CHECK ("status" IN ('CREATED','AWAITING_TRIGGER','READY','HELD','SENDING','RETRY_SCHEDULED','WAITING_CONFIRMATION','PROVIDER_STATUS_RECHECK','SUCCEEDED','FAILED_PERMANENT','FAILED_RETRY_EXHAUSTED','FAILED_UNKNOWN','SELLER_ACTION_REQUIRED','CANCEL_REQUESTED','CANCEL_BLOCKED_UNKNOWN','CANCELLED_BY_ADMIN','MANUAL_PROCESS_ALLOWED','RESOLVED_EXTERNALLY','NOT_APPLICABLE','CANCELLED_BEFORE_ISSUE')),
  -- 三種調整類 operation 都要掛 adjustment_id（漏掉 VOID_ALLOWANCE ⇒ 折讓作廢寫不進來）
  CONSTRAINT "chk_invop_adjustment_pair" CHECK (("operation_type" IN ('VOID','ALLOWANCE','VOID_ALLOWANCE')) = ("adjustment_id" IS NOT NULL)),
  CONSTRAINT "chk_invop_held_pair" CHECK (("status" = 'HELD') = ("held_until" IS NOT NULL)),   -- HELD 必有到期時間，否則它就是永久旗標
  CONSTRAINT "chk_invop_recheck_page" CHECK ("recheck_list_page" >= 0),
  CONSTRAINT "chk_invop_attempts" CHECK ("attempt_count" >= 0 AND "max_attempts" >= 1),
  CONSTRAINT "chk_invop_unknown_pair" CHECK (("unknown_result" = false) = ("unknown_since" IS NULL)),
  CONSTRAINT "chk_invop_two_person" CHECK ("cancel_confirmed_by" IS NULL OR ("cancel_requested_by" IS NOT NULL AND "cancel_confirmed_by" <> "cancel_requested_by")),
  CONSTRAINT "chk_invop_cancel_pair" CHECK (("cancel_requested_by" IS NULL) = ("cancel_requested_at" IS NULL)),
  -- CANCELLED_BY_ADMIN 有兩條合法來源，兩者的「憑什麼相信沒開出去」不同（§6.4）：
  --   J28 ADM 雙人確認（cancel_confirmed_by 非空，且 chk_invop_two_person 保證兩人不同）
  --   J24 INV.rc 自動落地——**沒有第二個人**，合法性完全來自 provider 出具的證據
  -- 舊版只認第一條 ⇒ J24 必定 23514（它的觸發條件正是 cancel_confirmed_at IS NULL）。
  -- 🔴 只有前兩個 kind 能免人工：後兩值（賣家截圖／對帳檔）需要人判讀，放行它們
  --    等於讓賣家自己提供的截圖就能取消一張票。
  -- 🔴 第三個 disjunct 必須是 NULL-safe（2026-09-05 修）。
  -- 原本寫 `"cancel_evidence_kind" IN (...)`：當狀態是 CANCELLED_BY_ADMIN 且兩欄皆 NULL 時
  -- 整條運算式是 `FALSE OR FALSE OR NULL` = **NULL**，而 PostgreSQL 的 CHECK 只拒絕 FALSE、
  -- **放行 NULL** ⇒ 這道約束對它要擋的那一格是 no-op。`convalidated = t`，所以不是
  -- 「沒驗過的舊資料」，是它從第一天起就沒有在擋。
  -- dev 實證（op 7500463275386077184，2026-09-05）：J24 落 CANCELLED_BY_ADMIN 而兩欄皆 NULL，
  -- 靜默通過 ⇒ §6.4 的「人工出口四樣紀律」少了證據那一樣，且沒有任何東西會出聲。
  CONSTRAINT "chk_invop_admin_cancel_needs_confirm" CHECK ("status" <> 'CANCELLED_BY_ADMIN' OR "cancel_confirmed_by" IS NOT NULL OR COALESCE("cancel_evidence_kind", '') IN ('PROVIDER_QUERY_71', 'PROVIDER_LIST_ABSENT')),
  -- objectKey 只有需要人判讀的兩種證據才有；provider 直接作證的兩值沒有檔案。
  CONSTRAINT "chk_invop_evidence_object_key" CHECK ("cancel_evidence_object_key" IS NULL OR "cancel_evidence_kind" IN ('MOF_PLATFORM_SCREENSHOT', 'AMEGO_RECONCILE_FILE')),
  CONSTRAINT "chk_invop_request_no_len" CHECK (length("provider_request_no") <= 40),
  CONSTRAINT "chk_invop_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invop_operation_key" ON "invoice_operations" ("operation_key");
CREATE UNIQUE INDEX "uq_invop_open_adjustment_per_document" ON "invoice_operations" ("document_id") WHERE (((operation_type)::text = ANY ((ARRAY['VOID'::character varying, 'REISSUE'::character varying, 'ALLOWANCE'::character varying, 'VOID_ALLOWANCE'::character varying])::text[])) AND ((status)::text <> ALL ((ARRAY['SUCCEEDED'::character varying, 'NOT_APPLICABLE'::character varying, 'CANCELLED_BEFORE_ISSUE'::character varying, 'RESOLVED_EXTERNALLY'::character varying, 'CANCELLED_BY_ADMIN'::character varying])::text[])));
-- PostgreSQL 不會為 FK 自動建索引，缺這兩條就是每次全表掃
CREATE INDEX "idx_invop_document" ON "invoice_operations" ("document_id", "created_at" DESC);
CREATE INDEX "idx_invop_adjustment" ON "invoice_operations" ("adjustment_id", "created_at" DESC) WHERE "adjustment_id" IS NOT NULL;

-- ---------------------------------------------------------------------------
-- §5.1.9 invoice_inbox_events｜入向事件
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_inbox_events" (
  "id"               bigserial NOT NULL,
  "event_id"         varchar(64) NOT NULL,                        -- producer 產生的 UUID v4
  "event_type"       varchar(64) NOT NULL,                        -- CodeMeta EventType（UPPER_SNAKE），非 routing key
  "routing_key"      varchar(128) NOT NULL,
  "exchange"         varchar(64) NOT NULL,
  "business_dedup_key" varchar(160) NOT NULL,                     -- 業務層去重鍵，不一定等於 event_id（上游可能每次換新 UUID）
  "aggregate_ref"    varchar(64),                                 -- order_no / refund_no，便於 ADM 反查
  "payload"          jsonb NOT NULL,
  "status"           varchar(16) NOT NULL DEFAULT 'RECEIVED',     -- RECEIVED → DEFERRED（前置條件未到）→ PROCESSED / SKIPPED / FAILED
  "disposition"      varchar(40),                                 -- 處置決定（SKIPPED 時必填），例 IGNORED_DUPLICATE_PAYMENT
  "disposition_reason" text,
  "attempt_count"    integer NOT NULL DEFAULT 0,
  "next_attempt_at"  timestamptz NOT NULL DEFAULT NOW(),            -- 兼租約欄（《多副本排程搶佔規範》）
  "deferred_until"   timestamptz,
  "last_error"       text,
  "occurred_at"      timestamptz NOT NULL,                        -- 事件自帶的業務時間，不可與 received_at 混用
  "received_at"      timestamptz NOT NULL DEFAULT NOW(),
  "processed_at"     timestamptz,
  "payload_purged_at" timestamptz,                                -- payload 清理標記（§5.4.6）。本表永不刪列
  CONSTRAINT "invoice_inbox_events_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invinbox_status" CHECK ("status" IN ('RECEIVED','DEFERRED','PROCESSED','SKIPPED','FAILED')),
  CONSTRAINT "chk_invinbox_skip_reason" CHECK ("status" <> 'SKIPPED' OR "disposition" IS NOT NULL),
  CONSTRAINT "chk_invinbox_attempts" CHECK ("attempt_count" >= 0)
);

CREATE UNIQUE INDEX "uq_invinbox_event_id" ON "invoice_inbox_events" ("event_id");                                   -- 傳輸層去重
CREATE UNIQUE INDEX "uq_invinbox_business_key" ON "invoice_inbox_events" ("event_type", "business_dedup_key");       -- 業務層去重

-- ---------------------------------------------------------------------------
-- §5.1.9a invoice_cancelled_orders｜訂單已取消（買家面 state 的終態事實）
--
-- 為什麼需要一張表：端點 8 在「這張訂單沒有任何 invoice_documents 列」時，本來一律
-- 回 state=AWAITING_PAYMENT ＋「付款完成後會立即開立發票」。那句話對**已取消**的訂單
-- 永遠不會成真（它不會再被付款），而 invoice-service 的讀取路徑看不到訂單狀態
-- ⇒ 需要一個由 `order.cancelled` 落地、讀路徑查得到的本地事實。#2904。
--
-- 🔴 **buyer_id 不是裝飾欄，它是 Tier A 的授權述詞**（§8.1.4／E21）。讀路徑必須
--    `WHERE order_no = $1 AND buyer_id = $2`：只用 order_no 查 ⇒ 任何登入買家拿別人的
--    order_no 來問都會得到「這張訂單存在且已取消」，而訂單號是可枚舉的
--    ⇒ 憑空多出一個訂單存在性 oracle。同一支端點的另一半
--    （ListOrderDocumentsForBuyer）本來就雙欄過濾，這一半不跟上就是把它繞開。
--
-- 🔴 **本表只記「取消」這個事實，不記「不會開發票」這個結論**：後者是讀路徑依
--    「有沒有 document」現算的（先付款開票、事後才取消的訂單有 document，state 由
--    document status 決定，不看本表）。把結論寫進表 ⇒ 兩個地方各自為政，而它們
--    只在退款／作廢流程上才會分歧，正是最難測到的那條路徑。
--
-- 沒有 status 欄、沒有時間排程欄：本表是 append-only 的事實記錄，不是候選集合
-- ⇒ 沒有任何 worker 掃它，也就不存在「卡在某個狀態」的形狀。
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_cancelled_orders" (
  "order_no"      varchar(64) NOT NULL,
  "buyer_id"      bigint NOT NULL,                       -- 取自 order.cancelled payload 的 buyer_id（＝ orders.buyer_id，營運鍵非 account_id）
  "cancel_reason" varchar(32),                           -- OrderCancelReason.value()，買家自願取消時 producer 不寫 ⇒ 可為 NULL
  "occurred_at"   timestamptz NOT NULL,                  -- 事件自帶的業務時間，不可與 created_at 混用
  "created_at"    timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_cancelled_orders_pkey" PRIMARY KEY ("order_no"),
  CONSTRAINT "chk_invcanord_buyer" CHECK ("buyer_id" > 0)
);

-- 讀路徑的述詞是 (order_no, buyer_id)；PK 只有 order_no ⇒ 補一支涵蓋兩欄的索引，
-- 讓授權述詞本身就走索引而不是「先用 PK 撈出來再比對 buyer_id」。
CREATE INDEX "idx_invcanord_order_buyer" ON "invoice_cancelled_orders" ("order_no", "buyer_id");

-- ---------------------------------------------------------------------------
-- §5.1.10 invoice_outbox_events｜出向事件
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_outbox_events" (
  "id"                 bigserial NOT NULL,
  "aggregate_type"     varchar(24) NOT NULL,                              -- DOCUMENT / ADJUSTMENT / COMPLAINT / TAX_PROFILE
  "aggregate_id"       bigint NOT NULL,                                   -- 與 aggregate_type 成對，誰讀它要一起回答
  "event_type"         varchar(64) NOT NULL,                              -- CodeMeta InvoiceEventType
  "exchange"           varchar(64) NOT NULL DEFAULT 'hawkr.invoice',
  "routing_key"        varchar(128) NOT NULL,
  "payload"            jsonb NOT NULL,                                    -- 必須在 enqueue 當下（同一交易內）就快照完成
  "status"             varchar(16) NOT NULL DEFAULT 'pending',            -- pending / done / failed（'failed' 已不由程式寫入，見下）
  "attempt_count"      integer NOT NULL DEFAULT 0,
  "next_attempt_at"    timestamptz NOT NULL DEFAULT NOW(),                  -- 兼租約欄
  "last_error"         text,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  "completed_at"       timestamptz,
  CONSTRAINT "invoice_outbox_events_pkey" PRIMARY KEY ("id"),
  -- 🔴 'failed' 仍在值域內，但**沒有任何程式路徑會寫它**（2026-08-31 起）：耗盡重試走
  --    「停泊」（domain.OutboxRepository.Park：status 維持 'pending'、next_attempt_at 推 1 小時）。
  --    原因是本表的候選述詞與 idx_invoutbox_pending 全是 status='pending' 且沒有任何時間欄
  --    能讓 'failed' 回到候選集合 ⇒ 那是永久旗標；更糟的是 CountPending／OldestPending 的述詞
  --    同樣是 'pending' ⇒ 一列落到 'failed' 就退出 outbox_oldest_pending_age_seconds ⇒
  --    告警 InvoiceOutboxLag 會在事件真正卡死那一刻由 firing 轉 resolved。
  --    值域保留是刻意的：'failed' 現在的唯一用途是**維運 SQL 隔離**一列毒 payload，
  --    而 M31 invoice_outbox_stranded_rows 會立刻把它數出來（結構上沒有執行者）。
  CONSTRAINT "chk_invoutbox_status" CHECK ("status" IN ('pending','done','failed')),
  CONSTRAINT "chk_invoutbox_attempts" CHECK ("attempt_count" >= 0)
);

-- ---------------------------------------------------------------------------
-- §5.1.11 invoice_correction_requests｜買家付款後修改發票資料
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_correction_requests" (
  "id"                 bigint NOT NULL,                             -- TSID
  "document_id"        bigint NOT NULL,
  "requested_by_buyer_id" bigint NOT NULL,
  "expected_snapshot_version" integer NOT NULL,                     -- 樂觀鎖：買家送出時必須帶 expectedSnapshotVersion
  "before_payload"     jsonb NOT NULL,                              -- 已遮罩：密文欄位存 hash 與遮罩值，不存明文
  "after_payload"      jsonb NOT NULL,
  "status"             varchar(32) NOT NULL DEFAULT 'REQUESTED',    -- CodeMeta InvoiceCorrectionStatus，6 值見 chk_invcorr_status
  "handling_mode"      varchar(24),                                 -- UPDATE_SNAPSHOT / VOID_AND_REISSUE
  "reject_reason_code" varchar(32),                                 -- 婉拒理由碼，7 值見 chk_invcorr_reject_reason_code
  "reject_reason_detail" text,
  "void_adjustment_id" bigint,                                      -- VOID_AND_REISSUE ⇒ 對應的作廢
  "reissued_document_id" bigint,                                    -- 重開出來的新文件
  "version"            bigint NOT NULL DEFAULT 0,
  "created_at"         timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"         timestamptz NOT NULL DEFAULT NOW(),
  "completed_at"       timestamptz,
  CONSTRAINT "invoice_correction_requests_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcorr_document"  FOREIGN KEY ("document_id")          REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invcorr_void"      FOREIGN KEY ("void_adjustment_id")   REFERENCES "invoice_adjustments" ("id"),
  CONSTRAINT "fk_invcorr_reissued"  FOREIGN KEY ("reissued_document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "chk_invcorr_status" CHECK ("status" IN ('REQUESTED','REJECTED','UPDATED_BEFORE_ISSUE','VOID_REISSUE_IN_PROGRESS','COMPLETED','REISSUE_FAILED')),
  CONSTRAINT "chk_invcorr_mode" CHECK ("handling_mode" IS NULL OR "handling_mode" IN ('UPDATE_SNAPSHOT','VOID_AND_REISSUE')),
  CONSTRAINT "chk_invcorr_reject_reason" CHECK ("status" <> 'REJECTED' OR "reject_reason_code" IS NOT NULL),
  CONSTRAINT "chk_invcorr_reject_reason_code" CHECK ("reject_reason_code" IS NULL OR "reject_reason_code" IN ('ALREADY_REPORTED','PAST_VOID_DEADLINE','ALLOWANCE_EXISTS','TRACK_INSUFFICIENT','CARRIER_INVALID','PROVIDER_UNAVAILABLE','PROVIDER_STATE_CONFLICT')),
  CONSTRAINT "chk_invcorr_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invcorr_open_per_document" ON "invoice_correction_requests" ("document_id") WHERE ((status)::text = ANY ((ARRAY['REQUESTED'::character varying, 'VOID_REISSUE_IN_PROGRESS'::character varying])::text[]));

-- ---------------------------------------------------------------------------
-- §5.1.12 invoice_complaints ＋ invoice_complaint_events｜買家申訴與 3 工作天 SLA
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_complaints" (
  "id"                  bigint NOT NULL,                          -- TSID（對外＝案件編號）
  "document_id"         bigint NOT NULL,
  "issuer_id"           bigint NOT NULL,                          -- 反正規化：ADM 佇列與違規計數都按 issuer 篩
  "order_no"            varchar(64),                              -- 違規計數的去重鍵（同一訂單多次申訴只算一次）
  "complainant_buyer_id" bigint NOT NULL,
  "reason_code"         varchar(32) NOT NULL DEFAULT 'NOT_RECEIVED',
  "description"         text,
  "status"              varchar(32) NOT NULL DEFAULT 'OPEN',      -- CodeMeta InvoiceComplaintStatus，7 值見 chk_invcomp_status
  "sla_due_at"          timestamptz NOT NULL,                     -- 3 工作天。由 invoice_business_day_mirror ＋ pkg/bizcalendar 純函式在建案當下算好落庫
  "overdue_at"          timestamptz,                              -- 逾期時點（K05 的計時起點）。不是 claim 旗標、不是「已通知」旗標
  "buyer_review_due_at" timestamptz,                              -- 買家異議期（K02 寫 now + 72h；K06 到期視為接受 → RESOLVED）
  "first_responded_at"  timestamptz,
  "resolved_at"         timestamptz,
  "resolution_code"     varchar(32),                              -- SELLER_ISSUED / PLATFORM_REISSUED / NO_VIOLATION / VIOLATION
  "resolution_note"     text,
  "reopen_count"        smallint NOT NULL DEFAULT 0,
  "reopen_allowed_after" timestamptz,                             -- 重開冷卻
  "decided_by_account_id" bigint,
  "decided_at"          timestamptz,
  "version"             bigint NOT NULL DEFAULT 0,
  "created_at"          timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"          timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_complaints_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcomp_document" FOREIGN KEY ("document_id") REFERENCES "invoice_documents" ("id"),
  CONSTRAINT "fk_invcomp_issuer"   FOREIGN KEY ("issuer_id")   REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_invcomp_status" CHECK ("status" IN ('OPEN','SELLER_RESPONDED','OVERDUE','ESCALATED_TO_ADM','RESOLVED','VIOLATION_CONFIRMED','WITHDRAWN')),
  CONSTRAINT "chk_invcomp_overdue_pair" CHECK ("status" <> 'OVERDUE' OR "overdue_at" IS NOT NULL),
  CONSTRAINT "chk_invcomp_reopen" CHECK ("reopen_count" BETWEEN 0 AND 2),
  CONSTRAINT "chk_invcomp_resolved_pair" CHECK (("status" IN ('RESOLVED','VIOLATION_CONFIRMED')) = ("resolved_at" IS NOT NULL)),
  CONSTRAINT "chk_invcomp_violation_needs_decider" CHECK ("status" <> 'VIOLATION_CONFIRMED' OR "decided_by_account_id" IS NOT NULL),
  CONSTRAINT "chk_invcomp_version_nonneg" CHECK ("version" >= 0)
);

CREATE UNIQUE INDEX "uq_invcomp_open_per_document" ON "invoice_complaints" ("document_id") WHERE ((status)::text = ANY ((ARRAY['OPEN'::character varying, 'OVERDUE'::character varying, 'SELLER_RESPONDED'::character varying, 'ESCALATED_TO_ADM'::character varying, 'RESOLVED'::character varying])::text[]));

CREATE TABLE "invoice_complaint_events" (
  "id"              bigserial NOT NULL,
  "complaint_id"    bigint NOT NULL,
  "event_type"      varchar(32) NOT NULL,                         -- CodeMeta InvoiceComplaintEventType（7 值）
  "action"          varchar(24),                                  -- CodeMeta InvoiceComplaintAction 的 6 個生成常數（I-60）
  "actor_type"      varchar(16) NOT NULL,                         -- BUYER / SELLER / ADMIN / SYSTEM
  "actor_account_id" bigint,
  "message"         text,
  "attachment_object_keys" jsonb NOT NULL DEFAULT '[]',
  "occurred_at"     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_complaint_events_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invcompev_complaint" FOREIGN KEY ("complaint_id") REFERENCES "invoice_complaints" ("id") ON DELETE CASCADE,
  CONSTRAINT "chk_invcompev_actor" CHECK ("actor_type" IN ('BUYER','SELLER','ADMIN','SYSTEM')),
  CONSTRAINT "chk_invcompev_actor_id" CHECK ("actor_type" = 'SYSTEM' OR "actor_account_id" IS NOT NULL)
);

CREATE INDEX "idx_invcompev_complaint" ON "invoice_complaint_events" ("complaint_id", "occurred_at");
-- S09 賣家申訴分頁 ＋ A07 ADM 佇列（兩份 UI 分頁都是對狀態值域的查詢視圖，不各自維護狀態欄）
CREATE INDEX "idx_invcomp_issuer_status" ON "invoice_complaints" ("issuer_id", "status", "created_at" DESC);
CREATE INDEX "idx_invcomp_violation" ON "invoice_complaints" ("issuer_id", "resolved_at") WHERE "status" = 'VIOLATION_CONFIRMED';  -- 停權門檻計算

-- ---------------------------------------------------------------------------
-- §5.1.13 稽核三表（invoice_status_history／invoice_provider_audit_logs／invoice_admin_audit_logs）
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_status_history" (
  "id"              bigserial NOT NULL,
  "subject_type"    varchar(24) NOT NULL,                         -- 7 值見 chk_invhist_subject
  "subject_id"      bigint NOT NULL,
  "from_status"     varchar(32),                                  -- NULL ＝ 建立
  "to_status"       varchar(32) NOT NULL,
  "reason_code"     varchar(48),
  "reason_detail"   text,
  "actor_type"      varchar(16) NOT NULL,                         -- SYSTEM / ADMIN / SELLER / BUYER / PROVIDER
  "actor_account_id" bigint,
  "correlation_id"  varchar(64),                                  -- 對齊 pkg/middleware/grpc/interceptor 的 correlation_id
  "occurred_at"     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_status_history_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invhist_subject" CHECK ("subject_type" IN ('DOCUMENT','ADJUSTMENT','OPERATION','CORRECTION_REQUEST','TAX_PROFILE','PROVIDER_ACCOUNT','COMPLAINT')),
  CONSTRAINT "chk_invhist_actor" CHECK ("actor_type" IN ('SYSTEM','ADMIN','SELLER','BUYER','PROVIDER'))
);

CREATE INDEX "idx_invhist_subject" ON "invoice_status_history" ("subject_type", "subject_id", "occurred_at");

CREATE TABLE "invoice_provider_audit_logs" (
  "id"                bigserial NOT NULL,
  "operation_id"      bigint,
  "issuer_id"         bigint NOT NULL,
  "provider"          varchar(16) NOT NULL DEFAULT 'AMEGO',
  "endpoint"          varchar(64) NOT NULL,                       -- 例 '/json/f0401'
  "attempt_seq"       integer NOT NULL,
  "request_masked"    jsonb NOT NULL,                             -- 必須先遮罩再落庫
  "response_masked"   jsonb,
  "raw_body_hash"     varchar(64) NOT NULL,                       -- 遮罩前 canonical body 的 SHA-256（不可逆，可比對重送）
  "http_status"       integer,
  "provider_code"     varchar(16),
  -- #3089 第 7 項：與 provider_code 成對的訊息原文。proto 的
  -- ProviderExchangeItem.provider_message 逐字要求「必須同時保存 msg」——
  -- AMEGO 的同一個數字碼在官方表出現兩次語意不同（53 是最明確的例子），
  -- 只存 code 的話 ADM 分不出是哪一種。原文本來就在 response_masked 裡，
  -- 但那是一團 jsonb，要在列表上一眼看到必須有獨立欄。
  "provider_message"  text,
  "latency_ms"        integer,
  "error_kind"        varchar(32),                                -- TIMEOUT / CONNECTION / PROVIDER_ERROR / DECODE
  "correlation_id"    varchar(64),                                -- ADM 稽核 UI 要一鍵對到 Loki 與 Tempo
  "occurred_at"       timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_provider_audit_logs_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invpaudit_operation" FOREIGN KEY ("operation_id") REFERENCES "invoice_operations" ("id"),
  CONSTRAINT "chk_invpaudit_attempt" CHECK ("attempt_seq" >= 1),
  CONSTRAINT "chk_invpaudit_latency" CHECK ("latency_ms" IS NULL OR "latency_ms" >= 0)
);

CREATE INDEX "idx_invpaudit_operation" ON "invoice_provider_audit_logs" ("operation_id", "attempt_seq");
CREATE INDEX "idx_invpaudit_issuer_time" ON "invoice_provider_audit_logs" ("issuer_id", "occurred_at" DESC);

CREATE TABLE "invoice_admin_audit_logs" (
  "id"                bigserial NOT NULL,
  "actor_account_id"  bigint NOT NULL,
  "action"            varchar(64) NOT NULL,                       -- 例 'INVOICE_OPERATION_CANCEL'
  "resource_type"     varchar(32) NOT NULL,
  "resource_id"       bigint NOT NULL,
  "before_state"      jsonb,
  "after_state"       jsonb,
  "reason_code"       varchar(48) NOT NULL,                       -- 強制填理由：NOT NULL 就是那個強制
  "reason_detail"     text NOT NULL,
  "approval_actor_account_id" bigint,                             -- 雙人確認時的第二人
  "trace_id"          varchar(64),
  "request_id"        varchar(64),
  "correlation_id"    varchar(64),
  "client_ip"         varchar(45),                                -- IPv6 最長 45 字元
  "user_agent"        varchar(512),
  "occurred_at"       timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_admin_audit_logs_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "chk_invadmaudit_reason" CHECK (length("reason_detail") >= 1),
  CONSTRAINT "chk_invadmaudit_two_person" CHECK ("approval_actor_account_id" IS NULL OR "approval_actor_account_id" <> "actor_account_id")
);

CREATE INDEX "idx_invadmaudit_resource" ON "invoice_admin_audit_logs" ("resource_type", "resource_id", "occurred_at" DESC);
CREATE INDEX "idx_invadmaudit_actor" ON "invoice_admin_audit_logs" ("actor_account_id", "occurred_at" DESC);
CREATE UNIQUE INDEX "uq_invadmaudit_client_request" ON "invoice_admin_audit_logs" ("actor_account_id", "action", "request_id")
  WHERE "request_id" IS NOT NULL;

-- ---------------------------------------------------------------------------
-- §5.1.14 invoice_business_day_mirror｜台灣國定假日唯讀鏡像
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_business_day_mirror" (
  "holiday_date"  date NOT NULL,
  "synced_at"     timestamptz NOT NULL,   -- 整段覆蓋 ⇒ 同一批的每一列 synced_at 相同
  CONSTRAINT "invoice_business_day_mirror_pkey" PRIMARY KEY ("holiday_date")
);

COMMENT ON TABLE invoice_business_day_mirror IS
  '台灣國定假日唯讀鏡像。owner 是 shipping-service 的 holidays 表；本表每日由 INV.gov 整段覆蓋。禁止人工寫入、禁止 ADM 編輯、禁止在本服務加 seed migration。';

-- ---------------------------------------------------------------------------
-- §5.1.15 invoice_provider_cutovers｜自動開立切點歷史（端點 46）
--
-- 為什麼是獨立表而不是靠稽核表回放：`invoice_provider_accounts` 只存**現值**
-- （automation_cutover_at ＋ version），改一次就把上一次抹掉；而端點 46 要回的是
-- 「這個賣家什麼時候開始／停止走自動開立」的**清單**。用 invoice_admin_audit_logs
-- 回放不成立——那張表只收 ADM 動作，而切點是**賣家**在端點 26 設的。
--
-- 🔴 本表只記錄「自動開立切點」這一件事，`mode` 的值域因此只有一個值。第二個值一旦
--    出現就代表本表的語意變了（例如要兼記 LEGACY_MANUAL 的回退），那是設計書層級的
--    改動，不得靠加一個 CHECK 值順手完成。撤銷／停用**不新增列**，而是把當前那一列的
--    `deactivated_at` 補上；新訂單回 LEGACY_MANUAL 由 §7.1.3 決策樹判定，不由本表承載。
--
-- ⚠️ `created_by_account_id` 在賣家發起的切點上填的是 **store surrogate**，不是登入
--    principal——與 `invoice_status_history.actor_account_id` 同一個既有慣例（§6 身分
--    分離：賣家路徑的營運鍵是 store_id）。欄名沿用 proto `ProviderCutoverItem.created_by_account_id`。
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_provider_cutovers" (
  "id"                    bigserial NOT NULL,                     -- 內部歷史表 ⇒ BIGSERIAL（§7）
  "provider_account_id"   bigint NOT NULL,
  "issuer_id"             bigint NOT NULL,                        -- 反正規化：端點 46 以 store_id 進來，避免每列回查
  "account_version"       bigint NOT NULL,                        -- 設定當下 invoice_provider_accounts.version 的新值（＝ ProviderCutoverItem.version）
  "cutover_at"            timestamptz NOT NULL,                   -- 這一次設定的切點
  "deactivated_at"        timestamptz,                            -- 被下一次切點取代／連線被撤銷的時刻；NULL ＝ 現行有效
  "mode"                  varchar(24) NOT NULL DEFAULT 'AMEGO_AUTO',
  "created_by_account_id" bigint NOT NULL,
  "reason"                text,
  "created_at"            timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_provider_cutovers_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invpc_account" FOREIGN KEY ("provider_account_id") REFERENCES "invoice_provider_accounts" ("id"),
  CONSTRAINT "fk_invpc_issuer"  FOREIGN KEY ("issuer_id")          REFERENCES "invoice_issuers" ("id"),
  CONSTRAINT "chk_invpc_mode" CHECK ("mode" IN ('AMEGO_AUTO')),
  CONSTRAINT "chk_invpc_version_nonneg" CHECK ("account_version" >= 0),
  CONSTRAINT "chk_invpc_deactivated_after" CHECK ("deactivated_at" IS NULL OR "deactivated_at" >= "created_at")
);

-- 同一個 provider account 最多只有一列現行有效的切點。這條 partial unique 是
-- 「先關舊列、再插新列」那段順序的唯一守衛——少了它，兩位賣家分頁同時送出端點 26
-- 會留下兩列 deactivated_at IS NULL，而端點 46 的清單看起來完全正常。
CREATE UNIQUE INDEX "uq_invpc_open_per_account" ON "invoice_provider_cutovers" ("provider_account_id") WHERE "deactivated_at" IS NULL;
CREATE INDEX "idx_invpc_account_time" ON "invoice_provider_cutovers" ("provider_account_id", "created_at" DESC);

-- ---------------------------------------------------------------------------
-- §5.1.16 invoice_manual_unlocks｜A4 人工出口的雙人解鎖（端點 54／55）
--
-- 為什麼是獨立表而不是擴充 invoice_operations 的雙人取消四欄：那四欄
-- （cancel_requested_by／_at／cancel_confirmed_by／_at）表達不出 `unlock_id`
-- （端點 55 的 path param）、24 小時 `expires_at`、REJECTED 與 EXPIRED 兩個狀態，
-- 也表達不出「同一個 job 逾期後可以再發起一次」——那需要多列，而 operation 只有一列。
--
-- 🔴 **EXPIRED 不由排程器寫入**。逾期是時間函數，不是事件：任何「掃描把它改成 EXPIRED」
--    的 worker 都要嘛是《多副本排程搶佔規範》禁止的永久旗標、要嘛是一支沒有搶佔的
--    裸迴圈。本表的收斂點在**請求路徑**上：端點 54 進來時先把該 job 底下已過期的
--    AWAITING_SECOND_APPROVAL 就地轉 EXPIRED（同一交易），uq_invmu_open_per_operation
--    才騰得出名額給新的一列。讀取面（端點 51）另外把「仍是 AWAITING_SECOND_APPROVAL
--    但 expires_at <= now」投影成 EXPIRED——**DB 就是 SSOT，不需要有人去改那一列**。
--
-- 🔴 **雙人守衛在 SQL 的 WHERE**（`requested_by_account_id <> $approver`），不在應用層的 if。
--    chk_invmu_two_person 是 DB 兜底：兩道都要，因為 CHECK 回的是 23514（對 ADM 是一句 500），
--    而 CAS 的 rows=0 才回得出 9345。
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_manual_unlocks" (
  "id"                      bigint NOT NULL,                      -- TSID（＝ 對外的 unlock_id，端點 55 的 path param）
  "operation_id"            bigint NOT NULL,
  "status"                  varchar(24) NOT NULL DEFAULT 'AWAITING_SECOND_APPROVAL',  -- 4 值見 chk_invmu_status
  "requested_by_account_id" bigint NOT NULL,                      -- 第一位管理員（登入 principal）
  "request_reason"          text NOT NULL,                        -- 端點 54 的 reason（必填）
  "provider_check_evidence" text NOT NULL,                        -- 1–1000 字：向光貿查證的方式與結果
  "provider_checked_at"     timestamptz NOT NULL,
  "requested_at"            timestamptz NOT NULL DEFAULT NOW(),
  "expires_at"              timestamptz NOT NULL,                 -- 定版：requested_at + 24 小時
  -- J28 的證據（#3089 A 組第 2 項）。**只有 CANCEL_BLOCKED_UNKNOWN 那條路徑要填**：
  -- 從 CANCELLED_BY_ADMIN 進來的解鎖請求，它的取消已經在 J21→J24 完成過查證。
  -- 值域＝InvoiceCancelEvidenceKind 中**需人判讀**的兩值（另兩值是 provider 自動採信）。
  "cancel_evidence_kind"       varchar(48),
  "cancel_evidence_object_key" text,                              -- §25：只存 objectKey
  "approved_by_account_id"  bigint,                               -- 第二位管理員（核准或駁回的那一位）
  "decision_reason"         text,                                 -- 端點 55 的 reason
  "decided_at"              timestamptz,
  "version"                 bigint NOT NULL DEFAULT 0,
  "created_at"              timestamptz NOT NULL DEFAULT NOW(),
  "updated_at"              timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT "invoice_manual_unlocks_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "fk_invmu_operation" FOREIGN KEY ("operation_id") REFERENCES "invoice_operations" ("id"),
  CONSTRAINT "chk_invmu_status" CHECK ("status" IN ('AWAITING_SECOND_APPROVAL','APPROVED','REJECTED','EXPIRED')),
  -- 1–1000 字（§6.2.1 RequestManualUnlockForAdminRequest.provider_check_evidence 逐字）。
  -- 在 DB 擋而不是只在應用層：這是「憑什麼相信 provider 端沒開出去」的唯一書面依據，
  -- 而它會被 5 年稽核引用。
  CONSTRAINT "chk_invmu_evidence_len" CHECK (length("provider_check_evidence") BETWEEN 1 AND 1000),
  CONSTRAINT "chk_invmu_reason_len" CHECK (length("request_reason") >= 1),
  CONSTRAINT "chk_invmu_expiry_window" CHECK ("expires_at" > "requested_at"),
  CONSTRAINT "chk_invmu_two_person" CHECK ("approved_by_account_id" IS NULL OR "approved_by_account_id" <> "requested_by_account_id"),
  -- APPROVED／REJECTED ⇔ 有第二人且有決策時間。反向也要成立：AWAITING／EXPIRED 卻掛著
  -- 一個 approved_by 會讓「誰核准的」在稽核上出現兩個互相矛盾的答案。
  CONSTRAINT "chk_invmu_decided_pair" CHECK (("status" IN ('APPROVED','REJECTED')) = ("approved_by_account_id" IS NOT NULL AND "decided_at" IS NOT NULL)),
  -- 駁回一定要有理由：沒有理由的駁回在事後無法還原「為什麼不放行」。
  CONSTRAINT "chk_invmu_reject_reason" CHECK ("status" <> 'REJECTED' OR ("decision_reason" IS NOT NULL AND length("decision_reason") >= 1)),
  CONSTRAINT "chk_invmu_version_nonneg" CHECK ("version" >= 0),
  -- 🔴 這裡的值域**比 invoice_operations.cancel_evidence_kind 窄，而且是故意的**。
  -- CodeMeta InvoiceCancelEvidenceKind 有四值，前兩個（PROVIDER_QUERY_71／
  -- PROVIDER_LIST_ABSENT）是 provider 直接作證、由 J24 自動落地且**沒有第二人**；
  -- 本表只承載 J28（雙人確認）那條路徑，而 J28 存在的理由正是「證據需要人判讀」。
  -- ⇒ 收下一個 PROVIDER_QUERY_71 等於讓人替系統蓋一個系統自己蓋得了的章。
  -- 第二條逐字鏡射 chk_invop_evidence_object_key（檔案只能配那兩個需人判讀的種類）；
  -- 在 kind 已被上一條限死的前提下它等價於「object_key 有值就要有 kind」。
  -- 少了這兩條，這裡收得下的東西到了 J28 那一句會 23514，而那時**第一個人早就送出了**
  -- ——錯誤會在第二個人按核准時才爆，訊息還讀不出原因。
  CONSTRAINT "chk_invmu_evidence_kind" CHECK ("cancel_evidence_kind" IS NULL OR "cancel_evidence_kind" IN ('MOF_PLATFORM_SCREENSHOT', 'AMEGO_RECONCILE_FILE')),
  CONSTRAINT "chk_invmu_evidence_object_key" CHECK ("cancel_evidence_object_key" IS NULL OR "cancel_evidence_kind" IS NOT NULL)
);

-- 一個 job 同時只能有一個未決的解鎖請求。逾期那一列必須先被就地轉成 EXPIRED 才騰得出
-- 名額——這正是「EXPIRED 由請求路徑收斂」那條規則的機械保證。
CREATE UNIQUE INDEX "uq_invmu_open_per_operation" ON "invoice_manual_unlocks" ("operation_id") WHERE "status" = 'AWAITING_SECOND_APPROVAL';
CREATE INDEX "idx_invmu_operation" ON "invoice_manual_unlocks" ("operation_id", "created_at" DESC);

-- ---------------------------------------------------------------------------
-- §5.2.1 排程 worker 的候選查詢索引
-- ---------------------------------------------------------------------------
CREATE INDEX "idx_invop_due" ON "invoice_operations" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['READY'::character varying, 'RETRY_SCHEDULED'::character varying, 'SENDING'::character varying])::text[]));
CREATE INDEX "idx_invop_recheck_due" ON "invoice_operations" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['WAITING_CONFIRMATION'::character varying, 'PROVIDER_STATUS_RECHECK'::character varying, 'FAILED_UNKNOWN'::character varying, 'CANCEL_REQUESTED'::character varying, 'CANCEL_BLOCKED_UNKNOWN'::character varying])::text[]));
CREATE INDEX "idx_invinbox_due" ON "invoice_inbox_events" ("next_attempt_at") WHERE ((status)::text = ANY ((ARRAY['RECEIVED'::character varying, 'DEFERRED'::character varying, 'FAILED'::character varying])::text[]));
CREATE INDEX "idx_invinbox_deferred_aggregate" ON "invoice_inbox_events" ("aggregate_ref") WHERE "status" = 'DEFERRED';
CREATE INDEX "idx_invoutbox_pending" ON "invoice_outbox_events" ("next_attempt_at") WHERE "status" = 'pending';
CREATE INDEX "idx_invcomp_sla_overdue" ON "invoice_complaints" ("sla_due_at") WHERE "status" = 'OPEN';
CREATE INDEX "idx_invcomp_overdue_escalate" ON "invoice_complaints" ("overdue_at") WHERE "status" = 'OVERDUE';
CREATE INDEX "idx_invcomp_buyer_review_due" ON "invoice_complaints" ("buyer_review_due_at") WHERE "status" = 'SELLER_RESPONDED';
-- 法定存證期限逼近告警的資料面（《硬上限依賴接入治理規範》要求的「逼近上限」告警）
CREATE INDEX "idx_invdoc_deadline" ON "invoice_documents" ("statutory_deadline_at") WHERE ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'ISSUING'::character varying, 'ISSUE_FAILED'::character varying, 'AWAITING_MANUAL'::character varying])::text[]));
-- 上傳狀態輪詢（AMEGO 無 webhook，只能自己輪詢 /json/invoice_status）
CREATE INDEX "idx_invdoc_upload_recheck" ON "invoice_documents" ("upload_recheck_next_at")
  WHERE "status" = 'ISSUED' AND "upload_recheck_next_at" IS NOT NULL
    AND ("provider_upload_status" IS NULL OR "provider_upload_status" IN (1,2,3,31,32));
-- PDF 抓取排程（INV.pdf）。述詞用 pdf_object_key IS NULL，已回填的列必須離開候選集合
CREATE INDEX "idx_invdoc_pdf_fetch" ON "invoice_documents" ("pdf_fetch_next_at")
  WHERE "pdf_fetch_next_at" IS NOT NULL AND "pdf_object_key" IS NULL;

-- ---------------------------------------------------------------------------
-- §5.3.3 序號生成（折讓單號與收據號）
--
-- 🔴 **這兩支不是由 Atlas declarative 建立的，它們由手寫 migration 建立**
--    （`migrations/20260829152318_add_invoice_number_sequences.sql`）。
--    Atlas 社群版的 postgres driver 只追蹤被 serial／identity 欄位擁有的 sequence，
--    對 standalone `CREATE SEQUENCE` 完全看不見——實測 `atlas schema inspect` 回 0 命中。
--    留在本檔是為了「讀 schema.sql 就看得到完整結構」，**改動它不會產生 migration**，
--    要改必須同時改那支手寫 migration（或新增一支）。
--
-- 2026-08-30 補：建 schema 時只抄了 §5.1，漏了寫在 §5.3.3 的這兩支。缺了它們，
-- `invoice_adjustments.allowance_number`（chk_invadj_allowance_shape 要求 ALLOWANCE
-- 必填）與 `invoice_documents.receipt_no`（chk_invdoc_receipt 雙向等式）**沒有任何
-- 產生器** —— 呼叫端無處取號，而症狀是「折讓／收據永遠開不出來」不是編譯錯。
--
-- 為什麼是 SEQUENCE 不是 TSID：AMEGO 開立折讓成功後**不回傳任何識別碼**（只有
-- code / msg），且 `AllowanceNumber` 上限 16 字 ⇒ 單號必須我方自產、**送出前先落庫**
-- （否則「送出成功但回應遺失」就永遠查不回來，唯一救回路徑是拿我方自己產的單號打
-- allowance_query）。TSID 十進位 19 碼加前綴就爆 16 字。
--
-- 多副本天然安全：nextval 不受交易回滾影響。跳號可接受——法規只要求不可重複、
-- 無連號義務（U-5）。SEQUENCE 全域共用跨 issuer：AMEGO 只要求帳戶內唯一，
-- 全域唯一是它的安全超集。
--
--   allowance_number = 'A' || lpad(nextval('invoice_allowance_number_seq')::text, 12, '0')
--   receipt_no       = 'R' || lpad(nextval('invoice_receipt_number_seq')::text,   12, '0')
-- ---------------------------------------------------------------------------
CREATE SEQUENCE "invoice_allowance_number_seq" AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;
CREATE SEQUENCE "invoice_receipt_number_seq"   AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;

-- ---------------------------------------------------------------------------
-- 冪等鍵（#3015 裁決 A，DEC-20260902-163404）
--
-- 🔴 **為什麼放在 invoice-service 而不是 gateway 或 Redis**：
--   - gateway 的 in-process 紀錄只活在單一 pod，而 prod gateway 是 `replicas: 2`
--     ⇒ 重送落到另一顆就查不到，退化成沒有冪等（那是 PR #3025 的已知邊界）。
--   - Redis 被 §28 明文擋住：冪等紀錄是「不能失敗的狀態」，禁止放進與無成長上限的
--     快取共用同一個 `allkeys-*` 逐出預算的實例，而現況 Redis 正是該結構。
--   - 放這裡才能與**業務寫入同一個 transaction** —— 那是「同一把鍵絕不產生第二筆」
--     唯一成立的方式。跨服務兩段寫入之間永遠有窗口。
--
-- 🔴 主鍵是 (scope, actor_id, idem_key) 三欄而不是只有 idem_key：
--   - 少了 scope，同一把鍵在不同端點之間會互撞（前端常對整個頁面用同一把鍵）
--   - 少了 actor_id，A 買家的鍵會擋住 B 買家 —— 而鍵是前端產生的，不保證全域唯一
--
-- request_hash 是「同鍵不同內容」的判別依據（回 9350）。**不存請求原文**：
-- 那裡面有載具明碼與買受人資料，而本表沒有加密欄（同 invoice_correction_requests
-- 的理由）。雜湊只用來比對相同與否，不需要還原。
--
-- result_ref 是第一次成功的業務主鍵，重送時用它回放。允許 NULL：
-- 「已認領但業務寫入還沒完成」是同一個交易內的中間狀態，交易失敗會一起 rollback。
-- ---------------------------------------------------------------------------
CREATE TABLE "invoice_idempotency_keys" (
  "scope"        varchar(64)  NOT NULL,   -- 端點語意，例 'BUYER_INVOICE_PROFILE_CREATE'
  "actor_id"     bigint       NOT NULL,   -- buyer_id／store_id／actor_account_id，依 scope
  "idem_key"     varchar(128) NOT NULL,   -- 呼叫端送的 Idempotency-Key 原值
  "request_hash" char(64)     NOT NULL,   -- sha256 hex；同鍵不同內容的判別依據
  "result_ref"   bigint,                  -- 第一次成功的業務主鍵，供回放
  "created_at"   timestamptz  NOT NULL DEFAULT now(),
  CONSTRAINT "pk_invidem" PRIMARY KEY ("scope", "actor_id", "idem_key"),
  CONSTRAINT "chk_invidem_hash" CHECK ("request_hash" ~ '^[0-9a-f]{64}$'),
  CONSTRAINT "chk_invidem_key_nonblank" CHECK (length("idem_key") >= 1)
);

-- GC 用：本表只需要保留「重送窗口」那段時間，不是永久帳。
-- 清理是一次性維運 SQL（§22 不進 application code），索引讓它便宜。
CREATE INDEX "idx_invidem_created" ON "invoice_idempotency_keys" ("created_at");

COMMENT ON TABLE invoice_idempotency_keys IS
  '冪等鍵（#3015）。與業務寫入同一個 transaction ⇒ 同一把鍵絕不產生第二筆。保留期由維運 SQL 依 created_at 清理，非永久帳。';
