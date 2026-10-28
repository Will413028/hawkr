package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// #2444 授權列治理七支端點的契約。
//
// 這裡守的每一條，壞掉之後後端仍然回 200：ADM 的 parser 會在解析階段把整份回應判成
// 契約錯誤（502），管理員看到的是「載入失敗」而 log 裡什麼都沒有。純靠 review 看不出
// 少一個 omitempty 的後果，只有機械斷言擋得住。

// ==================== stub ====================

type authorizationStub struct {
	accountpb.DelegationServiceClient

	grants  []*accountpb.GrantDelegationAuthorizationRequest
	updates []*accountpb.UpdateDelegationAuthorizationRequest
	revokes []*accountpb.RevokeDelegationAuthorizationRequest
	lists   []*accountpb.ListDelegationAuthorizationsRequest
}

func (s *authorizationStub) ListDelegationAuthorizations(ctx context.Context, in *accountpb.ListDelegationAuthorizationsRequest, opts ...grpc.CallOption) (*accountpb.ListDelegationAuthorizationsResponse, error) {
	s.lists = append(s.lists, in)
	return &accountpb.ListDelegationAuthorizationsResponse{
		Code:     0,
		Records:  []*accountpb.DelegationAuthorization{sampleAuthorization()},
		Total:    "1",
		Page:     "1",
		PageSize: "20",
	}, nil
}

func (s *authorizationStub) GetDelegationAuthorization(ctx context.Context, in *accountpb.GetDelegationAuthorizationRequest, opts ...grpc.CallOption) (*accountpb.GetDelegationAuthorizationResponse, error) {
	return &accountpb.GetDelegationAuthorizationResponse{
		Code: 0,
		Data: &accountpb.GetDelegationAuthorizationData{
			Exists:             true,
			Authorization:      sampleAuthorization(),
			ActiveSessionCount: "2",
		},
	}, nil
}

func (s *authorizationStub) GrantDelegationAuthorization(ctx context.Context, in *accountpb.GrantDelegationAuthorizationRequest, opts ...grpc.CallOption) (*accountpb.GrantDelegationAuthorizationResponse, error) {
	s.grants = append(s.grants, in)
	return &accountpb.GrantDelegationAuthorizationResponse{
		Code: 0,
		Data: &accountpb.GrantDelegationAuthorizationData{
			Authorization: sampleAuthorization(),
			Outcome:       "created",
		},
	}, nil
}

func (s *authorizationStub) UpdateDelegationAuthorization(ctx context.Context, in *accountpb.UpdateDelegationAuthorizationRequest, opts ...grpc.CallOption) (*accountpb.UpdateDelegationAuthorizationResponse, error) {
	s.updates = append(s.updates, in)
	return &accountpb.UpdateDelegationAuthorizationResponse{
		Code: 0,
		Data: &accountpb.UpdateDelegationAuthorizationData{Authorization: sampleAuthorization()},
	}, nil
}

func (s *authorizationStub) RevokeDelegationAuthorization(ctx context.Context, in *accountpb.RevokeDelegationAuthorizationRequest, opts ...grpc.CallOption) (*accountpb.RevokeDelegationAuthorizationResponse, error) {
	s.revokes = append(s.revokes, in)
	return &accountpb.RevokeDelegationAuthorizationResponse{
		Code: 0,
		Data: &accountpb.RevokeDelegationAuthorizationData{
			Authorization:  sampleAuthorization(),
			AlreadyRevoked: false,
		},
	}, nil
}

func (s *authorizationStub) ListDelegationAuthorizationHistory(ctx context.Context, in *accountpb.ListDelegationAuthorizationHistoryRequest, opts ...grpc.CallOption) (*accountpb.ListDelegationAuthorizationHistoryResponse, error) {
	return &accountpb.ListDelegationAuthorizationHistoryResponse{
		Code:     0,
		Records:  []*accountpb.DelegationAuthorizationAuditEntry{sampleAuthorizationAuditEntry()},
		Total:    "1",
		Page:     "1",
		PageSize: "20",
	}, nil
}

