package delegation

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/delegation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/delegation/v1alpha1/delegationv1alpha1connect"
	socialpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpsocial "github.com/Will413028/hawkr/backend/internal/httpapi/social"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	goodSession = "good-session"
	storeID     = int64(5)
)

// fakeSessions 以憑證回 session；沒有的憑證回模組的「憑證無效」拒絕。
type fakeSessions struct {
	sessions map[string]moddelegation.DelegationSession
	resolved []string
	activity []bool
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

func session(status moddelegation.DelegationStatus, caps ...moddelegation.Capability) moddelegation.DelegationSession {
	return moddelegation.DelegationSession{ID: 9, Status: status, Capabilities: caps, Target: moddelegation.DelegationTarget{StoreID: storeID}}
}

var allCaps = []moddelegation.Capability{moddelegation.CapabilityPostsRead, moddelegation.CapabilityPostsWrite, moddelegation.CapabilityPostsPublish}

// fakePosts 記錄 use case 的呼叫與收到的輸入，回固定的結果；它不檢查能力與冪等（那是模組的規則，見模組的 integration 測試）。
type fakePosts struct {
	calls   []string
	caller  moddelegation.DelegatedCaller
	create  moddelegation.DelegatedCreatePostInput
	update  moddelegation.DelegatedUpdatePostInput
	trans   moddelegation.DelegatedPostTransitionInput
	sched   moddelegation.DelegatedSchedulePostInput
	status  social.PostStatus
	after   *social.OwnPostCursor
	readID  uuid.UUID
	audit   uuid.UUID
	post    social.Post
	page    social.OwnPostPage
	err     error
	publish *time.Time
}

func (f *fakePosts) rec(op string, c moddelegation.DelegatedCaller) error {
	f.calls = append(f.calls, op)
	f.caller = c
	return f.err
}

func (f *fakePosts) result() moddelegation.DelegatedPostResult {
	return moddelegation.DelegatedPostResult{Post: f.post, Store: seller.Store{StoreID: storeID}, AuditLogID: f.audit, PublishAt: f.publish}
}

func (f *fakePosts) DelegatedCreatePost(_ context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedCreatePostInput) (moddelegation.DelegatedPostResult, error) {
	f.create = in
	return f.result(), f.rec("create", c)
}

func (f *fakePosts) DelegatedUpdatePost(_ context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedUpdatePostInput) (moddelegation.DelegatedPostResult, error) {
	f.update = in
	return f.result(), f.rec("update", c)
}

func (f *fakePosts) DelegatedPublishPost(_ context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedPostTransitionInput) (moddelegation.DelegatedPostResult, error) {
	f.trans = in
	return f.result(), f.rec("publish", c)
}

func (f *fakePosts) DelegatedUnpublishPost(_ context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedPostTransitionInput) (moddelegation.DelegatedPostResult, error) {
	f.trans = in
	return f.result(), f.rec("unpublish", c)
}

func (f *fakePosts) DelegatedSchedulePost(_ context.Context, c moddelegation.DelegatedCaller, in moddelegation.DelegatedSchedulePostInput) (moddelegation.DelegatedPostResult, error) {
	f.sched = in
	return f.result(), f.rec("schedule", c)
}

func (f *fakePosts) DelegatedPost(_ context.Context, c moddelegation.DelegatedCaller, id uuid.UUID) (moddelegation.DelegatedPostResult, error) {
	f.readID = id
	return f.result(), f.rec("get", c)
}

func (f *fakePosts) DelegatedPosts(_ context.Context, c moddelegation.DelegatedCaller, status social.PostStatus, after *social.OwnPostCursor, _ int32) (moddelegation.DelegatedPostPageResult, error) {
	f.status, f.after = status, after
	return moddelegation.DelegatedPostPageResult{Page: f.page, Store: seller.Store{StoreID: storeID}}, f.rec("list", c)
}

type noBuyers struct{}

func (noBuyers) CurrentBuyerID(context.Context, int64) (int64, error)           { return 0, nil }
func (noBuyers) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) { return nil, nil }

type cards struct{}

func (cards) CurrentStore(context.Context, int64) (seller.Store, error) { return seller.Store{}, nil }
func (cards) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	name := "shop"
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		out[i] = seller.StoreCard{StoreID: id, StoreCode: id, Name: &name}
	}
	return out, nil
}
func (cards) StoresBlockingBuyer(context.Context, int64, []int64) (map[int64]bool, error) {
	return nil, nil
}

