package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/domain/certholder"
)

func certHolderServer(fn func(ctx context.Context, storeID int64) (domain.CertHolderNames, bool, error)) *SellerServer {
	return newSellerServerWithCertRepo(&testSellerRepo{}, nil, &testCertRepo{certHolderFn: fn})
}

func certHolderRow(real, company string) func(context.Context, int64) (domain.CertHolderNames, bool, error) {
	return func(context.Context, int64) (domain.CertHolderNames, bool, error) {
		return domain.CertHolderNames{RealName: real, CompanyName: company}, true, nil
	}
}

// TestGRPC_MatchCertificateHolder walks the five rows of the 19 號設計書 §8.5.1
// decision table end to end, over the wire types the caller actually reads.
func TestGRPC_MatchCertificateHolder(t *testing.T) {
	cases := []struct {
		name        string
		repo        func(context.Context, int64) (domain.CertHolderNames, bool, error)
		holder      string
		wantMatched bool
		wantField   string
		wantHasRef  bool
	}{
		{
			name:        "matches real_name",
			repo:        certHolderRow("王小明", ""),
			holder:      "王小明",
			wantMatched: true,
			wantField:   "REAL_NAME",
			wantHasRef:  true,
		},
		{
			name:        "matches company_name",
			repo:        certHolderRow("王小明", "範例生鮮股份有限公司"),
			holder:      "範例生鮮有限公司",
			wantMatched: true,
			wantField:   "COMPANY_NAME",
			wantHasRef:  true,
		},
		{
			// 1-009-300022 / "Acme Road LLC" in miniature: a real certificate,
			// somebody else's holder.
			name:        "both names present, neither matches",
			repo:        certHolderRow("王小明", "範例生鮮股份有限公司"),
			holder:      "Acme Road LLC",
			wantMatched: false,
			wantField:   "NONE",
			wantHasRef:  true,
		},
		{
			name: "seller has no approved KYC",
			repo: func(context.Context, int64) (domain.CertHolderNames, bool, error) {
				return domain.CertHolderNames{}, false, nil
			},
			holder:      "Acme Road LLC",
			wantMatched: false,
			wantField:   "NONE",
			wantHasRef:  false,
		},
		{
			name:        "row exists but both names normalize to empty",
			repo:        certHolderRow(" ", "有限公司"),
			holder:      "有限公司",
			wantMatched: false,
			wantField:   "NONE",
			wantHasRef:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := certHolderServer(tc.repo)
			resp, err := srv.MatchCertificateHolder(context.Background(), &pb.MatchCertificateHolderRequest{
				StoreId:    2636840881,
				HolderName: tc.holder,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Matched != tc.wantMatched {
				t.Errorf("matched = %v, want %v", resp.Matched, tc.wantMatched)
			}
			if resp.MatchedField != tc.wantField {
				t.Errorf("matched_field = %q, want %q", resp.MatchedField, tc.wantField)
			}
			if resp.HasReference != tc.wantHasRef {
				t.Errorf("has_reference = %v, want %v", resp.HasReference, tc.wantHasRef)
			}
			if resp.Matched && !resp.HasReference {
				t.Errorf("matched=true with has_reference=false: approving on no basis")
			}
		})
	}
}

// TestGRPC_MatchCertificateHolder_NoPIIOnTheWire is the reason this RPC exists in
// seller-service at all (19 號 §8.5.1): the response may carry a verdict and a
// field name, never a name value. Asserted on a match, where the tempting
// "return what matched" regression would land.
func TestGRPC_MatchCertificateHolder_NoPIIOnTheWire(t *testing.T) {
	const realName = "王小明"
	const companyName = "範例生鮮股份有限公司"

	srv := certHolderServer(certHolderRow(realName, companyName))
	resp, err := srv.MatchCertificateHolder(context.Background(), &pb.MatchCertificateHolderRequest{
		StoreId:    2636840881,
		HolderName: realName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Matched {
		t.Fatalf("expected a match on the fixture")
	}
	// String() renders every populated field, so this catches a leak through any
	// field including ones added later.
	wire := resp.String()
	for _, secret := range []string{realName, companyName} {
		if strings.Contains(wire, secret) {
			t.Errorf("response leaks a KYC name: %q appears in %q", secret, wire)
		}
	}
}

// TestGRPC_MatchCertificateHolder_LookupFailureIsAnError pins fail-closed
// behaviour: the caller must see an error, not a verdict, when the names could not
// be read — the two lead to the same manual review but only one is honest about why.
func TestGRPC_MatchCertificateHolder_LookupFailureIsAnError(t *testing.T) {
	boom := errors.New("connection refused")
	srv := certHolderServer(func(context.Context, int64) (domain.CertHolderNames, bool, error) {
		return domain.CertHolderNames{}, false, boom
	})

	resp, err := srv.MatchCertificateHolder(context.Background(), &pb.MatchCertificateHolderRequest{
		StoreId:    2636840881,
		HolderName: "王小明",
	})
	if err == nil {
		t.Fatalf("expected an error, got response %+v", resp)
	}
	if resp != nil && resp.Matched {
		t.Errorf("matched=true alongside a lookup failure")
	}
}

// TestCertHolderResponse_ClampsInconsistentResult exercises the wire-mapping
// guards directly, with inputs the producing path is not supposed to emit. A guard
// that only ever sees valid input is a guard nobody can prove still works — these
// two rows are the ones that go red if either clamp is removed.
func TestCertHolderResponse_ClampsInconsistentResult(t *testing.T) {
	t.Run("matched cannot survive has_reference=false", func(t *testing.T) {
		resp := certHolderResponse(true, certholder.MatchRealName, false)
		if resp.Matched {
			t.Errorf("matched = true with has_reference = false; must clamp to false")
		}
		if resp.MatchedField != "NONE" {
			t.Errorf("matched_field = %q, want NONE", resp.MatchedField)
		}
		if resp.HasReference {
			t.Errorf("has_reference = true, want false (the input said otherwise)")
		}
	})

	t.Run("field cannot survive a negative verdict", func(t *testing.T) {
		resp := certHolderResponse(false, certholder.MatchCompanyName, true)
		if resp.Matched {
			t.Errorf("matched = true, want false")
		}
		if resp.MatchedField != "NONE" {
			t.Errorf("matched_field = %q on a non-match; audit would read it as a hit", resp.MatchedField)
		}
		if !resp.HasReference {
			t.Errorf("has_reference = false, want true")
		}
	})
}
