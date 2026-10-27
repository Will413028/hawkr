package interceptor

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestSellerSuspendedFromContext_WithKey(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataKeySellerSuspended, "SELLER_SUSPENDED_VIOLATION"))

	reasonCode, suspended := SellerSuspendedFromContext(ctx)

	if !suspended {
		t.Fatalf("expected suspended=true when metadata key present with non-empty value")
	}
	if reasonCode != "SELLER_SUSPENDED_VIOLATION" {
		t.Fatalf("expected reasonCode=%q, got %q", "SELLER_SUSPENDED_VIOLATION", reasonCode)
	}
}

func TestSellerSuspendedFromContext_WithoutKey(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-other-key", "value"))

	reasonCode, suspended := SellerSuspendedFromContext(ctx)

	if suspended {
		t.Fatalf("expected suspended=false when metadata key absent")
	}
	if reasonCode != "" {
		t.Fatalf("expected empty reasonCode when metadata key absent, got %q", reasonCode)
	}
}

func TestSellerSuspendedFromContext_EmptyValue(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataKeySellerSuspended, ""))

	reasonCode, suspended := SellerSuspendedFromContext(ctx)

	if suspended {
		t.Fatalf("expected suspended=false when metadata value is empty string")
	}
	if reasonCode != "" {
		t.Fatalf("expected empty reasonCode when metadata value is empty string, got %q", reasonCode)
	}
}

func TestSellerSuspendedFromContext_NoIncomingMetadata(t *testing.T) {
	reasonCode, suspended := SellerSuspendedFromContext(context.Background())

	if suspended {
		t.Fatalf("expected suspended=false when no incoming metadata on context")
	}
	if reasonCode != "" {
		t.Fatalf("expected empty reasonCode when no incoming metadata on context, got %q", reasonCode)
	}
}
