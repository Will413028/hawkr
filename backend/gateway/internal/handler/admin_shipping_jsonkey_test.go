package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAdminShipmentDTOs_OutwardJSONKeyIsStoreID(t *testing.T) {
	cases := []struct {
		name string
		v    any
	}{
		{
			name: "AdminShipmentSummaryResp",
			v:    AdminShipmentSummaryResp{StoreId: "2688657200"},
		},
		{
			name: "AdminShipmentDetailResp",
			// CvsStoreCode / CvsStoreName carry `,omitempty` — must be
			// non-empty or the fields drop out of the output and the
			// assertions below would pass for the wrong reason.
			v: AdminShipmentDetailResp{
				StoreId:      "2688657200",
				BuyerId:      "2688657201",
				CvsStoreCode: "990211",
				CvsStoreName: "全家 板橋文化店",
			},
		},
		{
			name: "AdminShipmentAnomalyRecordResp",
			v:    AdminShipmentAnomalyRecordResp{StoreId: "2688657200"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			got := string(b)

			if !strings.Contains(got, `"storeId"`) {
				t.Errorf("%s must expose the store id under %q; got %s", tc.name, "storeId", got)
			}
			if strings.Contains(got, `"sellerAccountId"`) {
				t.Errorf("%s regressed back to the pre-rename key %q; got %s", tc.name, "sellerAccountId", got)
			}

			if tc.name == "AdminShipmentDetailResp" {
				if !strings.Contains(got, `"buyerId"`) {
					t.Errorf("%s must expose the buyer id under %q; got %s", tc.name, "buyerId", got)
				}
				if strings.Contains(got, `"buyerAccountId"`) {
					t.Errorf("%s regressed back to the pre-rename key %q; got %s", tc.name, "buyerAccountId", got)
				}
				if !strings.Contains(got, `"cvsStoreCode"`) {
					t.Errorf("%s must expose the CVS pickup-store code under %q; got %s", tc.name, "cvsStoreCode", got)
				}
				if !strings.Contains(got, `"cvsStoreName"`) {
					t.Errorf("%s must expose the CVS pickup-store name under %q; got %s", tc.name, "cvsStoreName", got)
				}
			}
		})
	}
}
