package characterization

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// restDriver 以舊 gateway 的 REST API（統一 envelope）操作舊的 social-service：路徑與 wire 型別照舊 gateway 的
// handler，r.rest.Do 送出請求並以 ch.RESTKind 分類錯誤，social 的業務碼先在 restBusinessKind 對應。
// social-service 刪除時一起刪除。
type restDriver struct {
	rest ch.REST

	mu     sync.Mutex
	images map[string]string
}

var _ Driver = (*restDriver)(nil)

func newRESTDriver(base string, tokens map[ch.Caller]string, client *http.Client) *restDriver {
	return &restDriver{
		rest:   ch.REST{Base: base, Tokens: tokens, HTTP: client, Business: restBusinessKind},
		images: map[string]string{},
	}
}

// restBusinessKind 對應 social 的業務碼：hashtag 規則（3015 字元、3016 長度、3017 數量）與社群守則（9210）都是輸入不合法。
func restBusinessKind(code int32) error {
	switch code {
	case 3015, 3016, 3017, 9210:
		return ch.ErrInvalid
	}
	return nil
}

func (d *restDriver) Legacy() bool { return true }

type wirePost struct {
	ID           string   `json:"id"`
	SellerID     string   `json:"sellerId"`
	Content      string   `json:"content"`
	Tags         []string `json:"tags"`
	PublishState string   `json:"publishStatus"`
	HiddenReason string   `json:"hiddenReason"`
	ViewCount    int      `json:"viewCount"`
	ShareCount   int      `json:"shareCount"`
	CommentCount int      `json:"commentCount"`
	LikeCount    int      `json:"likeCount"`
	IsLiked      bool     `json:"isLiked"`
}

func (w wirePost) post() Post {
	status := ""
	switch w.PublishState {
	case "4401":
		status = "DRAFT"
	case "4402":
		status = "PUBLISHED"
	case "4403":
		status = "UNPUBLISHED"
	}
	return Post{
		ID: w.ID, SellerID: w.SellerID, Content: w.Content, Tags: w.Tags, Status: status, HiddenReason: w.HiddenReason,
		ViewCount: w.ViewCount, ShareCount: w.ShareCount, CommentCount: w.CommentCount, LikeCount: w.LikeCount, Liked: w.IsLiked,
	}
}

type wireList[T any] struct {
	Records []T    `json:"records"`
	Total   string `json:"total"`
}

func (l wireList[T]) total() int {
	n, _ := strconv.Atoi(l.Total)
	return n
}

func (d *restDriver) CreatePost(ctx context.Context, c ch.Caller, in PostInput) (Post, error) {
	body := map[string]any{"content": in.Content, "stagedToken": in.Image}
	if in.Tags != nil {
		body["tags"] = in.Tags
	}
	var out wirePost
	if err := d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/sellers/posts", body, &out); err != nil {
		return Post{}, err
	}
	d.mu.Lock()
	d.images[out.ID] = in.Image
	d.mu.Unlock()
	return out.post(), nil
}

func (d *restDriver) UpdatePost(ctx context.Context, c ch.Caller, id string, in PostInput) (Post, error) {
	body := map[string]any{"content": in.Content}
	if in.Tags != nil {
		body["tags"] = in.Tags
	}
	d.mu.Lock()
	kept := d.images[id]
	d.mu.Unlock()
	if in.Image != "" {
		body["newStagedToken"] = in.Image
	} else {
		body["keepAssetId"] = kept
	}
	var out wirePost
	if err := d.rest.Do(ctx, c, http.MethodPut, "/api/v1/app/sellers/posts/"+id, body, &out); err != nil {
		return Post{}, err
	}
	return out.post(), nil
}

func (d *restDriver) PublishPost(ctx context.Context, c ch.Caller, id string) error {
	return d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/sellers/posts/"+id+"/publish", nil, nil)
}

func (d *restDriver) UnpublishPost(ctx context.Context, c ch.Caller, id string) error {
	return d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/sellers/posts/"+id+"/unpublish", nil, nil)
}

func (d *restDriver) DeletePost(ctx context.Context, c ch.Caller, id string) error {
	return d.rest.Do(ctx, c, http.MethodDelete, "/api/v1/app/sellers/posts/"+id, nil, nil)
}

func (d *restDriver) GetMyPost(ctx context.Context, c ch.Caller, id string) (Post, error) {
	var out wirePost
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/app/sellers/posts/"+id, nil, &out); err != nil {
		return Post{}, err
	}
	return out.post(), nil
}

func (d *restDriver) ListMyPosts(ctx context.Context, c ch.Caller, status string) ([]Post, error) {
	var out wireList[wirePost]
	q := url.Values{"page_size": {"50"}}
	if status != "" {
		q.Set("status", status)
	}
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/app/sellers/me/posts?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return mapPosts(out.Records), nil
}

