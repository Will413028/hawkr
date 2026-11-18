package measure

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// 批次寫入把每個對象放在各陣列的同一個索引：CAPTURED 帶本文、發文時間、張數與連結；UNAVAILABLE 的快照欄位是哨兵（SQL 轉 NULL），連結沒填的一組是零值。
// mutation：陣列填錯索引（例如 States 用倒序）→ 同一個對象的狀態與本文錯位，斷言失敗；UNAVAILABLE 也寫入本文 → Bodies 斷言失敗。
func TestInsertContentsParamsKeepsParallelArraysAligned(t *testing.T) {
	m := ids(1)[0]
	post, thread := ids(3)[1], ids(3)[2]
	at := time.Date(2026, 11, 1, 3, 4, 5, 0, time.FixedZone("x", 8*3600))
	in := []model.TakedownContent{
		{ContentRef: model.ContentRef{Kind: model.ContentComment, ID: ids(4)[3]}, Purged: true},
		{ContentRef: model.ContentRef{Kind: model.ContentForumEntry, ID: ids(5)[4]}, Snapshot: &model.ContentSnapshot{Body: "討論", AuthoredAt: at, ImageCount: 0, Link: model.ContentLink{StoreID: 12, ThreadID: thread}}},
		{ContentRef: model.ContentRef{Kind: model.ContentPost, ID: post}, Snapshot: &model.ContentSnapshot{Body: "貼文", AuthoredAt: at, ImageCount: 2, Link: model.ContentLink{PostID: post}}},
	}
	got := insertContentsParams(m, in)
	want := store.InsertMeasureContentsParams{
		MeasureID:   m,
		Kinds:       []string{"COMMENT", "FORUM_ENTRY", "POST"},
		Ids:         []uuid.UUID{in[0].ID, in[1].ID, in[2].ID},
		States:      []string{"UNAVAILABLE", "CAPTURED", "CAPTURED"},
		Bodies:      []string{"", "討論", "貼文"},
		AuthoredAts: []time.Time{{}, at.UTC(), at.UTC()},
		ImageCounts: []int32{0, 0, 2},
		LinkPostIds: []uuid.UUID{uuid.Nil(), uuid.Nil(), post}, LinkStoreIds: []int64{0, 12, 0}, LinkThreadIds: []uuid.UUID{uuid.Nil(), thread, uuid.Nil()},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

// 清單讀取的列只有摘要前綴與是否更長：contentOf 用同一條規則組摘要，Snapshot.Body 留空、FullBody 為假；非 CAPTURED 沒有摘要與快照。
// mutation：contentOf 忽略 SnapshotTruncated → 超過上限的摘要缺「…」。
func TestContentOfBuildsTheExcerptFromThePrefixAndTheTruncatedFlag(t *testing.T) {
	at, n := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), int32(1)
	prefix := strings.Repeat("字", model.ExcerptRunes)
	row := store.MeasureContentsRow{ContentKind: "POST", ContentID: ids(1)[0], SnapshotState: "CAPTURED", SnapshotExcerpt: prefix, SnapshotTruncated: true,
		SnapshotAuthoredAt: &at, SnapshotImageCount: &n, LinkPostID: &ids(1)[0]}
	c := contentOf(row)
	if c.Excerpt != prefix+"…" || c.FullBody || c.Snapshot == nil || c.Snapshot.Body != "" || c.Snapshot.Link.PostID != ids(1)[0] || c.Snapshot.ImageCount != 1 {
		t.Fatalf("%+v", c)
	}
	row.SnapshotTruncated = false
	if got := contentOf(row).Excerpt; got != prefix {
		t.Errorf("未截斷不加省略號：%q", got)
	}
	if c := contentOf(store.MeasureContentsRow{ContentKind: "POST", ContentID: ids(1)[0], SnapshotState: "CLEARED"}); c.Excerpt != "" || c.Snapshot != nil {
		t.Errorf("已清空：%+v", c)
	}
}
