// Package purge 是買家刪帳與店家退場的實體清除，以及瀏覽去重資料的週期清理。買家刪帳的可見性在 job 跑之前已經改好：訂閱者在買家刪帳的
// transaction 內把他記進 inactive_authors 並讓他的內容重新求值（旗標與計數一起改，stats.Service.MarkAuthorInactive），讀取看旗標；店家：
// seller.pub_exposable_stores 在讀取時擋住。job 冪等（以「還沒刪除」與「還存在」為述詞、分批處理），重跑結果相同。每一批在一個 transaction 內寫入
// （計數由內容與讚的 trigger 在同一個 transaction 加減；買家的內容旗標已是不計入，軟刪的差額是 0），並排受影響貼文的快取失效。
package purge

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/stats"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// batchSize 是每一批處理的列數。
const batchSize = 200

// Deps 是清除的依賴。
type Deps struct {
	DB    *db.DB
	Jobs  jobs.Enqueuer
	Stats *stats.Service
	Cache readcache.Cache
	Now   func() time.Time
}

// Service 執行清除。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// PurgeBuyerContentArgs 清除一位買家的社群內容。
type PurgeBuyerContentArgs struct {
	BuyerID int64 `json:"buyer_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (PurgeBuyerContentArgs) Kind() string { return "social.purge_buyer_content" }

// InsertOpts 讓同一位買家還沒結束的 job 合併（申請刪除與關閉各排一次，冪等）：唯一鍵是 BuyerID，涵蓋排隊中、執行中與重試中的狀態
// （River 的規定：執行中也要在內），不含已完成的。執行中被略過的那一次不會漏：買家進入刪除中之後沒有新內容（buyer.LockActive），
// 執行中的 job 會處理完剩下的。
func (PurgeBuyerContentArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: unfinishedStates()}}
}

func unfinishedStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}
}

// PurgeStoreContentArgs 清除一家退場店家的社群內容。
type PurgeStoreContentArgs struct {
	StoreID int64 `json:"store_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (PurgeStoreContentArgs) Kind() string { return "social.purge_store_content" }

// InsertOpts 同 PurgeBuyerContentArgs，唯一鍵是 StoreID。
func (PurgeStoreContentArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: unfinishedStates()}}
}

// CleanupViewsArgs 清掉瀏覽去重用的舊資料。
type CleanupViewsArgs struct{}

// Kind 是 River 的 job 種類。
func (CleanupViewsArgs) Kind() string { return "social.cleanup_views" }

// InsertOpts 讓同一個時段內的排程與重啟合併。
func (CleanupViewsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 12 * time.Hour}}
}

// EnqueueBuyer 排入買家的清除 job（在呼叫方的 transaction 內）。
func EnqueueBuyer(ctx context.Context, jc jobs.Enqueuer, buyerID int64) error {
	opts := PurgeBuyerContentArgs{}.InsertOpts()
	return jc.Enqueue(ctx, PurgeBuyerContentArgs{BuyerID: buyerID}, &opts)
}

// EnqueueStore 排入店家的清除 job（在呼叫方的 transaction 內）。
func EnqueueStore(ctx context.Context, jc jobs.Enqueuer, storeID int64) error {
	opts := PurgeStoreContentArgs{}.InsertOpts()
	return jc.Enqueue(ctx, PurgeStoreContentArgs{StoreID: storeID}, &opts)
}

// batch 收集一批清除影響到的對象：touch 是這一批的 trigger 會寫的 stats 列（貼文、父留言、父討論區留言）；likedComments 是只有讚數變的留言
// （只鎖）；bust 是要排快取失效的貼文。
type batch struct {
	touch         stats.Set
	likedComments []uuid.UUID
	bust          []uuid.UUID
	bustStores    []int64 // 討論區留言被刪的店家：店面頁的快取失效（與隱藏討論區留言相同）
	n             int
}

// lock 依固定順序（貼文的 stats、留言的 stats、討論區留言的 stats，各自依 ID）鎖住這一批會碰到的全部 stats 列。必須在這一批的
// 第一個寫入之前呼叫：清除會同時碰多列（counts-brief.md §4.6），不在寫入當中才取鎖，才不會和 trigger 的順序相反而死結。
func (s *Service) lock(ctx context.Context, b *batch) error {
	set := b.touch
	set.Comments = append(slices.Clone(set.Comments), b.likedComments...)
	return s.d.Stats.Lock(ctx, set)
}

