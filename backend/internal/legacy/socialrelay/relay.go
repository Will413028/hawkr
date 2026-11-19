// Package socialrelay 將 social 與 seller 的模組事件轉成 notification-service 還在消費的社群 RabbitMQ 訊息（notification 還沒搬進單體）。
//
// 對應（全部送到 hawkr.content，source 是 social-service，wire 形狀由 wire_test 逐欄位釘住，模板讀的 key 由 golden_test 對照）：
//
//	social.PostPublished   → post.created，一個追蹤者一則（觸發點是發布，不是建立草稿）
//	social.CommentCreated  → comment.created
//	social.CommentReplied  → comment.replied
//	social.PostLiked       → post.liked
//	social.CommentLiked    → comment.like_changed（只送 action=LIKE，舊 template 本來就只在 LIKE 通知）
//	seller.MeasureCreated  → post.hidden_by_platform，內容下架措施的每則貼文一則；其他措施種類不送
//
// 不轉送：comment.updated、comment.deleted（只給 gateway 的留言 SSE，SSE 刪除）、counter.delta（計數與寫入同 transaction）。
//
// 封鎖雙方之間不送通知（留言、回覆、讚、追蹤的店家的新貼文）：worker 在送出前查 buyer、seller 的封鎖 API，是送出當下的狀態。
// 互動者與收件店家（或被回覆、被按讚的對象）屬同一帳號時不送（自己留言、回覆、讚自己的內容；店主以買家身分追蹤自己的店也不收新貼文），
// 一律在 relay 以帳號比對：notification 的自我略過拿原始的互動者 ID 比店家 ID，兩個 ID 空間不同，只當最後一道。
// 留言與貼文在送出當下已不可見（刪除、隱藏、下架、不公開）時不送。
//
// 隨 notification 搬進單體而刪除。
package socialrelay

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	exchangeContent = "hawkr.content"
	source          = "social-service"

	// followerPageSize 是一個 fan-out job 一次處理的追蹤者數。
	followerPageSize = 100

	oldActorKindBuyer int32 = 1
	oldActorKindStore int32 = 2
	typeSeller              = "seller"
	typeBuyer               = "buyer"
)

type eventFields struct {
	EventID  string    `json:"event_id"`
	Occurred time.Time `json:"occurred"`
}

// followerCursor 是 fan-out 下一頁的位置。
type followerCursor struct {
	Time time.Time `json:"time"`
	ID   int64     `json:"id"`
}

// PostPublishedArgs 轉成 post.created；After 是 fan-out 的位置（第一頁是 nil），一個 job 處理一頁追蹤者再排入下一頁。
type PostPublishedArgs struct {
	eventFields
	PostID  string          `json:"post_id"`
	StoreID int64           `json:"store_id"`
	Preview string          `json:"preview"`
	After   *followerCursor `json:"after,omitempty"`
}

// CommentCreatedArgs 轉成 comment.created。
type CommentCreatedArgs struct {
	eventFields
	CommentID string `json:"comment_id"`
	PostID    string `json:"post_id"`
	StoreID   int64  `json:"store_id"`
	ActorKind string `json:"actor_kind"`
	ActorID   int64  `json:"actor_id"`
	Preview   string `json:"preview"`
}

// CommentRepliedArgs 轉成 comment.replied。
type CommentRepliedArgs struct {
	eventFields
	CommentID   string `json:"comment_id"`
	PostID      string `json:"post_id"`
	StoreID     int64  `json:"store_id"`
	ParentID    string `json:"parent_id"`
	ActorKind   string `json:"actor_kind"`
	ActorID     int64  `json:"actor_id"`
	ReplyToKind string `json:"reply_to_kind"`
	ReplyToID   int64  `json:"reply_to_id"`
	Preview     string `json:"preview"`
}

// PostLikedArgs 轉成 post.liked。
type PostLikedArgs struct {
	eventFields
	PostID  string `json:"post_id"`
	StoreID int64  `json:"store_id"`
	BuyerID int64  `json:"buyer_id"`
}

