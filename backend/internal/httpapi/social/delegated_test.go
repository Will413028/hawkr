package social

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	goodSession     = "good-session"
	delegatedStore  = int64(5)
	delegatedSessID = int64(9)
)

// fakeSessions 以憑證回 session；沒有的憑證回模組的「憑證無效」拒絕。AuthorizeDelegated 以同樣的規則（非 ACTIVE 或缺能力）拒絕，並記下被拒絕的呼叫
// （模組為這些寫 FAILED 稽核；稽核本身由模組的 integration 測試守）。
type fakeSessions struct {
	sessions map[string]moddelegation.DelegationSession
	resolved []string
	activity []bool
	denied   []string
	touched  []int64
}

func (f *fakeSessions) TouchDelegation(_ context.Context, id int64) error {
	f.touched = append(f.touched, id)
	return nil
}

func (f *fakeSessions) ResolveDelegation(_ context.Context, c string, activity bool) (moddelegation.DelegationSession, error) {
	f.resolved = append(f.resolved, c)
	f.activity = append(f.activity, activity)
	s, ok := f.sessions[c]
	if !ok {
		return moddelegation.DelegationSession{}, apperr.Unauthenticated("代行憑證無效")
	}
	return s, nil
}

func (f *fakeSessions) AuthorizeDelegated(_ context.Context, c moddelegation.DelegatedCaller) error {
	var err error
	switch {
	case c.Session.Status != moddelegation.DelegationActive:
		err = apperr.FailedPrecondition("代行工作階段已結束")
	case !slices.Contains(c.Session.Capabilities, c.Method.Capability):
		err = apperr.PermissionDenied("此工作階段未取得執行該操作的能力")
	}
	if err != nil {
		f.denied = append(f.denied, string(c.Method.Action))
	}
	return err
}

func delegatedSession(status moddelegation.DelegationStatus, caps ...moddelegation.Capability) moddelegation.DelegationSession {
	return moddelegation.DelegationSession{ID: delegatedSessID, Status: status, Capabilities: caps, Target: moddelegation.DelegationTarget{StoreID: delegatedStore}}
}

var allDelegatedCaps = []moddelegation.Capability{moddelegation.CapabilityPostsRead, moddelegation.CapabilityPostsWrite, moddelegation.CapabilityPostsPublish}

// fakeRunner 記錄 handler 交給元件的呼叫，並像元件一樣先授權、再執行 read 與 do；它不寫稽核也不做冪等（那是模組的規則，見模組的 integration 測試）。
type fakeRunner struct {
	calls    []string
	caller   moddelegation.DelegatedCaller
	write    moddelegation.DelegatedWrite
	audit    uuid.UUID
	replayed bool
	replayID string
	err      error
}

func (f *fakeRunner) DelegatedRead(ctx context.Context, c moddelegation.DelegatedCaller, read func(context.Context, seller.Store) error) error {
	f.calls = append(f.calls, "read:"+string(c.Method.Action))
	f.caller = c
	if f.err != nil {
		return f.err
	}
	return read(ctx, seller.Store{StoreID: c.Session.Target.StoreID})
}

func (f *fakeRunner) DelegatedWrite(ctx context.Context, c moddelegation.DelegatedCaller, w moddelegation.DelegatedWrite,
	do func(context.Context, seller.Store) (moddelegation.DelegatedOutcome, error)) (moddelegation.DelegatedWriteResult, error) {
	f.calls = append(f.calls, "write:"+string(c.Method.Action))
	f.caller, f.write = c, w
	if f.err != nil {
		return moddelegation.DelegatedWriteResult{}, f.err
	}
	st := seller.Store{StoreID: c.Session.Target.StoreID}
	if f.replayed {
		return moddelegation.DelegatedWriteResult{Store: st, AuditLogID: f.audit, ResourceID: f.replayID, Replayed: true}, nil
	}
	out, err := do(ctx, st)
	if err != nil {
		return moddelegation.DelegatedWriteResult{}, err
	}
	return moddelegation.DelegatedWriteResult{Store: st, AuditLogID: f.audit, ResourceID: out.ResourceID}, nil
}

