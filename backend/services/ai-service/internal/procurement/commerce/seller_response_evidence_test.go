package commerce

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
)

func (f *fakeSellerStore) FindSellerResponseCommandByIdempotencyKey(_ context.Context, _ int64, key string) (repository.SellerResponseCommand, error) {
	for _, c := range f.commands {
		if c.ClientRequestID == key {
			return c, nil
		}
	}
	return repository.SellerResponseCommand{}, repository.ErrSellerResponseNotFound
}

func (f *fakeSellerStore) InsertSellerResponseCommand(_ context.Context, in repository.InsertSellerResponseCommandInput) (repository.SellerResponseCommand, bool, error) {
	for _, c := range f.commands {
		if c.ClientRequestID == in.ClientRequestID {
			return c, false, nil
		}
	}
	c := repository.SellerResponseCommand{
		ID: int64(len(f.commands) + 1), SellerResponseID: in.SellerResponseID, Kind: in.Kind,
		WithdrawnVersion: in.WithdrawnVersion, ClientRequestID: in.ClientRequestID,
		ReasonCode: in.ReasonCode, Verification: in.Verification,
	}
	f.commands = append(f.commands, c)
	return c, true, nil
}

func completeVerification() *repository.SellerResponseVerification {
	return &repository.SellerResponseVerification{
		Method:     repository.VerificationMethodGoogle,
		Email:      "vendor@example.com",
		VerifiedAt: time.Date(2026, 10, 4, 8, 0, 0, 0, time.FixedZone("TST", 8*3600)),
		SessionID:  "6f1c3a52-0d7e-4c55-9d52-3a2f4f0f9a10",
	}
}

func externalStore() *fakeSellerStore {
	store := baseSellerStore()
	store.invite = repository.PublicationInvite{ID: 4242, PublicationID: 10, ProspectID: 55}
	store.identity.Lane = repository.SellerResponseLaneExternal
	store.identity.StoreID = nil
	prospect := int64(55)
	store.identity.ProspectID = &prospect
	return store
}

var externalPrincipal = SellerPrincipal{InviteID: 4242}

func incompleteVerifications() map[string]*repository.SellerResponseVerification {
	mutate := func(f func(v *repository.SellerResponseVerification)) *repository.SellerResponseVerification {
		v := completeVerification()
		f(v)
		return v
	}
	return map[string]*repository.SellerResponseVerification{
		"nil":            nil,
		"empty method":   mutate(func(v *repository.SellerResponseVerification) { v.Method = "" }),
		"unknown method": mutate(func(v *repository.SellerResponseVerification) { v.Method = "EMAIL" }),
		"empty email":    mutate(func(v *repository.SellerResponseVerification) { v.Email = "" }),
		"zero time":      mutate(func(v *repository.SellerResponseVerification) { v.VerifiedAt = time.Time{} }),
		"empty session":  mutate(func(v *repository.SellerResponseVerification) { v.SessionID = "" }),
	}
}

func TestExternalCommands_IncompleteEvidenceIsRejectedBeforeAnyWrite(t *testing.T) {
	for name, v := range incompleteVerifications() {
		t.Run(name, func(t *testing.T) {
			store := externalStore()
			store.identity.LatestVersion = 1
			uc := newSellerUC(store)

			_, submitErr := uc.Submit(context.Background(), SubmitSellerResponseInput{
				Principal: externalPrincipal, ClientRequestID: "k-s", Quote: validQuote(), Verification: v,
			})
			_, withdrawErr := uc.Withdraw(context.Background(), WithdrawSellerResponseInput{
				Principal: externalPrincipal, ClientRequestID: "k-w", ExpectedSellerResponseVersion: "1", Verification: v,
			})
			_, declineErr := uc.Decline(context.Background(), DeclineRfqPublicationInput{
				Principal: externalPrincipal, ClientRequestID: "k-d", Verification: v,
			})
			for op, err := range map[string]error{"submit": submitErr, "withdraw": withdrawErr, "decline": declineErr} {
				if !errors.Is(err, ErrVerificationRequired) {
					t.Fatalf("%s err = %v, want ErrVerificationRequired", op, err)
				}
			}
			if len(store.insertCalls) != 0 || len(store.commands) != 0 || len(store.bumpCalls) != 0 {
				t.Fatalf("wrote despite missing evidence: versions=%d commands=%d bumps=%d",
					len(store.insertCalls), len(store.commands), len(store.bumpCalls))
			}
		})
	}
}

