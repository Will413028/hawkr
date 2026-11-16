//go:build integration

package characterization

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminsellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	adminsellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	buyerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	buyerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	sellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// connectDriver 以單體的 Connect API 操作 social（SocialPublicService、SocialAppService，封鎖在 BuyerAppService 與 SellerAppService）。
// 每個呼叫者一組帶著自己 Bearer token 的 client；錯誤只以 ch.ConnectKind 分類，不在這裡改寫。
//
// Connect 的更新與狀態切換要帶 etag，業務情境不認識 etag：driver 記下每個資源最後一次回應裡的 etag，之後任何呼叫者的請求都帶它，
// 非店主的呼叫才走得到授權檢查。留言的所屬貼文與父留言、討論區留言的所屬店家也由回應記下，供只給留言 ID 的方法使用。
type connectDriver struct {
	clients map[ch.Caller]*connectClients

	mu      sync.Mutex
	etag    map[string]string
	postOf  map[string]string
	parent  map[string]string
	storeOf map[string]string
	// measureOf 與 measureEtag 是 ModeratePost 建立的內容下架措施（貼文 ID 對措施 ID、措施 ID 對最後一次回應的 etag），還原時撤銷它。
	measureOf   map[string]string
	measureEtag map[string]string
}

type connectClients struct {
	public socialv1alpha1connect.SocialPublicServiceClient
	app    socialv1alpha1connect.SocialAppServiceClient
	buyer  buyerconnect.BuyerAppServiceClient
	seller sellerconnect.SellerAppServiceClient
	// governance 是後台的措施 service（SellerGovernanceAdminService）：管理員以 CreateMeasure 的內容下架、RevokeMeasure 取代舊的隱藏與還原。
	governance adminsellerconnect.SellerGovernanceAdminServiceClient
}

var _ Driver = (*connectDriver)(nil)

// newConnectDriver 以 base 為單體位址、tokens 為各呼叫者的 session token（沒有 token 的呼叫者是匿名）建立 driver。
func newConnectDriver(httpClient *http.Client, base string, tokens map[ch.Caller]string) *connectDriver {
	d := &connectDriver{
		clients: map[ch.Caller]*connectClients{},
		etag:    map[string]string{}, postOf: map[string]string{}, parent: map[string]string{}, storeOf: map[string]string{},
		measureOf: map[string]string{}, measureEtag: map[string]string{},
	}
	for _, c := range []ch.Caller{ch.Anonymous, ch.Buyer, ch.Buyer2, ch.Seller, ch.Seller2, ch.Admin} {
		token := tokens[c]
		auth := connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if token != "" {
					req.Header().Set("Authorization", "Bearer "+token)
				}
				return next(ctx, req)
			}
		}))
		d.clients[c] = &connectClients{
			public:     socialv1alpha1connect.NewSocialPublicServiceClient(httpClient, base, auth),
			app:        socialv1alpha1connect.NewSocialAppServiceClient(httpClient, base, auth),
			buyer:      buyerconnect.NewBuyerAppServiceClient(httpClient, base, auth),
			seller:     sellerconnect.NewSellerAppServiceClient(httpClient, base, auth),
			governance: adminsellerconnect.NewSellerGovernanceAdminServiceClient(httpClient, base, auth),
		}
	}
	return d
}

func (d *connectDriver) Legacy() bool { return false }

func (d *connectDriver) of(c ch.Caller) *connectClients { return d.clients[c] }

// pageSize 是 driver 走分頁時每頁的筆數。
const pageSize = 50

