-- name: RetireDueStores :many
-- 關店後兩段清理的時間條件，只有這一處（second_stage 選哪一段）：
--   第一段（退場，second_stage = false）：已關閉、還沒退場；hours 是 model.StoreRetireAfterHours（30 天）。恢復營業問的「保留期是否已過」也是它
--   （closing.ReopenPlan，加上 StoreHolds 的申覆 hold）。
--   第二段（個資清理，second_stage = true）：已退場、個資還沒清理；hours 是 model.StoreRetentionHours（184 天，詐欺犯罪危害防制條例第 37 條第 1 項）。
-- 兩段都要現在晚於 due_after：因終止合作而關閉的是 greatest(關店後 hours, 帳號上未撤銷的終止合作中最晚的申覆期限)——申覆期限是措施的 starts_at 加
-- appeal_window_hours，帳號可以有多筆終止（關店之後才發現的 21.3 事由、關聯帳號的終止），保留到最晚的申覆期限，已撤銷的不延長（造成關店的那一筆撤銷後
-- 也不再延長）；其他關閉是關店後 hours。時數由呼叫方給（Go 的常數是唯一來源）。保存條件（申覆的 legal hold、來函的保存）不在這裡：只有 StoreHolds 一處；
-- 清理在鎖住候選之後以同一支查詢再判斷一次時間，再問 StoreHolds。store_ids 為 NULL 時不限店家（清理的批次：SKIP LOCKED，正被別的流程鎖住的列這一輪略過；
-- after_store_id 是 keyset，被保存而略過的店家不會讓下一批一直選到同一批）；給定時只回其中符合的。
SELECT s.store_id, s.store_code, s.account_id, s.logo_asset_id, s.cover_asset_id
FROM seller.stores s
JOIN seller.store_closures c ON c.store_id = s.store_id AND c.reopened_at IS NULL
WHERE s.status = 'CLOSED'
  AND CASE WHEN sqlc.arg(second_stage)::boolean THEN s.retired_at IS NOT NULL AND s.pii_purged_at IS NULL ELSE s.retired_at IS NULL END
  AND (sqlc.narg(store_ids)::bigint[] IS NULL OR s.store_id = ANY(sqlc.narg(store_ids)::bigint[]))
  AND (sqlc.narg(after_store_id)::bigint IS NULL OR s.store_id > sqlc.narg(after_store_id)::bigint)
  AND sqlc.arg(now)::timestamptz > CASE
        WHEN c.closed_by = 'TERMINATION'
        THEN GREATEST(
          c.closed_at + make_interval(hours => sqlc.arg(hours)::int),
          (SELECT max(lt.starts_at) FROM seller.measures lt
           WHERE lt.account_id = s.account_id AND lt.kind = 'TERMINATION' AND lt.status = 'ENFORCED') + make_interval(hours => sqlc.arg(appeal_window_hours)::int))
        ELSE c.closed_at + make_interval(hours => sqlc.arg(hours)::int)
      END
ORDER BY s.store_id
LIMIT sqlc.arg(batch_limit)
FOR UPDATE OF s SKIP LOCKED;

-- name: StoreHolds :many
-- 保存條件的唯一定義：這些店家此刻被哪一種保存擋下（沒有被擋的不回）。
--   appeal_hold（legal hold）：帳號上未撤銷的終止合作措施有待處理的申覆，或帳號上未撤銷的關聯終止（LINKED_ACCOUNT）的來源措施有待處理的申覆（來源撤銷時
--   關聯終止同一個 transaction 連帶撤銷，所以未撤銷的關聯終止的來源也一定未撤銷）：帳號全部已關閉的 stint 都擋。擋兩段清理，也讓恢復營業視為
--   「保留期還沒過」（爭議還沒結束，店家應該還能恢復）。
--   preservation_hold：帳號上有沒撤銷登記、hold_until 晚於 now 的主管機關來函指名（不指定店家，或就是這家店）。擋兩段清理，
--   不影響恢復營業（第三方的保存義務只凍結刪除）。
-- 清理在鎖住候選之後才問它（新的語句、新的快照）：寫入保存條件的一方都先鎖同一組店家列，所以候選鎖住之後不會再出現新的保存（step10e-3cb §2.4）。
SELECT s.store_id,
       EXISTS (
         SELECT 1
         FROM seller.measures tm
         WHERE tm.account_id = s.account_id AND tm.kind = 'TERMINATION' AND tm.status = 'ENFORCED'
           AND EXISTS (
             SELECT 1 FROM seller.measure_requests r
             WHERE r.state = 'PENDING' AND r.measure_id IN (tm.id, tm.source_measure_id)
           )
       )::boolean AS appeal_hold,
       EXISTS (
         SELECT 1
         FROM seller.authority_order_targets pt
         JOIN seller.authority_orders po ON po.id = pt.order_id
         WHERE pt.account_id = s.account_id AND (pt.store_id IS NULL OR pt.store_id = s.store_id)
           AND po.revoked_at IS NULL AND po.hold_until > sqlc.arg(now)::timestamptz
       )::boolean AS preservation_hold
