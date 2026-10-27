package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔驗 A4 雙人解鎖（端點 54／55）。每一支陰性都額外數 repo 的呼叫次數——
// 「回了錯誤碼」不足以證明什麼都沒發生，而「回了錯誤但前面已經寫了一半」正是
// 本流程最貴的失敗形狀（解鎖列建了、job 沒轉，或反過來）。

const (
	unlockRequester = int64(4201)
	unlockApprover  = int64(4202)
	unlockCanceller = int64(4203)
)

// cancelledJob 是 A4 的合法起點：受控取消已落地，等雙人解鎖。
func cancelledJob(version int64) domain.Operation {
	op := opAt("CANCELLED_BY_ADMIN", version)
	canceller := unlockCanceller
	requestedAt := adminNow.Add(-2 * time.Hour)
	op.CancelRequestedBy = &canceller
	op.CancelRequestedAt = &requestedAt
	return op
}

func requestInput() RequestManualUnlockInput {
	return RequestManualUnlockInput{
		ActorAccountID:        unlockRequester,
		JobID:                 7001,
		ProviderCheckEvidence: "致電光貿客服王小姐，確認 requestId ORD-1-1 未產生任何發票",
		ProviderCheckedAt:     adminNow.Add(-30 * time.Minute),
		Reason:                "AMEGO 回應逾時，已查證未開立",
		ExpectedVersion:       5,
		IdempotencyKey:        "idem-unlock-1",
	}
}

// ---------------------------------------------------------------------------
// 端點 54：發起
// ---------------------------------------------------------------------------

func TestManualUnlock_Request_CreatesAwaitingRow(t *testing.T) {
	fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))

	got, err := fx.uc.RequestManualUnlock(context.Background(), requestInput())
	if err != nil {
		t.Fatalf("RequestManualUnlock 失敗：%v", err)
	}
	if got.Status != domain.ManualUnlockAwaitingSecondApproval {
		t.Fatalf("狀態應為 AWAITING_SECOND_APPROVAL，實得 %q", got.Status)
	}
	if got.NeedsSecondApproval {
		t.Fatal("第一次發起不得回 9344（那會讓 ADM 以為請求早就存在）")
	}
	if got.UnlockID == 0 {
		t.Fatal("unlockId 不得為 0：它是端點 55 的 path param")
	}
	// 24 小時窗口是定版值（§6.1.14）。少了它，逾期那條路完全沒有意義。
	if want := adminNow.Add(24 * time.Hour); !got.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt 應為發起後 24 小時（%v），實得 %v", want, got.ExpiresAt)
	}
	if len(fx.unlocks.created) != 1 {
		t.Fatalf("應建立一列解鎖請求，實得 %d", len(fx.unlocks.created))
	}
	if len(fx.audit.admin) != 1 || fx.audit.admin[0].Action != unlockAuditActionRequest {
		t.Fatalf("ADM 稽核應寫一列 %s，實得 %+v", unlockAuditActionRequest, fx.audit.admin)
	}
	if fx.audit.admin[0].RequestID == nil || *fx.audit.admin[0].RequestID != "idem-unlock-1" {
		t.Fatalf("Idempotency-Key 必須落進稽核 request_id，實得 %v", fx.audit.admin[0].RequestID)
	}
	assertNoPIIInAudit(t, fx.audit.admin[0])
	// 查證說明是自由文字（可能寫進第三方客服姓名），不得被複製進保存 5 年的稽核列。
	for _, blob := range [][]byte{fx.audit.admin[0].BeforeState, fx.audit.admin[0].AfterState} {
		if containsSubstring(string(blob), "王小姐") {
			t.Fatalf("providerCheckEvidence 不得進稽核 state：%s", blob)
		}
	}
}

func TestManualUnlock_Request_SecondRequestReturns9344WithExistingID(t *testing.T) {
	fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))
	first, err := fx.uc.RequestManualUnlock(context.Background(), requestInput())
	if err != nil {
		t.Fatalf("第一次發起失敗：%v", err)
	}

	second, err := fx.uc.RequestManualUnlock(context.Background(), requestInput())
	if err != nil {
		t.Fatalf("第二次發起不應是錯誤（9344 是 preserveData 碼）：%v", err)
	}
	if !second.NeedsSecondApproval {
		t.Fatal("已有等待覆核的請求時必須回 9344，否則 ADM 會以為自己建立了新請求")
	}
	if second.UnlockID != first.UnlockID {
		t.Fatalf("9344 帶的必須是**既有那一列**的 unlockId：want %d got %d", first.UnlockID, second.UnlockID)
	}
	if len(fx.unlocks.created) != 1 {
		t.Fatalf("第二次不得再建列，實得 %d 列", len(fx.unlocks.created))
	}
}

