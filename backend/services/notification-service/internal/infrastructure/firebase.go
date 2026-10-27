package infrastructure

import (
	"context"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"

	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// FirebaseNotifier sends push notifications via Firebase Cloud Messaging.
type FirebaseNotifier struct {
	client *messaging.Client
	logger *zap.Logger
}

// ErrPushDisabled is returned by every send when no Firebase credentials are configured.
var ErrPushDisabled = domain.ErrPushDisabled

// disabledNotifier is the PushNotifier used when push is not configured. It never
// contacts Firebase and reports every send as ErrPushDisabled, which the use case
// records as skipped with the reason instead of claiming delivery.
type disabledNotifier struct{}

func (disabledNotifier) Send(context.Context, []string, string, string, string, map[string]string) (map[string]error, error) {
	return nil, ErrPushDisabled
}

func (disabledNotifier) SendDataOnly(context.Context, []string, map[string]string) (map[string]error, error) {
	return nil, ErrPushDisabled
}

func (disabledNotifier) SendToTopic(context.Context, string, string, string, string, map[string]string) (string, error) {
	return "", ErrPushDisabled
}

// NewFirebaseNotifier creates a FirebaseNotifier from config. Empty credentials
// disable push: no credential lookup happens and the service starts normally.
func NewFirebaseNotifier(cfg *config.Config, logger *zap.Logger) (domain.PushNotifier, error) {
	if strings.TrimSpace(cfg.Firebase.CredentialsJSON) == "" {
		logger.Warn("FIREBASE_CREDENTIALS_JSON is empty, push sending is disabled")
		return disabledNotifier{}, nil
	}

	ctx := context.Background()
	opt := option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(cfg.Firebase.CredentialsJSON))
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, err
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info("Firebase Cloud Messaging client initialized")
	return &FirebaseNotifier{client: client, logger: logger}, nil
}

// Send sends a push notification to the given FCM tokens.
// Returns a map of token → error (nil means success). Tokens not in the map succeeded.
// imageURL sets the banner image on FCM Notification and APNS when non-empty.
func (n *FirebaseNotifier) Send(ctx context.Context, tokens []string, title, body, imageURL string, data map[string]string) (map[string]error, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	notif := &messaging.Notification{Title: title, Body: body}
	apns := &messaging.APNSConfig{
		Payload: &messaging.APNSPayload{Aps: &messaging.Aps{Sound: "default"}},
	}
	// Only attach imageURL when caller provided one
	if imageURL != "" {
		notif.ImageURL = imageURL
		apns.FCMOptions = &messaging.APNSFCMOptions{ImageURL: imageURL}
	}

	msg := &messaging.MulticastMessage{
		Tokens:       tokens,
		Notification: notif,
		Data:         data,
		APNS:         apns,
	}

	resp, err := n.client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, err
	}

	failures := make(map[string]error)
	for i, result := range resp.Responses {
		if !result.Success {
			failures[tokens[i]] = result.Error
		}
	}

	n.logger.Info("Push notification sent",
		zap.Int("total", len(tokens)),
		zap.Int("success", resp.SuccessCount),
		zap.Int("failure", resp.FailureCount),
	)

	return failures, nil
}

// SendToTopic broadcasts a push to all devices subscribed to the given FCM topic.
// Returns the FCM message ID on success.
func (n *FirebaseNotifier) SendToTopic(ctx context.Context, topic, title, body, imageURL string, data map[string]string) (string, error) {
	notif := &messaging.Notification{Title: title, Body: body}
	apns := &messaging.APNSConfig{
		Payload: &messaging.APNSPayload{Aps: &messaging.Aps{Sound: "default"}},
	}
	// Only attach imageURL when caller provided one
	if imageURL != "" {
		notif.ImageURL = imageURL
		apns.FCMOptions = &messaging.APNSFCMOptions{ImageURL: imageURL}
	}

	msg := &messaging.Message{
		Topic:        topic,
		Notification: notif,
		Data:         data,
		APNS:         apns,
	}
	id, err := n.client.Send(ctx, msg)
	if err != nil {
		n.logger.Error("FCM topic broadcast failed", zap.String("topic", topic), zap.Error(err))
		return "", err
	}
	n.logger.Info("FCM topic broadcast sent", zap.String("topic", topic), zap.String("message_id", id))
	return id, nil
}

// SendDataOnly sends a data-only FCM message (no notification payload).
// This allows the client app to fully control notification display.
func (n *FirebaseNotifier) SendDataOnly(ctx context.Context, tokens []string, data map[string]string) (map[string]error, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Data:   data,
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
				},
			},
		},
	}

	resp, err := n.client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, err
	}

	failures := make(map[string]error)
	for i, result := range resp.Responses {
		if !result.Success {
			failures[tokens[i]] = result.Error
		}
	}

	n.logger.Info("Data-only push sent",
		zap.Int("total", len(tokens)),
		zap.Int("success", resp.SuccessCount),
		zap.Int("failure", resp.FailureCount),
	)

	return failures, nil
}

// ShouldDeactivateToken reports whether a per-token FCM error means the token is
// permanently undeliverable for us, so it should be deactivated.
//
// It reads the FCM messagingErrorCode via the SDK's typed helpers — NOT a string
// match on the human-readable message (the old IsTokenInvalid did, which missed
// SENDER_ID_MISMATCH / APNs-auth codes whose message text contains none of the
// matched substrings, so stale tokens were never deactivated → perpetual partial).
//
// Deactivate ONLY on unambiguous, per-token-permanent codes:
//   - Unregistered: app uninstalled / token expired → gone.
//   - SenderIDMismatch: token was issued by a different Firebase sender/project,
//     so it can never receive from us.
//
// Deliberately NOT deactivated:
//   - ThirdPartyAuthError (APNs cert/key wrong): project-level config error — would
//     mass-deactivate EVERY iOS token on one bad cert window. Alert ops instead.
//   - InvalidArgument: often a bad message payload that fails ALL tokens at once;
//     deactivating would wipe healthy tokens. Handle via payload validation / alert.
//   - Transient (Unavailable/Internal/QuotaExceeded): retry, token is fine.
func ShouldDeactivateToken(err error) bool {
	if err == nil {
		return false
	}
	return messaging.IsUnregistered(err) || messaging.IsSenderIDMismatch(err)
}

// NewTokenDeactivationPolicy provides the FCM token-deactivation policy for DI.
func NewTokenDeactivationPolicy() domain.TokenDeactivationPolicy {
	return ShouldDeactivateToken
}
