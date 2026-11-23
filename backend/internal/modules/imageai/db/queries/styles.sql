-- name: ListEnabledStyles :many
-- 一個用途的啟用中風格，含預設風格（呼叫端把 is_default 那列分出來）；單一語句，預設風格與清單來自同一個快照。依顯示順序，同值依名稱。
SELECT id, name, label, prompt, thumbnail_key, display_order, is_default
FROM imageai.styles
WHERE category = @category AND enabled
ORDER BY display_order, name;