// fakePosts 記錄 social 的呼叫與收到的輸入，回固定的結果；沒有覆寫的 Owner 方法被呼叫會 panic。
type fakePosts struct {
	Owner
	create social.CreatePostInput
	update social.UpdatePostInput
	trans  struct {
		post    uuid.UUID
		version int64
	}
	sched   social.SchedulePostInput
	status  social.PostStatus
	after   *social.OwnPostCursor
	store   seller.Store
	post    social.Post
	page    social.OwnPostPage
	pending *social.Schedule
	publish time.Time
	err     error
}

func (f *fakePosts) CreatePost(_ context.Context, st seller.Store, in social.CreatePostInput) (social.Post, error) {
	f.store, f.create = st, in
	return f.post, f.err
}

func (f *fakePosts) UpdatePost(_ context.Context, st seller.Store, in social.UpdatePostInput) (social.Post, error) {
	f.store, f.update = st, in
	return f.post, f.err
}

func (f *fakePosts) PublishPost(_ context.Context, st seller.Store, id uuid.UUID, v int64) (social.Post, error) {
	f.store, f.trans.post, f.trans.version = st, id, v
	return f.post, f.err
}

func (f *fakePosts) UnpublishPost(_ context.Context, st seller.Store, id uuid.UUID, v int64) (social.Post, error) {
	f.store, f.trans.post, f.trans.version = st, id, v
	return f.post, f.err
}

func (f *fakePosts) SchedulePost(_ context.Context, st seller.Store, in social.SchedulePostInput) (social.ScheduledPost, error) {
	f.store, f.sched = st, in
	return social.ScheduledPost{Post: f.post, Schedule: social.Schedule{PublishAt: f.publish}}, f.err
}

func (f *fakePosts) MyPost(_ context.Context, st seller.Store, _ uuid.UUID) (social.Post, error) {
	f.store = st
	return f.post, f.err
}

func (f *fakePosts) ListMyPosts(_ context.Context, st seller.Store, status social.PostStatus, after *social.OwnPostCursor, _ int32) (social.OwnPostPage, error) {
	f.store, f.status, f.after = st, status, after
	return f.page, f.err
}

func (f *fakePosts) PendingSchedule(context.Context, int64, uuid.UUID) (social.Schedule, error) {
	if f.pending == nil {
		return social.Schedule{}, apperr.NotFound("沒有待發布的排程")
	}
	return *f.pending, nil
}

type noBuyers struct{}

func (noBuyers) CurrentBuyerID(context.Context, int64) (int64, error)           { return 0, nil }
func (noBuyers) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) { return nil, nil }

type delegatedCards struct{}

func (delegatedCards) CurrentStore(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, nil
}

func (delegatedCards) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	name := "shop"
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		out[i] = seller.StoreCard{StoreID: id, StoreCode: id, Name: &name}
	}
	return out, nil
}

func (delegatedCards) StoresBlockingBuyer(context.Context, int64, []int64) (map[int64]bool, error) {
	return nil, nil
}

type delegatedEnv struct {
	sessions *fakeSessions
	runner   *fakeRunner
	posts    *fakePosts
	client   socialv1alpha1connect.SocialDelegatedServiceClient
	logs     *bytes.Buffer
}

func newDelegatedEnv(t *testing.T, sessions map[string]moddelegation.DelegationSession) *delegatedEnv {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSessions{sessions: sessions}
	r := &fakeRunner{audit: uuid.New()}
	p := &fakePosts{post: social.Post{ID: uuid.New(), StoreID: delegatedStore, Body: "內文", Version: 3, Status: social.StatusDraft, CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	h, err := NewDelegated(r, p, noBuyers{}, delegatedCards{}, images, web)
	if err != nil {
		t.Fatal(err)
	}
	in, err := httpdelegation.NewInterceptor(s, logger, DelegatedDeclaration)
	if err != nil {
		t.Fatal(err)
	}
	path, handler := socialv1alpha1connect.NewSocialDelegatedServiceHandler(h, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)), in))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &delegatedEnv{sessions: s, runner: r, posts: p, logs: logs, client: socialv1alpha1connect.NewSocialDelegatedServiceClient(srv.Client(), srv.URL)}
}

