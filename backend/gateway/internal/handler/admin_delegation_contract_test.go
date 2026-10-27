package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 56 號代行營運工作階段端點（S1–S6）的回應形狀契約。
//
// 這裡守的每一條，壞掉之後後端都仍然回 200：前端在 parser 階段就把整份回應判成契約
// 錯誤（502），操作者看到的是「代行不可用」或「工作階段與剛建立的稽核上下文不相符」，
// 而 log 裡什麼都沒有。這種失敗只有機械斷言擋得住。

// newDelegationTestContext 建一個可直接餵給 handler 的 gin context。
func newDelegationTestContext(t *testing.T, method, target, body string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	return w, c
}

// decodeDelegationEnvelope 取出 envelope 的 code 與 data（data 保留原始 bytes 才驗得了「欄位在
// 不在」——先解成 map 再看，null 與缺席就分不出來了）。
func decodeDelegationEnvelope(t *testing.T, body []byte) (int32, json.RawMessage) {
	t.Helper()
	var env struct {
		Code int32           `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("回應不是合法 JSON：%v（body=%s）", err, body)
	}
	return env.Code, env.Data
}

// delegationJSONKeys 回傳 JSON object 的 key 集合（排序後）。
func delegationJSONKeys(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("不是 JSON object：%v（raw=%s）", err, raw)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sampleSessionSummary() *accountpb.OperationSessionSummary {
	return &accountpb.OperationSessionSummary{
		SessionId:   "2701445580000001",
		DisplayCode: "OPS-000QLKYYT2O01",
		Target: &accountpb.DelegationTarget{
			StoreId:         "2685914682",
			SellerAccountId: "2685914682",
			StoreName:       "梨山盛花園",
		},
		Actor:        &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
		Reason:       &accountpb.DelegationReason{Code: "seller_request", Label: "賣家請求（管理員記錄）", CaseReferenceRequired: true},
		Capabilities: []string{"storefront.read", "products.read"},
		IssuedAt:     "2026-08-09T06:12:30Z",
		ExpiresAt:    "2026-08-09T06:42:30Z",
		Status:       "active",
		Revision:     0,
	}
}

// TestDelegationSessionData_CaseReferenceNullSurvives 是全案最容易被序列化器吃掉的
// 一欄：caseReference 為 null 時欄位必須真的出現在 JSON 裡。
//
// 鑑別力：把 DelegationSessionData.CaseReference 的 tag 改成 `json:"caseReference,omitempty"`，
// 本測試必須失敗。欄位消失（undefined）＝前端 502 ＝操作者進不了工作區。
func TestDelegationSessionData_CaseReferenceNullSurvives(t *testing.T) {
	summary := sampleSessionSummary() // 未設定 case_reference
	b, err := json.Marshal(toDelegationSessionData(summary))
	if err != nil {
		t.Fatalf("序列化失敗：%v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("不是 JSON object：%v", err)
	}
	raw, ok := m["caseReference"]
	if !ok {
		t.Fatalf("caseReference 欄位消失（undefined）；JSON=%s", b)
	}
	if string(raw) != "null" {
		t.Fatalf("caseReference 應為 null，實際為 %s", raw)
	}
}

// 空字串是合法的 case_reference 值嗎？不是（值域 4–200），但「設定成空字串」與
// 「未設定」在 proto3 optional 上是可分的兩件事，投影必須忠實反映，不能把空字串
// 悄悄變成 null——那會讓 create 與 current 的九欄對帳在一邊出現 "" 一邊出現 null。
func TestDelegationSessionData_EmptyCaseReferenceIsNotNull(t *testing.T) {
	summary := sampleSessionSummary()
	summary.CaseReference = proto.String("")
	b, _ := json.Marshal(toDelegationSessionData(summary))
	if !strings.Contains(string(b), `"caseReference":""`) {
		t.Fatalf("顯式空字串被改寫了；JSON=%s", b)
	}
}

// TestDelegationSessionData_KeySet 鎖住 SessionSummary 的欄位集合。
//
// 多一欄：credential／token 一旦被誰加進來，這裡就轉紅（C11/F7 的機械保證）。
// 少一欄：九欄對帳所需的欄位漂移，前端在建立後的全等比對必失敗。
func TestDelegationSessionData_KeySet(t *testing.T) {
	b, _ := json.Marshal(toDelegationSessionData(sampleSessionSummary()))
	got := delegationJSONKeys(t, b)
	want := []string{
		"actor", "capabilities", "caseReference", "displayCode", "expiresAt",
		"issuedAt", "reason", "revision", "sessionId", "status", "target",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("SessionSummary 欄位集合漂移：\ngot  %v\nwant %v", got, want)
	}
	// 九欄對帳所依賴的欄位逐個點名：漏任一個，操作者建立完就進不了工作區。
	for _, k := range []string{"caseReference", "issuedAt", "expiresAt", "status", "revision"} {
		if !strings.Contains(string(b), `"`+k+`"`) {
			t.Fatalf("九欄對帳欄位 %s 不在回應內；JSON=%s", k, b)
		}
	}
}

// capabilities 為空時必須輸出 []，不得是 null：前端在進入 status 分支之前就對它做
// 非空校驗，null 會讓整份回應被判契約錯誤。
func TestDelegationSessionData_CapabilitiesNeverNull(t *testing.T) {
	summary := sampleSessionSummary()
	summary.Capabilities = nil
	b, _ := json.Marshal(toDelegationSessionData(summary))
	if !strings.Contains(string(b), `"capabilities":[]`) {
		t.Fatalf("capabilities 應為 []，實際 JSON=%s", b)
	}
}

// 連續兩次投影必須位元等價（C8/C16）：本端點每 30 秒被輪詢，任一欄在兩次之間變動
// 都會讓建立後的九欄全等比對失敗。
func TestDelegationSessionData_ProjectionIsStable(t *testing.T) {
	summary := sampleSessionSummary()
	first, _ := json.Marshal(toDelegationSessionData(summary))
	second, _ := json.Marshal(toDelegationSessionData(summary))
	if string(first) != string(second) {
		t.Fatalf("兩次投影不等價：\n1: %s\n2: %s", first, second)
	}
}

// S4 的 data 恰為 {"ended": true}，不得附加任何其他 key。
func TestEndCurrentDelegatedOperation_NoCredentialStillEnds(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodDelete, "/api/v1/admin/delegated-operations/current", "")
	h := &Holder{}
	h.EndCurrentDelegatedOperation(c)

	if w.Code != http.StatusOK {
		t.Fatalf("credential 缺席時必須回 200（回 401／404 會把工作區寫入永久封鎖），實際 %d", w.Code)
	}
	code, data := decodeDelegationEnvelope(t, w.Body.Bytes())
	if code != 0 {
		t.Fatalf("業務碼應為 0，實際 %d", code)
	}
	if keys := delegationJSONKeys(t, data); strings.Join(keys, ",") != "ended" {
		t.Fatalf("data 必須恰為 {\"ended\":true}，實際 keys=%v", keys)
	}
	if !strings.Contains(string(data), `"ended":true`) {
		t.Fatalf("ended 必須為 true，實際 %s", data)
	}
}

// S3 缺 credential → 9274 / 401（不是 404：404 會與「endpoint 沒部署」混淆）。
func TestGetCurrentDelegatedOperation_MissingCredential(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/current", "")
	h := &Holder{}
	h.GetCurrentDelegatedOperation(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("HTTP 應為 401，實際 %d", w.Code)
	}
	code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
	if code != bizcode.DelegationCredentialInvalid {
		t.Fatalf("業務碼應為 %d，實際 %d", bizcode.DelegationCredentialInvalid, code)
	}
}

// S6 的失敗一律 9282 / 410，四種成因共用同一個碼；body 壞掉或格式不符也走同一個出口
// ——任何能區分「格式對但不存在」與「格式就不對」的回應都是一支探測器。
func TestRedeemDelegationHandoff_InvalidCodeAlwaysGone(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"body 非 JSON", "not-json"},
		{"缺欄位", `{}`},
		{"空字串", `{"handoffCode":""}`},
		{"前綴錯（用了 credential 的 dops_）", `{"handoffCode":"dops_9tJ0mQx7Kd2ZfVb3Rn8sYuC1Ea6Ph4Lw5Ti0Og2NkJ8"}`},
		{"長度不足", `{"handoffCode":"dhof_short"}`},
		{"含非 base64url 字元", `{"handoffCode":"dhof_4kR2sPy8Mf3ZgWc5Tn9uZvD1Fb7Qi6Nx8Uj0Rh3Pm!2"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodPost,
				"/api/v1/admin/delegated-operations/handoff/redeem", tc.body)
			h := &Holder{}
			h.RedeemDelegationHandoff(c)

			if w.Code != http.StatusGone {
				t.Fatalf("HTTP 應為 410，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationHandoffInvalid {
				t.Fatalf("業務碼應為 %d，實際 %d", bizcode.DelegationHandoffInvalid, code)
			}
		})
	}
}

