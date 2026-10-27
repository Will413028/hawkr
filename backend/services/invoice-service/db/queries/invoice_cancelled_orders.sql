-- invoice_cancelled_orders｜訂單已取消（買家面 state 的終態事實）
-- SSOT：設計書 58 號 §5.1.9a ＋ §6.1.3；緣由 issue #2904
--
-- 只有兩支查詢：一支由 `order.cancelled` 的 consumer 寫、一支由端點 8 的讀路徑查。
-- 本表沒有 status 欄也沒有排程欄 ⇒ 沒有任何 worker 掃它，不存在「卡在某個狀態」的形狀。

-- name: InsertCancelledOrder :execrows
-- 走 ON CONFLICT DO NOTHING，**刻意不指名 conflict target**。
--
-- 不指名的理由與 InsertInboxEvent 同一條（§11.1）：指名某一個唯一約束之後，若日後
-- 這張表多出第二個唯一約束，撞到另一個時會拋 23505 而讓**整個交易 aborted（25P02）**，
-- 之後每一句 SQL 都被拒。DO NOTHING 不指名時對任何唯一約束衝突都靜默跳過。
--
-- 回 :execrows 而不是 :exec：rows=0 是「這張訂單已經記過了」，那是正常的去重結果
-- （at-least-once 重投），呼叫端要能分辨它與「真的寫進去了」，而不是兩者都當成成功
-- 卻無從得知發生了哪一件。
INSERT INTO invoice_cancelled_orders (order_no, buyer_id, cancel_reason, occurred_at, created_at)
VALUES (sqlc.arg(order_no), sqlc.arg(buyer_id), sqlc.narg(cancel_reason), sqlc.arg(occurred_at), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: IsOrderCancelledForBuyer :one
-- 端點 8 的讀路徑：這張訂單對**這位買家**而言是不是已取消。
--
-- 🔴 **buyer_id 是授權述詞不是過濾條件**（§8.1.4 Tier A ／ E21）。少了它，任何登入
-- 買家拿別人的 order_no 來問都會得到 true ⇒ 訂單號可枚舉 ⇒ 憑空多一個訂單存在性
-- oracle。同一支端點的另一半（ListOrderInvoiceDocumentsForBuyer）本來就雙欄過濾，
-- 這一半不跟上就是把它繞開，而症狀是零錯誤、零 log、畫面完全正常。
--
-- 回 EXISTS 而不是回整列：讀路徑只需要一個布林，把整列撈上來只會讓「將來有人順手
-- 把 cancel_reason 顯示給買家」變得容易——那是內部代碼（含 SELLER_SUSPENDED），
-- 不在 §B04 的任何一格文案裡。
SELECT EXISTS (
  SELECT 1 FROM invoice_cancelled_orders
  WHERE order_no = sqlc.arg(order_no) AND buyer_id = sqlc.arg(buyer_id)
);
