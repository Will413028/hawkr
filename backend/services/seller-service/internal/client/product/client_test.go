package product

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"google.golang.org/grpc"
)

// stubProductService implements only the RPCs these tests exercise. The embedded
// interface is nil on purpose: any other method panics loudly rather than
// returning a zero value that a test could silently mistake for a real answer.
type stubProductService struct {
	pb.ProductServiceClient

	listResp *pb.ListStoreIDsWithPublicProductsInternalResponse
	listErr  error
	listCall int
}

func (s *stubProductService) ListStoreIDsWithPublicProductsInternal(
	_ context.Context,
	_ *pb.ListStoreIDsWithPublicProductsInternalRequest,
	_ ...grpc.CallOption,
) (*pb.ListStoreIDsWithPublicProductsInternalResponse, error) {
	s.listCall++
	return s.listResp, s.listErr
}

func newClientWith(stub pb.ProductServiceClient) *Client { return &Client{client: stub} }

// TestListStoreIDsWithPublicProducts_ParsesStoreIDs is the happy path: the wire
// format is numeric strings (proto §4 keeps ids as string), the domain wants
// int64.
func TestListStoreIDsWithPublicProducts_ParsesStoreIDs(t *testing.T) {
	stub := &stubProductService{
		listResp: &pb.ListStoreIDsWithPublicProductsInternalResponse{
			StoreIds: []string{"2685914682", "2632049831"},
		},
	}

	got, err := newClientWith(stub).ListStoreIDsWithPublicProducts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int64{2685914682, 2632049831}
	if len(got) != len(want) {
		t.Fatalf("store ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("store ids = %v, want %v (order preserved)", got, want)
		}
	}
	if stub.listCall != 1 {
		t.Errorf("RPC called %d times, want 1", stub.listCall)
	}
}

// TestListStoreIDsWithPublicProducts_EmptyIsNotAnError: "nobody has a public
// product" is a real answer. The caller models it as resolved-empty and filters
// everything out, which is only correct if this layer reports it as success.
func TestListStoreIDsWithPublicProducts_EmptyIsNotAnError(t *testing.T) {
	stub := &stubProductService{
		listResp: &pb.ListStoreIDsWithPublicProductsInternalResponse{StoreIds: nil},
	}

	got, err := newClientWith(stub).ListStoreIDsWithPublicProducts(context.Background())
	if err != nil {
		t.Fatalf("an empty set must be a successful answer, got error: %v", err)
	}
	if got == nil {
		t.Fatal("want a non-nil empty slice — a nil result reads as 'no answer' one layer up")
	}
	if len(got) != 0 {
		t.Fatalf("store ids = %v, want empty", got)
	}
}

// TestListStoreIDsWithPublicProducts_NonZeroCodeIsAnError is the load-bearing
// case for this whole package. A non-zero business code MUST surface as an
// error, because the usecase distinguishes "could not ask" (fail open, show
// everything) from "asked, nobody qualifies" (filter everything). Degrading a
// failure into a successful empty set collapses those two into the second one
// and silently blanks the entire storefront rail — with no error anywhere.
func TestListStoreIDsWithPublicProducts_NonZeroCodeIsAnError(t *testing.T) {
	stub := &stubProductService{
		listResp: &pb.ListStoreIDsWithPublicProductsInternalResponse{
			Code:     500,
			Message:  "internal error",
			StoreIds: nil,
		},
	}

	got, err := newClientWith(stub).ListStoreIDsWithPublicProducts(context.Background())
	if err == nil {
		t.Fatalf("non-zero code must be an error, got success with %v — the caller would read that as 'no store has a public product' and empty the rail", got)
	}
	if got != nil {
		t.Errorf("result = %v, want nil alongside the error", got)
	}
	// The code and message are the only diagnosis a fail-open caller logs.
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "internal error") {
		t.Errorf("error = %q, want it to carry code=500 and the peer's message", err)
	}
}

// TestListStoreIDsWithPublicProducts_MalformedStoreIDIsAnError: dropping an
// unparsable id would quietly hide that one seller from the carousel with no
// signal at all. Failing the call instead routes it to the caller's fail-open
// path, which shows everyone — the safe direction for a public rail.
func TestListStoreIDsWithPublicProducts_MalformedStoreIDIsAnError(t *testing.T) {
	stub := &stubProductService{
		listResp: &pb.ListStoreIDsWithPublicProductsInternalResponse{
			StoreIds: []string{"2685914682", "not-a-number", "2632049831"},
		},
	}

	got, err := newClientWith(stub).ListStoreIDsWithPublicProducts(context.Background())
	if err == nil {
		t.Fatalf("a malformed store_id must fail the call, got %v (the bad id would just vanish, hiding that seller with no signal)", got)
	}
	if got != nil {
		t.Errorf("result = %v, want nil alongside the error", got)
	}
	if !strings.Contains(err.Error(), "not-a-number") {
		t.Errorf("error = %q, want it to name the offending value", err)
	}
}

// TestListStoreIDsWithPublicProducts_TransportErrorIsWrapped keeps the transport
// failure distinguishable from the business-code failure above.
func TestListStoreIDsWithPublicProducts_TransportErrorIsWrapped(t *testing.T) {
	sentinel := errors.New("connection refused")
	stub := &stubProductService{listErr: sentinel}

	got, err := newClientWith(stub).ListStoreIDsWithPublicProducts(context.Background())
	if err == nil {
		t.Fatalf("transport failure must be an error, got %v", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the transport error", err)
	}
	if got != nil {
		t.Errorf("result = %v, want nil alongside the error", got)
	}
}
