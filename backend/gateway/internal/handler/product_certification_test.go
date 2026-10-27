package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 19 號設計書 §6.2 / §6.2.1：非法認證輸入一律「剔除該筆 ＋ warn log」，**不回 4xx**。
//
// 這一條沒有任何自然訊號會在它壞掉時響：改成 400 之後，後端測試照樣全綠、CI 照樣過，
// 症狀只出現在**已上架 App** 的 AI 上架流程——助手 patch 的 certNo 繞過畫面驗證原樣送出，
// 從今天起整個儲存動作失敗。所以這裡把兩個方向都釘死：
//   ① 非法輸入 → 200 且那一筆不在送往 gRPC 的 request 內（不是 400、也不是照送）
//   ② 合法輸入 → 照樣送下去（證明 ① 的斷言不是「什麼都被剔掉」造成的恆真）

// certCapturingProductStub 記下最後一次送往 product-service 的 create / update request。
type certCapturingProductStub struct {
	productpb.ProductServiceClient
	lastCreate *productpb.CreateProductRequest
	lastUpdate *productpb.UpdateProductRequest
	// upstreamWarnings 模擬收斂點判定出來的 CONCURRENT_CONFLICT。
	upstreamWarnings []*productpb.CertificationWarning
}

func (s *certCapturingProductStub) CreateProduct(_ context.Context, in *productpb.CreateProductRequest, _ ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
	s.lastCreate = in
	return &productpb.CreateProductResponse{
		Code:                  0,
		Data:                  &productpb.ProductDetail{Id: "843856384063471755", Name: in.GetName(), Version: "1"},
		CertificationWarnings: s.upstreamWarnings,
	}, nil
}

func (s *certCapturingProductStub) UpdateProduct(_ context.Context, in *productpb.UpdateProductRequest, _ ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
	s.lastUpdate = in
	return &productpb.UpdateProductResponse{
		Code:                  0,
		Data:                  &productpb.ProductDetail{Id: in.GetProductId(), Version: "2"},
		CertificationWarnings: s.upstreamWarnings,
	}, nil
}

func newSellerCertRouter(stub *certCapturingProductStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Holder{Product: grpcclient.NewProductClientForTest(stub), Store: storeResolverForTest(2685914682)}
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.POST("/products", h.CreateSellerProduct)
	app.PUT("/products/:id", h.UpdateSellerProduct)
	return r
}

