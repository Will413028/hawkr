// Package delegation carries the Go half of the DelegationContext validation
// contract (56 號設計書 §6.2.4) plus the store-operability predicate (§8.1.3
// 第 8 項). The Java half lives in common-core's DelegationContextValidator; the
// two implement the same numbered steps (S1a–S5) so that one request cannot be
// accepted by seller-service and rejected by product-service. Both sides run the
// same fixture set (§13).
//
// seller-service, social-service and account-service share this one package —
// none of them may grow its own copy.
package delegation

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Capability is the closed enumeration of admin capability tokens (§6.1.1).
// Comparison is case sensitive on purpose: the token is a member of the
// frontend allow-list, so its casing is part of the contract. Normalising here
// would let the Gateway mint a value the frontend cannot recognise without the
// server-side check ever noticing.
type Capability string

const (
	CapStorefrontRead  Capability = "storefront.read"
	CapStorefrontWrite Capability = "storefront.write"
	CapProductsRead    Capability = "products.read"
	CapProductsWrite   Capability = "products.write"
	CapProductsPublish Capability = "products.publish"
	CapPostsRead       Capability = "posts.read"
	CapPostsWrite      Capability = "posts.write"
	CapPostsPublish    Capability = "posts.publish"
	CapOrdersRead      Capability = "orders.read"
)

// AllCapabilities lists the nine tokens in §6.1.1 order — the order the admin
// contract test asserts and the order both DB CHECKs enumerate.
var AllCapabilities = []Capability{
	CapStorefrontRead,
	CapStorefrontWrite,
	CapProductsRead,
	CapProductsWrite,
	CapProductsPublish,
	CapPostsRead,
	CapPostsWrite,
	CapPostsPublish,
	CapOrdersRead,
}

// knownCapabilities is the S2 allow-list. Deny by default: a token absent from
// this map is rejected even if it looks well formed.
var knownCapabilities = func() map[Capability]struct{} {
	m := make(map[Capability]struct{}, len(AllCapabilities))
	for _, c := range AllCapabilities {
		m[c] = struct{}{}
	}
	return m
}()

// IsKnown reports whether cap is one of the nine tokens. account-service uses it
// to reject a CreateSession request before the DB CHECK has to.
func IsKnown(cap Capability) bool {
	_, ok := knownCapabilities[cap]
	return ok
}

// requiresAuditLogID reports whether S1b applies to cap. The input is the verb
// segment: .write / .publish always carry a two-phase audit intent, .read never
// does (a successful read writes no audit row at all, §7.1.2). Making the field
// unconditionally mandatory would take every delegated read endpoint down with
// 9260 while each service's own unit tests — fed only write fixtures — stayed
// green.
func requiresAuditLogID(cap Capability) bool {
	switch cap {
	case CapStorefrontWrite, CapProductsWrite, CapProductsPublish, CapPostsWrite, CapPostsPublish:
		return true
	default:
		return false
	}
}

// Option tunes a single Validate call.
type Option func(*options)

type options struct {
	requestIdempotencyKey    string
	requestIdempotencyKeySet bool
}

// WithRequestIdempotencyKey feeds S5 the idempotency key the request message
// carries in its own field. Only the RPCs whose request actually has such a
// field pass it (product series, post create/update/status); for every other
// RPC S5 has nothing to compare and is skipped.
//
// It is an option rather than a fifth parameter so that the signature stays the
// one published in §6.2.4 — the RPCs without the field would otherwise all have
// to pass an empty string, and "empty string" is exactly the value S5 must not
// treat as "the caller sent a key".
func WithRequestIdempotencyKey(key string) Option {
	return func(o *options) {
		o.requestIdempotencyKey = key
		o.requestIdempotencyKeySet = true
	}
}

// bizStatus builds a gRPC status whose message carries the "[NNNN] " business
// code prefix the Gateway parses (§6.5.2, gateway handler GrpcDomainErr). Go
// services have no exception advice to prepend it for them the way the Java side
// does, and errors.NewBusiness is not a substitute: ErrorInterceptor's
// domainCodeToGRPC has no entry for the 9xxx range, so a business error would
// arrive as codes.Internal and lose both the status code §6.2.4 mandates and the
// business code the Gateway needs.
func bizStatus(c codes.Code, code int32, msg string) error {
	return status.Error(c, fmt.Sprintf("[%d] %s", code, msg))
}

// InvalidCommand is the 9260 rejection a domain service returns for a request
// its own field validation refuses (an out-of-range value, a field combination
// that has no meaning, a missing companion field).
//
// It exists so those rejections travel in the same shape as the ones Validate
// and ParseExpectedVersion produce: a gRPC status whose message carries the
// "[NNNN] " prefix the Gateway parses. A service that hand-rolls its own error
// here loses the business code somewhere in ErrorInterceptor and the operator
// sees a bare 500 instead of the field they got wrong.
func InvalidCommand(msg string) error {
	return bizStatus(codes.InvalidArgument, bizcode.DelegationInvalidCommand, msg)
}

