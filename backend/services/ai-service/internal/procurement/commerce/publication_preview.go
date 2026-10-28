package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// publicationApprovalSchemaVersion 是 approval 這個 DB 實體的 **data schema** 版本，
// 與 PublicationApproval.Version（狀態轉移的 CAS token）是兩件事。欄位形狀改變時
// 才動這個字串。
const publicationApprovalSchemaVersion = "hawkr.agent.publication-approval.v1"

// publicationApprovalTTL 是 approval 從建立到必須 execute 的時限。與
// pricePreviewTTL（15 分鐘）刻意不同量級：那是「試算結果還算不算新鮮」，這是
// 「買家的核准意圖還算不算數」——買家可能在核准後去確認庫存或跟同事討論再回來
// 送出，15 分鐘會把正常流程逼成反覆重新核准。
//
// ⛔ 到期不是這裡執行的：本 slice 只寫 expires_at，把它轉成 EXPIRED 狀態是
// Slice 2 execute 的 hard block 與後續的到期掃描要做的事。
const publicationApprovalTTL = 24 * time.Hour

var (
	// ErrRevisionStale：買家帶的 expectedRevisionId 與現行 active revision 不符。
	// 與 ErrStalePricePreviewRevision 分開命名是刻意的——那個屬於 price preview 的
	// CAS 語意，這裡的復原動作是「重新讀 request 再核准」。
	ErrRevisionStale = errors.New("procurement: expectedRevisionId doesn't match current active revision")

	// ErrConfirmedSnapshotStale：confirmed snapshot 不屬於這次 preview 的 request，
	// 或它是針對另一個 revision 確認的。⛔ 這條不是 404：snapshot 確實存在也確實是
	// 這個買家的，錯的是它與現況對不上，正確復原是重新確認一次。
	ErrConfirmedSnapshotStale = errors.New("procurement: confirmed rfq snapshot doesn't match this request or revision")

	// ErrCandidateSetMismatch：candidate set 不屬於本 request／本買家，或選定的候選
	// 不屬於這個 candidate set，或兩支 pipeline 的形狀矛盾（WISH_POOL 帶了 candidate
	// set、DIRECT_RECIPIENTS 沒有選任何候選）。
	ErrCandidateSetMismatch = errors.New("procurement: candidate set or selection doesn't match this request")

	// ErrUnknownAudienceMode：值域外的 audience mode。⛔ 不得靜默落到任一支 pipeline
	// ——兩支的完成定義不同（doc 64 §4.2），猜錯等於把公開池的詢價當成指定投遞。
	ErrUnknownAudienceMode = errors.New("procurement: unknown audience mode")

	// ErrInvalidResponseDeadline：responseDeadline 不是 YYYY-MM-DD。
	//
	// ⛔ 必須在編譯 payload 之前擋下，不能靠持久層的寬容轉換吸收：那一層對解不開的
	// 字串寫 SQL NULL，而同一個字串會原樣進 seller-facing payload 並被
	// seller_facing_payload_hash 涵蓋——結果是 response_deadline 欄位是 NULL、hash 卻
	// 蓋著一個垃圾值，兩邊對同一件事給出不同答案，而且沒有任何錯誤訊號。
	ErrInvalidResponseDeadline = errors.New("procurement: responseDeadline must be YYYY-MM-DD")

	// ErrCandidateIdentityUnresolved：候選列的 lane 與身分欄位對不上（PLATFORM 沒有
	// store_id、EXTERNAL 沒有 prospect_id，或 lane 根本不在值域內）。
	//
	// ⛔ 這是**伺服器端的資料不變式被違反**，不是買家的輸入問題——買家只送了
	// candidateId，那一列長什麼樣不由他決定。所以映射成 Internal 而不是 400：回
	// 400 等於要買家去修一個他碰不到的東西。
	//
	// 不靜默略過那一位收件人：略過等於偷偷把買家核准的受眾改小，而 audienceHash
	// 會照著被改小的名單算出來，事後從資料上看不出少了誰。
	ErrCandidateIdentityUnresolved = errors.New("procurement: candidate lane and identity columns disagree")
)

