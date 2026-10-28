package delegation

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
)

// 讀取拒絕列是「有人試圖跨店讀取」唯一的痕跡（56 號 §6.5.3 的四個 READ action）。它壞掉的方式全部
// 無聲：成功的讀取本來就不落稽核，這一列若寫錯面、寫錯 capability、或三個 transport 欄位
// 空著，稽核表看起來仍然完全正常——每一列都合法，只是再也分不出哪一列是攻擊嘗試。
//
// 直接驅動 Orchestrator 而不是打 HTTP：要斷言的是「Gateway 送了什麼給 account-service」。

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
			// Q4：capability 是有的，被拒的是資源歸屬 ⇒ 空 Capability 落回該面的 read。
			name: "capability 留空 → 落回該面的 read",
			in: ReadDenial{
				Session: denialSession(), ResourceType: "product", ResourceID: "77",
				BizCode: bizcode.DelegationResourceNotOwned,
			},
			wantAction: "PRODUCT_READ", wantCapability: "products.read",
		},
		{
			// Q3：被拒的正是這個 capability，記它本身（§7.1.3）。
			name: "帶 capability → 原樣記錄",
			in: ReadDenial{
				Session: denialSession(), ResourceType: "post", ResourceID: "88",
				Capability: authz.AdminCapPostsPublish, BizCode: bizcode.DelegationCapabilityDenied,
			},
			wantAction: "POST_READ", wantCapability: "posts.publish",
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
				t.Errorf("action = %q，預期 %q（四個 READ action 是拒絕列唯一的合法值）", got.GetAction(), tc.wantAction)
			}
			if got.GetCapability() != tc.wantCapability {
				t.Errorf("capability = %q，預期 %q", got.GetCapability(), tc.wantCapability)
			}
			if got.GetResourceType() != tc.in.ResourceType || got.GetResourceId() != tc.in.ResourceID {
				t.Errorf("resourceType/resourceId = %q/%q，預期 %q/%q",
					got.GetResourceType(), got.GetResourceId(), tc.in.ResourceType, tc.in.ResourceID)
			}
			// 這四項是鎖在函式內、不交給呼叫端決定的：拒絕列沒有使用者輸入可記，
			// 而 notify_seller=true 的那一天會把「有人試圖跨店讀你的資料」推播給第三方。
			if got.GetOperationReason() != "" || got.GetNotifySeller() || got.GetOrigin() != "interactive" {
				t.Errorf("operationReason=%q notifySeller=%v origin=%q，預期 \"\"／false／interactive",
					got.GetOperationReason(), got.GetNotifySeller(), got.GetOrigin())
			}
			// transport 三欄漏掉不會有任何錯誤，只會讓稽核面板對這一類列顯示「—」。
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

// 寫不出合法 action 的輸入一律不送。送出去的下場是 account-service 回 9260（action 不在
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
