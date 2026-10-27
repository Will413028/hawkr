package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/fake"
	"github.com/Will413028/hawkr/go-pkg/mq"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubInviteClient struct {
	secret string
	email  string
	err    error
}

func (s *stubInviteClient) EnsureInvite(_ context.Context, prospectID, rfqRecipientID, expiresAt string) (inviteID, secret, invitedEmail string, err error) {
	return "555", s.secret, s.email, s.err
}

const externalBody = `{"inviteId":"123","publicationId":"456","prospectId":"789","expiresAt":"2026-10-11T15:59:59Z"}`

func TestVendorInviteHandler_SendsMailWithPortalLink(t *testing.T) {
	sender := &fake.Sender{Receipt: mail.Receipt{ProviderMessageID: "m-1"}}
	var marked, markedSecret string
	h := newVendorInviteHandler(vendorInviteDeps{
		Invites:  &stubInviteClient{secret: "abc123", email: "vendor@example.com"},
		Mail:     sender,
		From:     mail.Address{Email: "supports@example.com", Name: "範例生鮮"},
		BaseURL:  "https://www.example.com",
		MarkSent: func(_ context.Context, id, secret string) error { marked, markedSecret = id, secret; return nil },
		Logger:   zap.NewNop(),
	})

	require.NoError(t, h(context.Background(),
		"procurement.rfq_publication_invite.external_created", []byte(externalBody)))

	require.Len(t, sender.Sent, 1)
	got := sender.Sent[0]
	require.Equal(t, "supports@example.com", got.From.Email)
	require.Equal(t, "vendor@example.com", got.To.Email)
	require.Contains(t, got.Text, "https://www.example.com/vendor/rfq#invite=abc123")
	require.Equal(t, map[string]string{"rfq_invite_id": "123"}, got.Tags)
	require.Empty(t, got.IdempotencyKey,
		"⛔ 這條線刻意不用 provider idempotency：它會擋住輪替後的重寄，讓對方手上那封信的連結對不上 DB")
	require.Equal(t, "555", marked)
	require.Equal(t, "abc123", markedSecret, "Mark 要帶 Ensure 回來、寄進信裡的那一把 secret")

	// ⛔ 信裡不得出現價格、配送地址或買家身分。
	require.NotContains(t, got.Text, "budget")
	require.NotContains(t, got.HTML, "deliveryAddress")
}

// 已交付 ⇒ no-op ack，不得再寄一封。
func TestVendorInviteHandler_AlreadyDeliveredIsNoOp(t *testing.T) {
	sender := &fake.Sender{}
	h := newVendorInviteHandler(vendorInviteDeps{
		Invites: &stubInviteClient{err: status.Error(codes.AlreadyExists, "already delivered")},
		Mail:    sender, From: mail.Address{Email: "supports@example.com"},
		BaseURL:  "https://www.example.com",
		MarkSent: func(context.Context, string, string) error { return nil },
		Logger:   zap.NewNop(),
	})

	require.NoError(t, h(context.Background(),
		"procurement.rfq_publication_invite.external_created", []byte(externalBody)))
	require.Empty(t, sender.Sent)
}

// ErrPermanent ⇒ ack（不重試）；ErrTransient ⇒ 回錯誤讓 MQ 重投。
func TestVendorInviteHandler_ErrorClassificationDrivesAck(t *testing.T) {
	base := func(sendErr error) mq.HandlerFunc {
		return newVendorInviteHandler(vendorInviteDeps{
			Invites:  &stubInviteClient{secret: "s"},
			Mail:     &fake.Sender{Err: sendErr},
			From:     mail.Address{Email: "supports@example.com"},
			BaseURL:  "https://www.example.com",
			MarkSent: func(context.Context, string, string) error { return nil },
			Logger:   zap.NewNop(),
		})
	}

	require.NoError(t, base(fmt.Errorf("%w: boom", mail.ErrPermanent))(
		context.Background(), "procurement.rfq_publication_invite.external_created", []byte(externalBody)),
		"permanent 要 ack：持續重送死信箱會拉低網域信譽，連坐買家的訂單信")

	require.Error(t, base(mail.ErrTransient)(
		context.Background(), "procurement.rfq_publication_invite.external_created", []byte(externalBody)))
}

// 其他 routing key 必須原樣交給既有的 template handler。
func TestVendorInviteHandler_DelegatesOtherRoutingKeys(t *testing.T) {
	called := false
	wrapped := withVendorInvite(
		func(context.Context, string, []byte) error { called = true; return nil },
		func(context.Context, string, []byte) error { t.Fatal("不該進 vendor 分支"); return nil },
	)
	require.NoError(t, wrapped(context.Background(), "procurement.quote_invite.created", []byte(`{}`)))
	require.True(t, called)
}

func statusWithReason(t *testing.T, code codes.Code, reason string) error {
	t.Helper()
	st, err := status.New(code, "refused").WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "hawkr"})
	require.NoError(t, err)
	return st.Err()
}

type markRecorder struct {
	calls     int
	inviteID  string
	secret    string
	returnErr error
}

func (m *markRecorder) mark(_ context.Context, inviteID, secret string) error {
	m.calls++
	m.inviteID, m.secret = inviteID, secret
	return m.returnErr
}

func runVendorInvite(t *testing.T, ensureErr error, rec *markRecorder) (error, *fake.Sender) {
	t.Helper()
	sender := &fake.Sender{Receipt: mail.Receipt{ProviderMessageID: "m-1"}}
	h := newVendorInviteHandler(vendorInviteDeps{
		Invites: &stubInviteClient{secret: "abc123", email: "vendor@example.com", err: ensureErr},
		Mail:    sender, From: mail.Address{Email: "supports@example.invalid"},
		BaseURL:  "https://portal.example.invalid",
		MarkSent: rec.mark,
		Logger:   zap.NewNop(),
	})
	err := h(context.Background(), "procurement.rfq_publication_invite.external_created", []byte(externalBody))
	return err, sender
}

