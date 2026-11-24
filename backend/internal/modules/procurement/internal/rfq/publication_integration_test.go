//go:build integration

package rfq_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var deadline = time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)

func TestPreviewPublicationFreezesTheRecipientAndTheHashesAreRecomputable(t *testing.T) {
	f := newFixture(t)
	in := f.publicationIn()
	in.ResponseDeadline = &deadline
	p := f.approved(in)
	if p.RecipientCount != 1 {
		t.Fatalf("受眾數：%+v", p)
	}
	if p.ApprovalVersion != 1 || !p.ExpiresAt.Equal(f.now.Add(rfq.ApprovalTTL)) {
		t.Fatalf("版本或到期時間：%+v", p)
	}
	if p.Payload.Item != "小白菜" || p.Payload.DeliveryCity != "台北市" || p.Payload.LowerBoundAmount != 100 || p.Payload.UpperBoundAmount != 200 ||
		p.Payload.ResponseDate == nil || !p.Payload.ResponseDate.Equal(deadline) {
		t.Fatalf("賣家看得到的內容：%+v", p.Payload)
	}
	again, err := f.service().PreviewPublication(context.Background(), in) // 重送回同一份核准
	must(t, err)
	if again.ApprovalID != p.ApprovalID || again.SellerFacingHash != p.SellerFacingHash || again.AudienceHash != p.AudienceHash {
		t.Fatalf("重送要回同一份核准：%+v vs %+v", again, p)
	}
	other := f.approved(f.publicationIn()) // 另一把鍵、沒有期限：內容不同，hash 不同
	if other.ApprovalID == p.ApprovalID || other.SellerFacingHash == p.SellerFacingHash {
		t.Fatal("有沒有回價期限，賣家可見內容的 hash 要不同")
	}
	if other.AudienceHash != p.AudienceHash {
		t.Fatal("受眾相同，hash 要相同")
	}
	if n := f.count("procurement.rfq_publication_approvals"); n != 2 {
		t.Fatalf("rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationDirectKeepsTheSelectionOrderAndAudienceHashDependsOnIt(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20, 30)
	forward := f.approved(f.directIn(set, c[0], c[2]))
	reverse := f.approved(f.directIn(set, c[2], c[0]))
	if forward.RecipientCount != 2 {
		t.Fatalf("指定對象：%+v", forward)
	}
	if forward.AudienceHash == reverse.AudienceHash {
		t.Fatal("順序不同，受眾的 hash 要不同")
	}
	rows, err := store.New(f.d.Conn(context.Background())).ListApprovalRecipients(context.Background(), store.ListApprovalRecipientsParams{ApprovalID: reverse.ApprovalID, BuyerID: &f.buyer})
	must(t, err)
	if len(rows) != 2 || rows[0].StoreID != 30 || rows[1].StoreID != 10 {
		t.Fatalf("受眾要依選擇的順序凍結：%+v", rows)
	}
	if forward.SellerFacingHash != reverse.SellerFacingHash {
		t.Fatal("賣家可見的內容相同，hash 要相同")
	}
}

func TestPreviewPublicationDirectOnlyAcceptsTheEffectiveCandidateSet(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	oldSet, oldCands := f.searched(10, 20)
	newSet, newCands := f.searched(10, 20)

	_, err := f.service().PreviewPublication(ctx, f.directIn(oldSet, oldCands[0]))
	wantConflict(t, err, "candidate_set") // 有更新的搜尋成功
	if _, err := f.service().PreviewPublication(ctx, f.directIn(newSet, newCands[0])); err != nil {
		t.Fatalf("有效的候選集要收：%v", err)
	}

	f.setStatus("CLARIFYING")
	_, err = f.service().PreviewPublication(ctx, f.directIn(newSet, newCands[1]))
	wantKind(t, err, apperr.KindFailedPrecondition) // 需求已離開 MATCHING：狀態檢查先於候選集檢查
	if n := f.count("procurement.rfq_publication_approvals"); n != 1 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationDirectBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	set, c := f.searched(10, 20)
	other := f.sibling(buyer)
	otherSet, otherCands := other.searched(10)

	_, err := f.service().PreviewPublication(ctx, f.directIn(set, otherCands[0]))
	wantInvalid(t, err, "SelectedCandidateIDs[0]") // 候選不在這個候選集
	_, err = f.service().PreviewPublication(ctx, f.directIn(set, c[0], uuid.NewV7()))
	wantInvalid(t, err, "SelectedCandidateIDs[1]")
	_, err = f.service().PreviewPublication(ctx, f.directIn(uuid.NewV7(), c[0]))
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().PreviewPublication(ctx, f.directIn(otherSet, otherCands[0])) // 同一位買家另一張需求的候選集
	wantKind(t, err, apperr.KindNotFound)
	foreign := f.sibling(buyer + 1)
	theirSet, theirCands := foreign.searched(10)
	_, err = f.service().PreviewPublication(ctx, f.directIn(theirSet, theirCands[0])) // 別的買家的候選集
	wantKind(t, err, apperr.KindNotFound)
	if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationShapeErrorsPointAtTheField(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	set, c := f.searched(10, 20)

	noSet := f.directIn(set, c[0])
	noSet.CandidateSetID = nil
	_, err := f.service().PreviewPublication(ctx, noSet)
	wantInvalid(t, err, "CandidateSetID")
	_, err = f.service().PreviewPublication(ctx, f.directIn(set))
	wantInvalid(t, err, "SelectedCandidateIDs")
	_, err = f.service().PreviewPublication(ctx, f.directIn(set, c[0], c[0]))
	wantInvalid(t, err, "SelectedCandidateIDs[1]")
	if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
		t.Fatalf("形狀錯誤不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationStaleBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	stale := f.publicationIn()
	stale.ExpectedRevisionID = uuid.NewV7()
	_, err := f.service().PreviewPublication(ctx, stale)
	wantConflict(t, err, "request_revision")

	foreign := f.sibling(buyer + 1)
	notMine := f.publicationIn()
	notMine.RequestID, notMine.ExpectedRevisionID = foreign.request, foreign.revision // 買家是 f.buyer，需求是別人的
	_, err = f.service().PreviewPublication(ctx, notMine)
	wantKind(t, err, apperr.KindNotFound)

	old := f.publicationIn()
	f.revise(func(*store.InsertRevisionParams) {}) // 買家帶著舊版本來預覽
	_, err = f.service().PreviewPublication(ctx, old)
	wantConflict(t, err, "request_revision")
	if n := f.count("procurement.rfq_publication_approvals WHERE request_id = $1", f.request); n != 0 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

// 輸入檢查先於讀取與鎖：區間不合法、補充說明過長又帶著過時的版本，回的是輸入的錯誤，不是版本衝突。
func TestPreviewPublicationValidatesInputsBeforeTheRevisionCheck(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	badRange := f.publicationIn()
	badRange.ExpectedRevisionID = uuid.NewV7()
	badRange.LowerBoundAmount, badRange.UpperBoundAmount = 20, 10
	_, err := f.service().PreviewPublication(ctx, badRange)
	wantInvalid(t, err, "LowerBoundAmount")
	longNote := f.publicationIn()
	longNote.ExpectedRevisionID = uuid.NewV7()
	longNote.BuyerNote = strings.Repeat("字", rfq.MaxBuyerNoteRunes+1)
	_, err = f.service().PreviewPublication(ctx, longNote)
	wantInvalid(t, err, "BuyerNote")
	if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
		t.Fatalf("rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationStoresTheRangeAndTheTrimmedNote(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	in := f.publicationIn()
	in.BuyerNote = "  請盡快  "
	p := f.approved(in)
	if p.Payload.BuyerNote != "請盡快" || p.Payload.LowerBoundAmount != 100 || p.Payload.UpperBoundAmount != 200 || p.Payload.BasisUnit != "KG" || p.Payload.BasisQuantity != 1 {
		t.Fatalf("賣家看得到的內容 = %+v", p.Payload)
	}
	row, err := store.New(f.d.Conn(ctx)).GetApproval(ctx, store.GetApprovalParams{ID: p.ApprovalID, RequestID: f.request, BuyerID: &f.buyer})
	must(t, err)
	if row.BuyerNote != "請盡快" || row.RangeLowerAmount != 100 || row.RangeUpperAmount != 200 || row.CandidateSetID != nil {
		t.Fatalf("核准存的欄位 = %+v", row)
	}
	exact := f.publicationIn()
	exact.BuyerNote = strings.Repeat("字", rfq.MaxBuyerNoteRunes)
	if _, err := f.service().PreviewPublication(ctx, exact); err != nil {
		t.Fatalf("剛好上限要收：%v", err)
	}
	blank := f.publicationIn()
	blank.BuyerNote = "   "
	b := f.approved(blank)
	if b.Payload.BuyerNote != "" {
		t.Fatalf("只有空白的補充說明存成空字串：%q", b.Payload.BuyerNote)
	}
	// 賣家看到的範圍與補充說明都進 hash：不同的區間、不同的說明得到不同的 hash
	wider := f.publicationIn()
	wider.UpperBoundAmount = 300
	if f.approved(wider).SellerFacingHash == f.approved(f.publicationIn()).SellerFacingHash {
		t.Fatal("單價區間不同，賣家可見內容的 hash 要不同")
	}
	if f.approved(in).SellerFacingHash == f.approved(f.publicationIn()).SellerFacingHash {
		t.Fatal("補充說明不同，賣家可見內容的 hash 要不同")
	}
}

func TestPreviewPublicationRequiresAReadyOrMatchingRequestAndAUnit(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.setStatus("CLARIFYING")
	_, err := f.service().PreviewPublication(ctx, f.publicationIn())
	wantKind(t, err, apperr.KindFailedPrecondition)
	wantReason(t, err, apperr.ReasonRequirementNotReady, true)
	f.setStatus("MATCHING")
	if _, err := f.service().PreviewPublication(ctx, f.publicationIn()); err != nil {
		t.Fatalf("MATCHING 要收：%v", err)
	}
	f.revise(func(p *store.InsertRevisionParams) { p.Unit = nil })
	_, err = f.service().PreviewPublication(ctx, f.publicationIn())
	wantKind(t, err, apperr.KindFailedPrecondition)
	wantReason(t, err, apperr.ReasonRequirementNotReady, true)
	if n := f.count("procurement.rfq_publication_approvals"); n != 1 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

// 回價期限以營業時區（台北）的今天為界：期限是今天可以，早於今天回 INVALID_ARGUMENT。
// 時間選在台北已經是 11/23、UTC 還是 11/22：以 UTC 判斷的實作會把 11/22 當成今天。
func TestPreviewPublicationResponseDeadlineIsBoundedByTodayInTaipei(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.now = time.Date(2026, 11, 22, 16, 30, 0, 0, time.UTC)
	yesterday := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

	past := f.publicationIn()
	past.ResponseDeadline = &yesterday
	_, err := f.service().PreviewPublication(ctx, past)
	wantInvalid(t, err, "ResponseDeadline")
	if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
	ok := f.publicationIn()
	ok.ResponseDeadline = &today
	p := f.approved(ok)
	if p.Payload.ResponseDate == nil || !p.Payload.ResponseDate.Equal(today) {
		t.Fatalf("期限是今天要收：%+v", p.Payload.ResponseDate)
	}
}

func TestPreviewPublicationReplayIsNotBlockedByADeadlineThatPassedSinceTheFirstCall(t *testing.T) {
	f := newFixture(t)
	in := f.publicationIn()
	d := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	in.ResponseDeadline = &d
	first := f.approved(in)
	f.now = f.now.Add(23 * time.Hour) // 台北已是隔天，期限過了
	again, err := f.service().PreviewPublication(context.Background(), in)
	must(t, err)
	if again.ApprovalID != first.ApprovalID {
		t.Fatalf("重送要回同一份核准：%+v vs %+v", again, first)
	}
}

func TestPreviewPublicationRejectsTheSameKeyWithDifferentContent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	set, c := f.searched(10, 20)
	otherSet, otherCands := f.searched(10, 20) // 之後的搜尋：同樣的店、不同的候選集
	direct := f.directIn(otherSet, otherCands[0], otherCands[1])
	f.approved(direct)
	for name, edit := range map[string]func(*rfq.PreviewPublicationInput){
		"order": func(i *rfq.PreviewPublicationInput) {
			i.SelectedCandidateIDs = []uuid.UUID{otherCands[1], otherCands[0]}
		},
		"fewer":    func(i *rfq.PreviewPublicationInput) { i.SelectedCandidateIDs = []uuid.UUID{otherCands[0]} },
		"deadline": func(i *rfq.PreviewPublicationInput) { i.ResponseDeadline = &deadline },
		"lower":    func(i *rfq.PreviewPublicationInput) { i.LowerBoundAmount++ },
		"upper":    func(i *rfq.PreviewPublicationInput) { i.UpperBoundAmount++ },
		"note":     func(i *rfq.PreviewPublicationInput) { i.BuyerNote = "x" },
		"revision": func(i *rfq.PreviewPublicationInput) { i.ExpectedRevisionID = uuid.NewV7() },
		"request":  func(i *rfq.PreviewPublicationInput) { i.RequestID = uuid.NewV7() },
		// 同一批店、同樣的順序，但候選集是另一個：沒有比對候選集本身的話這個重送會被當成同一份
		"candidate set": func(i *rfq.PreviewPublicationInput) {
			i.CandidateSetID, i.SelectedCandidateIDs = &set, []uuid.UUID{c[0], c[1]}
		},
	} {
		changed := direct
		edit(&changed)
		_, err := f.service().PreviewPublication(ctx, changed)
		if err == nil {
			t.Errorf("同一個鍵、不同的%s要回錯誤", name)
			continue
		}
		wantInvalid(t, err, "client_request_id")
	}
	if n := f.count("procurement.rfq_publication_approvals"); n != 1 {
		t.Fatalf("rfq_publication_approvals %d 列", n)
	}
}

func TestExecuteCreatesThePublicationOneEventAndNoRun(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	pub, err := f.execute(f.executeIn(p))
	must(t, err)
	if pub.Replayed || pub.State != model.PublicationStatePublished || pub.Version != 1 ||
		pub.AudienceHash != p.AudienceHash || pub.SellerFacingHash != p.SellerFacingHash || !pub.PublishedAt.Equal(f.now) {
		t.Fatalf("發布 = %+v，預覽 %+v", pub, p)
	}
	if state, version := f.approvalState(p.ApprovalID); state != "SUCCEEDED" || version != 3 {
		t.Fatalf("核准 %s v%d", state, version)
	}
	evs := f.events()
	if len(evs) != 1 || evs[0].PublicationID != pub.ID || evs[0].RequestID != f.request || evs[0].BuyerID != f.buyer || evs[0].ApprovalID != p.ApprovalID ||
		len(evs[0].Invites) != 1 {
		t.Fatalf("事件 = %+v", evs)
	}
	if n := f.count("procurement.rfq_publication_invites"); n != 1 {
		t.Fatalf("一個受眾一列邀請，得到 %d 列", n)
	}
	if n := f.count("procurement.agent_runs WHERE run_type <> 'CANDIDATE_SEARCH'"); n != 0 {
		t.Fatalf("發布不建 run，得到 %d 列", n)
	}
}

func TestExecuteDirectWritesOneQueuedInvitePerRecipientInOrderAndTheEventCarriesThem(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20, 30)
	p := f.approved(f.directIn(set, c[2], c[0], c[1]))
	pub, err := f.execute(f.executeIn(p))
	must(t, err)
	invites, err := store.New(f.d.Conn(context.Background())).ListInvites(context.Background(), store.ListInvitesParams{PublicationID: pub.ID, BuyerID: &f.buyer})
	must(t, err)
	if len(invites) != 3 {
		t.Fatalf("邀請 %d 列", len(invites))
	}
	for i, want := range []int64{30, 10, 20} {
		if invites[i].StoreID != want || invites[i].Position != int32(i) || invites[i].DeliveryState != "QUEUED" {
			t.Fatalf("邀請 %d = %+v", i, invites[i])
		}
	}
	evs := f.events()
	if len(evs) != 1 || len(evs[0].Invites) != 3 {
		t.Fatalf("事件 = %+v", evs)
	}
	for i, inv := range evs[0].Invites {
		if inv.InviteID != invites[i].ID || inv.StoreID != invites[i].StoreID {
			t.Fatalf("事件的邀請 %d = %+v，資料庫 %+v", i, inv, invites[i])
		}
	}
}

func TestExecuteReplaysTheFirstPublicationWithoutNewInvitesOrEvents(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	in := f.executeIn(p)
	first, err := f.execute(in)
	must(t, err)
	f.now = f.now.Add(rfq.ApprovalTTL + time.Hour) // 核准已過期，重送仍回第一次的發布
	again, err := f.execute(in)
	must(t, err)
	if !again.Replayed || again.ID != first.ID || again.SellerFacingHash != first.SellerFacingHash || again.AudienceHash != first.AudienceHash || !again.PublishedAt.Equal(first.PublishedAt) {
		t.Fatalf("重送 = %+v，第一次 %+v", again, first)
	}
	if n, m, e := f.count("procurement.rfq_publications"), f.count("procurement.rfq_publication_invites"), len(f.events()); n != 1 || m != 2 || e != 1 {
		t.Fatalf("發布 %d、邀請 %d、事件 %d（要 1、2、1）", n, m, e)
	}
}

func TestExecuteRejectsTheSameKeyWithDifferentContent(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	in := f.executeIn(p)
	_, err := f.execute(in)
	must(t, err)
	next := f.approved(f.publicationIn())
	for name, edit := range map[string]func(*rfq.ExecuteInput){
		"approval": func(i *rfq.ExecuteInput) {
			i.ApprovalID, i.ApprovalVersion, i.PayloadHash = next.ApprovalID, next.ApprovalVersion, next.SellerFacingHash
		},
		"payload":  func(i *rfq.ExecuteInput) { i.PayloadHash = "x" },
		"revision": func(i *rfq.ExecuteInput) { i.ExpectedRevisionID = uuid.NewV7() },
		"request":  func(i *rfq.ExecuteInput) { i.RequestID = uuid.NewV7() },
	} {
		changed := in
		edit(&changed)
		_, err := f.execute(changed)
		if err == nil {
			t.Errorf("同一個鍵、不同的%s要回錯誤", name)
			continue
		}
		wantInvalid(t, err, "client_request_id")
	}
	if n, e := f.count("procurement.rfq_publications"), len(f.events()); n != 1 || e != 1 {
		t.Fatalf("發布 %d、事件 %d（要 1、1）", n, e)
	}
	if state, _ := f.approvalState(next.ApprovalID); state != "PENDING" {
		t.Fatalf("另一份核准不受影響，卻是 %s", state)
	}
}

func TestExecuteAnApprovalThatWasAlreadyExecutedIsAVersionConflict(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	_, err := f.execute(f.executeIn(p))
	must(t, err)
	_, err = f.execute(f.executeIn(p)) // 新的 client_request_id，同一份核准
	wantConflict(t, err, "publication_approval")
	if n, e := f.count("procurement.rfq_publications"), len(f.events()); n != 1 || e != 1 {
		t.Fatalf("發布 %d、事件 %d（要 1、1）", n, e)
	}
}

func TestExecuteAnExpiredApprovalIsAVersionConflictAndLeavesItPending(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	f.now = f.now.Add(rfq.ApprovalTTL + time.Second)
	_, err := f.execute(f.executeIn(p))
	wantConflict(t, err, "publication_approval")
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("被拒的請求不改核准，卻是 %s v%d", state, version)
	}
	if n, e := f.count("procurement.rfq_publications"), len(f.events()); n != 0 || e != 0 {
		t.Fatalf("發布 %d、事件 %d（要 0、0）", n, e)
	}
}

func TestExecuteStaleBranchesLeaveTheApprovalPending(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	pending := func(why string) {
		t.Helper()
		if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
			t.Fatalf("%s：被拒的請求不改核准，卻是 %s v%d", why, state, version)
		}
		if n, e := f.count("procurement.rfq_publications"), len(f.events()); n != 0 || e != 0 {
			t.Fatalf("%s：發布 %d、事件 %d（要 0、0）", why, n, e)
		}
	}

	stale := f.executeIn(p)
	stale.ExpectedRevisionID = uuid.NewV7()
	_, err := f.execute(stale)
	wantConflict(t, err, "request_revision")
	pending("需求版本")

	version := f.executeIn(p)
	version.ApprovalVersion = 2
	_, err = f.execute(version)
	wantConflict(t, err, "publication_approval")
	pending("核准版本")

	hash := f.executeIn(p)
	hash.PayloadHash = p.SellerFacingHash[:63] + "0"
	if hash.PayloadHash == p.SellerFacingHash {
		hash.PayloadHash = p.SellerFacingHash[:63] + "1"
	}
	_, err = f.execute(hash)
	wantConflict(t, err, "publication_approval")
	pending("payload hash")

	missing := f.executeIn(p)
	missing.ApprovalID = uuid.NewV7()
	_, err = f.execute(missing)
	wantKind(t, err, apperr.KindNotFound)

	foreign := f.sibling(buyer + 1)
	theirs := foreign.approved(foreign.publicationIn())
	notMine := f.executeIn(theirs)
	_, err = f.service().ExecutePublication(ctx, notMine)
	wantKind(t, err, apperr.KindNotFound)
	if state, _ := foreign.approvalState(theirs.ApprovalID); state != "PENDING" {
		t.Fatalf("別人的核准不受影響，卻是 %s", state)
	}

	again := f.sibling(buyer)
	crossed := again.approved(again.publicationIn())
	other := f.executeIn(crossed) // 同一位買家另一張需求的核准
	_, err = f.execute(other)
	wantConflict(t, err, "publication_approval")
	if state, _ := again.approvalState(crossed.ApprovalID); state != "PENDING" {
		t.Fatalf("另一張需求的核准不受影響，卻是 %s", state)
	}

	f.revise(func(*store.InsertRevisionParams) {}) // 核准之後需求又改了一版：核准凍的版本不是現行版本
	_, err = f.execute(f.executeIn(p))
	wantConflict(t, err, "publication_approval")
	pending("核准凍的版本")
}

// 版本對得上但狀態不是 PENDING 的核准不能領走。只靠 version 擋的實作（拿掉 state = 'PENDING'）會讓這個測試變紅。
func TestExecuteRejectsAnApprovalWhoseVersionMatchesButIsNotPending(t *testing.T) {
	for _, state := range []string{"EXECUTING", "SUCCEEDED"} {
		f := newFixture(t)
		p := f.approved(f.publicationIn())
		f.exec(`UPDATE procurement.rfq_publication_approvals SET state = $2 WHERE id = $1`, p.ApprovalID, state) // 版本不變
		_, err := f.execute(f.executeIn(p))
		wantConflict(t, err, "publication_approval")
		if got, version := f.approvalState(p.ApprovalID); got != state || version != 1 {
			t.Fatalf("%s：被拒的請求不改核准，卻是 %s v%d", state, got, version)
		}
		if n, e := f.count("procurement.rfq_publications"), len(f.events()); n != 0 || e != 0 {
			t.Fatalf("%s：發布 %d、事件 %d（要 0、0）", state, n, e)
		}
	}
}

// 狀態對（PENDING）但版本不是預覽回的那個：只靠 state 擋的實作（拿掉 version = $版本）會讓這個測試變紅。
func TestExecuteRejectsAnApprovalVersionThatIsNotTheOnePreviewReturned(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	in := f.executeIn(p)
	in.ApprovalVersion = p.ApprovalVersion + 1
	_, err := f.execute(in)
	wantConflict(t, err, "publication_approval")
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("被拒的請求不改核准，卻是 %s v%d", state, version)
	}
	if n := f.count("procurement.rfq_publications"); n != 0 {
		t.Fatalf("發布 %d（要 0）", n)
	}
}

// 核准到 expires_at 的那一刻起就過期（expires_at 之前有效、等於 expires_at 已過期）。
func TestExecuteApprovalExpiryBoundary(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	f.now = p.ExpiresAt
	_, err := f.execute(f.executeIn(p))
	wantConflict(t, err, "publication_approval")
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("到期當下被拒，核准不變，卻是 %s v%d", state, version)
	}
	f.now = p.ExpiresAt.Add(-time.Microsecond)
	if _, err := f.execute(f.executeIn(p)); err != nil {
		t.Fatalf("到期前一刻要能執行：%v", err)
	}
}

// 需求狀態改變不產生新版本，所以 Execute 要再查一次；被拒時核准維持 PENDING（領取一起還原）。
func TestExecuteRequiresAReadyOrMatchingRequest(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	f.setStatus("CLARIFYING")
	_, err := f.execute(f.executeIn(p))
	wantKind(t, err, apperr.KindFailedPrecondition)
	wantReason(t, err, apperr.ReasonRequirementNotReady, true)
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("被拒的請求不改核准，卻是 %s v%d", state, version)
	}
	if n, e := f.count("procurement.rfq_publications"), len(f.events()); n != 0 || e != 0 {
		t.Fatalf("發布 %d、事件 %d（要 0、0）", n, e)
	}
	f.setStatus("MATCHING")
	if _, err := f.execute(f.executeIn(p)); err != nil {
		t.Fatalf("MATCHING 要能執行：%v", err)
	}
}