// PublicationApprovalRepo／ConfirmedRfqSnapshotByIDReader／PricePreviewByIDReader／
// CandidateSetReader 的方法簽章逐字對齊 repository.Repo，沿用本套件既有慣例：
// usecase 直接依賴 repository 的型別，不重新發明一組平行型別。
type PublicationApprovalRepo interface {
	InsertPublicationApproval(ctx context.Context, in repository.InsertPublicationApprovalInput) (repository.PublicationApproval, error)
	FindPublicationApprovalByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, previewClientRequestID string) (repository.PublicationApproval, error)
}

type ConfirmedRfqSnapshotByIDReader interface {
	FindConfirmedRfqSnapshotByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.ConfirmedRfqSnapshot, error)
}

type PricePreviewByIDReader interface {
	FindPricePreviewByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.PricePreview, error)
}

type CandidateSetReader interface {
	GetCandidateSetWithCandidates(ctx context.Context, candidateSetID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error)
}

// PreviewRfqPublicationInput 是 preview 的輸入。⛔ 只能帶參照，不得重送 subject／
// quantity／unit／budget／range／price／quality／candidate facts 或任何 free-form
// RFQ 內容——伺服器從 confirmed snapshot 編譯 canonical seller-facing payload。
type PreviewRfqPublicationInput struct {
	BuyerID                identity.BuyerID
	RequestID              int64
	PreviewClientRequestID string
	ExpectedRevisionID     int64
	ConfirmedRfqSnapshotID int64
	AudienceMode           string
	// CandidateSetID／SelectedCandidateIDs 只有 DIRECT_RECIPIENTS 用。
	// SelectedCandidateIDs 的順序有意義（#2450 Scope 2「ordered selected candidate IDs」）。
	CandidateSetID       int64
	SelectedCandidateIDs []int64
	// ResponseDeadline 是 YYYY-MM-DD，publication 自己的屬性（doc 64 §3.1）。
	ResponseDeadline *string
}

// PreviewRfqPublicationResult 是 preview 的產出。
//
// ⛔ ApprovalID ＋ ApprovalVersion 兩者都必須回給呼叫端：Slice 2 的 execute 以它們
// 做 CAS，而回應是呼叫端唯一的來源。既有 `POST .../runs/{runId}/cancel` 就是反面
// 教材——它必填 expectedRunVersion，但 runVersion 只出現在 cancel 自己的回應裡，
// 呼叫之前無處可讀，端點因此實際上呼叫不成功。
type PreviewRfqPublicationResult struct {
	ApprovalID      int64
	ApprovalVersion int

	AudienceHash string
	// AudiencePolicyID／AudiencePolicyVersion 要吐回去，買家介面才講得出「這會發給誰」。
	// WISH_POOL v1 是 WISH_POOL_ALL_ACTIVE_SELLERS_V1 / 1 ＝ 全站未停權賣家——
	// typed 地說「無限制」，不是讓欄位消失。
	AudiencePolicyID      string
	AudiencePolicyVersion int
	// RecipientCount 是 DIRECT 的選定家數；WISH_POOL 恆為 0（無名單是本模式的定義，
	// 不是資料缺漏）。
	RecipientCount int

	SellerFacingPayload     domain.SellerFacingPayload
	SellerFacingPayloadJSON json.RawMessage
	SellerFacingPayloadHash string

	ExpiresAt time.Time
}

type PublicationPreviewUseCase struct {
	approvals     PublicationApprovalRepo
	snapshots     ConfirmedRfqSnapshotByIDReader
	previews      PricePreviewByIDReader
	candidateSets CandidateSetReader
	revisions     pricing.RevisionReader
	// now 同 AnswerUseCase.now／ConfirmRfqSnapshotUseCase.now 的既有慣例：預設
	// time.Now().UTC()，測試可直接覆寫（同套件內可見）注入固定時間。
	now func() time.Time
}

