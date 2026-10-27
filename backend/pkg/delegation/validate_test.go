package delegation

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testStoreID = int64(2685914682)

func fullContext() *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       "2701445580000001",
		OperatorAdminId: "2701338845",
		Capability:      string(CapProductsWrite),
		OperationReason: "賣家請求協助修正商品說明",
		AuditLogId:      "2701445588392017",
	}
}

// TestValidateEquivalenceFixtures is the §6.2.4 §4.5 table (E1–E13). The Java
// twin runs the same cases; a divergence between the two is exactly what this
// fixture set exists to catch.
//
// E3–E9 all expect 9289 since #2744. They used to split between 9260 and 9262,
// which said "you sent a bad value" and "you lack this permission" about a
// context the operator never touches — the Gateway assembles it from the
// session. Both readings sent people to do work that could not fix anything,
// and 9262 additionally landed in admin_operation_audit_logs as
// "this administrator lacked permission" when the truth was our own routing.
//
// ⚠️ wantGRPC deliberately did NOT change. The Gateway parses the "[NNNN] "
// prefix first; the status is only its fallback, and codes.Internal is on that
// fallback's transport-failure list (9280). Making the status "more honest"
// would hide the honest code.
func TestValidateEquivalenceFixtures(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*commonpb.DelegationContext)
		nilCtx     bool
		storeID    int64
		declared   Capability
		requestKey string
		hasReqKey  bool
		wantCode   int32 // 0 = expect pass
		wantGRPC   codes.Code
	}{
		{
			name:     "E1 nil delegation is the seller's own path",
			nilCtx:   true,
			storeID:  testStoreID,
			declared: CapProductsWrite,
		},
		{
			name:     "E2 all fields present, capability matches",
			storeID:  testStoreID,
			declared: CapProductsWrite,
		},
		{
			name:     "E3 empty audit_log_id on a write capability",
			mutate:   func(d *commonpb.DelegationContext) { d.AuditLogId = "" },
			storeID:  testStoreID,
			declared: CapProductsWrite,
			wantCode: bizcode.DelegationInternalContractMismatch,
			wantGRPC: codes.InvalidArgument,
		},
		{
			name:     "E4 empty session_id",
			mutate:   func(d *commonpb.DelegationContext) { d.SessionId = "" },
			storeID:  testStoreID,
			declared: CapProductsWrite,
			wantCode: bizcode.DelegationInternalContractMismatch,
			wantGRPC: codes.InvalidArgument,
		},
		{
			name:     "E5 capability outside the closed enum",
			mutate:   func(d *commonpb.DelegationContext) { d.Capability = "products.delete" },
			storeID:  testStoreID,
			declared: CapProductsWrite,
			wantCode: bizcode.DelegationInternalContractMismatch,
			wantGRPC: codes.PermissionDenied,
		},
		{
			name:     "E6 upper-cased capability is not normalised first",
			mutate:   func(d *commonpb.DelegationContext) { d.Capability = "PRODUCTS.WRITE" },
			storeID:  testStoreID,
			declared: CapProductsWrite,
			wantCode: bizcode.DelegationInternalContractMismatch,
			wantGRPC: codes.PermissionDenied,
		},
		{
			name:     "E7 capability legal but not the one this RPC declares",
			mutate:   func(d *commonpb.DelegationContext) { d.Capability = string(CapProductsRead) },
			storeID:  testStoreID,
			declared: CapProductsWrite,
			wantCode: bizcode.DelegationInternalContractMismatch,
			wantGRPC: codes.PermissionDenied,
		},
		{
			name:     "E8 missing ownership key",
			storeID:  0,
			declared: CapProductsWrite,
			wantCode: bizcode.DelegationInternalContractMismatch,
			wantGRPC: codes.InvalidArgument,
		},
		{
			name:       "E9 request and context idempotency keys disagree",
			mutate:     func(d *commonpb.DelegationContext) { d.IdempotencyKey = "ik_b" },
			storeID:    testStoreID,
			declared:   CapProductsWrite,
			requestKey: "ik_a",
			hasReqKey:  true,
			wantCode:   bizcode.DelegationInternalContractMismatch,
			wantGRPC:   codes.InvalidArgument,
		},
		{
			name:       "E10 idempotency keys identical",
			mutate:     func(d *commonpb.DelegationContext) { d.IdempotencyKey = "ik_a" },
			storeID:    testStoreID,
			declared:   CapProductsWrite,
			requestKey: "ik_a",
			hasReqKey:  true,
		},
		{
			name:       "E11 request carries the field but left it empty",
			mutate:     func(d *commonpb.DelegationContext) { d.IdempotencyKey = "ik_a" },
			storeID:    testStoreID,
			declared:   CapProductsWrite,
			requestKey: "",
			hasReqKey:  true,
		},
		{
			name: "E12 products.read with empty audit_log_id passes",
			mutate: func(d *commonpb.DelegationContext) {
				d.Capability = string(CapProductsRead)
				d.AuditLogId = ""
			},
			storeID:  testStoreID,
			declared: CapProductsRead,
		},
		{
			name: "E13 storefront.read with empty audit_log_id passes",
			mutate: func(d *commonpb.DelegationContext) {
				d.Capability = string(CapStorefrontRead)
				d.AuditLogId = ""
			},
			storeID:  testStoreID,
			declared: CapStorefrontRead,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d *commonpb.DelegationContext
			if !tc.nilCtx {
				d = fullContext()
				if tc.mutate != nil {
					tc.mutate(d)
				}
			}

			var opts []Option
			if tc.hasReqKey {
				opts = append(opts, WithRequestIdempotencyKey(tc.requestKey))
			}

			err := Validate(context.Background(), d, tc.storeID, tc.declared, opts...)

			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("預期通過，實得 %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("預期業務碼 %d，實得 nil", tc.wantCode)
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("預期 gRPC status error，實得 %T", err)
			}
			if st.Code() != tc.wantGRPC {
				t.Errorf("gRPC code = %v，預期 %v", st.Code(), tc.wantGRPC)
			}
			if want := bizPrefix(tc.wantCode); !strings.HasPrefix(st.Message(), want) {
				t.Errorf("message = %q，預期以 %q 開頭（Gateway 以此前綴解析業務碼）", st.Message(), want)
			}
		})
	}
}

