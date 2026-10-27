-- name: GetPublicationInviteSellerView :one
-- p.version 是給站外業者做正式送出時的 CAS 用的：把報價綁在「業者當時看到的那一版
-- RFQ」上。站外業者從 email 連結進來，可能隔好幾天才回來填，期間買家改過 RFQ 的
-- 機率比站內賣家高 ⇒ 這道 CAS 對本條 lane 更重要，不是更不重要。
-- ⚠️ 不多一輪查詢：procurement_rfq_publications 本來就在下面 join 著。
SELECT a.seller_facing_payload_json, a.response_deadline, p.version AS publication_version
  FROM procurement_rfq_publication_invites i
  JOIN procurement_rfq_publications        p ON p.id = i.publication_id
  JOIN procurement_rfq_publication_approvals a ON a.id = p.approval_id
 WHERE i.id = $1;
