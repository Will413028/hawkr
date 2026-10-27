package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

type fakeSelectionStore struct {
	request      repository.Request
	candidateSet repository.CandidateSet
	candidate    repository.Candidate
	candidateErr error
	upsertCalled bool
	upsertInput  repository.UpsertSelectionInput
	// upsertInputs 保留每一次呼叫，讓「勾選再取消」這種多步情境能斷言兩次都打到同一個
	// conflict key（單一 upsertInput 只留得住最後一次）。
	upsertInputs []repository.UpsertSelectionInput
}

func (f *fakeSelectionStore) GetSnapshot(context.Context, int64, identity.BuyerID) (repository.Request, repository.Revision, error) {
	return f.request, repository.Revision{}, nil
}
func (f *fakeSelectionStore) GetCandidateSetWithCandidates(context.Context, int64, int64, identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	return f.candidateSet, nil, nil
}
func (f *fakeSelectionStore) GetCandidate(context.Context, int64, int64) (repository.Candidate, error) {
	return f.candidate, f.candidateErr
}
func (f *fakeSelectionStore) UpsertSelection(_ context.Context, in repository.UpsertSelectionInput) (repository.CandidateSelection, error) {
	f.upsertCalled = true
	f.upsertInput = in
	f.upsertInputs = append(f.upsertInputs, in)
	return repository.CandidateSelection{ID: 1, Selected: in.Selected}, nil
}

