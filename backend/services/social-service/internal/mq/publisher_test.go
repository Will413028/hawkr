package mq

import (
	"context"
	"testing"
)

// 41 號設計書 §3.3.1:publisher 一律 publish,不論 commenter 與 post author 關係。
// Self-skip 邏輯下放 notification consumer (template_handler.go) 判斷。

// PublishCommentCreated:self 也應該穿透到底層 publish(底層 p.pub == nil → 預期 panic)。
func TestPublisher_PublishCommentCreated_Self_NoLongerSkipped(t *testing.T) {
	p := &Publisher{pub: nil}
	defer func() {
		if r := recover(); r == nil {
			t.Error("self-author case must publish (no skip): expected nil-deref panic, got nil")
		}
	}()
	_ = p.PublishCommentCreated(context.Background(),
		1,        // commentID
		10,       // postID
		50,       // commenterID
		3001,     // commenterPrincipalID
		"自己",     // commenterName
		3001,     // postAuthorID == commenterPrincipalID(舊版會 skip,新版 §3.3.1 必須 publish)
		9001,     // postAuthorStoreID (raw store_id, distinct sample value)
		"seller", // postAuthorType
		"自言自語",   // contentPreview
		0, "", nil,
	)
}

// PublishCommentReplied:self 也應該穿透到底層 publish。
func TestPublisher_PublishCommentReplied_Self_NoLongerSkipped(t *testing.T) {
	p := &Publisher{pub: nil}
	defer func() {
		if r := recover(); r == nil {
			t.Error("self-reply case must publish (no skip): expected nil-deref panic, got nil")
		}
	}()
	_ = p.PublishCommentReplied(context.Background(),
		77,      // replyToUserID
		77,      // replierID == replyToUserID(舊版會 skip,新版必須 publish)
		77,      // replierPrincipalID
		9002,    // replyToUserStoreID (raw store_id, distinct sample value)
		"buyer", // replyToUserType
		"自己",    // replierName
		10, 1, "自己回自己",
		0, "", nil,
	)
}

// 非 self 情況也穿透 publish:確認所有 case 都走 publish path。
func TestPublisher_PublishCommentCreated_NotSelf_WouldHitUnderlying(t *testing.T) {
	p := &Publisher{pub: nil}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when underlying pub is nil and not self-skip, but got nil")
		}
	}()
	_ = p.PublishCommentCreated(context.Background(),
		1, 10,
		100, // commenterID
		100, // commenterPrincipalID
		"留言者",
		50,   // postAuthorID（!= commenterPrincipalID）
		9003, // postAuthorStoreID (raw store_id, distinct sample value)
		"seller",
		"hello",
		0, "", nil,
	)
}

func TestPublisher_PublishCommentReplied_NotSelf_WouldHitUnderlying(t *testing.T) {
	p := &Publisher{pub: nil}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when underlying pub is nil and not self-skip, but got nil")
		}
	}()
	_ = p.PublishCommentReplied(context.Background(),
		200,  // replyToUserID
		100,  // replierID（!= replyToUserID）
		100,  // replierPrincipalID
		9004, // replyToUserStoreID (raw store_id, distinct sample value)
		"buyer",
		"回覆者",
		10, 1, "回覆",
		0, "", nil,
	)
}
