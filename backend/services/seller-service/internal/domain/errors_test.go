package domain

import (
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

func TestErrMerchantProfileVersionConflict_Code(t *testing.T) {
	if got := pkgerrors.GetCode(ErrMerchantProfileVersionConflict); int32(got) != bizcode.PayuniMerchantProfileVersionConflict {
		t.Fatalf("code = %d, want %d", got, bizcode.PayuniMerchantProfileVersionConflict)
	}
}

func TestErrMerchantProfileIdempotencyKeyReused_Code(t *testing.T) {
	if got := pkgerrors.GetCode(ErrMerchantProfileIdempotencyKeyReused); int32(got) != bizcode.PayuniMerchantProfileIdempotencyKeyReused {
		t.Fatalf("code = %d, want %d", got, bizcode.PayuniMerchantProfileIdempotencyKeyReused)
	}
}

func TestErrMerchantProfileNotActive_Code(t *testing.T) {
	if got := pkgerrors.GetCode(ErrMerchantProfileNotActive); int32(got) != bizcode.PayuniMerchantProfileNotActive {
		t.Fatalf("code = %d, want %d", got, bizcode.PayuniMerchantProfileNotActive)
	}
}

func TestErrInvalidTransition_Error_IsChinese(t *testing.T) {
	err := &ErrInvalidTransition{From: StateForwarded, To: StateRejected}

	want := "目前狀態為「已送出至金流商」，無法轉換到「已退回」"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if got := err.DomainMessage(); got != want {
		t.Fatalf("DomainMessage() = %q, want %q (must match Error() — grpc/handler.go's 55 call sites all read err.Error(), not DomainMessage())", got, want)
	}
}

func TestErrInvalidTransition_Error_UnknownStateFallsBackToRaw(t *testing.T) {
	err := &ErrInvalidTransition{From: "not_a_real_state", To: StateForwarded}

	got := err.Error()
	if got == "" {
		t.Fatal("Error() must not be empty for an unmapped state")
	}
	want := "目前狀態為「not_a_real_state」，無法轉換到「已送出至金流商」"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
