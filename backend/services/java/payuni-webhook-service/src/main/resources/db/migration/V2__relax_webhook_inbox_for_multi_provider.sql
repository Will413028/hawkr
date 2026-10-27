-- webhook_inbox 由「PayUni 專用」放寬為「多 provider 共用」（66 §5.1.6、65 §4.5.1 代價 ①③）
--
-- 背景：Uber Direct 的 webhook 沿用本表，不新建 uber_webhook_inbox ——
-- 表本來就有 provider 維度（V1 逐字 `CONSTRAINT uq_inbox_event UNIQUE (provider, webhook_type, event_key)`）。
-- 但 V1 有兩個「只有 PayUni 才成立」的假設，不處理的話第一筆 Uber webhook 就寫不進來。

-- ① mer_id：PayUni 的商家代號。Uber 沒有這個概念 ⇒ NOT NULL 會讓第一筆就 23502。
--
--    為什麼放寬成 nullable 而不是塞一個 sentinel（例如 'UBER'）：
--    sentinel 會讓「這一列有沒有 MerID」變成要比對字面值才知道，而那個字面值必然散落到
--    查詢與報表裡；NULL 讓「不適用」在型別層就成立。
--
--    相容性：這是放寬（expand），不是收縮 —— 既有列一律保留原值，舊版 pod 仍會寫入非空值，
--    新舊並存的窗口內兩邊都合法，不需要回填。
--    NULL 對既有查詢的影響：實查全 repo，**沒有任何 application code 以 mer_id 過濾 webhook_inbox**
--    （唯一命中是 tests/e2e-runs 的歷史 run 腳本，且那是 projection 不是 filter）。
--    若日後新增 `WHERE mer_id = ?`，SQL 三值邏輯天然排除 NULL 列 ⇒ PayUni 的查詢不會撈到 Uber 的列。
ALTER TABLE webhook_inbox ALTER COLUMN mer_id DROP NOT NULL;

-- ② 欄位註解：V1 的 `-- PayUniEventType value（8251-8254）` 是 SQL 行內註解，
--    **只存在於 migration 檔的文字裡，從未進入 pg_description**（V1 全檔零 COMMENT）。
--    ⇒ 這裡是「新增」欄位註解而不是「覆蓋」，V1 檔內那句過時文字則因 Flyway V1 凍結而動不得
--    （改一個空白就破 checksum），它的更正寫在 36 §3.5 與設計書 10 §3。
COMMENT ON COLUMN webhook_inbox.provider IS
    'WebhookProvider 的 Value（數字碼字串，例：PayUni=8241）。不是 code name —— 寫 ''UBER'' 會讓 consumer 拿字串去比數字碼（66 §6.3.3）。';

COMMENT ON COLUMN webhook_inbox.mer_id IS
    'PayUni 商家代號（HAWK<store_id>）。僅 provider=PayUni 適用；其他 provider 為 NULL，不得填 sentinel。';

COMMENT ON COLUMN webhook_inbox.webhook_type IS
    'provider 原生事件型別，值域依 provider 而異：PayUni 為 PayUniEventType 的 Value（8251-8254）；Uber 為 webhook 的原生 kind（例：event.delivery_status）。本欄刻意不做跨 provider 的統一正規化 —— 正規化是 consumer 側 InboundCapability 的職責。';
