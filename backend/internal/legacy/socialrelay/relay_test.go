package socialrelay

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type recordingJobs struct {
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}

func (q *recordingJobs) published() []rabbitmq.PublishArgs {
	var out []rabbitmq.PublishArgs
	for _, a := range q.args {
		if p, ok := a.(rabbitmq.PublishArgs); ok {
			out = append(out, p)
		}
	}
	return out
}

type fakeBuyers struct {
	profiles      map[int64]buyer.Profile
	pages         map[int64]buyer.Page // After.ID（第一頁是 0）→ 頁
	blockedBetway bool
	blockingStore bool
}

func (b *fakeBuyers) ProfileByBuyerID(_ context.Context, id int64) (buyer.Profile, error) {
	p, ok := b.profiles[id]
	if !ok {
		return buyer.Profile{}, apperr.NotFound("no buyer")
	}
	return p, nil
}

func (b *fakeBuyers) Subscribers(_ context.Context, in buyer.SubscribersInput) (buyer.Page, error) {
	var after int64
	if in.After != nil {
		after = in.After.ID
	}
	return b.pages[after], nil
}

func (b *fakeBuyers) IsBlockedBetween(context.Context, int64, int64) (bool, error) {
	return b.blockedBetway, nil
}

func (b *fakeBuyers) IsBlockingStore(context.Context, int64, int64) (bool, error) {
	return b.blockingStore, nil
}

type fakeStores struct {
	stores      map[int64]seller.Store
	storeBlocks bool
	measure     seller.Measure
	measureErr  error
}

func (s *fakeStores) Store(_ context.Context, id int64) (seller.Store, error) {
	st, ok := s.stores[id]
	if !ok {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return st, nil
}

func (s *fakeStores) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	return s.storeBlocks, nil
}

func (s *fakeStores) Measure(context.Context, uuid.UUID) (seller.Measure, error) {
	return s.measure, s.measureErr
}

type fakeComments struct {
	c        social.Comment
	notShown bool
	gone     bool
}

func (f *fakeComments) CommentForRelay(context.Context, uuid.UUID) (social.Comment, error) {
	return f.c, nil
}

func (f *fakeComments) CommentLikeCount(context.Context, uuid.UUID) (int64, error) {
	return f.c.Likes, nil
}

func (f *fakeComments) ContentOwner(context.Context, social.Target) (social.ContentOwner, error) {
	return social.ContentOwner{Exists: !f.gone, Readable: !f.notShown}, nil
}

var (
	eventID    = uuid.MustParse("01990000-0000-7000-8000-000000000001")
	postID     = uuid.MustParse("01990000-0000-7000-8000-0000000000a1")
	commentID  = uuid.MustParse("01990000-0000-7000-8000-0000000000b1")
	parentID   = uuid.MustParse("01990000-0000-7000-8000-0000000000b2")
	replyToID  = uuid.MustParse("01990000-0000-7000-8000-0000000000b3")
	measureID  = uuid.MustParse("01990000-0000-7000-8000-0000000000c1")
	occurredAt = time.Date(2026, 10, 5, 9, 30, 15, 123456789, time.FixedZone("TST", 8*3600))
	// timestamp 是 Go time.Time 的 JSON：UTC、RFC3339Nano。
	wireTimestamp = "2026-10-05T01:30:15.123456789Z"
	createdAt     = time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
)

func strp(s string) *string { return &s }

func newStore(storeID, accountID int64, name, logo string) seller.Store {
	s := seller.Store{StoreID: storeID, AccountID: accountID, LogoKey: strp(logo)}
	s.StoreName = name
	return s
}

func newFixture() (*Worker, *fakeBuyers, *fakeStores, *recordingJobs) {
	w, b, s, q, _ := newFixtureWithComments()
	return w, b, s, q
}

func newFixtureWithComments() (*Worker, *fakeBuyers, *fakeStores, *recordingJobs, *fakeComments) {
	b := &fakeBuyers{
		profiles: map[int64]buyer.Profile{
			301: {BuyerID: 301, DisplayName: "小明", Avatar: strp("avatars/301")},
			302: {BuyerID: 302, DisplayName: "小華"},
			303: {BuyerID: 303, AccountID: 7001, DisplayName: "店主本人"},
		},
		pages: map[int64]buyer.Page{},
	}
	s := &fakeStores{stores: map[int64]seller.Store{91: newStore(91, 7001, "好店", "logos/91")}}
	q := &recordingJobs{}
	comment := social.Comment{
		ID: commentID, PostID: postID, Actor: social.Actor{Kind: social.ActorBuyer, ID: 301}, Body: "留言全文",
		CreatedAt: createdAt, UpdatedAt: createdAt, Likes: 3, ReplyCount: 1,
	}
	fc := &fakeComments{c: comment}
	return NewWorker(b, s, fc, q, nil), b, s, q, fc
}

