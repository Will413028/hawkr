package domain

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

type PostRepository interface {
	GetByID(ctx context.Context, id int64) (*gen.CmsPost, error)
	// GetByIDTx is GetByID bound to a caller-supplied tx handle (mirrors the
	// gen.DBTX pattern HashtagRepository already uses) rather than borrowing a
	// second connection from the pool. It exists for exactly one caller today:
	// UpdatePost's sameTx closure (#2772 review round 2, FIX 2) needs the
	// post's current publish_status/deleted_at/hidden_at *after* the UPDATE has
	// taken the row's lock, because PublishScheduledPost (db/queries/
	// post_schedule.sql) can flip publish_status without bumping version — a
	// successful CAS on UpdatePost's own UPDATE does not prove nothing else
	// changed. Reading through the pool here would race the still-open tx
	// (Task 3's ListByPostID hit the same connection-pool deadlock class); a
	// plain GetByID(ctx, id) call from inside that closure is stale by
	// construction, not merely inconvenient.
	GetByIDTx(ctx context.Context, q gen.DBTX, id int64) (*gen.CmsPost, error)
	List(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsPost, error)
	Count(ctx context.Context, sellerID int64) (int64, error)
	ListMy(ctx context.Context, sellerID int64, status string, limit, offset int64) ([]*gen.CmsPost, error)
	CountMy(ctx context.Context, sellerID int64, status string) (int64, error)
	AdminList(ctx context.Context, status string, limit, offset int64) ([]gen.AdminListPostsRow, error)
	AdminCount(ctx context.Context, status string) (int64, error)
	HidePost(ctx context.Context, id int64, reason string) error
	RestorePost(ctx context.Context, id int64) error
	// publishStatus is written explicitly rather than left to the column
	// default: the default is PUBLISHED, and a delegated create must be a
	// draft (56 §6.1.9).
	//
	// sameTx runs inside the same transaction as the INSERT and is the hashtag
	// work's landing point (#2773): it used to run after the INSERT's own
	// autocommit, so a failure left "API returned failure, post already
	// committed" with no reconciliation job allowed to find it (責任邊界規範).
	// A nil sameTx means there is nothing to do alongside.
	Create(ctx context.Context, id, sellerID int64, content string, imageURL string, imageAssetID *int64,
		productID *int64, publishStatus int16, sameTx func(ctx context.Context, q gen.DBTX) error) (*gen.CmsPost, error)
	// expectedVersion 一律非 nil：呼叫端必須傳進入時讀到的 version（#2772）。
	// 保留指標型別只是為了與 SetPublishStatus / SchedulePublish 的代行路徑一致；
	// 傳 nil 會停用 CAS，那是本票的成因，不得再出現在貼文更新路徑上。
	//
	// sameTx runs inside the same transaction as the UPDATE and is the hashtag
	// work's landing point, same shape as Create. A nil sameTx means there is
	// nothing to do alongside.
	Update(ctx context.Context, id int64, content string, imageURL string, imageAssetID *int64,
		productID *int64, expectedVersion *int64,
		sameTx func(ctx context.Context, q gen.DBTX) error) error
	// SetPublishStatus is the interactive publish / unpublish command (56 §6.1.9).
	//
	// sameTx runs inside the same transaction as the status change and only when
	// the CAS actually won: the hashtag counters move with visibility, and a
	// counter adjusted in a second transaction leaves a post whose tags are
	// counted one off after a crash — with no reconciliation job allowed to find
	// it (責任邊界規範). A nil sameTx means there is nothing to do alongside.
	SetPublishStatus(ctx context.Context, id int64, publishStatus int16, now time.Time,
		expectedVersion *int64, sameTx func(ctx context.Context, q gen.DBTX) error) error
	// SchedulePublish creates or re-times a pending schedule (56 §7.6.1 T0). It
	// does not touch publish_status: the post stays a draft until the worker's
	// terminal write. The statement also refuses an already-published row, and
	// that case surfaces as ErrVersionConflict like any other lost CAS — the
	// readable rejection is the caller's read-first check; this is the guard for
	// the window between that read and this write.
	SchedulePublish(ctx context.Context, id int64, scheduledAt time.Time, sessionID int64,
		now time.Time, expectedVersion *int64) error
	// SoftDelete 回傳 affected rows，不是只回 error。
	//
	// 述詞是 `id = $1 AND seller_id = $2 AND deleted_at IS NULL`，所以 0 列同時涵蓋
	// 「不屬於這個賣家」與「已經刪過了」。呼叫端必須在 0 時中止：counter 會被扣兩次，
	// 而 AdjustHashtagPostCount 沒有 GREATEST(0, …) 下限（#2757）。
	SoftDelete(ctx context.Context, id, sellerID int64) (int64, error)
	IncrViewCount(ctx context.Context, id int64) error
	IncrShareCount(ctx context.Context, id int64) error
	IncrLikeCount(ctx context.Context, id int64) error
	DecrLikeCount(ctx context.Context, id int64) error
	IncrCommentCount(ctx context.Context, id int64) error
	DecrCommentCount(ctx context.Context, id int64) error
	GetLike(ctx context.Context, postID int64, kind int16, userID int64) (*gen.CmsPostLike, error)
	CreateLike(ctx context.Context, postID int64, kind int16, userID int64) error
	DeleteLike(ctx context.Context, postID int64, kind int16, userID int64) error
	BatchGetLikedPostIDs(ctx context.Context, postIDs []int64, kind int16, userID int64) ([]int64, error)
	SoftDeleteBySellerID(ctx context.Context, sellerID int64) error
	// ListPostIDsBySellerID 回傳該賣家的全部貼文 id（含已軟刪列），供
	// seller.deleted handler 批次釋放媒體引用。述詞不看 deleted_at，所以 MQ
	// 重投時回的是同一個集合——理由見 posts.sql 的同名查詢註解。
	ListPostIDsBySellerID(ctx context.Context, sellerID int64) ([]int64, error)
	DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error
	ReconcileLikeCounts(ctx context.Context) error
	ReconcileCommentCounts(ctx context.Context) error
	ListByHashtag(ctx context.Context, tagName string, limit, offset int64) ([]*gen.CmsPost, error)
	CountByHashtag(ctx context.Context, tagName string) (int64, error)
}

