package post

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// 代行者建立貼文不可同時發布：建立時順便發布的 PostPublished 不帶代行者，會產生沒有 POST_PUBLISH 稽核的發布（A7-D27）。拒絕發生在任何讀寫之前。
func TestDelegatedCreateRejectsPublish(t *testing.T) {
	s := &Service{}
	_, err := s.Create(context.Background(), seller.Store{}, model.CreatePostInput{Body: "x", Publish: true}, &delegatedactor.Actor{SessionID: 1})
	if err == nil {
		t.Fatal("代行者建立並發布應該被拒絕")
	}
}
