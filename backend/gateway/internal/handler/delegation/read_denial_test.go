package delegation

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
)


func denialSession() *accountpb.OperationSessionSummary {
	return &accountpb.OperationSessionSummary{SessionId: "2701445588392017"}
}

func TestRecordReadDenialShape(t *testing.T) {
	cases := []struct {
		name           string
		in             ReadDenial
		wantAction     string
		wantCapability string
	}{
		{
			name: "capability 留空 → 落回該面的 read",
			in: ReadDenial{
				Session: denialSession(), ResourceType: "product", ResourceID: "77",
				BizCode: bizcode.DelegationResourceNotOwned,
			},
			wantAction: "PRODUCT_READ", wantCapability: "products.read",
		},
		{
			name: "帶 capability → 原樣記錄",
			in: ReadDenial{
				Session: denialSession(), ResourceType: "product", ResourceID: "77",
				Capability: authz.AdminCapProductsPublish, BizCode: bizcode.DelegationCapabilityDenied,
			},
			wantAction: "PRODUCT_READ", wantCapability: "products.publish",
		},
		{
			name: "訂單面",
			in: ReadDenial{
				Session: denialSession(), ResourceType: "order", ResourceID: "O005001417",
				BizCode: bizcode.DelegationTargetMismatch,
			},
			wantAction: "ORDER_READ", wantCapability: "orders.read",
		},
		{
			name: "店面",
			in: ReadDenial{
				Session: denialSession(), ResourceType: "storefront",
				BizCode: bizcode.DelegationTargetMismatch,
			},
			wantAction: "STOREFRONT_READ", wantCapability: "storefront.read",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.RequestID = "req-1"
			tc.in.CorrelationID = "corr-1"
			tc.in.UserAgent = "ADM-E2E/1.0"

			stub := &closureAuditStub{}
			closureOrchestrator(stub).RecordReadDenial(context.Background(), tc.in)

			if len(stub.intents) != 1 || len(stub.confirms) != 1 {
				t.Fatalf("預期恰好 1 對 intent+confirm，實得 %d／%d", len(stub.intents), len(stub.confirms))
			}
			got := stub.intents[0]
			if got.GetAction() != tc.wantAction {
				t.Errorf("action = %q，預期 %q（三個 READ action 是拒絕列唯一的合法值）", got.GetAction(), tc.wantAction)
			}
			if got.GetCapability() != tc.wantCapability {
				t.Errorf("capability = %q，預期 %q", got.GetCapability(), tc.wantCapability)
			}
			if got.GetResourceType() != tc.in.ResourceType || got.GetResourceId() != tc.in.ResourceID {
				t.Errorf("resourceType/resourceId = %q/%q，預期 %q/%q",
					got.GetResourceType(), got.GetResourceId(), tc.in.ResourceType, tc.in.ResourceID)
			}
			if got.GetOperationReason() != "" || got.GetNotifySeller() || got.GetOrigin() != "interactive" {
				t.Errorf("operationReason=%q notifySeller=%v origin=%q，預期 \"\"／false／interactive",
					got.GetOperationReason(), got.GetNotifySeller(), got.GetOrigin())
			}
			if got.GetRequestId() != "req-1" || got.GetCorrelationId() != "corr-1" ||
				got.GetUserAgent() != "ADM-E2E/1.0" {
				t.Errorf("transport 欄位 = %q/%q/%q，預期 req-1/corr-1/ADM-E2E/1.0",
					got.GetRequestId(), got.GetCorrelationId(), got.GetUserAgent())
			}

			cf := stub.confirms[0]
			if cf.GetResult() != "FAILED" || cf.GetBizCode() != tc.in.BizCode {
				t.Errorf("confirm result/bizCode = %q/%d，預期 FAILED/%d", cf.GetResult(), cf.GetBizCode(), tc.in.BizCode)
			}
			if cf.GetAuditLogId() != "9001" {
				t.Errorf("confirm 關掉的是 %q，不是剛開的那一列", cf.GetAuditLogId())
			}
		})
	}
}

// 寫不出合法 action 的輸入一律不送。送出去的下場是 delegation 模組回 9260（action 不在
// 已登錄的稽核動作內），而呼叫端只會 log 一行 Warn —— 白白付一次 gRPC 往返換一個沒人看
// 的錯誤。
func TestRecordReadDenialSkipsUnwritableRows(t *testing.T) {
	cases := []struct {
		name string
		in   ReadDenial
	}{
		{"未知資源面", ReadDenial{Session: denialSession(), ResourceType: "invoice"}},
		{"resourceType 為空", ReadDenial{Session: denialSession()}},
		{"session 為 nil", ReadDenial{ResourceType: "product", ResourceID: "77"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &closureAuditStub{}
			closureOrchestrator(stub).RecordReadDenial(context.Background(), tc.in)
			if len(stub.intents) != 0 || len(stub.confirms) != 0 {
				t.Fatalf("不該送出任何稽核 RPC，實得 intent %d／confirm %d", len(stub.intents), len(stub.confirms))
			}
		})
	}
}
