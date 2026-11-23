//go:build integration

package quote_test

import (
	"slices"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func byID(rs []model.BuyerSellerResponse) map[uuid.UUID]model.BuyerSellerResponse {
	out := map[uuid.UUID]model.BuyerSellerResponse{}
	for _, r := range rs {
		out[r.ID] = r
	}
	return out
}

func TestListShowsEachSellersCurrentVersionWithStatusAcceptabilityAndRange(t *testing.T) {
	f := newFixture(t)
	past := start.Add(-time.Hour)
	active := f.respond(f.publication, 11, version{goods: 400}) // 3 KG × 100~200 = 300~600：區間內
	expired := f.respond(f.publication, 12, version{goods: 400, validUntil: past})
	withdrawn := f.respond(f.publication, 13, version{goods: 400})
	f.exec(`UPDATE procurement.seller_responses SET withdrawn_version = 1, withdrawn_at = $2 WHERE id = $1`, withdrawn, start)
	superseded := f.respond(f.publication, 14, version{goods: 400}, version{goods: 700}) // 目前版是 2，貨款 700 超出區間
	closed := f.respond(f.publication, 15, version{goods: 400})
	f.exec(`UPDATE procurement.seller_responses SET write_closed_at = $2, write_closed_reason = 'DECLINED' WHERE id = $1`, closed, start)
	low := f.respond(f.publication, 16, version{goods: 299})
	otherUnit := f.respond(f.publication, 17, version{goods: 1, unit: "G", quantity: "4"})
	fractional := f.respond(f.publication, 19, version{goods: 250, quantity: "2.5"}) // 2.5 KG × 100~200 = 250~500：不是整數倍也比較
	fractionalLow := f.respond(f.publication, 20, version{goods: 249, quantity: "2.5"})
	noVersions := uuid.NewV7() // 還沒送出任何版本的回價身分列不列
	f.exec(`INSERT INTO procurement.seller_responses (id, publication_id, request_id, store_id, created_at, updated_at) VALUES ($1, $2, $3, 18, $4, $4)`, noVersions, f.publication, f.request, start)

	page, err := f.service().List(t.Context(), f.listIn())
	must(t, err)
	if page.NextPageToken != "" || len(page.Responses) != 9 {
		t.Fatalf("要 9 筆（沒有版本的不列）且沒有下一頁：%d %q", len(page.Responses), page.NextPageToken)
	}
	got := byID(page.Responses)
	for name, c := range map[string]struct {
		id            uuid.UUID
		version       int32
		status        string
		acceptability string
		outside       bool
	}{
		"進行中":     {active, 1, model.ResponseActive, model.AcceptabilityAcceptable, false},
		"過期":      {expired, 1, model.ResponseExpired, model.AcceptabilityExpired, false},
		"撤回":      {withdrawn, 1, model.ResponseWithdrawn, model.AcceptabilityWithdrawn, false},
		"只列目前版本":  {superseded, 2, model.ResponseActive, model.AcceptabilityAcceptable, true},
		"賣家停止寫入":  {closed, 1, model.ResponseActive, model.AcceptabilityIneligible, false},
		"低於區間":    {low, 1, model.ResponseActive, model.AcceptabilityAcceptable, true},
		"報價單位不同":  {otherUnit, 1, model.ResponseActive, model.AcceptabilityAcceptable, false},
		"小數數量區間內": {fractional, 1, model.ResponseActive, model.AcceptabilityAcceptable, false},
		"小數數量低於":  {fractionalLow, 1, model.ResponseActive, model.AcceptabilityAcceptable, true},
	} {
		r, ok := got[c.id]
		if !ok {
			t.Fatalf("%s：沒有列出", name)
		}
		if r.Version != c.version || r.Status != c.status || r.Acceptability != c.acceptability || r.OutsideBuyerRequestedRange != c.outside || len(r.PayloadHash) != 64 {
			t.Errorf("%s：版本 %d、狀態 %s、可否接受 %s、超出區間 %v、hash %q", name, r.Version, r.Status, r.Acceptability, r.OutsideBuyerRequestedRange, r.PayloadHash)
		}
	}
	if q := got[active].Quote; q.UnitBasisQuantity != "1.000" || q.OfferedQuantity != "3.000" || q.GoodsAmount != 400 || q.TotalAmount != 415 || q.Currency != "TWD" {
		t.Errorf("報價內容 = %+v", q)
	}

	// 這張需求有進行中的成交之後，進行中的版本不能再接受。
	_, err = f.service().Accept(t.Context(), f.acceptIn(active, 1))
	must(t, err)
	page, err = f.service().List(t.Context(), f.listIn())
	must(t, err)
	got = byID(page.Responses)
	if got[low].Acceptability != model.AcceptabilityIneligible || got[expired].Acceptability != model.AcceptabilityExpired {
		t.Errorf("成交之後：低價那筆 %s、過期那筆 %s", got[low].Acceptability, got[expired].Acceptability)
	}
}

func TestListMarksTheShortlistOnlyOnTheVersionThatIsInIt(t *testing.T) {
	f := newFixture(t)
	two := f.respond(f.publication, 11, version{goods: 400}, version{goods: 450})
	other := f.respond(f.publication, 12, version{goods: 400})
	_, err := f.service().PutShortlist(t.Context(), f.buyer, f.request, two, 1)
	must(t, err)
	_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, other, 1)
	must(t, err)
	page, err := f.service().List(t.Context(), f.listIn())
	must(t, err)
	got := byID(page.Responses)
	if got[two].Shortlisted || !got[other].Shortlisted {
		t.Fatalf("入圍清單掛的是舊版本時，目前版本不標記：%v %v", got[two].Shortlisted, got[other].Shortlisted)
	}
	_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, two, 2)
	must(t, err)
	page, _ = f.service().List(t.Context(), f.listIn())
	if !byID(page.Responses)[two].Shortlisted {
		t.Fatal("換成目前版本之後要標記")
	}
}

