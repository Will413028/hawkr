// Package comment 是貼文的留言：兩層（回覆的父一律壓成頂層）、作者本人編輯（被隱藏的內容不能編輯）、作者或貼文所屬店刪除（冪等），
// 與列表（頂層留言不可見而底下有可見的回覆時回佔位項目）。互動者 (kind, id) 由伺服器依貼文歸屬推導，不存作者名稱與頭像快照。
package comment

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
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Deps 是留言服務的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Actors actor.Resolver
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Service 是留言的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// postIsPublic 回貼文對觀看者是不是公開可讀（草稿與下架的貼文、店家不可曝光、觀看者封鎖了這家店時都不是）。
func (s *Service) postIsPublic(ctx context.Context, postID uuid.UUID, now time.Time, v visibility.Viewer) (bool, error) {
	return reads.Flag(ctx, s.d.DB.Conn(ctx), reads.PostIsPublic(postID, now, v))
}

func commentNotFound(ctx context.Context) error {
	return apperr.NotFound("%s", text.CommentNotFound(uitext.LocaleFrom(ctx)))
}
func postNotFound(ctx context.Context) error {
	return apperr.NotFound("%s", text.PostNotFound(uitext.LocaleFrom(ctx)))
}

func authorOf(c store.SocialComment) model.Actor {
	return model.Actor{Kind: model.ActorKind(c.ActorKind), ID: c.ActorID}
}

func visibleComment(c store.SocialComment) model.Comment {
	return model.Comment{
		ID: c.ID, PostID: c.PostID, ParentID: c.ParentID, ReplyToID: c.ReplyToID, State: model.CommentVisible,
		Actor: authorOf(c), Body: model.BodyText(c.Body), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// Create 新增留言或回覆。貼文要公開可讀（草稿與下架的貼文不能留言）。回覆只限貼文所屬的店家（舊 social 的規則）；回覆的父一律壓成
// 頂層，reply_to 只接受同篇同一串（父本身或父底下的回覆）、沒有刪除、沒有隱藏、作者沒有停用的留言（SD4），沒給時是父留言。
func (s *Service) Create(ctx context.Context, c model.Caller, in model.CommentInput) (model.Comment, error) {
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.Comment, in.Body, model.MaxCommentBodyRunes, true)
	if err != nil {
		return model.Comment{}, err
	}
	if in.ReplyToID != nil && in.ParentID == nil {
		return model.Comment{}, apperr.InvalidInputOn([]string{"ReplyToID"}, "%s", text.ReplyParentRequired(uitext.LocaleFrom(ctx)))
	}
	now := s.d.Now().UTC()
	var created store.SocialComment
	var postStore int64
	var replyTo *store.SocialComment
	var a model.Actor
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		post, err := q.GetPost(ctx, in.PostID)
		if errors.Is(err, pgx.ErrNoRows) {
			return postNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：讀取貼文 %s：%w", in.PostID, err)
		}
		postStore = post.SocialPost.StoreID
		viewer := actor.ViewerFor(c, postStore)
		public, err := s.postIsPublic(ctx, in.PostID, now, viewer)
		if err != nil {
			return fmt.Errorf("social：讀取貼文 %s：%w", in.PostID, err)
		}
		if !public {
			return postNotFound(ctx)
		}
		a, err = s.d.Actors.ForWrite(ctx, c, postStore, seller.CapMarketing)
		if err != nil {
			return err
		}
		var parentID *uuid.UUID
		var replyToID *uuid.UUID
		if in.ParentID != nil {
			if a.Kind != model.ActorStore {
				return apperr.PermissionDenied("%s", text.CommentReplyStoreOnly(uitext.LocaleFrom(ctx)))
			}
			parent, err := q.GetComment(ctx, *in.ParentID)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && (parent.SocialComment.DeletedAt != nil || parent.SocialComment.PostID != in.PostID) {
				return commentNotFound(ctx)
			}
			if err != nil {
				return fmt.Errorf("social：讀取留言 %s：%w", *in.ParentID, err)
			}
			top := parent.SocialComment.ID
			if parent.SocialComment.ParentID != nil {
				top = *parent.SocialComment.ParentID
			}
			parentID = &top
			target := top
			if in.ReplyToID != nil {
				target = *in.ReplyToID
			}
			ok, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.ReplyTargetIsReadable(target, in.PostID, top, viewer))
			if err != nil {
				return fmt.Errorf("social：讀取留言 %s：%w", target, err)
			}
			if !ok {
				return apperr.InvalidInputOn([]string{"ReplyToID"}, "%s", text.ReplySameThreadOnly(uitext.LocaleFrom(ctx)))
			}
			replyToID = &target
			rt, err := q.GetComment(ctx, target)
			if err != nil {
				return fmt.Errorf("social：讀取留言 %s：%w", target, err)
			}
			replyTo = &rt.SocialComment
		}
		created, err = q.InsertComment(ctx, store.InsertCommentParams{
			ID: s.d.NewID(), PostID: in.PostID, ParentID: parentID, ReplyToID: replyToID,
			ActorKind: string(a.Kind), ActorID: a.ID, Body: body, Now: now,
		})
		if err != nil {
			return fmt.Errorf("social：新增留言：%w", err)
		}
		preview := model.TruncateRunes(body, model.PreviewMaxRunes)
		if replyTo == nil {
			return events.Publish(ctx, s.d.Events, model.CommentCreated{
				EventID: uuid.NewV7(), OccurredAt: now, CommentID: created.ID, PostID: in.PostID, StoreID: postStore, Actor: a, Preview: preview,
			})
		}
		return events.Publish(ctx, s.d.Events, model.CommentReplied{
			EventID: uuid.NewV7(), OccurredAt: now, CommentID: created.ID, PostID: in.PostID, StoreID: postStore,
			ParentID: *created.ParentID, ReplyToID: replyTo.ID, Actor: a, ReplyToActor: authorOf(*replyTo), Preview: preview,
		})
	})
	if err != nil {
		return model.Comment{}, err
	}
	return visibleComment(created), nil
}

