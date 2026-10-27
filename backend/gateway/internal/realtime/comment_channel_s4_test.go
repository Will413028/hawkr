package realtime

import (
	"encoding/json"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/config"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// 49 §6.7 S4：comment.created / comment.replied 的 SSE payload 中 avatarKey 必須是裸 objectKey，
// 組裝點＝接收端前端（buildImageUrl）。發送端（Gateway SSE dispatch）不得組 URL。
func TestCommentChannel_SSE_AvatarKeyIsBareKey(t *testing.T) {
	ch := NewCommentChannel(&config.Config{MediaBaseURL: "https://test-media.example.com"})
	const objectKey = "public/seller_avatars/123.jpg"

	for _, rk := range []string{pkgmq.RoutingKeyCommentCreated, pkgmq.RoutingKeyCommentReplied} {
		ev := map[string]any{
			"postId":  1,
			"comment": map[string]any{"commentId": "1", "postId": "1", "userId": "2", "avatarKey": objectKey},
		}
		payload, _ := json.Marshal(ev)

		_, data, err := ch.BuildSSEEvent(rk, payload)
		if err != nil {
			t.Fatalf("%s BuildSSEEvent error: %v", rk, err)
		}
		var out struct {
			Comment struct {
				AvatarKey string `json:"avatarKey"`
			} `json:"comment"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("%s unmarshal SSE data: %v", rk, err)
		}
		if out.Comment.AvatarKey != objectKey {
			t.Errorf("%s: avatarKey = %q, want bare objectKey %q (WS/SSE must not build URL — S4)", rk, out.Comment.AvatarKey, objectKey)
		}
	}
}