// S1／S2／S5 的 ADM 身分缺席 → 9261 / 401（www host 組的 9274 絕不出現在這三支上）。
func TestDelegationAdminEndpoints_RequireAdminIdentity(t *testing.T) {
	h := &Holder{}
	cases := []struct {
		name   string
		invoke func(c *gin.Context)
	}{
		{"preflight", h.GetDelegatedOperationPreflight},
		{"create", h.CreateDelegatedOperation},
		{"revoke", h.RevokeDelegatedOperation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodPost, "/api/v1/admin/delegated-operations", `{}`)
			tc.invoke(c)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("HTTP 應為 401，實際 %d", w.Code)
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationAdminSessionRequired {
				t.Fatalf("業務碼應為 9261，實際 %d", code)
			}
		})
	}
}

// S2 的逐欄值域：每一列都是「照 mockup 或照直覺送就會踩到」的那一種。
func TestCreateDelegatedOperation_BodyValidation(t *testing.T) {
	const goodKey = "8f0e1c9a-2b74-4c11-9a1e-5d6f7a8b9c01" // gitleaks:allow（UUID 形式的冪等鍵範例）
	body := func(extra string) string {
		return `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["storefront.read"],"ttlSeconds":1800` + extra + `}`
	}
	cases := []struct {
		name     string
		key      string
		body     string
		wantCode int32
	}{
		{"缺 Idempotency-Key", "", body(`,"caseReference":"HK-2026-0817"`), bizcode.DelegationInvalidCommand},
		{"Idempotency-Key 太短", "abc", body(`,"caseReference":"HK-2026-0817"`), bizcode.DelegationInvalidCommand},
		{"caseReference 整個缺席", goodKey, body(``), bizcode.DelegationInvalidCommand},
		{"caseReference 沒有數字", goodKey, body(`,"caseReference":"seller call"`), bizcode.DelegationInvalidCommand},
		{"caseReference 太短", goodKey, body(`,"caseReference":"12"`), bizcode.DelegationInvalidCommand},
		{"storeId 含斜線", goodKey, `{"storeId":"268/914","reasonCode":"seller_request","capabilities":["storefront.read"],"ttlSeconds":1800,"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"reasonCode 缺席", goodKey, `{"storeId":"2685914682","capabilities":["storefront.read"],"ttlSeconds":1800,"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"capabilities 空陣列", goodKey, `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":[],"ttlSeconds":1800,"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"capabilities 有未知 token", goodKey, `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["orders.write"],"ttlSeconds":1800,"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"capabilities 重複", goodKey, `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["posts.read","posts.read"],"ttlSeconds":1800,"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"ttlSeconds 缺席", goodKey, `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["storefront.read"],"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"ttlSeconds 越界", goodKey, `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["storefront.read"],"ttlSeconds":1801,"caseReference":null}`, bizcode.DelegationInvalidCommand},
		{"ttlSeconds 送 string", goodKey, `{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["storefront.read"],"ttlSeconds":"1800","caseReference":null}`, bizcode.DelegationInvalidCommand},

		// 依賴閉包（#2744）：授權列端一直有這道檢查，工作階段端沒有 —— 而它只驗
		// 「已知／不重複」，子集本身可以不自洽。9287 而不是 9260：前端要據此自動補上
		// 缺的那幾項，而 9260 只告訴他「參數不正確」。
		//
		// 沒有這道檢查的後果不是「少一道」，是**把失敗延後並換掉主體**：工作階段建得
		// 起來、憑證發出去，操作者進到編輯畫面才被 orchestrator 的 ReadBefore 以 9262
		// 擋下，而 9262 的文案是「此代行工作階段未取得該項操作權限」。
		{"只勾 products.write（缺 products.read）→ 9287", goodKey,
			`{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["products.write"],"ttlSeconds":1800,"caseReference":null}`,
			bizcode.DelegationCapabilityDependencyIncomplete},
		{"只勾 posts.publish（缺兩項）→ 9287", goodKey,
			`{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["posts.publish"],"ttlSeconds":1800,"caseReference":null}`,
			bizcode.DelegationCapabilityDependencyIncomplete},
		{"依賴補齊後不再被本道擋下（走到下一道 ttl 檢查）", goodKey,
			`{"storeId":"2685914682","reasonCode":"seller_request","capabilities":["products.read","products.write"],"ttlSeconds":0,"caseReference":null}`,
			bizcode.DelegationInvalidCommand},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodPost, "/api/v1/admin/delegated-operations", tc.body)
			c.Set("accountId", int64(2701338845))
			if tc.key != "" {
				c.Request.Header.Set("Idempotency-Key", tc.key)
			}
			h := &Holder{}
			h.CreateDelegatedOperation(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("HTTP 應為 400，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != tc.wantCode {
				t.Fatalf("業務碼應為 %d，實際 %d", tc.wantCode, code)
			}
		})
	}
}

