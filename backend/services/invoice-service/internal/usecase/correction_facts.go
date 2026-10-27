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

// 更正請求的事實蒐集（§7.3.5 R02／R03／R04 分流所需的六項）。
//
// 為什麼獨立成一支而不是塞進 Submit：**R01 與後續分支必須看到同一份事實**。
// 分兩次查會讓「判定時 A、執行時 B」，而兩者之間沒有任何鎖——那正是 §3.10
// 那類「機制的觀測窗比它以為的小」的形狀。
//
// 🔴 六項裡**五項是純函式或本地查詢，只有 TrackSufficient 要打 AMEGO**。
// 那一次外呼是整段唯一的網路往返，也是唯一會逾時的地方 ⇒ 它的失敗必須是
// **fail-closed**（當成字軌不足而婉拒），不能當成「查不到就放行」：
// 放行的後果是先作廢再發現開不出來，而作廢不可逆。

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

	// 🔴 tax_period_code 是 *string：**未開立的 document 可能還沒有期別**。
	// 沒有期別時兩個判定一律 fail-closed（當成已申報、已逾期限）——那會把它推向
	// RouteReject，而不是讓一份沒有期別的憑證走進作廢流程。
	// ⚠️ 這不影響 R03：未開立走 R03 的分支在 decideCorrectionRoute 裡先判，
	// 那條路徑根本不看這兩項（見該函式第 1 條判定順序）。
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

	// 已存在折讓：作廢會使折讓失去依據 ⇒ 只要有任何非終態的 ALLOWANCE 就擋。
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
		DocumentStatus: doc.Status,
		OperationOpen:  in.OpenOperationID != 0,
		// 🔴 三態裡只有 NOT_REPORTED 允許作廢：UNKNOWN 是 fail-closed 的那一格
		// （期已結、申報期限未到），把它當成「還沒申報」會讓已申報的發票被作廢。
		AlreadyReported:     reported != domain.ReportedStatusNotReported,
		WithinVoidDeadline:  withinVoid,
		NoExistingAllowance: noAllowance,
		ProviderAvailable:   in.ProviderStatus == codemeta.InvoiceProviderAccountStatusActive.Value,
		// CarrierValid 由呼叫端在解析買家輸入時就判定（那一層才有原始輸入）。
		CarrierValid: true,
		// TrackSufficient 預設 true；只有真的要作廢重開時才去問 AMEGO（見下）。
		TrackSufficient: true,
	}

	// 未開立 ⇒ 走 R03，不作廢也不重開 ⇒ 不必問字軌。
	if doc.Status != codemeta.InvoiceDocumentStatusIssued.Value {
		return facts, nil
	}

	facts.TrackSufficient = u.hasSpareTrack(ctx, tracks, in.Credential, now)
	return facts, nil
}

// hasSpareTrack 問 AMEGO「重開需要的那個號碼還有沒有」。
//
// 🔴 **fail-closed**：任何錯誤（逾時、斷路器開路、解析失敗）一律回 false ＝ 字軌不足。
// 放行的後果是先作廢再發現開不出來，而作廢不可逆且明令不可用 DB restore 回復
// ⇒ 買家手上一張都沒有。婉拒的後果只是他稍後再送一次
// （REJECTED 是終態且不佔 uq_invcorr_open_per_document 的名額）。
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
