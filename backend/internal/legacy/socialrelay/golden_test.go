package socialrelay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

// goldenKeys 讀通知模板契約的 golden（tests/contract/golden/event_payload_keys.json）：模板讀的 key 必須在
// emitted_keys 裡，這裡反過來確認 relay 真的送出這些 key。
func goldenKeys(t *testing.T, eventType, routingKey string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "contract", "golden", "event_payload_keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]struct {
		RoutingKey  string   `json:"routing_key"`
		EmittedKeys []string `json:"emitted_keys"`
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	entry, ok := golden[eventType]
	if !ok || entry.RoutingKey != routingKey || len(entry.EmittedKeys) == 0 {
		t.Fatalf("golden %s = %+v", eventType, entry)
	}
	return entry.EmittedKeys
}

// 每個由 relay 送出的 golden event_type 都對著 relay 真正發出的 payload 比對：golden 宣稱的 key 必須都在。
func TestGoldenEmittedKeysAreSubsetOfRelayedPayload(t *testing.T) {
	storeActor := social.Actor{Kind: social.ActorStore, ID: 91}
	buyerActor := social.Actor{Kind: social.ActorBuyer, ID: 301}
	repliedBy := func(w *Worker, fc *fakeComments, _ *fakeBuyers, _ *fakeStores) []map[string]any {
		fc.c = social.Comment{
			ID: commentID, PostID: postID, ParentID: &parentID, ReplyToID: &replyToID, Actor: storeActor,
			Body: "店家回覆", CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		return payloads(t, run(t, w, OnCommentReplied, social.CommentReplied{
			EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, ParentID: parentID, ReplyToID: replyToID,
			Actor: storeActor, ReplyToActor: buyerActor, Preview: "回覆預覽",
		}))
	}
	created := func(w *Worker, _ *fakeComments, b *fakeBuyers, s *fakeStores) []map[string]any {
		return payloads(t, run(t, w, OnCommentCreated, social.CommentCreated{
			EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: buyerActor, Preview: "留言預覽",
		}))
	}
	cases := []struct {
		golden, key string
		send        func(*Worker, *fakeComments, *fakeBuyers, *fakeStores) []map[string]any
	}{
		{"comment.created", "comment.created", created},
		{"comment.created.buyer", "comment.created", created},
		{"comment.like_changed", "comment.like_changed", func(w *Worker, _ *fakeComments, b *fakeBuyers, s *fakeStores) []map[string]any {
			return payloads(t, run(t, w, OnCommentLiked, social.CommentLiked{
				EventID: eventID, OccurredAt: occurredAt, CommentID: commentID, PostID: postID, StoreID: 91, Actor: storeActor, Author: buyerActor,
			}))
		}},
		{"comment.replied.buyer", "comment.replied", repliedBy},
		{"comment.replied.seller", "comment.replied", repliedBy},
		{"post.created", "post.created", func(w *Worker, _ *fakeComments, b *fakeBuyers, s *fakeStores) []map[string]any {
			b.pages[0] = buyer.Page{Items: []buyer.Item{{ID: 301}}}
			return payloads(t, run(t, w, OnPostPublished, social.PostPublished{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91, Preview: "預覽"}))
		}},
		{"post.hidden_by_platform.seller", "post.hidden_by_platform", func(w *Worker, _ *fakeComments, b *fakeBuyers, s *fakeStores) []map[string]any {
			s.measure = seller.Measure{StatementToSeller: "違反社群規範"}
			return payloads(t, run(t, w, OnMeasureCreated, measureEvent(seller.MeasureContentTakedown, seller.ContentRef{Kind: contentkind.Post, ID: postID})))
		}},
		{"post.liked", "post.liked", func(w *Worker, _ *fakeComments, b *fakeBuyers, s *fakeStores) []map[string]any {
			return payloads(t, run(t, w, OnPostLiked, social.PostLiked{EventID: eventID, OccurredAt: occurredAt, PostID: postID, StoreID: 91, BuyerID: 301}))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			emitted := goldenKeys(t, tc.golden, tc.key)
			w, b, s, _, fc := newFixtureWithComments()
			got := tc.send(w, fc, b, s)
			if len(got) != 1 {
				t.Fatalf("messages=%d", len(got))
			}
			for _, key := range emitted {
				if _, ok := got[0][key]; !ok {
					t.Errorf("golden %s 宣稱送出 %q，payload 沒有：%v", tc.golden, key, got[0])
				}
			}
		})
	}
}

func payloads(t *testing.T, published []rabbitmq.PublishArgs) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, len(published))
	for _, a := range published {
		out = append(out, body(t, a))
	}
	return out
}
