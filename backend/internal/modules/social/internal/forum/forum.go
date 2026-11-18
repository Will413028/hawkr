// Package forum 是店家的討論區：買家與店家都可以留言與回覆（回覆的父一律壓成頂層）；規則同貼文的留言（作者本人編輯、被隱藏不能編輯、
// 作者或該店刪除且冪等、頂層不可見而底下有可見回覆時回佔位項目），容器是店家：店家要可曝光才公開。
package forum

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
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
		Actor: authorOf(e), Body: model.BodyText(e.Body), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Version: e.Version,
	}
}

func versionConflict(ctx context.Context) error {
	return apperr.VersionConflict("%s", text.CommentVersionConflict(uitext.LocaleFrom(ctx)))
}

// withStats 補上討論區項目此刻的回覆數。
func withStats(ctx context.Context, q *store.Queries, out model.ForumEntry) (model.ForumEntry, error) {
	n, err := q.GetForumEntryReplyCount(ctx, out.ID)
	if err != nil {
		return model.ForumEntry{}, fmt.Errorf("social：讀取討論區留言的計數 %s：%w", out.ID, err)
	}
	out.ReplyCount = n
	return out, nil
}

// currentOf 是重送命中時回的項目：列表讀取算出的目前狀態（已刪除、被隱藏、作者停用後是佔位）與計數，不是第一次建立當下的樣子。
// row 是剛讀到的列，內文與版本直接用它。
func (s *Service) currentOf(ctx context.Context, q *store.Queries, row store.SocialForumEntry) (model.ForumEntry, error) {
	st, err := reads.One[reads.StateRow](ctx, s.d.DB.Conn(ctx), reads.ForumEntryState(row.ID))
	if err != nil {
		return model.ForumEntry{}, fmt.Errorf("social：讀取討論區留言的狀態 %s：%w", row.ID, err)
	}
	out := visible(row)
	if out.State = model.CommentState(st.State); out.State != model.CommentVisible {
		out.Actor, out.Body = model.Actor{}, ""
	}
	return withStats(ctx, q, out)
}

// Readable 回店 storeID 的討論區對呼叫者此刻可不可以讀與寫：店家可曝光，而且（買家身分時）沒有封鎖這家店；店主以店家身分讀，不套個人封鎖。
// 新增留言的前置檢查與 viewer_can_comment 同用這一個判斷。
func (s *Service) Readable(ctx context.Context, c model.Caller, storeID int64, now time.Time) (bool, error) {
	ok, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.ForumIsReadable(storeID, now, actor.ViewerFor(c, storeID)))
	if err != nil {
		return false, fmt.Errorf("social：讀取店家 %d：%w", storeID, err)
	}
	return ok, nil
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
	var replayed model.ForumEntry
	var isReplay bool
	var hash []byte
	if in.RequestID != nil {
		hash = model.RequestHash("forum", rpcID(in.StoreID), model.OptionalID(in.ParentID), model.OptionalID(in.ReplyToID), body)
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		viewer := actor.ViewerFor(c, in.StoreID)
		readable, err := s.Readable(ctx, c, in.StoreID, now)
		if err != nil {
			return err
		}
		if !readable {
			return apperr.NotFound("%s", text.ForumStoreNotFound(uitext.LocaleFrom(ctx)))
		}
		a, err := s.d.Actors.ForWrite(ctx, c, in.StoreID, seller.CapMarketing)
		if err != nil {
			return err
		}
		if row, ok, err := s.replay(ctx, q, a, in.RequestID, hash); err != nil || ok {
			if err == nil {
				isReplay = true
				replayed, err = s.currentOf(ctx, q, row)
			}
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
			ActorKind: string(a.Kind), ActorID: a.ID, Body: body, Now: now, RequestID: in.RequestID, RequestHash: hash,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			// 同一個 (互動者, request_id) 在上面的查詢之後才出現（並行的重送）：INSERT 等到對方 commit 才回 0 列，這時讀得到它。
			row, ok, rerr := s.replay(ctx, q, a, in.RequestID, hash)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return fmt.Errorf("social：討論區留言的 request_id %s 衝突，卻查不到原本那一則", in.RequestID)
			}
			isReplay = true
			replayed, rerr = s.currentOf(ctx, q, row)
			return rerr
		}
		if err != nil {
			return fmt.Errorf("social：新增討論區留言：%w", err)
		}
		return nil
	})
	if err != nil {
		return model.ForumEntry{}, err
	}
	// 討論區可讀可寫是寫入的前置檢查，成功的寫入（含重送命中）就是 true，不再查。
	if isReplay {
		replayed.Readable = true
		return replayed, nil
	}
	out := visible(created)
	out.Readable = true
	return out, nil
}

