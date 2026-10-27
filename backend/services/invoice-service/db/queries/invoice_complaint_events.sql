-- invoice_complaint_events｜申訴時間軸
-- SSOT：設計書 58 號 §5.1.12
-- 本波只放骨架：申訴回覆／升級／結案的事件寫入在本波範圍外。

-- name: ListInvoiceComplaintEvents :many
SELECT * FROM invoice_complaint_events
WHERE complaint_id = sqlc.arg(complaint_id)
ORDER BY occurred_at;

-- name: GetInvoiceComplaintEventByID :one
SELECT * FROM invoice_complaint_events WHERE id = sqlc.arg(id);

-- name: InsertInvoiceComplaintEvent :one
-- 申訴時間軸的唯一寫入點（append-only）。賣家回覆採**追加事件、不覆蓋前次**（K02）。
-- chk_invcompev_actor_id：actor_type <> 'SYSTEM' ⇒ actor_account_id 不得為空。
-- ⚠️ `action` 只寫 InvoiceComplaintAction 的 **6 個生成常數**（I-60），不是 ADM 端點的
--    request 動詞——那是兩張碼表，混用時 DB 不會報錯（欄位是 varchar(24)），
--    只有 ADM 畫面會顯示不出來。
-- ⚠️ 買家發起的三條轉移（K07 端點 63 ⇒ ESCALATED、K11 端點 64 ⇒ WITHDRAWN、
--    K13 端點 65 ⇒ REOPENED）的 `action` 欄一律 **NULL**：那 6 值是「賣家／ADM 在畫面上
--    點了哪一顆」，買家的動作不進那個軸。
-- attachment_object_keys 只存 objectKey，禁 URL／presigned／JWT（§25）。
INSERT INTO invoice_complaint_events (
  complaint_id, event_type, action, actor_type, actor_account_id,
  message, attachment_object_keys, occurred_at)
VALUES (
  sqlc.arg(complaint_id), sqlc.arg(event_type), sqlc.narg(action),
  sqlc.arg(actor_type), sqlc.narg(actor_account_id),
  sqlc.narg(message), sqlc.arg(attachment_object_keys), sqlc.arg(now))
RETURNING *;
