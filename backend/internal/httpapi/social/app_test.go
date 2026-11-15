package social

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type writeCall struct {
	op     string
	caller social.Caller
	buyer  int64
	id     uuid.UUID
	body   string
	ver    int64
	delVer int64
	cin    social.CommentInput
	fin    social.ForumInput
}

type fakeWrites struct {
	post     social.Post
	calls    []writeCall
	err      error
	sanction social.SanctionPage
	hidden   social.HiddenPage
	gotSize  int32
	gotBuyer int64
	gotSAft  *social.SanctionCursor
	gotHAft  *social.HiddenCursor
}

func (f *fakeWrites) MySanctions(_ context.Context, b int64, after *social.SanctionCursor, size int32) (social.SanctionPage, error) {
	f.gotBuyer, f.gotSize, f.gotSAft = b, size, after
	return f.sanction, f.err
}

func (f *fakeWrites) MyHiddenContent(_ context.Context, b int64, after *social.HiddenCursor, size int32) (social.HiddenPage, error) {
	f.gotBuyer, f.gotSize, f.gotHAft = b, size, after
	return f.hidden, f.err
}

func (f *fakeWrites) rec(c writeCall) error { f.calls = append(f.calls, c); return f.err }

func (f *fakeWrites) RecordShare(_ context.Context, b int64, id uuid.UUID) (bool, error) {
	return true, f.rec(writeCall{op: "share", buyer: b, id: id})
}

func (f *fakeWrites) LikePost(_ context.Context, b int64, id uuid.UUID) (bool, error) {
	return true, f.rec(writeCall{op: "likePost", buyer: b, id: id})
}

func (f *fakeWrites) UnlikePost(_ context.Context, b int64, id uuid.UUID) (bool, error) {
	return true, f.rec(writeCall{op: "unlikePost", buyer: b, id: id})
}

func (f *fakeWrites) LikeComment(_ context.Context, c social.Caller, id uuid.UUID) (bool, error) {
	return true, f.rec(writeCall{op: "likeComment", caller: c, id: id})
}

func (f *fakeWrites) UnlikeComment(_ context.Context, c social.Caller, id uuid.UUID) (bool, error) {
	return true, f.rec(writeCall{op: "unlikeComment", caller: c, id: id})
}

func (f *fakeWrites) CreateComment(_ context.Context, c social.Caller, in social.CommentInput) (social.Comment, error) {
	if err := f.rec(writeCall{op: "createComment", caller: c, cin: in}); err != nil {
		return social.Comment{}, err
	}
	at := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	return social.Comment{
		ID: uuid.New(), PostID: in.PostID, ParentID: in.ParentID, ReplyToID: in.ReplyToID, State: social.CommentVisible,
		Actor: social.Actor{Kind: social.ActorBuyer, ID: c.BuyerID}, Body: in.Body, CreatedAt: at, UpdatedAt: at, Version: 1, StoreID: f.post.StoreID,
	}, nil
}

func (f *fakeWrites) UpdateComment(_ context.Context, c social.Caller, id uuid.UUID, version int64, body string) (social.Comment, error) {
	err := f.rec(writeCall{op: "updateComment", caller: c, id: id, ver: version, body: body})
	at := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	return social.Comment{
		ID: id, PostID: f.post.ID, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: c.BuyerID},
		Body: body, CreatedAt: at, UpdatedAt: at, Version: version + 1, Likes: 4, ReplyCount: 2, StoreID: f.post.StoreID,
	}, err
}

func (f *fakeWrites) DeleteComment(_ context.Context, c social.Caller, id uuid.UUID, version int64) error {
	return f.rec(writeCall{op: "deleteComment", caller: c, id: id, delVer: version})
}

func (f *fakeWrites) CreateForumEntry(_ context.Context, c social.Caller, in social.ForumInput) (social.ForumEntry, error) {
	if err := f.rec(writeCall{op: "createForum", caller: c, fin: in}); err != nil {
		return social.ForumEntry{}, err
	}
	at := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	return social.ForumEntry{
		ID: uuid.New(), StoreID: in.StoreID, ParentID: in.ParentID, State: social.CommentVisible,
		Actor: social.Actor{Kind: social.ActorBuyer, ID: c.BuyerID}, Body: in.Body, CreatedAt: at, UpdatedAt: at, Version: 1,
	}, nil
}

