package buyerid

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/calleridentity"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func currentBuyer(id string) GetterFunc {
	return func(_ context.Context, accountID int64) (*pb.BuyerInfo, error) {
		return &pb.BuyerInfo{Id: id, AccountId: fmt.Sprintf("%d", accountID)}, nil
	}
}

func fatalGetter(t *testing.T) GetterFunc {
	t.Helper()
	return func(context.Context, int64) (*pb.BuyerInfo, error) {
		t.Fatal("buyer-service must not be called")
		return nil, nil
	}
}

func TestCurrentBuyerID_DirectLookup(t *testing.T) {
	r := New(currentBuyer("7001"))

	got, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42))
	if err != nil || int64(got) != 7001 {
		t.Fatalf("CurrentBuyerID = %d, %v; want 7001, nil", got, err)
	}
}

// 404 = 無現任 buyer stint row（buyerRepository.GetByAccountID 回
// pkgerrors.NewNotFound，handler 轉成 envelope code 404）。404 顯式收在
// envelopeCodeToGRPC，EnvelopeGuardInterceptor 轉成 codes.NotFound——mock 必須
// 回傳 interceptor 轉換「後」的樣子（err!=nil, status.Code==NotFound），而非
// err=nil+resp=nil 這種正式環境不會出現的組合（真正過 interceptor 見
// resolver_envelope_test.go）。
func TestCurrentBuyerID_NoRowIsNoCurrentBuyer(t *testing.T) {
	r := New(GetterFunc(func(context.Context, int64) (*pb.BuyerInfo, error) {
		return nil, status.Error(codes.NotFound, "buyer not found")
	}))

	if _, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42)); !errors.Is(err, ErrNoCurrentBuyer) {
		t.Fatalf("err = %v, want ErrNoCurrentBuyer", err)
	}
}

// 防禦性分支：err == nil 但 resp 為 nil（buyer 模組的 handler 現行邏輯不會
// 產出這個組合，見 resolver.go 註解，但保留給未來 nil-Data 成功 response）。
func TestCurrentBuyerID_NilResponseIsNoCurrentBuyer(t *testing.T) {
	r := New(GetterFunc(func(context.Context, int64) (*pb.BuyerInfo, error) {
		return nil, nil
	}))

	if _, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42)); !errors.Is(err, ErrNoCurrentBuyer) {
		t.Fatalf("err = %v, want ErrNoCurrentBuyer", err)
	}
}

func TestCurrentBuyerID_WithoutMonolithCallsRPC(t *testing.T) {
	r := New(currentBuyer("7001"))

	got, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42))
	if err != nil || int64(got) != 7001 {
		t.Fatalf("CurrentBuyerID = %d, %v; want 7001, nil", got, err)
	}
}

func TestCurrentBuyerID_UpstreamErrorPropagates(t *testing.T) {
	boom := fmt.Errorf("buyer-service down")
	r := New(GetterFunc(func(context.Context, int64) (*pb.BuyerInfo, error) {
		return nil, boom
	}))

	if _, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42)); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want propagated upstream error", err)
	}
}

func TestCurrentBuyerID_UnexpectedCodeIsError(t *testing.T) {
	r := New(GetterFunc(func(context.Context, int64) (*pb.BuyerInfo, error) {
		return nil, status.Error(codes.Internal, "boom")
	}))

	_, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42))
	if err == nil || errors.Is(err, ErrNoCurrentBuyer) {
		t.Fatalf("err = %v, want generic error (not ErrNoCurrentBuyer)", err)
	}
}

func TestCurrentBuyerID_MonolithCaller(t *testing.T) {
	r := New(fatalGetter(t))
	for _, id := range []int64{0, 7001} {
		ctx := calleridentity.With(context.Background(), calleridentity.Identity{AccountID: 42, BuyerID: id})
		got, err := r.CurrentBuyerID(ctx, 42)
		if id == 0 {
			if !errors.Is(err, ErrNoCurrentBuyer) {
				t.Fatalf("err = %v", err)
			}
		} else if err != nil || int64(got) != id {
			t.Fatalf("got %d, %v", got, err)
		}
	}
}

func TestCurrentBuyerID_OtherAccountCallsRPC(t *testing.T) {
	calls := 0
	r := New(GetterFunc(func(_ context.Context, accountID int64) (*pb.BuyerInfo, error) {
		calls++
		if accountID != 43 {
			t.Fatalf("accountID = %d", accountID)
		}
		return &pb.BuyerInfo{Id: "8002"}, nil
	}))
	ctx := calleridentity.With(context.Background(), calleridentity.Identity{AccountID: 42, BuyerID: 7001})
	for range 2 {
		got, err := r.CurrentBuyerID(ctx, 43)
		if err != nil || got != 8002 {
			t.Fatalf("got %d, %v", got, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestCurrentBuyerID_AdminIDIsNotAccountID(t *testing.T) {
	calls := 0
	r := New(GetterFunc(func(_ context.Context, accountID int64) (*pb.BuyerInfo, error) {
		calls++
		if accountID != 42 {
			t.Fatalf("account = %d", accountID)
		}
		return &pb.BuyerInfo{Id: "8002"}, nil
	}))
	ctx := calleridentity.With(context.Background(), calleridentity.Identity{AccountID: 42, IsAdmin: true})
	got, err := r.CurrentBuyerID(ctx, 42)
	if err != nil || got != 8002 || calls != 1 {
		t.Fatalf("got %d, %v, calls %d", got, err, calls)
	}
}
