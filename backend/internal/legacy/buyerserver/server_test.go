package buyerserver

import (
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
)

// Java shipping 只接得住業務碼（轉成「Uber 不可用」），gRPC 錯誤會讓整個報價失敗。
func TestValidateAddressAnswersTheDisabledBusinessCode(t *testing.T) {
	res, err := New(nil).ValidateAddress(t.Context(), connect.NewRequest(&pb.ValidateAddressRequest{Address: "臺北市信義區"}))
	if err != nil {
		t.Fatalf("回 gRPC 錯誤：%v", err)
	}
	if res.Msg.GetCode() != 9700 || res.Msg.GetData().GetValid() {
		t.Fatalf("回應 = %+v", res.Msg)
	}
}

func TestAddressConversionAndUnknownCode(t *testing.T) {
	s := New(nil)
	ctx := t.Context()

	// 正常代碼轉換為內政部名稱（含「臺」字）
	info, err := s.address(ctx, buyer.Address{
		AddressID:     101,
		BuyerID:       201,
		ReceiverName:  "王小明",
		ReceiverPhone: "0912345678",
		TownCode:      "63000050", // 臺北市中正區
		AddressDetail: "忠孝東路一段 1 號",
		IsDefault:     true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if info.GetCity() != "臺北市" || info.GetDistrict() != "中正區" || info.GetDetailAddress() != "忠孝東路一段 1 號" {
		t.Fatalf("info = %+v", info)
	}
	if info.GetLabel() != "" {
		t.Fatalf("label = %q, want empty", info.GetLabel())
	}

	// 未知代碼回報 CodeInternal
	_, err = s.address(ctx, buyer.Address{
		AddressID:     102,
		BuyerID:       201,
		TownCode:      "99999999",
		AddressDetail: "路邊",
	})
	if err == nil || connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("expected CodeInternal for unknown code, got: %v", err)
	}
}
