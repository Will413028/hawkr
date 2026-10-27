package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ErrPublicationApprovalNotFound 是 FindPublicationApprovalByIdempotencyKey 找不到列時的
// 哨兵錯誤，用途同 ErrConfirmedRfqSnapshotNotFound：呼叫端只用它判斷「這個
// previewClientRequestId 是不是第一次見到」，不拿它當存取控制錯誤。
var ErrPublicationApprovalNotFound = errors.New("procurement: rfq publication approval not found")

// ErrPublicationApprovalIdempotencyConflict 是同一個 (buyerID, previewClientRequestID)
// 撞鍵、但既有列的內容與這次輸入不同時的哨兵錯誤（#2450 Scope 7：同 key 換內容是衝突，
// 零 mutation）。
//
// 比對的是 seller_facing_payload_hash 與 audience_hash 兩個 server-owned hash，不是逐欄
// 比對：這兩者各自涵蓋了「賣家會看到什麼」與「會發給誰」的全部內容，是 execute 的 CAS
// 對象。任何真正改變核准語意的漂移都會反映在其中一個上；其餘欄位（例如 expires_at）
// 是同一次核准的衍生值，不該讓重送因為時鐘走了幾毫秒就變成衝突。
var ErrPublicationApprovalIdempotencyConflict = errors.New("procurement: preview client request id already used for a different publication approval")

// PublicationApproval 是買家「我要把這張詢價發出去」的不可變意圖（#2450 Scope 3）。
//
// Version 是**狀態轉移的 CAS token**（同 procurement_agent_runs.version 的模型），
// 不是 data schema 版本——後者是 SchemaVersion。⛔ 它與 ID 都必須被 preview 的回應
// 吐回給呼叫端：Slice 2 的 execute 拿這兩者做 CAS，回應是呼叫端唯一的來源。
type PublicationApproval struct {
	ID      int64
	Version int
	BuyerID identity.BuyerID

	RequestID               int64
	RequestRevisionID       int64
	ConfirmedRfqSnapshotID  int64
	ConfirmedRfqPayloadHash string

	PricePreviewID          int64
	PricePreviewVersion     int64
	PricePreviewPayloadHash string
	PricePreviewInputHash   string

	QualityKnowledgeState   string
	BuyerRequestedRangeHash string

	AudienceMode          string
	AudiencePolicyID      string
	AudiencePolicyVersion int
	AudienceHash          string
	// CandidateSetID／CandidateSetPolicyVersion 只有 DIRECT_RECIPIENTS 有值；
	// WISH_POOL 是 nil 而不是 0——「本模式沒有這個概念」與「有概念但值為 0」在
	// 讀取端不得混為一談。
	CandidateSetID            *int64
	CandidateSetPolicyVersion *int

	// ResponseDeadline 是 YYYY-MM-DD；nil 代表買家沒有指定截止日。
	ResponseDeadline        *string
	SellerFacingPayloadJSON json.RawMessage
	SellerFacingPayloadHash string

	State         string
	SchemaVersion string

	PreviewClientRequestID string
	GeneratedAt            time.Time
	ExpiresAt              time.Time
}

