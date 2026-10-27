package actor_test

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/actor"
	"google.golang.org/grpc/metadata"
)

func TestResolveAdminGRPCMetadata(t *testing.T) {
	tests := []struct {
		name string
		md   metadata.MD
		want actor.AdminMetadataState
	}{
		{"admin", metadata.Pairs("x-account-type", "4"), actor.AdminMetadataAdmin},
		{"seller", metadata.Pairs("x-account-type", "2"), actor.AdminMetadataNotAdmin},
		{"buyer", metadata.Pairs("x-account-type", "1"), actor.AdminMetadataNotAdmin},
		{"both buyer and seller is not admin", metadata.Pairs("x-account-type", "3"), actor.AdminMetadataNotAdmin},
		// 完全沒帶 key（跟「帶了但值不是 admin」是不同狀態：client 沒帶身分，
		// 不是「身分不是 admin」——呼叫端要能分開判 Unauthenticated／PermissionDenied）。
		{"absent", metadata.MD{}, actor.AdminMetadataAbsent},
		{"empty value", metadata.Pairs("x-account-type", ""), actor.AdminMetadataNotAdmin},
		{"non-numeric", metadata.Pairs("x-account-type", "admin"), actor.AdminMetadataNotAdmin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(t.Context(), tt.md)
			if got := actor.ResolveAdminGRPCMetadata(ctx); got != tt.want {
				t.Errorf("ResolveAdminGRPCMetadata = %v, want %v", got, tt.want)
			}
		})
	}
}

// 沒有 incoming metadata（例如直接呼叫、非 gRPC 路徑）必須是 AdminMetadataAbsent
// 而不是 panic，且不能被誤判成 AdminMetadataNotAdmin。
func TestResolveAdminGRPCMetadataNoMetadata(t *testing.T) {
	if got := actor.ResolveAdminGRPCMetadata(t.Context()); got != actor.AdminMetadataAbsent {
		t.Errorf("ResolveAdminGRPCMetadata = %v with no incoming metadata, want AdminMetadataAbsent", got)
	}
}
