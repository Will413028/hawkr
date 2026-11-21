// Package adksession 以 pgx 實作 ADK v2 的 session.Service（A9-D5、A9-D19 S7）。
//
// 四張表（sessions、events、app_states、user_states）由呼叫的模組在自己的 schema 建（形狀見 assistant 的 db/schema.sql），
// New 收 schema 名稱；本套件不擁有任何表。
//
// 驗收是 ADK 公開的 session/sessiontestsuite.RunServiceTests（各模組的 session_integration_test.go，在 CI 的 Postgres 上跑）。
// 寫入都在 db.InTx 內，transaction 內沒有任何外部 provider 呼叫（backend/AGENTS.md 規則 5）。
package adksession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/adk/v2/platform"
	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Service 實作 session.Service。
type Service struct {
	db *db.DB
	q  *queries
}

var _ session.Service = (*Service)(nil)

// New 建立 session service；schema 是呼叫的模組放四張 ADK 表的 schema（小寫字母、數字、底線）。
// schema 不合法是程式錯誤（寫死在模組的組裝處），所以直接 panic。
func New(d *db.DB, schema string) *Service {
	q, err := newQueries(schema, d.Conn)
	if err != nil {
		panic(err)
	}
	return &Service{db: d, q: q}
}

// ErrInOuterTx 是在已有 transaction 的 ctx 上呼叫 Create 或 AppendEvent 時回傳的錯誤：這兩個方法在提交後才更新
// 傳入或回傳的 live session，外層 transaction 之後 rollback 會讓記憶體與資料庫分岔，所以必須是最上層的呼叫。
var ErrInOuterTx = errors.New("adksession：寫入不能開在外層 transaction 內")

// ErrEventExists 是 event id 已存在於該 session 時回傳（以 errors.Is 判斷）。InsertEvent 用 ON CONFLICT DO NOTHING
// 看影響列數，不依賴解析驅動回傳的 unique violation 錯誤。
var ErrEventExists = errors.New("adksession：event 已存在")

func requireNoOuterTx(ctx context.Context) error {
	if _, ok := db.TxFrom(ctx); ok {
		return ErrInOuterTx
	}
	return nil
}

func clampEvents(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n) //nolint:gosec // 上面已夾到 MaxInt32；n 是 event 數量，不會是負數
}

func marshalState(m map[string]any) ([]byte, error) {
	if m == nil {
		m = map[string]any{}
	}
	return json.Marshal(m)
}

func unmarshalState(b []byte) (map[string]any, error) {
	m := map[string]any{}
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("解 state JSON：%w", err)
	}
	return m, nil
}

