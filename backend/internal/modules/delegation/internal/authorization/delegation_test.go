package authorization

import (
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

func TestAuthorizationDisplayNamesMissingAdmins(t *testing.T) {
	approverID, revokerID := int64(7), int64(8)
	row := store.DelegationSellerDelegationAuthorization{ApprovedByAdminID: approverID, RevokedByAdminID: &revokerID}
	got := toDelegationAuthorization(row, nil, time.Time{})
	if got.ApprovedByDisplayName != "" || got.RevokedByDisplayName != nil {
		t.Fatalf("缺少管理員時 = %+v", got)
	}
	admins := map[int64]identity.Admin{
		approverID: {Email: "approver@example.test"},
		revokerID:  {Email: "revoker@example.test"},
	}
	got = toDelegationAuthorization(row, admins, time.Time{})
	if got.ApprovedByDisplayName != "approver@example.test" || got.RevokedByDisplayName == nil || *got.RevokedByDisplayName != "revoker@example.test" {
		t.Fatalf("存在管理員時 = %+v", got)
	}
}

func TestValidateAuthorizationChange(t *testing.T) {
	good := model.DelegationAuthorizationChange{
		StoreID: 1, Source: model.AuthorizationContract, SourceReference: "CT-2026-001", OperationReason: "合約簽署",
		Capabilities: []model.Capability{model.CapabilityProductsRead},
	}
	if err := validateAuthorizationChange(good); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(*model.DelegationAuthorizationChange){
		"店家":     func(c *model.DelegationAuthorizationChange) { c.StoreID = 0 },
		"依據":     func(c *model.DelegationAuthorizationChange) { c.Source = "email" },
		"編號太短":   func(c *model.DelegationAuthorizationChange) { c.SourceReference = "C-1" },
		"編號沒有數字": func(c *model.DelegationAuthorizationChange) { c.SourceReference = "CONTRACT" },
		"理由是空的":  func(c *model.DelegationAuthorizationChange) { c.OperationReason = "" },
		"理由太長":   func(c *model.DelegationAuthorizationChange) { c.OperationReason = strings.Repeat("理", 201) },
		"沒有能力":   func(c *model.DelegationAuthorizationChange) { c.Capabilities = nil },
		"不認得的能力": func(c *model.DelegationAuthorizationChange) { c.Capabilities = []model.Capability{"orders.refund"} },
		"重複的能力": func(c *model.DelegationAuthorizationChange) {
			c.Capabilities = []model.Capability{"posts.read", "posts.read"}
		},
		"超過九個的能力數": func(c *model.DelegationAuthorizationChange) {
			c.Capabilities = append(model.Capabilities(), "posts.read")
		},
	}
	for name, mut := range bad {
		c := good
		mut(&c)
		if d, ok := model.DelegationDenialOf(validateAuthorizationChange(c)); !ok || d.Denial != model.DenialInvalidCommand {
			t.Errorf("%s 應該是 9260", name)
		}
	}
}