func withCredential[T any](msg *T, c string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if c != "" {
		req.Header().Set(httpdelegation.OperationSessionHeader, c)
	}
	return req
}

func v4() string {
	id := uuid.New().String()
	return id[:14] + "4" + id[15:19] + "8" + id[20:]
}

func delegatedOp() *pb.DelegatedOperation {
	return &pb.DelegatedOperation{RequestId: v4(), OperationReason: "  店家請我們代發  "}
}

func goodSessions() map[string]moddelegation.DelegationSession {
	return map[string]moddelegation.DelegationSession{goodSession: delegatedSession(moddelegation.DelegationActive, allDelegatedCaps...)}
}

// 每個方法的一個合法請求，用來驗證「進得去 handler」與「被攔下」。
func delegatedCalls(t *testing.T, e *delegatedEnv, cred string) map[string]func() error {
	t.Helper()
	id := uuid.New().String()
	ctx := t.Context()
	future := timestamppb.New(time.Now().Add(24 * time.Hour))
	return map[string]func() error{
		socialv1alpha1connect.SocialDelegatedServiceListStorePostsProcedure: func() error {
			_, err := e.client.ListStorePosts(ctx, withCredential(&pb.ListStorePostsRequest{}, cred))
			return err
		},
		socialv1alpha1connect.SocialDelegatedServiceGetStorePostProcedure: func() error {
			_, err := e.client.GetStorePost(ctx, withCredential(&pb.GetStorePostRequest{PostId: id}, cred))
			return err
		},
		socialv1alpha1connect.SocialDelegatedServiceCreateStorePostProcedure: func() error {
			_, err := e.client.CreateStorePost(ctx, withCredential(&pb.CreateStorePostRequest{Operation: delegatedOp(), Body: "x"}, cred))
			return err
		},
		socialv1alpha1connect.SocialDelegatedServiceUpdateStorePostProcedure: func() error {
			_, err := e.client.UpdateStorePost(ctx, withCredential(&pb.UpdateStorePostRequest{Operation: delegatedOp(), Post: &pb.Post{Id: id, Etag: "1", Body: "x"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}}}, cred))
			return err
		},
		socialv1alpha1connect.SocialDelegatedServicePublishStorePostProcedure: func() error {
			_, err := e.client.PublishStorePost(ctx, withCredential(&pb.PublishStorePostRequest{Operation: delegatedOp(), PostId: id, Etag: "1"}, cred))
			return err
		},
		socialv1alpha1connect.SocialDelegatedServiceUnpublishStorePostProcedure: func() error {
			_, err := e.client.UnpublishStorePost(ctx, withCredential(&pb.UnpublishStorePostRequest{Operation: delegatedOp(), PostId: id, Etag: "1"}, cred))
			return err
		},
		socialv1alpha1connect.SocialDelegatedServiceScheduleStorePostProcedure: func() error {
			_, err := e.client.ScheduleStorePost(ctx, withCredential(&pb.ScheduleStorePostRequest{Operation: delegatedOp(), PostId: id, Etag: "1", PublishTime: future}, cred))
			return err
		},
	}
}

func TestEveryDelegatedMethodReachesTheComponentWithAGoodSession(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	all := delegatedCalls(t, e, goodSession)
	if len(all) != len(DelegatedDeclaration.Methods) {
		t.Fatalf("測試涵蓋 %d 個方法，宣告 %d 個", len(all), len(DelegatedDeclaration.Methods))
	}
	for proc, do := range all {
		before := len(e.runner.calls)
		if err := do(); err != nil {
			t.Errorf("%s：%v", proc, err)
		}
		if len(e.runner.calls) != before+1 {
			t.Errorf("%s 沒有到元件", proc)
		}
		if want := DelegatedDeclaration.Methods[proc]; e.runner.caller.Method != want {
			t.Errorf("%s 交給元件的方法宣告 = %+v，預期 %+v", proc, e.runner.caller.Method, want)
		}
	}
	// 每個請求只唯讀解析一次（不回頭再解析），成功之後才順延閒置逾時。
	if len(e.sessions.activity) != len(all) || slices.Contains(e.sessions.activity, true) {
		t.Fatalf("解析 %v，預期每個請求唯讀解析 1 次", e.sessions.activity)
	}
	if len(e.sessions.touched) != len(all) {
		t.Fatalf("成功的請求順延 %d 次，預期 %d 次", len(e.sessions.touched), len(all))
	}
	if len(e.sessions.denied) != 0 {
		t.Fatalf("好的 session 不該被拒絕：%v", e.sessions.denied)
	}
}

