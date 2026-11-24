package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

type fakeRfqOrders struct {
	got orderservice.RfqOrderCommand
	res orderservice.RfqOrderResult
	err error
}

func (f *fakeRfqOrders) CreateRfqOrder(_ context.Context, cmd orderservice.RfqOrderCommand) (orderservice.RfqOrderResult, error) {
	f.got = cmd
	return f.res, f.err
}

func TestProcurementOrdersMapsTheCommandAndTheResult(t *testing.T) {
	deadline := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeRfqOrders{res: orderservice.RfqOrderResult{Code: 0, OrderNo: "HK1", PaymentDeadline: deadline}}
	res, err := procurementOrders{f}.CreateOrder(t.Context(), procurement.OrderCommand{
		OrderRef: 7, BuyerID: 3, StoreID: 11, TotalAmount: 415, ShippingFee: 10, UnitPrice: 405, ItemName: "小白菜 3 KG",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := orderservice.RfqOrderCommand{AcceptanceID: 7, BuyerID: 3, StoreID: 11, TotalAmount: 415, ShippingFee: 10, UnitPrice: 405, SkuName: "小白菜 3 KG", Lane: "PLATFORM"}
	if f.got != want {
		t.Fatalf("命令 = %+v，要 %+v", f.got, want)
	}
	if res.OrderNo != "HK1" || !res.PaymentDeadline.Equal(deadline) || res.Code != 0 {
		t.Fatalf("結果 = %+v", res)
	}
}

func TestProcurementOrdersPassesRejectionsAndTransportErrors(t *testing.T) {
	f := &fakeRfqOrders{res: orderservice.RfqOrderResult{Code: 9504, Message: "no"}}
	res, err := procurementOrders{f}.CreateOrder(t.Context(), procurement.OrderCommand{})
	if err != nil || res.Code != 9504 || res.Message != "no" {
		t.Fatalf("拒絕要原樣帶回 code：%+v %v", res, err)
	}
	boom := errors.New("boom")
	if _, err := (procurementOrders{&fakeRfqOrders{err: boom}}).CreateOrder(t.Context(), procurement.OrderCommand{}); !errors.Is(err, boom) {
		t.Fatalf("傳輸錯誤要回 error：%v", err)
	}
}

// 並行期單一生效者：建單 worker 在 A9-6 之前只有測試登記，worker 角色不得登記它或它的排程。
func TestWorkerRoleDoesNotRegisterProcurementOrderProvisioning(t *testing.T) {
	for _, fn := range []string{"runWorker", "runServe"} {
		for _, name := range callsIn(t, "main.go", fn) {
			if name == "procurementModule.AddOrderWorkers" || name == "procurementModule.OrderPeriodicJobs" {
				t.Errorf("%s 呼叫了 %s：建單的單一生效者是舊服務，A9-6 才接上", fn, name)
			}
		}
	}
	for _, name := range callsIn(t, "procurement.go", "addProcurementWorkers") {
		if name == "m.AddOrderWorkers" {
			t.Error("addProcurementWorkers 不得登記建單 worker")
		}
	}
}
