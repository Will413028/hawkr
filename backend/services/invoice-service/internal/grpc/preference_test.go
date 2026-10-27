package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/protobuf/proto"
)

// 本檔驗 B08 寫入面的**型別邊界與 Tier A 出口形狀**；業務規則本身在
// internal/usecase/preference_test.go。
//
// 🔴 這裡的核心斷言是「越權與查無的回應逐位元相同」，而且是用 proto.Marshal 比對
// 不是比 code：比 code 只證明兩者同碼，證明不了 message、data 或任何未來新增的欄位
// 沒有洩漏「這筆資料存在，只是不是你的」。

const (
	pBuyerA int64 = 2623900520
	pBuyerB int64 = 2688637900
)

// pfxTx 直接執行 callback（交易語意由 usecase 的單元測試守）。
type pfxTx struct{}

func (pfxTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// pfxPrefs 照 SQL 述詞回應：id ＋ buyer_id ＋ 存活，四條缺一即「查不到」。
type pfxPrefs struct {
	domain.BuyerPreferenceRepository
	rows map[int64]domain.BuyerPreference
}

func (p pfxPrefs) GetByID(_ context.Context, id, buyerID int64) (domain.BuyerPreference, error) {
	row, ok := p.rows[id]
	if !ok || row.BuyerID != buyerID {
		return domain.BuyerPreference{}, notFoundBiz(bizcode.InvoiceProfileNotFound)
	}
	return row, nil
}

func (p pfxPrefs) ClearDefault(context.Context, int64, time.Time) (int64, error) { return 0, nil }

func (p pfxPrefs) SetDefault(_ context.Context, id, buyerID int64, _ time.Time) (bool, error) {
	row, ok := p.rows[id]
	return ok && row.BuyerID == buyerID, nil
}

func (p pfxPrefs) SoftDelete(_ context.Context, id, buyerID int64, _ time.Time) (bool, error) {
	row, ok := p.rows[id]
	return ok && row.BuyerID == buyerID, nil
}

func newPrefServer(rows ...domain.BuyerPreference) *InvoiceServer {
	repo := pfxPrefs{rows: map[int64]domain.BuyerPreference{}}
	for _, r := range rows {
		repo.rows[r.ID] = r
	}
	var seq int64 = 8000000000
	uc := usecase.NewPreference(pfxTx{}, repo, nil, func() int64 { seq++; return seq }, nil)
	return NewInvoiceServer(nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func prefRow(id, buyerID int64, isDefault bool) domain.BuyerPreference {
	mask := "/T7R***P"
	ct := "3J0002"
	return domain.BuyerPreference{
		ID:               id,
		BuyerID:          buyerID,
		PreferenceKind:   codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		CarrierType:      &ct,
		CarrierValueMask: &mask,
		IsDefault:        isDefault,
		CreatedAt:        time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
	}
}

// ---------------------------------------------------------------------------
// Tier A：越權與查無**逐位元**相同
// ---------------------------------------------------------------------------

// TestPreference_TierA_ByteIdentical 是 §8.1.4(b) 的機械保證。
//
// 三支帶 profile_id 的 RPC 各跑兩次：一次打「別人的存在的列」、一次打「不存在的列」，
// 兩次的 response 必須 proto.Marshal 之後逐位元相同。任何一個欄位（message／data／
// 未來新增的欄位）洩漏了差異，這支就會紅。
func TestPreference_TierA_ByteIdentical(t *testing.T) {
	const foreignID, missingID = 111, 999

	cases := []struct {
		name string
		call func(s *InvoiceServer, id int64) (proto.Message, error)
	}{
		{"Update", func(s *InvoiceServer, id int64) (proto.Message, error) {
			return s.UpdateBuyerInvoiceProfile(context.Background(), &pb.UpdateBuyerInvoiceProfileRequest{
				BuyerId: pBuyerA, ProfileId: fmtInt64(id), Value: "/T7R3U+P",
			})
		}},
		{"Delete", func(s *InvoiceServer, id int64) (proto.Message, error) {
			return s.DeleteBuyerInvoiceProfile(context.Background(), &pb.DeleteBuyerInvoiceProfileRequest{
				BuyerId: pBuyerA, ProfileId: fmtInt64(id),
			})
		}},
		{"SetDefault", func(s *InvoiceServer, id int64) (proto.Message, error) {
			return s.SetDefaultBuyerInvoiceProfile(context.Background(), &pb.SetDefaultBuyerInvoiceProfileRequest{
				BuyerId: pBuyerA, ProfileId: fmtInt64(id),
			})
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// foreignID 屬於 buyer B 且真的存在；missingID 不存在。
			s := newPrefServer(prefRow(foreignID, pBuyerB, false))

			foreign, err := c.call(s, foreignID)
			if err != nil {
				t.Fatalf("越權不得回 gRPC error（要走 envelope），實際 %v", err)
			}
			missing, err := c.call(s, missingID)
			if err != nil {
				t.Fatalf("查無不得回 gRPC error（要走 envelope），實際 %v", err)
			}

			fb, err := proto.Marshal(foreign)
			if err != nil {
				t.Fatalf("marshal foreign: %v", err)
			}
			mb, err := proto.Marshal(missing)
			if err != nil {
				t.Fatalf("marshal missing: %v", err)
			}
			if string(fb) != string(mb) {
				t.Fatalf("越權與查無的回應不是逐位元相同：\n foreign=%v\n missing=%v", foreign, missing)
			}
			if code := bizCodeOf(t, foreign); code != bizcode.InvoiceProfileNotFound {
				t.Fatalf("預期業務碼 9302，實際 %d", code)
			}
		})
	}
}

func bizCodeOf(t *testing.T, m proto.Message) int32 {
	t.Helper()
	switch v := m.(type) {
	case *pb.BuyerInvoiceProfileResponse:
		return v.GetCode()
	case *pb.DeleteBuyerInvoiceProfileResponse:
		return v.GetCode()
	default:
		t.Fatalf("未預期的回應型別 %T", m)
		return 0
	}
}

// ---------------------------------------------------------------------------
// 型別邊界
// ---------------------------------------------------------------------------

// TestPreference_ProfileIDZeroIsRejected 釘住 §6.1.1「端點 2／3／4 的 profile_id = "0"
// 一律 400」：它是「清除預設」的哨兵，不是一筆資料。
func TestPreference_ProfileIDZeroIsRejected(t *testing.T) {
	s := newPrefServer()

	if _, err := s.UpdateBuyerInvoiceProfile(context.Background(), &pb.UpdateBuyerInvoiceProfileRequest{
		BuyerId: pBuyerA, ProfileId: "0", Value: "/T7R3U+P",
	}); err == nil {
		t.Fatalf("端點 3 收到 profile_id=0 必須回 InvalidArgument")
	}
	if _, err := s.DeleteBuyerInvoiceProfile(context.Background(), &pb.DeleteBuyerInvoiceProfileRequest{
		BuyerId: pBuyerA, ProfileId: "0",
	}); err == nil {
		t.Fatalf("端點 4 收到 profile_id=0 必須回 InvalidArgument")
	}
}

// TestPreference_SetDefaultZeroClears 釘住端點 5 的哨兵語意：
// 回 {"profileId":"0","isDefault":false}，且不得夾帶任何時間戳（那不是一筆資料）。
func TestPreference_SetDefaultZeroClears(t *testing.T) {
	s := newPrefServer(prefRow(222, pBuyerA, true))

	resp, err := s.SetDefaultBuyerInvoiceProfile(context.Background(), &pb.SetDefaultBuyerInvoiceProfileRequest{
		BuyerId: pBuyerA, ProfileId: "0",
	})
	if err != nil {
		t.Fatalf("清除預設不得回 gRPC error，實際 %v", err)
	}
	if resp.GetCode() != 0 || resp.GetData().GetProfileId() != "0" || resp.GetData().GetIsDefault() {
		t.Fatalf("預期 {profileId:\"0\", isDefault:false}，實際 %v", resp)
	}
	if resp.GetData().GetCreatedAt() != "" || resp.GetData().GetUpdatedAt() != "" {
		t.Fatalf("哨兵回應不得夾帶時間戳，實際 %v", resp.GetData())
	}
}

// TestPreference_MissingBuyerIDIsRejected 釘住 requirePrincipal：buyer_id=0 是
// 「Gateway 沒注入」，不是「沒有這個人」。放行會讓述詞建在 0 上。
func TestPreference_MissingBuyerIDIsRejected(t *testing.T) {
	s := newPrefServer()

	if _, err := s.CreateBuyerInvoiceProfile(context.Background(), &pb.CreateBuyerInvoiceProfileRequest{
		CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value, Value: "/T7R3U+P",
	}); err == nil {
		t.Fatalf("缺 buyer_id 必須回 InvalidArgument")
	}
}

// TestPreference_CreateReturnsMaskedValue 釘住「明碼不出服務邊界」：
// Create 的回應只回遮罩，request 帶進來的明碼一個字都不得出現在 wire 上。
func TestPreference_CreateReturnsMaskedValue(t *testing.T) {
	repo := &pfxCreateRepo{}
	uc := usecase.NewPreference(pfxTx{}, repo, nil, func() int64 { return 8000000001 }, nil)
	s := NewInvoiceServer(nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.CreateBuyerInvoiceProfile(context.Background(), &pb.CreateBuyerInvoiceProfileRequest{
		BuyerId:     pBuyerA,
		CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		Value:       "/T7R3U+P",
	})
	if err != nil {
		t.Fatalf("建立不得回 gRPC error，實際 %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("預期成功，實際 code=%d message=%q", resp.GetCode(), resp.GetMessage())
	}
	raw, err := proto.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if containsBytes(raw, []byte("/T7R3U+P")) {
		t.Fatalf("回應的 wire bytes 內含載具明碼")
	}
}

// pfxCreateRepo 是 Create 路徑的最小 repo：空集合 ＋ 一定插得進去。
type pfxCreateRepo struct {
	domain.BuyerPreferenceRepository
}

func (r *pfxCreateRepo) ListByBuyer(context.Context, int64, int32) ([]domain.BuyerPreference, error) {
	return nil, nil
}

func (r *pfxCreateRepo) Create(_ context.Context, in domain.NewBuyerPreference, now time.Time) (domain.BuyerPreference, bool, error) {
	mask := in.CarrierMask
	ct := in.CarrierType
	return domain.BuyerPreference{
		ID:               in.ID,
		BuyerID:          in.BuyerID,
		PreferenceKind:   in.PreferenceKind,
		CarrierType:      &ct,
		CarrierValueMask: &mask,
		IsDefault:        in.IsDefault,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, true, nil
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