// CommentLikedArgs 轉成 comment.like_changed（action=LIKE）。
type CommentLikedArgs struct {
	eventFields
	CommentID  string `json:"comment_id"`
	PostID     string `json:"post_id"`
	StoreID    int64  `json:"store_id"`
	ActorKind  string `json:"actor_kind"`
	ActorID    int64  `json:"actor_id"`
	AuthorKind string `json:"author_kind"`
	AuthorID   int64  `json:"author_id"`
}

// ContentTakedownArgs 轉成 post.hidden_by_platform：措施下架的每則貼文一則；reason 在 relay 當下讀措施。
type ContentTakedownArgs struct {
	eventFields
	MeasureID string   `json:"measure_id"`
	StoreID   int64    `json:"store_id"`
	AccountID int64    `json:"account_id"`
	PostIDs   []string `json:"post_ids"`
}

func (PostPublishedArgs) Kind() string  { return "legacy.social_post_published" }
func (CommentCreatedArgs) Kind() string { return "legacy.social_comment_created" }
func (CommentRepliedArgs) Kind() string { return "legacy.social_comment_replied" }
func (PostLikedArgs) Kind() string      { return "legacy.social_post_liked" }
func (CommentLikedArgs) Kind() string   { return "legacy.social_comment_liked" }
func (ContentTakedownArgs) Kind() string {
	return "legacy.social_content_takedown"
}

func fields(id uuid.UUID, at time.Time) eventFields {
	return eventFields{EventID: id.String(), Occurred: at}
}

func kindOf(k social.ActorKind) string { return string(k) }

// OnPostPublished 等 OnX 在發布事件的 transaction 內把事件轉成 job（transactional outbox）。
func OnPostPublished(q jobs.Enqueuer) func(context.Context, social.PostPublished) error {
	return func(ctx context.Context, e social.PostPublished) error {
		return q.Enqueue(ctx, PostPublishedArgs{eventFields: fields(e.EventID, e.OccurredAt), PostID: e.PostID.String(), StoreID: e.StoreID, Preview: e.Preview}, nil)
	}
}

func OnCommentCreated(q jobs.Enqueuer) func(context.Context, social.CommentCreated) error {
	return func(ctx context.Context, e social.CommentCreated) error {
		return q.Enqueue(ctx, CommentCreatedArgs{
			eventFields: fields(e.EventID, e.OccurredAt), CommentID: e.CommentID.String(), PostID: e.PostID.String(), StoreID: e.StoreID,
			ActorKind: kindOf(e.Actor.Kind), ActorID: e.Actor.ID, Preview: e.Preview,
		}, nil)
	}
}

func OnCommentReplied(q jobs.Enqueuer) func(context.Context, social.CommentReplied) error {
	return func(ctx context.Context, e social.CommentReplied) error {
		return q.Enqueue(ctx, CommentRepliedArgs{
			eventFields: fields(e.EventID, e.OccurredAt), CommentID: e.CommentID.String(), PostID: e.PostID.String(), StoreID: e.StoreID,
			ParentID: e.ParentID.String(), ActorKind: kindOf(e.Actor.Kind), ActorID: e.Actor.ID,
			ReplyToKind: kindOf(e.ReplyToActor.Kind), ReplyToID: e.ReplyToActor.ID, Preview: e.Preview,
		}, nil)
	}
}

func OnPostLiked(q jobs.Enqueuer) func(context.Context, social.PostLiked) error {
	return func(ctx context.Context, e social.PostLiked) error {
		return q.Enqueue(ctx, PostLikedArgs{eventFields: fields(e.EventID, e.OccurredAt), PostID: e.PostID.String(), StoreID: e.StoreID, BuyerID: e.BuyerID}, nil)
	}
}

func OnCommentLiked(q jobs.Enqueuer) func(context.Context, social.CommentLiked) error {
	return func(ctx context.Context, e social.CommentLiked) error {
		return q.Enqueue(ctx, CommentLikedArgs{
			eventFields: fields(e.EventID, e.OccurredAt), CommentID: e.CommentID.String(), PostID: e.PostID.String(), StoreID: e.StoreID,
			ActorKind: kindOf(e.Actor.Kind), ActorID: e.Actor.ID, AuthorKind: kindOf(e.Author.Kind), AuthorID: e.Author.ID,
		}, nil)
	}
}

