package testutil

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAssertGRPCCode_Match(t *testing.T) {
	err := status.Error(codes.NotFound, "not found")
	AssertGRPCCode(t, err, codes.NotFound)
}

func TestAssertGRPCCode_VariousCodes(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
	}{
		{"InvalidArgument", codes.InvalidArgument},
		{"Internal", codes.Internal},
		{"PermissionDenied", codes.PermissionDenied},
		{"AlreadyExists", codes.AlreadyExists},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := status.Error(tt.code, "test")
			AssertGRPCCode(t, err, tt.code)
		})
	}
}
