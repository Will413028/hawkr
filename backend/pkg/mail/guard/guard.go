// Package guard is the recipient seatbelt for outbound mail.
//
// It wraps any mail.Sender and decides, before the provider ever sees the
// message, whether this deployment is allowed to mail this address. The
// failure it exists to prevent is the one every platform hits once: a non-prod
// environment that holds real credentials and a real recipient list sends real
// mail to real people, because one config value said "production" when it
// should not have.
//
// ⛔ It belongs here, in the seam, not in a caller. Order mail and invoice mail
// will travel this path too; a check written into one consumer is a check the
// next consumer forgets. That is also why the policy is chosen at startup and
// cannot be relaxed per message.
package guard

import (
	"context"
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/mail"
)

// Mode is how the guard treats a recipient.
type Mode string

const (
	// ModeUnrestricted sends to whatever the caller asked for. Only production
	// may use it — it is the absence of the seatbelt, stated explicitly so that
	// "no restriction" is a deliberate deployment choice rather than a value
	// somebody forgot to set.
	ModeUnrestricted Mode = "unrestricted"

	// ModeAllowlist sends only to addresses matching Policy.Allowed and fails
	// everything else as permanent.
	ModeAllowlist Mode = "allowlist"

	// ModeRedirect rewrites every recipient to Policy.RedirectTo. Safe by
	// construction: there is no address the message can reach other than the
	// configured one, so a new caller cannot leak through a gap in a list.
	ModeRedirect Mode = "redirect"
)

// Policy is the deployment's recipient rule.
type Policy struct {
	Mode Mode

	// Allowed is consulted only in ModeAllowlist. An entry beginning with "@"
	// matches any address in that domain; anything else must match the whole
	// address. Both comparisons are case-insensitive.
	Allowed []string

	// RedirectTo is the single recipient used in ModeRedirect.
	RedirectTo string
}

// Sender applies a Policy and then delegates.
type Sender struct {
	inner  mail.Sender
	policy Policy
}

// New validates the policy and wraps inner.
//
// ⛔ Fail-closed by design: a policy that does not describe a reachable rule is
// an error here, at startup, not a message silently dropped later. The mode
// itself is required — defaulting an unset mode to unrestricted would rebuild
// exactly the hazard this package removes.
func New(inner mail.Sender, p Policy) (*Sender, error) {
	if inner == nil {
		return nil, fmt.Errorf("mail/guard: inner sender is nil")
	}
	switch p.Mode {
	case ModeUnrestricted:
	case ModeAllowlist:
		if len(p.Allowed) == 0 {
			return nil, fmt.Errorf("mail/guard: mode %q requires a non-empty allowlist", p.Mode)
		}
	case ModeRedirect:
		if strings.TrimSpace(p.RedirectTo) == "" {
			return nil, fmt.Errorf("mail/guard: mode %q requires a redirect address", p.Mode)
		}
	case "":
		return nil, fmt.Errorf("mail/guard: recipient policy mode is required (one of %q, %q, %q)",
			ModeUnrestricted, ModeAllowlist, ModeRedirect)
	default:
		return nil, fmt.Errorf("mail/guard: unknown recipient policy mode %q", p.Mode)
	}
	return &Sender{inner: inner, policy: p}, nil
}

// Send applies the policy and forwards, or refuses.
//
// A refusal is mail.ErrPermanent: no retry can make a blocked recipient
// allowed, and the caller's existing permanent branch already does the right
// thing with it (stop, record, do not mark as delivered). The blocked address
// is masked in the error — the guard runs in environments whose logs are less
// protected than production's.
func (s *Sender) Send(ctx context.Context, m mail.Message) (mail.Receipt, error) {
	switch s.policy.Mode {
	case ModeUnrestricted:
		return s.inner.Send(ctx, m)

	case ModeAllowlist:
		if !allowed(m.To.Email, s.policy.Allowed) {
			return mail.Receipt{}, fmt.Errorf("%w: recipient %s is not on MAIL_ALLOWED_RECIPIENTS",
				mail.ErrPermanent, MaskEmail(m.To.Email))
		}
		return s.inner.Send(ctx, m)

	case ModeRedirect:
		original := m.To.Email
		m.To = mail.Address{Email: s.policy.RedirectTo}
		// The original recipient goes in the subject, not a header: mail.Message
		// has no header field, and losing "who was this for" would make a
		// redirected inbox useless for the thing it exists to check.
		m.Subject = "[→ " + original + "] " + m.Subject
		return s.inner.Send(ctx, m)
	}
	return mail.Receipt{}, fmt.Errorf("%w: unreachable recipient policy %q", mail.ErrPermanent, s.policy.Mode)
}

func allowed(email string, list []string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return false
	}
	for _, entry := range list {
		v := strings.ToLower(strings.TrimSpace(entry))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "@") {
			if strings.HasSuffix(e, v) {
				return true
			}
			continue
		}
		if e == v {
			return true
		}
	}
	return false
}

// MaskEmail keeps the first character of the local part and the whole domain:
// enough to tell "internal or not" apart without writing the address out.
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

// ParseMode maps a config string onto a Mode without deciding a default.
func ParseMode(s string) Mode { return Mode(strings.ToLower(strings.TrimSpace(s))) }

// ParseAllowed splits a comma-separated MAIL_ALLOWED_RECIPIENTS value.
func ParseAllowed(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}