// ModeratePost 是平台下架或還原貼文（A7-D5′：平台下架是賣家的內容下架措施，A7-D28 第 8 點）：hide 以 CreateMeasure 的內容下架建立措施
// （statement 與註記是 reason），還原是撤銷這支 driver 建立的措施。呼叫者不是管理員時請求照送，由後台 service 的授權擋下。
func (d *connectDriver) ModeratePost(ctx context.Context, c ch.Caller, postID string, hide bool, reason string) error {
	g := d.of(c).governance
	if !hide {
		measureID, ok := d.lookup(d.measureOf, postID)
		if !ok {
			return fmt.Errorf("driver 沒有記錄貼文 %s 的下架措施", postID)
		}
		etag, _ := d.lookup(d.measureEtag, measureID)
		_, err := g.RevokeMeasure(ctx, connect.NewRequest(&adminsellerpb.RevokeMeasureRequest{
			Id: measureID, Reason: adminsellerpb.AdminRevokeReason_ADMIN_REVOKE_REASON_ADMIN_CORRECTION, Etag: etag,
		}))
		if err == nil {
			d.forget(d.measureOf, postID)
		}
		return ch.ConnectKind(err)
	}
	storeID, ok := d.lookup(d.storeOf, postID)
	if !ok {
		return fmt.Errorf("driver 沒有記錄貼文 %s 的店家", postID)
	}
	res, err := g.CreateMeasure(ctx, connect.NewRequest(&adminsellerpb.CreateMeasureRequest{
		RequestId: uuid.NewV4().String(), StoreId: storeID, Statement: reason, Note: reason,
		Measure: &adminsellerpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminsellerpb.ContentTakedownInput{
			Contents: []*sellerpb.ContentRef{{Kind: sellerpb.ContentKind_CONTENT_KIND_POST, Id: postID}},
			Violation: &adminsellerpb.ViolationBasis{Finding: &adminsellerpb.NewFinding{
				Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Severity: sellerpb.ViolationSeverity_VIOLATION_SEVERITY_LOW,
			}},
		}},
	}))
	if err != nil {
		return ch.ConnectKind(err)
	}
	m := res.Msg.GetMeasure()
	d.remember(d.measureOf, postID, m.GetId())
	d.remember(d.measureEtag, m.GetId(), m.GetEtag())
	return nil
}

func (d *connectDriver) forget(m map[string]string, k string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(m, k)
}

// hiddenReason 是店主讀到的下架理由：貼文帶下架措施的 ID 時，經賣家的 GetMyMeasure 讀措施的 statement（理由不在 social，A7-D28 第 1 點）；沒被下架是空字串。
func (d *connectDriver) hiddenReason(ctx context.Context, c ch.Caller, p *pb.Post) (string, error) {
	if p.TakedownMeasureId == nil {
		return "", nil
	}
	res, err := d.of(c).seller.GetMyMeasure(ctx, connect.NewRequest(&sellerpb.GetMyMeasureRequest{MeasureId: p.GetTakedownMeasureId()}))
	if err != nil {
		return "", ch.ConnectKind(err)
	}
	return res.Msg.GetMeasure().GetStatement(), nil
}

func (d *connectDriver) remember(m map[string]string, k, v string) {
	if k == "" || v == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	m[k] = v
}

func (d *connectDriver) lookup(m map[string]string, k string) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := m[k]
	return v, ok
}

func collect[T any](fetch func(token string) ([]T, string, error)) ([]T, error) {
	var all []T
	token := ""
	for {
		items, next, err := fetch(token)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == "" {
			return all, nil
		}
		token = next
	}
}

func trimEnum(prefix, name string) string {
	s := strings.TrimPrefix(name, prefix)
	if s == "UNSPECIFIED" {
		return ""
	}
	return s
}

func (d *connectDriver) post(p *pb.Post) Post {
	d.remember(d.etag, p.GetId(), p.GetEtag())
	d.remember(d.storeOf, p.GetId(), p.GetStore().GetId())
	tags := make([]string, 0, len(p.GetHashtags()))
	for _, h := range p.GetHashtags() {
		tags = append(tags, h.GetName())
	}
	return Post{
		ID: p.GetId(), SellerID: p.GetStore().GetId(), Content: p.GetBody(), Tags: tags,
		Status:    trimEnum("POST_STATUS_", p.GetStatus().String()),
		ViewCount: int(p.GetViewCount()), ShareCount: int(p.GetShareCount()), CommentCount: int(p.GetCommentCount()),
		LikeCount: int(p.GetLikeCount()), Liked: p.GetViewerLiked(),
	}
}

func (d *connectDriver) posts(in []*pb.Post) []Post {
	out := make([]Post, 0, len(in))
	for _, p := range in {
		out = append(out, d.post(p))
	}
	return out
}

func (d *connectDriver) comment(c *pb.Comment) Comment {
	d.remember(d.etag, c.GetId(), c.GetEtag())
	d.remember(d.postOf, c.GetId(), c.GetPostId())
	d.remember(d.parent, c.GetId(), c.GetParentId())
	role := "BUYER"
	if c.GetAuthor().GetKind() == pb.ActorKind_ACTOR_KIND_STORE {
		role = "SELLER"
	}
	return Comment{
		ID: c.GetId(), PostID: c.GetPostId(), ParentID: c.GetParentId(), ReplyToID: c.GetReplyToId(), Content: c.GetBody(),
		Role: role, UserName: c.GetAuthor().GetName(), IsMine: c.GetIsMine(), Liked: c.GetViewerLiked(),
		State: placeholderState(c.GetState()),
	}
}

