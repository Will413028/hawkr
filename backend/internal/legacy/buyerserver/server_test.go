package buyerserver

import (
	"strings"
	"testing"
	"uuid"

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

	info, err := s.address(ctx, buyer.Address{
		AddressID:     101,
		BuyerID:       201,
		ReceiverName:  "王小明",
		ReceiverPhone: "0912345678",
		TownCode:      "63000050",
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

// 回應以呼叫者送來的原字串為 key：大寫的 UUID 解析後同一個 ID，key 仍是原樣；非 UUID（舊服務的整數 ID）略過，同一個 ID 的不同寫法只問一次。
func TestParseUUIDsKeepsTheCallersRawString(t *testing.T) {
	id := uuid.NewV7()
	lower, upper := id.String(), strings.ToUpper(id.String())
	parsed := parseUUIDs([]string{upper, "11", lower})
	if len(parsed) != 2 || parsed[0].raw != upper || parsed[0].id != id || parsed[1].raw != lower || parsed[1].id != id {
		t.Fatalf("parsed = %+v", parsed)
	}
	if ids := distinct(parsed); len(ids) != 1 || ids[0] != id {
		t.Fatalf("distinct = %v", ids)
	}
}