// OnMeasureCreated 只轉內容下架的措施，而且只在對象有貼文時排 job；警告、停權、終止合作什麼都不送。
func OnMeasureCreated(q jobs.Enqueuer) func(context.Context, seller.MeasureCreated) error {
	return func(ctx context.Context, e seller.MeasureCreated) error {
		if e.Kind != seller.MeasureContentTakedown {
			return nil
		}
		var posts []string
		for _, c := range e.Contents {
			if c.Kind == contentkind.Post {
				posts = append(posts, c.ID.String())
			}
		}
		if len(posts) == 0 {
			return nil
		}
		return q.Enqueue(ctx, ContentTakedownArgs{
			eventFields: fields(e.EventID, e.OccurredAt), MeasureID: e.MeasureID.String(), StoreID: e.StoreID, AccountID: e.AccountID, PostIDs: posts,
		}, nil)
	}
}

// Buyers 是 relay 讀買家與封鎖的 port；buyer.Module 實作它。
type Buyers interface {
	ProfileByBuyerID(context.Context, int64) (buyer.Profile, error)
	Subscribers(context.Context, buyer.SubscribersInput) (buyer.Page, error)
	IsBlockedBetween(ctx context.Context, buyerA, buyerB int64) (bool, error)
	IsBlockingStore(ctx context.Context, buyerID, storeID int64) (bool, error)
}

// Stores 是 relay 讀店家、店家封鎖與措施的 port；seller.Module 實作它。
type Stores interface {
	Store(context.Context, int64) (seller.Store, error)
	StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
	Measure(context.Context, uuid.UUID) (seller.Measure, error)
}

// Comments 是 relay 讀留言與可見性的 port；social.Module 實作它。
type Comments interface {
	CommentForRelay(context.Context, uuid.UUID) (social.Comment, error)
	// ContentOwner 的 Exists 與 Readable 是內容此刻存在且公開可讀（留言連同所屬貼文）。
	ContentOwner(context.Context, social.Target) (social.ContentOwner, error)
}

// Worker 組出舊 publisher 的 payload 並排入送出的 job。
type Worker struct {
	Buyers   Buyers
	Stores   Stores
	Comments Comments
	Jobs     jobs.Enqueuer
	Log      *slog.Logger
}

// NewWorker 建立 Worker。
func NewWorker(buyers Buyers, stores Stores, comments Comments, queue jobs.Enqueuer, log *slog.Logger) *Worker {
	return &Worker{Buyers: buyers, Stores: stores, Comments: comments, Jobs: queue, Log: log}
}

type typedWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	work func(context.Context, T) error
}

func (w *typedWorker[T]) Work(ctx context.Context, job *river.Job[T]) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return w.work(ctx, job.Args)
}

// AddWorkers 登記全部 relay 的 River worker。
func (w *Worker) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &typedWorker[PostPublishedArgs]{work: w.postPublished})
	river.AddWorker(workers, &typedWorker[CommentCreatedArgs]{work: w.commentCreated})
	river.AddWorker(workers, &typedWorker[CommentRepliedArgs]{work: w.commentReplied})
	river.AddWorker(workers, &typedWorker[PostLikedArgs]{work: w.postLiked})
	river.AddWorker(workers, &typedWorker[CommentLikedArgs]{work: w.commentLiked})
	river.AddWorker(workers, &typedWorker[ContentTakedownArgs]{work: w.contentTakedown})
}

// baseEvent 是舊 pkg/mq 的 BaseEvent：timestamp 是 time.Time 的 JSON（RFC3339Nano、UTC），version 固定 "1.0"；
// correlation_id 與 metadata 在舊 publisher 是空值，omitempty 之下不出現，這裡不帶。
type baseEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

func base(eventID string, at time.Time, eventType string) baseEvent {
	return baseEvent{EventID: eventID, EventType: eventType, Version: "1.0", Timestamp: at.UTC(), Source: source}
}

