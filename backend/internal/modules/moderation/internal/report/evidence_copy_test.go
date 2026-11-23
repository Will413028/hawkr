package report

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeMedia struct {
	copied    []uuid.UUID
	confirmed []string
	uploaders []media.Uploader
	copyErr   error
	confirms  map[string]uuid.UUID
	keys      map[uuid.UUID]string
}

func (f *fakeMedia) CopyPublic(_ context.Context, u media.Uploader, src uuid.UUID) (media.PutPublicResult, error) {
	if f.copyErr != nil {
		return media.PutPublicResult{}, f.copyErr
	}
	f.copied = append(f.copied, src)
	f.uploaders = append(f.uploaders, u)
	return media.PutPublicResult{Token: "tok-" + src.String()}, nil
}

func (f *fakeMedia) Confirm(_ context.Context, u media.Uploader, token string, _ media.Visibility) (media.ConfirmedAsset, error) {
	f.confirmed = append(f.confirmed, token)
	f.uploaders = append(f.uploaders, u)
	id, ok := f.confirms[token]
	if !ok {
		id = uuid.NewV7()
	}
	return media.ConfirmedAsset{ID: id, ObjectKey: f.keys[id]}, nil
}

func testService(m Media) *Service {
	return &Service{d: Deps{Media: m}, accountOf: func(_ context.Context, in model.SubmitInput) (int64, error) {
		if in.ReporterStoreID > 0 {
			return in.ReporterStoreID + 100, nil
		}
		return in.ReporterBuyerID + 100, nil
	}}
}

func post(refs ...string) snapshot.Content {
	return snapshot.Content{Kind: model.TargetPost, Social: &snapshot.Social{AuthorKind: "STORE", AuthorID: 1, StoreID: 1, Body: "x", MediaRefs: refs}}
}

func TestOwnCopiesAsksForTheMissingSourcesBeforeConfirmingAnything(t *testing.T) {
	m := &fakeMedia{}
	s := testService(m)
	a, b := uuid.NewV7().String(), uuid.NewV7().String()
	_, _, err := s.ownCopies(t.Context(), model.SubmitInput{ReporterBuyerID: 5}, post(a, b), []string{a, b}, map[string]string{a: "tok-a"})
	var need needCopies
	if !errors.As(err, &need) || !slices.Equal(need.sources, []string{b}) {
		t.Fatalf("err = %v，預期只缺 %s", err, b)
	}
	if len(m.confirmed) != 0 {
		t.Fatalf("複本沒備齊就不能 Confirm：%v", m.confirmed)
	}
}

func TestOwnCopiesConfirmsEveryCopyAndSwapsTheReferences(t *testing.T) {
	a, b := uuid.NewV7(), uuid.NewV7()
	ca, cb := uuid.NewV7(), uuid.NewV7()
	m := &fakeMedia{confirms: map[string]uuid.UUID{"tok-a": ca, "tok-b": cb}}
	s := testService(m)
	content := post(a.String(), b.String())
	got, owned, err := s.ownCopies(t.Context(), model.SubmitInput{ReporterStoreID: 9}, content, []string{a.String(), b.String()}, map[string]string{a.String(): "tok-a", b.String(): "tok-b"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{ca.String(), cb.String()}; !slices.Equal(slices.Sorted(slices.Values(got.AssetIDs())), slices.Sorted(slices.Values(want))) {
		t.Fatalf("快照的圖片 = %v，預期 %v", got.AssetIDs(), want)
	}
	if len(owned) != 2 || !slices.Contains(owned, ca) || !slices.Contains(owned, cb) {
		t.Fatalf("owned = %v", owned)
	}
	if !slices.Equal(content.AssetIDs(), slices.Sorted(slices.Values([]string{a.String(), b.String()}))) {
		t.Fatalf("傳入的內容被改動：%v", content.AssetIDs())
	}
	for _, u := range m.uploaders {
		if u != (media.Uploader{ID: 109}) {
			t.Fatalf("上傳者 = %+v，預期店家檢舉人本人", u)
		}
	}
}

func TestOwnCopiesPreservesTheBuyerAvatarLikeAnyOtherEvidenceImage(t *testing.T) {
	src, copyID := uuid.NewV7(), uuid.NewV7()
	m := &fakeMedia{confirms: map[string]uuid.UUID{"tok-a": copyID}, keys: map[uuid.UUID]string{copyID: "public/copy.jpg"}}
	s := testService(m)
	content := snapshot.Content{Kind: model.TargetBuyer, Buyer: &snapshot.Buyer{DisplayName: "小明", AvatarAssetID: src.String()}}
	sources := content.AssetIDs()
	if !slices.Equal(sources, []string{src.String()}) {
		t.Fatalf("頭像要進證據保全：%v", sources)
	}
	got, owned, err := s.ownCopies(t.Context(), model.SubmitInput{ReporterBuyerID: 5}, content, sources, map[string]string{src.String(): "tok-a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Buyer.AvatarAssetID != copyID.String() || !slices.Equal(owned, []uuid.UUID{copyID}) {
		t.Fatalf("buyer = %+v owned = %v", got.Buyer, owned)
	}
	if content.Buyer.AvatarAssetID != src.String() {
		t.Fatalf("傳入的內容被改動：%+v", content.Buyer)
	}
}

func TestCopySourcesCopiesEachSourceOnceAndKeepsEarlierCopies(t *testing.T) {
	m := &fakeMedia{}
	s := testService(m)
	a, b := uuid.NewV7(), uuid.NewV7()
	copies := map[string]string{a.String(): "old"}
	if err := s.copySources(t.Context(), model.SubmitInput{ReporterBuyerID: 5}, []string{a.String(), b.String()}, copies); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.copied, []uuid.UUID{b}) || copies[a.String()] != "old" || copies[b.String()] != "tok-"+b.String() {
		t.Fatalf("copied = %v，copies = %v", m.copied, copies)
	}
	if m.uploaders[0] != (media.Uploader{ID: 105}) {
		t.Fatalf("買家檢舉人的上傳者 = %+v", m.uploaders[0])
	}
}

func TestCopySourcesPropagatesTheErrorKindAndRejectsNonUUIDs(t *testing.T) {
	s := testService(&fakeMedia{copyErr: apperr.NotFound("gone")})
	err := s.copySources(t.Context(), model.SubmitInput{ReporterBuyerID: 5}, []string{uuid.NewV7().String()}, map[string]string{})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("來源消失要保持 NotFound 讓呼叫方重來：%v", err)
	}
	s = testService(&fakeMedia{})
	if err := s.copySources(t.Context(), model.SubmitInput{ReporterBuyerID: 5}, []string{"not-a-uuid"}, map[string]string{}); err == nil || apperr.KindOf(err) == apperr.KindNotFound {
		t.Fatalf("不是 UUID 的資產 ID 是資料錯誤：%v", err)
	}
}

func TestCopyRoundsExhaustedReportsAGoneSourceAsNotFoundAndOtherwiseAPlainError(t *testing.T) {
	if err := copyRoundsExhausted(t.Context(), true); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("來源圖片已刪除要回 NotFound：%v", err)
	}
	if err := copyRoundsExhausted(t.Context(), false); err == nil || apperr.KindOf(err) == apperr.KindNotFound {
		t.Fatalf("內容一直在變動不是 NotFound：%v", err)
	}
}