// replay 查 (互動者, request_id)：命中且內容相同回傳第一次建立的那一則；內容不同是 InvalidInput；沒帶 request_id 或沒命中回 false。
func (s *Service) replay(ctx context.Context, q *store.Queries, a model.Actor, requestID *uuid.UUID, hash []byte) (store.SocialForumEntry, bool, error) {
	if requestID == nil {
		return store.SocialForumEntry{}, false, nil
	}
	row, err := q.ForumEntryByRequest(ctx, store.ForumEntryByRequestParams{ActorKind: string(a.Kind), ActorID: a.ID, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SocialForumEntry{}, false, nil
	}
	if err != nil {
		return store.SocialForumEntry{}, false, fmt.Errorf("social：讀取討論區留言的 request_id %s：%w", requestID, err)
	}
	if !bytes.Equal(row.RequestHash, hash) {
		return store.SocialForumEntry{}, false, apperr.InvalidInputOn([]string{"RequestID"}, "%s", text.RequestIdReused(uitext.LocaleFrom(ctx)))
	}
	return row, true, nil
}

func rpcID(storeID int64) string { return strconv.FormatInt(storeID, 10) }

// Update 修改討論區留言：只限作者本人；被隱藏的不能修改；已刪除的是 NotFound（SD7）。version 與目前的不符回 VersionConflict、不寫入。回傳修改後的完整項目（含回覆數）。
func (s *Service) Update(ctx context.Context, c model.Caller, id uuid.UUID, version int64, newText string) (model.ForumEntry, error) {
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.Comment, newText, model.MaxCommentBodyRunes, true)
	if err != nil {
		return model.ForumEntry{}, err
	}
	now := s.d.Now().UTC()
	var updated model.ForumEntry
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
		if row.Version != version {
			return versionConflict(ctx)
		}
		row2, err := q.UpdateForumEntryBody(ctx, store.UpdateForumEntryBodyParams{ID: id, Body: body, Now: now})
		if err != nil {
			return fmt.Errorf("social：修改討論區留言 %s：%w", id, err)
		}
		updated, err = withStats(ctx, q, visible(row2))
		if err != nil {
			return err
		}
		// 修改不要求討論區可讀，回應的 viewer_can_reply 要用，所以求值。
		updated.Readable, err = s.Readable(ctx, c, pre.StoreID, now)
		return err
	})
	if err != nil {
		return model.ForumEntry{}, err
	}
	return updated, nil
}

// Delete 刪除討論區留言（軟刪）：作者本人，或該店。已經刪除的冪等；version 非 0 時與目前的不符回 VersionConflict。店家身分的刪除要 CapContentContract。
func (s *Service) Delete(ctx context.Context, c model.Caller, id uuid.UUID, version int64) error {
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
		if version != 0 && row.Version != version {
			return versionConflict(ctx)
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
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version, ReplyCount: r.ReplyCount,
	}
	if out.State == model.CommentVisible {
		out.Actor = model.Actor{Kind: model.ActorKind(r.ActorKind), ID: r.ActorID}
		out.Body = r.Body
	}
	return out
}

// finish 把（可能多取一列的）列轉成一頁，並帶這家店的討論區對呼叫者可不可以讀寫：有列就是（列的查詢帶同一個條件），空頁才查一次。
func (s *Service) finish(ctx context.Context, c model.Caller, storeID int64, rows []reads.EntryRow, size int32, now time.Time) (model.ForumPage, error) {
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.ForumPage{Entries: make([]model.ForumEntry, len(rows)), Readable: len(rows) > 0}
	if len(rows) == 0 {
		var err error
		if out.Readable, err = s.Readable(ctx, c, storeID, now); err != nil {
			return model.ForumPage{}, err
		}
	}
	for i, r := range rows {
		out.Entries[i] = toEntry(r)
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.CommentCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
}

// ListTopLevel 列出店家討論區的頂層留言（依建立時間由舊到新的 keyset）；店家要可曝光而且觀看者沒有封鎖這家店，否則是空的一頁。
// 店主以店家身分讀自己的討論區，不套個人封鎖；被觀看者的封鎖排除的作者，整則不出現。
func (s *Service) ListTopLevel(ctx context.Context, c model.Caller, storeID int64, after *model.CommentCursor, size int32) (model.ForumPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ForumPage{}, err
	}
	now := s.d.Now().UTC()
	at, id := after.Key()
	rows, err := reads.Many[reads.EntryRow](ctx, s.d.DB.Conn(ctx), reads.ListTopLevelForumEntries(storeID, now, actor.ViewerFor(c, storeID), reads.CommentPage{AfterCreated: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.ForumPage{}, fmt.Errorf("social：列出討論區留言：%w", err)
	}
	return s.finish(ctx, c, storeID, rows, size, now)
}

// ListReplies 列出店 storeID 的頂層討論區留言 parentID 的回覆（父留言可以已刪除或被隱藏，但不能被觀看者的封鎖排除）。
// 父留言不存在、不是頂層或不屬於 storeID 都是 NotFound，不論這一頁有沒有內容。
func (s *Service) ListReplies(ctx context.Context, c model.Caller, storeID int64, parentID uuid.UUID, after *model.CommentCursor, size int32) (model.ForumPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ForumPage{}, err
	}
	parent, err := s.q(ctx).GetForumEntry(ctx, parentID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (parent.StoreID != storeID || parent.ParentID != nil) {
		return model.ForumPage{}, entryNotFound(ctx)
	}
	if err != nil {
		return model.ForumPage{}, fmt.Errorf("social：讀取討論區留言 %s：%w", parentID, err)
	}
	now := s.d.Now().UTC()
	at, id := after.Key()
	rows, err := reads.Many[reads.EntryRow](ctx, s.d.DB.Conn(ctx), reads.ListForumReplies(parentID, now, actor.ViewerFor(c, parent.StoreID), reads.CommentPage{AfterCreated: at, AfterID: id, Limit: int64(size) + 1}))
	if err != nil {
		return model.ForumPage{}, fmt.Errorf("social：列出討論區回覆：%w", err)
	}
	return s.finish(ctx, c, parent.StoreID, rows, size, now)
}
