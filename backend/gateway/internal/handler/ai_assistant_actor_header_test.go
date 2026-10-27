package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mustNotResolveBuyer (used below) is defined in post_test.go — fails the
// test if consulted, proving a seller-route call never touches the buyer
// resolver.

func newActorHeaderTestContext(accountID int64) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/whatever", nil)
	c.Request = req
	c.Set("accountId", accountID)
	return c, w
}

// TestResolveAssistantActorHeader_SellerRoute_UsesStoreResolver pins the
// branch that keeps ai-service from ever seeing a raw login account_id for
// seller assistant routes: /seller/... must resolve via the STORE resolver
// and forward X-Store-Id, never touching the buyer resolver.
func TestResolveAssistantActorHeader_SellerRoute_UsesStoreResolver(t *testing.T) {
	c, _ := newActorHeaderTestContext(42)
	h := &Holder{Store: storeResolverForTest(8001)}

	header, value, ok := h.resolveAssistantActorHeader(c, "/seller/assistant/chat")
	if !ok {
		t.Fatal("resolveAssistantActorHeader() ok = false, want true")
	}
	if header != "X-Store-Id" {
		t.Errorf("header = %q, want X-Store-Id", header)
	}
	if value != "8001" {
		t.Errorf("value = %q, want the resolved store_id 8001, not the raw account_id 42", value)
	}
}

// TestResolveAssistantActorHeader_BuyerRoute_UsesBuyerResolver is the mirror:
// /buyer/... must resolve via the BUYER resolver and forward X-Buyer-Id.
func TestResolveAssistantActorHeader_BuyerRoute_UsesBuyerResolver(t *testing.T) {
	c, _ := newActorHeaderTestContext(42)
	h := &Holder{BuyerID: buyerResolverForTest(55321)}

	header, value, ok := h.resolveAssistantActorHeader(c, "/buyer/assistant/orders")
	if !ok {
		t.Fatal("resolveAssistantActorHeader() ok = false, want true")
	}
	if header != "X-Buyer-Id" {
		t.Errorf("header = %q, want X-Buyer-Id", header)
	}
	if value != "55321" {
		t.Errorf("value = %q, want the resolved buyer_id 55321, not the raw account_id 42", value)
	}
}

// TestResolveAssistantActorHeader_SellerRoute_NeverConsultsBuyerResolver is
// the discriminating test for a resolver swap regression: if a future edit
// accidentally routes /seller/... through resolveBuyerID, this fails loudly
// (mustNotResolveBuyer panics the test) instead of silently forwarding the
// wrong business key to ai-service.
func TestResolveAssistantActorHeader_SellerRoute_NeverConsultsBuyerResolver(t *testing.T) {
	c, _ := newActorHeaderTestContext(42)
	h := &Holder{Store: storeResolverForTest(8001), BuyerID: mustNotResolveBuyer(t)}

	if _, _, ok := h.resolveAssistantActorHeader(c, "/seller/assistant/inventory"); !ok {
		t.Fatal("resolveAssistantActorHeader() ok = false, want true")
	}
}

// TestResolveAssistantActorHeader_BuyerRoute_NeverConsultsStoreResolver is
// the mirror of the above for buyer routes.
func TestResolveAssistantActorHeader_BuyerRoute_NeverConsultsStoreResolver(t *testing.T) {
	c, _ := newActorHeaderTestContext(42)
	h := &Holder{BuyerID: buyerResolverForTest(55321), Store: mustNotResolveStore(t)}

	if _, _, ok := h.resolveAssistantActorHeader(c, "/buyer/assistant/tracking"); !ok {
		t.Fatal("resolveAssistantActorHeader() ok = false, want true")
	}
}

// TestResolveAssistantActorHeader_NoCurrentStore_FailsClosed proves a seller
// route with no current store stint returns ok=false (403 already written by
// resolveStoreID) instead of falling back to the raw account_id.
func TestResolveAssistantActorHeader_NoCurrentStore_FailsClosed(t *testing.T) {
	c, w := newActorHeaderTestContext(42)
	h := &Holder{Store: noStoreResolverForTest()}

	if _, _, ok := h.resolveAssistantActorHeader(c, "/seller/assistant/chat"); ok {
		t.Fatal("resolveAssistantActorHeader() ok = true, want false when the seller has no current store")
	}
	if w.Code != 403 {
		t.Errorf("response status = %d, want 403 (already written by resolveStoreID)", w.Code)
	}
}

// TestResolveAssistantActorHeader_NoCurrentBuyer_FailsClosed is the buyer-side
// mirror.
func TestResolveAssistantActorHeader_NoCurrentBuyer_FailsClosed(t *testing.T) {
	c, w := newActorHeaderTestContext(42)
	h := &Holder{BuyerID: noBuyerResolverForTest()}

	if _, _, ok := h.resolveAssistantActorHeader(c, "/buyer/assistant/chat"); ok {
		t.Fatal("resolveAssistantActorHeader() ok = true, want false when the buyer has no current stint")
	}
	if w.Code != 403 {
		t.Errorf("response status = %d, want 403 (already written by resolveBuyerID)", w.Code)
	}
}