// HashtagRepository wraps cms_hashtag + cms_post_hashtag access.
//
// Mutation methods accept a gen.DBTX so the caller may pass a pgx.Tx for
// transactional flows; nil falls back to the pool. This is the first repo in
// social-service to use the tx-aware pattern — motivated by spec 34 §4
// (counter delta MUST be same-tx as the binding insert/delete to avoid drift).
// Other repos should adopt this pattern as they gain tx requirements.
//
// **Only ListHashtagIDsByPostID and ListByPostID accept a tx.** Both are read
// BEFORE any write in their callers (Apply's diff step), so read-after-write
// is not a concern for them. The remaining reads always run on the pool —
// callers that need to enumerate post-write state for those should compute it
// in-memory from the diff they just applied (see spec 34 §4 counter-delta flow
// + Task 6 Apply implementation).
//
// **Counter delta deadlock-freedom**: callers that invoke AdjustPostCount for
// multiple hashtag IDs in the same tx MUST sort the IDs ASC and call this
// method in that order (spec 34 §4.2). The wrapper does not enforce this —
// it's the caller's responsibility.
type HashtagRepository interface {
	// Mutations (may participate in tx).
	UpsertByName(ctx context.Context, q gen.DBTX, id int64, canonical string) (int64, error)
	// AdjustPostCount: positive delta increases, negative decreases. Caller MUST
	// invoke in hashtag.id ASC order across a batch to avoid deadlock (§4.2).
	AdjustPostCount(ctx context.Context, q gen.DBTX, hashtagID int64, delta int32) error
	InsertBinding(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error
	UpdateBinding(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error
	DeleteBindings(ctx context.Context, q gen.DBTX, postID int64, hashtagIDs []int64) error
	DeleteAllBindings(ctx context.Context, q gen.DBTX, postID int64) error
	// ListHashtagIDsByPostID and ListByPostID are the only **reads** that
	// participate in tx — used by Apply (#2773 / Task 6) to read pre-write
	// binding state inside the caller's transaction, so a concurrent writer
	// racing the same tx cannot be missed nor double-counted.
	// ListHashtagIDsByPostID returns IDs in ASC order so callers can use the
	// result directly for deadlock-free counter delta.
	ListHashtagIDsByPostID(ctx context.Context, q gen.DBTX, postID int64) ([]int64, error)
	ListByPostID(ctx context.Context, q gen.DBTX, postID int64) ([]gen.CmsPostHashtag, error)

	// Reads (always pool-bound, NOT tx-aware — see type doc).
	ListByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]gen.CmsPostHashtag, error)
	ListHashtagsByIDs(ctx context.Context, ids []int64) (map[int64]gen.CmsHashtag, error)
	SearchByPrefix(ctx context.Context, prefix string, limit int32) ([]gen.SearchHashtagsByPrefixRow, error)
	SearchByContain(ctx context.Context, query string, limit int32) ([]gen.SearchHashtagsByContainRow, error)
	PopularInRange(ctx context.Context, start, end time.Time, limit int32) ([]gen.SelectPopularHashtagsInRangeRow, error)
}