func (d *restDriver) GetPost(ctx context.Context, c ch.Caller, id string) (Post, error) {
	var out wirePost
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/posts/"+id, nil, &out); err != nil {
		return Post{}, err
	}
	return out.post(), nil
}

func (d *restDriver) ViewPost(ctx context.Context, c ch.Caller, id string) error {
	_, err := d.GetPost(ctx, c, id)
	return err
}

func (d *restDriver) ListPosts(ctx context.Context, c ch.Caller, tag string) ([]Post, error) {
	var out wireList[wirePost]
	q := url.Values{"page_size": {"50"}, "tag": {tag}}
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/posts?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return mapPosts(out.Records), nil
}

func (d *restDriver) ListSellerPosts(ctx context.Context, c ch.Caller, sellerID string, page, pageSize int) (Page[Post], error) {
	var out wireList[wirePost]
	q := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/sellers/"+sellerID+"/posts?"+q.Encode(), nil, &out); err != nil {
		return Page[Post]{}, err
	}
	return Page[Post]{Items: mapPosts(out.Records), Total: out.total()}, nil
}

func mapPosts(in []wirePost) []Post {
	out := make([]Post, 0, len(in))
	for _, w := range in {
		out = append(out, w.post())
	}
	return out
}

func (d *restDriver) ToggleLike(ctx context.Context, c ch.Caller, postID string) (bool, error) {
	var out struct {
		Liked bool `json:"liked"`
	}
	err := d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/posts/"+postID+"/like", nil, &out)
	return out.Liked, err
}

func (d *restDriver) Share(ctx context.Context, c ch.Caller, postID string) (int, error) {
	var out struct {
		ShareCount int `json:"shareCount"`
	}
	err := d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/posts/"+postID+"/share", map[string]any{"platform": "line"}, &out)
	return out.ShareCount, err
}

func (d *restDriver) ModeratePost(ctx context.Context, c ch.Caller, postID string, hide bool, reason string) error {
	body := map[string]any{"action": "RESTORE"}
	if hide {
		body = map[string]any{"action": "HIDE", "reason": reason, "reasonCode": "SPAM"}
	}
	return d.rest.Do(ctx, c, http.MethodPatch, "/api/v1/admin/posts/"+postID, body, nil)
}

type wireComment struct {
	ID        string        `json:"id"`
	PostID    string        `json:"postId"`
	ParentID  string        `json:"parentId"`
	ReplyToID string        `json:"replyToId"`
	Content   string        `json:"content"`
	Role      int           `json:"role"`
	UserName  string        `json:"userName"`
	IsMine    bool          `json:"isMine"`
	IsLiked   bool          `json:"isLiked"`
	Replies   []wireComment `json:"replies"`
}

func zeroAsEmpty(id string) string {
	if id == "0" {
		return ""
	}
	return id
}

func (w wireComment) comment() Comment {
	role := "BUYER"
	if w.Role == 2 {
		role = "SELLER"
	}
	out := Comment{
		ID: w.ID, PostID: w.PostID, ParentID: zeroAsEmpty(w.ParentID), ReplyToID: zeroAsEmpty(w.ReplyToID), Content: w.Content,
		Role: role, UserName: w.UserName, IsMine: w.IsMine, Liked: w.IsLiked,
	}
	for _, r := range w.Replies {
		out.Replies = append(out.Replies, r.comment())
	}
	return out
}

func mapComments(in []wireComment) []Comment {
	out := make([]Comment, 0, len(in))
	for _, w := range in {
		out = append(out, w.comment())
	}
	return out
}

func (d *restDriver) CreateComment(ctx context.Context, c ch.Caller, postID, content string) (Comment, error) {
	var out wireComment
	body := map[string]any{"content": content, "clientRequestId": ch.Nonce()}
	if err := d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/posts/"+postID+"/comments", body, &out); err != nil {
		return Comment{}, err
	}
	return out.comment(), nil
}

func (d *restDriver) ReplyComment(ctx context.Context, c ch.Caller, postID, parentID, replyToID, content string) (Comment, error) {
	var out wireComment
	body := map[string]any{"content": content, "postId": postID, "clientRequestId": ch.Nonce()}
	if replyToID != "" {
		body["replyToId"] = replyToID
	}
	if err := d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/posts/comments/"+parentID+"/replies", body, &out); err != nil {
		return Comment{}, err
	}
	return out.comment(), nil
}

func (d *restDriver) UpdateComment(ctx context.Context, c ch.Caller, id, content string) (Comment, error) {
	var out wireComment
	if err := d.rest.Do(ctx, c, http.MethodPut, "/api/v1/app/posts/comments/"+id, map[string]any{"content": content}, &out); err != nil {
		return Comment{}, err
	}
	return out.comment(), nil
}

func (d *restDriver) DeleteComment(ctx context.Context, c ch.Caller, id string) error {
	return d.rest.Do(ctx, c, http.MethodDelete, "/api/v1/app/posts/comments/"+id, nil, nil)
}

