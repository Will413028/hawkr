package handler

import (
	"net/http"
	"strings"
	"testing"
)

func TestGetBuyerMeta_OutwardJSONKeyIsBuyerID(t *testing.T) {
	h := &Holder{
		Buyer:   buyerMetaMock(),
		Order:   orderRelMock("1"),
		Store:   grandfatherResolverForTest(),
		BuyerID: buyerResolverForTest(idorBuyerID),
	}
	// path 帶的就是 buyer_id，原值查詢、原值回吐。
	w := idorRequest(t, h, "/api/v1/app/sellers/me/customers/2688657200/meta")

	assertHTTPStatus(t, w, http.StatusOK)
	got := w.Body.String()

	if !strings.Contains(got, `"buyerId":"2688657200"`) {
		t.Errorf("response must expose the buyer_id under %q; got %s", "buyerId", got)
	}
	if strings.Contains(got, `"accountId"`) {
		t.Errorf("response used %q for a value that is a buyer_id, not an account_id; got %s", "accountId", got)
	}
}