type CommentRepository interface {
	GetByID(ctx context.Context, id int64) (*gen.CmsComment, error)
	ListTopLevel(ctx context.Context, postID, limit, offset int64) ([]*gen.CmsComment, error)
	CountTopLevel(ctx context.Context, postID int64) (int64, error)
	ListReplies(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsComment, error)
	CountReplies(ctx context.Context, parentID int64) (int64, error)
	GetLatestReplies(ctx context.Context, parentID, limit int64) ([]*gen.CmsComment, error)
	BatchGetLatestReplies(ctx context.Context, parentIDs []int64, limit int32) ([]*gen.CmsComment, error)
	Create(ctx context.Context, id, postID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsComment, error)
	UpdateContent(ctx context.Context, id int64, content string) (*gen.CmsComment, error)
	SoftDelete(ctx context.Context, id int64) error
	HideComment(ctx context.Context, id int64, reason string) error
	IncrReplyCount(ctx context.Context, id int64) error
	DecrReplyCount(ctx context.Context, id int64) error
	IncrLikeCount(ctx context.Context, id int64) error
	DecrLikeCount(ctx context.Context, id int64) error
	GetLike(ctx context.Context, commentID int64, kind int16, userID int64) (*gen.CmsCommentLike, error)
	CreateLike(ctx context.Context, commentID int64, kind int16, userID int64) error
	DeleteLike(ctx context.Context, commentID int64, kind int16, userID int64) error
	BatchGetLikedCommentIDs(ctx context.Context, commentIDs []int64, kind int16, userID int64) ([]int64, error)
	SoftDeleteByActor(ctx context.Context, kind int16, userID int64) error
	DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error
	ReconcileLikeCounts(ctx context.Context) error
	ReconcileReplyCounts(ctx context.Context) error
}

type ForumRepository interface {
	GetByID(ctx context.Context, id int64) (*gen.CmsSellerForum, error)
	ListTopLevel(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsSellerForum, error)
	CountTopLevel(ctx context.Context, sellerID int64) (int64, error)
	ListReplies(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsSellerForum, error)
	CountReplies(ctx context.Context, parentID int64) (int64, error)
	Create(ctx context.Context, id, sellerID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsSellerForum, error)
	UpdateContent(ctx context.Context, id int64, content string) (*gen.CmsSellerForum, error)
	SoftDelete(ctx context.Context, id int64) error
	HideEntry(ctx context.Context, id int64, reason string) error
	IncrReplyCount(ctx context.Context, id int64) error
	DecrReplyCount(ctx context.Context, id int64) error
	SoftDeleteByActor(ctx context.Context, kind int16, userID int64) error
}

