package grpc

import (
	invoicepb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
)

// NewInvoiceServiceClientForTest wraps a generated invoice proto client so a
// handler test can reach Holder.Invoice, whose `client` field is unexported.
//
// ⚠️ Pass a proto client built over a *grpc.ClientConn that was dialed WITH
// EnvelopeGuardInterceptor. The 58 號 §6.1.0(13) preserveData codes are defined
// entirely by that interceptor's `return nil` branch: injecting a bare mock
// makes every non-zero envelope code arrive as a raw reply, which is a shape the
// production chain never produces for non-preserveData codes — a test built that
// way would pass whether or not the handler checks resp.GetCode(). This mirrors
// the OrderClient pattern in checkout_suspension_envelope_test.go.
//
// This lives in its own file rather than in testing.go purely to avoid colliding
// with concurrent edits to that file; it belongs to the same family of
// constructors.
func NewInvoiceServiceClientForTest(c invoicepb.InvoiceServiceClient) *InvoiceServiceClient {
	return &InvoiceServiceClient{client: c}
}
