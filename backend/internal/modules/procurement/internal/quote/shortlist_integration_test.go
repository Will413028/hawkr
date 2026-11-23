//go:build integration

package quote_test

import (
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (f *fixture) entries(response uuid.UUID) []int32 {
	f.t.Helper()
	rows, err := f.d.Pool().Query(f.t.Context(), `SELECT seller_response_version FROM procurement.buyer_shortlist_entries WHERE seller_response_id = $1`, response)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var out []int32
	for rows.Next() {
		var v int32
		must(f.t, rows.Scan(&v))
		out = append(out, v)
	}
	must(f.t, rows.Err())
	return out
}

func TestPutShortlistOverwritesTheVersionOfTheSameResponse(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400}, version{goods: 450})
	other := f.respond(f.publication, 12, version{goods: 400})

	e, err := f.service().PutShortlist(t.Context(), f.buyer, f.request, r, 1)
	must(t, err)
	if e.SellerResponseID != r || e.Version != 1 || !e.Present {
		t.Fatalf("第一次 = %+v", e)
	}
	e, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, r, 1)
	must(t, err)
	if e.Version != 1 || len(f.entries(r)) != 1 {
		t.Fatalf("重送同一個版本：%+v %v", e, f.entries(r))
	}
	e, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, r, 2)
	must(t, err)
	if e.Version != 2 || !e.Present || len(f.entries(r)) != 1 || f.entries(r)[0] != 2 {
		t.Fatalf("同一則回價換版本要覆蓋舊的並回新版本：%+v %v", e, f.entries(r))
	}
	if _, err := f.service().PutShortlist(t.Context(), f.buyer, f.request, other, 1); err != nil {
		t.Fatal(err)
	}
	if f.count("procurement.buyer_shortlist_entries WHERE request_id = $1", f.request) != 2 {
		t.Fatal("不同回價各一筆")
	}
}

func TestPutShortlistChecksOwnershipOfTheResponseAndTheVersion(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	g := f.sibling(2)
	theirs := g.respond(g.publication, 21, version{goods: 400})

	_, err := f.service().PutShortlist(t.Context(), f.buyer, f.request, r, 2)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().PutShortlist(t.Context(), 99, f.request, r, 1)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, theirs, 1)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().PutShortlist(t.Context(), f.buyer, g.request, theirs, 1)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, uuid.NewV7(), 1)
	wantKind(t, err, apperr.KindNotFound)
	for _, v := range []int32{0, -1} {
		_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, r, v)
		wantInvalid(t, err, "seller_response_version")
	}
	if f.count("procurement.buyer_shortlist_entries") != 0 {
		t.Fatal("被拒絕的請求不該寫入")
	}
}

func TestDeleteShortlistIsIdempotentAndReportsAbsence(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400}, version{goods: 450})
	e, err := f.service().DeleteShortlist(t.Context(), f.buyer, f.request, r, 1)
	must(t, err)
	if e.Present || e.SellerResponseID != r || e.Version != 1 {
		t.Fatalf("不在清單時 = %+v", e)
	}
	_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, r, 2)
	must(t, err)
	for range 2 {
		e, err = f.service().DeleteShortlist(t.Context(), f.buyer, f.request, r, 2)
		must(t, err)
		if e.Present || len(f.entries(r)) != 0 {
			t.Fatalf("刪除之後 = %+v %v", e, f.entries(r))
		}
	}
	// 請求帶的版本與清單上掛的不同時，仍以回價為單位移除（唯一鍵是 (request, response)），回的是被移除那一列掛的版本。
	_, err = f.service().PutShortlist(t.Context(), f.buyer, f.request, r, 2)
	must(t, err)
	e, err = f.service().DeleteShortlist(t.Context(), f.buyer, f.request, r, 1)
	must(t, err)
	if len(f.entries(r)) != 0 || e.Present || e.Version != 2 {
		t.Fatalf("以回價為單位移除、回被移除的版本 2：%+v %v", e, f.entries(r))
	}
}

func TestDeleteShortlistOnlyTouchesTheBuyersOwnRequest(t *testing.T) {
	f := newFixture(t)
	g := f.sibling(2)
	theirs := g.respond(g.publication, 21, version{goods: 400})
	_, err := g.service().PutShortlist(t.Context(), g.buyer, g.request, theirs, 1)
	must(t, err)

	_, err = f.service().DeleteShortlist(t.Context(), f.buyer, g.request, theirs, 1)
	wantKind(t, err, apperr.KindNotFound)
	if len(f.entries(theirs)) != 1 {
		t.Fatal("別人的需求不能動")
	}
	_, err = f.service().DeleteShortlist(t.Context(), f.buyer, f.request, uuid.NewV7(), 1)
	must(t, err)
	_, err = f.service().DeleteShortlist(t.Context(), f.buyer, f.request, theirs, 0)
	wantInvalid(t, err, "seller_response_version")
}
