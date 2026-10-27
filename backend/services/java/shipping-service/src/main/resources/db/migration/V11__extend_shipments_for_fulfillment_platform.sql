-- V11：正規化配送域擴充（65 號物流履約平台 Shared Foundation）
--
-- 本檔只做 Foundation 骨幹需要的 expand，**不含任何 feature 專屬結構**：
--   - shipment_batches 與 shipments.batch_id 屬 68（7-11 大宗寄倉）⇒ 由該波自己的 migration 建
--   - MerchantDeliveryJobStatus 相關表屬 69 ⇒ 同上
--
-- ⚠️ DROP 與 ADD CONSTRAINT 必須同檔（Flyway 預設單檔單交易），否則兩句之間
--    有一個「完全不受約束」的窗口。

-- ─────────────────────────────────────────────────────────────
-- ① 配送方式值域擴充：3004（7-11 大宗寄倉 B2C）／3005（商家自行配送）
--    CodeMeta 的 ShippingMethod 已同批加上這兩值；沒有這一句 DB 會以 23514 拒絕寫入，
--    而該錯誤會被 MQ consumer 的重試包住 ⇒ 對呼叫端是靜默的。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE shipments DROP CONSTRAINT chk_shipping_method;
ALTER TABLE shipments ADD  CONSTRAINT chk_shipping_method
    CHECK (shipping_method IN ('3001', '3002', '3003', '3004', '3005'));

-- ─────────────────────────────────────────────────────────────
-- ② status_entered_at：aging 指標的唯一錨
--    禁用 updated_at 當 aging 基準——任何無關欄位的 UPDATE 都會重置它，
--    於是「卡在某狀態多久」會被每一次標籤重印、每一次租約續期洗掉。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE shipments ADD COLUMN status_entered_at TIMESTAMPTZ;
UPDATE shipments SET status_entered_at = COALESCE(updated_at, created_at) WHERE status_entered_at IS NULL;
ALTER TABLE shipments ALTER COLUMN status_entered_at SET NOT NULL;
ALTER TABLE shipments ALTER COLUMN status_entered_at SET DEFAULT NOW();

-- ─────────────────────────────────────────────────────────────
-- ③ provider 原生值加寬到 32
--    🔴 **不新增 provider_raw_status**：既有 provider_status（V1 comment 逐字
--    「PayUni ShipStatus / Uber status」）已經就是那一欄。新增同義欄的代價是兩份
--    provider 值，而「哪一份才是真的」沒有任何 gate 守得住。
--    加寬的理由：Uber 的狀態字串（如 pickup_complete）超過既有的 VARCHAR(20)。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE shipments ALTER COLUMN provider_status TYPE VARCHAR(32);
ALTER TABLE shipment_status_logs ALTER COLUMN provider_status TYPE VARCHAR(32);

-- ─────────────────────────────────────────────────────────────
-- ④ 取消申請（992 CANCEL_PENDING）的事實欄
--    992 的語意是「取消申請已受理，待 provider 確認」，它是唯一允許回退到非終態的狀態。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE shipments ADD COLUMN cancel_requested_at  TIMESTAMPTZ;
ALTER TABLE shipments ADD COLUMN cancel_request_actor VARCHAR(16);
ALTER TABLE shipments ADD CONSTRAINT chk_shipments_cancel_actor
    CHECK (cancel_request_actor IS NULL OR cancel_request_actor IN ('BUYER', 'SELLER', 'ADMIN', 'SYSTEM'));

-- ─────────────────────────────────────────────────────────────
-- ⑤ 兩支新排程器的租約欄
--    🔴 租約欄不是「已做過旗標」：NULL 或 <= NOW() 表示可搶佔，禁止當成 done flag 回填。
--    pod 在副作用中途崩潰時，租約到期會讓該筆自動回到候選集合；永久旗標則會讓它卡死且無告警。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE shipments ADD COLUMN create_query_next_at   TIMESTAMPTZ;
ALTER TABLE shipments ADD COLUMN cancel_confirm_next_at TIMESTAMPTZ;

COMMENT ON COLUMN shipments.status_entered_at IS
    '本筆進入當前 status 的時間；aging 指標一律錨在它，禁用 updated_at（65 §7.1.1.5）';
