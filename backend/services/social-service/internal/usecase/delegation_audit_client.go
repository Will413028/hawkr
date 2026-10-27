package usecase

import "context"

// DelegationAuditIntent mirrors RecordAuditIntentRequest (56 §7.2). Field
// semantics are the proto's; only the ones this service can populate are worth
// setting, the rest stay zero.
//
// It is a separate interface from AccountClient rather than three more methods
// on it because the consumer is different: AccountClient answers "who is this
// account", this one writes the audit trail for a delegated write. They share
// one gRPC connection (see internal/client/di.go) — a second connection to the
// same downstream would give the circuit breaker and the pool two independent
// views of one dependency.
type DelegationAuditIntent struct {
	SessionID  string
	Capability string
	// Action is a DelegationAuditAction value (CodeMeta).
	Action       string
	ResourceType string
	// ResourceID is empty for create-type operations; confirm fills it in.
	ResourceID   string
	ResourceName string
	// OperationReason may only be empty for the READ-denied audit rows;
	// account-service rejects an empty value anywhere else.
	OperationReason       string
	ResourceVersionBefore string
	// BeforeSnapshotJSON is the allow-listed snapshot; never tokens or PII.
	BeforeSnapshotJSON string
	IdempotencyKey     string
	RequestID          string
	CorrelationID      string
	UserAgent          string
	PayloadHash        string
	// NotifySeller must always be set explicitly. proto3 cannot tell "unset"
	// from false on the wire, so leaving it to the zero value means no outbox
	// row is written and the seller is never told — with the audit row itself
	// looking perfectly normal.
	NotifySeller bool
	// Origin is "interactive" or "scheduled". account-service only accepts
	// "scheduled" from a caller whose HMAC signature says social-service
	// (§7.6.3), which is why this client must carry the internal-auth
	// interceptor.
	Origin string
}

// DelegationAuditIntentResult is RecordAuditIntentData.
type DelegationAuditIntentResult struct {
	AuditLogID string
	// Replayed is true when the same idempotency key already produced a row;
	// the caller decides whether to re-run the domain write.
	Replayed bool
	Result   string
}

// DelegationChangedField is one row of the audit diff table.
type DelegationChangedField struct {
	Field  string
	Label  string
	Before string
	After  string
}

// DelegationAuditConfirm mirrors ConfirmAuditResultRequest.
//
// It has no NotifySeller: that was written at intent time and this phase only
// reads it back to decide whether to write the outbox row.
type DelegationAuditConfirm struct {
	AuditLogID string
	// Result is "SUCCEEDED" or "FAILED".
	Result               string
	BizCode              int32
	ResourceID           string
	ResourceVersionAfter string
	AfterSnapshotJSON    string
	ChangedFields        []DelegationChangedField
}

// DelegationAuditClient is the two-phase audit contract a delegated write must
// go through: intent before the domain write, confirm after it.
type DelegationAuditClient interface {
	RecordAuditIntent(ctx context.Context, in DelegationAuditIntent) (*DelegationAuditIntentResult, error)
	ConfirmAuditResult(ctx context.Context, in DelegationAuditConfirm) error
}