func (d *restDriver) ToggleCommentLike(ctx context.Context, c ch.Caller, id string) (bool, error) {
	var out struct {
		Liked bool `json:"liked"`
	}
	err := d.rest.Do(ctx, c, http.MethodPost, "/api/v1/app/posts/comments/"+id+"/like", nil, &out)
	return out.Liked, err
}

func (d *restDriver) ListComments(ctx context.Context, c ch.Caller, postID string) ([]Comment, error) {
	var out wireList[wireComment]
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/posts/"+postID+"/comments?page_size=50", nil, &out); err != nil {
		return nil, err
	}
	return mapComments(out.Records), nil
}

func (d *restDriver) ListReplies(ctx context.Context, c ch.Caller, commentID string) ([]Comment, error) {
	var out wireList[wireComment]
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/posts/comments/"+commentID+"/replies?page_size=50", nil, &out); err != nil {
		return nil, err
	}
	return mapComments(out.Records), nil
}

type wireForum struct {
	ID         string `json:"id"`
	SellerID   string `json:"sellerId"`
	ParentID   string `json:"parentId"`
	Content    string `json:"content"`
	UserName   string `json:"userName"`
	IsMine     bool   `json:"isMine"`
	ReplyCount int    `json:"replyCount"`
}

func (w wireForum) entry() ForumEntry {
	return ForumEntry{
		ID: w.ID, SellerID: w.SellerID, ParentID: zeroAsEmpty(w.ParentID), Content: w.Content, UserName: w.UserName,
		IsMine: w.IsMine, ReplyCount: w.ReplyCount,
	}
}

func mapForum(in []wireForum) []ForumEntry {
	out := make([]ForumEntry, 0, len(in))
	for _, w := range in {
		out = append(out, w.entry())
	}
	return out
}

func (d *restDriver) forumWrite(ctx context.Context, c ch.Caller, method, path, content string) (ForumEntry, error) {
	var out wireForum
	if err := d.rest.Do(ctx, c, method, path, map[string]any{"content": content}, &out); err != nil {
		return ForumEntry{}, err
	}
	return out.entry(), nil
}

func (d *restDriver) CreateForumEntry(ctx context.Context, c ch.Caller, sellerID, content string) (ForumEntry, error) {
	return d.forumWrite(ctx, c, http.MethodPost, "/api/v1/app/sellers/"+sellerID+"/forum", content)
}

func (d *restDriver) ReplyForumEntry(ctx context.Context, c ch.Caller, id, content string) (ForumEntry, error) {
	return d.forumWrite(ctx, c, http.MethodPost, "/api/v1/app/sellers/forum/"+id+"/reply", content)
}

func (d *restDriver) UpdateForumEntry(ctx context.Context, c ch.Caller, id, content string) (ForumEntry, error) {
	return d.forumWrite(ctx, c, http.MethodPut, "/api/v1/app/sellers/forum/"+id, content)
}

func (d *restDriver) DeleteForumEntry(ctx context.Context, c ch.Caller, id string) error {
	return d.rest.Do(ctx, c, http.MethodDelete, "/api/v1/app/sellers/forum/"+id, nil, nil)
}

func (d *restDriver) ListForum(ctx context.Context, c ch.Caller, sellerID string) ([]ForumEntry, error) {
	var out wireList[wireForum]
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/sellers/"+sellerID+"/forum?page_size=50", nil, &out); err != nil {
		return nil, err
	}
	return mapForum(out.Records), nil
}

func (d *restDriver) ListForumReplies(ctx context.Context, c ch.Caller, id string) ([]ForumEntry, error) {
	var out wireList[wireForum]
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/sellers/forum/"+id+"/replies?page_size=50", nil, &out); err != nil {
		return nil, err
	}
	return mapForum(out.Records), nil
}

func (d *restDriver) SuggestTags(ctx context.Context, c ch.Caller, q string) ([]TagCount, error) {
	var out struct {
		Items []struct {
			Name      string `json:"name"`
			PostCount string `json:"postCount"`
		} `json:"items"`
	}
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/tags/suggest?"+url.Values{"q": {q}, "limit": {"20"}}.Encode(), nil, &out); err != nil {
		return nil, err
	}
	res := make([]TagCount, 0, len(out.Items))
	for _, i := range out.Items {
		n, _ := strconv.Atoi(i.PostCount)
		res = append(res, TagCount{Name: i.Name, Count: n})
	}
	return res, nil
}

func (d *restDriver) TrendingTags(ctx context.Context, c ch.Caller) ([]TagCount, error) {
	var out struct {
		Items []struct {
			Hashtag string `json:"hashtag"`
			Count   string `json:"count"`
		} `json:"items"`
	}
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/tags/trending?limit=50", nil, &out); err != nil {
		return nil, err
	}
	res := make([]TagCount, 0, len(out.Items))
	for _, i := range out.Items {
		n, _ := strconv.Atoi(i.Count)
		res = append(res, TagCount{Name: i.Hashtag, Count: n})
	}
	return res, nil
}
