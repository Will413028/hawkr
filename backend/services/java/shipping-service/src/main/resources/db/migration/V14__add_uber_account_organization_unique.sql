-- V14：uber_account／uber_organization 的唯一性約束（66 號詳細設計書 UP-20 結案）
--
-- 由架構裁決 DEC-20260912-184025 第 16 條定案：這是 Type A schema integrity，直接決定並實作。
-- 🔴 裁決逐字：「不得讓 application-only check 代替 DB constraint」。
--
-- 背景（66 UP-20）：V13 依「照 66 建、不發明約束」落地 ⇒ 兩張表**只有 PK**，
-- 而 §5.1.1 的「只會有 1～2 列」與 §5.1.2 的「一個賣家一個組織」都是文字上的唯一性宣告，
-- 從未被翻譯成約束。缺約束是**完全無聲**的：多出來的那一列不會報錯，
-- 只會讓某個查詢在某天取到另一列（而「哪一列」由 planner 決定，不是由我們決定）。
--
-- ⚠️ 現在補比以後補便宜：兩張表目前零列（寫入者在 66 的 W2）⇒ 加 UNIQUE 不需要先清資料。
--    等有列之後才加，就要先處理既有重複，而那些重複本身是無聲產生的。
--
-- 🔴 本檔**不動 V13**（已套用的 migration 唯讀，Flyway checksum）。

-- =====================================================================
-- 1. uber_account —— provider account ＋ environment 的 natural key
-- =====================================================================
-- 裁決第 16 條：「能唯一代表 provider account ＋ environment 的 business key 必須有 DB UNIQUE」。
-- 實查 V13 的 uber_account 欄位（id／env／client_id／organization_id／enabled／時戳）後認定：
--   provider account 的身分 ＝ client_id（OAuth2 client_credentials 的帳號本體）
--   environment       ＝ env
-- ⇒ natural business key ＝ (env, client_id)。
--
-- ⚠️ **不取 66 §5.1.1 提案的 UNIQUE (env)**：那會讓「保留一列停用的舊憑證」變成不可能，
--    而憑證輪替的正確順序正是「新舊並存 ⇒ 切換 ⇒ 舊列留著備查」。
--    UNIQUE (env) 會強迫輪替必須先刪舊列，等於把可回退的操作變成不可回退的。
ALTER TABLE uber_account
    ADD CONSTRAINT uk_uber_account_env_client UNIQUE (env, client_id);

-- 🔴 但 natural key 本身**解決不了 66 指出的那個問題**——它擋得住「同一組憑證登記兩次」，
--    擋不住「同一個 env 有兩列 enabled = true」，而後者才是「取 token 用哪一列」沒有唯一解的來源。
--    ⇒ 第二道：每個 env 至多一列 enabled。
--
-- 為什麼是 partial unique index 而不是把 enabled 塞進上面那個 UNIQUE：
--   UNIQUE (env, client_id, enabled) 允許同一 env 下兩列都是 enabled（只要 client_id 不同），
--   它守的是完全不同的東西。「至多一列 enabled」只能用 partial index 表達。
--
-- ⚠️ 它對維運的要求：憑證輪替必須**先停用舊列、再啟用新列**（同一個交易內兩句 UPDATE 即可）。
--    順序反了會被這道 index 擋下——那正是它存在的目的：
--    兩列同時 enabled 的那個瞬間，「這個環境的 Uber 帳號是哪一個」沒有答案。
CREATE UNIQUE INDEX uk_uber_account_one_enabled_per_env
    ON uber_account (env)
    WHERE enabled;

COMMENT ON CONSTRAINT uk_uber_account_env_client ON uber_account IS
    'natural business key：provider account（client_id）＋ environment（env）。'
    '擋「同一組憑證在同一環境登記兩次」。⚠️ 刻意不用 UNIQUE (env)——那會禁止保留停用的舊憑證列，'
    '使憑證輪替變成不可回退的操作（66 UP-20／裁決 DEC-20260912-184025 第 16 條）。';

-- =====================================================================
-- 2. uber_organization —— 一個賣家在一個 Uber 帳號下只有一個組織
-- =====================================================================
-- 裁決逐字：`UNIQUE (uber_account_id, store_id)`
--   理由：避免同一 account 下同一 Store 重複綁定，同時允許 sandbox／prod 透過不同 account 並存。
--
-- ⚠️ **不取 UNIQUE (store_id)**：那會讓同一個賣家在 sandbox 與 production 兩個 uber_account 下
--    無法並存，而 §5.1.1 的 env 欄正暗示兩者要能並存（66 §5.1.2 逐字）。
--
-- ⚠️ 與「賣家關店不刪列」相容：66 §5.1.2 的推走路徑是 status='SUSPENDED'（UPDATE），不是 DELETE
--    ⇒ 重新開通走 UPDATE 把同一列推回 ACTIVE，不會 INSERT 出第二列。
--    若有人改成「INSERT 一列新的」，本約束會當場擋下——那是對的，
--    因為兩列並存時「這個賣家的組織是哪一個」同樣沒有答案。
ALTER TABLE uber_organization
    ADD CONSTRAINT uk_uber_organization_account_store UNIQUE (uber_account_id, store_id);

COMMENT ON CONSTRAINT uk_uber_organization_account_store ON uber_organization IS
    '一個賣家在一個 Uber 帳號下只有一個組織（66 §5.1.2「一個賣家一個組織」的約束化）。'
    '⚠️ 刻意不是 UNIQUE (store_id)：同一賣家要能在 sandbox 與 prod 兩個 uber_account 下並存。';

-- =====================================================================
-- 3. 兩道約束合起來給出的保證（這才是 UP-20 真正要的東西）
-- =====================================================================
-- 「某個 env 下，某個賣家的 Uber 組織是哪一個」的解析鏈：
--   ① uk_uber_account_one_enabled_per_env  ⇒ 該 env 的啟用帳號至多一列  → 唯一的 uber_account_id
--   ② uk_uber_organization_account_store   ⇒ (那個 account_id, store_id) 至多一列 → 唯一的組織
-- 缺任一道，這條鏈在某一步就會有兩個答案，而兩個答案在程式裡長得跟一個答案一模一樣
-- （`selectOne` 取到其中一列，零錯誤零 log——同 memory reference_sqlc_one_returns_arbitrary_row 的形狀）。
--
-- ⚠️ idx_uber_organization_store（V13 建的**非唯一** B-tree）刻意保留：
--    uk_uber_organization_account_store 的前導欄是 uber_account_id，
--    對「只給 store_id」的查詢用不上，兩者不重複。
