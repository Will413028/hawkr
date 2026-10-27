package grpc

// #2744：pkg/delegation.Validate 自身的拒絕（S1a–S5）一律改發 9289
// DELEGATION_INTERNAL_CONTRACT_MISMATCH，不再分散在 9260／9262。
//
// 理由：DelegationContext 由 Gateway 依工作階段組裝，操作者碰不到它。缺 session_id、
// capability 不在封閉列舉內、capability 與本 RPC 宣告不符 —— 每一條都是「我方組錯」，
// 而 9260 說「你填錯了」、9262 說「你沒有這個權限」，兩者都把操作者送去做一件不會
// 改變任何事的工作。9262 更嚴重：它會以 Result=FAILED 寫進 admin_operation_audit_logs，
// 讓稽核列指控一位其實沒做錯任何事的管理員。
//
// ⚠️ 本檔**沒有**全部改：領域服務自己的欄位驗證（pkg/delegation.InvalidCommand）仍是
// 9260，真正的能力拒絕仍是 9262。只有 Validate 內部那幾條變了。

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

const (
	delegTargetStore = int64(2685914682)
	delegOtherStore  = int64(2632049831)
)

// writeCtx is a delegation context for a mutating capability: audit_log_id is
// populated because S1b demands it on .write / .publish.
func writeCtx(cap string) *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       "7100000000000001",
		OperatorAdminId: "9100000000000001",
		Capability:      cap,
		OperationReason: "協助賣家整理貼文",
		AuditLogId:      "8100000000000001",
	}
}

// readCtx is a delegation context for a read capability: audit_log_id is empty,
// which is legal only because S1b does not apply to .read (fixture E12/E13).
func readCtx(cap string) *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       "7100000000000001",
		OperatorAdminId: "9100000000000001",
		Capability:      cap,
	}
}

// stubMediaClient answers every staged token with one linked asset.
type stubMediaClient struct{}

func (stubMediaClient) FinalizeAndLink(context.Context, string, string, string) (usecase.AssetInfo, error) {
	return usecase.AssetInfo{AssetId: "501", ObjectKey: "posts/a.jpg"}, nil
}

func (stubMediaClient) ReleaseReferences(context.Context, string, []string, []string) error {
	return nil
}

func newDelegationServer(repo *stubPostRepo) *ContentServer {
	return newDelegationServerWithHashtags(repo, &stubHashtagRepo{})
}

// newDelegationServerWithHashtags is the same server with a hashtag repository
// the caller keeps a handle on, so a test can read back the counter deltas.
func newDelegationServerWithHashtags(repo *stubPostRepo, tags *stubHashtagRepo) *ContentServer {
	postUC := usecase.NewPostUseCase(
		repo,
		usecase.NewHashtagUseCase(tags, func() int64 { return 88888 }),
		nil, nil,
		stubMediaClient{},
		&stubSellerClient{},
		&stubBuyerClient{},
		nil, // productClient: only reached when the post has a product_id
		nil,
		&config.Config{},
		func() int64 { return 4242 },
		nil,
	)
	return NewContentServer(postUC, nil, nil, nil, nil, nil, &stubBuyerClient{}, &stubSellerClient{}, nil, zap.NewNop())
}

// assertBizStatus asserts the error is a gRPC status whose message carries the
// "[NNNN] " prefix the Gateway parses. Asserting only "an error happened" would
// pass for every one of the six delegation rejection reasons alike.
func assertBizStatus(t *testing.T, err error, wantCode int32) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error carrying business code %d, got nil", wantCode)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %T: %v", err, err)
	}
	want := "[" + itoa(wantCode) + "] "
	if !strings.HasPrefix(st.Message(), want) {
		t.Fatalf("message = %q, want prefix %q", st.Message(), want)
	}
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func itoa(v int32) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

// ==================== GetPostDetail: Q4 ownership ====================

func TestGetPostDetail_Delegated_CrossStore_Denied(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID:            555,
		SellerID:      delegOtherStore, // belongs to a store this session does not target
		PublishStatus: domain.PostPublishStatusPublished,
		Content:       stringPointer("別家的貼文"),
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "555",
		OwnerStoreId: delegTargetStore,
		Delegation:   readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.DelegationResourceNotOwned {
		t.Fatalf("code = %d, want %d (DELEGATION_RESOURCE_NOT_OWNED)", resp.GetCode(), bizcode.DelegationResourceNotOwned)
	}
	if resp.GetData() != nil {
		t.Fatalf("a denied cross-store read must not return post data, got %+v", resp.GetData())
	}
}

// A published post that belongs to the target must still come back — the denial
// above has to be about ownership, not about the post being unreachable.
func TestGetPostDetail_Delegated_OwnStore_Allowed(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID:            555,
		SellerID:      delegTargetStore,
		PublishStatus: domain.PostPublishStatusPublished,
		Content:       stringPointer("自家的貼文"),
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "555",
		OwnerStoreId: delegTargetStore,
		Delegation:   readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0", resp.GetCode())
	}
	if resp.GetData().GetId() != "555" {
		t.Fatalf("data.id = %q, want \"555\"", resp.GetData().GetId())
	}
}

// A draft belonging to the target is visible to the delegated reader: the
// workspace is the seller's own view of their own content.
func TestGetPostDetail_Delegated_OwnDraft_Visible(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID:            556,
		SellerID:      delegTargetStore,
		PublishStatus: domain.PostPublishStatusDraft,
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "556",
		OwnerStoreId: delegTargetStore,
		Delegation:   readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0 (a draft of the target store is visible)", resp.GetCode())
	}
	if got := resp.GetData().GetPublishStatus(); got != "4401" {
		t.Fatalf("publishStatus = %q, want \"4401\"", got)
	}
}

