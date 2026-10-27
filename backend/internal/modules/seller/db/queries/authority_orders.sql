-- name: InsertAuthorityOrder :one
-- 登記一份來函：同一位管理員的同一個 request_id 已經有時不寫入、回 0 列（不讓 transaction 失效，呼叫方接著讀原本那一份）。
INSERT INTO seller.authority_orders (
    id, agency, document_no, received_at, kind, summary, due_at, hold_until, registered_at, registered_by_admin_id, request_id, request_hash)
VALUES (
    sqlc.arg(id), sqlc.arg(agency), sqlc.arg(document_no), sqlc.arg(received_at), sqlc.arg(kind), sqlc.arg(summary), sqlc.arg(due_at),
    sqlc.arg(hold_until), sqlc.arg(now), sqlc.arg(admin_id), sqlc.narg(request_id), sqlc.narg(request_hash))
ON CONFLICT (registered_by_admin_id, request_id) DO NOTHING
RETURNING *;

-- name: AuthorityOrderByRequest :one
-- 冪等重送的查詢：範圍是 (管理員, request_id)。
SELECT *
FROM seller.authority_orders
WHERE registered_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: InsertAuthorityOrderTarget :exec
-- 一個指名的對象；告知在完成時才寫。店家為 NULL 是帳號全部的店家。
INSERT INTO seller.authority_order_targets (order_id, position, account_id, store_id, product_ids)
VALUES (sqlc.arg(order_id), sqlc.arg(position), sqlc.arg(account_id), sqlc.narg(store_id), sqlc.arg(product_ids)::text[]);

-- name: InsertAuthorityOrderPrepurgedStores :exec
-- 登記當下（已鎖住被指名帳號的店家列）個資已經清理的店家快照：涵蓋狀態 PURGED_BEFORE_REGISTRATION 的事實來源，與應用程式時鐘無關。
INSERT INTO seller.authority_order_prepurged_stores (order_id, store_id)
SELECT DISTINCT t.order_id, s.store_id
FROM seller.authority_order_targets t
JOIN seller.stores s ON s.account_id = t.account_id AND (t.store_id IS NULL OR s.store_id = t.store_id)
WHERE t.order_id = $1 AND s.pii_purged_at IS NOT NULL;

-- name: AuthorityOrderByID :one
SELECT *
FROM seller.authority_orders
WHERE id = $1;

-- name: LockAuthorityOrder :one
-- 鎖順序：店家列（被指名的帳號，account_id 升序）、通報列、案件列、詢問列、措施列之後，來函列只由完成與延長鎖。
SELECT *
FROM seller.authority_orders
WHERE id = $1
FOR UPDATE;

-- name: AuthorityOrderTargets :many
-- 一批來函的對象，依來函與位置；店名在個資已清理後是 NULL。
SELECT t.order_id, t.position, t.account_id, t.store_id, t.product_ids, t.notice, t.notified_at, t.notice_note, t.recheck_on,
       t.notice_recorded_at, t.notice_recorded_by_admin_id, t.prev_position
FROM seller.authority_order_targets t
WHERE t.order_id = ANY(sqlc.arg(order_ids)::uuid[])
ORDER BY t.order_id, t.position;

-- name: AuthorityTargetStores :many
-- 一批來函的對象在 seller 範圍內的店家（指名店家時只有那一家，不指定時是帳號全部的 stint），逐家列出；登記時的清理快照（purged_at_registration）與個資清理的時間決定涵蓋狀態（model.AuthorityOrder.StoreCoverageOf）。
-- closed 與 latest_of_account（帳號 store_id 最大的 stint，同 LatestStoreOfAccount）是建立措施的入口用的事實（model.TerminationTarget）。
SELECT t.order_id, t.position, s.store_id, s.store_name, s.pii_purged_at, (p.store_id IS NOT NULL)::boolean AS purged_at_registration,
       (s.status = 'CLOSED')::boolean AS closed,
       (s.store_id = (SELECT max(l.store_id) FROM seller.stores l WHERE l.account_id = s.account_id))::boolean AS latest_of_account
FROM seller.authority_order_targets t
JOIN seller.stores s ON s.account_id = t.account_id AND (t.store_id IS NULL OR s.store_id = t.store_id)
LEFT JOIN seller.authority_order_prepurged_stores p ON p.order_id = t.order_id AND p.store_id = s.store_id
WHERE t.order_id = ANY(sqlc.arg(order_ids)::uuid[])
ORDER BY t.order_id, t.position, s.store_id;

