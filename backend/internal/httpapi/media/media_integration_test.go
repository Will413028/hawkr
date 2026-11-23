//go:build integration

package media_test

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	httpmedia "github.com/Will413028/hawkr/backend/internal/httpapi/media"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type stack struct {
	d      *db.DB
	clock  *clock
	client mediav1alpha1connect.MediaAppServiceClient
}

// start 以真的 media 模組、真的資料庫與記憶體的物件儲存掛上 Connect handler；header X-Test-Account 是帳號 ID，X-Test-Admin 非空時是管理員。
func start(t *testing.T) *stack {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "media", Dir: media.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	clk := &clock{t: time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := media.New(media.Deps{DB: d, Jobs: jc, Store: media.NewMemoryStore(), NewID: uuid.NewV7, Now: clk.Now, Logger: log})
	path, handler := mediav1alpha1connect.NewMediaAppServiceHandler(httpmedia.NewApp(m, log), connect.WithInterceptors(rpc.NewInterceptor(log)))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch id := req.Header.Get("X-Test-Account"); {
		case req.Header.Get("X-Test-Admin") != "":
			req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 1}))
		case id != "":
			var n int64
			for _, c := range id {
				n = n*10 + int64(c-'0')
			}
			req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: n}))
		}
		handler.ServeHTTP(rw, req)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &stack{d: d, clock: clk, client: mediav1alpha1connect.NewMediaAppServiceClient(srv.Client(), srv.URL)}
}

func as[T any](account string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Account", account)
	return req
}

var md5b64 = base64.StdEncoding.EncodeToString(make([]byte, 16))

func (s *stack) begin(t *testing.T, account string) *pb.BeginUploadResponse {
	t.Helper()
	resp, err := s.client.BeginUpload(t.Context(), as(account, &pb.BeginUploadRequest{Size: 1024, ContentType: "image/png", Md5: md5b64}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg
}

func (s *stack) rows(t *testing.T) int {
	t.Helper()
	var n int
	if err := s.d.Conn(context.Background()).QueryRow(t.Context(), `SELECT count(*) FROM media.staged_uploads`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBeginCompleteGetRoundTrip(t *testing.T) {
	s := start(t)
	b := s.begin(t, "7")
	if b.GetStagedToken() == "" || b.GetUploadUrl() == "" || b.GetUploadHeaders()["Content-Type"] != "image/png" || b.GetUploadHeaders()["Content-MD5"] != md5b64 {
		t.Fatalf("BeginUpload 回應：%v", b)
	}
	if !b.GetExpireTime().AsTime().Equal(s.clock.Now().Add(media.UploadTTL)) {
		t.Fatalf("到期時間：%v", b.GetExpireTime().AsTime())
	}
	ctx := t.Context()
	got, err := s.client.GetUpload(ctx, as("7", &pb.GetUploadRequest{StagedToken: b.GetStagedToken()}))
	if err != nil || got.Msg.GetState() != pb.UploadState_UPLOAD_STATE_PENDING {
		t.Fatalf("Complete 前：%v %v", err, got)
	}
	for range 2 {
		if _, err := s.client.CompleteUpload(ctx, as("7", &pb.CompleteUploadRequest{StagedToken: b.GetStagedToken()})); err != nil {
			t.Fatalf("CompleteUpload（含重送）：%v", err)
		}
	}
	got, err = s.client.GetUpload(ctx, as("7", &pb.GetUploadRequest{StagedToken: b.GetStagedToken()}))
	if err != nil || got.Msg.GetState() != pb.UploadState_UPLOAD_STATE_UPLOADED || got.Msg.RejectReason != nil {
		t.Fatalf("Complete 後：%v %v", err, got)
	}
}

func TestOverTheLimitIsInvalidArgumentWithNoURLAndNoRow(t *testing.T) {
	s := start(t)
	resp, err := s.client.BeginUpload(t.Context(), as("7", &pb.BeginUploadRequest{Size: media.MaxPublicUploadSize + 1, ContentType: "image/png", Md5: md5b64}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || resp != nil {
		t.Fatalf("超過上限：%v %v", err, resp)
	}
	if n := s.rows(t); n != 0 {
		t.Fatalf("超過上限不該建暫存列：%d", n)
	}
}

func TestAnotherUsersTokenIsNotFoundOnCompleteAndGet(t *testing.T) {
	s := start(t)
	b := s.begin(t, "7")
	ctx := t.Context()
	_, errGet := s.client.GetUpload(ctx, as("8", &pb.GetUploadRequest{StagedToken: b.GetStagedToken()}))
	_, errComplete := s.client.CompleteUpload(ctx, as("8", &pb.CompleteUploadRequest{StagedToken: b.GetStagedToken()}))
	_, errMissing := s.client.GetUpload(ctx, as("7", &pb.GetUploadRequest{StagedToken: "no-such-token"}))
	for name, err := range map[string]error{"Get": errGet, "Complete": errComplete, "不存在的 token": errMissing} {
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s：%v，預期 not_found", name, err)
		}
	}
	got, err := s.client.GetUpload(ctx, as("7", &pb.GetUploadRequest{StagedToken: b.GetStagedToken()}))
	if err != nil || got.Msg.GetState() != pb.UploadState_UPLOAD_STATE_PENDING {
		t.Fatalf("別人的 Complete 不能改動狀態：%v %v", err, got)
	}
}

func TestCompleteAfterExpiryIsFailedPrecondition(t *testing.T) {
	s := start(t)
	b := s.begin(t, "7")
	s.clock.Advance(media.UploadTTL + time.Minute)
	_, err := s.client.CompleteUpload(t.Context(), as("7", &pb.CompleteUploadRequest{StagedToken: b.GetStagedToken()}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("過期後 Complete：%v", err)
	}
}

func TestAdminAndAnonymousAreRefusedBeforeTheModule(t *testing.T) {
	s := start(t)
	req := connect.NewRequest(&pb.BeginUploadRequest{Size: 1, ContentType: "image/png", Md5: md5b64})
	if _, err := s.client.BeginUpload(t.Context(), req); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("未登入：%v", err)
	}
	req = connect.NewRequest(&pb.BeginUploadRequest{Size: 1, ContentType: "image/png", Md5: md5b64})
	req.Header().Set("X-Test-Admin", "1")
	if _, err := s.client.BeginUpload(t.Context(), req); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("管理員：%v", err)
	}
	if n := s.rows(t); n != 0 {
		t.Fatalf("被拒絕的請求不該建列：%d", n)
	}
}
