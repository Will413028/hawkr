package seller

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// testStats 建立測試用的 StatsSource（來源與 logger 都是必要的）。
func testStats(followers FollowerCounts, ratings Ratings) *StatsSource {
	s, err := NewStatsSource(followers, ratings, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	return s
}

type fakePublic struct {
	store    model.PublicStore
	list     model.PublicStorePage
	listErr  error
	listIn   model.ListPublicStoresInput
	storeErr error
	byCode   int64
	cards    []model.StoreCard
	cardIDs  []int64
	viewed   []int64
	viewErr  error
}

func (f *fakePublic) PublicStoreByCode(_ context.Context, code int64) (model.PublicStore, error) {
	f.byCode = code
	return f.store, f.storeErr
}

func (f *fakePublic) ListPublicStores(_ context.Context, in model.ListPublicStoresInput) (model.PublicStorePage, error) {
	f.listIn = in
	return f.list, f.listErr
}

func (f *fakePublic) StoreCards(_ context.Context, ids []int64) ([]model.StoreCard, error) {
	f.cardIDs = ids
	return f.cards, nil
}

func (f *fakePublic) RecordProductView(_ context.Context, id int64) error {
	f.viewed = append(f.viewed, id)
	return f.viewErr
}

type fakeFollowerCounts struct {
	counts map[int64]int64
	err    error
}

func (f fakeFollowerCounts) SubscriberCounts(context.Context, []int64) (map[int64]int64, error) {
	return f.counts, f.err
}

type fakeRatings struct {
	stats map[int64]RatingStats
	err   error
}

func (f fakeRatings) StoreRatingStats(context.Context, []int64) (map[int64]RatingStats, error) {
	return f.stats, f.err
}

var publicUpdateTime = time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC)

func publicStore() model.PublicStore {
	text := func(s string) *string { return &s }
	logo := "stores/logo"
	return model.PublicStore{Store: model.Store{
		Profile: model.Profile{
			StoreName: "範例小農", Description: text("介紹"), TownCode: text("63000050"), ContactPhone: text("0912345678"),
			AddressDetail: text("測試路一號"), Prize: text("優良農民"), WebsiteURL: text("https://farm.example.invalid"),
		},
		StoreID: 11, StoreCode: 2600000001, AccountID: 7, Version: 3, Status: model.Active, LogoKey: &logo,
		ApprovedFoodRegistration: &model.FoodRegistration{RegistrationNumber: "A-123"},
	}, UpdateTime: publicUpdateTime}
}

func newPublic(t *testing.T, m *fakePublic, followers fakeFollowerCounts, ratings fakeRatings) *Public {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return mustPublic(t, m, testStats(followers, ratings), images)
}

