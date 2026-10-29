package stats

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

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
		default:
			return contentSet{}, fmt.Errorf("social：不認得的內容下架種類 %q（%s）", c.Kind, c.ID)
		}
	}
	out.posts, out.comments, out.entries = Sorted(out.posts), Sorted(out.comments), Sorted(out.entries)
	return out, nil
}

func (c contentSet) empty() bool { return len(c.posts)+len(c.comments)+len(c.entries) == 0 }

// ApplyTakedowns 把一個內容下架措施的對象寫進複本（social.content_takedowns），讓留言與討論區留言重新求值，並排受影響的貼文與店面頁的快取失效。
// 它是 seller 的 MeasureCreated（內容下架）的訂閱者與每日對帳共用的唯一入口，必須在呼叫方的 transaction 內：複本、旗標、計數與 job 一起提交或回滾。
// 鎖的順序同 MarkAuthorInactive：先依 id 鎖內容列，再寫複本，再以兩次集合呼叫求值（函式依序鎖整批 stats），不另呼叫 Lock。
// 貼文沒有計入旗標，公開讀取的述詞直接看複本。冪等：複本已經有的列不重複寫，沒有新增時不重新求值。
func (s *Service) ApplyTakedowns(ctx context.Context, measureID uuid.UUID, at time.Time, contents []Content) error {
	set, err := splitContents(contents)
	if err != nil {
		return err
	}
	if set.empty() {
		return nil
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if err := s.lockContent(ctx, set); err != nil {
			return err
		}
		n, err := q.InsertTakedowns(ctx, store.InsertTakedownsParams{
			MeasureID: measureID, CreatedAt: at, PostIds: set.posts, CommentIds: set.comments, ForumEntryIds: set.entries,
		})
		if err != nil {
			return fmt.Errorf("social：記下內容下架 %s：%w", measureID, err)
		}
		if n == 0 {
			return nil
		}
		return s.afterTakedownChange(ctx, set)
	})
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

// syncTakedowns 是對帳的一步：seller.pub_content_takedowns() 有而複本沒有的補上，複本有而 seller 沒有的刪掉（訂閱者漏了的漂移，正常是 0）。
// 每個措施一個 transaction。
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
		if err := s.ApplyTakedowns(ctx, id, s.d.Now().UTC(), byMeasure[id]); err != nil {
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