// derivedID 是 fan-out 或一個事件對多則訊息時，每則訊息自己的穩定 event_id（版本 8 的 UUID，由事件 ID 與 key 雜湊而來）：
// PublishArgs 以 event_id 做 unique，重試與重複排入只會有一則。舊的 post.created 每則訊息一個隨機 UUID，這裡是穩定的。
func derivedID(eventID, key string) string {
	sum := sha256.Sum256([]byte(eventID + "/" + key))
	var u uuid.UUID
	copy(u[:], sum[:16])
	u[6] = u[6]&0x0f | 0x80
	u[8] = u[8]&0x3f | 0x80
	return u.String()
}

// enqueuePublish 不在 transaction 內排入（relay 要讀 buyer、seller、social）。
func (w *Worker) enqueuePublish(ctx context.Context, eventID, exchange, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.Jobs.Enqueue(ctx, rabbitmq.PublishArgs{EventID: eventID, Exchange: exchange, RoutingKey: key, Body: body}, rabbitmq.InsertOpts())
}

func (w *Worker) warn(ctx context.Context, msg string, args ...any) {
	if w.Log != nil {
		w.Log.WarnContext(ctx, msg, args...)
	}
}

// who 是一個互動者在通知裡的身分：名稱、頭像 objectKey、收件匣的身分空間（"seller" 是 account_id、"buyer" 是 buyer_id）與該空間的值。
type who struct {
	Name, Avatar string
	Space        string
	Principal    int64
	Account      int64
	OldKind      int32
}

// identity 讀互動者此刻的名稱、頭像與身分空間。找不到（已刪帳或已退場）是永久狀態：名稱與頭像留空、principal 原樣回傳輸入（舊
// ResolvePrincipal 失敗時的行為），不讓 job 重試到上限。
func (w *Worker) identity(ctx context.Context, kind social.ActorKind, id int64) (who, error) {
	switch kind {
	case social.ActorStore:
		s, err := w.Stores.Store(ctx, id)
		if apperr.KindOf(err) == apperr.KindNotFound {
			w.warn(ctx, "socialrelay：店家不存在，名稱留空", "store_id", id)
			return who{Space: typeSeller, Principal: id, OldKind: oldActorKindStore}, nil
		}
		if err != nil {
			return who{}, err
		}
		out := who{Name: s.StoreName, Space: typeSeller, Principal: s.AccountID, Account: s.AccountID, OldKind: oldActorKindStore}
		if s.LogoKey != nil {
			out.Avatar = *s.LogoKey
		}
		return out, nil
	case social.ActorBuyer:
		p, err := w.Buyers.ProfileByBuyerID(ctx, id)
		if apperr.KindOf(err) == apperr.KindNotFound {
			w.warn(ctx, "socialrelay：買家不存在，名稱留空", "buyer_id", id)
			return who{Space: typeBuyer, Principal: id, OldKind: oldActorKindBuyer}, nil
		}
		if err != nil {
			return who{}, err
		}
		out := who{Name: p.DisplayName, Space: typeBuyer, Principal: id, Account: p.AccountID, OldKind: oldActorKindBuyer}
		if p.Avatar != nil {
			out.Avatar = *p.Avatar
		}
		return out, nil
	}
	return who{}, river.JobCancel(fmt.Errorf("socialrelay：不認得的互動者種類 %q", kind))
}

// blocked 回報兩個互動者之間任一方向是否有封鎖：買家與買家（雙向）、買家封鎖店、店封鎖買家；店與店沒有封鎖。
func (w *Worker) blocked(ctx context.Context, a social.ActorKind, aID int64, b social.ActorKind, bID int64) (bool, error) {
	switch {
	case a == social.ActorBuyer && b == social.ActorBuyer:
		return w.Buyers.IsBlockedBetween(ctx, aID, bID)
	case a == social.ActorBuyer && b == social.ActorStore:
		return w.buyerStore(ctx, aID, bID)
	case a == social.ActorStore && b == social.ActorBuyer:
		return w.buyerStore(ctx, bID, aID)
	}
	return false, nil
}

func (w *Worker) buyerStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	if ok, err := w.Buyers.IsBlockingStore(ctx, buyerID, storeID); err != nil || ok {
		return ok, err
	}
	return w.Stores.StoreBlocksBuyer(ctx, storeID, buyerID)
}

