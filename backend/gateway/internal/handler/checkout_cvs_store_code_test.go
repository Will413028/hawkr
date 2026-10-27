package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// resolveCvsStore 是 checkout 三個端點（shippingFee／preview／confirm）對「超商取貨
// 門市」的唯一解析點。這組測試釘住兩件事：
//
//  1. **正名相容**：新 key `cvsStoreCode` 與舊 key `storeId` 語意相同，新的優先；舊 key
//     不能拔——已上架的買家 App 直接呼叫這些端點，只送舊 key。
//  2. **污染閘門**：非超商取貨帶門市代號一律 400。這正是 dev 上 79 列宅配單被寫進
//     賣家 `store_id` 當門市代號的原因（`orders.cvs_store_id`，2026-07-18～08-30，
//     43 個買家，全部出自驗證腳本）——後端當時完全不驗，靜默收下。
func TestResolveCvsStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name            string
		shippingMethod  string
		cvsStoreCode    string
		cvsStoreName    string
		legacyStoreID   string
		legacyStoreName string
		wantOK          bool
		wantCode        string
		wantName        string
	}{
		{
			name:           "CVS pickup with new key",
			shippingMethod: "3002",
			cvsStoreCode:   "252698",
			cvsStoreName:   "大安門市",
			wantOK:         true,
			wantCode:       "252698",
			wantName:       "大安門市",
		},
		{
			name:            "CVS pickup with legacy key still accepted",
			shippingMethod:  "3002",
			legacyStoreID:   "252698",
			legacyStoreName: "大安門市",
			wantOK:          true,
			wantCode:        "252698",
			wantName:        "大安門市",
		},
		{
			name:            "new key wins over legacy key",
			shippingMethod:  "3002",
			cvsStoreCode:    "252698",
			cvsStoreName:    "大安門市",
			legacyStoreID:   "968757",
			legacyStoreName: "士東門市",
			wantOK:          true,
			wantCode:        "252698",
			wantName:        "大安門市",
		},
		{
			name:           "home delivery with no store code is fine",
			shippingMethod: "3001",
			wantOK:         true,
		},
		{
			// 這一格就是 dev 上那 79 列髒訂單的形狀：宅配單帶著賣家 store_id。
			name:           "home delivery rejects a platform store_id posing as a branch code",
			shippingMethod: "3001",
			legacyStoreID:  "7487768729113814280",
			wantOK:         false,
		},
		{
			name:           "home delivery rejects even a well-formed branch code",
			shippingMethod: "3001",
			cvsStoreCode:   "252698",
			wantOK:         false,
		},
		{
			name:           "instant delivery rejects a branch code",
			shippingMethod: "3003",
			cvsStoreCode:   "252698",
			wantOK:         false,
		},
		{
			// 門市名稱單獨留著會寫出「有店名沒店號」的訂單列，一併丟掉。
			name:            "home delivery drops a stray store name",
			shippingMethod:  "3001",
			legacyStoreName: "大安門市",
			wantOK:          true,
		},
		{
			// 買家先選 3002 看運費、再挑門市——preview 這一步代號還是空的，必須放行。
			name:           "CVS pickup before a branch is picked is allowed",
			shippingMethod: "3002",
			wantOK:         true,
		},
		{
			name:           "CVS pickup rejects a 19-digit snowflake store_id",
			shippingMethod: "3002",
			cvsStoreCode:   "7487768729113814280",
			wantOK:         false,
		},
		{
			name:           "CVS pickup rejects a 10-digit platform store_id",
			shippingMethod: "3002",
			cvsStoreCode:   "2675237784",
			wantOK:         false,
		},
		{
			name:           "CVS pickup rejects a non-numeric code",
			shippingMethod: "3002",
			cvsStoreCode:   "task22-spotcheck-store",
			wantOK:         false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/confirm", nil)

			code, name, ok := resolveCvsStore(c, tc.shippingMethod, tc.cvsStoreCode, tc.cvsStoreName, tc.legacyStoreID, tc.legacyStoreName)

			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (body: %s)", ok, tc.wantOK, w.Body.String())
			}
			if !tc.wantOK {
				if w.Code != http.StatusBadRequest {
					t.Fatalf("rejected request must answer 400, got %d (body: %s)", w.Code, w.Body.String())
				}
				return
			}
			if code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
		})
	}
}