// placeholderState 只有佔位項目（DELETED、HIDDEN）有值，可見的留言是空字串。
func placeholderState(s pb.CommentState) string {
	if s == pb.CommentState_COMMENT_STATE_VISIBLE {
		return ""
	}
	return trimEnum("COMMENT_STATE_", s.String())
}

func (d *connectDriver) entry(e *pb.ForumEntry) ForumEntry {
	d.remember(d.etag, e.GetId(), e.GetEtag())
	d.remember(d.storeOf, e.GetId(), e.GetStoreId())
	return ForumEntry{
		ID: e.GetId(), SellerID: e.GetStoreId(), ParentID: e.GetParentId(), Content: e.GetBody(), UserName: e.GetAuthor().GetName(),
		IsMine: e.GetIsMine(), ReplyCount: int(e.GetReplyCount()),
	}
}

// etagOf 回傳資源最後一次回應的 etag；沒有記錄是情境的寫法超出 driver 能代勞的範圍，回 harness 的錯誤，不送出沒有 etag 的寫入。
func (d *connectDriver) etagOf(id string) (string, error) {
	if v, ok := d.lookup(d.etag, id); ok {
		return v, nil
	}
	return "", fmt.Errorf("driver 沒有記錄 %s 的 etag", id)
}

func (d *connectDriver) CreatePost(ctx context.Context, c ch.Caller, in PostInput) (Post, error) {
	res, err := d.of(c).app.CreatePost(ctx, connect.NewRequest(&pb.CreatePostRequest{
		Body: in.Content, ImageStagedToken: in.Image, HashtagInputs: in.Tags, Publish: true,
	}))
	if err != nil {
		return Post{}, ch.ConnectKind(err)
	}
	return d.post(res.Msg.GetPost()), nil
}

func (d *connectDriver) UpdatePost(ctx context.Context, c ch.Caller, id string, in PostInput) (Post, error) {
	etag, err := d.etagOf(id)
	if err != nil {
		return Post{}, err
	}
	paths := []string{"body"}
	if in.Tags != nil {
		paths = append(paths, "hashtag_inputs")
	}
	if in.Image != "" {
		paths = append(paths, "image_staged_token")
	}
	res, err := d.of(c).app.UpdatePost(ctx, connect.NewRequest(&pb.UpdatePostRequest{
		Post:       &pb.Post{Id: id, Etag: etag, Body: in.Content, HashtagInputs: in.Tags, ImageStagedToken: in.Image},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	}))
	if err != nil {
		return Post{}, ch.ConnectKind(err)
	}
	return d.post(res.Msg.GetPost()), nil
}

func (d *connectDriver) PublishPost(ctx context.Context, c ch.Caller, id string) error {
	etag, err := d.etagOf(id)
	if err != nil {
		return err
	}
	res, err := d.of(c).app.PublishPost(ctx, connect.NewRequest(&pb.PublishPostRequest{PostId: id, Etag: etag}))
	if err != nil {
		return ch.ConnectKind(err)
	}
	d.post(res.Msg.GetPost())
	return nil
}

func (d *connectDriver) UnpublishPost(ctx context.Context, c ch.Caller, id string) error {
	etag, err := d.etagOf(id)
	if err != nil {
		return err
	}
	res, err := d.of(c).app.UnpublishPost(ctx, connect.NewRequest(&pb.UnpublishPostRequest{PostId: id, Etag: etag}))
	if err != nil {
		return ch.ConnectKind(err)
	}
	d.post(res.Msg.GetPost())
	return nil
}

// DeletePost 不帶 etag（刪除的 etag 是選填，沒帶是無條件刪除）：已刪除的貼文再刪要回 not_found，而不是版本衝突。
func (d *connectDriver) DeletePost(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.of(c).app.DeletePost(ctx, connect.NewRequest(&pb.DeletePostRequest{PostId: id}))
	return ch.ConnectKind(err)
}