// finish 排入受影響貼文與店面頁的快取失效（店面頁：討論區留言被刪，依店家去重、由小到大）。
func (s *Service) finish(ctx context.Context, b *batch) error {
	if err := s.d.Cache.BustPosts(ctx, stats.Sorted(append(slices.Clone(b.bust), b.touch.Posts...))); err != nil {
		return err
	}
	stores := slices.Clone(b.bustStores)
	slices.Sort(stores)
	for _, id := range slices.Compact(stores) {
		if err := s.d.Cache.BustStore(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// PurgeBuyer 清除買家的內容：軟刪他的留言與討論區留言、刪他的讚與分享，最後清掉他全部已刪除的留言與討論區留言的內文（保留列，見 scrubBodies）。重跑只處理剩下的。買家此時已不是營運中（進入刪除中之後，他的
// 寫入被 buyer.LockActive 擋下），所以資料集不會再長。
// 買家的內容在申請刪除（或直接關閉）的 transaction 就已經不計入、不可見（訂閱者經 MarkAuthorInactive 修好旗標與計數），所以這裡不必再求值：
// 之後的軟刪由 trigger 照常維護，申請之後被作者或店家刪掉的留言也因為旗標已是假而差額為 0。
func (s *Service) PurgeBuyer(ctx context.Context, buyerID int64) error {
	for {
		b := &batch{}
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := s.q(ctx)
			now := s.d.Now().UTC()
			comments, err := q.LockBuyerCommentBatch(ctx, store.LockBuyerCommentBatchParams{BuyerID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：鎖定買家的留言：%w", err)
			}
			entries, err := q.LockBuyerForumEntryBatch(ctx, store.LockBuyerForumEntryBatchParams{BuyerID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：鎖定買家的討論區留言：%w", err)
			}
			likes, err := q.ListBuyerLikedPostBatch(ctx, store.ListBuyerLikedPostBatchParams{BuyerID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：列出買家的貼文讚：%w", err)
			}
			shares, err := q.ListBuyerSharedPostBatch(ctx, store.ListBuyerSharedPostBatchParams{BuyerID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：列出買家的分享：%w", err)
			}
			commentLikes, err := q.ListActorCommentLikeBatch(ctx, store.ListActorCommentLikeBatchParams{ActorKind: string(model.ActorBuyer), ActorID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：列出買家的留言讚：%w", err)
			}
			commentIDs := make([]uuid.UUID, len(comments))
			for i, c := range comments {
				commentIDs[i] = c.ID
				b.touch.Posts = append(b.touch.Posts, c.PostID)
				if c.ParentID != nil {
					b.touch.Comments = append(b.touch.Comments, *c.ParentID)
				}
			}
			entryIDs := make([]uuid.UUID, len(entries))
			for i, e := range entries {
				entryIDs[i] = e.ID
				b.bustStores = append(b.bustStores, e.StoreID)
				if e.ParentID != nil {
					b.touch.ForumEntries = append(b.touch.ForumEntries, *e.ParentID)
				}
			}
			b.touch.Posts = append(append(b.touch.Posts, likes...), shares...)
			likedIDs := make([]uuid.UUID, len(commentLikes))
			for i, l := range commentLikes {
				likedIDs[i] = l.CommentID
				b.likedComments = append(b.likedComments, l.CommentID)
				b.bust = append(b.bust, l.PostID)
			}
			b.n = len(comments) + len(entries) + len(likes) + len(shares) + len(commentLikes)
			if b.n == 0 {
				return nil
			}
			if err := s.lock(ctx, b); err != nil {
				return err
			}
			if len(commentIDs) > 0 {
				if err := q.SoftDeleteCommentBatch(ctx, store.SoftDeleteCommentBatchParams{Ids: commentIDs, Now: &now}); err != nil {
					return fmt.Errorf("social：刪除買家的留言：%w", err)
				}
			}
			if len(entryIDs) > 0 {
				if err := q.SoftDeleteForumEntryBatch(ctx, store.SoftDeleteForumEntryBatchParams{Ids: entryIDs, Now: &now}); err != nil {
					return fmt.Errorf("social：刪除買家的討論區留言：%w", err)
				}
			}
			if err := q.DeleteBuyerPostLikes(ctx, store.DeleteBuyerPostLikesParams{BuyerID: buyerID, PostIds: likes}); err != nil {
				return fmt.Errorf("social：刪除買家的貼文讚：%w", err)
			}
			if err := q.DeleteBuyerPostShares(ctx, store.DeleteBuyerPostSharesParams{BuyerID: buyerID, PostIds: shares}); err != nil {
				return fmt.Errorf("social：刪除買家的分享：%w", err)
			}
			if err := q.DeleteActorCommentLikes(ctx, store.DeleteActorCommentLikesParams{ActorKind: string(model.ActorBuyer), ActorID: buyerID, CommentIds: likedIDs}); err != nil {
				return fmt.Errorf("social：刪除買家的留言讚：%w", err)
			}
			return s.finish(ctx, b)
		})
		if err != nil {
			return err
		}
		if b.n == 0 {
			break
		}
	}
	return s.scrubBodies(ctx, buyerID)
}

// scrubBodies 把買家全部已刪除的留言與討論區留言的內文設為 NULL，每批一個 transaction，直到沒有剩下的（DR-A4，計畫 P2：實體刪除由 job 執行）。
// 軟刪只設 deleted_at，內文還在資料庫裡；買家刪帳之後他的文字不該留著。列保留（deleted_at 已設）：討論串的結構與頂層的佔位項目要靠它們，
// 而讀取本來就不回已刪除內容的內文。這一步也處理買家在申請刪除之前自己刪掉的留言（內文還在）。留言與討論區留言沒有圖片欄位，
// 所以沒有媒體引用要釋放（貼文才有，mediaref.PostRef）；計數不受影響（只改 body，不碰 deleted_at 與 stats 的 trigger）。
func (s *Service) scrubBodies(ctx context.Context, buyerID int64) error {
	for {
		var n int64
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := s.q(ctx)
			c, err := q.ScrubBuyerCommentBodyBatch(ctx, store.ScrubBuyerCommentBodyBatchParams{BuyerID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：清除買家留言的內文：%w", err)
			}
			e, err := q.ScrubBuyerForumEntryBodyBatch(ctx, store.ScrubBuyerForumEntryBodyBatchParams{BuyerID: buyerID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：清除買家討論區留言的內文：%w", err)
			}
			n = c + e
			return nil
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

// 店面頁的失效：整店已不可曝光，退場時 seller 自己送店面頁的 tag（seller 的清除與 stint 退場），這裡只送貼文的 tag。
// PurgeStore 清除退場店家的內容：軟刪它的貼文並釋放圖片引用（引用由同步 job 解除）、軟刪它的討論區留言（含買家寫的）與店家身分的留言、
// 刪它身分的留言讚。重跑只處理剩下的。計數由內容的 trigger 維護。
func (s *Service) PurgeStore(ctx context.Context, storeID int64) error {
	for {
		b := &batch{}
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := s.q(ctx)
			now := s.d.Now().UTC()
			posts, err := q.LockStorePostBatch(ctx, store.LockStorePostBatchParams{StoreID: storeID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：鎖定店家的貼文：%w", err)
			}
			comments, err := q.LockStoreCommentBatch(ctx, store.LockStoreCommentBatchParams{StoreID: storeID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：鎖定店家身分的留言：%w", err)
			}
			entries, err := q.LockStoreForumEntryBatch(ctx, store.LockStoreForumEntryBatchParams{StoreID: storeID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：鎖定店家的討論區留言：%w", err)
			}
			commentLikes, err := q.ListActorCommentLikeBatch(ctx, store.ListActorCommentLikeBatchParams{ActorKind: string(model.ActorStore), ActorID: storeID, MaxRows: batchSize})
			if err != nil {
				return fmt.Errorf("social：列出店家身分的留言讚：%w", err)
			}
			postIDs := make([]uuid.UUID, len(posts))
			for i, p := range posts {
				postIDs[i] = p.ID
				b.touch.Posts = append(b.touch.Posts, p.ID)
			}
			commentIDs := make([]uuid.UUID, len(comments))
			for i, c := range comments {
				commentIDs[i] = c.ID
				b.touch.Posts = append(b.touch.Posts, c.PostID)
				if c.ParentID != nil {
					b.touch.Comments = append(b.touch.Comments, *c.ParentID)
				}
			}
			entryIDs := make([]uuid.UUID, len(entries))
			for i, e := range entries {
				entryIDs[i] = e.ID
				if e.ParentID != nil {
					b.touch.ForumEntries = append(b.touch.ForumEntries, *e.ParentID)
				}
			}
			likedIDs := make([]uuid.UUID, len(commentLikes))
			for i, l := range commentLikes {
				likedIDs[i] = l.CommentID
				b.likedComments = append(b.likedComments, l.CommentID)
				b.bust = append(b.bust, l.PostID)
			}
			b.n = len(posts) + len(comments) + len(entries) + len(commentLikes)
			if b.n == 0 {
				return nil
			}
			if err := s.lock(ctx, b); err != nil {
				return err
			}
			for _, p := range posts {
				var retired []string
				if p.ImageAssetID != nil {
					retired = append(retired, *p.ImageAssetID)
				}
				if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.PostRef(p.ID), retired...); err != nil {
					return err
				}
			}
			if len(postIDs) > 0 {
				if err := q.PurgePostBatch(ctx, store.PurgePostBatchParams{Ids: postIDs, Now: now}); err != nil {
					return fmt.Errorf("social：清除店家的貼文：%w", err)
				}
			}
			if len(commentIDs) > 0 {
				if err := q.SoftDeleteCommentBatch(ctx, store.SoftDeleteCommentBatchParams{Ids: commentIDs, Now: &now}); err != nil {
					return fmt.Errorf("social：刪除店家身分的留言：%w", err)
				}
			}
			if len(entryIDs) > 0 {
				if err := q.SoftDeleteForumEntryBatch(ctx, store.SoftDeleteForumEntryBatchParams{Ids: entryIDs, Now: &now}); err != nil {
					return fmt.Errorf("social：刪除店家的討論區留言：%w", err)
				}
			}
			if err := q.DeleteActorCommentLikes(ctx, store.DeleteActorCommentLikesParams{ActorKind: string(model.ActorStore), ActorID: storeID, CommentIds: likedIDs}); err != nil {
				return fmt.Errorf("social：刪除店家身分的留言讚：%w", err)
			}
			return s.finish(ctx, b)
		})
		if err != nil {
			return err
		}
		if b.n == 0 {
			return nil
		}
	}
}

// CleanupViews 刪掉瀏覽去重用的舊資料：前一個台北日以前的 salt 與前兩個台北日以前的去重列（去重只需要當日與前一日）。刪除不扣瀏覽數。
func (s *Service) CleanupViews(ctx context.Context) error {
	today := twtime.Day(s.d.Now().UTC())
	q := s.q(ctx)
	if _, err := q.DeleteOldViewSalts(ctx, today); err != nil {
		return fmt.Errorf("social：清除舊的瀏覽 salt：%w", err)
	}
	if _, err := q.DeleteOldPostViews(ctx, today.AddDate(0, 0, -1)); err != nil {
		return fmt.Errorf("social：清除舊的瀏覽去重列：%w", err)
	}
	return nil
}

type purgeBuyerWorker struct {
	river.WorkerDefaults[PurgeBuyerContentArgs]
	svc *Service
}

func (w *purgeBuyerWorker) Work(ctx context.Context, job *river.Job[PurgeBuyerContentArgs]) error {
	return w.svc.PurgeBuyer(ctx, job.Args.BuyerID)
}
func (w *purgeBuyerWorker) Timeout(*river.Job[PurgeBuyerContentArgs]) time.Duration { return time.Hour }

type purgeStoreWorker struct {
	river.WorkerDefaults[PurgeStoreContentArgs]
	svc *Service
}

func (w *purgeStoreWorker) Work(ctx context.Context, job *river.Job[PurgeStoreContentArgs]) error {
	return w.svc.PurgeStore(ctx, job.Args.StoreID)
}
func (w *purgeStoreWorker) Timeout(*river.Job[PurgeStoreContentArgs]) time.Duration { return time.Hour }

type cleanupWorker struct {
	river.WorkerDefaults[CleanupViewsArgs]
	svc *Service
}

func (w *cleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupViewsArgs]) error {
	return w.svc.CleanupViews(ctx)
}
func (w *cleanupWorker) Timeout(*river.Job[CleanupViewsArgs]) time.Duration { return 5 * time.Minute }

// AddWorkers 登記清除的 worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &purgeBuyerWorker{svc: s})
	river.AddWorker(workers, &purgeStoreWorker{svc: s})
	river.AddWorker(workers, &cleanupWorker{svc: s})
}

// PeriodicJobs 是每日清理瀏覽去重資料（啟動時也跑一次）。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return CleanupViewsArgs{}, nil },
		&river.PeriodicJobOpts{ID: "social.cleanup_views", RunOnStart: true},
	)}
}
