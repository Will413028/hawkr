package main

import (
	"context"
	"errors"
	"go/ast"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

type fakeRfqOrders struct {
	got     orderservice.RfqOrderCommand
	res     orderservice.RfqOrderResult
	err     error
	parties orderservice.OrderParties
}

func (f *fakeRfqOrders) OrderParties(context.Context, string) (orderservice.OrderParties, error) {
	return f.parties, f.err
}

func (f *fakeRfqOrders) CreateRfqOrder(_ context.Context, cmd orderservice.RfqOrderCommand) (orderservice.RfqOrderResult, error) {
	f.got = cmd
	return f.res, f.err
}

func TestProcurementOrdersMapsTheCommandAndTheResult(t *testing.T) {
	deadline := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeRfqOrders{res: orderservice.RfqOrderResult{Code: 0, OrderNo: "HK1", PaymentDeadline: deadline, Replayed: true}, parties: orderservice.OrderParties{BuyerID: 3, StoreID: 11, Status: "0"}}
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
	if res.OrderNo != "HK1" || !res.PaymentDeadline.Equal(deadline) || res.Code != 0 || !res.Replayed {
		t.Fatalf("結果 = %+v", res)
	}
	parties, err := procurementOrders{f}.OrderParties(t.Context(), "HK1")
	if err != nil || parties.BuyerID != 3 || parties.StoreID != 11 || parties.Status != "0" {
		t.Fatalf("買家與店家 = %+v %v", parties, err)
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

// 並行期單一生效者：舊 AI 服務與它的路由已刪，建單的單一生效者是單體：worker 角色登記建單 worker 與它的 reconciler 排程，
// 組採購模組時帶舊 ordering 的轉接（沒給 Orders 時 AddOrderWorkers 會 panic）。serve 角色不執行 job，不登記。
func TestWorkerRoleRegistersProcurementOrderProvisioning(t *testing.T) {
	requireCalls(t, "procurement.go", "addProcurementWorkers", "m.AddOrderWorkers")
	requireCalls(t, "main.go", "runWorker", "addProcurementWorkers", "procurementModule.OrderPeriodicJobs")
	for _, name := range callsIn(t, "main.go", "runServe") {
		if name == "addProcurementWorkers" || name == "procurementModule.AddOrderWorkers" || name == "procurementModule.OrderPeriodicJobs" {
			t.Errorf("runServe 不該呼叫 %s：job 只在 worker 角色執行", name)
		}
	}
	found := false
	ast.Inspect(funcBody(t, "runWorker"), func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "newProcurementWorker" {
			return true
		}
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.CompositeLit); ok {
				if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "procurementOrders" {
					found = true
				}
			}
		}
		return true
	})
	if !found {
		t.Error("runWorker 組採購模組時沒有帶 procurementOrders{orders}（建單 port）")
	}
}
