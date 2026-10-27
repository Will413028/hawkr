// Package forum 是店家的討論區：買家與店家都可以留言與回覆（回覆的父一律壓成頂層）；規則同貼文的留言（作者本人編輯、被隱藏不能編輯、
// 作者或該店刪除且冪等、頂層不可見而底下有可見回覆時回佔位項目），容器是店家：店家要可曝光才公開。
package forum

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/actor"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/textcheck"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Deps 是討論區服務的依賴。
type Deps struct {
	DB     *db.DB
	Actors actor.Resolver
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Service 是討論區的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func entryNotFound(ctx context.Context) error {
	return apperr.NotFound("%s", text.ForumEntryNotFound(uitext.LocaleFrom(ctx)))
}

func authorOf(e store.SocialForumEntry) model.Actor {
	return model.Actor{Kind: model.ActorKind(e.ActorKind), ID: e.ActorID}
}

func visible(e store.SocialForumEntry) model.ForumEntry {
	return model.ForumEntry{
		ID: e.ID, StoreID: e.StoreID, ParentID: e.ParentID, ReplyToID: e.ReplyToID, State: model.CommentVisible,
		Actor: authorOf(e), Body: model.BodyText(e.Body), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

// Create 在店家的討論區新增留言或回覆。店家要可曝光；互動者依歸屬推導（呼叫者此刻的店是這家店 → STORE，要有 CapMarketing，否則是買家）。
// 回覆的父一律壓成頂層，reply_to 只接受同一個討論區同一串（父本身或父底下的回覆）、看得到的留言，沒給時是父留言。
func (s *Service) Create(ctx context.Context, c model.Caller, in model.ForumInput) (model.ForumEntry, error) {
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.Comment, in.Body, model.MaxCommentBodyRunes, true)
	if err != nil {
		return model.ForumEntry{}, err
	}
	if in.ReplyToID != nil && in.ParentID == nil {
		return model.ForumEntry{}, apperr.InvalidInputOn([]string{"ReplyToID"}, "%s", text.ReplyParentRequired(uitext.LocaleFrom(ctx)))
	}
	now := s.d.Now().UTC()
	var created store.SocialForumEntry
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		viewer := actor.ViewerFor(c, in.StoreID)
		readable, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.ForumIsReadable(in.StoreID, now, viewer))
		if err != nil {
			return fmt.Errorf("social：讀取店家 %d：%w", in.StoreID, err)
		}
		if !readable {
			return apperr.NotFound("%s", text.ForumStoreNotFound(uitext.LocaleFrom(ctx)))
		}
		a, err := s.d.Actors.ForWrite(ctx, c, in.StoreID, seller.CapMarketing)
		if err != nil {
			return err
		}
		var parentID, replyToID *uuid.UUID
		if in.ParentID != nil {
			parent, err := q.GetForumEntry(ctx, *in.ParentID)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && (parent.DeletedAt != nil || parent.StoreID != in.StoreID) {
				return entryNotFound(ctx)
			}
			if err != nil {
				return fmt.Errorf("social：讀取討論區留言 %s：%w", *in.ParentID, err)
			}
			top := parent.ID
			if parent.ParentID != nil {
				top = *parent.ParentID
			}
			parentID = &top
			target := top
			if in.ReplyToID != nil {
				target = *in.ReplyToID
			}
			ok, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.ForumReplyTargetIsReadable(target, in.StoreID, top, viewer))
			if err != nil {
				return fmt.Errorf("social：讀取討論區留言 %s：%w", target, err)
			}
			if !ok {
				return apperr.InvalidInputOn([]string{"ReplyToID"}, "%s", text.ReplySameThreadOnly(uitext.LocaleFrom(ctx)))
			}
			replyToID = &target
		}
		created, err = q.InsertForumEntry(ctx, store.InsertForumEntryParams{
			ID: s.d.NewID(), StoreID: in.StoreID, ParentID: parentID, ReplyToID: replyToID,
			ActorKind: string(a.Kind), ActorID: a.ID, Body: body, Now: now,
		})
		if err != nil {
			return fmt.Errorf("social：新增討論區留言：%w", err)
		}
		return nil
	})
	if err != nil {
		return model.ForumEntry{}, err
	}
	return visible(created), nil
}