// 憑證缺少或無效：unauthenticated，不到元件、沒有 session 可歸屬所以不授權也不寫稽核列，只記一筆結構化的安全 log（procedure，不含憑證的值）。
func TestMissingOrInvalidCredentialIsUnauthenticatedAndOnlyLogged(t *testing.T) {
	for name, cred := range map[string]string{"沒有標頭": "", "不認得的憑證": "secret-credential-value"} {
		e := newDelegatedEnv(t, goodSessions())
		for proc, do := range delegatedCalls(t, e, cred) {
			if err := do(); connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Errorf("%s %s：%v，預期 unauthenticated", name, proc, err)
			}
		}
		if len(e.runner.calls) != 0 || len(e.sessions.denied) != 0 {
			t.Fatalf("%s：不該到元件或授權：%v %v", name, e.runner.calls, e.sessions.denied)
		}
		logs := e.logs.String()
		if n := strings.Count(logs, "delegation_credential_unresolved"); n != len(DelegatedDeclaration.Methods) {
			t.Fatalf("%s：安全 log %d 筆，預期每個請求一筆（%d）：%s", name, n, len(DelegatedDeclaration.Methods), logs)
		}
		if !strings.Contains(logs, socialv1alpha1connect.SocialDelegatedServiceCreateStorePostProcedure) || strings.Contains(logs, "secret-credential-value") {
			t.Fatalf("%s：log 要有 procedure、不得有憑證的值：%s", name, logs)
		}
	}
}

// session 已結束、到期或撤銷：failed_precondition，不到元件；每個被拒絕的請求（含讀取）都交給授權（模組為它寫 FAILED 稽核），帶著該方法的稽核動作。
func TestEndedSessionsAreRejectedAndAudited(t *testing.T) {
	for _, status := range []moddelegation.DelegationStatus{moddelegation.DelegationRevoked, moddelegation.DelegationExpired} {
		e := newDelegatedEnv(t, map[string]moddelegation.DelegationSession{goodSession: delegatedSession(status, allDelegatedCaps...)})
		for proc, do := range delegatedCalls(t, e, goodSession) {
			if err := do(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Errorf("%s 的 session（%s）：%v，預期 failed_precondition", proc, status, err)
			}
		}
		if len(e.runner.calls) != 0 {
			t.Fatalf("%s 的 session 不該到元件：%v", status, e.runner.calls)
		}
		if slices.Contains(e.sessions.activity, true) || len(e.sessions.touched) != 0 {
			t.Fatalf("%s：被拒絕的請求不該算活動（順延閒置逾時）：%v %v", status, e.sessions.activity, e.sessions.touched)
		}
		if len(e.sessions.denied) != len(DelegatedDeclaration.Methods) || !slices.Contains(e.sessions.denied, "POST_READ") {
			t.Fatalf("%s：被拒絕的請求 %v，預期每個方法一次（含讀取的 POST_READ）", status, e.sessions.denied)
		}
	}
}

// 缺能力被擋下：每個方法依宣告表的能力，拿掉那一個能力（其他兩個都在）就被 permission_denied 擋下，不到元件。
func TestMissingCapabilityIsRejected(t *testing.T) {
	for proc, method := range DelegatedDeclaration.Methods {
		var caps []moddelegation.Capability
		for _, c := range allDelegatedCaps {
			if c != method.Capability {
				caps = append(caps, c)
			}
		}
		e := newDelegatedEnv(t, map[string]moddelegation.DelegationSession{goodSession: delegatedSession(moddelegation.DelegationActive, caps...)})
		err := delegatedCalls(t, e, goodSession)[proc]()
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s 缺 %s：%v，預期 permission_denied", proc, method.Capability, err)
		}
		if len(e.runner.calls) != 0 {
			t.Errorf("%s 缺 %s 不該到元件：%v", proc, method.Capability, e.runner.calls)
		}
		if len(e.sessions.denied) != 1 || e.sessions.denied[0] != string(method.Action) {
			t.Errorf("%s 缺 %s：被拒絕的稽核動作 %v，預期 [%s]", proc, method.Capability, e.sessions.denied, method.Action)
		}
	}
}

