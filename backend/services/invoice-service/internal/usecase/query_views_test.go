package usecase

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// TestAdminJobTabs_PartitionsAllOperationStatuses 是 §6.1.14「兩條機械驗算」的第一條。
//
// 六個集合的**聯集 ＝ 20 值、兩兩交集 ＝ 空**。設計書逐字說明了為什麼這條必須被寫成
// 測試：少一個值 ⇒ 那個狀態的 job 在六個分頁裡都看不到（不是報錯，是消失）；
// 多算一個 ⇒ 同一筆出現在兩頁、ADM 處理兩次。
// **兩種失敗都不會有任何其他測試轉紅。**
func TestAdminJobTabs_PartitionsAllOperationStatuses(t *testing.T) {
	seen := map[string]string{} // status -> 第一個宣告它的 tab
	for tab, statuses := range adminJobTabs {
		for _, st := range statuses {
			if prev, dup := seen[st]; dup {
				t.Fatalf("狀態 %q 同時屬於 %q 與 %q —— 同一筆 job 會出現在兩個分頁，ADM 會處理兩次", st, prev, tab)
			}
			seen[st] = tab
		}
	}

	all := codemeta.AllInvoiceOperationStatuss
	if len(seen) != len(all) {
		t.Fatalf("分割計數不符：六個分頁共 %d 個狀態，CodeMeta 有 %d 個", len(seen), len(all))
	}
	for _, meta := range all {
		if _, ok := seen[meta.Value]; !ok {
			t.Fatalf("狀態 %q 不屬於任何分頁 —— 它的 job 在 A05 的六個分頁裡都看不到（不會報錯，是消失）", meta.Value)
		}
	}

	// 分割計數 4＋2＋3＋7＋2＋2＝20（§6.1.14 逐字）。逐 tab 釘住筆數，
	// 讓「兩個 tab 各搬走一個值」這種聯集仍成立的變異也轉紅。
	want := map[string]int{
		"PENDING": 4, "RETRYING": 2, "NEEDS_ATTENTION": 3,
		"NEEDS_MANUAL": 7, "COMPLETED": 2, "CANCELLED": 2,
	}
	for tab, n := range want {
		if got := len(adminJobTabs[tab]); got != n {
			t.Fatalf("分頁 %q 應有 %d 個狀態，實得 %d", tab, n, got)
		}
	}
}

// TestSellerComplaintViews_PartitionAllComplaintStatuses 釘住 §6.1.11 的
// 「三個視圖互斥且窮盡」（7 個 DB 值各落在且只落在一個視圖）。
//
// ⚠️ 它與端點 58 的 ADM tab **規則相反**（那裡的 OVERDUE 刻意是橫切視圖）
// ⇒ 下面那支測試斷言的正是「不窮盡也不互斥」，兩支必須並存才擋得住
// 「順手把兩邊統一」這個變異。
func TestSellerComplaintViews_PartitionAllComplaintStatuses(t *testing.T) {
	seen := map[string]string{}
	for view, statuses := range sellerComplaintViews {
		for _, st := range statuses {
			if prev, dup := seen[st]; dup {
				t.Fatalf("狀態 %q 同時屬於視圖 %q 與 %q，賣家會在兩個分頁看到同一件案子", st, prev, view)
			}
			seen[st] = view
		}
	}
	all := codemeta.AllInvoiceComplaintStatuss
	if len(seen) != len(all) {
		t.Fatalf("三個視圖共涵蓋 %d 個狀態，CodeMeta 有 %d 個（必須窮盡）", len(seen), len(all))
	}
	for _, meta := range all {
		if _, ok := seen[meta.Value]; !ok {
			t.Fatalf("狀態 %q 不屬於任何視圖 —— 那件案子在 S09 三個分頁裡都看不到", meta.Value)
		}
	}
}

