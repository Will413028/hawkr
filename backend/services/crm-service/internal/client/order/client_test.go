package order

import (
	"strings"
	"testing"

	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
)

// 這幾支測的是 orderInfoFromView —— GetOrderByNo 唯一沒有網路依賴的那一段。
//
// ⚠️ fixture 直接用 orderpb.OrderAfterSalesView（proto 定義本身），不是手寫一份
// 「我覺得 order-service 會回成這樣」的結構：proto 是這條契約的機器可檢定義，
// 欄位改名或改型別時這裡會編譯失敗，而手寫 fixture 不會
// （03-delegation-templates.md §0.3）。
//
// 為什麼原本的 filterExactOrderNo 測試不見了：那個函式隨 #3367 的修復一起移除
// （改用精確 RPC 之後不再需要 client 端過濾），留著測一個已刪的函式沒有意義。
func TestOrderInfoFromView_ParsesIdentity(t *testing.T) {
	view := &orderpb.OrderAfterSalesView{
		OrderNo: "O922495710",
		BuyerId: "2623900520",
		StoreId: "2685914682",
	}
	got, err := orderInfoFromView(view, "O922495710")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OrderNo != "O922495710" {
		t.Errorf("OrderNo = %q，預期 O922495710", got.OrderNo)
	}
	// 🔴 字面值硬寫，不從 view 推導：期望值跟著被測物一起變的話，
	// 把 ParseInt 換成回傳 0 也驗不出來（§0.3 第二種形態）。
	if got.StoreID != 2685914682 {
		t.Errorf("StoreID = %d，預期 2685914682", got.StoreID)
	}
	if got.BuyerID != 2623900520 {
		t.Errorf("BuyerID = %d，預期 2623900520", got.BuyerID)
	}
}

// server 回 code=0 卻沒有 data，是「成功但空」。
//
// 🔴 這支測的重點**不是**「會不會回 error」——protobuf 的 getter 對 nil receiver
// 是安全的（回零值），所以就算拿掉 nil 檢查，後面的 ParseInt("") 也會回 error。
// 2026-09-12 實測：只斷言「err != nil」時，拿掉 nil 檢查測試照樣全綠（變異不轉紅）。
//
// 真正要守的是**錯誤訊息說對了原因**：少了 nil 檢查，值班看到的是
// 「parse store_id "" 失敗」⇒ 會往「order-service 回了壞資料」查；
// 而實際情況是「server 回 code=0 但整包 data 是空的」，那是完全不同的故障。
func TestOrderInfoFromView_NilData_SaysEmptyData(t *testing.T) {
	_, err := orderInfoFromView(nil, "O1")
	if err == nil {
		t.Fatal("data 為空時必須回 error，不得回 nil OrderInfo 讓呼叫端炸在別處")
	}
	if !strings.Contains(err.Error(), "O1") {
		t.Errorf("錯誤訊息要帶 order_no 才追得到是哪一筆，得到：%v", err)
	}
	// 這一行是本測試的鑑別力所在：它區分「空 data」與「欄位解析失敗」兩種故障。
	if !strings.Contains(err.Error(), "empty data") {
		t.Errorf("錯誤訊息必須指出是 server 回了空 data，否則會把值班導向錯誤方向，得到：%v", err)
	}
}

func TestOrderInfoFromView_UnparsableIDs_ReturnError(t *testing.T) {
	cases := []struct {
		name    string
		view    *orderpb.OrderAfterSalesView
		wantSub string
	}{
		{
			name:    "store_id 不是數字",
			view:    &orderpb.OrderAfterSalesView{OrderNo: "O1", BuyerId: "1", StoreId: "not-a-number"},
			wantSub: "store_id",
		},
		{
			name:    "buyer_id 不是數字",
			view:    &orderpb.OrderAfterSalesView{OrderNo: "O1", BuyerId: "", StoreId: "1"},
			wantSub: "buyer_id",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := orderInfoFromView(c.view, "O1")
			if err == nil {
				t.Fatal("預期回 error")
			}
			// 指名是哪個欄位壞掉——兩個欄位共用一種錯誤訊息的話，
			// 值班要自己去猜是 store 還是 buyer。
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("錯誤訊息要指名 %q，得到：%v", c.wantSub, err)
			}
		})
	}
}
