-- 刪帳與退場的抹除：每批一個 transaction，批次取值升序。ledger 隨 order_nets（ON DELETE CASCADE）、customer_tags 隨 customers 與 tags 一起刪。

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
