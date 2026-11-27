package stats

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/schedule"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

// TakedownContentError 是內容下架的對象不成立：內容存在但不是措施的店家自己寫的（買家寫的內容走隱藏）。訂閱者回這個錯誤讓建立措施的
// transaction 回滾；呼叫方用 errors.As 取出是哪一則。
type TakedownContentError struct {
	Kind contentkind.Kind
	ID   uuid.UUID
}

func (e *TakedownContentError) Error() string {
	return fmt.Sprintf("social：內容下架的對象是別家店寫的：%s %s", e.Kind, e.ID)
}

// Content 是內容下架的一個對象：種類加 social 的列 ID。
type Content struct {
	Kind contentkind.Kind
	ID   uuid.UUID
}

type contentSet struct {
	posts, comments, entries []uuid.UUID
}

func splitContents(cs []Content) (contentSet, error) {
	var out contentSet
	for _, c := range cs {
		switch c.Kind {
		case contentkind.Post:
			out.posts = append(out.posts, c.ID)
		case contentkind.Comment:
			out.comments = append(out.comments, c.ID)
		case contentkind.ForumEntry:
			out.entries = append(out.entries, c.ID)
		case contentkind.ChatMessage, contentkind.ReviewReply:
			return contentSet{}, fmt.Errorf("social：%s 不是社群內容，由 im、review 處理（%s）", c.Kind, c.ID)
		default:
			return contentSet{}, fmt.Errorf("social：不認得的內容下架種類 %q（%s）", c.Kind, c.ID)
		}
	}
	out.posts, out.comments, out.entries = Sorted(out.posts), Sorted(out.comments), Sorted(out.entries)
	return out, nil
}

func (c contentSet) empty() bool { return len(c.posts)+len(c.comments)+len(c.entries) == 0 }

// ApplyTakedowns 把一個內容下架措施的對象寫進複本（social.content_takedowns），讓留言與討論區留言重新求值，並排受影響的貼文與店面頁的快取失效。
// 它是 seller 的 MeasureCreated（內容下架）的訂閱者，措施的每一條建立入口（後台 handler、CreateInTx 的呼叫方）都經過它，所以「存在的內容，作者是
// 措施的店家 storeID」（已軟刪或已清除的內容成立但不寫複本，沒有可隱藏的東西；已清除內容的歸屬由 moderation 以案件快照核對）這條不變量在這裡、同一個 transaction 內核對：鎖住留言與討論區留言的列之後核對，不成立回 *model.TakedownContentError 讓措施回滾。貼文的作者是所屬店家、不可變，歸屬核對不受鎖影響；核對之後被店家軟刪只是多一列複本，對帳不動它。必須在呼叫方的 transaction 內：複本、旗標、計數與 job 一起提交或回滾。
// 鎖的順序同 MarkAuthorInactive：先依 id 鎖貼文列（排程發布的鎖順序是貼文列在前，與店家清除一致），再鎖留言與討論區留言的列，再核對，取消被下架貼文的待發布排程
// （TAKEN_DOWN），再寫複本，再以兩次集合呼叫求值（函式依序鎖整批 stats），不另呼叫 Lock。貼文沒有計入旗標，公開讀取的述詞直接看複本。冪等：複本已經有的列不重複寫，沒有新增時不重新求值。
func (s *Service) ApplyTakedowns(ctx context.Context, measureID uuid.UUID, storeID int64, at time.Time, contents []Content) error {
	return s.applyTakedowns(ctx, measureID, at, contents, func(ctx context.Context, set contentSet) (contentSet, error) {
		return s.checkTakedownOwnership(ctx, storeID, set)
	}, nil, func(ctx context.Context, q *store.Queries, p store.InsertTakedownsParams) (inserted, error) {
		rows, err := q.InsertTakedowns(ctx, p)
		return insertedOf(rows, func(r store.InsertTakedownsRow) *uuid.UUID { return r.PostID }), err
	})
}

// ApplyMissingTakedowns 是每日對帳的補漏：把 seller 發布的生效中下架裡複本沒有的列補上。不核對歸屬（措施已經生效，內容之後可以被作者刪除）；
// transaction 的第一句鎖住 seller 措施生效中的對象（seller.pub_lock_content_takedowns，seller 先於 social，在鎖貼文列之前）：撤銷尚未提交時等它，提交後寫入的語句讀不到已撤銷的列，
// 所以讀到「缺」的清單之後才被撤銷的措施不會被寫回；補漏持鎖期間撤銷也等它，撤銷再刪這些列。
func (s *Service) ApplyMissingTakedowns(ctx context.Context, measureID uuid.UUID, at time.Time, contents []Content) error {
	return s.applyTakedowns(ctx, measureID, at, contents, nil, func(ctx context.Context, q *store.Queries) error {
		if _, err := q.LockActiveTakedownsOfMeasure(ctx, measureID); err != nil {
			return fmt.Errorf("social：鎖定措施 %s 生效中的內容下架：%w", measureID, err)
		}
		return nil
	}, func(ctx context.Context, q *store.Queries, p store.InsertTakedownsParams) (inserted, error) {
		rows, err := q.InsertActiveTakedowns(ctx, store.InsertActiveTakedownsParams(p))
		return insertedOf(rows, func(r store.InsertActiveTakedownsRow) *uuid.UUID { return r.PostID }), err
	})
}