// InsertPublicationApprovalInput 是一次 approval 寫入的完整輸入。ID 與 Version 不在
// 這裡：前者由 r.newID() 產生，後者由 DB 的 DEFAULT 1 決定。
type InsertPublicationApprovalInput struct {
	BuyerID identity.BuyerID

	RequestID               int64
	RequestRevisionID       int64
	ConfirmedRfqSnapshotID  int64
	ConfirmedRfqPayloadHash string

	PricePreviewID          int64
	PricePreviewVersion     int64
	PricePreviewPayloadHash string
	PricePreviewInputHash   string

	QualityKnowledgeState   string
	BuyerRequestedRangeHash string

	AudienceMode              string
	AudiencePolicyID          string
	AudiencePolicyVersion     int
	AudienceHash              string
	CandidateSetID            *int64
	CandidateSetPolicyVersion *int
	// Recipients 是 DIRECT_RECIPIENTS 的 ordered recipient list，與 approval 本體同一個
	// 交易寫進 procurement_rfq_publication_approval_recipients。WISH_POOL 恆為空——沒有
	// 名單是這個模式的定義，不是資料缺漏。順序是契約的一部分：position 依這個切片的索引
	// 寫入，換順序就是換受眾（domain.PublicationAudience.Hash 的既有規則），Slice 2 的
	// execute 要重驗「發給誰」就是靠這份名單本身——audience_hash 不可逆，光有 hash 重驗不了。
	Recipients []domain.PublicationRecipient

	ResponseDeadline        *string
	SellerFacingPayloadJSON json.RawMessage
	SellerFacingPayloadHash string

	State         string
	SchemaVersion string

	PreviewClientRequestID string
	GeneratedAt            time.Time
	ExpiresAt              time.Time
}

func toPublicationApprovalEntity(row gen.ProcurementRfqPublicationApproval) PublicationApproval {
	return PublicationApproval{
		ID: row.ID, Version: int(row.Version), BuyerID: identity.BuyerID(row.BuyerID),
		RequestID:               row.RequestID,
		RequestRevisionID:       row.RequestRevisionID,
		ConfirmedRfqSnapshotID:  row.ConfirmedRfqSnapshotID,
		ConfirmedRfqPayloadHash: row.ConfirmedRfqPayloadHash,

		PricePreviewID:          row.PricePreviewID,
		PricePreviewVersion:     row.PricePreviewVersion,
		PricePreviewPayloadHash: row.PricePreviewPayloadHash,
		PricePreviewInputHash:   row.PricePreviewInputHash,

		QualityKnowledgeState:   row.QualityKnowledgeState,
		BuyerRequestedRangeHash: row.BuyerRequestedRangeHash,

		AudienceMode:              row.AudienceMode,
		AudiencePolicyID:          row.AudiencePolicyID,
		AudiencePolicyVersion:     int(row.AudiencePolicyVersion),
		AudienceHash:              row.AudienceHash,
		CandidateSetID:            row.CandidateSetID,
		CandidateSetPolicyVersion: int32PtrToIntPtr(row.CandidateSetPolicyVersion),

		ResponseDeadline:        dateValue(row.ResponseDeadline),
		SellerFacingPayloadJSON: row.SellerFacingPayloadJson,
		SellerFacingPayloadHash: row.SellerFacingPayloadHash,

		State:         row.State,
		SchemaVersion: row.SchemaVersion,

		PreviewClientRequestID: row.PreviewClientRequestID,
		GeneratedAt:            row.GeneratedAt.Time,
		ExpiresAt:              row.ExpiresAt.Time,
	}
}

