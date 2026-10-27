package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/domain/certholder"
)

func certHolderUC(fn func(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error)) *CertificationUseCase {
	return &CertificationUseCase{certRepo: &mockCertificationRepo{GetCertHolderNamesFn: fn}}
}

func names(real, company string) func(context.Context, int64) (domain.CertHolderNames, bool, error) {
	return func(context.Context, int64) (domain.CertHolderNames, bool, error) {
		return domain.CertHolderNames{RealName: real, CompanyName: company}, true, nil
	}
}

// TestMatchCertificateHolder covers the five outcomes of 19 號設計書 §8.5.1 —
// including the one the table draws a red line under: "no approved KYC" and
// "compared, did not match" both report matched=false, but only the first reports
// has_reference=false.
func TestMatchCertificateHolder(t *testing.T) {
	cases := []struct {
		name        string
		repo        func(context.Context, int64) (domain.CertHolderNames, bool, error)
		holder      string
		wantMatched bool
		wantField   certholder.MatchField
		wantHasRef  bool
	}{
		{
			name:        "matches real_name",
			repo:        names("王小明", ""),
			holder:      "王小明",
			wantMatched: true,
			wantField:   certholder.MatchRealName,
			wantHasRef:  true,
		},
		{
			name:        "matches company_name",
			repo:        names("王小明", "範例生鮮股份有限公司"),
			holder:      "範例生鮮",
			wantMatched: true,
			wantField:   certholder.MatchCompanyName,
			wantHasRef:  true,
		},
		{
			// The certificate that motivated this whole check: a real, valid
			// number whose holder is a different business entirely.
			name:        "both names present, neither matches",
			repo:        names("王小明", "範例生鮮股份有限公司"),
			holder:      "Acme Road LLC",
			wantMatched: false,
			wantField:   certholder.MatchNone,
			wantHasRef:  true,
		},
		{
			name: "no approved KYC row",
			repo: func(context.Context, int64) (domain.CertHolderNames, bool, error) {
				return domain.CertHolderNames{}, false, nil
			},
			holder:      "Acme Road LLC",
			wantMatched: false,
			wantField:   certholder.MatchNone,
			wantHasRef:  false,
		},
		{
			// Row exists but carries nothing identifying — normalization strips
			// "有限公司" down to the empty string, which must never match.
			name:        "row exists but both names normalize to empty",
			repo:        names("  ", "有限公司"),
			holder:      "有限公司",
			wantMatched: false,
			wantField:   certholder.MatchNone,
			wantHasRef:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := certHolderUC(tc.repo).MatchCertificateHolder(context.Background(), 2636840881, tc.holder)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Matched != tc.wantMatched {
				t.Errorf("Matched = %v, want %v", got.Matched, tc.wantMatched)
			}
			if got.MatchedField != tc.wantField {
				t.Errorf("MatchedField = %q, want %q", got.MatchedField, tc.wantField)
			}
			if got.HasReference != tc.wantHasRef {
				t.Errorf("HasReference = %v, want %v", got.HasReference, tc.wantHasRef)
			}
			// The invariant the caller's auto-approval rests on, asserted on every
			// row rather than only the two that exercise it directly.
			if got.Matched && !got.HasReference {
				t.Errorf("matched=true with has_reference=false: approving on no basis")
			}
		})
	}
}

// TestMatchCertificateHolder_RepoErrorPropagates pins that a lookup failure stays
// an error. Flattening it into matched=false would be indistinguishable from a
// checked-and-rejected verdict, and the caller could no longer tell that it is
// failing closed rather than deciding.
func TestMatchCertificateHolder_RepoErrorPropagates(t *testing.T) {
	boom := errors.New("connection refused")
	uc := certHolderUC(func(context.Context, int64) (domain.CertHolderNames, bool, error) {
		return domain.CertHolderNames{}, false, boom
	})

	got, err := uc.MatchCertificateHolder(context.Background(), 2636840881, "王小明")
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if got.Matched {
		t.Errorf("Matched = true on lookup failure; must fail closed")
	}
}

// TestMatchCertificateHolder_PassesStoreIDThrough guards the key axis: the
// comparison basis must be the storefront that submitted the certificate. Reading
// a different seller's KYC would compare against the wrong names entirely.
func TestMatchCertificateHolder_PassesStoreIDThrough(t *testing.T) {
	var seen int64
	uc := certHolderUC(func(_ context.Context, storeID int64) (domain.CertHolderNames, bool, error) {
		seen = storeID
		return domain.CertHolderNames{RealName: "王小明"}, true, nil
	})

	if _, err := uc.MatchCertificateHolder(context.Background(), 2636840881, "王小明"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen != 2636840881 {
		t.Errorf("repository received store_id %d, want 2636840881", seen)
	}
}
