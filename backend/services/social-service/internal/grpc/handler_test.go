package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ==================== mock types ====================

type mockBuyerClientH struct {
	getBuyerProfileFn   func(ctx context.Context, buyerID int64) (*usecase.AccountInfo, error)
	getBuyerMetaBatchFn func(ctx context.Context, buyerIDs []int64) (map[int64]*usecase.AccountInfo, error)
	resolvePrincipalFn  func(ctx context.Context, buyerID int64) int64
}

func (m *mockBuyerClientH) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	if m.resolvePrincipalFn != nil {
		return m.resolvePrincipalFn(ctx, buyerID)
	}
	return buyerID
}

func (m *mockBuyerClientH) GetBuyerProfile(ctx context.Context, buyerID int64) (*usecase.AccountInfo, error) {
	if m.getBuyerProfileFn != nil {
		return m.getBuyerProfileFn(ctx, buyerID)
	}
	return &usecase.AccountInfo{}, nil
}

func (m *mockBuyerClientH) GetBuyerMetaBatch(ctx context.Context, buyerIDs []int64) (map[int64]*usecase.AccountInfo, error) {
	if m.getBuyerMetaBatchFn != nil {
		return m.getBuyerMetaBatchFn(ctx, buyerIDs)
	}
	return make(map[int64]*usecase.AccountInfo), nil
}

func (m *mockBuyerClientH) GetSubscriberIDs(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}

// ResolvePrincipalStrict is not exercised by this package's handler tests; it
// exists only to satisfy usecase.BuyerClient after the 2026-08-05 fail-closed
// ban-path change added the method to the interface. Mirrors resolvePrincipalFn:
// resolved (differs from input) -> Found=true, otherwise -> zero value.
func (m *mockBuyerClientH) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (usecase.PrincipalResolution, error) {
	if m.resolvePrincipalFn != nil {
		if resolved := m.resolvePrincipalFn(ctx, buyerID); resolved != buyerID {
			return usecase.PrincipalResolution{AccountID: resolved, Found: true}, nil
		}
	}
	return usecase.PrincipalResolution{}, nil
}

type mockSellerClientH struct {
	getSellerInfoFn      func(ctx context.Context, storeID int64) (*usecase.AccountInfo, error)
	getSellerMetaBatchFn func(ctx context.Context, storeIDs []int64) (map[int64]*usecase.AccountInfo, error)
	resolvePrincipalFn   func(ctx context.Context, storeID int64) int64
}

func (m *mockSellerClientH) GetSellerInfo(ctx context.Context, storeID int64) (*usecase.AccountInfo, error) {
	if m.getSellerInfoFn != nil {
		return m.getSellerInfoFn(ctx, storeID)
	}
	return &usecase.AccountInfo{}, nil
}

func (m *mockSellerClientH) GetSellerMetaBatch(ctx context.Context, storeIDs []int64) (map[int64]*usecase.AccountInfo, error) {
	if m.getSellerMetaBatchFn != nil {
		return m.getSellerMetaBatchFn(ctx, storeIDs)
	}
	return make(map[int64]*usecase.AccountInfo), nil
}

func (m *mockSellerClientH) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	if m.resolvePrincipalFn != nil {
		return m.resolvePrincipalFn(ctx, storeID)
	}
	return storeID
}

// ResolvePrincipalStrict is not exercised by this package's handler tests; it
// exists only to satisfy usecase.SellerClient after the 2026-08-05 fail-closed
// ban-path change added the method to the interface. Mirrors resolvePrincipalFn:
// resolved (differs from input) -> Found=true, otherwise -> zero value.
func (m *mockSellerClientH) ResolvePrincipalStrict(ctx context.Context, storeID int64) (usecase.PrincipalResolution, error) {
	if m.resolvePrincipalFn != nil {
		if resolved := m.resolvePrincipalFn(ctx, storeID); resolved != storeID {
			return usecase.PrincipalResolution{AccountID: resolved, Found: true}, nil
		}
	}
	return usecase.PrincipalResolution{}, nil
}

// ==================== helper ====================