func counter(result string) float64 {
	var m dto.Metric
	if err := vendorInviteEmailTotal.WithLabelValues(result).Write(&m); err != nil {
		panic(err)
	}
	return m.GetCounter().GetValue()
}

// Ensure 的三種 FailedPrecondition、Mark 的 superseded 各有自己的 label，都不重試、不寄信（superseded 除外：信已寄出）。
func TestVendorInviteHandler_EachOutcomeHasItsOwnLabelAndIsNotRetried(t *testing.T) {
	cases := []struct {
		name      string
		ensureErr error
		markErr   error
		label     string
		sent      int
		permanent bool
	}{
		{"已撤銷", statusWithReason(t, codes.FailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_REVOKED.String()), nil, "invite_revoked", 0, false},
		{"業者不是進行中", statusWithReason(t, codes.FailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_PROSPECT_NOT_ACTIVE.String()), nil, "prospect_not_active", 0, false},
		{"沒有可寄的 email", statusWithReason(t, codes.FailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_NO_DELIVERABLE_EMAIL.String()), nil, "no_email", 0, false},
		{"沒有 reason 的 FailedPrecondition", status.Error(codes.FailedPrecondition, "legacy"), nil, "precondition_unknown", 0, true},
		{"不認得的 reason", statusWithReason(t, codes.FailedPrecondition, "SOMETHING_NEW"), nil, "precondition_unknown", 0, true},
		{"reason 是 UNSPECIFIED", statusWithReason(t, codes.FailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_UNSPECIFIED.String()), nil, "precondition_unknown", 0, true},
		{"Mark 的 secret 過時", nil, statusWithReason(t, codes.Aborted, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_SECRET_SUPERSEDED.String()), "secret_superseded", 1, false},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := counter(tc.label)
			rec := &markRecorder{returnErr: tc.markErr}
			err, sender := runVendorInvite(t, tc.ensureErr, rec)
			if tc.permanent {
				require.ErrorIs(t, err, mq.ErrPermanent, "未知的 precondition 要進死信，之後可重放")
			} else {
				require.NoError(t, err, "不可重試的結果要 ack")
			}
			require.Len(t, sender.Sent, tc.sent)
			require.Equal(t, before+1, counter(tc.label))
		})
		seen[tc.label] = true
	}
	require.Len(t, seen, 5, "四種結果加未知的 precondition 共五個互不相同的 label")
}

// 一種結果只動自己的 label，不動別人的。
func TestVendorInviteHandler_OutcomeOnlyMovesItsOwnLabel(t *testing.T) {
	labels := []string{"no_email", "invite_revoked", "prospect_not_active", "precondition_unknown", "secret_superseded"}
	revoked := statusWithReason(t, codes.FailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_REVOKED.String())
	before := map[string]float64{}
	for _, l := range labels {
		before[l] = counter(l)
	}
	err, _ := runVendorInvite(t, revoked, &markRecorder{})
	require.NoError(t, err)
	for _, l := range labels {
		want := before[l]
		if l == "invite_revoked" {
			want++
		}
		require.Equal(t, want, counter(l), l)
	}
}

// 其他 Mark 錯誤照舊回傳（讓 MQ 重投）：Aborted 但沒有 superseded 的 reason 也一樣。
func TestVendorInviteHandler_OtherMarkErrorsAreReturned(t *testing.T) {
	for name, markErr := range map[string]error{
		"Unavailable":        status.Error(codes.Unavailable, "down"),
		"Aborted 沒有 reason":  status.Error(codes.Aborted, "conflict"),
		"Aborted 別的 reason":  statusWithReason(t, codes.Aborted, "SOMETHING_NEW"),
		"FailedPrecondition": statusWithReason(t, codes.FailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_REVOKED.String()),
	} {
		t.Run(name, func(t *testing.T) {
			rec := &markRecorder{returnErr: markErr}
			err, _ := runVendorInvite(t, nil, rec)
			require.Error(t, err)
			require.Equal(t, "abc123", rec.secret)
		})
	}
}

// InvalidArgument 與 NotFound 不會因重試而改變：直接進死信（mq.ErrPermanent）、記 rejected；暫時性的 code 不是 permanent。
func TestVendorInviteHandler_NonRetryableCodesDeadLetterImmediately(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ensureErr error
		markErr   error
		permanent bool
	}{
		{"Ensure InvalidArgument", status.Error(codes.InvalidArgument, "bad"), nil, true},
		{"Ensure NotFound", status.Error(codes.NotFound, "no prospect"), nil, true},
		{"Mark InvalidArgument", nil, status.Error(codes.InvalidArgument, "bad"), true},
		{"Mark NotFound", nil, status.Error(codes.NotFound, "no invite"), true},
		{"Ensure Unavailable", status.Error(codes.Unavailable, "down"), nil, false},
		{"Mark Unavailable", nil, status.Error(codes.Unavailable, "down"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := counter("rejected")
			err, _ := runVendorInvite(t, tc.ensureErr, &markRecorder{returnErr: tc.markErr})
			require.Error(t, err)
			require.Equal(t, tc.permanent, errors.Is(err, mq.ErrPermanent))
			want := before
			if tc.permanent {
				want++
			}
			require.Equal(t, want, counter("rejected"))
		})
	}
}
