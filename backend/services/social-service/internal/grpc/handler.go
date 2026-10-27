package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/delegation"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/domain/hashtag"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// sellerSuspendedFromContext reads the forwarded suspension flag from incoming
// gRPC metadata (46 號 §4.4 self-vs-other 細分). Fail-open at the transport
// read; the actual self-vs-other gating decision happens in the usecase
// layer. Propagation chain and metadata key documented on the shared helper:
// pkg/middleware/grpc/interceptor.SellerSuspendedFromContext.
func sellerSuspendedFromContext(ctx context.Context) bool {
	_, suspended := interceptor.SellerSuspendedFromContext(ctx)
	return suspended
}

type ContentServer struct {
	pb.UnimplementedSocialServiceServer
	postUC       *usecase.PostUseCase
	hashtagUC    *usecase.HashtagUseCase
	commentUC    *usecase.CommentUseCase
	forumUC      *usecase.ForumUseCase
	blockUC      *usecase.BlockUseCase
	reportUC     *usecase.ReportUseCase
	buyerClient  usecase.BuyerClient
	sellerClient usecase.SellerClient
	rdb          *redis.Client
	logger       *zap.Logger
}

func NewContentServer(
	postUC *usecase.PostUseCase,
	hashtagUC *usecase.HashtagUseCase,
	commentUC *usecase.CommentUseCase,
	forumUC *usecase.ForumUseCase,
	blockUC *usecase.BlockUseCase,
	reportUC *usecase.ReportUseCase,
	buyerClient usecase.BuyerClient,
	sellerClient usecase.SellerClient,
	rdb *redis.Client,
	logger *zap.Logger,
) *ContentServer {
	return &ContentServer{
		postUC:       postUC,
		hashtagUC:    hashtagUC,
		commentUC:    commentUC,
		forumUC:      forumUC,
		blockUC:      blockUC,
		reportUC:     reportUC,
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		rdb:          rdb,
		logger:       logger,
	}
}

// hashtagErrCode returns the 3015-3018 envelope code for a hashtag domain
// error, or 0 if the error isn't one of them. Caller uses 0 as "not a
// hashtag error — fall through to default handling". B-153.
func hashtagErrCode(err error) int32 {
	switch {
	case errors.Is(err, hashtag.ErrInvalidChar):
		return hashtag.CodeInvalidChar
	case errors.Is(err, hashtag.ErrLengthExceeded):
		return hashtag.CodeLengthExceeded
	case errors.Is(err, hashtag.ErrCountExceeded):
		return hashtag.CodeCountExceeded
	case errors.Is(err, hashtag.ErrEmptyAfterNormalize):
		return hashtag.CodeEmptyAfterNormalize
	}
	return 0
}

// ==================== Post RPCs ====================

