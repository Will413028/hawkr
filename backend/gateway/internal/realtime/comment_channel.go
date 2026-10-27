package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Will413028/hawkr/gateway/internal/config"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// commentChannelName / commentBatchEventName are the canonical strings used
// for this channel — see 41 號設計書 §2.3 (topic naming) / §3 (channel：comment)。
const (
	commentChannelName    = "comment"
	commentBatchEventName = "comment.batch"
)

// CommentChannel implements Channel for post comment realtime updates.
// 41 號設計書 §三。匿名訂閱、per-IP cap、topic = comment:post:{post_id}。
type CommentChannel struct{}

// NewCommentChannel constructs the comment channel.
// 49 §6.7 S4：WS/SSE payload 只帶 objectKey（user_avatar 值為裸 key），組裝點＝接收端前端
// （buildImageUrl dual-mode）；發送端不組 URL，與服務側禁 BuildURL 一致。cfg 保留以對齊 channel 建構簽章。
func NewCommentChannel(_ *config.Config) *CommentChannel {
	return &CommentChannel{}
}

func (c *CommentChannel) Name() string { return commentChannelName }

func (c *CommentChannel) Exchange() string { return pkgmq.ExchangeContent }

// RoutingKeys returns the literal routing keys this channel handles。
// QueueBind uses "comment.#" wildcard at the consumer level；Registry.Dispatch
// matches the exact incoming routing key against this list.
func (c *CommentChannel) RoutingKeys() []string {
	return []string{
		pkgmq.RoutingKeyCommentCreated,
		pkgmq.RoutingKeyCommentReplied,
		pkgmq.RoutingKeyCommentUpdated,
		pkgmq.RoutingKeyCommentDeleted,
		pkgmq.RoutingKeyCommentLikeChanged,
	}
}

func (c *CommentChannel) BatchEventName() string { return commentBatchEventName }

// AuthorizeSubscribe — comment endpoint is anonymous (per-IP cap enforced by Hub)。
func (c *CommentChannel) AuthorizeSubscribe(_ context.Context, _ string, _ int64) error {
	return nil
}

// commentTopicResolver is a minimal struct used to parse only postId from the
// MQ payload. Different event types share the postId field but differ in
// other shape — we only need postId for topic resolution。
type commentTopicResolver struct {
	PostID int64 `json:"postId"`
}

func (c *CommentChannel) ResolveTopic(routingKey string, payload []byte) (string, bool) {
	var p commentTopicResolver
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", false
	}
	if p.PostID <= 0 {
		return "", false
	}
	return TopicCommentPost(p.PostID), true
}

// BuildSSEEvent maps the routing key to an SSE event name and emits a curated
// data payload tailored to the front-end's needs. We deliberately re-emit a
// shaped JSON (rather than forwarding the full BaseEvent) so the SSE payload
// stays minimal and stable independently of producer-side BaseEvent changes。
//
// 41 號設計書 §3.5 SSE event payload 表。
func (c *CommentChannel) BuildSSEEvent(routingKey string, payload []byte) (string, []byte, error) {
	switch routingKey {
	case pkgmq.RoutingKeyCommentCreated:
		var ev pkgmq.CommentCreatedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("comment.created unmarshal: %w", err)
		}
		// 49 §6.7 S4：user_avatar 維持裸 objectKey 直送，前端 buildImageUrl 組 URL。
		out := commentCreatedSSE{
			Comment:         ev.Comment,
			ClientRequestID: ev.ClientRequestID,
		}
		data, err := json.Marshal(out)
		return "comment.created", data, err

	case pkgmq.RoutingKeyCommentReplied:
		var ev pkgmq.CommentRepliedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("comment.replied unmarshal: %w", err)
		}
		// 49 §6.7 S4：user_avatar 維持裸 objectKey 直送，前端 buildImageUrl 組 URL。
		out := commentCreatedSSE{ // same shape — fat event 含完整 CommentSnapshot
			Comment:         ev.Comment,
			ClientRequestID: ev.ClientRequestID,
		}
		data, err := json.Marshal(out)
		return "comment.replied", data, err

	case pkgmq.RoutingKeyCommentUpdated:
		var ev pkgmq.CommentUpdatedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("comment.updated unmarshal: %w", err)
		}
		out := commentUpdatedSSE{
			CommentID: fmt.Sprintf("%d", ev.CommentID),
			PostID:    fmt.Sprintf("%d", ev.PostID),
			Content:   ev.Content,
			UpdatedAt: ev.UpdatedAt,
		}
		data, err := json.Marshal(out)
		return "comment.updated", data, err

	case pkgmq.RoutingKeyCommentDeleted:
		var ev pkgmq.CommentDeletedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("comment.deleted unmarshal: %w", err)
		}
		out := commentDeletedSSE{
			CommentID:       fmt.Sprintf("%d", ev.CommentID),
			PostID:          fmt.Sprintf("%d", ev.PostID),
			ParentCommentID: fmt.Sprintf("%d", ev.ParentCommentID),
		}
		data, err := json.Marshal(out)
		return "comment.deleted", data, err

	case pkgmq.RoutingKeyCommentLikeChanged:
		var ev pkgmq.CommentLikeChangedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("comment.like_changed unmarshal: %w", err)
		}
		out := commentLikeChangedSSE{
			CommentID: fmt.Sprintf("%d", ev.CommentID),
			PostID:    fmt.Sprintf("%d", ev.PostID),
			LikeCount: ev.LikeCount,
			Action:    ev.Action,
			UserID:    fmt.Sprintf("%d", ev.UserID),
			UserKind:  ev.LikerKind,
		}
		data, err := json.Marshal(out)
		return "comment.like_changed", data, err
	}
	return "", nil, nil
}

// TopicCommentPost is the canonical topic for a single post's comment stream。
func TopicCommentPost(postID int64) string {
	return fmt.Sprintf("comment:post:%d", postID)
}

// ---------------- SSE payload shapes（與 §3.5 表格對齊） ---------------- //
//
// 數字 ID 一律 string（與 Gateway HTTP JSON 對外規範 §4.1 對齊）。
// CommentSnapshot 內部本來就是 int64 — front-end 期待 string 形式由 channel
// 在 marshal 前自行映射；但為避免額外型別轉換成本，本期沿用 snapshot 原樣輸出
// 由 front-end SSE handler 處理（與 §13.3 dedup 偽碼一致：byClientRequestId / id 比對）。

type commentCreatedSSE struct {
	Comment         *pkgmq.CommentSnapshot `json:"comment"`
	ClientRequestID string                 `json:"clientRequestId,omitempty"`
}

type commentUpdatedSSE struct {
	CommentID string `json:"commentId"`
	PostID    string `json:"postId"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updatedAt"`
}

type commentDeletedSSE struct {
	CommentID       string `json:"commentId"`
	PostID          string `json:"postId"`
	ParentCommentID string `json:"parentCommentId"`
}

type commentLikeChangedSSE struct {
	CommentID string `json:"commentId"`
	PostID    string `json:"postId"`
	LikeCount int32  `json:"likeCount"`
	Action    string `json:"action"` // LIKE / UNLIKE
	// UserID、UserKind 是按讚者的 (kind, id)：Hub 內部的共用形狀，不離開 gateway——
	// 事件寫出前由 CommentChannel.Personalizer 換成 likedByViewer。
	UserID   string `json:"userId"`
	UserKind int32  `json:"userKind"`
}
