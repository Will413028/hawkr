//go:build integration

package rfq_test

import (
	"context"
	"errors"
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

func TestPreviewPublicationWishPoolHasNoRecipientsAndTheHashesAreRecomputable(t *testing.T) {
	f := newFixture(t)
	snap := f.confirmed()
	in := f.publicationIn(snap)
	in.ResponseDeadline = &deadline
	p := f.approved(in)
	if p.RecipientCount != 0 || p.PolicyID == nil || *p.PolicyID != rfq.WishPoolPolicyID || p.PolicyVersion == nil || *p.PolicyVersion != rfq.WishPoolPolicyVersion {
		t.Fatalf("公開池：%+v", p)
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
	other := f.approved(f.publicationIn(snap)) // 另一把鍵、沒有期限：內容不同，hash 不同
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
	snap := f.confirmed()
	set, c := f.searched(10, 20, 30)
	forward := f.approved(f.directIn(snap, set, c[0], c[2]))
	reverse := f.approved(f.directIn(snap, set, c[2], c[0]))
	if forward.RecipientCount != 2 || forward.PolicyID != nil || forward.PolicyVersion != nil {
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
	snap := f.confirmed()
	oldSet, oldCands := f.searched(10, 20)
	newSet, newCands := f.searched(10, 20)

	_, err := f.service().PreviewPublication(ctx, f.directIn(snap, oldSet, oldCands[0]))
	wantConflict(t, err, "candidate_set") // 有更新的搜尋成功
	if _, err := f.service().PreviewPublication(ctx, f.directIn(snap, newSet, newCands[0])); err != nil {
		t.Fatalf("有效的候選集要收：%v", err)
	}

	f.exec(`UPDATE procurement.requests SET status = 'CLARIFYING' WHERE id = $1`, f.request)
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, newSet, newCands[1]))
	wantConflict(t, err, "candidate_set") // 需求已離開 MATCHING
	if n := f.count("procurement.rfq_publication_approvals"); n != 1 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationDirectBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	snap := f.confirmed()
	set, c := f.searched(10, 20)
	other := f.sibling(buyer)
	otherSet, otherCands := other.searched(10)

	_, err := f.service().PreviewPublication(ctx, f.directIn(snap, set, otherCands[0]))
	wantInvalid(t, err, "SelectedCandidateIDs[0]") // 候選不在這個候選集
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, set, c[0], uuid.NewV7()))
	wantInvalid(t, err, "SelectedCandidateIDs[1]")
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, uuid.NewV7(), c[0]))
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, otherSet, otherCands[0])) // 同一位買家另一張需求的候選集
	wantKind(t, err, apperr.KindNotFound)
	foreign := f.sibling(buyer + 1)
	theirSet, theirCands := foreign.searched(10)
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, theirSet, theirCands[0])) // 別的買家的候選集
	wantKind(t, err, apperr.KindNotFound)
	if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationShapeErrorsPointAtTheField(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	snap := f.confirmed()
	set, c := f.searched(10, 20)

	in := f.publicationIn(snap)
	in.AudienceMode = ""
	_, err := f.service().PreviewPublication(ctx, in)
	wantInvalid(t, err, "AudienceMode")
	wish := f.publicationIn(snap)
	wish.CandidateSetID = &set
	_, err = f.service().PreviewPublication(ctx, wish)
	wantInvalid(t, err, "CandidateSetID")
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, set))
	wantInvalid(t, err, "SelectedCandidateIDs")
	_, err = f.service().PreviewPublication(ctx, f.directIn(snap, set, c[0], c[0]))
	wantInvalid(t, err, "SelectedCandidateIDs[1]")
	if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
		t.Fatalf("形狀錯誤不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationStaleBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	snap := f.confirmed()

	stale := f.publicationIn(snap)
	stale.ExpectedRevisionID = uuid.NewV7()
	_, err := f.service().PreviewPublication(ctx, stale)
	wantConflict(t, err, "request_revision")

	missing := f.publicationIn(snap)
	missing.SnapshotID = uuid.NewV7()
	_, err = f.service().PreviewPublication(ctx, missing)
	wantKind(t, err, apperr.KindNotFound)

	foreign := f.sibling(buyer + 1)
	notMine := f.publicationIn(snap)
	notMine.BuyerID, notMine.RequestID, notMine.ExpectedRevisionID = foreign.buyer, foreign.request, foreign.revision
	_, err = f.service().PreviewPublication(ctx, notMine)
	wantKind(t, err, apperr.KindNotFound)

	again := f.sibling(buyer)
	crossed := again.publicationIn(again.confirmed())
	crossed.SnapshotID = snap.ID // 同一位買家另一張需求的快照
	_, err = f.service().PreviewPublication(ctx, crossed)
	wantConflict(t, err, "rfq_snapshot")

	f.revise(func(*store.InsertRevisionParams) {}) // 確認之後需求又改了一版
	_, err = f.service().PreviewPublication(ctx, f.publicationIn(snap))
	wantConflict(t, err, "rfq_snapshot")
	if n := f.count("procurement.rfq_publication_approvals WHERE request_id = $1", f.request); n != 0 {
		t.Fatalf("被拒的請求不寫入，rfq_publication_approvals %d 列", n)
	}
}

