-- name: InsertSession :execrows
-- 已存在回 0 列（呼叫端轉成「已存在」錯誤）；不用撞唯一鍵再處理，因為撞鍵會讓外層 transaction 進入 aborted。
INSERT INTO assistant.sessions (app_name, user_id, id, state, created_at, updated_at)
VALUES (@app_name, @user_id, @id, @state, @now, @now)
ON CONFLICT (app_name, user_id, id) DO NOTHING;

-- name: GetSession :one
-- 一個語句取 session 與它的 app、user 層 state，兩層 state 與 session 讀到同一份快照。
SELECT s.app_name, s.user_id, s.id, s.state, s.updated_at,
       COALESCE(a.state, '{}'::jsonb)::jsonb AS app_state,
       COALESCE(u.state, '{}'::jsonb)::jsonb AS user_state
FROM assistant.sessions s
LEFT JOIN assistant.app_states a ON a.app_name = s.app_name
LEFT JOIN assistant.user_states u ON u.app_name = s.app_name AND u.user_id = s.user_id
WHERE s.app_name = @app_name AND s.user_id = @user_id AND s.id = @id;

-- name: ListSessions :many
-- user_id 為空字串時列出該 app 的全部使用者（ADK session.Service 的語意）。
SELECT s.app_name, s.user_id, s.id, s.state, s.updated_at,
       COALESCE(a.state, '{}'::jsonb)::jsonb AS app_state,
       COALESCE(u.state, '{}'::jsonb)::jsonb AS user_state
FROM assistant.sessions s
LEFT JOIN assistant.app_states a ON a.app_name = s.app_name
LEFT JOIN assistant.user_states u ON u.app_name = s.app_name AND u.user_id = s.user_id
WHERE s.app_name = @app_name AND (@user_id::text = '' OR s.user_id = @user_id::text)
ORDER BY s.created_at, s.user_id, s.id;

-- name: DeleteSession :exec
-- events 由 FK 的 ON DELETE CASCADE 一併刪除。
DELETE FROM assistant.sessions
WHERE app_name = @app_name AND user_id = @user_id AND id = @id;

-- name: TouchSessionForAppend :one
-- AppendEvent 的第一步：鎖 session 列（同一個對話的並行 append 依序進行）、合併 session 層的 state delta、
-- 更新時間。0 列表示 session 不存在（或屬於別的使用者）。
UPDATE assistant.sessions
SET state = state || @state_delta::jsonb, updated_at = @now
WHERE app_name = @app_name AND user_id = @user_id AND id = @id
RETURNING id;

-- name: InsertEvent :execrows
-- 重複的 event id 回 0 列（呼叫端轉成錯誤）；不用撞主鍵再處理，因為撞鍵會讓外層 transaction 進入 aborted。
INSERT INTO assistant.events (app_name, user_id, session_id, id, timestamp, event)
VALUES (@app_name, @user_id, @session_id, @id, @timestamp, @event)
ON CONFLICT (app_name, user_id, session_id, id) DO NOTHING;

-- name: ListEvents :many
-- 先套 after 篩選再取最近 limit 筆（gorm 版相同），以 timestamp、seq（寫入順序）為全序；回傳由新到舊，呼叫端反轉成時間順序。
SELECT id, timestamp, event
FROM assistant.events
WHERE app_name = @app_name AND user_id = @user_id AND session_id = @session_id
  AND (sqlc.narg('after')::timestamptz IS NULL OR timestamp >= sqlc.narg('after')::timestamptz)
ORDER BY timestamp DESC, seq DESC
LIMIT sqlc.narg('max_events')::int;

-- name: MergeAppState :exec
INSERT INTO assistant.app_states (app_name, state, updated_at)
VALUES (@app_name, @state_delta::jsonb, @now)
ON CONFLICT (app_name) DO UPDATE
SET state = assistant.app_states.state || EXCLUDED.state, updated_at = EXCLUDED.updated_at;

-- name: MergeUserState :exec
INSERT INTO assistant.user_states (app_name, user_id, state, updated_at)
VALUES (@app_name, @user_id, @state_delta::jsonb, @now)
ON CONFLICT (app_name, user_id) DO UPDATE
SET state = assistant.user_states.state || EXCLUDED.state, updated_at = EXCLUDED.updated_at;
