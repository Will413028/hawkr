-- invoice_status_history｜狀態轉移稽核
-- SSOT：設計書 58 號 §5.1.13
-- 本波放骨架 ＋ 一支 append：每一條走 transition helper 的狀態轉移都要在**同一交易內**留一列，
-- 這是 §7 全部狀態機共用的留痕落點（含 T16「我看過了但還不能決定」的自轉移，from_status = to_status）。

-- name: InsertInvoiceStatusHistory :exec
-- from_status NULL ＝ 建立。correlation_id 對齊 pkg/middleware/grpc/interceptor 的同名欄，
-- ADM 稽核 UI 要靠它一鍵對到 Loki 與 Tempo。
INSERT INTO invoice_status_history (
  subject_type, subject_id, from_status, to_status, reason_code, reason_detail,
  actor_type, actor_account_id, correlation_id, occurred_at)
VALUES (
  sqlc.arg(subject_type), sqlc.arg(subject_id), sqlc.narg(from_status), sqlc.arg(to_status),
  sqlc.narg(reason_code), sqlc.narg(reason_detail),
  sqlc.arg(actor_type), sqlc.narg(actor_account_id), sqlc.narg(correlation_id), sqlc.arg(now));

-- name: ListInvoiceStatusHistory :many
SELECT * FROM invoice_status_history
WHERE subject_type = sqlc.arg(subject_type) AND subject_id = sqlc.arg(subject_id)
ORDER BY occurred_at;
