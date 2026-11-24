package registry

import (
	"context"
	"slices"
	"strconv"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Party 是事件裡的一方：Side 是 BUYER 或 STORE，ID 是該身分面的號碼（buyer_id 或 store_id）。只存參照，名稱與頭像讀取時向擁有模組解析。
type Party struct {
	Side model.Audience `json:"side"`
	ID   int64          `json:"id"`
}

func (p Party) principal() Principal { return Principal(p) }

// CommentCreatedParams 是 comment_created 的參數快照，欄位取自 social.CommentCreated（Preview 是留言內文，不存）。
type CommentCreatedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	CommentID  uuid.UUID `json:"comment_id"`
	PostID     uuid.UUID `json:"post_id"`
	StoreID    int64     `json:"store_id"`
	Actor      Party     `json:"actor"`
}

// CommentRepliedParams 是 comment_replied_buyer 與 comment_replied_store 的參數快照，欄位取自 social.CommentReplied；
// ReplyTo 是被回覆那則留言的作者，也就是收件人。
type CommentRepliedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	CommentID  uuid.UUID `json:"comment_id"`
	PostID     uuid.UUID `json:"post_id"`
	StoreID    int64     `json:"store_id"`
	ReplyTo    Party     `json:"reply_to"`
	Actor      Party     `json:"actor"`
}

// CommentLikedParams 是 comment_liked_buyer 與 comment_liked_store 的參數快照，欄位取自 social.CommentLiked；Author 是留言作者，也就是收件人。
type CommentLikedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	CommentID  uuid.UUID `json:"comment_id"`
	PostID     uuid.UUID `json:"post_id"`
	StoreID    int64     `json:"store_id"`
	Author     Party     `json:"author"`
	Actor      Party     `json:"actor"`
}

// PostLikedParams 是 post_liked 的參數快照，欄位取自 social.PostLiked。
type PostLikedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	PostID     uuid.UUID `json:"post_id"`
	StoreID    int64     `json:"store_id"`
	BuyerID    int64     `json:"buyer_id"`
}

// StoreFollowedParams 是 store_followed 的參數快照，欄位取自 buyer.SellerFollowed。
type StoreFollowedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	StoreID    int64     `json:"store_id"`
	BuyerID    int64     `json:"buyer_id"`
}

// ReviewCreatedParams 是 review_created 的參數快照，欄位取自 review.ReviewCreated。
type ReviewCreatedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	ReviewID   uuid.UUID `json:"review_id"`
	BuyerID    int64     `json:"buyer_id"`
	StoreID    int64     `json:"store_id"`
	ProductID  int64     `json:"product_id"`
}

// PostPublishedParams 是 post_published 母意圖的參數快照，欄位取自 social.PostPublished。
type PostPublishedParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	PostID     uuid.UUID `json:"post_id"`
	StoreID    int64     `json:"store_id"`
}

// PostPublishedItemParams 是 post_published_item 的參數快照：fan-out 為每位追蹤者建一個。
type PostPublishedItemParams struct {
	PostID  uuid.UUID `json:"post_id"`
	StoreID int64     `json:"store_id"`
	BuyerID int64     `json:"buyer_id"`
}

// ProductRestockedParams 是 product_restocked 的參數快照，欄位取自 buyer.RestockTargeted；ProductName 是事件當時的商品名稱快照（公開目錄資料，受保留期約束）。
type ProductRestockedParams struct {
	EventID     uuid.UUID `json:"event_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	BuyerID     int64     `json:"buyer_id"`
	ProductID   int64     `json:"product_id"`
	ProductName string    `json:"product_name"`
	ImageKey    string    `json:"image_key"`
}

// ChatMessageParams 是 chat_message_buyer 與 chat_message_store 的參數快照，欄位取自 im.MessageSent（訊息內容不存）。
type ChatMessageParams struct {
	EventID        uuid.UUID `json:"event_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	ConversationID uuid.UUID `json:"conversation_id"`
	BuyerID        int64     `json:"buyer_id"`
	StoreID        int64     `json:"store_id"`
}

var (
	exampleEventID = uuid.MustParse("0190a000-0000-7000-8000-0000000000c1")
	exampleObject  = uuid.MustParse("0190a000-0000-7000-8000-0000000000c2")
	examplePostID  = uuid.MustParse("0190a000-0000-7000-8000-0000000000c3")
	exampleStore   = Loaded{StoreCode: 20260001}
)

const (
	exampleStoreID = 7
	exampleBuyerID = 9
)