// TestAdminComplaintTabs_OverdueIsACrossCuttingView 釘住 §6.1.16 與上一支**相反**的規則。
//
// OVERDUE 的述詞是 OR 不是單純 status='OVERDUE'：OVERDUE 由 INV.dl 掃描寫入且只從
// OPEN 進入 ⇒ ①掃描還沒跑到 ②賣家逾期後才回覆 ③已升級到 ADM 這三類會從單純述詞
// 下消失，而它們正是 ADM 最該看到的。
func TestAdminComplaintTabs_OverdueIsACrossCuttingView(t *testing.T) {
	overdue, ok := adminComplaintTabs["OVERDUE"]
	if !ok {
		t.Fatal("A07 必須有 OVERDUE 分頁")
	}
	if !overdue.IncludeDueOverdue {
		t.Fatal("🔴 OVERDUE 退化成單純 status='OVERDUE' —— 三類最該被看到的逾期案件會從這一頁消失")
	}
	for _, want := range []string{
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
		codemeta.InvoiceComplaintStatusEscalatedToAdm.Value,
	} {
		if !containsString(overdue.DueOverdue, want) {
			t.Fatalf("OVERDUE 的橫切集合缺 %q（§6.1.16 三類之一）", want)
		}
	}
	// 與 AWAITING_SELLER 交集非空 —— 這正是「加總 ≠ 總筆數」的來源。
	if !containsString(adminComplaintTabs["AWAITING_SELLER"].Statuses, codemeta.InvoiceComplaintStatusOverdue.Value) {
		t.Fatal("AWAITING_SELLER 必須含 OVERDUE：兩者對賣家的手勢相同（該你回了）")
	}
}

// TestUploadStatusProjections_PendingCoversNullAndInFlight 釘住「PENDING 同時涵蓋
// NULL 與在途碼」。
//
// NULL 表達不進整數陣列 ⇒ IncludeNull 是獨立旗標。把它拿掉的話，「還沒查過上傳狀態」
// 的那批 job 在四個投影值底下都撈不出來，而 upload_status 是
// 「發票開出來了但財政部端沒收到」這一類的唯一入口。
func TestUploadStatusProjections_PendingCoversNullAndInFlight(t *testing.T) {
	pending := uploadStatusProjections["PENDING"]
	if !pending.IncludeNull {
		t.Fatal("🔴 PENDING 必須涵蓋 NULL（還沒查過）—— 少了它那批 job 在四個投影值下都撈不出來")
	}
	for _, code := range []int16{1, 2, 3, 31, 32} {
		if !containsInt16(pending.Codes, code) {
			t.Fatalf("PENDING 缺在途碼 %d（chk_invdoc_upload_recheck 的正面枚舉）", code)
		}
	}
	// 三個終態各自獨立，且都**不得**含 NULL：含了就會把「還沒查過」誤報成結論。
	for name, want := range map[string]int16{"UPLOADED": 99, "FAILED": 91, "NOT_FOUND": -1} {
		p := uploadStatusProjections[name]
		if len(p.Codes) != 1 || p.Codes[0] != want {
			t.Fatalf("%s 應恰好對應 AMEGO 碼 %d，實得 %v", name, want, p.Codes)
		}
		if p.IncludeNull {
			t.Fatalf("%s 不得涵蓋 NULL —— 那會把「還沒查過」誤報成已有結論", name)
		}
	}
}

// TestOpenJobStatuses_DerivesFromTabsNotASecondList 釘住「openJobStatuses 由分割表
// 推導」這件事本身：另打一份清單就會在有人改分頁歸屬時安靜地漂移。
func TestOpenJobStatuses_DerivesFromTabsNotASecondList(t *testing.T) {
	open := openJobStatuses()
	for _, tab := range []string{"PENDING", "RETRYING", "NEEDS_ATTENTION", adminJobNeedsManualTab} {
		for _, st := range adminJobTabs[tab] {
			if !containsString(open, st) {
				t.Fatalf("openJobStatuses 漏了 %q（來自分頁 %q）", st, tab)
			}
		}
	}
	// 已了結的兩個分頁不得混進來：混進來 A01 的 priorityJobs 會被已完成的工作淹沒。
	for _, tab := range []string{"COMPLETED", "CANCELLED"} {
		for _, st := range adminJobTabs[tab] {
			if containsString(open, st) {
				t.Fatalf("🔴 已了結的狀態 %q（分頁 %q）混進了 openJobStatuses", st, tab)
			}
		}
	}
}

// TestProjectProviderStatus_ThreeStates 釘住 §6.1.13 的三態表。
//
// 🔴 需求書寫「狀態空值視為配置錯誤」，照字面實作會讓上線當天每一個既有賣家都被標成
// 配置錯誤 —— 查無列是 NOT_STARTED（＝上線當天的全體）。
func TestProjectProviderStatus_ThreeStates(t *testing.T) {
	if got := projectProviderStatus(false, ""); got != codemeta.InvoiceProviderAccountStatusNotStarted.Value {
		t.Fatalf("查無列應投影成 NOT_STARTED（上線當天的全體），實得 %q", got)
	}
	if got := projectProviderStatus(true, ""); got != codemeta.InvoiceProviderAccountStatusConfigError.Value {
		t.Fatalf("有列但 status 為空字串應投影成 CONFIG_ERROR，實得 %q", got)
	}
	if got := projectProviderStatus(true, "NO_SUCH_STATUS"); got != codemeta.InvoiceProviderAccountStatusConfigError.Value {
		t.Fatalf("有列但 status 為值域外應投影成 CONFIG_ERROR，實得 %q", got)
	}
	active := codemeta.InvoiceProviderAccountStatusActive.Value
	if got := projectProviderStatus(true, active); got != active {
		t.Fatalf("已知狀態應原樣回傳，實得 %q", got)
	}
}

