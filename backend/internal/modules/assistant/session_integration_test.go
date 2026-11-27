//go:build integration

package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/session/sessiontestsuite"

	"github.com/Will413028/hawkr/backend/internal/modules/assistant/internal/adksession"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func newDB(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "assistant", Dir: Migrations()}))
}

// ADK 公開的 session.Service 合格測試；每個子測試用全新的資料庫。
func TestADKServiceConformance(t *testing.T) {
	sessiontestsuite.RunServiceTests(t, sessiontestsuite.SuiteOptions{
		SupportsUserProvidedSessionID: true,
		ProvidesServerAssignedEventID: false,
	}, func(t *testing.T) session.Service { return adksession.New(newDB(t)) })
}

const app = "hawkr_ai"

func mustCreate(t *testing.T, s *adksession.Service, user string, state map[string]any) session.Session {
	t.Helper()
	r, err := s.Create(context.Background(), &session.CreateRequest{AppName: app, UserID: user, State: state})
	if err != nil {
		t.Fatal(err)
	}
	return r.Session
}

func get(t *testing.T, s *adksession.Service, user, id string) session.Session {
	t.Helper()
	r, err := s.Get(context.Background(), &session.GetRequest{AppName: app, UserID: user, SessionID: id})
	if err != nil {
		t.Fatal(err)
	}
	return r.Session
}

func stateOf(sess session.Session) map[string]any {
	m := map[string]any{}
	for k, v := range sess.State().All() {
		m[k] = v
	}
	return m
}

func TestLiveSessionReflectsAppendedEventAndTempStaysInMemoryOnly(t *testing.T) {
	s := adksession.New(newDB(t))
	ctx := context.Background()
	sess := mustCreate(t, s, "u1", nil)
	ev := &session.Event{Author: "user", InvocationID: "inv", Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 6789, time.UTC),
		Actions: session.EventActions{StateDelta: map[string]any{"temp:t": "x", "k": "v"}}}
	if err := s.AppendEvent(ctx, sess, ev); err != nil {
		t.Fatal(err)
	}
	if ev.ID == "" || ev.Timestamp.Nanosecond()%1000 != 0 {
		t.Fatalf("event not normalized in place: id=%q ts=%v", ev.ID, ev.Timestamp)
	}
	if _, ok := ev.Actions.StateDelta["temp:t"]; !ok {
		t.Fatalf("caller's event lost its temp: key")
	}
	if got := stateOf(sess); got["temp:t"] != "x" || got["k"] != "v" {
		t.Fatalf("live state = %v", got)
	}
	if sess.Events().Len() != 1 {
		t.Fatalf("live events = %d", sess.Events().Len())
	}
	for k := range sess.Events().At(0).Actions.StateDelta {
		if strings.HasPrefix(k, "temp:") {
			t.Fatalf("live event kept temp key %q", k)
		}
	}
	back := get(t, s, "u1", sess.ID())
	if _, ok := stateOf(back)["temp:t"]; ok {
		t.Fatalf("temp key persisted: %v", stateOf(back))
	}
}

func TestDuplicateCreateKeepsOriginalAndDoesNotWriteSharedState(t *testing.T) {
	s := adksession.New(newDB(t))
	ctx := context.Background()
	first, err := s.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1", SessionID: "s1", State: map[string]any{"k": "orig", "app:a": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1", SessionID: "s1", State: map[string]any{"k": "dup", "app:a": "2", "user:u": "x"}})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already exists", err)
	}
	got := stateOf(get(t, s, "u1", first.Session.ID()))
	if got["k"] != "orig" || got["app:a"] != "1" {
		t.Fatalf("state = %v", got)
	}
	if _, ok := got["user:u"]; ok {
		t.Fatalf("failed create wrote user state: %v", got)
	}
}

