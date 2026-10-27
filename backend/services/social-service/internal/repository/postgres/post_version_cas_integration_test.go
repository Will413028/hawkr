//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

// 56 §5.4 的樂觀鎖 CAS 與 §6.1.9 的草稿建立，對真實 SQL 各驗一次。
//
// 兩者都只在資料庫裡才成立：CAS 的「哨兵值停用比對／不符即 rows = 0」是述詞的性質，
// 草稿的正確性則取決於 INSERT 有沒有真的把 4401 寫進去——欄位的 DB DEFAULT 是 4402
// （已發布），所以「漏傳」與「傳錯」的後果一樣是貼文當場進入買家 feed。
func TestPostVersionCASAndDraftCreate(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewPostRepository(pool)

	const sellerID int64 = 2685914682

	t.Run("delegated create is a draft", func(t *testing.T) {
		post, err := repo.Create(ctx, 9000000000000001, sellerID, "代行草稿", "", nil, nil,
			domain.PostPublishStatusDraft, nil)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusDraft, post.PublishStatus,
			"a delegated create must land as a draft — the column default is PUBLISHED, so getting this wrong puts the post straight into the buyer feed")
		require.EqualValues(t, 0, post.Version)
		require.Nil(t, post.PublishedAt,
			"a draft has not been published, so it must carry no publish time — the schedule worker or the publish endpoint writes it")
	})

	t.Run("seller create is published", func(t *testing.T) {
		post, err := repo.Create(ctx, 9000000000000002, sellerID, "賣家自己發的", "", nil, nil,
			domain.PostPublishStatusPublished, nil)
		require.NoError(t, err)
		require.EqualValues(t, domain.PostPublishStatusPublished, post.PublishStatus)
		// The migration backfilled published_at for rows that already existed; it
		// cannot reach rows created afterwards. Without the INSERT writing this
		// column, every post published from here on would repeat exactly the state
		// that backfill was there to remove.
		require.NotNil(t, post.PublishedAt,
			"a post created as published must carry its publish time")
	})

	t.Run("drafts stay out of the public list", func(t *testing.T) {
		// The public feed must show only the published one of the two rows above.
		posts, err := repo.List(ctx, sellerID, 50, 0)
		require.NoError(t, err)
		require.Len(t, posts, 1, "the draft must not appear in the buyer-facing list")
		require.EqualValues(t, 9000000000000002, posts[0].ID)

		total, err := repo.Count(ctx, sellerID)
		require.NoError(t, err)
		require.EqualValues(t, 1, total, "Count must use the same predicate as List, or the pager reports a page that cannot be filled")

		// The seller's own list still shows both — that is the point of the split.
		mine, err := repo.ListMy(ctx, sellerID, "", 50, 0)
		require.NoError(t, err)
		require.Len(t, mine, 2, "the seller (and the delegated operator) must see their own draft")
	})

	t.Run("matching version writes and bumps", func(t *testing.T) {
		const postID int64 = 9000000000000001
		before, err := repo.GetByID(ctx, postID)
		require.NoError(t, err)

		expected := before.Version
		require.NoError(t, repo.Update(ctx, postID, "改過的內容", "", nil, nil, &expected, nil))

		after, err := repo.GetByID(ctx, postID)
		require.NoError(t, err)
		require.NotNil(t, after.Content)
		require.Equal(t, "改過的內容", *after.Content)
		require.EqualValues(t, expected+1, after.Version,
			"a successful write must bump version, otherwise the next CAS compares against a stale value and never conflicts")
	})

	t.Run("stale version is refused and changes nothing", func(t *testing.T) {
		const postID int64 = 9000000000000001
		before, err := repo.GetByID(ctx, postID)
		require.NoError(t, err)

		stale := before.Version - 1
		err = repo.Update(ctx, postID, "不該被寫進去的內容", "", nil, nil, &stale, nil)
		require.ErrorIs(t, err, domain.ErrVersionConflict)

		after, err := repo.GetByID(ctx, postID)
		require.NoError(t, err)
		require.Equal(t, *before.Content, *after.Content,
			"a refused CAS must leave the row untouched — not losing the concurrent edit is the entire purpose")
		require.Equal(t, before.Version, after.Version)
	})

	// #2772 fix round: there used to be a fourth subtest here,
	// "nil expectedVersion still disables CAS at the repository layer", proving
	// that a nil expectedVersion made it through to a successful write. That
	// subtest is gone, not renamed again — UpdatePost's own SQL no longer
	// carries the `expected_version < 0 OR` disjunct (posts.sql), so passing
	// nil (→ casVersion's -1 sentinel) now makes the predicate unmatchable and
	// the write fails closed as domain.ErrVersionConflict instead of succeeding.
	// Keeping a test that asserted NoError for that call would have kept
	// blessing a call shape this task exists to make unreachable. The *int64
	// parameter and casVersion/noVersionCAS helpers stay (SetPublishStatus and
	// SchedulePublish still legitimately accept nil on the seller's own
	// publish/unpublish/schedule path — see posts.sql and post_repository.go).
}
