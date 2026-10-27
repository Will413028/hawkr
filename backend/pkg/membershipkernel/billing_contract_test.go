package membershipkernel_test

import (
	"testing"

	membershippb "github.com/Will413028/hawkr/go-pkg/proto/membership"
	billingpb "github.com/Will413028/hawkr/go-pkg/proto/membershipbilling"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestBillingProtoExposesStableInternalServiceContract(t *testing.T) {
	service := billingpb.File_membership_billing_proto.Services().ByName("MembershipBillingService")
	if service == nil {
		t.Fatal("MembershipBillingService descriptor is missing")
	}
	wantMethods := []string{
		"CreateMembershipSubscription",
		"SuspendMembershipSubscription",
		"RestartMembershipSubscription",
		"EndMembershipSubscription",
	}
	if service.Methods().Len() != len(wantMethods) {
		t.Fatalf("method count = %d, want %d", service.Methods().Len(), len(wantMethods))
	}
	for _, methodName := range wantMethods {
		if service.Methods().ByName(protoreflect.Name(methodName)) == nil {
			t.Errorf("missing RPC %s", methodName)
		}
	}
	if billingpb.File_membership_billing_proto.Messages().ByName("BillingEvent") != nil {
		t.Fatal("BillingEvent wrapper must not be introduced; envelope is canonical")
	}
	if billingpb.File_membership_billing_proto.Messages().ByName("BuyerEvent") != nil {
		t.Fatal("BuyerEvent wrapper must not be introduced; envelope is canonical")
	}
	if billingpb.File_membership_billing_proto.Messages().ByName("SellerEvent") != nil {
		t.Fatal("SellerEvent wrapper must not be introduced; envelope is canonical")
	}
}

func TestBillingProtoUsesStringForNumericCommercialFields(t *testing.T) {
	quote := billingpb.File_membership_billing_proto.Messages().ByName("BillingQuote")
	for _, fieldName := range []string{"unit_amount_minor", "interval_count"} {
		field := quote.Fields().ByName(protoreflect.Name(fieldName))
		if field == nil {
			t.Fatalf("BillingQuote.%s is missing", fieldName)
		}
		if field.Kind() != protoreflect.StringKind {
			t.Errorf("BillingQuote.%s kind = %s, want string", fieldName, field.Kind())
		}
	}
	providerResult := billingpb.File_membership_billing_proto.Messages().ByName("ProviderChargeResultData")
	for _, fieldName := range []string{"amount_minor", "this_period", "valid_from", "valid_until"} {
		field := providerResult.Fields().ByName(protoreflect.Name(fieldName))
		if field == nil {
			t.Fatalf("ProviderChargeResultData.%s is missing", fieldName)
		}
		if field.Kind() != protoreflect.StringKind {
			t.Errorf("ProviderChargeResultData.%s kind = %s, want string", fieldName, field.Kind())
		}
	}
}

func TestBillingProtoBindsToCanonicalSubjectAndAnyEnvelope(t *testing.T) {
	create := billingpb.File_membership_billing_proto.Messages().ByName("CreateMembershipSubscriptionRequest")
	subjectField := create.Fields().ByName("subject")
	if subjectField == nil || subjectField.Message() != membershippb.File_membership_entitlement_proto.Messages().ByName("SubjectRef") {
		t.Fatal("Create request subject must use membership.v1.SubjectRef")
	}
	envelope := membershippb.File_membership_entitlement_proto.Messages().ByName("MembershipEventEnvelope")
	dataField := envelope.Fields().ByName("data")
	if dataField == nil || dataField.Message() != anypb.File_google_protobuf_any_proto.Messages().ByName("Any") {
		t.Fatal("event envelope data must be google.protobuf.Any")
	}
}