func TestListPaginatesByIdentityCreationTimeThenIDWithoutDuplicatesOrGaps(t *testing.T) {
	f := newFixture(t)
	type row struct {
		id uuid.UUID
		at time.Time
	}
	var rows []row
	for i := range 7 {
		at := start.Add(time.Duration(i/2) * time.Minute) // 兩兩同時間，靠回價 ID 排序
		id := f.respondAt(at, f.publication, int64(100+i), version{goods: 400})
		rows = append(rows, row{id, at})
	}
	slices.SortFunc(rows, func(a, b row) int {
		if c := b.at.Compare(a.at); c != 0 {
			return c
		}
		return b.id.Compare(a.id)
	})
	var want []uuid.UUID
	for _, r := range rows {
		want = append(want, r.id)
	}
	var got []uuid.UUID
	in := f.listIn()
	in.PageSize = 3
	pages := 0
	maxPages := (len(want)+int(in.PageSize)-1)/int(in.PageSize) + 2
	seen := map[uuid.UUID]bool{}
	for {
		page, err := f.service().List(t.Context(), in)
		must(t, err)
		pages++
		if pages > maxPages {
			t.Fatalf("翻頁超過 %d 頁仍有下一頁，已取得 %v", maxPages, got)
		}
		for _, r := range page.Responses {
			if seen[r.ID] {
				t.Fatalf("第 %d 頁出現重複的回價 %v", pages, r.ID)
			}
			seen[r.ID] = true
			got = append(got, r.ID)
		}
		if page.NextPageToken == "" {
			break
		}
		in.PageToken = page.NextPageToken
	}
	if !slices.Equal(got, want) || pages != 3 {
		t.Fatalf("翻頁（%d 頁）要依 (建立時間 DESC, ID DESC) 不漏不重：\n得到 %v\n要   %v", pages, got, want)
	}
}

