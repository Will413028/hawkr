package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
)

// 代行訂單搜尋比對 order_items 的 product_name／sku_name（OrderRepository.SELLER_ORDER_FILTER），
// 而列表的商品欄只印得出件數 ⇒ 搜一個品名拿到四筆、四列全是「共 2 件」，操作者無從判斷是
// 哪一件命中。品名一直都在上游（OrderSummaryView.items），也一直不在代行投影的 clear 清單內
// （OrderGrpcService.java:863-885）——缺口只在 Gateway 這一層的 DTO。
//
// 這支測試守兩件事：欄位存在（否則前端讀到 undefined，畫面靜默退回破折號），以及取值規則
// （只取第一件、product_name 為空退 sku_name）。

func TestFirstDelegatedProductName(t *testing.T) {
	cases := []struct {
		name   string
		items  []*orderpb.OrderItemView
		want   string
		reason string
	}{
		{
			name:   "emptyItems",
			items:  nil,
			want:   "",
			reason: "沒有品項就回空字串，由前端走 fallback，不得自造文案",
		},
		{
			name: "prefersProductName",
			items: []*orderpb.OrderItemView{
				{ProductName: "有機小松菜", SkuName: "300g／包"},
			},
			want:   "有機小松菜",
			reason: "product_name 是商品名，sku_name 是規格名；有商品名就用商品名",
		},
		{
			name: "fallsBackToSkuName",
			items: []*orderpb.OrderItemView{
				{ProductName: "", SkuName: "溫室小黃瓜 600g"},
			},
			want: "溫室小黃瓜 600g",
			reason: "order_items.product_name 可為 NULL 而 sku_name NOT NULL；回空字串會讓" +
				"一筆搜得到（keyword 比對 sku_name）的訂單在畫面上顯示破折號",
		},
		{
			name: "blankProductNameIsTreatedAsEmpty",
			items: []*orderpb.OrderItemView{
				{ProductName: "   ", SkuName: "紅蘿蔔 1kg"},
			},
			want:   "紅蘿蔔 1kg",
			reason: "只有空白的商品名在畫面上與空字串沒有分別，不能因為它非空就採用",
		},
		{
			name: "takesOnlyTheFirstItem",
			items: []*orderpb.OrderItemView{
				{ProductName: "青花菜"},
				{ProductName: "高麗菜"},
			},
			want:   "青花菜",
			reason: "這一欄只承載第一件；其餘品名靠 itemCount 表達成「等 N 件」",
		},
		{
			name: "trimsSurroundingSpace",
			items: []*orderpb.OrderItemView{
				{ProductName: "  日光番茄  "},
			},
			want:   "日光番茄",
			reason: "快照欄位帶頭尾空白時，畫面會排出一段對不齊的縮排",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstDelegatedProductName(tc.items); got != tc.want {
				t.Errorf("firstDelegatedProductName() = %q, want %q —— %s", got, tc.want, tc.reason)
			}
		})
	}
}

// 反向的鑑別力：上面那支測的是取值規則，這支測的是「取到的值真的送得出去」。
// 少了這一欄，前端讀到 undefined ⇒ 商品欄靜默退回破折號，而後端與測試全綠。
func TestDelegatedOrderListItem_CarriesFirstProductName(t *testing.T) {
	const field = "firstProductName"
	for _, f := range collectJSONFieldNames(reflect.TypeOf(DelegatedOrderListItem{}), map[reflect.Type]bool{}) {
		if strings.EqualFold(f, field) {
			return
		}
	}
	t.Errorf("DelegatedOrderListItem 缺少 %q —— 搜尋比對品名，列表就必須說得出命中的是哪一件", field)
}

// 上面兩支合起來仍然驗不出「品名真的送得出去」：一支測純函式的取值規則，一支測 DTO
// 宣告了那個鍵。把 ListDelegatedOrders 裡 `FirstProductName:` 那一行賦值刪掉、欄位宣告
// 留著，兩支照樣綠——連 `go build`、gateway 全 package 的 `go test`、
// scripts/gateway-json-contract-check.py 與 gofmt 也一起放行，而畫面上每一列印的是破折號
// （前端資料層對「後端沒帶值」的處置就是空字串，渲染層再 `|| "—"`）。
//
// 恆為空的欄位比沒有這個欄位更糟：契約上它看起來已經做到了，於是沒有人會再回來做。
//
// 這支從 HTTP 回應的 JSON 下手，錨在**值**而不是鍵——鍵沒有 omitempty，永遠都在。
// 三列一次送出，順帶釘住「每一列取的是自己的第一件」：把取值搬到迴圈外算一次也是綠的
// 那種寫法，逐列比對才擋得住。
func TestListDelegatedOrders_ResponseRowsCarryFirstProductName(t *testing.T) {
	upstream := &orderpb.GetSellerOrdersResponse{
		Code: 0,
		Orders: []*orderpb.OrderSummaryView{
			{
				OrderNo:   "O005001417",
				ItemCount: "2",
				Items: []*orderpb.OrderItemView{
					{ProductName: "有機小松菜", SkuName: "300g／包"},
					{ProductName: "梨山高麗菜", SkuName: "1顆"},
				},
			},
			{
				OrderNo:   "O005001418",
				ItemCount: "1",
				Items: []*orderpb.OrderItemView{
					{ProductName: "   ", SkuName: "溫室小黃瓜 600g"},
				},
			},
			{OrderNo: "O005001419", ItemCount: "0"},
		},
		Meta: &orderpb.PageMeta{TotalItems: "3", CurrentPage: "1"},
	}
	want := map[string]string{
		// 有商品名就用商品名，且只取第一件（第二件的「梨山高麗菜」不得出現）。
		"O005001417": "有機小松菜",
		// product_name 只有空白 ⇒ 退回 sku_name。keyword 比對得到 sku_name，
		// 這一列若印破折號就是「搜得到卻看不出為什麼」。
		"O005001418": "溫室小黃瓜 600g",
		// 沒有品項的訂單仍要有這個鍵，值為空字串由前端走 fallback。
		"O005001419": "",
	}

	r, seen := newDelegatedOrderListRouterReturning(t, upstream)
	w := getDelegatedOrders(t, r, "")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	if len(*seen) != 1 {
		t.Fatalf("上游被呼叫 %d 次，want 1", len(*seen))
	}

	var body struct {
		Data struct {
			Records []map[string]any `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是 JSON envelope：%v（body=%s）", err, w.Body.String())
	}
	if len(body.Data.Records) != len(want) {
		t.Fatalf("records 有 %d 列，want %d —— 列數對不上時逐列斷言驗的是別的東西，此時的綠燈是假的",
			len(body.Data.Records), len(want))
	}

	for _, rec := range body.Data.Records {
		orderNo, _ := rec["orderNo"].(string)
		expected, known := want[orderNo]
		if !known {
			t.Fatalf("回應出現預期外的 orderNo %q", orderNo)
		}
		raw, ok := rec["firstProductName"]
		if !ok {
			t.Errorf("%s 的回應缺少 firstProductName 鍵 —— 前端讀到 undefined，商品欄靜默退回破折號", orderNo)
			continue
		}
		got, _ := raw.(string)
		if got != expected {
			t.Errorf("%s 的 firstProductName = %q, want %q —— 欄位在、值沒帶到，"+
				"畫面上與「沒做這個功能」完全一樣，而四道 gate 全部放行", orderNo, got, expected)
		}
	}
}