type env struct {
	sessions *fakeSessions
	posts    *fakePosts
	client   delegationv1alpha1connect.DelegationDelegatedServiceClient
	url      string
}

func newEnv(t *testing.T, sessions map[string]moddelegation.DelegationSession) *env {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	own, err := httpsocial.NewOwnPosts(noBuyers{}, cards{}, images, web)
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSessions{sessions: sessions}
	f := &fakePosts{audit: uuid.New(), post: social.Post{ID: uuid.New(), StoreID: storeID, Body: "內文", Version: 3, Status: social.StatusDraft, CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}
	in, err := NewInterceptor(s)
	if err != nil {
		t.Fatal(err)
	}
	path, handler := delegationv1alpha1connect.NewDelegationDelegatedServiceHandler(New(f, own),
		connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler)), in))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &env{sessions: s, posts: f, url: srv.URL, client: delegationv1alpha1connect.NewDelegationDelegatedServiceClient(srv.Client(), srv.URL)}
}

func withCredential[T any](msg *T, c string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if c != "" {
		req.Header().Set(OperationSessionHeader, c)
	}
	return req
}

func v4() string {
	id := uuid.New().String()
	return id[:14] + "4" + id[15:19] + "8" + id[20:]
}

func op() *pb.DelegatedOperation {
	return &pb.DelegatedOperation{RequestId: v4(), OperationReason: "  店家請我們代發  "}
}

func good() map[string]moddelegation.DelegationSession {
	return map[string]moddelegation.DelegationSession{goodSession: session(moddelegation.DelegationActive, allCaps...)}
}

// 每個方法的一個合法請求，用來驗證「進得去 handler」與「被攔下」。
func calls(t *testing.T, e *env, cred string) map[string]func() error {
	t.Helper()
	id := uuid.New().String()
	ctx := t.Context()
	future := timestamppb.New(time.Now().Add(24 * time.Hour))
	return map[string]func() error{
		delegationv1alpha1connect.DelegationDelegatedServiceListPostsProcedure: func() error {
			_, err := e.client.ListPosts(ctx, withCredential(&pb.ListPostsRequest{}, cred))
			return err
		},
		delegationv1alpha1connect.DelegationDelegatedServiceGetPostProcedure: func() error {
			_, err := e.client.GetPost(ctx, withCredential(&pb.GetPostRequest{PostId: id}, cred))
			return err
		},
		delegationv1alpha1connect.DelegationDelegatedServiceCreatePostProcedure: func() error {
			_, err := e.client.CreatePost(ctx, withCredential(&pb.CreatePostRequest{Operation: op(), Body: "x"}, cred))
			return err
		},
		delegationv1alpha1connect.DelegationDelegatedServiceUpdatePostProcedure: func() error {
			_, err := e.client.UpdatePost(ctx, withCredential(&pb.UpdatePostRequest{Operation: op(), Post: &socialpb.Post{Id: id, Etag: "1", Body: "x"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}}}, cred))
			return err
		},
		delegationv1alpha1connect.DelegationDelegatedServicePublishPostProcedure: func() error {
			_, err := e.client.PublishPost(ctx, withCredential(&pb.PublishPostRequest{Operation: op(), PostId: id, Etag: "1"}, cred))
			return err
		},
		delegationv1alpha1connect.DelegationDelegatedServiceUnpublishPostProcedure: func() error {
			_, err := e.client.UnpublishPost(ctx, withCredential(&pb.UnpublishPostRequest{Operation: op(), PostId: id, Etag: "1"}, cred))
			return err
		},
		delegationv1alpha1connect.DelegationDelegatedServiceSchedulePostProcedure: func() error {
			_, err := e.client.SchedulePost(ctx, withCredential(&pb.SchedulePostRequest{Operation: op(), PostId: id, Etag: "1", PublishTime: future}, cred))
			return err
		},
	}
}

func TestEveryMethodReachesTheUseCaseWithAGoodSession(t *testing.T) {
	e := newEnv(t, good())
	all := calls(t, e, goodSession)
	if len(all) != len(declarations) {
		t.Fatalf("測試涵蓋 %d 個方法，宣告 %d 個", len(all), len(declarations))
	}
	for proc, do := range all {
		before := len(e.posts.calls)
		if err := do(); err != nil {
			t.Errorf("%s：%v", proc, err)
		}
		if len(e.posts.calls) != before+1 {
			t.Errorf("%s 沒有到 use case", proc)
		}
	}
	for _, a := range e.sessions.activity {
		if !a {
			t.Fatal("代行資源的路由要算活動（順延閒置逾時）")
		}
	}
}

