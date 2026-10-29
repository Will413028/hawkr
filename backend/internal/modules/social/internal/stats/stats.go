// Package stats 是計數的重算與每日對帳（counts-brief.md §4 第 8 點）。計數平常由資料庫的 trigger 在寫入的 transaction 內維護
// （db/objects.sql）：每則留言存一個旗標，記它此刻是否計入，trigger 讓受影響的內容重新求值（reevaluate_*），旗標改變才加減計數。
// 「計入」的輸入全是 social 自己的表；買家刪帳這個外部事實由訂閱者在買家刪帳的 transaction 內經 MarkAuthorInactive 記進 inactive_authors，
// 並讓他的內容重新求值（一次集合呼叫，鎖的順序在 reevaluate_* 裡）。每日對帳與手動全量重算（子命令 social-recount）分三步：先核對複本與 buyer.pub_inactive_buyers、seller.pub_content_takedowns()
// （補漏與刪多），再讓全部內容重新求值（修旗標），最後把計數覆寫成旗標的加總（reads 的重算語句）。
// 重算先 SELECT ... FOR UPDATE 鎖 stats 列，再用另一個語句計數，不蓋掉並行的加減。瀏覽數是只增的事件計數，沒有可重算的來源，不在範圍。
package stats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是重算的依賴。
type Deps struct {
	DB     *db.DB
	Logger *slog.Logger
	Now    func() time.Time
	// Cache 排內容下架影響的貼文與店面頁的快取失效。
	Cache readcache.Cache
}

// Service 重算計數。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Drift 是一個計數與重算結果不同的對象：Kind 是 POST、COMMENT 或 FORUM_ENTRY，Before 與 After 是依序的欄位值（貼文：讚、留言、分享；
// 留言：讚、回覆；討論區留言：回覆）。
type Drift struct {
	Kind          string
	ID            uuid.UUID
	Before, After []int64
}

// Report 是一次全量重算的結果。Drifts 最多留 maxDriftDetails 筆，Drifted 是全部的數量（含旗標被修正的內容）。
// FlagsFixed 是重新求值時旗標與求值結果不同的內容數（trigger 或外部事實的維護漏了，正常是 0）。
type Report struct {
	Posts, Comments, ForumEntries int
	FlagsFixed                    int
	// AuthorsAdded 是複本（inactive_authors）缺、由 buyer.pub_inactive_buyers 補上的買家數（訂閱者漏了，正常是 0）。
	AuthorsAdded int
	// TakedownsAdded、TakedownsRemoved 是內容下架的複本與 seller.pub_content_takedowns() 不一致、對帳補上與刪除的列數（訂閱者漏了，正常是 0）。
	TakedownsAdded, TakedownsRemoved int
	Drifted                          int
	Drifts                           []Drift
}

const (
	pageSize        = 200 // 重新求值（reevaluate_*）與買家內容的分頁
	batchSize       = 500 // 全量重算的一批：鎖住的 stats 列數（寫入者最多等一批的時間）
	maxDriftDetails = 100
)

// Set 是一次要鎖定或重算的 stats 列：貼文、留言（父項）、討論區留言（父項）的 ID。
type Set struct {
	Posts, Comments, ForumEntries []uuid.UUID
}

// Sorted 回傳去重並依 ID 由小到大排好的 ID：鎖多列時的固定順序（不依 Go 的 map 迭代順序）。
func Sorted(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, uuid.UUID.Compare)
	return slices.Compact(out)
}

// Lock 在呼叫方的 transaction 內鎖住 set 的 stats 列，順序固定：先貼文的 stats，再留言的 stats，再討論區留言的 stats，各自依 ID 由小到大
// （counts-brief.md §4.6，與 trigger 的鎖順序相同：貼文再留言）。要同時寫多個對象的計數的 transaction（清除 job）在第一個寫入之前呼叫，
// 之後的寫入與重算只碰已經持有的鎖。沒有 stats 列的 ID 略過。
func (s *Service) Lock(ctx context.Context, set Set) error {
	q := store.New(s.d.DB.Conn(ctx))
	for _, id := range Sorted(set.Posts) {
		if _, err := q.LockPostStats(ctx, id); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("social：鎖定貼文計數 %s：%w", id, err)
		}
	}
	for _, id := range Sorted(set.Comments) {
		if _, err := q.LockCommentStats(ctx, id); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("social：鎖定留言計數 %s：%w", id, err)
		}
	}
	for _, id := range Sorted(set.ForumEntries) {
		if _, err := q.LockForumEntryStats(ctx, id); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("social：鎖定討論區留言計數 %s：%w", id, err)
		}
	}
	return nil
}

