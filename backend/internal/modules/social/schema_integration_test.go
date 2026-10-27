//go:build integration

package social_test

import (
	"context"
	"regexp"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func stringsOf[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// TestChecksMatchGo：DB 的 CHECK 列出的值與 Go 的列舉一致（儲存的代碼是 UPPER_SNAKE）。
func TestChecksMatchGo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	upperSnake := regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`)
	for _, tt := range []struct {
		table, name string
		values      []string
	}{
		{"posts", "posts_status_check", stringsOf(social.PostStatusValues())},
		{"comments", "comments_actor_kind_check", stringsOf(social.ActorKindValues())},
		{"comment_likes", "comment_likes_actor_kind_check", stringsOf(social.ActorKindValues())},
		{"forum_entries", "forum_entries_actor_kind_check", stringsOf(social.ActorKindValues())},
		{"hides", "hides_reason_category_check", stringsOf(policy.CommunityAssessable())},
		{"sanctions", "sanctions_reason_category_check", stringsOf(policy.CommunityAssessable())},
		{"sanctions", "sanctions_kind_check", stringsOf(social.SanctionKindValues())},
	} {
		for _, v := range tt.values {
			if !upperSnake.MatchString(v) {
				t.Fatalf("%s：%q 不是 UPPER_SNAKE", tt.name, v)
			}
		}
		var def string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = $1::regclass AND conname = $2", "social."+tt.table, tt.name).Scan(&def))
		var actual []string
		for _, m := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(def, -1) {
			actual = append(actual, m[1])
		}
		slices.Sort(actual)
		want := slices.Clone(tt.values)
		slices.Sort(want)
		if !slices.Equal(actual, want) {
			t.Fatalf("%s：DB=%v Go=%v", tt.name, actual, want)
		}
	}
}

// 同篇由 DB 保證：parent_id、reply_to_id 指到別篇的留言時 DB 就拒絕；兩層（父必須是頂層）由 Go 保證；討論區同理（同店）。
func TestSamePostIsEnforcedByTheDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p1 := f.publishedPost(t, storeID, "一")
	p2 := f.publishedPost(t, storeID, "二")
	b := f.newBuyer(t)
	c1 := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p1.ID, Body: "x"}))
	ins := func(post, parent, replyTo any) error {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'STORE', 1, 'x', now(), now())`, uuid.NewV7(), post, parent, replyTo)
		return err
	}
	if err := ins(p2.ID, c1.ID, nil); err == nil {
		t.Fatal("parent 在別篇要被拒絕")
	}
	if err := ins(p2.ID, nil, c1.ID); err == nil {
		t.Fatal("reply_to 在別篇要被拒絕")
	}
	if err := ins(p1.ID, nil, c1.ID); err == nil {
		t.Fatal("reply_to 沒有 parent 要被拒絕")
	}
	if err := ins(p1.ID, c1.ID, c1.ID); err != nil {
		t.Fatalf("同篇可以：%v", err)
	}
	e1 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "x"}))
	other, _ := f.exposableStore(t)
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO social.forum_entries (id, store_id, parent_id, actor_kind, actor_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, 'STORE', 1, 'x', now(), now())`, uuid.NewV7(), other, e1.ID)
	if err == nil {
		t.Fatal("討論區的 parent 在別家店要被拒絕")
	}
}

// 計數的 CHECK (>= 0)：維護出錯時寫入失敗，不靜默漂移成負數。
func TestStatsRejectNegativeCounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "x")
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE social.post_stats SET like_count = -1 WHERE post_id = $1", p.ID); err == nil {
		t.Fatal("負數要被拒絕")
	}
}

// 讀取只看旗標 counted（visibility.Counted）：列出回覆與探測「底下有沒有可見的回覆」的 partial index 的述詞必須是 counted。planner 無法從
// counted 證明 deleted_at IS NULL，述詞若還是後者，索引用不上，熱門貼文頂層留言從 1.7 ms 變 196 ms（Seq Scan comments，step-1a-perf 的 R4 重量）。
func TestReplyIndexesAreOnTheCountedFlag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	for _, name := range []string{"comments_replies", "forum_entries_replies"} {
		var def string
		if err := f.db.Conn(context.Background()).QueryRow(context.Background(),
			"SELECT indexdef FROM pg_indexes WHERE schemaname = 'social' AND indexname = $1", name).Scan(&def); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if !regexp.MustCompile(`WHERE counted$`).MatchString(def) {
			t.Errorf("%s 的述詞要是旗標 counted：%s", name, def)
		}
	}
}