func (d *connectDriver) GetMyPost(ctx context.Context, c ch.Caller, id string) (Post, error) {
	res, err := d.of(c).app.GetMyPost(ctx, connect.NewRequest(&pb.GetMyPostRequest{PostId: id}))
	if err != nil {
		return Post{}, ch.ConnectKind(err)
	}
	out := d.post(res.Msg.GetPost())
	if out.HiddenReason, err = d.hiddenReason(ctx, c, res.Msg.GetPost()); err != nil {
		return Post{}, err
	}
	return out, nil
}

// ListMyPosts 的 status 是 ""（全部）、"published" 或 "hidden"：單體的貼文沒有「隱藏」狀態，被平台隱藏是帶下架措施 ID 的貼文（A7-D5′、A7-D28 第 1 點）。
func (d *connectDriver) ListMyPosts(ctx context.Context, c ch.Caller, status string) ([]Post, error) {
	var filter pb.PostStatus
	switch status {
	case "", "hidden":
	case "published":
		filter = pb.PostStatus_POST_STATUS_PUBLISHED
	default:
		return nil, fmt.Errorf("單體的 ListMyPosts 沒有狀態 %q", status)
	}
	return collect(func(token string) ([]Post, string, error) {
		res, err := d.of(c).app.ListMyPosts(ctx, connect.NewRequest(&pb.ListMyPostsRequest{PageSize: pageSize, PageToken: token, Status: filter}))
		if err != nil {
			return nil, "", ch.ConnectKind(err)
		}
		posts := res.Msg.GetPosts()
		if status == "hidden" {
			posts = slices.DeleteFunc(slices.Clone(posts), func(p *pb.Post) bool { return p.TakedownMeasureId == nil })
		}
		return d.posts(posts), res.Msg.GetNextPageToken(), nil
	})
}

func (d *connectDriver) GetPost(ctx context.Context, c ch.Caller, id string) (Post, error) {
	res, err := d.of(c).public.GetPost(ctx, connect.NewRequest(&pb.GetPostRequest{PostId: id}))
	if err != nil {
		return Post{}, ch.ConnectKind(err)
	}
	return d.post(res.Msg.GetPost()), nil
}

func (d *connectDriver) ViewPost(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.of(c).public.RecordPostView(ctx, connect.NewRequest(&pb.RecordPostViewRequest{PostId: id}))
	return ch.ConnectKind(err)
}

func (d *connectDriver) ListPosts(ctx context.Context, c ch.Caller, tag string) ([]Post, error) {
	return collect(func(token string) ([]Post, string, error) {
		res, err := d.of(c).public.ListPosts(ctx, connect.NewRequest(&pb.ListPostsRequest{PageSize: pageSize, PageToken: token, Hashtag: &tag}))
		if err != nil {
			return nil, "", ch.ConnectKind(err)
		}
		return d.posts(res.Msg.GetPosts()), res.Msg.GetNextPageToken(), nil
	})
}

// ListSellerPosts 走游標到第 page 頁（從 1 起算）取那一頁；Total 是走完全部游標數到的筆數。
func (d *connectDriver) ListSellerPosts(ctx context.Context, c ch.Caller, sellerID string, page, size int) (Page[Post], error) {
	fetch := func(token string, n int) ([]Post, string, error) {
		res, err := d.of(c).public.ListPosts(ctx, connect.NewRequest(&pb.ListPostsRequest{PageSize: int32(n), PageToken: token, StoreId: &sellerID}))
		if err != nil {
			return nil, "", ch.ConnectKind(err)
		}
		return d.posts(res.Msg.GetPosts()), res.Msg.GetNextPageToken(), nil
	}
	var out Page[Post]
	token := ""
	for i := 1; ; i++ {
		items, next, err := fetch(token, size)
		if err != nil {
			return Page[Post]{}, err
		}
		if i == page {
			out.Items = items
		}
		out.Total += len(items)
		if next == "" {
			break
		}
		token = next
	}
	return out, nil
}

// ToggleLike 先以呼叫者的身分讀 viewer_liked，再按讚或取消；讀不到貼文就回出讀取的原因。
func (d *connectDriver) ToggleLike(ctx context.Context, c ch.Caller, postID string) (bool, error) {
	cur, err := d.GetPost(ctx, c, postID)
	if err != nil {
		return false, err
	}
	if cur.Liked {
		_, err = d.of(c).app.UnlikePost(ctx, connect.NewRequest(&pb.UnlikePostRequest{PostId: postID}))
		return false, ch.ConnectKind(err)
	}
	_, err = d.of(c).app.LikePost(ctx, connect.NewRequest(&pb.LikePostRequest{PostId: postID}))
	if err != nil {
		return false, ch.ConnectKind(err)
	}
	return true, nil
}