// The buyer / anonymous path is untouched: with no delegation context there is
// no ownership assertion, so cross-store reads keep working exactly as before.
func TestGetPostDetail_NoDelegation_CrossStoreStillAllowed(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID:            557,
		SellerID:      delegOtherStore,
		PublishStatus: domain.PostPublishStatusPublished,
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{PostId: "557"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0 (non-delegated reads are unchanged)", resp.GetCode())
	}
}

// ==================== GetPostDetail: owner read ====================

// An owner read (no delegation, owner_store_id set) returns the store's own post
// in every publish state and hidden state, and is not a view.
func TestGetPostDetail_Owner_OwnPost_EveryState(t *testing.T) {
	hiddenAt := time.Now()
	reason := "違規"
	cases := []struct {
		name string
		post *gen.CmsPost
		want string
	}{
		{"draft", &gen.CmsPost{ID: 601, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusDraft}, "4401"},
		{"published", &gen.CmsPost{ID: 602, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished}, "4402"},
		{"self-unpublished", &gen.CmsPost{ID: 603, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusUnpublished}, "4403"},
		{"platform-hidden", &gen.CmsPost{ID: 604, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished, HiddenAt: &hiddenAt, HiddenReason: &reason}, "4402"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubPostRepo{post: tc.post}
			srv := newDelegationServer(repo)

			resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
				PostId:       itoa64(tc.post.ID),
				OwnerStoreId: delegTargetStore,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.GetCode() != 0 {
				t.Fatalf("code = %d, want 0 (an owner reads its own post in every state)", resp.GetCode())
			}
			if got := resp.GetData().GetPublishStatus(); got != tc.want {
				t.Fatalf("publishStatus = %q, want %q", got, tc.want)
			}
			if repo.viewIncrements != 0 {
				t.Fatalf("view_count incremented %d times, want 0 for an owner read", repo.viewIncrements)
			}
			wantHidden := tc.post.HiddenAt != nil
			if gotHidden := resp.GetData().GetHiddenAt() != ""; gotHidden != wantHidden {
				t.Fatalf("hiddenAt present = %v, want %v", gotHidden, wantHidden)
			}
			if wantHidden && resp.GetData().GetHiddenReason() != reason {
				t.Fatalf("hiddenReason = %q, want %q", resp.GetData().GetHiddenReason(), reason)
			}
		})
	}
}

// Another store's post answers NotFound, not ResourceNotOwned: the owner path
// must not disclose that the post exists.
func TestGetPostDetail_Owner_OtherStorePost_NotFound(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID:            605,
		SellerID:      delegOtherStore,
		PublishStatus: domain.PostPublishStatusPublished,
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "605",
		OwnerStoreId: delegTargetStore,
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != int32(pkgerrors.CodeNotFound) {
		t.Fatalf("code = %d, want %d (NotFound)", resp.GetCode(), pkgerrors.CodeNotFound)
	}
	if resp.GetCode() == bizcode.DelegationResourceNotOwned {
		t.Fatalf("owner path answered ResourceNotOwned, which discloses existence")
	}
	if resp.GetData() != nil {
		t.Fatalf("a not-owned owner read must not return post data, got %+v", resp.GetData())
	}
	if repo.viewIncrements != 0 {
		t.Fatalf("view_count incremented %d times, want 0", repo.viewIncrements)
	}
}

// A post that does not exist answers the same NotFound as another store's post.
func TestGetPostDetail_Owner_MissingPost_NotFound(t *testing.T) {
	srv := newDelegationServer(&stubPostRepo{})

	resp, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "606",
		OwnerStoreId: delegTargetStore,
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != int32(pkgerrors.CodeNotFound) {
		t.Fatalf("code = %d, want %d (NotFound)", resp.GetCode(), pkgerrors.CodeNotFound)
	}
}

// The delegated branch keeps its code: a post of another store is
// DELEGATION_RESOURCE_NOT_OWNED (the Gateway writes the audit row from it), and a
// delegated read of the target's own post still counts as a view and carries the
// platform-hidden fields.
func TestGetPostDetail_Delegated_BranchUnchangedByOwnerRead(t *testing.T) {
	other := &stubPostRepo{post: &gen.CmsPost{ID: 607, SellerID: delegOtherStore, PublishStatus: domain.PostPublishStatusPublished}}
	resp, err := newDelegationServer(other).GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "607",
		OwnerStoreId: delegTargetStore,
		Delegation:   readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.DelegationResourceNotOwned {
		t.Fatalf("code = %d, want %d (DELEGATION_RESOURCE_NOT_OWNED)", resp.GetCode(), bizcode.DelegationResourceNotOwned)
	}

	hiddenAt := time.Now()
	reason := "違規"
	own := &stubPostRepo{post: &gen.CmsPost{ID: 608, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished, HiddenAt: &hiddenAt, HiddenReason: &reason}}
	ownResp, err := newDelegationServer(own).GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "608",
		OwnerStoreId: delegTargetStore,
		Delegation:   readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ownResp.GetData().GetHiddenAt() == "" || ownResp.GetData().GetHiddenReason() != reason {
		t.Fatalf("the delegated branch is the seller's own view and must carry hidden fields, got %+v", ownResp.GetData())
	}
	if own.viewIncrements != 1 {
		t.Fatalf("delegated read view increments = %d, want 1 (unchanged)", own.viewIncrements)
	}
}

