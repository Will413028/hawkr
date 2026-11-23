package media

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	delegatedAdminID = int64(41)
	delegatedStoreID = int64(5)
)

// fakeSessions 以憑證回 session；AuthorizeDelegated 以與模組相同的規則（非 ACTIVE 或缺方法宣告的能力）拒絕並記下動作與資源
// （模組為這些寫 FAILED 稽核；稽核本身由模組的測試守）。
type fakeSessions struct {
	sessions map[string]moddelegation.DelegationSession
	denied   []moddelegation.DelegatedMethod
	touched  []int64
	// storeErr 是 DelegatedRead 的店家檢查結果（nil 是可代行）；reads 是通過檢查、實際執行 read 的次數。
	storeErr error
	reads    int
	// readStore 非 0 時是 DelegatedRead 交回的店家，與 session 綁定的店家不同，用來證明店家取自它的回傳。
	readStore int64
}

// DelegatedRead 像 delegation 模組：店家不可代行時寫一列 FAILED 稽核（記進 denied）並回模組的拒絕種類（FailedPrecondition）。
func (f *fakeSessions) DelegatedRead(ctx context.Context, c moddelegation.DelegatedCaller, read func(context.Context, seller.Store) error) error {
	if f.storeErr != nil {
		f.denied = append(f.denied, c.Method)
		return f.storeErr
	}
	f.reads++
	id := c.Session.Target.StoreID
	if f.readStore != 0 {
		id = f.readStore
	}
	return read(ctx, seller.Store{StoreID: id})
}

func (f *fakeSessions) TouchDelegation(_ context.Context, id int64) error {
	f.touched = append(f.touched, id)
	return nil
}

func (f *fakeSessions) ResolveDelegation(_ context.Context, c string, _ bool) (moddelegation.DelegationSession, error) {
	s, ok := f.sessions[c]
	if !ok {
		return moddelegation.DelegationSession{}, apperr.Unauthenticated("代行憑證無效")
	}
	return s, nil
}

func (f *fakeSessions) AuthorizeDelegated(_ context.Context, c moddelegation.DelegatedCaller) error {
	if c.Session.Status != moddelegation.DelegationActive {
		f.denied = append(f.denied, c.Method)
		return apperr.FailedPrecondition("代行工作階段已撤銷或逾時")
	}
	if !slices.Contains(c.Session.Capabilities, c.Method.Capability) {
		f.denied = append(f.denied, c.Method)
		return apperr.PermissionDenied("此工作階段未取得執行該操作的能力")
	}
	return nil
}

// fakeRunner 像 delegation 的 DelegatedRead 那樣先授權再執行 read；它不寫稽核（成功的讀取不稽核，見模組的測試）。
type fakeRunner struct {
	reads int
	err   error
}

func (f *fakeRunner) DelegatedRead(ctx context.Context, c moddelegation.DelegatedCaller, read func(context.Context, seller.Store) error) error {
	f.reads++
	if f.err != nil {
		return f.err
	}
	return read(ctx, seller.Store{StoreID: c.Session.Target.StoreID})
}

type fakeBegin struct {
	who  media.Uploader
	size int64
	err  error

	in    media.BeginInput
	token string
	state media.UploadStatus
}

func (f *fakeBegin) BeginUpload(_ context.Context, u media.Uploader, in media.BeginInput) (media.BeginResult, error) {
	f.who, f.in = u, in
	if f.err != nil {
		return media.BeginResult{}, f.err
	}
	return media.BeginResult{Token: "pub", URL: "https://store.example/put", Headers: map[string]string{"Content-MD5": in.MD5}, ExpiresAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)}, nil
}

func (f *fakeBegin) CompleteUpload(_ context.Context, u media.Uploader, token string) error {
	f.who, f.token = u, token
	return f.err
}

func (f *fakeBegin) GetUpload(_ context.Context, u media.Uploader, token string) (media.UploadStatus, error) {
	f.who, f.token = u, token
	return f.state, f.err
}

func (f *fakeBegin) BeginSecureUpload(_ context.Context, u media.Uploader, size int64) (media.SecureBeginResult, error) {
	f.who, f.size = u, size
	if f.err != nil {
		return media.SecureBeginResult{}, f.err
	}
	return media.SecureBeginResult{Token: "tok", ChunkSize: media.SecureChunkSize, ChunkCount: 3, ExpiresAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)}, nil
}

type delegatedEnv struct {
	sessions *fakeSessions
	runner   *fakeRunner
	begin    *fakeBegin
	client   mediav1alpha1connect.MediaDelegatedServiceClient
}