COMMENT ON COLUMN shipments.cancel_requested_at IS
    '取消申請受理時間（992 CANCEL_PENDING 的事實欄）';
COMMENT ON COLUMN shipments.cancel_request_actor IS
    '取消申請發起者：BUYER / SELLER / ADMIN / SYSTEM';
COMMENT ON COLUMN shipments.create_query_next_at IS
    '建單結果回查（991 CREATE_UNKNOWN）的租約到期時間（非「已查過旗標」）。NULL 或 <= NOW() 表示可搶佔。';
COMMENT ON COLUMN shipments.cancel_confirm_next_at IS
    '取消確認輪詢（992 CANCEL_PENDING）的租約到期時間（非「已確認旗標」）。NULL 或 <= NOW() 表示可搶佔。';
COMMENT ON COLUMN shipments.provider_status IS
    'provider 原生狀態值（PayUni ShipStatus / Uber status 等）。正規化值在 status 欄，兩者分開儲存；'
    '每個 provider 都必須走顯式映射表，PayUni 那張是恆等映射但必須存在（65 §7.1.2.0 ②-b）。';
COMMENT ON COLUMN shipments.uber_delivery_id IS
    '⚠️ 本波（Foundation）無任何寫入者——實查零 setter 呼叫點。語意與寫入時機由 66（Uber Direct）落地時定義；'
    '在那之前讀到的一律是 NULL，不得當成「這筆不是 Uber 單」的判準。';

-- ─────────────────────────────────────────────────────────────
-- ⑥ shipment_status_logs 擴充成「一個服務一張、帶 subject_type」的狀態歷程表
--
-- 🔴 **刻意不做 shipment_id → subject_id 的 rename**（設計書 §5.2.3 原文要求 rename）：
--    insertStatusLog 是在**業務交易內**被呼叫的（ShippingService 的建單／確認出貨／
--    叫件／逆物流建單四條路徑都是 shipmentMapper.insert 之後緊接著寫），
--    rolling deploy 期間舊 pod 仍寫 shipment_id ⇒ 欄位不存在 ⇒ **整筆業務交易 rollback**，
--    症狀是建單直接失敗，不是稽核少一列。
--    ⇒ 欄名的收斂延到 68（它引入 SHIPMENT_BATCH 時才真正需要「subject」語意），
--      屆時走擴張—收縮（先 ADD subject_id 雙寫 → 收縮），不要在 Foundation 一次改掉。
--    ⚠️ 本波 subject_type 只有 'SHIPMENT' 一個值會被寫入 ⇒ shipment_id 這個名字現在完全誠實。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE shipment_status_logs ADD COLUMN subject_type     VARCHAR(24) NOT NULL DEFAULT 'SHIPMENT';
ALTER TABLE shipment_status_logs ADD COLUMN actor_type       VARCHAR(16) NOT NULL DEFAULT 'SYSTEM';
ALTER TABLE shipment_status_logs ADD COLUMN actor_account_id BIGINT;
ALTER TABLE shipment_status_logs ADD COLUMN correlation_id   VARCHAR(64);

ALTER TABLE shipment_status_logs ADD CONSTRAINT chk_ssl_subject
    CHECK (subject_type IN ('SHIPMENT', 'SHIPMENT_BATCH'));
ALTER TABLE shipment_status_logs ADD CONSTRAINT chk_ssl_actor
    CHECK (actor_type IN ('SYSTEM', 'ADMIN', 'SELLER', 'BUYER', 'PROVIDER'));

DROP INDEX IF EXISTS idx_status_logs_shipment;
CREATE INDEX IF NOT EXISTS idx_status_logs_subject
    ON shipment_status_logs (subject_type, shipment_id, created_at);

COMMENT ON COLUMN shipment_status_logs.subject_type IS
    '狀態歷程的主體種類。🔴 由具名 transition 函式自己填，呼叫端不得傳——'
    '共用表最致命的失效模式是「寫進去的是另一條軸的軌跡」，而那完全無聲（本 repo 的 invoice 側已踩過一次）。';
COMMENT ON COLUMN shipment_status_logs.actor_type IS
    '推動這次轉移的角色：SYSTEM / ADMIN / SELLER / BUYER / PROVIDER';