// run 把模組事件交給 OnX，取出排入的 job，再交給 worker，回傳送出的訊息。
func run[E any](t *testing.T, w *Worker, on func(jobs.Enqueuer) func(context.Context, E) error, event E) []rabbitmq.PublishArgs {
	t.Helper()
	relayed := &recordingJobs{}
	if err := on(relayed)(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(relayed.args) != 1 {
		t.Fatalf("relay jobs=%d", len(relayed.args))
	}
	published := &recordingJobs{}
	w2 := *w
	w2.Jobs = published
	if err := work(&w2, relayed.args[0]); err != nil {
		t.Fatal(err)
	}
	return published.published()
}

func work(w *Worker, args river.JobArgs) error {
	ctx := context.Background()
	switch a := args.(type) {
	case PostPublishedArgs:
		return w.postPublished(ctx, a)
	case CommentCreatedArgs:
		return w.commentCreated(ctx, a)
	case CommentRepliedArgs:
		return w.commentReplied(ctx, a)
	case PostLikedArgs:
		return w.postLiked(ctx, a)
	case CommentLikedArgs:
		return w.commentLiked(ctx, a)
	case ContentTakedownArgs:
		return w.contentTakedown(ctx, a)
	}
	panic("unknown relay job")
}

func body(t *testing.T, a rabbitmq.PublishArgs) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(a.Body, &m); err != nil {
		t.Fatal(err)
	}
	if a.EventID != m["event_id"] {
		t.Fatalf("PublishArgs.EventID=%q body event_id=%v", a.EventID, m["event_id"])
	}
	return m
}

func check(t *testing.T, a rabbitmq.PublishArgs, key string, want map[string]any) {
	t.Helper()
	if a.Exchange != "hawkr.content" || a.RoutingKey != key {
		t.Errorf("published to %s/%s, want hawkr.content/%s", a.Exchange, a.RoutingKey, key)
	}
	if got := body(t, a); !reflect.DeepEqual(got, want) {
		t.Errorf("%s payload\n got  %#v\n want %#v", key, got, want)
	}
}

func baseWire(id, typ string) map[string]any {
	return map[string]any{"event_id": id, "event_type": typ, "version": "1.0", "timestamp": wireTimestamp, "source": "social-service"}
}

func with(m map[string]any, kv map[string]any) map[string]any {
	for k, v := range kv {
		m[k] = v
	}
	return m
}

func TestPostCreatedIsOneMessagePerFollowerAndPaginates(t *testing.T) {
	w, b, _, _ := newFixture()
	b.pages[0] = buyer.Page{Items: []buyer.Item{{ID: 301}, {ID: 302}}}
	got := run(t, w, OnPostPublished, social.PostPublished{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91, Preview: "預覽"})
	if len(got) != 2 {
		t.Fatalf("messages=%d, want one per follower", len(got))
	}
	if got[0].EventID == got[1].EventID || got[0].EventID == eventID.String() {
		t.Errorf("each follower message needs its own stable event id: %q %q", got[0].EventID, got[1].EventID)
	}
	for i, f := range []int64{301, 302} {
		check(t, got[i], "post.created", with(baseWire(got[i].EventID, "POST_CREATED"), map[string]any{
			"post_id": postID.String(), "store_id": float64(91), "follower_id": float64(f),
			"seller_name": "好店", "seller_avatar_key": "logos/91", "content_preview": "預覽",
		}))
	}

	// 兩頁：第一頁帶 Next，relay 排入下一頁的 job，第二頁各一則。
	b.pages[0] = buyer.Page{Items: []buyer.Item{{ID: 301}}, Next: &buyer.Cursor{Time: createdAt, ID: 301}}
	b.pages[301] = buyer.Page{Items: []buyer.Item{{ID: 302}}}
	q := &recordingJobs{}
	w.Jobs = q
	if err := w.postPublished(context.Background(), PostPublishedArgs{eventFields: fields(eventID, occurredAt), PostID: postID.String(), StoreID: 91}); err != nil {
		t.Fatal(err)
	}
	var next *PostPublishedArgs
	for _, a := range q.args {
		if n, ok := a.(PostPublishedArgs); ok {
			next = &n
		}
	}
	if next == nil || next.After == nil || next.After.ID != 301 || len(q.published()) != 1 {
		t.Fatalf("first page must publish one and enqueue the next page: %+v published=%d", next, len(q.published()))
	}
	if err := w.postPublished(context.Background(), *next); err != nil {
		t.Fatal(err)
	}
	if len(q.published()) != 2 {
		t.Fatalf("second page must publish the second follower")
	}
}