// The public path (no delegation, no owner_store_id) still counts a view.
func TestGetPostDetail_Public_CountsView(t *testing.T) {
	hiddenAt := time.Now()
	reason := "違規"
	repo := &stubPostRepo{post: &gen.CmsPost{ID: 609, SellerID: delegOtherStore, PublishStatus: domain.PostPublishStatusPublished, HiddenAt: &hiddenAt, HiddenReason: &reason}}
	resp, err := newDelegationServer(repo).GetPostDetail(context.Background(), &pb.GetPostDetailRequest{PostId: "609"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetData().GetHiddenAt() != "" || resp.GetData().GetHiddenReason() != "" {
		t.Fatalf("the public branch must not carry hidden fields, got %+v", resp.GetData())
	}
	if repo.viewIncrements != 1 {
		t.Fatalf("public read view increments = %d, want 1", repo.viewIncrements)
	}
}

// ==================== S1a–S5 at the RPC boundary ====================

func TestGetPostDetail_Delegated_WrongCapability_Denied(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{ID: 1, SellerID: delegTargetStore}}
	srv := newDelegationServer(repo)

	// storefront.read is a legal token (S2 passes) but not the one this RPC
	// declares, so S3 must reject it.
	_, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:       "1",
		OwnerStoreId: delegTargetStore,
		Delegation:   readCtx("storefront.read"),
	})
	assertBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

func TestGetPostDetail_Delegated_MissingOwnerStore_Denied(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{ID: 1, SellerID: delegTargetStore}}
	srv := newDelegationServer(repo)

	_, err := srv.GetPostDetail(context.Background(), &pb.GetPostDetailRequest{
		PostId:     "1",
		Delegation: readCtx("posts.read"),
	})
	assertBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