// TestTimelineEventOf_ProjectsFiveValuesOnly 釘住時間軸的 5 值投影。
//
// 直接把 to_status 塞進 event 會讓前端拿到 PENDING／ISSUING 這種時間軸上不存在的值，
// 而 codeLabel 查不到就顯示原始碼 —— 一個 DB 狀態值就這樣洩漏到買家畫面上。
func TestTimelineEventOf_ProjectsFiveValuesOnly(t *testing.T) {
	cases := []struct {
		subject, toStatus, want string
		ok                      bool
	}{
		{subjectDocument, codemeta.InvoiceDocumentStatusIssued.Value, codemeta.InvoiceDocumentTimelineEventIssued.Value, true},
		{subjectDocument, codemeta.InvoiceDocumentStatusVoided.Value, codemeta.InvoiceDocumentTimelineEventVoided.Value, true},
		// REISSUED 落在**舊**文件的時間軸上（to_status=SUPERSEDED）。
		{subjectDocument, codemeta.InvoiceDocumentStatusSuperseded.Value, codemeta.InvoiceDocumentTimelineEventReissued.Value, true},
		{subjectCorrectionRequest, codemeta.InvoiceCorrectionStatusUpdatedBeforeIssue.Value, codemeta.InvoiceDocumentTimelineEventUpdated.Value, true},
		// 時間軸沒有「處理中」與「失敗」的值 —— 那些由 state ＋ stateNotice 承擔。
		{subjectDocument, codemeta.InvoiceDocumentStatusPending.Value, "", false},
		{subjectDocument, codemeta.InvoiceDocumentStatusIssueFailed.Value, "", false},
		// 主體型別必須參與判定：CORRECTION_REQUEST 的 COMPLETED 不是文件的 ISSUED。
		{subjectCorrectionRequest, codemeta.InvoiceDocumentStatusIssued.Value, "", false},
	}
	for _, c := range cases {
		got, ok := timelineEventOf(c.subject, c.toStatus)
		if ok != c.ok || got != c.want {
			t.Fatalf("timelineEventOf(%q,%q) = (%q,%v)，預期 (%q,%v)", c.subject, c.toStatus, got, ok, c.want, c.ok)
		}
	}
}

// TestParseInvoiceDateRange_RejectsLooseFormatsAndUsesTaipeiDayBoundary 釘住兩件事。
//
// ① 格式嚴格（§6.1.10 的 AMEGO 實測坑：帶連字號與空字串都回 code:0 ＋ 全部歷史）
// ② 日界是**台北**不是 UTC：直接當 UTC 解析會讓每天前八小時開立的發票落到前一天
func TestParseInvoiceDateRange_RejectsLooseFormatsAndUsesTaipeiDayBoundary(t *testing.T) {
	for _, bad := range []string{"2026-8-1", "2026/08/01", "20260801", "2026-13-01", "2026-08-01T00:00:00Z", "今天"} {
		if _, err := ParseInvoiceDateRange(bad, ""); err == nil {
			t.Fatalf("%q 應該被拒絕（格式驗證要在最上游一次做對）", bad)
		}
	}

	got, err := ParseInvoiceDateRange("2026-08-01", "2026-08-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 台北 2026-08-01 00:00 ＝ UTC 2026-07-31 16:00。取 UTC 00:00 的實作在這裡當場紅。
	wantFrom := time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC)
	if !got.From.Equal(wantFrom) {
		t.Fatalf("下界應為台北日界換算的 %v，實得 %v", wantFrom, got.From.UTC())
	}
	// 上界是當日 23:59:59（台北）＝ UTC 2026-08-31 15:59:59。
	wantTo := time.Date(2026, 8, 31, 15, 59, 59, 0, time.UTC)
	if !got.To.Equal(wantTo) {
		t.Fatalf("上界應為當日台北 23:59:59，實得 %v", got.To.UTC())
	}

	// 兩側皆可缺席（＝該側不篩）。
	empty, err := ParseInvoiceDateRange("", "")
	if err != nil || empty.From != nil || empty.To != nil {
		t.Fatalf("空字串應是「不篩」，實得 %+v err=%v", empty, err)
	}
}

func containsInt16(set []int16, v int16) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
