package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// publicationSchemaVersion 是 publication 這個 DB 實體的 **data schema** 版本，
// 與 Publication.Version（狀態轉移用）是兩件事，命名規則同
// publicationApprovalSchemaVersion。欄位形狀改變時才動這個字串。
const publicationSchemaVersion = "hawkr.agent.publication.v1"

// publicationRunDomainRefType 是 execute 的 ledger run 與 RFQ_PUBLICATION_RESULT
// artifact 共用的 domain ref type（doc 57 §4.5 allowlisted domain 參照，同
// candidate search 的 "candidateSetId"）。
const publicationRunDomainRefType = "publicationId"

// ---------------------------------------------------------------------------
// 價格依據 hard block 的值域（#2450 Scope 5，2026-09-08 controller 裁決）
//
// 這三個都是字面值常數而不是 CodeMeta：
// procurement_price_previews.reference_availability 與 pricing_mode 兩欄的值域**從來
// 沒有登記過**（price_preview.go 只寫這幾個字面值），為了單一個 gate 去登記一組新值域
// 是獨立決定，不在本 slice 範圍。
// ---------------------------------------------------------------------------

// referenceUnavailable 是「伺服器算不出價格依據」這件事寫在 approval 上的樣子。
//
// ⚠️ **今天沒有任何路徑會寫出這個值，這道 block 是防禦性的。** codemeta 裡叫
// PRICE_BASIS_UNAVAILABLE 的那一個是 ProcurementErrorReason（422 錯誤碼，**不同軸**），
// 而它自己的登記描述明寫「零 side effect、不建立 preview 列」——沒有 preview 列就沒有
// approval，也就永遠到不了 execute。留著它是為了在這個值哪天真的被持久化時，Scope 5
// 的字面要求已經成立，成本為零。
const referenceUnavailable = "PRICE_BASIS_UNAVAILABLE"

// legacyReferenceUnavailable 是 reference_availability 目前**唯一**的非 AVAILABLE 值。
//
// ⛔ 它本身不是錯誤訊號：`BUYER_ENTERED_NO_REFERENCE` 這條 pricing mode 的正常產出就是
// 它（price_preview.go 的該分支——買家自己出價，本來就不參考 Pricebook）。無條件擋下它
// 會讓**每一張買家自訂價的 publication 都發不出去**，而那正是系統在 guided 模式找不到
// 可靠 mapping 時（ErrMappingUnavailableForGuidedMode）叫買家改用的模式。
//
// Scope 5 真正禁止的是「把 UNAVAILABLE **自動**當成人工模式」，也就是 guided 請求被靜默
// 降級成手動——那件事在 preview 端就已經擋掉了（見 price_preview.go 兩條 mode 各自獨立
// 的路徑），execute 沒有立場重新審一次已經凍在 approval 上的決定。所以判準不是這個值本身，
// 而是它有沒有一個明確宣告的 pricing_mode 支撐它。
const legacyReferenceUnavailable = "UNAVAILABLE"

// buyerEnteredNoReferencePricingMode 是「買家自己出價、不參考 Pricebook」的 pricing mode，
// 也是 legacyReferenceUnavailable 唯一合法的搭配。
const buyerEnteredNoReferencePricingMode = "BUYER_ENTERED_NO_REFERENCE"

var (
	// ErrApprovalStale：買家手上的 approval 已經不是可執行的那一個。四種成因對呼叫端
	// 是同一個結論——「重讀 approval（必要時重新 preview）再試」——所以共用一個 sentinel：
	// CAS 失敗（approval 不存在／不是這個買家的／version 已被推進／state 已不是 PENDING）、
	// approval 屬於另一個 request、DIRECT 的受眾名單與核准當下不一致。
	ErrApprovalStale = errors.New("procurement: publication approval is stale or already consumed")

	// ErrPayloadHashMismatch：呼叫端帶的 payloadHash 與 approval 凍下來的
	// seller_facing_payload_hash 不符。
	//
	// ⛔ 這是**買家看到的內容與伺服器要發出去的內容不是同一份**，不是格式錯誤：
	// 放行等於讓買家核准了 A、系統卻發出 B。復原動作是重新 preview 再核准一次。
	ErrPayloadHashMismatch = errors.New("procurement: payloadHash doesn't match the approved seller-facing payload")

	// ErrApprovalExpired：approval 已超過 expires_at。Slice 1 只寫了這個欄位，把它變成
	// hard block 是本 slice 的事——一份 24 小時前的核准意圖不該還能把詢價發出去。
	ErrApprovalExpired = errors.New("procurement: publication approval has expired")
)

