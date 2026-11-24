//go:build integration

package rfq_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestPreviewRejectsAStoreThatBlocksTheBuyerOrIsNotOpen(t *testing.T) {
	for name, set := range map[string]func(*fixture){
		"封鎖這位買家": func(f *fixture) { f.stores.set(map[int64]bool{10: true}, nil, nil) },
		"不是營業中":  func(f *fixture) { f.stores.set(nil, map[int64]bool{10: true}, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			in := f.publicationIn()
			set(f)
			_, err := f.service().PreviewPublication(context.Background(), in)
			wantConflict(t, err, rfq.TargetCandidateSet)
			if n := f.count("procurement.rfq_publication_approvals"); n != 0 {
				t.Fatalf("被擋下的預覽不該留下核准：%d", n)
			}
		})
	}
}

func TestPreviewRejectsWhenOnlyOneSelectedStoreIsBlocked(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20, 30)
	in := f.directIn(set, c[0], c[1], c[2])
	f.stores.set(map[int64]bool{20: true}, nil, nil)
	_, err := f.service().PreviewPublication(context.Background(), in)
	wantConflict(t, err, rfq.TargetCandidateSet)
}

func TestPreviewIgnoresABlockingStoreThatWasNotSelected(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	f.stores.set(map[int64]bool{20: true}, nil, nil)
	f.approved(f.directIn(set, c[0]))
}

func TestExecuteRejectsAStoreThatBlocksTheBuyerOrClosedSinceThePreview(t *testing.T) {
	for name, set := range map[string]func(*fixture){
		"封鎖這位買家": func(f *fixture) { f.stores.set(map[int64]bool{10: true}, nil, nil) },
		"不是營業中":  func(f *fixture) { f.stores.set(nil, map[int64]bool{10: true}, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			p := f.approved(f.publicationIn())
			set(f)
			_, err := f.execute(f.executeIn(p))
			wantConflict(t, err, rfq.TargetCandidateSet)
			if state, _ := f.approvalState(p.ApprovalID); state != "PENDING" {
				t.Fatalf("被擋下的執行要還原核准的領取：%s", state)
			}
			if f.count("procurement.rfq_publications") != 0 || len(f.events()) != 0 {
				t.Fatal("被擋下的執行不該留下發布或事件")
			}
			f.stores.set(nil, nil, nil)
			if _, err := f.execute(f.executeIn(p)); err != nil {
				t.Fatalf("店家恢復後同一份核准仍可執行：%v", err)
			}
		})
	}
}

func TestStoreCheckFailsClosedWhenThePortFails(t *testing.T) {
	f := newFixture(t)
	in := f.publicationIn()
	p := f.approved(in)
	f.stores.set(nil, nil, errors.New("seller 無法回應"))

	_, err := f.service().PreviewPublication(context.Background(), f.directIn(f.lastSet, f.lastCands[0]))
	if err == nil || apperr.KindOf(err) == apperr.KindVersionConflict {
		t.Fatalf("預覽：讀取失敗要回一般錯誤，得到 %v", err)
	}
	_, err = f.execute(f.executeIn(p))
	if err == nil || apperr.KindOf(err) == apperr.KindVersionConflict {
		t.Fatalf("執行：讀取失敗要回一般錯誤，得到 %v", err)
	}
	if f.count("procurement.rfq_publications") != 0 || len(f.events()) != 0 {
		t.Fatal("讀取失敗不得發布")
	}
}

func TestStoreConflictMessageDoesNotRevealWhichStore(t *testing.T) {
	f := newFixture(t)
	set, c := f.searched(10, 20)
	p := f.approved(f.directIn(set, c[0], c[1]))
	f.stores.set(map[int64]bool{20: true}, map[int64]bool{10: true}, nil)

	_, previewErr := f.service().PreviewPublication(context.Background(), f.directIn(set, c[0], c[1]))
	_, executeErr := f.execute(f.executeIn(p))
	for _, err := range []error{previewErr, executeErr} {
		wantConflict(t, err, rfq.TargetCandidateSet)
		var ae *apperr.Error
		if !errors.As(err, &ae) {
			t.Fatal(err)
		}
		msg := err.Error()
		for _, leak := range []string{"10", "20", "封鎖", "營業", "店"} {
			if strings.Contains(msg, leak) {
				t.Fatalf("訊息不得透露 %q：%s", leak, msg)
			}
		}
		for k, v := range ae.Metadata {
			if k != "conflictTarget" {
				t.Fatalf("metadata 只該有 conflictTarget，多了 %s=%v", k, v)
			}
		}
	}
}