func sampleAuthorization() *accountpb.DelegationAuthorization {
	return &accountpb.DelegationAuthorization{
		Id:                         "7495464541774413824",
		TargetStoreId:              "2685914682",
		Source:                     "contract",
		SourceReference:            "HK-CONTRACT-2026-0031",
		GrantedCapabilities:        []string{"products.read", "products.write"},
		EffectiveFrom:              "2026-08-19T02:00:00Z",
		ApprovedByAdminId:          "2701338845",
		ApprovedByAdminDisplayName: "carl@example.com",
		ApprovedAt:                 "2026-08-19T02:00:00Z",
		Effective:                  true,
		CreatedAt:                  "2026-08-19T02:00:00Z",
		UpdatedAt:                  "2026-08-19T02:00:00Z",
	}
}

func sampleAuthorizationAuditEntry() *accountpb.DelegationAuthorizationAuditEntry {
	return &accountpb.DelegationAuthorizationAuditEntry{
		Id:                "7495464600540807168",
		AuthorizationId:   "7495464541774413824",
		TargetStoreId:     "2685914682",
		Action:            "GRANT",
		ActionLabel:       "開通授權",
		ActorAdminId:      "2701338845",
		ActorDisplayName:  "carl@example.com",
		OperationReason:   "依合約開通商品代管",
		CapabilitiesAfter: []string{"products.read", "products.write"},
		CreatedAt:         "2026-08-19T02:00:00Z",
	}
}

func newAuthorizationHolder(stub *authorizationStub) *Holder {
	return &Holder{Account: grpcclient.NewAccountDelegationClientForTest(stub)}
}

const authorizationIdempotencyKey = "8f0e1c9a-2b74-4c11-9a1e-5d6f7a8b9c01"

func validGrantBody(extra string) string {
	return `{"storeId":"2685914682","source":"contract","sourceReference":"HK-CONTRACT-2026-0031",` +
		`"grantedCapabilities":["products.read","products.write"],"operationReason":"依合約開通商品代管"` + extra + `}`
}

// ==================== 形狀契約 ====================

// 欄位集合全等。多一欄：任何憑證／PII 被誰加進來時這裡轉紅。少一欄：ADM 的 parser
// 在必填欄缺席時整份判 502，面板打不開。
func TestDelegationAuthorizationData_KeySet(t *testing.T) {
	b, _ := json.Marshal(toDelegationAuthorizationData(sampleAuthorization()))
	got := delegationJSONKeys(t, b)
	want := []string{
		"approvedAt", "approvedByAdminId", "approvedByAdminName", "createdAt",
		"effective", "effectiveFrom", "effectiveUntil", "grantedCapabilities",
		"id", "revokeReason", "revokedAt", "revokedByAdminId", "revokedByAdminName",
		"source", "sourceReference", "storeId", "updatedAt",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("授權列欄位集合漂移：\ngot  %v\nwant %v", got, want)
	}
}

// 五個可空欄位為 null 時必須真的出現在 JSON 裡。
//
// 鑑別力：對任一欄加上 `,omitempty`，本測試必須失敗。欄位消失（undefined）之後，
// 「這列沒有被撤回」與「後端忘了回撤回資訊」在前端上完全一樣，而面板據此決定要不要
// 顯示「撤回」按鈕。
func TestDelegationAuthorizationData_NullableFieldsSurvive(t *testing.T) {
	b, _ := json.Marshal(toDelegationAuthorizationData(sampleAuthorization()))
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("不是 JSON object：%v", err)
	}
	for _, k := range []string{"effectiveUntil", "revokedAt", "revokedByAdminId", "revokedByAdminName", "revokeReason"} {
		raw, ok := m[k]
		if !ok {
			t.Fatalf("%s 欄位消失（undefined）；JSON=%s", k, b)
		}
		if string(raw) != "null" {
			t.Fatalf("%s 應為 null，實際 %s", k, raw)
		}
	}

	// 有值時原樣帶出，不得被改寫成別的形狀。
	withValues := sampleAuthorization()
	withValues.EffectiveUntil = proto.String("2026-12-31T23:59:59Z")
	withValues.RevokedAt = proto.String("2026-09-01T00:00:00Z")
	withValues.RevokedByAdminId = proto.String("2701338846")
	withValues.RevokedByAdminDisplayName = proto.String("amy@example.com")
	withValues.RevokeReason = proto.String("合約終止")
	vb, _ := json.Marshal(toDelegationAuthorizationData(withValues))
	for _, want := range []string{
		`"effectiveUntil":"2026-12-31T23:59:59Z"`, `"revokedAt":"2026-09-01T00:00:00Z"`,
		`"revokedByAdminId":"2701338846"`, `"revokedByAdminName":"amy@example.com"`,
		`"revokeReason":"合約終止"`,
	} {
		if !strings.Contains(string(vb), want) {
			t.Fatalf("缺 %s；JSON=%s", want, vb)
		}
	}
}