func TestPreviewPublicationRejectsTheSameKeyWithDifferentContent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	snap := f.confirmed()
	set, c := f.searched(10, 20)
	direct := f.directIn(snap, set, c[0], c[1])
	deadlineIn := direct
	deadlineIn.ResponseDeadline = &deadline
	f.approved(direct)
	for name, edit := range map[string]func(*rfq.PreviewPublicationInput){
		"order":    func(i *rfq.PreviewPublicationInput) { i.SelectedCandidateIDs = []uuid.UUID{c[1], c[0]} },
		"fewer":    func(i *rfq.PreviewPublicationInput) { i.SelectedCandidateIDs = []uuid.UUID{c[0]} },
		"deadline": func(i *rfq.PreviewPublicationInput) { i.ResponseDeadline = &deadline },
		"mode": func(i *rfq.PreviewPublicationInput) {
			i.AudienceMode, i.CandidateSetID, i.SelectedCandidateIDs = model.AudienceWishPool, nil, nil
		},
		"snapshot": func(i *rfq.PreviewPublicationInput) { i.SnapshotID = uuid.NewV7() },
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

func TestExecuteWishPoolCreatesThePublicationOneEventAndNoRun(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn(f.confirmed()))
	pub, err := f.execute(f.executeIn(p))
	must(t, err)
	if pub.Replayed || pub.State != model.PublicationStatePublished || pub.AudienceMode != model.AudienceWishPool || pub.Version != 1 ||
		pub.AudienceHash != p.AudienceHash || pub.SellerFacingHash != p.SellerFacingHash || !pub.PublishedAt.Equal(f.now) {
		t.Fatalf("發布 = %+v，預覽 %+v", pub, p)
	}
	if state, version := f.approvalState(p.ApprovalID); state != "SUCCEEDED" || version != 3 {
		t.Fatalf("核准 %s v%d", state, version)
	}
	evs := f.events()
	if len(evs) != 1 || evs[0].PublicationID != pub.ID || evs[0].RequestID != f.request || evs[0].BuyerID != f.buyer || evs[0].ApprovalID != p.ApprovalID ||
		evs[0].AudienceMode != model.AudienceWishPool || len(evs[0].Invites) != 0 {
		t.Fatalf("事件 = %+v", evs)
	}
	if n := f.count("procurement.rfq_publication_invites"); n != 0 {
		t.Fatalf("公開池沒有邀請，得到 %d 列", n)
	}
	if n := f.count("procurement.agent_runs"); n != 0 {
		t.Fatalf("發布不建 run，得到 %d 列", n)
	}
}

func TestExecuteDirectWritesOneQueuedInvitePerRecipientInOrderAndTheEventCarriesThem(t *testing.T) {
	f := newFixture(t)
	snap := f.confirmed()
	set, c := f.searched(10, 20, 30)
	p := f.approved(f.directIn(snap, set, c[2], c[0], c[1]))
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
	snap := f.confirmed()
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(snap, set, c[0], c[1]))
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
	snap := f.confirmed()
	p := f.approved(f.publicationIn(snap))
	in := f.executeIn(p)
	_, err := f.execute(in)
	must(t, err)
	next := f.approved(f.publicationIn(snap))
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
	p := f.approved(f.publicationIn(f.confirmed()))
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
	p := f.approved(f.publicationIn(f.confirmed()))
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
	snap := f.confirmed()
	p := f.approved(f.publicationIn(snap))
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
	theirs := foreign.approved(foreign.publicationIn(foreign.confirmed()))
	notMine := f.executeIn(theirs)
	_, err = f.service().ExecutePublication(ctx, notMine)
	wantKind(t, err, apperr.KindNotFound)
	if state, _ := foreign.approvalState(theirs.ApprovalID); state != "PENDING" {
		t.Fatalf("別人的核准不受影響，卻是 %s", state)
	}

	again := f.sibling(buyer)
	crossed := again.approved(again.publicationIn(again.confirmed()))
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

func TestExecuteWithoutAnEventBusIsAnAssemblyError(t *testing.T) {
	f := newFixture(t)
	p := f.approved(f.publicationIn(f.confirmed()))
	svc := f.service()
	svc.Events = nil
	if _, err := svc.ExecutePublication(context.Background(), f.executeIn(p)); err == nil || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("要組裝錯誤，得到 %v", err)
	}
	if state, _ := f.approvalState(p.ApprovalID); state != "PENDING" {
		t.Fatalf("核准是 %s", state)
	}
}

// 原子性：事件訂閱者失敗時，發布、邀請與核准的 PENDING→EXECUTING 一起還原，同一份核准與同一把鍵可以重試成功。
func TestExecuteIsAtomicWhenTheEventSubscriberFails(t *testing.T) {
	f := newFixture(t)
	snap := f.confirmed()
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(snap, set, c[0], c[1]))
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
	snap := f.confirmed()
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(snap, set, c[0], c[1]))
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
	snap := f.confirmed()
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(snap, set, c[0], c[1]))
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