// PublicationCommandRepo 是 execute 的冪等 claim ＋ publication 持久層能力。
// 方法簽章逐字對齊 repository.Repo，沿用本套件既有慣例（usecase 直接依賴
// repository 的型別，不重新發明一組平行型別）。
type PublicationCommandRepo interface {
	ClaimPublicationCommand(ctx context.Context, in repository.ClaimPublicationCommandInput) (repository.PublicationCommand, bool, error)
	InsertPublication(ctx context.Context, in repository.InsertPublicationInput) (repository.Publication, error)
	FindPublicationByCommandID(ctx context.Context, commandID int64) (repository.Publication, error)
	SetPublicationCommandPublicationID(ctx context.Context, commandID, publicationID int64) error
}

// PublicationApprovalExecuteRepo 是 approval 狀態機在 execute 這一側需要的三支：
// 兩次 CAS 轉移，加上重驗受眾用的 ordered recipient list。
type PublicationApprovalExecuteRepo interface {
	ClaimPublicationApprovalForExecute(ctx context.Context, approvalID int64, expectedVersion int, buyerID identity.BuyerID) (repository.PublicationApproval, error)
	MarkPublicationApprovalSucceeded(ctx context.Context, approvalID int64, expectedVersion int) error
	ListPublicationApprovalRecipients(ctx context.Context, approvalID int64) ([]domain.PublicationRecipient, error)
}

// PublicationRunWriter 是 RFQ_PUBLICATION_RESULT artifact 的承載能力
// （#2450 設計決定 1-A）。procurement_agent_run_artifacts.run_id 是 NOT NULL，
// 所以 execute 必須先有一個自己的 run。
type PublicationRunWriter interface {
	CreateLedgerRunForPublication(ctx context.Context, in repository.CreateLedgerRunForPublicationInput) (int64, error)
	UpsertRunArtifact(ctx context.Context, in repository.UpsertRunArtifactInput) (repository.RunArtifact, error)
	FinishLedgerRunForPublication(ctx context.Context, runID int64, buyerID identity.BuyerID) (bool, error)
}

// PublicationInviteRepo 是 DIRECT 投遞層在 execute 這一側需要的兩支：把核准當下那份
// ordered recipient list 落成 invite 列，再為其中**有真實投遞管道**的那些排一個 outbox
// command。寫法同上，簽章逐字對齊 repository.Repo。
type PublicationInviteRepo interface {
	InsertPublicationInvites(ctx context.Context, in repository.InsertPublicationInvitesInput) ([]repository.PublicationInvite, error)
	InsertPublicationOutbox(ctx context.Context, inviteID int64, eventType string, payload []byte, now time.Time) error
}

// ExecuteRfqPublicationInput 是 execute 的輸入。
//
// ⛔ 只有參照與 CAS token，沒有任何 RFQ／price／audience 內容（#2450 Scope 4）：
// 伺服器要發什麼、發給誰全部從 approval 讀。ClientRequestID 是 execute 自己的冪等鍵，
// 與 approval 上的 preview_client_request_id 是不同的鍵（Scope 7：不可跨 phase 重用）。
type ExecuteRfqPublicationInput struct {
	BuyerID            identity.BuyerID
	RequestID          int64
	ClientRequestID    string
	ApprovalID         int64
	ApprovalVersion    int
	ExpectedRevisionID int64
	PayloadHash        string
}

// ExecuteRfqPublicationResult 是 execute 的產出。
//
// ⛔ State = PUBLISHED **只代表 server transaction 完成**，不代表 Wish Pool 已建索引、
// 也不代表賣家或站外業者已收到（Scope 9）。Slice 2b 之後 DIRECT 會在同一交易內落下
// invite 與 outbox command，但那是「已排程」不是「已送出」——真正的投遞由 outbox
// worker 負責，WISH_POOL 的 projection 仍是 Slice 2c。
//
// Replayed = true 代表這次呼叫沒有建立新的 publication，回的是同一把 clientRequestId
// 先前建立的那一張。
type ExecuteRfqPublicationResult struct {
	PublicationID      int64
	PublicationVersion int
	State              string

	AudienceMode            string
	AudienceHash            string
	SellerFacingPayloadHash string

	PublishedAt time.Time
	Replayed    bool
}

