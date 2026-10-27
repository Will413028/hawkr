package membershipkernel

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestMembershipEventEnvelopeValidateRequiresCanonicalIdentity(t *testing.T) {
	subject, err := NewSubjectRef(SubjectTypeBuyer, "123")
	if err != nil {
		t.Fatal(err)
	}
	envelope := MembershipEventEnvelope{
		EventID:                   "evt-1",
		EventType:                 EventTypePaymentMembershipBuyerSuccess,
		SchemaVersion:             "1",
		Producer:                  "payment-service",
		Subject:                   subject,
		MembershipID:              "membership-1",
		MembershipChangeRequestID: "change-1",
		OccurredAt:                "2026-09-02T10:00:00Z",
		Data:                      mustAny(t, wrapperspb.String("provider-result")),
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMembershipEventTypesUseMetadataConventionAndRoutesStaySeparate(t *testing.T) {
	for _, eventType := range []string{
		EventTypeMembershipEntitlementsChanged,
		EventTypeMembershipEntitlementRevisionPublished,
		EventTypeMembershipPeriodStarted,
		EventTypeMembershipChangeRequested,
		EventTypeMembershipCanceled,
		EventTypePaymentMembershipBuyerSuccess,
		EventTypePaymentMembershipBuyerFailed,
		EventTypePaymentMembershipSellerSuccess,
		EventTypePaymentMembershipSellerFailed,
	} {
		if eventType == "" || eventType != strings.ToUpper(eventType) || strings.Contains(eventType, ".") {
			t.Fatalf("event type %q is not UPPER_SNAKE_CASE metadata", eventType)
		}
	}
	if RoutingKeyPaymentMembershipBuyerSuccess == EventTypePaymentMembershipBuyerSuccess {
		t.Fatal("event type and routing key must remain distinct contracts")
	}
}

func TestMembershipEventEnvelopeRejectsMissingInitialChangeAndBadAny(t *testing.T) {
	subject, err := NewSubjectRef(SubjectTypeBuyer, "123")
	if err != nil {
		t.Fatal(err)
	}
	base := MembershipEventEnvelope{
		EventID:       "evt-1",
		EventType:     EventTypePaymentMembershipBuyerSuccess,
		SchemaVersion: "1",
		Producer:      "payment-service",
		Subject:       subject,
		MembershipID:  "membership-1",
		OccurredAt:    "2026-09-02T10:00:00Z",
		Data:          mustAny(t, wrapperspb.String("provider-result")),
	}
	if err := base.Validate(); !errors.Is(err, ErrMissingMembershipChangeRequest) {
		t.Fatalf("Validate() error = %v, want ErrMissingMembershipChangeRequest", err)
	}

	base.MembershipChangeRequestID = "change-1"
	base.Data = &anypb.Any{TypeUrl: "", Value: []byte("not typed")}
	if err := base.Validate(); !errors.Is(err, ErrInvalidEventData) {
		t.Fatalf("Validate() error = %v, want ErrInvalidEventData", err)
	}
}

func TestMembershipEventEnvelopeRoutingScope(t *testing.T) {
	buyer, _ := NewSubjectRef(SubjectTypeBuyer, "123")
	seller, _ := NewSubjectRef(SubjectTypeSeller, "456")
	for _, tc := range []struct {
		name  string
		subj  SubjectRef
		route string
		want  bool
	}{
		{name: "buyer success", subj: buyer, route: RoutingKeyPaymentMembershipBuyerSuccess, want: true},
		{name: "buyer cannot consume seller route", subj: buyer, route: RoutingKeyPaymentMembershipSellerSuccess, want: false},
		{name: "seller success", subj: seller, route: RoutingKeyPaymentMembershipSellerSuccess, want: true},
		{name: "seller cannot consume buyer route", subj: seller, route: RoutingKeyPaymentMembershipBuyerSuccess, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidateRoutingScope(tc.subj, tc.route); got != nil != !tc.want {
				t.Fatalf("ValidateRoutingScope() error = %v, want valid=%v", got, tc.want)
			}
		})
	}
}

func mustAny(t *testing.T, message proto.Message) *anypb.Any {
	t.Helper()
	any, err := anypb.New(message)
	if err != nil {
		t.Fatalf("anypb.New() error = %v", err)
	}
	return any
}
