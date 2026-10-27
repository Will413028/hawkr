package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 責任判定 maker-checker（65 §6.1.1 A-4／A-5）在 Gateway 側的守衛。
//
// 🔴 這裡驗的是**保證**不是改動：A-5 的保證是「只有真的被人批准或駁回的提案才會往下走」。
// 它的反面（未知 decision 不得抵達下游）才是那個保證的實質內容——只斷言 400 而不斷言
// 「下游零呼叫」的測試，對「回了 400 但仍然發出 RPC」完全沒有鑑別力，而那正是最壞的
// 失效形狀：買家收不到退款的 N16 已經排出去了，前端卻看到一個錯誤。

// fakeResponsibilityClient 只接受唯一正解，其餘一律回 error。
//
// 🔴 fake 的鑑別力來自它**拒絕**什麼：一個無論收到什麼都回成功的替身，會讓「守衛失效」
// 與「守衛有效」在斷言上長得一模一樣。
type fakeResponsibilityClient struct {
	proposeCalls  int
	reviewCalls   int
	lastPropose   *pb.ProposeResponsibilityResolutionForAdminRequest
	lastReview    *pb.ReviewResponsibilityResolutionForAdminRequest
	proposeResult *pb.ProposeResponsibilityResolutionForAdminResponse
	reviewResult  *pb.ReviewResponsibilityResolutionForAdminResponse
}

func (f *fakeResponsibilityClient) ProposeResponsibilityResolutionForAdmin(
	_ context.Context, in *pb.ProposeResponsibilityResolutionForAdminRequest, _ ...grpc.CallOption,
) (*pb.ProposeResponsibilityResolutionForAdminResponse, error) {
	f.proposeCalls++
	f.lastPropose = in
	if in.GetCaseNo() == "" || in.GetResolutionType() == "" ||
		in.GetResponsibilityCause() == "" || in.GetReason() == "" {
		return nil, fmt.Errorf("propose 收到不完整的提案：%+v", in)
	}
	return f.proposeResult, nil
}

func (f *fakeResponsibilityClient) ReviewResponsibilityResolutionForAdmin(
	_ context.Context, in *pb.ReviewResponsibilityResolutionForAdminRequest, _ ...grpc.CallOption,
) (*pb.ReviewResponsibilityResolutionForAdminResponse, error) {
	f.reviewCalls++
	f.lastReview = in
	if in.GetDecision() != "APPROVE" && in.GetDecision() != "REJECT" {
		return nil, fmt.Errorf("review 收到非法 decision=%q —— gateway 的守衛沒擋住", in.GetDecision())
	}
	return f.reviewResult, nil
}

func setupResponsibilityRouter(h *Holder) *gin.Engine {
	r := gin.New()
	admin := r.Group("/api/v1/admin")
	admin.POST("/logistics/exception_cases/:case_no/resolutions", h.ProposeResponsibilityResolutionForAdmin)
	admin.POST("/logistics/exception_cases/:case_no/resolutions/:resolution_id/approvals",
		h.ReviewResponsibilityResolutionForAdmin)
	return r
}

func newResponsibilityHolder(f *fakeResponsibilityClient) *Holder {
	return &Holder{Payment: grpcclient.NewPaymentResponsibilityClientForTest(f)}
}

func postJSON(t *testing.T, h *Holder, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	setupResponsibilityRouter(h).ServeHTTP(w, req)
	return w
}

const (
	reviewPath  = "/api/v1/admin/logistics/exception_cases/EXC-1/resolutions/R-1/approvals"
	proposePath = "/api/v1/admin/logistics/exception_cases/EXC-1/resolutions"
)

// ==================== A-5 decision 守衛 ====================

// 🔴 陰性驗證：非 APPROVE/REJECT 的值必須在抵達 payment-service 之前就被擋下。
func TestReviewResponsibility_UnknownDecisionNeverReachesDownstream(t *testing.T) {
	for _, decision := range []string{
		"MAYBE",       // 完全未知
		"approve",     // 🔴 大小寫敏感：小寫不是 APPROVE
		"Approve",     //
		"APPROVE ",    // 尾隨空白
		"APPROVE,YES", // 夾帶
		"",            // 空字串（binding:"required" 這一層就該擋）
	} {
		t.Run("decision="+decision, func(t *testing.T) {
			f := &fakeResponsibilityClient{}
			w := postJSON(t, newResponsibilityHolder(f),
				reviewPath, `{"decision":"`+decision+`","reviewComment":"ok"}`)

			assertHTTPStatus(t, w, http.StatusBadRequest)
			if f.reviewCalls != 0 {
				t.Fatalf("decision=%q 被送到下游 %d 次 —— 一筆沒有人批准的提案可能因此排出 N16",
					decision, f.reviewCalls)
			}
		})
	}
}