FROM seller.stores s
WHERE s.store_id = ANY(sqlc.arg(store_ids)::bigint[])
ORDER BY s.store_id;

-- name: RetireStores :many
-- 第一段清理：清掉店家的公開內容（說明、獎項、網站）並記下 retired_at，同時釋放 logo 與封面（呼叫方把它們記成退場，引用由同步 job 解除）。
-- 之後不能恢復營業，呼叫方發 StoreRetired（下游刪除商品、貼文、聊天的個資等）。時間與保存的判斷在 RetireDueStores（第一段）與 StoreHolds（候選已被鎖住）；
-- stores_retired_check 是保底。店名與聯絡資料留到第二段。
UPDATE seller.stores
SET description = NULL,
    prize = NULL,
    website_url = NULL,
    logo_key = NULL,
    logo_asset_id = NULL,
    cover_key = NULL,
    cover_asset_id = NULL,
    retired_at = sqlc.arg(now),
    updated_at = sqlc.arg(now)
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND status = 'CLOSED' AND retired_at IS NULL
RETURNING store_id;

-- name: PurgeStores :many
-- 第二段清理：清掉店家列的註冊資料（店名、聯絡、地址）並記下 pii_purged_at。只作用於已退場的店家；時間與保存的判斷在 RetireDueStores（第二段）與 StoreHolds
-- （候選已被鎖住）；stores_pii_purged_check 是保底。
UPDATE seller.stores
SET store_name = NULL,
    contact_phone = NULL,
    contact_name = NULL,
    contact_mobile = NULL,
    support_email = NULL,
    town_code = NULL,
    address_detail = NULL,
    pii_purged_at = sqlc.arg(now),
    updated_at = sqlc.arg(now)
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND retired_at IS NOT NULL AND pii_purged_at IS NULL
RETURNING store_id, store_code;

-- name: PurgeKYCApplications :many
-- 清掉這些店家的 KYC 申請的個人欄位（全部狀態）。只作用於店家列已清理的申請：保留期限的判斷只有 PurgeStores 一處。
-- 審核的事實（status、時間、kyc_reviews）、seller_type、product_type 保留。回傳被清理的申請，
-- 呼叫方刪證件並把證件記成退場。
UPDATE seller.kyc_applications a
SET real_name = NULL,
    birth_date = NULL,
    id_card_no_enc = NULL,
    id_card_hash = NULL,
    id_card_hash_key = NULL,
    id_card_issue_date = NULL,
    id_card_issue_type = NULL,
    id_card_issue_location = NULL,
    english_name = NULL,
    bank_name = NULL,
    bank_code = NULL,
    bank_account_enc = NULL,
    bank_account_hash = NULL,
    bank_account_hash_key = NULL,
    website_url = NULL,
    company_name = NULL,
    tax_id = NULL,
    tax_id_hash = NULL,
    tax_id_hash_key = NULL,
    company_register_date = NULL,
    company_town_code = NULL,
    company_address_detail = NULL,
    company_phone = NULL,
    rep_id_card_no_enc = NULL,
    rep_id_card_hash = NULL,
    rep_id_card_hash_key = NULL,
    pii_purged_at = sqlc.arg(now)
FROM seller.stores s
WHERE a.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = a.store_id AND s.pii_purged_at IS NOT NULL AND a.pii_purged_at IS NULL
RETURNING a.application_id;

-- name: PurgeKYCIdentityHashes :exec
-- 清掉已清理的申請的比對 hash（blind index 與申請的個人欄位同時清除；措施的身分快照是措施擁有的，不在這裡）。
DELETE FROM seller.kyc_identity_hashes WHERE application_id = ANY(sqlc.arg(application_ids)::uuid[]);

-- name: PurgeKYCDocuments :many
-- 刪掉已清理的申請的證件列（asset ID 與 object key），回傳刪掉的 asset：呼叫方把它們記成退場，media 的引用由同步 job 解除。
DELETE FROM seller.kyc_documents
WHERE application_id = ANY(sqlc.arg(application_ids)::uuid[])
RETURNING application_id, asset_id;

-- name: PurgePayUniRegistrations :execrows
-- 清掉這些店家的 PayUni 註冊的 reason；狀態碼、outcome 與綁定狀態保留。同樣只作用於店家列已清理的註冊。
UPDATE seller.payuni_registrations r
SET reason = NULL,
    pii_purged_at = sqlc.arg(now)
FROM seller.stores s
WHERE r.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = r.store_id AND s.pii_purged_at IS NOT NULL AND r.pii_purged_at IS NULL;

-- name: PurgePayUniObservations :execrows
-- 清掉這些店家的觀測紀錄的 reason（自由文字）；狀態碼、事件時間與 event_key 保留。
UPDATE seller.payuni_observations o
SET reason = NULL
FROM seller.stores s
WHERE o.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = o.store_id AND s.pii_purged_at IS NOT NULL AND o.reason IS NOT NULL;
