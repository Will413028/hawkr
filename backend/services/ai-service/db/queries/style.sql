-- name: ListAiStyles :many
SELECT * FROM ai_styles
WHERE sqlc.arg(category)::text = '' OR category = sqlc.arg(category)::text
ORDER BY sort_order ASC, id ASC;

-- name: GetAiStyleByID :one
SELECT * FROM ai_styles WHERE id = $1;

-- name: CreateAiStyle :one
INSERT INTO ai_styles (id, name, label, prompt, thumbnail_key, sort_order, is_default, enabled, category)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ClearCategoryDefaultStyle :execrows
-- 清除範圍由「目標所屬分類」決定，與 idx_ai_styles_default 的述詞 (category) 同源
-- （取代原本不分 category 的 ClearAllDefaultStyles）。必須先於
-- MarkAiStyleDefault 執行、兩者包在同一個 uc.txManager.WithTx
-- 交易內（見 style_usecase.go）：這句只會把 is_default 從 true 改成 false，
-- 不會讓任何一列變成 true，所以不可能撞 idx_ai_styles_default 這個 partial
-- unique index。曾經嘗試過用一句「is_default = (id = sqlc.arg(id))」的
-- CASE 式單一敘述同時搬移整個分類，理論上（單一敘述內對外不可見中間狀態）該是
-- atomic，但實測會間歇性撞 23505 duplicate key：同一句 UPDATE 掃描多列時，
-- Postgres 對 partial unique index 是逐列即時檢查、不是等整句跑完才驗證，若掃描
-- 順序先碰到「即將變成 true」的目標列、後碰到「即將變成 false」的舊預設列，
-- 目標列插入索引當下舊列仍是 true，直接違反唯一性。拆成「先全部清空
-- （只降不升，不可能撞）」再「才把目標設成 true（此時分類內已無 true，不可能
-- 撞）」兩句，順序由 Go 呼叫端保證，兩句對外仍是同一個 transaction 的原子效果。
--
-- 這個「不可能撞」只涵蓋同一句敘述內的掃描順序，不涵蓋跨交易的並發：兩個不同
-- transaction 同時把同一分類的兩個不同 style 都設成預設，仍可能在 READ
-- COMMITTED 下都通過這句的檢查、然後在各自的 MarkAiStyleDefault 撞
-- idx_ai_styles_default 的 23505，其中一個會整個交易一起回滾。這是已知且接受
-- 的風險（admin-only 操作、invariant 不會被破壞、失敗方乾淨回滾），刻意不加鎖或
-- retry。
UPDATE ai_styles AS t SET is_default = false, updated_at = NOW()
WHERE t.category = (SELECT s.category FROM ai_styles AS s WHERE s.id = sqlc.arg(id))
  AND t.is_default = true;

-- name: MarkAiStyleDefault :execrows
-- 只對單一 id 生效，不含任何 category 範圍——只有緊接在同一個交易內的
-- ClearCategoryDefaultStyle 之後呼叫才是安全的（此時該分類已無其他 true 列）。
-- 單獨呼叫這句可能撞 idx_ai_styles_default，不要在這個配對之外使用它。
UPDATE ai_styles SET is_default = true, updated_at = NOW() WHERE id = sqlc.arg(id);

-- name: UpdateAiStyle :execrows
UPDATE ai_styles SET
  label = COALESCE(sqlc.narg(label), label),
  prompt = COALESCE(sqlc.narg(prompt), prompt),
  thumbnail_key = COALESCE(sqlc.narg(thumbnail_key), thumbnail_key),
  sort_order = COALESCE(sqlc.narg(sort_order), sort_order),
  enabled = COALESCE(sqlc.narg(enabled), enabled),
  updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: DeleteAiStyle :execrows
DELETE FROM ai_styles WHERE id = $1;