type ReportRepository interface {
	// Legacy POST-only facade used by usecase/post.go ReportPost.
	GetReport(ctx context.Context, postID, reporterID int64) (*gen.Report, error)
	CreateReport(ctx context.Context, id, postID, reporterID int64, reasonCode string, description string) (*gen.Report, error)

	// Generic B-135 API.
	GetByReporterAndTarget(ctx context.Context, reporterID int64, targetType string, targetID int64) (*gen.Report, error)
	CreateGenericReport(ctx context.Context, p CreateReportParams) (*gen.Report, error)
	ListByReporter(ctx context.Context, reporterID int64, limit, offset int64) ([]*gen.Report, error)
	AdminList(ctx context.Context, status, targetType, reasonCode string, limit, offset int64) ([]*gen.Report, error)
	AdminGet(ctx context.Context, id int64) (*gen.Report, error)
	AdminResolve(ctx context.Context, id int64, status, action string, resolvedBy int64) (*gen.Report, error)
	CountByReporterSince(ctx context.Context, reporterID int64, since time.Time) (int64, error)
	Reopen(ctx context.Context, id int64) (*gen.Report, error)
	CountRecentActionsByOwner(ctx context.Context, ownerID int64) (int64, error)
}

// TargetOwnerResolver resolves the owning user id for a target, used to
// snapshot reports.target_owner_id at submission/admin-direct-action time.
// Only in-process target types (POST/POST_COMMENT/SELLER_FORUM/USER) resolve;
// REVIEW/CHAT_MESSAGE return 0 (unknown) until review-service/im-service
// expose an owner-lookup RPC — no client submits reports for those types yet.
type TargetOwnerResolver interface {
	ResolveOwner(ctx context.Context, targetType string, targetID int64) (int64, error)
}

// CreateReportParams holds the fields for SubmitReport (D17 6 種 targetType).
type CreateReportParams struct {
	ID             int64
	ReporterID     int64
	TargetType     string
	TargetID       int64
	TargetOwnerID  int64 // 0 = unknown（跨服務 target 尚未接 owner lookup）
	ReasonCode     string
	Description    string
	Evidence       []byte // JSONB raw bytes
	TargetSnapshot []byte // JSONB raw bytes (nullable)
}

type BlockRepository interface {
	Create(ctx context.Context, id, blockerID, blockedID int64) (*gen.UserBlock, error)
	Delete(ctx context.Context, blockerID, blockedID int64) error
	Get(ctx context.Context, blockerID, blockedID int64) (*gen.UserBlock, error)
	ListByBlocker(ctx context.Context, blockerID int64, limit, offset int64) ([]*gen.UserBlock, error)
}

// ModerationActionRepository writes audit log entries for B-135 moderation
// actions (auto-reject, admin resolve, etc).
type ModerationActionRepository interface {
	Create(ctx context.Context, p ModerationActionParams) error
}

// ModerationActionParams describes a single audit-log entry. Action values:
// AUTO_REJECT (dictionary hit, Phase 2) / HIDE_TARGET / WARN_USER / BAN_USER / DISMISS / REOPEN.
type ModerationActionParams struct {
	ID               int64
	ReportID         int64 // 0 = no associated report
	AdminID          int64 // 0 = system action (e.g. AUTO_REJECT)
	AffectedUserID   int64 // 0 = unknown
	Action           string
	Note             string
	AttemptedContent string // for AUTO_REJECT records
	PreviousState    []byte // JSONB raw bytes, nullable
	NewState         []byte // JSONB raw bytes, nullable
}