// 陣列欄位為空時輸出 []，不得是 null——前端在渲染之前對它做 .map／.length。
func TestDelegationAuthorizationData_ArraysNeverNull(t *testing.T) {
	a := sampleAuthorization()
	a.GrantedCapabilities = nil
	b, _ := json.Marshal(toDelegationAuthorizationData(a))
	if !strings.Contains(string(b), `"grantedCapabilities":[]`) {
		t.Fatalf("grantedCapabilities 應為 []，實際 JSON=%s", b)
	}

	e := sampleAuthorizationAuditEntry()
	e.CapabilitiesBefore, e.CapabilitiesAfter = nil, nil
	eb, _ := json.Marshal(toDelegationAuthorizationHistoryEntry(e))
	if !strings.Contains(string(eb), `"capabilitiesBefore":[]`) || !strings.Contains(string(eb), `"capabilitiesAfter":[]`) {
		t.Fatalf("稽核列的兩個能力陣列應為 []，實際 JSON=%s", eb)
	}

	rb, _ := json.Marshal(DelegationAuthorizationRevokeData{
		Authorization:     toDelegationAuthorizationData(sampleAuthorization()),
		RevokedSessionIDs: []string{},
	})
	if !strings.Contains(string(rb), `"revokedSessionIds":[]`) {
		t.Fatalf("revokedSessionIds 應為 []，實際 JSON=%s", rb)
	}
}

// 兩次投影必須位元等價：面板會在撤回前後各抓一次做比對，任一欄在兩次之間變動都會
// 讓「這次操作改了什麼」算錯。
func TestDelegationAuthorizationData_ProjectionIsStable(t *testing.T) {
	a := sampleAuthorization()
	first, _ := json.Marshal(toDelegationAuthorizationData(a))
	second, _ := json.Marshal(toDelegationAuthorizationData(a))
	if string(first) != string(second) {
		t.Fatalf("兩次投影不等價：\n1: %s\n2: %s", first, second)
	}
}

func TestDelegationAuthorizationHistoryEntry_KeySet(t *testing.T) {
	b, _ := json.Marshal(toDelegationAuthorizationHistoryEntry(sampleAuthorizationAuditEntry()))
	got := delegationJSONKeys(t, b)
	want := []string{
		"action", "actionLabel", "actorAdminId", "actorDisplayName", "authorizationId",
		"capabilitiesAfter", "capabilitiesBefore", "createdAt", "effectiveUntilAfter",
		"effectiveUntilBefore", "id", "operationReason", "source", "sourceReference", "storeId",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("稽核列欄位集合漂移：\ngot  %v\nwant %v", got, want)
	}
}

// ==================== 1｜能力目錄 ====================

