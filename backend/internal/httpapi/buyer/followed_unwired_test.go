package buyer

import (
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
)

// catalog 在 B4-6 切換前不接進 cmd/hawkr：沒組裝 Followed 時回 unimplemented，不是 internal。
func TestListFollowedStoreProductsIsUnimplementedUntilWired(t *testing.T) {
	_, err := (&App{}).ListFollowedStoreProducts(t.Context(), connect.NewRequest(&pb.ListFollowedStoreProductsRequest{}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("code = %v, err = %v", connect.CodeOf(err), err)
	}
}