func TestExternalSubmit_StoresEvidenceOnTheVersion(t *testing.T) {
	store := externalStore()
	uc := newSellerUC(store)
	v := completeVerification()

	if _, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: externalPrincipal, ClientRequestID: "k-s", Quote: validQuote(), Verification: v,
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := store.insertCalls[0].Verification
	if got == nil || got.Method != "GOOGLE" || got.Email != "vendor@example.com" || got.SessionID != v.SessionID {
		t.Fatalf("stored verification = %+v", got)
	}
	if got.VerifiedAt.Location() != time.UTC || !got.VerifiedAt.Equal(v.VerifiedAt) {
		t.Fatalf("verifiedAt = %v, want the same instant in UTC", got.VerifiedAt)
	}
}

func TestPlatformCommands_NeverStoreEvidence(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 1
	uc := newSellerUC(store)
	principal := SellerPrincipal{StoreID: 77}

	if _, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: principal, PublicationID: 10, ClientRequestID: "k-s", ExpectedSellerResponseVersion: "1",
		Quote: validQuote(), Verification: completeVerification(),
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := uc.Withdraw(context.Background(), WithdrawSellerResponseInput{
		Principal: principal, PublicationID: 10, ClientRequestID: "k-w", ExpectedSellerResponseVersion: "1",
		Verification: completeVerification(),
	}); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if store.insertCalls[0].Verification != nil {
		t.Fatalf("platform version carries evidence: %+v", store.insertCalls[0].Verification)
	}
	if len(store.commands) != 1 || store.commands[0].Verification != nil {
		t.Fatalf("platform command = %+v", store.commands)
	}

	store2 := baseSellerStore()
	if _, err := newSellerUC(store2).Decline(context.Background(), DeclineRfqPublicationInput{
		Principal: principal, PublicationID: 10, ClientRequestID: "k-d",
	}); err != nil {
		t.Fatalf("decline: %v", err)
	}
	if len(store2.commands) != 1 || store2.commands[0].Verification != nil {
		t.Fatalf("platform decline command = %+v", store2.commands)
	}
}

func TestExternalWithdraw_WritesOneRecordAndDerivesIdentityFromIt(t *testing.T) {
	store := externalStore()
	store.identity.LatestVersion = 2
	uc := newSellerUC(store)

	out, err := uc.Withdraw(context.Background(), WithdrawSellerResponseInput{
		Principal: externalPrincipal, ClientRequestID: "k-w", ExpectedSellerResponseVersion: "2",
		ReasonCode: "PRICE_CHANGED", Verification: completeVerification(),
	})
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if len(store.commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(store.commands))
	}
	c := store.commands[0]
	if c.Kind != repository.SellerResponseCommandWithdraw || c.WithdrawnVersion == nil || *c.WithdrawnVersion != 2 ||
		c.ReasonCode != "PRICE_CHANGED" || c.Verification == nil || c.ClientRequestID != "k-w" {
		t.Fatalf("command = %+v", c)
	}
	if out.WithdrawnVersion == nil || *out.WithdrawnVersion != *c.WithdrawnVersion {
		t.Fatalf("identity withdrawnVersion = %v, record = %v", out.WithdrawnVersion, c.WithdrawnVersion)
	}
}

func TestExternalWithdraw_ReplayReturnsSameResultAndWritesNothing(t *testing.T) {
	store := externalStore()
	store.identity.LatestVersion = 2
	uc := newSellerUC(store)
	in := WithdrawSellerResponseInput{
		Principal: externalPrincipal, ClientRequestID: "k-w", ExpectedSellerResponseVersion: "2",
		ReasonCode: "X", Verification: completeVerification(),
	}
	if _, err := uc.Withdraw(context.Background(), in); err != nil {
		t.Fatalf("first: %v", err)
	}
	store.withdrawErr = errors.New("must not be called on replay")
	if _, err := uc.Withdraw(context.Background(), in); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(store.commands) != 1 {
		t.Fatalf("commands = %d after replay, want 1", len(store.commands))
	}
}

