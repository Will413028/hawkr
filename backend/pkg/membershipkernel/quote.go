package membershipkernel

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BillingIntervalUnit is intentionally a string contract so adding WEEK or YEAR later does
// not change the wire shape. MONTH is the only supported cadence in phase one.
type BillingIntervalUnit string

const BillingIntervalMonth BillingIntervalUnit = "MONTH"

var (
	ErrInvalidBillingQuote = errors.New("invalid membership billing quote")
	// Hashes are persisted in lowercase and included in the canonical contract. Keeping one
	// representation avoids accepting a quote in the service layer that the DB would reject.
	quoteHashPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	quoteCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

// BillingQuote is an immutable commercial snapshot issued by a buyer or seller control plane.
// QuoteID and SnapshotHash are identities/audit metadata; the remaining fields form the
// canonical commercial payload.
type BillingQuote struct {
	QuoteID                   string
	Subject                   SubjectRef
	MembershipID              string
	MembershipChangeRequestID string
	OfferID                   string
	OfferVersionNo            string
	UnitAmountMinor           int64
	Currency                  string
	IntervalUnit              BillingIntervalUnit
	IntervalCount             int64
	ProviderAccountRef        string
	FirstPeriodDate           time.Time
	ExpiresAt                 time.Time
	SnapshotHash              string
}

// CanonicalJSON serializes the commercial payload with a fixed field order and numeric form.
// This must stay byte-for-byte aligned with the Java BillingQuoteCanonicalizer.
func (q BillingQuote) CanonicalJSON() ([]byte, error) {
	if err := q.validateCanonicalFields(); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.Grow(512)
	b.WriteByte('{')
	appendJSONStringField(&b, "subject_type", string(q.Subject.SubjectType))
	appendJSONStringField(&b, "subject_id", q.Subject.SubjectID)
	appendJSONStringField(&b, "membership_id", q.MembershipID)
	appendJSONStringField(&b, "membership_change_request_id", q.MembershipChangeRequestID)
	appendJSONStringField(&b, "offer_id", q.OfferID)
	appendJSONStringField(&b, "offer_version_no", q.OfferVersionNo)
	appendJSONNumberField(&b, "unit_amount_minor", strconv.FormatInt(q.UnitAmountMinor, 10))
	appendJSONStringField(&b, "currency", q.Currency)
	appendJSONStringField(&b, "interval_unit", string(q.IntervalUnit))
	appendJSONNumberField(&b, "interval_count", strconv.FormatInt(q.IntervalCount, 10))
	appendJSONStringField(&b, "provider_account_ref", q.ProviderAccountRef)
	appendJSONStringField(&b, "first_period_date", q.FirstPeriodDate.UTC().Format(time.RFC3339Nano))
	appendJSONStringField(&b, "expires_at", q.ExpiresAt.UTC().Format(time.RFC3339Nano))
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// SnapshotHashHex calculates the lowercase SHA-256 hash expected in SnapshotHash.
func (q BillingQuote) SnapshotHashHex() string {
	canonical, err := q.CanonicalJSON()
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

// Validate checks the domain-issued quote at the moment it is sent to payment-service.
func (q BillingQuote) Validate(nowUTC time.Time) error {
	if err := q.validateCanonicalFields(); err != nil {
		return err
	}
	if strings.TrimSpace(q.QuoteID) == "" {
		return invalidQuote("quote_id is required")
	}
	if q.UnitAmountMinor <= 0 {
		return invalidQuote("unit_amount_minor must be positive")
	}
	if q.Currency == "" || !quoteCurrencyPattern.MatchString(q.Currency) {
		return invalidQuote("currency must be an ISO 4217 uppercase code")
	}
	if q.IntervalUnit != BillingIntervalMonth || q.IntervalCount <= 0 {
		return invalidQuote("MONTH and positive interval_count are required")
	}
	if nowUTC.IsZero() || !q.ExpiresAt.After(nowUTC.UTC()) {
		return invalidQuote("expires_at must be in the future")
	}
	if !quoteHashPattern.MatchString(q.SnapshotHash) {
		return invalidQuote("snapshot_hash must be a SHA-256 hex string")
	}
	want := q.SnapshotHashHex()
	if subtle.ConstantTimeCompare([]byte(want), []byte(q.SnapshotHash)) != 1 {
		return invalidQuote("snapshot_hash does not match the canonical quote")
	}
	return nil
}

func (q BillingQuote) validateCanonicalFields() error {
	if err := q.Subject.Validate(); err != nil {
		return invalidQuote("subject: %v", err)
	}
	fields := []struct {
		name  string
		value string
	}{
		{"membership_id", q.MembershipID},
		{"membership_change_request_id", q.MembershipChangeRequestID},
		{"offer_id", q.OfferID},
		{"offer_version_no", q.OfferVersionNo},
		{"currency", q.Currency},
		{"provider_account_ref", q.ProviderAccountRef},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return invalidQuote("%s is required", field.name)
		}
	}
	if q.IntervalUnit == "" {
		return invalidQuote("interval_unit is required")
	}
	if q.FirstPeriodDate.IsZero() || q.ExpiresAt.IsZero() {
		return invalidQuote("first_period_date and expires_at are required")
	}
	return nil
}

func appendJSONStringField(b *strings.Builder, key, value string) {
	if b.Len() > 1 {
		b.WriteByte(',')
	}
	appendJSONString(b, key)
	b.WriteByte(':')
	appendJSONString(b, value)
}

func appendJSONNumberField(b *strings.Builder, key, value string) {
	if b.Len() > 1 {
		b.WriteByte(',')
	}
	appendJSONString(b, key)
	b.WriteByte(':')
	b.WriteString(value)
}

func appendJSONString(b *strings.Builder, value string) {
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

func invalidQuote(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidBillingQuote, fmt.Sprintf(format, args...))
}
