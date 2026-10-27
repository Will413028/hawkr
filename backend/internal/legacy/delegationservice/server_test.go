package delegationservice

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

func TestEveryDenialHasABusinessCode(t *testing.T) {
	seen := map[int32]delegation.DelegationDenial{}
	for d := delegation.DenialInvalidCommand; d <= delegation.DenialAdminNotAllowlisted; d++ {
		code, ok := bizCodes[d]
		if !ok || code < 9260 || code > 9299 {
			t.Errorf("拒絕 %d 沒有業務碼（%d）", d, code)
			continue
		}
		if other, dup := seen[code]; dup {
			t.Errorf("拒絕 %d 與 %d 共用業務碼 %d", d, other, code)
		}
		seen[code] = d
	}
}

// 事件的形狀是 notification-service 讀的舊契約：BaseEvent 的欄位（metadata 是空的所以省略）加上業務欄位，每個 ID 都是
// 字串；resource_name、case_reference 沒有時省略，resource_id、operation_reason 沒有時是空字串。
func TestCompletedEventKeepsTheLegacyShape(t *testing.T) {
	var got []rabbitmq.PublishArgs
	on := OnDelegationOperationCompleted(func(_ context.Context, a rabbitmq.PublishArgs) error {
		got = append(got, a)
		return nil
	})
	audit, session := uuid.NewV7(), int64(1234567)
	at := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	name, rid := "梨山水蜜桃", "2701445588392017"
	e := delegation.DelegationOperationCompleted{
		AuditLogID: audit, SessionID: session, SessionDisplayCode: "OPS-0123456789ABC", ActorAdminID: 3, ActorDisplayName: "ops@hawkr.example",
		TargetStoreID: 2685914682, TargetSellerAccountID: 2685914689, Capability: delegation.CapabilityProductsWrite,
		Action: delegation.AuditProductUpdate, ActionLabel: "編輯商品", RiskLevel: delegation.RiskMedium, ResourceType: delegation.ResourceProduct,
		ResourceID: &rid, ResourceName: &name, CorrelationID: "corr-1", OccurredAt: at.Add(-time.Second + 300*time.Millisecond), NotifiedAt: at,
	}
	if err := on(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	e.ResourceID, e.ResourceName, e.CorrelationID = nil, nil, ""
	if err := on(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Exchange != "hawkr.delegation" || got[0].RoutingKey != "delegation.operation.completed" {
		t.Fatalf("= %+v", got)
	}
	var full, bare map[string]any
	if err := json.Unmarshal(got[0].Body, &full); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got[1].Body, &bare); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"event_type": "DELEGATION_OPERATION_COMPLETED", "version": "1.0", "source": "account-service", "correlation_id": "corr-1",
		"timestamp": "2026-09-27T12:00:00Z", "audit_log_id": audit.String(), "session_id": "1234567",
		"session_display_code": "OPS-0123456789ABC", "actor_admin_id": "3", "actor_display_name": "ops@hawkr.example",
		"target_store_id": "2685914682", "target_seller_account_id": "2685914689", "capability": "products.write",
		"action": "PRODUCT_UPDATE", "action_label": "編輯商品", "risk_level": "medium", "resource_type": "product",
		"resource_id": "2701445588392017", "resource_name": "梨山水蜜桃", "operation_reason": "", "occurred_at": "2026-09-27T11:59:59Z",
	}
	for k, v := range want {
		if full[k] != v {
			t.Errorf("%s = %v，預期 %v", k, full[k], v)
		}
	}
	if id, _ := full["event_id"].(string); id == "" || id != got[0].EventID || len(full) != len(want)+1 {
		t.Errorf("event_id 與多出的欄位：%v", full)
	}
	for _, k := range []string{"metadata", "case_reference", "resource_name", "correlation_id"} {
		if _, ok := bare[k]; ok {
			t.Errorf("沒有值時不輸出 %s", k)
		}
	}
	if bare["resource_id"] != "" || bare["operation_reason"] != "" || bare["event_id"] == full["event_id"] {
		t.Errorf("= %v", bare)
	}
}

func TestParsing(t *testing.T) {
	for s, want := range map[string]bool{"1": true, "2685914682": true, "0": false, "-1": false, "": false, "7494590778405027840x": false} {
		if _, ok := parseID(s); ok != want {
			t.Errorf("parseID(%q) = %v", s, ok)
		}
	}
	if tm, ok := parseTime("2026-09-27T12:00:00.123+08:00"); !ok || !tm.Equal(time.Date(2026, 9, 27, 4, 0, 0, 123e6, time.UTC)) {
		t.Errorf("= %v, %v", tm, ok)
	}
	if _, ok := parseTime("2026-09-27"); ok {
		t.Error("只接受 ISO 8601 的時刻")
	}
	if formatTime(time.Date(2026, 9, 27, 12, 0, 0, 999e6, time.FixedZone("TPE", 8*3600))) != "2026-09-27T04:00:00Z" {
		t.Error("對外的時間是 UTC、截到秒")
	}
	if parseOptionalInt64("O123456789") != nil || *parseOptionalInt64("13") != 13 {
		t.Error("parseOptionalInt64")
	}
}
