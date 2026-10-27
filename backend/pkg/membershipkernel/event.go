package membershipkernel

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// Event type metadata is UPPER_SNAKE_CASE; routing keys are the separate dotted constants below.
	EventTypeMembershipEntitlementsChanged          = "MEMBERSHIP_ENTITLEMENTS_CHANGED"
	EventTypeMembershipEntitlementRevisionPublished = "MEMBERSHIP_ENTITLEMENT_REVISION_PUBLISHED"
	EventTypeMembershipPeriodStarted                = "MEMBERSHIP_PERIOD_STARTED"
	EventTypeMembershipChangeRequested              = "MEMBERSHIP_CHANGE_REQUESTED"
	EventTypeMembershipCanceled                     = "MEMBERSHIP_CANCELED"
	EventTypePaymentMembershipBuyerSuccess          = "PAYMENT_MEMBERSHIP_BUYER_SUCCESS"
	EventTypePaymentMembershipBuyerFailed           = "PAYMENT_MEMBERSHIP_BUYER_FAILED"
	EventTypePaymentMembershipSellerSuccess         = "PAYMENT_MEMBERSHIP_SELLER_SUCCESS"
	EventTypePaymentMembershipSellerFailed          = "PAYMENT_MEMBERSHIP_SELLER_FAILED"
)

const (
	RoutingKeyPayUniPaymentRecurring         = "payuni.payment.recurring"
	RoutingKeyPaymentMembershipBuyerSuccess  = "payment.membership.buyer.success"
	RoutingKeyPaymentMembershipBuyerFailed   = "payment.membership.buyer.failed"
	RoutingKeyPaymentMembershipSellerSuccess = "payment.membership.seller.success"
	RoutingKeyPaymentMembershipSellerFailed  = "payment.membership.seller.failed"
)

var (
	ErrInvalidEvent                   = errors.New("invalid membership event envelope")
	ErrMissingMembershipChangeRequest = errors.New("missing membership change request")
	ErrInvalidEventData               = errors.New("invalid membership event data")
	ErrEventRoutingScope              = errors.New("membership event routing scope mismatch")
)

// MembershipEventEnvelope is the canonical wire-level event shape. Provider
// or domain-specific data is carried as one versioned protobuf Any; consumers
// must not introduce a second buyer/seller wrapper around it.
type MembershipEventEnvelope struct {
	EventID                   string
	EventType                 string
	SchemaVersion             string
	Producer                  string
	Subject                   SubjectRef
	MembershipID              string
	MembershipPeriodID        string
	PlanID                    string
	OfferID                   string
	EntitlementRevisionID     string
	RevisionNo                string
	EffectiveAt               string
	OccurredAt                string
	CommandID                 string
	IdempotencyKey            string
	Data                      *anypb.Any
	MembershipChangeRequestID string
}

// Validate enforces the fields that every canonical event must carry. Initial
// payment events may omit MembershipPeriodID, but then a change request is
// mandatory so the domain owner can resolve the pending transition.
func (e MembershipEventEnvelope) Validate() error {
	for name, value := range map[string]string{
		"event_id":       e.EventID,
		"event_type":     e.EventType,
		"schema_version": e.SchemaVersion,
		"producer":       e.Producer,
		"membership_id":  e.MembershipID,
		"occurred_at":    e.OccurredAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s is required", ErrInvalidEvent, name)
		}
	}
	if err := e.Subject.Validate(); err != nil {
		return fmt.Errorf("%w: subject: %w", ErrInvalidEvent, err)
	}
	if err := validateUTC(e.OccurredAt); err != nil {
		return fmt.Errorf("%w: occurred_at: %w", ErrInvalidEvent, err)
	}
	if e.EffectiveAt != "" {
		if err := validateUTC(e.EffectiveAt); err != nil {
			return fmt.Errorf("%w: effective_at: %w", ErrInvalidEvent, err)
		}
	}
	if e.Data == nil || strings.TrimSpace(e.Data.TypeUrl) == "" || !strings.HasPrefix(e.Data.TypeUrl, "type.googleapis.com/") {
		return fmt.Errorf("%w: Any must have a type.googleapis.com type URL", ErrInvalidEventData)
	}
	if isProviderMembershipEvent(e.EventType) {
		if e.Producer != "payment-service" {
			return fmt.Errorf("%w: provider result producer must be payment-service", ErrInvalidEvent)
		}
		if e.MembershipPeriodID == "" && strings.TrimSpace(e.MembershipChangeRequestID) == "" {
			return ErrMissingMembershipChangeRequest
		}
	}
	return nil
}

func validateUTC(value string) error {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	_, offset := parsed.Zone()
	if offset != 0 {
		return errors.New("timestamp must be UTC")
	}
	return nil
}

func isProviderMembershipEvent(eventType string) bool {
	switch eventType {
	case EventTypePaymentMembershipBuyerSuccess, EventTypePaymentMembershipBuyerFailed,
		EventTypePaymentMembershipSellerSuccess, EventTypePaymentMembershipSellerFailed:
		return true
	default:
		return false
	}
}

// ValidateRoutingScope prevents a buyer consumer from silently accepting a
// seller billing event (and vice versa). The ingress recurring route is
// provider-scoped and intentionally has no subject-domain suffix.
func ValidateRoutingScope(subject SubjectRef, routingKey string) error {
	if err := subject.Validate(); err != nil {
		return fmt.Errorf("%w: subject: %s", ErrEventRoutingScope, err.Error())
	}
	switch routingKey {
	case RoutingKeyPaymentMembershipBuyerSuccess, RoutingKeyPaymentMembershipBuyerFailed:
		if subject.SubjectType != SubjectTypeBuyer {
			return fmt.Errorf("%w: buyer route received %s", ErrEventRoutingScope, subject.SubjectType)
		}
	case RoutingKeyPaymentMembershipSellerSuccess, RoutingKeyPaymentMembershipSellerFailed:
		if subject.SubjectType != SubjectTypeSeller {
			return fmt.Errorf("%w: seller route received %s", ErrEventRoutingScope, subject.SubjectType)
		}
	case RoutingKeyPayUniPaymentRecurring:
		// Provider ingress is normalized before domain scope is selected.
	default:
		return fmt.Errorf("%w: unknown routing key %q", ErrEventRoutingScope, routingKey)
	}
	return nil
}
