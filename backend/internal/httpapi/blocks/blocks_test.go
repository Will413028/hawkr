package blocks

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
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
	if _, err := ResolveAuthor(context.Background(), a, social.Caller{}, Ref{CommentID: id.String()}); err != nil || a.called.Kind != social.TargetComment || a.called.ID != id {
		t.Fatalf("留言：%v %+v", err, a.called)
	}
	if _, err := ResolveAuthor(context.Background(), a, social.Caller{}, Ref{ForumEntryID: id.String()}); err != nil || a.called.Kind != social.TargetForumEntry {
		t.Fatalf("討論區：%v %+v", err, a.called)
	}
}

func TestResolveAuthorMapsAnUnreadableReferenceToNotFound(t *testing.T) {
	a := &fakeAuthors{}
	_, err := ResolveAuthor(context.Background(), a, social.Caller{}, Ref{CommentID: uuid.UUID{1}.String()})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("讀不到要 not_found：%v", err)
	}
}

func TestResolveAuthorRejectsAnAmbiguousOrEmptyReference(t *testing.T) {
	a := &fakeAuthors{}
	id := uuid.UUID{1}.String()
	for name, ref := range map[string]Ref{"都沒設": {}, "都設了": {CommentID: id, ForumEntryID: id}, "格式錯誤": {CommentID: "x"}} {
		if _, err := ResolveAuthor(context.Background(), a, social.Caller{}, ref); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s要 invalid_argument：%v", name, err)
		}
	}
	boom := errors.New("boom")
	a.err = boom
	if _, err := ResolveAuthor(context.Background(), a, social.Caller{}, Ref{CommentID: id}); !errors.Is(err, boom) {
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