// S5：sessionId 形狀不對 ＝ 那個工作階段不存在（9277 / 410），不是 9260；
// revokeReason 缺失或超長才是 9260。
func TestRevokeDelegatedOperation_PathAndBody(t *testing.T) {
	t.Run("sessionId 非數字 → 9277/410", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPost,
			"/api/v1/admin/delegated-operations/abc/revoke", `{"revokeReason":"測試"}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "session_id", Value: "abc"}}
		c.Request.Header.Set("Idempotency-Key", "8f0e1c9a-2b74-4c11-9a1e-5d6f7a8b9c01") // gitleaks:allow（UUID 形式的冪等鍵範例）
		h := &Holder{}
		h.RevokeDelegatedOperation(c)

		if w.Code != http.StatusGone {
			t.Fatalf("HTTP 應為 410，實際 %d", w.Code)
		}
		code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
		if code != bizcode.DelegationSessionNotFound {
			t.Fatalf("業務碼應為 9277，實際 %d", code)
		}
	})

	t.Run("revokeReason 空白 → 9260/400", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPost,
			"/api/v1/admin/delegated-operations/2701445580000003/revoke", `{"revokeReason":"   "}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "session_id", Value: "2701445580000003"}}
		c.Request.Header.Set("Idempotency-Key", "8f0e1c9a-2b74-4c11-9a1e-5d6f7a8b9c01") // gitleaks:allow（UUID 形式的冪等鍵範例）
		h := &Holder{}
		h.RevokeDelegatedOperation(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("HTTP 應為 400，實際 %d", w.Code)
		}
		code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
		if code != bizcode.DelegationInvalidCommand {
			t.Fatalf("業務碼應為 9260，實際 %d", code)
		}
	})

	t.Run("缺 Idempotency-Key → 9260/400", func(t *testing.T) {
		w, c := newDelegationTestContext(t, http.MethodPost,
			"/api/v1/admin/delegated-operations/2701445580000003/revoke", `{"revokeReason":"測試"}`)
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "session_id", Value: "2701445580000003"}}
		h := &Holder{}
		h.RevokeDelegatedOperation(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("HTTP 應為 400，實際 %d", w.Code)
		}
	})
}

