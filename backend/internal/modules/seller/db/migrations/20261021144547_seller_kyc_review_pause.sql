-- 待審核商家認證與食品業者登錄在店家不是營運中（與商家認證在停權）時暫停（step10e-5b-design.md F3）：新增 seller.account_sanction_lifted_at，
-- 改寫 review_kyc_application 與 review_food_registration 的成員與開始時間。手寫（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同
-- （TestSchemaFileMatchesMigrations），註解見 objects.sql。兩支函式被 pub_review_items 依賴，用 CREATE OR REPLACE 保留依賴（回傳型別不變）。
-- 帳號最近一次停權結束的時間：帳號在 at 以前已經結束的停權與終止合作措施的結束時間的最大值（到期是 ends_at，撤銷是 revoked_at，兩者都有時取較早的）；
-- 只算真的生效過的（結束時間晚於 starts_at：還沒開始就被撤銷的不算）；沒有時是 NULL。「措施生效中」（effective_measures）的第五種讀法
-- （前四種是 EffectiveMeasures、EffectiveMeasuresAll、ScheduledMeasureIDs 與 Go 的 StateOf，見 AGENTS.md），只給 review_kyc_application 算停權期間不計的審核期限起點，
-- 由資料集測試在邊界（ends_at、revoked_at 等於 at 的瞬間）與 effective_measures 鎖住。
CREATE FUNCTION seller.account_sanction_lifted_at(lifted_account_id BIGINT, at TIMESTAMPTZ) RETURNS TIMESTAMPTZ
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT max(least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END))
    FROM seller.measures m
    WHERE m.account_id = lifted_account_id AND m.kind IN ('SUSPENSION', 'TERMINATION')
      AND least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END) <= at
      AND least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END) > m.starts_at;
END;

-- FOOD_REGISTRATION 審核食品業者登錄：現在可以裁定的（FoodRegistrationReview.CanApprove）：待審、店家營運中、店家目前的商家認證已核准；
-- 等商家認證的不在這裡。開始是 max(送出時間, 目前核准的商家認證的核准時間, 這個 stint 最近一次恢復營業的時間)（3b D8「從可以裁定時起算」；關店期間不裁定，恢復營業時重新起算），期限 168 小時。裁定本來就不看停權，所以停權不影響成員與開始時間。
CREATE OR REPLACE FUNCTION seller.review_food_registration(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'FOOD_REGISTRATION'::text, f.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, f.id, NULL::uuid, f.store_id, COALESCE(s.store_name, ''),
           greatest(f.submitted_at, k.decided_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = f.store_id)),
           greatest(f.submitted_at, k.decided_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = f.store_id)) + INTERVAL '168 hours'
    FROM seller.food_registrations f
    JOIN seller.stores s ON s.store_id = f.store_id
    JOIN seller.kyc_applications k ON k.store_id = f.store_id AND k.status = 'APPROVED'
    WHERE f.state = 'PENDING' AND s.status = 'ACTIVE';
END;

-- KYC_APPLICATION 審核商家認證：審核中（SUBMITTED）、店家營運中（已關閉與等待刪除時審核暫停）、帳號沒有生效中的停權（停權時不能核准，審核暫停；退回雖然可用，件不在待辦）。
-- 成員對應的 Go 定義是 APPROVE_KYC 的原因清單為空（model.KYCActionUnavailableReasons）：身分命中別的帳號生效中的處分不在原因裡，照舊留在待辦讓員工退回；
-- 商家清單的「待審核」分頁與統計讀這個函式，所以待審核只有這一個定義。開始是 max(送出時間, 這個 stint 最近一次恢復營業的時間, 帳號最近一次停權結束的時間)，
-- 暫停的期間不算審核期限，期限 168 小時。
CREATE OR REPLACE FUNCTION seller.review_kyc_application(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'KYC_APPLICATION'::text, k.application_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, k.application_id, k.store_id, COALESCE(s.store_name, ''),
           greatest(k.submitted_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = k.store_id), seller.account_sanction_lifted_at(s.account_id, at)),
           greatest(k.submitted_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = k.store_id), seller.account_sanction_lifted_at(s.account_id, at)) + INTERVAL '168 hours'
    FROM seller.kyc_applications k
    JOIN seller.stores s ON s.store_id = k.store_id
    WHERE k.status = 'SUBMITTED' AND s.status = 'ACTIVE'
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = s.account_id);
END;