func NewPublicationPreviewUseCase(
	approvals PublicationApprovalRepo,
	snapshots ConfirmedRfqSnapshotByIDReader,
	previews PricePreviewByIDReader,
	candidateSets CandidateSetReader,
	revisions pricing.RevisionReader,
) *PublicationPreviewUseCase {
	return &PublicationPreviewUseCase{
		approvals: approvals, snapshots: snapshots, previews: previews,
		candidateSets: candidateSets, revisions: revisions,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// PreviewRfqPublication 產生一份 publication preview 並在伺服器上留下一個 versioned、
// 內容不可變的 approval。
//
// ⛔ 本 slice 到此為止：**不建立** publication、invite、outbox、projection，也不發任何
// 對外通知。approval 與 execute 分離是刻意的（doc 64 §4.1）——doc 57 §1.1-1 的「不得在
// 無人確認下觸發對外通知」需要一個可稽核的「人確認過」的物件。
//
// 流程固定：比對 active revision → 讀 confirmed snapshot 並驗歸屬 → 讀 price preview →
// 依 mode 編譯 audience → 編譯 seller-facing payload → 算兩個 hash → 寫 approval。
// 任何一步失敗一律在寫入之前 return，不得有部分寫入。
//
// 這裡刻意**沒有** usecase 層的冪等短路：判斷「同 key 重送算不算同一次核准」需要比對
// server 算出來的兩個 hash，而算它們就是這支函式的全部工作，短路省不掉任何 IO；
// repository 的 INSERT-先試、撞 23505 才查 是唯一仲裁者，也是併發下正確的那一條路。
func (uc *PublicationPreviewUseCase) PreviewRfqPublication(ctx context.Context, in PreviewRfqPublicationInput) (PreviewRfqPublicationResult, error) {
	_, rev, err := uc.revisions.GetSnapshot(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}
	if rev.ID != in.ExpectedRevisionID {
		return PreviewRfqPublicationResult{}, ErrRevisionStale
	}

	// 依 (id, buyerID) 查——wrong-owner 在 repository 層就是 not-found，跟真的不存在
	// 同一種 404 shape，不洩漏「這個 id 存在，只是不是你的」。
	snap, err := uc.snapshots.FindConfirmedRfqSnapshotByIDAndBuyer(ctx, in.ConfirmedRfqSnapshotID, in.BuyerID)
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}
	// 歸屬與新鮮度：snapshot 的 id 是呼叫端傳的，不驗證等於讓買家把 A 案的定案內容
	// 發成 B 案的詢價；revision 對不上則代表定案之後需求又被改過。兩者都是可復原的
	// stale，不是死路。
	if snap.RequestID != in.RequestID || snap.RequestRevisionID != rev.ID {
		return PreviewRfqPublicationResult{}, ErrConfirmedSnapshotStale
	}

	preview, err := uc.previews.FindPricePreviewByIDAndBuyer(ctx, snap.PricePreviewID, in.BuyerID)
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}

	audience, err := uc.compileAudience(ctx, in, rev.ID)
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}
	audienceHash, err := audience.Hash()
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}

	// 先驗格式再編譯：payload 與 DB 欄位讀的是同一個字串，任一邊靜默吸收掉錯誤格式
	// 都會讓兩者分家（見 ErrInvalidResponseDeadline 的說明）。
	if in.ResponseDeadline != nil {
		if _, parseErr := time.Parse(domain.ResponseDeadlineLayout, *in.ResponseDeadline); parseErr != nil {
			return PreviewRfqPublicationResult{}, ErrInvalidResponseDeadline
		}
	}

	payload, err := domain.CompileSellerFacingPayload(snap.CanonicalPayloadJSON, in.ResponseDeadline)
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}
	payloadHash, err := payload.Hash()
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return PreviewRfqPublicationResult{}, fmt.Errorf("marshal seller-facing payload: %w", err)
	}

	now := uc.now()
	row, err := uc.approvals.InsertPublicationApproval(ctx, repository.InsertPublicationApprovalInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, RequestRevisionID: rev.ID,
		ConfirmedRfqSnapshotID:  snap.ID,
		ConfirmedRfqPayloadHash: snap.PayloadHash,

		PricePreviewID:          preview.ID,
		PricePreviewVersion:     preview.Version,
		PricePreviewPayloadHash: preview.PayloadHash,
		PricePreviewInputHash:   preview.PreviewInputHash,

		BuyerRequestedRangeHash: pricing.HashJSON(json.RawMessage(preview.BuyerRequestedRangeJSON)),

		AudienceMode:              audience.Mode,
		AudiencePolicyID:          audience.PolicyID,
		AudiencePolicyVersion:     audience.PolicyVersion,
		AudienceHash:              audienceHash,
		CandidateSetID:            candidateSetIDParam(audience),
		CandidateSetPolicyVersion: candidateSetPolicyVersionParam(audience),
		// Recipients 與算出 audienceHash 的那份 audience.Recipients 是同一份——execute
		// 要重驗「發給誰」需要名單本身，只存 hash 不夠（hash 不可逆，Slice 1 留下的洞）。
		Recipients: audience.Recipients,

		ResponseDeadline:        in.ResponseDeadline,
		SellerFacingPayloadJSON: payloadJSON,
		SellerFacingPayloadHash: payloadHash,

		State:         codemeta.ProcurementRfqPublicationApprovalStatePending.Value,
		SchemaVersion: publicationApprovalSchemaVersion,

		PreviewClientRequestID: in.PreviewClientRequestID,
		GeneratedAt:            now,
		ExpiresAt:              now.Add(publicationApprovalTTL),
	})
	if err != nil {
		return PreviewRfqPublicationResult{}, err
	}

	return PreviewRfqPublicationResult{
		ApprovalID: row.ID, ApprovalVersion: row.Version,
		AudienceHash:          row.AudienceHash,
		AudiencePolicyID:      row.AudiencePolicyID,
		AudiencePolicyVersion: row.AudiencePolicyVersion,
		RecipientCount:        len(audience.Recipients),

		SellerFacingPayload:     payload,
		SellerFacingPayloadJSON: row.SellerFacingPayloadJSON,
		SellerFacingPayloadHash: row.SellerFacingPayloadHash,

		ExpiresAt: row.ExpiresAt,
	}, nil
}

