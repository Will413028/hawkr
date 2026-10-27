package usecase

import (
	"context"
	"errors"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// ErrNotFound used in mocks to simulate pgx.ErrNoRows behaviour
var errNotFoundMock = errors.New("not found")

// ==================== mockPostRepository ====================

type mockPostRepo struct {
	getByIDFn func(ctx context.Context, id int64) (*gen.CmsPost, error)
	// getByIDTxFn overrides GetByIDTx (FIX 2's tx-aware re-read inside
	// UpdatePost's sameTx closure). Left nil in every existing test on
	// purpose: GetByIDTx falls back to getByIDFn/GetByID by default, so a
	// test that only sets up getByIDFn keeps seeing the same post state it
	// always did — only a test that specifically wants to exercise the
	// "publish_status changed between GetByID and the tx-scoped re-read" race
	// needs to set this.
	getByIDTxFn      func(ctx context.Context, id int64) (*gen.CmsPost, error)
	listFn           func(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsPost, error)
	countFn          func(ctx context.Context, sellerID int64) (int64, error)
	createFn         func(ctx context.Context, id, sellerID int64, content string, imageURL string, imageAssetID *int64, productID *int64) (*gen.CmsPost, error)
	updateFn         func(ctx context.Context, id int64, content string, imageURL string, imageAssetID *int64, productID *int64) error
	softDeleteFn     func(ctx context.Context, id, sellerID int64) (int64, error)
	incrViewCountFn  func(ctx context.Context, id int64) error
	incrShareCountFn func(ctx context.Context, id int64) error
	incrLikeCountFn  func(ctx context.Context, id int64) error
	decrLikeCountFn  func(ctx context.Context, id int64) error
	incrCommentFn    func(ctx context.Context, id int64) error
	decrCommentFn    func(ctx context.Context, id int64) error
	getLikeFn        func(ctx context.Context, postID int64, kind int16, userID int64) (*gen.CmsPostLike, error)
	createLikeFn     func(ctx context.Context, postID int64, kind int16, userID int64) error
	deleteLikeFn     func(ctx context.Context, postID int64, kind int16, userID int64) error
	listMyFn         func(ctx context.Context, sellerID int64, status string, limit, offset int64) ([]*gen.CmsPost, error)
	countMyFn        func(ctx context.Context, sellerID int64, status string) (int64, error)
	adminListFn      func(ctx context.Context, status string, limit, offset int64) ([]gen.AdminListPostsRow, error)
	adminCountFn     func(ctx context.Context, status string) (int64, error)
	listByHashtagFn  func(ctx context.Context, tagName string, limit, offset int64) ([]*gen.CmsPost, error)
	countByHashtagFn func(ctx context.Context, tagName string) (int64, error)
	hidePostFn       func(ctx context.Context, id int64, reason string) error
	restorePostFn    func(ctx context.Context, id int64) error
	// updateErr, when set, is what Update returns — used to simulate the CAS
	// coming back with zero affected rows.
	updateErr           error
	setPublishStatusErr error
	schedulePublishErr  error
	// lastScheduledAt / lastScheduledBySessionID record what a schedule write was
	// handed; nil means no schedule was written at all.
	lastScheduledAt          *time.Time
	lastScheduledBySessionID int64
	// lastPublishStatus / lastExpectedVersion record what the most recent write
	// was handed, so a test can distinguish "the value reached the repository"
	// from "the value was silently dropped and the write happened anyway".
	lastPublishStatus   int16
	lastExpectedVersion *int64
}

func (m *mockPostRepo) GetByID(ctx context.Context, id int64) (*gen.CmsPost, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errNotFoundMock
}
func (m *mockPostRepo) GetByIDTx(ctx context.Context, _ gen.DBTX, id int64) (*gen.CmsPost, error) {
	if m.getByIDTxFn != nil {
		return m.getByIDTxFn(ctx, id)
	}
	return m.GetByID(ctx, id)
}
func (m *mockPostRepo) List(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsPost, error) {
	if m.listFn != nil {
		return m.listFn(ctx, sellerID, limit, offset)
	}
	return nil, nil
}
func (m *mockPostRepo) Count(ctx context.Context, sellerID int64) (int64, error) {
	if m.countFn != nil {
		return m.countFn(ctx, sellerID)
	}
	return 0, nil
}
func (m *mockPostRepo) Create(ctx context.Context, id, sellerID int64, content string,
	imageURL string, imageAssetID *int64, productID *int64, publishStatus int16,
	sameTx func(ctx context.Context, q gen.DBTX) error) (*gen.CmsPost, error) {
	m.lastPublishStatus = publishStatus
	var row *gen.CmsPost
	var err error
	if m.createFn != nil {
		row, err = m.createFn(ctx, id, sellerID, content, imageURL, imageAssetID, productID)
	} else {
		// The stored row echoes the publish_status it was created with, so a test
		// asserting on hashtag counters sees the same visibility the real row would.
		row = &gen.CmsPost{ID: id, SellerID: sellerID, Content: testStringPointer(content), PublishStatus: publishStatus}
	}
	if err != nil {
		return nil, err
	}
	if sameTx != nil {
		if serr := sameTx(ctx, nil); serr != nil {
			return nil, serr
		}
	}
	return row, nil
}
func (m *mockPostRepo) Update(ctx context.Context, id int64, content string, imageURL string, imageAssetID *int64,
	productID *int64, expectedVersion *int64, sameTx func(ctx context.Context, q gen.DBTX) error) error {
	m.lastExpectedVersion = expectedVersion
	if m.updateErr != nil {
		return m.updateErr
	}
	if m.updateFn != nil {
		if err := m.updateFn(ctx, id, content, imageURL, imageAssetID, productID); err != nil {
			return err
		}
	}
	// The real repository runs sameTx inside the winning transaction. Running it
	// here keeps the hashtag diff observable, same as Create/SetPublishStatus.
	if sameTx != nil {
		return sameTx(ctx, nil)
	}
	return nil
}
func (m *mockPostRepo) SetPublishStatus(ctx context.Context, id int64, publishStatus int16, _ time.Time,
	expectedVersion *int64, sameTx func(ctx context.Context, q gen.DBTX) error) error {
	m.lastPublishStatus = publishStatus
	m.lastExpectedVersion = expectedVersion
	if m.setPublishStatusErr != nil {
		return m.setPublishStatusErr
	}
	// The real repository runs sameTx inside the winning transaction. Running it
	// here keeps the counter delta observable: a mock that skipped it would let a
	// wrong delta pass every test.
	if sameTx != nil {
		return sameTx(ctx, nil)
	}
	return nil
}

func (m *mockPostRepo) SchedulePublish(_ context.Context, _ int64, scheduledAt time.Time, sessionID int64,
	_ time.Time, expectedVersion *int64) error {
	m.lastScheduledAt = &scheduledAt
	m.lastScheduledBySessionID = sessionID
	m.lastExpectedVersion = expectedVersion
	return m.schedulePublishErr
}

func (m *mockPostRepo) SoftDelete(ctx context.Context, id, sellerID int64) (int64, error) {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id, sellerID)
	}
	return 1, nil
}
func (m *mockPostRepo) IncrViewCount(ctx context.Context, id int64) error {
	if m.incrViewCountFn != nil {
		return m.incrViewCountFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) IncrShareCount(ctx context.Context, id int64) error {
	if m.incrShareCountFn != nil {
		return m.incrShareCountFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) IncrLikeCount(ctx context.Context, id int64) error {
	if m.incrLikeCountFn != nil {
		return m.incrLikeCountFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) DecrLikeCount(ctx context.Context, id int64) error {
	if m.decrLikeCountFn != nil {
		return m.decrLikeCountFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) IncrCommentCount(ctx context.Context, id int64) error {
	if m.incrCommentFn != nil {
		return m.incrCommentFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) DecrCommentCount(ctx context.Context, id int64) error {
	if m.decrCommentFn != nil {
		return m.decrCommentFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) GetLike(ctx context.Context, postID int64, kind int16, userID int64) (*gen.CmsPostLike, error) {
	if m.getLikeFn != nil {
		return m.getLikeFn(ctx, postID, kind, userID)
	}
	return nil, errNotFoundMock
}
func (m *mockPostRepo) CreateLike(ctx context.Context, postID int64, kind int16, userID int64) error {
	if m.createLikeFn != nil {
		return m.createLikeFn(ctx, postID, kind, userID)
	}
	return nil
}
func (m *mockPostRepo) DeleteLike(ctx context.Context, postID int64, kind int16, userID int64) error {
	if m.deleteLikeFn != nil {
		return m.deleteLikeFn(ctx, postID, kind, userID)
	}
	return nil
}
func (m *mockPostRepo) BatchGetLikedPostIDs(_ context.Context, _ []int64, _ int16, _ int64) ([]int64, error) {
	return nil, nil
}
func (m *mockPostRepo) ListMy(ctx context.Context, sellerID int64, status string, limit, offset int64) ([]*gen.CmsPost, error) {
	if m.listMyFn != nil {
		return m.listMyFn(ctx, sellerID, status, limit, offset)
	}
	return nil, nil
}
func (m *mockPostRepo) CountMy(ctx context.Context, sellerID int64, status string) (int64, error) {
	if m.countMyFn != nil {
		return m.countMyFn(ctx, sellerID, status)
	}
	return 0, nil
}

func (m *mockPostRepo) AdminList(ctx context.Context, status string, limit, offset int64) ([]gen.AdminListPostsRow, error) {
	if m.adminListFn != nil {
		return m.adminListFn(ctx, status, limit, offset)
	}
	return nil, nil
}
func (m *mockPostRepo) AdminCount(ctx context.Context, status string) (int64, error) {
	if m.adminCountFn != nil {
		return m.adminCountFn(ctx, status)
	}
	return 0, nil
}
func (m *mockPostRepo) HidePost(ctx context.Context, id int64, reason string) error {
	if m.hidePostFn != nil {
		return m.hidePostFn(ctx, id, reason)
	}
	return nil
}
func (m *mockPostRepo) RestorePost(ctx context.Context, id int64) error {
	if m.restorePostFn != nil {
		return m.restorePostFn(ctx, id)
	}
	return nil
}
func (m *mockPostRepo) SoftDeleteBySellerID(_ context.Context, _ int64) error { return nil }
func (m *mockPostRepo) ListPostIDsBySellerID(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}
func (m *mockPostRepo) DeleteLikesByActor(_ context.Context, _ int16, _ int64) error { return nil }
func (m *mockPostRepo) ReconcileLikeCounts(_ context.Context) error                  { return nil }
func (m *mockPostRepo) ReconcileCommentCounts(_ context.Context) error               { return nil }

func (m *mockPostRepo) ListByHashtag(ctx context.Context, tagName string, limit, offset int64) ([]*gen.CmsPost, error) {
	if m.listByHashtagFn != nil {
		return m.listByHashtagFn(ctx, tagName, limit, offset)
	}
	return nil, nil
}
func (m *mockPostRepo) CountByHashtag(ctx context.Context, tagName string) (int64, error) {
	if m.countByHashtagFn != nil {
		return m.countByHashtagFn(ctx, tagName)
	}
	return 0, nil
}

// ==================== mockReportRepository ====================

type mockReportRepo struct {
	getReportFn    func(ctx context.Context, postID, reporterID int64) (*gen.Report, error)
	createReportFn func(ctx context.Context, id, postID, reporterID int64, reasonCode, description string) (*gen.Report, error)
}

func (m *mockReportRepo) GetReport(ctx context.Context, postID, reporterID int64) (*gen.Report, error) {
	if m.getReportFn != nil {
		return m.getReportFn(ctx, postID, reporterID)
	}
	return nil, errNotFoundMock
}
func (m *mockReportRepo) CreateReport(ctx context.Context, id, postID, reporterID int64, reasonCode, description string) (*gen.Report, error) {
	if m.createReportFn != nil {
		return m.createReportFn(ctx, id, postID, reporterID, reasonCode, description)
	}
	return &gen.Report{ID: id, TargetType: "POST", TargetID: postID, ReporterID: reporterID}, nil
}

// Generic B-135 methods — unused by existing post tests, return zero values.
func (m *mockReportRepo) GetByReporterAndTarget(_ context.Context, _ int64, _ string, _ int64) (*gen.Report, error) {
	return nil, errNotFoundMock
}
func (m *mockReportRepo) CreateGenericReport(_ context.Context, _ domain.CreateReportParams) (*gen.Report, error) {
	return nil, errNotFoundMock
}
func (m *mockReportRepo) ListByReporter(_ context.Context, _ int64, _, _ int64) ([]*gen.Report, error) {
	return nil, nil
}
func (m *mockReportRepo) AdminList(_ context.Context, _, _, _ string, _, _ int64) ([]*gen.Report, error) {
	return nil, nil
}
func (m *mockReportRepo) AdminGet(_ context.Context, _ int64) (*gen.Report, error) {
	return nil, errNotFoundMock
}
func (m *mockReportRepo) AdminResolve(_ context.Context, _ int64, _, _ string, _ int64) (*gen.Report, error) {
	return nil, errNotFoundMock
}
func (m *mockReportRepo) CountByReporterSince(_ context.Context, _ int64, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockReportRepo) Reopen(_ context.Context, _ int64) (*gen.Report, error) {
	return nil, errNotFoundMock
}
func (m *mockReportRepo) CountRecentActionsByOwner(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

// ==================== mockCommentRepository ====================

type mockCommentRepo struct {
	getByIDFn          func(ctx context.Context, id int64) (*gen.CmsComment, error)
	listTopLevelFn     func(ctx context.Context, postID, limit, offset int64) ([]*gen.CmsComment, error)
	countTopLevelFn    func(ctx context.Context, postID int64) (int64, error)
	listRepliesFn      func(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsComment, error)
	countRepliesFn     func(ctx context.Context, parentID int64) (int64, error)
	getLatestRepliesFn func(ctx context.Context, parentID, limit int64) ([]*gen.CmsComment, error)
	createFn           func(ctx context.Context, id, postID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsComment, error)
	updateContentFn    func(ctx context.Context, id int64, content string) (*gen.CmsComment, error)
	softDeleteFn       func(ctx context.Context, id int64) error
	incrReplyCountFn   func(ctx context.Context, id int64) error
	decrReplyCountFn   func(ctx context.Context, id int64) error
	incrLikeCountFn    func(ctx context.Context, id int64) error
	decrLikeCountFn    func(ctx context.Context, id int64) error
	getLikeFn          func(ctx context.Context, commentID int64, kind int16, userID int64) (*gen.CmsCommentLike, error)
	createLikeFn       func(ctx context.Context, commentID int64, kind int16, userID int64) error
	deleteLikeFn       func(ctx context.Context, commentID int64, kind int16, userID int64) error
}

func (m *mockCommentRepo) GetByID(ctx context.Context, id int64) (*gen.CmsComment, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errNotFoundMock
}
func (m *mockCommentRepo) ListTopLevel(ctx context.Context, postID, limit, offset int64) ([]*gen.CmsComment, error) {
	if m.listTopLevelFn != nil {
		return m.listTopLevelFn(ctx, postID, limit, offset)
	}
	return nil, nil
}
func (m *mockCommentRepo) CountTopLevel(ctx context.Context, postID int64) (int64, error) {
	if m.countTopLevelFn != nil {
		return m.countTopLevelFn(ctx, postID)
	}
	return 0, nil
}
func (m *mockCommentRepo) ListReplies(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsComment, error) {
	if m.listRepliesFn != nil {
		return m.listRepliesFn(ctx, parentID, limit, offset)
	}
	return nil, nil
}
func (m *mockCommentRepo) CountReplies(ctx context.Context, parentID int64) (int64, error) {
	if m.countRepliesFn != nil {
		return m.countRepliesFn(ctx, parentID)
	}
	return 0, nil
}
func (m *mockCommentRepo) GetLatestReplies(ctx context.Context, parentID, limit int64) ([]*gen.CmsComment, error) {
	if m.getLatestRepliesFn != nil {
		return m.getLatestRepliesFn(ctx, parentID, limit)
	}
	return nil, nil
}
func (m *mockCommentRepo) BatchGetLatestReplies(ctx context.Context, parentIDs []int64, limit int32) ([]*gen.CmsComment, error) {
	return nil, nil
}
func (m *mockCommentRepo) Create(ctx context.Context, id, postID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsComment, error) {
	if m.createFn != nil {
		return m.createFn(ctx, id, postID, userID, userName, userAvatar, content, parentID, replyToID, replyToName, role)
	}
	return &gen.CmsComment{ID: id, PostID: postID, UserID: userID, Role: role}, nil
}
func (m *mockCommentRepo) UpdateContent(ctx context.Context, id int64, content string) (*gen.CmsComment, error) {
	if m.updateContentFn != nil {
		return m.updateContentFn(ctx, id, content)
	}
	return &gen.CmsComment{ID: id}, nil
}
func (m *mockCommentRepo) SoftDelete(ctx context.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockCommentRepo) HideComment(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockCommentRepo) IncrReplyCount(ctx context.Context, id int64) error {
	if m.incrReplyCountFn != nil {
		return m.incrReplyCountFn(ctx, id)
	}
	return nil
}
func (m *mockCommentRepo) DecrReplyCount(ctx context.Context, id int64) error {
	if m.decrReplyCountFn != nil {
		return m.decrReplyCountFn(ctx, id)
	}
	return nil
}
func (m *mockCommentRepo) IncrLikeCount(ctx context.Context, id int64) error {
	if m.incrLikeCountFn != nil {
		return m.incrLikeCountFn(ctx, id)
	}
	return nil
}
func (m *mockCommentRepo) DecrLikeCount(ctx context.Context, id int64) error {
	if m.decrLikeCountFn != nil {
		return m.decrLikeCountFn(ctx, id)
	}
	return nil
}
func (m *mockCommentRepo) GetLike(ctx context.Context, commentID int64, kind int16, userID int64) (*gen.CmsCommentLike, error) {
	if m.getLikeFn != nil {
		return m.getLikeFn(ctx, commentID, kind, userID)
	}
	return nil, errNotFoundMock
}
func (m *mockCommentRepo) CreateLike(ctx context.Context, commentID int64, kind int16, userID int64) error {
	if m.createLikeFn != nil {
		return m.createLikeFn(ctx, commentID, kind, userID)
	}
	return nil
}
func (m *mockCommentRepo) DeleteLike(ctx context.Context, commentID int64, kind int16, userID int64) error {
	if m.deleteLikeFn != nil {
		return m.deleteLikeFn(ctx, commentID, kind, userID)
	}
	return nil
}
func (m *mockCommentRepo) BatchGetLikedCommentIDs(_ context.Context, _ []int64, _ int16, _ int64) ([]int64, error) {
	return nil, nil
}
func (m *mockCommentRepo) SoftDeleteByActor(_ context.Context, _ int16, _ int64) error  { return nil }
func (m *mockCommentRepo) DeleteLikesByActor(_ context.Context, _ int16, _ int64) error { return nil }
func (m *mockCommentRepo) ReconcileLikeCounts(_ context.Context) error                  { return nil }
func (m *mockCommentRepo) ReconcileReplyCounts(_ context.Context) error                 { return nil }

// ==================== mockForumRepository ====================

type mockForumRepo struct {
	getByIDFn       func(ctx context.Context, id int64) (*gen.CmsSellerForum, error)
	listTopLevelFn  func(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsSellerForum, error)
	countTopLevelFn func(ctx context.Context, sellerID int64) (int64, error)
	listRepliesFn   func(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsSellerForum, error)
	countRepliesFn  func(ctx context.Context, parentID int64) (int64, error)
	createFn        func(ctx context.Context, id, sellerID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsSellerForum, error)
	updateContentFn func(ctx context.Context, id int64, content string) (*gen.CmsSellerForum, error)
	softDeleteFn    func(ctx context.Context, id int64) error
	incrReplyFn     func(ctx context.Context, id int64) error
	decrReplyFn     func(ctx context.Context, id int64) error
}

func (m *mockForumRepo) GetByID(ctx context.Context, id int64) (*gen.CmsSellerForum, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errNotFoundMock
}
func (m *mockForumRepo) ListTopLevel(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsSellerForum, error) {
	if m.listTopLevelFn != nil {
		return m.listTopLevelFn(ctx, sellerID, limit, offset)
	}
	return nil, nil
}
func (m *mockForumRepo) CountTopLevel(ctx context.Context, sellerID int64) (int64, error) {
	if m.countTopLevelFn != nil {
		return m.countTopLevelFn(ctx, sellerID)
	}
	return 0, nil
}
func (m *mockForumRepo) ListReplies(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsSellerForum, error) {
	if m.listRepliesFn != nil {
		return m.listRepliesFn(ctx, parentID, limit, offset)
	}
	return nil, nil
}
func (m *mockForumRepo) CountReplies(ctx context.Context, parentID int64) (int64, error) {
	if m.countRepliesFn != nil {
		return m.countRepliesFn(ctx, parentID)
	}
	return 0, nil
}
func (m *mockForumRepo) Create(ctx context.Context, id, sellerID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsSellerForum, error) {
	if m.createFn != nil {
		return m.createFn(ctx, id, sellerID, userID, userName, userAvatar, content, parentID, replyToID, replyToName, role)
	}
	return &gen.CmsSellerForum{ID: id, SellerID: sellerID, UserID: userID, Role: role}, nil
}
func (m *mockForumRepo) UpdateContent(ctx context.Context, id int64, content string) (*gen.CmsSellerForum, error) {
	if m.updateContentFn != nil {
		return m.updateContentFn(ctx, id, content)
	}
	return &gen.CmsSellerForum{ID: id, Content: content}, nil
}
func (m *mockForumRepo) SoftDelete(ctx context.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockForumRepo) HideEntry(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockForumRepo) IncrReplyCount(ctx context.Context, id int64) error {
	if m.incrReplyFn != nil {
		return m.incrReplyFn(ctx, id)
	}
	return nil
}
func (m *mockForumRepo) DecrReplyCount(ctx context.Context, id int64) error {
	if m.decrReplyFn != nil {
		return m.decrReplyFn(ctx, id)
	}
	return nil
}
func (m *mockForumRepo) SoftDeleteByActor(_ context.Context, _ int16, _ int64) error { return nil }

// ==================== mockProductClient ====================

type mockProductClient struct {
	getProductsByIDsFn func(ctx context.Context, productIDs []string) (map[string]*ProductBasicInfo, error)
}

func (m *mockProductClient) GetProductsByIDs(ctx context.Context, productIDs []string) (map[string]*ProductBasicInfo, error) {
	if m.getProductsByIDsFn != nil {
		return m.getProductsByIDsFn(ctx, productIDs)
	}
	return nil, nil
}

// ==================== mockBuyerClient ====================

type mockBuyerClient struct {
	getBuyerProfileFn   func(ctx context.Context, buyerID int64) (*AccountInfo, error)
	getBuyerMetaBatchFn func(ctx context.Context, buyerIDs []int64) (map[int64]*AccountInfo, error)
	resolvePrincipalFn  func(ctx context.Context, buyerID int64) int64
	// resolvePrincipalStrictFn overrides the default three-way answer; see
	// ResolvePrincipalStrict below.
	resolvePrincipalStrictFn func(ctx context.Context, buyerID int64) (PrincipalResolution, error)
}

func (m *mockBuyerClient) GetBuyerProfile(ctx context.Context, buyerID int64) (*AccountInfo, error) {
	if m.getBuyerProfileFn != nil {
		return m.getBuyerProfileFn(ctx, buyerID)
	}
	return nil, errNotFoundMock
}

func (m *mockBuyerClient) GetBuyerMetaBatch(ctx context.Context, buyerIDs []int64) (map[int64]*AccountInfo, error) {
	if m.getBuyerMetaBatchFn != nil {
		return m.getBuyerMetaBatchFn(ctx, buyerIDs)
	}
	return make(map[int64]*AccountInfo), nil
}

func (m *mockBuyerClient) GetSubscriberIDs(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}

func (m *mockBuyerClient) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	if m.resolvePrincipalFn != nil {
		return m.resolvePrincipalFn(ctx, buyerID)
	}
	return buyerID
}

// ResolvePrincipalStrict defaults to the grandfathered production baseline:
// every buyer_id is live and resolves (Found=true) to whatever
// resolvePrincipalFn says — identity when that is nil.
//
// It used to derive Found from "the value changed", which modelled every
// undiverged id as unresolved. That was harmless while nothing read Found on
// this mock; once BlockUseCase started failing closed on it (#2789) it would
// have made every grandfathered block look unresolvable. Set
// resolvePrincipalStrictFn to exercise the real not-found / cannot-answer
// states instead.
func (m *mockBuyerClient) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (PrincipalResolution, error) {
	if m.resolvePrincipalStrictFn != nil {
		return m.resolvePrincipalStrictFn(ctx, buyerID)
	}
	return PrincipalResolution{AccountID: m.ResolvePrincipal(ctx, buyerID), Found: true}, nil
}

// ==================== mockSellerClient ====================

type mockSellerClient struct {
	getSellerInfoFn      func(ctx context.Context, storeID int64) (*AccountInfo, error)
	getSellerMetaBatchFn func(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error)
	resolvePrincipalFn   func(ctx context.Context, storeID int64) int64
	// resolvePrincipalStrictFn overrides the default three-way answer; see
	// ResolvePrincipalStrict below.
	resolvePrincipalStrictFn func(ctx context.Context, storeID int64) (PrincipalResolution, error)
}

func (m *mockSellerClient) GetSellerInfo(ctx context.Context, storeID int64) (*AccountInfo, error) {
	if m.getSellerInfoFn != nil {
		return m.getSellerInfoFn(ctx, storeID)
	}
	return nil, errNotFoundMock
}

func (m *mockSellerClient) GetSellerMetaBatch(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error) {
	if m.getSellerMetaBatchFn != nil {
		return m.getSellerMetaBatchFn(ctx, storeIDs)
	}
	return make(map[int64]*AccountInfo), nil
}

func (m *mockSellerClient) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	if m.resolvePrincipalFn != nil {
		return m.resolvePrincipalFn(ctx, storeID)
	}
	return storeID
}

// ResolvePrincipalStrict mirrors mockBuyerClient's; see that comment.
func (m *mockSellerClient) ResolvePrincipalStrict(ctx context.Context, storeID int64) (PrincipalResolution, error) {
	if m.resolvePrincipalStrictFn != nil {
		return m.resolvePrincipalStrictFn(ctx, storeID)
	}
	return PrincipalResolution{AccountID: m.ResolvePrincipal(ctx, storeID), Found: true}, nil
}

// ==================== mockMediaClient ====================

type mockMediaClient struct {
	finalizeAndLinkFn   func(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error)
	releaseReferencesFn func(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error
}

func (m *mockMediaClient) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error) {
	if m.finalizeAndLinkFn != nil {
		return m.finalizeAndLinkFn(ctx, stagedToken, refType, refID)
	}
	return AssetInfo{AssetId: "1", ObjectKey: "posts/default.jpg"}, nil
}

func (m *mockMediaClient) ReleaseReferences(ctx context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
	if m.releaseReferencesFn != nil {
		return m.releaseReferencesFn(ctx, refType, refIDs, keepAssetIDs)
	}
	return nil
}

// ==================== mockContentEventPublisher ====================

type publishPostCreatedCall struct {
	sellerName     string
	sellerAvatar   string
	contentPreview string
	postID         int64
	storeID        int64
	followerID     int64
}

type publishCommentCreatedCall struct {
	commenterName        string
	postAuthorType       string
	contentPreview       string
	clientRequestID      string
	commentID            int64
	postID               int64
	commenterID          int64
	commenterPrincipalID int64
	postAuthorID         int64
	postAuthorStoreID    int64
	parentCommentID      int64
	snapshot             *pkgmq.CommentSnapshot
}

type publishCommentRepliedCall struct {
	replyToUserType    string
	replierName        string
	contentPreview     string
	clientRequestID    string
	replyToUserID      int64
	replierID          int64
	replierPrincipalID int64
	replyToUserStoreID int64
	postID             int64
	commentID          int64
	parentCommentID    int64
	snapshot           *pkgmq.CommentSnapshot
}

type publishCommentUpdatedCall struct {
	content   string
	updatedAt string
	commentID int64
	postID    int64
}

type publishCommentDeletedCall struct {
	commentID       int64
	postID          int64
	parentCommentID int64
}

type publishCommentLikeChangedCall struct {
	action               string
	likerName            string
	commentAuthorType    string
	commentID            int64
	postID               int64
	likeCount            int32
	userID               int64
	likerKind            int32
	commentAuthorID      int64
	commentAuthorStoreID int64
	likerPrincipalID     int64
}

type publishPostLikedCall struct {
	likerName string
	sellerID  int64
	storeID   int64
	postID    int64
}

type publishPostHiddenCall struct {
	sellerID int64
	storeID  int64
	postID   int64
	reason   string
}

type publishCounterDeltaCall struct {
	entityType string
	entityID   int64
	field      string
	delta      int32
}

type mockContentEventPublisher struct {
	postCreatedCalls         []publishPostCreatedCall
	commentCreatedCalls      []publishCommentCreatedCall
	commentRepliedCalls      []publishCommentRepliedCall
	commentUpdatedCalls      []publishCommentUpdatedCall
	commentDeletedCalls      []publishCommentDeletedCall
	commentLikeChangedCalls  []publishCommentLikeChangedCall
	postLikedCalls           []publishPostLikedCall
	postHiddenCalls          []publishPostHiddenCall
	publishCounterDeltaCalls []publishCounterDeltaCall
}

func (m *mockContentEventPublisher) PublishPostCreated(_ context.Context, postID, storeID, followerID int64, sellerName, sellerAvatar, contentPreview string) error {
	m.postCreatedCalls = append(m.postCreatedCalls, publishPostCreatedCall{
		postID: postID, storeID: storeID, followerID: followerID,
		sellerName: sellerName, sellerAvatar: sellerAvatar, contentPreview: contentPreview,
	})
	return nil
}

func (m *mockContentEventPublisher) PublishCommentCreated(_ context.Context, commentID, postID, commenterID, commenterPrincipalID int64, commenterName string, postAuthorID, postAuthorStoreID int64, postAuthorType, contentPreview string, parentCommentID int64, clientRequestID string, snapshot *pkgmq.CommentSnapshot) error {
	m.commentCreatedCalls = append(m.commentCreatedCalls, publishCommentCreatedCall{
		commentID: commentID, postID: postID, commenterID: commenterID, commenterPrincipalID: commenterPrincipalID,
		commenterName: commenterName,
		postAuthorID:  postAuthorID, postAuthorStoreID: postAuthorStoreID, postAuthorType: postAuthorType, contentPreview: contentPreview,
		parentCommentID: parentCommentID, clientRequestID: clientRequestID, snapshot: snapshot,
	})
	return nil
}

func (m *mockContentEventPublisher) PublishPostLiked(_ context.Context, sellerID, postID, storeID int64, likerName string) error {
	m.postLikedCalls = append(m.postLikedCalls, publishPostLikedCall{sellerID: sellerID, storeID: storeID, postID: postID, likerName: likerName})
	return nil
}

func (m *mockContentEventPublisher) PublishCommentReplied(_ context.Context, replyToUserID, replierID, replierPrincipalID, replyToUserStoreID int64, replyToUserType, replierName string, postID, commentID int64, contentPreview string, parentCommentID int64, clientRequestID string, snapshot *pkgmq.CommentSnapshot) error {
	m.commentRepliedCalls = append(m.commentRepliedCalls, publishCommentRepliedCall{
		replyToUserID: replyToUserID, replierID: replierID, replierPrincipalID: replierPrincipalID, replyToUserStoreID: replyToUserStoreID,
		replyToUserType: replyToUserType, replierName: replierName,
		postID: postID, commentID: commentID, contentPreview: contentPreview,
		parentCommentID: parentCommentID, clientRequestID: clientRequestID, snapshot: snapshot,
	})
	return nil
}

func (m *mockContentEventPublisher) PublishCommentUpdated(_ context.Context, commentID, postID int64, content, updatedAt string) error {
	m.commentUpdatedCalls = append(m.commentUpdatedCalls, publishCommentUpdatedCall{
		commentID: commentID, postID: postID, content: content, updatedAt: updatedAt,
	})
	return nil
}

func (m *mockContentEventPublisher) PublishCommentDeleted(_ context.Context, commentID, postID, parentCommentID int64) error {
	m.commentDeletedCalls = append(m.commentDeletedCalls, publishCommentDeletedCall{
		commentID: commentID, postID: postID, parentCommentID: parentCommentID,
	})
	return nil
}

func (m *mockContentEventPublisher) PublishCommentLikeChanged(_ context.Context, commentID, postID int64, likeCount int32, action string, userID int64, likerKind int32, commentAuthorID, commentAuthorStoreID, likerPrincipalID int64, likerName, commentAuthorType string) error {
	m.commentLikeChangedCalls = append(m.commentLikeChangedCalls, publishCommentLikeChangedCall{
		commentID: commentID, postID: postID, likeCount: likeCount, action: action,
		userID: userID, likerKind: likerKind, commentAuthorID: commentAuthorID, commentAuthorStoreID: commentAuthorStoreID, likerPrincipalID: likerPrincipalID,
		likerName: likerName, commentAuthorType: commentAuthorType,
	})
	return nil
}

func (m *mockContentEventPublisher) PublishPostHiddenByPlatform(_ context.Context, sellerID, postID, storeID int64, reason string) error {
	m.postHiddenCalls = append(m.postHiddenCalls, publishPostHiddenCall{sellerID: sellerID, storeID: storeID, postID: postID, reason: reason})
	return nil
}

func (m *mockContentEventPublisher) PublishCounterDelta(_ context.Context, entityType string, entityID int64, field string, delta int32) error {
	m.publishCounterDeltaCalls = append(m.publishCounterDeltaCalls, publishCounterDeltaCall{
		entityType: entityType, entityID: entityID, field: field, delta: delta,
	})
	return nil
}

// subscriberMockBuyerClient overrides GetSubscriberIDs to return a fixed list.
type subscriberMockBuyerClient struct {
	*mockBuyerClient
	subs []int64
}

func (m *subscriberMockBuyerClient) GetSubscriberIDs(_ context.Context, _ int64) ([]int64, error) {
	return m.subs, nil
}

// ==================== mockHashtagRepo ====================

type mockHashtagRepo struct {
	upsertByNameFn         func(ctx context.Context, q gen.DBTX, id int64, canonical string) (int64, error)
	adjustPostCountFn      func(ctx context.Context, q gen.DBTX, hashtagID int64, delta int32) error
	listByPostIDFn         func(ctx context.Context, q gen.DBTX, postID int64) ([]gen.CmsPostHashtag, error)
	listByPostIDsFn        func(ctx context.Context, postIDs []int64) (map[int64][]gen.CmsPostHashtag, error)
	listHashtagsByIDsFn    func(ctx context.Context, ids []int64) (map[int64]gen.CmsHashtag, error)
	listHashtagIDsByPostFn func(ctx context.Context, q gen.DBTX, postID int64) ([]int64, error)
	insertBindingFn        func(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error
	updateBindingFn        func(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error
	deleteBindingsFn       func(ctx context.Context, q gen.DBTX, postID int64, hashtagIDs []int64) error
	deleteAllBindingsFn    func(ctx context.Context, q gen.DBTX, postID int64) error
	searchByPrefixFn       func(ctx context.Context, prefix string, limit int32) ([]gen.SearchHashtagsByPrefixRow, error)
	searchByContainFn      func(ctx context.Context, query string, limit int32) ([]gen.SearchHashtagsByContainRow, error)
	popularInRangeFn       func(ctx context.Context, start, end time.Time, limit int32) ([]gen.SelectPopularHashtagsInRangeRow, error)

	// adjustCalls records every AdjustPostCount call in order — for asserting ASC-order invariant.
	adjustCalls []adjustCall
}

type adjustCall struct {
	hashtagID int64
	delta     int32
}

func (m *mockHashtagRepo) UpsertByName(ctx context.Context, q gen.DBTX, id int64, canonical string) (int64, error) {
	if m.upsertByNameFn != nil {
		return m.upsertByNameFn(ctx, q, id, canonical)
	}
	return id, nil
}
func (m *mockHashtagRepo) AdjustPostCount(ctx context.Context, q gen.DBTX, hashtagID int64, delta int32) error {
	m.adjustCalls = append(m.adjustCalls, adjustCall{hashtagID, delta})
	if m.adjustPostCountFn != nil {
		return m.adjustPostCountFn(ctx, q, hashtagID, delta)
	}
	return nil
}
func (m *mockHashtagRepo) ListByPostID(ctx context.Context, q gen.DBTX, postID int64) ([]gen.CmsPostHashtag, error) {
	if m.listByPostIDFn != nil {
		return m.listByPostIDFn(ctx, q, postID)
	}
	return nil, nil
}
func (m *mockHashtagRepo) ListByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]gen.CmsPostHashtag, error) {
	if m.listByPostIDsFn != nil {
		return m.listByPostIDsFn(ctx, postIDs)
	}
	return nil, nil
}
func (m *mockHashtagRepo) ListHashtagsByIDs(ctx context.Context, ids []int64) (map[int64]gen.CmsHashtag, error) {
	if m.listHashtagsByIDsFn != nil {
		return m.listHashtagsByIDsFn(ctx, ids)
	}
	return nil, nil
}
func (m *mockHashtagRepo) ListHashtagIDsByPostID(ctx context.Context, q gen.DBTX, postID int64) ([]int64, error) {
	if m.listHashtagIDsByPostFn != nil {
		return m.listHashtagIDsByPostFn(ctx, q, postID)
	}
	return nil, nil
}
func (m *mockHashtagRepo) InsertBinding(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error {
	if m.insertBindingFn != nil {
		return m.insertBindingFn(ctx, q, postID, hashtagID, displayName, position)
	}
	return nil
}
func (m *mockHashtagRepo) UpdateBinding(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error {
	if m.updateBindingFn != nil {
		return m.updateBindingFn(ctx, q, postID, hashtagID, displayName, position)
	}
	return nil
}
func (m *mockHashtagRepo) DeleteBindings(ctx context.Context, q gen.DBTX, postID int64, hashtagIDs []int64) error {
	if m.deleteBindingsFn != nil {
		return m.deleteBindingsFn(ctx, q, postID, hashtagIDs)
	}
	return nil
}
func (m *mockHashtagRepo) DeleteAllBindings(ctx context.Context, q gen.DBTX, postID int64) error {
	if m.deleteAllBindingsFn != nil {
		return m.deleteAllBindingsFn(ctx, q, postID)
	}
	return nil
}
func (m *mockHashtagRepo) SearchByPrefix(ctx context.Context, prefix string, limit int32) ([]gen.SearchHashtagsByPrefixRow, error) {
	if m.searchByPrefixFn != nil {
		return m.searchByPrefixFn(ctx, prefix, limit)
	}
	return nil, nil
}
func (m *mockHashtagRepo) SearchByContain(ctx context.Context, query string, limit int32) ([]gen.SearchHashtagsByContainRow, error) {
	if m.searchByContainFn != nil {
		return m.searchByContainFn(ctx, query, limit)
	}
	return nil, nil
}
func (m *mockHashtagRepo) PopularInRange(ctx context.Context, start, end time.Time, limit int32) ([]gen.SelectPopularHashtagsInRangeRow, error) {
	if m.popularInRangeFn != nil {
		return m.popularInRangeFn(ctx, start, end, limit)
	}
	return nil, nil
}
