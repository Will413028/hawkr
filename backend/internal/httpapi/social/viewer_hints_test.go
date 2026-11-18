package social

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// 店主的 viewer_can_comment 與 viewer_can_reply 要與寫入的前置檢查（貼文公開可讀）一致：草稿與下架的貼文寫入會 NotFound，提示不能是 true。
func TestOwnerPostViewerCanCommentRequiresReadablePost(t *testing.T) {
	for _, c := range []struct {
		name       string
		unreadable bool
		want       bool
	}{{"店主的貼文公開可讀", false, true}, {"店主的貼文是草稿或已下架", true, false}} {
		t.Run(c.name, func(t *testing.T) {
			p := ownPost(5, social.StatusDraft)
			f := &fakeOwner{post: p}
			if c.unreadable {
				f.unreadable = map[uuid.UUID]bool{p.ID: true}
			}
			e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
			resp, err := e.client.GetMyPost(t.Context(), signedIn(&pb.GetMyPostRequest{PostId: p.ID.String()}))
			if err != nil {
				t.Fatal(err)
			}
			if got := resp.Msg.GetPost().GetViewerCanComment(); got != c.want {
				t.Errorf("viewer_can_comment = %v，預期 %v", got, c.want)
			}
		})
	}
}

func TestOwnerCommentViewerCanReplyRequiresReadablePost(t *testing.T) {
	at := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	open, closed := uuid.New(), uuid.New()
	mk := func(post uuid.UUID) social.Comment {
		return social.Comment{ID: uuid.New(), PostID: post, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: 9}, Body: "請問", CreatedAt: at, UpdatedAt: at, Version: 2}
	}
	f := &fakeOwner{comments: social.CommentPage{StoreID: 5, Comments: []social.Comment{mk(open), mk(closed), mk(open)}}}
	f.unreadable = map[uuid.UUID]bool{closed: true}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	resp, err := e.client.ListStoreComments(t.Context(), signedIn(&pb.ListStoreCommentsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetComments()
	if !got[0].GetViewerCanReply() || got[1].GetViewerCanReply() || !got[2].GetViewerCanReply() {
		t.Errorf("viewer_can_reply = %v %v %v，預期 true false true", got[0].GetViewerCanReply(), got[1].GetViewerCanReply(), got[2].GetViewerCanReply())
	}
	if got[0].GetViewerCanDelete() != true || got[1].GetViewerCanDelete() != true {
		t.Error("刪除不受貼文是否公開影響")
	}
}

// 討論區的提示與新增討論區留言的前置檢查同源：店家不可曝光（或觀看者被封鎖）時寫入 NotFound，店主與買家的提示都要是 false。
func TestForumViewerHintsFollowForumReadability(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	e := social.ForumEntry{ID: uuid.New(), StoreID: 5, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: 8}, Body: "x", CreatedAt: now, UpdatedAt: now}
	for _, c := range []struct {
		name   string
		store  *seller.Store
		closed bool
		want   bool
	}{
		{"買家，討論區可用", nil, false, true},
		{"買家，店家不可曝光", nil, true, false},
		{"店主，討論區可用", &seller.Store{StoreID: 5}, false, true},
		{"店主，店家不可曝光", &seller.Store{StoreID: 5}, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := &fakeReads{forum: social.ForumPage{Entries: []social.ForumEntry{e}}}
			r.forumClosed = c.closed
			h := newPublic(t, r, fakeBuyers{id: 7}, &fakeStores{own: c.store})
			resp, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: "5"}))
			if err != nil {
				t.Fatal(err)
			}
			if resp.Msg.GetViewerCanComment() != c.want || resp.Msg.GetEntries()[0].GetViewerCanReply() != c.want {
				t.Errorf("viewer_can_comment=%v viewer_can_reply=%v，預期都是 %v", resp.Msg.GetViewerCanComment(), resp.Msg.GetEntries()[0].GetViewerCanReply(), c.want)
			}
		})
	}
	t.Run("被店家封鎖的買家", func(t *testing.T) {
		r := &fakeReads{}
		r.forumClosed = true
		h := newPublic(t, r, fakeBuyers{id: 7}, &fakeStores{blocking: map[int64]bool{5: true}})
		resp, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: "5"}))
		if err != nil || resp.Msg.GetViewerCanComment() {
			t.Errorf("err=%v can=%v", err, resp.Msg.GetViewerCanComment())
		}
	})
}