// 逾期之後可以重新發起——這正是 ExpireStale 存在的理由（partial unique 的名額釋放）。
func TestManualUnlock_Request_AfterExpiryCanRequestAgain(t *testing.T) {
	fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))
	if _, err := fx.uc.RequestManualUnlock(context.Background(), requestInput()); err != nil {
		t.Fatalf("第一次發起失敗：%v", err)
	}

	// 25 小時後：第一列已逾期。
	fx.uc.WithClock(func() time.Time { return adminNow.Add(25 * time.Hour) })
	in := requestInput()
	in.ProviderCheckedAt = adminNow.Add(24 * time.Hour)
	got, err := fx.uc.RequestManualUnlock(context.Background(), in)
	if err != nil {
		t.Fatalf("逾期後應可重新發起，實得錯誤：%v", err)
	}
	if got.NeedsSecondApproval {
		t.Fatal("逾期那一列不得再被當成「等待覆核」")
	}
	if fx.unlocks.expired != 1 {
		t.Fatalf("應把逾期那一列就地轉 EXPIRED 一次，實得 %d", fx.unlocks.expired)
	}
	if len(fx.unlocks.created) != 2 {
		t.Fatalf("應建立第二列，實得 %d", len(fx.unlocks.created))
	}
}

func TestManualUnlock_Request_RejectsNonCancelledJob(t *testing.T) {
	fx := newJobFixture(opAt("FAILED_PERMANENT", 5), docAt("ISSUE_FAILED", ""))

	_, err := fx.uc.RequestManualUnlock(context.Background(), requestInput())
	assertBizCode(t, err, bizcode.InvoiceJobVersionConflict)
	if len(fx.unlocks.created) != 0 {
		t.Fatalf("守衛不成立時不得建列，實得 %d", len(fx.unlocks.created))
	}
	if len(fx.audit.admin) != 0 {
		t.Fatalf("守衛不成立時不得寫稽核，實得 %d", len(fx.audit.admin))
	}
}

func TestManualUnlock_Request_RejectsVersionMismatch(t *testing.T) {
	fx := newJobFixture(cancelledJob(6), docAt("ISSUE_FAILED", ""))

	_, err := fx.uc.RequestManualUnlock(context.Background(), requestInput()) // expected=5
	assertBizCode(t, err, bizcode.InvoiceJobVersionConflict)
	if len(fx.unlocks.created) != 0 {
		t.Fatalf("版本不符時不得建列，實得 %d", len(fx.unlocks.created))
	}
}

func TestManualUnlock_Request_RejectsBadEvidence(t *testing.T) {
	cases := map[string]func(*RequestManualUnlockInput){
		"空查證說明": func(in *RequestManualUnlockInput) { in.ProviderCheckEvidence = "   " },
		"查證說明超過一千字": func(in *RequestManualUnlockInput) {
			in.ProviderCheckEvidence = repeatRune('查', 1001)
		},
		"查證時間在未來": func(in *RequestManualUnlockInput) {
			in.ProviderCheckedAt = adminNow.Add(time.Minute)
		},
		"查證時間空白": func(in *RequestManualUnlockInput) { in.ProviderCheckedAt = time.Time{} },
		"沒有填理由":  func(in *RequestManualUnlockInput) { in.Reason = " " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))
			in := requestInput()
			mutate(&in)
			if _, err := fx.uc.RequestManualUnlock(context.Background(), in); err == nil {
				t.Fatal("應回契約違規錯誤")
			}
			if len(fx.unlocks.created) != 0 {
				t.Fatalf("驗證失敗時不得建列，實得 %d", len(fx.unlocks.created))
			}
		})
	}
}

// 1000 字整是**合法**的：邊界寫錯一格會讓 DB 的 CHECK 與應用層互相矛盾。
func TestManualUnlock_Request_AcceptsExactly1000Runes(t *testing.T) {
	fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))
	in := requestInput()
	in.ProviderCheckEvidence = repeatRune('查', 1000)
	if _, err := fx.uc.RequestManualUnlock(context.Background(), in); err != nil {
		t.Fatalf("1000 字應合法（DB CHECK 的上界是 <= 1000），實得 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 55：覆核
// ---------------------------------------------------------------------------

func approveFixture(t *testing.T) (*jobFixture, int64) {
	t.Helper()
	fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))
	view, err := fx.uc.RequestManualUnlock(context.Background(), requestInput())
	if err != nil {
		t.Fatalf("前置發起失敗：%v", err)
	}
	return fx, view.UnlockID
}