// Update 修改留言：只限作者本人；被隱藏的留言作者不能編輯（SD7）；已刪除的留言是 NotFound。
func (s *Service) Update(ctx context.Context, c model.Caller, id uuid.UUID, newText string) (model.Comment, error) {
	body, err := textcheck.Body(uitext.LocaleFrom(ctx), "Body", textcheck.Comment, newText, model.MaxCommentBodyRunes, true)
	if err != nil {
		return model.Comment{}, err
	}
	now := s.d.Now().UTC()
	var updated store.SocialComment
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		// 鎖的順序 identity → buyer → seller → social：作者與歸屬在鎖之前用不加鎖的讀取確認（作者不會變），買家列的共享鎖先於留言列的鎖。
		// 反過來（先鎖留言、再鎖買家）會與刪帳申請（持有買家列的排他鎖，再由訂閱者鎖作者的內容）互相等待而死結（A7-1a R3）。
		pre, err := q.GetComment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && pre.SocialComment.DeletedAt != nil {
			return commentNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：讀取留言 %s：%w", id, err)
		}
		author := authorOf(pre.SocialComment)
		if !c.IsAuthor(author) {
			return apperr.PermissionDenied("%s", text.CommentEditAuthorOnly(uitext.LocaleFrom(ctx)))
		}
		if err := s.d.Actors.ForAuthor(ctx, c, author, pre.PostStoreID, seller.CapMarketing); err != nil {
			return err
		}
		row, err := q.LockComment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && row.SocialComment.DeletedAt != nil {
			return commentNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：鎖定留言 %s：%w", id, err)
		}
		hidden, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.CommentIsHidden(id))
		if err != nil {
			return fmt.Errorf("social：讀取留言的隱藏 %s：%w", id, err)
		}
		if hidden {
			return apperr.FailedPrecondition("%s", text.CommentEditHidden(uitext.LocaleFrom(ctx)))
		}
		updated, err = q.UpdateCommentBody(ctx, store.UpdateCommentBodyParams{ID: id, Body: body, Now: now})
		if err != nil {
			return fmt.Errorf("social：修改留言 %s：%w", id, err)
		}
		return nil
	})
	if err != nil {
		return model.Comment{}, err
	}
	return visibleComment(updated), nil
}

// Delete 刪除留言（軟刪）：作者本人，或貼文所屬的店。已經刪除的冪等（不做副作用，SD5）。店家身分的刪除（刪自己的，或刪別人的）要
// CapContentContract。
func (s *Service) Delete(ctx context.Context, c model.Caller, id uuid.UUID) error {
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.LockComment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return commentNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("social：鎖定留言 %s：%w", id, err)
		}
		isAuthor := c.IsAuthor(authorOf(row.SocialComment))
		ownsPost := c.OwnsStore(row.PostStoreID)
		if !isAuthor && !ownsPost {
			return apperr.PermissionDenied("%s", text.CommentDeleteAuthorOrStoreOnly(uitext.LocaleFrom(ctx)))
		}
		if row.SocialComment.DeletedAt != nil {
			return nil
		}
		if ownsPost || row.SocialComment.ActorKind == string(model.ActorStore) {
			if err := seller.Authorize(*c.Store, seller.CapContentContract); err != nil {
				return err
			}
		}
		if _, err := q.SoftDeleteComment(ctx, store.SoftDeleteCommentParams{ID: id, Now: &now}); err != nil {
			return fmt.Errorf("social：刪除留言 %s：%w", id, err)
		}
		return nil
	})
}

func toComment(r reads.CommentRow) model.Comment {
	out := model.Comment{
		ID: r.ID, PostID: r.PostID, ParentID: r.ParentID, ReplyToID: r.ReplyToID, State: model.CommentState(r.State),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Likes: r.LikeCount, ReplyCount: r.ReplyCount,
	}
	if out.State == model.CommentVisible {
		out.Actor = model.Actor{Kind: model.ActorKind(r.ActorKind), ID: r.ActorID}
		out.Body = r.Body
	}
	return out
}

// scope 是讀取留言時對貼文的要求：公開讀取要貼文對觀看者公開可讀（觀看者封鎖了這家店時讀不到）；店主讀自己的貼文不必公開，也不套個人封鎖。
type scope struct {
	ownerStore int64 // > 0：店主讀取
	post       store.GetPostRow
}