// 指定對象的核准存候選集，執行時重驗它仍是有效的候選集；預覽之後同一版本又搜尋成功就失效。
func TestExecuteDirectRechecksTheEffectiveCandidateSet(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	f.searched(10, 20) // 預覽之後又搜尋成功：舊的候選集不再有效
	_, err := f.execute(f.executeIn(p))
	wantConflict(t, err, "candidate_set")
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("被拒的請求不改核准（領取要一起還原），卻是 %s v%d", state, version)
	}
	if n, m, e := f.count("procurement.rfq_publications"), f.count("procurement.rfq_publication_invites"), len(f.events()); n != 0 || m != 0 || e != 0 {
		t.Fatalf("發布 %d、邀請 %d、事件 %d（要 0、0、0）", n, m, e)
	}
}

func TestExecuteDirectStillWorksWhileTheCandidateSetIsEffective(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	row, err := store.New(f.d.Conn(context.Background())).GetApproval(context.Background(), store.GetApprovalParams{ID: p.ApprovalID, RequestID: f.request, BuyerID: &f.buyer})
	must(t, err)
	if row.CandidateSetID == nil || *row.CandidateSetID != set {
		t.Fatalf("核准要存候選集：%+v", row.CandidateSetID)
	}
	if _, err := f.execute(f.executeIn(p)); err != nil {
		t.Fatalf("候選集仍有效要能執行：%v", err)
	}
}

