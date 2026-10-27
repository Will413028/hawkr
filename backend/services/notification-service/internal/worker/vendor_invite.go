package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mq"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var vendorInviteEmailTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "vendor_portal_invite_email_total",
		Help: "Count of vendor portal invite email send outcomes by result.",
	},
	[]string{"result"},
)

// Ensure 與 Mark 結果的 metric label（result）。accepted、permanent、transient 是寄信結果。
const (
	resultNoEmail             = "no_email"
	resultInviteRevoked       = "invite_revoked"
	resultProspectNotActive   = "prospect_not_active"
	resultPreconditionUnknown = "precondition_unknown"
	resultSecretSuperseded    = "secret_superseded"
	resultRejected            = "rejected"
)

// rejected 回報 seller 以 InvalidArgument 或 NotFound 拒絕這個請求：重試不會改變結果（AIP-194 只重試暫時性的 code），
// 記錄後以 mq.Permanent 直接進死信。其他 code 回傳 false，由呼叫端照舊回傳錯誤重試。
func rejected(d vendorInviteDeps, op string, p externalInvitePayload, err error) (permanent error, ok bool) {
	code := status.Code(err)
	if code != codes.InvalidArgument && code != codes.NotFound {
		return nil, false
	}
	d.Logger.Error("[vendor-invite] "+op+" rejected, dead-lettering",
		zap.String("prospect_id", p.ProspectID), zap.String("invite_id", p.InviteID),
		zap.String("code", code.String()), zap.Error(err))
	vendorInviteEmailTotal.WithLabelValues(resultRejected).Inc()
	return mq.Permanent("seller_"+op+"_"+code.String(), err), true
}

// ensureRefusals 是 Ensure 的 FailedPrecondition 依 ErrorInfo.reason 分出的結果；reason 字串是 seller proto 的列舉名稱。
var ensureRefusals = map[string]struct{ result, msg string }{
	pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_REVOKED.String():       {resultInviteRevoked, "invite revoked, not sending"},
	pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_PROSPECT_NOT_ACTIVE.String():  {resultProspectNotActive, "prospect not active, not sending"},
	pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_NO_DELIVERABLE_EMAIL.String(): {resultNoEmail, "prospect has no deliverable email, not sending"},
}

var supersededReason = pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_SECRET_SUPERSEDED.String()

// errorReason 回傳 gRPC 錯誤的 ErrorInfo.reason；沒有 ErrorInfo 時是空字串。
func errorReason(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

const (
	vendorInviteSubject = "您有一筆新的採購詢價"
	vendorInviteTextFmt = "您好，Hawkr 平台上有買家想向您採購。\n\n請點擊以下連結查看詳情並回覆：\n%s\n\n此連結將於 %s 到期。"
	vendorInviteHTMLFmt = "<p>您好，Hawkr 平台上有買家想向您採購。</p><p>請點擊以下連結查看詳情並回覆：<br><a href=\"%[1]s\">%[1]s</a></p><p>此連結將於 %[2]s 到期。</p>"
)

type externalInvitePayload struct {
	InviteID      string `json:"inviteId"`
	PublicationID string `json:"publicationId"`
	ProspectID    string `json:"prospectId"`
	ExpiresAt     string `json:"expiresAt"`
}

type inviteClient interface {
	EnsureInvite(ctx context.Context, prospectID, rfqRecipientID, expiresAt string) (inviteID, secret, invitedEmail string, err error)
}

type vendorInviteDeps struct {
	Invites  inviteClient
	Mail     mail.Sender
	From     mail.Address
	BaseURL  string
	MarkSent func(ctx context.Context, inviteID, secret string) error
	Logger   *zap.Logger
}

func buildVendorInviteMessage(from mail.Address, to, baseURL, secret string, p externalInvitePayload) mail.Message {
	link := baseURL + "/vendor/rfq#invite=" + secret
	return mail.Message{
		From:    from,
		To:      mail.Address{Email: to},
		Subject: vendorInviteSubject,
		Text:    fmt.Sprintf(vendorInviteTextFmt, link, p.ExpiresAt),
		HTML:    fmt.Sprintf(vendorInviteHTMLFmt, link, p.ExpiresAt),
		Tags:    map[string]string{"rfq_invite_id": p.InviteID},
	}
}

func newVendorInviteHandler(d vendorInviteDeps) mq.HandlerFunc {
	return func(ctx context.Context, _ string, body []byte) error {
		var p externalInvitePayload
		if err := json.Unmarshal(body, &p); err != nil {
			d.Logger.Error("[vendor-invite] bad payload", zap.Error(err), zap.ByteString("body", body))
			return nil
		}

		inviteID, secret, invitedEmail, err := d.Invites.EnsureInvite(ctx, p.ProspectID, p.InviteID, p.ExpiresAt)
		if status.Code(err) == codes.AlreadyExists {
			return nil
		}
		if status.Code(err) == codes.FailedPrecondition {
			reason := errorReason(err)
			if refusal, ok := ensureRefusals[reason]; ok {
				d.Logger.Error("[vendor-invite] "+refusal.msg,
					zap.String("prospect_id", p.ProspectID), zap.String("invite_id", p.InviteID), zap.String("reason", reason))
				vendorInviteEmailTotal.WithLabelValues(refusal.result).Inc()
				return nil
			}
			d.Logger.Error("[vendor-invite] ensure refused with an unknown precondition, not sending",
				zap.String("prospect_id", p.ProspectID), zap.String("invite_id", p.InviteID),
				zap.String("code", codes.FailedPrecondition.String()), zap.String("reason", reason), zap.Error(err))
			vendorInviteEmailTotal.WithLabelValues(resultPreconditionUnknown).Inc()
			return mq.Permanent("seller_ensure_precondition_unknown", err)
		}
		if perm, ok := rejected(d, "ensure", p, err); ok {
			return perm
		}
		if err != nil {
			return fmt.Errorf("vendor-invite: ensure invite: %w", err)
		}

		msg := buildVendorInviteMessage(d.From, invitedEmail, d.BaseURL, secret, p)
		if _, err := d.Mail.Send(ctx, msg); err != nil {
			switch {
			case errors.Is(err, mail.ErrPermanent):
				d.Logger.Error("[vendor-invite] permanent send failure",
					zap.String("invite_id", p.InviteID), zap.Error(err))
				vendorInviteEmailTotal.WithLabelValues("permanent").Inc()
				return nil
			default:
				vendorInviteEmailTotal.WithLabelValues("transient").Inc()
				return err
			}
		}
		vendorInviteEmailTotal.WithLabelValues("accepted").Inc()

		if err := d.MarkSent(ctx, inviteID, secret); err != nil {
			if status.Code(err) == codes.Aborted && errorReason(err) == supersededReason {
				d.Logger.Warn("[vendor-invite] secret superseded by a newer ensure, the newer invite sends its own link",
					zap.String("prospect_id", p.ProspectID), zap.String("invite_id", p.InviteID))
				vendorInviteEmailTotal.WithLabelValues(resultSecretSuperseded).Inc()
				return nil
			}
			if perm, ok := rejected(d, "mark", p, err); ok {
				return perm
			}
			return err
		}
		return nil
	}
}

func withVendorInvite(templateHandler, vendorHandler mq.HandlerFunc) mq.HandlerFunc {
	return func(ctx context.Context, routingKey string, body []byte) error {
		if routingKey == mq.RoutingKeyProcurementRfqPublicationInviteExternalCreated {
			return vendorHandler(ctx, routingKey, body)
		}
		return templateHandler(ctx, routingKey, body)
	}
}
