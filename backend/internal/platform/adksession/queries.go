package adksession

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

var schemaPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// queries 是四張 ADK 表的 SQL。表放在呼叫的模組自己的 schema（sessions、events、app_states、user_states），
// schema 名稱在建構時驗過格式後才代入 SQL，不接受執行期的任意輸入。
type queries struct {
	insertSession  string
	getSession     string
	listSessions   string
	deleteSession  string
	touchSession   string
	insertEvent    string
	listEvents     string
	mergeAppState  string
	mergeUserState string
	conn           func(context.Context) db.DBTX
}

func newQueries(schema string, conn func(context.Context) db.DBTX) (*queries, error) {
	if !schemaPattern.MatchString(schema) {
		return nil, fmt.Errorf("adksession：schema 名稱 %q 不合法", schema)
	}
	f := func(s string) string { return fmt.Sprintf(s, schema) }
	return &queries{
		conn: conn,
		insertSession: f(`INSERT INTO %[1]s.sessions (app_name, user_id, id, state, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (app_name, user_id, id) DO NOTHING`),
		getSession: f(`SELECT s.app_name, s.user_id, s.id, s.state, s.updated_at,
       COALESCE(a.state, '{}'::jsonb)::jsonb AS app_state,
       COALESCE(u.state, '{}'::jsonb)::jsonb AS user_state
FROM %[1]s.sessions s
LEFT JOIN %[1]s.app_states a ON a.app_name = s.app_name
LEFT JOIN %[1]s.user_states u ON u.app_name = s.app_name AND u.user_id = s.user_id
WHERE s.app_name = $1 AND s.user_id = $2 AND s.id = $3`),
		listSessions: f(`SELECT s.app_name, s.user_id, s.id, s.state, s.updated_at,
       COALESCE(a.state, '{}'::jsonb)::jsonb AS app_state,
       COALESCE(u.state, '{}'::jsonb)::jsonb AS user_state
FROM %[1]s.sessions s
LEFT JOIN %[1]s.app_states a ON a.app_name = s.app_name
LEFT JOIN %[1]s.user_states u ON u.app_name = s.app_name AND u.user_id = s.user_id
WHERE s.app_name = $1 AND ($2::text = '' OR s.user_id = $2::text)
ORDER BY s.created_at, s.user_id, s.id`),
		deleteSession: f(`DELETE FROM %[1]s.sessions WHERE app_name = $1 AND user_id = $2 AND id = $3`),
		touchSession: f(`UPDATE %[1]s.sessions
SET state = state || $4::jsonb, updated_at = $5
WHERE app_name = $1 AND user_id = $2 AND id = $3
RETURNING id`),
		insertEvent: f(`INSERT INTO %[1]s.events (app_name, user_id, session_id, id, timestamp, event)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (app_name, user_id, session_id, id) DO NOTHING`),
		listEvents: f(`SELECT id, timestamp, event
FROM %[1]s.events
WHERE app_name = $1 AND user_id = $2 AND session_id = $3
  AND ($4::timestamptz IS NULL OR timestamp >= $4::timestamptz)
ORDER BY timestamp DESC, seq DESC
LIMIT $5::int`),
		mergeAppState: f(`INSERT INTO %[1]s.app_states (app_name, state, updated_at)
VALUES ($1, $2::jsonb, $3)
ON CONFLICT (app_name) DO UPDATE
SET state = %[1]s.app_states.state || EXCLUDED.state, updated_at = EXCLUDED.updated_at`),
		mergeUserState: f(`INSERT INTO %[1]s.user_states (app_name, user_id, state, updated_at)
VALUES ($1, $2, $3::jsonb, $4)
ON CONFLICT (app_name, user_id) DO UPDATE
SET state = %[1]s.user_states.state || EXCLUDED.state, updated_at = EXCLUDED.updated_at`),
	}, nil
}

type sessionRow struct {
	AppName, UserID, ID string
	State               []byte
	UpdatedAt           time.Time
	AppState, UserState []byte
}

type eventRow struct {
	ID        string
	Timestamp time.Time
	Event     []byte
}

func scanSession(row pgx.Row) (sessionRow, error) {
	var r sessionRow
	err := row.Scan(&r.AppName, &r.UserID, &r.ID, &r.State, &r.UpdatedAt, &r.AppState, &r.UserState)
	return r, err
}

func (q *queries) InsertSession(ctx context.Context, appName, userID, id string, state []byte, now time.Time) (int64, error) {
	tag, err := q.conn(ctx).Exec(ctx, q.insertSession, appName, userID, id, state, now)
	return tag.RowsAffected(), err
}

func (q *queries) GetSession(ctx context.Context, appName, userID, id string) (sessionRow, error) {
	return scanSession(q.conn(ctx).QueryRow(ctx, q.getSession, appName, userID, id))
}

func (q *queries) ListSessions(ctx context.Context, appName, userID string) ([]sessionRow, error) {
	rows, err := q.conn(ctx).Query(ctx, q.listSessions, appName, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		r, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *queries) DeleteSession(ctx context.Context, appName, userID, id string) error {
	_, err := q.conn(ctx).Exec(ctx, q.deleteSession, appName, userID, id)
	return err
}

func (q *queries) TouchSessionForAppend(ctx context.Context, appName, userID, id string, stateDelta []byte, now time.Time) (string, error) {
	var got string
	err := q.conn(ctx).QueryRow(ctx, q.touchSession, appName, userID, id, stateDelta, now).Scan(&got)
	return got, err
}

func (q *queries) InsertEvent(ctx context.Context, appName, userID, sessionID, id string, ts time.Time, event []byte) (int64, error) {
	tag, err := q.conn(ctx).Exec(ctx, q.insertEvent, appName, userID, sessionID, id, ts, event)
	return tag.RowsAffected(), err
}

func (q *queries) ListEvents(ctx context.Context, appName, userID, sessionID string, after *time.Time, maxEvents *int32) ([]eventRow, error) {
	rows, err := q.conn(ctx).Query(ctx, q.listEvents, appName, userID, sessionID, after, maxEvents)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var r eventRow
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Event); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *queries) MergeAppState(ctx context.Context, appName string, delta []byte, now time.Time) error {
	_, err := q.conn(ctx).Exec(ctx, q.mergeAppState, appName, delta, now)
	return err
}

func (q *queries) MergeUserState(ctx context.Context, appName, userID string, delta []byte, now time.Time) error {
	_, err := q.conn(ctx).Exec(ctx, q.mergeUserState, appName, userID, delta, now)
	return err
}