func TestListMyPosts_Delegated_WrongCapability_Denied(t *testing.T) {
	srv := newDelegationServer(&stubPostRepo{})

	_, err := srv.ListMyPosts(context.Background(), &pb.ListMyPostsRequest{
		SellerId:   delegTargetStore,
		Delegation: readCtx("posts.write"),
	})
	assertBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

// ListMyPosts 的每一列都要帶得出 publish_status。
//
// 這一層是三層之中唯一沒有守衛的一層：上游 PostListItem 一度根本沒有這個欄位，而
// Gateway 那支契約測試是掃 struct tag 的，它證明得了「DTO 有這個鍵」，證明不了「值有流
// 過來」。⇒ 刪掉 handler 的 PublishStatus 賦值，兩邊全套測試都是綠的，前端拿到空字串，
// 而空字串在 UI 上與「沒有這個欄位」無法分辨 —— 兩者都是不畫 pill，草稿就與已發布長得
// 一模一樣。
//
// 詳情路徑已有 TestGetPostDetail_Delegated_OwnDraft_Visible 與
// TestCreatePost_Delegated_IsDraft 兩支在守；列表是最後一條沒人守的路徑，而它才是管理
// 員判斷這家店經營狀態的那一頁。
func TestListMyPosts_Delegated_CarriesPublishStatus(t *testing.T) {
	repo := &stubPostRepo{myPosts: []*gen.CmsPost{{
		ID:            601,
		SellerID:      delegTargetStore,
		PublishStatus: domain.PostPublishStatusDraft,
		Content:       stringPointer("還沒發布的草稿"),
	}}}
	srv := newDelegationServer(repo)

	resp, err := srv.ListMyPosts(context.Background(), &pb.ListMyPostsRequest{
		SellerId:   delegTargetStore,
		Page:       1,
		PageSize:   20,
		Delegation: readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0", resp.GetCode())
	}
	if len(resp.GetData()) != 1 {
		t.Fatalf("data length = %d, want 1 —— fixture 沒進到回應，下面斷言等於沒驗", len(resp.GetData()))
	}
	if got := resp.GetData()[0].GetPublishStatus(); got != "4401" {
		t.Fatalf("publishStatus = %q, want \"4401\"（草稿）—— 空字串代表列表根本沒帶狀態", got)
	}
}

// 店主與代行者的列表要看得出「這則草稿排了發布時間」：只有排程尚未執行（pending）才帶，
// 已經執行或取消的排程不帶，否則列表會對一則已經發布或不會發布的貼文說「排程於…發布」。
func TestListMyPosts_CarriesScheduledPublishAtOnlyWhilePending(t *testing.T) {
	at := time.Date(2026, 10, 20, 3, 30, 0, 0, time.UTC)
	pending, done := domain.ScheduleStatePending, domain.ScheduleStateDone
	repo := &stubPostRepo{myPosts: []*gen.CmsPost{
		{ID: 701, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusDraft, ScheduledPublishAt: &at, ScheduleState: &pending},
		{ID: 702, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished, ScheduledPublishAt: &at, ScheduleState: &done},
		{ID: 703, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusDraft},
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.ListMyPosts(context.Background(), &pb.ListMyPostsRequest{
		SellerId:   delegTargetStore,
		Page:       1,
		PageSize:   20,
		Delegation: readCtx("posts.read"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetData()) != 3 {
		t.Fatalf("data length = %d, want 3", len(resp.GetData()))
	}
	want := timeutil.FormatISO8601(at)
	for i, w := range []string{want, "", ""} {
		if got := resp.GetData()[i].GetScheduledPublishAt(); got != w {
			t.Errorf("data[%d].scheduledPublishAt = %q, want %q", i, got, w)
		}
	}
}

// S1b: a mutating capability with no audit_log_id means someone reached the
// domain service without going through the two-phase audit.
func TestCreatePost_Delegated_MissingAuditLogID_Denied(t *testing.T) {
	srv := newDelegationServer(&stubPostRepo{})

	d := writeCtx("posts.write")
	d.AuditLogId = ""
	_, err := srv.CreatePost(context.Background(), &pb.CreatePostRequest{
		SellerId:   delegTargetStore,
		Content:    "內容",
		Delegation: d,
	})
	assertBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

// S5: the request's own idempotency_key must agree with the context's.
func TestUpdatePost_Delegated_IdempotencyKeyMismatch_Denied(t *testing.T) {
	srv := newDelegationServer(&stubPostRepo{post: &gen.CmsPost{ID: 9, SellerID: delegTargetStore}})

	d := writeCtx("posts.write")
	d.IdempotencyKey = "ik_b"
	_, err := srv.UpdatePost(context.Background(), &pb.UpdatePostRequest{
		PostId:          "9",
		SellerId:        delegTargetStore,
		Content:         "新內容",
		ExpectedVersion: "3",
		IdempotencyKey:  "ik_a",
		Delegation:      d,
	})
	assertBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

// ==================== delegated create is a draft ====================

func TestCreatePost_Delegated_IsDraft(t *testing.T) {
	repo := &stubPostRepo{}
	srv := newDelegationServer(repo)

	resp, err := srv.CreatePost(context.Background(), &pb.CreatePostRequest{
		SellerId:   delegTargetStore,
		Content:    "秋季產季貼文",
		Delegation: writeCtx("posts.write"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0", resp.GetCode())
	}
	if repo.lastPublishStatus != domain.PostPublishStatusDraft {
		t.Fatalf("publish_status written = %d, want %d (draft)", repo.lastPublishStatus, domain.PostPublishStatusDraft)
	}
	if got := resp.GetData().GetPublishStatus(); got != "4401" {
		t.Fatalf("response publishStatus = %q, want \"4401\"", got)
	}
}

func TestCreatePost_SellerOwnPath_IsPublished(t *testing.T) {
	repo := &stubPostRepo{}
	srv := newDelegationServer(repo)

	if _, err := srv.CreatePost(context.Background(), &pb.CreatePostRequest{
		SellerId:     delegTargetStore,
		Content:      "賣家自己發的",
		StagedTokens: []string{"tok"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastPublishStatus != domain.PostPublishStatusPublished {
		t.Fatalf("publish_status written = %d, want %d (published)", repo.lastPublishStatus, domain.PostPublishStatusPublished)
	}
}

// B1：公開中的貼文恰好 1 張照片、內文必填，每一條進入公開的路徑都由 social 擋。
// 被擋的請求不得寫入任何東西。
func assertInvalidInput(t *testing.T, resp *pb.PostResponse, err error, wantMsg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != 400 || resp.GetMessage() != wantMsg {
		t.Fatalf("code/message = %d / %q, want 400 / %q", resp.GetCode(), resp.GetMessage(), wantMsg)
	}
}

func TestCreatePost_SellerOwnPath_B1Rejections(t *testing.T) {
	cases := []struct {
		name string
		req  *pb.CreatePostRequest
		want string
	}{
		{"沒有照片", &pb.CreatePostRequest{Content: "內文"}, "每則貼文需要 1 張照片才能發布"},
		{"內文空白", &pb.CreatePostRequest{Content: "  \n ", StagedTokens: []string{"a"}}, "貼文內容不能空白"},
		{"兩張照片", &pb.CreatePostRequest{Content: "內文", StagedTokens: []string{"a", "b"}}, "每則貼文只能有 1 張照片"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &stubPostRepo{}
			srv := newDelegationServer(repo)
			c.req.SellerId = delegTargetStore
			resp, err := srv.CreatePost(context.Background(), c.req)
			assertInvalidInput(t, resp, err, c.want)
			if repo.lastPublishStatus != 0 {
				t.Fatal("被拒絕的建立不得寫入")
			}
		})
	}
}

// 草稿可以沒有照片、沒有內文；但多於 1 張照片連草稿都拒絕（不再靜默截斷）。
func TestCreatePost_Delegated_DraftMayLackImage_ButNotExceedOne(t *testing.T) {
	srv := newDelegationServer(&stubPostRepo{})
	resp, err := srv.CreatePost(context.Background(), &pb.CreatePostRequest{
		SellerId: delegTargetStore, Content: "草稿", Delegation: writeCtx("posts.write"),
	})
	if err != nil || resp.GetCode() != 0 {
		t.Fatalf("草稿沒有照片應成功: err=%v code=%d", err, resp.GetCode())
	}
	resp, err = srv.CreatePost(context.Background(), &pb.CreatePostRequest{
		SellerId: delegTargetStore, Content: "草稿", StagedTokens: []string{"a", "b"}, Delegation: writeCtx("posts.write"),
	})
	assertInvalidInput(t, resp, err, "每則貼文只能有 1 張照片")
}

func TestUpdatePost_B1Rejections(t *testing.T) {
	published := func() *gen.CmsPost {
		return &gen.CmsPost{ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished,
			Content: ptrString("內文"), Image: ptrString("posts/a.jpg")}
	}
	pending := domain.ScheduleStatePending
	scheduled := func() *gen.CmsPost {
		p := published()
		p.PublishStatus = domain.PostPublishStatusDraft
		p.ScheduleState = &pending
		return p
	}
	cases := []struct {
		name string
		post *gen.CmsPost
		req  *pb.UpdatePostRequest
		want string
	}{
		{"已發布清圖", published(), &pb.UpdatePostRequest{Content: "內文"}, "每則貼文需要 1 張照片才能發布"},
		{"排程中清圖", scheduled(), &pb.UpdatePostRequest{Content: "內文"}, "每則貼文需要 1 張照片才能發布"},
		{"已發布內文清空", published(), &pb.UpdatePostRequest{Content: "  \n", KeepAssetIds: []string{"1"}}, "貼文內容不能空白"},
		{"排程中內文清空", scheduled(), &pb.UpdatePostRequest{Content: "", KeepAssetIds: []string{"1"}}, "貼文內容不能空白"},
		{"保留加新增共兩張", published(), &pb.UpdatePostRequest{Content: "內文", KeepAssetIds: []string{"1"}, NewStagedTokens: []string{"a"}}, "每則貼文只能有 1 張照片"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &stubPostRepo{post: c.post}
			srv := newDelegationServer(repo)
			c.req.PostId, c.req.SellerId, c.req.ExpectedVersion = "9", delegTargetStore, "1"
			c.req.Delegation = writeCtx("posts.write")
			resp, err := srv.UpdatePost(context.Background(), c.req)
			assertInvalidInput(t, resp, err, c.want)
		})
	}
	t.Run("草稿清圖照舊", func(t *testing.T) {
		d := published()
		d.PublishStatus = domain.PostPublishStatusDraft
		srv := newDelegationServer(&stubPostRepo{post: d})
		resp, err := srv.UpdatePost(context.Background(), &pb.UpdatePostRequest{
			PostId: "9", SellerId: delegTargetStore, Content: "內文", ExpectedVersion: "1", Delegation: writeCtx("posts.write"),
		})
		if err != nil || resp.GetCode() != 0 {
			t.Fatalf("草稿清圖應成功: err=%v code=%d", err, resp.GetCode())
		}
	})
}

func TestUpdateSellerPostStatus_B1Rejections(t *testing.T) {
	noImage := func() *gen.CmsPost {
		p := draftPost()
		p.Image = nil
		return p
	}
	blank := func() *gen.CmsPost {
		p := draftPost()
		p.Content = ptrString(" ")
		return p
	}
	at := time.Now().UTC().Add(2 * time.Hour).Format("2006-01-02T15:04:05Z")
	cases := []struct {
		name     string
		post     *gen.CmsPost
		schedule string
		want     string
	}{
		{"發布沒有照片", noImage(), "", "每則貼文需要 1 張照片才能發布"},
		{"發布內文空白", blank(), "", "貼文內容不能空白"},
		{"排程沒有照片", noImage(), at, "每則貼文需要 1 張照片才能發布"},
		{"排程內文空白", blank(), at, "貼文內容不能空白"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &stubPostRepo{post: c.post}
			srv := newDelegationServer(repo)
			req := statusReq("4402")
			req.ScheduledPublishAt = c.schedule
			resp, err := srv.UpdateSellerPostStatus(context.Background(), req)
			assertInvalidInput(t, resp, err, c.want)
			if repo.lastPublishStatus != 0 || repo.lastScheduledAt != nil {
				t.Fatal("被拒絕的發布不得寫入")
			}
		})
	}
	t.Run("下架不檢查", func(t *testing.T) {
		p := noImage()
		p.PublishStatus = domain.PostPublishStatusPublished
		srv := newDelegationServer(&stubPostRepo{post: p})
		resp, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4403"))
		if err != nil || resp.GetCode() != 0 {
			t.Fatalf("下架應成功: err=%v code=%d", err, resp.GetCode())
		}
	})
}

// ==================== expectedVersion reaches the CAS ====================

func TestUpdatePost_Delegated_ExpectedVersionReachesRepository(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{ID: 9, SellerID: delegTargetStore}}
	srv := newDelegationServer(repo)

	if _, err := srv.UpdatePost(context.Background(), &pb.UpdatePostRequest{
		PostId:          "9",
		SellerId:        delegTargetStore,
		Content:         "新內容",
		ExpectedVersion: "7",
		Delegation:      writeCtx("posts.write"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastExpectedVersion == nil {
		t.Fatal("expectedVersion never reached the repository — the CAS predicate was disabled, so a delegated write would overwrite the seller silently")
	}
	if *repo.lastExpectedVersion != 7 {
		t.Fatalf("expectedVersion = %d, want 7", *repo.lastExpectedVersion)
	}
}

// #2772: this used to be TestUpdatePost_SellerOwnPath_NoCAS and asserted the
// seller's own path reached the repository with a nil expectedVersion (CAS
// disabled). That is exactly the A/B race the ticket closes: two concurrent
// image swaps on the seller's own path both used to commit, and whichever
// broad Release landed second deleted the other's just-linked reference. CAS
// is mandatory now on every path — internal/usecase/post.go's UpdatePost
// defaults expectedVersion to the version it just read via GetByID when the
// caller (here, the seller's own path) passed nil. Version is set to 5, not
// left at its zero value, so this test can tell "the read version reached the
// repository" from "some nil/zero default coincidentally reached it".
func TestUpdatePost_SellerOwnPath_UsesReadVersionAsCAS(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{ID: 9, SellerID: delegTargetStore, Version: 5}}
	srv := newDelegationServer(repo)

	if _, err := srv.UpdatePost(context.Background(), &pb.UpdatePostRequest{
		PostId:   "9",
		SellerId: delegTargetStore,
		Content:  "新內容",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastExpectedVersion == nil {
		t.Fatal("expectedVersion never reached the repository — CAS is mandatory now even on the seller's own path (#2772)")
	}
	if *repo.lastExpectedVersion != 5 {
		t.Fatalf("expectedVersion = %d, want 5 (the version GetByID returned on entry)", *repo.lastExpectedVersion)
	}
}

func TestUpdatePost_Delegated_VersionConflict_Returns9266(t *testing.T) {
	repo := &stubPostRepo{
		post:      &gen.CmsPost{ID: 9, SellerID: delegTargetStore},
		updateErr: domain.ErrVersionConflict,
	}
	srv := newDelegationServer(repo)

	resp, err := srv.UpdatePost(context.Background(), &pb.UpdatePostRequest{
		PostId:          "9",
		SellerId:        delegTargetStore,
		Content:         "新內容",
		ExpectedVersion: "7",
		Delegation:      writeCtx("posts.write"),
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.DelegationResourceVersionConflict {
		t.Fatalf("code = %d, want %d (DELEGATION_RESOURCE_VERSION_CONFLICT)",
			resp.GetCode(), bizcode.DelegationResourceVersionConflict)
	}
}

func TestUpdatePost_Delegated_MalformedExpectedVersion_Denied(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{ID: 9, SellerID: delegTargetStore}}
	srv := newDelegationServer(repo)

	_, err := srv.UpdatePost(context.Background(), &pb.UpdatePostRequest{
		PostId:          "9",
		SellerId:        delegTargetStore,
		ExpectedVersion: "not-a-number",
		Delegation:      writeCtx("posts.write"),
	})
	assertBizStatus(t, err, bizcode.DelegationInvalidCommand)
	if repo.lastExpectedVersion != nil {
		t.Fatal("a malformed expectedVersion must not reach the repository at all")
	}
}

// ==================== UpdateSellerPostStatus ====================

// statusReq builds a publish/unpublish request with the publish capability.
func statusReq(target string) *pb.UpdateSellerPostStatusRequest {
	return &pb.UpdateSellerPostStatusRequest{
		PostId:          "9",
		SellerId:        delegTargetStore,
		TargetStatus:    target,
		ExpectedVersion: "3",
		Delegation:      writeCtx("posts.publish"),
	}
}

// draftPost is the usual starting point: a delegated create leaves a draft, and
// publishing it is the command under test.
func draftPost() *gen.CmsPost {
	return &gen.CmsPost{ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusDraft,
		Content: ptrString("內文"), Image: ptrString("posts/a.jpg")}
}

func ptrString(s string) *string { return &s }

// S3 at this RPC: posts.write is a legal token but not the one publishing needs.
// Without it the publish capability would be enforced only by the Gateway route,
// and anything that reached this service with a write token could publish.
func TestUpdateSellerPostStatus_WrongCapability_Denied(t *testing.T) {
	repo := &stubPostRepo{post: draftPost()}
	srv := newDelegationServer(repo)

	req := statusReq("4402")
	req.Delegation = writeCtx("posts.write")
	_, err := srv.UpdateSellerPostStatus(context.Background(), req)
	assertBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
	if repo.lastPublishStatus != 0 {
		t.Fatal("a denied capability must not reach the repository")
	}
}

func TestUpdateSellerPostStatus_ImmediatePublish(t *testing.T) {
	repo := &stubPostRepo{post: draftPost()}
	tags := &stubHashtagRepo{boundIDs: []int64{101, 102}}
	srv := newDelegationServerWithHashtags(repo, tags)

	resp, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4402"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0", resp.GetCode())
	}
	if repo.lastPublishStatus != domain.PostPublishStatusPublished {
		t.Fatalf("publish_status written = %d, want %d", repo.lastPublishStatus, domain.PostPublishStatusPublished)
	}
	if repo.lastScheduledAt != nil {
		t.Fatalf("立即發布不得寫任何排程列，卻寫了 %v", *repo.lastScheduledAt)
	}
	if repo.lastExpectedVersion == nil || *repo.lastExpectedVersion != 3 {
		t.Fatalf("expectedVersion = %v, want 3 —— CAS 述詞被丟掉就會靜默覆蓋賣家的編輯", repo.lastExpectedVersion)
	}
	if want := []int32{1, 1}; !equalDeltas(tags.deltas, want) {
		t.Fatalf("hashtag delta = %v, want %v（草稿轉已發布，兩個標籤各 +1）", tags.deltas, want)
	}
}

func TestUpdateSellerPostStatus_Unpublish(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished,
	}}
	tags := &stubHashtagRepo{boundIDs: []int64{101}}
	srv := newDelegationServerWithHashtags(repo, tags)

	if _, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4403")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastPublishStatus != domain.PostPublishStatusUnpublished {
		t.Fatalf("publish_status written = %d, want %d", repo.lastPublishStatus, domain.PostPublishStatusUnpublished)
	}
	if want := []int32{-1}; !equalDeltas(tags.deltas, want) {
		t.Fatalf("hashtag delta = %v, want %v（已發布轉下架）", tags.deltas, want)
	}
}

// A post the platform has hidden contributes to no hashtag count in either
// publish_status, so a status command on it must move no counter. Deriving the
// delta from the status change alone would decrement a count the post never
// contributed, and AdjustPostCount has no floor at zero.
func TestUpdateSellerPostStatus_HiddenPost_MovesNoCounter(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusDraft,
		HiddenAt: timePointer(time.Now().UTC()),
	}}
	tags := &stubHashtagRepo{boundIDs: []int64{101}}
	srv := newDelegationServerWithHashtags(repo, tags)

	if _, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4402")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags.deltas) != 0 {
		t.Fatalf("hashtag delta = %v, want 空——被平台下架的貼文在兩個狀態下都不公開可見", tags.deltas)
	}
}

// Publishing an already-published post is a no-op for visibility. Counting it
// again would double the post's contribution to every tag it carries.
func TestUpdateSellerPostStatus_RepublishMovesNoCounter(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished,
	}}
	tags := &stubHashtagRepo{boundIDs: []int64{101}}
	srv := newDelegationServerWithHashtags(repo, tags)

	if _, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4402")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags.deltas) != 0 {
		t.Fatalf("hashtag delta = %v, want 空——重複發布不會讓貼文變得更可見", tags.deltas)
	}
}

func TestUpdateSellerPostStatus_Scheduled(t *testing.T) {
	repo := &stubPostRepo{post: draftPost()}
	srv := newDelegationServer(repo)

	at := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	req := statusReq("4402")
	req.ScheduledPublishAt = at.Format("2006-01-02T15:04:05Z")
	if _, err := srv.UpdateSellerPostStatus(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastScheduledAt == nil {
		t.Fatal("排程沒有寫進資料庫——貼文永遠不會被發出去，而端點回了成功")
	}
	if !repo.lastScheduledAt.Equal(at) {
		t.Fatalf("scheduled_publish_at = %v, want %v", *repo.lastScheduledAt, at)
	}
	if repo.lastScheduledBySessionID != 7100000000000001 {
		t.Fatalf("scheduled_by_session_id = %d, want 7100000000000001——worker 靠它重建授權，"+
			"缺了就是一列永遠發不出去的 pending", repo.lastScheduledBySessionID)
	}
	if repo.lastPublishStatus != 0 {
		t.Fatalf("排程不得同時改 publish_status（寫了 %d）：pending 的列必須是草稿，"+
			"否則 worker 到點會對一則已計數的貼文再 +1", repo.lastPublishStatus)
	}
}

// The schedule window is checked at both ends. Only rejecting past times would
// accept "one second from now" (a row racing the immediate path) and "ten years
// from now" (a row pending forever) — neither reports anything until much later.
func TestUpdateSellerPostStatus_ScheduleWindow(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name string
		at   time.Time
	}{
		{"距今不足 5 分鐘", now.Add(time.Minute)},
		{"已經是過去", now.Add(-time.Hour)},
		{"超過 90 天", now.Add(91 * 24 * time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &stubPostRepo{post: draftPost()}
			srv := newDelegationServer(repo)

			req := statusReq("4402")
			req.ScheduledPublishAt = c.at.Format("2006-01-02T15:04:05Z")
			_, err := srv.UpdateSellerPostStatus(context.Background(), req)
			assertBizStatus(t, err, bizcode.DelegationInvalidCommand)
			if repo.lastScheduledAt != nil {
				t.Fatal("超出值域的排程時間不得寫進資料庫")
			}
		})
	}
}

// One wire format, not several: a value that parses two ways is a value the
// Gateway's pre-check and this service can disagree about.
func TestUpdateSellerPostStatus_ScheduleFormat(t *testing.T) {
	at := time.Now().UTC().Add(2 * time.Hour)
	for _, raw := range []string{
		at.Format("2006-01-02T15:04:05") + "+08:00", // 帶時區偏移
		at.Format("2006-01-02T15:04:05.000") + "Z",  // 毫秒精度
		at.Format("2006-01-02 15:04:05") + "Z",      // 空格分隔
		at.Format("2006-01-02"),                     // 只有日期
	} {
		t.Run(raw, func(t *testing.T) {
			repo := &stubPostRepo{post: draftPost()}
			srv := newDelegationServer(repo)

			req := statusReq("4402")
			req.ScheduledPublishAt = raw
			_, err := srv.UpdateSellerPostStatus(context.Background(), req)
			assertBizStatus(t, err, bizcode.DelegationInvalidCommand)
		})
	}
}

// /unpublish has no scheduled mode. Accepting the field and ignoring it would
// take a post down now and report a takedown scheduled for later.
func TestUpdateSellerPostStatus_UnpublishRejectsSchedule(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished,
	}}
	srv := newDelegationServer(repo)

	req := statusReq("4403")
	req.ScheduledPublishAt = time.Now().UTC().Add(2 * time.Hour).Format("2006-01-02T15:04:05Z")
	_, err := srv.UpdateSellerPostStatus(context.Background(), req)
	assertBizStatus(t, err, bizcode.DelegationInvalidCommand)
	if repo.lastPublishStatus != 0 || repo.lastScheduledAt != nil {
		t.Fatal("被拒絕的命令不得有任何寫入")
	}
}

// "Back to draft" is not a publish command; this iteration has no such mode.
func TestUpdateSellerPostStatus_RejectsDraftTarget(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished,
	}}
	srv := newDelegationServer(repo)

	_, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4401"))
	assertBizStatus(t, err, bizcode.DelegationInvalidCommand)
	if repo.lastPublishStatus != 0 {
		t.Fatal("4401 被接受了——那會用一支稽核成 PUBLISH 的端點把貼文改回草稿")
	}
}