// compileAudience 依 mode 編譯受眾。⛔ audienceHash 的 preimage 完全由伺服器決定，
// 呼叫端傳入的任何 hash 都不可信（#2450 Scope 2）。
func (uc *PublicationPreviewUseCase) compileAudience(ctx context.Context, in PreviewRfqPublicationInput, activeRevisionID int64) (domain.PublicationAudience, error) {
	switch in.AudienceMode {
	case codemeta.ProcurementPublicationAudienceModeWishPool.Value:
		// 形狀矛盾在應用層就擋下（DB CHECK 是最終防線，不是唯一防線）：Wish Pool
		// 沒有 candidate set 這個概念，帶了它代表呼叫端搞錯了模式。
		if in.CandidateSetID != 0 || len(in.SelectedCandidateIDs) > 0 {
			return domain.PublicationAudience{}, ErrCandidateSetMismatch
		}
		return domain.WishPoolAudience(), nil

	case codemeta.ProcurementPublicationAudienceModeDirectRecipients.Value:
		// ⛔ 空名單不得冒充 Wish Pool：兩支 pipeline 的完成定義不同，一個沒有收件人
		// 的 DIRECT publication 永遠不會完成，卻看起來像是成功發布了。
		if in.CandidateSetID == 0 || len(in.SelectedCandidateIDs) == 0 {
			return domain.PublicationAudience{}, ErrCandidateSetMismatch
		}
		// 查詢同時收斂到 requestID ＋ buyerID：別的 request 或別的買家的 candidate set
		// 在這裡就查不到。
		set, candidates, err := uc.candidateSets.GetCandidateSetWithCandidates(ctx, in.CandidateSetID, in.RequestID, in.BuyerID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return domain.PublicationAudience{}, ErrCandidateSetMismatch
			}
			return domain.PublicationAudience{}, err
		}
		if set.RequestRevisionID != activeRevisionID {
			return domain.PublicationAudience{}, ErrCandidateSetMismatch
		}

		byID := make(map[int64]repository.Candidate, len(candidates))
		for _, c := range candidates {
			byID[c.ID] = c
		}
		// 依 SelectedCandidateIDs 的順序解析——順序是契約的一部分，不得改用
		// candidates 的排列順序。
		recipients := make([]domain.PublicationRecipient, 0, len(in.SelectedCandidateIDs))
		seen := make(map[int64]bool, len(in.SelectedCandidateIDs))
		for _, id := range in.SelectedCandidateIDs {
			if seen[id] {
				return domain.PublicationAudience{}, ErrCandidateSetMismatch
			}
			seen[id] = true

			c, ok := byID[id]
			if !ok {
				return domain.PublicationAudience{}, ErrCandidateSetMismatch
			}
			// 兩條 lane 的身分住在不同欄位（procurement_candidates 的既有形狀）：
			// PLATFORM 用 store_id，EXTERNAL 用 prospect_id 且 store_id 恆為 0。
			// ⛔ 不可拿 store_id 當通用身分——EXTERNAL 全部會塌成 0，讓兩個不同的
			// 站外業者算出同一個 audienceHash。
			switch domain.QuoteLane(c.Lane) {
			case domain.QuoteLanePlatform:
				if c.StoreID == 0 {
					return domain.PublicationAudience{}, ErrCandidateIdentityUnresolved
				}
				recipients = append(recipients, domain.PlatformRecipient(int64(c.StoreID)))
			case domain.QuoteLaneExternal:
				// ⚠️ EXTERNAL 收件人**可以**進核准，但 doc 64 §4.3 明文：在
				// vendor portal／email 這條真實投遞管道（T17）落地之前，它的對外
				// 狀態必須是「尚未送出」而不是「已送出」。Slice 2 的 execute 不得
				// 因為這裡收下了就宣稱送達——既有 quote-invite 那條路正是踩在這裡
				// （publisher 跳過 EXTERNAL、outbox 列卻標 done）。
				if c.ProspectID == nil || *c.ProspectID == 0 {
					return domain.PublicationAudience{}, ErrCandidateIdentityUnresolved
				}
				recipients = append(recipients, domain.ExternalRecipient(*c.ProspectID))
			default:
				return domain.PublicationAudience{}, ErrCandidateIdentityUnresolved
			}
		}
		return domain.DirectRecipientsAudience(set.ID, set.PolicyVersion, recipients), nil

	default:
		return domain.PublicationAudience{}, ErrUnknownAudienceMode
	}
}

// candidateSetIDParam／candidateSetPolicyVersionParam 把 audience 的 candidate set 欄位
// 轉成 DB 參數。WISH_POOL 回 nil 而不是 0——「本模式沒有這個概念」與「有概念但值為 0」
// 在讀取端不得混為一談，DB 的兩條 CHECK 也是照 NULL 判形狀。
func candidateSetIDParam(a domain.PublicationAudience) *int64 {
	if a.CandidateSetID == 0 {
		return nil
	}
	v := a.CandidateSetID
	return &v
}

func candidateSetPolicyVersionParam(a domain.PublicationAudience) *int {
	if a.CandidateSetID == 0 {
		return nil
	}
	v := a.CandidateSetPolicyVersion
	return &v
}