func TestMissingOrInvalidCredentialIsUnauthenticated(t *testing.T) {
	for name, cred := range map[string]string{"沒有標頭": "", "不認得的憑證": "nope"} {
		e := newEnv(t, good())
		for proc, do := range calls(t, e, cred) {
			if err := do(); connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Errorf("%s %s：%v，預期 unauthenticated", name, proc, err)
			}
		}
		if len(e.posts.calls) != 0 {
			t.Fatalf("%s：不該到 use case：%v", name, e.posts.calls)
		}
	}
}

func TestEndedSessionsAreRejected(t *testing.T) {
	for _, status := range []moddelegation.DelegationStatus{moddelegation.DelegationRevoked, moddelegation.DelegationExpired} {
		e := newEnv(t, map[string]moddelegation.DelegationSession{goodSession: session(status, allCaps...)})
		for proc, do := range calls(t, e, goodSession) {
			if err := do(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Errorf("%s 的 session（%s）：%v，預期 failed_precondition", proc, status, err)
			}
		}
		if len(e.posts.calls) != 0 {
			t.Fatalf("%s 的 session 不該到 use case：%v", status, e.posts.calls)
		}
	}
}

// 缺能力被擋下：每個方法依宣告表的能力，拿掉那一個能力（其他兩個都在）就被 permission_denied 擋下。
func TestMissingCapabilityIsRejected(t *testing.T) {
	for proc, required := range declarations {
		var caps []moddelegation.Capability
		for _, c := range allCaps {
			if c != required {
				caps = append(caps, c)
			}
		}
		e := newEnv(t, map[string]moddelegation.DelegationSession{goodSession: session(moddelegation.DelegationActive, caps...)})
		err := calls(t, e, goodSession)[proc]()
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s 缺 %s：%v，預期 permission_denied", proc, required, err)
		}
		if len(e.posts.calls) != 0 {
			t.Errorf("%s 缺 %s 不該到 use case：%v", proc, required, e.posts.calls)
		}
	}
}

func TestMissingPostsWriteIsRejectedForCreate(t *testing.T) {
	e := newEnv(t, map[string]moddelegation.DelegationSession{goodSession: session(moddelegation.DelegationActive, moddelegation.CapabilityPostsRead, moddelegation.CapabilityPostsPublish)})
	_, err := e.client.CreatePost(t.Context(), withCredential(&pb.CreatePostRequest{Operation: op(), Body: "x"}, goodSession))
	if connect.CodeOf(err) != connect.CodePermissionDenied || len(e.posts.calls) != 0 {
		t.Fatalf("缺 posts.write：%v，呼叫 %v", err, e.posts.calls)
	}
}

func TestCapabilitiesAreDeclaredPerMethod(t *testing.T) {
	want := map[string]moddelegation.Capability{
		"ListPosts": moddelegation.CapabilityPostsRead, "GetPost": moddelegation.CapabilityPostsRead,
		"CreatePost": moddelegation.CapabilityPostsWrite, "UpdatePost": moddelegation.CapabilityPostsWrite,
		"PublishPost": moddelegation.CapabilityPostsPublish, "UnpublishPost": moddelegation.CapabilityPostsPublish, "SchedulePost": moddelegation.CapabilityPostsPublish,
	}
	methods := serviceDescriptor().Methods()
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		got := declarations["/hawkr.delegation.v1alpha1.DelegationDelegatedService/"+name]
		if got != want[name] {
			t.Errorf("%s：宣告 %q，預期 %q", name, got, want[name])
		}
	}
}

func TestStartupRejectsMissingOrExtraDeclarations(t *testing.T) {
	missing := map[string]moddelegation.Capability{}
	for k, v := range declarations {
		missing[k] = v
	}
	delete(missing, delegationv1alpha1connect.DelegationDelegatedServiceSchedulePostProcedure)
	if _, err := newInterceptor(missing, &fakeSessions{}); err == nil {
		t.Error("少一列宣告應該不能啟動")
	}
	extra := map[string]moddelegation.Capability{}
	for k, v := range declarations {
		extra[k] = v
	}
	extra["/hawkr.delegation.v1alpha1.DelegationDelegatedService/Gone"] = moddelegation.CapabilityPostsRead
	if _, err := newInterceptor(extra, &fakeSessions{}); err == nil {
		t.Error("多一列宣告應該不能啟動")
	}
	bad := map[string]moddelegation.Capability{}
	for k, v := range declarations {
		bad[k] = v
	}
	bad[delegationv1alpha1connect.DelegationDelegatedServiceGetPostProcedure] = "posts.fly"
	if _, err := newInterceptor(bad, &fakeSessions{}); err == nil {
		t.Error("不存在的能力應該不能啟動")
	}
	if _, err := NewInterceptor(nil); err == nil {
		t.Error("沒有 Sessions 應該不能啟動")
	}
}

