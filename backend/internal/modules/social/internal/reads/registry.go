package reads

import (
	"time"
	"uuid"

	. "github.com/go-jet/jet/v2/postgres"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// Entry 登記一支 reads 的 query：用固定的樣本參數建出語句，給守門測試（邊界、形狀、golden、keyset）逐支檢查。
// 新增的 query 沒有登記，TestEveryQueryIsRegistered 失敗。
type Entry struct {
	Name string
	// Build 以樣本參數建出語句；viewer 不同的語句文字可能不同（個人封鎖）；不依觀看者的 query 忽略它。
	Build func(v visibility.Viewer) Statement
	// Row 是結果的 row struct 的零值；UPDATE … RETURNING 也有。
	Row any
	// Keyset 為真表示這是 keyset 分頁的列表：游標條件必須是單一列比較。
	Keyset bool
}

// 樣本參數：固定值，golden 檔才穩定。
var (
	sampleNow  = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	sampleID   = uuid.UUID{0x01, 0x8f, 0, 0, 0, 0, 0x70, 0, 0x80, 0, 0, 0, 0, 0, 0, 0x01}
	sampleID2  = uuid.UUID{0x01, 0x8f, 0, 0, 0, 0, 0x70, 0, 0x80, 0, 0, 0, 0, 0, 0, 0x02}
	sampleID3  = uuid.UUID{0x01, 0x8f, 0, 0, 0, 0, 0x70, 0, 0x80, 0, 0, 0, 0, 0, 0, 0x03}
	samplePost = PostPage{AfterPublished: sampleNow, AfterID: sampleID, Limit: 51}
	sampleCmt  = CommentPage{AfterCreated: sampleNow, AfterID: sampleID, Limit: 51}
)

// Registry 是 reads 全部的 query。
func Registry() []Entry {
	return []Entry{
		{Name: "GetPublicPost", Row: PostRow{}, Build: func(v visibility.Viewer) Statement { return GetPublicPost(sampleID, sampleNow, v) }},
		{Name: "PostIsPublic", Row: BoolRow{}, Build: func(v visibility.Viewer) Statement { return PostIsPublic(sampleID, sampleNow, v) }},
		{Name: "ListFeedPosts", Row: PostRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListFeedPosts(sampleNow, v, samplePost) }},
		{Name: "ListStorePublicPosts", Row: PostRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListStorePublicPosts(7, sampleNow, v, samplePost) }},
		{Name: "ListHashtagPosts", Row: PostRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListHashtagPosts("organic", sampleNow, v, samplePost) }},
		{Name: "TopPosts", Row: PostRow{}, Build: func(v visibility.Viewer) Statement { return TopPosts(sampleNow, v, 30, 5) }},
		{Name: "SuggestHashtags", Row: HashtagCountRow{}, Build: func(visibility.Viewer) Statement { return SuggestHashtags("org%", sampleNow, 10) }},
		{Name: "TrendingHashtags", Row: HashtagCountRow{}, Build: func(visibility.Viewer) Statement {
			return TrendingHashtags(sampleNow.AddDate(0, 0, -7), sampleNow, sampleNow, 20)
		}},
		{Name: "ListTopLevelComments", Row: CommentRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListTopLevelComments(sampleID, sampleNow, v, sampleCmt) }},
		{Name: "ListOwnerTopLevelComments", Row: CommentRow{}, Keyset: true, Build: func(visibility.Viewer) Statement { return ListOwnerTopLevelComments(sampleID, 7, sampleCmt) }},
		{Name: "ListReplies", Row: CommentRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListReplies(sampleID, sampleNow, v, sampleCmt) }},
		{Name: "ListOwnerReplies", Row: CommentRow{}, Keyset: true, Build: func(visibility.Viewer) Statement { return ListOwnerReplies(sampleID, 7, sampleCmt) }},
		{Name: "ReadableCommentPost", Row: StoreIDRow{}, Build: func(v visibility.Viewer) Statement { return ReadableCommentPost(sampleID, sampleNow, v) }},
		{Name: "ReplyTargetIsReadable", Row: BoolRow{}, Build: func(v visibility.Viewer) Statement { return ReplyTargetIsReadable(sampleID, sampleID2, sampleID3, v) }},
		{Name: "CommentIsHidden", Row: BoolRow{}, Build: func(visibility.Viewer) Statement { return CommentIsHidden(sampleID) }},
		{Name: "ListTopLevelForumEntries", Row: EntryRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListTopLevelForumEntries(7, sampleNow, v, sampleCmt) }},
		{Name: "ListForumReplies", Row: EntryRow{}, Keyset: true, Build: func(v visibility.Viewer) Statement { return ListForumReplies(sampleID, sampleNow, v, sampleCmt) }},
		{Name: "ForumReplyTargetIsReadable", Row: BoolRow{}, Build: func(v visibility.Viewer) Statement { return ForumReplyTargetIsReadable(sampleID, 7, sampleID3, v) }},
		{Name: "ForumEntryIsHidden", Row: BoolRow{}, Build: func(visibility.Viewer) Statement { return ForumEntryIsHidden(sampleID) }},
		{Name: "ForumIsReadable", Row: BoolRow{}, Build: func(v visibility.Viewer) Statement { return ForumIsReadable(7, sampleNow, v) }},
		{Name: "GetCommentOwner", Row: OwnerRow{}, Build: func(visibility.Viewer) Statement { return GetCommentOwner(sampleID, sampleNow) }},
		{Name: "GetForumEntryOwner", Row: OwnerRow{}, Build: func(visibility.Viewer) Statement { return GetForumEntryOwner(sampleID, sampleNow) }},
		{Name: "GetPostOwner", Row: PostOwnerRow{}, Build: func(visibility.Viewer) Statement { return GetPostOwner(sampleID, sampleNow) }},
		{Name: "RecountPostStatsBatch", Row: PostDriftRow{}, Build: func(visibility.Viewer) Statement { return RecountPostStatsBatch(sampleID, sampleID2) }},
		{Name: "RecountCommentStatsBatch", Row: CommentDriftRow{}, Build: func(visibility.Viewer) Statement { return RecountCommentStatsBatch(sampleID, sampleID2) }},
		{Name: "RecountForumEntryStatsBatch", Row: EntryDriftRow{}, Build: func(visibility.Viewer) Statement { return RecountForumEntryStatsBatch(sampleID, sampleID2) }},
	}
}