// contractMismatch is every rejection Validate itself produces (#2744).
//
// None of S1a–S5 can be caused by an operator: DelegationContext is assembled by
// the Gateway from the session, never from anything a person types. A missing
// session_id, a capability outside the closed enumeration, a capability that
// disagrees with the RPC's own declaration — each of those means the Gateway
// built a context that contradicts its own contract, and no amount of retrying
// or re-filling a form changes it. The operator's next step is "tell an
// engineer", which differs from both 9260 (you sent a bad value) and 9280 (try
// again shortly) — and that difference is the whole reason the three codes exist
// separately.
//
// ⚠️ This is NOT InvalidCommand below: that one is a domain service rejecting a
// field the operator really did fill in wrong, and it stays 9260. The split is
// "who can fix this", not "which layer noticed".
//
// ⚠️ The gRPC status codes stay as they were. The Gateway parses the "[NNNN] "
// prefix first and only falls back to the status (admin_delegated_common.go
// denialFromGrpcErr) — and codes.Internal is in that function's transport-failure
// list, so switching to "the honest status" would short-circuit to 9280 before
// the honest code is ever read.
func contractMismatch(c codes.Code, msg string) error {
	return bizStatus(c, bizcode.DelegationInternalContractMismatch, msg)
}

// Validate runs steps S1a–S5 of §6.2.4 in fixed order, failing closed at the
// first violation.
//
// d == nil means this is the seller's own path: the judgement does not apply and
// the caller falls through to its existing seller authorisation (S0).
//
// storeID is the ownership key of this request, taken by each RPC from its own
// message. cap is the capability this RPC declares — it MUST be a literal
// constant written at the call site. Passing d.GetCapability() here collapses S3
// into comparing the value with itself, which leaves only format checking while
// every test still passes.
//
// S4 can only establish that an ownership key exists, never that it is the right
// one: DelegationContext deliberately carries no target_store_id. "The resource
// really belongs to this store" is Q4 (§7.1.2) and belongs in the query layer of
// each domain service (WHERE id = ? AND store_id = ?), which answers 9275.
func Validate(_ context.Context, d *commonpb.DelegationContext, storeID int64, cap Capability, opts ...Option) error {
	if d == nil {
		return nil
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	// S1a: the three always-required fields.
	if d.GetSessionId() == "" || d.GetOperatorAdminId() == "" || d.GetCapability() == "" {
		return contractMismatch(codes.InvalidArgument,
			"delegation context is missing session_id / operator_admin_id / capability")
	}

	// S1b: audit_log_id, only where an audit intent must already exist.
	if requiresAuditLogID(cap) && d.GetAuditLogId() == "" {
		return contractMismatch(codes.InvalidArgument,
			"delegation context is missing audit_log_id for a mutating capability")
	}

	// S2: capability is inside the closed enumeration.
	if !IsKnown(Capability(d.GetCapability())) {
		return contractMismatch(codes.PermissionDenied,
			"delegation capability is not recognised")
	}

	// S3: capability matches what this RPC declares.
	if Capability(d.GetCapability()) != cap {
		return contractMismatch(codes.PermissionDenied,
			"delegation capability does not match the capability this RPC requires")
	}

	// S4: ownership key present.
	if storeID <= 0 {
		return contractMismatch(codes.InvalidArgument,
			"delegation request is missing its owner store id")
	}

	// S5: the request's own idempotency key must agree with the context's.
	// Either side being empty means there is nothing to compare — picking one of
	// two differing keys is never correct, so only a real disagreement fails.
	if o.requestIdempotencyKeySet &&
		o.requestIdempotencyKey != "" && d.GetIdempotencyKey() != "" &&
		o.requestIdempotencyKey != d.GetIdempotencyKey() {
		return contractMismatch(codes.InvalidArgument,
			"idempotency_key differs between request and delegation context")
	}

	return nil
}

// IsDelegated reports whether d marks this call as a delegated operation.
// Callers use it for the branches that only exist on the delegated path (Q4
// ownership assertions, draft visibility), never as an authorisation decision on
// its own.
func IsDelegated(d *commonpb.DelegationContext) bool { return d != nil }

// ctxKey is the private context key for the validated delegation context.
type ctxKey struct{}

// NewContext stores an already-validated delegation context for the usecase and
// repository layers.
func NewContext(ctx context.Context, d *commonpb.DelegationContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, d)
}

// FromContext returns the validated delegation context. A false second value
// means the seller's own path.
//
// There is deliberately no generic request parser: every RPC's request type
// differs, so each call site calls req.GetDelegation() itself and hands the
// result to Validate.
func FromContext(ctx context.Context) (*commonpb.DelegationContext, bool) {
	d, ok := ctx.Value(ctxKey{}).(*commonpb.DelegationContext)
	if !ok || d == nil {
		return nil, false
	}
	return d, true
}