func (f *fakeWrites) UpdateForumEntry(_ context.Context, c social.Caller, id uuid.UUID, version int64, body string) (social.ForumEntry, error) {
	err := f.rec(writeCall{op: "updateForum", caller: c, id: id, ver: version, body: body})
	at := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	return social.ForumEntry{
		ID: id, StoreID: 5, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: c.BuyerID},
		Body: body, CreatedAt: at, UpdatedAt: at, Version: version + 1, ReplyCount: 3,
	}, err
}

func (f *fakeWrites) DeleteForumEntry(_ context.Context, c social.Caller, id uuid.UUID, version int64) error {
	return f.rec(writeCall{op: "deleteForum", caller: c, id: id, delVer: version})
}

type appEnv struct {
	w      *fakeWrites
	stores *fakeStores
	client socialv1alpha1connect.SocialAppServiceClient
	url    string
}

// newAppEnv 掛上真的 Connect handler 與 rpc 的 interceptor（授權、錯誤轉換）；header X-Test-Account 非空時以該帳號登入。
func newAppEnv(t *testing.T, w *fakeWrites, s *fakeStores) *appEnv {
	t.Helper()
	return newAppEnvWith(t, w, &fakeOwner{}, s)
}

// newAppEnvWith 是 newAppEnv 加上店主方法的來源。
func newAppEnvWith(t *testing.T, w *fakeWrites, owner Owner, s *fakeStores) *appEnv {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewApp(w, w, owner, fakeBuyers{id: 7}, s, images, web)
	if err != nil {
		t.Fatal(err)
	}
	path, handler := socialv1alpha1connect.NewSocialAppServiceHandler(h, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Account") != "" {
			r = r.WithContext(asBuyer(r.Context()))
		}
		handler.ServeHTTP(rw, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &appEnv{w: w, stores: s, url: srv.URL, client: socialv1alpha1connect.NewSocialAppServiceClient(srv.Client(), srv.URL)}
}

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func signedIn[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Account", "1")
	return req
}

func TestSocialAppRequiresALogin(t *testing.T) {
	e := newAppEnv(t, &fakeWrites{}, &fakeStores{})
	methods := pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialAppService").Methods()
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, e.url+"/hawkr.social.v1alpha1.SocialAppService/"+name, strings.NewReader("{}"))
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
	if len(e.w.calls) != 0 {
		t.Fatalf("未登入不該到模組：%v", e.w.calls)
	}
}

