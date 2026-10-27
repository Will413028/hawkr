// Package characterization 是 social 對外 API 的特性測試（ADR 2026-09-24-go-modular-monolith D8 的第一層
// 安全網、ADR 2026-09-26-external-contract-protobuf-connect D4），共用的部分在 internal/httpapi/characterize。
//
// 情境只寫一次（scenarios_test.go），透過 Driver 操作：REST driver 經 gateway 打舊的 social-service；Connect driver
// 打單體，在 A7-2 加上（連同 connect_test.go 與 monolith_integration_test.go，照 content 的寫法）。本步只跑舊的那一端（scripts/characterize.sh social，build tag gateway）。
//
// 兩個 driver 的 wire 不同，情境只看業務上的結果：ID 當成不透明的字串、錯誤只比 characterize 的分類。與舊實作
// 刻意不同的行為（計畫的 SD 編號）寫成共用的情境，舊的一端以 skipOnLegacy 跳過並附 SD 編號，單體的一端照新行為斷言。
// social-service 刪除時，REST driver、legacy.sh 與 TestLegacy 一起刪除。
//
// 本機的舊堆疊沒有 RabbitMQ：留言數、讚數由 counter.delta 非同步更新，在這裡永遠是 0，所以舊的一端不斷言它們；
// 讚與留言的結果看 liked 旗標與列表內容，分享數與瀏覽數是同步寫入的。
package characterization

import (
	"context"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// PostInput 是建立與更新貼文的輸入。Image 是不透明的圖片參照：REST driver 當成 staged token 送出（假的 media-service
// 一律轉正），更新時沿用同一貼文已有的圖片。
type PostInput struct {
	Content string
	Tags    []string
	Image   string
}

// Post 是貼文。Status 只有店主讀得到：PUBLISHED 或 UNPUBLISHED；HiddenReason 非空表示被平台隱藏（只有店主讀得到）。
type Post struct {
	ID           string
	SellerID     string
	Content      string
	Tags         []string
	Status       string
	HiddenReason string
	ViewCount    int
	ShareCount   int
	CommentCount int
	LikeCount    int
	Liked        bool
}

// Comment 是留言。Role 是 BUYER 或 SELLER；State 只有佔位項目有值（DELETED、HIDDEN），佔位項目沒有內文與作者。
type Comment struct {
	ID        string
	PostID    string
	ParentID  string
	ReplyToID string
	Content   string
	Role      string
	UserName  string
	IsMine    bool
	Liked     bool
	State     string
	Replies   []Comment
}

// ForumEntry 是店家討論區的留言或回覆。
type ForumEntry struct {
	ID         string
	SellerID   string
	ParentID   string
	Content    string
	UserName   string
	IsMine     bool
	ReplyCount int
}

// TagCount 是標籤與使用它的貼文數。
type TagCount struct {
	Name  string
	Count int
}

// Page 是分頁的結果：Total 是符合條件的總筆數。
type Page[T any] struct {
	Items []T
	Total int
}

// Driver 是情境操作 social 的方式：每個對外操作一個方法，參數與回傳值是業務上的型別（不是 wire 的型別），
// 錯誤對應到 characterize 的分類。
type Driver interface {
	// Legacy 回報這個 driver 打的是舊的 social-service。
	Legacy() bool

	CreatePost(ctx context.Context, c ch.Caller, in PostInput) (Post, error)
	UpdatePost(ctx context.Context, c ch.Caller, id string, in PostInput) (Post, error)
	PublishPost(ctx context.Context, c ch.Caller, id string) error
	UnpublishPost(ctx context.Context, c ch.Caller, id string) error
	DeletePost(ctx context.Context, c ch.Caller, id string) error
	// GetMyPost 是店主讀自己的貼文（含狀態與隱藏理由），不計瀏覽。
	GetMyPost(ctx context.Context, c ch.Caller, id string) (Post, error)
	ListMyPosts(ctx context.Context, c ch.Caller, status string) ([]Post, error)
	// GetPost 是公開讀單篇貼文。
	GetPost(ctx context.Context, c ch.Caller, id string) (Post, error)
	// ViewPost 是貼文被看到一次：舊的是公開讀單篇（讀取就計數），單體是 RecordView。
	ViewPost(ctx context.Context, c ch.Caller, id string) error
	ListPosts(ctx context.Context, c ch.Caller, tag string) ([]Post, error)
	ListSellerPosts(ctx context.Context, c ch.Caller, sellerID string, page, pageSize int) (Page[Post], error)
	ToggleLike(ctx context.Context, c ch.Caller, postID string) (bool, error)
	// Share 回傳分享之後的分享數。
	Share(ctx context.Context, c ch.Caller, postID string) (int, error)
	// ModeratePost 是管理員隱藏或還原貼文。
	ModeratePost(ctx context.Context, c ch.Caller, postID string, hide bool, reason string) error

	CreateComment(ctx context.Context, c ch.Caller, postID, content string) (Comment, error)
	// ReplyComment 回覆頂層留言（parentID）；replyToID 是被回覆的那則（可以是 parentID 底下的回覆）。
	ReplyComment(ctx context.Context, c ch.Caller, postID, parentID, replyToID, content string) (Comment, error)
	UpdateComment(ctx context.Context, c ch.Caller, id, content string) (Comment, error)
	DeleteComment(ctx context.Context, c ch.Caller, id string) error
	ToggleCommentLike(ctx context.Context, c ch.Caller, id string) (bool, error)
	ListComments(ctx context.Context, c ch.Caller, postID string) ([]Comment, error)
	ListReplies(ctx context.Context, c ch.Caller, commentID string) ([]Comment, error)

	CreateForumEntry(ctx context.Context, c ch.Caller, sellerID, content string) (ForumEntry, error)
	ReplyForumEntry(ctx context.Context, c ch.Caller, id, content string) (ForumEntry, error)
	UpdateForumEntry(ctx context.Context, c ch.Caller, id, content string) (ForumEntry, error)
	DeleteForumEntry(ctx context.Context, c ch.Caller, id string) error
	ListForum(ctx context.Context, c ch.Caller, sellerID string) ([]ForumEntry, error)
	ListForumReplies(ctx context.Context, c ch.Caller, id string) ([]ForumEntry, error)

	SuggestTags(ctx context.Context, c ch.Caller, q string) ([]TagCount, error)
	TrendingTags(ctx context.Context, c ch.Caller) ([]TagCount, error)
}