// 陽性對照：兩個合法值都必須原樣送達，且回應逐欄對應。
func TestReviewResponsibility_ValidDecisionsPassThrough(t *testing.T) {
	for _, tc := range []struct {
		decision     string
		resolution   string
		caseStatus   string
		cause        string
		n16Enqueued  bool
		wantN16InAPI bool
	}{
		{"APPROVE", "APPROVED", "RESOLVED", "SELLER_CAUSED", true, true},
		{"REJECT", "REJECTED", "IN_PROGRESS", "", false, false},
	} {
		t.Run(tc.decision, func(t *testing.T) {
			f := &fakeResponsibilityClient{
				reviewResult: &pb.ReviewResponsibilityResolutionForAdminResponse{
					Code: 0,
					Data: &pb.ReviewResponsibilityResolutionForAdminData{
						CaseNo:              "EXC-1",
						ResolutionId:        "R-1",
						ResolutionStatus:    tc.resolution,
						CaseStatus:          tc.caseStatus,
						ResponsibilityCause: tc.cause,
						N16Enqueued:         tc.n16Enqueued,
					},
				},
			}
			w := postJSON(t, newResponsibilityHolder(f),
				reviewPath, `{"decision":"`+tc.decision+`","reviewComment":"看過了"}`)

			assertHTTPStatus(t, w, http.StatusOK)
			body := parseJSON(t, w.Body)
			assertCode(t, body, 0)

			if f.reviewCalls != 1 {
				t.Fatalf("reviewCalls = %d, want 1", f.reviewCalls)
			}
			if got := f.lastReview.GetDecision(); got != tc.decision {
				t.Errorf("送到下游的 decision = %q, want %q", got, tc.decision)
			}
			// path param 要真的被用上——兩個都寫死在 URL 裡，抄錯任一個都會讓覆核落到別張單上。
			if f.lastReview.GetCaseNo() != "EXC-1" || f.lastReview.GetResolutionId() != "R-1" {
				t.Errorf("path param 沒有正確帶下去：caseNo=%q resolutionId=%q",
					f.lastReview.GetCaseNo(), f.lastReview.GetResolutionId())
			}

			data, ok := body["data"].(map[string]interface{})
			if !ok {
				t.Fatalf("data 不是物件；body = %v", body)
			}
			if data["resolutionStatus"] != tc.resolution {
				t.Errorf("resolutionStatus = %v, want %v", data["resolutionStatus"], tc.resolution)
			}
			if data["responsibilityCause"] != tc.cause {
				t.Errorf("responsibilityCause = %v, want %v", data["responsibilityCause"], tc.cause)
			}
			// n16Enqueued 必須原樣透出：前端要靠它分辨「重送的冪等結果」與「真的沒做到」。
			if data["n16Enqueued"] != tc.wantN16InAPI {
				t.Errorf("n16Enqueued = %v, want %v", data["n16Enqueued"], tc.wantN16InAPI)
			}
		})
	}
}

// ==================== A-4 提案 ====================