-- name: AdminAuthorityOrders :many
-- 來函清單。state 是 OPEN：還沒完成也沒撤銷，依期限舊到新（due_at, id）；COMPLETED：已完成、REVOKED：已撤銷登記，依收文時間新到舊；沒有 state：全部，依收文時間新到舊。
-- account_id 篩選指名這個帳號的來函。keyset 的位置（after_at, after_id）的意義隨排序：OPEN 是 (due_at, id) 之後，其他是 (received_at, id) 之前。
SELECT o.*
FROM seller.authority_orders o
WHERE (sqlc.narg(state)::text IS NULL
        OR (sqlc.narg(state)::text = 'OPEN' AND o.completed_at IS NULL AND o.revoked_at IS NULL)
        OR (sqlc.narg(state)::text = 'COMPLETED' AND o.completed_at IS NOT NULL)
        OR (sqlc.narg(state)::text = 'REVOKED' AND o.revoked_at IS NOT NULL))
  AND (sqlc.narg(account_id)::bigint IS NULL OR EXISTS (
    SELECT 1 FROM seller.authority_order_targets t WHERE t.order_id = o.id AND t.account_id = sqlc.narg(account_id)::bigint))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL
    OR (sqlc.narg(state)::text = 'OPEN' AND (o.due_at, o.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
    OR (sqlc.narg(state)::text IS DISTINCT FROM 'OPEN' AND (o.received_at, o.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)))
ORDER BY
    CASE WHEN sqlc.narg(state)::text = 'OPEN' THEN o.due_at END ASC,
    CASE WHEN sqlc.narg(state)::text = 'OPEN' THEN o.id END ASC,
    o.received_at DESC, o.id DESC
LIMIT sqlc.arg(page_size);

-- name: CompleteAuthorityOrder :one
-- 完成來函；只在還沒完成而且版本相符時寫入，否則 0 列。hold_until 是完成之後的保存期限：沒有變就帶目前的值；調取資料的來函完成時依提供時間延長（呼叫方另寫一列
-- DATA_PROVIDED 的延長歷史），所以只能不早於目前的值。version 前進一次。
UPDATE seller.authority_orders
SET completed_at = sqlc.arg(now)::timestamptz, completed_by_admin_id = sqlc.arg(admin_id)::bigint, resolution_summary = sqlc.arg(resolution_summary)::text,
    hold_until = sqlc.arg(hold_until)::timestamptz, version = version + 1
WHERE id = sqlc.arg(id) AND completed_at IS NULL AND revoked_at IS NULL AND version = sqlc.arg(expected_version) AND hold_until <= sqlc.arg(hold_until)::timestamptz
RETURNING *;

-- name: RevokeAuthorityOrder :one
-- 撤銷登記（登記錯誤）：只在還沒完成、還沒撤銷而且版本相符時寫入，否則 0 列。保存隨之解除（StoreHolds 不看已撤銷的來函），hold_until 不改（紀錄保留）。
UPDATE seller.authority_orders
SET revoked_at = sqlc.arg(now)::timestamptz, revoked_by_admin_id = sqlc.arg(admin_id)::bigint, revoke_reason = sqlc.arg(reason)::text, version = version + 1
WHERE id = sqlc.arg(id) AND completed_at IS NULL AND revoked_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: ExtendAuthorityOrderHold :one
-- 延長保存期限（只能延長：新值必須晚於目前）；版本相符才寫入，否則 0 列。version 前進。
UPDATE seller.authority_orders
SET hold_until = sqlc.arg(hold_until), version = version + 1
WHERE id = sqlc.arg(id) AND revoked_at IS NULL AND version = sqlc.arg(expected_version) AND hold_until < sqlc.arg(hold_until)::timestamptz
RETURNING *;

-- name: BumpAuthorityOrder :one
-- 記下告知讓來函的版本前進；版本相符才寫入，否則 0 列。
UPDATE seller.authority_orders
SET version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: InsertAuthorityHoldExtension :exec
-- 追加一列延長歷史（seq 接在最後一列之後）；呼叫方已鎖住來函列，所以 seq 不會撞。
INSERT INTO seller.authority_order_hold_extensions (order_id, seq, from_until, to_until, cause, reason, admin_id, created_at)
SELECT sqlc.arg(order_id)::uuid, COALESCE(max(e.seq), 0) + 1, sqlc.arg(from_until)::timestamptz, sqlc.arg(to_until)::timestamptz,
       sqlc.arg(cause)::text, sqlc.narg(reason)::text, sqlc.arg(admin_id)::bigint, sqlc.arg(now)::timestamptz
FROM seller.authority_order_hold_extensions e
WHERE e.order_id = sqlc.arg(order_id)::uuid;

-- name: AuthorityHoldExtensions :many
-- 一批來函的延長歷史，舊到新。
SELECT order_id, seq, from_until, to_until, cause, reason, admin_id, created_at
FROM seller.authority_order_hold_extensions
WHERE order_id = ANY(sqlc.arg(order_ids)::uuid[])
ORDER BY order_id, seq;

-- name: SetAuthorityTargetNotice :execrows
-- 完成來函時寫一個對象的告知（只寫還沒有告知的）。
UPDATE seller.authority_order_targets
SET notice = sqlc.arg(notice)::text, notified_at = sqlc.narg(notified_at), notice_note = sqlc.narg(notice_note), recheck_on = sqlc.narg(recheck_on),
    notice_recorded_at = sqlc.arg(now), notice_recorded_by_admin_id = sqlc.arg(admin_id)
WHERE order_id = sqlc.arg(order_id) AND position = sqlc.arg(position) AND notice IS NULL;

-- name: ReplaceWithheldAuthorityTargetNotice :execrows
-- 把暫不告知的告知換成新的值（已告知或新的確認日期）；只換目前是 WITHHELD 的，被取代的值由呼叫方先寫進歷史。
UPDATE seller.authority_order_targets
SET notice = sqlc.arg(notice)::text, notified_at = sqlc.narg(notified_at), notice_note = sqlc.narg(notice_note), recheck_on = sqlc.narg(recheck_on),
    notice_recorded_at = sqlc.arg(now), notice_recorded_by_admin_id = sqlc.arg(admin_id)
WHERE order_id = sqlc.arg(order_id) AND position = sqlc.arg(position) AND notice = 'WITHHELD';

-- name: InsertAuthorityTargetNoticeHistory :exec
-- 被取代的暫不告知（從目前列複製，seq 接在最後一列之後）；呼叫方已鎖住來函列。
INSERT INTO seller.authority_target_notice_history (
    order_id, position, seq, notice, notice_note, recheck_on, recorded_at, recorded_by_admin_id, replaced_at, replaced_by_admin_id)
SELECT t.order_id, t.position,
       (SELECT COALESCE(max(h.seq), 0) + 1 FROM seller.authority_target_notice_history h WHERE h.order_id = t.order_id AND h.position = t.position),
       t.notice, t.notice_note, t.recheck_on, t.notice_recorded_at, t.notice_recorded_by_admin_id, sqlc.arg(now)::timestamptz, sqlc.arg(admin_id)::bigint
FROM seller.authority_order_targets t
WHERE t.order_id = sqlc.arg(order_id) AND t.position = sqlc.arg(position) AND t.notice = 'WITHHELD';

-- name: AuthorityTargetNoticeHistory :many
-- 一個對象被取代的告知，舊到新。
SELECT order_id, position, seq, notice, notice_note, recheck_on, recorded_at, recorded_by_admin_id, replaced_at, replaced_by_admin_id
FROM seller.authority_target_notice_history
WHERE order_id = sqlc.arg(order_id)
ORDER BY position, seq;

-- name: OrdersNamingMeasureTarget :many
-- 這些來函之中，指名了（帳號 account_id 的）店家 store_id 的：不指定店家的對象帳號的任何店家都算，指定時只算那一家（model.AuthorityOrderTarget.Names）。
-- 建立措施驗證來源來函與認定引用的來函用；不存在的來函不會出現在結果裡，revoked 是來函已撤銷登記（不能當作來源）。來函列以 FOR SHARE 鎖住：
-- 撤銷登記鎖同一列（FOR UPDATE），所以措施不會在撤銷之後還記成它導出的（鎖順序：呼叫方已鎖住措施帳號的店家列，來函列在後）。
SELECT o.id, (o.revoked_at IS NOT NULL)::boolean AS revoked
FROM seller.authority_orders o
WHERE o.id = ANY(sqlc.arg(order_ids)::uuid[])
  AND EXISTS (
    SELECT 1 FROM seller.authority_order_targets t
    WHERE t.order_id = o.id AND t.account_id = sqlc.arg(account_id) AND (t.store_id IS NULL OR t.store_id = sqlc.arg(store_id)))
ORDER BY o.id
FOR SHARE OF o;

-- name: AuthorityOrderMeasureTargets :many
-- 這份來函導出的措施所在的（帳號, 店家）：與 ListMeasures(source_authority_order_id) 同一個定義（明確連結 ∪ 認定引用），不看措施的狀態（已撤銷的措施
-- 當時的理由書也是告知）。完成來函時驗證「由措施告知」用；定義與清單的一致由資料集測試鎖住。
SELECT DISTINCT m.account_id, m.store_id
FROM seller.measures m
WHERE m.source_authority_order_id = sqlc.arg(order_id) OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.arg(order_id));

-- name: InsertFindingAuthorityOrders :exec
-- 認定引用的來函（集合；只在建立措施的 transaction 寫入）。
INSERT INTO seller.finding_authority_orders (violation_id, order_id)
SELECT sqlc.arg(violation_id)::uuid, unnest(sqlc.arg(order_ids)::uuid[]);

-- name: ReviewAuthorityOrderByIDs :many
-- 這些來函的處理期限（資源的 deadline）。規則只有 seller.review_authority_order(at)（objects.sql）；已完成或撤銷的來函沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.authority_order_id::uuid AS authority_order_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_authority_order(sqlc.arg(at)::timestamptz) r
WHERE r.authority_order_id = ANY(sqlc.arg(ids)::uuid[]);