// Scheduling a published post would have the worker's terminal write add a
// second +1 to every hashtag on it: that write adjusts counters unconditionally,
// and what makes that correct is "a pending row is always a draft".
func TestUpdateSellerPostStatus_ScheduleOnPublishedPost_Rejected(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegTargetStore, PublishStatus: domain.PostPublishStatusPublished,
	}}
	srv := newDelegationServer(repo)

	req := statusReq("4402")
	req.ScheduledPublishAt = time.Now().UTC().Add(2 * time.Hour).Format("2006-01-02T15:04:05Z")
	resp, err := srv.UpdateSellerPostStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	// 9290 而不是 9260（#2744）：這個請求**完全合法**，只是貼文已經在你要求的狀態。
	// 套 9260「代行請求參數不正確」會讓稽核列指控操作者送了錯的指令，而 `錯誤碼.md`
	// 為 9283/9284 寫下的正是同一個論證。斷言換碼是本次修復的重點，不是附帶調整。
	if resp.GetCode() != bizcode.DelegationResourceAlreadyInState {
		t.Fatalf("code = %d, want %d", resp.GetCode(), bizcode.DelegationResourceAlreadyInState)
	}
	// 這一條與碼無關、且是本測試原本的主張（見上方註解：排程已發布貼文會讓 worker
	// 的終局寫入對每個 hashtag 多 +1 一次）。換碼不得讓它鬆掉。
	if repo.lastScheduledAt != nil {
		t.Fatal("已發布的貼文不得被排入 pending")
	}
}

