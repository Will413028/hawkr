// Package handler 保留 procurement 域曾經的 HTTP 邊界留下的少數 transport-agnostic
// 契約（MaxMessageRunes／SnapshotReader／CandidateReader）。HTTP handler 本體（原
// Handler/New/PostTurn/GetSnapshot 等）已在 Phase C（移除 ai-service HTTP business
// surface，procurement 全面走 AIProcurementService gRPC 後 additive 雙 transport
// 收斂為單一 transport）刪除，見 wiki
// projects/backend-payuni/decisions/2026-08-08-ai-service-internal-grpc-unification.md。
// 這三個符號繼續留在這裡（沒有搬去別的 package）是因為 internal/grpc/procurement_handler.go
// 與 internal/procurement/module.go 仍在 import 使用，搬遷屬於另一輪範圍更大的重構，
// 不在本次純刪除範圍內。
package handler

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// MaxMessageRunes 與 usecase.MaxMessageRunes 同值，但這一層是「拒絕」而非「截斷」：
// 邊界要讓使用者知道超長，不是靜默砍掉再照做。Export 給 internal/grpc 的
// SubmitTurn／SubmitTurnStream 共用同一個定義，讓拒絕門檻不會各自維護一份而漂移。
const MaxMessageRunes = 2000

// SnapshotReader 刻意只暴露 GetSnapshot 這一個讀取能力，不是 *repository.Repo 全部。
// GetSnapshot 用一次查詢就同時交出 request 與它「當下確切指向」的那個 revision，
// 這個 pair 的自洽是 repository 那邊花了一整段註解論證的不變式；只要這一層還拿得到
// 第二個讀取入口（例如 GetSlots，它內部會再跑一次 GetSnapshot），就永遠有人能把
// 「revisionId 取自第一次讀取、slots 取自第二次讀取」這個不自洽組合寫回來。
// 收窄成單一方法後，那個組合連編譯都過不了。
type SnapshotReader interface {
	GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error)
	FindLatestPublicationForRequest(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.LatestPublicationForRequest, error)
}

// CandidateReader 是「寄送報價單」把 candidateId 清單翻譯成 lane/product_id 需要
// 的唯一讀取能力。收窄成單一方法的理由同 SnapshotReader：這一層不該握著
// *repository.Repo 全部。
//
// 選 GetCandidateSetWithCandidates 而不是逐一 GetCandidate(candidateSetID, id)：
// 這支的三個參數 (candidate_set_id, request_id, buyer_id) 一起構成 WHERE 條件，
// 一次查詢就同時證明「這個 candidate_set 屬於這個 request，而這個 request 屬於這個
// 買家」——這正是既有 /selections 端點（usecase.SelectionUseCase.Toggle）用來擋跨
// 買家存取的那一支。只帶 candidate_set_id 的 GetCandidate 少了買家這一維：買家 A
// 拿買家 B 的 candidateSetId 一樣查得到候選，等於把別人的候選當成自己報價單的收件人。
// 順帶也省掉 N+1 次查詢——候選一次全撈回來，在記憶體裡對 id。
type CandidateReader interface {
	GetCandidateSetWithCandidates(ctx context.Context, candidateSetID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error)
}
