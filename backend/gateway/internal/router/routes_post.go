package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerPostRoutes 配置貼文與討論區路由
// 貼文：/public/sellers/:id/posts, /public/posts/:id, /app/sellers/posts, /app/posts/:id/...
// 討論區：/public/sellers/:id/forum, /app/sellers/:id/forum, /app/sellers/forum/:id/...
func registerPostRoutes(public, app *gin.RouterGroup, h *handler.Holder) {
	// ------------------------------------------
	// 貼文 Hashtag — 公開（spec 34 §5.1 / §5.2）
	// ------------------------------------------
	// GET /api/v1/public/tags/suggest
	// GET /api/v1/public/tags/trending
	pubTags := public.Group("/tags")
	{
		pubTags.GET("/suggest", h.SearchPostTags)
		pubTags.GET("/trending", h.GetTrendingPostTags)
	}

	// ------------------------------------------
	// 貼文 — 公開（匿名可存取）
	// ------------------------------------------
	// GET /api/v1/public/sellers/:id/posts
	public.GET(SellersPosts, middleware.OptionalJWTAuth(), h.GetSellerPosts)

	// GET /api/v1/public/posts — 全站最新貼文
	// GET /api/v1/public/posts/:id
	// GET /api/v1/public/posts/:id/comments
	// GET /api/v1/public/posts/comments/:id/replies
	pubPosts := public.Group(PostsPath)
	pubPosts.Use(middleware.OptionalJWTAuth())
	{
		pubPosts.GET("", h.GetLatestPosts)
		pubPosts.GET(PostIDRel, h.GetPostDetail)
		pubPosts.GET(PostCommentsRel, h.GetPostComments)
		pubPosts.GET("/comments/:id/replies", h.GetCommentReplies)
		// 41 號設計書：留言 SSE 流（per-post 匿名訂閱）
		pubPosts.GET("/:id/comments/stream", h.StreamPostComments)
	}

	// ------------------------------------------
	// 貼文 — 需要 JWT（所有已登入用戶）
	// ------------------------------------------
	appPosts := app.Group(PostsPath)
	appPosts.Use(middleware.JWTAuth())
	{
		appPosts.POST(PostLikeRel, h.TogglePostLike)
		appPosts.POST(PostShareRel, h.SharePost)
		// 留言寫入/互動：買賣共用，Gateway 看不到 self-vs-other → 只注入
		// X-Seller-Suspended 供 social usecase 細分（46 §4.4 唯一例外類），不 deny。
		appPosts.POST(PostCommentsRel, injectSellerSuspension(), h.CreatePostComment)
		appPosts.POST("/comments/:id/replies", injectSellerSuspension(), h.ReplyPostComment)
		appPosts.PUT("/comments/:id", injectSellerSuspension(), h.UpdatePostComment)
		appPosts.DELETE("/comments/:id", injectSellerSuspension(), h.DeletePostComment)
		appPosts.POST("/comments/:id/like", injectSellerSuspension(), h.ToggleCommentLike)
	}

	// 賣家帖文 CRUD + 查看自己的貼文（賣家能力層）：發佈/編輯/單篇與列表讀取=CAP_MARKETING（唯讀也鎖），
	// 刪自己貼文=CAP_CONTENT_CONTRACT（收縮型 → 白名單，停權仍可自清）。
	//
	// publish 與 unpublish 分屬兩類，不共用 capability：發布是把內容推向買家＝擴張型，
	// 停權賣家不該做得到；下架只減不增，與 DeletePost 同屬收縮型，停權賣家必須留得住
	// 自清的手（46 §3.3 分類原則）。
	appSellerPosts := app.Group(SellersPath)
	appSellerPosts.Use(middleware.JWTAuth())
	{
		sellerHandle(appSellerPosts, "POST", SellerPosts, authz.CapMarketing, h.CreatePost)
		sellerHandle(appSellerPosts, "GET", SellerPostID, authz.CapMarketing, h.GetMyPostDetail)
		sellerHandle(appSellerPosts, "PUT", SellerPostID, authz.CapMarketing, h.UpdatePost)
		sellerHandle(appSellerPosts, "DELETE", SellerPostID, authz.CapContentContract, h.DeletePost)
		sellerHandle(appSellerPosts, "GET", SellerMePosts, authz.CapMarketing, h.ListMyPosts)
		sellerHandle(appSellerPosts, "POST", SellerPostPublish, authz.CapMarketing, h.PublishMyPost)
		sellerHandle(appSellerPosts, "POST", SellerPostUnpublish, authz.CapContentContract, h.UnpublishMyPost)
	}

	// 舉報貼文（所有已登入用戶）
	app.POST(SellersPath+SellerPostReports, middleware.JWTAuth(), h.ReportPost)

	// ------------------------------------------
	// 討論區 — 公開
	// ------------------------------------------
	// GET /api/v1/public/sellers/:id/forum
	public.GET(SellersForum, middleware.OptionalJWTAuth(), h.GetSellerForum)
	// GET /api/v1/public/sellers/forum/:id/replies
	public.GET(ForumPath+ForumIDReplies, middleware.OptionalJWTAuth(), h.GetForumReplies)

	// ------------------------------------------
	// 討論區 — 需要 JWT（所有已登入用戶）
	// ------------------------------------------
	// POST /api/v1/app/sellers/:id/forum（留言寫入，買賣共用 → 注入 suspension header 細分）
	app.POST(SellersForum, middleware.JWTAuth(), injectSellerSuspension(), h.CreateForumComment)

	// 討論區 reply / delete（在 /sellers/forum 下）
	appForum := app.Group(ForumPath)
	appForum.Use(middleware.JWTAuth())
	{
		appForum.POST(ForumIDReply, injectSellerSuspension(), h.ReplyForumComment)
		appForum.PUT(ForumID, h.UpdateForumComment)
		appForum.DELETE(ForumID, h.DeleteForumComment)
	}
}