// 目錄必須涵蓋全部九值、依宣告順序、每一項都帶 risk 與 dependsOn（後者恆為陣列）。
// 少任何一項，ADM 的勾選畫面就少一個能力而沒有任何錯誤；dependsOn 是 null 的話連動
// 規則整段不執行，使用者送出的組合會被後端以 9287 擋下而畫面說不出為什麼。
func TestListDelegationCapabilities_Catalog(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/capabilities", "")
	c.Set("accountId", int64(2701338845))
	(&Holder{}).ListDelegationCapabilities(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	_, data := decodeDelegationEnvelope(t, w.Body.Bytes())
	var out DelegationCapabilityCatalogData
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("data 解析失敗：%v", err)
	}
	if len(out.Capabilities) != 9 {
		t.Fatalf("目錄應有 9 個能力，實得 %d", len(out.Capabilities))
	}

	byValue := map[string]DelegationCapabilityDescriptor{}
	for _, d := range out.Capabilities {
		byValue[d.Value] = d
		if d.Risk != "low" && d.Risk != "medium" && d.Risk != "high" {
			t.Errorf("%s 的 risk = %q，必須是 low／medium／high", d.Value, d.Risk)
		}
	}
	if got := byValue["products.publish"].DependsOn; len(got) != 2 || got[0] != "products.read" || got[1] != "products.write" {
		t.Errorf("products.publish 的 dependsOn = %v", got)
	}
	if got := byValue["orders.read"]; got.Risk != "high" {
		t.Errorf("orders.read 必須是 high（唯讀但每次揭露都落 PII 稽核），實際 %q", got.Risk)
	}
	// 無依賴者序列化成 []，不是 null。
	if !strings.Contains(string(data), `"value":"orders.read","risk":"high","dependsOn":[]`) {
		t.Errorf("無依賴的能力必須輸出 dependsOn:[]；JSON=%s", data)
	}
}

// ==================== 身分 ====================

// 七支全部掛在 ADM host 組：身分缺席一律 9261／401，絕不出現 www host 組的 9274。
func TestDelegationAuthorizationEndpoints_RequireAdminIdentity(t *testing.T) {
	h := &Holder{}
	cases := []struct {
		name   string
		invoke func(c *gin.Context)
	}{
		{"capabilities", h.ListDelegationCapabilities},
		{"list", h.ListDelegationAuthorizations},
		{"get", h.GetDelegationAuthorization},
		{"grant", h.GrantDelegationAuthorization},
		{"update", h.UpdateDelegationAuthorization},
		{"revoke", h.RevokeDelegationAuthorization},
		{"history", h.ListDelegationAuthorizationHistory},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodPost,
				"/api/v1/admin/delegated-operations/authorizations", `{}`)
			c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
			tc.invoke(c)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("HTTP 應為 401，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationAdminSessionRequired {
				t.Fatalf("業務碼應為 9261，實際 %d", code)
			}
		})
	}
}

// ==================== 4｜開通的值域 ====================

func TestGrantDelegationAuthorization_BodyValidation(t *testing.T) {
	cases := []struct {
		name       string
		key        string
		body       string
		wantStatus int
		wantCode   int32
	}{
		{"缺 Idempotency-Key", "", validGrantBody(""), http.StatusBadRequest, bizcode.DelegationInvalidCommand},
		{"Idempotency-Key 太短", "abc", validGrantBody(""), http.StatusBadRequest, bizcode.DelegationInvalidCommand},
		{
			"未知欄位不得被靜默忽略（effectiveUntil 拼錯＝永久授權）",
			authorizationIdempotencyKey,
			validGrantBody(`,"effectiveUnitl":"2026-12-31T23:59:59Z"`),
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"storeId 含斜線",
			authorizationIdempotencyKey,
			`{"storeId":"268/914","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["products.read"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"source 不在三值內",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"verbal","sourceReference":"HK-2026-0031","grantedCapabilities":["products.read"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"sourceReference 沒有數字（＝說明文字而不是依據編號）",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"seller asked","grantedCapabilities":["products.read"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"sourceReference 太短",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"12","grantedCapabilities":["products.read"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"grantedCapabilities 空陣列",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":[],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"grantedCapabilities 有未知 token",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["orders.write"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"grantedCapabilities 重複",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["products.read","products.read"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"依賴不齊 → 9287 而不是 9260（前端要據此自動補上缺的那幾項）",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["products.write"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationCapabilityDependencyIncomplete,
		},
		{
			"依賴不齊（publish 缺兩項）",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["posts.publish"],"operationReason":"x1"}`,
			http.StatusBadRequest, bizcode.DelegationCapabilityDependencyIncomplete,
		},
		{
			"effectiveUntil 不是 ISO 8601 UTC 秒精度",
			authorizationIdempotencyKey,
			validGrantBody(`,"effectiveUntil":"2026-12-31"`),
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"effectiveUntil 帶時區偏移（非 Z）",
			authorizationIdempotencyKey,
			validGrantBody(`,"effectiveUntil":"2026-12-31T23:59:59+08:00"`),
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
		{
			"operationReason 只有空白",
			authorizationIdempotencyKey,
			`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["products.read"],"operationReason":"   "}`,
			http.StatusBadRequest, bizcode.DelegationInvalidCommand,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodPost,
				"/api/v1/admin/delegated-operations/authorizations", tc.body)
			c.Set("accountId", int64(2701338845))
			if tc.key != "" {
				c.Request.Header.Set("Idempotency-Key", tc.key)
			}
			stub := &authorizationStub{}
			newAuthorizationHolder(stub).GrantDelegationAuthorization(c)

			if w.Code != tc.wantStatus {
				t.Fatalf("HTTP 應為 %d，實際 %d（body=%s）", tc.wantStatus, w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != tc.wantCode {
				t.Fatalf("業務碼應為 %d，實際 %d", tc.wantCode, code)
			}
			// 被擋下的請求一律不得抵達領域服務：驗證的意義正是「在稽核與寫入之前失敗」。
			if len(stub.grants) != 0 {
				t.Fatalf("驗證失敗的請求仍被送往 delegation 模組：%v", stub.grants[0])
			}
		})
	}
}

// 9287 的訊息必須指名缺了哪幾項——只說「組合不完整」的話，前端只能請人自己猜。
func TestGrantDelegationAuthorization_DependencyMessageNamesMissing(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodPost,
		"/api/v1/admin/delegated-operations/authorizations",
		`{"storeId":"2685914682","source":"contract","sourceReference":"HK-2026-0031","grantedCapabilities":["products.publish"],"operationReason":"x1"}`)
	c.Set("accountId", int64(2701338845))
	c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
	newAuthorizationHolder(&authorizationStub{}).GrantDelegationAuthorization(c)

	body := w.Body.String()
	if !strings.Contains(body, "products.read") || !strings.Contains(body, "products.write") {
		t.Fatalf("9287 的訊息未指名缺少的能力：%s", body)
	}
}