// 讀取方法標 NO_SIDE_EFFECTS（client 以 GET 呼叫），寫入方法不得標。
func TestOnlyReadsAreNoSideEffects(t *testing.T) {
	reads := []string{"ListPosts", "GetPost"}
	methods := serviceDescriptor().Methods()
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

// 每個寫入請求都帶 request_id（UUIDv4）與操作原因，缺少或不合法在到 use case 之前就被擋下，指向 operation 底下的欄位。
func TestWritesRequireAWellFormedOperation(t *testing.T) {
	e := newEnv(t, good())
	id := uuid.New().String()
	bad := map[string]*pb.DelegatedOperation{
		"沒有 operation":  nil,
		"沒有 request_id": {OperationReason: "r"},
		"不是 UUIDv4":     {RequestId: uuid.New().String()[:14] + "1" + uuid.New().String()[15:], OperationReason: "r"},
		"沒有原因":          {RequestId: v4()},
		"原因只有空白":        {RequestId: v4(), OperationReason: "   "},
		"原因超過 100 字":    {RequestId: v4(), OperationReason: string(make([]rune, 101))},
	}
	bad["原因超過 100 字"].OperationReason = ""
	for i := 0; i < 101; i++ {
		bad["原因超過 100 字"].OperationReason += "字"
	}
	for name, o := range bad {
		_, err := e.client.PublishPost(t.Context(), withCredential(&pb.PublishPostRequest{Operation: o, PostId: id, Etag: "1"}, goodSession))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if len(e.posts.calls) != 0 {
		t.Fatalf("不合法的 operation 不該到 use case：%v", e.posts.calls)
	}
}

// request_id 與店家都由伺服器端確定：request_id 原樣傳給 use case，店家只來自 session（請求沒有 store_id），原因去除首尾空白，通知預設為是。
func TestCreatePassesTheRequestIDAndTheSessionToTheUseCase(t *testing.T) {
	e := newEnv(t, good())
	o := op()
	resp, err := e.client.CreatePost(t.Context(), withCredential(&pb.CreatePostRequest{Operation: o, Body: "新貼文", HashtagInputs: []string{"#a"}, ImageStagedToken: "tok"}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	if e.posts.create.Op.RequestID.String() != o.GetRequestId() || e.posts.create.Op.Reason != "店家請我們代發" || !e.posts.create.Op.NotifySeller {
		t.Fatalf("Op = %+v", e.posts.create.Op)
	}
	if e.posts.create.Body != "新貼文" || e.posts.create.ImageToken != "tok" || !slices.Equal(e.posts.create.Hashtags, []string{"#a"}) {
		t.Fatalf("create = %+v", e.posts.create)
	}
	if e.posts.caller.Session.ID != 9 || e.posts.caller.Session.Target.StoreID != storeID {
		t.Fatalf("caller = %+v", e.posts.caller)
	}
	if resp.Msg.GetAuditLogId() != e.posts.audit.String() || resp.Msg.GetPost().GetId() != e.posts.post.ID.String() || resp.Msg.GetPost().GetEtag() != "3" ||
		resp.Msg.GetPost().GetStatus() != socialpb.PostStatus_POST_STATUS_DRAFT {
		t.Fatalf("response = %v", resp.Msg)
	}
	off := false
	o2 := op()
	o2.NotifySeller = &off
	if _, err := e.client.CreatePost(t.Context(), withCredential(&pb.CreatePostRequest{Operation: o2}, goodSession)); err != nil {
		t.Fatal(err)
	}
	if e.posts.create.Op.NotifySeller {
		t.Fatal("notify_seller=false 要傳下去")
	}
}

func TestUpdateMergesOnlyTheMaskedFields(t *testing.T) {
	e := newEnv(t, good())
	id := uuid.New()
	_, err := e.client.UpdatePost(t.Context(), withCredential(&pb.UpdatePostRequest{
		Operation: op(), Post: &socialpb.Post{Id: id.String(), Etag: "7", Body: "新內文", HashtagInputs: []string{"x"}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}},
	}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	u := e.posts.update.Update
	if u.PostID != id || u.Version != 7 || u.Body == nil || *u.Body != "新內文" || u.Hashtags != nil || u.SetProduct || u.ClearImage || u.ImageToken != "" {
		t.Fatalf("update = %+v", u)
	}
	for name, req := range map[string]*pb.UpdatePostRequest{
		"沒有 mask": {Operation: op(), Post: &socialpb.Post{Id: id.String(), Etag: "1"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"etag"}}},
		"不能改的欄位":  {Operation: op(), Post: &socialpb.Post{Id: id.String(), Etag: "1"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"like_count"}}},
		"沒有 etag": {Operation: op(), Post: &socialpb.Post{Id: id.String()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}}},
	} {
		e.posts.calls = nil
		if _, err := e.client.UpdatePost(t.Context(), withCredential(req, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument || len(e.posts.calls) != 0 {
			t.Errorf("%s：%v，呼叫 %v", name, err, e.posts.calls)
		}
	}
}

func TestSchedulePassesTheTimeAndReturnsTheScheduledTime(t *testing.T) {
	e := newEnv(t, good())
	at := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	e.posts.publish = &at
	id := uuid.New()
	resp, err := e.client.SchedulePost(t.Context(), withCredential(&pb.SchedulePostRequest{Operation: op(), PostId: id.String(), Etag: "2", PublishTime: timestamppb.New(at)}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	if e.posts.sched.PostID != id || e.posts.sched.Version != 2 || !e.posts.sched.PublishAt.Equal(at) || !resp.Msg.GetPublishTime().AsTime().Equal(at) {
		t.Fatalf("sched = %+v、回應 %v", e.posts.sched, resp.Msg)
	}
	if _, err := e.client.SchedulePost(t.Context(), withCredential(&pb.SchedulePostRequest{Operation: op(), PostId: id.String(), Etag: "2"}, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("沒有 publish_time：%v", err)
	}
}

func TestListPassesStatusAndCursorAndPages(t *testing.T) {
	e := newEnv(t, good())
	last := social.Post{ID: uuid.New(), StoreID: storeID, Version: 1, Status: social.StatusPublished, CreatedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
	e.posts.page = social.OwnPostPage{Posts: []social.Post{e.posts.post, last}, Next: &social.OwnPostCursor{CreatedAt: last.CreatedAt, ID: last.ID}}
	resp, err := e.client.ListPosts(t.Context(), withCredential(&pb.ListPostsRequest{Status: socialpb.PostStatus_POST_STATUS_DRAFT}, goodSession))
	if err != nil {
		t.Fatal(err)
	}
	if e.posts.status != social.StatusDraft || e.posts.after != nil || len(resp.Msg.GetPosts()) != 2 || resp.Msg.GetNextPageToken() == "" {
		t.Fatalf("status %q after %v 回應 %v", e.posts.status, e.posts.after, resp.Msg)
	}
	e.posts.page.Next = nil
	if _, err := e.client.ListPosts(t.Context(), withCredential(&pb.ListPostsRequest{Status: socialpb.PostStatus_POST_STATUS_DRAFT, PageToken: resp.Msg.GetNextPageToken()}, goodSession)); err != nil {
		t.Fatal(err)
	}
	if e.posts.after == nil || e.posts.after.ID != last.ID {
		t.Fatalf("游標 = %v", e.posts.after)
	}
	// page_token 綁定 status：換一個 status 用同一個 token 被擋下。
	if _, err := e.client.ListPosts(t.Context(), withCredential(&pb.ListPostsRequest{PageToken: resp.Msg.GetNextPageToken()}, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換 status 的 token：%v", err)
	}
	if _, err := e.client.ListPosts(t.Context(), withCredential(&pb.ListPostsRequest{Status: socialpb.PostStatus(99)}, goodSession)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("未知的 status：%v", err)
	}
}

func TestUseCaseDenialsReachTheClientWithTheirKind(t *testing.T) {
	e := newEnv(t, good())
	e.posts.err = apperr.FailedPrecondition("request_id 已用於不同的請求內容")
	_, err := e.client.PublishPost(t.Context(), withCredential(&pb.PublishPostRequest{Operation: op(), PostId: uuid.New().String(), Etag: "1"}, goodSession))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("request_id 用在不同內容：%v，預期 failed_precondition", err)
	}
}
