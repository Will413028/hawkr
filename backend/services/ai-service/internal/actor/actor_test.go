package actor

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc/metadata"
)

func TestNewStoreActor(t *testing.T) {
	a := NewStoreActor(identity.StoreID(2685914682))
	if a.StateKey != StateKeyStore {
		t.Fatalf("StateKey = %q, want %q", a.StateKey, StateKeyStore)
	}
	if a.UserID != "2685914682" || a.StateVal != "2685914682" {
		t.Fatalf("UserID/StateVal = %q/%q, want 2685914682/2685914682", a.UserID, a.StateVal)
	}
}

func TestNewBuyerActor(t *testing.T) {
	a := NewBuyerActor(identity.BuyerID(7489534863566774901))
	if a.StateKey != StateKeyBuyer {
		t.Fatalf("StateKey = %q, want %q", a.StateKey, StateKeyBuyer)
	}
	if a.UserID != "7489534863566774901" {
		t.Fatalf("UserID = %q, want 7489534863566774901", a.UserID)
	}
}

func TestActor_StoreID_StoreActor_ReturnsID(t *testing.T) {
	a := NewStoreActor(identity.StoreID(2685914682))
	id, ok := a.StoreID()
	if !ok {
		t.Fatal("StoreID() returned ok=false for a store actor, want true")
	}
	if id != identity.StoreID(2685914682) {
		t.Fatalf("StoreID() = %v, want 2685914682", id)
	}
}

func TestActor_StoreID_BuyerActor_ReturnsFalse(t *testing.T) {
	// 一支 buyer actor 沒有 store 語意——StateKey 不是 StateKeyStore，即使
	// UserID 剛好是合法數字也要拒絕，不能只憑「解析得出數字」就放行。
	a := NewBuyerActor(identity.BuyerID(7489534863566774901))
	if _, ok := a.StoreID(); ok {
		t.Fatal("StoreID() returned ok=true for a buyer actor, want false")
	}
}

func TestActor_StoreID_ZeroValueActor_ReturnsFalse(t *testing.T) {
	// 零值 Actor（StateKey==""）不能被誤認成 store actor。
	var a Actor
	if _, ok := a.StoreID(); ok {
		t.Fatal("StoreID() returned ok=true for a zero-value actor, want false")
	}
}

func TestParseGRPCMetadata_Buyer_Valid(t *testing.T) {
	md := metadata.Pairs("x-buyer-id", "2623900520")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	a, ok := ParseGRPCMetadata(ctx, StateKeyBuyer)
	if !ok {
		t.Fatal("ParseGRPCMetadata returned ok=false, want true")
	}
	if a.UserID != "2623900520" {
		t.Fatalf("UserID = %q, want 2623900520", a.UserID)
	}
}

func TestParseGRPCMetadata_MissingHeader(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
	if _, ok := ParseGRPCMetadata(ctx, StateKeyBuyer); ok {
		t.Fatal("ParseGRPCMetadata returned ok=true for missing header, want false")
	}
}

func TestParseGRPCMetadata_ZeroValue(t *testing.T) {
	md := metadata.Pairs("x-buyer-id", "0")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if _, ok := ParseGRPCMetadata(ctx, StateKeyBuyer); ok {
		t.Fatal("ParseGRPCMetadata returned ok=true for zero value, want false")
	}
}

func TestParseGRPCMetadata_InvalidStateKey(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-buyer-id", "123"))
	if _, ok := ParseGRPCMetadata(ctx, "not_a_real_key"); ok {
		t.Fatal("ParseGRPCMetadata returned ok=true for unknown stateKey, want false")
	}
}