// TestValidateS1bScopeIsDeclaredCapability pins the scope decision: S1b must key
// off the capability the RPC declares, and it must not fire for reads. Without
// this the "audit_log_id 一律必填" mistake passes every write fixture.
func TestValidateS1bScopeIsDeclaredCapability(t *testing.T) {
	for _, cap := range AllCapabilities {
		d := fullContext()
		d.Capability = string(cap)
		d.AuditLogId = ""

		err := Validate(context.Background(), d, testStoreID, cap)
		if requiresAuditLogID(cap) {
			if err == nil {
				t.Errorf("%s：空 audit_log_id 應被 S1b 拒絕", cap)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s：讀取類 capability 的空 audit_log_id 應通過，實得 %v", cap, err)
		}
	}
}

// TestValidateWithoutOptionSkipsS5 guards the reason WithRequestIdempotencyKey is
// an option: an RPC whose request has no such field must not be treated as
// "sent an empty key".
func TestValidateWithoutOptionSkipsS5(t *testing.T) {
	d := fullContext()
	d.IdempotencyKey = "ik_a"
	if err := Validate(context.Background(), d, testStoreID, CapProductsWrite); err != nil {
		t.Fatalf("未帶 option 時 S5 應跳過，實得 %v", err)
	}
}

func TestIsKnownAndAllCapabilities(t *testing.T) {
	if len(AllCapabilities) != 9 {
		t.Fatalf("AllCapabilities 長度 = %d，預期 9（封閉列舉）", len(AllCapabilities))
	}
	for _, c := range AllCapabilities {
		if !IsKnown(c) {
			t.Errorf("IsKnown(%s) = false", c)
		}
	}
	for _, c := range []Capability{"", "products.bulk", "audit.read", "orders.write", "fulfillment"} {
		if IsKnown(c) {
			t.Errorf("IsKnown(%q) = true，該值不在封閉九值內", c)
		}
	}
}

func TestContextRoundTrip(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("空 context 應回 false（賣家本人路徑）")
	}
	d := fullContext()
	got, ok := FromContext(NewContext(context.Background(), d))
	if !ok || got != d {
		t.Fatalf("FromContext = (%v, %v)，預期取回同一個指標", got, ok)
	}
}

func bizPrefix(code int32) string {
	switch code {
	case bizcode.DelegationInvalidCommand:
		return "[9260] "
	case bizcode.DelegationCapabilityDenied:
		return "[9262] "
	default:
		return ""
	}
}
