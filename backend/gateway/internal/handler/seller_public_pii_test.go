package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// GET /api/v1/public/sellers/{id} 掛在 OptionalJWTAuth 底下 —— **匿名可打**，而它
// 正是官網賣家頁與買家 App 商家頁的資料來源。公開面一旦帶上店主的 email、已驗證
// 手機、生日、性別或寄退件街道地址，任何人帶一個 store_id 就拿得到。
//
// 本檔是防復發的機械保證：任何人日後把這些欄位加回公開 DTO，都會在這裡轉紅。
//
// 鑑別力：把 publicSellerPIIDenylist 的任一欄位加進 PublicSellerInfo，這兩個測試
// 必須失敗。

// publicSellerPIIDenylist 是絕對不得出現在公開賣家端點的 JSON 欄名。
//
// 分類與理由（詳見 PublicSellerInfo 型別註解）：
//   - email/phone/contactPhone：帳號註冊時已驗證的個人信箱與手機（seller-service
//     toResponse 讓 phone == contactPhone，且 GetShippingProfile 拿它們當寄件人身分）
//   - birthday/gender：店主個人資料
//   - address/addressDetail/postalCode：街道級寄退件地址（公開面只到 city+district）
//   - certificationDraftId/currentCertificationId：認證流程內部記錄 id
//   - accountType/needsOnboarding/lifecycleStatus/suspensionReasonCode：帳號與治理內部狀態
var publicSellerPIIDenylist = []string{
	"email",
	"phone",
	"contactPhone",
	"birthday",
	"gender",
	"address",
	"addressDetail",
	"postalCode",
	"certificationDraftId",
	"currentCertificationId",
	"accountType",
	"needsOnboarding",
	"lifecycleStatus",
	"suspensionReasonCode",
}

// fullySeededSellerProto 把每個帶 PII 的 proto 欄位都填成非空值 —— 若公開 DTO 少
// 掉一層過濾，空值會讓測試假綠（同 feedback_no_blind_measurement：量測要有鑑別力）。
func fullySeededSellerProto() *sellerpb.SellerInfo {
	return &sellerpb.SellerInfo{
		AccountId:                  42,
		StoreId:                    "777",
		StoreCode:                  "3012345678",
		Name:                       "老王鮮魚",
		Nickname:                   "老王鮮魚",
		Description:                "花蓮壽豐的每日現撈",
		LogoKey:                    "public/seller/777/logo.png",
		CoverKey:                   "public/seller/777/cover.png",
		Prize:                      "十大優良農產",
		City:                       "花蓮縣",
		District:                   "壽豐鄉",
		AddressDetail:              "志學村中山路一段100號",
		PostalCode:                 "974",
		ContactPhone:               "0912345678",
		Phone:                      "0912345678",
		Email:                      "owner@example.com",
		Birthday:                   "1980-01-01",
		Gender:                     "male",
		Rating:                     4.8,
		SellerType:                 "personal",
		OverallCertificationStatus: "approved",
		CurrentCertificationId:     "1234567890123456789",
		// 舊 proto 還有這兩欄；公開 DTO 不輸出（FieldSetIsExhaustive 守住，ADR 2026-10-09-store-seo-platform-generated）。
		SeoTitle:             "老王鮮魚",
		SeoDescription:       "花蓮現撈",
		WebsiteUrl:           "https://example.com",
		IsProcessedFood:      true,
		FoodBusinessRegNo:    "F-123456789-00000-1",
		AccountType:          2,
		OnboardingCompleted:  true,
		LifecycleStatus:      "active",
		SuspensionReasonCode: "9601",
	}
}

func servePublicSeller(t *testing.T) map[string]interface{} {
	t.Helper()
	mock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{Code: 0, Data: fullySeededSellerProto()}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock), MediaBaseURL: "https://img.example.com"}
	r := gin.New()
	r.GET("/api/v1/public/sellers/:id", h.GetSellerInfo)

	req, _ := http.NewRequest("GET", "/api/v1/public/sellers/777", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := parseJSON(t, w.Body)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data object; body=%v", body)
	}
	return data
}

// TestPublicSellerInfo_NoPIIOnWire 斷言匿名 GET /public/sellers/{id} 的實際 JSON
// 不含任何 PII 欄名 —— 這是對 wire 的斷言（走完整 handler + encoding/json），
// 不是對 struct 定義的斷言，所以連換成含個資的型別也擋得住。
func TestPublicSellerInfo_NoPIIOnWire(t *testing.T) {
	data := servePublicSeller(t)

	for _, banned := range publicSellerPIIDenylist {
		if v, ok := data[banned]; ok {
			// 只回報欄名，不回報值：測試輸出會進 CI log。
			t.Errorf("public seller response leaks %q (present=%v, type=%T) — 公開端點不得回傳店主個人資料／內部狀態", banned, ok, v)
		}
	}
}

// TestPublicSellerInfo_KeepsStorefrontFields 是上面那條的對稱面：擋掉 PII 不能
// 順手把店面該顯示的東西也擋掉（官網賣家頁與買家 App 商家頁靠這些欄位渲染）。
func TestPublicSellerInfo_KeepsStorefrontFields(t *testing.T) {
	data := servePublicSeller(t)

	want := map[string]interface{}{
		"storeId":                    "777",
		"storeCode":                  "3012345678",
		"storeName":                  "老王鮮魚",
		"sellerName":                 "老王鮮魚",
		"description":                "花蓮壽豐的每日現撈",
		"logoKey":                    "public/seller/777/logo.png",
		"coverKey":                   "public/seller/777/cover.png",
		"prize":                      "十大優良農產",
		"city":                       "花蓮縣",
		"district":                   "壽豐鄉",
		"sellerType":                 "personal",
		"overallCertificationStatus": "approved",
		"foodBusinessRegNo":          "F-123456789-00000-1",
		"websiteUrl":                 "https://example.com",
		"rating":                     4.8,
		"isProcessedFood":            true,
	}
	for k, expected := range want {
		got, ok := data[k]
		if !ok {
			t.Errorf("public seller response missing storefront field %q", k)
			continue
		}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("%s = %v (%T), want %v (%T)", k, got, got, expected, expected)
		}
	}
	// 49 §6.1 成對契約：key 是 SSOT，url 由 Gateway 組。
	for _, k := range []string{"logoUrl", "coverUrl"} {
		if v, _ := data[k].(string); v == "" {
			t.Errorf("%s is empty, want a Gateway-composed URL", k)
		}
	}
}

// TestPublicSellerInfo_FieldSetIsExhaustive 把公開面的欄位集合釘死。新增欄位必須
// 同步這份清單 —— 強迫加欄位的人回答一次「這是營業資訊還是店主個人資料」，而不是
// 讓欄位隨 proto 長胖靜默流到公開面（denylist 只擋得住已知的名字，這條擋未知的）。
func TestPublicSellerInfo_FieldSetIsExhaustive(t *testing.T) {
	data := servePublicSeller(t)

	want := []string{
		"city", "coverKey", "coverUrl", "description", "district",
		"foodBusinessRegNo", "isProcessedFood", "logoKey", "logoUrl",
		"overallCertificationStatus", "prize", "rating", "sellerName",
		"sellerType", "storeCode", "storeId",
		"storeName", "websiteUrl",
	}
	got := make([]string, 0, len(data))
	for k := range data {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("public seller field set drifted\n got: %v\nwant: %v\n"+
			"新增公開欄位請先確認它不是店主個人資料，再同步本清單與 PublicSellerInfo 註解", got, want)
	}
}