// Create 建立 session。同一個 (AppName, UserID, SessionID) 已存在時回錯，不覆蓋。
// 初始 state 依前綴分流：app:、user: 寫進共用的 app、user 層，temp: 丟棄，其餘是 session 層。
func (s *Service) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if req.AppName == "" || req.UserID == "" {
		return nil, fmt.Errorf("app_name and user_id are required, got app_name: %q, user_id: %q", req.AppName, req.UserID)
	}
	if err := requireNoOuterTx(ctx); err != nil {
		return nil, err
	}
	id := req.SessionID
	if id == "" {
		id = platform.NewUUID(ctx)
	}
	now := platform.Now(ctx).Truncate(time.Microsecond)
	appDelta, userDelta, sessDelta := splitDelta(req.State)
	sessJSON, err := marshalState(sessDelta)
	if err != nil {
		return nil, fmt.Errorf("編碼 session state：%w", err)
	}
	appJSON, err := marshalState(appDelta)
	if err != nil {
		return nil, fmt.Errorf("編碼 app state：%w", err)
	}
	userJSON, err := marshalState(userDelta)
	if err != nil {
		return nil, fmt.Errorf("編碼 user state：%w", err)
	}

	var out *liveSession
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		q := s.q
		n, err := q.InsertSession(ctx, req.AppName, req.UserID, id, sessJSON, now)
		if err != nil {
			return fmt.Errorf("寫入 session：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("session %q already exists", id)
		}
		// 鎖順序：session 列、app 列、user 列；AppendEvent 同序。
		if len(appDelta) > 0 {
			if err := q.MergeAppState(ctx, req.AppName, appJSON, now); err != nil {
				return fmt.Errorf("寫入 app state：%w", err)
			}
		}
		if len(userDelta) > 0 {
			if err := q.MergeUserState(ctx, req.AppName, req.UserID, userJSON, now); err != nil {
				return fmt.Errorf("寫入 user state：%w", err)
			}
		}
		row, err := q.GetSession(ctx, req.AppName, req.UserID, id)
		if err != nil {
			return fmt.Errorf("讀回 session：%w", err)
		}
		out, err = toLive(row.AppName, row.UserID, row.ID, row.State, row.AppState, row.UserState, row.UpdatedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &session.CreateResponse{Session: out}, nil
}

func toLive(appName, userID, id string, sess, app, user []byte, updatedAt time.Time) (*liveSession, error) {
	sm, err := unmarshalState(sess)
	if err != nil {
		return nil, err
	}
	am, err := unmarshalState(app)
	if err != nil {
		return nil, err
	}
	um, err := unmarshalState(user)
	if err != nil {
		return nil, err
	}
	return &liveSession{appName: appName, userID: userID, sessionID: id, state: mergeStates(am, um, sm), updatedAt: updatedAt}, nil
}

// Get 取 session 與它的 events（時間由舊到新）。NumRecentEvents 與 After 同時給時，先篩 After 再取最近 N 筆。
// 不存在或屬於別的使用者時回傳包裝 session.ErrNotFound 的錯誤。
func (s *Service) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return nil, fmt.Errorf("app_name, user_id, session_id are required, got app_name: %q, user_id: %q, session_id: %q", req.AppName, req.UserID, req.SessionID)
	}
	var live *liveSession
	err := s.db.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		live, err = s.getInSnapshot(ctx, req)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &session.GetResponse{Session: live}, nil
}

// getInSnapshot 在同一份快照內讀 session 列與 events（backend/AGENTS.md 規則 3）。
func (s *Service) getInSnapshot(ctx context.Context, req *session.GetRequest) (*liveSession, error) {
	q := s.q
	row, err := q.GetSession(ctx, req.AppName, req.UserID, req.SessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %q", session.ErrNotFound, req.SessionID)
		}
		return nil, fmt.Errorf("讀 session：%w", err)
	}
	live, err := toLive(row.AppName, row.UserID, row.ID, row.State, row.AppState, row.UserState, row.UpdatedAt)
	if err != nil {
		return nil, err
	}
	var after *time.Time
	if !req.After.IsZero() {
		a := req.After
		after = &a
	}
	var maxEvents *int32
	if req.NumRecentEvents > 0 {
		n := clampEvents(req.NumRecentEvents)
		maxEvents = &n
	}
	rows, err := q.ListEvents(ctx, req.AppName, req.UserID, req.SessionID, after, maxEvents)
	if err != nil {
		return nil, fmt.Errorf("讀 events：%w", err)
	}
	live.events = make([]*session.Event, 0, len(rows))
	for _, r := range slices.Backward(rows) {
		var ev session.Event
		if err := json.Unmarshal(r.Event, &ev); err != nil {
			return nil, fmt.Errorf("解 event %q：%w", r.ID, err)
		}
		live.events = append(live.events, &ev)
	}
	return live, nil
}

// List 列出 app 的 sessions，不含 events。UserID 為空時列出該 app 全部使用者的 sessions。
func (s *Service) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	if req.AppName == "" {
		return nil, fmt.Errorf("app_name is required, got app_name: %q", req.AppName)
	}
	rows, err := s.q.ListSessions(ctx, req.AppName, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("列 sessions：%w", err)
	}
	out := make([]session.Session, 0, len(rows))
	for _, r := range rows {
		live, err := toLive(r.AppName, r.UserID, r.ID, r.State, r.AppState, r.UserState, r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, live)
	}
	return &session.ListResponse{Sessions: out}, nil
}