func TestListPaginationSurvivesASellerRevisingBetweenPages(t *testing.T) {
	f := newFixture(t)
	var ids []uuid.UUID
	for i := range 4 {
		ids = append(ids, f.respondAt(start.Add(time.Duration(i)*time.Minute), f.publication, int64(100+i), version{goods: 400}))
	}
	// 回價依建立時間由新到舊：ids[3]、ids[2]、ids[1]、ids[0]。
	in := f.listIn()
	in.PageSize = 2
	first, err := f.service().List(t.Context(), in)
	must(t, err)
	if len(first.Responses) != 2 || first.Responses[0].ID != ids[3] || first.Responses[1].ID != ids[2] || first.NextPageToken == "" {
		t.Fatalf("第一頁 = %+v", first.Responses)
	}
	// 第二頁還沒取之前，後面那一則回價改版，新版本的送出時間晚於其他所有版本。
	f.exec(`UPDATE procurement.seller_responses SET latest_version = 2 WHERE id = $1`, ids[0])
	f.exec(`INSERT INTO procurement.seller_response_versions (
			id, seller_response_id, version, response_type, offered_quality_grade, offered_quality_inspection_notes, unit_basis_quantity, unit_basis_unit,
			offered_quantity, offered_quantity_unit, conversion_basis, currency, goods_amount, shipping_amount, tax_amount, tax_inclusion_basis, total_amount,
			valid_until, outside_buyer_requested_range, publication_version, client_request_id, submitted_at, created_at)
		SELECT $2, seller_response_id, 2, response_type, offered_quality_grade, offered_quality_inspection_notes, unit_basis_quantity, unit_basis_unit,
			offered_quantity, offered_quantity_unit, conversion_basis, currency, goods_amount, shipping_amount, tax_amount, tax_inclusion_basis, total_amount,
			valid_until, outside_buyer_requested_range, publication_version, $3, $4, $4
		FROM procurement.seller_response_versions WHERE seller_response_id = $1 AND version = 1`, ids[0], uuid.NewV7(), uuid.NewV7(), start.Add(time.Hour))
	in.PageToken = first.NextPageToken
	second, err := f.service().List(t.Context(), in)
	must(t, err)
	if len(second.Responses) != 2 || second.Responses[0].ID != ids[1] || second.Responses[1].ID != ids[0] || second.Responses[1].Version != 2 || second.NextPageToken != "" {
		t.Fatalf("改版的那一則仍要在第二頁（版本 2）：%+v", second.Responses)
	}
}

func TestListPageSizeBoundsAndTokens(t *testing.T) {
	f := newFixture(t)
	for i := range 101 {
		f.respond(f.publication, int64(1000+i), version{goods: 400, submitted: start.Add(time.Duration(i) * time.Second)})
	}
	for name, c := range map[string]struct {
		size int32
		want int
	}{"0 用預設 50": {0, 50}, "超過 100 以 100 計": {101, 100}, "剛好 100": {100, 100}, "1": {1, 1}} {
		in := f.listIn()
		in.PageSize = c.size
		page, err := f.service().List(t.Context(), in)
		must(t, err)
		if len(page.Responses) != c.want || page.NextPageToken == "" {
			t.Errorf("%s：%d 筆、token %q，要 %d 筆且還有下一頁", name, len(page.Responses), page.NextPageToken, c.want)
		}
	}
	in := f.listIn()
	in.PageSize = -1
	_, err := f.service().List(t.Context(), in)
	wantInvalid(t, err, "page_size")
	for _, bad := range []string{"x", "!!!", "e30"} {
		in = f.listIn()
		in.PageToken = bad
		_, err = f.service().List(t.Context(), in)
		wantInvalid(t, err, "page_token")
	}
}

func TestListOnlyShowsTheRequestedPublicationOfTheBuyersOwnRequest(t *testing.T) {
	f := newFixture(t)
	first := f.respond(f.publication, 11, version{goods: 400})
	second, _ := f.publish()
	other := f.respond(second, 12, version{goods: 400})

	page, err := f.service().List(t.Context(), f.listIn())
	must(t, err)
	if len(page.Responses) != 1 || page.Responses[0].ID != first {
		t.Fatalf("只列這次發布的回價：%+v", page.Responses)
	}
	in := f.listIn()
	in.PublicationID = second
	page, err = f.service().List(t.Context(), in)
	must(t, err)
	if len(page.Responses) != 1 || page.Responses[0].ID != other {
		t.Fatalf("另一次發布：%+v", page.Responses)
	}

	stranger := f.listIn()
	stranger.BuyerID = 99
	_, err = f.service().List(t.Context(), stranger)
	wantKind(t, err, apperr.KindNotFound)
	unknown := f.listIn()
	unknown.PublicationID = uuid.NewV7()
	_, err = f.service().List(t.Context(), unknown)
	wantKind(t, err, apperr.KindNotFound)
	wrongRequest := f.listIn()
	wrongRequest.RequestID = uuid.NewV7()
	_, err = f.service().List(t.Context(), wrongRequest)
	wantKind(t, err, apperr.KindNotFound)

	// 別的買家的需求與發布，同一個資料庫裡也看不到。
	g := f.sibling(2)
	g.respond(g.publication, 11, version{goods: 400})
	cross := g.listIn()
	cross.BuyerID = f.buyer
	_, err = f.service().List(t.Context(), cross)
	wantKind(t, err, apperr.KindNotFound)
	crossRequest := f.listIn()
	crossRequest.PublicationID = g.publication
	_, err = f.service().List(t.Context(), crossRequest)
	wantKind(t, err, apperr.KindNotFound)
}