// S1：storeId 值域（含 `/`、控制字元、超長）→ 9260 / 400。
func TestGetDelegatedOperationPreflight_StoreIDValidation(t *testing.T) {
	for _, storeID := range []string{"", " 2685914682", "268/914", "268\\914", "26859\x0014682", strings.Repeat("9", 129)} {
		w, c := newDelegationTestContext(t, http.MethodGet,
			"/api/v1/admin/sellers/x/delegated-operation/preflight", "")
		c.Set("accountId", int64(2701338845))
		c.Params = gin.Params{{Key: "store_id", Value: storeID}}
		h := &Holder{}
		h.GetDelegatedOperationPreflight(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("storeId=%q 應為 400，實際 %d", storeID, w.Code)
		}
	}
}

// credential 讀完必須從請求上移除：它是「單獨持有即可操作」的憑證，暴露面要壓到最小。
func TestDelegationCredential_HeaderStripped(t *testing.T) {
	_, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/current", "")
	c.Request.Header.Set(HeaderOperationCredential, "dops_test")
	if got := delegationCredential(c); got != "dops_test" {
		t.Fatalf("應讀得到 credential，實際 %q", got)
	}
	if left := c.Request.Header.Get(HeaderOperationCredential); left != "" {
		t.Fatalf("credential header 應已被移除，實際還在：%q", left)
	}
}

// preflight 的兩個「只在拒絕時出現」的欄位：允許時不得出現（前端只在 allowed:false
// 分支讀它們），拒絕時必須出現且非零。
func TestDelegationPreflightData_DeniedFieldsOnlyWhenDenied(t *testing.T) {
	allowed, _ := json.Marshal(DelegationPreflightData{
		Allowed:       true,
		ReasonOptions: []DelegationReasonOptionData{{Code: "seller_request", Label: "賣家請求"}},
		Capabilities:  []string{"storefront.read"},
		TTLSeconds:    1800,
		IdleSeconds:   900,
	})
	if strings.Contains(string(allowed), "deniedCode") || strings.Contains(string(allowed), "deniedLabel") {
		t.Fatalf("allowed:true 不該出現拒絕欄位：%s", allowed)
	}

	denied, _ := json.Marshal(DelegationPreflightData{
		Allowed:       false,
		ReasonOptions: []DelegationReasonOptionData{{Code: "seller_request", Label: "賣家請求"}},
		Capabilities:  []string{"storefront.read"},
		TTLSeconds:    1800,
		IdleSeconds:   900,
		DeniedCode:    bizcode.DelegationStoreNotOperable,
		DeniedLabel:   "此店家目前停權中，無法代行",
	})
	if !strings.Contains(string(denied), `"deniedCode":9272`) {
		t.Fatalf("allowed:false 必須帶 deniedCode：%s", denied)
	}
	// ttlSeconds／idleSeconds 是 JSON number（值域安全例外）：送 string 直接 502。
	if !strings.Contains(string(denied), `"ttlSeconds":1800`) || !strings.Contains(string(denied), `"idleSeconds":900`) {
		t.Fatalf("ttlSeconds／idleSeconds 必須是 JSON number：%s", denied)
	}
}

