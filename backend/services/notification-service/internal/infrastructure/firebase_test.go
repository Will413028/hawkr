package infrastructure

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// Verify FirebaseNotifier satisfies the domain.PushNotifier interface at compile time.
// If any method signature drifts from the interface, this assertion fails to compile.
var _ domain.PushNotifier = (*FirebaseNotifier)(nil)

// ShouldDeactivateToken classifies via the FCM SDK's messagingErrorCode (typed
// helpers), not string matching. The positive paths (real UNREGISTERED /
// SENDER_ID_MISMATCH) can only be produced by *internal.FirebaseError, which the
// firebase SDK does not let external code construct, so they are exercised by the
// SDK's own tests + integration, not here. What we CAN (and must) lock down is the
// negative contract: arbitrary non-FCM errors must NOT be treated as a dead token
// — this is the regression vs the old string-matching IsTokenInvalid, which
// false-positived on any message containing "UNREGISTERED"/"not found"/etc. and
// could have wrongly deactivated tokens on unrelated errors.
func TestShouldDeactivateToken_NegativeContract(t *testing.T) {
	cases := []struct {
		err  error
		name string
	}{
		{name: "nil error", err: nil},
		{name: "network timeout", err: errors.New("network timeout")},
		{name: "plain text mentioning unregistered", err: errors.New("device was UNREGISTERED from our CRM")},
		{name: "plain text mentioning not found", err: errors.New("order not found")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ShouldDeactivateToken(tc.err) {
				t.Errorf("ShouldDeactivateToken(%v) = true, want false (non-FCM error must not deactivate)", tc.err)
			}
		})
	}
}

// NewTokenDeactivationPolicy returns a usable policy wired as domain.TokenDeactivationPolicy.
func TestNewTokenDeactivationPolicy(t *testing.T) {
	policy := NewTokenDeactivationPolicy()
	if policy == nil {
		t.Fatal("expected non-nil policy")
	}
	if policy(nil) {
		t.Error("policy(nil) must be false")
	}
}

func TestNewFirebaseNotifier_EmptyCredentialsDisablesPush(t *testing.T) {
	for _, creds := range []string{"", "  \n"} {
		cfg := &config.Config{Firebase: config.FirebaseConfig{CredentialsJSON: creds}}
		n, err := NewFirebaseNotifier(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("empty credentials must not fail startup: %v", err)
		}
		if _, ok := n.(disabledNotifier); !ok {
			t.Fatalf("empty credentials: want disabledNotifier, got %T", n)
		}
		ctx := context.Background()
		if _, err := n.Send(ctx, []string{"t"}, "a", "b", "", nil); !errors.Is(err, ErrPushDisabled) {
			t.Fatalf("Send: want ErrPushDisabled, got %v", err)
		}
		if _, err := n.SendDataOnly(ctx, []string{"t"}, nil); !errors.Is(err, ErrPushDisabled) {
			t.Fatalf("SendDataOnly: want ErrPushDisabled, got %v", err)
		}
		if _, err := n.SendToTopic(ctx, "topic", "a", "b", "", nil); !errors.Is(err, ErrPushDisabled) {
			t.Fatalf("SendToTopic: want ErrPushDisabled, got %v", err)
		}
	}
}