func (s *Service) scope(ctx context.Context, c model.Caller, postID uuid.UUID) (scope, error) {
	q := s.q(ctx)
	post, err := q.GetPost(ctx, postID)
	if errors.Is(err, pgx.ErrNoRows) {
		return scope{}, postNotFound(ctx)
	}
	if err != nil {
		return scope{}, fmt.Errorf("social：讀取貼文 %s：%w", postID, err)
	}
	if c.OwnsStore(post.SocialPost.StoreID) {
		return scope{ownerStore: post.SocialPost.StoreID, post: post}, nil
	}
	ok, err := s.postIsPublic(ctx, postID, s.d.Now().UTC(), actor.ViewerFor(c, post.SocialPost.StoreID))
	if err != nil {
		return scope{}, fmt.Errorf("social：讀取貼文 %s：%w", postID, err)
	}
	if !ok {
		return scope{}, postNotFound(ctx)
	}
	return scope{post: post}, nil
}

func (s *Service) finish(ctx context.Context, c model.Caller, sc scope, rows []reads.CommentRow, size int32) (model.CommentPage, error) {
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.CommentPage{Comments: make([]model.Comment, len(rows))}
	ids := make([]uuid.UUID, 0, len(rows))
	for i, r := range rows {
		out.Comments[i] = toComment(r)
		if out.Comments[i].State == model.CommentVisible {
			ids = append(ids, r.ID)
		}
	}
	if a, ok := actor.ForViewer(c, sc.post.SocialPost.StoreID); ok && len(ids) > 0 {
		liked, err := s.q(ctx).CommentsLikedBy(ctx, store.CommentsLikedByParams{ActorKind: string(a.Kind), ActorID: a.ID, CommentIds: ids})
		if err != nil {
			return model.CommentPage{}, fmt.Errorf("social：讀取留言的讚：%w", err)
		}
		set := map[uuid.UUID]bool{}
		for _, id := range liked {
			set[id] = true
		}
		for i := range out.Comments {
			out.Comments[i].LikedByMe = set[out.Comments[i].ID]
		}
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.CommentCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
}

// ListTopLevel 列出貼文的頂層留言（依建立時間由舊到新的 keyset，不回總數）。公開讀取要貼文公開可讀（草稿與下架的貼文的留言讀不到，
// SD2）；貼文所屬的店主讀自己的貼文不必公開。頂層留言刪除或被隱藏而底下有可見的回覆時回佔位項目。
func (s *Service) ListTopLevel(ctx context.Context, c model.Caller, postID uuid.UUID, after *model.CommentCursor, size int32) (model.CommentPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.CommentPage{}, err
	}
	sc, err := s.scope(ctx, c, postID)
	if err != nil {
		return model.CommentPage{}, err
	}
	at, id := after.Key()
	pg := reads.CommentPage{AfterCreated: at, AfterID: id, Limit: int64(size) + 1}
	var st reads.Stmt
	if sc.ownerStore > 0 {
		st = reads.ListOwnerTopLevelComments(postID, sc.ownerStore, pg)
	} else {
		st = reads.ListTopLevelComments(postID, s.d.Now().UTC(), actor.ViewerFor(c, sc.post.SocialPost.StoreID), pg)
	}
	rows, err := reads.Many[reads.CommentRow](ctx, s.d.DB.Conn(ctx), st)
	if err != nil {
		return model.CommentPage{}, fmt.Errorf("social：列出留言：%w", err)
	}
	return s.finish(ctx, c, sc, rows, size)
}

// ListReplies 列出頂層留言的回覆（父留言可以已刪除或被隱藏：回覆照常顯示）。規則同 ListTopLevel。
func (s *Service) ListReplies(ctx context.Context, c model.Caller, parentID uuid.UUID, after *model.CommentCursor, size int32) (model.CommentPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.CommentPage{}, err
	}
	q := s.q(ctx)
	parent, err := q.GetComment(ctx, parentID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && parent.SocialComment.ParentID != nil {
		return model.CommentPage{}, commentNotFound(ctx)
	}
	if err != nil {
		return model.CommentPage{}, fmt.Errorf("social：讀取留言 %s：%w", parentID, err)
	}
	sc, err := s.scope(ctx, c, parent.SocialComment.PostID)
	if err != nil {
		return model.CommentPage{}, err
	}
	at, id := after.Key()
	pg := reads.CommentPage{AfterCreated: at, AfterID: id, Limit: int64(size) + 1}
	var st reads.Stmt
	if sc.ownerStore > 0 {
		st = reads.ListOwnerReplies(parentID, sc.ownerStore, pg)
	} else {
		st = reads.ListReplies(parentID, s.d.Now().UTC(), actor.ViewerFor(c, sc.post.SocialPost.StoreID), pg)
	}
	rows, err := reads.Many[reads.CommentRow](ctx, s.d.DB.Conn(ctx), st)
	if err != nil {
		return model.CommentPage{}, fmt.Errorf("social：列出回覆：%w", err)
	}
	return s.finish(ctx, c, sc, rows, size)
}