// Delete 刪除 session（events 由 FK 連帶刪除）。不存在或屬於別的使用者時不是錯誤。
func (s *Service) Delete(ctx context.Context, req *session.DeleteRequest) error {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return fmt.Errorf("app_name, user_id, session_id are required, got app_name: %q, user_id: %q, session_id: %q", req.AppName, req.UserID, req.SessionID)
	}
	if err := s.q.DeleteSession(ctx, req.AppName, req.UserID, req.SessionID); err != nil {
		return fmt.Errorf("刪 session：%w", err)
	}
	return nil
}

// AppendEvent 把 event 寫進 session：partial event 不寫；沒有 ID 的補 ID（就地）；時間截到微秒（就地，
// 與 timestamptz 精度一致）；StateDelta 的 temp: key 在寫入前剝除。state delta 依前綴合併進 session、app、user 層，
// 合併在 SQL 內完成（jsonb 的 ||），不是讀出再寫回，所以並行的 append 不會互相蓋掉 key。
// session 不存在時回傳包裝 session.ErrNotFound 的錯誤。
func (s *Service) AppendEvent(ctx context.Context, cur session.Session, ev *session.Event) error {
	if cur == nil {
		return fmt.Errorf("session is nil")
	}
	if ev == nil {
		return fmt.Errorf("event is nil")
	}
	if ev.Partial {
		return nil
	}
	if err := requireNoOuterTx(ctx); err != nil {
		return err
	}
	if ev.ID == "" {
		ev.ID = platform.NewUUID(ctx)
	}
	ev.Timestamp = ev.Timestamp.Truncate(time.Microsecond)

	enc, err := encodeAppend(ev)
	if err != nil {
		return err
	}

	appName, userID, id := cur.AppName(), cur.UserID(), cur.ID()
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		q := s.q
		if _, err := q.TouchSessionForAppend(ctx, appName, userID, id, enc.sessJSON, ev.Timestamp); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: %q, cannot apply event", session.ErrNotFound, id)
			}
			return fmt.Errorf("更新 session：%w", err)
		}
		n, err := q.InsertEvent(ctx, appName, userID, id, ev.ID, ev.Timestamp, enc.eventJSON)
		if err != nil {
			return fmt.Errorf("寫入 event：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("%w: event %q in session %q", ErrEventExists, ev.ID, id)
		}
		if enc.hasApp {
			if err := q.MergeAppState(ctx, appName, enc.appJSON, ev.Timestamp); err != nil {
				return fmt.Errorf("寫入 app state：%w", err)
			}
		}
		if enc.hasUser {
			if err := q.MergeUserState(ctx, appName, userID, enc.userJSON, ev.Timestamp); err != nil {
				return fmt.Errorf("寫入 user state：%w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if live, ok := cur.(*liveSession); ok {
		live.applyAppended(ev.Actions.StateDelta, enc.stored, ev.Timestamp)
	}
	return nil
}

// appendEncoding 是 AppendEvent 要寫進資料庫的內容，在開 transaction 之前算好（純函式，不碰資料庫）。
type appendEncoding struct {
	stored    *session.Event // 已剝除 temp: key 的 event，也是放進 live session 的版本
	eventJSON []byte
	sessJSON  []byte
	appJSON   []byte
	userJSON  []byte
	hasApp    bool
	hasUser   bool
}

func encodeAppend(ev *session.Event) (*appendEncoding, error) {
	stored := withoutTempKeys(ev)
	appDelta, userDelta, sessDelta := splitDelta(stored.Actions.StateDelta)
	enc := &appendEncoding{stored: stored, hasApp: len(appDelta) > 0, hasUser: len(userDelta) > 0}
	var err error
	if enc.eventJSON, err = json.Marshal(stored); err != nil {
		return nil, fmt.Errorf("編碼 event：%w", err)
	}
	if enc.sessJSON, err = marshalState(sessDelta); err != nil {
		return nil, fmt.Errorf("編碼 session state：%w", err)
	}
	if enc.appJSON, err = marshalState(appDelta); err != nil {
		return nil, fmt.Errorf("編碼 app state：%w", err)
	}
	if enc.userJSON, err = marshalState(userDelta); err != nil {
		return nil, fmt.Errorf("編碼 user state：%w", err)
	}
	return enc, nil
}
