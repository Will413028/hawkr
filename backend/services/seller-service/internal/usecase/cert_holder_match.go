package usecase

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/seller-service/internal/domain/certholder"
	"go.uber.org/zap"
)

// CertHolderMatchResult answers "does this certificate belong to this seller?"
// for 業者名稱歸屬比對 (19 號設計書 §8.5.1).
//
// Matched and HasReference are NOT redundant and must not be collapsed:
//
//	Matched=false, HasReference=true   compared against a verified name, no match
//	Matched=false, HasReference=false  nothing to compare against at all
//
// Both send the certificate to manual review, but only the first is evidence
// about the certificate; the second is evidence about the seller's KYC state, and
// an audit trail that cannot tell them apart cannot answer "why did this go
// manual?" months later.
type CertHolderMatchResult struct {
	MatchedField certholder.MatchField
	Matched      bool
	HasReference bool
}

// MatchCertificateHolder reports whether holderName — the 業者名稱 a government
// certificate lookup returned — belongs to the seller owning storeID.
//
// 🔴 The seller's names never leave this call: only the verdict and the name of
// the field that matched are returned, and only those are logged. real_name is
// PII (18 號設計書) and holderName is third-party data about a possibly unrelated
// business; neither belongs in a log line or a trace attribute.
//
// A repository failure is returned as an error rather than being flattened into a
// negative verdict. The caller must fail closed on it (轉人工審核), and it can only
// do that if "could not check" is distinguishable from "checked, does not match".
func (uc *CertificationUseCase) MatchCertificateHolder(ctx context.Context, storeID int64, holderName string) (CertHolderMatchResult, error) {
	names, found, err := uc.certRepo.GetCertHolderNames(ctx, storeID)
	if err != nil {
		return CertHolderMatchResult{}, err
	}
	if !found {
		// No approved KYC on the current stint ⇒ no verified name exists, so there
		// is nothing this certificate could have been checked against.
		logger.Info(ctx, "cert holder match: no approved KYC for store",
			zap.Int64("store_id", storeID),
			zap.String("matched_field", string(certholder.MatchNone)))
		return CertHolderMatchResult{MatchedField: certholder.MatchNone}, nil
	}

	matched, field, hasReference := certholder.Match(holderName, names.RealName, names.CompanyName)
	logger.Info(ctx, "cert holder match",
		zap.Int64("store_id", storeID),
		zap.Bool("matched", matched),
		zap.String("matched_field", string(field)),
		zap.Bool("has_reference", hasReference))
	return CertHolderMatchResult{MatchedField: field, Matched: matched, HasReference: hasReference}, nil
}