func TestManualUnlock_Approve_UnlocksJobInSameTransaction(t *testing.T) {
	fx, unlockID := approveFixture(t)

	got, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "已複核光貿查證紀錄", ExpectedVersion: 5,
		IdempotencyKey: "idem-approve-1",
	})
	if err != nil {
		t.Fatalf("ApproveManualUnlock 失敗：%v", err)
	}
	if got.Status != domain.ManualUnlockApproved {
		t.Fatalf("解鎖狀態應為 APPROVED，實得 %q", got.Status)
	}
	// 🔴 兩段必須同一交易：只轉解鎖列而 job 沒進 MANUAL_PROCESS_ALLOWED，
	// 端點 56／62 會繼續回 9349，而 ADM 看到的是「已核准」。
	if fx.ops.stored.Status != "MANUAL_PROCESS_ALLOWED" {
		t.Fatalf("job 必須同時進 MANUAL_PROCESS_ALLOWED，實得 %q", fx.ops.stored.Status)
	}
	if len(fx.ops.allowManual) != 1 {
		t.Fatalf("J29 應被呼叫一次，實得 %d", len(fx.ops.allowManual))
	}
	if len(fx.audit.admin) != 2 {
		t.Fatalf("發起＋覆核應各一列稽核，實得 %d", len(fx.audit.admin))
	}
	approval := fx.audit.admin[1]
	if approval.ApprovalActorAccountID == nil || *approval.ApprovalActorAccountID != unlockRequester {
		t.Fatalf("稽核的第二人欄應填**發起人**（%d），實得 %v", unlockRequester, approval.ApprovalActorAccountID)
	}
	if approval.ActorAccountID == derefInt64(approval.ApprovalActorAccountID) {
		t.Fatal("chk_invadmaudit_two_person：兩個 actor 欄不得相同")
	}
	// 狀態歷程要記下 job 的那一次轉移（誰把它解鎖的）。
	if len(fx.audit.entries) == 0 {
		t.Fatal("應寫一列 invoice_status_history")
	}
}

func TestManualUnlock_Approve_RejectsSameActorAsRequester(t *testing.T) {
	fx, unlockID := approveFixture(t)

	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockRequester, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "自己核准自己", ExpectedVersion: 5,
	})
	assertBizCode(t, err, bizcode.InvoiceManualUnlockSameActor)
	if fx.ops.stored.Status != "CANCELLED_BY_ADMIN" {
		t.Fatalf("被 9345 擋下時 job 不得轉態，實得 %q", fx.ops.stored.Status)
	}
	if len(fx.unlocks.decided) != 0 {
		t.Fatalf("被 9345 擋下時不得動解鎖列，實得 %d 次 Decide", len(fx.unlocks.decided))
	}
}

// 第二道雙人守衛：覆核的人不得是當初**提出取消**的那一位（J29 的 SQL 述詞）。
func TestManualUnlock_Approve_RejectsCancelRequesterAsApprover(t *testing.T) {
	fx, unlockID := approveFixture(t)

	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockCanceller, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "取消發起人自己來覆核", ExpectedVersion: 5,
	})
	assertBizCode(t, err, bizcode.InvoiceManualUnlockSameActor)
	if fx.ops.stored.Status != "CANCELLED_BY_ADMIN" {
		t.Fatalf("被擋下時 job 不得轉態，實得 %q", fx.ops.stored.Status)
	}
	if len(fx.unlocks.decided) != 0 {
		t.Fatalf("被擋下時不得動解鎖列，實得 %d 次 Decide", len(fx.unlocks.decided))
	}
}

func TestManualUnlock_Approve_RejectsExpiredRequest(t *testing.T) {
	fx, unlockID := approveFixture(t)
	fx.uc.WithClock(func() time.Time { return adminNow.Add(25 * time.Hour) })

	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "遲到的覆核", ExpectedVersion: 5,
	})
	assertBizCode(t, err, bizcode.InvoiceJobVersionConflict)
	if fx.ops.stored.Status != "CANCELLED_BY_ADMIN" {
		t.Fatalf("逾期覆核不得解鎖 job，實得 %q", fx.ops.stored.Status)
	}
	if len(fx.unlocks.decided) != 0 {
		t.Fatalf("逾期時不得動解鎖列，實得 %d 次 Decide", len(fx.unlocks.decided))
	}
}

