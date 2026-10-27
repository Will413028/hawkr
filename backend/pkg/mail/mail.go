// Package mail is the platform's outbound email seam. It defines one Sender
// interface, one normalized error taxonomy, and nothing else — provider
// implementations live in sub-packages (mail/resend, mail/logmail).
//
// It carries no business semantics: no templates, no retry, no recipient
// resolution. Those live in the calling service, exactly as pkg/sms keeps OTP
// semantics out of the Mitake client.
//
// ⛔ The package never retries. Callers drive retry through their outbox
// worker (pkg/outbox already owns backoff and terminal handling); a second
// retry layer here would multiply attempts against the provider's rate limit
// and, worse, hide permanent failures behind apparent transience.
package mail

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrTransient and ErrPermanent are the entire outward error vocabulary.
//
// ⛔ Provider errors must be wrapped into one of these, never returned raw.
// A raw provider error makes every caller's retry logic grow Resend's status
// codes into it, so swapping providers would mean rewriting all of them.
//
// The split is not cosmetic. Retrying an ErrPermanent keeps hammering dead
// mailboxes, which degrades the sending domain's reputation — and
// example.com is shared with order and invoice mail, so the cost of
// misclassifying lands on other lines entirely.
var (
	ErrTransient = errors.New("mail: transient send failure")
	ErrPermanent = errors.New("mail: permanent send failure")
)

// Address is one sender or recipient.
type Address struct {
	Email string
	Name  string // optional display name
}

// String renders the address for a mail header. Bare email when there is no
// display name, RFC 5322 name-addr form otherwise.
func (a Address) String() string {
	if a.Name == "" {
		return a.Email
	}
	return fmt.Sprintf("%q <%s>", a.Name, a.Email)
}

// Message is one outbound email. Subject and body arrive fully rendered.
type Message struct {
	From Address
	// To is the recipient. Provider adapters may use only Address.Email:
	// Resend's API documents the "Name <email>" display-name form for `from`
	// but not for `to`, so the resend adapter ignores To.Name. Check the
	// specific adapter's doc before relying on a display name reaching To.
	To      Address
	Subject string

	// HTML and Text: at least one must be set. Providers that can derive the
	// plain-text part from HTML will do so when Text is empty.
	HTML string
	Text string

	// IdempotencyKey lets a provider collapse duplicate submissions of the
	// same logical message.
	//
	// ⚠️ Support is provider-specific. Each adapter's doc comment must state
	// whether it honours this field — a caller that assumes protection it does
	// not have will double-send under redelivery.
	IdempotencyKey string

	// Tags are provider-side labels echoed back on delivery webhooks. Use them
	// to correlate a later webhook to a domain row without storing anything at
	// send time.
	Tags map[string]string
}

// Validate reports whether the message is well formed enough to submit.
//
// Every failure is ErrPermanent: a malformed message is the caller's bug, and
// resubmitting identical bytes cannot fix it.
func (m Message) Validate() error {
	switch {
	case m.From.Email == "":
		return fmt.Errorf("%w: from address is empty", ErrPermanent)
	case m.To.Email == "":
		return fmt.Errorf("%w: to address is empty", ErrPermanent)
	case m.Subject == "":
		return fmt.Errorf("%w: subject is empty", ErrPermanent)
	case m.HTML == "" && m.Text == "":
		return fmt.Errorf("%w: both html and text bodies are empty", ErrPermanent)
	}
	return nil
}

// Receipt is what the provider gave back when it accepted the message.
//
// ⛔ A Receipt means accepted for delivery, NOT delivered. Nothing in this
// package can observe delivery; that evidence arrives later, out of band.
type Receipt struct {
	ProviderMessageID string
	AcceptedAt        time.Time
}

// Sender submits one message to a provider.
type Sender interface {
	Send(ctx context.Context, m Message) (Receipt, error)
}