func TestCommentCreatedWire(t *testing.T) {
	w, _, _, _ := newFixture()
	got := run(t, w, OnCommentCreated, social.CommentCreated{
		EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91,
		Actor: social.Actor{Kind: social.ActorBuyer, ID: 301}, Preview: "留言預覽",
	})
	if len(got) != 1 {
		t.Fatalf("messages=%d", len(got))
	}
	check(t, got[0], "comment.created", with(baseWire(eventID.String(), "COMMENT_CREATED"), map[string]any{
		"commentId": commentID.String(), "postId": postID.String(), "commenterId": float64(301), "commenterPrincipalId": float64(301),
		"commenterName": "小明", "postAuthorId": float64(7001), "postAuthorStoreId": float64(91), "postAuthorType": "seller",
		"contentPreview": "留言預覽",
		"comment": map[string]any{
			"commentId": commentID.String(), "postId": postID.String(), "userId": "301", "userName": "小明", "avatarKey": "avatars/301",
			"role": float64(1), "content": "留言全文", "parentId": "", "replyToId": "", "replyToName": "",
			"likeCount": float64(3), "replyCount": float64(1), "createdAt": "2026-10-05T01:00:00Z", "updatedAt": "2026-10-05T01:00:00Z",
		},
	}))
}

func TestCommentRepliedWire(t *testing.T) {
	w, _, _, _ := newFixture()
	w.Comments = &fakeComments{c: social.Comment{
		ID: commentID, PostID: postID, ParentID: &parentID, ReplyToID: &replyToID, Actor: social.Actor{Kind: social.ActorStore, ID: 91},
		Body: "店家回覆", CreatedAt: createdAt, UpdatedAt: createdAt,
	}}
	got := run(t, w, OnCommentReplied, social.CommentReplied{
		EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, ParentID: parentID, ReplyToID: replyToID,
		Actor: social.Actor{Kind: social.ActorStore, ID: 91}, ReplyToActor: social.Actor{Kind: social.ActorBuyer, ID: 301}, Preview: "回覆預覽",
	})
	if len(got) != 1 {
		t.Fatalf("messages=%d", len(got))
	}
	check(t, got[0], "comment.replied", with(baseWire(eventID.String(), "COMMENT_REPLIED"), map[string]any{
		"replyToUserId": float64(301), "replyToUserStoreId": float64(301), "replyToUserType": "buyer",
		"replierId": float64(91), "replierPrincipalId": float64(7001), "replierName": "好店",
		"postId": postID.String(), "commentId": commentID.String(), "contentPreview": "回覆預覽", "parentCommentId": parentID.String(),
		"comment": map[string]any{
			"commentId": commentID.String(), "postId": postID.String(), "userId": "91", "userName": "好店", "avatarKey": "logos/91",
			"role": float64(2), "content": "店家回覆", "parentId": parentID.String(), "replyToId": replyToID.String(), "replyToName": "小明",
			"likeCount": float64(0), "replyCount": float64(0), "createdAt": "2026-10-05T01:00:00Z", "updatedAt": "2026-10-05T01:00:00Z",
		},
	}))
}

func TestPostLikedWire(t *testing.T) {
	w, _, _, _ := newFixture()
	got := run(t, w, OnPostLiked, social.PostLiked{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91, BuyerID: 301})
	if len(got) != 1 {
		t.Fatalf("messages=%d", len(got))
	}
	check(t, got[0], "post.liked", with(baseWire(eventID.String(), "POST_LIKED"), map[string]any{
		"seller_id": float64(7001), "store_id": float64(91), "likerName": "小明", "post_id": postID.String(),
	}))
}