// Create、AppendEvent 在提交後才更新 live session，外層 transaction 之後 rollback 會讓記憶體分岔，所以在外層 transaction 內一律拒絕。
func TestCreateAndAppendRefuseToRunInsideOuterTx(t *testing.T) {
	d := newDB(t)
	s := adksession.New(d)
	ctx := context.Background()
	sess := mustCreate(t, s, "u1", nil)
	err := d.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1", SessionID: "x"}); !errors.Is(err, adksession.ErrInOuterTx) {
			return fmt.Errorf("Create err = %v", err) //nolint:errorlint // err may be nil here; %w would print %!w(<nil>)
		}
		if err := s.AppendEvent(ctx, sess, &session.Event{ID: "e", Author: "user", InvocationID: "i"}); !errors.Is(err, adksession.ErrInOuterTx) {
			return fmt.Errorf("AppendEvent err = %v", err) //nolint:errorlint // err may be nil here; %w would print %!w(<nil>)
		}
		if err := s.AppendEvent(ctx, sess, &session.Event{ID: "p", Partial: true}); err != nil {
			return fmt.Errorf("partial AppendEvent err = %v, want nil", err) //nolint:errorlint // err may be nil here; %w would print %!w(<nil>)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 規則 3：Get 的 session 列與 events 在同一份唯讀快照；快照不能開在 transaction 內，所以在外層 transaction 內呼叫會被 db 拒絕。
// 若 Get 拆成兩個沒有快照的語句，這裡會成功而不是回 ErrSnapshotInTx。
func TestGetReadsInOneSnapshot(t *testing.T) {
	d := newDB(t)
	s := adksession.New(d)
	ctx := context.Background()
	sess := mustCreate(t, s, "u1", nil)
	err := d.InTx(ctx, func(ctx context.Context) error {
		_, err := s.Get(ctx, &session.GetRequest{AppName: app, UserID: "u1", SessionID: sess.ID()})
		if !errors.Is(err, db.ErrSnapshotInTx) {
			return fmt.Errorf("Get err = %v, want ErrSnapshotInTx", err) //nolint:errorlint // err may be nil here; %w would print %!w(<nil>)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEventsWithEqualTimestampKeepInsertionOrder(t *testing.T) {
	s := adksession.New(newDB(t))
	ctx := context.Background()
	sess := mustCreate(t, s, "u1", nil)
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ids := []string{"z", "m", "a", "q", "b"}
	for _, id := range ids {
		if err := s.AppendEvent(ctx, sess, &session.Event{ID: id, Author: "user", InvocationID: "i", Timestamp: ts}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for ev := range get(t, s, "u1", sess.ID()).Events().All() {
		got = append(got, ev.ID)
	}
	if strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Fatalf("order = %v, want %v", got, ids)
	}
}

func TestFailedAppendRollsBackStateDelta(t *testing.T) {
	s := adksession.New(newDB(t))
	ctx := context.Background()
	sess := mustCreate(t, s, "u1", nil)
	ok := &session.Event{ID: "e1", Author: "user", InvocationID: "i"}
	if err := s.AppendEvent(ctx, sess, ok); err != nil {
		t.Fatal(err)
	}
	dup := &session.Event{ID: "e1", Author: "user", InvocationID: "i", Actions: session.EventActions{StateDelta: map[string]any{"k": "v", "app:a": "1", "user:b": "2"}}}
	if err := s.AppendEvent(ctx, sess, dup); !errors.Is(err, adksession.ErrEventExists) {
		t.Fatalf("duplicate event id err = %v, want ErrEventExists", err)
	}
	got := stateOf(get(t, s, "u1", sess.ID()))
	for _, k := range []string{"k", "app:a", "user:b"} {
		if _, found := got[k]; found {
			t.Fatalf("rolled-back delta key %q visible: %v", k, got)
		}
	}
	if _, found := stateOf(sess)["k"]; found {
		t.Fatal("live session applied a delta whose write failed")
	}
}

func TestAppendToAnotherUsersSessionIsNotFound(t *testing.T) {
	s := adksession.New(newDB(t))
	sess := mustCreate(t, s, "u1", nil)
	other := foreignSession{appName: app, userID: "u2", id: sess.ID()}
	err := s.AppendEvent(context.Background(), other, &session.Event{ID: "e", Author: "user", InvocationID: "i"})
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if get(t, s, "u1", sess.ID()).Events().Len() != 0 {
		t.Fatal("event written into another user's session")
	}
}

func TestDeleteCascadesEventsAndOnlyOwnSession(t *testing.T) {
	d := newDB(t)
	s := adksession.New(d)
	ctx := context.Background()
	a := mustCreate(t, s, "u1", nil)
	b := mustCreate(t, s, "u1", nil)
	for _, sess := range []session.Session{a, b} {
		if err := s.AppendEvent(ctx, sess, &session.Event{Author: "user", InvocationID: "i"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Delete(ctx, &session.DeleteRequest{AppName: app, UserID: "u1", SessionID: a.ID()}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM assistant.events").Scan(&n); err != nil || n != 1 {
		t.Fatalf("events left = %d err=%v, want 1", n, err)
	}
	if _, err := s.Get(ctx, &session.GetRequest{AppName: app, UserID: "u1", SessionID: a.ID()}); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("get deleted = %v", err)
	}
}

func TestConcurrentAppendsKeepEveryStateKey(t *testing.T) {
	s := adksession.New(newDB(t))
	ctx := context.Background()
	sess := mustCreate(t, s, "u1", nil)
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.AppendEvent(ctx, sess, &session.Event{Author: "user", InvocationID: "i", Actions: session.EventActions{StateDelta: map[string]any{
				fmt.Sprintf("k%d", i): i, fmt.Sprintf("app:a%d", i): i, fmt.Sprintf("user:u%d", i): i,
			}}})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := stateOf(get(t, s, "u1", sess.ID()))
	for i := range n {
		for _, k := range []string{fmt.Sprintf("k%d", i), fmt.Sprintf("app:a%d", i), fmt.Sprintf("user:u%d", i)} {
			if _, ok := got[k]; !ok {
				t.Fatalf("lost key %q: %v", k, got)
			}
		}
	}
	if got := get(t, s, "u1", sess.ID()).Events().Len(); got != n {
		t.Fatalf("events = %d, want %d", got, n)
	}
}

func TestListEmptyUserIDListsAllUsersWithTheirOwnUserState(t *testing.T) {
	s := adksession.New(newDB(t))
	mustCreate(t, s, "u1", map[string]any{"user:name": "a"})
	mustCreate(t, s, "u2", map[string]any{"user:name": "b"})
	r, err := s.List(context.Background(), &session.ListRequest{AppName: app})
	if err != nil || len(r.Sessions) != 2 {
		t.Fatalf("list = %v err=%v", r, err)
	}
	for _, sess := range r.Sessions {
		want := map[string]string{"u1": "a", "u2": "b"}[sess.UserID()]
		if got := stateOf(sess)["user:name"]; got != want {
			t.Fatalf("user %s user:name = %v, want %s", sess.UserID(), got, want)
		}
	}
}

type foreignSession struct{ appName, userID, id string }

func (f foreignSession) ID() string                { return f.id }
func (f foreignSession) AppName() string           { return f.appName }
func (f foreignSession) UserID() string            { return f.userID }
func (f foreignSession) State() session.State      { return nil }
func (f foreignSession) Events() session.Events    { return nil }
func (f foreignSession) LastUpdateTime() time.Time { return time.Time{} }
