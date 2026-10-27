package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPaymentDTOs_OutwardJSONKeyIsBuyerID(t *testing.T) {
	cases := []struct {
		name string
		v    any
	}{
		{
			name: "PaymentDetailResponse",
			// BuyerID carries `,omitempty` — must be non-empty or the field
			// drops out of the output and the assertion below would pass
			// for the wrong reason.
			v: PaymentDetailResponse{BuyerID: "2688657200"},
		},
		{
			name: "AdminPaymentTransactionSummaryResp",
			v:    AdminPaymentTransactionSummaryResp{BuyerId: "2688657200"},
		},
		{
			name: "AdminPaymentTransactionDetailResp",
			v:    AdminPaymentTransactionDetailResp{BuyerId: "2688657200"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			got := string(b)

			if !strings.Contains(got, `"buyerId"`) {
				t.Errorf("%s must expose the buyer id under %q; got %s", tc.name, "buyerId", got)
			}
			if strings.Contains(got, `"accountId"`) {
				t.Errorf("%s regressed back to the pre-rename key %q; got %s", tc.name, "accountId", got)
			}
		})
	}
}