type PublicationExecuteUseCase struct {
	commands  PublicationCommandRepo
	approvals PublicationApprovalExecuteRepo
	runs      PublicationRunWriter
	invites   PublicationInviteRepo
	revisions pricing.RevisionReader
	tx        TxRunner
	// now 同 PublicationPreviewUseCase.now 的既有慣例：預設 time.Now().UTC()，
	// 測試可直接覆寫（同套件內可見）注入固定時間。
	now func() time.Time
}

func NewPublicationExecuteUseCase(
	commands PublicationCommandRepo,
	approvals PublicationApprovalExecuteRepo,
	runs PublicationRunWriter,
	invites PublicationInviteRepo,
	revisions pricing.RevisionReader,
	tx TxRunner,
) *PublicationExecuteUseCase {
	return &PublicationExecuteUseCase{
		commands: commands, approvals: approvals, runs: runs, invites: invites,
		revisions: revisions, tx: tx,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// ExecuteRfqPublication 對一份 approval 執行一次冪等的發布。
//
// 整段流程落在**單一 DB transaction** 內（#2450 Scope 6）：command claim、approval 的
// 兩次 CAS、publication、command 回填、DIRECT 的 invite ＋ outbox command、ledger run ＋
// artifact，任一步失敗全部 rollback。
// 這不只是「乾淨」而已，是三個保證的來源：
//
//  1. **拒絕路徑零寫入。** 每一道 gate 都排在 InsertPublication 之前，但 approval 的
//     PENDING → EXECUTING CAS 排在更前面（它本身就是 exactly-one 的仲裁者），所以
//     「零寫入」只有在 rollback 也涵蓋那次 CAS 時才成立——否則一次 stale 拒絕會把
//     買家的 approval 永久卡在 EXECUTING。
//  2. **失敗不燒掉冪等鍵。** command claim 同樣被 rollback 涵蓋，買家修正問題後可以
//     用同一把 clientRequestId 重送。
//  3. **artifact 與 publication 不會分家。** 只有 publication 沒有 artifact，買家的
//     replay 永遠看不到這次發布。
//
// ⛔ 本 slice 到此為止：DIRECT 的 invite ＋ outbox command（PLATFORM 與 EXTERNAL 皆有，
// Vendor Portal E1 起）已在本交易內落庫，但這裡不發任何對外通知——那由讀 outbox 的
// worker 做；WISH_POOL 的 projection 仍是 Slice 2c。
func (uc *PublicationExecuteUseCase) ExecuteRfqPublication(ctx context.Context, in ExecuteRfqPublicationInput) (ExecuteRfqPublicationResult, error) {
	var out ExecuteRfqPublicationResult
	err := uc.tx.WithTx(ctx, func(txCtx context.Context) error {
		cmd, replayed, err := uc.claimCommand(txCtx, in)
		if err != nil {
			return err
		}
		if replayed {
			// 冪等命中。command 與 publication 是在同一個交易內建立的，所以
			// 「command 在庫裡」蘊含「它的 publication 也在庫裡」——查不到代表
			// 資料不變式被違反，原樣把錯誤回上去，不靜默往下走再建一張。
			pub, findErr := uc.commands.FindPublicationByCommandID(txCtx, cmd.ID)
			if findErr != nil {
				return findErr
			}
			out = publicationResult(pub, true)
			return nil
		}

		approval, recipients, err := uc.gate(txCtx, in)
		if err != nil {
			return err
		}

		now := uc.now()
		pub, err := uc.commands.InsertPublication(txCtx, repository.InsertPublicationInput{
			BuyerID:           in.BuyerID,
			RequestID:         approval.RequestID,
			RequestRevisionID: approval.RequestRevisionID,

			ApprovalID: approval.ID,
			// 記的是**買家核准的那一版**（in.ApprovalVersion），不是 claim 之後被 +1
			// 的那一版：publication 要回答的是「這是依據哪一次核准發出去的」，而
			// procurement_rfq_publication_commands.approval_version 存的也是同一個值，
			// 兩張表對同一件事必須給同一個答案。
			ApprovalVersion: in.ApprovalVersion,

			CommandID:              cmd.ID,
			ConfirmedRfqSnapshotID: approval.ConfirmedRfqSnapshotID,

			SellerFacingPayloadHash: approval.SellerFacingPayloadHash,
			AudienceMode:            approval.AudienceMode,
			AudienceHash:            approval.AudienceHash,

			State:         codemeta.ProcurementPublicationStatePublished.Value,
			SchemaVersion: publicationSchemaVersion,
			PublishedAt:   now,
		})
		if err != nil {
			return err
		}

		// 回放錨點：下一次同 key 重送靠 command 找回這一張（見 schema 該欄註解）。
		if err := uc.commands.SetPublicationCommandPublicationID(txCtx, cmd.ID, pub.ID); err != nil {
			return err
		}

		// DIRECT 的投遞層（Slice 2b）。WISH_POOL 的 projection ＋ index command 是 2c。
		//
		// ⛔ 這一段的任何失敗都讓整筆 execute rollback，那是 Scope 6 既有的保證（單一
		// 交易），不需要新機制：invite 唯一鍵撞到代表上游去重壞了，不是可回放的冪等命中。
		// invites 宣告在 if 外面（而不是 Task 3 原本的 :=）：produceResultArtifact 的
		// counts 由這份真正落庫的 delivery_state 決定，不是重新推導 lane→state 的規則
		// ——那條規則的單一來源是 InsertPublicationInvites（見該檔註解），這裡沿用它的
		// 產出而不是抄一份平行判斷，判斷邏輯改了這裡也不會悄悄跟著漂。WISH_POOL（或空
		// 名單）維持 nil，三個 count 欄位自然全為 0。
		var invites []repository.PublicationInvite
		if len(recipients) > 0 {
			invites, err = uc.invites.InsertPublicationInvites(txCtx, repository.InsertPublicationInvitesInput{
				PublicationID: pub.ID,
				BuyerID:       in.BuyerID,
				Recipients:    recipients,
				Now:           now,
			})
			if err != nil {
				return err
			}
			for _, inv := range invites {
				var (
					eventType string
					payload   []byte
				)
				switch inv.Lane {
				case domain.QuoteLanePlatform:
					eventType = repository.PublicationOutboxEventInviteCreated
					payload, err = json.Marshal(inviteCreatedPayload{
						InviteID:      strconv.FormatInt(inv.ID, 10),
						PublicationID: strconv.FormatInt(pub.ID, 10),
						StoreID:       strconv.FormatInt(inv.StoreID, 10),
					})
				case domain.QuoteLaneExternal:
					eventType = repository.PublicationOutboxEventInviteExternalCreated
					payload, err = json.Marshal(inviteExternalCreatedPayload{
						InviteID:      strconv.FormatInt(inv.ID, 10),
						PublicationID: strconv.FormatInt(pub.ID, 10),
						ProspectID:    strconv.FormatInt(inv.ProspectID, 10),
						ExpiresAt:     inviteExpiry(approval, now).Format(time.RFC3339),
					})
				default:
					return fmt.Errorf("unknown lane %q on invite %d", inv.Lane, inv.ID)
				}
				if err != nil {
					return fmt.Errorf("marshal invite payload: %w", err)
				}
				if err := uc.invites.InsertPublicationOutbox(txCtx, inv.ID, eventType, payload, now); err != nil {
					return err
				}
			}
		}

		if err := uc.produceResultArtifact(txCtx, in.BuyerID, pub, invites); err != nil {
			return err
		}

		// approval.Version 是 claim 之後的版本（ClaimPublicationApprovalForExecute
		// 回的是 UPDATE ... RETURNING 的那一列，version 已經 +1）。
		if err := uc.approvals.MarkPublicationApprovalSucceeded(txCtx, approval.ID, approval.Version); err != nil {
			if errors.Is(err, repository.ErrPublicationApprovalCASFailed) {
				return ErrApprovalStale
			}
			return err
		}

		out = publicationResult(pub, false)
		return nil
	})
	if err != nil {
		return ExecuteRfqPublicationResult{}, err
	}
	return out, nil
}

// claimCommand 取得（或回放）這次 execute 的冪等 claim。
//
// repository.ClaimPublicationCommand 的冪等由 INSERT ... ON CONFLICT 保證：撞鍵不拋錯，
// 交易不會被 25P02 打成 aborted（見 PostgreSQL 規範第十二章）。這一層因此只剩編排：
// 把 isNew 轉成 replayed，錯誤原樣往上回。哨兵：repository 套件的
// TestClaimPublicationCommand_ReplaysInsideCallerTransaction。
//
// Slice 2a 曾經在這一層自己再回查一次冪等鍵。那份在 repository 自我保護之後同時變成多餘
// 與**無測試守著**（拿掉它一切照樣綠），也就是「同一條保護存在兩個地方而其中一份不受
// 約束」——與 #2450 Scope 7 那條 canonical hash 規則被寫兩份是同一種病。
//
// ⛔ 不得在這裡補一次「失敗後回查冪等鍵」的 fallback：ClaimPublicationCommand 回錯就
// 代表這次失敗與冪等鍵無關（撞鍵已經在它裡面被吸收成回放或
// repository.ErrPublicationCommandConflict），此時外層交易可能已經因為別的原因不可用，
// 再送一句查詢只會把真正的錯誤蓋成 25P02。
func (uc *PublicationExecuteUseCase) claimCommand(txCtx context.Context, in ExecuteRfqPublicationInput) (repository.PublicationCommand, bool, error) {
	cmd, isNew, err := uc.commands.ClaimPublicationCommand(txCtx, repository.ClaimPublicationCommandInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID,
		ApprovalID: in.ApprovalID, ApprovalVersion: in.ApprovalVersion,
		ClientRequestID:      in.ClientRequestID,
		CanonicalRequestHash: canonicalPublicationRequestHash(in),
	})
	if err != nil {
		return repository.PublicationCommand{}, false, err
	}
	return cmd, !isNew, nil
}

// gate 是 execute 的完整 CAS gate。⛔ 每一道都必須排在 InsertPublication 之前——
// 拒絕路徑的定義就是「零寫入」（approval 的 CAS 由外層交易的 rollback 還原）。
func (uc *PublicationExecuteUseCase) gate(txCtx context.Context, in ExecuteRfqPublicationInput) (repository.PublicationApproval, []domain.PublicationRecipient, error) {
	_, rev, err := uc.revisions.GetSnapshot(txCtx, in.RequestID, in.BuyerID)
	if err != nil {
		return repository.PublicationApproval{}, nil, err
	}
	if rev.ID != in.ExpectedRevisionID {
		return repository.PublicationApproval{}, nil, ErrRevisionStale
	}

	// PENDING → EXECUTING。述詞含 version，但不是為了擋併發——READ COMMITTED 的
	// EvalPlanQual 下，state = 'PENDING' 自己就擋得下併發輸家（見
	// repository.ErrPublicationApprovalCASFailed 的說明）。version 真正擋的是
	// 非併發的過期輸入：呼叫端手上是更早讀到的 version，state 當下可能仍是
	// PENDING。哨兵：TestClaimPublicationApprovalForExecute_VersionDivergesWhileStatePendingIsRejected。
	approval, err := uc.approvals.ClaimPublicationApprovalForExecute(txCtx, in.ApprovalID, in.ApprovalVersion, in.BuyerID)
	if err != nil {
		if errors.Is(err, repository.ErrPublicationApprovalCASFailed) {
			return repository.PublicationApproval{}, nil, ErrApprovalStale
		}
		return repository.PublicationApproval{}, nil, err
	}

	if approval.SellerFacingPayloadHash != in.PayloadHash {
		return repository.PublicationApproval{}, nil, ErrPayloadHashMismatch
	}

	// Scope 5 的 hard block。讀 approval 上凍下來的值，**不重查 Pricebook**：核准當下
	// 算不出參考價這件事已經發生過了，事後 Pricebook 恰好查得到不會讓那次核准變得有依據。
	//
	// 兩個值的判準不同，理由見上面兩個常數各自的註解：
	//   - PRICE_BASIS_UNAVAILABLE：無條件擋。今天不可達，純防禦。
	//   - UNAVAILABLE：只有在 pricing_mode **沒有**明確宣告買家自訂價時才擋。
	//     它與 BUYER_ENTERED_NO_REFERENCE 的組合是那條模式的正常產出，擋下它等於讓
	//     買家自訂價永遠發不出去；它與其他 mode 的組合則是不自洽的資料（今天的程式
	//     產不出來），擋下才對。
	if approval.ReferenceAvailability == referenceUnavailable {
		return repository.PublicationApproval{}, nil, pricing.ErrPriceBasisUnavailable
	}
	if approval.ReferenceAvailability == legacyReferenceUnavailable &&
		approval.PricingMode != buyerEnteredNoReferencePricingMode {
		return repository.PublicationApproval{}, nil, pricing.ErrPriceBasisUnavailable
	}

	// 這份核准是不是這個 request 的（別把 A 案的核准發成 B 案的詢價）。
	if approval.RequestID != in.RequestID {
		return repository.PublicationApproval{}, nil, ErrApprovalStale
	}
	// 這份核准凍的是不是**現行** revision 的內容。expectedRevisionId 由呼叫端提供，
	// 所以上面那道比對只證明「呼叫端知道現在是哪一版」，證不到「核准是針對那一版做的」：
	// 買家改了需求（revision 前進）之後，帶著新的 revisionId ＋ 舊的 approvalId 送過來
	// 會通過上面那道，卻把舊需求當成新需求發出去。
	if approval.RequestRevisionID != rev.ID {
		return repository.PublicationApproval{}, nil, ErrRevisionStale
	}

	// Scope 5 的 approval expiry。Slice 1 只寫了 expires_at，把它變成 hard block 是
	// 本 slice 的事。
	if uc.now().After(approval.ExpiresAt) {
		return repository.PublicationApproval{}, nil, ErrApprovalExpired
	}

	recipients, err := uc.verifyAudience(txCtx, approval)
	if err != nil {
		return repository.PublicationApproval{}, nil, err
	}
	return approval, recipients, nil
}

// verifyAudience 重驗「發給誰」。
//
// DIRECT_RECIPIENTS 必須把核准當下那份 ordered recipient list 讀回來重算 audienceHash
// 再比對（Scope 5「DIRECT candidate set/version/ordered IDs/seller identities」）——
// hash 不可逆，光有 approval.AudienceHash 重驗不了任何東西，這正是 Task 2b 要把名單
// 落庫的理由。順序是契約的一部分：ListPublicationApprovalRecipients 依 position 排序，
// 換順序就是換受眾（domain.PublicationAudience.Hash 的既有規則）。
//
// WISH_POOL 沒有名單可讀（沒有名單正是這個模式的定義），policy 決定受眾，這裡不需要
// 也無法做等價的重驗。
//
// ⛔ 值域外的 mode 一律擋下，不得靜默落到任一支 pipeline：兩支的完成定義不同
// （doc 64 §4.2），猜錯等於把公開池的詢價當成指定投遞。
//
// 回傳的就是它為了重驗而讀進來的那一份名單（WISH_POOL 為 nil）。投遞層直接沿用它，
// 而不是自己再查一次：第二次查詢在同一交易內拿到的是同一批列，多的只是「兩份名單可能
// 不同」這個本來不存在的問句。
func (uc *PublicationExecuteUseCase) verifyAudience(txCtx context.Context, approval repository.PublicationApproval) ([]domain.PublicationRecipient, error) {
	switch approval.AudienceMode {
	case codemeta.ProcurementPublicationAudienceModeWishPool.Value:
		return nil, nil

	case codemeta.ProcurementPublicationAudienceModeDirectRecipients.Value:
		recipients, err := uc.approvals.ListPublicationApprovalRecipients(txCtx, approval.ID)
		if err != nil {
			return nil, err
		}
		// 空名單不得冒充 WISH_POOL：一個沒有收件人的 DIRECT publication 永遠不會完成，
		// 卻看起來像是成功發布了。核准當下不可能是空的（preview 端擋掉了），所以這裡
		// 代表名單掉了。
		if len(recipients) == 0 {
			return nil, ErrApprovalStale
		}
		audience := domain.DirectRecipientsAudience(
			int64Deref(approval.CandidateSetID),
			intDeref(approval.CandidateSetPolicyVersion),
			recipients,
		)
		got, err := audience.Hash()
		if err != nil {
			return nil, err
		}
		if got != approval.AudienceHash {
			return nil, ErrApprovalStale
		}
		return recipients, nil

	default:
		return nil, ErrUnknownAudienceMode
	}
}

// inviteCreatedPayload 是 outbox 列的 JSONB 內容。
//
// ⛔ 只帶 id：不帶 contact、token、私有預算或任何 Pricebook internals（Scope 9）。
// ID 一律字串（API/Proto 規範：避免 JavaScript 精度問題）。
type inviteCreatedPayload struct {
	InviteID      string `json:"inviteId"`
	PublicationID string `json:"publicationId"`
	StoreID       string `json:"storeId"`
}

type inviteExternalCreatedPayload struct {
	InviteID      string `json:"inviteId"`
	PublicationID string `json:"publicationId"`
	ProspectID    string `json:"prospectId"`
	ExpiresAt     string `json:"expiresAt"`
}

func inviteExpiry(approval repository.PublicationApproval, now time.Time) time.Time {
	if approval.ResponseDeadline == nil {
		return now.Add(30 * 24 * time.Hour)
	}
	loc := time.FixedZone("Asia/Taipei", 8*3600)
	d, err := time.ParseInLocation(domain.ResponseDeadlineLayout, *approval.ResponseDeadline, loc)
	if err != nil {
		return now.Add(30 * 24 * time.Hour)
	}
	endOfDay := time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, loc)
	return endOfDay.Add(7 * 24 * time.Hour).UTC()
}

