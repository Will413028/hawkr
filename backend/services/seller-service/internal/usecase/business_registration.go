package usecase

import (
	"context"
	"regexp"
)

// BusinessRegistration is the seller-domain view of a company's 經濟部商業司
// (GCIS) registration record.
type BusinessRegistration struct {
	Ban               string `json:"ban"`
	BusinessNm        string `json:"business_nm"`
	BusinessAddress   string `json:"business_address"`
	BusinessSetupDate string `json:"business_setup_date"` // 西元 YYYY-MM-DD
	BusinessType      string `json:"business_type"`
	CapitalAmount     int64  `json:"capital_amount"`
}

// GcisClient looks up a company in the 經濟部商業司 GCIS registry.
//
// Implementations MUST return (nil, nil) when GCIS is reachable but no such
// company exists (definitive not-found), and (nil, err) on transient upstream
// failure (timeout / connection / 5xx) so the caller can fail-soft.
type GcisClient interface {
	Lookup(ctx context.Context, ban string) (*BusinessRegistration, error)
}

// BusinessRegistrationCache is a cache-aside store for GCIS lookups. Company
// registration is slow-changing reference data, so a long TTL is appropriate.
// Implementations fail open (a cache miss / unavailable Redis returns ok=false).
type BusinessRegistrationCache interface {
	Get(ctx context.Context, ban string) (*BusinessRegistration, bool)
	Set(ctx context.Context, ban string, br *BusinessRegistration)
}

var banFormatRe = regexp.MustCompile(`^\d{8}$`)

// BusinessRegistrationUseCase owns 統一編號 lookup for the seller KYC bounded
// context: format gate + cache-aside over the GCIS anti-corruption client.
type BusinessRegistrationUseCase struct {
	gcis  GcisClient
	cache BusinessRegistrationCache
}

func NewBusinessRegistrationUseCase(gcis GcisClient, cache BusinessRegistrationCache) *BusinessRegistrationUseCase {
	return &BusinessRegistrationUseCase{gcis: gcis, cache: cache}
}

// Lookup returns (data, nil) when found, (nil, nil) for definitive not-found,
// and (nil, err) for a bad format (*ValidationError → 400) or a transient
// upstream error (so the caller can fail-soft — GCIS being down must not block
// the seller). Not-found results are intentionally not cached.
func (u *BusinessRegistrationUseCase) Lookup(ctx context.Context, ban string) (*BusinessRegistration, error) {
	if !banFormatRe.MatchString(ban) {
		return nil, &ValidationError{Messages: []string{"統一編號格式不符"}}
	}
	if cached, ok := u.cache.Get(ctx, ban); ok {
		return cached, nil
	}
	br, err := u.gcis.Lookup(ctx, ban)
	if err != nil {
		return nil, err
	}
	if br == nil {
		return nil, nil
	}
	u.cache.Set(ctx, ban, br)
	return br, nil
}
