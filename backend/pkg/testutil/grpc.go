package testutil

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AssertGRPCCode 驗證 error 的 gRPC 狀態碼是否符合預期。
func AssertGRPCCode(tb testing.TB, err error, want codes.Code) {
	tb.Helper()
	if err == nil {
		tb.Fatalf("expected error with code %v, got nil", want)
	}
	st, ok := status.FromError(err)
	if !ok {
		tb.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != want {
		tb.Errorf("code = %v, want %v", st.Code(), want)
	}
}