func TestCapabilitiesAndActionsAreDeclaredPerMethod(t *testing.T) {
	want := map[string]moddelegation.DelegatedMethod{
		"ListStorePosts":     postMethod(moddelegation.CapabilityPostsRead, moddelegation.AuditPostRead),
		"GetStorePost":       postMethod(moddelegation.CapabilityPostsRead, moddelegation.AuditPostRead),
		"CreateStorePost":    postMethod(moddelegation.CapabilityPostsWrite, moddelegation.AuditPostCreate),
		"UpdateStorePost":    postMethod(moddelegation.CapabilityPostsWrite, moddelegation.AuditPostUpdate),
		"PublishStorePost":   postMethod(moddelegation.CapabilityPostsPublish, moddelegation.AuditPostPublish),
		"UnpublishStorePost": postMethod(moddelegation.CapabilityPostsPublish, moddelegation.AuditPostUnpublish),
		"ScheduleStorePost":  postMethod(moddelegation.CapabilityPostsPublish, moddelegation.AuditPostSchedule),
	}
	methods := DelegatedDeclaration.Service.Methods()
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		got := DelegatedDeclaration.Methods["/hawkr.social.v1alpha1.SocialDelegatedService/"+name]
		if got != want[name] {
			t.Errorf("%s：宣告 %+v，預期 %+v", name, got, want[name])
		}
	}
}

func TestStartupRejectsMissingOrExtraDeclarations(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	clone := func() httpdelegation.Declaration {
		d := DelegatedDeclaration
		d.Methods = maps.Clone(DelegatedDeclaration.Methods)
		return d
	}
	missing := clone()
	delete(missing.Methods, socialv1alpha1connect.SocialDelegatedServiceScheduleStorePostProcedure)
	if _, err := httpdelegation.NewInterceptor(&fakeSessions{}, logger, missing); err == nil {
		t.Error("少一列宣告應該不能啟動")
	}
	extra := clone()
	extra.Methods["/hawkr.social.v1alpha1.SocialDelegatedService/Gone"] = postMethod(moddelegation.CapabilityPostsRead, moddelegation.AuditPostRead)
	if _, err := httpdelegation.NewInterceptor(&fakeSessions{}, logger, extra); err == nil {
		t.Error("多一列宣告應該不能啟動")
	}
	bad := clone()
	bad.Methods[socialv1alpha1connect.SocialDelegatedServiceGetStorePostProcedure] = postMethod("posts.fly", moddelegation.AuditPostRead)
	if _, err := httpdelegation.NewInterceptor(&fakeSessions{}, logger, bad); err == nil {
		t.Error("不存在的能力應該不能啟動")
	}
	noAction := clone()
	noAction.Methods[socialv1alpha1connect.SocialDelegatedServiceGetStorePostProcedure] = moddelegation.DelegatedMethod{Capability: moddelegation.CapabilityPostsRead}
	if _, err := httpdelegation.NewInterceptor(&fakeSessions{}, logger, noAction); err == nil {
		t.Error("沒有稽核動作的宣告應該不能啟動")
	}
	if _, err := httpdelegation.NewInterceptor(nil, logger, DelegatedDeclaration); err == nil {
		t.Error("沒有 Sessions 應該不能啟動")
	}
	if _, err := httpdelegation.NewInterceptor(&fakeSessions{}, nil, DelegatedDeclaration); err == nil {
		t.Error("沒有 Logger 應該不能啟動")
	}
}

