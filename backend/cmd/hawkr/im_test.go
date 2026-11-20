package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
)

type fakeIMOrders struct {
	has     bool
	orderNo string
	hasErr  error
	parties orderservice.OrderParties
	partErr error
}

func (f fakeIMOrders) HasOrderBetween(context.Context, int64, int64) (bool, string, error) {
	return f.has, f.orderNo, f.hasErr
}

func (f fakeIMOrders) OrderParties(context.Context, string) (orderservice.OrderParties, error) {
	return f.parties, f.partErr
}

func TestIMOpenOrderBetween(t *testing.T) {
	parties := func(status string) orderservice.OrderParties {
		return orderservice.OrderParties{OrderNo: "N1", BuyerID: 11, StoreID: 22, Status: status}
	}
	boom := errors.New("boom")
	cases := []struct {
		name    string
		f       fakeIMOrders
		want    bool
		wantErr bool
	}{
		{"no order", fakeIMOrders{}, false, false},
		{"has error", fakeIMOrders{hasErr: boom}, false, true},
		{"parties error", fakeIMOrders{has: true, orderNo: "N1", partErr: boom}, false, true},
		{"order not found is an error, not closed", fakeIMOrders{has: true, orderNo: "N1", partErr: orderservice.ErrOrderNotFound}, false, true},
		{"other buyer", fakeIMOrders{has: true, orderNo: "N1", parties: orderservice.OrderParties{OrderNo: "N1", BuyerID: 12, StoreID: 22, Status: "40"}}, false, true},
		{"other store", fakeIMOrders{has: true, orderNo: "N1", parties: orderservice.OrderParties{OrderNo: "N1", BuyerID: 11, StoreID: 23, Status: "40"}}, false, true},
		{"completed", fakeIMOrders{has: true, orderNo: "N1", parties: parties("50")}, false, false},
		{"partial refund", fakeIMOrders{has: true, orderNo: "N1", parties: parties("60")}, false, false},
		{"cancelled", fakeIMOrders{has: true, orderNo: "N1", parties: parties("80")}, false, false},
		{"unknown status", fakeIMOrders{has: true, orderNo: "N1", parties: parties("99")}, false, false},
	}
	for _, status := range []string{"10", "20", "30", "32", "35", "40"} {
		cases = append(cases, struct {
			name    string
			f       fakeIMOrders
			want    bool
			wantErr bool
		}{"open " + status, fakeIMOrders{has: true, orderNo: "N1", parties: parties(status)}, true, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := imOrders{tc.f}.OpenOrderBetween(context.Background(), 11, 22)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}