// happy path：送出去的 gRPC request 逐欄正確，且回應形狀正確。
//
// 這一支測的是「Gateway 送了什麼」而不只是「回了什麼」：只斷言回應的測試在
// operationReason 沒被傳下去時依然全綠，而那一欄是稽核列的 NOT NULL 欄位。
func TestGrantDelegationAuthorization_ForwardsAndProjects(t *testing.T) {
	stub := &authorizationStub{}
	w, c := newDelegationTestContext(t, http.MethodPost,
		"/api/v1/admin/delegated-operations/authorizations",
		validGrantBody(`,"effectiveUntil":"2026-12-31T23:59:59Z"`))
	c.Set("accountId", int64(2701338845))
	c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
	newAuthorizationHolder(stub).GrantDelegationAuthorization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	if len(stub.grants) != 1 {
		t.Fatalf("應恰好呼叫一次 GrantDelegationAuthorization，實際 %d 次", len(stub.grants))
	}
	got := stub.grants[0]
	if got.GetActorAdminId() != "2701338845" {
		t.Errorf("actorAdminId = %q，必須是 ADM session 解析出的管理員", got.GetActorAdminId())
	}
	if got.GetTargetStoreId() != "2685914682" || got.GetSource() != "contract" ||
		got.GetSourceReference() != "HK-CONTRACT-2026-0031" || got.GetOperationReason() != "依合約開通商品代管" {
		t.Errorf("授權欄位未逐欄傳遞：%+v", got)
	}
	if got.GetIdempotencyKey() != authorizationIdempotencyKey {
		t.Errorf("冪等鍵未傳遞：%q", got.GetIdempotencyKey())
	}
	if got.EffectiveUntil == nil || got.GetEffectiveUntil() != "2026-12-31T23:59:59Z" {
		t.Errorf("effectiveUntil 未傳遞：%v", got.EffectiveUntil)
	}

	_, data := decodeDelegationEnvelope(t, w.Body.Bytes())
	if keys := delegationJSONKeys(t, data); strings.Join(keys, ",") != "authorization,outcome" {
		t.Fatalf("開通回應必須恰兩欄，實際 keys=%v", keys)
	}
}

