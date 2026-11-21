//go:build integration

package seller_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func sameSnapshot(a, b *ContentSnapshot) bool {
	return a.Body == b.Body && a.AuthoredAt.Equal(b.AuthoredAt) && a.ImageCount == b.ImageCount && a.Link == b.Link
}

// 措施存入處分依據的內容快照，讀回時本文全文、發文時間、圖片張數與連結目標都在。
// mutation：InsertMeasureContents 少寫 link_thread_id 或 snapshot_body → CHECK 擋下或讀回的欄位不符。
func TestContentTakedownStoresAndReadsBackTheSnapshot(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	postID, threadID := uuid.NewV7(), uuid.NewV7()
	post := TakedownContent{ContentRef: ContentRef{Kind: ContentPost, ID: uuid.NewV7()}, Snapshot: &ContentSnapshot{
		Body: "貼文全文", AuthoredAt: snapshotAuthored, ImageCount: 1, Link: ContentLink{PostID: postID}}}
	forum := TakedownContent{ContentRef: ContentRef{Kind: ContentForumEntry, ID: uuid.NewV7()}, Snapshot: &ContentSnapshot{
		Body: "", AuthoredAt: snapshotAuthored, ImageCount: 0, Link: ContentLink{StoreID: s.StoreID, ThreadID: threadID}}}
	comment := TakedownContent{ContentRef: ContentRef{Kind: ContentComment, ID: uuid.NewV7()}, Snapshot: &ContentSnapshot{
		Body: "留言", AuthoredAt: snapshotAuthored, ImageCount: 0, Link: ContentLink{PostID: postID}}}
	m := f.takedownSnapshots(t, s.StoreID, []TakedownContent{post, forum, comment})

	for _, got := range [][]MeasureContent{m.Contents, f.measureOf(t, m.ID).Contents} {
		by := map[ContentKind]MeasureContent{}
		for _, c := range got {
			by[c.Kind] = c
		}
		p, fe, co := by[ContentPost], by[ContentForumEntry], by[ContentComment]
		if p.State != SnapshotCaptured || p.Snapshot == nil || !sameSnapshot(p.Snapshot, post.Snapshot) {
			t.Errorf("貼文快照 %+v", p)
		}
		if fe.State != SnapshotCaptured || fe.Snapshot == nil || !sameSnapshot(fe.Snapshot, forum.Snapshot) {
			t.Errorf("討論區留言快照（空本文也是有效快照）%+v", fe)
		}
		if co.State != SnapshotCaptured || co.Snapshot == nil || !sameSnapshot(co.Snapshot, comment.Snapshot) {
			t.Errorf("留言快照 %+v", co)
		}
	}
}