func newDelegatedEnv(t *testing.T, caps ...moddelegation.Capability) *delegatedEnv {
	t.Helper()
	s := &fakeSessions{sessions: map[string]moddelegation.DelegationSession{
		"cred": {ID: 9, Status: moddelegation.DelegationActive, Capabilities: caps,
			Actor: moddelegation.DelegationActor{AdminID: delegatedAdminID}, Target: moddelegation.DelegationTarget{StoreID: delegatedStoreID}},
	}}
	r, b := &fakeRunner{}, &fakeBegin{}
	in, err := httpdelegation.NewInterceptor(s, slog.New(slog.DiscardHandler), DelegatedDeclaration)
	if err != nil {
		t.Fatal(err)
	}
	h := NewDelegated(r, b, nil)
	path, handler := mediav1alpha1connect.NewMediaDelegatedServiceHandler(h, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)), in))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &delegatedEnv{sessions: s, runner: r, begin: b, client: mediav1alpha1connect.NewMediaDelegatedServiceClient(srv.Client(), srv.URL)}
}

func begin(e *delegatedEnv, cred string, size int64) (*connect.Response[pb.MediaDelegatedServiceBeginSecureUploadResponse], error) {
	req := connect.NewRequest(&pb.MediaDelegatedServiceBeginSecureUploadRequest{Size: size})
	if cred != "" {
		req.Header().Set(httpdelegation.OperationSessionHeader, cred)
	}
	return e.client.BeginSecureUpload(context.Background(), req)
}

// 缺 media.upload 的代行被拒絕（permission_denied）：記下的稽核動作是 MEDIA_UPLOAD、資源是 MEDIA，會話沒有被開。
func TestDelegatedBeginSecureUploadWithoutTheCapabilityIsDenied(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityStorefrontWrite, moddelegation.CapabilityPostsWrite, moddelegation.CapabilityProductsWrite)
	_, err := begin(e, "cred", 1024)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("err = %v，預期 permission_denied", err)
	}
	if len(e.sessions.denied) != 1 || e.sessions.denied[0].Capability != moddelegation.CapabilityMediaUpload ||
		e.sessions.denied[0].Action != moddelegation.AuditMediaUpload || e.sessions.denied[0].Resource != moddelegation.ResourceMedia {
		t.Fatalf("被拒絕的宣告 = %+v", e.sessions.denied)
	}
	if e.begin.size != 0 || e.runner.reads != 0 {
		t.Fatalf("被拒絕的請求不該開會話：size=%d reads=%d", e.begin.size, e.runner.reads)
	}
}

func TestDelegatedBeginSecureUploadWithoutCredentialIsUnauthenticated(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityMediaUpload)
	for _, cred := range []string{"", "bad"} {
		if _, err := begin(e, cred, 1024); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("憑證 %q：err = %v，預期 unauthenticated", cred, err)
		}
	}
}

// 會話的上傳者是管理員（{ID: 代行的管理員, Admin: true}）並綁定被代行的店家（Store），不是店家或其他人；回應帶 token、段大小與段總數。
func TestDelegatedBeginSecureUploadUploaderIsTheAdmin(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityMediaUpload)
	res, err := begin(e, "cred", 4096)
	if err != nil {
		t.Fatal(err)
	}
	if e.begin.who != (media.Uploader{ID: delegatedAdminID, Admin: true, Store: delegatedStoreID}) || e.begin.size != 4096 {
		t.Fatalf("模組收到 uploader=%+v size=%d", e.begin.who, e.begin.size)
	}
	if res.Msg.GetStagedToken() != "tok" || res.Msg.GetChunkSize() != media.SecureChunkSize || res.Msg.GetChunkCount() != 3 || res.Msg.GetExpireTime() == nil {
		t.Fatalf("回應 = %v", res.Msg)
	}
	if len(e.sessions.denied) != 0 {
		t.Fatalf("放行的請求不該被拒絕：%+v", e.sessions.denied)
	}
}

func TestDelegatedBeginSecureUploadErrors(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityMediaUpload)
	e.begin.err = media.ErrNoKeyring
	if _, err := begin(e, "cred", 10); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("沒有金鑰環：err = %v，預期 unavailable", err)
	}
	e.begin.err = apperr.InvalidInputOn([]string{"size"}, "大小不合法")
	if _, err := begin(e, "cred", 0); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("大小不合法：err = %v，預期 invalid_argument", err)
	}
	e.begin.err = nil
	e.runner.err = errors.New("店家不可代行")
	if _, err := begin(e, "cred", 10); err == nil {
		t.Error("DelegatedRead 的拒絕要原樣回傳")
	}
}

// media 還沒接上（nil 模組）時，授權過後回 unavailable。
func TestDelegatedBeginSecureUploadWithoutModuleIsUnavailable(t *testing.T) {
	s := &fakeSessions{sessions: map[string]moddelegation.DelegationSession{
		"cred": {ID: 9, Status: moddelegation.DelegationActive, Capabilities: []moddelegation.Capability{moddelegation.CapabilityMediaUpload}},
	}}
	in, err := httpdelegation.NewInterceptor(s, slog.New(slog.DiscardHandler), DelegatedDeclaration)
	if err != nil {
		t.Fatal(err)
	}
	path, handler := mediav1alpha1connect.NewMediaDelegatedServiceHandler(NewDelegated(&fakeRunner{}, nil, nil), connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)), in))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	req := connect.NewRequest(&pb.MediaDelegatedServiceBeginSecureUploadRequest{Size: 1})
	req.Header().Set(httpdelegation.OperationSessionHeader, "cred")
	_, err = mediav1alpha1connect.NewMediaDelegatedServiceClient(srv.Client(), srv.URL).BeginSecureUpload(context.Background(), req)
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("err = %v，預期 unavailable", err)
	}
}

