package usecase

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var ErrCandidateFactsStale = errors.New("procurement: candidate set no longer matches the request's active revision")

// ErrLaneMismatch 代表 client 送來的 lane 與候選實際所屬的 lane 不符。
//
// Toggle 本來就不採用 in.Lane（權威值一律取自 GetCandidate 回來的候選，理由見 Toggle
// 內的註解），所以「不符」在資料正確性上已經無害。但無害不等於該沉默：client 送錯 lane
// 是它自己的 bug——它可能正拿著錯的候選卡渲染 UI、或把兩條 lane 的 id 空間搞混了——
// 靜默接受等於讓那個 bug 永遠不會被發現，而 API 還回 200 告訴它一切正常。
// 這裡選擇回報而不是修正，跟 handler 對空 lane 直接 400 是同一個立場。
var ErrLaneMismatch = errors.New("procurement: lane does not match the candidate's actual lane")

// SelectionStore 是勾選功能需要的持久層能力。GetSnapshot 沿用既有簽名
// （與 handler.SnapshotReader／usecase.RevisionStore 的 GetSnapshot 結構相同，
// *repository.Repo 已經實作，不需要額外轉接）。
type SelectionStore interface {
	GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
	GetCandidateSetWithCandidates(ctx context.Context, candidateSetID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error)
	GetCandidate(ctx context.Context, candidateSetID, candidateID int64) (repository.Candidate, error)
	UpsertSelection(ctx context.Context, in repository.UpsertSelectionInput) (repository.CandidateSelection, error)
}

type SelectionUseCase struct {
	store SelectionStore
}

func NewSelectionUseCase(store SelectionStore) *SelectionUseCase {
	return &SelectionUseCase{store: store}
}

type SelectionInput struct {
	RequestID      int64
	BuyerID        identity.BuyerID
	CandidateSetID int64
	// Lane 是 client 在 request body 帶進來的值。它「不」被拿去寫任何東西——寫入用的
	// lane 一律以 GetCandidate 讀回來的候選自身為準（理由見 Toggle 內的註解）——但它會
	// 被拿去跟權威值比對，不符就回 ErrLaneMismatch。換句話說：這個欄位是一個 client
	// 自我宣告的斷言，不是資料來源。
	Lane        string
	CandidateID int64
	Selected    bool
}

type SelectionResult struct {
	CandidateSetID int64
	Lane           string
	CandidateID    int64
	Selected       bool
}

// Toggle 勾選／取消勾選一個候選。候選新鮮度用 candidate_set.request_revision_id 是否仍等於
// request 目前的 active_revision_id 判斷（見 candidate-matching-a3 計畫 Design Decisions #2）——
// 買家送出新的 turn 就會產生新 revision，讓舊 candidate_set 自動變成 stale，不需要額外版本欄位。
func (uc *SelectionUseCase) Toggle(ctx context.Context, in SelectionInput) (SelectionResult, error) {
	req, _, err := uc.store.GetSnapshot(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return SelectionResult{}, err
	}

	set, _, err := uc.store.GetCandidateSetWithCandidates(ctx, in.CandidateSetID, in.RequestID, in.BuyerID)
	if err != nil {
		return SelectionResult{}, err
	}
	if set.RequestRevisionID != req.ActiveRevisionID {
		return SelectionResult{}, ErrCandidateFactsStale
	}

	// lane 一律以候選自己的 lane 為準，完全不採用 in.Lane。這次讀取本來就要做（驗證候選
	// 確實屬於這個 candidate_set），順手就拿到了權威值。
	//
	// 為什麼不能信 in.Lane：它是 request body 直接帶進來的字串，handler 只檢查非空、沒有
	// 對 PLATFORM/EXTERNAL 做 enum 驗證；而 (request_revision_id, lane, candidate_id) 正是
	// UpsertSelection 的 ON CONFLICT 目標鍵。client 若用 A lane 勾選、再用 B lane 取消，
	// 撞不到同一列，DB 會靜默插入第二列——API 回 200 {"selected": false}，原本那筆勾選卻
	// 還活著。用候選自己的 lane 之後，同一個 candidate_id 永遠映到同一個 conflict key，
	// 勾選／取消一定作用在同一列。
	cand, err := uc.store.GetCandidate(ctx, in.CandidateSetID, in.CandidateID)
	if err != nil {
		return SelectionResult{}, err
	}
	// 拿到權威值之後，順便告訴 client 它送錯了。上面那段講的是「為什麼不能信 in.Lane」，
	// 這一句講的是「不信它之後要怎麼處置」：不是默默忽略（那樣 client 的 bug 永遠不會浮
	// 出水面，API 還回 200 說一切正常），而是回一個明確的錯誤。寫入用的仍然是 cand.Lane，
	// 不是 in.Lane——這道檢查若哪天被拿掉，資料層的防線還在（defense in depth）。
	if in.Lane != cand.Lane {
		return SelectionResult{}, ErrLaneMismatch
	}

	sel, err := uc.store.UpsertSelection(ctx, repository.UpsertSelectionInput{
		RequestID: in.RequestID, RequestRevisionID: req.ActiveRevisionID, BuyerID: in.BuyerID,
		CandidateSetID: in.CandidateSetID, Lane: cand.Lane, CandidateID: in.CandidateID, Selected: in.Selected,
	})
	if err != nil {
		return SelectionResult{}, err
	}

	return SelectionResult{
		CandidateSetID: in.CandidateSetID, Lane: cand.Lane, CandidateID: in.CandidateID, Selected: sel.Selected,
	}, nil
}