// 回價期限在 Execute 時再查一次：預覽時是今天、執行時台北已是隔天，核准不能執行，也不改核准。
func TestExecuteRechecksTheResponseDeadlineInTaipei(t *testing.T) {
	f := newFixture(t)
	in := f.publicationIn()
	today := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	in.ResponseDeadline = &today
	p := f.approved(in)
	f.now = f.now.Add(23 * time.Hour) // 核准還沒過期（24 小時），但台北已是 11/24
	_, err := f.execute(f.executeIn(p))
	wantConflict(t, err, "publication_approval")
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("被拒的請求不改核准，卻是 %s v%d", state, version)
	}
	if n := f.count("procurement.rfq_publications"); n != 0 {
		t.Fatalf("發布 %d（要 0）", n)
	}
}

// Execute 重送比對 approval_version；版本不同回 INVALID_ARGUMENT，不回放。
func TestExecuteReplayComparesTheApprovalVersion(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn())
	in := f.executeIn(p)
	first, err := f.execute(in)
	must(t, err)
	same, err := f.execute(in)
	must(t, err)
	if !same.Replayed || same.ID != first.ID {
		t.Fatalf("同樣的版本要回放：%+v", same)
	}
	changed := in
	changed.ApprovalVersion = in.ApprovalVersion + 1
	_, err = f.execute(changed)
	wantInvalid(t, err, "client_request_id")
}

