package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/moderation"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/domain/hashtag"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"go.uber.org/zap"
)

type PostUseCase struct {
	postRepo       domain.PostRepository
	hashtagUC      *HashtagUseCase
	reportRepo     domain.ReportRepository
	reportUC       *ReportUseCase
	mediaClient    MediaClient
	sellerClient   SellerClient
	buyerClient    BuyerClient
	productClient  ProductClient
	eventPublisher ContentEventPublisher
	revalidate     *revalidate.Client
	tsid           func() int64
	mediaBaseURL   string
}

func NewPostUseCase(
	postRepo domain.PostRepository,
	hashtagUC *HashtagUseCase,
	reportRepo domain.ReportRepository,
	reportUC *ReportUseCase,
	mediaClient MediaClient,
	sellerClient SellerClient,
	buyerClient BuyerClient,
	productClient ProductClient,
	eventPublisher ContentEventPublisher,
	cfg *config.Config,
	tsid func() int64,
	revalidateClient *revalidate.Client,
) *PostUseCase {
	return &PostUseCase{
		postRepo:       postRepo,
		hashtagUC:      hashtagUC,
		reportRepo:     reportRepo,
		reportUC:       reportUC,
		mediaClient:    mediaClient,
		sellerClient:   sellerClient,
		buyerClient:    buyerClient,
		productClient:  productClient,
		eventPublisher: eventPublisher,
		revalidate:     revalidateClient,
		mediaBaseURL:   cfg.MediaBaseURL,
		tsid:           tsid,
	}
}

// bustPost busts the website caches that embed a post: post:<id> (which the
// revalidate boundary expands to posts:list, the public feed) plus one
// hashtag:<id> per hashtag whose post_count moved, which expands to
// hashtags:list — the trending rail (後端服務責任邊界規範 §9.3).
//
// Only entity tags are sent; domain-collection tags are the web boundary's job
// (§9.8). Best-effort and fire-and-forget: BustAsync never blocks or fails the
// mutation, and a nil/unconfigured client is a no-op.
//
// hashtagIDs is the set whose counters actually changed, not every tag on the
// post: an edit that leaves the tags alone does not reorder the trending list.
func (uc *PostUseCase) bustPost(ctx context.Context, postID int64, hashtagIDs ...int64) {
	if !uc.revalidate.Enabled() {
		return
	}
	tags := append([]string{revalidate.PostTag(postID)}, revalidate.HashtagTags(hashtagIDs...)...)
	uc.revalidate.BustAsync(ctx, tags)
}

func (uc *PostUseCase) GetPost(ctx context.Context, postID int64) (*gen.CmsPost, error) {
	logger.Info(ctx, "查詢貼文", zap.Int64("post_id", postID))
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil {
		logger.Warn(ctx, "貼文不存在", zap.Int64("post_id", postID))
		return nil, ErrNotFound
	}
	logger.Info(ctx, "查詢貼文成功", zap.Int64("post_id", postID), zap.Int64("seller_id", post.SellerID))
	return post, nil
}

