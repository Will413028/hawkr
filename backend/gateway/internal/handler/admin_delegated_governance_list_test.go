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
	"google.golang.org/grpc"
)

// 跨工作階段治理查詢（§11.3 P1）的兩支端點契約。
//
// 這兩支與其他代行端點的差別在於「誰看得到什麼」：它們列的是**全平台**的代行活動，
// 一頁裡的每一列是不同的 admin 與不同的店家。因此除了形狀之外，這裡守的是
// 「篩選條件不得被靜默丟掉」—— 一個被丟掉的 store_id 篩選會讓操作者以為自己看的是
// 一家店，實際拿到的是全部，而回應長得完全正常。

type governanceListStub struct {
	accountpb.DelegationServiceClient

	sessionQueries []*accountpb.ListSessionsRequest
	auditQueries   []*accountpb.ListAllAuditLogsRequest
}

func (s *governanceListStub) ListSessions(ctx context.Context, in *accountpb.ListSessionsRequest, opts ...grpc.CallOption) (*accountpb.ListSessionsResponse, error) {
	s.sessionQueries = append(s.sessionQueries, in)
	return &accountpb.ListSessionsResponse{
		Code:     0,
		Records:  []*accountpb.DelegatedOperationSessionRecord{sampleSessionRecord()},
		Total:    "1",
		Page:     "1",
		PageSize: "20",
	}, nil
}

func (s *governanceListStub) ListAllAuditLogs(ctx context.Context, in *accountpb.ListAllAuditLogsRequest, opts ...grpc.CallOption) (*accountpb.ListAllAuditLogsResponse, error) {
	s.auditQueries = append(s.auditQueries, in)
	return &accountpb.ListAllAuditLogsResponse{
		Code:     0,
		Records:  []*accountpb.AuditLogEntry{},
		Total:    "0",
		Page:     "1",
		PageSize: "20",
	}, nil
}

func newGovernanceListHolder(stub *governanceListStub) *Holder {
	return &Holder{Account: grpcclient.NewAccountDelegationClientForTest(stub)}
}

func sampleSessionRecord() *accountpb.DelegatedOperationSessionRecord {
	reason := "本人請求代為調整商品"
	revokedAt := "2026-08-20T04:15:00Z"
	revokedBy := "2701338845"
	revokedByName := "ops@example.com"
	return &accountpb.DelegatedOperationSessionRecord{
		SessionId:   "2701445580000001",
		DisplayCode: "OPS-000QLKYYT2O01",
		Target: &accountpb.DelegationTarget{
			StoreId: "2685914682", SellerAccountId: "2685914682", StoreName: "梨山盛花園",
		},
		Actor:        &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "ops@example.com"},
		Reason:       &accountpb.DelegationReason{Code: "seller_request", Label: "賣家請求", CaseReferenceRequired: true},
		Capabilities: []string{"products.read", "products.write"},
		IssuedAt:     "2026-08-20T04:00:00Z",
		ExpiresAt:    "2026-08-20T04:30:00Z",
		Status:       "revoked",
		RevokedAt:    &revokedAt,

		RevokedByAdminId:          &revokedBy,
		RevokedByAdminDisplayName: &revokedByName,
		RevokeReason:              &reason,
	}
}

// ==================== 1｜值域：越界一律 9260，不夾值、不忽略 ====================

func TestListDelegatedOperations_QueryValidation(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"page 為 0", "?page=0"},
		{"page_size 越界（不靜默夾值）", "?page_size=51"},
		{"page_size 為 0", "?page_size=0"},
		{"actor_admin_id 非數字", "?actor_admin_id=abc"},
		{"store_id 含反斜線", `?store_id=268\914`},
		{"display_code 形狀不符", "?display_code=OPS-123"},
		{"date_from 非 ISO 8601", "?date_from=2026-08-20"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodGet,
				"/api/v1/admin/delegated-operations"+tc.query, "")
			stub := &governanceListStub{}
			newGovernanceListHolder(stub).ListDelegatedOperations(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("HTTP 應為 400，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationInvalidCommand {
				t.Fatalf("業務碼應為 9260，實際 %d", code)
			}
			if len(stub.sessionQueries) != 0 {
				t.Fatal("驗證失敗的查詢仍被送往 account-service")
			}
		})
	}
}

func TestListAllDelegatedAuditLogs_QueryValidation(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"page_size 越界（不靜默夾值）", "?page_size=51"},
		{"actor_admin_id 非數字", "?actor_admin_id=x1"},
		{"store_id 含反斜線", `?store_id=268\914`},
		{"result 不在值域內", "?result=OK"},
		{"result 多值", "?result=SUCCEEDED,FAILED"},
		{"resource_type 不在值域內", "?resource_type=shipment"},
		{"date_to 非 ISO 8601", "?date_to=昨天"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newDelegationTestContext(t, http.MethodGet,
				"/api/v1/admin/delegated-operations/audit-logs"+tc.query, "")
			stub := &governanceListStub{}
			newGovernanceListHolder(stub).ListAllDelegatedAuditLogs(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("HTTP 應為 400，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationInvalidCommand {
				t.Fatalf("業務碼應為 9260，實際 %d", code)
			}
			if len(stub.auditQueries) != 0 {
				t.Fatal("驗證失敗的查詢仍被送往 account-service")
			}
		})
	}
}