// 內容下架沒有結束時間，快照只從撤銷起算：撤銷後 184 天（含）清空，183 天與 184 天前一秒不清；未撤銷的一直保留。
// mutation：保留期常數改成 183 天 → 183 天的斷言失敗；WHERE 去掉 status = 'REVOKED' → 未撤銷的措施被清。
func TestPurgeContentSnapshotsClearsOnlyRevokedAfterRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	revoked := f.takedown(t, s.StoreID, takedownContents(2))
	kept := f.takedown(t, s.StoreID, takedownContents(1))
	revokedAt := f.now
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: revoked.ID, ExpectedVersion: revoked.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)

	for _, at := range []time.Duration{183 * 24 * time.Hour, 184*24*time.Hour - time.Second} {
		f.now = revokedAt.Add(at)
		if n, err := f.module.PurgeContentSnapshots(ctx); err != nil || n != 0 {
			t.Fatalf("撤銷後 %s 不清：n=%d err=%v", at, n, err)
		}
	}
	for _, c := range f.measureOf(t, revoked.ID).Contents {
		if c.State != SnapshotCaptured || c.Snapshot == nil {
			t.Fatalf("未滿 184 天快照還在：%+v", c)
		}
	}
	f.now = revokedAt.Add(184 * 24 * time.Hour)
	if n, err := f.module.PurgeContentSnapshots(ctx); err != nil || n != 2 {
		t.Fatalf("撤銷後 184 天清空兩列：n=%d err=%v", n, err)
	}
	for _, c := range f.measureOf(t, revoked.ID).Contents {
		if c.State != SnapshotCleared || c.Snapshot != nil {
			t.Errorf("清空後只剩已清空狀態：%+v", c)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measure_contents WHERE measure_id = $1 AND snapshot_body IS NULL AND link_post_id IS NULL AND link_store_id IS NULL AND link_thread_id IS NULL`, revoked.ID); n != 2 {
		t.Fatalf("快照欄位與連結都清掉：%d", n)
	}
	f.now = revokedAt.Add(2000 * 24 * time.Hour)
	if n, err := f.module.PurgeContentSnapshots(ctx); err != nil || n != 0 {
		t.Fatalf("重跑冪等：n=%d err=%v", n, err)
	}
	if got := f.measureOf(t, kept.ID).Contents; len(got) != 1 || got[0].State != SnapshotCaptured || got[0].Snapshot == nil {
		t.Fatalf("未撤銷的措施一直保留：%+v", got)
	}
}

// 店家退場清個資（第二段）時清空這家店全部措施的內容快照，不論措施是否撤銷、是否滿 184 天；別家店的不動。
// mutation：purgeBatch 拿掉 ClearContentSnapshotsOfStores → 退場後快照仍在；query 去掉 store_id 條件 → 別家店的也被清。
func TestStoreExitPurgeClearsContentSnapshots(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.exposable(t)
	_, other := f.exposable(t)
	mine := f.takedown(t, s.StoreID, takedownContents(2))
	theirs := f.takedown(t, other.StoreID, takedownContents(1))
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now

	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	_, err = f.module.PurgePII(ctx)
	must(t, err)
	if got := f.measureOf(t, mine.ID).Contents; got[0].State != SnapshotCaptured {
		t.Fatalf("第一段（退場）不清快照：%+v", got)
	}
	f.now = closedAt.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("第二段清個資：n=%d err=%v", n, err)
	}
	for _, c := range f.measureOf(t, mine.ID).Contents {
		if c.State != SnapshotCleared || c.Snapshot != nil {
			t.Errorf("退場後快照清空：%+v", c)
		}
	}
	if got := f.measureOf(t, theirs.ID).Contents; len(got) != 1 || got[0].State != SnapshotCaptured {
		t.Fatalf("別家店的快照不動：%+v", got)
	}
}

// 資料庫的保底：快照欄位與狀態必須一致、連結依種類（貼文與留言連貼文、討論區留言連店家加討論串）。
func TestContentSnapshotSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	m := f.takedown(t, s.StoreID, []ContentRef{{Kind: ContentPost, ID: uuid.NewV7()}})
	for name, c := range map[string]struct{ constraint, sql string }{
		"已存快照但本文為空":  {"measure_contents_snapshot_fields_check", "UPDATE seller.measure_contents SET snapshot_body = NULL WHERE measure_id = $1"},
		"已清空但本文還在":   {"measure_contents_snapshot_fields_check", "UPDATE seller.measure_contents SET snapshot_state = 'CLEARED' WHERE measure_id = $1"},
		"貼文同時有討論串連結": {"measure_contents_link_check", "UPDATE seller.measure_contents SET link_store_id = 1, link_thread_id = gen_random_uuid() WHERE measure_id = $1"},
		"聊天訊息帶貼文連結":  {"measure_contents_link_check", "UPDATE seller.measure_contents SET content_kind = 'CHAT_MESSAGE' WHERE measure_id = $1"},
		"評價回覆帶貼文連結":  {"measure_contents_link_check", "UPDATE seller.measure_contents SET content_kind = 'REVIEW_REPLY' WHERE measure_id = $1"},
		"圖片張數為負":     {"measure_contents_snapshot_image_count_check", "UPDATE seller.measure_contents SET snapshot_image_count = -1 WHERE measure_id = $1"},
		"狀態不合法":      {"measure_contents_snapshot_state_check", "UPDATE seller.measure_contents SET snapshot_state = 'OTHER', snapshot_body = NULL, snapshot_authored_at = NULL, snapshot_image_count = NULL, link_post_id = NULL, link_store_id = NULL, link_thread_id = NULL WHERE measure_id = $1"},
	} {
		f.wantViolates(t, name, c.constraint, c.sql, m.ID)
	}
}

// 清單讀取只載入摘要：資料庫截出前 ExcerptRunes 個字元與「是否更長」（以字元計，不是位元組），全文不進清單；單筆讀才帶全文。
// 恰好 ExcerptRunes 個字元不加「…」，多一個才加；四位元組字元也以字元計。
// mutation：MeasureContents 的 left 改成 substring(… for 119) → 摘要長度斷言失敗；把 char_length 換成 octet_length → 字元數剛好 120 的多位元組本文被誤標截斷。
func TestListReadsCarryOnlyTheExcerptAndSingleReadCarriesTheFullBody(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	bodies := map[string]string{
		"exact":   strings.Repeat("字", ExcerptRunes),
		"over":    strings.Repeat("字", ExcerptRunes+1),
		"emoji":   strings.Repeat("😀", ExcerptRunes+3),
		"emojiOK": strings.Repeat("😀", ExcerptRunes),
	}
	var contents []TakedownContent
	byID := map[uuid.UUID]string{}
	for name, body := range bodies {
		c := TakedownContent{ContentRef: ContentRef{Kind: ContentPost, ID: uuid.NewV7()}, Snapshot: &ContentSnapshot{
			Body: body, AuthoredAt: snapshotAuthored, Link: ContentLink{PostID: uuid.NewV7()}}}
		contents = append(contents, c)
		byID[c.ID] = name
	}
	m := f.takedownSnapshots(t, s.StoreID, contents)
	want := func(name string) string {
		b := []rune(bodies[name])
		if len(b) > ExcerptRunes {
			return string(b[:ExcerptRunes]) + "…"
		}
		return bodies[name]
	}
	page, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID})
	must(t, err)
	if len(page.Items) != 1 || len(page.Items[0].Contents) != 4 {
		t.Fatalf("清單 %+v", page.Items)
	}
	for _, c := range page.Items[0].Contents {
		if c.Excerpt != want(byID[c.ID]) || c.FullBody || c.Snapshot == nil || c.Snapshot.Body != "" {
			t.Errorf("清單的 %s：excerpt 長度 %d、FullBody=%v、Body 長度 %d", byID[c.ID], len([]rune(c.Excerpt)), c.FullBody, len(c.Snapshot.Body))
		}
	}
	for _, c := range f.measureOf(t, m.ID).Contents {
		if !c.FullBody || c.Snapshot.Body != bodies[byID[c.ID]] || c.Excerpt != want(byID[c.ID]) {
			t.Errorf("單筆讀 %s：FullBody=%v、Body 長度 %d", byID[c.ID], c.FullBody, len([]rune(c.Snapshot.Body)))
		}
	}
}
