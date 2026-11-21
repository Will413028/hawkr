-- 店家寄出的訊息的內容下架（seller 的措施，A6-D21）：im 不存複本，讀取述詞直接 join seller.pub_content_takedowns()；訂閱者只鎖訊息列、核對歸屬並發 NOTIFY。

-- name: LockMessagesForTakedown :many
-- 依 id 升序鎖住存在的訊息列（不存在的列不回，已抹除的列仍在）。
SELECT m.id, m.conversation_id, c.buyer_id, c.store_id, m.seq, m.sender_kind FROM im.messages AS m
JOIN im.conversations AS c ON c.id = m.conversation_id
WHERE m.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY m.id
FOR NO KEY UPDATE OF m;