// publicationResultData 是 RFQ_PUBLICATION_RESULT 的 publicDataJson。
//
// ⛔ **allowlist，恰好六欄，不多不少**（#2450 Scope 9、doc 64 §4.4 的時序圖）：
// 不得含聯絡方式、token、買家預算、Pricebook internals、prompt 或 reasoning。
// 加欄位之前先問「賣家／任何能讀到這個 artifact 的人看到它可以嗎」。
//
// publicationId／version／sellerFacingPayloadHash 這三個是 ID／雜湊，數字以字串過線，
// 同 audiencePreimage 與 proto 的既有慣例（CLAUDE.md §4）。下面三個 count 欄位是刻意的
// 例外：它們是給人看的統計數字，不是 ID，過線用原生 JSON number（無 omitempty——
// WISH_POOL 沒有收件人名單時三欄都是 0，也要讓 client 看到完整形狀，而不是欄位消失）。
type publicationResultData struct {
	PublicationID           string `json:"publicationId"`
	Version                 string `json:"version"`
	SellerFacingPayloadHash string `json:"sellerFacingPayloadHash"`

	// counts 是**發布當下的意圖分布**，不是投遞結果（Scope 9 allowlisted counts）。
	// ⛔ 刻意不含 sentCount：artifact 有 checksum 且不隨 worker 進度重算，放一個
	// 永遠是 0 的欄位會誤導。投遞進度的真相由 #2539 的 typed evidence 提供。
	//
	// recipientCount 在本 slice 恆等於 queuedCount + notSentCount（可推導），保留它是
	// 為了讓 client 不必自己加總，也為了 #2539 加入 sentCount 之後總數仍有一個不隨
	// 投遞進度變動的錨。
	RecipientCount int `json:"recipientCount"`
	QueuedCount    int `json:"queuedCount"`
	NotSentCount   int `json:"notSentCount"`
}