func TestManualUnlock_Approve_RejectDoesNotTouchJob(t *testing.T) {
	fx, unlockID := approveFixture(t)

	got, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: false, Reason: "查證紀錄不足，退回", ExpectedVersion: 5,
	})
	if err != nil {
		t.Fatalf("駁回是正常業務結果不是錯誤：%v", err)
	}
	if got.Status != domain.ManualUnlockRejected {
		t.Fatalf("狀態應為 REJECTED，實得 %q", got.Status)
	}
	if fx.ops.stored.Status != "CANCELLED_BY_ADMIN" {
		t.Fatalf("駁回不得動 job 狀態，實得 %q", fx.ops.stored.Status)
	}
	if len(fx.ops.allowManual) != 0 {
		t.Fatalf("駁回不得呼叫 J29，實得 %d 次", len(fx.ops.allowManual))
	}
}

func TestManualUnlock_Approve_RejectsUnlockFromAnotherJob(t *testing.T) {
	fx, _ := approveFixture(t)

	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: 999999,
		Approved: true, Reason: "不存在的解鎖 id", ExpectedVersion: 5,
	})
	assertBizCode(t, err, bizcode.InvoiceJobNotFound)
	if len(fx.unlocks.decided) != 0 {
		t.Fatalf("查無時不得動解鎖列，實得 %d 次", len(fx.unlocks.decided))
	}
}

func TestManualUnlock_Approve_RejectsSecondApproval(t *testing.T) {
	fx, unlockID := approveFixture(t)
	if _, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "第一次覆核", ExpectedVersion: 5,
	}); err != nil {
		t.Fatalf("第一次覆核失敗：%v", err)
	}
	before := len(fx.ops.allowManual)

	// job 的 version 已被 J29 推進 ⇒ 第二次帶舊版本，兩道守衛都會擋。
	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "重送", ExpectedVersion: 5,
	})
	assertBizCode(t, err, bizcode.InvoiceJobVersionConflict)
	if len(fx.ops.allowManual) != before {
		t.Fatalf("重送不得再呼叫 J29，實得 %d（原 %d）", len(fx.ops.allowManual), before)
	}
}

// ---------------------------------------------------------------------------
// 讀取面：逾期投影
// ---------------------------------------------------------------------------

func TestManualUnlock_LatestProjectsExpired(t *testing.T) {
	fx, _ := approveFixture(t)

	// 未逾期：仍是 AWAITING_SECOND_APPROVAL。
	view, found, err := fx.uc.LatestManualUnlock(context.Background(), 7001)
	if err != nil || !found {
		t.Fatalf("應找得到最近一列：found=%v err=%v", found, err)
	}
	if view.Status != domain.ManualUnlockAwaitingSecondApproval {
		t.Fatalf("未逾期時狀態應為 AWAITING_SECOND_APPROVAL，實得 %q", view.Status)
	}

	// 逾期後：投影成 EXPIRED，且**沒有任何人改過那一列**（不需要排程器）。
	fx.uc.WithClock(func() time.Time { return adminNow.Add(25 * time.Hour) })
	view, _, err = fx.uc.LatestManualUnlock(context.Background(), 7001)
	if err != nil {
		t.Fatalf("讀取失敗：%v", err)
	}
	if view.Status != domain.ManualUnlockExpired {
		t.Fatalf("逾期應投影成 EXPIRED，實得 %q", view.Status)
	}
	if fx.unlocks.expired != 0 {
		t.Fatal("讀取路徑不得寫入：逾期投影是唯讀的")
	}
}

