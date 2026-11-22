package media

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

type fakeUploads struct {
	begun    []media.BeginInput
	who      []media.Uploader
	tokens   []string
	beginRes media.BeginResult
	status   media.UploadStatus
	err      error
}

func (f *fakeUploads) BeginUpload(_ context.Context, u media.Uploader, in media.BeginInput) (media.BeginResult, error) {
	f.who, f.begun = append(f.who, u), append(f.begun, in)
	return f.beginRes, f.err
}

func (f *fakeUploads) CompleteUpload(_ context.Context, u media.Uploader, token string) error {
	f.who, f.tokens = append(f.who, u), append(f.tokens, token)
	return f.err
}

func (f *fakeUploads) GetUpload(_ context.Context, u media.Uploader, token string) (media.UploadStatus, error) {
	f.who, f.tokens = append(f.who, u), append(f.tokens, token)
	return f.status, f.err
}

type noJobs struct{}

func (noJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

type env struct {
	client mediav1alpha1connect.MediaAppServiceClient
	url    string
}

// newEnv 掛上真的 Connect handler 與 rpc 的 interceptor；header X-Test-Account 非空時以帳號 7 登入，X-Test-Admin 非空時以管理員登入。
func newEnv(t *testing.T, app *App) *env {
	t.Helper()
	path, handler := mediav1alpha1connect.NewMediaAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.NewTextHandler(io.Discard, nil)))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case req.Header.Get("X-Test-Account") != "":
			req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
		case req.Header.Get("X-Test-Admin") != "":
			req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}))
		}
		handler.ServeHTTP(rw, req)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &env{url: srv.URL, client: mediav1alpha1connect.NewMediaAppServiceClient(srv.Client(), srv.URL)}
}

func signedIn[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Account", "1")
	return req
}

func asAdmin[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Admin", "1")
	return req
}

const token = "x"

func TestMediaAppRequiresALogin(t *testing.T) {
	f := &fakeUploads{}
	e := newEnv(t, newApp(f))
	methods := pb.File_hawkr_media_v1alpha1_media_proto.Services().ByName("MediaAppService").Methods()
	if methods.Len() != 3 {
		t.Fatalf("MediaAppService 有 %d 支方法，預期 3", methods.Len())
	}
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, e.url+"/hawkr.media.v1alpha1.MediaAppService/"+name, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s 未登入回 %d，預期 401", name, resp.StatusCode)
		}
	}
	if len(f.who) != 0 {
		t.Fatalf("未登入不該到模組：%v", f.who)
	}
}

func TestOnlyAnAccountCanCall(t *testing.T) {
	f := &fakeUploads{}
	e := newEnv(t, newApp(f))
	ctx := t.Context()
	_, err1 := e.client.BeginUpload(ctx, asAdmin(&pb.BeginUploadRequest{Size: 1, ContentType: "image/png", Md5: "x"}))
	_, err2 := e.client.CompleteUpload(ctx, asAdmin(&pb.CompleteUploadRequest{StagedToken: token}))
	_, err3 := e.client.GetUpload(ctx, asAdmin(&pb.GetUploadRequest{StagedToken: token}))
	for i, err := range []error{err1, err2, err3} {
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("方法 %d：管理員回 %v，預期 permission_denied", i, err)
		}
	}
	if len(f.who) != 0 {
		t.Fatalf("管理員不該到模組：%v", f.who)
	}
}