// produceResultArtifact 建 execute 自己的 ledger run 並在上面產一版
// RFQ_PUBLICATION_RESULT（設計決定 1-A）。
//
// 順序是 run → artifact → 終態，不可換：ledger 消費端讀到終態事件就收線，排在終態
// 後面的 ARTIFACT_UPSERTED 等於不存在。
//
// 這裡的失敗**不是**「記 log 就好」：本 slice 的主要價值就是這份 artifact
// （B4a），只留下一張沒有 artifact 的 publication 等於這次 execute 對買家不可見。
// 與 candidate search 那條「ledger 是投影不是 SSOT、失敗只記 log」不同，因為那裡
// 的 SSOT 是 candidate_set，這裡沒有第二個 SSOT。
func (uc *PublicationExecuteUseCase) produceResultArtifact(txCtx context.Context, buyerID identity.BuyerID, pub repository.Publication, invites []repository.PublicationInvite) error {
	runID, err := uc.runs.CreateLedgerRunForPublication(txCtx, repository.CreateLedgerRunForPublicationInput{
		BuyerID: buyerID, RequestID: pub.RequestID, PublicationID: pub.ID,
	})
	if err != nil {
		return err
	}

	// 直接數 invites 每一列真正落庫的 delivery_state，不重新推導 lane→state 規則
	// ——那條規則的單一來源是 InsertPublicationInvites，這裡沿用它的產出。
	// WISH_POOL（或空名單）的 invites 是 nil，三個 count 自然全為 0。
	var queuedCount, notSentCount int
	for _, inv := range invites {
		switch inv.DeliveryState {
		case repository.InviteDeliveryStateQueued:
			queuedCount++
		case repository.InviteDeliveryStateNotSent:
			notSentCount++
		}
	}

	payload, err := json.Marshal(publicationResultData{
		PublicationID:           strconv.FormatInt(pub.ID, 10),
		Version:                 strconv.Itoa(pub.Version),
		SellerFacingPayloadHash: pub.SellerFacingPayloadHash,
		RecipientCount:          len(invites),
		QueuedCount:             queuedCount,
		NotSentCount:            notSentCount,
	})
	if err != nil {
		return fmt.Errorf("marshal publication result artifact: %w", err)
	}

	if _, err := uc.runs.UpsertRunArtifact(txCtx, repository.UpsertRunArtifactInput{
		RunID: runID, BuyerID: buyerID,
		ArtifactType:   codemeta.ProcurementArtifactTypeRfqPublicationResult.Value,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: payload,
		Checksum:       domain.ArtifactChecksum(payload),
		DomainRefType:  publicationRunDomainRefType,
		DomainRefID:    strconv.FormatInt(pub.ID, 10),
		// AsOf 是這版 artifact 據以產製的**事實**時間（doc 57 §21），也就是發布時間，
		// 不是寫入時間。
		AsOf: pub.PublishedAt,
	}); err != nil {
		return err
	}

	wrote, err := uc.runs.FinishLedgerRunForPublication(txCtx, runID, buyerID)
	if err != nil {
		return err
	}
	if !wrote {
		// 這一列 run 是本交易剛建的，不可能已經是終態——擋下代表資料不變式被違反。
		return fmt.Errorf("procurement: publication ledger run %d refused to terminalize", runID)
	}
	return nil
}