func (uc *PostUseCase) ListPosts(ctx context.Context, sellerID int64, page, pageSize int32) ([]*gen.CmsPost, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	posts, err := uc.postRepo.List(ctx, sellerID, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.postRepo.Count(ctx, sellerID)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

// checkPublishable 是「公開中的貼文恰好 1 張照片、內文必填」的唯一檢查（B1）。
// 照片數量上限（多於 1 張一律拒絕）在 gRPC 邊界，因為 usecase 只收單一 token。
func checkPublishable(content string, hasImage bool) error {
	if !hasImage {
		return ErrPostImageRequired
	}
	if strings.TrimSpace(content) == "" {
		return ErrPostContentBlank
	}
	return nil
}

func checkPublishableRow(post *gen.CmsPost) error {
	return checkPublishable(stringOrEmpty(post.Content), stringOrEmpty(post.Image) != "" || post.ImageAssetID != nil)
}

// CreatePost writes a new post. publishStatus decides whether it is visible to
// buyers straight away: the seller's own path passes PUBLISHED (the historical
// behaviour), a delegated create passes DRAFT because publishing is a separate
// capability behind its own endpoint (56 §6.1.9).
func (uc *PostUseCase) CreatePost(ctx context.Context, sellerID int64, content string, stagedToken string, productID *int64, tags []string, publishStatus int16) (*gen.CmsPost, error) {
	// B1：以已發布建立等於進入公開，要有 1 張照片與內文；草稿可以不完整。
	if publishStatus == domain.PostPublishStatusPublished {
		if err := checkPublishable(content, stagedToken != ""); err != nil {
			return nil, err
		}
	}
	// B-135: profanity dictionary
	if hit, _ := moderation.Check(content); hit {
		return nil, ErrModerationRejected
	}
	// Validate hashtags before any DB write (fail-fast). RequireSet returns
	// (nil, nil) for empty input, so this is a no-op when tags is nil/empty.
	if _, err := hashtag.RequireSet(tags); err != nil {
		return nil, err
	}
	postID := uc.tsid()
	imageURL, imageAssetID, err := uc.finalizeAndLinkAsset(ctx, stagedToken, media.RefTypePost, postID)
	if err != nil {
		return nil, fmt.Errorf("finalize image: %w", err)
	}

	// #2773：hashtag 進同一個交易。原本它跑在 INSERT 之後的獨立自動提交裡，任一步失敗
	// 就留下「API 回失敗、貼文已提交」——沒有任何 reconciliation job 被允許去找它。
	var touchedHashtags []int64
	post, err := uc.postRepo.Create(ctx, postID, sellerID, content, imageURL, imageAssetID,
		productID, publishStatus,
		func(ctx context.Context, q gen.DBTX) error {
			if len(tags) == 0 {
				return nil
			}
			// A fresh row has deleted_at + hidden_at both null, so public
			// visibility reduces to publish_status alone: a seller-authored post
			// is PUBLISHED and counts, a delegated draft does not — otherwise a
			// tag only ever used by a draft shows up in the public autocomplete
			// (post_count > 0 is its predicate).
			//
			// Reads publishStatus (the input), not the row Create will return:
			// post isn't in scope yet here — this closure is built as part of
			// the same `post, err := uc.postRepo.Create(...)` statement that
			// declares post, and Go does not make an identifier usable on the
			// right-hand side of its own short declaration. The input and the
			// row are equal in practice (CreatePost's SQL writes publish_status
			// verbatim, no server-side override), and using it here also keeps
			// this callback's shape aligned with SetPublishStatus's sameTx for
			// Task 4 to reuse.
			res, herr := uc.hashtagUC.Apply(ctx, q, postID, tags,
				publishStatus == domain.PostPublishStatusPublished)
			if herr != nil {
				return herr
			}
			touchedHashtags = res.Added
			return nil
		})
	if err != nil {
		return nil, err
	}

	// A newly published post enters the public feed immediately, so the feed cache
	// must not wait out its TTL. Fired after the writes above so a failed create
	// never busts. A draft busts nothing meaningful — the feed query filters it
	// out — but the extra tag costs one cache miss, not correctness.
	uc.bustPost(ctx, post.ID, touchedHashtags...)

	// Best-effort: fan-out one post.created event per follower for push notifications
	if uc.eventPublisher != nil && uc.buyerClient != nil {
		preview := content
		if len(preview) > domain.PreviewMaxLength {
			preview = preview[:domain.PreviewMaxLength]
		}
		var sellerName, sellerAvatar string
		if info, serr := uc.sellerClient.GetSellerInfo(ctx, sellerID); serr == nil && info != nil {
			sellerName = info.Nickname
			// 49 §6.3/S3：post.created MQ payload 帶 objectKey（seller_avatar_key），
			// 消費端（notification worker）於 push 前組 URL；服務側不組。
			sellerAvatar = info.Avatar
		}
		subscriberIDs, serr := uc.buyerClient.GetSubscriberIDs(ctx, sellerID)
		if serr != nil {
			logger.Warn(ctx, "failed to get subscriber IDs for post fan-out", zap.Error(serr))
		} else {
			for _, followerID := range subscriberIDs {
				_ = uc.eventPublisher.PublishPostCreated(ctx, post.ID, sellerID, followerID, sellerName, sellerAvatar, preview)
			}
		}
	}

	return post, nil
}

// expectedVersion is set on a delegated write, where a mismatch must surface
// as 9266 instead of overwriting whatever the seller changed in the meantime
// (56 §5.4). #2772: CAS is now mandatory on every path, not just the
// delegated one — see the nil-defaulting below.
func (uc *PostUseCase) UpdatePost(ctx context.Context, postID, sellerID int64, content string, keepAssetID string, newStagedToken string, productID *int64, tagsPresent bool, tags []string, expectedVersion *int64) (*gen.CmsPost, error) {
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, ErrNotFound
	}
	if post.SellerID != sellerID {
		return nil, ErrForbidden
	}

	// #2772：賣家自助路徑原本傳 nil ⇒ CAS 停用 ⇒ 兩個併發換圖都提交。
	// 代行路徑傳進來的 expectedVersion 優先；自助路徑改用進入時讀到的版本。
	if expectedVersion == nil {
		v := post.Version
		expectedVersion = &v
	}

	// PATCH semantics for tags:
	//   tagsPresent=false           → no hashtag touch
	//   tagsPresent=true, tags=[]   → clear all (Apply normalizes empty → diff removes all)
	//   tagsPresent=true, tags=[..] → diff/apply (counter delta only if visible)
	// Validate-first when caller marks tags as present (fail-fast).
	if tagsPresent {
		if _, herr := hashtag.RequireSet(tags); herr != nil {
			return nil, herr
		}
	}

	var imageURL string
	var imageAssetID *int64
	// 這次更新有沒有動到「這一列宣告了哪個資產」。只有動到時才需要在 commit 之後
	// 送 ③ Release（順序契約見 pkg/media.LinkImages）：內容編輯的 desired 與 commit
	// 前一模一樣，送出去只是一次確定的 no-op RPC。
	var releaseRefs bool
	var keepAssetIDs []string
	switch {
	case newStagedToken != "":
		// #1489 / #2222: the post image is single-image under ref_id=postID.
		// 先掛新圖（失敗即中止），舊圖引用在 commit 之後由本函式尾端的 ③ 釋放。
		key, assetID, ferr := uc.finalizeAndLinkAsset(ctx, newStagedToken, media.RefTypePost, postID)
		if ferr != nil {
			return nil, fmt.Errorf("finalize image: %w", ferr)
		}
		imageURL, imageAssetID = key, assetID
		if assetID != nil {
			releaseRefs, keepAssetIDs = true, []string{strconv.FormatInt(*assetID, 10)}
		}
	case keepAssetID != "":
		// post.Image / post.ImageAssetID are read from the same pre-fetch as the
		// hashtag visibility check further down, and are safe for the same
		// reason: posts.sql's UpdatePost is the only query that ever writes
		// these two columns on an existing row (CreatePost only writes them on
		// INSERT), and it always bumps version — so a successful CAS on THIS
		// call already proves no other UpdatePost committed between this
		// GetByID and now. This is the half of the staleness argument that
		// actually matters for reference lifetime: get it wrong here — thread
		// through a stale or nil asset id — and an ordinary content-only edit
		// declares an empty desired set, and the ③ below would release the
		// reference to an image the post still displays (#2772 inverted).
		imageURL, imageAssetID = stringOrEmpty(post.Image), post.ImageAssetID
	default:
		// 清圖：image 與 image_asset_id 一起清空，這一列宣告的 desired 因此是空集合，
		// commit 之後送 keep=nil 把引用全部釋放。#2772 的第二半原本完全沒有釋放。
		//
		// 條件用「舊宣告非空」而不是「舊 image_asset_id 非 NULL」，才涵蓋得到 legacy 列
		// （image_asset_id 是 NULL 但 image 有值，回填前的舊資料）：那種列在 media 上
		// 可能有引用，使用者把圖清掉之後 desired 確實是空集合。這與 column_contract.md
		// 禁止把 legacy 列放進 reconciler 候選集合**不是同一件事**——那裡 desired 是
		// 「讀不出來」（貼文還顯示著那張圖），這裡是「使用者剛把它清掉」。
		imageURL, imageAssetID = "", nil
		releaseRefs = post.ImageAssetID != nil || stringOrEmpty(post.Image) != ""
	}

	// B1：已發布或排程中的貼文要持續滿足進入公開的檢查（照片與內文），
	// 否則排程中的貼文可以在 worker 發布之前被清空。
	if post.PublishStatus == domain.PostPublishStatusPublished ||
		(post.ScheduleState != nil && *post.ScheduleState == domain.ScheduleStatePending) {
		if err := checkPublishable(content, imageURL != "" || imageAssetID != nil); err != nil {
			return nil, err
		}
	}

	// #2773/#2772：hashtag 進同一個交易，理由同 CreatePost。
	//
	// visible 不能沿用 GetByID 當初讀到的 post（#2772 review round 2, FIX 2）：那次讀取
	// 在 UPDATE 之前、沒有拿任何鎖，而 PublishScheduledPost（db/queries/
	// post_schedule.sql）明白宣告自己不比較版本、不 bump version，卻會把 publish_status
	// 從草稿翻成已發布——夾在 GetByID 與這裡的 UPDATE 之間插進來，「這次 UPDATE 的 CAS
	// 贏了」並不能證明 publish_status 沒變。命中時的後果：一個實際已發布的貼文用
	// visible=false 綁標籤，hashtag.post_count 少算，該標籤從 post_count > 0 的自動完成
	// 清單消失，touchedHashtags 也是空的，trending rail 不會被 bust。窗口本身很窄
	// （FinalizeAndLink 的 gRPC 往返夾在中間，最長到 finalize 10s + link 15.6s），今天
	// 只能透過代行排程路徑碰到（互動式 publish/unpublish 都會 bump version），且
	// DELEGATION_ENABLED 預設 false、replicas=1——但窄不等於不存在，用 tx 內重讀關掉它，
	// 不是靠這些條件把它記成「不可達」。
	//
	// 讀法：UPDATE 陳述式此刻已經拿到這一列的鎖，任何會動 publish_status 的並發寫入
	// （含 PublishScheduledPost）若還沒 commit 就會卡在那個鎖後面，若已經 commit 則
	// READ COMMITTED 下這一句直接看得到——兩種情況都不會看到「半途」的狀態。用的是
	// postRepo.GetByIDTx（同一個 q，不重新借連線），不是 postRepo.GetByID：後者會從
	// pool 借第二條連線、卡在已開啟但尚未 commit 的這個 tx 之外——Task 3 的 hashtag.go
	// 已經因為同一種連線池死結類別修過一次（ListByPostID 改吃 tx handle），這裡不重踩。
	var touchedHashtags []int64
	if err := uc.postRepo.Update(ctx, postID, content, imageURL, imageAssetID, productID, expectedVersion,
		func(ctx context.Context, q gen.DBTX) error {
			if !tagsPresent {
				return nil
			}
			cur, gerr := uc.postRepo.GetByIDTx(ctx, q, postID)
			if gerr != nil {
				return gerr
			}
			visible := cur.PublishStatus == domain.PostPublishStatusPublished &&
				cur.DeletedAt == nil && cur.HiddenAt == nil
			res, herr := uc.hashtagUC.Apply(ctx, q, postID, tags, visible)
			if herr != nil {
				return herr
			}
			// Counters only moved when the post is publicly visible; a hidden post's
			// tag edit changes no ranking, so it must not bust the trending rail.
			if visible {
				touchedHashtags = append(append([]int64{}, res.Added...), res.Removed...)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	// ③ 釋放舊引用。順序契約見 pkg/media.LinkImages：① Link（致命）→ ② 業務列寫入
	// （致命）→ ③ Release（best-effort），六個媒體擁有者共用同一條，貼文不再是例外。
	//
	// 為什麼放在 commit 之後、而且失敗只 Warn：這一步唯一能造成的損害是「舊資產的引用
	// 沒被拿掉」＝ 該資產永遠不被 GC 回收 ＝ 儲存洩漏，可事後用維運 SQL 清。反過來
	// 若排在 commit 之前、或把失敗當致命回滾，就會出現「業務列指著、卻沒有引用」的
	// 窗口，那種資產會被 media GC 連同 R2 物件實體刪除，事後無跡可循（2026-08 dev 上
	// 16 個賣家 logo 即因此消失）。洩漏可逆、實刪不可逆，方向只能是這一邊。
	//
	// keep 直接用這次寫進 image_asset_id 的值：換圖 = [新資產]（media-service 在單一
	// 交易內刪舊留新，不存在新資產一度沒有引用的窗口）；清圖 = nil = 全部釋放。
	if releaseRefs {
		if err := uc.mediaClient.ReleaseReferences(ctx, media.RefTypePost,
			[]string{strconv.FormatInt(postID, 10)}, keepAssetIDs); err != nil {
			logger.Warn(ctx, "release post media references failed",
				zap.Int64("post_id", postID), zap.Strings("keep_asset_ids", keepAssetIDs), zap.Error(err))
		}
	}

	uc.bustPost(ctx, postID, touchedHashtags...)

	return uc.postRepo.GetByID(ctx, postID)
}

// UpdatePublishStatusInput is one publish / unpublish / schedule command
// (56 §6.1.9). It is a struct rather than a parameter list because the three
// commands differ only in which fields are set, and a positional signature would
// make "schedule" and "publish now" one nil apart at every call site.
type UpdatePublishStatusInput struct {
	PostID   int64
	SellerID int64
	// TargetStatus is PUBLISHED or UNPUBLISHED; DRAFT is not a publish command
	// and is rejected at the RPC boundary.
	TargetStatus int16
	// ScheduledAt nil = publish now. Non-nil is only legal with TargetStatus
	// PUBLISHED, which the RPC boundary has already checked.
	ScheduledAt *time.Time
	// ScheduledBySessionID is the delegation session that owns the schedule. It
	// is the audit link the worker uses at execution time (§5.1.5), so a
	// schedule cannot be created without one.
	ScheduledBySessionID int64
	ExpectedVersion      *int64
	// Delegated picks which denial an ownership mismatch is: 9275 for a delegated
	// operator reaching another store's post, plain forbidden for the seller's
	// own path.
	Delegated bool
}

// UpdatePublishStatus runs one publish / unpublish / schedule command.
//
// The read comes first because both branches need the row's current state: the
// ownership assertion (Q4, §7.1.2 — GetPost has no seller filter of its own),
// and the visibility delta that decides whether the post's hashtags gain or lose
// a count. Neither can be derived from the request alone.
func (uc *PostUseCase) UpdatePublishStatus(ctx context.Context, in UpdatePublishStatusInput) (*gen.CmsPost, error) {
	post, err := uc.postRepo.GetByID(ctx, in.PostID)
	if err != nil {
		return nil, ErrNotFound
	}
	if post.SellerID != in.SellerID {
		if in.Delegated {
			return nil, domain.ErrResourceNotOwned
		}
		return nil, ErrForbidden
	}

	// B1：發布與排程發布都是進入公開，要有 1 張照片與內文（下面兩處檢查）。下架不檢查；
	// 已發布的貼文再發布不是進入公開，照舊。
	if in.ScheduledAt != nil {
		// Scheduling an already-published post would have the worker's terminal
		// write add a second +1 to every hashtag on it: that write adjusts the
		// counters unconditionally, and what makes that correct is the invariant
		// that a pending row is always a draft (§5.1.5).
		if post.PublishStatus == domain.PostPublishStatusPublished {
			return nil, ErrPostAlreadyPublished
		}
		if err := checkPublishableRow(post); err != nil {
			return nil, err
		}
		if err := uc.postRepo.SchedulePublish(ctx, in.PostID, *in.ScheduledAt,
			in.ScheduledBySessionID, time.Now().UTC(), in.ExpectedVersion); err != nil {
			return nil, err
		}
		// No cache bust: nothing a buyer can see changed. The post is still a
		// draft, and busting here would be a write to the website's cache for a
		// state transition it cannot render.
		return uc.postRepo.GetByID(ctx, in.PostID)
	}

	if in.TargetStatus == domain.PostPublishStatusPublished && post.PublishStatus != domain.PostPublishStatusPublished {
		if err := checkPublishableRow(post); err != nil {
			return nil, err
		}
	}

	// Public visibility, not publish_status alone: a post that is hidden by
	// governance or deleted contributes nothing to its hashtags' counts in either
	// state, so publishing or unpublishing it must move no counter at all.
	// Deriving the delta from the status change alone would decrement a count the
	// post never contributed, and AdjustPostCount has no floor at zero.
	wasVisible := post.PublishStatus == domain.PostPublishStatusPublished &&
		post.DeletedAt == nil && post.HiddenAt == nil
	willBeVisible := in.TargetStatus == domain.PostPublishStatusPublished &&
		post.DeletedAt == nil && post.HiddenAt == nil

	var delta int32
	switch {
	case !wasVisible && willBeVisible:
		delta = 1
	case wasVisible && !willBeVisible:
		delta = -1
	}

	var touchedHashtags []int64
	err = uc.postRepo.SetPublishStatus(ctx, in.PostID, in.TargetStatus, time.Now().UTC(), in.ExpectedVersion,
		func(txCtx context.Context, q gen.DBTX) error {
			adjusted, cerr := uc.hashtagUC.AdjustCountersForVisibilityChange(txCtx, q, in.PostID, delta)
			touchedHashtags = adjusted
			return cerr
		})
	if err != nil {
		return nil, err
	}

	uc.bustPost(ctx, in.PostID, touchedHashtags...)

	return uc.postRepo.GetByID(ctx, in.PostID)
}

func (uc *PostUseCase) DeletePost(ctx context.Context, postID, sellerID int64) error {
	// Need to know if post was public-visible BEFORE delete to decide counter delta.
	// GetByID filters out soft-deleted rows, so deleted_at is already known null.
	// publish_status must be in the predicate: deleting a draft that was never
	// counted would otherwise -1 a counter it never contributed to, and
	// AdjustHashtagPostCount has no GREATEST(0, …) floor — the tag's post_count
	// drifts down and real published posts lose it from the public autocomplete.
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil {
		return ErrNotFound
	}
	// #2757：GetPost 本身沒有 seller 述詞——那是刻意的，四條治理路徑依賴它讀別人的
	// 貼文——所以歸屬判定必須落在這裡。少了它，只要知道別人的貼文 ID 就能觸發下面
	// 全部的刪除後副作用：軟刪本身、hashtag 計數扣減、快取失效。這份清單**不**包含
	// 釋放圖片引用——軟刪刻意保留引用，理由見尾段註解。
	if post.SellerID != sellerID {
		return ErrForbidden
	}
	wasVisible := post.PublishStatus == domain.PostPublishStatusPublished && post.HiddenAt == nil
	rows, err := uc.postRepo.SoftDelete(ctx, postID, sellerID)
	if err != nil {
		return err
	}
	// #2757：rows = 0 代表這次刪除沒有發生（並發重複刪除，或 read 與 write 之間
	// 被別人刪掉）。副作用一個都不能做——counter 會被扣兩次而且沒有下限。
	// 述詞裡的 seller_id 是縱深防禦：上面的 ownership 判斷已經擋過一次。
	if rows == 0 {
		return ErrNotFound
	}
	var touchedHashtags []int64
	if wasVisible {
		adjusted, cerr := uc.hashtagUC.AdjustCountersForVisibilityChange(ctx, nil, postID, -1)
		if cerr != nil {
			logger.Warn(ctx, "hashtag counter delta failed on delete",
				zap.Int64("post_id", postID), zap.Error(cerr))
		}
		touchedHashtags = adjusted
	}
	// The post just left the public feed — bust even when the counter delta
	// above failed, since the removal itself is already committed.
	uc.bustPost(ctx, postID, touchedHashtags...)
	// media 不在這裡碰，引用也不會在別處被釋放：2026-09-05 拍板軟刪刻意保留引用
	// （image_asset_id 沒被 SoftDelete 動過，釋放會製造「業務列指著、卻沒有引用」，
	// 資產隨後被 GC 實體刪除）。media reconciler 的候選集合因此濾掉 deleted_at
	// 非 NULL 的列，見 db/queries/posts.sql 的 ListLiveMediaRefs。代價是軟刪列的資產
	// 永遠不會被回收，而且軟刪沒有還原路徑，所以那是永久的存儲成本——刻意接受的；
	// 真正要清除的情境（帳號終止）走 seller.deleted handler 自己批次釋放，
	// 見 internal/worker/seller_deleted.go。
	//
	// 舊寫法在這裡做 best-effort Release：失敗只記 WARN 仍回成功，而下一次重試會在
	// GetByID（deleted_at IS NULL）就短路，永遠走不到這裡。差別不在「保留」本身——
	// 現在的設計也保留——而在那份保留是失敗的殘留、沒有任何路徑補得回來，而 media GC
	// 只掃「沒有任何引用」的資產，救不回來；今天的保留是刻意的，且帳號終止時有一條
	// 明確的釋放路徑（#2774）。
	return nil
}

func (uc *PostUseCase) IncrViewCount(ctx context.Context, postID int64) error {
	return uc.postRepo.IncrViewCount(ctx, postID)
}

func (uc *PostUseCase) SharePost(ctx context.Context, postID int64) (int32, error) {
	if err := uc.postRepo.IncrShareCount(ctx, postID); err != nil {
		return 0, err
	}
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil {
		return 0, nil
	}
	return post.ShareCount, nil
}

// ToggleLike 切換貼文讚。貼文讚永遠是本人身分（buyer）：不論呼叫者在哪個工作區、貼文是不是自己店的，
// 讚都記在 buyerID 名下。storeID 是呼叫者此刻的店（0 = 沒有），只用來判斷店主讚自己的貼文，
// 這種讚不推 post.liked 通知；比對 cms_post.seller_id。
func (uc *PostUseCase) ToggleLike(ctx context.Context, postID, buyerID, storeID int64) (liked bool, err error) {
	if buyerID == 0 {
		return false, ErrForbidden
	}
	// 確認 post 存在且未刪除（GetByID 查詢含 deleted_at IS NULL）
	if _, getErr := uc.postRepo.GetByID(ctx, postID); getErr != nil {
		return false, ErrNotFound
	}

	_, err = uc.postRepo.GetLike(ctx, postID, domain.ActorKindBuyer, buyerID)
	if err != nil {
		// not found → create like
		if dbErr := uc.postRepo.CreateLike(ctx, postID, domain.ActorKindBuyer, buyerID); dbErr != nil {
			return false, dbErr
		}
		// like_count 改非同步更新（2026-07-31 design），理由同 comment ToggleLike。
		if uc.eventPublisher != nil {
			_ = uc.eventPublisher.PublishCounterDelta(ctx, "post", postID, "like_count", 1)
		}

		// Publish post.liked event (skip if liker is the post author)
		if uc.eventPublisher != nil {
			if post, perr := uc.postRepo.GetByID(ctx, postID); perr == nil && post != nil && (storeID == 0 || post.SellerID != storeID) {
				var likerName string
				if info, berr := uc.buyerClient.GetBuyerProfile(ctx, buyerID); berr == nil && info != nil {
					likerName = info.Nickname
				}
				resolvedSellerAccountID := uc.sellerClient.ResolvePrincipal(ctx, post.SellerID)
				_ = uc.eventPublisher.PublishPostLiked(ctx, resolvedSellerAccountID, postID, post.SellerID, likerName)
			}
		}

		return true, nil
	}
	// already liked → remove
	if dbErr := uc.postRepo.DeleteLike(ctx, postID, domain.ActorKindBuyer, buyerID); dbErr != nil {
		return false, dbErr
	}
	// like_count 改非同步更新（2026-07-31 design）。
	if uc.eventPublisher != nil {
		_ = uc.eventPublisher.PublishCounterDelta(ctx, "post", postID, "like_count", -1)
	}
	return false, nil
}

func (uc *PostUseCase) IsLiked(ctx context.Context, postID, buyerID int64) bool {
	if buyerID == 0 {
		return false
	}
	_, err := uc.postRepo.GetLike(ctx, postID, domain.ActorKindBuyer, buyerID)
	return err == nil
}

// BatchIsLiked returns a set of postIDs that the buyer has liked.
func (uc *PostUseCase) BatchIsLiked(ctx context.Context, postIDs []int64, buyerID int64) map[int64]bool {
	if buyerID == 0 || len(postIDs) == 0 {
		return nil
	}
	ids, err := uc.postRepo.BatchGetLikedPostIDs(ctx, postIDs, domain.ActorKindBuyer, buyerID)
	if err != nil {
		return nil
	}
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func (uc *PostUseCase) GetProductsByIDs(ctx context.Context, productIDs []string) map[string]*ProductBasicInfo {
	if len(productIDs) == 0 {
		return nil
	}
	logger.Info(ctx, "呼叫 product-service 取得商品資料", zap.Strings("product_ids", productIDs))
	result, err := uc.productClient.GetProductsByIDs(ctx, productIDs)
	if err != nil {
		logger.Warn(ctx, "呼叫 product-service 失敗", zap.Strings("product_ids", productIDs), zap.Error(err))
		return nil
	}
	logger.Info(ctx, "product-service 回傳成功", zap.Int("count", len(result)))
	return result
}

// GetSellerMetaBatch resolves store_id -> seller meta for *display*, with no visibility filter.
//
// Distinct from GetSellerInfo on purpose: GetSellerInfo lands on a query carrying
// `AND deleted_at IS NULL`, so it returns a blank name for soft-deleted sellers. Anything that renders an author name
// must use this one instead.
func (uc *PostUseCase) GetSellerMetaBatch(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error) {
	return uc.sellerClient.GetSellerMetaBatch(ctx, storeIDs)
}

func (uc *PostUseCase) GetSellerInfo(ctx context.Context, sellerID int64) (name, logo string) {
	if sellerID == 0 {
		return "", ""
	}
	info, err := uc.sellerClient.GetSellerInfo(ctx, sellerID)
	if err != nil || info == nil {
		return "", ""
	}
	return info.Nickname, info.Avatar
}

func (uc *PostUseCase) ListMyPosts(ctx context.Context, sellerID int64, status string, page, pageSize int32) ([]*gen.CmsPost, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	posts, err := uc.postRepo.ListMy(ctx, sellerID, status, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.postRepo.CountMy(ctx, sellerID, status)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (uc *PostUseCase) AdminListPosts(ctx context.Context, status string, page, pageSize int32) ([]gen.AdminListPostsRow, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxModerationPageSize)
	offset := int64(page-1) * int64(pageSize)
	posts, err := uc.postRepo.AdminList(ctx, status, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.postRepo.AdminCount(ctx, status)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (uc *PostUseCase) RestorePost(ctx context.Context, postID int64) error {
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil {
		return ErrNotFound
	}
	if post.HiddenAt == nil {
		return nil
	}
	if err := uc.postRepo.RestorePost(ctx, postID); err != nil {
		return err
	}
	// Visibility went hidden → public, so +1 for each bound hashtag — but only
	// when publish_status says the post is actually public. RestorePost does not
	// touch publish_status, so `post` (read before the restore) still carries the
	// authoritative value: restoring a draft that was force-hidden must not lift
	// a counter it never contributed to, which is exactly how a tag used only by
	// a delegated draft would leak into the public autocomplete.
	var adjusted []int64
	if post.PublishStatus == domain.PostPublishStatusPublished {
		var cerr error
		adjusted, cerr = uc.hashtagUC.AdjustCountersForVisibilityChange(ctx, nil, postID, 1)
		if cerr != nil {
			logger.Warn(ctx, "hashtag counter delta failed on restore",
				zap.Int64("post_id", postID), zap.Error(cerr))
		}
	}
	// The post is back in the public feed.
	uc.bustPost(ctx, postID, adjusted...)
	return nil
}

// ReportPost delegates to the unified ReportUseCase.SubmitReport (target_type
// always "POST"). Wire contract for callers (gateway handler, buyer app) is
// unchanged — only the internal validation/dedup/rate-limit logic is now
// shared with the B-135 unified path.
func (uc *PostUseCase) ReportPost(ctx context.Context, reporterID, postID int64, reasonCode, description string) (*gen.Report, error) {
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, ErrNotFound
	}
	if post.HiddenAt != nil || post.DeletedAt != nil {
		return nil, ErrNotFound
	}
	return uc.reportUC.SubmitReport(ctx, domain.CreateReportParams{
		ReporterID:  reporterID,
		TargetType:  "POST",
		TargetID:    postID,
		ReasonCode:  reasonCode,
		Description: description,
	})
}

// finalizeAndLinkAsset 轉正 staged token，回傳 objectKey（49 §6.2：cms_post.image 只存
// objectKey，顯示 URL 由 Gateway 組；服務端不再組對外 URL）與 asset id——業務列要記住
// 自己引用的是哪個資產，否則任何精確解除都不可能，只能送「刪掉除了 X 以外的全部」
// 這種在併發下必然出錯的相對指令（#2772）。
func (uc *PostUseCase) finalizeAndLinkAsset(ctx context.Context, token, refType string, refID int64) (string, *int64, error) {
	if token == "" {
		return "", nil, nil
	}
	info, err := uc.mediaClient.FinalizeAndLink(ctx, token, refType, fmt.Sprintf("%d", refID))
	if err != nil {
		return "", nil, err
	}
	assetID, err := strconv.ParseInt(info.AssetId, 10, 64)
	if err != nil {
		return "", nil, fmt.Errorf("parse asset id %q: %w", info.AssetId, err)
	}
	return info.ObjectKey, &assetID, nil
}

// ListPostsByTag returns posts containing the given tag. The server normalizes
// the input (canonical or raw form accepted); a normalize-to-empty input
// returns an empty list with total=0 rather than an error so frontends can
// render "no results" naturally.
func (uc *PostUseCase) ListPostsByTag(ctx context.Context, tagName string, page, pageSize int32) ([]*gen.CmsPost, int64, error) {
	canonical, _, ok := hashtag.Normalize(tagName)
	if !ok {
		return nil, 0, nil
	}
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	posts, err := uc.postRepo.ListByHashtag(ctx, canonical, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.postRepo.CountByHashtag(ctx, canonical)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

// FillTags returns canonical tag names per post for batch list/detail rendering.
// Delegates to HashtagUseCase.GetTagsByPostIDs. Returns nil map for empty input.
func (uc *PostUseCase) FillTags(ctx context.Context, postIDs []int64) (map[int64][]string, error) {
	return uc.hashtagUC.GetTagsByPostIDs(ctx, postIDs)
}