var (
	exampleBuyerParty = Party{Side: model.AudienceBuyer, ID: exampleBuyerID}
	exampleStoreParty = Party{Side: model.AudienceStore, ID: exampleStoreID}
)

var interactionChannels = []model.Channel{model.ChannelInbox, model.ChannelPush}

// storeCodeLoad 讀取一批參數裡店家的 store_code（一批一次查詢）；店家號碼不合法或查不到的店留 0，深連結因此留空，不讓一列壞資料拖垮整頁。
func storeCodeLoad[P any](storeID func(P) int64) func(context.Context, Sources, []P) ([]Loaded, error) {
	return func(ctx context.Context, src Sources, ps []P) ([]Loaded, error) {
		ids := make([]int64, 0, len(ps))
		for _, p := range ps {
			if id := storeID(p); id > 0 {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		codes, err := src.StoreCodes(ctx, slices.Compact(ids))
		if err != nil {
			return nil, err
		}
		out := make([]Loaded, len(ps))
		for i, p := range ps {
			out[i] = Loaded{StoreCode: codes[storeID(p)]}
		}
		return out, nil
	}
}

func storeCodePath(ld Loaded, suffix string) string {
	if ld.StoreCode <= 0 {
		return ""
	}
	return "/sellers/" + strconv.FormatInt(ld.StoreCode, 10) + suffix
}

func postsLink(ld Loaded) string { return storeCodePath(ld, "/posts") }

var commentCreated = Define(Definition[CommentCreatedParams]{
	Name:              "comment_created",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryComment,
	Presentation:      model.PresentationSocial,
	Channels:          interactionChannels,
	IdempotencySource: "social.CommentCreated.EventID",
	IdempotencyKey:    func(p CommentCreatedParams) string { return p.EventID.String() },
	OccurredAt:        func(p CommentCreatedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p CommentCreatedParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.StoreID, p.Actor.principal())
	},
	Load: storeCodeLoad(func(p CommentCreatedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, _ CommentCreatedParams) (Content, error) {
		return Content{Title: text.CommentCreatedTitle(l), Body: text.CommentCreatedBody(l)}, nil
	},
	DeepLinkLoaded: func(_ CommentCreatedParams, ld Loaded) string { return postsLink(ld) },
	Example: CommentCreatedParams{
		EventID: exampleEventID, OccurredAt: exampleAt, CommentID: exampleObject, PostID: examplePostID, StoreID: exampleStoreID, Actor: exampleBuyerParty,
	},
	ExampleLoaded: exampleStore,
})

func commentReplied(name string, aud model.Audience, recipient func(context.Context, Resolver, CommentRepliedParams) (*Expansion, error),
	title func(uitext.Locale) uitext.Message, body func(uitext.Locale) uitext.Message, replyTo Party,
) Entry {
	return Define(Definition[CommentRepliedParams]{
		Name:              name,
		Audiences:         []model.Audience{aud},
		Category:          model.CategoryComment,
		Presentation:      model.PresentationSocial,
		Channels:          interactionChannels,
		IdempotencySource: "social.CommentReplied.EventID",
		IdempotencyKey:    func(p CommentRepliedParams) string { return p.EventID.String() },
		OccurredAt:        func(p CommentRepliedParams) time.Time { return p.OccurredAt },
		Expand:            recipient,
		Load:              storeCodeLoad(func(p CommentRepliedParams) int64 { return p.StoreID }),
		Text: func(l uitext.Locale, _ CommentRepliedParams) (Content, error) {
			return Content{Title: title(l), Body: body(l)}, nil
		},
		DeepLinkLoaded: func(_ CommentRepliedParams, ld Loaded) string { return postsLink(ld) },
		Example: CommentRepliedParams{
			EventID: exampleEventID, OccurredAt: exampleAt, CommentID: exampleObject, PostID: examplePostID, StoreID: exampleStoreID,
			ReplyTo: replyTo, Actor: otherSide(replyTo),
		},
		ExampleLoaded: exampleStore,
	})
}

func otherSide(p Party) Party {
	if p.Side == model.AudienceBuyer {
		return exampleStoreParty
	}
	return exampleBuyerParty
}

var commentRepliedBuyer = commentReplied("comment_replied_buyer", model.AudienceBuyer,
	func(ctx context.Context, r Resolver, p CommentRepliedParams) (*Expansion, error) {
		return BuyerRecipient(ctx, r, p.ReplyTo.ID, p.Actor.principal())
	},
	func(l uitext.Locale) uitext.Message { return text.CommentRepliedBuyerTitle(l) },
	func(l uitext.Locale) uitext.Message { return text.CommentRepliedBuyerBody(l) },
	exampleBuyerParty)

var commentRepliedStore = commentReplied("comment_replied_store", model.AudienceStore,
	func(ctx context.Context, r Resolver, p CommentRepliedParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.ReplyTo.ID, p.Actor.principal())
	},
	func(l uitext.Locale) uitext.Message { return text.CommentRepliedStoreTitle(l) },
	func(l uitext.Locale) uitext.Message { return text.CommentRepliedStoreBody(l) },
	exampleStoreParty)

func commentLiked(name string, aud model.Audience, recipient func(context.Context, Resolver, CommentLikedParams) (*Expansion, error),
	title func(uitext.Locale) uitext.Message, body func(uitext.Locale) uitext.Message, author Party,
) Entry {
	return Define(Definition[CommentLikedParams]{
		Name:              name,
		Audiences:         []model.Audience{aud},
		Category:          model.CategoryLike,
		Presentation:      model.PresentationSocial,
		Channels:          interactionChannels,
		IdempotencySource: "social.CommentLiked.EventID",
		IdempotencyKey:    func(p CommentLikedParams) string { return p.EventID.String() },
		OccurredAt:        func(p CommentLikedParams) time.Time { return p.OccurredAt },
		Aggregatable:      true,
		AggregationKey:    func(p CommentLikedParams) string { return "comment_like:" + p.CommentID.String() },
		Expand:            recipient,
		Load:              storeCodeLoad(func(p CommentLikedParams) int64 { return p.StoreID }),
		Text: func(l uitext.Locale, _ CommentLikedParams) (Content, error) {
			return Content{Title: title(l), Body: body(l)}, nil
		},
		DeepLinkLoaded: func(_ CommentLikedParams, ld Loaded) string { return postsLink(ld) },
		Example: CommentLikedParams{
			EventID: exampleEventID, OccurredAt: exampleAt, CommentID: exampleObject, PostID: examplePostID, StoreID: exampleStoreID,
			Author: author, Actor: otherSide(author),
		},
		ExampleLoaded: exampleStore,
	})
}

var commentLikedBuyer = commentLiked("comment_liked_buyer", model.AudienceBuyer,
	func(ctx context.Context, r Resolver, p CommentLikedParams) (*Expansion, error) {
		return BuyerRecipient(ctx, r, p.Author.ID, p.Actor.principal())
	},
	func(l uitext.Locale) uitext.Message { return text.CommentLikedBuyerTitle(l) },
	func(l uitext.Locale) uitext.Message { return text.CommentLikedBuyerBody(l) },
	exampleBuyerParty)

var commentLikedStore = commentLiked("comment_liked_store", model.AudienceStore,
	func(ctx context.Context, r Resolver, p CommentLikedParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.Author.ID, p.Actor.principal())
	},
	func(l uitext.Locale) uitext.Message { return text.CommentLikedStoreTitle(l) },
	func(l uitext.Locale) uitext.Message { return text.CommentLikedStoreBody(l) },
	exampleStoreParty)

