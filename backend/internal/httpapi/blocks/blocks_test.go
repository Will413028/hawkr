package blocks

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeBuyers map[int64]int64

func (f fakeBuyers) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) {
	if a, ok := f[id]; ok {
		return a, nil
	}
	return 0, apperr.NotFound("no buyer")
}

type fakeStores map[int64]int64

func (f fakeStores) Store(_ context.Context, id int64) (seller.Store, error) {
	if a, ok := f[id]; ok {
		return seller.Store{StoreID: id, AccountID: a}, nil
	}
	return seller.Store{}, apperr.NotFound("no store")
}

func ids() Identities {
	// 帳號 100 擁有買家 1 與店 10；帳號 200 擁有買家 2 與店 20。
	return Identities{Buyers: fakeBuyers{1: 100, 2: 200}, Stores: fakeStores{10: 100, 20: 200}}
}

func TestRejectSamePersonRejectsABuyerBlockingTheirOwnStore(t *testing.T) {
	err := ids().RejectSamePerson(context.Background(), 1, 10)
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("買家封鎖自己的店要 invalid_argument：%v", err)
	}
}

func TestRejectSamePersonRejectsAStoreBlockingTheOwnersBuyerIdentity(t *testing.T) {
	// 店 20 的店主（帳號 200）的買家身分是 2：同一個函式、另一個方向的入口。
	err := ids().RejectSamePerson(context.Background(), 2, 20)
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("店家封鎖店主本人的買家身分要 invalid_argument：%v", err)
	}
}

func TestRejectSamePersonAllowsDifferentPeople(t *testing.T) {
	for _, c := range []struct{ buyer, store int64 }{{1, 20}, {2, 10}} {
		if err := ids().RejectSamePerson(context.Background(), c.buyer, c.store); err != nil {
			t.Fatalf("買家 %d 與店 %d 是不同的人：%v", c.buyer, c.store, err)
		}
	}
}

func TestRejectSamePersonHidesNothingForMissingParties(t *testing.T) {
	if err := ids().RejectSamePerson(context.Background(), 1, 999); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("店家不存在：%v", err)
	}
	if err := ids().RejectSamePerson(context.Background(), 999, 10); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("買家不存在：%v", err)
	}
}

type fakeAuthors struct {
	got    social.ContentAuthor
	err    error
	called social.Target
}

func (f *fakeAuthors) ContentAuthorFor(_ context.Context, _ social.Caller, t social.Target) (social.ContentAuthor, error) {
	f.called = t
	return f.got, f.err
}

func TestResolveAuthorReadsTheReferenceByItsKind(t *testing.T) {
	id := uuid.UUID{1}
	a := &fakeAuthors{got: social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorBuyer, ID: 5}}}
	if _, err := (Sources{Authors: a}).ResolveAuthor(context.Background(), social.Caller{}, Ref{CommentID: id.String()}); err != nil || a.called.Kind != social.TargetComment || a.called.ID != id {
		t.Fatalf("留言：%v %+v", err, a.called)
	}
	if _, err := (Sources{Authors: a}).ResolveAuthor(context.Background(), social.Caller{}, Ref{ForumEntryID: id.String()}); err != nil || a.called.Kind != social.TargetForumEntry {
		t.Fatalf("討論區：%v %+v", err, a.called)
	}
}

func TestResolveAuthorMapsAnUnreadableReferenceToNotFound(t *testing.T) {
	a := &fakeAuthors{}
	_, err := (Sources{Authors: a}).ResolveAuthor(context.Background(), social.Caller{}, Ref{CommentID: uuid.UUID{1}.String()})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("讀不到要 not_found：%v", err)
	}
}

func TestResolveAuthorRejectsAnAmbiguousOrEmptyReference(t *testing.T) {
	a := &fakeAuthors{}
	id := uuid.UUID{1}.String()
	for name, ref := range map[string]Ref{
		"都沒設": {}, "都設了": {CommentID: id, ForumEntryID: id}, "留言與評價": {CommentID: id, ReviewID: id},
		"格式錯誤": {CommentID: "x"}, "評價格式錯誤": {ReviewID: "x"},
	} {
		if _, err := (Sources{Authors: a}).ResolveAuthor(context.Background(), social.Caller{}, ref); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s要 invalid_argument：%v", name, err)
		}
	}
	boom := errors.New("boom")
	a.err = boom
	if _, err := (Sources{Authors: a}).ResolveAuthor(context.Background(), social.Caller{}, Ref{CommentID: id}); !errors.Is(err, boom) {
		t.Fatalf("模組錯誤原樣回傳：%v", err)
	}
}

type fakeDisplays struct {
	rows  []buyer.Display
	calls int
}

func (f *fakeDisplays) BatchDisplay(_ context.Context, _ []int64) ([]buyer.Display, error) {
	f.calls++
	return f.rows, nil
}