func doSellerJSON(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "2685914682")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// certWarningsFromBody 取出 data.certificationWarnings，並回報該 key 是否存在。
func certWarningsFromBody(t *testing.T, w *httptest.ResponseRecorder) ([]map[string]any, bool) {
	t.Helper()
	var env struct {
		Code int32           `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v（body=%s）", err, w.Body.String())
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v（body=%s）", err, w.Body.String())
	}
	raw, present := data["certificationWarnings"]
	if !present {
		return nil, false
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal certificationWarnings: %v（raw=%s）", err, string(raw))
	}
	return out, true
}

const certValidSku = `{"skuName":"600g","unit":"3301","price":"350","stockQty":"20",` +
	`"weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10"}`

func TestCreateSellerProduct_IllegalCertificationIsDroppedNotRejected(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	// 三筆：一筆合法、一筆 type 非法（送了中文標籤而不是碼值）、一筆 certNo 含空白。
	body := `{"idempotencyKey":"k-cert-1","name":"有機小松菜","categoryId":"10001",` +
		`"skus":[` + certValidSku + `],` +
		`"certifications":[` +
		`{"name":"3701","certNo":"1-009-300022"},` +
		`{"name":"產銷履歷","certNo":"58589605051"},` +
		`{"name":"3703","certNo":"010 105"}` +
		`]}`
	w := doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", body)

	// ① 非法輸入不得變成 400 —— 這正是會弄壞已上架 App 的那個行為。
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200（非法認證必須剔除而不是拒絕整個請求）；body = %s", w.Code, w.Body.String())
	}
	if stub.lastCreate == nil {
		t.Fatal("CreateProduct 沒有被呼叫，下面的斷言會空過")
	}

	// ② 被剔掉的那兩筆不得離開 Gateway。
	got := stub.lastCreate.GetCertifications()
	if len(got) != 1 {
		t.Fatalf("送往 gRPC 的 certifications 筆數 = %d, want 1；實際 = %+v", len(got), got)
	}
	if got[0].GetName() != "3701" || got[0].GetCertNo() != "1-009-300022" {
		t.Errorf("留下來的應該是合法那筆，實際 = {name:%q certNo:%q}", got[0].GetName(), got[0].GetCertNo())
	}

	// ③ 每一筆被剔除的輸入都要有一個 warning，且原樣回送賣家送來的值。
	warnings, present := certWarningsFromBody(t, w)
	if !present {
		t.Fatal("回應缺少 certificationWarnings —— 前端分不出「後端沒實作」與「這次沒有警告」")
	}
	if len(warnings) != 2 {
		t.Fatalf("certificationWarnings 筆數 = %d, want 2；實際 = %+v", len(warnings), warnings)
	}
	if warnings[0]["reason"] != certWarningInvalidType {
		t.Errorf("第一筆 reason = %v, want %s", warnings[0]["reason"], certWarningInvalidType)
	}
	if warnings[0]["certificationType"] != "產銷履歷" {
		t.Errorf("warning 必須原樣回送送進來的值，實際 certificationType = %v", warnings[0]["certificationType"])
	}
	if warnings[1]["reason"] != certWarningInvalidCertNo {
		t.Errorf("第二筆 reason = %v, want %s", warnings[1]["reason"], certWarningInvalidCertNo)
	}
	if warnings[1]["certificateNo"] != "010 105" {
		t.Errorf("warning 必須原樣回送送進來的值，實際 certificateNo = %v", warnings[1]["certificateNo"])
	}
}

// 控制組：證明上面那支的「只剩一筆」不是因為 filter 把什麼都剔掉了。
// 同時釘住「無警告時是空陣列不是省略」（19 §9.5）。
func TestCreateSellerProduct_LegalCertificationsPassThroughWithEmptyWarnings(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	body := `{"idempotencyKey":"k-cert-2","name":"有機小松菜","categoryId":"10001",` +
		`"skus":[` + certValidSku + `],` +
		`"certifications":[{"name":"3701","certNo":"1-009-300022"},{"name":"3706","certNo":"GMP_2026-01"}]}`
	w := doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", body)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	if n := len(stub.lastCreate.GetCertifications()); n != 2 {
		t.Fatalf("合法認證應原樣送下去，實際筆數 = %d", n)
	}
	// 3706（GMP）不在可線上驗證的三型內，但它是合法值域——拿 verifiableCertTypes 當
	// 值域會把它剔掉，這個斷言就是釘住那個錯法。
	if stub.lastCreate.GetCertifications()[1].GetName() != "3706" {
		t.Errorf("3706（GMP）被誤剔了：實際第二筆 = %q", stub.lastCreate.GetCertifications()[1].GetName())
	}

	warnings, present := certWarningsFromBody(t, w)
	if !present {
		t.Fatal("無警告時 certificationWarnings 仍必須存在（空陣列），不得省略")
	}
	if len(warnings) != 0 {
		t.Errorf("certificationWarnings 應為空陣列，實際 = %+v", warnings)
	}
}

// 上游（收斂點）判定出來的 CONCURRENT_CONFLICT 必須一起帶回前端，不能只回 Gateway 自己
// 剔掉的那些——漏了它，賣家會看到「儲存成功」但某一張標章沒出現，且沒有任何解釋。
func TestCreateSellerProduct_MergesUpstreamCertificationWarnings(t *testing.T) {
	stub := &certCapturingProductStub{upstreamWarnings: []*productpb.CertificationWarning{{
		CertificationType: "3703",
		CertificateNo:     "010105",
		Reason:            "CONCURRENT_CONFLICT",
		Message:           "另一個視窗剛送出同類型的認證，這一筆未被採用，請重新儲存",
	}}}
	r := newSellerCertRouter(stub)

	body := `{"idempotencyKey":"k-cert-3","name":"有機小松菜","categoryId":"10001",` +
		`"skus":[` + certValidSku + `],` +
		`"certifications":[{"name":"3703","certNo":"010105"},{"name":"HACCP","certNo":"x"}]}`
	w := doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", body)

	warnings, _ := certWarningsFromBody(t, w)
	if len(warnings) != 2 {
		t.Fatalf("Gateway 剔除 1 筆 ＋ 上游 1 筆 = 2，實際 = %d（%+v）", len(warnings), warnings)
	}
	if warnings[1]["reason"] != "CONCURRENT_CONFLICT" {
		t.Errorf("上游 warning 沒有被帶回來：實際第二筆 = %+v", warnings[1])
	}
}

func TestUpdateSellerProduct_IllegalCertificationIsDroppedNotRejected(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	body := `{"idempotencyKey":"k-cert-4","certifications":[` +
		`{"name":"3702","certNo":"TAP-2025-0032"},` +
		`{"name":"3702","certNo":"這不是合法編號"}` +
		`]}`
	w := doSellerJSON(t, r, http.MethodPut, "/api/v1/app/products/843856384063471755", body)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	if stub.lastUpdate == nil {
		t.Fatal("UpdateProduct 沒有被呼叫，下面的斷言會空過")
	}
	got := stub.lastUpdate.GetCertifications()
	if len(got) != 1 || got[0].GetCertNo() != "TAP-2025-0032" {
		t.Fatalf("送往 gRPC 的 certifications = %+v, want 只留合法那一筆", got)
	}
	// 剔除不得影響「這次要全量取代」的旗標——那個旗標看的是 body 有沒有帶欄位。
	if !stub.lastUpdate.GetUpdateCertifications() {
		t.Error("updateCertifications 必須維持 true：body 帶了 certifications 就是要全量取代")
	}
	if warnings, _ := certWarningsFromBody(t, w); len(warnings) != 1 {
		t.Errorf("certificationWarnings 筆數 = %d, want 1", len(warnings))
	}
}

// 筆數上限是**另一件事**：它擋的是 DoS，超量回 400 是正確行為（19 §6.2.1 的 ✅ 那條）。
// 這支測試防的是「為了讓非法值不回 400，順手把筆數上限也拿掉」。
func TestCreateSellerProduct_CertificationCountLimitStillRejects(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	certs := make([]string, 0, 11)
	for i := 0; i < 11; i++ {
		certs = append(certs, `{"name":"3701","certNo":"1-009-30002`+string(rune('0'+i%10))+`"}`)
	}
	body := `{"idempotencyKey":"k-cert-5","name":"有機小松菜","categoryId":"10001",` +
		`"skus":[` + certValidSku + `],"certifications":[` + strings.Join(certs, ",") + `]}`
	w := doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("HTTP status = %d, want 400（11 筆超過上限 10）；body = %s", w.Code, w.Body.String())
	}
	if stub.lastCreate != nil {
		t.Error("超量請求不得送到 product-service")
	}
}

// ==================== 代行路徑（W3／W4）====================

func newDelegatedCertRouter(stub *certCapturingProductStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Holder{
		Account: grpcclient.NewAccountDelegationClientForTest(&delegationStub{}),
		Product: grpcclient.NewProductClientForTest(stub),
	}
	r := gin.New()
	g := r.Group("/api/v1/admin")
	g.Use(func(c *gin.Context) {
		middleware.SetDelegationSession(c, &accountpb.OperationSessionSummary{
			SessionId: "2701445580000002",
			Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
			Target:    &accountpb.DelegationTarget{StoreId: "2685914682"},
			Status:    "active",
		})
		c.Next()
	})
	g.POST("/sellers/:store_id/products", h.CreateDelegatedProduct)
	return r
}

func TestDelegatedProductCreate_IllegalCertificationIsDroppedNotRejected(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newDelegatedCertRouter(stub)

	// 代行走另一個 DTO（DelegatedProductCertification），而 validateProductArrays 只擋
	// 筆數——值域這一層是本次新加的，漏了它非法值會直接落進申請表與 ADM 佇列。
	postDelegated(t, r, "/api/v1/admin/sellers/2685914682/products", "ik-cert-delegated-0001",
		`{"operationReason":"協助新品上架","name":"嘉義布袋鮮蚵","categoryId":"10001",`+
			`"skus":[{"spec":"600g","price":"350","stockQty":"20","unit":"3301"}],`+
			`"certifications":[{"name":"3701","certNo":"1-008-131005"},{"name":"9999","certNo":"x"}]}`)

	if stub.lastCreate == nil {
		t.Fatal("CreateProduct 沒有被呼叫，下面的斷言會空過")
	}
	got := stub.lastCreate.GetCertifications()
	if len(got) != 1 || got[0].GetName() != "3701" {
		t.Fatalf("送往 gRPC 的 certifications = %+v, want 只留 3701 那一筆", got)
	}
}

// ==================== 值域規則本身 ====================

// certificationInputRejection 是賣家與代行兩條路徑共用的唯一規則來源。
// 它必須引用既有的 certNoPattern / certNoMaxLength，不得再抄一份字面值。
func TestCertificationInputRejection_Rules(t *testing.T) {
	long := strings.Repeat("A", certNoMaxLength+1)
	longCJK := strings.Repeat("號", certNoMaxLength)
	cases := []struct {
		name       string
		certType   string
		certNo     string
		wantReason string
	}{
		// ---- type 值域（兩類共用）----
		{"type 是中文標籤", "產銷履歷", "58589605051", certWarningInvalidType},
		{"type 不在 3701–3706", "9999", "1-009-300022", certWarningInvalidType},
		{"type 空字串", "", "1-009-300022", certWarningInvalidType},

		// ---- 可線上驗證三型：嚴格白名單（值會組進農業部 API 的查詢字串）----
		{"3701 合法", "3701", "1-009-300022", ""},
		{"3702 合法", "3702", "TAP-2025-0032", ""},
		{"3701 certNo 空字串", "3701", "", certWarningInvalidCertNo},
		{"3701 certNo 含空白", "3701", "010 105", certWarningInvalidCertNo},
		{"3701 certNo 含 URI 保留字元", "3701", "a&api_key=x", certWarningInvalidCertNo},
		{"3701 certNo 超過長度上限", "3701", long, certWarningInvalidCertNo},
		{"3701 certNo 剛好在長度上限", "3701", long[:certNoMaxLength], ""},

		// ---- 不可線上驗證三型：放寬（值不會離開系統，格式由 ADM 人眼判斷）----
		{"3705 ISO 帶冒號", "3705", "ISO 22000:2018", ""},
		{"3704 HACCP 帶空格與斜線", "3704", "HACCP 2026/01 A", ""},
		{"3706 GMP 中文編號", "3706", "食品GMP認證字第 0047 號", ""},
		{"3706 底線連字號（原本就合法）", "3706", "GMP_2026-01", ""},
		{"3705 全空白視同空", "3705", "   ", certWarningInvalidCertNo},
		{"3705 空字串", "3705", "", certWarningInvalidCertNo},
		{"3705 含控制字元", "3705", "ISO 22000\u0001", certWarningInvalidCertNo},
		{"3705 含換行", "3705", "ISO\n22000", certWarningInvalidCertNo},
		{"3705 含角括號", "3705", "<script>alert(1)</script>", certWarningInvalidCertNo},
		{"3705 trim 後剛好在長度上限", "3705", "  " + long[:certNoMaxLength] + "  ", ""},
		{"3705 trim 後超過長度上限", "3705", "  " + long + "  ", certWarningInvalidCertNo},
		{"3706 中文剛好 64 字（算字元不算位元組）", "3706", longCJK, ""},
		{"3706 中文 65 字", "3706", longCJK + "號", certWarningInvalidCertNo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, message := certificationInputRejection(tc.certType, tc.certNo)
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tc.wantReason)
			}
			if reason != "" && message == "" {
				t.Error("被剔除的輸入必須帶可直接顯示的中文說明，否則前端只能顯示一個代碼")
			}
		})
	}
}

// 🔴 同一個 certNo、兩個 type、相反的結果——這是分流規則最鋒利的形狀（19 §6.2.9）。
//
// `ISO 22000:2018` 是 ISO 認證的真實編號寫法。它對 3705 是合法輸入，對 3701 則不是
// 合法的政府 API 查詢鍵（冒號會原字未編碼被組進查詢字串，後面緊接著 &api_key=）。
// 一套規則走天下的兩個方向都會壞：全嚴格 ⇒ 賣家填對的 ISO 編號被靜默吃掉；
// 全放寬 ⇒ 冒號被送進農業部 API 的 URL。這支測試同時釘住兩邊。
func TestCertificationFilter_CertNoRuleSplitsByType(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	const isoNo = "ISO 22000:2018"
	body := `{"idempotencyKey":"k-cert-split","name":"有機小松菜","categoryId":"10001",` +
		`"skus":[` + certValidSku + `],` +
		`"certifications":[` +
		`{"name":"3701","certNo":"` + isoNo + `"},` + // 可線上驗證 ⇒ 必須剔除
		`{"name":"3705","certNo":"` + isoNo + `"}` + // 不可線上驗證 ⇒ 必須保留
		`]}`
	w := doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", body)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	if stub.lastCreate == nil {
		t.Fatal("CreateProduct 沒有被呼叫，下面的斷言會空過")
	}

	got := stub.lastCreate.GetCertifications()
	if len(got) != 1 {
		t.Fatalf("送往 gRPC 的 certifications 筆數 = %d, want 1（只有 3705 那筆該留）；實際 = %+v", len(got), got)
	}
	if got[0].GetName() != "3705" {
		t.Errorf("留下來的是 %q，want 3705 —— 分流反了：可線上驗證那型才是要嚴格擋的", got[0].GetName())
	}
	if got[0].GetCertNo() != isoNo {
		t.Errorf("3705 的 certNo = %q, want %q（放寬類必須原值送下去，不得被改寫）", got[0].GetCertNo(), isoNo)
	}

	warnings, _ := certWarningsFromBody(t, w)
	if len(warnings) != 1 {
		t.Fatalf("certificationWarnings 筆數 = %d, want 1；實際 = %+v", len(warnings), warnings)
	}
	if warnings[0]["certificationType"] != "3701" || warnings[0]["reason"] != certWarningInvalidCertNo {
		t.Errorf("被剔除的應該是 3701 那筆且原因為 %s，實際 = %+v", certWarningInvalidCertNo, warnings[0])
	}
}

// 放寬三型的常見真實編號必須整批通過並照常建立申請——這是「賣家填對的東西不會被
// 靜默吃掉」的正面證據，缺了它，上面那支只證明了「至少有一筆活著」。
func TestCertificationFilter_UnverifiableTypesAcceptRealWorldFormats(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	body := `{"idempotencyKey":"k-cert-relaxed","name":"冷凍蔬菜","categoryId":"10001",` +
		`"skus":[` + certValidSku + `],` +
		`"certifications":[` +
		`{"name":"3704","certNo":"HACCP 2026/01 A"},` +
		`{"name":"3705","certNo":"ISO 22000:2018"},` +
		`{"name":"3706","certNo":"食品GMP認證字第 0047 號"}` +
		`]}`
	w := doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", body)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	got := stub.lastCreate.GetCertifications()
	if len(got) != 3 {
		t.Fatalf("三筆真實編號應全部通過，實際筆數 = %d（%+v）", len(got), got)
	}
	wantNos := []string{"HACCP 2026/01 A", "ISO 22000:2018", "食品GMP認證字第 0047 號"}
	for i, want := range wantNos {
		if got[i].GetCertNo() != want {
			t.Errorf("第 %d 筆 certNo = %q, want %q", i+1, got[i].GetCertNo(), want)
		}
	}
	if warnings, _ := certWarningsFromBody(t, w); len(warnings) != 0 {
		t.Errorf("不該有任何 warning，實際 = %+v", warnings)
	}
}

// certSkuMap 是 certValidSku 的 map 版本，供需要 json.Marshal 組 body 的案例使用：
// certNo 含控制字元時不能用字串拼接，那些字元必須經過 JSON 轉義才送得出去。
func certSkuMap() map[string]string {
	return map[string]string{
		"skuName": "600g", "unit": "3301", "price": "350", "stockQty": "20",
		"weight": "1", "lengthCm": "10", "widthCm": "10", "heightCm": "10",
	}
}

func postSellerProductWithCert(t *testing.T, r *gin.Engine, key, certType, certNo string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"idempotencyKey": key,
		"name":           "冷凍蔬菜",
		"categoryId":     "10001",
		"skus":           []map[string]string{certSkuMap()},
		"certifications": []map[string]string{{"name": certType, "certNo": certNo}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return doSellerJSON(t, r, http.MethodPost, "/api/v1/app/products", string(payload))
}

// 驗過的值與送出的值必須是同一個：放寬類驗的是 trim 後的結果，送出的就得是 trim 後的
// 結果。若送原值，一個從試算表貼進來的尾端換行會通過驗證然後原樣落進 ADM 佇列——
// 驗證看起來有做，被擋的東西卻進去了。
func TestCertificationFilter_UnverifiableCertNoIsTrimmedBeforeSend(t *testing.T) {
	stub := &certCapturingProductStub{}
	r := newSellerCertRouter(stub)

	w := postSellerProductWithCert(t, r, "k-cert-trim", "3705", "  ISO 22000:2018\n")
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
	}
	got := stub.lastCreate.GetCertifications()
	if len(got) != 1 {
		t.Fatalf("筆數 = %d, want 1（貼上來的空白不該讓整筆被剔除）", len(got))
	}
	if got[0].GetCertNo() != "ISO 22000:2018" {
		t.Errorf("送出的 certNo = %q, want %q（驗的是 trim 後的值，送的就必須是同一個）",
			got[0].GetCertNo(), "ISO 22000:2018")
	}
}

// 放寬不等於不驗：控制字元與角括號仍然要擋，且擋在 Gateway（不讓它進申請表與 ADM 佇列）。
func TestCertificationFilter_UnverifiableTypesStillBlockControlCharsAndAngleBrackets(t *testing.T) {
	cases := map[string]string{
		"角括號":    "<img src=x onerror=alert(1)>",
		"控制字元":   "ISO 22000" + "\u0001",
		"字串中間換行": "ISO\n22000",
	}
	for name, certNo := range cases {
		t.Run(name, func(t *testing.T) {
			stub := &certCapturingProductStub{}
			r := newSellerCertRouter(stub)

			w := postSellerProductWithCert(t, r, "k-cert-block", "3705", certNo)
			// 仍然是 200（剔除不是拒絕），但那一筆不得離開 Gateway。
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200；body = %s", w.Code, w.Body.String())
			}
			if n := len(stub.lastCreate.GetCertifications()); n != 0 {
				t.Errorf("送往 gRPC 的 certifications 筆數 = %d, want 0；實際 = %+v", n, stub.lastCreate.GetCertifications())
			}
			warnings, _ := certWarningsFromBody(t, w)
			if len(warnings) != 1 || warnings[0]["reason"] != certWarningInvalidCertNo {
				t.Errorf("應有一筆 %s warning，實際 = %+v", certWarningInvalidCertNo, warnings)
			}
		})
	}
}