var postLiked = Define(Definition[PostLikedParams]{
	Name:              "post_liked",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryLike,
	Presentation:      model.PresentationSocial,
	Channels:          interactionChannels,
	IdempotencySource: "social.PostLiked.EventID",
	IdempotencyKey:    func(p PostLikedParams) string { return p.EventID.String() },
	OccurredAt:        func(p PostLikedParams) time.Time { return p.OccurredAt },
	Aggregatable:      true,
	AggregationKey:    func(p PostLikedParams) string { return "post_like:" + p.PostID.String() },
	Expand: func(ctx context.Context, r Resolver, p PostLikedParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.StoreID, Principal{Side: model.AudienceBuyer, ID: p.BuyerID})
	},
	Load: storeCodeLoad(func(p PostLikedParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, _ PostLikedParams) (Content, error) {
		return Content{Title: text.PostLikedTitle(l), Body: text.PostLikedBody(l)}, nil
	},
	DeepLinkLoaded: func(_ PostLikedParams, ld Loaded) string { return postsLink(ld) },
	Example:        PostLikedParams{EventID: exampleEventID, OccurredAt: exampleAt, PostID: examplePostID, StoreID: exampleStoreID, BuyerID: exampleBuyerID},
	ExampleLoaded:  exampleStore,
})

