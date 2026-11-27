
-- name: DeleteOrderNetsByBuyer :execrows
DELETE FROM crm.order_nets WHERE order_no IN (
    SELECT o.order_no FROM crm.order_nets AS o WHERE o.buyer_id = sqlc.arg(buyer_id) ORDER BY o.order_no LIMIT sqlc.arg(max_rows)
);

-- name: DeleteOrderNetsByStore :execrows
DELETE FROM crm.order_nets WHERE order_no IN (
    SELECT o.order_no FROM crm.order_nets AS o WHERE o.store_id = sqlc.arg(store_id) ORDER BY o.order_no LIMIT sqlc.arg(max_rows)
);

-- name: DeleteCustomersByBuyer :execrows
DELETE FROM crm.customers AS c USING (
    SELECT s.store_id, s.buyer_id FROM crm.customers AS s WHERE s.buyer_id = sqlc.arg(buyer_id) ORDER BY s.store_id LIMIT sqlc.arg(max_rows)
) AS b WHERE c.store_id = b.store_id AND c.buyer_id = b.buyer_id;

-- name: DeleteCustomersByStore :execrows
DELETE FROM crm.customers AS c USING (
    SELECT s.store_id, s.buyer_id FROM crm.customers AS s WHERE s.store_id = sqlc.arg(store_id) ORDER BY s.buyer_id LIMIT sqlc.arg(max_rows)
) AS b WHERE c.store_id = b.store_id AND c.buyer_id = b.buyer_id;

-- name: DeleteTagsByStore :execrows
DELETE FROM crm.tags WHERE id IN (
    SELECT t.id FROM crm.tags AS t WHERE t.store_id = sqlc.arg(store_id) ORDER BY t.id LIMIT sqlc.arg(max_rows)
);

-- name: ListInactivePartiesWithData :many
SELECT p.kind::text AS kind, p.party_id::bigint AS party_id FROM (
    SELECT 'BUYER' AS kind, cb.buyer_id AS party_id FROM buyer.pub_closed_buyers() AS cb
    WHERE EXISTS (SELECT 1 FROM crm.customers AS c WHERE c.buyer_id = cb.buyer_id)
       OR EXISTS (SELECT 1 FROM crm.order_nets AS o WHERE o.buyer_id = cb.buyer_id)
    UNION ALL
    SELECT 'STORE' AS kind, rs.store_id AS party_id FROM seller.pub_retired_stores() AS rs
    WHERE EXISTS (SELECT 1 FROM crm.customers AS c WHERE c.store_id = rs.store_id)
       OR EXISTS (SELECT 1 FROM crm.order_nets AS o WHERE o.store_id = rs.store_id)
       OR EXISTS (SELECT 1 FROM crm.tags AS t WHERE t.store_id = rs.store_id)
) AS p
WHERE (p.kind, p.party_id) > (sqlc.arg(after_kind)::text, sqlc.arg(after_id)::bigint)
ORDER BY p.kind, p.party_id
LIMIT sqlc.arg(max_rows);