func TestSocialAppSuccessPaths(t *testing.T) {
	p := post(5)
	id := uuid.New()
	parent := uuid.New()
	rid := uuid.New().String()
	rid = rid[:14] + "4" + rid[15:]
	w := &fakeWrites{post: p}
	e := newAppEnv(t, w, &fakeStores{})
	ctx := t.Context()

	if _, err := e.client.SharePost(ctx, signedIn(&pb.SharePostRequest{PostId: p.ID.String()})); err != nil {
		t.Fatal("SharePost", err)
	}
	if _, err := e.client.LikePost(ctx, signedIn(&pb.LikePostRequest{PostId: p.ID.String()})); err != nil {
		t.Fatal("LikePost", err)
	}
	if _, err := e.client.UnlikePost(ctx, signedIn(&pb.UnlikePostRequest{PostId: p.ID.String()})); err != nil {
		t.Fatal("UnlikePost", err)
	}
	if _, err := e.client.LikeComment(ctx, signedIn(&pb.LikeCommentRequest{CommentId: id.String()})); err != nil {
		t.Fatal("LikeComment", err)
	}
	if _, err := e.client.UnlikeComment(ctx, signedIn(&pb.UnlikeCommentRequest{CommentId: id.String()})); err != nil {
		t.Fatal("UnlikeComment", err)
	}
	pid := parent.String()
	cc, err := e.client.CreateComment(ctx, signedIn(&pb.CreateCommentRequest{PostId: p.ID.String(), Body: "好吃", ParentId: &pid, RequestId: rid}))
	if err != nil {
		t.Fatal("CreateComment", err)
	}
	if got := cc.Msg.GetComment(); got.GetBody() != "好吃" || !got.GetIsMine() || !got.GetViewerCanEdit() || !got.GetViewerCanDelete() || got.GetParentId() != pid || got.GetAuthor().GetId() != "7" {
		t.Errorf("CreateComment 回應 = %v", got)
	}
	uc, err := e.client.UpdateComment(ctx, signedIn(&pb.UpdateCommentRequest{Comment: &pb.Comment{Id: id.String(), Etag: "3", Body: "改過"}, UpdateMask: mask("body")}))
	if got := uc.Msg.GetComment(); err != nil || got.GetBody() != "改過" || got.GetEtag() != "4" || got.GetLikeCount() != 4 || got.GetReplyCount() != 2 || got.GetUpdateTime() == nil || !got.GetIsMine() {
		t.Fatal("UpdateComment", err, uc)
	}
	if _, err := e.client.DeleteComment(ctx, signedIn(&pb.DeleteCommentRequest{CommentId: id.String()})); err != nil {
		t.Fatal("DeleteComment", err)
	}
	cf, err := e.client.CreateForumEntry(ctx, signedIn(&pb.CreateForumEntryRequest{StoreId: "5", Body: "請問", RequestId: rid}))
	if err != nil {
		t.Fatal("CreateForumEntry", err)
	}
	if got := cf.Msg.GetEntry(); got.GetBody() != "請問" || got.GetStoreId() != "5" || !got.GetIsMine() {
		t.Errorf("CreateForumEntry 回應 = %v", got)
	}
	uf, err := e.client.UpdateForumEntry(ctx, signedIn(&pb.UpdateForumEntryRequest{Entry: &pb.ForumEntry{Id: id.String(), Etag: "5", Body: "改過"}, UpdateMask: mask("body")}))
	if got := uf.Msg.GetEntry(); err != nil || got.GetBody() != "改過" || got.GetEtag() != "6" || got.GetReplyCount() != 3 {
		t.Fatal("UpdateForumEntry", err)
	}
	if _, err := e.client.DeleteForumEntry(ctx, signedIn(&pb.DeleteForumEntryRequest{EntryId: id.String(), Etag: "6"})); err != nil {
		t.Fatal("DeleteForumEntry", err)
	}

	ops := make([]string, len(w.calls))
	for i, c := range w.calls {
		ops[i] = c.op
	}
	want := "share likePost unlikePost likeComment unlikeComment createComment updateComment deleteComment createForum updateForum deleteForum"
	if strings.Join(ops, " ") != want {
		t.Fatalf("呼叫順序 = %v", ops)
	}
	if c := w.calls[0]; c.buyer != 7 || c.id != p.ID {
		t.Errorf("SharePost 的買家與貼文 = %+v", c)
	}
	if c := w.calls[3]; c.caller.BuyerID != 7 || c.id != id {
		t.Errorf("LikeComment 的呼叫者 = %+v", c)
	}
	if c := w.calls[6]; c.ver != 3 || c.body != "改過" || c.id != id {
		t.Errorf("UpdateComment 的版本與內文 = %+v", c)
	}
	if c := w.calls[7]; c.delVer != 0 {
		t.Errorf("DeleteComment 沒帶 etag 卻傳了版本 %d", c.delVer)
	}
	if c := w.calls[10]; c.delVer != 6 {
		t.Errorf("DeleteForumEntry 的版本 = %+v", c)
	}
	cin := w.calls[5].cin
	if cin.PostID != p.ID || cin.ParentID == nil || *cin.ParentID != parent || cin.ReplyToID != nil || cin.RequestID == nil || cin.RequestID.String() != rid {
		t.Errorf("CreateComment 的輸入 = %+v", cin)
	}
	if fin := w.calls[8].fin; fin.StoreID != 5 || fin.RequestID == nil || fin.RequestID.String() != rid {
		t.Errorf("CreateForumEntry 的輸入 = %+v", fin)
	}
}

