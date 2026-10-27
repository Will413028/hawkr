package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// WS frame 的發送者頭像欄位必須是 `avatarKey`（49 §6.6 preamble：對外 JSON 的頭像
// 欄不帶主體前綴；§6.7 S4：只帶裸 objectKey，前端 buildImageUrl 組顯示 URL）。
//
// 這個欄位帶 `,omitempty`，所以樣本值必須非空，否則欄位整個消失、斷言會因為錯誤
// 的理由通過。
func TestMessage_SenderAvatarWireKeyIsAvatarKey(t *testing.T) {
	const key = "public/defaults/buyer_avatar.png"

	b, err := json.Marshal(&Message{Type: "chat", SenderName: "小明", SenderAvatar: key})
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"avatarKey":"`+key+`"`) {
		t.Errorf("WS frame 缺少 avatarKey；got %s", got)
	}
	if strings.Contains(got, `"senderAvatar"`) {
		t.Errorf("WS frame 回退到舊欄位 senderAvatar；got %s", got)
	}
}