// Share 分享之後以 GetPost 讀分享數（SharePost 的回應是空的）。
func (d *connectDriver) Share(ctx context.Context, c ch.Caller, postID string) (int, error) {
	if _, err := d.of(c).app.SharePost(ctx, connect.NewRequest(&pb.SharePostRequest{PostId: postID})); err != nil {
		return 0, ch.ConnectKind(err)
	}
	p, err := d.GetPost(ctx, c, postID)
	return p.ShareCount, err
}

func (d *connectDriver) CreateComment(ctx context.Context, c ch.Caller, postID, content string) (Comment, error) {
	return d.createComment(ctx, c, &pb.CreateCommentRequest{PostId: postID, Body: content})
}

func (d *connectDriver) ReplyComment(ctx context.Context, c ch.Caller, postID, parentID, replyToID, content string) (Comment, error) {
	req := &pb.CreateCommentRequest{PostId: postID, Body: content, ParentId: &parentID}
	if replyToID != "" {
		req.ReplyToId = &replyToID
	}
	return d.createComment(ctx, c, req)
}

func (d *connectDriver) createComment(ctx context.Context, c ch.Caller, req *pb.CreateCommentRequest) (Comment, error) {
	res, err := d.of(c).app.CreateComment(ctx, connect.NewRequest(req))
	if err != nil {
		return Comment{}, ch.ConnectKind(err)
	}
	return d.comment(res.Msg.GetComment()), nil
}

func (d *connectDriver) UpdateComment(ctx context.Context, c ch.Caller, id, content string) (Comment, error) {
	etag, err := d.etagOf(id)
	if err != nil {
		return Comment{}, err
	}
	res, err := d.of(c).app.UpdateComment(ctx, connect.NewRequest(&pb.UpdateCommentRequest{
		Comment:    &pb.Comment{Id: id, Etag: etag, Body: content},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}},
	}))
	if err != nil {
		return Comment{}, ch.ConnectKind(err)
	}
	return d.comment(res.Msg.GetComment()), nil
}

// DeleteComment 不帶 etag（無條件刪除），已刪除的留言再刪回 not_found。
func (d *connectDriver) DeleteComment(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.of(c).app.DeleteComment(ctx, connect.NewRequest(&pb.DeleteCommentRequest{CommentId: id}))
	return ch.ConnectKind(err)
}

// ToggleCommentLike 先以呼叫者的身分在留言所在的列表讀 viewer_liked，再按讚或取消；讀不到留言是 harness 或讀取的錯誤，原樣回傳。
func (d *connectDriver) ToggleCommentLike(ctx context.Context, c ch.Caller, id string) (bool, error) {
	postID, ok := d.lookup(d.postOf, id)
	if !ok {
		return false, fmt.Errorf("driver 沒有記錄留言 %s 所屬的貼文", id)
	}
	var parent *string
	if p, isReply := d.lookup(d.parent, id); isReply {
		parent = &p
	}
	list, err := d.listComments(ctx, c, postID, parent)
	if err != nil {
		return false, err
	}
	got, found := findComment(list, id)
	if !found {
		return false, fmt.Errorf("留言 %s 不在呼叫者讀得到的列表裡", id)
	}
	liked := got.Liked
	if liked {
		_, err := d.of(c).app.UnlikeComment(ctx, connect.NewRequest(&pb.UnlikeCommentRequest{CommentId: id}))
		return false, ch.ConnectKind(err)
	}
	if _, err := d.of(c).app.LikeComment(ctx, connect.NewRequest(&pb.LikeCommentRequest{CommentId: id})); err != nil {
		return false, ch.ConnectKind(err)
	}
	return true, nil
}