func (w *Worker) skipBlocked(ctx context.Context, event string, eventID string, a social.ActorKind, aID int64, b social.ActorKind, bID int64) (bool, error) {
	ok, err := w.blocked(ctx, a, aID, b, bID)
	if err != nil {
		return false, err
	}
	if ok {
		w.warn(ctx, "socialrelay：封鎖雙方之間不送互動通知", "event", event, "event_id", eventID)
	}
	return ok, nil
}

// parseID 解析 job 帶的 UUID；壞掉的 ID 重試也不會好，直接取消。
func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, river.JobCancel(fmt.Errorf("socialrelay：壞掉的 ID %q：%w", s, err))
	}
	return id, nil
}

// selfAction 回報兩個互動者是否屬同一帳號（互動者在自己的內容上互動）；任一方沒有帳號（已刪除）不算。
func (w *Worker) selfAction(ctx context.Context, event, eventID string, a, b who) bool {
	if a.Account == 0 || a.Account != b.Account {
		return false
	}
	w.warn(ctx, "socialrelay：自己對自己的內容互動，不送通知", "event", event, "event_id", eventID)
	return true
}

// visible 回報內容此刻是否還存在且公開可讀；不是的話記一筆並讓呼叫方略過。
func (w *Worker) visible(ctx context.Context, kind social.TargetKind, id uuid.UUID, event, eventID string) (bool, error) {
	o, err := w.Comments.ContentOwner(ctx, social.Target{Kind: kind, ID: id})
	if err != nil {
		return false, err
	}
	if !o.Exists || !o.Readable {
		w.warn(ctx, "socialrelay：內容已不可見，不送通知", "event", event, "event_id", eventID)
		return false, nil
	}
	return true, nil
}

// postCreated 是舊 PostCreatedEvent：id 是 JSON 數字，post_id 是貼文 UUID 字串。
type postCreated struct {
	baseEvent
	PostID          string `json:"post_id"`
	StoreID         int64  `json:"store_id"`
	FollowerID      int64  `json:"follower_id"`
	SellerName      string `json:"seller_name"`
	SellerAvatarKey string `json:"seller_avatar_key"`
	ContentPreview  string `json:"content_preview"`
}