func TestSocialAppRejectsMalformedInput(t *testing.T) {
	w := &fakeWrites{post: post(5)}
	e := newAppEnv(t, w, &fakeStores{})
	ctx := t.Context()
	notV4 := uuid.New().String()
	notV4 = notV4[:14] + "7" + notV4[15:]
	wrongVariant := uuid.New().String()
	wrongVariant = wrongVariant[:14] + "4" + wrongVariant[15:19] + "c" + wrongVariant[20:]
	cases := map[string]error{}
	_, cases["post_id"] = e.client.LikePost(ctx, signedIn(&pb.LikePostRequest{PostId: "x"}))
	_, cases["request_id variant"] = e.client.CreateComment(ctx, signedIn(&pb.CreateCommentRequest{PostId: uuid.New().String(), Body: "a", RequestId: wrongVariant}))
	_, cases["request_id"] = e.client.CreateComment(ctx, signedIn(&pb.CreateCommentRequest{PostId: uuid.New().String(), Body: "a", RequestId: notV4}))
	_, cases["parent_id"] = e.client.CreateComment(ctx, signedIn(&pb.CreateCommentRequest{PostId: uuid.New().String(), Body: "a", ParentId: new("x")}))
	_, cases["store_id"] = e.client.CreateForumEntry(ctx, signedIn(&pb.CreateForumEntryRequest{StoreId: "abc", Body: "a"}))
	_, cases["update etag 缺"] = e.client.UpdateComment(ctx, signedIn(&pb.UpdateCommentRequest{Comment: &pb.Comment{Id: uuid.New().String(), Body: "a"}, UpdateMask: mask("body")}))
	_, cases["update 沒帶資源"] = e.client.UpdateComment(ctx, signedIn(&pb.UpdateCommentRequest{UpdateMask: mask("body")}))
	_, cases["update_mask 沒列 body"] = e.client.UpdateComment(ctx, signedIn(&pb.UpdateCommentRequest{Comment: &pb.Comment{Id: uuid.New().String(), Etag: "1", Body: "a"}, UpdateMask: mask("etag")}))
	_, cases["update_mask 不認得的路徑"] = e.client.UpdateComment(ctx, signedIn(&pb.UpdateCommentRequest{Comment: &pb.Comment{Id: uuid.New().String(), Etag: "1", Body: "a"}, UpdateMask: mask("nope")}))
	_, cases["forum etag 缺"] = e.client.UpdateForumEntry(ctx, signedIn(&pb.UpdateForumEntryRequest{Entry: &pb.ForumEntry{Id: uuid.New().String(), Body: "a"}, UpdateMask: mask("body")}))
	for name, err := range cases {
		if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
			t.Errorf("%s：err = %v，預期 invalid_argument", name, err)
		}
	}
	_, err := e.client.DeleteComment(ctx, signedIn(&pb.DeleteCommentRequest{CommentId: uuid.New().String(), Etag: "x"}))
	if code := connect.CodeOf(err); code != connect.CodeAborted {
		t.Errorf("delete etag 格式錯：err = %v，預期 aborted", err)
	}
	if len(w.calls) != 0 {
		t.Fatalf("格式錯誤不該到模組：%v", w.calls)
	}
}

// 模組的拒絕（被店家封鎖的買家留言、被隱藏的留言不能改）原樣傳到 client，不被吞掉或改寫成成功。
func TestSocialAppPropagatesModuleRefusals(t *testing.T) {
	ctx := t.Context()
	w := &fakeWrites{post: post(5), err: apperr.PermissionDenied("這家店已限制你在此互動")}
	e := newAppEnv(t, w, &fakeStores{})
	pid := uuid.New().String()
	if _, err := e.client.CreateComment(ctx, signedIn(&pb.CreateCommentRequest{PostId: pid, Body: "a"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("被封鎖者留言：%v", err)
	}
	if _, err := e.client.CreateForumEntry(ctx, signedIn(&pb.CreateForumEntryRequest{StoreId: "5", Body: "a"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("被封鎖者發言：%v", err)
	}
	if _, err := e.client.LikePost(ctx, signedIn(&pb.LikePostRequest{PostId: pid})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("被封鎖者按讚：%v", err)
	}
	w.err = apperr.FailedPrecondition("hidden")
	if _, err := e.client.UpdateComment(ctx, signedIn(&pb.UpdateCommentRequest{Comment: &pb.Comment{Id: pid, Etag: "1", Body: "a"}, UpdateMask: mask("body")})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("被隱藏的留言：%v", err)
	}
	w.err = apperr.InvalidInputOn([]string{"RequestID"}, "%s", "reused")
	_, err := e.client.CreateComment(ctx, signedIn(&pb.CreateCommentRequest{PostId: pid, Body: "a"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("request_id 重複使用：%v", err)
	}
}

// 回應的 viewer_* 欄位依呼叫者此刻的身分：店主在自己店的貼文留言，回應仍帶店主的刪除與回覆權。
func TestCreateCommentResponseForTheStoreOwner(t *testing.T) {
	own := &seller.Store{StoreID: 5}
	w := &fakeWrites{post: post(5)}
	e := newAppEnv(t, w, &fakeStores{own: own})
	resp, err := e.client.CreateComment(t.Context(), signedIn(&pb.CreateCommentRequest{PostId: uuid.New().String(), Body: "謝謝"}))
	if err != nil {
		t.Fatal(err)
	}
	if c := resp.Msg.GetComment(); !c.GetViewerCanReply() || !c.GetViewerCanDelete() {
		t.Errorf("comment = %v", c)
	}
	if w.calls[0].caller.Store == nil || w.calls[0].caller.Store.StoreID != 5 {
		t.Errorf("呼叫者的店 = %+v", w.calls[0].caller)
	}
}
