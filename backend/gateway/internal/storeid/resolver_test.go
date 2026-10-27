package storeid

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/calleridentity"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func activeSeller(storeID int64) GetterFunc {
	return func(_ context.Context, _ int64) (int64, error) {
		return storeID, nil
	}
}

func fatalGetter(t *testing.T) GetterFunc {
	t.Helper()
	return func(context.Context, int64) (int64, error) {
		t.Fatal("seller-service must not be called")
		return 0, nil
	}
}

func TestCurrentStoreID_DirectLookup(t *testing.T) {
	r := New(activeSeller(7001))

	got, err := r.CurrentStoreID(context.Background(), identity.AccountID(42))
	if err != nil || int64(got) != 7001 {
		t.Fatalf("CurrentStoreID = %d, %v; want 7001, nil", got, err)
	}
}

func TestCurrentStoreID_NoCurrentStore(t *testing.T) {
	r := New(GetterFunc(func(context.Context, int64) (int64, error) {
		return 0, status.Error(codes.NotFound, "no current store for account")
	}))

	if _, err := r.CurrentStoreID(context.Background(), identity.AccountID(42)); !errors.Is(err, ErrNoCurrentStore) {
		t.Fatalf("err = %v, want ErrNoCurrentStore", err)
	}
}

func TestCurrentStoreID_WithoutMonolithCallsRPC(t *testing.T) {
	r := New(activeSeller(7001))

	got, err := r.CurrentStoreID(context.Background(), identity.AccountID(42))
	if err != nil || int64(got) != 7001 {
		t.Fatalf("CurrentStoreID = %d, %v; want 7001, nil", got, err)
	}
}

func TestCurrentStoreID_UpstreamErrorPropagates(t *testing.T) {
	boom := fmt.Errorf("seller-service down")
	r := New(GetterFunc(func(context.Context, int64) (int64, error) {
		return 0, boom
	}))

	if _, err := r.CurrentStoreID(context.Background(), identity.AccountID(42)); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want propagated upstream error", err)
	}
}

func TestCurrentStoreID_UnexpectedCodeIsError(t *testing.T) {
	r := New(GetterFunc(func(context.Context, int64) (int64, error) {
		return 0, status.Error(codes.Internal, "boom")
	}))

	_, err := r.CurrentStoreID(context.Background(), identity.AccountID(42))
	if err == nil || errors.Is(err, ErrNoCurrentStore) {
		t.Fatalf("err = %v, want generic error (not ErrNoCurrentStore)", err)
	}
}

func TestCurrentStoreID_MonolithCaller(t *testing.T) {
	r := New(fatalGetter(t))
	for _, id := range []int64{0, 7001} {
		ctx := calleridentity.With(context.Background(), calleridentity.Identity{AccountID: 42, StoreID: id})
		got, err := r.CurrentStoreID(ctx, 42)
		if id == 0 {
			if !errors.Is(err, ErrNoCurrentStore) {
				t.Fatalf("err = %v", err)
			}
		} else if err != nil || int64(got) != id {
			t.Fatalf("got %d, %v", got, err)
		}
	}
}

func TestCurrentStoreID_OtherAccountCallsRPC(t *testing.T) {
	calls := 0
	r := New(GetterFunc(func(_ context.Context, accountID int64) (int64, error) {
		calls++
		if accountID != 43 {
			t.Fatalf("accountID = %d", accountID)
		}
		return 8002, nil
	}))
	ctx := calleridentity.With(context.Background(), calleridentity.Identity{AccountID: 42, StoreID: 7001})
	for range 2 {
		got, err := r.CurrentStoreID(ctx, 43)
		if err != nil || got != 8002 {
			t.Fatalf("got %d, %v", got, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestCurrentStoreID_AdminIDIsNotAccountID(t *testing.T) {
	calls := 0
	r := New(GetterFunc(func(_ context.Context, accountID int64) (int64, error) {
		calls++
		if accountID != 42 {
			t.Fatalf("account = %d", accountID)
		}
		return 8002, nil
	}))
	ctx := calleridentity.With(context.Background(), calleridentity.Identity{AccountID: 42, IsAdmin: true})
	got, err := r.CurrentStoreID(ctx, 42)
	if err != nil || got != 8002 || calls != 1 {
		t.Fatalf("got %d, %v, calls %d", got, err, calls)
	}
}