func TestProposeResponsibility_SendsOnlyTheThreeDeclaredFields(t *testing.T) {
	f := &fakeResponsibilityClient{
		proposeResult: &pb.ProposeResponsibilityResolutionForAdminResponse{
			Code: 0,
			Data: &pb.ProposeResponsibilityResolutionForAdminData{
				ResolutionId:              "R-9",
				CaseNo:                    "EXC-1",
				CaseStatus:                "IN_PROGRESS",
				ImpactPreviewRefundAmount: "480",
				ImpactComputedAt:          "2026-09-17T03:00:00Z",
			},
		},
	}
	// 🔴 body 刻意夾帶四個被禁的欄位。它們必須被完全忽略——被覆核的金額由 server 算，
	// 提案者身分由 auth context 決定；任何一個被讀進去，雙人制就只剩形式。
	w := postJSON(t, newResponsibilityHolder(f), proposePath, `{
		"resolutionType":"RESPONSIBILITY_ATTRIBUTE",
		"responsibilityCause":"SELLER_CAUSED",
		"reason":"賣家未於時限內備妥商品",
		"refundAmount":"99999",
		"impactPreview":{"refundAmount":"99999"},
		"proposedByAccountId":"2600000001",
		"platformFeeDelta":"123"
	}`)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	if f.proposeCalls != 1 {
		t.Fatalf("proposeCalls = %d, want 1", f.proposeCalls)
	}
	// 送出去的 proto 只有四個欄位（caseNo 來自 path + 三個 body 欄位），夾帶的金額
	// 與身分沒有任何落腳處——這由 proto 保證，這裡斷言的是 gateway 沒有另闢蹊徑。
	if got := f.lastPropose.GetResponsibilityCause(); got != "SELLER_CAUSED" {
		t.Errorf("responsibilityCause = %q", got)
	}
	if got := f.lastPropose.GetCaseNo(); got != "EXC-1" {
		t.Errorf("caseNo = %q, want EXC-1（來自 path param）", got)
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是物件；body = %v", body)
	}
	// 🔴 回應的金額必須是 server 算出來的那個，不是請求夾帶的 99999。
	if data["impactPreviewRefundAmount"] != "480" {
		t.Errorf("impactPreviewRefundAmount = %v, want \"480\"（server 算的值）",
			data["impactPreviewRefundAmount"])
	}
	if data["resolutionId"] != "R-9" {
		t.Errorf("resolutionId = %v, want R-9", data["resolutionId"])
	}
}

func TestProposeResponsibility_RejectsMissingRequiredFields(t *testing.T) {
	for name, body := range map[string]string{
		"缺 resolutionType":      `{"responsibilityCause":"SELLER_CAUSED","reason":"x"}`,
		"缺 responsibilityCause": `{"resolutionType":"RESPONSIBILITY_ATTRIBUTE","reason":"x"}`,
		"缺 reason":              `{"resolutionType":"RESPONSIBILITY_ATTRIBUTE","responsibilityCause":"SELLER_CAUSED"}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeResponsibilityClient{}
			w := postJSON(t, newResponsibilityHolder(f), proposePath, body)
			assertHTTPStatus(t, w, http.StatusBadRequest)
			if f.proposeCalls != 0 {
				t.Fatalf("不完整的提案被送到下游 %d 次", f.proposeCalls)
			}
		})
	}
}

// ==================== request DTO 的欄位釘子 ====================

// 🔴 斷言欄位集合**恰好相等**而不是「不含某些禁字」：後者擋不住拼寫變體
// （refundAmt／previewAmount／operatorId…），而那些變體造成的傷害一模一樣。
//
// 這支測試對應 payment-service 那邊釘住 FinalizeCommand 的反射契約測試：兩端各有一道，
// 因為兩端各自都可能長回那些欄位，而只要有一端長回來，覆核者就能自己決定被覆核的數字。
func TestResponsibilityRequestDTOsFieldSetsArePinned(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want []string
	}{
		{
			// 🔴 不得出現：impactPreview／refundAmount／proposedByAccountId／
			// platformFeeDelta／sellerNetDelta。
			name: "ProposeResponsibilityResolutionRequest",
			typ:  reflect.TypeOf(ProposeResponsibilityResolutionRequest{}),
			want: []string{"reason", "resolutionType", "responsibilityCause"},
		},
		{
			// 🔴 不得出現：responsibilityCause／orderNo／shipmentId／shippingMethod／
			// refundAmount／reviewedByAccountId。加回任何一欄，雙人制就退化成
			// 「有兩個人碰過這張單」。
			name: "ReviewResponsibilityResolutionRequest",
			typ:  reflect.TypeOf(ReviewResponsibilityResolutionRequest{}),
			want: []string{"decision", "reviewComment"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for i := 0; i < tc.typ.NumField(); i++ {
				tag := tc.typ.Field(i).Tag.Get("json")
				name, _, _ := strings.Cut(tag, ",")
				if name == "" {
					name = tc.typ.Field(i).Name
				}
				got = append(got, name)
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s 的欄位集合 = %v, want %v —— 多出來的欄位會讓被覆核的數字或身分"+
					"由呼叫端決定；少掉的欄位則是端點壞了", tc.name, got, want)
			}
		})
	}
}