func makeComment(id, postID, userID int64) *gen.CmsComment {
	return &gen.CmsComment{
		ID:         id,
		PostID:     postID,
		UserID:     userID,
		Content:    stringPointer("test"),
		CreateTime: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func newTestGRPCServer(t *testing.T, srv *ContentServer) (pb.SocialServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := grpc.NewServer()
	pb.RegisterSocialServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	return pb.NewSocialServiceClient(conn), func() { _ = conn.Close(); grpcSrv.Stop() }
}

// ==================== helper ====================

func commentInfos(rows []*gen.CmsComment) []*pb.CommentInfo {
	out := make([]*pb.CommentInfo, len(rows))
	for i, r := range rows {
		out[i] = toCommentInfo(r, usecase.Caller{}, false, nil)
	}
	return out
}

func makeCommentWithRole(id, postID, userID int64, role int16) *gen.CmsComment {
	c := makeComment(id, postID, userID)
	c.Role = role
	return c
}

// ==================== tests ====================

func TestEnrichCommentUserInfo_BuyerRole(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				100: {Nickname: "Alice", Avatar: "alice.jpg"},
				200: {Nickname: "Bob", Avatar: "bob.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: &mockSellerClientH{},
		logger:       zap.NewNop(),
	}
	rows := []*gen.CmsComment{
		makeCommentWithRole(1, 10, 100, 1),
		makeCommentWithRole(2, 10, 200, 1),
	}

	srv.enrichCommentUserInfo(context.Background(), rows)
	data := commentInfos(rows)

	if data[0].UserName != "Alice" {
		t.Errorf("UserName = %q, want %q", data[0].UserName, "Alice")
	}
	if data[0].UserAvatarKey != "alice.jpg" {
		t.Errorf("UserAvatar = %q, want %q", data[0].UserAvatarKey, "alice.jpg")
	}
	if data[1].UserName != "Bob" {
		t.Errorf("UserName = %q, want %q", data[1].UserName, "Bob")
	}
}

func TestEnrichCommentUserInfo_SellerRole(t *testing.T) {
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				300: {Nickname: "我的店", Avatar: "shop.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  &mockBuyerClientH{},
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}
	rows := []*gen.CmsComment{
		makeCommentWithRole(1, 10, 300, 2),
	}

	srv.enrichCommentUserInfo(context.Background(), rows)
	data := commentInfos(rows)

	if data[0].UserName != "我的店" {
		t.Errorf("UserName = %q, want %q", data[0].UserName, "我的店")
	}
	if data[0].UserAvatarKey != "shop.jpg" {
		t.Errorf("UserAvatar = %q, want %q", data[0].UserAvatarKey, "shop.jpg")
	}
}

func TestEnrichCommentUserInfo_MixedRoles(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				100: {Nickname: "BuyerAlice", Avatar: "buyer_alice.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				200: {Nickname: "SellerShop", Avatar: "shop.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}
	rows := []*gen.CmsComment{
		makeCommentWithRole(1, 10, 100, 1),
		makeCommentWithRole(2, 10, 200, 2),
	}

	srv.enrichCommentUserInfo(context.Background(), rows)
	data := commentInfos(rows)

	if data[0].UserName != "BuyerAlice" {
		t.Errorf("buyer UserName = %q, want %q", data[0].UserName, "BuyerAlice")
	}
	if data[1].UserName != "SellerShop" {
		t.Errorf("seller UserName = %q, want %q", data[1].UserName, "SellerShop")
	}
}

func TestEnrichCommentUserInfo_NestedReplies(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				100: {Nickname: "Buyer", Avatar: "buyer.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				200: {Nickname: "Seller", Avatar: "seller.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}

	replyRow := makeCommentWithRole(2, 10, 200, 2)
	parentRow := makeCommentWithRole(1, 10, 100, 1)

	srv.enrichCommentUserInfo(context.Background(), []*gen.CmsComment{parentRow, replyRow})
	reply := toCommentInfo(replyRow, usecase.Caller{}, false, nil)
	data := []*pb.CommentInfo{toCommentInfo(parentRow, usecase.Caller{}, false, []*pb.CommentInfo{reply})}

	if data[0].UserName != "Buyer" {
		t.Errorf("parent UserName = %q, want %q", data[0].UserName, "Buyer")
	}
	if data[0].Replies[0].UserName != "Seller" {
		t.Errorf("reply UserName = %q, want %q", data[0].Replies[0].UserName, "Seller")
	}
	if data[0].Replies[0].UserAvatarKey != "seller.jpg" {
		t.Errorf("reply UserAvatar = %q, want %q", data[0].Replies[0].UserAvatarKey, "seller.jpg")
	}
}

func TestEnrichCommentUserInfo_BuyerServiceDown_Fallback(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return make(map[int64]*usecase.AccountInfo), nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: &mockSellerClientH{},
		logger:       zap.NewNop(),
	}

	c := makeCommentWithRole(1, 10, 100, 1)
	c.UserName = stringPointer("DBName")
	c.UserAvatar = stringPointer("db_avatar.jpg")
	rows := []*gen.CmsComment{c}

	srv.enrichCommentUserInfo(context.Background(), rows)
	data := commentInfos(rows)

	if data[0].UserName != "DBName" {
		t.Errorf("UserName = %q, want %q (DB fallback)", data[0].UserName, "DBName")
	}
	if data[0].UserAvatarKey != "db_avatar.jpg" {
		t.Errorf("UserAvatar = %q, want %q (DB fallback)", data[0].UserAvatarKey, "db_avatar.jpg")
	}
}

func TestEnrichCommentUserInfo_SellerServiceDown_Fallback(t *testing.T) {
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return make(map[int64]*usecase.AccountInfo), nil
		},
	}
	srv := &ContentServer{
		buyerClient:  &mockBuyerClientH{},
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}

	c := makeCommentWithRole(1, 10, 300, 2)
	c.UserName = stringPointer("DBSeller")
	rows := []*gen.CmsComment{c}

	srv.enrichCommentUserInfo(context.Background(), rows)
	data := commentInfos(rows)

	if data[0].UserName != "DBSeller" {
		t.Errorf("UserName = %q, want %q (DB fallback)", data[0].UserName, "DBSeller")
	}
}