// listComments 列出貼文的頂層留言（parent 為 nil）或某則頂層留言的回覆，走完全部游標。
func (d *connectDriver) listComments(ctx context.Context, c ch.Caller, postID string, parent *string) ([]Comment, error) {
	return collect(func(token string) ([]Comment, string, error) {
		res, err := d.of(c).public.ListComments(ctx, connect.NewRequest(&pb.ListCommentsRequest{PostId: postID, PageSize: pageSize, PageToken: token, ParentId: parent}))
		if err != nil {
			return nil, "", ch.ConnectKind(err)
		}
		out := make([]Comment, 0, len(res.Msg.GetComments()))
		for _, cm := range res.Msg.GetComments() {
			out = append(out, d.comment(cm))
		}
		return out, res.Msg.GetNextPageToken(), nil
	})
}

// ListComments 回傳頂層留言，每則帶它底下的回覆（Connect 的回覆要另外列出）。
func (d *connectDriver) ListComments(ctx context.Context, c ch.Caller, postID string) ([]Comment, error) {
	top, err := d.listComments(ctx, c, postID, nil)
	if err != nil {
		return nil, err
	}
	for i := range top {
		id := top[i].ID
		replies, err := d.listComments(ctx, c, postID, &id)
		if err != nil {
			return nil, err
		}
		top[i].Replies = replies
	}
	return top, nil
}

func (d *connectDriver) ListReplies(ctx context.Context, c ch.Caller, commentID string) ([]Comment, error) {
	postID, ok := d.lookup(d.postOf, commentID)
	if !ok {
		return nil, fmt.Errorf("driver 沒有記錄留言 %s 所屬的貼文", commentID)
	}
	return d.listComments(ctx, c, postID, &commentID)
}

func (d *connectDriver) createEntry(ctx context.Context, c ch.Caller, req *pb.CreateForumEntryRequest) (ForumEntry, error) {
	res, err := d.of(c).app.CreateForumEntry(ctx, connect.NewRequest(req))
	if err != nil {
		return ForumEntry{}, ch.ConnectKind(err)
	}
	return d.entry(res.Msg.GetEntry()), nil
}

func (d *connectDriver) CreateForumEntry(ctx context.Context, c ch.Caller, sellerID, content string) (ForumEntry, error) {
	return d.createEntry(ctx, c, &pb.CreateForumEntryRequest{StoreId: sellerID, Body: content})
}

func (d *connectDriver) ReplyForumEntry(ctx context.Context, c ch.Caller, id, content string) (ForumEntry, error) {
	store, ok := d.lookup(d.storeOf, id)
	if !ok {
		return ForumEntry{}, fmt.Errorf("driver 沒有記錄討論區留言 %s 所屬的店家", id)
	}
	return d.createEntry(ctx, c, &pb.CreateForumEntryRequest{StoreId: store, Body: content, ParentId: &id})
}

func (d *connectDriver) UpdateForumEntry(ctx context.Context, c ch.Caller, id, content string) (ForumEntry, error) {
	etag, err := d.etagOf(id)
	if err != nil {
		return ForumEntry{}, err
	}
	res, err := d.of(c).app.UpdateForumEntry(ctx, connect.NewRequest(&pb.UpdateForumEntryRequest{
		Entry:      &pb.ForumEntry{Id: id, Etag: etag, Body: content},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}},
	}))
	if err != nil {
		return ForumEntry{}, ch.ConnectKind(err)
	}
	return d.entry(res.Msg.GetEntry()), nil
}

func (d *connectDriver) DeleteForumEntry(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.of(c).app.DeleteForumEntry(ctx, connect.NewRequest(&pb.DeleteForumEntryRequest{EntryId: id}))
	return ch.ConnectKind(err)
}

func (d *connectDriver) listForum(ctx context.Context, c ch.Caller, sellerID string, parent *string) ([]ForumEntry, error) {
	return collect(func(token string) ([]ForumEntry, string, error) {
		res, err := d.of(c).public.ListForumEntries(ctx, connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: sellerID, PageSize: pageSize, PageToken: token, ParentId: parent}))
		if err != nil {
			return nil, "", ch.ConnectKind(err)
		}
		out := make([]ForumEntry, 0, len(res.Msg.GetEntries()))
		for _, e := range res.Msg.GetEntries() {
			out = append(out, d.entry(e))
		}
		return out, res.Msg.GetNextPageToken(), nil
	})
}

func (d *connectDriver) ListForum(ctx context.Context, c ch.Caller, sellerID string) ([]ForumEntry, error) {
	return d.listForum(ctx, c, sellerID, nil)
}