// Reevaluate 讓這批留言與討論區留言重新求值「是否計入」（冪等；鎖的順序與求值都在資料庫函式裡，見 db/objects.sql 的 reevaluate_*）。
// 回傳旗標改變的筆數。在自己的 transaction 內做（呼叫方已有 transaction 時加入它）。
func (s *Service) Reevaluate(ctx context.Context, comments, entries []uuid.UUID) (int, error) {
	changed := 0
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if len(comments) > 0 {
			n, err := q.ReevaluateComments(ctx, comments)
			if err != nil {
				return fmt.Errorf("social：讓留言重新求值：%w", err)
			}
			changed += int(n)
		}
		if len(entries) > 0 {
			n, err := q.ReevaluateForumEntries(ctx, entries)
			if err != nil {
				return fmt.Errorf("social：讓討論區留言重新求值：%w", err)
			}
			changed += int(n)
		}
		return nil
	})
	return changed, err
}

// MarkAuthorInactive 記下「買家不再營運中」的事實（social.inactive_authors），並讓他寫過的全部留言與討論區留言（含已刪除的）重新求值。
// 這是 BuyerDeletionRequested、BuyerDeleted 的訂閱者與每日對帳共用的唯一入口，必須在呼叫方的 transaction 內（買家刪帳的 transaction）：
// 事實與旗標、計數在同一個 transaction 改變，所以資料庫與 API 的讀取立即看得到（沒有 job 的延遲；網站快取頁由清除 job 失效），不存在「事實已記、旗標還沒改」的中間狀態。
// 買家的狀態不會回到營運中，所以只插入不刪除；已經記過的回傳 false，不重新求值（冪等）。
//
// 鎖的順序：先依固定順序鎖內容列（id 由小到大，留言再討論區留言），再以兩次集合呼叫求值（reevaluate_comments、reevaluate_forum_entries），
// 函式各自依「內容列、貼文 stats、父留言 stats、父討論區留言 stats，各依 id」鎖整批，所以整體順序是貼文 stats、留言 stats、討論區留言 stats，
// 與單列 trigger 的順序相同。這裡不呼叫 Lock：求值是集合呼叫，函式在寫任何一列之前已經把這一批的 stats 全鎖好（Lock 取的是同一批同一個順序，
// 重複取鎖是 no-op，刪掉它沒有任何行為差異）。需要 Lock 的是逐列寫入再靠 trigger 求值的批次（清除 job，見 purge），那種批次靠 trigger 自己排不了序。
// TestMarkAuthorInactiveLocksStatsInTheSameOrderAsTheTriggers 釘住這個順序。
func (s *Service) MarkAuthorInactive(ctx context.Context, buyerID int64, since time.Time) (bool, error) {
	inserted := false
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		n, err := q.InsertInactiveAuthor(ctx, store.InsertInactiveAuthorParams{BuyerID: buyerID, Since: since})
		if err != nil {
			return fmt.Errorf("social：記下不營運中的買家 %d：%w", buyerID, err)
		}
		if n == 0 {
			return nil
		}
		inserted = true
		comments, err := q.LockAuthorComments(ctx, buyerID)
		if err != nil {
			return fmt.Errorf("social：鎖定買家的留言：%w", err)
		}
		entries, err := q.LockAuthorForumEntries(ctx, buyerID)
		if err != nil {
			return fmt.Errorf("social：鎖定買家的討論區留言：%w", err)
		}
		commentIDs := make([]uuid.UUID, len(comments))
		for i, c := range comments {
			commentIDs[i] = c.ID
		}
		entryIDs := make([]uuid.UUID, len(entries))
		for i, e := range entries {
			entryIDs[i] = e.ID
		}
		_, err = s.Reevaluate(ctx, commentIDs, entryIDs)
		return err
	})
	return inserted, err
}

// syncInactiveAuthors 是對帳的第一步：buyer.pub_inactive_buyers 裡複本還沒有的買家補進來（訂閱者漏了的漂移，正常是 0）。
func (s *Service) syncInactiveAuthors(ctx context.Context, rep *Report) error {
	missing, err := store.New(s.d.DB.Conn(ctx)).MissingInactiveAuthors(ctx)
	if err != nil {
		return fmt.Errorf("social：核對不營運中的買家：%w", err)
	}
	for _, id := range missing {
		inserted, err := s.MarkAuthorInactive(ctx, id, s.d.Now().UTC())
		if err != nil {
			return err
		}
		if inserted {
			rep.AuthorsAdded++
			rep.Drifted++
		}
	}
	return nil
}