func TestManualUnlock_LatestReturnsNotFoundWhenNeverRequested(t *testing.T) {
	fx := newJobFixture(cancelledJob(5), docAt("ISSUE_FAILED", ""))
	if _, found, err := fx.uc.LatestManualUnlock(context.Background(), 7001); err != nil || found {
		t.Fatalf("從未發起過應回 found=false（proto 的 manual_unlock 為 null），實得 found=%v err=%v", found, err)
	}
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

func assertBizCode(t *testing.T, err error, want int32) {
	t.Helper()
	if err == nil {
		t.Fatalf("應回業務碼 %d，實得 nil", want)
	}
	code, msg, ok := errAsBiz(err)
	if !ok {
		t.Fatalf("應是 DomainError，實得 %T：%v", err, err)
	}
	if code != want {
		t.Fatalf("業務碼應為 %d，實得 %d（%s）", want, code, msg)
	}
}

func repeatRune(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}

func containsSubstring(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// ---------------------------------------------------------------------------
// J28：CANCEL_BLOCKED_UNKNOWN 那條入口（#3089 A 組第 2 項）
//
// 這一組在 2026-09-04 之前**一支都不存在**，因為那條邊零呼叫端。它的價值集中在兩件
// 逐項驗不到的事：① 同一交易內走完 J28→J29（少一段就是「ADM 看到已核准而 job 沒動」）
// ② 兩句 CAS 的版本必須是 v 與 **v+1**（J28 的 SQL 有 version = version + 1，
//    傳錯是靜默的 rows=0）。
// ---------------------------------------------------------------------------

// blockedJob 是 J28 的合法起點：取消已提出但 provider 狀態不明，等雙人確認。
func blockedJob(version int64) domain.Operation {
	op := opAt("CANCEL_BLOCKED_UNKNOWN", version)
	canceller := unlockCanceller
	requestedAt := adminNow.Add(-3 * time.Hour)
	op.CancelRequestedBy = &canceller
	op.CancelRequestedAt = &requestedAt
	return op
}

func blockedRequestInput() RequestManualUnlockInput {
	in := requestInput()
	in.CancelEvidenceKind = "MOF_PLATFORM_SCREENSHOT"
	in.CancelEvidenceObjectKey = "invoice/unlock/7001/mof.png"
	return in
}

func blockedApproveFixture(t *testing.T) (*jobFixture, int64) {
	t.Helper()
	fx := newJobFixture(blockedJob(5), docAt("ISSUE_FAILED", ""))
	view, err := fx.uc.RequestManualUnlock(context.Background(), blockedRequestInput())
	if err != nil {
		t.Fatalf("前置發起失敗：%v", err)
	}
	return fx, view.UnlockID
}

func TestManualUnlock_Request_AcceptsBlockedUnknownWithEvidence(t *testing.T) {
	fx := newJobFixture(blockedJob(5), docAt("ISSUE_FAILED", ""))

	got, err := fx.uc.RequestManualUnlock(context.Background(), blockedRequestInput())
	if err != nil {
		t.Fatalf("CANCEL_BLOCKED_UNKNOWN 必須是合法起點（J28 的唯一入口）：%v", err)
	}
	if got.Status != domain.ManualUnlockAwaitingSecondApproval {
		t.Fatalf("狀態應為 AWAITING_SECOND_APPROVAL，實得 %q", got.Status)
	}
	// 證據要落在那一列上——覆核者從這裡讀，讀不到就是對著看不見的東西按同意。
	if got.CancelEvidenceKind == nil || *got.CancelEvidenceKind != "MOF_PLATFORM_SCREENSHOT" {
		t.Fatalf("回應必須帶回證據種類，實得 %v", got.CancelEvidenceKind)
	}
	if len(fx.unlocks.created) != 1 {
		t.Fatalf("應建一列，實得 %d", len(fx.unlocks.created))
	}
	if k := fx.unlocks.created[0].CancelEvidenceKind; k == nil || *k != "MOF_PLATFORM_SCREENSHOT" {
		t.Fatalf("寫入 DB 的證據種類錯：%v", k)
	}
}

func TestManualUnlock_Request_RejectsBadEvidenceKind(t *testing.T) {
	cases := map[string]struct {
		op   domain.Operation
		mut  func(*RequestManualUnlockInput)
		want error
	}{
		"BLOCKED 但沒帶證據": {
			op:   blockedJob(5),
			mut:  func(in *RequestManualUnlockInput) { in.CancelEvidenceKind = "" },
			want: ErrUnlockEvidenceKindRequired,
		},
		// PROVIDER_QUERY_71 是 CodeMeta 的合法值，但**不是 J28 的合法值**：它是
		// provider 直接作證、由 J24 自動落地。收下它等於讓人替系統蓋章。
		"BLOCKED 帶系統自動採信的種類": {
			op:   blockedJob(5),
			mut:  func(in *RequestManualUnlockInput) { in.CancelEvidenceKind = "PROVIDER_QUERY_71" },
			want: ErrUnlockEvidenceKindInvalid,
		},
		"CANCELLED_BY_ADMIN 卻帶了證據": {
			op:   cancelledJob(5),
			mut:  func(in *RequestManualUnlockInput) { in.CancelEvidenceKind = "MOF_PLATFORM_SCREENSHOT" },
			want: ErrUnlockEvidenceKindUnwanted,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newJobFixture(tc.op, docAt("ISSUE_FAILED", ""))
			in := requestInput()
			tc.mut(&in)
			_, err := fx.uc.RequestManualUnlock(context.Background(), in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("錯誤應為 %v，實得 %v", tc.want, err)
			}
			if len(fx.unlocks.created) != 0 {
				t.Fatalf("驗證不過時不得建列，實得 %d", len(fx.unlocks.created))
			}
		})
	}
}

func TestManualUnlock_Approve_ChainsJ28ThenJ29(t *testing.T) {
	fx, unlockID := blockedApproveFixture(t)

	got, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "已複核財政部平台截圖", ExpectedVersion: 5,
		IdempotencyKey: "idem-approve-j28", // gitleaks:allow（測試值）
	})
	if err != nil {
		t.Fatalf("ApproveManualUnlock 失敗：%v", err)
	}
	if got.Status != domain.ManualUnlockApproved {
		t.Fatalf("解鎖狀態應為 APPROVED，實得 %q", got.Status)
	}
	if fx.ops.stored.Status != "MANUAL_PROCESS_ALLOWED" {
		t.Fatalf("同一交易要走完 J28→J29，實得 %q", fx.ops.stored.Status)
	}

	// ── 版本鏈：J28 用 v，J29 用 v+1 ──
	if len(fx.ops.confirmCancel) != 1 {
		t.Fatalf("J28 應被呼叫一次，實得 %d", len(fx.ops.confirmCancel))
	}
	if v := fx.ops.confirmCancel[0].Version; v != 5 {
		t.Fatalf("J28 應以 v=5 做 CAS，實得 %d", v)
	}
	if len(fx.ops.allowManual) != 1 {
		t.Fatalf("J29 應被呼叫一次，實得 %d", len(fx.ops.allowManual))
	}
	if v := fx.ops.allowManual[0].Version; v != 6 {
		t.Fatalf("J29 必須用 **v+1**（J28 的 UPDATE 有 version+1），實得 %d", v)
	}

	// 證據要寫進 operation，chk_invop_admin_cancel_needs_confirm 靠它放行。
	if k := fx.ops.confirmCancel[0].Kind; k != "MOF_PLATFORM_SCREENSHOT" {
		t.Fatalf("J28 的證據種類應由解鎖列帶過來，實得 %q", k)
	}
	if fx.ops.confirmCancel[0].ObjectKey == nil {
		t.Fatal("截圖類必須把 objectKey 一併帶進 operation")
	}
	if fx.ops.confirmCancel[0].ConfirmedBy != unlockApprover {
		t.Fatal("cancel_confirmed_by 應是覆核者（第二人）")
	}

	// ── 兩列狀態歷程，reason_code 要分得出哪一列是哪一段 ──
	codes := make([]string, 0, len(fx.audit.entries))
	for _, e := range fx.audit.entries {
		codes = append(codes, derefString(e.ReasonCode))
	}
	if len(codes) != 2 {
		t.Fatalf("J28 與 J29 各要一列狀態歷程，實得 %d：%v", len(codes), codes)
	}
	if codes[0] != unlockReasonCancelConfirmed || codes[1] != unlockReasonApproved {
		t.Fatalf("兩列的 reason_code 應為 [%s %s]，實得 %v",
			unlockReasonCancelConfirmed, unlockReasonApproved, codes)
	}
}

