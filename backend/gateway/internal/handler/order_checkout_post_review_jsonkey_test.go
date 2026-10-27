package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOrderCheckoutPostReviewDTOs_OutwardJSONKeyIsOperationalKey(t *testing.T) {
	const buyerID = "2688657200"
	const storeID = "2685914682"

	cases := []struct {
		name         string
		v            any
		wantContain  []string
		wantNotExist []string
	}{
		{
			name:         "AdminAbnormalOrderSummaryResp",
			v:            AdminAbnormalOrderSummaryResp{BuyerId: buyerID, StoreId: storeID},
			wantContain:  []string{`"buyerId":"` + buyerID + `"`, `"storeId":"` + storeID + `"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
		},
		{
			name:         "OrderGroupPreviewResponse",
			v:            OrderGroupPreviewResponse{StoreID: storeID},
			wantContain:  []string{`"storeId":"` + storeID + `"`},
			wantNotExist: []string{`"sellerAccountId"`},
		},
		{
			name:         "OrderDetailResponse",
			v:            OrderDetailResponse{BuyerID: buyerID, StoreID: storeID},
			wantContain:  []string{`"buyerId":"` + buyerID + `"`, `"storeId":"` + storeID + `"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
		},
		{
			name:         "OrderSummaryResponse",
			v:            OrderSummaryResponse{BuyerID: buyerID, StoreID: storeID},
			wantContain:  []string{`"buyerId":"` + buyerID + `"`, `"storeId":"` + storeID + `"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
		},
		{
			name:         "OrderReviewRecord",
			v:            OrderReviewRecord{BuyerID: buyerID, StoreID: storeID},
			wantContain:  []string{`"buyerId":"` + buyerID + `"`, `"storeId":"` + storeID + `"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
		},
		{
			// social 的 admin DTO 走 snake_case，正名目標鍵同步為 snake_case。
			name:         "AdminPostListItemData",
			v:            AdminPostListItemData{StoreID: storeID},
			wantContain:  []string{`"storeId":"` + storeID + `"`},
			wantNotExist: []string{`"seller_account_id"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			got := string(b)

			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("%s must expose %q; got %s", tc.name, want, got)
				}
			}
			for _, notWant := range tc.wantNotExist {
				if strings.Contains(got, notWant) {
					t.Errorf("%s regressed back to the pre-rename key %q; got %s", tc.name, notWant, got)
				}
			}
		})
	}
}
