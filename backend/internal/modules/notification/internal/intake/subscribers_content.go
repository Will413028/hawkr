package intake

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// 內容側的同步訂閱者，規則同 subscribers.go：只做 Accept。收件人可能是買家或店家的事件各有兩個訂閱者，各自只處理自己那一種收件人。

func partyOf(kind social.ActorKind, id int64) registry.Party {
	side := model.AudienceBuyer
	if kind == social.ActorStore {
		side = model.AudienceStore
	}
	return registry.Party{Side: side, ID: id}
}

// OnCommentCreated 訂閱 social.CommentCreated。
func (s *Service) OnCommentCreated(ctx context.Context, e social.CommentCreated) error {
	return s.Accept(ctx, "comment_created", registry.CommentCreatedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, CommentID: e.CommentID, PostID: e.PostID, StoreID: e.StoreID, Actor: partyOf(e.Actor.Kind, e.Actor.ID),
	})
}

// OnCommentRepliedToBuyer 訂閱 social.CommentReplied：被回覆的留言作者是買家。
func (s *Service) OnCommentRepliedToBuyer(ctx context.Context, e social.CommentReplied) error {
	if e.ReplyToActor.Kind != social.ActorBuyer {
		return nil
	}
	return s.Accept(ctx, "comment_replied_buyer", registry.CommentRepliedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, CommentID: e.CommentID, PostID: e.PostID, StoreID: e.StoreID,
		ReplyTo: partyOf(e.ReplyToActor.Kind, e.ReplyToActor.ID), Actor: partyOf(e.Actor.Kind, e.Actor.ID),
	})
}

// OnCommentRepliedToStore 訂閱 social.CommentReplied：被回覆的留言作者是店家。
func (s *Service) OnCommentRepliedToStore(ctx context.Context, e social.CommentReplied) error {
	if e.ReplyToActor.Kind != social.ActorStore {
		return nil
	}
	return s.Accept(ctx, "comment_replied_store", registry.CommentRepliedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, CommentID: e.CommentID, PostID: e.PostID, StoreID: e.StoreID,
		ReplyTo: partyOf(e.ReplyToActor.Kind, e.ReplyToActor.ID), Actor: partyOf(e.Actor.Kind, e.Actor.ID),
	})
}

// OnCommentLikedBuyerAuthor 訂閱 social.CommentLiked：留言作者是買家。
func (s *Service) OnCommentLikedBuyerAuthor(ctx context.Context, e social.CommentLiked) error {
	if e.Author.Kind != social.ActorBuyer {
		return nil
	}
	return s.Accept(ctx, "comment_liked_buyer", registry.CommentLikedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, CommentID: e.CommentID, PostID: e.PostID, StoreID: e.StoreID,
		Author: partyOf(e.Author.Kind, e.Author.ID), Actor: partyOf(e.Actor.Kind, e.Actor.ID),
	})
}

// OnCommentLikedStoreAuthor 訂閱 social.CommentLiked：留言作者是店家。
func (s *Service) OnCommentLikedStoreAuthor(ctx context.Context, e social.CommentLiked) error {
	if e.Author.Kind != social.ActorStore {
		return nil
	}
	return s.Accept(ctx, "comment_liked_store", registry.CommentLikedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, CommentID: e.CommentID, PostID: e.PostID, StoreID: e.StoreID,
		Author: partyOf(e.Author.Kind, e.Author.ID), Actor: partyOf(e.Actor.Kind, e.Actor.ID),
	})
}

// OnPostLiked 訂閱 social.PostLiked。
func (s *Service) OnPostLiked(ctx context.Context, e social.PostLiked) error {
	return s.Accept(ctx, "post_liked", registry.PostLikedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, PostID: e.PostID, StoreID: e.StoreID, BuyerID: e.BuyerID,
	})
}

// OnPostPublished 訂閱 social.PostPublished：fan-out 的母意圖，追蹤者由 worker 分頁展開。
func (s *Service) OnPostPublished(ctx context.Context, e social.PostPublished) error {
	return s.Accept(ctx, "post_published", registry.PostPublishedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, PostID: e.PostID, StoreID: e.StoreID,
	})
}

// OnSellerFollowed 訂閱 buyer.SellerFollowed。
func (s *Service) OnSellerFollowed(ctx context.Context, e buyer.SellerFollowed) error {
	return s.Accept(ctx, "store_followed", registry.StoreFollowedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, StoreID: e.StoreID, BuyerID: e.BuyerID,
	})
}

// OnReviewCreated 訂閱 review.ReviewCreated。
func (s *Service) OnReviewCreated(ctx context.Context, e review.ReviewCreated) error {
	return s.Accept(ctx, "review_created", registry.ReviewCreatedParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, ReviewID: e.ReviewID, StoreID: e.StoreID, ProductID: e.ProductID,
	})
}

// OnMessageSentToBuyer 訂閱 im.MessageSent：店家傳的訊息，收件人是買家。
func (s *Service) OnMessageSentToBuyer(ctx context.Context, e im.MessageSent) error {
	if e.SenderKind != im.PartyStore {
		return nil
	}
	return s.Accept(ctx, "chat_message_buyer", registry.ChatMessageParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, ConversationID: e.ConversationID, BuyerID: e.BuyerID, StoreID: e.StoreID,
	})
}

// OnMessageSentToStore 訂閱 im.MessageSent：買家傳的訊息，收件人是店家。
func (s *Service) OnMessageSentToStore(ctx context.Context, e im.MessageSent) error {
	if e.SenderKind != im.PartyBuyer {
		return nil
	}
	return s.Accept(ctx, "chat_message_store", registry.ChatMessageParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, ConversationID: e.ConversationID, BuyerID: e.BuyerID, StoreID: e.StoreID,
	})
}