func (s *ContentServer) GetPostList(ctx context.Context, req *pb.GetPostListRequest) (*pb.PostListResponse, error) {
	var (
		posts []*gen.CmsPost
		total int64
		err   error
	)
	if req.Tag != "" {
		// B-153 §5.3: hashtag-filtered list (server normalizes tag input).
		posts, total, err = s.postUC.ListPostsByTag(ctx, req.Tag, req.Page, req.PageSize)
	} else {
		posts, total, err = s.postUC.ListPosts(ctx, parseInt64(req.SellerId), req.Page, req.PageSize)
	}
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	// B-135: filter posts whose author is mutually blocked with the viewer.
	posts = filterPostsByBlocklist(ctx, s.rdb, req.BuyerId, posts)

	productMap := s.fetchProductMap(ctx, posts)
	sellerMap := s.fetchSellerMap(ctx, posts)

	// Batch fetch liked status
	postIDs := make([]int64, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}
	likedSet := s.postUC.BatchIsLiked(ctx, postIDs, req.BuyerId)

	items := make([]*pb.PostListItem, 0, len(posts))
	for _, p := range posts {
		sellerName, sellerLogo := "", ""
		if info, ok := sellerMap[p.SellerID]; ok {
			sellerName = info[0]
			sellerLogo = info[1]
		}
		item := &pb.PostListItem{
			Id:            strconv.FormatInt(p.ID, 10),
			ImageKey:      optionalString(p.Image),
			SellerId:      strconv.FormatInt(p.SellerID, 10),
			SellerName:    sellerName,
			SellerLogoKey: sellerLogo,
			Content:       optionalString(p.Content),
			LikeCount:     p.LikeCount,
			CommentCount:  p.CommentCount,
			ViewCount:     p.ViewCount,
			ShareCount:    p.ShareCount,
			IsLiked:       likedSet[p.ID],
			CreatedAt:     p.CreateTime.Time.Unix(),
			Product:       postProductFromPointer(p.ProductID, productMap),
		}
		items = append(items, item)
	}

	// B-153 §5.3 / §9.5: fill tags for all posts in 2 SQL (no N+1).
	// FillTags failure is best-effort — log + continue with empty tags
	// rather than 500 the entire list.
	s.fillPostListItemTags(ctx, posts, items)

	return &pb.PostListResponse{
		Code:     0,
		Data:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// fillPostListItemTags is a small helper used by every []*pb.PostListItem
// caller (GetPostList / ListMyPosts) to attach the
// canonical hashtag names per post. items and posts must be index-aligned.
// On FillTags error we log and leave Tags empty (best-effort per spec §9.5).
func (s *ContentServer) fillPostListItemTags(ctx context.Context, posts []*gen.CmsPost, items []*pb.PostListItem) {
	if len(posts) == 0 || len(items) == 0 {
		return
	}
	postIDs := make([]int64, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}
	tagsByPost, err := s.postUC.FillTags(ctx, postIDs)
	if err != nil {
		s.logger.Warn("FillTags failed in list response", zap.Error(err))
		return
	}
	for i, p := range posts {
		if tags, ok := tagsByPost[p.ID]; ok {
			items[i].Tags = tags
		}
	}
}

func (s *ContentServer) GetPostDetail(ctx context.Context, req *pb.GetPostDetailRequest) (*pb.PostDetailResponse, error) {
	// S1a–S4 run before the read: nothing about the post can make a malformed
	// delegation context legal, and validating first keeps a rejected caller from
	// learning whether the post exists.
	delegated := req.GetDelegation()
	if err := delegation.Validate(ctx, delegated, req.OwnerStoreId, delegation.CapPostsRead); err != nil {
		return nil, err
	}

	postID := parseInt64(req.PostId)
	post, err := s.postUC.GetPost(ctx, postID)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostDetailResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	// Three read branches, decided once here; each owns its own ownership check,
	// view_count and hidden-field disclosure so a condition on one cannot leak
	// into another.
	//
	// Delegated (Q4, §7.1.2): GetPost deliberately has no seller filter in SQL —
	// comment, report dispatch, hide and detail all use it as an existence check,
	// and filtering there would put drafts out of governance's reach — so the
	// ownership assertion has to happen here. Without it a delegated reader
	// retrieves any seller's post detail, and because a successful read writes no
	// audit row it leaves no trace at all. Published posts are included: what
	// bounds a delegated session is the target store, not public visibility.
	// A post that DOES belong to the target is visible in every publish_status —
	// draft 4401 and unpublished 4403 as well — because the delegated workspace is
	// the seller's own view of their own content.
	//
	// Owner (no delegation, owner_store_id set): the store reads its own post in
	// every publish_status and hidden state; a post of another store answers
	// NotFound, the same as a missing one, so existence is not disclosed. An owner
	// read is not a view, so view_count is not incremented.
	//
	// Public (everything else): buyer and anonymous paths.
	//
	// hidden_at / hidden_reason describe the post itself and are filled on every
	// branch that returns the row for the seller's own view; whether a hidden post
	// is returned at all is what controls visibility.
	var fillHidden bool
	switch {
	case delegated != nil:
		if post.SellerID != req.OwnerStoreId {
			return &pb.PostDetailResponse{
				Code:    int32(pkgerrors.GetCode(domain.ErrResourceNotOwned)),
				Message: domain.ErrResourceNotOwned.Error(),
			}, nil
		}
		_ = s.postUC.IncrViewCount(ctx, postID)
		fillHidden = true
	case req.OwnerStoreId != 0:
		if post.SellerID != req.OwnerStoreId {
			return &pb.PostDetailResponse{
				Code:    int32(pkgerrors.GetCode(usecase.ErrNotFound)),
				Message: usecase.ErrNotFound.Error(),
			}, nil
		}
		fillHidden = true
	default:
		_ = s.postUC.IncrViewCount(ctx, postID)
	}

	// 走 resolveSellerDisplay 而非 GetSellerInfo：後者帶公開可見性過濾，賣家軟刪後回空白。
	// 貼文詳情是唯一沒有 gateway enrichPostsWithSellerInfo 兜底的路徑（分享卡片 OG 標題也吃這裡）。
	var sellerName, sellerLogo string
	if pair, ok := s.resolveSellerDisplay(ctx, []int64{post.SellerID})[post.SellerID]; ok {
		sellerName, sellerLogo = pair[0], pair[1]
	}

	// Enrich product data
	var productMap map[string]*usecase.ProductBasicInfo
	if post.ProductID != nil {
		pid := fmt.Sprintf("%d", *post.ProductID)
		productMap = s.postUC.GetProductsByIDs(ctx, []string{pid})
	}

	detail := toPostDetailInfo(post, req.BuyerId, s.postUC.IsLiked(ctx, post.ID, req.BuyerId), sellerName, sellerLogo, productMap)
	if fillHidden {
		detail.HiddenAt = formatOptionalISO8601(post.HiddenAt)
		detail.HiddenReason = optionalString(post.HiddenReason)
	}
	// B-153 §5.4 / §9.5: best-effort tag fill — log + continue on error.
	if tagsByPost, terr := s.postUC.FillTags(ctx, []int64{post.ID}); terr != nil {
		s.logger.Warn("FillTags failed in GetPostDetail", zap.Error(terr))
	} else if tags, ok := tagsByPost[post.ID]; ok {
		detail.Tags = tags
	}
	return &pb.PostDetailResponse{Code: 0, Data: detail}, nil
}

func (s *ContentServer) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.PostResponse, error) {
	// S5 applies here: CreatePostRequest carries its own idempotency_key, so it
	// must agree with the delegation context's. Two differing keys is never a
	// case where one of them can be picked.
	delegated := req.GetDelegation()
	if err := delegation.Validate(ctx, delegated, req.SellerId, delegation.CapPostsWrite,
		delegation.WithRequestIdempotencyKey(req.IdempotencyKey)); err != nil {
		return nil, err
	}

	// B1：多於 1 張照片一律拒絕，不再靜默只取第一張。
	if len(req.StagedTokens) > 1 {
		return &pb.PostResponse{Code: int32(pkgerrors.GetCode(usecase.ErrPostTooManyImages)), Message: usecase.ErrPostTooManyImages.Error()}, nil
	}
	var stagedToken string
	if len(req.StagedTokens) > 0 {
		stagedToken = req.StagedTokens[0]
	}
	var productID *int64
	if len(req.ProductIds) > 0 {
		pid := parseInt64(req.ProductIds[0])
		productID = &pid
	}

	post, err := s.postUC.CreatePost(ctx, req.SellerId, req.Content, stagedToken, productID, req.Tags,
		createPublishStatus(delegated))
	if err != nil {
		// Gateway envelope_guard_interceptor wraps as `[NNNN] message` for
		// messageZh parsing — handler returns the bare message to avoid
		// double-prefixing (same convention as im-service biz code 9201).
		if c := hashtagErrCode(err); c != 0 {
			return &pb.PostResponse{Code: c, Message: err.Error()}, nil
		}
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	detail := toPostDetailInfo(post, req.SellerId, false, "", "", nil)
	// B-153 §5.4: include tags in the create response (best-effort).
	if tagsByPost, terr := s.postUC.FillTags(ctx, []int64{post.ID}); terr != nil {
		s.logger.Warn("FillTags failed in CreatePost", zap.Error(terr))
	} else if tags, ok := tagsByPost[post.ID]; ok {
		detail.Tags = tags
	}
	return &pb.PostResponse{Code: 0, Data: detail}, nil
}

// createPublishStatus decides what a new post's publish_status is.
//
// A delegated create is always a draft: publishing is a separate capability
// (posts.publish) behind its own endpoint, so CreatePostRequest deliberately has
// no publish_mode field (§6.1.9). A seller creating their own post keeps the
// historical behaviour and is published immediately.
func createPublishStatus(d *commonpb.DelegationContext) int16 {
	if d != nil {
		return domain.PostPublishStatusDraft
	}
	return domain.PostPublishStatusPublished
}

func (s *ContentServer) UpdatePost(ctx context.Context, req *pb.UpdatePostRequest) (*pb.PostResponse, error) {
	if err := delegation.Validate(ctx, req.GetDelegation(), req.SellerId, delegation.CapPostsWrite,
		delegation.WithRequestIdempotencyKey(req.IdempotencyKey)); err != nil {
		return nil, err
	}
	expectedVersion, err := delegation.ParseExpectedVersion(req.ExpectedVersion)
	if err != nil {
		return nil, err
	}

	if len(req.KeepAssetIds)+len(req.NewStagedTokens) > 1 {
		return &pb.PostResponse{Code: int32(pkgerrors.GetCode(usecase.ErrPostTooManyImages)), Message: usecase.ErrPostTooManyImages.Error()}, nil
	}
	var keepAssetID string
	if len(req.KeepAssetIds) > 0 {
		keepAssetID = req.KeepAssetIds[0]
	}
	var newStagedToken string
	if len(req.NewStagedTokens) > 0 {
		newStagedToken = req.NewStagedTokens[0]
	}
	var productID *int64
	if len(req.ProductIds) > 0 {
		pid := parseInt64(req.ProductIds[0])
		productID = &pid
	}

	post, err := s.postUC.UpdatePost(ctx, parseInt64(req.PostId), req.SellerId, req.Content, keepAssetID, newStagedToken, productID, req.TagsPresent, req.Tags, expectedVersion)
	if err != nil {
		// Bare message; gateway envelope_guard_interceptor adds `[NNNN]` once.
		if c := hashtagErrCode(err); c != 0 {
			return &pb.PostResponse{Code: c, Message: err.Error()}, nil
		}
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	detail := toPostDetailInfo(post, req.SellerId, false, "", "", nil)
	// B-153 §5.4: reflect updated tag set in the response (best-effort).
	if tagsByPost, terr := s.postUC.FillTags(ctx, []int64{post.ID}); terr != nil {
		s.logger.Warn("FillTags failed in UpdatePost", zap.Error(terr))
	} else if tags, ok := tagsByPost[post.ID]; ok {
		detail.Tags = tags
	}
	return &pb.PostResponse{Code: 0, Data: detail}, nil
}

// UpdateSellerPostStatus is the server side of the two command endpoints
// POST .../posts/{postId}/publish and .../unpublish (56 §6.1.9), including the
// scheduled variant of publish (§7.6 T0).
//
// It is one RPC and not three because the three commands write the same row
// through the same optimistic lock; what differs is only the target value and
// whether a time was given. Splitting them would give the version predicate
// three implementations to stay in step with.
//
// Field validation runs before the store is touched: a malformed command must
// not tell the caller whether the post exists, and every rejection below is a
// property of the request alone.
func (s *ContentServer) UpdateSellerPostStatus(ctx context.Context, req *pb.UpdateSellerPostStatusRequest) (*pb.PostResponse, error) {
	delegated := req.GetDelegation()
	if err := delegation.Validate(ctx, delegated, req.SellerId, delegation.CapPostsPublish,
		delegation.WithRequestIdempotencyKey(req.IdempotencyKey)); err != nil {
		return nil, err
	}
	expectedVersion, err := delegation.ParseExpectedVersion(req.ExpectedVersion)
	if err != nil {
		return nil, err
	}

	target, err := parseTargetPublishStatus(req.TargetStatus)
	if err != nil {
		return nil, err
	}

	// now() is taken here and never from the request: the bounds are the only
	// thing standing between "schedule" and "a row that fires in one second" or
	// "a row that is pending for ten years", and a caller-supplied clock would
	// let the caller choose which side of them to land on. The Gateway runs the
	// same check against its own clock — that one is for fast failure, this one
	// is authoritative.
	scheduledAt, err := parseScheduledPublishAt(req.ScheduledPublishAt, target, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	var scheduledBySessionID int64
	if scheduledAt != nil {
		// The worker reads scheduled_by_session_id at execution time to rebuild
		// who authorised the publish, and refuses to publish a pending row that
		// has none. A schedule created outside a delegated session could never be
		// executed, so it is refused here rather than written and stranded.
		scheduledBySessionID = parseInt64(delegated.GetSessionId())
		if scheduledBySessionID <= 0 {
			return nil, delegation.InvalidCommand("排程發布必須在代行工作階段內進行")
		}
	}

	post, err := s.postUC.UpdatePublishStatus(ctx, usecase.UpdatePublishStatusInput{
		PostID:               parseInt64(req.PostId),
		SellerID:             req.SellerId,
		TargetStatus:         target,
		ScheduledAt:          scheduledAt,
		ScheduledBySessionID: scheduledBySessionID,
		ExpectedVersion:      expectedVersion,
		Delegated:            delegation.IsDelegated(delegated),
	})
	if err != nil {
		// Bare message; gateway envelope_guard_interceptor adds `[NNNN]` once.
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	detail := toPostDetailInfo(post, req.SellerId, false, "", "", nil)
	if tagsByPost, terr := s.postUC.FillTags(ctx, []int64{post.ID}); terr != nil {
		s.logger.Warn("FillTags failed in UpdateSellerPostStatus", zap.Error(terr))
	} else if tags, ok := tagsByPost[post.ID]; ok {
		detail.Tags = tags
	}
	return &pb.PostResponse{Code: 0, Data: detail}, nil
}

// parseTargetPublishStatus accepts exactly the two publish commands.
//
// DRAFT is rejected rather than accepted-and-ignored: "back to draft" is not a
// publish command and the endpoint has no such mode this iteration. Accepting it
// would silently unpublish a post through an endpoint whose audit action says
// PUBLISH.
func parseTargetPublishStatus(raw string) (int16, error) {
	switch raw {
	case strconv.Itoa(int(domain.PostPublishStatusPublished)):
		return domain.PostPublishStatusPublished, nil
	case strconv.Itoa(int(domain.PostPublishStatusUnpublished)):
		return domain.PostPublishStatusUnpublished, nil
	default:
		return 0, delegation.InvalidCommand("targetStatus 只接受 4402（已發布）或 4403（已下架）")
	}
}

// scheduledPublishAtLayout is ISO 8601 UTC at second precision. A lone trailing
// "Z" is a literal in Go's reference layout (only "Z07:00" / "Z0700" mean "zone
// here"), so this rejects offsets and fractional seconds instead of quietly
// accepting them — the wire contract is one format, and a value that parses two
// ways is a value the Gateway and this service can disagree about.
const scheduledPublishAtLayout = "2006-01-02T15:04:05Z"

// parseScheduledPublishAt validates the optional schedule time against now.
//
// Empty means "publish immediately" and yields nil. The bounds are checked in
// both directions on purpose: only refusing past times accepts both "one second
// from now" (a row racing the immediate path) and "ten years from now" (a row
// pending forever), and neither reports anything until long afterwards.
func parseScheduledPublishAt(raw string, target int16, now time.Time) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if target != domain.PostPublishStatusPublished {
		return nil, delegation.InvalidCommand("scheduledPublishAt 只能與 targetStatus=4402 一起使用")
	}
	t, err := time.Parse(scheduledPublishAtLayout, raw)
	// Re-formatting and comparing is the actual second-precision check: time.Parse
	// accepts a fractional second even when the layout has none (documented Go
	// behaviour), so the layout alone would let "…:05.250Z" through — a value the
	// Gateway's own check would also accept and then hand on in a different form.
	if err != nil || t.Format(scheduledPublishAtLayout) != raw {
		return nil, delegation.InvalidCommand("scheduledPublishAt 必須是秒精度、Z 結尾的 ISO 8601 UTC 時間")
	}
	if t.Before(now.Add(domain.SchedulePublishMinLead)) {
		return nil, delegation.InvalidCommand("scheduledPublishAt 必須至少在 5 分鐘之後")
	}
	if t.After(now.Add(domain.SchedulePublishMaxLead)) {
		return nil, delegation.InvalidCommand("scheduledPublishAt 不得超過 90 天之後")
	}
	return &t, nil
}

func (s *ContentServer) DeletePost(ctx context.Context, req *pb.DeletePostRequest) (*pb.PostOperationResponse, error) {
	if err := s.postUC.DeletePost(ctx, parseInt64(req.PostId), req.SellerId); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostOperationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.PostOperationResponse{Code: 0}, nil
}

func (s *ContentServer) TogglePostLike(ctx context.Context, req *pb.TogglePostLikeRequest) (*pb.TogglePostLikeResponse, error) {
	liked, err := s.postUC.ToggleLike(ctx, parseInt64(req.PostId), req.BuyerId, req.StoreId)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.TogglePostLikeResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.TogglePostLikeResponse{Code: 0, Liked: liked}, nil
}

func (s *ContentServer) SharePost(ctx context.Context, req *pb.SharePostRequest) (*pb.SharePostResponse, error) {
	count, err := s.postUC.SharePost(ctx, parseInt64(req.PostId))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.SharePostResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.SharePostResponse{Code: 0, ShareCount: count}, nil
}

func (s *ContentServer) ReportPost(ctx context.Context, req *pb.ReportPostRequest) (*pb.ReportPostResponse, error) {
	report, err := s.postUC.ReportPost(ctx, req.ReporterId, parseInt64(req.PostId), req.ReasonCode, req.Description)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ReportPostResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ReportPostResponse{
		Code:      0,
		ReportId:  strconv.FormatInt(report.ID, 10),
		CreatedAt: timeutil.FormatISO8601(report.CreateTime.Time),
	}, nil
}

func (s *ContentServer) ListMyPosts(ctx context.Context, req *pb.ListMyPostsRequest) (*pb.PostListResponse, error) {
	// seller_id IS the ownership key here, and the query is already scoped to it
	// (ListMyPosts filters WHERE seller_id = $1), so Q4 needs no separate step —
	// unlike GetPostDetail, this one cannot return another store's rows.
	if err := delegation.Validate(ctx, req.GetDelegation(), req.SellerId, delegation.CapPostsRead); err != nil {
		return nil, err
	}
	posts, total, err := s.postUC.ListMyPosts(ctx, req.SellerId, req.Status, req.Page, req.PageSize)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PostListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	var sellerName, sellerLogo string
	if len(posts) > 0 {
		if pair, ok := s.resolveSellerDisplay(ctx, []int64{req.SellerId})[req.SellerId]; ok {
			sellerName, sellerLogo = pair[0], pair[1]
		}
	}
	productMap := s.fetchProductMap(ctx, posts)

	items := make([]*pb.PostListItem, 0, len(posts))
	for _, p := range posts {
		item := &pb.PostListItem{
			Id:            strconv.FormatInt(p.ID, 10),
			ImageKey:      optionalString(p.Image),
			SellerId:      strconv.FormatInt(p.SellerID, 10),
			SellerName:    sellerName,
			SellerLogoKey: sellerLogo,
			Content:       optionalString(p.Content),
			LikeCount:     p.LikeCount,
			CommentCount:  p.CommentCount,
			ViewCount:     p.ViewCount,
			ShareCount:    p.ShareCount,
			CreatedAt:     p.CreateTime.Time.Unix(),
			Product:       postProductFromPointer(p.ProductID, productMap),
			// 只有 ListMyPosts 帶它：這是賣家／代行者看自己的清單，需要分辨草稿與已發布。
			// 公開清單（GetPostList）看得到的一律是已發布，帶了只是多一個恆定值。
			PublishStatus: strconv.Itoa(int(p.PublishStatus)),
		}
		if p.ScheduleState != nil && *p.ScheduleState == domain.ScheduleStatePending {
			item.ScheduledPublishAt = formatOptionalISO8601(p.ScheduledPublishAt)
		}
		if p.HiddenAt != nil {
			item.HiddenAt = p.HiddenAt.Unix()
			item.HiddenReason = optionalString(p.HiddenReason)
		}
		items = append(items, item)
	}
	s.fillPostListItemTags(ctx, posts, items)
	return &pb.PostListResponse{
		Code:     0,
		Data:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *ContentServer) AdminListPosts(ctx context.Context, req *pb.AdminListPostsRequest) (*pb.AdminPostListResponse, error) {
	posts, total, err := s.postUC.AdminListPosts(ctx, req.Status, req.Page, req.PageSize)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.AdminPostListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	sellerNameMap := s.fetchAdminSellerMap(ctx, posts)

	items := make([]*pb.AdminPostListItem, 0, len(posts))
	for _, p := range posts {
		status := "published"
		if p.DeletedAt != nil {
			status = "deleted"
		} else if p.HiddenAt != nil {
			status = "hidden"
		}

		item := &pb.AdminPostListItem{
			PostId:      strconv.FormatInt(p.ID, 10),
			StoreId:     strconv.FormatInt(p.SellerID, 10),
			SellerName:  sellerNameMap[p.SellerID],
			Content:     optionalString(p.Content),
			ReportCount: p.ReportCount,
			Status:      status,
			CreatedAt:   p.CreateTime.Time.Unix(),
		}
		if p.Image != nil && *p.Image != "" {
			item.ImageKeys = []string{*p.Image}
		}
		if p.HiddenAt != nil {
			item.HiddenReason = optionalString(p.HiddenReason)
		}
		item.LatestReportReason = p.LatestReportReason
		if p.LatestReportAt.Valid {
			item.LatestReportAt = p.LatestReportAt.Time.Unix()
		}
		items = append(items, item)
	}
	return &pb.AdminPostListResponse{
		Code:     0,
		Data:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *ContentServer) AdminModeratePost(ctx context.Context, req *pb.AdminModeratePostRequest) (*pb.PostOperationResponse, error) {
	switch req.Action {
	case "HIDE":
		report, err := s.reportUC.CreateAdminReport(ctx, codemeta.ReportTargetTypePost.Value, parseInt64(req.PostId), req.ReasonCode, req.Reason)
		if err != nil {
			if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
				return &pb.PostOperationResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
		if _, err := s.reportUC.AdminResolve(ctx, report.ID, codemeta.ReportResolveActionHideTarget.Value, req.OperatorId, req.Reason); err != nil {
			if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
				return &pb.PostOperationResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
	case "RESTORE":
		if err := s.postUC.RestorePost(ctx, parseInt64(req.PostId)); err != nil {
			if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
				return &pb.PostOperationResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
	default:
		return &pb.PostOperationResponse{Code: 400, Message: "invalid action: must be HIDE or RESTORE"}, nil
	}
	return &pb.PostOperationResponse{Code: 0}, nil
}

// ==================== Hashtag RPCs (B-153) ====================

// SearchHashtags returns auto-complete suggestions for hashtag input. §5.1.
func (s *ContentServer) SearchHashtags(ctx context.Context, req *pb.SearchHashtagsRequest) (*pb.SearchHashtagsResponse, error) {
	mode := usecase.SearchModePrefix
	if req.Mode == pb.HashtagSearchMode_HASHTAG_SEARCH_CONTAIN {
		mode = usecase.SearchModeContain
	}
	items, err := s.hashtagUC.Search(ctx, req.Q, mode, req.Limit)
	if err != nil {
		// Bare message; gateway envelope_guard_interceptor adds `[NNNN]` once.
		if c := hashtagErrCode(err); c != 0 {
			return &pb.SearchHashtagsResponse{Code: c, Message: err.Error()}, nil
		}
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.SearchHashtagsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	data := make([]*pb.HashtagSuggestItem, len(items))
	for i, it := range items {
		data[i] = &pb.HashtagSuggestItem{
			Name:      it.Name,
			PostCount: strconv.FormatInt(int64(it.PostCount), 10), // 對外數字一律 string（docs/開發規範/Java 微服務開發規範.md §2）
		}
	}
	return &pb.SearchHashtagsResponse{Code: 0, Message: "success", Data: data}, nil
}

// GetTrendingHashtags returns trending hashtags for a date range. §5.2.
func (s *ContentServer) GetTrendingHashtags(ctx context.Context, req *pb.GetTrendingHashtagsRequest) (*pb.GetTrendingHashtagsResponse, error) {
	items, err := s.hashtagUC.Popular(ctx, req.StartFrom, req.EndTo, req.Limit)
	if err != nil {
		// Bare message; gateway envelope_guard_interceptor adds `[NNNN]` once.
		if c := hashtagErrCode(err); c != 0 {
			return &pb.GetTrendingHashtagsResponse{Code: c, Message: err.Error()}, nil
		}
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetTrendingHashtagsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	data := make([]*pb.TrendingHashtagItem, len(items))
	for i, it := range items {
		data[i] = &pb.TrendingHashtagItem{
			Hashtag: it.Hashtag,
			Count:   strconv.FormatInt(int64(it.Count), 10), // 對外數字一律 string（docs/開發規範/Java 微服務開發規範.md §2）
		}
	}
	return &pb.GetTrendingHashtagsResponse{Code: 0, Message: "success", Data: data}, nil
}

// ==================== Comment RPCs ====================

func callerOf(buyerID, storeID int64) usecase.Caller {
	return usecase.Caller{BuyerID: buyerID, StoreID: storeID}
}

func (s *ContentServer) GetCommentList(ctx context.Context, req *pb.GetCommentListRequest) (*pb.CommentListResponse, error) {
	caller := callerOf(req.BuyerId, req.StoreId)
	parentID := parseInt64(req.ParentId)
	if parentID > 0 {
		replies, total, err := s.commentUC.ListReplies(ctx, parentID, req.Page, req.PageSize)
		if err != nil {
			if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
				return &pb.CommentListResponse{Code: int32(code), Message: err.Error()}, nil
			}
			return nil, err
		}
		viewer := s.commentUC.ViewerForComment(ctx, parentID, caller)
		// B-135: drop replies whose author is mutually blocked with viewer.
		replies = filterCommentsByBlocklist(ctx, s.rdb, viewer.ID, replies)
		replyIDs := make([]int64, len(replies))
		for i, c := range replies {
			replyIDs[i] = c.ID
		}
		likedSet := s.commentUC.BatchIsLiked(ctx, replyIDs, viewer)
		s.enrichCommentUserInfo(ctx, replies)
		data := make([]*pb.CommentInfo, 0, len(replies))
		for _, c := range replies {
			data = append(data, toCommentInfo(c, caller, likedSet[c.ID], nil))
		}
		return &pb.CommentListResponse{
			Code:     0,
			Data:     data,
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
		}, nil
	}

	postID := parseInt64(req.PostId)
	comments, total, err := s.commentUC.ListTopLevel(ctx, postID, req.Page, req.PageSize)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CommentListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	viewer := s.commentUC.ViewerForPost(ctx, postID, caller)
	// B-135: drop top-level comments whose author is mutually blocked with viewer.
	comments = filterCommentsByBlocklist(ctx, s.rdb, viewer.ID, comments)
	// Batch fetch latest 3 replies for all top-level comments (1 query instead of N)
	parentIDs := make([]int64, len(comments))
	for i, c := range comments {
		parentIDs[i] = c.ID
	}
	repliesMap := s.commentUC.BatchGetLatestReplies(ctx, parentIDs, 3)

	// Collect all comment + reply IDs for batch like check (1 query instead of N)
	allComments := make([]*gen.CmsComment, 0, len(comments)*4)
	allCommentIDs := make([]int64, 0, len(comments)*4)
	for _, c := range comments {
		allComments = append(allComments, c)
		allCommentIDs = append(allCommentIDs, c.ID)
		for _, r := range repliesMap[c.ID] {
			allComments = append(allComments, r)
			allCommentIDs = append(allCommentIDs, r.ID)
		}
	}
	likedSet := s.commentUC.BatchIsLiked(ctx, allCommentIDs, viewer)
	s.enrichCommentUserInfo(ctx, allComments)

	data := make([]*pb.CommentInfo, 0, len(comments))
	for _, c := range comments {
		replyInfos := make([]*pb.CommentInfo, 0, len(repliesMap[c.ID]))
		for _, r := range repliesMap[c.ID] {
			replyInfos = append(replyInfos, toCommentInfo(r, caller, likedSet[r.ID], nil))
		}
		data = append(data, toCommentInfo(c, caller, likedSet[c.ID], replyInfos))
	}
	return &pb.CommentListResponse{
		Code:     0,
		Data:     data,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// actorRef 是互動者 (kind, id)。buyer_id 與 store_id 是兩個獨立命名空間，同一個數值在兩邊是不同的人，
// 所以名稱查表一律以 (kind, id) 為鍵，不可合併成單一 map[int64]。
type actorRef struct {
	Kind int16
	ID   int64
}

// lookupActors 批次向 buyer-service、seller-service 查互動者的名稱與頭像。查不到的 key 不在結果裡，
// 呼叫端保留 DB 反正規化的值。
func (s *ContentServer) lookupActors(ctx context.Context, refs []actorRef) map[actorRef]*usecase.AccountInfo {
	var buyerIDs, storeIDs []int64
	seen := make(map[actorRef]struct{}, len(refs))
	for _, r := range refs {
		if r.ID == 0 {
			continue
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		switch r.Kind {
		case domain.ActorKindStore:
			storeIDs = append(storeIDs, r.ID)
		case domain.ActorKindBuyer:
			buyerIDs = append(buyerIDs, r.ID)
		}
		// 未知的 kind 不查任何服務：呼叫端保留 DB 反正規化的值。
	}
	out := make(map[actorRef]*usecase.AccountInfo, len(seen))
	if len(buyerIDs) > 0 {
		if m, err := s.buyerClient.GetBuyerMetaBatch(ctx, buyerIDs); err == nil {
			for id, info := range m {
				if info != nil {
					out[actorRef{Kind: domain.ActorKindBuyer, ID: id}] = info
				}
			}
		}
	}
	if len(storeIDs) > 0 {
		if m, err := s.sellerClient.GetSellerMetaBatch(ctx, storeIDs); err == nil {
			for id, info := range m {
				if info != nil {
					out[actorRef{Kind: domain.ActorKindStore, ID: id}] = info
				}
			}
		}
	}
	return out
}

// enrichCommentUserInfo 以留言者 (kind, id) 現查名稱與頭像，覆寫列上的 user_name／user_avatar
// （含巢狀回覆）；查不到時保留 DB 反正規化的值。
func (s *ContentServer) enrichCommentUserInfo(ctx context.Context, comments []*gen.CmsComment) {
	refs := make([]actorRef, 0, len(comments))
	for _, c := range comments {
		refs = append(refs, actorRef{Kind: c.Role, ID: c.UserID})
	}
	infos := s.lookupActors(ctx, refs)
	for _, c := range comments {
		if info, ok := infos[actorRef{Kind: c.Role, ID: c.UserID}]; ok {
			name, avatar := info.Nickname, info.Avatar
			c.UserName, c.UserAvatar = &name, &avatar
		}
	}
}

func (s *ContentServer) CreateComment(ctx context.Context, req *pb.CreateCommentRequest) (*pb.CommentResponse, error) {
	comment, err := s.commentUC.CreateComment(ctx, parseInt64(req.PostId), callerOf(req.BuyerId, req.StoreId), req.Content, parseInt64(req.ParentId), parseInt64(req.ReplyToId), req.ClientRequestId, sellerSuspendedFromContext(ctx))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CommentResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CommentResponse{Code: 0, Data: toCommentInfo(comment, callerOf(req.BuyerId, req.StoreId), false, nil)}, nil
}

func (s *ContentServer) UpdateComment(ctx context.Context, req *pb.UpdateCommentRequest) (*pb.CommentResponse, error) {
	caller := callerOf(req.BuyerId, req.StoreId)
	comment, err := s.commentUC.UpdateComment(ctx, parseInt64(req.CommentId), caller, req.Content)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CommentResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CommentResponse{Code: 0, Data: toCommentInfo(comment, caller, s.commentUC.IsLiked(ctx, comment.ID, s.commentUC.ViewerForPost(ctx, comment.PostID, caller)), nil)}, nil
}

func (s *ContentServer) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.CommentOperationResponse, error) {
	if err := s.commentUC.DeleteComment(ctx, parseInt64(req.CommentId), callerOf(req.BuyerId, req.StoreId)); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.CommentOperationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.CommentOperationResponse{Code: 0}, nil
}

func (s *ContentServer) ToggleCommentLike(ctx context.Context, req *pb.ToggleCommentLikeRequest) (*pb.ToggleCommentLikeResponse, error) {
	liked, err := s.commentUC.ToggleLike(ctx, parseInt64(req.CommentId), callerOf(req.BuyerId, req.StoreId))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ToggleCommentLikeResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ToggleCommentLikeResponse{Code: 0, Liked: liked}, nil
}

// ==================== Forum RPCs ====================

func (s *ContentServer) GetSellerForum(ctx context.Context, req *pb.GetSellerForumRequest) (*pb.ForumListResponse, error) {
	sellerID := parseInt64(req.SellerId)
	entries, total, err := s.forumUC.ListForum(ctx, sellerID, req.Page, req.PageSize)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ForumListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	caller := callerOf(req.BuyerId, req.StoreId)
	// B-135: drop forum entries whose author is mutually blocked with viewer.
	entries = filterForumByBlocklist(ctx, s.rdb, s.forumUC.ViewerForForum(sellerID, caller).ID, entries)
	s.enrichForumUserInfo(ctx, entries)
	data := make([]*pb.ForumEntryInfo, 0, len(entries))
	for _, e := range entries {
		data = append(data, toForumEntryInfo(e, caller))
	}
	return &pb.ForumListResponse{
		Code:     0,
		Data:     data,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *ContentServer) GetForumReplies(ctx context.Context, req *pb.GetForumRepliesRequest) (*pb.ForumListResponse, error) {
	parentID := parseInt64(req.ParentId)
	entries, total, err := s.forumUC.ListReplies(ctx, parentID, req.Page, req.PageSize)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ForumListResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	caller := callerOf(req.BuyerId, req.StoreId)
	// B-135: drop forum replies whose author is mutually blocked with viewer.
	entries = filterForumByBlocklist(ctx, s.rdb, s.forumUC.ViewerForEntry(ctx, parentID, caller).ID, entries)
	s.enrichForumUserInfo(ctx, entries)
	data := make([]*pb.ForumEntryInfo, 0, len(entries))
	for _, e := range entries {
		data = append(data, toForumEntryInfo(e, caller))
	}
	return &pb.ForumListResponse{
		Code:     0,
		Data:     data,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// enrichForumUserInfo 以留言者 (kind, id) 現查名稱與頭像，覆寫列上的 user_name／user_avatar，
// 讓使用者改資料後頭像保持同步；查不到或新值為空時保留 DB 反正規化的值（對象未設頭像時不把它蓋成空字串）。
func (s *ContentServer) enrichForumUserInfo(ctx context.Context, entries []*gen.CmsSellerForum) {
	refs := make([]actorRef, 0, len(entries))
	for _, e := range entries {
		refs = append(refs, actorRef{Kind: e.Role, ID: e.UserID})
	}
	infos := s.lookupActors(ctx, refs)
	for _, e := range entries {
		info, ok := infos[actorRef{Kind: e.Role, ID: e.UserID}]
		if !ok {
			continue
		}
		if info.Nickname != "" {
			name := info.Nickname
			e.UserName = &name
		}
		if info.Avatar != "" {
			avatar := info.Avatar
			e.UserAvatar = &avatar
		}
	}
}

func (s *ContentServer) CreateForumComment(ctx context.Context, req *pb.CreateForumCommentRequest) (*pb.ForumCommentResponse, error) {
	entry, err := s.forumUC.CreateComment(ctx, parseInt64(req.SellerId), callerOf(req.BuyerId, req.StoreId), req.Content, sellerSuspendedFromContext(ctx))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ForumCommentResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ForumCommentResponse{Code: 0, Data: toForumEntryInfo(entry, callerOf(req.BuyerId, req.StoreId))}, nil
}

func (s *ContentServer) ReplyForumComment(ctx context.Context, req *pb.ReplyForumCommentRequest) (*pb.ForumCommentResponse, error) {
	entry, err := s.forumUC.ReplyComment(ctx, parseInt64(req.ParentId), callerOf(req.BuyerId, req.StoreId), req.Content, sellerSuspendedFromContext(ctx))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ForumCommentResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ForumCommentResponse{Code: 0, Data: toForumEntryInfo(entry, callerOf(req.BuyerId, req.StoreId))}, nil
}

func (s *ContentServer) UpdateForumComment(ctx context.Context, req *pb.UpdateForumCommentRequest) (*pb.ForumCommentResponse, error) {
	entry, err := s.forumUC.UpdateComment(ctx, parseInt64(req.EntryId), callerOf(req.BuyerId, req.StoreId), req.Content)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ForumCommentResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ForumCommentResponse{Code: 0, Data: toForumEntryInfo(entry, callerOf(req.BuyerId, req.StoreId))}, nil
}

func (s *ContentServer) DeleteForumComment(ctx context.Context, req *pb.DeleteForumCommentRequest) (*pb.ForumOperationResponse, error) {
	if err := s.forumUC.DeleteComment(ctx, parseInt64(req.EntryId), callerOf(req.BuyerId, req.StoreId)); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ForumOperationResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ForumOperationResponse{Code: 0}, nil
}

// parseInt64 converts a request-side string id (proto §4：經 Gateway 暴露給前端的
// 可控 id 一律 string，避免 18-19 位 TSID 在 JS Number 精度丟失) to int64 for the
// usecase layer. 空字串 / 非數字 / 溢位 → 0；usecase 視 0 為「查無此 id」或「未指定」
// （如 sellerId=0 代表全部、parentId=0 代表頂層）。
func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