func TestBeginUploadPassesTheDeclarationAndReturnsTheSignedUpload(t *testing.T) {
	exp := time.Date(2026, 11, 23, 1, 2, 3, 0, time.UTC)
	f := &fakeUploads{beginRes: media.BeginResult{ID: uuid.New(), Token: "tok", URL: "https://s.example/tmp/a", Headers: map[string]string{"Content-Type": "image/png", "Content-MD5": "m"}, ExpiresAt: exp}}
	e := newEnv(t, newApp(f))
	resp, err := e.client.BeginUpload(t.Context(), signedIn(&pb.BeginUploadRequest{Size: 1234, ContentType: "image/png", Md5: "m"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.begun) != 1 || f.begun[0] != (media.BeginInput{Size: 1234, ContentType: "image/png", MD5: "m"}) || f.who[0] != (media.Uploader{ID: 7}) {
		t.Fatalf("送給模組的輸入：%+v %+v", f.begun, f.who)
	}
	m := resp.Msg
	if m.GetStagedToken() != "tok" || m.GetUploadUrl() != "https://s.example/tmp/a" || m.GetUploadHeaders()["Content-MD5"] != "m" || m.GetUploadHeaders()["Content-Type"] != "image/png" || !m.GetExpireTime().AsTime().Equal(exp) {
		t.Fatalf("回應：%v", m)
	}
}

func TestModuleErrorsAreMapped(t *testing.T) {
	cases := map[string]struct {
		err  error
		code connect.Code
	}{
		"InvalidInput":       {apperr.InvalidInputOn([]string{"size"}, "太大"), connect.CodeInvalidArgument},
		"NotFound":           {apperr.NotFound("找不到這筆上傳"), connect.CodeNotFound},
		"FailedPrecondition": {apperr.FailedPrecondition("這筆上傳已過期"), connect.CodeFailedPrecondition},
	}
	for name, c := range cases {
		e := newEnv(t, newApp(&fakeUploads{err: c.err}))
		ctx := t.Context()
		_, err1 := e.client.BeginUpload(ctx, signedIn(&pb.BeginUploadRequest{Size: 1, ContentType: "image/png", Md5: "m"}))
		_, err2 := e.client.CompleteUpload(ctx, signedIn(&pb.CompleteUploadRequest{StagedToken: token}))
		_, err3 := e.client.GetUpload(ctx, signedIn(&pb.GetUploadRequest{StagedToken: token}))
		for i, err := range []error{err1, err2, err3} {
			if connect.CodeOf(err) != c.code {
				t.Errorf("%s 方法 %d：%v，預期 %v", name, i, err, c.code)
				continue
			}
			var lm bool
			var ce *connect.Error
			_ = errors.As(err, &ce)
			for _, d := range ce.Details() {
				if v, derr := d.Value(); derr == nil {
					_, ok := v.(*errdetails.LocalizedMessage)
					lm = lm || ok
				}
			}
			if !lm {
				t.Errorf("%s 方法 %d：沒有 LocalizedMessage", name, i)
			}
		}
	}
}

func TestExpiredCompleteIsFailedPreconditionNotNotFound(t *testing.T) {
	e := newEnv(t, newApp(&fakeUploads{err: apperr.FailedPrecondition("這筆上傳已過期")}))
	_, err := e.client.CompleteUpload(t.Context(), signedIn(&pb.CompleteUploadRequest{StagedToken: token}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("過期：%v", err)
	}
}

func TestGetUploadReportsRejectionAsStateAndReasonNotAnError(t *testing.T) {
	exp := time.Date(2026, 11, 23, 1, 2, 3, 0, time.UTC)
	f := &fakeUploads{status: media.UploadStatus{State: media.UploadRejected, RejectReason: media.RejectUnrecognizedFormat, ExpiresAt: exp}}
	e := newEnv(t, newApp(f))
	resp, err := e.client.GetUpload(t.Context(), signedIn(&pb.GetUploadRequest{StagedToken: "tok"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetState() != pb.UploadState_UPLOAD_STATE_REJECTED || resp.Msg.GetRejectReason() != pb.RejectReason_REJECT_REASON_UNRECOGNIZED_FORMAT || resp.Msg.RejectReason == nil || !resp.Msg.GetExpireTime().AsTime().Equal(exp) || f.tokens[0] != "tok" {
		t.Fatalf("回應：%v", resp.Msg)
	}
	f.status = media.UploadStatus{State: media.UploadReady, ExpiresAt: exp}
	resp, err = e.client.GetUpload(t.Context(), signedIn(&pb.GetUploadRequest{StagedToken: "tok"}))
	if err != nil || resp.Msg.GetState() != pb.UploadState_UPLOAD_STATE_READY || resp.Msg.RejectReason != nil {
		t.Fatalf("READY：%v %v", err, resp)
	}
}

func TestEnumsMatchTheModuleValues(t *testing.T) {
	states := []media.UploadState{media.UploadPending, media.UploadUploaded, media.UploadReady, media.UploadRejected, media.UploadConfirmed, media.UploadDeleting, media.UploadDeleted}
	if bad := rpc.EnumMismatches[pb.UploadState](states); len(bad) != 0 {
		t.Fatal(bad)
	}
	reasons := []media.RejectReason{media.RejectUnrecognizedFormat, media.RejectSizeMismatch, media.RejectChecksumMismatch, media.RejectProcessingFailed, media.RejectExpired}
	if bad := rpc.EnumMismatches[pb.RejectReason](reasons); len(bad) != 0 {
		t.Fatal(bad)
	}
}

func TestWithoutAModuleEveryMethodIsUnavailable(t *testing.T) {
	e := newEnv(t, NewApp((*media.Module)(nil)))
	ctx := t.Context()
	_, err1 := e.client.BeginUpload(ctx, signedIn(&pb.BeginUploadRequest{Size: 1, ContentType: "image/png", Md5: "m"}))
	_, err2 := e.client.CompleteUpload(ctx, signedIn(&pb.CompleteUploadRequest{StagedToken: token}))
	_, err3 := e.client.GetUpload(ctx, signedIn(&pb.GetUploadRequest{StagedToken: token}))
	for i, err := range []error{err1, err2, err3} {
		if connect.CodeOf(err) != connect.CodeUnavailable {
			t.Errorf("方法 %d：沒有接上模組回 %v，預期 unavailable", i, err)
		}
	}
}

// 超過上限的宣告由模組的大小檢查擋下：真的模組（驗證在碰資料庫與儲存之前），不發網址、不寫列。
func TestBeginUploadOverTheLimitIsInvalidArgumentAndIssuesNoURL(t *testing.T) {
	store := media.NewMemoryStore()
	m := media.New(media.Deps{DB: &db.DB{}, Jobs: noJobs{}, Store: store, NewID: uuid.NewV7, Now: time.Now})
	e := newEnv(t, NewApp(m))
	resp, err := e.client.BeginUpload(t.Context(), signedIn(&pb.BeginUploadRequest{Size: media.MaxPublicUploadSize + 1, ContentType: "image/png", Md5: "1B2M2Y8AsgTpgAmY7PhCfg=="}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("超過上限：%v", err)
	}
	if resp != nil {
		t.Fatalf("不該有回應：%v", resp)
	}
	if store.Presigns() != 0 {
		t.Fatalf("超過上限仍發了 %d 次預簽網址", store.Presigns())
	}
	var bad *errdetails.BadRequest
	var ce *connect.Error
	_ = errors.As(err, &ce)
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if br, ok := v.(*errdetails.BadRequest); ok {
				bad = br
			}
		}
	}
	if bad == nil || len(bad.GetFieldViolations()) != 1 || bad.GetFieldViolations()[0].GetField() != "size" {
		t.Fatalf("BadRequest 應指向 size：%v", err)
	}
}