// effectiveUntil 未送 ＝ 無期限：不得被填成空字串（空字串在 proto3 optional 上是
// 「有設定且為空」，DB 會拿它去 parse 時間而炸）。
func TestGrantDelegationAuthorization_AbsentEffectiveUntilStaysUnset(t *testing.T) {
	stub := &authorizationStub{}
	w, c := newDelegationTestContext(t, http.MethodPost,
		"/api/v1/admin/delegated-operations/authorizations", validGrantBody(""))
	c.Set("accountId", int64(2701338845))
	c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
	newAuthorizationHolder(stub).GrantDelegationAuthorization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	if stub.grants[0].EffectiveUntil != nil {
		t.Fatalf("未送 effectiveUntil 時必須維持未設定，實際 %q", stub.grants[0].GetEffectiveUntil())
	}
}

// ==================== 3｜單店詳情 ====================

func TestGetDelegationAuthorization_LookupShape(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet,
		"/api/v1/admin/delegated-operations/authorizations/2685914682", "")
	c.Set("accountId", int64(2701338845))
	c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
	newAuthorizationHolder(&authorizationStub{}).GetDelegationAuthorization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	_, data := decodeDelegationEnvelope(t, w.Body.Bytes())
	if keys := delegationJSONKeys(t, data); strings.Join(keys, ",") != "activeSessionCount,authorization,exists" {
		t.Fatalf("單店詳情欄位集合漂移，實際 keys=%v", keys)
	}
	if !strings.Contains(string(data), `"activeSessionCount":"2"`) {
		t.Fatalf("activeSessionCount 未下發（撤回二次確認的唯一來源）：%s", data)
	}
}

// exists=false 是正常狀態：回 200、authorization 為 null（不得是欄位消失），
// activeSessionCount 仍必須是可解析的數字字串。
func TestGetDelegationAuthorization_NotGrantedIsNotAnError(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet,
		"/api/v1/admin/delegated-operations/authorizations/2685914682", "")
	c.Set("accountId", int64(2701338845))
	c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
	h := &Holder{Account: grpcclient.NewAccountDelegationClientForTest(&emptyLookupStub{})}
	h.GetDelegationAuthorization(c)

	if w.Code != http.StatusOK {
		t.Fatalf("查無授權列必須回 200（用 404 會與『上游掛了』混淆），實際 %d", w.Code)
	}
	_, data := decodeDelegationEnvelope(t, w.Body.Bytes())
	if !strings.Contains(string(data), `"exists":false`) || !strings.Contains(string(data), `"authorization":null`) {
		t.Fatalf("exists=false 時 authorization 必須是 null 且欄位仍在：%s", data)
	}
	if !strings.Contains(string(data), `"activeSessionCount":"0"`) {
		t.Fatalf("計數欄缺席時必須補 \"0\"（空字串會讓前端算出 NaN）：%s", data)
	}
}

// emptyLookupStub 只覆寫單店查詢，回一個沒有授權列也沒有計數的回應。
type emptyLookupStub struct {
	accountpb.DelegationServiceClient
}

func (s *emptyLookupStub) GetDelegationAuthorization(ctx context.Context, in *accountpb.GetDelegationAuthorizationRequest, opts ...grpc.CallOption) (*accountpb.GetDelegationAuthorizationResponse, error) {
	return &accountpb.GetDelegationAuthorizationResponse{
		Code: 0,
		Data: &accountpb.GetDelegationAuthorizationData{Exists: false},
	}, nil
}

// ==================== 2／7｜分頁與篩選 ====================

func TestListDelegationAuthorizations_QueryValidation(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"status 不在二分值域內", "?status=revoked"},
		{"status 多值", "?status=effective,ineffective"},
		{"page 為 0", "?page=0"},
		{"page_size 越界（不靜默夾值）", "?page_size=51"},
		{"store_id 含反斜線", `?store_id=268\914`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodGet,
				"/api/v1/admin/delegated-operations/authorizations"+tc.query, "")
			c.Set("accountId", int64(2701338845))
			stub := &authorizationStub{}
			newAuthorizationHolder(stub).ListDelegationAuthorizations(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("HTTP 應為 400，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationInvalidCommand {
				t.Fatalf("業務碼應為 9260，實際 %d", code)
			}
			if len(stub.lists) != 0 {
				t.Fatal("驗證失敗的查詢仍被送往 delegation 模組")
			}
		})
	}
}