func TestCommentLikedWireIsActionLikeWithTheCurrentCount(t *testing.T) {
	w, _, _, _ := newFixture()
	got := run(t, w, OnCommentLiked, social.CommentLiked{
		EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91,
		Actor: social.Actor{Kind: social.ActorStore, ID: 91}, Author: social.Actor{Kind: social.ActorBuyer, ID: 301},
	})
	if len(got) != 1 {
		t.Fatalf("messages=%d", len(got))
	}
	check(t, got[0], "comment.like_changed", with(baseWire(eventID.String(), "COMMENT_LIKE_CHANGED"), map[string]any{
		"commentId": commentID.String(), "postId": postID.String(), "likeCount": float64(3), "action": "LIKE",
		"userId": float64(91), "likerKind": float64(2), "commentAuthorId": float64(301), "commentAuthorStoreId": float64(301),
		"likerPrincipalId": float64(7001), "likerName": "好店", "commentAuthorType": "buyer",
	}))
}

func measureEvent(kind seller.MeasureKind, contents ...seller.ContentRef) seller.MeasureCreated {
	return seller.MeasureCreated{EventID: eventID, OccurredAt: occurredAt, MeasureID: measureID, StoreID: 91, AccountID: 7001, Kind: kind, Contents: contents}
}

func TestContentTakedownSendsOneMessagePerPost(t *testing.T) {
	w, _, s, _ := newFixture()
	s.measure = seller.Measure{StatementToSeller: "違反社群規範"}
	p1, p2 := uuid.MustParse("01990000-0000-7000-8000-0000000000d1"), uuid.MustParse("01990000-0000-7000-8000-0000000000d2")
	got := run(t, w, OnMeasureCreated, measureEvent(seller.MeasureContentTakedown,
		seller.ContentRef{Kind: contentkind.Post, ID: p1}, seller.ContentRef{Kind: contentkind.Comment, ID: commentID}, seller.ContentRef{Kind: contentkind.Post, ID: p2}))
	if len(got) != 2 || got[0].EventID == got[1].EventID {
		t.Fatalf("one measure over two posts must send two distinct messages (comments are not posts): %d", len(got))
	}
	for i, p := range []uuid.UUID{p1, p2} {
		check(t, got[i], "post.hidden_by_platform", with(baseWire(got[i].EventID, "POST_HIDDEN_BY_PLATFORM"), map[string]any{
			"seller_id": float64(7001), "store_id": float64(91), "post_id": p.String(), "reason": "違反社群規範",
		}))
	}
}

func TestOtherMeasureKindsSendNothing(t *testing.T) {
	for _, kind := range seller.MeasureKindValues() {
		if kind == seller.MeasureContentTakedown {
			continue
		}
		q := &recordingJobs{}
		if err := OnMeasureCreated(q)(context.Background(), measureEvent(kind, seller.ContentRef{Kind: contentkind.Post, ID: postID})); err != nil {
			t.Fatal(err)
		}
		if len(q.args) != 0 {
			t.Errorf("%s: relay jobs=%d, want none", kind, len(q.args))
		}
	}
	q := &recordingJobs{}
	if err := OnMeasureCreated(q)(context.Background(), measureEvent(seller.MeasureContentTakedown, seller.ContentRef{Kind: contentkind.Comment, ID: commentID})); err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 0 {
		t.Errorf("a takedown with no post has nothing to send")
	}
}

// 每個互動通知各自在三種封鎖（買家雙向、買家封鎖店、店封鎖買家）下都不送；沒有封鎖時送。
func TestBlockedPartiesGetNoInteractionNotification(t *testing.T) {
	type scenario struct {
		name string
		send func(t *testing.T, w *Worker) int
	}
	buyerActor, storeActor := social.Actor{Kind: social.ActorBuyer, ID: 301}, social.Actor{Kind: social.ActorStore, ID: 91}
	scenarios := []scenario{
		{"comment.created（買家留言給店）", func(t *testing.T, w *Worker) int {
			return len(run(t, w, OnCommentCreated, social.CommentCreated{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: buyerActor}))
		}},
		{"comment.replied（店回覆買家）", func(t *testing.T, w *Worker) int {
			return len(run(t, w, OnCommentReplied, social.CommentReplied{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, ParentID: parentID, ReplyToID: replyToID, Actor: storeActor, ReplyToActor: buyerActor}))
		}},
		{"post.liked（買家讚店的貼文）", func(t *testing.T, w *Worker) int {
			return len(run(t, w, OnPostLiked, social.PostLiked{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91, BuyerID: 301}))
		}},
		{"comment.like_changed（店讚買家的留言）", func(t *testing.T, w *Worker) int {
			return len(run(t, w, OnCommentLiked, social.CommentLiked{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: storeActor, Author: buyerActor}))
		}},
		{"comment.like_changed（買家讚買家的留言）", func(t *testing.T, w *Worker) int {
			return len(run(t, w, OnCommentLiked, social.CommentLiked{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: social.Actor{Kind: social.ActorBuyer, ID: 302}, Author: buyerActor}))
		}},
	}
	blocks := map[string]func(*fakeBuyers, *fakeStores){
		"買家之間":  func(b *fakeBuyers, _ *fakeStores) { b.blockedBetway = true },
		"買家封鎖店": func(b *fakeBuyers, _ *fakeStores) { b.blockingStore = true },
		"店封鎖買家": func(_ *fakeBuyers, s *fakeStores) { s.storeBlocks = true },
	}
	for _, sc := range scenarios {
		w, _, _, _ := newFixture()
		if n := sc.send(t, w); n != 1 {
			t.Errorf("%s：沒有封鎖時應送 1 則，送了 %d", sc.name, n)
		}
		for name, block := range blocks {
			w, b, s, _ := newFixture()
			block(b, s)
			relevant := map[string]bool{"買家封鎖店": true, "店封鎖買家": true}
			if sc.name == "comment.like_changed（買家讚買家的留言）" {
				relevant = map[string]bool{"買家之間": true}
			}
			want := 1
			if relevant[name] {
				want = 0
			}
			if n := sc.send(t, w); n != want {
				t.Errorf("%s 在「%s」封鎖下送了 %d 則，預期 %d", sc.name, name, n, want)
			}
		}
	}
}

