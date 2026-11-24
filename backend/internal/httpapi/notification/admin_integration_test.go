//go:build integration

package notification

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1/notificationv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// steppingClock 是注入的時鐘：從固定時刻起，每讀一次前進一毫秒，讓先建立的公告的建立時間一定早於後建立的，排序不靠真實時間或 sleep。
type steppingClock struct {
	mu  sync.Mutex
	now time.Time
}

func newSteppingClock() *steppingClock {
	return &steppingClock{now: time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)}
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(time.Millisecond)
	return c.now
}

type noVouchers struct{}

func (noVouchers) VoucherAvailable(context.Context, int64, uuid.UUID) (bool, error) { return true, nil }

type nopJobs struct{}

func (nopJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	var fields []string
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			t.Fatal(e)
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				fields = append(fields, fv.GetField())
			}
		}
	}
	return fields
}

// 後台公告的兩支 RPC 經真的 Connect handler 與資料庫：建立回公告（受眾排序、尚未派送）、同一個 request_id 重送回同一則、內容不同與欄位錯誤是 invalid_argument 指向請求欄位，
// 建立寫一筆後台紀錄（動作是 procedure、管理員是呼叫者），列表新到舊並可分頁。
func TestAnnouncementAdminRPCs(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
		migrate.Module{Name: "notification", Dir: notification.Migrations()},
	))
	clock := newSteppingClock()
	m := notification.New(notification.Deps{DB: d, Validity: noVouchers{}, Jobs: nopJobs{}, Now: clock.Now, NewID: uuid.NewV7, Logger: slog.New(slog.DiscardHandler)})
	path, handler := notificationv1alpha1connect.NewNotificationAdminServiceHandler(NewAdmin(m), connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 77})))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := notificationv1alpha1connect.NewNotificationAdminServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()
	both := []pb.NotificationAudience{pb.NotificationAudience_NOTIFICATION_AUDIENCE_STORE, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER}
	rid := uuid.NewV4().String()

	first, err := client.CreateAnnouncement(ctx, connect.NewRequest(&adminpb.CreateAnnouncementRequest{Audiences: both, Title: " Maintenance ", Body: "Tonight.", RequestId: rid}))
	if err != nil {
		t.Fatal(err)
	}
	a := first.Msg.GetAnnouncement()
	if a.GetTitle() != "Maintenance" || a.GetBody() != "Tonight." || a.GetDispatched() || a.GetInboxCount() != 0 || a.GetCreateTime() == nil || a.GetId() == "" {
		t.Errorf("announcement = %v", a)
	}
	if !slices.Equal(a.GetAudiences(), []pb.NotificationAudience{pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER, pb.NotificationAudience_NOTIFICATION_AUDIENCE_STORE}) {
		t.Errorf("audiences = %v，預期 BUYER、STORE", a.GetAudiences())
	}
	again, err := client.CreateAnnouncement(ctx, connect.NewRequest(&adminpb.CreateAnnouncementRequest{Audiences: both, Title: "Maintenance", Body: "Tonight.", RequestId: rid}))
	if err != nil || again.Msg.GetAnnouncement().GetId() != a.GetId() {
		t.Fatalf("同一個 request_id 重送：%v，%v", again, err)
	}
	_, err = client.CreateAnnouncement(ctx, connect.NewRequest(&adminpb.CreateAnnouncementRequest{Audiences: both, Title: "Other", Body: "Tonight.", RequestId: rid}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldsOf(t, err), []string{"request_id"}) {
		t.Errorf("內容不同的重送：%v（欄位 %v）", err, fieldsOf(t, err))
	}
	for name, c := range map[string]struct {
		req   *adminpb.CreateAnnouncementRequest
		field string
	}{
		"沒有受眾":                 {&adminpb.CreateAnnouncementRequest{Title: "t", Body: "b"}, "audiences"},
		"UNSPECIFIED":          {&adminpb.CreateAnnouncementRequest{Audiences: []pb.NotificationAudience{pb.NotificationAudience_NOTIFICATION_AUDIENCE_UNSPECIFIED}, Title: "t", Body: "b"}, "audiences"},
		"空標題":                  {&adminpb.CreateAnnouncementRequest{Audiences: both[:1], Title: "  ", Body: "b"}, "title"},
		"空內文":                  {&adminpb.CreateAnnouncementRequest{Audiences: both[:1], Title: "t"}, "body"},
		"request_id 不是 UUIDv4": {&adminpb.CreateAnnouncementRequest{Audiences: both[:1], Title: "t", Body: "b", RequestId: uuid.NewV7().String()}, "request_id"},
	} {
		_, err := client.CreateAnnouncement(ctx, connect.NewRequest(c.req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldsOf(t, err), []string{c.field}) {
			t.Errorf("%s：%v（欄位 %v），預期 invalid_argument（%s）", name, err, fieldsOf(t, err), c.field)
		}
	}
	second, err := client.CreateAnnouncement(ctx, connect.NewRequest(&adminpb.CreateAnnouncementRequest{Audiences: both[:1], Title: "Second", Body: "b"}))
	if err != nil {
		t.Fatal(err)
	}

	audit := adminaudit.New(d, clock.Now)
	recs, err := audit.List(ctx, adminaudit.Filter{Action: notificationv1alpha1connect.NotificationAdminServiceCreateAnnouncementProcedure})
	if err != nil || len(recs) != 2 {
		t.Fatalf("稽核紀錄 %d 筆（%v），預期 2（重送與失敗的不記）", len(recs), err)
	}
	for _, r := range recs {
		if r.AdminID != 77 || r.TargetType != "announcement" {
			t.Errorf("稽核紀錄 = %+v", r)
		}
	}

	p1, err := client.ListAnnouncements(ctx, connect.NewRequest(&adminpb.ListAnnouncementsRequest{PageSize: 1}))
	if err != nil || len(p1.Msg.GetAnnouncements()) != 1 || p1.Msg.GetAnnouncements()[0].GetId() != second.Msg.GetAnnouncement().GetId() || p1.Msg.GetNextPageToken() == "" {
		t.Fatalf("第一頁 = %v, %v", p1, err)
	}
	p2, err := client.ListAnnouncements(ctx, connect.NewRequest(&adminpb.ListAnnouncementsRequest{PageSize: 1, PageToken: p1.Msg.GetNextPageToken()}))
	if err != nil || len(p2.Msg.GetAnnouncements()) != 1 || p2.Msg.GetAnnouncements()[0].GetId() != a.GetId() || p2.Msg.GetNextPageToken() != "" {
		t.Fatalf("第二頁 = %v, %v", p2, err)
	}
	_, err = client.ListAnnouncements(ctx, connect.NewRequest(&adminpb.ListAnnouncementsRequest{PageToken: "garbage"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("壞的 page_token：%v", err)
	}
	_, err = client.ListAnnouncements(ctx, connect.NewRequest(&adminpb.ListAnnouncementsRequest{PageSize: -1}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldsOf(t, err), []string{"page_size"}) {
		t.Errorf("負的 page_size：%v", err)
	}
}
