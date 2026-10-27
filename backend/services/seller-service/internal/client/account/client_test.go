package account

import (
	"context"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"google.golang.org/grpc"
)

// fakeAccountServiceClient satisfies pb.AccountServiceClient by embedding the
// interface (unimplemented methods panic-if-called) and overriding only
// RemoveRole, capturing the outgoing request for assertions.
type fakeAccountServiceClient struct {
	pb.AccountServiceClient
	gotReq *pb.RemoveRoleRequest
}

func (f *fakeAccountServiceClient) RemoveRole(_ context.Context, in *pb.RemoveRoleRequest, _ ...grpc.CallOption) (*pb.RemoveRoleResponse, error) {
	f.gotReq = in
	return &pb.RemoveRoleResponse{Code: 0}, nil
}

// TestRemoveSellerRole_SendsMappedOtherRoleState 證明 rolestatepb 的對應表真的
// 被掛到線上的 RemoveRole 請求（而不是算出來就丟掉），涵蓋 call site 本身接錯
// 欄位的情況。對應表本身（ACTIVE/ABSENT/UNKNOWN 的映射，反過來會讓
// account-service 對「對側角色還活著」的帳號做不可逆的 cascade 軟刪）由共用套件
// pkg/rolestate/rolestatepb 的 table test 直接釘住，這裡不重複。
func TestRemoveSellerRole_SendsMappedOtherRoleState(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   rolestate.OtherRoleState
		want pb.OtherRoleState
	}{
		{"ACTIVE", rolestate.OtherRoleActive, pb.OtherRoleState_OTHER_ROLE_STATE_ACTIVE},
		{"ABSENT", rolestate.OtherRoleAbsent, pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT},
		{"UNKNOWN", rolestate.OtherRoleUnknown, pb.OtherRoleState_OTHER_ROLE_STATE_UNKNOWN},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeAccountServiceClient{}
			c := &Client{client: fake}

			if err := c.RemoveSellerRole(context.Background(), 42, tt.in, 7001); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fake.gotReq == nil {
				t.Fatal("RemoveRole was not called")
			}
			if got := fake.gotReq.OtherRoleState; got != tt.want {
				t.Errorf("OtherRoleState on the wire = %v, want %v", got, tt.want)
			}
			if got := fake.gotReq.Role; got != pb.Role_ROLE_SELLER {
				t.Errorf("Role on the wire = %v, want ROLE_SELLER", got)
			}
		})
	}
}