// create 只交出 handoff code；credential 只從 redeem 交給工作區。
func TestDelegationCreateAndRedeem_CredentialExposureIsOnlyRedeem(t *testing.T) {
	create, _ := json.Marshal(DelegationCreateData{
		HandoffCode: "dhof_y",
		Session:     toDelegationSessionData(sampleSessionSummary()),
	})
	if keys := delegationJSONKeys(t, create); strings.Join(keys, ",") != "handoffCode,session" {
		t.Fatalf("create data 必須恰兩欄並列，實際 keys=%v", keys)
	}
	replayed, err := json.Marshal(DelegationCreateData{Session: toDelegationSessionData(sampleSessionSummary())})
	if err != nil {
		t.Fatal(err)
	}
	if keys := delegationJSONKeys(t, replayed); strings.Join(keys, ",") != "session" {
		t.Fatalf("已兌換後重放只回 session，實際 keys=%v", keys)
	}
	redeem, _ := json.Marshal(DelegationRedeemData{
		OperationCredential: "dops_x",
		Session:             toDelegationSessionData(sampleSessionSummary()),
	})
	if keys := delegationJSONKeys(t, redeem); strings.Join(keys, ",") != "operationCredential,session" {
		t.Fatalf("redeem data 必須是 credential ＋ session（summary 不攤平），實際 keys=%v", keys)
	}
	// session 這一層絕不得帶出憑證。
	session, _ := json.Marshal(toDelegationSessionData(sampleSessionSummary()))
	for _, forbidden := range []string{"operationCredential", "handoffCode", "accessToken", "refreshToken", "sellerToken"} {
		if strings.Contains(string(session), forbidden) {
			t.Fatalf("SessionSummary 不得含 %s：%s", forbidden, session)
		}
	}
}

// 業務碼經 gRPC status 傳回時（message 帶 "[NNNN] " 前綴）必須還原成該碼與登錄表決定
// 的 HTTP status，不得一律塌成 424／9280。
//
// 這條是實打抓到的：Gateway 的 EnvelopeGuardInterceptor 把「gRPC OK ＋ resp.Code != 0」
// 統一轉成 FailedPrecondition + "[9282] …"，第一版 handler 只看 resp.GetCode()、把 err
// 一律當成上游故障 ⇒ handoff code 重放回 424／9280，前端走「結果不明」而不是
// 「此連結已失效，返回 ADM」。代行的每一個業務拒絕都走這一側。
func TestDelegationGrpcFailure_RestoresBizCode(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int32
	}{
		{
			"帶業務碼前綴 → 還原 9282／410",
			grpcstatus.Error(grpccodes.FailedPrecondition, "[9282] 此代行連結已失效，請回管理後台重新開始"),
			http.StatusGone, bizcode.DelegationHandoffInvalid,
		},
		{
			"帶業務碼前綴 → 還原 9268／409（登錄表沒收的碼塌縮 409，與既有行為一致）",
			grpcstatus.Error(grpccodes.FailedPrecondition, "[9268] 您已有進行中的代行工作階段"),
			http.StatusConflict, bizcode.DelegationConflictActorOtherStore,
		},
		{
			"沒有業務碼 → 9280／424",
			grpcstatus.Error(grpccodes.Unavailable, "connection refused"),
			http.StatusFailedDependency, bizcode.DelegationUpstreamUnavailable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodPost, "/api/v1/admin/delegated-operations", "")
			delegationGrpcFailure(c, "測試", tc.err)
			if w.Code != tc.wantStatus {
				t.Fatalf("HTTP 應為 %d，實際 %d（body=%s）", tc.wantStatus, w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != tc.wantCode {
				t.Fatalf("業務碼應為 %d，實際 %d", tc.wantCode, code)
			}
		})
	}
}