func (w *Worker) postPublished(ctx context.Context, a PostPublishedArgs) error {
	postID, err := parseID(a.PostID)
	if err != nil {
		return err
	}
	if ok, err := w.visible(ctx, social.TargetPost, postID, "post.created", a.EventID); err != nil || !ok {
		return err
	}
	store, err := w.identity(ctx, social.ActorStore, a.StoreID)
	if err != nil {
		return err
	}
	in := buyer.SubscribersInput{StoreID: a.StoreID, PageSize: followerPageSize}
	if a.After != nil {
		in.After = &buyer.Cursor{Time: a.After.Time, ID: a.After.ID}
	}
	page, err := w.Buyers.Subscribers(ctx, in)
	if err != nil {
		return err
	}
	for _, f := range page.Items {
		if skip, err := w.skipBlocked(ctx, "post.created", a.EventID, social.ActorBuyer, f.ID, social.ActorStore, a.StoreID); err != nil {
			return err
		} else if skip {
			continue
		}
		follower, err := w.identity(ctx, social.ActorBuyer, f.ID)
		if err != nil {
			return err
		}
		if w.selfAction(ctx, "post.created", a.EventID, follower, store) {
			continue
		}
		id := derivedID(a.EventID, strconv.FormatInt(f.ID, 10))
		if err := w.enqueuePublish(ctx, id, exchangeContent, "post.created", postCreated{
			baseEvent: base(id, a.Occurred, "POST_CREATED"), PostID: a.PostID, StoreID: a.StoreID, FollowerID: f.ID,
			SellerName: store.Name, SellerAvatarKey: store.Avatar, ContentPreview: a.Preview,
		}); err != nil {
			return err
		}
	}
	if page.Next == nil {
		return nil
	}
	next := a
	next.After = &followerCursor{Time: page.Next.Time, ID: page.Next.ID}
	return w.Jobs.Enqueue(ctx, next, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
}

// commentSnapshot 是舊 CommentSnapshot：id 的 ,string 在 UUID 之下不再需要（字串本來就是 JSON 字串）；沒有的 parentId、replyToId 是空字串
// （舊的是 "0"，只有 gateway 的留言 SSE 讀它，SSE 已刪除，notification 不讀 comment 欄位）。
type commentSnapshot struct {
	CommentID   string `json:"commentId"`
	PostID      string `json:"postId"`
	UserID      int64  `json:"userId,string"`
	UserName    string `json:"userName"`
	AvatarKey   string `json:"avatarKey"`
	Role        int32  `json:"role"`
	Content     string `json:"content"`
	ParentID    string `json:"parentId"`
	ReplyToID   string `json:"replyToId"`
	ReplyToName string `json:"replyToName"`
	LikeCount   int32  `json:"likeCount"`
	ReplyCount  int32  `json:"replyCount"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func snapshotOf(c social.Comment, author who, replyToName string) *commentSnapshot {
	s := &commentSnapshot{
		CommentID: c.ID.String(), PostID: c.PostID.String(), UserID: c.Actor.ID, UserName: author.Name, AvatarKey: author.Avatar,
		Role: author.OldKind, Content: c.Body, ReplyToName: replyToName, LikeCount: count32(c.Likes), ReplyCount: count32(c.ReplyCount),
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c.ParentID != nil {
		s.ParentID = c.ParentID.String()
	}
	if c.ReplyToID != nil {
		s.ReplyToID = c.ReplyToID.String()
	}
	return s
}

// commentCreated 是舊 CommentCreatedEvent。clientRequestId 不送：它是舊 SSE 的去重用欄位（omitempty），notification 不讀。
type commentCreated struct {
	baseEvent
	CommentID            string           `json:"commentId"`
	PostID               string           `json:"postId"`
	CommenterID          int64            `json:"commenterId"`
	CommenterPrincipalID int64            `json:"commenterPrincipalId"`
	CommenterName        string           `json:"commenterName"`
	PostAuthorID         int64            `json:"postAuthorId"`
	PostAuthorStoreID    int64            `json:"postAuthorStoreId"`
	PostAuthorType       string           `json:"postAuthorType"`
	ContentPreview       string           `json:"contentPreview"`
	ParentCommentID      string           `json:"parentCommentId,omitempty"`
	ClientRequestID      string           `json:"clientRequestId,omitempty"`
	Comment              *commentSnapshot `json:"comment,omitempty"`
}

func (w *Worker) commentCreated(ctx context.Context, a CommentCreatedArgs) error {
	commentID, err := parseID(a.CommentID)
	if err != nil {
		return err
	}
	if ok, err := w.visible(ctx, social.TargetComment, commentID, "comment.created", a.EventID); err != nil || !ok {
		return err
	}
	kind := social.ActorKind(a.ActorKind)
	if skip, err := w.skipBlocked(ctx, "comment.created", a.EventID, kind, a.ActorID, social.ActorStore, a.StoreID); err != nil || skip {
		return err
	}
	commenter, err := w.identity(ctx, kind, a.ActorID)
	if err != nil {
		return err
	}
	// 貼文作者恆為店家。
	author, err := w.identity(ctx, social.ActorStore, a.StoreID)
	if err != nil {
		return err
	}
	if w.selfAction(ctx, "comment.created", a.EventID, commenter, author) {
		return nil
	}
	c, err := w.Comments.CommentForRelay(ctx, commentID)
	if err != nil {
		return err
	}
	snap := snapshotOf(c, commenter, "")
	return w.enqueuePublish(ctx, a.EventID, exchangeContent, "comment.created", commentCreated{
		baseEvent: base(a.EventID, a.Occurred, "COMMENT_CREATED"), CommentID: a.CommentID, PostID: a.PostID,
		CommenterID: a.ActorID, CommenterPrincipalID: commenter.Principal, CommenterName: commenter.Name,
		PostAuthorID: author.Principal, PostAuthorStoreID: a.StoreID, PostAuthorType: typeSeller, ContentPreview: a.Preview, Comment: snap,
	})
}

// commentReplied 是舊 CommentRepliedEvent。
type commentReplied struct {
	baseEvent
	ReplyToUserID      int64            `json:"replyToUserId"`
	ReplyToUserStoreID int64            `json:"replyToUserStoreId"`
	ReplyToUserType    string           `json:"replyToUserType"`
	ReplierID          int64            `json:"replierId"`
	ReplierPrincipalID int64            `json:"replierPrincipalId"`
	ReplierName        string           `json:"replierName"`
	PostID             string           `json:"postId"`
	CommentID          string           `json:"commentId"`
	ContentPreview     string           `json:"contentPreview"`
	ParentCommentID    string           `json:"parentCommentId,omitempty"`
	ClientRequestID    string           `json:"clientRequestId,omitempty"`
	Comment            *commentSnapshot `json:"comment,omitempty"`
}

func (w *Worker) commentReplied(ctx context.Context, a CommentRepliedArgs) error {
	commentID, err := parseID(a.CommentID)
	if err != nil {
		return err
	}
	if ok, err := w.visible(ctx, social.TargetComment, commentID, "comment.replied", a.EventID); err != nil || !ok {
		return err
	}
	kind, to := social.ActorKind(a.ActorKind), social.ActorKind(a.ReplyToKind)
	if skip, err := w.skipBlocked(ctx, "comment.replied", a.EventID, kind, a.ActorID, to, a.ReplyToID); err != nil || skip {
		return err
	}
	replier, err := w.identity(ctx, kind, a.ActorID)
	if err != nil {
		return err
	}
	target, err := w.identity(ctx, to, a.ReplyToID)
	if err != nil {
		return err
	}
	if w.selfAction(ctx, "comment.replied", a.EventID, replier, target) {
		return nil
	}
	c, err := w.Comments.CommentForRelay(ctx, commentID)
	if err != nil {
		return err
	}
	snap := snapshotOf(c, replier, target.Name)
	return w.enqueuePublish(ctx, a.EventID, exchangeContent, "comment.replied", commentReplied{
		baseEvent:     base(a.EventID, a.Occurred, "COMMENT_REPLIED"),
		ReplyToUserID: target.Principal, ReplyToUserStoreID: a.ReplyToID, ReplyToUserType: target.Space,
		ReplierID: a.ActorID, ReplierPrincipalID: replier.Principal, ReplierName: replier.Name,
		PostID: a.PostID, CommentID: a.CommentID, ContentPreview: a.Preview, ParentCommentID: a.ParentID, Comment: snap,
	})
}

// postLiked 是舊 PostLikedEvent：seller_id 是店家的 account_id。
type postLiked struct {
	baseEvent
	SellerID  int64  `json:"seller_id"`
	StoreID   int64  `json:"store_id"`
	LikerName string `json:"likerName"`
	PostID    string `json:"post_id"`
}

func (w *Worker) postLiked(ctx context.Context, a PostLikedArgs) error {
	postID, err := parseID(a.PostID)
	if err != nil {
		return err
	}
	if ok, err := w.visible(ctx, social.TargetPost, postID, "post.liked", a.EventID); err != nil || !ok {
		return err
	}
	if skip, err := w.skipBlocked(ctx, "post.liked", a.EventID, social.ActorBuyer, a.BuyerID, social.ActorStore, a.StoreID); err != nil || skip {
		return err
	}
	liker, err := w.identity(ctx, social.ActorBuyer, a.BuyerID)
	if err != nil {
		return err
	}
	author, err := w.identity(ctx, social.ActorStore, a.StoreID)
	if err != nil {
		return err
	}
	if author.Account == 0 {
		return river.JobCancel(fmt.Errorf("socialrelay：貼文的店家 %d 不存在，沒有收件帳號", a.StoreID))
	}
	if w.selfAction(ctx, "post.liked", a.EventID, liker, author) {
		return nil
	}
	return w.enqueuePublish(ctx, a.EventID, exchangeContent, "post.liked", postLiked{
		baseEvent: base(a.EventID, a.Occurred, "POST_LIKED"), SellerID: author.Principal, StoreID: a.StoreID, LikerName: liker.Name, PostID: a.PostID,
	})
}

// commentLikeChanged 是舊 CommentLikeChangedEvent；likeCount 是 relay 當下的真值（計數與讚同 transaction 更新，舊的是樂觀的 +1）。
type commentLikeChanged struct {
	baseEvent
	CommentID            string `json:"commentId"`
	PostID               string `json:"postId"`
	LikeCount            int32  `json:"likeCount"`
	Action               string `json:"action"`
	UserID               int64  `json:"userId"`
	LikerKind            int32  `json:"likerKind"`
	CommentAuthorID      int64  `json:"commentAuthorId"`
	CommentAuthorStoreID int64  `json:"commentAuthorStoreId"`
	LikerPrincipalID     int64  `json:"likerPrincipalId"`
	LikerName            string `json:"likerName"`
	CommentAuthorType    string `json:"commentAuthorType"`
}

func (w *Worker) commentLiked(ctx context.Context, a CommentLikedArgs) error {
	commentID, err := parseID(a.CommentID)
	if err != nil {
		return err
	}
	if ok, err := w.visible(ctx, social.TargetComment, commentID, "comment.like_changed", a.EventID); err != nil || !ok {
		return err
	}
	kind, ak := social.ActorKind(a.ActorKind), social.ActorKind(a.AuthorKind)
	if skip, err := w.skipBlocked(ctx, "comment.like_changed", a.EventID, kind, a.ActorID, ak, a.AuthorID); err != nil || skip {
		return err
	}
	liker, err := w.identity(ctx, kind, a.ActorID)
	if err != nil {
		return err
	}
	author, err := w.identity(ctx, ak, a.AuthorID)
	if err != nil {
		return err
	}
	if w.selfAction(ctx, "comment.like_changed", a.EventID, liker, author) {
		return nil
	}
	c, err := w.Comments.CommentForRelay(ctx, commentID)
	if err != nil {
		return err
	}
	return w.enqueuePublish(ctx, a.EventID, exchangeContent, "comment.like_changed", commentLikeChanged{
		baseEvent: base(a.EventID, a.Occurred, "COMMENT_LIKE_CHANGED"), CommentID: a.CommentID, PostID: a.PostID, LikeCount: count32(c.Likes),
		Action: "LIKE", UserID: a.ActorID, LikerKind: liker.OldKind, CommentAuthorID: author.Principal, CommentAuthorStoreID: a.AuthorID,
		LikerPrincipalID: liker.Principal, LikerName: liker.Name, CommentAuthorType: author.Space,
	})
}

// postHiddenByPlatform 是舊 PostHiddenByPlatformEvent：seller_id 是店家的 account_id；reason 是措施給賣家的說明（StatementToSeller）。
type postHiddenByPlatform struct {
	baseEvent
	SellerID int64  `json:"seller_id"`
	StoreID  int64  `json:"store_id"`
	PostID   string `json:"post_id"`
	Reason   string `json:"reason"`
}

func (w *Worker) contentTakedown(ctx context.Context, a ContentTakedownArgs) error {
	id, err := uuid.Parse(a.MeasureID)
	if err != nil {
		return river.JobCancel(err)
	}
	m, err := w.Stores.Measure(ctx, id)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return river.JobCancel(fmt.Errorf("socialrelay：找不到措施 %s：%w", id, err))
	}
	if err != nil {
		return err
	}
	for _, post := range a.PostIDs {
		eventID := derivedID(a.EventID, post)
		if err := w.enqueuePublish(ctx, eventID, exchangeContent, "post.hidden_by_platform", postHiddenByPlatform{
			baseEvent: base(eventID, a.Occurred, "POST_HIDDEN_BY_PLATFORM"), SellerID: a.AccountID, StoreID: a.StoreID, PostID: post, Reason: m.StatementToSeller,
		}); err != nil {
			return err
		}
	}
	return nil
}

// count32 把計數轉成舊 wire 的 int32，超過範圍時取上限（舊的欄位就是 int32）。
func count32(n int64) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n) //nolint:gosec // 上面已限制上界，計數不為負
}