func TestExternalWithdraw_SameKeyDifferentContentIsConflict(t *testing.T) {
	for name, mutate := range map[string]func(*WithdrawSellerResponseInput){
		"version": func(in *WithdrawSellerResponseInput) { in.ExpectedSellerResponseVersion = "3" },
		"reason":  func(in *WithdrawSellerResponseInput) { in.ReasonCode = "OTHER" },
	} {
		t.Run(name, func(t *testing.T) {
			store := externalStore()
			store.identity.LatestVersion = 3
			uc := newSellerUC(store)
			first := WithdrawSellerResponseInput{
				Principal: externalPrincipal, ClientRequestID: "k-w", ExpectedSellerResponseVersion: "2",
				ReasonCode: "X", Verification: completeVerification(),
			}
			store.commands = []repository.SellerResponseCommand{{
				SellerResponseID: 300, Kind: repository.SellerResponseCommandWithdraw,
				WithdrawnVersion: ptr32(2), ClientRequestID: "k-w", ReasonCode: "X",
			}}
			mutate(&first)
			_, err := uc.Withdraw(context.Background(), first)
			if !errors.Is(err, repository.ErrSellerResponseIdempotencyConflict) {
				t.Fatalf("err = %v, want ErrSellerResponseIdempotencyConflict", err)
			}
			if len(store.commands) != 1 {
				t.Fatalf("commands = %d, want the original only", len(store.commands))
			}
		})
	}
}

func TestExternalWithdraw_KeyUsedByDeclineIsConflict(t *testing.T) {
	store := externalStore()
	store.identity.LatestVersion = 1
	store.commands = []repository.SellerResponseCommand{{
		SellerResponseID: 300, Kind: repository.SellerResponseCommandDecline, ClientRequestID: "k",
	}}
	_, err := newSellerUC(store).Withdraw(context.Background(), WithdrawSellerResponseInput{
		Principal: externalPrincipal, ClientRequestID: "k", ExpectedSellerResponseVersion: "1",
		Verification: completeVerification(),
	})
	if !errors.Is(err, repository.ErrSellerResponseIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrSellerResponseIdempotencyConflict", err)
	}
}

func TestExternalDecline_WritesOneRecordAndReplaysByKey(t *testing.T) {
	store := externalStore()
	uc := newSellerUC(store)
	in := DeclineRfqPublicationInput{
		Principal: externalPrincipal, ClientRequestID: "k-d", ReasonCode: "NO_STOCK", Verification: completeVerification(),
	}
	if _, err := uc.Decline(context.Background(), in); err != nil {
		t.Fatalf("decline: %v", err)
	}
	if len(store.commands) != 1 || store.commands[0].Kind != repository.SellerResponseCommandDecline ||
		store.commands[0].ReasonCode != "NO_STOCK" || store.commands[0].Verification == nil {
		t.Fatalf("commands = %+v", store.commands)
	}

	at := sellerResponseTestNow
	store.identity.WriteClosedAt = &at
	store.closeErr = errors.New("must not be called on replay")
	if _, err := uc.Decline(context.Background(), in); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(store.commands) != 1 {
		t.Fatalf("commands = %d after replay, want 1", len(store.commands))
	}

	in.ReasonCode = "OTHER"
	if _, err := uc.Decline(context.Background(), in); !errors.Is(err, repository.ErrSellerResponseIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrSellerResponseIdempotencyConflict", err)
	}
}

func TestExternalWithdraw_ReplayAfterResubmitReturnsTheOriginalOutcome(t *testing.T) {
	store := externalStore()
	store.identity.LatestVersion = 2
	store.identity.WithdrawnVersion = nil
	at := time.Date(2026, 10, 4, 1, 0, 0, 0, time.UTC)
	store.commands = []repository.SellerResponseCommand{{
		SellerResponseID: 300, Kind: repository.SellerResponseCommandWithdraw,
		WithdrawnVersion: ptr32(1), ClientRequestID: "k-w", CreatedAt: at,
	}}
	got, err := newSellerUC(store).Withdraw(context.Background(), WithdrawSellerResponseInput{
		Principal: externalPrincipal, ClientRequestID: "k-w", ExpectedSellerResponseVersion: "1",
		Verification: completeVerification(),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got.WithdrawnVersion == nil || *got.WithdrawnVersion != 1 || got.LatestVersion != 1 ||
		got.WithdrawnAt == nil || !got.WithdrawnAt.Equal(at) {
		t.Fatalf("replay = latest %d withdrawn %v at %v, want the original outcome (1/1/%v)",
			got.LatestVersion, got.WithdrawnVersion, got.WithdrawnAt, at)
	}
}