// InsertPublicationApproval 在**同一個交易**內寫入一筆 approval 與它的 ordered
// recipient list（procurement_rfq_publication_approval_recipients）。分兩次寫會產生
// 「approval 存在但名單缺失」的中間態，而 execute 會把那個中間態誤判成 WISH_POOL
// （沒有名單正是 WISH_POOL 的定義）——這是 Slice 1 留下的洞，本次補上。
//
// 冪等：同一買家同一 previewClientRequestID 重送時回傳前次寫入的那一列，不建立第二筆
// approval，也不重寫第二份 recipients。
//
// 撞鍵偵測仍是「INSERT 先試、撞 23505 才查」（同 AppendRevision 對
// uq_procurement_answer_patches_idempotency 的既有模式），但**復原查詢搬到 WithTx 之
// 外**：Postgres 的交易只要有一句出錯就整個進入 aborted 狀態，同一個交易裡後續任何語句
// （含查詢）都會被拒絕，不能在同一個 txCtx 內「撞錯之後接著查」。真正併發撞鍵時，輸的
// 那個 transaction（連同它剛寫的 recipients）整段 ROLLBACK，函式外層再用交易外的 ctx
// 重新查一次，讀到的是贏家 commit 之後的完整結果，跟正常序列重送走同一條路徑——這正是
// revision.go AppendRevision／recoverIdempotentAppend 的既有模式，這裡照搬。
//
// ⛔ 「搬到 WithTx 之外」只在**呼叫端自己沒有交易**時就夠了。呼叫端已經在交易裡時，
// WithTx 是沿用那個交易、不開新的，撞鍵因此把**呼叫端的**交易打成 aborted，那次復原
// 查詢照樣拿到 25P02。所以整段改用 r.withNestableTx（見該 helper 的說明）。
//
// ⛔ 撞鍵之後不得無條件回放既有列：兩個「同一個 key、不同核准內容」的呼叫真的併發時，
// 其中一個會撞 23505，此時必須比對內容才知道這是重送還是衝突。比對 seller-facing payload
// 與 audience 兩個 hash——它們是 execute 的 CAS 對象，任何改變核准語意的漂移都會反映在
// 其中一個上。不一致時回 ErrPublicationApprovalIdempotencyConflict，既有列零 mutation。
func (r *Repo) InsertPublicationApproval(ctx context.Context, in InsertPublicationApprovalInput) (PublicationApproval, error) {
	var out PublicationApproval
	// WithTx 保證 approval ＋ ordered recipient list 的原子性。撞冪等鍵不拋錯
	// （query 用 ON CONFLICT DO UPDATE，見 PostgreSQL 規範第十二章）。
	var replayed bool
	approvalID := r.newID()
	txErr := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		row, err := q.InsertProcurementRfqPublicationApproval(txCtx, gen.InsertProcurementRfqPublicationApprovalParams{
			ID: approvalID, BuyerID: int64(in.BuyerID), RequestID: in.RequestID,
			RequestRevisionID:       in.RequestRevisionID,
			ConfirmedRfqSnapshotID:  in.ConfirmedRfqSnapshotID,
			ConfirmedRfqPayloadHash: in.ConfirmedRfqPayloadHash,

			PricePreviewID:          in.PricePreviewID,
			PricePreviewVersion:     in.PricePreviewVersion,
			PricePreviewPayloadHash: in.PricePreviewPayloadHash,
			PricePreviewInputHash:   in.PricePreviewInputHash,

			QualityKnowledgeState:   in.QualityKnowledgeState,
			BuyerRequestedRangeHash: in.BuyerRequestedRangeHash,
			ResponseDeadline:        dateParam(in.ResponseDeadline),

			AudienceMode:              in.AudienceMode,
			AudiencePolicyID:          in.AudiencePolicyID,
			AudiencePolicyVersion:     int32(in.AudiencePolicyVersion),
			AudienceHash:              in.AudienceHash,
			CandidateSetID:            in.CandidateSetID,
			CandidateSetPolicyVersion: intPtrToInt32Ptr(in.CandidateSetPolicyVersion),

			SellerFacingPayloadJson: in.SellerFacingPayloadJSON,
			SellerFacingPayloadHash: in.SellerFacingPayloadHash,

			State:         in.State,
			SchemaVersion: in.SchemaVersion,

			PreviewClientRequestID: in.PreviewClientRequestID,
			GeneratedAt:            timestamptzParam(in.GeneratedAt),
			ExpiresAt:              timestamptzParam(in.ExpiresAt),
		})
		if err != nil {
			return err
		}
		out = toPublicationApprovalEntity(row)
		// row.ID != approvalID ⇒ 撞冪等鍵、回傳的是既有列。recipients 早就由贏家寫好，
		// 這次不能再插一輪。
		if row.ID != approvalID {
			replayed = true
			return nil
		}

		// position 即 in.Recipients 的切片索引——順序是契約的一部分，見 schema 該表上方
		// 的註解與 domain.PublicationAudience.Hash。WISH_POOL 呼叫端傳空切片，這個迴圈
		// 天然是 no-op，不需要另外分支。
		for i, recipient := range in.Recipients {
			storeID, prospectID, identityErr := recipientIdentityParams(recipient)
			if identityErr != nil {
				return identityErr
			}
			if err := q.InsertProcurementRfqPublicationApprovalRecipient(txCtx, gen.InsertProcurementRfqPublicationApprovalRecipientParams{
				ApprovalID: row.ID, Position: int32(i), Lane: string(recipient.Lane),
				StoreID: storeID, ProspectID: prospectID,
			}); err != nil {
				return err
			}
		}

		return nil
	})
	if txErr != nil {
		return PublicationApproval{}, txErr
	}
	if replayed && (out.SellerFacingPayloadHash != in.SellerFacingPayloadHash || out.AudienceHash != in.AudienceHash) {
		return PublicationApproval{}, ErrPublicationApprovalIdempotencyConflict
	}
	return out, nil
}

