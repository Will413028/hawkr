package buyer

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeBuyerServiceClient satisfies pb.BuyerServiceClient by embedding the
// interface (unimplemented methods panic-if-called) and overriding only
// GetBuyerProfile.
type fakeBuyerServiceClient struct {
	pb.BuyerServiceClient
	resp *pb.BuyerProfileResponse
	err  error
}

func (f *fakeBuyerServiceClient) GetBuyerProfile(_ context.Context, _ *pb.GetBuyerProfileRequest, _ ...grpc.CallOption) (*pb.BuyerProfileResponse, error) {
	return f.resp, f.err
}

// TestHasActiveBuyer covers every branch of the translation from
// buyer-service's GetBuyerProfile envelope to the boolean that
// rolestate.Resolve turns into ACTIVE / ABSENT / UNKNOWN.
//
// 這個 seam 沒有測試的話，把 true/false 對調、或把「任何業務碼都當成對側不存在」
// 寫回去，整包測試都還是綠的 —— 而 account-service 會照著這個布林值去做不可逆的
// cascade 軟刪（清空 auth_providers）。錯的方向沒有任何後續關卡擋得住。
func TestHasActiveBuyer(t *testing.T) {
	tests := []struct {
		name    string
		resp    *pb.BuyerProfileResponse
		err     error
		want    bool
		wantErr bool
	}{
		{
			name: "有 active buyer → true",
			resp: &pb.BuyerProfileResponse{Code: 0, Data: &pb.BuyerInfo{AccountId: "42"}},
			want: true,
		},
		{
			name: "gRPC NotFound → 對側確認不存在（false, nil）",
			err:  status.Error(codes.NotFound, "buyer not found"),
			want: false,
		},
		{
			name: "envelope 帶 CodeNotFound → 對側確認不存在（false, nil）",
			resp: &pb.BuyerProfileResponse{Code: int32(pkgerrors.CodeNotFound), Message: "buyer not found"},
			want: false,
		},
		{
			name: "Code=0 但 Data 為 nil → 沒有 buyer（false, nil）",
			resp: &pb.BuyerProfileResponse{Code: 0},
			want: false,
		},
		{
			// 這一條是 fail-closed 的重點：不是 not-found 的業務碼代表「查不出來」，
			// 不是「確認沒有」。回 error 才會被 rolestate.Resolve 映射成 UNKNOWN。
			name:    "其他業務碼 → 查詢失敗（false, error）",
			resp:    &pb.BuyerProfileResponse{Code: bizcode.BuyerAccountDeleting, Message: "帳號刪除中"},
			want:    false,
			wantErr: true,
		},
		{
			name:    "非 NotFound 的 gRPC 錯誤 → 查詢失敗（false, error）",
			err:     status.Error(codes.Unavailable, "buyer-service unavailable"),
			want:    false,
			wantErr: true,
		},
		{
			name:    "非 gRPC status 的傳輸錯誤 → 查詢失敗（false, error）",
			err:     errors.New("boom"),
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{client: &fakeBuyerServiceClient{resp: tt.resp, err: tt.err}}

			got, err := c.HasActiveBuyer(context.Background(), 42)

			if got != tt.want {
				t.Errorf("HasActiveBuyer() = %v, want %v", got, tt.want)
			}
			if tt.wantErr && err == nil {
				t.Error("HasActiveBuyer() error = nil, want non-nil（才會 fail-closed 成 UNKNOWN）")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("HasActiveBuyer() unexpected error = %v", err)
			}
		})
	}
}