// 覆核者不得是當初提出取消的那一位——J28 這條路徑上，違反它的後果是 DB 的
// chk_invop_two_person 回 23514（一句 500），所以應用層的 pre-read 必須先擋。
func TestManualUnlock_Approve_BlockedPathRejectsCancelRequester(t *testing.T) {
	fx, unlockID := blockedApproveFixture(t)

	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockCanceller, JobID: 7001, UnlockID: unlockID,
		Approved: true, Reason: "已複核", ExpectedVersion: 5, IdempotencyKey: "idem-same",
	})
	assertBizCode(t, err, bizcode.InvoiceManualUnlockSameActor)
	if len(fx.ops.confirmCancel) != 0 {
		t.Fatalf("守衛不成立時不得走到 J28，實得 %d 次", len(fx.ops.confirmCancel))
	}
	if fx.ops.stored.Status != "CANCEL_BLOCKED_UNKNOWN" {
		t.Fatalf("job 狀態不得改變，實得 %q", fx.ops.stored.Status)
	}
}

// 駁回不走 J28：job 留在 CANCEL_BLOCKED_UNKNOWN，之後仍可重新發起。
func TestManualUnlock_Approve_BlockedPathRejectDoesNotConfirmCancel(t *testing.T) {
	fx, unlockID := blockedApproveFixture(t)

	_, err := fx.uc.ApproveManualUnlock(context.Background(), ApproveManualUnlockInput{
		ActorAccountID: unlockApprover, JobID: 7001, UnlockID: unlockID,
		Approved: false, Reason: "截圖看不出單號", ExpectedVersion: 5, IdempotencyKey: "idem-rej",
	})
	if err != nil {
		t.Fatalf("駁回不應失敗：%v", err)
	}
	if len(fx.ops.confirmCancel) != 0 {
		t.Fatalf("駁回不得確認取消，實得 %d 次", len(fx.ops.confirmCancel))
	}
	if fx.ops.stored.Status != "CANCEL_BLOCKED_UNKNOWN" {
		t.Fatalf("駁回後 job 必須留在原狀態，實得 %q", fx.ops.stored.Status)
	}
}