// 讀取方法標 NO_SIDE_EFFECTS（client 以 GET 呼叫），寫入方法不得標。
func TestOnlyReadsAreNoSideEffects(t *testing.T) {
	reads := []string{"ListStorePosts", "GetStorePost"}
	methods := DelegatedDeclaration.Service.Methods()
	seen := 0
	for i := range methods.Len() {
		m := methods.Get(i)
		opts, _ := m.Options().(*descriptorpb.MethodOptions)
		no := opts.GetIdempotencyLevel() == descriptorpb.MethodOptions_NO_SIDE_EFFECTS
		if slices.Contains(reads, string(m.Name())) {
			seen++
			if !no {
				t.Errorf("%s 是讀取，要標 NO_SIDE_EFFECTS", m.FullName())
			}
		} else if no {
			t.Errorf("%s 是寫入，不能標 NO_SIDE_EFFECTS", m.FullName())
		}
	}
	if seen != len(reads) {
		t.Fatalf("只找到 %d 個讀取方法", seen)
	}
}

// 每個寫入請求都帶 request_id（UUIDv4）與操作原因，缺少或不合法在到元件之前就被擋下，指向 operation 底下的欄位。
func TestWritesRequireAWellFormedOperation(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	id := uuid.New().String()
	bad := map[string]*pb.DelegatedOperation{
		"沒有 operation":  nil,
		"沒有 request_id": {OperationReason: "r"},
		"不是 UUIDv4":     {RequestId: uuid.New().String()[:14] + "1" + uuid.New().String()[15:], OperationReason: "r"},
		"沒有原因":          {RequestId: v4()},
		"原因只有空白":        {RequestId: v4(), OperationReason: "   "},
		"原因超過 100 字":    {RequestId: v4(), OperationReason: strings.Repeat("字", 101)},
		"原因含控制字元":       {RequestId: v4(), OperationReason: "a\x07b"},
		"原因含換行":         {RequestId: v4(), OperationReason: "a\nb"},
	}
	for name, o := range bad {
		_, err := e.client.PublishStorePost(t.Context(), withCredential(&pb.PublishStorePostRequest{Operation: o, PostId: id, Etag: "1"}, goodSession))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	_, err := e.client.PublishStorePost(t.Context(), withCredential(&pb.PublishStorePostRequest{Operation: bad["原因含控制字元"], PostId: id, Etag: "1"}, goodSession))
	if err == nil || !strings.Contains(err.Error(), "operation.operation_reason") {
		t.Errorf("控制字元的錯誤沒有指向 operation.operation_reason：%v", err)
	}
	if len(e.runner.calls) != 0 {
		t.Fatalf("不合法的 operation 不該到元件：%v", e.runner.calls)
	}
}

// request_id 與店家都由伺服器端確定：request_id 與請求內容雜湊原樣交給元件，店家只來自 session（請求沒有 store_id），原因去除首尾空白，通知預設為是。
func TestCreatePassesTheOperationAndTheSessionToTheComponent(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	o := delegatedOp()
	resp, err := e.client.CreateStorePost(t.Context(), withCredential(&pb.CreateStorePostRequest{Operation: o, Body: "新貼文", HashtagInputs: []string{"#a"}, ImageStagedToken: "tok"}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	w := e.runner.write
	if w.Op.RequestID.String() != o.GetRequestId() || w.Op.Reason != "店家請我們代發" || !w.Op.NotifySeller || w.PayloadHash == "" {
		t.Fatalf("write = %+v", w)
	}
	if e.posts.create.Body != "新貼文" || e.posts.create.ImageToken != "tok" || !slices.Equal(e.posts.create.Hashtags, []string{"#a"}) {
		t.Fatalf("create = %+v", e.posts.create)
	}
	if e.runner.caller.Session.ID != delegatedSessID || e.posts.store.StoreID != delegatedStore {
		t.Fatalf("caller = %+v、店家 %+v", e.runner.caller, e.posts.store)
	}
	if resp.Msg.GetAuditLogId() != e.runner.audit.String() || resp.Msg.GetPost().GetId() != e.posts.post.ID.String() || resp.Msg.GetPost().GetEtag() != "3" ||
		resp.Msg.GetPost().GetStatus() != pb.PostStatus_POST_STATUS_DRAFT {
		t.Fatalf("response = %v", resp.Msg)
	}
	off := false
	o2 := delegatedOp()
	o2.NotifySeller = &off
	if _, err := e.client.CreateStorePost(t.Context(), withCredential(&pb.CreateStorePostRequest{Operation: o2}, goodSession)); err != nil {
		t.Fatal(err)
	}
	if e.runner.write.Op.NotifySeller {
		t.Fatal("notify_seller=false 要傳下去")
	}
	// 內容雜湊綁定內容：不同的內容有不同的雜湊（同一個 request_id 才不會被當成重送）。
	first := e.runner.write.PayloadHash
	if _, err := e.client.CreateStorePost(t.Context(), withCredential(&pb.CreateStorePostRequest{Operation: o2, Body: "另一篇"}, goodSession)); err != nil {
		t.Fatal(err)
	}
	if e.runner.write.PayloadHash == first {
		t.Fatal("不同內容的雜湊相同")
	}
}

func TestUpdateMergesOnlyTheMaskedFields(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	id := uuid.New()
	_, err := e.client.UpdateStorePost(t.Context(), withCredential(&pb.UpdateStorePostRequest{
		Operation: delegatedOp(), Post: &pb.Post{Id: id.String(), Etag: "7", Body: "新內文", HashtagInputs: []string{"x"}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}},
	}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	u := e.posts.update
	if u.PostID != id || u.Version != 7 || u.Body == nil || *u.Body != "新內文" || u.Hashtags != nil || u.SetProduct || u.ClearImage || u.ImageToken != "" {
		t.Fatalf("update = %+v", u)
	}
	if e.runner.write.ResourceID != id.String() || e.runner.write.Version == nil || *e.runner.write.Version != 7 {
		t.Fatalf("稽核的資源與版本 = %+v", e.runner.write)
	}
	for name, req := range map[string]*pb.UpdateStorePostRequest{
		"沒有 mask": {Operation: delegatedOp(), Post: &pb.Post{Id: id.String(), Etag: "1"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"etag"}}},
		"不能改的欄位":  {Operation: delegatedOp(), Post: &pb.Post{Id: id.String(), Etag: "1"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"like_count"}}},
		"沒有 etag": {Operation: delegatedOp(), Post: &pb.Post{Id: id.String()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}}},
	} {
		e.runner.calls = nil
		if _, err := e.client.UpdateStorePost(t.Context(), withCredential(req, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument || len(e.runner.calls) != 0 {
			t.Errorf("%s：%v，呼叫 %v", name, err, e.runner.calls)
		}
	}
}

func TestSchedulePassesTheTimeAndReturnsTheScheduledTime(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	at := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	e.posts.publish = at
	id := uuid.New()
	resp, err := e.client.ScheduleStorePost(t.Context(), withCredential(&pb.ScheduleStorePostRequest{Operation: delegatedOp(), PostId: id.String(), Etag: "2", PublishTime: timestamppb.New(at)}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	s := e.posts.sched
	if s.PostID != id || s.Version != 2 || !s.PublishAt.Equal(at) || !resp.Msg.GetPublishTime().AsTime().Equal(at) {
		t.Fatalf("sched = %+v、回應 %v", s, resp.Msg)
	}
	if s.By.Kind != social.ScheduleByDelegation || s.By.Ref != "9" {
		t.Fatalf("排程的發起者 = %+v，預期 DELEGATION 與 session ID", s.By)
	}
	if _, err := e.client.ScheduleStorePost(t.Context(), withCredential(&pb.ScheduleStorePostRequest{Operation: delegatedOp(), PostId: id.String(), Etag: "2"}, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("沒有 publish_time：%v", err)
	}
}

// 重送（元件沒有呼叫寫入）：handler 以第一次寫入的資源 ID 讀回貼文目前的樣子，排程的重送也回排定的時間；social 不被寫第二次。
func TestReplayReadsBackTheFirstResource(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	e.runner.replayed, e.runner.replayID = true, e.posts.post.ID.String()
	at := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	e.posts.pending = &social.Schedule{PublishAt: at}
	resp, err := e.client.CreateStorePost(t.Context(), withCredential(&pb.CreateStorePostRequest{Operation: delegatedOp(), Body: "x"}, goodSession))
	if err != nil || resp.Msg.GetPost().GetId() != e.posts.post.ID.String() || resp.Msg.GetAuditLogId() != e.runner.audit.String() {
		t.Fatalf("重送 = %v %v", resp, err)
	}
	if e.posts.create.Body != "" {
		t.Fatal("重送不得再呼叫 social 的寫入")
	}
	sched, err := e.client.ScheduleStorePost(t.Context(), withCredential(&pb.ScheduleStorePostRequest{Operation: delegatedOp(), PostId: e.posts.post.ID.String(), Etag: "3", PublishTime: timestamppb.New(at)}, goodSession))
	if err != nil || !sched.Msg.GetPublishTime().AsTime().Equal(at) {
		t.Fatalf("排程的重送 = %v %v", sched, err)
	}
}

func TestListPassesStatusAndCursorAndPages(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	last := social.Post{ID: uuid.New(), StoreID: delegatedStore, Version: 1, Status: social.StatusPublished, CreatedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
	e.posts.page = social.OwnPostPage{Posts: []social.Post{e.posts.post, last}, Next: &social.OwnPostCursor{CreatedAt: last.CreatedAt, ID: last.ID}}
	resp, err := e.client.ListStorePosts(t.Context(), withCredential(&pb.ListStorePostsRequest{Status: pb.PostStatus_POST_STATUS_DRAFT}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	if e.posts.status != social.StatusDraft || e.posts.after != nil || len(resp.Msg.GetPosts()) != 2 || resp.Msg.GetNextPageToken() == "" {
		t.Fatalf("status %q after %v 回應 %v", e.posts.status, e.posts.after, resp.Msg)
	}
	e.posts.page.Next = nil
	if _, err := e.client.ListStorePosts(t.Context(), withCredential(&pb.ListStorePostsRequest{Status: pb.PostStatus_POST_STATUS_DRAFT, PageToken: resp.Msg.GetNextPageToken()}, goodSession)); err != nil {
		t.Fatal(err)
	}
	if e.posts.after == nil || e.posts.after.ID != last.ID {
		t.Fatalf("游標 = %v", e.posts.after)
	}
	// page_token 綁定 status：換一個 status 用同一個 token 被擋下。
	if _, err := e.client.ListStorePosts(t.Context(), withCredential(&pb.ListStorePostsRequest{PageToken: resp.Msg.GetNextPageToken()}, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換 status 的 token：%v", err)
	}
	if _, err := e.client.ListStorePosts(t.Context(), withCredential(&pb.ListStorePostsRequest{Status: pb.PostStatus(99)}, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("未知的 status：%v", err)
	}
}

// 元件與 social 回的錯誤帶著各自的分類到 client。
func TestComponentAndSocialErrorsReachTheClientWithTheirKind(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	e.runner.err = apperr.FailedPrecondition("request_id 已用於不同的請求內容")
	_, err := e.client.PublishStorePost(t.Context(), withCredential(&pb.PublishStorePostRequest{Operation: delegatedOp(), PostId: uuid.New().String(), Etag: "1"}, goodSession))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("request_id 用在不同內容：%v，預期 failed_precondition", err)
	}
	if len(e.sessions.touched) != 0 {
		t.Fatalf("元件內被拒絕的請求不該算活動：%v", e.sessions.touched)
	}
	e.runner.err = nil
	e.posts.err = apperr.NotFound("貼文不存在")
	_, err = e.client.GetStorePost(t.Context(), withCredential(&pb.GetStorePostRequest{PostId: uuid.New().String()}, goodSession))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("social 的 not_found：%v", err)
	}
}

// 委派建立的 request_id 在 operation 之內：模組回報 RequestID 欄位錯誤時，指向 operation.request_id，不是頂層的 request_id。
func TestDelegatedCreateMapsRequestIDToTheOperation(t *testing.T) {
	e := newDelegatedEnv(t, goodSessions())
	e.posts.err = apperr.InvalidInputOn([]string{"RequestID"}, "%s", "reused")
	_, err := e.client.CreateStorePost(t.Context(), withCredential(&pb.CreateStorePostRequest{Operation: delegatedOp(), Body: "x"}, goodSession))
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	var fields []string
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if br, ok := v.(*errdetails.BadRequest); ok {
				for _, fv := range br.GetFieldViolations() {
					fields = append(fields, fv.GetField())
				}
			}
		}
	}
	if !slices.Equal(fields, []string{"operation.request_id"}) {
		t.Fatalf("欄位 = %v，預期 operation.request_id", fields)
	}
}
