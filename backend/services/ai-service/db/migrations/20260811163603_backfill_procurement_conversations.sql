-- 回填：procurement_conversations 表存在但直到 Task 2 接線前 100% 死碼，
-- 所有本次部署前建立的 procurement_requests 都沒有對應的 conversation row，
-- 會讓 AppendRevision 交易尾端的 GetProcurementConversationByRequestID
-- 查詢吃到 pgx.ErrNoRows。id 直接借用 r.id（request 自己的 TSID）而非另外
-- 產生新值：兩者是不同表各自的 PK namespace，用同一個數值不會撞號；
-- created_at 也借用 request 的建立時間，維持時間排序語意正確
-- （不用 now()，否則舊對話會在依 id DESC 排序的列表裡看起來像剛建立）。
-- uq_procurement_conversations_request 保證這個 INSERT 對同一個 request
-- 重跑也不會重複處理。
INSERT INTO procurement_conversations (id, buyer_id, request_id, title, title_source, created_at, updated_at)
SELECT r.id, r.buyer_id, r.id, '', 'AUTO', r.created_at, r.created_at
FROM procurement_requests r
WHERE NOT EXISTS (
    SELECT 1 FROM procurement_conversations c WHERE c.request_id = r.id
);