// #3112 的根本修法：非開立類不得進入 A4 的解鎖環節。
//
// 解鎖的目的是「讓賣家自行開立並回報號碼」（端點 56），而作廢／折讓／查詢
// 沒有那個號碼 ⇒ 解鎖之後**沒有任何出口**，job 會永遠停在 MANUAL_PROCESS_ALLOWED。
// 在入口擋掉比在端點 56 才說「走不通」好：後者是已經走進死路才告訴你。
func TestManualUnlock_Request_RejectsNonIssuanceType(t *testing.T) {
	for _, ot := range []string{
		codemeta.InvoiceOperationTypeVoid.Value,
		codemeta.InvoiceOperationTypeAllowance.Value,
		codemeta.InvoiceOperationTypeQuery.Value,
	} {
		t.Run(ot, func(t *testing.T) {
			op := cancelledJob(5)
			op.OperationType = ot
			fx := newJobFixture(op, docAt("ISSUE_FAILED", ""))

			_, err := fx.uc.RequestManualUnlock(context.Background(), requestInput())
			assertBizCode(t, err, bizcode.InvoiceExternalIssuanceNotAllowed)
			if len(fx.unlocks.created) != 0 {
				t.Fatalf("守衛不成立時不得建列，實得 %d", len(fx.unlocks.created))
			}
		})
	}
}

// 開立類仍走得通（守衛沒有放寬過頭）。
func TestManualUnlock_Request_IssuanceTypeStillAllowed(t *testing.T) {
	op := cancelledJob(5)
	op.OperationType = codemeta.InvoiceOperationTypeReissue.Value
	fx := newJobFixture(op, docAt("ISSUE_FAILED", ""))

	if _, err := fx.uc.RequestManualUnlock(context.Background(), requestInput()); err != nil {
		t.Fatalf("REISSUE 是開立類，必須走得通：%v", err)
	}
}

// #3108：證據檔的 staged token 換發。

type fakeUnlockEvidence struct {
	key    string
	err    error
	tokens []string
	ops    []int64

	readData []byte
	readType string
	readName string
	readErr  error
	readKeys []string
	readOps  []int64
}

func (f *fakeUnlockEvidence) FinalizeUnlockEvidence(_ context.Context, token string, opID int64) (string, error) {
	f.tokens = append(f.tokens, token)
	f.ops = append(f.ops, opID)
	return f.key, f.err
}

// ReadUnlockEvidence 鏡射 media 的授權形狀：(objectKey, refID) 一起授權，
// refID 傳錯的症狀是「資產不存在」而不是「無權存取」⇒ 替身也用 refID 判。
func (f *fakeUnlockEvidence) ReadUnlockEvidence(_ context.Context, objectKey string, opID int64) ([]byte, string, string, error) {
	f.readKeys = append(f.readKeys, objectKey)
	f.readOps = append(f.readOps, opID)
	if f.readErr != nil {
		return nil, "", "", f.readErr
	}
	return f.readData, f.readType, f.readName, nil
}

func TestManualUnlock_Request_FinalizesStagedToken(t *testing.T) {
	fx := newJobFixture(blockedJob(5), docAt("ISSUE_FAILED", ""))
	fin := &fakeUnlockEvidence{key: "invoice/unlock/7001/mof.png"}
	fx.uc.unlockEvidence = fin

	in := blockedRequestInput()
	in.CancelEvidenceObjectKey = ""
	in.CancelEvidenceStagedToken = "staged-abc"

	if _, err := fx.uc.RequestManualUnlock(context.Background(), in); err != nil {
		t.Fatalf("RequestManualUnlock 失敗：%v", err)
	}
	if len(fin.tokens) != 1 || fin.tokens[0] != "staged-abc" {
		t.Fatalf("換發應收到 token，實得 %v", fin.tokens)
	}
	// 🔴 refID 必須是 operation.id——證據屬於那一件工作。
	if len(fin.ops) != 1 || fin.ops[0] != 7001 {
		t.Fatalf("換發的 refID 應為 operation.id=7001，實得 %v", fin.ops)
	}
	if k := fx.unlocks.created[0].CancelEvidenceObjectKey; k == nil || *k != fin.key {
		t.Fatalf("落庫的應是換發後的 objectKey，實得 %v", k)
	}
}