func withCred[T any](m *T, c *connect.Request[T]) *connect.Request[T] {
	c.Header().Set(httpdelegation.OperationSessionHeader, "cred")
	return c
}

func TestDelegatedPublicUploadUploaderIsTheAdminOnTheStore(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityMediaUpload)
	want := media.Uploader{ID: delegatedAdminID, Admin: true, Store: delegatedStoreID}
	ctx := context.Background()

	bres, err := e.client.BeginUpload(ctx, withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceBeginUploadRequest{Size: 10, ContentType: "image/png", Md5: "AAAAAAAAAAAAAAAAAAAAAA=="})))
	if err != nil {
		t.Fatal(err)
	}
	if e.begin.who != want || e.begin.in.Size != 10 || e.begin.in.ContentType != "image/png" || e.begin.in.MD5 != "AAAAAAAAAAAAAAAAAAAAAA==" {
		t.Fatalf("BeginUpload：uploader=%+v in=%+v", e.begin.who, e.begin.in)
	}
	if bres.Msg.GetStagedToken() != "pub" || bres.Msg.GetUploadUrl() != "https://store.example/put" || bres.Msg.GetUploadHeaders()["Content-MD5"] == "" || bres.Msg.GetExpireTime() == nil {
		t.Fatalf("回應 = %v", bres.Msg)
	}

	e.begin.who = media.Uploader{}
	if _, err := e.client.CompleteUpload(ctx, withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceCompleteUploadRequest{StagedToken: "t1"}))); err != nil {
		t.Fatal(err)
	}
	if e.begin.who != want || e.begin.token != "t1" {
		t.Fatalf("CompleteUpload：uploader=%+v token=%q", e.begin.who, e.begin.token)
	}

	e.begin.who = media.Uploader{}
	e.begin.state = media.UploadStatus{State: "REJECTED", RejectReason: "SIZE_MISMATCH", ExpiresAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)}
	gres, err := e.client.GetUpload(ctx, withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceGetUploadRequest{StagedToken: "t2"})))
	if err != nil {
		t.Fatal(err)
	}
	if e.begin.who != want || e.begin.token != "t2" {
		t.Fatalf("GetUpload：uploader=%+v token=%q", e.begin.who, e.begin.token)
	}
	if gres.Msg.GetState() != pb.UploadState_UPLOAD_STATE_REJECTED || gres.Msg.RejectReason == nil || gres.Msg.GetRejectReason() != pb.RejectReason_REJECT_REASON_SIZE_MISMATCH || gres.Msg.GetExpireTime() == nil {
		t.Fatalf("回應 = %v", gres.Msg)
	}
	if len(e.sessions.denied) != 0 {
		t.Fatalf("放行的請求不該被拒絕：%+v", e.sessions.denied)
	}
}

func TestDelegatedPublicUploadWithoutTheCapabilityIsDenied(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityPostsWrite)
	ctx := context.Background()
	calls := map[string]func() error{
		"BeginUpload": func() error {
			_, err := e.client.BeginUpload(ctx, withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceBeginUploadRequest{Size: 1})))
			return err
		},
		"CompleteUpload": func() error {
			_, err := e.client.CompleteUpload(ctx, withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceCompleteUploadRequest{StagedToken: "t"})))
			return err
		},
		"GetUpload": func() error {
			_, err := e.client.GetUpload(ctx, withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceGetUploadRequest{StagedToken: "t"})))
			return err
		},
	}
	for name, call := range calls {
		if connect.CodeOf(call()) != connect.CodePermissionDenied {
			t.Errorf("%s 缺能力要 permission_denied", name)
		}
	}
	if e.runner.reads != 0 || e.begin.token != "" || e.begin.in.Size != 0 {
		t.Fatalf("被拒絕的請求不該進模組：%+v", e.begin)
	}
}

func TestDelegatedPublicUploadWithoutCredentialIsUnauthenticated(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityMediaUpload)
	_, err := e.client.GetUpload(context.Background(), connect.NewRequest(&pb.MediaDelegatedServiceGetUploadRequest{StagedToken: "t"}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("err = %v", err)
	}
}

func TestDelegatedPublicUploadModuleErrorsPassThrough(t *testing.T) {
	e := newDelegatedEnv(t, moddelegation.CapabilityMediaUpload)
	e.begin.err = apperr.NotFound("暫存不存在")
	_, err := e.client.CompleteUpload(context.Background(), withCred(nil, connect.NewRequest(&pb.MediaDelegatedServiceCompleteUploadRequest{StagedToken: "x"})))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}
