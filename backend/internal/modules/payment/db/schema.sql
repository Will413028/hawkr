-- payment 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（payment.<表>），不建跨 schema 的 FK；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA payment;

-- 收款帳戶：擁有者在金流商那邊的外部帳戶的本地快取（收款開通 ADR D13、D19、D28）。擁有者鍵現階段是 store_id（seller 的店家；不建跨 schema 的 FK，
-- 出現第二種擁有者時再通用化）。一個擁有者一列，關店再開沿用；只存狀態與能力快照，不存聯絡信箱（D23）。
-- seller 核准的 transaction 只 INSERT「建立中」的列（external_account_id 是 NULL、status 是 NOT_CREATED；ON CONFLICT DO NOTHING，既有的列不改）
-- 並排入建立 job；之後的寫入都由本模組的 job 與管理員的指令做：建立 job 在 transaction 外分兩步建立外部帳戶（取得 ID、完成設定，D55），第一步拿到外部 ID 就填上，
-- 第二步成功才記 create_completed_at；同步 job 重讀外部帳戶後整列覆寫（沒有觀測歷史，重讀即最新）。
-- 「已建立」看 create_completed_at，不看有沒有外部 ID（D59）：有外部 ID 而 create_completed_at 是 NULL 的是半建立的列，對外仍是未建立（status 維持 NOT_CREATED，
-- 不可開通、不輪詢、不同步）。create_called_at 是第一次呼叫金流商之前寫入的時刻（NULL 表示從沒呼叫過，這種列沒有外部帳戶，清理時直接刪）；
-- 管理員確認金流商那邊沒有這家店的帳戶而換新冪等鍵時改成新鍵第一次呼叫之前的時刻（D63）。「呼叫過、沒有外部 ID」的列（結果不明）由週期掃描以同一把鍵重送取回
-- ID；從 create_called_at 起 7 天（小於金流商保存冪等鍵的 30 天）仍了結不了就進終局：非清理的列記 create_failed_at（建立失敗的待辦），清理的列進 purge_blocked（原因 unresolved_creation），
-- 不重送、不換新鍵、不列出帳戶。
-- status 是中立的五個狀態，由轉帳能力的狀態與 status_details 推導；transfers_active 與 payouts_active 是外部帳戶的能力是否啟用（seller 的開通閘門讀
-- 經 PayoutAccountChanged 複製的 transfers_active；ENABLED 等於 transfers_active）。requirements 是快照的原始內容，status_reason 是導出目前狀態的金流商原因碼（只給後台；外部帳戶已在金流商關閉時是 closed，終局，不再輪詢，D61）。
-- create_failed_at 是建立重試用完的時刻（只有還沒完成建立的列才有；後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED 的開始時間，重試建立時清空）。
-- create_refused_at 是建立失敗是「金流商明確拒絕（永久失敗）」時的標記（與 create_failed_at 同時寫、同時清；重試用完的暫時性失敗沒有它）：呼叫過、沒有外部 ID 的列有它時，週期掃描不再以同一把鍵重送
-- （重送只會再被拒絕），管理員重試建立與重試關閉不看它。
-- seller_read_claimed_at 是上次賣家觸發的讀取被受理的時刻（含失敗與並行的；條件式 UPDATE 搶名額），同步 job 不讀寫。
-- purge_pending_at 是擁有者的清理已要求、還沒完成的時刻，與有沒有外部 ID 無關（清理 job 呼叫 provider 之前先標；刪列之前一直在）：本地列留著（保住 store_id 與外部 ID 或冪等鍵的對應），
-- 不被建立、不進同步輪詢、不被賣家／管理員的重新整理讀，由 payment 的週期掃描重排「關閉後刪除」，關閉（沒有外部 ID 的列先以冪等鍵取得外部帳戶）成功才刪列。
-- purge_blocked_at 與 purge_blocked_reason 是清理的終局（D59、D60）：關閉被金流商拒絕、或標成等待清理後 7 天仍然關不掉或問不到金流商；列留著，不再被週期掃描重排，
-- 出現在後台待辦 PAYMENT_ACCOUNT_PURGE_STUCK，由管理員「重試關閉」（清除終局並重新計算期限；原因是 unresolved_creation 的，冪等鍵保存期內以同一把鍵重送取回 ID 再關閉，超過保存期不能重試）或「已在金流商處理，刪除對應」（同一個 transaction 寫一筆 adminaudit 的後台紀錄並刪列，D62）。
-- purge_replay_requested_at 是管理員對清理終局（unresolved_creation）的列要求「以同一把鍵重送」的時刻（RetryPurge 排入 Replay job 時寫，重送結束不論結果都清除，取回 ID 重啟清理時一併清除）：
-- 有它表示重送排隊中或進行中，這段期間後端拒絕「刪除對應」與再次「重試關閉」，避免重送取回的帳戶 ID 因列被刪而無人關閉（K1）。只有終局的列才有（CHECK accounts_purge_replay_check）。
-- create_idempotency_key 是寫入「建立中」的列時產生的隨機 UUID（v4），建立外部帳戶的每次重試都用它當冪等鍵（Stripe 建議每個邏輯請求用隨機 UUID；
-- 不用 store_id，兩個資料庫共用同一個 Stripe 帳號時才不會撞到別人的帳戶）。同一個理由，建立外部帳戶時除了 metadata.store_id 還寫 metadata.hawkr_namespace
-- （設定 STRIPE_ACCOUNT_NAMESPACE，每個環境不同，D61）；程式不以 metadata 列出對帳（D63），寫它是為了之後要做全平台孤兒帳戶稽核時不必回填。
CREATE TABLE payment.accounts (
    store_id               BIGINT      PRIMARY KEY,
    provider               TEXT        NOT NULL,
    external_account_id    TEXT,
    requirement_collection TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'NOT_CREATED',
    transfers_active       BOOLEAN     NOT NULL DEFAULT false,
    payouts_active         BOOLEAN     NOT NULL DEFAULT false,
    requirements           JSONB       NOT NULL DEFAULT '{}',
    status_reason          TEXT,
    fetched_at             TIMESTAMPTZ,
    create_failed_at       TIMESTAMPTZ,
    create_refused_at      TIMESTAMPTZ,
    create_called_at       TIMESTAMPTZ,
    create_completed_at    TIMESTAMPTZ,
    create_idempotency_key UUID        NOT NULL DEFAULT gen_random_uuid(),
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    seller_read_claimed_at TIMESTAMPTZ,
    purge_pending_at       TIMESTAMPTZ,
    purge_blocked_at       TIMESTAMPTZ,
    purge_blocked_reason   TEXT,
    purge_replay_requested_at TIMESTAMPTZ,
    CONSTRAINT accounts_store_id_check CHECK (store_id > 0),
    CONSTRAINT accounts_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT accounts_external_id_check CHECK (external_account_id IS NULL OR char_length(external_account_id) BETWEEN 1 AND 255),
    CONSTRAINT accounts_collection_check CHECK (requirement_collection IN ('PROVIDER', 'PLATFORM')),
    CONSTRAINT accounts_status_check CHECK (status IN ('NOT_CREATED', 'UNDER_REVIEW', 'ACTION_REQUIRED', 'ENABLED', 'DISABLED')),
    CONSTRAINT accounts_created_check CHECK ((status = 'NOT_CREATED') = (create_completed_at IS NULL)),
    CONSTRAINT accounts_enabled_check CHECK ((status = 'ENABLED') = transfers_active),
    CONSTRAINT accounts_snapshot_check CHECK (create_completed_at IS NOT NULL OR (fetched_at IS NULL AND NOT payouts_active AND status_reason IS NULL)),
    CONSTRAINT accounts_requirements_check CHECK (jsonb_typeof(requirements) = 'object'),
    CONSTRAINT accounts_status_reason_check CHECK (status_reason IS NULL OR char_length(status_reason) BETWEEN 1 AND 200),
    CONSTRAINT accounts_create_failed_check CHECK (create_failed_at IS NULL OR create_completed_at IS NULL),
    CONSTRAINT accounts_create_refused_check CHECK (create_refused_at IS NULL OR create_failed_at IS NOT NULL),
    CONSTRAINT accounts_completed_check CHECK (create_completed_at IS NULL OR external_account_id IS NOT NULL),
    CONSTRAINT accounts_called_check CHECK (external_account_id IS NULL OR create_called_at IS NOT NULL),
    CONSTRAINT accounts_purge_blocked_check CHECK ((purge_blocked_at IS NULL) = (purge_blocked_reason IS NULL) AND (purge_blocked_at IS NULL OR purge_pending_at IS NOT NULL)),
    CONSTRAINT accounts_purge_replay_check CHECK (purge_replay_requested_at IS NULL OR purge_blocked_at IS NOT NULL),
    CONSTRAINT accounts_purge_blocked_reason_check CHECK (purge_blocked_reason IS NULL OR char_length(purge_blocked_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX accounts_external ON payment.accounts (provider, external_account_id) WHERE external_account_id IS NOT NULL;
CREATE INDEX accounts_poll ON payment.accounts (store_id) WHERE status IN ('UNDER_REVIEW', 'ACTION_REQUIRED', 'DISABLED');
CREATE INDEX accounts_create_failed ON payment.accounts (create_failed_at) WHERE create_failed_at IS NOT NULL;
CREATE INDEX accounts_purge_pending ON payment.accounts (store_id) WHERE purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL;
CREATE INDEX accounts_unresolved ON payment.accounts (store_id) WHERE external_account_id IS NULL AND create_called_at IS NOT NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL AND create_refused_at IS NULL;
CREATE INDEX accounts_purge_blocked ON payment.accounts (purge_blocked_at) WHERE purge_blocked_at IS NOT NULL;

-- 付款：買家對一張訂單的一次收款（一個 PaymentIntent；計畫 2026-11-23-b2-payment 的 D7、D8、D9）。實體，主鍵 UUIDv7（應用層產生，不設 DEFAULT，row-identity ADR D1）；
-- order_no 是訂單的人看的編號，UNIQUE：一張訂單一列、同時只有一個有效的 PaymentIntent，重試在同一列上（D8；唯一索引 payments_order 守「一張訂單一列」，每次嘗試的 PaymentIntent 留在 payment_attempts）。
-- 一筆付款只對一家店（store_id；轉帳的 transfer_group 與金額依它）。amount 是整數，不帶幣別（D9：幣別由 adapter 的設定決定）。不建跨 schema 的 FK：
-- store_id、buyer_id 與 order_no 是組裝點注入的訂單 port 在建立當下回的事實，寫成快照。
-- 建立付款分兩段（規則 5）：請求內的 transaction 只寫「建立中」的列（CREATING，沒有外部 ID）並排補做 job，提交之後在 transaction 外帶 create_idempotency_key
-- 呼叫 provider；成功就在新的 transaction 寫外部 ID 與狀態，失敗或逾時留 CREATING，補做 job 與週期掃描以同一把鍵重送。create_called_at 是第一次呼叫 provider
-- 之前寫入的時刻（NULL 表示從沒呼叫過）：「呼叫過、沒有外部 ID」是結果不明（ADR D68），冪等鍵保存期的下限之前以同一把鍵重送，之後只查回：找到就收下，
-- 查無要再過最短等待才進 CREATE_FAILED（failure_reason）；provider 已知拒絕（可重試）換新鍵，舊嘗試記 superseded_at、新嘗試另寫一列 payment_attempts，不計期限；
-- 與 provider 對不起來或查回連續失敗（create_lookup_failures）進 ANOMALY（failure_reason 記原因，佔住這張訂單的唯一性，補掃不再重試，由後台人工結案）。
-- CREATE_FAILED 與 CANCELED 的列在買家再次建立同一張訂單的付款時換新鍵重設回 CREATING。
-- status：CREATING 等建立；AWAITING_PAYMENT 已建立、等買家付款（含需要 3DS）；PROCESSING 付款處理中；SUCCEEDED 已入帳（終局，external_charge_id 是轉帳的
-- source_transaction，paid_at 是入帳時刻）；CANCELED 已取消（取消後仍可能因競態變成 SUCCEEDED：provider 的成功一律勝出）；CREATE_FAILED 建立的終局；ANOMALY 待人工結案。
-- 狀態只由 provider 的快照推導（讀取即最新），external_charge_id 與 paid_at 只在 SUCCEEDED 有。fetched_at 是寫入快照的讀取開始時間：較早開始的讀取不覆蓋較晚的（同 accounts 的 D32）。
-- last_error 是最近一次付款嘗試失敗的 provider 原因碼（只給後台與事件；成功後清除）。client_secret 不存：它是持有即授權的憑證，GetPayment 每次向 provider 讀。
CREATE TABLE payment.payments (
    id                     UUID        PRIMARY KEY,
    order_no               TEXT        NOT NULL,
    store_id               BIGINT      NOT NULL,
    buyer_id               BIGINT      NOT NULL,
    amount                 BIGINT      NOT NULL,
    provider               TEXT        NOT NULL,
    description            TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'CREATING',
    external_payment_id    TEXT,
    external_charge_id     TEXT,
    paid_at                TIMESTAMPTZ,
    last_error             TEXT,
    failure_reason         TEXT,
    fetched_at             TIMESTAMPTZ,
    create_idempotency_key UUID        NOT NULL DEFAULT gen_random_uuid(),
    create_called_at       TIMESTAMPTZ,
    create_lookup_failures INTEGER     NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT payments_lookup_failures_check CHECK (create_lookup_failures >= 0),
    CONSTRAINT payments_order_no_check CHECK (char_length(order_no) BETWEEN 1 AND 64),
    CONSTRAINT payments_store_id_check CHECK (store_id > 0),
    CONSTRAINT payments_buyer_id_check CHECK (buyer_id > 0),
    CONSTRAINT payments_amount_check CHECK (amount > 0),
    CONSTRAINT payments_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT payments_description_check CHECK (char_length(description) BETWEEN 1 AND 200),
    CONSTRAINT payments_status_check CHECK (status IN ('CREATING', 'AWAITING_PAYMENT', 'PROCESSING', 'SUCCEEDED', 'CANCELED', 'CREATE_FAILED', 'ANOMALY')),
    CONSTRAINT payments_external_id_check CHECK (external_payment_id IS NULL OR char_length(external_payment_id) BETWEEN 1 AND 255),
    CONSTRAINT payments_charge_id_check CHECK (external_charge_id IS NULL OR char_length(external_charge_id) BETWEEN 1 AND 255),
    CONSTRAINT payments_created_check CHECK (status NOT IN ('AWAITING_PAYMENT', 'PROCESSING', 'SUCCEEDED') OR external_payment_id IS NOT NULL),
    CONSTRAINT payments_creating_check CHECK (status <> 'CREATING' OR external_payment_id IS NULL),
    CONSTRAINT payments_called_check CHECK (external_payment_id IS NULL OR create_called_at IS NOT NULL),
    CONSTRAINT payments_paid_check CHECK ((status = 'SUCCEEDED') = (paid_at IS NOT NULL) AND (status <> 'SUCCEEDED' OR external_charge_id IS NOT NULL)),
    CONSTRAINT payments_charge_only_when_paid_check CHECK (status = 'SUCCEEDED' OR external_charge_id IS NULL),
    CONSTRAINT payments_failure_check CHECK ((status IN ('CREATE_FAILED', 'ANOMALY')) = (failure_reason IS NOT NULL)),
    CONSTRAINT payments_failure_reason_check CHECK (failure_reason IS NULL OR char_length(failure_reason) BETWEEN 1 AND 200),
    CONSTRAINT payments_last_error_check CHECK (last_error IS NULL OR char_length(last_error) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX payments_order ON payment.payments (order_no);
CREATE UNIQUE INDEX payments_external ON payment.payments (provider, external_payment_id) WHERE external_payment_id IS NOT NULL;
CREATE INDEX payments_creating ON payment.payments (created_at) WHERE status = 'CREATING';
CREATE INDEX payments_open ON payment.payments (id) WHERE status IN ('AWAITING_PAYMENT', 'PROCESSING');

-- 付款嘗試歷程（D8 補充）：一張訂單的付款列一對多，每次建立 PaymentIntent 的嘗試一列（provider、外部 ID、冪等鍵、建立與呼叫時刻）。實體，主鍵 UUIDv7。
-- 一張訂單同時只有一個有效的 PaymentIntent：superseded_at 為 NULL 的是有效的那一個，唯一索引 payment_attempts_effective 守它；付款列的 external_payment_id、
-- create_idempotency_key、create_called_at 是有效嘗試的鏡像。買家在 Stripe 取消後重付時，舊嘗試記 superseded_at、新的嘗試另寫一列，舊 PaymentIntent 的外部 ID 留在這裡：
-- 事件與同步依外部 ID 經這張表對回付款列，舊 PaymentIntent 晚到的成功不會因為付款列已換新 ID 而找不到。
-- state 是這個 PaymentIntent 最近一次讀到的 provider 狀態（NULL 表示還沒讀過），fetched_at 是寫入它的讀取開始時間（較早開始的讀取不覆蓋較晚的）。
-- 舊嘗試晚到成功時，付款列還沒入帳就把它換成有效的嘗試（記成功並發事件，新的嘗試排取消）；付款列已經以另一個 PaymentIntent 入帳時，這一列記 excess_paid_at
-- （重複入帳：同一張訂單有兩個 PaymentIntent 都扣到款，不自動退款，由訂單一方發起退款或後台處理；索引 payment_attempts_excess 給後台查）。
CREATE TABLE payment.payment_attempts (
    id                  UUID        PRIMARY KEY,
    payment_id          UUID        NOT NULL REFERENCES payment.payments (id),
    provider            TEXT        NOT NULL,
    external_payment_id TEXT,
    idempotency_key     UUID        NOT NULL,
    called_at           TIMESTAMPTZ,
    state               TEXT,
    fetched_at          TIMESTAMPTZ,
    superseded_at       TIMESTAMPTZ,
    excess_paid_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_attempts_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT payment_attempts_external_id_check CHECK (external_payment_id IS NULL OR (char_length(external_payment_id) >= 1 AND char_length(external_payment_id) <= 255)),
    CONSTRAINT payment_attempts_state_check CHECK (state IS NULL OR state IN ('AWAITING_PAYMENT', 'PROCESSING', 'SUCCEEDED', 'CANCELED')),
    CONSTRAINT payment_attempts_called_check CHECK (external_payment_id IS NULL OR called_at IS NOT NULL),
    CONSTRAINT payment_attempts_state_external_check CHECK (state IS NULL OR external_payment_id IS NOT NULL),
    CONSTRAINT payment_attempts_excess_check CHECK (excess_paid_at IS NULL OR (state = 'SUCCEEDED' AND superseded_at IS NOT NULL))
);
CREATE UNIQUE INDEX payment_attempts_effective ON payment.payment_attempts (payment_id) WHERE superseded_at IS NULL;
CREATE UNIQUE INDEX payment_attempts_external ON payment.payment_attempts (provider, external_payment_id) WHERE external_payment_id IS NOT NULL;
CREATE UNIQUE INDEX payment_attempts_key ON payment.payment_attempts (idempotency_key);
CREATE INDEX payment_attempts_payment ON payment.payment_attempts (payment_id);
CREATE INDEX payment_attempts_excess ON payment.payment_attempts (excess_paid_at) WHERE excess_paid_at IS NOT NULL;

-- 轉帳：訂單完成後平台把這筆付款的款項轉給賣家（separate charges and transfers，D1、D6）。
-- 一筆付款最多一筆有效轉帳：唯一索引 transfers_active_payment 涵蓋 PENDING、SUCCEEDED 與 ANOMALY（FAILED 不佔，失敗後可以重建；ANOMALY 與 provider 對不起來，不能確定沒有搬錢，所以佔位）。amount 是轉給賣家的整數金額
-- （不超過付款金額；累計由轉帳 use case 在鎖付款列後守，Stripe 另外守「轉帳不超過付款扣掉已退款」）。destination_account_id 是寫列當下賣家收款帳戶的外部 ID 快照
-- （帳戶列日後被清理刪除時，轉帳仍查得到去向）。status：PENDING 已寫列、尚未呼叫或結果不明；SUCCEEDED provider 已建立轉帳（external_transfer_id）；FAILED 終局失敗（failure_reason）；ANOMALY 待人工結案（failure_reason 記原因，補掃不再重試）。
-- called_at 是這把鍵第一次呼叫 provider 之前寫入的時刻，語意同 payments.create_called_at；idempotency_key 是呼叫 provider 的冪等鍵，結果不明時每次重送沿用，
-- provider 已知拒絕（可重試）時換新鍵並清掉 called_at；它也是寫進 provider 物件的嘗試 ID，查回依它比對。lookup_failures 是連續查回失敗的次數。
-- 部分撤回：完成後退款要先撤回轉帳。一次撤回記在那筆退款列（refunds.reversal_amount、reversal_external_id），累計撤回額 reversed_amount 記在轉帳列，
-- 同一個 transaction 內鎖轉帳列後更新；reversed_amount 不超過 amount（CHECK transfers_reversed_check），剩餘可撤額是 amount - reversed_amount。賣家餘額不足不是錯誤，所以沒有對應的狀態。
CREATE TABLE payment.transfers (
    id                     UUID        PRIMARY KEY,
    payment_id             UUID        NOT NULL REFERENCES payment.payments (id),
    store_id               BIGINT      NOT NULL,
    amount                 BIGINT      NOT NULL,
    reversed_amount        BIGINT      NOT NULL DEFAULT 0,
    destination_account_id TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'PENDING',
    external_transfer_id   TEXT,
    failure_reason         TEXT,
    idempotency_key        UUID        NOT NULL DEFAULT gen_random_uuid(),
    called_at              TIMESTAMPTZ,
    lookup_failures        INTEGER     NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT transfers_lookup_failures_check CHECK (lookup_failures >= 0),
    CONSTRAINT transfers_store_id_check CHECK (store_id > 0),
    CONSTRAINT transfers_amount_check CHECK (amount > 0),
    CONSTRAINT transfers_reversed_check CHECK (reversed_amount BETWEEN 0 AND amount),
    CONSTRAINT transfers_destination_check CHECK (char_length(destination_account_id) BETWEEN 1 AND 255),
    CONSTRAINT transfers_status_check CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED', 'ANOMALY')),
    CONSTRAINT transfers_external_id_check CHECK (external_transfer_id IS NULL OR char_length(external_transfer_id) BETWEEN 1 AND 255),
    CONSTRAINT transfers_succeeded_check CHECK (status <> 'SUCCEEDED' OR external_transfer_id IS NOT NULL),
    CONSTRAINT transfers_external_state_check CHECK (external_transfer_id IS NULL OR status IN ('SUCCEEDED', 'FAILED')),
    CONSTRAINT transfers_reversal_only_when_succeeded_check CHECK (status = 'SUCCEEDED' OR reversed_amount = 0),
    CONSTRAINT transfers_called_check CHECK (external_transfer_id IS NULL OR called_at IS NOT NULL),
    CONSTRAINT transfers_failure_check CHECK ((status IN ('FAILED', 'ANOMALY')) = (failure_reason IS NOT NULL)),
    CONSTRAINT transfers_failure_reason_check CHECK (failure_reason IS NULL OR char_length(failure_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX transfers_active_payment ON payment.transfers (payment_id) WHERE status IN ('PENDING', 'SUCCEEDED', 'ANOMALY');
CREATE UNIQUE INDEX transfers_external ON payment.transfers (external_transfer_id) WHERE external_transfer_id IS NOT NULL;
CREATE INDEX transfers_pending ON payment.transfers (updated_at, id) WHERE status = 'PENDING';

-- 退款：一筆付款可以同時有多筆進行中的退款（D4），累計上限（非失敗退款加總不超過付款金額，ANOMALY 也算）由退款 use case 在同 transaction 鎖付款列後加總來守
-- （跨列加總，單列 CHECK 守不住；鎖順序：付款列先於退款列與轉帳列）。request_key 是呼叫端給的冪等鍵，(payment_id, request_key) 唯一：同一把鍵重送得到同一列。
-- auto 標示自動退款（D5 修訂：由訂單一方發起，payment 只標記原因，完成時發 AutoRefundCompleted）。
-- 狀態機：PENDING 已寫列；已轉帳的付款要先撤回再退款：REVERSING 撤回中（reversal_amount 是這筆要撤的額度，部分撤回；
-- 撤回的冪等鍵 reversal_idempotency_key，呼叫前記 reversal_called_at），撤回成功記 reversal_external_id 並進 REVERSED（「等待撤回」與「已呼叫退款」之間的中間狀態：
-- 重啟時從這裡接著呼叫退款，不再撤回）；不需要撤回的退款直接 PENDING → REFUNDING。REFUNDING 已呼叫退款（refund_called_at，冪等鍵 idempotency_key），
-- provider 的結果 SUCCEEDED（external_refund_id）或 FAILED（failure_reason，不佔額度）；ANOMALY 待人工結案（failure_reason 記原因，佔額度，補掃不再重試）。
-- reversal_lookup_failures 是撤回連續查回失敗的次數；reversal_idempotency_key 同時是撤回的嘗試 ID。reversal_amount 在 REVERSED 與之後的狀態必須有對應的 reversal_external_id（當 reversal_amount > 0）。
CREATE TABLE payment.refunds (
    id                       UUID        PRIMARY KEY,
    payment_id               UUID        NOT NULL REFERENCES payment.payments (id),
    request_key              TEXT        NOT NULL,
    amount                   BIGINT      NOT NULL,
    auto                     BOOLEAN     NOT NULL DEFAULT false,
    status                   TEXT        NOT NULL DEFAULT 'PENDING',
    reversal_amount          BIGINT      NOT NULL DEFAULT 0,
    reversal_idempotency_key UUID        NOT NULL DEFAULT gen_random_uuid(),
    reversal_called_at       TIMESTAMPTZ,
    reversal_external_id     TEXT,
    reversal_lookup_failures INTEGER     NOT NULL DEFAULT 0,
    idempotency_key          UUID        NOT NULL DEFAULT gen_random_uuid(),
    refund_called_at         TIMESTAMPTZ,
    external_refund_id       TEXT,
    failure_reason           TEXT,
    created_at               TIMESTAMPTZ NOT NULL,
    updated_at               TIMESTAMPTZ NOT NULL,
    CONSTRAINT refunds_request_key_check CHECK (char_length(request_key) BETWEEN 1 AND 128),
    CONSTRAINT refunds_amount_check CHECK (amount > 0),
    CONSTRAINT refunds_reversal_amount_check CHECK (reversal_amount BETWEEN 0 AND amount),
    CONSTRAINT refunds_status_check CHECK (status IN ('PENDING', 'REVERSING', 'REVERSED', 'REFUNDING', 'SUCCEEDED', 'FAILED', 'ANOMALY')),
    CONSTRAINT refunds_lookup_failures_check CHECK (reversal_lookup_failures >= 0),
    CONSTRAINT refunds_reversing_check CHECK (status <> 'REVERSING' OR reversal_amount > 0),
    CONSTRAINT refunds_reversed_check CHECK (status NOT IN ('REVERSED', 'REFUNDING', 'SUCCEEDED') OR reversal_amount = 0 OR reversal_external_id IS NOT NULL),
    CONSTRAINT refunds_reversal_called_check CHECK (reversal_external_id IS NULL OR reversal_called_at IS NOT NULL),
    CONSTRAINT refunds_reversal_external_id_check CHECK (reversal_external_id IS NULL OR char_length(reversal_external_id) BETWEEN 1 AND 255),
    CONSTRAINT refunds_refund_called_check CHECK (status NOT IN ('REFUNDING', 'SUCCEEDED') OR refund_called_at IS NOT NULL),
    CONSTRAINT refunds_succeeded_check CHECK (status <> 'SUCCEEDED' OR external_refund_id IS NOT NULL),
    CONSTRAINT refunds_external_state_check CHECK (external_refund_id IS NULL OR status IN ('REFUNDING', 'SUCCEEDED', 'FAILED')),
    CONSTRAINT refunds_external_called_check CHECK (external_refund_id IS NULL OR refund_called_at IS NOT NULL),
    CONSTRAINT refunds_external_id_check CHECK (external_refund_id IS NULL OR char_length(external_refund_id) BETWEEN 1 AND 255),
    CONSTRAINT refunds_failure_check CHECK ((status IN ('FAILED', 'ANOMALY')) = (failure_reason IS NOT NULL)),
    CONSTRAINT refunds_failure_reason_check CHECK (failure_reason IS NULL OR char_length(failure_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX refunds_request ON payment.refunds (payment_id, request_key);
CREATE UNIQUE INDEX refunds_external ON payment.refunds (external_refund_id) WHERE external_refund_id IS NOT NULL;
CREATE INDEX refunds_payment ON payment.refunds (payment_id);
CREATE INDEX refunds_reversing ON payment.refunds (updated_at, id) WHERE status = 'REVERSING';