var storeFollowed = Define(Definition[StoreFollowedParams]{
	Name:              "store_followed",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryFollow,
	Presentation:      model.PresentationSocial,
	Channels:          interactionChannels,
	IdempotencySource: "buyer.SellerFollowed.EventID",
	IdempotencyKey:    func(p StoreFollowedParams) string { return p.EventID.String() },
	OccurredAt:        func(p StoreFollowedParams) time.Time { return p.OccurredAt },
	Aggregatable:      true,
	AggregationKey:    func(p StoreFollowedParams) string { return "follow:" + strconv.FormatInt(p.StoreID, 10) },
	Expand: func(ctx context.Context, r Resolver, p StoreFollowedParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.StoreID, Principal{Side: model.AudienceBuyer, ID: p.BuyerID})
	},
	Text: func(l uitext.Locale, _ StoreFollowedParams) (Content, error) {
		return Content{Title: text.StoreFollowedTitle(l), Body: text.StoreFollowedBody(l)}, nil
	},
	Example: StoreFollowedParams{EventID: exampleEventID, OccurredAt: exampleAt, StoreID: exampleStoreID, BuyerID: exampleBuyerID},
})

// 賣家端沒有評價頁，沒有深連結。
var reviewCreated = Define(Definition[ReviewCreatedParams]{
	Name:              "review_created",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryReview,
	Presentation:      model.PresentationReview,
	Channels:          interactionChannels,
	IdempotencySource: "review.ReviewCreated.EventID",
	IdempotencyKey:    func(p ReviewCreatedParams) string { return p.EventID.String() },
	OccurredAt:        func(p ReviewCreatedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p ReviewCreatedParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.StoreID, Principal{Side: model.AudienceBuyer, ID: p.BuyerID})
	},
	Text: func(l uitext.Locale, _ ReviewCreatedParams) (Content, error) {
		return Content{Title: text.ReviewCreatedTitle(l), Body: text.ReviewCreatedBody(l)}, nil
	},
	Example: ReviewCreatedParams{EventID: exampleEventID, OccurredAt: exampleAt, ReviewID: exampleObject, BuyerID: exampleBuyerID, StoreID: exampleStoreID, ProductID: 11},
})

var postPublished = Define(Definition[PostPublishedParams]{
	Name:              "post_published",
	IdempotencySource: "social.PostPublished.PostID",
	IdempotencyKey:    func(p PostPublishedParams) string { return p.PostID.String() },
	OccurredAt:        func(p PostPublishedParams) time.Time { return p.OccurredAt },
	Broadcast: &Broadcast[PostPublishedParams]{
		Child: "post_published_item",
		Targets: func(ctx context.Context, r Resolver, p PostPublishedParams, req PageRequest) (TargetPage, error) {
			page, err := r.StoreFollowers(ctx, p.StoreID, req.After, req.Limit, req.OccurredAt)
			if err != nil {
				return TargetPage{}, err
			}
			targets := make([]Target, len(page))
			for i, f := range page {
				targets[i] = Target{ID: f.BuyerID, Params: PostPublishedItemParams{PostID: p.PostID, StoreID: p.StoreID, BuyerID: f.BuyerID}}
			}
			next, done := FollowersCursor(page, req.After, req.Limit)
			return TargetPage{Targets: targets, Next: next, Done: done}, nil
		},
	},
	Example: PostPublishedParams{EventID: exampleEventID, OccurredAt: exampleAt, PostID: examplePostID, StoreID: exampleStoreID},
})

var postPublishedItem = Define(Definition[PostPublishedItemParams]{
	Name:         "post_published_item",
	Audiences:    []model.Audience{model.AudienceBuyer},
	Category:     model.CategoryFollowedStorePost,
	Presentation: model.PresentationSocial,
	Channels:     interactionChannels,
	Spawned:      true,
	Expand: func(ctx context.Context, r Resolver, p PostPublishedItemParams) (*Expansion, error) {
		return BuyerRecipient(ctx, r, p.BuyerID, Principal{Side: model.AudienceStore, ID: p.StoreID})
	},
	Load: storeCodeLoad(func(p PostPublishedItemParams) int64 { return p.StoreID }),
	Text: func(l uitext.Locale, _ PostPublishedItemParams) (Content, error) {
		return Content{Title: text.PostPublishedItemTitle(l), Body: text.PostPublishedItemBody(l)}, nil
	},
	DeepLinkLoaded: func(_ PostPublishedItemParams, ld Loaded) string { return postsLink(ld) },
	Example:        PostPublishedItemParams{PostID: examplePostID, StoreID: exampleStoreID, BuyerID: exampleBuyerID},
	ExampleLoaded:  exampleStore,
})

