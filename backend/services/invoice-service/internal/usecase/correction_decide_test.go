package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// decideCorrectionRoute 是 §7.3.5 R02／R03／R04 的分流。
//
// 這支之所以值得逐格窮舉：**每一個婉拒理由都是買家會直接讀到的字**（端點 12 的
// rejectReasonCode），挑錯一個不會報錯、也不會有任何測試變紅——只會讓買家看到
// 一句與他處境無關的解釋。
func TestDecideCorrectionRoute(t *testing.T) {
	issued := codemeta.InvoiceDocumentStatusIssued.Value
	pending := codemeta.InvoiceDocumentStatusPending.Value

	// allPass 是「已開立且六項檢查全過」——R04 的基準線。
	// 每個 case 只翻動它的一項，這樣失敗訊息指得出是哪一項。
	allPass := CorrectionFacts{
		DocumentStatus:      issued,
		OperationOpen:       false,
		TrackSufficient:     true,
		CarrierValid:        true,
		WithinVoidDeadline:  true,
		NoExistingAllowance: true,
		ProviderAvailable:   true,
		AlreadyReported:     false,
	}
	with := func(mut func(*CorrectionFacts)) CorrectionFacts {
		f := allPass
		mut(&f)
		return f
	}

	cases := []struct {
		name       string
		facts      CorrectionFacts
		wantRoute  CorrectionRoute
		wantReason string
	}{
		{
			name:      "尚未開立 ＋ 還有 operation 可攔 → R03（不做任何 R04 預檢）",
			facts:     with(func(f *CorrectionFacts) { f.DocumentStatus = pending; f.OperationOpen = true }),
			wantRoute: RouteUpdateBeforeIssue,
		},
		{
			// 🔴 這一格是本表最重要的：R04 的四項預檢問的是「作廢重開會不會失敗」，
			// 而 R03 根本不作廢。拿它們擋 R03 會讓「字軌暫時不足」這種與本案無關的
			// 狀況擋掉一個只是改快照的請求 —— 而買家會收到一句他看不懂的理由。
			name: "尚未開立 ＋ 字軌不足／載具無效／已過期限／有折讓 → 仍走 R03",
			facts: with(func(f *CorrectionFacts) {
				f.DocumentStatus = pending
				f.OperationOpen = true
				f.TrackSufficient = false
				f.CarrierValid = false
				f.WithinVoidDeadline = false
				f.NoExistingAllowance = false
			}),
			wantRoute: RouteUpdateBeforeIssue,
		},
		{
			name:       "尚未開立 ＋ 沒有可攔的 operation → 婉拒（不得對未開立的憑證做作廢）",
			facts:      with(func(f *CorrectionFacts) { f.DocumentStatus = pending; f.OperationOpen = false }),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value,
		},
		{
			name:      "已開立 ＋ 六項全過 → R04 作廢重開",
			facts:     allPass,
			wantRoute: RouteVoidAndReissue,
		},
		{
			// 已申報排最前面：它是唯一不會回頭的（其餘都可能隨時間或補配號改變）。
			name: "已申報優先於其他理由（同時有多項不過時，回最終局的那一個）",
			facts: with(func(f *CorrectionFacts) {
				f.AlreadyReported = true
				f.TrackSufficient = false
				f.CarrierValid = false
			}),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonAlreadyReported.Value,
		},
		{
			name:       "已逾可作廢期限",
			facts:      with(func(f *CorrectionFacts) { f.WithinVoidDeadline = false }),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value,
		},
		{
			name:       "已存在折讓（作廢會使折讓失去依據）",
			facts:      with(func(f *CorrectionFacts) { f.NoExistingAllowance = false }),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonAllowanceExists.Value,
		},
		{
			name:       "字軌餘量不足（先確保可開再作廢）",
			facts:      with(func(f *CorrectionFacts) { f.TrackSufficient = false }),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonTrackInsufficient.Value,
		},
		{
			name:       "更正後的載具無效",
			facts:      with(func(f *CorrectionFacts) { f.CarrierValid = false }),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonCarrierInvalid.Value,
		},
		{
			name:       "服務商暫時不可用（買家稍後可再申請一次）",
			facts:      with(func(f *CorrectionFacts) { f.ProviderAvailable = false }),
			wantRoute:  RouteReject,
			wantReason: codemeta.InvoiceCorrectionRejectReasonProviderUnavailable.Value,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideCorrectionRoute(tc.facts)
			if got.Route != tc.wantRoute {
				t.Fatalf("Route = %v, want %v", got.Route, tc.wantRoute)
			}
			if got.RejectReason != tc.wantReason {
				t.Errorf("RejectReason = %q, want %q（買家會直接讀到這個碼對應的文字）",
					got.RejectReason, tc.wantReason)
			}
		})
	}
}

