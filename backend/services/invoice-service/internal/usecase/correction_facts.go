package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TrackLister 是事實蒐集需要的 AMEGO 出向面（只要一支）。
type TrackLister interface {
	ListTracks(ctx context.Context, c amego.Credential, year, period int) ([]amego.Track, error)
}

// CorrectionFactsInput 是蒐集事實所需的既有事實。
type CorrectionFactsInput struct {
	Document domain.Document
	// OpenOperationID 是這張憑證上待送出的那一筆（0 ＝ 沒有）。
	OpenOperationID int64
	// ProviderStatus 取自 invoice_provider_accounts.status。
	ProviderStatus string
	// Credential 供 ListTracks 使用；只在 document 已 ISSUED 時才會被用到。
	Credential amego.Credential
}

// GatherCorrectionFacts 把六項事實一次查齊。
//
// 🔴 **只有 document 已 ISSUED 時才打 AMEGO**：未開立走 R03，那條路徑不作廢也不重開，
// 字軌餘量與它無關。無條件打會讓每一筆「只是改個統編」的請求都付一次外呼成本，
// 而且在 AMEGO 掛掉時連 R03 都做不了——那是把不相干的依賴綁進來。
func (u *Correction) GatherCorrectionFacts(
	ctx context.Context, tracks TrackLister, in CorrectionFactsInput,
) (CorrectionFacts, error) {
	now := u.now()
	doc := in.Document

	reported := domain.ReportedStatusUnknown
	withinVoid := false
	if doc.TaxPeriodCode != nil {
		var err error
		reported, err = domain.ReportedStatus(*doc.TaxPeriodCode, now)
		if err != nil {
			return CorrectionFacts{}, fmt.Errorf("invoice: reported status: %w", err)
		}
		withinVoid, err = domain.WithinVoidWindow(*doc.TaxPeriodCode, now)
		if err != nil {
			return CorrectionFacts{}, fmt.Errorf("invoice: void window: %w", err)
		}
	}

	adjustments, err := u.adjustments.ListByDocument(ctx, doc.ID)
	if err != nil {
		return CorrectionFacts{}, fmt.Errorf("invoice: list adjustments: %w", err)
	}
	noAllowance := true
	for _, a := range adjustments {
		if a.AdjustmentType == codemeta.InvoiceAdjustmentTypeAllowance.Value {
			noAllowance = false
			break
		}
	}

	facts := CorrectionFacts{
		DocumentStatus:      doc.Status,
		OperationOpen:       in.OpenOperationID != 0,
		AlreadyReported:     reported != domain.ReportedStatusNotReported,
		WithinVoidDeadline:  withinVoid,
		NoExistingAllowance: noAllowance,
		ProviderAvailable:   in.ProviderStatus == codemeta.InvoiceProviderAccountStatusActive.Value,
		CarrierValid:        true,
		TrackSufficient:     true,
	}

	if doc.Status != codemeta.InvoiceDocumentStatusIssued.Value {
		return facts, nil
	}

	facts.TrackSufficient = u.hasSpareTrack(ctx, tracks, in.Credential, now)
	return facts, nil
}

func (u *Correction) hasSpareTrack(
	ctx context.Context, tracks TrackLister, cred amego.Credential, now time.Time,
) bool {
	year, period := domain.AmegoPeriod(now)
	list, err := tracks.ListTracks(ctx, cred, year, period)
	if err != nil {
		u.log.Warn("invoice.correction.track_probe_failed",
			zap.Error(err), zap.String("reason", "fail-closed：當成字軌不足而婉拒"))
		return false
	}
	for _, t := range list {
		if remainingNumbers(t) > 0 && t.Status != 3 && t.Status != 9 {
			return true
		}
	}
	return false
}
