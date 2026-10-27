package handler

import (
	"context"
	"errors"
	"testing"

	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// These guard the mock defects recorded on issue #2059: several
// mockAccountServiceClient methods invoked their Fn field unconditionally, so a
// test that had not opted into that call site died with a nil-func panic instead
// of failing on its own assertions — and SuspendAccount had no hook at all, so
// the suspension path could not be driven from a test in the first place.
//
// Each case calls the mock with the zero value (no Fn assigned), which is
// exactly the state that used to panic.

func TestMockAccountClient_GetWebSessionDefaultsToNotFound(t *testing.T) {
	m := &mockAccountServiceClient{}

	resp, err := m.GetWebSession(context.Background(), &accountpb.GetWebSessionRequest{TokenHash: "x"})

	if resp != nil {
		t.Errorf("default must not conjure a session, got %v", resp)
	}
	if grpcstatus.Code(err) != grpccodes.NotFound {
		t.Errorf("default = %v, want NotFound (an empty backing store holds no row)", err)
	}
}

func TestMockAccountClient_DeleteWebSessionDefaultsToSuccess(t *testing.T) {
	m := &mockAccountServiceClient{}

	resp, err := m.DeleteWebSession(context.Background(), &accountpb.DeleteWebSessionRequest{TokenHash: "x"})

	if err != nil {
		t.Errorf("delete is idempotent, so the default must succeed; got %v", err)
	}
	if resp == nil {
		t.Error("expected a non-nil response")
	}
}

func TestMockAccountClient_SuspendAccountDefaultsToSuccess(t *testing.T) {
	m := &mockAccountServiceClient{}

	resp, err := m.SuspendAccount(context.Background(), &accountpb.SuspendAccountRequest{AccountId: 1})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Errorf("default = %v, want Success=true", resp)
	}
}

// The point of the hook: a suspension-path test must be able to inject a
// failure and assert the call actually happened. Before #2059 the mock returned
// a hardcoded (nil, nil) with nowhere to intervene.
func TestMockAccountClient_SuspendAccountFnIsInjectable(t *testing.T) {
	want := grpcstatus.Error(grpccodes.Unavailable, "account-service down")
	var gotID int64

	m := &mockAccountServiceClient{
		SuspendAccountFn: func(ctx context.Context, in *accountpb.SuspendAccountRequest, opts ...grpc.CallOption) (*accountpb.SuspendAccountResponse, error) {
			gotID = in.AccountId
			return nil, want
		},
	}

	if _, err := m.SuspendAccount(context.Background(), &accountpb.SuspendAccountRequest{AccountId: 42}); !errors.Is(err, want) {
		t.Errorf("injected error not propagated: got %v", err)
	}
	if gotID != 42 {
		t.Errorf("hook saw account id %d, want 42", gotID)
	}
}