// 原子性：事件訂閱者失敗時，發布、邀請與核准的 PENDING→EXECUTING 一起還原，同一份核准與同一把鍵可以重試成功。
func TestExecuteIsAtomicWhenTheEventSubscriberFails(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	in := f.executeIn(p)
	f.subscriberErr = errors.New("subscriber down")
	if _, err := f.execute(in); err == nil {
		t.Fatal("訂閱者失敗，整筆要失敗")
	}
	if n, m := f.count("procurement.rfq_publications"), f.count("procurement.rfq_publication_invites"); n != 0 || m != 0 {
		t.Fatalf("發布 %d、邀請 %d 要一起還原", n, m)
	}
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("核准要還原成 PENDING v1，卻是 %s v%d", state, version)
	}
	f.subscriberErr = nil
	pub, err := f.execute(in)
	must(t, err)
	if pub.Replayed || f.count("procurement.rfq_publications") != 1 || f.count("procurement.rfq_publication_invites") != 2 || len(f.events()) != 1 {
		t.Fatalf("重試要成功一次：%+v", pub)
	}
}

// 原子性：第二個邀請寫入失敗時（第一個已寫入），發布、第一個邀請與核准一起還原，也沒有事件。
func TestExecuteIsAtomicWhenAnInviteInsertFails(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	f.exec(`CREATE FUNCTION procurement.fail_second_invite() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.position = 1 THEN RAISE EXCEPTION 'injected invite failure'; END IF; RETURN NEW; END $$`)
	f.exec(`CREATE TRIGGER fail_second_invite BEFORE INSERT ON procurement.rfq_publication_invites FOR EACH ROW EXECUTE FUNCTION procurement.fail_second_invite()`)
	in := f.executeIn(p)
	if _, err := f.execute(in); err == nil {
		t.Fatal("邀請寫入失敗，整筆要失敗")
	}
	if n, m := f.count("procurement.rfq_publications"), f.count("procurement.rfq_publication_invites"); n != 0 || m != 0 {
		t.Fatalf("發布 %d、邀請 %d 要一起還原", n, m)
	}
	if state, version := f.approvalState(p.ApprovalID); state != "PENDING" || version != 1 {
		t.Fatalf("核准要還原成 PENDING v1，卻是 %s v%d", state, version)
	}
	if e := f.events(); len(e) != 0 {
		t.Fatalf("事件 %d 則（要 0）", len(e))
	}
	f.exec(`DROP TRIGGER fail_second_invite ON procurement.rfq_publication_invites`)
	if _, err := f.execute(in); err != nil {
		t.Fatalf("移除注入之後同一份核准要能執行：%v", err)
	}
}

// 併發重送：同一把鍵同時兩次，只建一次發布與事件，另一個回放。
func TestExecuteConcurrentRetriesCreateOnePublicationAndOneEvent(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	in := f.executeIn(p)
	results := make([]model.Publication, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = f.execute(in)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("第 %d 個請求：%v", i, err)
		}
	}
	if results[0].ID != results[1].ID || results[0].Replayed == results[1].Replayed {
		t.Fatalf("要一個建立、一個回放：%+v %+v", results[0], results[1])
	}
	if n, m, e := f.count("procurement.rfq_publications"), f.count("procurement.rfq_publication_invites"), len(f.events()); n != 1 || m != 2 || e != 1 {
		t.Fatalf("發布 %d、邀請 %d、事件 %d（要 1、2、1）", n, m, e)
	}
}
