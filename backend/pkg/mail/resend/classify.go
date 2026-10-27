package resend

import (
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/mail"
)

// transientNames are the Resend error names worth retrying. Everything else
// resolves by status class (see classify).
//
// ⛔ 409 is the only status that carries BOTH kinds, so the name cannot be ignored:
// - concurrent_idempotent_requests (409) is a race that resolves on retry
// - invalid_idempotent_request (409) is the same key with different bytes
//
// All 403 codes are permanent: each requires human action (upgrade plan, verify domain,
// new API key, new scopes, support contact), so no retry clears it.
//
// Reference: Resend API error list, checked 2026-09-11.
var transientNames = map[string]bool{
	"rate_limit_exceeded":            true,
	"daily_quota_exceeded":           true,
	"monthly_quota_exceeded":         true,
	"application_error":              true,
	"service_unavailable":            true,
	"concurrent_idempotent_requests": true,
	"resource_locked":                true,
}

// classify maps one Resend error response onto the pkg/mail taxonomy.
//
// The name refines the decision but never replaces the status: Resend's error
// body shape is not part of its documented contract, so an unparseable or
// empty name must still produce a correct answer. Hence the fallback is by
// status class — 5xx retryable, everything else not — plus two statuses that
// are retryable regardless of name (see below).
func classify(status int, name, message string) error {
	kind := mail.ErrPermanent
	switch {
	case status == 429:
		// Resend-documented rate limit. Must resolve to transient even with an
		// empty/unparseable name (e.g. a CDN/WAF/proxy returning HTML instead of
		// Resend's JSON) — otherwise a rate limit becomes a permanent failure,
		// the consumer acks it, and the message is silently lost.
		kind = mail.ErrTransient
	case status == 408:
		// Resend itself does not return 408, but a proxy between us and Resend
		// can, and HTTP's own semantics make retry the defined remedy for it.
		kind = mail.ErrTransient
	case transientNames[name]:
		kind = mail.ErrTransient
	case status >= 500:
		kind = mail.ErrTransient
	}
	if status >= 200 && status < 300 {
		// Defensive: a 2xx reaching the error path means the caller mis-routed
		// it. Report loudly rather than let a "success" masquerade as one kind
		// of failure or the other.
		return fmt.Errorf("%w: classify called with success status %d", mail.ErrPermanent, status)
	}
	return fmt.Errorf("%w: resend status=%d name=%q message=%q", kind, status, name, message)
}