func (r *Report) add(d Drift) {
	r.Drifted++
	if len(r.Drifts) < maxDriftDetails {
		r.Drifts = append(r.Drifts, d)
	}
}

// reevaluateAll 讓全部留言與討論區留言重新求值，修旗標（每批一個 transaction，依 ID 的 keyset 分頁）。旗標先於計數修：reevaluate 依旗標的
// 差額加減計數，之後的覆寫才以同一個述詞的結果為準。
func (s *Service) reevaluateAll(ctx context.Context, rep *Report) error {
	q := store.New(s.d.DB.Conn(ctx))
	var after uuid.UUID
	for {
		ids, err := q.CommentStatsPage(ctx, store.CommentStatsPageParams{AfterID: after, MaxRows: pageSize})
		if err != nil {
			return fmt.Errorf("social：列出留言計數：%w", err)
		}
		n, err := s.Reevaluate(ctx, ids, nil)
		if err != nil {
			return err
		}
		rep.FlagsFixed += n
		if len(ids) < pageSize {
			break
		}
		after = ids[len(ids)-1]
	}
	after = uuid.UUID{}
	for {
		ids, err := q.ForumEntryStatsPage(ctx, store.ForumEntryStatsPageParams{AfterID: after, MaxRows: pageSize})
		if err != nil {
			return fmt.Errorf("social：列出討論區留言計數：%w", err)
		}
		n, err := s.Reevaluate(ctx, nil, ids)
		if err != nil {
			return err
		}
		rep.FlagsFixed += n
		if len(ids) < pageSize {
			break
		}
		after = ids[len(ids)-1]
	}
	rep.Drifted += rep.FlagsFixed
	return nil
}

// RecountAll 對全部可重算的計數跑重算、覆寫，並回報漂移：先修旗標（reevaluateAll），再對貼文、留言、討論區留言各自依 ID 的 keyset 分批，
// 每批一個 transaction、固定三個語句：依 ID 由小到大鎖住這批的 stats 列、一個分組計數語句算出新值並只覆寫不同的列、RETURNING 前後值。
// 不一次鎖住全表。完整掃描是刻意的：外部事實的改變（買家停用、隱藏）落在沒有被寫入的對象上，只追蹤被寫入的對象會漏。
func (s *Service) RecountAll(ctx context.Context) (Report, error) {
	var rep Report
	if err := s.syncInactiveAuthors(ctx, &rep); err != nil {
		return rep, err
	}
	if err := s.syncTakedowns(ctx, &rep); err != nil {
		return rep, err
	}
	if err := s.reevaluateAll(ctx, &rep); err != nil {
		return rep, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	kinds := []struct {
		count *int
		next  func(after uuid.UUID) ([]uuid.UUID, error)
		batch func(ctx context.Context, after, upper uuid.UUID, rep *Report) error
	}{
		{
			count: &rep.Posts,
			next: func(after uuid.UUID) ([]uuid.UUID, error) {
				return q.PostStatsPage(ctx, store.PostStatsPageParams{AfterID: after, MaxRows: batchSize})
			},
			batch: s.recountPosts,
		},
		{
			count: &rep.Comments,
			next: func(after uuid.UUID) ([]uuid.UUID, error) {
				return q.CommentStatsPage(ctx, store.CommentStatsPageParams{AfterID: after, MaxRows: batchSize})
			},
			batch: s.recountComments,
		},
		{
			count: &rep.ForumEntries,
			next: func(after uuid.UUID) ([]uuid.UUID, error) {
				return q.ForumEntryStatsPage(ctx, store.ForumEntryStatsPageParams{AfterID: after, MaxRows: batchSize})
			},
			batch: s.recountForumEntries,
		},
	}
	for _, k := range kinds {
		var after uuid.UUID
		for {
			ids, err := k.next(after)
			if err != nil {
				return rep, fmt.Errorf("social：列出計數的批次：%w", err)
			}
			if len(ids) == 0 {
				break
			}
			upper := ids[len(ids)-1]
			*k.count += len(ids)
			if err := k.batch(ctx, after, upper, &rep); err != nil {
				return rep, err
			}
			if len(ids) < batchSize {
				break
			}
			after = upper
		}
	}
	return rep, nil
}

func (s *Service) recountPosts(ctx context.Context, after, upper uuid.UUID, rep *Report) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		c := s.d.DB.Conn(ctx)
		if err := store.New(c).LockPostStatsRange(ctx, store.LockPostStatsRangeParams{AfterID: after, UpperID: upper}); err != nil {
			return fmt.Errorf("social：鎖定貼文計數：%w", err)
		}
		rows, err := reads.Many[reads.PostDriftRow](ctx, c, reads.RecountPostStatsBatch(after, upper))
		if err != nil {
			return fmt.Errorf("social：重算貼文計數：%w", err)
		}
		for _, r := range rows {
			rep.add(Drift{Kind: "POST", ID: r.ID, Before: []int64{r.OldLike, r.OldComment, r.OldShare}, After: []int64{r.NewLike, r.NewComment, r.NewShare}})
		}
		return nil
	})
}