func TestBuyerViewsShowsClosedAndMissingBuyersAsDeletedWithOneLookup(t *testing.T) {
	name, avatar := "Amy", "k/a.png"
	d := &fakeDisplays{rows: []buyer.Display{
		{BuyerID: 1, DisplayName: &name, Avatar: &avatar},
		{BuyerID: 2, DisplayName: &name, Avatar: &avatar, Deleted: true},
	}}
	got, err := BuyerViews(context.Background(), d, []int64{1, 2, 3})
	if err != nil || d.calls != 1 {
		t.Fatalf("批次一次：%v %d", err, d.calls)
	}
	if v := got[1]; v.Deleted || v.DisplayName != "Amy" || v.Avatar == nil {
		t.Fatalf("存活的買家：%+v", v)
	}
	for _, id := range []int64{2, 3} {
		if v := got[id]; !v.Deleted || v.DisplayName != "" || v.Avatar != nil {
			t.Fatalf("買家 %d 要顯示成已刪除、不帶名稱與頭像：%+v", id, v)
		}
	}
}

func TestLiveBuyerRejectsClosedAndMissingBuyers(t *testing.T) {
	name := "Amy"
	d := &fakeDisplays{rows: []buyer.Display{{BuyerID: 1, DisplayName: &name}, {BuyerID: 2, Deleted: true}}}
	if err := LiveBuyer(context.Background(), d, 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{2, 3} {
		if err := LiveBuyer(context.Background(), d, id); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("買家 %d：%v", id, err)
		}
	}
}

type fakeReviews struct {
	r         review.Review
	err       error
	gotID     uuid.UUID
	gotViewer int64
	calls     int
}

func (f *fakeReviews) GetReview(_ context.Context, id uuid.UUID, viewer int64) (review.Review, error) {
	f.calls++
	f.gotID, f.gotViewer = id, viewer
	return f.r, f.err
}

func TestResolveAuthorOfAReviewForABuyerIsTheReplyingStoreReadAsTheViewer(t *testing.T) {
	id := uuid.UUID{7}
	f := &fakeReviews{r: review.Review{ID: id, BuyerID: 3, StoreID: 20, Reply: &review.Reply{Content: "謝謝"}}}
	a := &fakeAuthors{}
	got, err := (Sources{Authors: a, Reviews: f}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 5}, Ref{ReviewID: id.String()})
	if err != nil || got.Author != (social.Actor{Kind: social.ActorStore, ID: 20}) || got.StoreID != 20 {
		t.Fatalf("買家以評價封鎖的是回覆它的店：%+v %v", got, err)
	}
	if f.gotID != id || f.gotViewer != 5 || a.called != (social.Target{}) {
		t.Fatalf("評價參照只問 review，而且帶買家自己的封鎖當觀看者：%+v 觀看者 %d social %+v", f.gotID, f.gotViewer, a.called)
	}
}

func TestResolveAuthorOfAReviewForAStoreIsTheReviewingBuyerReadAsAnAnonymousViewer(t *testing.T) {
	id := uuid.UUID{7}
	f := &fakeReviews{r: review.Review{ID: id, BuyerID: 3, StoreID: 20}}
	got, err := (Sources{Reviews: f}).ResolveAuthor(context.Background(), social.Caller{Store: &seller.Store{StoreID: 20}}, Ref{ReviewID: id.String()})
	if err != nil || got.Author != (social.Actor{Kind: social.ActorBuyer, ID: 3}) || got.StoreID != 20 {
		t.Fatalf("店家以評價封鎖的是寫評價的買家：%+v %v", got, err)
	}
	if f.gotViewer != 0 {
		t.Fatalf("店家以訪客的公開可讀規則讀評價：觀看者 %d", f.gotViewer)
	}
}

func TestResolveAuthorOfAReviewWithoutAVisibleReplyHasNoStoreToBlock(t *testing.T) {
	f := &fakeReviews{r: review.Review{ID: uuid.UUID{7}, BuyerID: 3, StoreID: 20}}
	_, err := (Sources{Reviews: f}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 5}, Ref{ReviewID: uuid.UUID{7}.String()})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有回覆（或回覆被下架）的評價沒有可封鎖的店：%v", err)
	}
}

func TestResolveAuthorOfAnUnreadableReviewIsNotFound(t *testing.T) {
	f := &fakeReviews{err: apperr.NotFound("no review")}
	for _, c := range []social.Caller{{BuyerID: 5}, {Store: &seller.Store{StoreID: 20}}} {
		_, err := (Sources{Reviews: f}).ResolveAuthor(context.Background(), c, Ref{ReviewID: uuid.UUID{7}.String()})
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("讀不到的評價要 not_found：%v", err)
		}
	}
	boom := errors.New("boom")
	f.err = boom
	if _, err := (Sources{Reviews: f}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 5}, Ref{ReviewID: uuid.UUID{7}.String()}); !errors.Is(err, boom) {
		t.Fatalf("模組錯誤原樣回傳：%v", err)
	}
}