func getStore(t *testing.T, h *Public, code string) (*pb.GetStoreResponse, error) {
	t.Helper()
	resp, err := h.GetStore(context.Background(), connect.NewRequest(&pb.GetStoreRequest{StoreCode: code}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func TestGetStoreReadsByStoreCodeAndExposesOnlyThePublicFields(t *testing.T) {
	m := &fakePublic{store: publicStore()}
	h := newPublic(t, m, fakeFollowerCounts{counts: map[int64]int64{11: 5}}, fakeRatings{stats: map[int64]RatingStats{11: {Count: 3, Average: 4.5}}})
	got, err := getStore(t, h, "2600000001")
	if err != nil {
		t.Fatal(err)
	}
	if m.byCode != 2600000001 {
		t.Fatalf("模組收到的 code=%d", m.byCode)
	}
	s := got.GetStore()
	if s.GetId() != "11" || s.GetStoreCode() != "2600000001" || s.GetStoreName() != "範例小農" || s.GetTownCode() != "63000050" ||
		s.GetLogo().GetUriTemplate() != "https://media.example.test/i/{preset}/stores/logo" || s.GetCoverImage() != nil {
		t.Fatalf("store=%v", s)
	}
	if s.GetFoodBusinessRegistrationNumber() != "A-123" || !s.GetUpdateTime().AsTime().Equal(publicUpdateTime) {
		t.Fatalf("registration=%q update_time=%v", s.GetFoodBusinessRegistrationNumber(), s.GetUpdateTime())
	}
	fields := s.ProtoReflect().Descriptor().Fields()
	for _, private := range []string{"contact_phone", "address_detail", "food_business_reg_no", "is_processed_food", "processed", "etag", "account_id"} {
		if fields.ByName(protoName(private)) != nil {
			t.Errorf("PublicStore 不該有 %s", private)
		}
	}
	if st := got.GetStats(); st.FollowerCount == nil || *st.FollowerCount != 5 || st.ReviewCount == nil || *st.ReviewCount != 3 || st.RatingAverage == nil || *st.RatingAverage != 4.5 {
		t.Fatalf("stats=%v", st)
	}
}

// 店面本身讀得到時，粉絲與評價各自讀取失敗不是 0：欄位不設，client 才分得出「沒有」與「讀不到」；另一邊照樣給。
func TestGetStoreStatsDistinguishAFailedReadFromZero(t *testing.T) {
	unavailable := errors.New("upstream unavailable")
	cases := map[string]struct {
		followers                    fakeFollowerCounts
		ratings                      fakeRatings
		wantFollowers, wantReviews   *int32
		wantAverageSet               bool
		wantFollowersSet, wantReview bool
	}{
		"both read, none yet": {
			followers: fakeFollowerCounts{counts: map[int64]int64{11: 0}}, ratings: fakeRatings{stats: map[int64]RatingStats{}},
			wantFollowersSet: true, wantReview: true,
		},
		"followers failed": {
			followers: fakeFollowerCounts{err: unavailable}, ratings: fakeRatings{stats: map[int64]RatingStats{11: {Count: 2, Average: 5}}},
			wantReview: true, wantAverageSet: true,
		},
		"ratings failed": {
			followers: fakeFollowerCounts{counts: map[int64]int64{11: 9}}, ratings: fakeRatings{err: unavailable}, wantFollowersSet: true,
		},
		"both failed": {followers: fakeFollowerCounts{err: unavailable}, ratings: fakeRatings{err: unavailable}},
		"followers answer without this store": {
			followers: fakeFollowerCounts{counts: map[int64]int64{}}, ratings: fakeRatings{stats: map[int64]RatingStats{}}, wantReview: true,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := newPublic(t, &fakePublic{store: publicStore()}, c.followers, c.ratings)
			got, err := getStore(t, h, "2600000001")
			if err != nil {
				t.Fatalf("統計讀不到不該讓店面失敗：%v", err)
			}
			st := got.GetStats()
			if (st.FollowerCount != nil) != c.wantFollowersSet || (st.ReviewCount != nil) != c.wantReview || (st.RatingAverage != nil) != c.wantAverageSet {
				t.Fatalf("stats=%v, want followers set=%v reviews set=%v average set=%v", st, c.wantFollowersSet, c.wantReview, c.wantAverageSet)
			}
			if st.ReviewCount != nil && c.wantReview && !c.wantAverageSet && *st.ReviewCount != 0 {
				t.Fatalf("沒有評價時 review_count 是 0：%v", st)
			}
		})
	}
}

func TestGetStoreRejectsAMalformedCodeAndPassesNotFoundThrough(t *testing.T) {
	m := &fakePublic{store: publicStore(), storeErr: apperr.NotFound("找不到店家")}
	h := newPublic(t, m, fakeFollowerCounts{}, fakeRatings{})
	for _, bad := range []string{"", "abc", "-5", "0", "12.5", "99999999999999999999"} {
		if _, err := getStore(t, h, bad); apperr.KindOf(err) != apperr.KindInvalidInput && !isInvalidArgument(err) {
			t.Errorf("store_code %q err=%v, want invalid argument", bad, err)
		}
	}
	if m.byCode != 0 {
		t.Fatalf("格式不對的 code 不該送到模組：%d", m.byCode)
	}
	if _, err := getStore(t, h, "2600000001"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("err=%v, want NotFound", err)
	}
}

func isInvalidArgument(err error) bool {
	var ce *connect.Error
	return errors.As(err, &ce) && ce.Code() == connect.CodeInvalidArgument
}

func TestBatchGetStoresMapsCardsAndLeavesAPurgedNameUnset(t *testing.T) {
	name, logo, cover := "範例小農", "stores/logo", "stores/cover"
	m := &fakePublic{cards: []model.StoreCard{
		{StoreID: 3, StoreCode: 2600000003, Name: &name, LogoKey: &logo, CoverKey: &cover, Status: model.CardOpen},
		{StoreID: 1, StoreCode: 2600000001, Status: model.CardClosed},
		{StoreID: 2, StoreCode: 2600000002, Name: &name, Status: model.CardPaused},
	}}
	h := newPublic(t, m, fakeFollowerCounts{}, fakeRatings{})
	resp, err := h.BatchGetStores(context.Background(), connect.NewRequest(&pb.BatchGetStoresRequest{Ids: []string{"3", "1", "2"}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.cardIDs) != 3 || m.cardIDs[0] != 3 || m.cardIDs[1] != 1 || m.cardIDs[2] != 2 {
		t.Fatalf("模組收到的 ids=%v", m.cardIDs)
	}
	got := resp.Msg.GetStores()
	if len(got) != 3 || got[0].GetId() != "3" || got[0].GetName() != "範例小農" || got[0].GetStatus() != commonpb.StoreCardStatus_STORE_CARD_STATUS_OPEN ||
		got[0].GetLogo().GetUriTemplate() != "https://media.example.test/i/{preset}/stores/logo" ||
		got[0].GetCoverImage().GetUriTemplate() != "https://media.example.test/i/{preset}/stores/cover" {
		t.Fatalf("cards=%v", got)
	}
	if got[1].Name != nil || got[1].GetStatus() != commonpb.StoreCardStatus_STORE_CARD_STATUS_CLOSED || got[1].GetLogo() != nil {
		t.Fatalf("個資清除的卡片不該有店名：%v", got[1])
	}
	if got[2].GetStatus() != commonpb.StoreCardStatus_STORE_CARD_STATUS_PAUSED {
		t.Fatalf("paused=%v", got[2])
	}
}

func TestBatchGetStoresValidatesIDs(t *testing.T) {
	h := newPublic(t, &fakePublic{}, fakeFollowerCounts{}, fakeRatings{})
	tooMany := make([]string, model.MaxBatchSize+1)
	for i := range tooMany {
		tooMany[i] = "1"
	}
	for name, ids := range map[string][]string{"empty": nil, "too many": tooMany, "not a number": {"1", "x"}, "non-positive": {"0"}} {
		_, err := h.BatchGetStores(context.Background(), connect.NewRequest(&pb.BatchGetStoresRequest{Ids: ids}))
		if !isInvalidArgument(err) && apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s: err=%v, want invalid argument", name, err)
		}
	}
}

func TestStoreCardStatusValuesMatchTheContract(t *testing.T) {
	if got := rpc.EnumMismatches[commonpb.StoreCardStatus](model.CardStatusValues()); len(got) != 0 {
		t.Fatal(strings.Join(got, "\n"))
	}
}

func viewCtx(signedIn bool) context.Context {
	if !signedIn {
		return context.Background()
	}
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
}

// 只計訪客：登入的呼叫者收到成功但不計數（買家的瀏覽由 buyer 的 ProductViewed 計入）。
func TestRecordProductViewCountsOnlyGuests(t *testing.T) {
	for name, c := range map[string]struct {
		signedIn bool
		want     int
	}{"guest": {false, 1}, "signed in": {true, 0}} {
		t.Run(name, func(t *testing.T) {
			m := &fakePublic{}
			h := newPublic(t, m, fakeFollowerCounts{}, fakeRatings{})
			if _, err := h.RecordProductView(viewCtx(c.signedIn), connect.NewRequest(&pb.RecordProductViewRequest{ProductId: "42"})); err != nil {
				t.Fatal(err)
			}
			if len(m.viewed) != c.want || (c.want == 1 && m.viewed[0] != 42) {
				t.Fatalf("viewed=%v, want %d", m.viewed, c.want)
			}
		})
	}
}

func TestRecordProductViewRejectsABadIDAndReportsWriteFailures(t *testing.T) {
	m := &fakePublic{viewErr: errors.New("db down")}
	h := newPublic(t, m, fakeFollowerCounts{}, fakeRatings{})
	for _, bad := range []string{"", "x", "0", "-1"} {
		if _, err := h.RecordProductView(viewCtx(false), connect.NewRequest(&pb.RecordProductViewRequest{ProductId: bad})); !isInvalidArgument(err) && apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("product_id %q err=%v", bad, err)
		}
	}
	if len(m.viewed) != 0 {
		t.Fatalf("不合法的 id 不該寫入：%v", m.viewed)
	}
	if _, err := h.RecordProductView(viewCtx(false), connect.NewRequest(&pb.RecordProductViewRequest{ProductId: "5"})); err == nil {
		t.Fatal("寫入失敗要回錯誤")
	}
}

func protoName(s string) protoreflect.Name { return protoreflect.Name(s) }

// ratingsThatHang 開始讀取之後一直等到 ctx 結束；deadline 記下它收到的時間上限。
type ratingsThatHang struct {
	started  chan struct{}
	deadline chan time.Duration
}

func (r ratingsThatHang) StoreRatingStats(ctx context.Context, _ []int64) (map[int64]RatingStats, error) {
	if d, ok := ctx.Deadline(); ok {
		r.deadline <- time.Until(d)
	}
	close(r.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

// followersAfter 等到評價的讀取已經開始才回答：兩項統計是同時讀的，序列讀取時它等不到而逾時。
type followersAfter struct{ started chan struct{} }

func (f followersAfter) SubscriberCounts(ctx context.Context, _ []int64) (map[int64]int64, error) {
	select {
	case <-f.started:
		return map[int64]int64{11: 4}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// 一項統計很慢時店面不被它拖住：每項統計有自己的 statsBudget（deadline），逾時只讓那一項不設，另一項照樣給；兩項同時讀。
func TestGetStoreStatsAreReadInParallelEachUnderItsOwnBudget(t *testing.T) {
	started := make(chan struct{})
	ratings := ratingsThatHang{started: started, deadline: make(chan time.Duration, 1)}
	h := mustPublic(t, &fakePublic{store: publicStore()}, testStats(followersAfter{started}, ratings), rpc.Images{})
	got, err := getStore(t, h, "2600000001")
	if err != nil {
		t.Fatal(err)
	}
	if budget := <-ratings.deadline; budget <= 0 || budget > statsBudget {
		t.Fatalf("評價讀取收到的時間上限=%s，預期不超過 %s", budget, statsBudget)
	}
	st := got.GetStats()
	if st.FollowerCount == nil || *st.FollowerCount != 4 || st.ReviewCount != nil || st.RatingAverage != nil {
		t.Fatalf("stats=%v, want followers set and the hung ratings unset", st)
	}
	if got.GetStore().GetId() != "11" {
		t.Fatalf("店面照樣回傳：%v", got.GetStore())
	}
}

// 四份入駐文件都回傳，依契約的數字順序，版本就是模組常數表的現行值，網址一律是絕對網址（站內路徑接在網站網域後面，本來就是絕對的原樣）；不需要登入（ctx 沒有呼叫者），也不讀店家。
func TestGetSellerAgreementsReturnsTheCurrentVersionsInProtoOrder(t *testing.T) {
	h, err := NewPublic(&fakePublic{}, testStats(fakeFollowerCounts{}, fakeRatings{}), rpc.Images{}, mustWebOrigin(t))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := h.GetSellerAgreements(context.Background(), connect.NewRequest(&pb.GetSellerAgreementsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetAgreements()
	wantKinds := []pb.AgreementKind{
		pb.AgreementKind_AGREEMENT_KIND_SELLER_TERMS,
		pb.AgreementKind_AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
		pb.AgreementKind_AGREEMENT_KIND_PAYUNI_TERMS,
		pb.AgreementKind_AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE,
	}
	if len(got) != len(wantKinds) {
		t.Fatalf("agreements=%v", got)
	}
	for i, a := range got {
		if a.GetKind() != wantKinds[i] {
			t.Errorf("[%d] kind=%s，預期 %s", i, a.GetKind(), wantKinds[i])
		}
		cur, ok := model.CurrentAgreement(rpc.EnumFromProto[model.AgreementKind](a.GetKind()))
		if !ok || a.GetVersion() != cur.Version || a.GetUrl() != mustAgreementURL(t, cur.URL) || a.GetVersion() == "" {
			t.Errorf("[%d] %v 與常數表 %+v 不一致", i, a, cur)
		}
		if !strings.HasPrefix(a.GetUrl(), "https://") {
			t.Errorf("[%d] url=%q 不是絕對網址", i, a.GetUrl())
		}
	}
	want := map[pb.AgreementKind]string{
		pb.AgreementKind_AGREEMENT_KIND_SELLER_TERMS: testWebOrigin + "/seller-terms",
		pb.AgreementKind_AGREEMENT_KIND_PAYUNI_TERMS: "https://www.payuni.com.tw/terms",
	}
	for _, a := range got {
		if w, ok := want[a.GetKind()]; ok && a.GetUrl() != w {
			t.Errorf("%s url=%q，預期 %q", a.GetKind(), a.GetUrl(), w)
		}
	}
}

func TestListStoresMapsThePublicFieldsAndPassesPagingToTheModule(t *testing.T) {
	second := publicStore()
	second.StoreID, second.StoreCode = 12, 2600000002
	second.ApprovedFoodRegistration = nil
	m := &fakePublic{list: model.PublicStorePage{Items: []model.PublicStore{publicStore(), second}, Next: &model.PublicStoreCursor{StoreID: 12}}}
	h := newPublic(t, m, fakeFollowerCounts{}, fakeRatings{})
	resp, err := h.ListStores(context.Background(), connect.NewRequest(&pb.ListStoresRequest{PageSize: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if m.listIn.PageSize != 2 || m.listIn.After != nil {
		t.Fatalf("模組收到 %+v", m.listIn)
	}
	got := resp.Msg.GetStores()
	if len(got) != 2 || got[0].GetId() != "11" || got[1].GetId() != "12" || got[0].GetFoodBusinessRegistrationNumber() != "A-123" ||
		got[1].GetFoodBusinessRegistrationNumber() != "" || !got[0].GetUpdateTime().AsTime().Equal(publicUpdateTime) {
		t.Fatalf("stores=%v", got)
	}
	if resp.Msg.GetNextPageToken() == "" {
		t.Fatal("還有下一頁時要有 next_page_token")
	}
	for _, store := range got {
		fields := store.ProtoReflect().Descriptor().Fields()
		for _, private := range []string{"contact_phone", "address_detail", "is_processed_food", "processed", "etag"} {
			if fields.ByName(protoName(private)) != nil {
				t.Errorf("PublicStore 不該有 %s", private)
			}
		}
	}

	next := &fakePublic{list: model.PublicStorePage{}}
	h = newPublic(t, next, fakeFollowerCounts{}, fakeRatings{})
	resp2, err := h.ListStores(context.Background(), connect.NewRequest(&pb.ListStoresRequest{PageToken: resp.Msg.GetNextPageToken()}))
	if err != nil {
		t.Fatal(err)
	}
	if next.listIn.After == nil || next.listIn.After.StoreID != 12 || resp2.Msg.GetNextPageToken() != "" || len(resp2.Msg.GetStores()) != 0 {
		t.Fatalf("after=%+v response=%v", next.listIn.After, resp2.Msg)
	}
}

func TestListStoresRejectsAnUnusablePageTokenWithoutCallingTheModule(t *testing.T) {
	for name, token := range map[string]string{"not a token": "garbage", "token of another query": rpc.TokenFor("another", model.PublicStoreCursor{StoreID: 5}), "zero cursor": rpc.TokenFor(rpc.FilterScope(sellerv1alpha1connect.SellerPublicServiceListStoresProcedure), model.PublicStoreCursor{})} {
		t.Run(name, func(t *testing.T) {
			m := &fakePublic{}
			h := newPublic(t, m, fakeFollowerCounts{}, fakeRatings{})
			_, err := h.ListStores(context.Background(), connect.NewRequest(&pb.ListStoresRequest{PageToken: token}))
			if err == nil || !strings.HasPrefix(err.Error(), connect.CodeInvalidArgument.String()) {
				t.Fatalf("err=%v", err)
			}
			if m.listIn != (model.ListPublicStoresInput{}) {
				t.Fatalf("不該呼叫模組：%+v", m.listIn)
			}
		})
	}
}

func TestListStoresPassesModuleFailuresThrough(t *testing.T) {
	h := newPublic(t, &fakePublic{listErr: apperr.InvalidInput("筆數不可為負數")}, fakeFollowerCounts{}, fakeRatings{})
	_, err := h.ListStores(context.Background(), connect.NewRequest(&pb.ListStoresRequest{PageSize: -1}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
}