// Q4 (§7.1.2): the row exists but belongs to another store. GetPost has no
// seller filter of its own, so without this assertion a delegated operator
// publishes a post in a store the session does not target.
func TestUpdateSellerPostStatus_CrossStore_Returns9275(t *testing.T) {
	repo := &stubPostRepo{post: &gen.CmsPost{
		ID: 9, SellerID: delegOtherStore, PublishStatus: domain.PostPublishStatusDraft,
	}}
	srv := newDelegationServer(repo)

	resp, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4402"))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.DelegationResourceNotOwned {
		t.Fatalf("code = %d, want %d (DELEGATION_RESOURCE_NOT_OWNED)",
			resp.GetCode(), bizcode.DelegationResourceNotOwned)
	}
	if repo.lastPublishStatus != 0 {
		t.Fatal("跨店的命令不得寫入")
	}
}

func TestUpdateSellerPostStatus_VersionConflict_Returns9266(t *testing.T) {
	repo := &stubPostRepo{post: draftPost(), updateErr: domain.ErrVersionConflict}
	srv := newDelegationServer(repo)

	resp, err := srv.UpdateSellerPostStatus(context.Background(), statusReq("4402"))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.DelegationResourceVersionConflict {
		t.Fatalf("code = %d, want %d", resp.GetCode(), bizcode.DelegationResourceVersionConflict)
	}
}

