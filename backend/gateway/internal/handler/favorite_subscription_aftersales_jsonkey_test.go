package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFavoriteSubscriptionAftersalesDTOs_OutwardJSONKeyIsBuyerID(t *testing.T) {
	cases := []struct {
		name         string
		v            any
		wantContain  []string
		wantNotExist []string
	}{
		{
			name:         "AdminAfterSaleCaseSummaryResp",
			v:            AdminAfterSaleCaseSummaryResp{BuyerId: "2688657200", StoreId: "2688657202"},
			wantContain:  []string{`"buyerId":"2688657200"`, `"storeId":"2688657202"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
		},
		{
			name:         "AfterSaleCaseDetailResponse",
			v:            AfterSaleCaseDetailResponse{BuyerId: "2688657200", StoreId: "2688657202"},
			wantContain:  []string{`"buyerId":"2688657200"`, `"storeId":"2688657202"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
		},
		{
			name:         "AppealSummaryResponse",
			v:            AppealSummaryResponse{BuyerId: "2688657200", StoreId: "2688657202"},
			wantContain:  []string{`"buyerId":"2688657200"`, `"storeId":"2688657202"`},
			wantNotExist: []string{`"buyerAccountId"`, `"sellerAccountId"`},
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