func TestGetCommentList_BufConn_ReturnsRoleAndEnrichedFields(t *testing.T) {
	commentUC := usecase.NewCommentUseCase(
		&stubCommentRepo{
			comments: []*gen.CmsComment{
				makeCommentWithRole(1, 10, 100, 1),
				makeCommentWithRole(2, 10, 200, 2),
			},
			total: 2,
		},
		&stubPostRepo{},
		&stubBuyerClient{},
		&stubSellerClient{},
		nil,
		func() int64 { return 99999 },
	)
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				100: {Nickname: "User100", Avatar: "u100.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				200: {Nickname: "Shop200", Avatar: "s200.jpg"},
			}, nil
		},
	}
	srv := NewContentServer(nil, nil, commentUC, nil, nil, nil, buyerClient, sellerClient, nil, zap.NewNop())
	client, cleanup := newTestGRPCServer(t, srv)
	defer cleanup()

	resp, err := client.GetCommentList(context.Background(), &pb.GetCommentListRequest{
		// 真實 18 位 TSID（> JS 2^53）：proto §4 已將前端可控 post_id 改 string，
		// 用大值鎖住 handler parseInt64 不丟精度的回歸（stub repo 不依賴此值）。
		PostId:   "857045288972403572",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("GetCommentList: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].UserName != "User100" {
		t.Errorf("Data[0].UserName = %q, want %q", resp.Data[0].UserName, "User100")
	}
	if resp.Data[0].ActorKind != 1 {
		t.Errorf("Data[0].Role = %d, want 1", resp.Data[0].ActorKind)
	}
	if resp.Data[1].UserName != "Shop200" {
		t.Errorf("Data[1].UserName = %q, want %q", resp.Data[1].UserName, "Shop200")
	}
	if resp.Data[1].UserAvatarKey != "s200.jpg" {
		t.Errorf("Data[1].UserAvatarKey = %q, want %q", resp.Data[1].UserAvatarKey, "s200.jpg")
	}
	if resp.Data[1].ActorKind != 2 {
		t.Errorf("Data[1].Role = %d, want 2", resp.Data[1].ActorKind)
	}
}

// ==================== stub types for bufconn test ====================

type stubCommentRepo struct {
	comments []*gen.CmsComment
	total    int64
}

func (s *stubCommentRepo) GetByID(_ context.Context, _ int64) (*gen.CmsComment, error) {
	return nil, usecase.ErrNotFound
}
func (s *stubCommentRepo) ListTopLevel(_ context.Context, _, _, _ int64) ([]*gen.CmsComment, error) {
	return s.comments, nil
}
func (s *stubCommentRepo) CountTopLevel(_ context.Context, _ int64) (int64, error) {
	return s.total, nil
}
func (s *stubCommentRepo) ListReplies(_ context.Context, _, _, _ int64) ([]*gen.CmsComment, error) {
	return nil, nil
}
func (s *stubCommentRepo) CountReplies(_ context.Context, _ int64) (int64, error) { return 0, nil }
func (s *stubCommentRepo) GetLatestReplies(_ context.Context, _, _ int64) ([]*gen.CmsComment, error) {
	return nil, nil
}
func (s *stubCommentRepo) BatchGetLatestReplies(_ context.Context, _ []int64, _ int32) ([]*gen.CmsComment, error) {
	return nil, nil
}
func (s *stubCommentRepo) Create(_ context.Context, _, _, _ int64, _, _, _ string, _, _ int64, _ string, _ int16) (*gen.CmsComment, error) {
	return nil, nil
}
func (s *stubCommentRepo) UpdateContent(_ context.Context, _ int64, _ string) (*gen.CmsComment, error) {
	return nil, nil
}
func (s *stubCommentRepo) SoftDelete(_ context.Context, _ int64) error            { return nil }
func (s *stubCommentRepo) HideComment(_ context.Context, _ int64, _ string) error { return nil }
func (s *stubCommentRepo) IncrReplyCount(_ context.Context, _ int64) error        { return nil }
func (s *stubCommentRepo) DecrReplyCount(_ context.Context, _ int64) error        { return nil }
func (s *stubCommentRepo) IncrLikeCount(_ context.Context, _ int64) error         { return nil }
func (s *stubCommentRepo) DecrLikeCount(_ context.Context, _ int64) error         { return nil }
func (s *stubCommentRepo) GetLike(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsCommentLike, error) {
	return nil, usecase.ErrNotFound
}
func (s *stubCommentRepo) CreateLike(_ context.Context, _ int64, _ int16, _ int64) error { return nil }
func (s *stubCommentRepo) DeleteLike(_ context.Context, _ int64, _ int16, _ int64) error { return nil }
func (s *stubCommentRepo) BatchGetLikedCommentIDs(_ context.Context, _ []int64, _ int16, _ int64) ([]int64, error) {
	return nil, nil
}
func (s *stubCommentRepo) SoftDeleteByActor(_ context.Context, _ int16, _ int64) error  { return nil }
func (s *stubCommentRepo) DeleteLikesByActor(_ context.Context, _ int16, _ int64) error { return nil }
func (s *stubCommentRepo) ReconcileLikeCounts(_ context.Context) error                  { return nil }
func (s *stubCommentRepo) ReconcileReplyCounts(_ context.Context) error                 { return nil }

// stubPostRepo answers not-found by default. The optional fields exist for the
// delegation tests: `post` makes GetByID succeed, and the two `last*` fields
// record what a write was handed so a test can tell "the value reached the
// repository" from "the value was dropped and the write happened anyway".
type stubPostRepo struct {
	post *gen.CmsPost
	// myPosts is what ListMy answers; nil keeps the empty-list default every other
	// test relies on. It exists so a test can assert what the LIST mapper puts on
	// the wire — GetByID cannot stand in for it, the two build their items from
	// different structs (PostDetailInfo vs PostListItem).
	myPosts             []*gen.CmsPost
	updateErr           error
	lastPublishStatus   int16
	lastExpectedVersion *int64
	// lastScheduledAt / lastScheduledBySessionID record what a schedule write was
	// handed; nil means the immediate path ran instead.
	lastScheduledAt          *time.Time
	lastScheduledBySessionID int64
	// viewIncrements counts IncrViewCount calls, so a test can assert a read did
	// or did not count as a view.
	viewIncrements int
}

func (s *stubPostRepo) GetByID(_ context.Context, _ int64) (*gen.CmsPost, error) {
	if s.post != nil {
		return s.post, nil
	}
	return nil, usecase.ErrNotFound
}

// GetByIDTx (FIX 2) mirrors GetByID: this stub has no real tx handle to bind
// to, and every test that exercises UpdatePost's hashtag path sets `post` up
// front, which is exactly what the tx-scoped re-read is meant to see here.
func (s *stubPostRepo) GetByIDTx(_ context.Context, _ gen.DBTX, _ int64) (*gen.CmsPost, error) {
	if s.post != nil {
		return s.post, nil
	}
	return nil, usecase.ErrNotFound
}
func (s *stubPostRepo) List(_ context.Context, _, _, _ int64) ([]*gen.CmsPost, error) {
	return nil, nil
}
func (s *stubPostRepo) Count(_ context.Context, _ int64) (int64, error) { return 0, nil }
func (s *stubPostRepo) Create(ctx context.Context, id, sellerID int64, content, _ string, _ *int64, _ *int64,
	publishStatus int16, sameTx func(ctx context.Context, q gen.DBTX) error) (*gen.CmsPost, error) {
	s.lastPublishStatus = publishStatus
	// Mirrors the real repository: sameTx runs inside the winning transaction, so
	// a stub that skipped it would hide a hashtag write that never happens.
	if sameTx != nil {
		if err := sameTx(ctx, nil); err != nil {
			return nil, err
		}
	}
	return &gen.CmsPost{
		ID:            id,
		SellerID:      sellerID,
		Content:       stringPointer(content),
		PublishStatus: publishStatus,
	}, nil
}
func (s *stubPostRepo) Update(ctx context.Context, _ int64, _, _ string, _ *int64,
	_ *int64, expectedVersion *int64, sameTx func(ctx context.Context, q gen.DBTX) error) error {
	s.lastExpectedVersion = expectedVersion
	if s.updateErr != nil {
		return s.updateErr
	}
	// Mirrors the real repository: sameTx runs inside the winning transaction, so
	// a stub that skipped it would hide a hashtag write that never happens.
	if sameTx != nil {
		return sameTx(ctx, nil)
	}
	return nil
}
func (s *stubPostRepo) SetPublishStatus(ctx context.Context, _ int64, publishStatus int16, _ time.Time,
	expectedVersion *int64, sameTx func(ctx context.Context, q gen.DBTX) error) error {
	s.lastPublishStatus = publishStatus
	s.lastExpectedVersion = expectedVersion
	if s.updateErr != nil {
		return s.updateErr
	}
	// Mirrors the real repository: sameTx runs inside the winning transaction, so
	// a stub that skipped it would hide a counter adjustment that never happens.
	if sameTx != nil {
		return sameTx(ctx, nil)
	}
	return nil
}

func (s *stubPostRepo) SchedulePublish(_ context.Context, _ int64, scheduledAt time.Time, sessionID int64,
	_ time.Time, expectedVersion *int64) error {
	s.lastScheduledAt = &scheduledAt
	s.lastScheduledBySessionID = sessionID
	s.lastExpectedVersion = expectedVersion
	return s.updateErr
}

func (s *stubPostRepo) SoftDelete(_ context.Context, _, _ int64) (int64, error) { return 1, nil }
func (s *stubPostRepo) IncrViewCount(_ context.Context, _ int64) error {
	s.viewIncrements++
	return nil
}
func (s *stubPostRepo) IncrShareCount(_ context.Context, _ int64) error   { return nil }
func (s *stubPostRepo) IncrLikeCount(_ context.Context, _ int64) error    { return nil }
func (s *stubPostRepo) DecrLikeCount(_ context.Context, _ int64) error    { return nil }
func (s *stubPostRepo) IncrCommentCount(_ context.Context, _ int64) error { return nil }
func (s *stubPostRepo) DecrCommentCount(_ context.Context, _ int64) error { return nil }
func (s *stubPostRepo) GetLike(_ context.Context, _ int64, _ int16, _ int64) (*gen.CmsPostLike, error) {
	return nil, usecase.ErrNotFound
}
func (s *stubPostRepo) CreateLike(_ context.Context, _ int64, _ int16, _ int64) error { return nil }
func (s *stubPostRepo) DeleteLike(_ context.Context, _ int64, _ int16, _ int64) error { return nil }
func (s *stubPostRepo) BatchGetLikedPostIDs(_ context.Context, _ []int64, _ int16, _ int64) ([]int64, error) {
	return nil, nil
}
func (s *stubPostRepo) ListMy(_ context.Context, _ int64, _ string, _, _ int64) ([]*gen.CmsPost, error) {
	return s.myPosts, nil
}
func (s *stubPostRepo) CountMy(_ context.Context, _ int64, _ string) (int64, error) {
	return int64(len(s.myPosts)), nil
}
func (s *stubPostRepo) AdminList(_ context.Context, _ string, _, _ int64) ([]gen.AdminListPostsRow, error) {
	return nil, nil
}
func (s *stubPostRepo) AdminCount(_ context.Context, _ string) (int64, error) { return 0, nil }
func (s *stubPostRepo) HidePost(_ context.Context, _ int64, _ string) error   { return nil }
func (s *stubPostRepo) RestorePost(_ context.Context, _ int64) error          { return nil }
func (s *stubPostRepo) SoftDeleteBySellerID(_ context.Context, _ int64) error { return nil }
func (s *stubPostRepo) ListPostIDsBySellerID(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}
func (s *stubPostRepo) DeleteLikesByActor(_ context.Context, _ int16, _ int64) error { return nil }
func (s *stubPostRepo) ReconcileLikeCounts(_ context.Context) error                  { return nil }
func (s *stubPostRepo) ReconcileCommentCounts(_ context.Context) error               { return nil }
func (s *stubPostRepo) ListByHashtag(_ context.Context, _ string, _, _ int64) ([]*gen.CmsPost, error) {
	return nil, nil
}
func (s *stubPostRepo) CountByHashtag(_ context.Context, _ string) (int64, error) { return 0, nil }

type stubBuyerClient struct{}

func (s *stubBuyerClient) GetBuyerProfile(_ context.Context, _ int64) (*usecase.AccountInfo, error) {
	return &usecase.AccountInfo{}, nil
}

func (s *stubBuyerClient) GetBuyerMetaBatch(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
	return make(map[int64]*usecase.AccountInfo), nil
}

func (s *stubBuyerClient) GetSubscriberIDs(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}

func (s *stubBuyerClient) ResolvePrincipal(_ context.Context, buyerID int64) int64 {
	return buyerID
}

// ResolvePrincipalStrict is not exercised here; unconfigurable stub always
// reports "not mine" (Found=false), mirroring ResolvePrincipal's unconditional
// passthrough above.
func (s *stubBuyerClient) ResolvePrincipalStrict(_ context.Context, _ int64) (usecase.PrincipalResolution, error) {
	return usecase.PrincipalResolution{}, nil
}

type stubSellerClient struct{}

func (s *stubSellerClient) GetSellerInfo(_ context.Context, _ int64) (*usecase.AccountInfo, error) {
	return &usecase.AccountInfo{}, nil
}

func (s *stubSellerClient) GetSellerMetaBatch(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
	return make(map[int64]*usecase.AccountInfo), nil
}

func (s *stubSellerClient) ResolvePrincipal(_ context.Context, storeID int64) int64 {
	return storeID
}

// ResolvePrincipalStrict is not exercised here; unconfigurable stub always
// reports "not mine" (Found=false), mirroring ResolvePrincipal's unconditional
// passthrough above.
func (s *stubSellerClient) ResolvePrincipalStrict(_ context.Context, _ int64) (usecase.PrincipalResolution, error) {
	return usecase.PrincipalResolution{}, nil
}

// TestParseInt64_TSIDPrecision 鎖住 proto §4 前端可控 id 改 string 後，handler 邊界
// parseInt64 對 18-19 位 TSID（> JS 2^53）的解析不丟精度，且空字串 / 非數字回 0
// （usecase 視 0 為查無此 id / 未指定）。
func TestParseInt64_TSIDPrecision(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"857045288972403572", 857045288972403572}, // 18 位 TSID
		{"9999999999999999999", 0},                 // 溢位 int64 → 0
		{"", 0},                                    // 空 → 0（未指定）
		{"abc", 0},                                 // 非數字 → 0（查無）
		{"0", 0},
	}
	for _, c := range cases {
		if got := parseInt64(c.in); got != c.want {
			t.Errorf("parseInt64(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// fakeReportRepoRateLimited implements domain.ReportRepository, returning a
// CountByReporterSince high enough to trip SubmitReport's rate limit. Other
// methods are never reached on the rate-limited path and are left as the
// embedded nil interface's zero behavior (would panic if called — that's
// intentional, it flags a test setup bug rather than masking one).
type fakeReportRepoRateLimited struct {
	domain.ReportRepository
	countSince int64
}

func (f *fakeReportRepoRateLimited) GetByReporterAndTarget(_ context.Context, _ int64, _ string, _ int64) (*gen.Report, error) {
	return nil, nil
}

func (f *fakeReportRepoRateLimited) CountByReporterSince(_ context.Context, _ int64, _ time.Time) (int64, error) {
	return f.countSince, nil
}

// TestSubmitReport_RateLimited_MapsToResourceExhausted guards the gap where
// usecase.ErrReportRateLimited fell through SubmitReport's error switch to
// the generic default branch (codes.Internal + an error-level log on every
// rate-limit hit, a false alarm for expected user behavior). It must now map
// to codes.ResourceExhausted, the standard gRPC code for rate limiting.
func TestSubmitReport_RateLimited_MapsToResourceExhausted(t *testing.T) {
	reportUC := usecase.NewReportUseCase(&fakeReportRepoRateLimited{countSince: 20}, nil, nil, nil, nil, func() int64 { return 1 })
	srv := &ContentServer{reportUC: reportUC, logger: zap.NewNop()}

	_, err := srv.SubmitReport(context.Background(), &pb.SubmitReportRequest{
		ReporterId: 1, TargetType: "POST", TargetId: "100", ReasonCode: "SPAM",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.ResourceExhausted {
		t.Errorf("status code = %v, want ResourceExhausted", got)
	}
}

// TestEnrichCommentUserInfo_NamespacesDoNotLeak 鎖住「buyer_id 與 store_id 是兩個
// 獨立命名空間」這個不變式（44 號設計書）。
//
// 舊實作把兩次批次查詢的結果合併進單一 map[int64]，再以 uid 單鍵查表——同一批留言裡
// 只要有一則賣家留言的 store_id 恰好等於某則買家留言的 buyer_id，後寫入的那一邊就會
// 蓋掉另一邊，把店家名稱與 logo 套到買家頭上（或反之）。
//
// 這正是 #2223 修復時被否決的「用 store_id 判定身分」方案的同一個坑：數值相等不代表
// 同一個實體。此處用兩邊都回傳 id=100 但內容不同來製造碰撞。
func TestEnrichCommentUserInfo_NamespacesDoNotLeak(t *testing.T) {
	const collidingID = 100

	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				collidingID: {Nickname: "買家小明", Avatar: "buyer.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				collidingID: {Nickname: "範例甲商行", Avatar: "seller.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}

	// 兩則留言 user_id 同為 100，但一則是買家、一則是賣家。
	rows := []*gen.CmsComment{
		makeCommentWithRole(1, 10, collidingID, 1), // role=1 買家
		makeCommentWithRole(2, 10, collidingID, 2), // role=2 賣家
	}

	srv.enrichCommentUserInfo(context.Background(), rows)
	data := commentInfos(rows)

	if data[0].UserName != "買家小明" || data[0].UserAvatarKey != "buyer.jpg" {
		t.Errorf("買家列被賣家資料污染：UserName=%q Avatar=%q，want 買家小明 / buyer.jpg",
			data[0].UserName, data[0].UserAvatarKey)
	}
	if data[1].UserName != "範例甲商行" || data[1].UserAvatarKey != "seller.jpg" {
		t.Errorf("賣家列被買家資料污染：UserName=%q Avatar=%q，want 範例甲商行 / seller.jpg",
			data[1].UserName, data[1].UserAvatarKey)
	}
}

// TestEnrichCommentUserInfo_UnresolvedRoleKeepsSnapshot：查不到就保留 DB 快照值，
// 不可因為「另一個命名空間剛好有這個 id」而套上別人的身分。
func TestEnrichCommentUserInfo_UnresolvedRoleKeepsSnapshot(t *testing.T) {
	srv := &ContentServer{
		buyerClient: &mockBuyerClientH{}, // 回空 map：買家查無此人
		sellerClient: &mockSellerClientH{
			getSellerMetaBatchFn: func(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
				return map[int64]*usecase.AccountInfo{777: {Nickname: "某商行", Avatar: "s.jpg"}}, nil
			},
		},
		logger: zap.NewNop(),
	}
	// role=1（買家）但 id 777 只在賣家那邊查得到 —— 不得套用。
	buyerRow := makeCommentWithRole(1, 10, 777, 1)
	sellerRow := makeCommentWithRole(2, 10, 777, 2)
	before := optionalString(buyerRow.UserName)

	srv.enrichCommentUserInfo(context.Background(), []*gen.CmsComment{buyerRow, sellerRow})

	if got := optionalString(buyerRow.UserName); got != before {
		t.Errorf("買家列查無資料卻被覆寫成 %q（原 %q）", got, before)
	}
	if got := optionalString(sellerRow.UserName); got != "某商行" {
		t.Errorf("賣家列 UserName = %q, want 某商行", got)
	}
}
