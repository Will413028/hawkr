// Package logmail writes the whole message to the structured log instead of
// sending it. It is the default provider in dev and in tests that exercise the
// full call path.
//
// ⚠️ In dev there is no inbox, so this log IS the inbox — it must carry enough
// to click the link by hand. That is why the body goes in verbatim rather than
// being summarised or truncated.
//
// ⛔ Never make this the default in production: it silently accepts everything,
// so a misconfigured prod would report success while nobody receives anything.
// The factory (mail/mailfactory) is where that choice is made explicit.
package logmail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"go.uber.org/zap"
)

// Sender logs messages. Safe for concurrent use (zap.Logger is).
type Sender struct {
	logger *zap.Logger
	now    func() time.Time
}

// New builds a Sender. logger must not be nil.
func New(logger *zap.Logger) *Sender {
	return &Sender{logger: logger, now: time.Now}
}

// Send validates the message, logs it, and returns a receipt whose id is
// derived from the message content.
//
// The id is deterministic rather than random so a dev e2e run can recompute it
// and match a log line to the message it was asserting about. It is derived
// from the whole message — addressing AND body — not just addressing: the
// intended consumer (a vendor RFQ invite) is expected to leave IdempotencyKey
// unset and reuse a fixed subject template, so hashing only
// "to|subject|idempotencyKey" would collapse every invite to one vendor onto
// the same id, including the second copy sent after a transient failure
// rotates the invite secret. That would defeat the only reason this id is
// deterministic.
//
// IdempotencyKey: logmail does NOT deduplicate on it — every call logs a line
// and returns a receipt. The key is only mixed into the id hash below.
func (s *Sender) Send(_ context.Context, m mail.Message) (mail.Receipt, error) {
	if err := m.Validate(); err != nil {
		return mail.Receipt{}, err
	}

	sum := sha256.Sum256([]byte(m.From.String() + "|" + m.To.Email + "|" + m.Subject + "|" + m.HTML + "|" + m.Text + "|" + m.IdempotencyKey))
	id := "logmail:" + hex.EncodeToString(sum[:])[:16]

	s.logger.Info("[logmail] message not sent — MAIL_PROVIDER=log",
		zap.String("provider_message_id", id),
		zap.String("from", m.From.String()),
		zap.String("to", m.To.Email),
		zap.String("subject", m.Subject),
		zap.String("text", m.Text),
		zap.String("html", m.HTML),
		zap.String("idempotency_key", m.IdempotencyKey),
		zap.Any("tags", m.Tags),
	)

	return mail.Receipt{ProviderMessageID: id, AcceptedAt: s.now()}, nil
}
