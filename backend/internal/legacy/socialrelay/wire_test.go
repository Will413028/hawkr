package socialrelay

import (
	"reflect"
	"strings"
	"testing"
)

type wireField struct{ typ, tag string }

func newFields(typ reflect.Type) map[string]wireField {
	out := map[string]wireField{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Anonymous {
			for k, v := range newFields(f.Type) {
				out[k] = v
			}
			continue
		}
		key, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		name := f.Type.String()
		if f.Type.Kind() == reflect.Pointer {
			name = "*" + f.Type.Elem().Name()
		}
		out[key] = wireField{typ: name, tag: opts}
	}
	return out
}

// wire 把 notification-service 消費的 payload 形狀釘在這裡：json key、Go 型別與 tag 選項逐欄位列出。
// 增減或改型別都要同時改這張表，golden 只守「模板讀的 key 有送」，這裡守「整個形狀不漂移」。
// clientRequestId 永遠是空值（omitempty），notification-service 不讀它。
var baseEventShape = map[string]wireField{
	"event_id": {"string", ""}, "event_type": {"string", ""}, "source": {"string", ""},
	"timestamp": {"time.Time", ""}, "version": {"string", ""},
}

func withBase(extra map[string]wireField) map[string]wireField {
	out := map[string]wireField{}
	for k, v := range baseEventShape {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestPayloadShapes(t *testing.T) {
	for name, c := range map[string]struct {
		mine reflect.Type
		want map[string]wireField
	}{
		"baseEvent": {reflect.TypeOf(baseEvent{}), baseEventShape},
		"post.created": {reflect.TypeOf(postCreated{}), withBase(map[string]wireField{
			"content_preview": {"string", ""}, "follower_id": {"int64", ""}, "post_id": {"string", ""},
			"seller_avatar_key": {"string", ""}, "seller_name": {"string", ""}, "store_id": {"int64", ""},
		})},
		"comment.created": {reflect.TypeOf(commentCreated{}), withBase(map[string]wireField{
			"clientRequestId": {"string", "omitempty"}, "comment": {"*commentSnapshot", "omitempty"}, "commentId": {"string", ""},
			"commenterId": {"int64", ""}, "commenterName": {"string", ""}, "commenterPrincipalId": {"int64", ""},
			"contentPreview": {"string", ""}, "parentCommentId": {"string", "omitempty"}, "postAuthorId": {"int64", ""},
			"postAuthorStoreId": {"int64", ""}, "postAuthorType": {"string", ""}, "postId": {"string", ""},
		})},
		"comment.replied": {reflect.TypeOf(commentReplied{}), withBase(map[string]wireField{
			"clientRequestId": {"string", "omitempty"}, "comment": {"*commentSnapshot", "omitempty"}, "commentId": {"string", ""},
			"contentPreview": {"string", ""}, "parentCommentId": {"string", "omitempty"}, "postId": {"string", ""},
			"replierId": {"int64", ""}, "replierName": {"string", ""}, "replierPrincipalId": {"int64", ""},
			"replyToUserId": {"int64", ""}, "replyToUserStoreId": {"int64", ""}, "replyToUserType": {"string", ""},
		})},
		"post.liked": {reflect.TypeOf(postLiked{}), withBase(map[string]wireField{
			"likerName": {"string", ""}, "post_id": {"string", ""}, "seller_id": {"int64", ""}, "store_id": {"int64", ""},
		})},
		"comment.like_changed": {reflect.TypeOf(commentLikeChanged{}), withBase(map[string]wireField{
			"action": {"string", ""}, "commentAuthorId": {"int64", ""}, "commentAuthorStoreId": {"int64", ""},
			"commentAuthorType": {"string", ""}, "commentId": {"string", ""}, "likeCount": {"int32", ""},
			"likerKind": {"int32", ""}, "likerName": {"string", ""}, "likerPrincipalId": {"int64", ""},
			"postId": {"string", ""}, "userId": {"int64", ""},
		})},
		"post.hidden_by_platform": {reflect.TypeOf(postHiddenByPlatform{}), withBase(map[string]wireField{
			"post_id": {"string", ""}, "reason": {"string", ""}, "seller_id": {"int64", ""}, "store_id": {"int64", ""},
		})},
		"CommentSnapshot": {reflect.TypeOf(commentSnapshot{}), map[string]wireField{
			"avatarKey": {"string", ""}, "commentId": {"string", ""}, "content": {"string", ""}, "createdAt": {"string", ""},
			"likeCount": {"int32", ""}, "parentId": {"string", ""}, "postId": {"string", ""}, "replyCount": {"int32", ""},
			"replyToId": {"string", ""}, "replyToName": {"string", ""}, "role": {"int32", ""}, "updatedAt": {"string", ""},
			"userId": {"int64", "string"}, "userName": {"string", ""},
		}},
	} {
		if got := newFields(c.mine); !reflect.DeepEqual(c.want, got) {
			t.Errorf("%s: payload shape changed\n want %v\n got  %v", name, c.want, got)
		}
	}
}