func (s *Service) recountComments(ctx context.Context, after, upper uuid.UUID, rep *Report) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		c := s.d.DB.Conn(ctx)
		if err := store.New(c).LockCommentStatsRange(ctx, store.LockCommentStatsRangeParams{AfterID: after, UpperID: upper}); err != nil {
			return fmt.Errorf("social：鎖定留言計數：%w", err)
		}
		rows, err := reads.Many[reads.CommentDriftRow](ctx, c, reads.RecountCommentStatsBatch(after, upper))
		if err != nil {
			return fmt.Errorf("social：重算留言計數：%w", err)
		}
		for _, r := range rows {
			rep.add(Drift{Kind: "COMMENT", ID: r.ID, Before: []int64{r.OldLike, r.OldReply}, After: []int64{r.NewLike, r.NewReply}})
		}
		return nil
	})
}

func (s *Service) recountForumEntries(ctx context.Context, after, upper uuid.UUID, rep *Report) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		c := s.d.DB.Conn(ctx)
		if err := store.New(c).LockForumEntryStatsRange(ctx, store.LockForumEntryStatsRangeParams{AfterID: after, UpperID: upper}); err != nil {
			return fmt.Errorf("social：鎖定討論區留言計數：%w", err)
		}
		rows, err := reads.Many[reads.EntryDriftRow](ctx, c, reads.RecountForumEntryStatsBatch(after, upper))
		if err != nil {
			return fmt.Errorf("social：重算討論區留言計數：%w", err)
		}
		for _, r := range rows {
			rep.add(Drift{Kind: "FORUM_ENTRY", ID: r.ID, Before: []int64{r.OldReply}, After: []int64{r.NewReply}})
		}
		return nil
	})
}

// Reconcile 是每日對帳：RecountAll，漂移不為 0 時記 warn log（trigger 有 bug 的警報），列出前幾個對象。
func (s *Service) Reconcile(ctx context.Context) (Report, error) {
	rep, err := s.RecountAll(ctx)
	if err != nil {
		return rep, err
	}
	if rep.Drifted > 0 {
		s.d.Logger.WarnContext(ctx, "social 計數對帳發現漂移並已覆寫", "drifted", rep.Drifted, "flags_fixed", rep.FlagsFixed, "authors_added", rep.AuthorsAdded, "takedowns_added", rep.TakedownsAdded, "takedowns_removed", rep.TakedownsRemoved, "examples", rep.Drifts)
	}
	return rep, nil
}

// ReconcileArgs 是每日對帳的 job。
type ReconcileArgs struct{}

// Kind 是 River 的 job 種類。
func (ReconcileArgs) Kind() string { return "social.reconcile_counts" }

// InsertOpts 讓同一個時段內的排程與重啟合併成一個 job。
func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 12 * time.Hour}, MaxAttempts: 3}
}

type reconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	svc *Service
}

// Work 跑一次對帳。
func (w *reconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	_, err := w.svc.Reconcile(ctx)
	return err
}

// Timeout 是一次對帳的時間上限。
func (w *reconcileWorker) Timeout(*river.Job[ReconcileArgs]) time.Duration { return time.Hour }

// AddWorkers 登記對帳的 worker。
func AddWorkers(workers *river.Workers, s *Service) {
	river.AddWorker(workers, &reconcileWorker{svc: s})
}

// PeriodicJobs 是每日一次的對帳（不在啟動時執行：全量重算在資料多時不輕）。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
		&river.PeriodicJobOpts{ID: "social.reconcile_counts"},
	)}
}