type inserted struct {
	n     int
	posts []uuid.UUID
}

func insertedOf[R any](rows []R, post func(R) *uuid.UUID) inserted {
	out := inserted{n: len(rows)}
	for _, r := range rows {
		if id := post(r); id != nil {
			out.posts = append(out.posts, *id)
		}
	}
	return out
}

func (s *Service) applyTakedowns(ctx context.Context, measureID uuid.UUID, at time.Time, contents []Content, check func(context.Context, contentSet) (contentSet, error), before func(context.Context, *store.Queries) error, insert func(context.Context, *store.Queries, store.InsertTakedownsParams) (inserted, error)) error {
	set, err := splitContents(contents)
	if err != nil {
		return err
	}
	if set.empty() {
		return nil
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if before != nil {
			if err := before(ctx, q); err != nil {
				return err
			}
		}
		if len(set.posts) > 0 {
			if _, err := q.LockPostsByID(ctx, set.posts); err != nil {
				return fmt.Errorf("social：鎖定被下架的貼文：%w", err)
			}
		}
		if err := s.lockContent(ctx, set); err != nil {
			return err
		}
		if check != nil {
			live, err := check(ctx, set)
			if err != nil {
				return err
			}
			set = live
			if set.empty() {
				return nil
			}
		}
		rows, err := insert(ctx, q, store.InsertTakedownsParams{
			MeasureID: measureID, PostIds: set.posts, CommentIds: set.comments, ForumEntryIds: set.entries,
		})
		if err != nil {
			return fmt.Errorf("social：記下內容下架 %s：%w", measureID, err)
		}
		if rows.n == 0 {
			return nil
		}
		if _, err := s.d.Cancel.FromLockedPosts(ctx, rows.posts, model.CancelTakenDown, at, schedule.BumpVersion); err != nil {
			return err
		}
		return s.afterTakedownChange(ctx, set)
	})
}

func (s *Service) checkTakedownOwnership(ctx context.Context, storeID int64, set contentSet) (contentSet, error) {
	q := store.New(s.d.DB.Conn(ctx))
	bad, err := q.InvalidTakedownContents(ctx, store.InvalidTakedownContentsParams{
		StoreID: storeID, PostIds: set.posts, CommentIds: set.comments, ForumEntryIds: set.entries,
	})
	if err != nil {
		return contentSet{}, fmt.Errorf("social：核對內容下架的歸屬：%w", err)
	}
	if len(bad) > 0 {
		return contentSet{}, &TakedownContentError{Kind: contentkind.Kind(bad[0].ContentKind), ID: bad[0].ContentID}
	}
	deleted, err := q.GoneTakedownContents(ctx, store.GoneTakedownContentsParams{
		PostIds: set.posts, CommentIds: set.comments, ForumEntryIds: set.entries,
	})
	if err != nil {
		return contentSet{}, fmt.Errorf("social：讀取已刪除或已清除的下架內容：%w", err)
	}
	gone := map[uuid.UUID]bool{}
	for _, d := range deleted {
		gone[d.ContentID] = true
	}
	return contentSet{posts: liveIDs(set.posts, gone), comments: liveIDs(set.comments, gone), entries: liveIDs(set.entries, gone)}, nil
}

func liveIDs(ids []uuid.UUID, gone map[uuid.UUID]bool) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range ids {
		if !gone[id] {
			out = append(out, id)
		}
	}
	return out
}