// recipientIdentityParams 把 domain.PublicationRecipient 轉成 DB 的 (store_id,
// prospect_id) 兩個互斥欄位，理由與 chk_prpar_identity_matches_lane 這條 CHECK 一致：
// 兩條 lane 的身分住不同欄位，不是「兩個都填」的方便寫法（domain.PublicationRecipient
// 的既有註解）。lane 與 id 欄位對不上時回 domain.ErrRecipientIdentityMismatch，讓一列
// 形狀錯誤的 recipient 在寫入前就被擋下，不是安靜寫出一列違反 DB CHECK（或更糟——身分
// 算錯但「看起來合法」）的資料。正常生產路徑（usecase.compileAudience）已經只用
// PlatformRecipient／ExternalRecipient 建構、不會走到這裡，這道檢查是給非典型呼叫端
// （例如未來的測試或工具）的防線，DB CHECK 仍是最終防線。
func recipientIdentityParams(recipient domain.PublicationRecipient) (storeID, prospectID *int64, err error) {
	switch recipient.Lane {
	case domain.QuoteLanePlatform:
		if recipient.StoreID == 0 || recipient.ProspectID != 0 {
			return nil, nil, fmt.Errorf("%w: lane=PLATFORM storeId=%d prospectId=%d", domain.ErrRecipientIdentityMismatch, recipient.StoreID, recipient.ProspectID)
		}
		v := recipient.StoreID
		return &v, nil, nil
	case domain.QuoteLaneExternal:
		if recipient.ProspectID == 0 || recipient.StoreID != 0 {
			return nil, nil, fmt.Errorf("%w: lane=EXTERNAL storeId=%d prospectId=%d", domain.ErrRecipientIdentityMismatch, recipient.StoreID, recipient.ProspectID)
		}
		v := recipient.ProspectID
		return nil, &v, nil
	default:
		return nil, nil, fmt.Errorf("%w: unknown lane %q", domain.ErrRecipientIdentityMismatch, recipient.Lane)
	}
}

// toPublicationRecipientEntity 把一列 DB row 轉回 domain.PublicationRecipient。lane 值域
// 只有 PLATFORM／EXTERNAL 兩者（chk_prpar_identity_matches_lane 擋住第三種），EXTERNAL
// 是唯一需要特判的分支，其餘（含理論上不會出現的未知值）落到 PLATFORM 的還原方式——
// 與 identity() 對未知 lane 一律當成資料不變式已被違反、留給呼叫端的既有假設一致。
func toPublicationRecipientEntity(row gen.ProcurementRfqPublicationApprovalRecipient) domain.PublicationRecipient {
	if domain.QuoteLane(row.Lane) == domain.QuoteLaneExternal {
		return domain.ExternalRecipient(int64Value(row.ProspectID))
	}
	return domain.PlatformRecipient(int64Value(row.StoreID))
}