// 聊天只推播、不寫收件匣（D3）；深連結的 targetId 是對方：買家端是店家 ID，賣家端是買家 ID。
var chatMessageBuyer = Define(Definition[ChatMessageParams]{
	Name:              "chat_message_buyer",
	Audiences:         []model.Audience{model.AudienceBuyer},
	Category:          model.CategoryChat,
	Presentation:      model.PresentationMessage,
	Channels:          []model.Channel{model.ChannelPush},
	IdempotencySource: "im.MessageSent.EventID",
	IdempotencyKey:    func(p ChatMessageParams) string { return p.EventID.String() },
	OccurredAt:        func(p ChatMessageParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p ChatMessageParams) (*Expansion, error) {
		return BuyerRecipient(ctx, r, p.BuyerID, Principal{Side: model.AudienceStore, ID: p.StoreID})
	},
	Text: func(l uitext.Locale, _ ChatMessageParams) (Content, error) {
		return Content{Title: text.ChatMessageBuyerTitle(l), Body: text.ChatMessageBuyerBody(l)}, nil
	},
	DeepLink: func(p ChatMessageParams) string { return "/account/messages/" + strconv.FormatInt(p.StoreID, 10) },
	Example: ChatMessageParams{
		EventID: exampleEventID, OccurredAt: exampleAt, ConversationID: exampleObject, BuyerID: exampleBuyerID, StoreID: exampleStoreID,
	},
})

var chatMessageStore = Define(Definition[ChatMessageParams]{
	Name:              "chat_message_store",
	Audiences:         []model.Audience{model.AudienceStore},
	Category:          model.CategoryChat,
	Presentation:      model.PresentationMessage,
	Channels:          []model.Channel{model.ChannelPush},
	IdempotencySource: "im.MessageSent.EventID",
	IdempotencyKey:    func(p ChatMessageParams) string { return p.EventID.String() },
	OccurredAt:        func(p ChatMessageParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p ChatMessageParams) (*Expansion, error) {
		return StoreRecipient(ctx, r, p.StoreID, Principal{Side: model.AudienceBuyer, ID: p.BuyerID})
	},
	Text: func(l uitext.Locale, _ ChatMessageParams) (Content, error) {
		return Content{Title: text.ChatMessageStoreTitle(l), Body: text.ChatMessageStoreBody(l)}, nil
	},
	DeepLink: func(p ChatMessageParams) string { return "/seller-center/messages/" + strconv.FormatInt(p.BuyerID, 10) },
	Example: ChatMessageParams{
		EventID: exampleEventID, OccurredAt: exampleAt, ConversationID: exampleObject, BuyerID: exampleBuyerID, StoreID: exampleStoreID,
	},
})

var productRestocked = Define(Definition[ProductRestockedParams]{
	Name:              "product_restocked",
	Audiences:         []model.Audience{model.AudienceBuyer},
	Category:          model.CategoryRestock,
	Presentation:      model.PresentationPromotion,
	Channels:          interactionChannels,
	IdempotencySource: "buyer.RestockTargeted.EventID",
	IdempotencyKey:    func(p ProductRestockedParams) string { return p.EventID.String() },
	OccurredAt:        func(p ProductRestockedParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p ProductRestockedParams) (*Expansion, error) {
		return BuyerRecipient(ctx, r, p.BuyerID)
	},
	Text: func(l uitext.Locale, p ProductRestockedParams) (Content, error) {
		return Content{Title: text.ProductRestockedTitle(l), Body: text.ProductRestockedBody(l, text.ProductRestockedBodyArgs{Product: p.ProductName})}, nil
	},
	DeepLink: func(p ProductRestockedParams) string { return "/products/" + strconv.FormatInt(p.ProductID, 10) },
	ImageKey: func(p ProductRestockedParams) string { return p.ImageKey },
	Example: ProductRestockedParams{
		EventID: exampleEventID, OccurredAt: exampleAt, BuyerID: exampleBuyerID, ProductID: 11, ProductName: "Apple", ImageKey: "public/apple",
	},
})

// contentTypes 是內容側型別（social、buyer 追蹤與補貨、review、im）。
var contentTypes = []Entry{
	commentCreated, commentRepliedBuyer, commentRepliedStore, commentLikedBuyer, commentLikedStore, postLiked, storeFollowed,
	reviewCreated, postPublished, postPublishedItem, chatMessageBuyer, chatMessageStore,
	productRestocked,
}