func publicationResult(pub repository.Publication, replayed bool) ExecuteRfqPublicationResult {
	return ExecuteRfqPublicationResult{
		PublicationID:      pub.ID,
		PublicationVersion: pub.Version,
		State:              pub.State,

		AudienceMode:            pub.AudienceMode,
		AudienceHash:            pub.AudienceHash,
		SellerFacingPayloadHash: pub.SellerFacingPayloadHash,

		PublishedAt: pub.PublishedAt,
		Replayed:    replayed,
	}
}

// canonicalPublicationRequestPreimage 是 command 的 canonical_request_hash 的白名單投影
// ——「同一把 clientRequestId 送來的是不是同一次請求」的判準（#2450 Scope 7）。
//
// 涵蓋這次 execute 的每一個決定性輸入：換了 request、換了 approval、換了 CAS token、
// 換了 revision、換了 payload hash，都是不同的請求。⛔ 不得漏掉其中任何一個——漏掉的
// 那個欄位就變成「可以偷偷換掉而不算衝突」的東西。
//
// 數字以字串進 preimage，同 domain.audiencePreimage 的理由：避免 JSON number 的精度
// 與格式差異讓同一次請求算出兩個 hash。
type canonicalPublicationRequestPreimage struct {
	RequestID          string `json:"requestId"`
	ApprovalID         string `json:"approvalId"`
	ApprovalVersion    string `json:"approvalVersion"`
	ExpectedRevisionID string `json:"expectedRevisionId"`
	PayloadHash        string `json:"payloadHash"`
}

func canonicalPublicationRequestHash(in ExecuteRfqPublicationInput) string {
	return pricing.HashJSON(canonicalPublicationRequestPreimage{
		RequestID:          strconv.FormatInt(in.RequestID, 10),
		ApprovalID:         strconv.FormatInt(in.ApprovalID, 10),
		ApprovalVersion:    strconv.Itoa(in.ApprovalVersion),
		ExpectedRevisionID: strconv.FormatInt(in.ExpectedRevisionID, 10),
		PayloadHash:        in.PayloadHash,
	})
}

// int64Deref／intDeref 把 approval 上「WISH_POOL 為 nil」的兩個 candidate set 欄位
// 還原成 domain.DirectRecipientsAudience 的值型別參數。只在 DIRECT 分支呼叫，那裡
// 兩者必然非 nil（DB 的 CHECK 保證）；nil 落到 0 與 preview 端 audience 的
// CandidateSetID == 0 走同一條 omitempty 路徑，行為一致。
func int64Deref(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func intDeref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