// Update 修改討論區留言：只限作者本人；被隱藏的不能修改；已刪除的是 NotFound（SD7）。
func (s *Service) Update(ctx context.Context, c model.Caller, id uuid.UUID, newText string) (model.ForumEntry, error) {
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.Comment, newText, model.MaxCommentBodyRunes, true)
	if err != nil {
		return model.ForumEntry{}, err
	}
	now := s.d.Now().UTC()
	var updated store.SocialForumEntry
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		// 鎖的順序 identity → buyer → seller → social：作者在鎖之前用不加鎖的讀取確認（作者不會變），買家列的共享鎖先於討論區留言列的鎖（A7-1a R3）。
		pre, err := q.GetForumEntry(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && pre.DeletedAt != nil {
			return entryNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：讀取討論區留言 %s：%w", id, err)
		}
		author := authorOf(pre)
		if !c.IsAuthor(author) {
			return apperr.PermissionDenied("%s", text.CommentEditAuthorOnly(uitext.LocaleFrom(ctx)))
		}
		if err := s.d.Actors.ForAuthor(ctx, c, author, pre.StoreID, seller.CapMarketing); err != nil {
			return err
		}
		row, err := q.LockForumEntry(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && row.DeletedAt != nil {
			return entryNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：鎖定討論區留言 %s：%w", id, err)
		}
		hidden, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.ForumEntryIsHidden(id))
		if err != nil {
			return fmt.Errorf("social：讀取討論區留言的隱藏 %s：%w", id, err)
		}
		if hidden {
			return apperr.FailedPrecondition("%s", text.CommentEditHidden(uitext.LocaleFrom(ctx)))
		}
		updated, err = q.UpdateForumEntryBody(ctx, store.UpdateForumEntryBodyParams{ID: id, Body: body, Now: now})
		if err != nil {
			return fmt.Errorf("social：修改討論區留言 %s：%w", id, err)
		}
		return nil
	})
	if err != nil {
		return model.ForumEntry{}, err
	}
	return visible(updated), nil
}

// Delete 刪除討論區留言（軟刪）：作者本人，或該店。已經刪除的冪等。店家身分的刪除要 CapContentContract。
func (s *Service) Delete(ctx context.Context, c model.Caller, id uuid.UUID) error {
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.LockForumEntry(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return entryNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：鎖定討論區留言 %s：%w", id, err)
		}
		isAuthor := c.IsAuthor(authorOf(row))
		ownsStore := c.OwnsStore(row.StoreID)
		if !isAuthor && !ownsStore {
			return apperr.PermissionDenied("%s", text.ForumDeleteAuthorOrStoreOnly(uitext.LocaleFrom(ctx)))
		}
		if row.DeletedAt != nil {
			return nil
		}
		if ownsStore || row.ActorKind == string(model.ActorStore) {
			if err := seller.Authorize(*c.Store, seller.CapContentContract); err != nil {
				return err
			}
		}
		if _, err := q.SoftDeleteForumEntry(ctx, store.SoftDeleteForumEntryParams{ID: id, Now: &now}); err != nil {
			return fmt.Errorf("social：刪除討論區留言 %s：%w", id, err)
		}
		return nil
	})
}

func toEntry(r reads.EntryRow) model.ForumEntry {
	out := model.ForumEntry{
		ID: r.ID, StoreID: r.StoreID, ParentID: r.ParentID, ReplyToID: r.ReplyToID, State: model.CommentState(r.State),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ReplyCount: r.ReplyCount,
	}
	if out.State == model.CommentVisible {
		out.Actor = model.Actor{Kind: model.ActorKind(r.ActorKind), ID: r.ActorID}
		out.Body = r.Body
	}
	return out
}

func finish(rows []reads.EntryRow, size int32) model.ForumPage {
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.ForumPage{Entries: make([]model.ForumEntry, len(rows))}
	for i, r := range rows {
		out.Entries[i] = toEntry(r)
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.CommentCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out
}

// ListTopLevel 列出店家討論區的頂層留言（依建立時間由舊到新的 keyset）；店家要可曝光而且觀看者沒有封鎖這家店，否則是空的一頁。
// 店主以店家身分讀自己的討論區，不套個人封鎖；被觀看者的封鎖排除的作者，整則不出現。
func (s *Service) ListTopLevel(ctx context.Context, c model.Caller, storeID int64, after *model.CommentCursor, size int32) (model.ForumPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ForumPage{}, err
	}
	at, id := after.Key()
	rows, err := reads.Many[reads.EntryRow](ctx, s.d.DB.Conn(ctx), reads.ListTopLevelForumEntries(storeID, s.d.Now().UTC(), actor.ViewerFor(c, storeID), reads.CommentPage{AfterCreated: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.ForumPage{}, fmt.Errorf("social：列出討論區留言：%w", err)
	}
	return finish(rows, size), nil
}

// ListReplies 列出頂層討論區留言的回覆（父留言可以已刪除或被隱藏，但不能被觀看者的封鎖排除）。父留言不存在是空的一頁。
func (s *Service) ListReplies(ctx context.Context, c model.Caller, parentID uuid.UUID, after *model.CommentCursor, size int32) (model.ForumPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ForumPage{}, err
	}
	parent, err := s.q(ctx).GetForumEntry(ctx, parentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ForumPage{Entries: []model.ForumEntry{}}, nil
	}
	if err != nil {
		return model.ForumPage{}, fmt.Errorf("social：讀取討論區留言 %s：%w", parentID, err)
	}
	at, id := after.Key()
	rows, err := reads.Many[reads.EntryRow](ctx, s.d.DB.Conn(ctx), reads.ListForumReplies(parentID, s.d.Now().UTC(), actor.ViewerFor(c, parent.StoreID), reads.CommentPage{AfterCreated: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.ForumPage{}, fmt.Errorf("social：列出討論區回覆：%w", err)
	}
	return finish(rows, size), nil
}