// 分頁一律走 response.Page：data 恰為 {records,total,page,pageSize}。handler 自組
// gin.H 必然漂出 page_size 這種 snake 變體，前端讀通用 data.pageSize 就拿到 undefined。
func TestListDelegationAuthorizations_PageEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		invoke func(h *Holder, c *gin.Context)
	}{
		{"授權列表", func(h *Holder, c *gin.Context) { h.ListDelegationAuthorizations(c) }},
		{"授權歷程", func(h *Holder, c *gin.Context) { h.ListDelegationAuthorizationHistory(c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodGet,
				"/api/v1/admin/delegated-operations/authorizations", "")
			c.Set("accountId", int64(2701338845))
			c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
			tc.invoke(newAuthorizationHolder(&authorizationStub{}), c)

			if w.Code != http.StatusOK {
				t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			_, data := decodeDelegationEnvelope(t, w.Body.Bytes())
			if keys := delegationJSONKeys(t, data); strings.Join(keys, ",") != "page,pageSize,records,total" {
				t.Fatalf("分頁 data 必須恰為 records/total/page/pageSize，實際 keys=%v", keys)
			}
		})
	}
}

// 列表篩選必須原樣送到領域服務。靜默丟掉篩選條件的症狀是「畫面說在看有效授權、
// 實際列出全部」，而列表本身仍然是 200。
func TestListDelegationAuthorizations_ForwardsFilters(t *testing.T) {
	stub := &authorizationStub{}
	_, c := newDelegationTestContext(t, http.MethodGet,
		"/api/v1/admin/delegated-operations/authorizations?status=ineffective&store_id=2685914682&page=2&page_size=5", "")
	c.Set("accountId", int64(2701338845))
	newAuthorizationHolder(stub).ListDelegationAuthorizations(c)

	if len(stub.lists) != 1 {
		t.Fatalf("應恰好呼叫一次列表，實際 %d 次", len(stub.lists))
	}
	got := stub.lists[0]
	if got.GetStatus() != "ineffective" || got.GetTargetStoreId() != "2685914682" ||
		got.GetPage() != "2" || got.GetPageSize() != "5" {
		t.Fatalf("篩選與分頁未逐欄傳遞：%+v", got)
	}
}

// ==================== 5／6｜寫入路徑的共通值域 ====================