func (d *connectDriver) ListForumReplies(ctx context.Context, c ch.Caller, id string) ([]ForumEntry, error) {
	store, ok := d.lookup(d.storeOf, id)
	if !ok {
		return nil, fmt.Errorf("driver 沒有記錄討論區留言 %s 所屬的店家", id)
	}
	return d.listForum(ctx, c, store, &id)
}

func (d *connectDriver) SuggestTags(ctx context.Context, c ch.Caller, q string) ([]TagCount, error) {
	res, err := d.of(c).public.SuggestTags(ctx, connect.NewRequest(&pb.SuggestTagsRequest{Query: q, MaxResults: 20}))
	if err != nil {
		return nil, ch.ConnectKind(err)
	}
	return tagCounts(res.Msg.GetTags()), nil
}

func (d *connectDriver) TrendingTags(ctx context.Context, c ch.Caller) ([]TagCount, error) {
	res, err := d.of(c).public.SuggestTrendingTags(ctx, connect.NewRequest(&pb.SuggestTrendingTagsRequest{MaxResults: 50}))
	if err != nil {
		return nil, ch.ConnectKind(err)
	}
	return tagCounts(res.Msg.GetTags()), nil
}

func tagCounts(in []*pb.TagCount) []TagCount {
	out := make([]TagCount, 0, len(in))
	for _, t := range in {
		out = append(out, TagCount{Name: t.GetName(), Count: int(t.GetPostCount())})
	}
	return out
}

func (d *connectDriver) BlockAuthor(ctx context.Context, c ch.Caller, commentID string) (bool, error) {
	res, err := d.of(c).buyer.BlockAuthor(ctx, connect.NewRequest(&buyerpb.BlockAuthorRequest{
		Source: &buyerpb.BlockAuthorRequest_CommentId{CommentId: commentID},
	}))
	if err != nil {
		return false, ch.ConnectKind(err)
	}
	return res.Msg.GetChanged(), nil
}

func (d *connectDriver) BlockStore(ctx context.Context, c ch.Caller, sellerID string) (bool, error) {
	res, err := d.of(c).buyer.BlockStore(ctx, connect.NewRequest(&buyerpb.BlockStoreRequest{StoreId: sellerID}))
	if err != nil {
		return false, ch.ConnectKind(err)
	}
	return res.Msg.GetChanged(), nil
}

func (d *connectDriver) UnblockStore(ctx context.Context, c ch.Caller, sellerID string) (bool, error) {
	res, err := d.of(c).buyer.UnblockStore(ctx, connect.NewRequest(&buyerpb.UnblockStoreRequest{StoreId: sellerID}))
	if err != nil {
		return false, ch.ConnectKind(err)
	}
	return res.Msg.GetChanged(), nil
}

func (d *connectDriver) SellerBlockAuthor(ctx context.Context, c ch.Caller, commentID string) (bool, error) {
	res, err := d.of(c).seller.BlockBuyer(ctx, connect.NewRequest(&sellerpb.BlockBuyerRequest{
		Source: &sellerpb.BlockBuyerRequest_CommentId{CommentId: commentID},
	}))
	if err != nil {
		return false, ch.ConnectKind(err)
	}
	return res.Msg.GetChanged(), nil
}

func (d *connectDriver) SellerBlockedBuyers(ctx context.Context, c ch.Caller) ([]BlockedBuyer, error) {
	return collect(func(token string) ([]BlockedBuyer, string, error) {
		res, err := d.of(c).seller.ListBlockedBuyers(ctx, connect.NewRequest(&sellerpb.ListBlockedBuyersRequest{PageSize: pageSize, PageToken: token}))
		if err != nil {
			return nil, "", ch.ConnectKind(err)
		}
		out := make([]BlockedBuyer, 0, len(res.Msg.GetBlockedBuyers()))
		for _, b := range res.Msg.GetBlockedBuyers() {
			out = append(out, BlockedBuyer{BuyerID: b.GetBuyer().GetId(), Deleted: b.GetBuyer().GetDeleted()})
		}
		return out, res.Msg.GetNextPageToken(), nil
	})
}

func (d *connectDriver) SellerUnblockBuyer(ctx context.Context, c ch.Caller, buyerID string) (bool, error) {
	res, err := d.of(c).seller.UnblockBuyer(ctx, connect.NewRequest(&sellerpb.UnblockBuyerRequest{BuyerId: buyerID}))
	if err != nil {
		return false, ch.ConnectKind(err)
	}
	return res.Msg.GetChanged(), nil
}