func TestToggle_SucceedsWhenCandidateSetMatchesActiveRevision(t *testing.T) {
	store := &fakeSelectionStore{
		request:      repository.Request{ID: 5, ActiveRevisionID: 9},
		candidateSet: repository.CandidateSet{ID: 100, RequestRevisionID: 9},
		candidate:    repository.Candidate{ID: 42, CandidateSetID: 100, Lane: "PLATFORM"},
	}
	uc := NewSelectionUseCase(store)

	res, err := uc.Toggle(context.Background(), SelectionInput{
		RequestID: 5, BuyerID: 1, CandidateSetID: 100, Lane: "PLATFORM", CandidateID: 42, Selected: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Selected {
		t.Error("Selected = false, want true")
	}
	if !store.upsertCalled {
		t.Fatal("UpsertSelection was never called")
	}
	if store.upsertInput.RequestRevisionID != 9 {
		t.Errorf("RequestRevisionID = %d, want 9 (from active revision)", store.upsertInput.RequestRevisionID)
	}
	if store.upsertInput.Lane != "PLATFORM" {
		t.Errorf("Lane = %q, want PLATFORM", store.upsertInput.Lane)
	}
}

// TestToggle_RejectsMismatchedLane 守住「client 送錯 lane 要被明確告知」。
//
// 這條規則跟「lane 以候選自身為準」是兩層不同的防線，不要混為一談：
//   - 資料層：寫進 UpsertSelection 的 lane 一律取自 GetCandidate 的權威值，所以就算這道
//     檢查哪天被拿掉，client 也不可能靠謊報 lane 把勾選／取消打散到兩個 conflict key
//     上（那是原本的 bug：API 回 200 {"selected": false}，原本那筆勾選卻還活著）。
//   - 契約層（這個測試）：無害不等於該沉默。client 送錯 lane 代表它自己有 bug——可能正
//     拿著錯的候選卡渲染 UI——靜默接受等於讓那個 bug 永遠不會浮出水面。
//
// 因此這裡同時斷言「回 ErrLaneMismatch」與「UpsertSelection 完全沒被呼叫」：只驗前者的話，
// 一個在 upsert 之後才回錯誤的實作也會通過，那就變成「寫都寫了才抱怨」。
func TestToggle_RejectsMismatchedLane(t *testing.T) {
	store := &fakeSelectionStore{
		request:      repository.Request{ID: 5, ActiveRevisionID: 9},
		candidateSet: repository.CandidateSet{ID: 100, RequestRevisionID: 9},
		candidate:    repository.Candidate{ID: 42, CandidateSetID: 100, Lane: "PLATFORM"},
	}
	uc := NewSelectionUseCase(store)

	_, err := uc.Toggle(context.Background(), SelectionInput{
		// client 謊報 lane（或單純送錯）——候選真正的 lane 是 PLATFORM。
		RequestID: 5, BuyerID: 1, CandidateSetID: 100, Lane: "EXTERNAL", CandidateID: 42, Selected: true,
	})
	if !errors.Is(err, ErrLaneMismatch) {
		t.Errorf("err = %v, want ErrLaneMismatch", err)
	}
	if store.upsertCalled {
		t.Error("lane 不符時不得寫入任何東西——UpsertSelection 不該被呼叫")
	}
}

// TestToggle_SelectThenUnselectTargetsTheSameRow 驗證「勾選再取消」兩次呼叫落在同一個
// conflict key (request_revision_id, lane, candidate_id) 上——這正是 DB 端會不會撞到
// 同一列的充分條件（UpsertSelectionInput 在單元測試裡不會真的碰到 DB）。
//
// 這個測試原本用「第二次送錯的 lane」來製造分裂（batch 1 的版本）；lane 不符現在會被
// ErrLaneMismatch 直接擋在 upsert 之前，那個輸入已經到不了這裡。改用兩次都送對的 lane，
// 守的仍是同一條不變式：寫入用的 lane 來自候選本身，所以兩次必然一致。
func TestToggle_SelectThenUnselectTargetsTheSameRow(t *testing.T) {
	store := &fakeSelectionStore{
		request:      repository.Request{ID: 5, ActiveRevisionID: 9},
		candidateSet: repository.CandidateSet{ID: 100, RequestRevisionID: 9},
		candidate:    repository.Candidate{ID: 42, CandidateSetID: 100, Lane: "PLATFORM"},
	}
	uc := NewSelectionUseCase(store)

	if _, err := uc.Toggle(context.Background(), SelectionInput{
		RequestID: 5, BuyerID: 1, CandidateSetID: 100, Lane: "PLATFORM", CandidateID: 42, Selected: true,
	}); err != nil {
		t.Fatalf("select: unexpected error: %v", err)
	}

	res, err := uc.Toggle(context.Background(), SelectionInput{
		RequestID: 5, BuyerID: 1, CandidateSetID: 100, Lane: "PLATFORM", CandidateID: 42, Selected: false,
	})
	if err != nil {
		t.Fatalf("unselect: unexpected error: %v", err)
	}

	if len(store.upsertInputs) != 2 {
		t.Fatalf("UpsertSelection called %d times, want 2", len(store.upsertInputs))
	}
	first, second := store.upsertInputs[0], store.upsertInputs[1]
	if first.RequestRevisionID != second.RequestRevisionID ||
		first.Lane != second.Lane || first.CandidateID != second.CandidateID {
		t.Errorf("conflict key 不一致：勾選用 (%d, %q, %d)、取消用 (%d, %q, %d)——取消會插入第二列而不是更新原本那列",
			first.RequestRevisionID, first.Lane, first.CandidateID,
			second.RequestRevisionID, second.Lane, second.CandidateID)
	}
	if second.Lane != "PLATFORM" {
		t.Errorf("取消時送出的 lane = %q, want PLATFORM", second.Lane)
	}
	if second.Selected {
		t.Error("第二次呼叫應該是取消勾選")
	}
	if res.Lane != "PLATFORM" {
		t.Errorf("SelectionResult.Lane = %q, want PLATFORM", res.Lane)
	}
}

func TestToggle_RejectsWhenCandidateSetIsStale(t *testing.T) {
	store := &fakeSelectionStore{
		request:      repository.Request{ID: 5, ActiveRevisionID: 10},        // 買家已經改過需求
		candidateSet: repository.CandidateSet{ID: 100, RequestRevisionID: 9}, // 候選還是舊版本算出來的
	}
	uc := NewSelectionUseCase(store)

	_, err := uc.Toggle(context.Background(), SelectionInput{
		RequestID: 5, BuyerID: 1, CandidateSetID: 100, Lane: "PLATFORM", CandidateID: 42, Selected: true,
	})
	if !errors.Is(err, ErrCandidateFactsStale) {
		t.Errorf("err = %v, want ErrCandidateFactsStale", err)
	}
}

func TestToggle_RejectsWhenCandidateNotInSet(t *testing.T) {
	store := &fakeSelectionStore{
		request:      repository.Request{ID: 5, ActiveRevisionID: 9},
		candidateSet: repository.CandidateSet{ID: 100, RequestRevisionID: 9},
		candidateErr: repository.ErrNotFound,
	}
	uc := NewSelectionUseCase(store)

	_, err := uc.Toggle(context.Background(), SelectionInput{
		RequestID: 5, BuyerID: 1, CandidateSetID: 100, Lane: "PLATFORM", CandidateID: 999, Selected: true,
	})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
