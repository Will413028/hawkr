package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/commerce"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

func TestVendorPortalDraft_RegistersActorNone(t *testing.T) {
	for _, m := range []string{
		"/hawkr.ai.AIProcurementService/GetVendorPortalResponseDraft",
		"/hawkr.ai.AIProcurementService/UpsertVendorPortalResponseDraft",
	} {
		policy, ok := methodPolicies[m]
		if !ok {
			t.Fatalf("%s not registered（未登記的方法 interceptor 一律 fail closed）", m)
		}
		if policy.Actor != ActorNone {
			t.Fatalf("%s policy.Actor = %v, want ActorNone", m, policy.Actor)
		}
	}
}

func TestSellerResponseDraft_AppSideStaysActorSeller(t *testing.T) {
	policy, ok := methodPolicies["/hawkr.ai.AIProcurementService/UpsertSellerResponseDraft"]
	if !ok {
		t.Fatal("UpsertSellerResponseDraft not registered")
	}
	if policy.Actor != ActorSeller {
		t.Fatalf("policy.Actor = %v, want ActorSeller", policy.Actor)
	}
}

type unreachableSellerStore struct{ commerce.SellerResponseStore }

func vendorPortalServerWithUnreachableStore() *AIProcurementGrpcServer {
	uc := commerce.NewSellerResponseUseCase(unreachableSellerStore{}, nil, nil)
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{SellerResponse: uc})
}

func completeVendorVerificationProto() *aipb.VendorPortalVerification {
	return &aipb.VendorPortalVerification{
		Method: "GOOGLE", Email: "vendor@example.com", SessionId: "6f1c3a52-0d7e-4c55-9d52-3a2f4f0f9a10",
		VerifiedAt: timestamppb.New(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)),
	}
}

func TestVendorPortalWrites_WithoutCompleteEvidenceArePermissionDenied(t *testing.T) {
	s := vendorPortalServerWithUnreachableStore()
	cases := map[string]*aipb.VendorPortalVerification{
		"missing":      nil,
		"empty":        {},
		"empty method": func() *aipb.VendorPortalVerification { v := completeVendorVerificationProto(); v.Method = ""; return v }(),
		"empty email":  func() *aipb.VendorPortalVerification { v := completeVendorVerificationProto(); v.Email = ""; return v }(),
		"no time": func() *aipb.VendorPortalVerification {
			v := completeVendorVerificationProto()
			v.VerifiedAt = nil
			return v
		}(),
		"empty session": func() *aipb.VendorPortalVerification {
			v := completeVendorVerificationProto()
			v.SessionId = ""
			return v
		}(),
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			_, submitErr := s.SubmitVendorPortalResponse(context.Background(), &aipb.SubmitVendorPortalResponseRequest{
				RfqRecipientId: "42", ClientRequestId: "k", Verification: v,
			})
			_, withdrawErr := s.WithdrawVendorPortalResponse(context.Background(), &aipb.WithdrawVendorPortalResponseRequest{
				RfqRecipientId: "42", SellerResponseId: "7", ClientRequestId: "k", Verification: v,
			})
			_, declineErr := s.DeclineVendorPortalRfqPublication(context.Background(), &aipb.DeclineVendorPortalRfqPublicationRequest{
				RfqRecipientId: "42", ClientRequestId: "k", Verification: v,
			})
			for op, err := range map[string]error{"submit": submitErr, "withdraw": withdrawErr, "decline": declineErr} {
				if status.Code(err) != codes.PermissionDenied {
					t.Fatalf("%s code = %v (%v), want PermissionDenied", op, status.Code(err), err)
				}
			}
		})
	}
}

func TestFromVendorPortalVerificationProto_CarriesEveryField(t *testing.T) {
	got := fromVendorPortalVerificationProto(completeVendorVerificationProto())
	if got == nil || !got.Complete() || got.Email != "vendor@example.com" || got.SessionID == "" ||
		!got.VerifiedAt.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("mapped = %+v", got)
	}
	if fromVendorPortalVerificationProto(nil) != nil {
		t.Fatal("nil proto must map to nil")
	}
}
