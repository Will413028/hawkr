//go:build integration

package moderation

import (
	"context"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 檢舉數只算進行中案件：貼文 A 的進行中案件有兩份檢舉，貼文 B 已結案的案件有一份檢舉（不算），沒被檢舉的貼文不在結果裡；
// 留言的案件不混進貼文的計數。
func TestOpenReportCountsCountsOnlyReportsInOpenCases(t *testing.T) {
	t.Parallel()
	f := setup(t)
	lw := f.listWorld(t)
	second, _ := f.newBuyer(t)
	f.submit(t, second, Target{Kind: TargetPost, ContentID: lw.postOpen.Target.ContentID}, policy.Spam)
	untouched := uuid.New()

	got := mustV[map[uuid.UUID]int64](t)(f.module.OpenReportCounts(context.Background(),
		[]uuid.UUID{lw.postOpen.Target.ContentID, lw.postClosed.Target.ContentID, lw.commentOpen.Target.ContentID, untouched}))

	if n := got[lw.postOpen.Target.ContentID]; n != 2 {
		t.Errorf("進行中案件的檢舉數 = %d，預期 2", n)
	}
	if n, ok := got[lw.postClosed.Target.ContentID]; ok || n != 0 {
		t.Errorf("已結案案件的檢舉不算：got %d（有列 %v）", n, ok)
	}
	if _, ok := got[lw.commentOpen.Target.ContentID]; ok {
		t.Error("留言的案件不算貼文的檢舉數")
	}
	if len(got) != 1 {
		t.Errorf("結果 = %v，預期只有一篇貼文", got)
	}
}

func TestOpenReportCountsOfNoPostsIsEmpty(t *testing.T) {
	t.Parallel()
	f := setup(t)
	got := mustV[map[uuid.UUID]int64](t)(f.module.OpenReportCounts(context.Background(), nil))
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
