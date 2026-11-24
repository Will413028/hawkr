//go:build integration

package notification

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// aliveAccounts 讓每個帳號都存活（身分模組不在這個測試的 database 裡）。
type aliveAccounts struct{}

func (aliveAccounts) ShareLockAccount(context.Context, int64) (bool, error) { return false, nil }

// fakeStores 是帳號此刻的店：只有 stores 裡的帳號有店，沒有的回 not_found（同 seller.ActingStore）。
type fakeStores struct{ stores map[int64]int64 }

func (f fakeStores) ActingStore(_ context.Context, accountID int64, c seller.Capability) (seller.Store, error) {
	if c != seller.CapNotification {
		return seller.Store{}, errors.New("通知只該要求 CapNotification")
	}
	id, ok := f.stores[accountID]
	if !ok {
		return seller.Store{}, apperr.NotFound("沒有店")
	}
	return seller.Store{StoreID: id}, nil
}

var (
	epoch      = time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	suspendID  = "0190a000-0000-7000-8000-000000000001"
	paramsSusp = `{"account_id":7,"suspension_id":"` + suspendID + `","category":"SPAM","suspended_at":"2026-11-23T04:30:00Z"}`
	paramsLift = `{"account_id":7,"suspension_id":"` + suspendID + `","released_at":"2026-11-23T04:30:00Z"}`
)

type stack struct {
	db     *db.DB
	client notificationv1alpha1connect.NotificationAppServiceClient
	now    time.Time
}

// setup 起真的 notification 模組（只套用本模組的 migration）與 interceptor 鏈；測試以 X-Test-Principal 指定登入者
// （account:<ID> 或 admin:<ID>，沒有就是沒登入）。帳號 9 有店 90，帳號 10 有店 100。
func setup(t *testing.T) *stack {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "notification", Dir: notification.Migrations()}))
	s := &stack{db: d, now: epoch}
	log := slog.New(slog.DiscardHandler)
	m := notification.New(notification.Deps{DB: d, Identity: aliveAccounts{}, Now: func() time.Time { return s.now }, NewID: uuid.NewV7, Logger: log})
	images, err := rpc.NewImages("https://media.example.com")
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(m, fakeStores{stores: map[int64]int64{9: 90, 10: 100}}, images, log)
	if err != nil {
		t.Fatal(err)
	}
	path, h := notificationv1alpha1connect.NewNotificationAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(log)))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch kind, id, _ := cutPrincipal(r.Header.Get("X-Test-Principal")); kind {
		case "account":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: id}))
		case "admin":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: id}))
		}
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s.client = notificationv1alpha1connect.NewNotificationAppServiceClient(srv.Client(), srv.URL)
	return s
}

func cutPrincipal(v string) (kind string, id int64, ok bool) {
	for i := range len(v) {
		if v[i] == ':' {
			id, err := strconv.ParseInt(v[i+1:], 10, 64)
			return v[:i], id, err == nil
		}
	}
	return "", 0, false
}

// as 把請求標成帳號 account 登入；lang 非空時帶 Accept-Language。
func as[T any](req *connect.Request[T], account int64, lang ...string) *connect.Request[T] {
	req.Header().Set("X-Test-Principal", "account:"+strconv.FormatInt(account, 10))
	if len(lang) > 0 {
		req.Header().Set("Accept-Language", lang[0])
	}
	return req
}

func (s *stack) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := s.db.Pool().Exec(t.Context(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (s *stack) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.Pool().QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// item 在收件匣放一列；storeID 為 0 是 BUYER 列。updated 越大越新。
func (s *stack) item(t *testing.T, account int64, audience string, storeID int64, typ, params string, updated int, readAt *time.Time) string {
	t.Helper()
	id := uuid.NewV7()
	var store *int64
	if storeID > 0 {
		store = &storeID
	}
	at := epoch.Add(time.Duration(updated) * time.Minute)
	var announcementID *uuid.UUID
	if audience == "STORE" {
		// 型別註冊表裡只有公告同時屬於 STORE 受眾（停權通知只給 BUYER），STORE 列一律放成公告列：先放它指向的公告。
		aid := uuid.NewV7()
		s.exec(t, `INSERT INTO notification.announcements (id, audiences, title, body, admin_id, created_at)
			VALUES ($1, ARRAY['STORE'], 'Store notice', 'Store body', 1, $2)`, aid, at)
		typ, params, announcementID = "announcement", `{"announcement_id":"`+aid.String()+`"}`, &aid
	}
	s.exec(t, `INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, occurred_at, read_at, created_at, updated_at, announcement_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7, $7, $9)`, id, account, audience, store, typ, params, at, readAt, announcementID)
	return id.String()
}

func errCode(t *testing.T, err error, code connect.Code) *connect.Error {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code {
		t.Fatalf("錯誤 = %v，預期 %v", err, code)
	}
	return ce
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	_ = errCode(t, err, code)
}

// invalidField 要求 err 是 invalid_argument 並回傳它指出的第一個欄位路徑。
func invalidField(t *testing.T, err error) string {
	t.Helper()
	ce := errCode(t, err, connect.CodeInvalidArgument)
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		if br, ok := v.(*errdetails.BadRequest); ok && len(br.GetFieldViolations()) > 0 {
			return br.GetFieldViolations()[0].GetField()
		}
	}
	t.Fatalf("invalid_argument 沒有指出欄位：%v", err)
	return ""
}