// objectKey 與 stagedToken 二擇一：兩個都給時無法判斷哪個才是這次要存的。
func TestManualUnlock_Request_RejectsBothKeyAndToken(t *testing.T) {
	fx := newJobFixture(blockedJob(5), docAt("ISSUE_FAILED", ""))
	fx.uc.unlockEvidence = &fakeUnlockEvidence{key: "k"}

	in := blockedRequestInput()
	in.CancelEvidenceStagedToken = "staged-abc" // objectKey 已由 blockedRequestInput 填

	_, err := fx.uc.RequestManualUnlock(context.Background(), in)
	if !errors.Is(err, ErrUnlockEvidenceKeyConflict) {
		t.Fatalf("兩者並存應回 ErrUnlockEvidenceKeyConflict，實得 %v", err)
	}
	if len(fx.unlocks.created) != 0 {
		t.Fatalf("驗證不過時不得建列，實得 %d", len(fx.unlocks.created))
	}
}

// 換發失敗 ⇒ 整筆失敗，不得留下一列指向不存在證據的解鎖請求。
func TestManualUnlock_Request_FinalizeFailureAborts(t *testing.T) {
	fx := newJobFixture(blockedJob(5), docAt("ISSUE_FAILED", ""))
	fx.uc.unlockEvidence = &fakeUnlockEvidence{err: errors.New("media down")}

	in := blockedRequestInput()
	in.CancelEvidenceObjectKey = ""
	in.CancelEvidenceStagedToken = "staged-abc"

	if _, err := fx.uc.RequestManualUnlock(context.Background(), in); err == nil {
		t.Fatal("換發失敗必須讓整筆失敗")
	}
	if len(fx.unlocks.created) != 0 {
		t.Fatalf("換發失敗時不得建列，實得 %d", len(fx.unlocks.created))
	}
}

// #3108 第 4 件：覆核者下載證據檔。
func TestManualUnlock_ReadEvidence(t *testing.T) {
	newFx := func(key *string) (*jobFixture, *fakeUnlockEvidence) {
		fx := newJobFixture(blockedJob(5), docAt("ISSUE_FAILED", ""))
		fin := &fakeUnlockEvidence{readData: []byte("PNGDATA"), readType: "image/png", readName: "mof.png"}
		fx.uc.unlockEvidence = fin
		fx.unlocks.rows = append(fx.unlocks.rows, domain.ManualUnlock{
			ID: 9001, OperationID: 7001, Status: domain.ManualUnlockAwaitingSecondApproval,
			CancelEvidenceObjectKey: key,
		})
		return fx, fin
	}
	key := "invoice/unlock/7001/mof.png"

	t.Run("有證據檔", func(t *testing.T) {
		fx, fin := newFx(&key)
		got, err := fx.uc.ReadUnlockEvidence(context.Background(), 7001, 9001)
		if err != nil {
			t.Fatalf("讀取失敗：%v", err)
		}
		if string(got.Data) != "PNGDATA" || got.ContentType != "image/png" || got.FileName != "mof.png" {
			t.Fatalf("回傳內容錯：%+v", got)
		}
		// 🔴 refID 必須是 operation.id——media 用 (objectKey, refID) 一起授權。
		if len(fin.readOps) != 1 || fin.readOps[0] != 7001 {
			t.Fatalf("refID 應為 operation.id=7001，實得 %v", fin.readOps)
		}
	})

	t.Run("unlock 不屬於這個 job ⇒ 9340", func(t *testing.T) {
		fx, fin := newFx(&key)
		_, err := fx.uc.ReadUnlockEvidence(context.Background(), 8888, 9001)
		assertBizCode(t, err, bizcode.InvoiceJobNotFound)
		if len(fin.readKeys) != 0 {
			t.Fatal("守衛不成立時不得讀 media")
		}
	})

	t.Run("沒有證據檔 ⇒ 9340 而不是空檔案", func(t *testing.T) {
		fx, fin := newFx(nil)
		_, err := fx.uc.ReadUnlockEvidence(context.Background(), 7001, 9001)
		assertBizCode(t, err, bizcode.InvoiceJobNotFound)
		if len(fin.readKeys) != 0 {
			t.Fatal("沒有 objectKey 時不得讀 media")
		}
	})
}