func equalDeltas(got, want []int32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// stubHashtagRepo answers every read empty unless boundIDs is set, and records
// every counter delta it is handed.
//
// PostUseCase.FillTags runs unconditionally on the read path so the repository
// has to exist at all; boundIDs / deltas exist for the publish-status tests,
// where the delta a status change produces is the thing that silently corrupts
// a hashtag's post_count when it is wrong.
type stubHashtagRepo struct {
	boundIDs []int64
	deltas   []int32
}

func (s *stubHashtagRepo) UpsertByName(_ context.Context, _ gen.DBTX, id int64, _ string) (int64, error) {
	return id, nil
}
func (s *stubHashtagRepo) AdjustPostCount(_ context.Context, _ gen.DBTX, _ int64, delta int32) error {
	s.deltas = append(s.deltas, delta)
	return nil
}
func (s *stubHashtagRepo) InsertBinding(_ context.Context, _ gen.DBTX, _, _ int64, _ string, _ int16) error {
	return nil
}
func (s *stubHashtagRepo) UpdateBinding(_ context.Context, _ gen.DBTX, _, _ int64, _ string, _ int16) error {
	return nil
}
func (s *stubHashtagRepo) DeleteBindings(_ context.Context, _ gen.DBTX, _ int64, _ []int64) error {
	return nil
}
func (s *stubHashtagRepo) DeleteAllBindings(_ context.Context, _ gen.DBTX, _ int64) error { return nil }
func (s *stubHashtagRepo) ListHashtagIDsByPostID(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
	return s.boundIDs, nil
}
func (s *stubHashtagRepo) ListByPostID(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
	return nil, nil
}
func (s *stubHashtagRepo) ListByPostIDs(_ context.Context, _ []int64) (map[int64][]gen.CmsPostHashtag, error) {
	return map[int64][]gen.CmsPostHashtag{}, nil
}
func (s *stubHashtagRepo) ListHashtagsByIDs(_ context.Context, _ []int64) (map[int64]gen.CmsHashtag, error) {
	return map[int64]gen.CmsHashtag{}, nil
}
func (s *stubHashtagRepo) SearchByPrefix(_ context.Context, _ string, _ int32) ([]gen.SearchHashtagsByPrefixRow, error) {
	return nil, nil
}
func (s *stubHashtagRepo) SearchByContain(_ context.Context, _ string, _ int32) ([]gen.SearchHashtagsByContainRow, error) {
	return nil, nil
}
func (s *stubHashtagRepo) PopularInRange(_ context.Context, _, _ time.Time, _ int32) ([]gen.SelectPopularHashtagsInRangeRow, error) {
	return nil, nil
}