// FindPublicationApprovalByIdempotencyKey 依冪等鍵 (buyer_id, preview_client_request_id)
// 找出既有的 approval。找不到回 ErrPublicationApprovalNotFound。
func (r *Repo) FindPublicationApprovalByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, previewClientRequestID string) (PublicationApproval, error) {
	row, err := r.q(ctx).FindProcurementRfqPublicationApprovalByIdempotencyKey(ctx, gen.FindProcurementRfqPublicationApprovalByIdempotencyKeyParams{
		BuyerID: int64(buyerID), PreviewClientRequestID: previewClientRequestID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicationApproval{}, ErrPublicationApprovalNotFound
		}
		return PublicationApproval{}, err
	}
	return toPublicationApprovalEntity(row), nil
}

// ErrPublicationApprovalCASFailed 是 (id, version, buyerID, state) 四項 CAS 沒有
// 命中任何列時的哨兵錯誤。四種成因對呼叫端是同一個結論——「你手上的 approval
// 狀態不是你以為的那個，重讀再試」——所以不細分：approval 不存在、不是這個買家的、
// version 已被推進（併發的另一個 execute 贏了）、state 已不是 PENDING。
//
// ⛔ 述詞必須含 version，不得拿掉——但理由不是併發。READ COMMITTED 下 Postgres
// 的 EvalPlanQual 會讓併發輸家的 UPDATE 卡在贏家的列鎖上，贏家 commit 後拿更新後
// 那一列重新求值述詞，state 已是 EXECUTING，光 state = 'PENDING' 自己就擋得下
// 併發輸家（只想併發情境的人會因此誤判 version 是多餘的）。version 真正擋的是
// 非併發的過期輸入：呼叫端手上是更早讀到的 version，這時 row 的 state 可能仍是
// PENDING。哨兵：TestClaimPublicationApprovalForExecute_VersionDivergesWhileStatePendingIsRejected。
var ErrPublicationApprovalCASFailed = errors.New("procurement: publication approval CAS failed")

// ClaimPublicationApprovalForExecute 把 approval 從 PENDING 推進到 EXECUTING。
// 單一 UPDATE 語句本身是原子的，所以不需要外層交易——搶輸的那一個命中 0 列。
func (r *Repo) ClaimPublicationApprovalForExecute(ctx context.Context, approvalID int64, expectedVersion int, buyerID identity.BuyerID) (PublicationApproval, error) {
	row, err := r.q(ctx).ClaimProcurementRfqPublicationApprovalForExecute(ctx, gen.ClaimProcurementRfqPublicationApprovalForExecuteParams{
		ID: approvalID, Version: int32(expectedVersion), BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicationApproval{}, ErrPublicationApprovalCASFailed
		}
		return PublicationApproval{}, err
	}
	return toPublicationApprovalEntity(row), nil
}

// MarkPublicationApprovalSucceeded 把 approval 從 EXECUTING 收成 SUCCEEDED。
// 呼叫端必須帶 claim 之後的 version（claim 已經 +1 過）。
func (r *Repo) MarkPublicationApprovalSucceeded(ctx context.Context, approvalID int64, expectedVersion int) error {
	_, err := r.q(ctx).MarkProcurementRfqPublicationApprovalSucceeded(ctx, gen.MarkProcurementRfqPublicationApprovalSucceededParams{
		ID: approvalID, Version: int32(expectedVersion),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPublicationApprovalCASFailed
	}
	return err
}

// ListPublicationApprovalRecipients 依 position 讀回一個 approval 的 ordered recipient
// list。WISH_POOL 的 approval 沒有任何 recipients 列——回空切片，不是錯誤：沒有名單是
// 這個模式的定義（domain.WishPoolAudience 的既有註解），不是資料缺漏。
func (r *Repo) ListPublicationApprovalRecipients(ctx context.Context, approvalID int64) ([]domain.PublicationRecipient, error) {
	rows, err := r.q(ctx).ListProcurementRfqPublicationApprovalRecipients(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicationRecipient, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPublicationRecipientEntity(row))
	}
	return out, nil
}
