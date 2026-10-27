package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/calleridentity"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/gin-gonic/gin"
)

const testCaps = "CAP_PRODUCT_MANAGE,CAP_ORDER_FULFILL"

func TestMonolithBusinessIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, optional := range []bool{false, true} {
		for _, tc := range []struct {
			name, buyer, store, reason, accountType, caps string
			valid                                         bool
		}{
			{"雙角色", "71", "82", "9601", "3", testCaps, true},
			{"無業務身分", "", "", "", "3", "", true},
			{"管理員", "", "", "", "4", "", true},
			{"買家", "71", "", "", "1", "", true},
			{"原因二", "", "82", "9602", "2", testCaps, true},
			{"原因三", "", "82", "9603", "2", testCaps, true},
			{"治理停權", "", "82", "SUSPENDED_GOVERNANCE", "2", testCaps, true},
			{"零 ID", "0", "", "", "1", "", false},
			{"負 ID", "", "-1", "", "2", "", false},
			{"溢位", "9223372036854775808", "", "", "1", "", false},
			{"非整數", "", "abc", "", "2", "", false},
			{"未知原因", "", "82", "OTHER", "2", "", false},
			{"沒有店家卻有原因", "", "", "9601", "2", "", false},
			{"沒有買家角色", "71", "", "", "2", "", false},
			{"管理員業務身分", "71", "82", "9601", "4", testCaps, false},
			{"有店家沒有能力", "", "82", "", "2", "", false},
			{"不認得的能力", "", "82", "", "2", "CAP_PRODUCT_MANAGE,CAP_UNKNOWN", false},
			{"空的能力 token", "", "82", "", "2", "CAP_PRODUCT_MANAGE,", false},
			{"沒有店家卻有能力", "", "", "", "2", "CAP_PRODUCT_MANAGE", false},
			{"等待刪除只有履約類", "", "82", "", "2", "CAP_ORDER_FULFILL,CAP_NOTIFICATION", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r := gin.New()
				mw := JWTAuth()
				if optional {
					mw = OptionalJWTAuth()
				}
				reached := false
				r.GET("/", mw, func(c *gin.Context) {
					reached = true
					id, ok := calleridentity.From(c.Request.Context())
					if ok != tc.valid || c.GetBool("identityFromMonolith") != tc.valid {
						t.Fatalf("source = %v", ok)
					}
					if tc.valid {
						if id.AccountID != 7 || id.SellerSuspended != tc.reason || c.GetString("sellerSuspended") != tc.reason {
							t.Fatalf("identity = %+v", id)
						}
						wantCaps := 0
						if tc.caps != "" {
							wantCaps = len(strings.Split(tc.caps, ","))
						}
						if got, _ := c.Get("sellerCapabilities"); len(got.([]authz.Capability)) != wantCaps {
							t.Fatalf("sellerCapabilities = %v, want %d 個", got, wantCaps)
						}
						if c.GetHeader("X-Seller-Capabilities") != "" {
							t.Fatal("X-Seller-Capabilities 不得往下游傳")
						}
						if c.GetInt64("buyerId") != id.BuyerID || c.GetInt64("storeId") != id.StoreID {
							t.Fatal("gin 身分不同步")
						}
					} else {
						for _, k := range []string{"X-Buyer-Id", "X-Store-Id", "X-Seller-Suspended", "X-Seller-Capabilities"} {
							if c.GetHeader(k) != "" {
								t.Fatalf("invalid %s survived", k)
							}
						}
					}
					c.Status(200)
				})
				req := httptest.NewRequest("GET", "/", nil)
				for k, v := range map[string]string{"X-Account-Id": "7", "X-Account-Type": tc.accountType, "X-Buyer-Id": tc.buyer, "X-Store-Id": tc.store, "X-Seller-Suspended": tc.reason, "X-Seller-Capabilities": tc.caps} {
					if v != "" {
						req.Header.Set(k, v)
					}
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				want := 200
				if !tc.valid && !optional {
					want = 401
				}
				if w.Code != want || reached != (tc.valid || optional) {
					t.Fatalf("status = %d, reached = %v", w.Code, reached)
				}
			})
		}
	}
}

func TestDomainHeadersUseMonolithContext(t *testing.T) {
	r := gin.New()
	buyers := buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		t.Fatal("不得呼叫 buyer RPC")
		return nil, nil
	}))
	stores := storeid.New(storeid.GetterFunc(func(context.Context, int64) (int64, error) { t.Fatal("不得呼叫 seller RPC"); return 0, nil }))
	r.GET("/", JWTAuth(), InjectBuyerIDHeader(buyers), InjectStoreIDHeader(stores), func(c *gin.Context) {
		if c.GetHeader("X-Buyer-Id") != "71" || c.GetHeader("X-Store-Id") != "82" {
			t.Fatal(c.Request.Header)
		}
		c.Status(200)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range map[string]string{"X-Account-Id": "7", "X-Account-Type": "3", "X-Buyer-Id": "71", "X-Store-Id": "82", "X-Seller-Capabilities": testCaps} {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestTicketIdentityHasNoMonolithSnapshot(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	applyIdentity(c, newIdentity(7, 3, 1))
	if _, ok := calleridentity.From(c.Request.Context()); ok || c.GetBool("identityFromMonolith") {
		t.Fatal("ticket 不得標記為單體身分")
	}
}
