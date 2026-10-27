package usecase

import (
	"context"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/moderation"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

type ForumUseCase struct {
	forumRepo    domain.ForumRepository
	buyerClient  BuyerClient
	sellerClient SellerClient
	tsid         func() int64
}

func NewForumUseCase(
	forumRepo domain.ForumRepository,
	buyerClient BuyerClient,
	sellerClient SellerClient,
	tsid func() int64,
) *ForumUseCase {
	return &ForumUseCase{
		forumRepo:    forumRepo,
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		tsid:         tsid,
	}
}

func (uc *ForumUseCase) ListForum(ctx context.Context, sellerID int64, page, pageSize int32) ([]*gen.CmsSellerForum, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	entries, err := uc.forumRepo.ListTopLevel(ctx, sellerID, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.forumRepo.CountTopLevel(ctx, sellerID)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (uc *ForumUseCase) ListReplies(ctx context.Context, parentID int64, page, pageSize int32) ([]*gen.CmsSellerForum, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	replies, err := uc.forumRepo.ListReplies(ctx, parentID, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.forumRepo.CountReplies(ctx, parentID)
	if err != nil {
		return nil, 0, err
	}
	return replies, total, nil
}

// ViewerForForum 回傳呼叫者在這家店討論區的互動身分：討論區屬於呼叫者的店 → 店家，否則買家。
func (uc *ForumUseCase) ViewerForForum(sellerID int64, caller Caller) Actor {
	return caller.ViewerActor(sellerID)
}

// ViewerForEntry 與 ViewerForForum 相同，目標是某則討論區留言所在的討論區。
func (uc *ForumUseCase) ViewerForEntry(ctx context.Context, entryID int64, caller Caller) Actor {
	entry, err := uc.forumRepo.GetByID(ctx, entryID)
	if err != nil || entry == nil {
		return caller.ViewerActor(0)
	}
	return caller.ViewerActor(entry.SellerID)
}

func (uc *ForumUseCase) CreateComment(ctx context.Context, sellerID int64, caller Caller, content string, sellerSuspended bool) (*gen.CmsSellerForum, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrBadRequest
	}
	// B-135: profanity dictionary
	if hit, _ := moderation.Check(content); hit {
		return nil, ErrModerationRejected
	}
	actor, err := caller.DeriveActor(sellerID)
	if err != nil {
		return nil, err
	}
	if sellerSuspended && actor.Kind == domain.ActorKindStore {
		return nil, ErrSellerSuspendedOperationBlocked
	}
	userName, userAvatar := uc.resolveForumIdentity(ctx, actor)
	return uc.forumRepo.Create(ctx, uc.tsid(), sellerID, actor.ID, userName, userAvatar, content, 0, 0, "", actor.Kind)
}

func (uc *ForumUseCase) ReplyComment(ctx context.Context, parentID int64, caller Caller, content string, sellerSuspended bool) (*gen.CmsSellerForum, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrBadRequest
	}
	// B-135: profanity dictionary
	if hit, _ := moderation.Check(content); hit {
		return nil, ErrModerationRejected
	}
	parent, err := uc.forumRepo.GetByID(ctx, parentID)
	if err != nil {
		return nil, ErrNotFound
	}

	// 兩層強制：若 parent 本身是回覆，改為指向其 parent
	actualParentID := parentID
	if parent.ParentID > 0 {
		actualParentID = parent.ParentID
		parent, err = uc.forumRepo.GetByID(ctx, actualParentID)
		if err != nil {
			return nil, ErrNotFound
		}
	}

	actor, err := caller.DeriveActor(parent.SellerID)
	if err != nil {
		return nil, err
	}
	if sellerSuspended && actor.Kind == domain.ActorKindStore {
		return nil, ErrSellerSuspendedOperationBlocked
	}

	userName, userAvatar := uc.resolveForumIdentity(ctx, actor)

	entry, err := uc.forumRepo.Create(ctx, uc.tsid(), parent.SellerID, actor.ID, userName, userAvatar, content, actualParentID, parentID, stringOrEmpty(parent.UserName), actor.Kind)
	if err != nil {
		return nil, err
	}
	_ = uc.forumRepo.IncrReplyCount(ctx, actualParentID)
	return entry, nil
}

func (uc *ForumUseCase) UpdateComment(ctx context.Context, entryID int64, caller Caller, content string) (*gen.CmsSellerForum, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrBadRequest
	}

	entry, err := uc.forumRepo.GetByID(ctx, entryID)
	if err != nil {
		return nil, ErrNotFound
	}
	if entry.DeletedAt != nil {
		return nil, ErrNotFound
	}
	if !caller.IsAuthor(entry.Role, entry.UserID) {
		return nil, ErrForbidden
	}

	updated, err := uc.forumRepo.UpdateContent(ctx, entryID, content)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (uc *ForumUseCase) DeleteComment(ctx context.Context, entryID int64, caller Caller) error {
	entry, err := uc.forumRepo.GetByID(ctx, entryID)
	if err != nil {
		return ErrNotFound
	}

	if !caller.IsAuthor(entry.Role, entry.UserID) && !caller.OwnsStore(entry.SellerID) {
		return ErrForbidden
	}

	if err := uc.forumRepo.SoftDelete(ctx, entryID); err != nil {
		return err
	}
	if entry.ParentID > 0 {
		_ = uc.forumRepo.DecrReplyCount(ctx, entry.ParentID)
	}
	return nil
}
