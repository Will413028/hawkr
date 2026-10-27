package social

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"google.golang.org/grpc"
)

// fakeSocialServiceClient satisfies pb.SocialServiceClient by embedding the
// interface (unimplemented methods panic-if-called) and overriding only
// GetPostList, so each test can control exactly what the "wire" returns
// without a live social-service. Mirrors the pattern used in
// internal/client/order/client_test.go.
type fakeSocialServiceClient struct {
	pb.SocialServiceClient
	resp *pb.PostListResponse
	err  error
}

func (f *fakeSocialServiceClient) GetPostList(_ context.Context, _ *pb.GetPostListRequest, _ ...grpc.CallOption) (*pb.PostListResponse, error) {
	return f.resp, f.err
}

// TestHasPublishedPost_TransportError covers the gRPC-call-failure branch
// (e.g. social-service down, connection refused, deadline exceeded). Before
// this fix the branch returned a plain fmt.Errorf, which does NOT implement
// pkgerrors.DomainCoder — CertificationUseCase.Submit()'s fail-closed gate
// would then propagate an error the gRPC interceptor maps to a raw internal/
// 500-shaped status instead of a clean "temporarily unavailable" one.
func TestHasPublishedPost_TransportError(t *testing.T) {
	transportErr := errors.New("connection refused")
	c := &Client{client: &fakeSocialServiceClient{err: transportErr}}

	hasPost, err := c.HasPublishedPost(context.Background(), 12345)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if hasPost {
		t.Error("expected hasPost = false on error")
	}
	if got := pkgerrors.GetCode(err); got != pkgerrors.CodeUnavailable {
		t.Fatalf("GetCode(err) = %v, want CodeUnavailable so it flows through the gRPC interceptor the same way as the resp.Code != 0 branch; err=%v", got, err)
	}
	var dc pkgerrors.DomainCoder
	if !errors.As(err, &dc) {
		t.Fatal("expected err to implement pkgerrors.DomainCoder")
	}
	if dc.DomainMessage() == "" {
		t.Error("expected a non-empty user-facing message")
	}
	// The user-facing message must stay clean (no raw transport details) —
	// the underlying cause is preserved on the DomainError for logs via
	// errors.Unwrap / %v on err.Error(), not surfaced to the caller.
	if dc.DomainMessage() == transportErr.Error() {
		t.Error("DomainMessage() should not be the raw transport error")
	}
	if !errors.Is(err, transportErr) {
		t.Error("expected the underlying transport error to remain unwrappable for logging")
	}
}

// TestHasPublishedPost_BusinessError covers the existing resp.Code != 0
// branch (social-service replied but its envelope carries a business-level
// error) — kept alongside the transport-error test above so both
// HasPublishedPost error paths have coverage in one place.
func TestHasPublishedPost_BusinessError(t *testing.T) {
	c := &Client{client: &fakeSocialServiceClient{resp: &pb.PostListResponse{Code: 1, Message: "boom"}}}

	hasPost, err := c.HasPublishedPost(context.Background(), 12345)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if hasPost {
		t.Error("expected hasPost = false on error")
	}
	if got := pkgerrors.GetCode(err); got != pkgerrors.CodeUnavailable {
		t.Fatalf("GetCode(err) = %v, want CodeUnavailable; err=%v", got, err)
	}
}

// TestHasPublishedPost_Success covers the happy path: Total > 0 means the
// seller has at least one published post.
func TestHasPublishedPost_Success(t *testing.T) {
	c := &Client{client: &fakeSocialServiceClient{resp: &pb.PostListResponse{Code: 0, Total: 3}}}

	hasPost, err := c.HasPublishedPost(context.Background(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasPost {
		t.Error("expected hasPost = true when Total > 0")
	}
}
