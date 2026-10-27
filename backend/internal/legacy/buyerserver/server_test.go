package buyerserver

import (
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
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

// 舊儀表板的週：台北週日 00:00 起算，本週到 now（不含），上週是上一個完整的週日到週六。
func TestSubscriberWeeksFollowTheTaipeiSunday(t *testing.T) {
	tp := func(y int, m time.Month, d, h, min, sec int) time.Time {
		return time.Date(y, m, d, h, min, sec, 0, twtime.Taipei)
	}
	for _, tc := range []struct {
		name                 string
		now                  time.Time
		thisStart, lastStart time.Time
	}{
		{"週日 00:00 是新的一週", tp(2026, 2, 1, 0, 0, 0), tp(2026, 2, 1, 0, 0, 0), tp(2026, 1, 25, 0, 0, 0)},
		{"週六 23:59:59 還是上一週", tp(2026, 1, 31, 23, 59, 59), tp(2026, 1, 25, 0, 0, 0), tp(2026, 1, 18, 0, 0, 0)},
		{"UTC 週六 16:00 是台北的週日", time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC), tp(2026, 2, 1, 0, 0, 0), tp(2026, 1, 25, 0, 0, 0)},
		{"週三", tp(2026, 2, 4, 12, 0, 0), tp(2026, 2, 1, 0, 0, 0), tp(2026, 1, 25, 0, 0, 0)},
		{"跨年", tp(2027, 1, 1, 8, 0, 0), tp(2026, 12, 27, 0, 0, 0), tp(2026, 12, 20, 0, 0, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			this, last := subscriberWeeks(tc.now)
			if !this.Start.Equal(tc.thisStart) || !this.End.Equal(tc.now) || !last.Start.Equal(tc.lastStart) || !last.End.Equal(tc.thisStart) {
				t.Fatalf("this=%v last=%v", this, last)
			}
		})
	}
}

// 成長率沿用舊公式：round((本週 − 上週) / max(上週, 1) × 100)，上週為 0 時 N 次顯示成 N × 100，半數遠離 0 進位。
func TestSubscriberGrowthPct(t *testing.T) {
	for _, tc := range []struct {
		this, last int64
		want       int32
	}{
		{0, 0, 0}, {5, 0, 500}, {1, 2, -50}, {0, 1, -100}, {3, 2, 50}, {1, 3, -67}, {2, 3, -33}, {1, 8, -88}, {3, 8, -63},
		{1 << 40, 0, 2147483647}, {0, 1 << 40, -100},
	} {
		if got := subscriberGrowthPct(tc.this, tc.last); got != tc.want {
			t.Errorf("growth(%d, %d)=%d, want %d", tc.this, tc.last, got, tc.want)
		}
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