// 非婉拒的兩條邊不得帶理由碼——帶了會讓 chk_invcorr_reject_reason 的語意鬆掉
// （REJECTED ⇒ 非空，但反向沒有約束，所以只能靠這裡守）。
func TestDecideCorrectionRoute_NonRejectCarriesNoReason(t *testing.T) {
	for _, f := range []CorrectionFacts{
		{DocumentStatus: codemeta.InvoiceDocumentStatusPending.Value, OperationOpen: true},
		{
			DocumentStatus:  codemeta.InvoiceDocumentStatusIssued.Value,
			TrackSufficient: true, CarrierValid: true, WithinVoidDeadline: true,
			NoExistingAllowance: true, ProviderAvailable: true,
		},
	} {
		if got := decideCorrectionRoute(f); got.Route != RouteReject && got.RejectReason != "" {
			t.Errorf("Route=%v 不是婉拒，卻帶了理由碼 %q", got.Route, got.RejectReason)
		}
	}
}

// #3020：作廢／開立進行中再送一次修改，必須是「請稍後再試」而不是婉拒。
//
// 🔴 這支的鑑別力來自**陰性那一半**：只斷言 VOIDING 回 RouteOperationInProgress，
// 修法寫成「非 ISSUED 一律回進行中」也會通過——而那會讓 VOIDED／SUPERSEDED
// 這些真終態的憑證變成「請稍後再試」，買家永遠等一個不會來的結果。
// 所以下表把真終態也一併列進來，它們必須仍然是婉拒。
func TestDecideCorrectionRoute_InProgressIsNotReject(t *testing.T) {
	// 進行中：operation 已離開 READY／RETRY_SCHEDULED（所以 OperationOpen=false），
	// 但憑證的實體流程還沒結束。
	inProgress := []string{
		codemeta.InvoiceDocumentStatusVoiding.Value,
		codemeta.InvoiceDocumentStatusIssuing.Value,
	}
	// 真終態：沒有東西會再動它 ⇒ 仍是婉拒。
	terminal := []string{
		codemeta.InvoiceDocumentStatusVoided.Value,
		codemeta.InvoiceDocumentStatusSuperseded.Value,
		codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value,
		codemeta.InvoiceDocumentStatusNotApplicable.Value,
		codemeta.InvoiceDocumentStatusResolvedExternally.Value,
	}

	for _, s := range inProgress {
		t.Run("進行中/"+s, func(t *testing.T) {
			got := decideCorrectionRoute(CorrectionFacts{DocumentStatus: s, OperationOpen: false})
			if got.Route != RouteOperationInProgress {
				t.Fatalf("Route = %v, want RouteOperationInProgress（呼叫端回 9351）", got.Route)
			}
			if got.RejectReason != "" {
				t.Errorf("RejectReason = %q，進行中不是婉拒，不得帶理由碼", got.RejectReason)
			}
			if got.RejectReason == codemeta.InvoiceCorrectionRejectReasonPastVoidDeadline.Value {
				t.Errorf("回了 PAST_VOID_DEADLINE —— 那句話對買家是終局的，而實情是等幾秒（#3020）")
			}
		})
	}

	for _, s := range terminal {
		t.Run("真終態/"+s, func(t *testing.T) {
			got := decideCorrectionRoute(CorrectionFacts{DocumentStatus: s, OperationOpen: false})
			if got.Route != RouteReject {
				t.Fatalf("Route = %v, want RouteReject（真終態沒有東西會再動它，"+
					"回『請稍後再試』會讓買家永遠等下去）", got.Route)
			}
		})
	}
}