func TestUpdateAndRevokeDelegationAuthorization_Validation(t *testing.T) {
	t.Run("update 的 storeId 走 path，格式錯即 9260", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPut,
			"/api/v1/admin/delegated-operations/authorizations/x", `{}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "store_id", Value: "268/914"}}
		c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
		stub := &authorizationStub{}
		newAuthorizationHolder(stub).UpdateDelegationAuthorization(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("HTTP 應為 400，實際 %d", w.Code)
		}
		if len(stub.updates) != 0 {
			t.Fatal("驗證失敗的請求仍被送往 delegation 模組")
		}
	})

	t.Run("update 同樣強制依賴閉包（9287）", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPut,
			"/api/v1/admin/delegated-operations/authorizations/2685914682",
			`{"source":"ticket","sourceReference":"HK-2026-0031","grantedCapabilities":["posts.write"],"operationReason":"縮小範圍"}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
		c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
		newAuthorizationHolder(&authorizationStub{}).UpdateDelegationAuthorization(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("HTTP 應為 400，實際 %d（body=%s）", w.Code, w.Body.String())
		}
		code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
		if code != bizcode.DelegationCapabilityDependencyIncomplete {
			t.Fatalf("業務碼應為 9287，實際 %d", code)
		}
	})

	t.Run("revoke 缺理由即 9260（撤回三欄同生同滅）", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPost,
			"/api/v1/admin/delegated-operations/authorizations/2685914682/revoke", `{"revokeReason":"   "}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
		c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
		stub := &authorizationStub{}
		newAuthorizationHolder(stub).RevokeDelegationAuthorization(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("HTTP 應為 400，實際 %d", w.Code)
		}
		code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
		if code != bizcode.DelegationInvalidCommand {
			t.Fatalf("業務碼應為 9260，實際 %d", code)
		}
		if len(stub.revokes) != 0 {
			t.Fatal("驗證失敗的撤回仍被送往 delegation 模組")
		}
	})

	t.Run("revoke 缺 Idempotency-Key 即 9260", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPost,
			"/api/v1/admin/delegated-operations/authorizations/2685914682/revoke", `{"revokeReason":"合約終止"}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
		newAuthorizationHolder(&authorizationStub{}).RevokeDelegationAuthorization(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("HTTP 應為 400，實際 %d", w.Code)
		}
	})

	t.Run("revoke happy path 的回應恰三欄", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPost,
			"/api/v1/admin/delegated-operations/authorizations/2685914682/revoke", `{"revokeReason":"合約終止"}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
		c.Request.Header.Set("Idempotency-Key", authorizationIdempotencyKey)
		stub := &authorizationStub{}
		newAuthorizationHolder(stub).RevokeDelegationAuthorization(c)

		if w.Code != http.StatusOK {
			t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
		}
		_, data := decodeDelegationEnvelope(t, w.Body.Bytes())
		if keys := delegationJSONKeys(t, data); strings.Join(keys, ",") != "alreadyRevoked,authorization,revokedSessionIds" {
			t.Fatalf("撤回回應欄位集合漂移，實際 keys=%v", keys)
		}
		// 上游沒給時仍必須是 []：UI 要拿它算「已同時中止 N 個工作階段」。
		if !strings.Contains(string(data), `"revokedSessionIds":[]`) {
			t.Fatalf("revokedSessionIds 必須是 []，實際 %s", data)
		}
		if stub.revokes[0].GetRevokeReason() != "合約終止" {
			t.Fatalf("撤回理由未傳遞：%q", stub.revokes[0].GetRevokeReason())
		}
	})
}

// TestDelegationCapabilityCatalogIsAdminOnlyByDesign 釘住「端點 1 不驗 allowlist」這個
// **刻意的**設計，不是待補的漏洞。
//
// 2026-08-19 的 dev 實測：一個 delegation_enabled=false 的管理員打
// GET /admin/delegated-operations/capabilities 會拿到 200 ＋ 完整能力目錄。
// 那不是 bug，補上閘門才是錯的修法，三個理由缺一不可：
//
//  1. 這支端點**不打領域服務**（整個 handler 只讀 authz package 的常數），而 allowlist
//     只存在於 delegation 模組的 DB ⇒ 要驗就得為一支回傳九個常數字串的端點多付一次
//     gRPC 往返。
//  2. 它回的內容**零租戶資料**，而且 ADM 前端 bundle 本來就把同一份能力名稱送到每個
//     管理員的瀏覽器（hawkr-admin 的 labels.ts）⇒ 加閘門買不到任何機密性。
//  3. **流程上游已經 fail-closed**：授權面板先打單店詳情（端點 3，有 allowlist 且回
//     9288），能力目錄只在對話框開啟時才取 ⇒ 未 allowlist 的管理員在面板層就被擋下，
//     對話框根本開不了。
//
// ⇒ 本測試斷言的是「它只要求身分、不要求 allowlist」。若日後真要改成 allowlist-gated，
// 必須連同 §6.1.15 的共通掛載規約與 9288 的說明一起改，並想清楚第 1 點那次往返值不值得。
func TestDelegationCapabilityCatalogIsAdminOnlyByDesign(t *testing.T) {
	h := &Holder{}

	// 有身分即可取得完整目錄——不需要任何 allowlist，也不需要任何 gRPC client
	// （h.deps 為零值，若實作偷偷加了下游呼叫，這裡會 panic 或非 200）。
	w, c := newDelegationTestContext(t, http.MethodGet,
		"/api/v1/admin/delegated-operations/capabilities", "")
	c.Set("accountId", int64(2661027599))
	h.ListDelegationCapabilities(c)

	if w.Code != http.StatusOK {
		t.Fatalf("有 admin 身分即應回 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
	if code != 0 {
		t.Fatalf("業務碼應為 0，實際 %d —— 若這裡變成 9288，表示有人把 allowlist 閘門加了上去；"+
			"那是契約變更，請先讀本測試上方的三個理由", code)
	}
}