// 觸發點是發布，不是建立草稿：relay 只訂閱 PostPublished，不訂閱貼文建立、修改、排程等事件。
func TestHandlersAreBoundToTheExpectedEvents(t *testing.T) {
	q := &recordingJobs{}
	want := map[string]reflect.Type{
		"OnPostPublished":  reflect.TypeFor[social.PostPublished](),
		"OnCommentCreated": reflect.TypeFor[social.CommentCreated](),
		"OnCommentReplied": reflect.TypeFor[social.CommentReplied](),
		"OnPostLiked":      reflect.TypeFor[social.PostLiked](),
		"OnCommentLiked":   reflect.TypeFor[social.CommentLiked](),
		"OnMeasureCreated": reflect.TypeFor[seller.MeasureCreated](),
	}
	got := map[string]reflect.Type{
		"OnPostPublished":  reflect.TypeOf(OnPostPublished(q)).In(1),
		"OnCommentCreated": reflect.TypeOf(OnCommentCreated(q)).In(1),
		"OnCommentReplied": reflect.TypeOf(OnCommentReplied(q)).In(1),
		"OnPostLiked":      reflect.TypeOf(OnPostLiked(q)).In(1),
		"OnCommentLiked":   reflect.TypeOf(OnCommentLiked(q)).In(1),
		"OnMeasureCreated": reflect.TypeOf(OnMeasureCreated(q)).In(1),
	}
	for name, typ := range want {
		if got[name] != typ {
			t.Errorf("%s 綁在 %v，預期 %v", name, got[name], typ)
		}
	}
	for name, typ := range got {
		if typ == reflect.TypeFor[social.PostCreated]() || typ == reflect.TypeFor[social.PostScheduled]() {
			t.Errorf("%s 綁在草稿相關的事件 %v", name, typ)
		}
	}
}

func TestDerivedIDIsStableDistinctAndAUUID(t *testing.T) {
	a, b := derivedID("e1", "x"), derivedID("e1", "y")
	if a == b || a != derivedID("e1", "x") {
		t.Fatalf("derivedID must be a pure function of (event, key): %s %s", a, b)
	}
	if _, err := uuid.Parse(a); err != nil {
		t.Fatal(err)
	}
}

func TestSelfLikeOfOwnPostSendsNothing(t *testing.T) {
	w, _, _, _ := newFixture()
	ev := social.PostLiked{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91, BuyerID: 303}
	if got := run(t, w, OnPostLiked, ev); len(got) != 0 {
		t.Fatalf("a store owner liking their own post sent %d messages", len(got))
	}
	ev.BuyerID = 301
	if got := run(t, w, OnPostLiked, ev); len(got) != 1 {
		t.Fatalf("another buyer's like sent %d messages", len(got))
	}
}

