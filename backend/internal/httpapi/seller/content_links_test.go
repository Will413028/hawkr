package seller

import (
	"context"
	"testing"
	"uuid"

	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
)

type fakeLinks struct {
	posts   map[uuid.UUID]bool
	threads map[social.ForumThread]bool
	calls   *[][]uuid.UUID
}

func (f fakeLinks) AvailableLinks(_ context.Context, postIDs []uuid.UUID, threads []social.ForumThread) (social.LinkAvailability, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, postIDs)
	}
	out := social.LinkAvailability{Posts: map[uuid.UUID]bool{}, Threads: map[social.ForumThread]bool{}}
	for _, id := range postIDs {
		out.Posts[id] = f.posts[id]
	}
	for _, t := range threads {
		out.Threads[t] = f.threads[t]
	}
	return out, nil
}

func linkedContent(kind model.ContentKind, link model.ContentLink) model.MeasureContent {
	return model.MeasureContent{
		ContentRef: model.ContentRef{Kind: kind, ID: uuid.NewV7()}, State: model.SnapshotCaptured, Excerpt: "x",
		Snapshot: &model.ContentSnapshot{AuthoredAt: snapshotAuthoredAt, Link: link},
	}
}

// 已刪除的目標 link_available 為 false，還在的為 true；整個回應只問 social 一次，重複的目標只問一次。
func TestLinkAvailabilityFollowsTheTargetAndAsksOnce(t *testing.T) {
	livePost, deadPost, thread := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	var calls [][]uuid.UUID
	src := fakeLinks{posts: map[uuid.UUID]bool{livePost: true}, threads: map[social.ForumThread]bool{{StoreID: 12, ThreadID: thread}: true}, calls: &calls}
	m1 := riskMeasureFixture()
	m1.Kind = model.MeasureContentTakedown
	m1.Contents = []model.MeasureContent{
		linkedContent(model.ContentPost, model.ContentLink{PostID: livePost}),
		linkedContent(model.ContentComment, model.ContentLink{PostID: deadPost}),
		linkedContent(model.ContentComment, model.ContentLink{PostID: livePost}),
		linkedContent(model.ContentForumEntry, model.ContentLink{StoreID: 12, ThreadID: thread}),
		linkedContent(model.ContentForumEntry, model.ContentLink{StoreID: 12, ThreadID: uuid.NewV7()}),
		{ContentRef: model.ContentRef{Kind: model.ContentPost, ID: uuid.NewV7()}, State: model.SnapshotCleared},
	}
	m2 := m1
	got, err := adminMeasures(context.Background(), src, []model.Measure{m1, m2})
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, true, true, false, false}
	for i := range got {
		for j, c := range got[i].GetContents() {
			if c.GetLinkAvailable() != want[j] {
				t.Errorf("措施 %d 內容 %d：link_available=%v，應為 %v", i, j, c.GetLinkAvailable(), want[j])
			}
		}
	}
	if len(calls) != 1 || len(calls[0]) != 2 {
		t.Fatalf("應只問一次、貼文目標去重成 2 個：%v", calls)
	}
	mine, err := sellerMeasure(context.Background(), src, m1)
	if err != nil || !mine.GetContents()[0].GetLinkAvailable() || mine.GetContents()[1].GetLinkAvailable() {
		t.Errorf("賣家端：%v err=%v", mine.GetContents(), err)
	}
}

// 措施沒有任何連結目標時不問 social；有連結卻沒有查詢來源是錯誤，不是悄悄回「不可用」。
func TestLinkAvailabilitySkipsWhenNoTargetsAndFailsWithoutSource(t *testing.T) {
	var calls [][]uuid.UUID
	plain := riskMeasureFixture()
	if _, err := adminMeasures(context.Background(), fakeLinks{calls: &calls}, []model.Measure{plain}); err != nil || len(calls) != 0 {
		t.Fatalf("沒有目標：calls=%v err=%v", calls, err)
	}
	linked := riskMeasureFixture()
	linked.Contents = []model.MeasureContent{linkedContent(model.ContentPost, model.ContentLink{PostID: uuid.NewV7()})}
	if _, err := adminMeasures(context.Background(), nil, []model.Measure{linked}); err == nil {
		t.Fatal("有連結卻沒有查詢來源應回錯誤")
	}
}

// 聊天訊息與評價回覆的快照沒有連結：不問 social，也不被當成討論串（零值連結不是討論串 0）。
func TestLinklessSnapshotsAreNotTreatedAsForumThreads(t *testing.T) {
	var calls [][]uuid.UUID
	m := riskMeasureFixture()
	m.Kind = model.MeasureContentTakedown
	m.Contents = []model.MeasureContent{
		linkedContent(model.ContentKind(contentkind.ChatMessage), model.ContentLink{}),
		linkedContent(model.ContentKind(contentkind.ReviewReply), model.ContentLink{}),
	}
	got, err := adminMeasures(context.Background(), fakeLinks{calls: &calls}, []model.Measure{m})
	if err != nil || len(calls) != 0 {
		t.Fatalf("沒有連結不該問 social（也不該因為沒有來源而失敗）：calls=%v err=%v", calls, err)
	}
	for i, c := range got[0].GetContents() {
		if c.GetLinkAvailable() {
			t.Errorf("內容 %d：沒有連結，link_available 應為 false", i)
		}
	}
	if _, err := adminMeasures(context.Background(), nil, []model.Measure{m}); err != nil {
		t.Fatalf("沒有連結時沒有來源也不是錯誤：%v", err)
	}
}