type fakeMessages struct {
	p     im.ConversationParties
	err   error
	gotID uuid.UUID
	calls int
}

func (f *fakeMessages) MessageParties(_ context.Context, id uuid.UUID) (im.ConversationParties, error) {
	f.calls++
	f.gotID = id
	return f.p, f.err
}

func TestResolveAuthorOfAMessageIsTheCounterpartOfTheCaller(t *testing.T) {
	id := uuid.UUID{7}
	f := &fakeMessages{p: im.ConversationParties{BuyerID: 3, StoreID: 20}}
	got, err := (Sources{Messages: f}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 3}, Ref{MessageID: id.String()})
	if err != nil || got.Author != (social.Actor{Kind: social.ActorStore, ID: 20}) || got.StoreID != 20 {
		t.Fatalf("買家以訊息封鎖的是對話的店：%+v %v", got, err)
	}
	got, err = (Sources{Messages: f}).ResolveAuthor(context.Background(), social.Caller{Store: &seller.Store{StoreID: 20}}, Ref{MessageID: id.String()})
	if err != nil || got.Author != (social.Actor{Kind: social.ActorBuyer, ID: 3}) || got.StoreID != 20 {
		t.Fatalf("店家以訊息封鎖的是對話的買家：%+v %v", got, err)
	}
	if f.gotID != id {
		t.Fatalf("問 im 的是請求的訊息：%v", f.gotID)
	}
}

func TestResolveAuthorOfAMessageTheCallerIsNotPartyToIsNotFoundLikeAMissingOne(t *testing.T) {
	ref := Ref{MessageID: uuid.UUID{7}.String()}
	f := &fakeMessages{p: im.ConversationParties{BuyerID: 3, StoreID: 20}}
	for name, c := range map[string]social.Caller{
		"別的買家":            {BuyerID: 4},
		"別家店":             {Store: &seller.Store{StoreID: 21}},
		"買家 ID 等於對話的店 ID": {BuyerID: 20},
	} {
		_, err := (Sources{Messages: f}).ResolveAuthor(context.Background(), c, ref)
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：%v", name, err)
		}
	}
	missing := &fakeMessages{err: apperr.NotFound("no message")}
	_, errMissing := (Sources{Messages: missing}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 3}, ref)
	_, errNotParty := (Sources{Messages: f}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 4}, ref)
	if errMissing == nil || errNotParty == nil || errMissing.Error() != errNotParty.Error() {
		t.Fatalf("不存在與非對話方要是同一個答案：%v vs %v", errMissing, errNotParty)
	}
	boom := errors.New("boom")
	if _, err := (Sources{Messages: &fakeMessages{err: boom}}).ResolveAuthor(context.Background(), social.Caller{BuyerID: 3}, ref); !errors.Is(err, boom) {
		t.Fatalf("模組錯誤原樣回傳：%v", err)
	}
}

func TestResolveAuthorRequiresExactlyOneReferenceIncludingMessageID(t *testing.T) {
	s := Sources{Messages: &fakeMessages{}}
	if _, err := s.ResolveAuthor(context.Background(), social.Caller{BuyerID: 3}, Ref{}); apperr.KindOf(err) != apperr.KindInvalidInput || !strings.Contains(err.Error(), "message_id") {
		t.Fatalf("都沒設要指出 message_id：%v", err)
	}
	if _, err := s.ResolveAuthor(context.Background(), social.Caller{BuyerID: 3}, Ref{MessageID: uuid.UUID{7}.String(), CommentID: uuid.UUID{8}.String()}); apperr.KindOf(err) != apperr.KindInvalidInput || !strings.Contains(err.Error(), "只能設一個") {
		t.Fatalf("設兩個：%v", err)
	}
	if _, err := s.ResolveAuthor(context.Background(), social.Caller{BuyerID: 3}, Ref{MessageID: "x"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("格式錯誤：%v", err)
	}
}

func TestSourcesCompleteNeedsEveryOwner(t *testing.T) {
	full := Sources{Authors: &fakeAuthors{}, Reviews: &fakeReviews{}, Messages: &fakeMessages{}, Identities: ids()}
	if !full.Complete() {
		t.Fatal("來源齊全")
	}
	for name, s := range map[string]Sources{
		"沒有 Authors":  {Reviews: full.Reviews, Messages: full.Messages, Identities: full.Identities},
		"沒有 Reviews":  {Authors: full.Authors, Messages: full.Messages, Identities: full.Identities},
		"沒有 Messages": {Authors: full.Authors, Reviews: full.Reviews, Identities: full.Identities},
		"沒有身分來源":      {Authors: full.Authors, Reviews: full.Reviews, Messages: full.Messages},
	} {
		if s.Complete() {
			t.Fatalf("%s 要不完整", name)
		}
	}
}