func TestPostCreatedSkipsFollowersBlockedWithTheStore(t *testing.T) {
	for name, set := range map[string]func(*fakeBuyers, *fakeStores){
		"買家封鎖店": func(b *fakeBuyers, _ *fakeStores) { b.blockingStore = true },
		"店封鎖買家": func(_ *fakeBuyers, s *fakeStores) { s.storeBlocks = true },
	} {
		w, b, s, _ := newFixture()
		b.pages[0] = buyer.Page{Items: []buyer.Item{{ID: 301}}}
		set(b, s)
		got := run(t, w, OnPostPublished, social.PostPublished{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91})
		if len(got) != 0 {
			t.Errorf("%s：封鎖中的追蹤者仍收到 %d 則", name, len(got))
		}
	}
}

func TestNothingIsSentForContentThatIsNoLongerVisible(t *testing.T) {
	buyerActor := social.Actor{Kind: social.ActorBuyer, ID: 301}
	for _, state := range []struct {
		name string
		set  func(*fakeComments)
	}{
		{"已刪除", func(c *fakeComments) { c.gone = true }},
		{"不可讀", func(c *fakeComments) { c.notShown = true }},
	} {
		sends := map[string]func(*testing.T, *Worker) int{
			"comment.created": func(t *testing.T, w *Worker) int {
				return len(run(t, w, OnCommentCreated, social.CommentCreated{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: buyerActor}))
			},
			"comment.replied": func(t *testing.T, w *Worker) int {
				return len(run(t, w, OnCommentReplied, social.CommentReplied{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, ParentID: parentID, ReplyToID: replyToID, Actor: social.Actor{Kind: social.ActorStore, ID: 91}, ReplyToActor: buyerActor}))
			},
			"comment.like_changed": func(t *testing.T, w *Worker) int {
				return len(run(t, w, OnCommentLiked, social.CommentLiked{EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: social.Actor{Kind: social.ActorStore, ID: 91}, Author: buyerActor}))
			},
			"post.created": func(t *testing.T, w *Worker) int {
				w.Buyers.(*fakeBuyers).pages[0] = buyer.Page{Items: []buyer.Item{{ID: 301}}}
				return len(run(t, w, OnPostPublished, social.PostPublished{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91}))
			},
		}
		for name, send := range sends {
			w, _, _, _, fc := newFixtureWithComments()
			if n := send(t, w); n != 1 {
				t.Fatalf("%s：可見時應送 1 則，送了 %d", name, n)
			}
			state.set(fc)
			if n := send(t, w); n != 0 {
				t.Errorf("%s（%s）：仍送了 %d 則", name, state.name, n)
			}
		}
	}
}

func TestNextFollowerPageIsEnqueuedUniquelyByArgs(t *testing.T) {
	w, b, _, q := newFixture()
	b.pages[0] = buyer.Page{Items: []buyer.Item{{ID: 301}}, Next: &buyer.Cursor{Time: createdAt, ID: 301}}
	if err := w.postPublished(context.Background(), PostPublishedArgs{eventFields: fields(eventID, occurredAt), PostID: postID.String(), StoreID: 91}); err != nil {
		t.Fatal(err)
	}
	for i, a := range q.args {
		if _, ok := a.(PostPublishedArgs); ok {
			if q.opts[i] == nil || !q.opts[i].UniqueOpts.ByArgs {
				t.Fatalf("next page opts=%+v, want UniqueOpts.ByArgs", q.opts[i])
			}
			return
		}
	}
	t.Fatal("no next page enqueued")
}

func isCancel(err error) bool {
	var c *rivertype.JobCancelError
	return errors.As(err, &c)
}

func TestUnknownActorKindCancelsTheJob(t *testing.T) {
	w, _, _, _ := newFixture()
	err := w.commentCreated(context.Background(), CommentCreatedArgs{eventFields: fields(eventID, occurredAt), CommentID: commentID.String(), PostID: postID.String(), StoreID: 91, ActorKind: "ROBOT", ActorID: 1})
	if !isCancel(err) {
		t.Fatalf("err=%v, want a cancel", err)
	}
}

func TestTakedownOfAMissingMeasureCancelsAndOtherErrorsRetry(t *testing.T) {
	w, _, s, _ := newFixture()
	args := ContentTakedownArgs{eventFields: fields(eventID, occurredAt), MeasureID: measureID.String(), StoreID: 91, AccountID: 7001, PostIDs: []string{postID.String()}}
	s.measureErr = apperr.NotFound("no measure")
	if err := w.contentTakedown(context.Background(), args); !isCancel(err) {
		t.Fatalf("not found err=%v, want a cancel", err)
	}
	s.measureErr = errors.New("db down")
	if err := w.contentTakedown(context.Background(), args); err == nil || isCancel(err) {
		t.Fatalf("transient err=%v, want a retryable error", err)
	}
}