// ==================== 2｜篩選條件必須真的送到上游 ====================

// 這一條守的是靜默失效：handler 讀了 query 卻沒填進 request，回應仍是 200 ＋ 一份看起來
// 正常的清單，只是內容是全平台的。沒有任何東西會轉紅，而操作者拿到的是別家店的資料。
func TestListDelegatedOperations_ForwardsEveryFilter(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet,
		"/api/v1/admin/delegated-operations?actor_admin_id=2701338845&store_id=2685914682"+
			"&display_code=OPS-000QLKYYT2O01&date_from=2026-08-01T00:00:00Z"+
			"&date_to=2026-08-31T23:59:59Z&page=2&page_size=5", "")
	stub := &governanceListStub{}
	newGovernanceListHolder(stub).ListDelegatedOperations(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	if len(stub.sessionQueries) != 1 {
		t.Fatalf("上游呼叫次數 = %d，應為 1", len(stub.sessionQueries))
	}
	got := stub.sessionQueries[0]
	for _, tc := range []struct{ field, want, have string }{
		{"actorAdminId", "2701338845", got.GetActorAdminId()},
		{"targetStoreId", "2685914682", got.GetTargetStoreId()},
		{"displayCode", "OPS-000QLKYYT2O01", got.GetDisplayCode()},
		{"dateFrom", "2026-08-01T00:00:00Z", got.GetDateFrom()},
		{"dateTo", "2026-08-31T23:59:59Z", got.GetDateTo()},
		{"page", "2", got.GetPage()},
		{"pageSize", "5", got.GetPageSize()},
	} {
		if tc.have != tc.want {
			t.Errorf("%s 未送達上游：got %q, want %q —— 靜默丟掉篩選會回一份全平台的清單，"+
				"而操作者以為自己看的是一家店", tc.field, tc.have, tc.want)
		}
	}
}

func TestListAllDelegatedAuditLogs_ForwardsEveryFilter(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet,
		"/api/v1/admin/delegated-operations/audit-logs?actor_admin_id=2701338845"+
			"&store_id=2685914682&action=PRODUCT_UPDATE&resource_type=product&result=SUCCEEDED"+
			"&date_from=2026-08-01T00:00:00Z&date_to=2026-08-31T23:59:59Z&page=3&page_size=10", "")
	stub := &governanceListStub{}
	newGovernanceListHolder(stub).ListAllDelegatedAuditLogs(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	if len(stub.auditQueries) != 1 {
		t.Fatalf("上游呼叫次數 = %d，應為 1", len(stub.auditQueries))
	}
	got := stub.auditQueries[0]
	for _, tc := range []struct{ field, want, have string }{
		{"actorAdminId", "2701338845", got.GetActorAdminId()},
		{"targetStoreId", "2685914682", got.GetTargetStoreId()},
		{"actionFilter", "PRODUCT_UPDATE", got.GetActionFilter()},
		{"resourceTypeFilter", "product", got.GetResourceTypeFilter()},
		{"resultFilter", "SUCCEEDED", got.GetResultFilter()},
		{"dateFrom", "2026-08-01T00:00:00Z", got.GetDateFrom()},
		{"dateTo", "2026-08-31T23:59:59Z", got.GetDateTo()},
		{"page", "3", got.GetPage()},
		{"pageSize", "10", got.GetPageSize()},
	} {
		if tc.have != tc.want {
			t.Errorf("%s 未送達上游：got %q, want %q", tc.field, tc.have, tc.want)
		}
	}
}

// ==================== 3｜分頁 envelope 與列形狀 ====================

// 分頁一律走 response.Page：data 恰為 {records,total,page,pageSize}。handler 自組 gin.H
// 必然漂出 page_size 這種 snake 變體，前端讀通用 data.pageSize 就拿到 undefined。
func TestListDelegatedOperations_PageEnvelopeAndRecordShape(t *testing.T) {
	w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations", "")
	newGovernanceListHolder(&governanceListStub{}).ListDelegatedOperations(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 應為 200，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	code, data := decodeDelegationEnvelope(t, w.Body.Bytes())
	if code != 0 {
		t.Fatalf("業務碼應為 0，實際 %d", code)
	}
	if keys := delegationJSONKeys(t, data); strings.Join(keys, ",") != "page,pageSize,records,total" {
		t.Fatalf("分頁 data 必須恰為 records/total/page/pageSize，實際 keys=%v", keys)
	}

	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("分頁 data 解析失敗：%v（data=%s）", err, data)
	}
	if len(payload.Records) != 1 {
		t.Fatalf("records 長度 = %d, want 1", len(payload.Records))
	}
	// 撤銷四件套要真的到得了前端 —— #2483 A9 的整個要點就是它們從來沒有出口。
	for _, key := range []string{"revokedAt", "revokedByAdminId", "revokedByAdminDisplayName", "revokeReason"} {
		v, ok := payload.Records[0][key]
		if !ok {
			t.Errorf("列缺少 %q", key)
			continue
		}
		if v == nil {
			t.Errorf("%q 為 null，但這一列是已撤銷的工作階段", key)
		}
	}
}
