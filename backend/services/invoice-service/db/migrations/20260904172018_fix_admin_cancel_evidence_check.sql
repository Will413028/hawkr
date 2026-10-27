-- 讓 chk_invop_admin_cancel_needs_confirm 對「兩欄皆 NULL」真的擋得住。
--
-- 🔴 舊寫法 `cancel_evidence_kind IN (...)` 在兩欄皆 NULL 時整條是
-- `FALSE OR FALSE OR NULL` = NULL，而 PostgreSQL 的 CHECK 只拒絕 FALSE、放行 NULL
-- ⇒ 它對自己要擋的那一格是 no-op（`convalidated = t`，不是「沒驗過的舊資料」）。
--
-- 先回填、再換約束：ADD CONSTRAINT 會驗既有列，不補值會直接失敗。
-- 回填的述詞刻意窄：**只補得出「確實是 J24 走到的」那些**
-- （status=CANCELLED_BY_ADMIN ＋ 兩欄皆 NULL ＋ 有 ADM 取消請求在途 ＝ J24 的定義，
--  見 §7.5.4 第 2 條的判別式）。不符合這個述詞的列會讓下面的 ADD CONSTRAINT 失敗
-- ——那是**刻意的 fail-loud**：那種列代表有一條我們還不知道的路徑寫出了它，
-- 憑猜補一個證據種類等於偽造稽核紀錄。
--
-- cancel_confirmed_at 一併補：少了它，「ADM 取消在途」判別式
-- （cancel_requested_at IS NOT NULL AND cancel_confirmed_at IS NULL）永遠成立，
-- 這些列在資料上會一直看起來像還沒處理完。取 updated_at 而不是 now()：
-- 那才是它真正被取消的時點。
UPDATE "invoice_operations"
SET "cancel_evidence_kind" = 'PROVIDER_QUERY_71',
    "cancel_confirmed_at" = COALESCE("cancel_confirmed_at", "updated_at")
WHERE "status" = 'CANCELLED_BY_ADMIN'
  AND "cancel_confirmed_by" IS NULL
  AND "cancel_evidence_kind" IS NULL
  AND "cancel_requested_at" IS NOT NULL;

-- Modify "invoice_operations" table
ALTER TABLE "invoice_operations" DROP CONSTRAINT "chk_invop_admin_cancel_needs_confirm", ADD CONSTRAINT "chk_invop_admin_cancel_needs_confirm" CHECK (((status)::text <> 'CANCELLED_BY_ADMIN'::text) OR (cancel_confirmed_by IS NOT NULL) OR ((COALESCE(cancel_evidence_kind, ''::character varying))::text = ANY ((ARRAY['PROVIDER_QUERY_71'::character varying, 'PROVIDER_LIST_ABSENT'::character varying])::text[])));