// ReleaseTakedowns 刪除一個措施的全部複本（MeasureRevoked 的訂閱者），讓留言與討論區留言重新求值並排快取失效；沒有複本時什麼都不做（冪等）。
func (s *Service) ReleaseTakedowns(ctx context.Context, measureID uuid.UUID) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.TakedownsOfMeasure(ctx, measureID)
		if err != nil {
			return fmt.Errorf("social：讀取措施的內容下架 %s：%w", measureID, err)
		}
		var set contentSet
		for _, r := range rows {
			switch {
			case r.PostID != nil:
				set.posts = append(set.posts, *r.PostID)
			case r.CommentID != nil:
				set.comments = append(set.comments, *r.CommentID)
			case r.ForumEntryID != nil:
				set.entries = append(set.entries, *r.ForumEntryID)
			}
		}
		set.posts, set.comments, set.entries = Sorted(set.posts), Sorted(set.comments), Sorted(set.entries)
		if set.empty() {
			return nil
		}
		if err := s.lockContent(ctx, set); err != nil {
			return err
		}
		if _, err := q.DeleteTakedownsOfMeasure(ctx, measureID); err != nil {
			return fmt.Errorf("social：刪除措施的內容下架 %s：%w", measureID, err)
		}
		return s.afterTakedownChange(ctx, set)
	})
}

func (s *Service) lockContent(ctx context.Context, set contentSet) error {
	q := store.New(s.d.DB.Conn(ctx))
	if len(set.comments) > 0 {
		if _, err := q.LockCommentsByID(ctx, set.comments); err != nil {
			return fmt.Errorf("social：鎖定被下架的留言：%w", err)
		}
	}
	if len(set.entries) > 0 {
		if _, err := q.LockForumEntriesByID(ctx, set.entries); err != nil {
			return fmt.Errorf("social：鎖定被下架的討論區留言：%w", err)
		}
	}
	return nil
}

func (s *Service) afterTakedownChange(ctx context.Context, set contentSet) error {
	if _, err := s.Reevaluate(ctx, set.comments, set.entries); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	posts, err := q.TakedownPostStores(ctx, store.TakedownPostStoresParams{PostIds: set.posts, CommentIds: set.comments})
	if err != nil {
		return fmt.Errorf("social：讀取內容下架影響的貼文：%w", err)
	}
	for _, p := range posts {
		if err := s.d.Cache.BustPost(ctx, p.ID, p.StoreID); err != nil {
			return err
		}
	}
	if len(set.entries) == 0 {
		return nil
	}
	stores, err := q.TakedownForumStores(ctx, set.entries)
	if err != nil {
		return fmt.Errorf("social：讀取內容下架影響的討論區：%w", err)
	}
	for _, id := range stores {
		if err := s.d.Cache.BustStore(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) syncTakedowns(ctx context.Context, rep *Report) error {
	q := store.New(s.d.DB.Conn(ctx))
	missing, err := q.MissingTakedowns(ctx)
	if err != nil {
		return fmt.Errorf("social：核對內容下架（缺）：%w", err)
	}
	byMeasure := map[uuid.UUID][]Content{}
	var order []uuid.UUID
	for _, m := range missing {
		if _, ok := byMeasure[m.MeasureID]; !ok {
			order = append(order, m.MeasureID)
		}
		byMeasure[m.MeasureID] = append(byMeasure[m.MeasureID], Content{Kind: contentkind.Kind(m.ContentKind), ID: m.ContentID})
	}
	for _, id := range order {
		if err := s.ApplyMissingTakedowns(ctx, id, s.d.Now().UTC(), byMeasure[id]); err != nil {
			return err
		}
		rep.TakedownsAdded += len(byMeasure[id])
		rep.Drifted += len(byMeasure[id])
	}

	extra, err := q.ExtraTakedowns(ctx)
	if err != nil {
		return fmt.Errorf("social：核對內容下架（多）：%w", err)
	}
	removals := map[uuid.UUID]contentSet{}
	order = order[:0]
	for _, e := range extra {
		set, ok := removals[e.MeasureID]
		if !ok {
			order = append(order, e.MeasureID)
		}
		switch {
		case e.PostID != nil:
			set.posts = append(set.posts, *e.PostID)
		case e.CommentID != nil:
			set.comments = append(set.comments, *e.CommentID)
		case e.ForumEntryID != nil:
			set.entries = append(set.entries, *e.ForumEntryID)
		}
		removals[e.MeasureID] = set
	}
	for _, id := range order {
		set := removals[id]
		set.posts, set.comments, set.entries = Sorted(set.posts), Sorted(set.comments), Sorted(set.entries)
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			if err := s.lockContent(ctx, set); err != nil {
				return err
			}
			if _, err := store.New(s.d.DB.Conn(ctx)).DeleteTakedownContents(ctx, store.DeleteTakedownContentsParams{
				MeasureID: id, PostIds: set.posts, CommentIds: set.comments, ForumEntryIds: set.entries,
			}); err != nil {
				return fmt.Errorf("social：刪除多出來的內容下架 %s：%w", id, err)
			}
			return s.afterTakedownChange(ctx, set)
		})
		if err != nil {
			return err
		}
		n := len(set.posts) + len(set.comments) + len(set.entries)
		rep.TakedownsRemoved += n
		rep.Drifted += n
	}
	return nil
}
