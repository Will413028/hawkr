package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ErrPublicationCommandConflict 是同一個 (buyer_id, client_request_id) 撞鍵、但既有列的
// canonical_request_hash 跟這次呼叫的輸入不同時的哨兵錯誤（#2450 Scope 7：同 key 換內容
// 是衝突，零 mutation）。
//
// ⛔ 這把鍵刻意跟 approval 的 preview_client_request_id 分開：Scope 7 明文「preview、
// confirmation、publication preview、execute 各用自己的 stable key，不可跨 phase
// 重用」，否則「重送 preview」與「重送 execute」會互相冒充。
var ErrPublicationCommandConflict = errors.New("procurement: client request id already used for a different publication command")

// ErrPublicationNotFound 是 FindPublicationByCommandID 找不到列時的哨兵錯誤，用途同
// ErrConfirmedRfqSnapshotNotFound／ErrPublicationApprovalNotFound：呼叫端只用它判斷
// 「這個 command 還沒有對應的 publication」，不拿它當存取控制錯誤。
var ErrPublicationNotFound = errors.New("procurement: rfq publication not found")

// PublicationCommand 是 execute 的冪等 claim（#2450 Scope 6 步驟 1）。PublicationID 在
// claim 當下恆為 nil，由編排層在同一個交易內建好 publication 之後才以
// SetPublicationCommandPublicationID 回填——nil 就是「還沒有 publication」的真實狀態，
// 不是欄位缺漏。
type PublicationCommand struct {
	ID      int64
	BuyerID identity.BuyerID

	RequestID       int64
	ApprovalID      int64
	ApprovalVersion int

	ClientRequestID      string
	CanonicalRequestHash string

	PublicationID *int64
}

// ClaimPublicationCommandInput 是一次 claim 的完整輸入。ID 不在這裡：由 r.newID() 產生。
type ClaimPublicationCommandInput struct {
	BuyerID identity.BuyerID

	RequestID       int64
	ApprovalID      int64
	ApprovalVersion int

	ClientRequestID      string
	CanonicalRequestHash string
}

func toPublicationCommandEntity(row gen.ProcurementRfqPublicationCommand) PublicationCommand {
	return PublicationCommand{
		ID: row.ID, BuyerID: identity.BuyerID(row.BuyerID),
		RequestID:       row.RequestID,
		ApprovalID:      row.ApprovalID,
		ApprovalVersion: int(row.ApprovalVersion),

		ClientRequestID:      row.ClientRequestID,
		CanonicalRequestHash: row.CanonicalRequestHash,

		PublicationID: row.PublicationID,
	}
}

// ClaimPublicationCommand 冪等地 claim 一次 execute。第二個回傳值是「這次是不是真的
// claim 到」——false 代表撞到既有列、走重播。
//
// 撞冪等鍵不拋錯（query 用 ON CONFLICT DO UPDATE，見 PostgreSQL 規範第十二章），因此
// 這支方法放進呼叫端的交易裡也不會把它打成 aborted。
//
// ⛔ 回傳既有列前必須跑 CheckReplayable：撞到 (buyer_id, client_request_id) 不代表這次
// 的請求內容跟當初那次相同。
func (r *Repo) ClaimPublicationCommand(ctx context.Context, in ClaimPublicationCommandInput) (PublicationCommand, bool, error) {
	id := r.newID()
	row, err := r.q(ctx).InsertProcurementRfqPublicationCommand(ctx, gen.InsertProcurementRfqPublicationCommandParams{
		ID: id, BuyerID: int64(in.BuyerID), RequestID: in.RequestID,
		ApprovalID:      in.ApprovalID,
		ApprovalVersion: int32(in.ApprovalVersion),

		ClientRequestID:      in.ClientRequestID,
		CanonicalRequestHash: in.CanonicalRequestHash,
	})
	if err != nil {
		return PublicationCommand{}, false, err
	}
	cmd := toPublicationCommandEntity(row)
	// row.ID != id ⇒ 回傳的是既有列，不是這次 claim 到的。
	if row.ID != id {
		if replayErr := cmd.CheckReplayable(in.CanonicalRequestHash); replayErr != nil {
			return PublicationCommand{}, false, replayErr
		}
		return cmd, false, nil
	}
	return cmd, true, nil
}

// CheckReplayable 回報「撞到 (buyer_id, client_request_id) 冪等鍵之後查回來的這一列既有
// command，能不能當成 canonicalRequestHash 這次請求的回放」：hash 相同是重送（nil），
// 不同是 #2450 Scope 7 的衝突（ErrPublicationCommandConflict，零 mutation）。
//
// 具名成一個方法而不是在 ClaimPublicationCommand 裡就地比一次 hash，是因為這條規則曾經
// 被寫成兩份（repository 的復原路徑一份、usecase 編排層的回查路徑一份），兩份可以各自
// 漂移：任一邊改了比對欄位或改了 sentinel，另一邊照樣編譯、照樣測試通過，而使用者會依走
// 的是哪一條路徑拿到不同答案。編排層那一份已移除（commerce/publication_execute.go 的
// claimCommand），所以今天只有上面一個呼叫點——
// 規則本身仍然只留這一份具名定義，下一個需要它的呼叫端要用的是它，不是再抄一次。
func (c PublicationCommand) CheckReplayable(canonicalRequestHash string) error {
	if c.CanonicalRequestHash != canonicalRequestHash {
		return ErrPublicationCommandConflict
	}
	return nil
}

// Publication 是一次 RFQ 發布（#2450 Scope 6 步驟 4）。SellerFacingPayloadHash／
// AudienceMode／AudienceHash 三個從 approval 複製下來，讓 publication 自己就能回答
// 「發出去的是什麼、發給誰」，不必回頭 join approval——見 schema procurement.sql 該表
// 上方的表註解。
//
// ⛔ State = PUBLISHED 只代表 server transaction 完成，不代表 Wish Pool 已建索引、也
// 不代表賣家已收到（Scope 9）；投遞與索引的真相由後續 slice 的 typed evidence 更新，
// 不得由這個欄位推導。
type Publication struct {
	ID      int64
	Version int
	BuyerID identity.BuyerID

	RequestID         int64
	RequestRevisionID int64

	ApprovalID      int64
	ApprovalVersion int

	CommandID              int64
	ConfirmedRfqSnapshotID int64

	SellerFacingPayloadHash string
	AudienceMode            string
	AudienceHash            string

	State         string
	SchemaVersion string

	PublishedAt time.Time
}

// InsertPublicationInput 是一次發布寫入的完整輸入。ID 與 Version 不在這裡：前者由
// r.newID() 產生，後者由 DB 的 DEFAULT 1 決定。
type InsertPublicationInput struct {
	BuyerID identity.BuyerID

	RequestID         int64
	RequestRevisionID int64

	ApprovalID      int64
	ApprovalVersion int

	CommandID              int64
	ConfirmedRfqSnapshotID int64

	SellerFacingPayloadHash string
	AudienceMode            string
	AudienceHash            string

	State         string
	SchemaVersion string

	PublishedAt time.Time
}

func toPublicationEntity(row gen.ProcurementRfqPublication) Publication {
	return Publication{
		ID: row.ID, Version: int(row.Version), BuyerID: identity.BuyerID(row.BuyerID),
		RequestID:         row.RequestID,
		RequestRevisionID: row.RequestRevisionID,

		ApprovalID:      row.ApprovalID,
		ApprovalVersion: int(row.ApprovalVersion),

		CommandID:              row.CommandID,
		ConfirmedRfqSnapshotID: row.ConfirmedRfqSnapshotID,

		SellerFacingPayloadHash: row.SellerFacingPayloadHash,
		AudienceMode:            row.AudienceMode,
		AudienceHash:            row.AudienceHash,

		State:         row.State,
		SchemaVersion: row.SchemaVersion,

		PublishedAt: row.PublishedAt.Time,
	}
}

// InsertPublication 寫入一筆發布。跟 ClaimPublicationCommand 不同，這裡刻意**不做**任何
// 撞鍵後的冪等回放：一個 command 只能有一張 publication 是機械保證
// （uq_procurement_rfq_publications_command），不是冪等鍵——「這個 command 已經有一張
// publication」在正常流程下代表呼叫端邏輯錯誤（同一個 execute claim 不該被處理兩次），
// 不是「重送同一個請求」，所以第二次 INSERT 撞到這個 unique index 時原樣把錯誤回傳給
// 呼叫端，不吞、不重查、不假裝成功。
func (r *Repo) InsertPublication(ctx context.Context, in InsertPublicationInput) (Publication, error) {
	id := r.newID()
	row, err := r.q(ctx).InsertProcurementRfqPublication(ctx, gen.InsertProcurementRfqPublicationParams{
		ID: id, BuyerID: int64(in.BuyerID), RequestID: in.RequestID,
		RequestRevisionID: in.RequestRevisionID,

		ApprovalID:      in.ApprovalID,
		ApprovalVersion: int32(in.ApprovalVersion),

		CommandID:              in.CommandID,
		ConfirmedRfqSnapshotID: in.ConfirmedRfqSnapshotID,

		SellerFacingPayloadHash: in.SellerFacingPayloadHash,
		AudienceMode:            in.AudienceMode,
		AudienceHash:            in.AudienceHash,

		State:         in.State,
		SchemaVersion: in.SchemaVersion,

		PublishedAt: timestamptzParam(in.PublishedAt),
	})
	if err != nil {
		return Publication{}, err
	}
	return toPublicationEntity(row), nil
}

// FindPublicationByCommandID 依 command_id 找出它產生的那一張 publication。找不到回
// ErrPublicationNotFound。
func (r *Repo) FindPublicationByCommandID(ctx context.Context, commandID int64) (Publication, error) {
	row, err := r.q(ctx).FindProcurementRfqPublicationByCommandID(ctx, commandID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Publication{}, ErrPublicationNotFound
		}
		return Publication{}, err
	}
	return toPublicationEntity(row), nil
}

// SetPublicationCommandPublicationID 把 command 指回它建出來的 publication（schema 該欄
// 的註解：失敗或仍在執行時為 NULL）。純 metadata 回填，不是狀態轉移——`publication_id
// IS NULL` 的 guard 讓重複呼叫安全地變成 no-op，真正的 exactly-one 保證是
// uq_procurement_rfq_publications_command。
//
// 命中 0 列不回錯：那代表這個 command 已經有 publication_id 了，是 no-op 的定義本身，
// 不是失敗（同 SetRunConversationID 的既有語意）。
func (r *Repo) SetPublicationCommandPublicationID(ctx context.Context, commandID, publicationID int64) error {
	return r.q(ctx).SetProcurementRfqPublicationCommandPublicationID(ctx, gen.SetProcurementRfqPublicationCommandPublicationIDParams{
		ID: commandID, PublicationID: &publicationID,
	})
}

// CreateLedgerRunForPublicationInput 是 execute 的 ledger run 投影輸入。
type CreateLedgerRunForPublicationInput struct {
	BuyerID       identity.BuyerID
	RequestID     int64
	PublicationID int64
}

// CreateLedgerRunForPublication 為一次 RFQ execute 投影一列 ledger run
// （#2450 Slice 2a 設計決定 1-A），回傳新建的 runID。
//
// 為什麼 execute 需要一個 run：procurement_agent_run_artifacts.run_id 是 **NOT NULL**，
// 而 RFQ_PUBLICATION_RESULT 這一型 artifact 是本 slice 的主要產出——artifact 必須掛在
// 某個 run 上。形狀對齊 CreateLedgerRunForCandidateSearch（domain 動作有自己的 run、
// domain_ref_* 指回領域物件），只是錨點是 publicationId。
//
// ⛔ 與其餘 run type 不同，這一列**沒有 worker**：呼叫端在同一個交易內建它、產 artifact、
// 再由 FinishLedgerRunForPublication 收成 SUCCEEDED，整段是買家的同步動作。所以這裡
// **不**呼叫 r.txManager.WithTx 另開交易——呼叫端已經在一個必須整體 commit 或整體
// rollback 的交易裡（Scope 6），在這裡巢狀開一層只會讓語意變模糊（WithTx 巢狀時本來就
// 沿用外層交易，多包一層不會多一份原子性，只會多一個誤導讀者的邊界）。
func (r *Repo) CreateLedgerRunForPublication(ctx context.Context, in CreateLedgerRunForPublicationInput) (int64, error) {
	q := r.q(ctx)

	// conversationID 解析同 ClaimRun／CreateLedgerRunForCandidateSearch：查不到回
	// 0, nil（不是錯誤），run 的 conversation_id 寫 NULL。execute 恆掛在一個既有
	// request 上，實務上總是解得出來。
	conversationID, err := r.conversationIDForRequest(ctx, in.RequestID)
	if err != nil {
		return 0, err
	}

	runID := r.newID()
	if _, err := q.CreateProcurementAgentRun(ctx, gen.CreateProcurementAgentRunParams{
		ID: runID, BuyerID: int64(in.BuyerID),
		ConversationID: int64OrNil(conversationID),
		RunType:        string(domain.RunTypePublication),
		DomainRefType:  "publicationId", DomainRefID: strconv.FormatInt(in.PublicationID, 10),
		Status: domain.RunStatusQueued(), LastEventSeq: 1,
		ControllerKind: string(domain.ControllerKindDurable),
	}); err != nil {
		return 0, err
	}
	if _, err := q.AppendProcurementAgentRunEvent(ctx, gen.AppendProcurementAgentRunEventParams{
		ID: r.newID(), RunID: runID, Seq: 1, EventType: codemeta.ProcurementLedgerEventTypeRunAccepted.Value,
		SchemaVersion: "v1", PublicPayloadJson: []byte(`{}`), OccurredAt: timestamptzParam(nowUTC()), TraceID: ledgerTraceID(ctx),
	}); err != nil {
		return 0, err
	}
	return runID, nil
}

// FinishLedgerRunForPublication 把 execute 的 ledger run 收成 SUCCEEDED 並補上終態
// event，必須在 artifact 產製**之後**呼叫——ledger 消費端讀到終態 event 就收線，
// 排在它後面的 ARTIFACT_UPSERTED 等於不存在。
//
// 直接走 terminalizeRunTx（而不是匯出的 TerminalizeRun）有兩個理由，都是這個 run
// 特有的：
//
//  1. 呼叫端已經在交易裡。TerminalizeRun 會再包一層 WithTx（巢狀時沿用外層交易，
//     行為相同但邊界多餘）。
//
//  2. TerminalizeRun 在 WithTx 回來之後會發 notifyRunWake，而那是**交易外的另一條
//     連線**——從交易內呼叫它會在 commit 之前就 NOTIFY，正是該函式註解點名要避免的
//     時序。這一列 run 從建立到終態都在同一個同步交易內完成，runId 也不在 wire 上，
//     沒有任何訂閱者可能在等它，所以正確做法是不發，而不是提早發。
//
//     （⚠️ 同一條路徑上的 UpsertRunArtifact 目前仍會在它自己的 WithTx 回來之後
//     發一次 notifyRunWake，對本路徑而言那同樣落在外層 commit 之前。對這一型 run
//     無害（沒有訂閱者），但它是既有共用方法、另外兩個 artifact producer 也在用，
//     時序調整屬獨立變更，這裡不順手改。）
//
// 回傳 wrote=false 代表狀態守衛擋下（這個 run 已經是終態）。對這條路徑而言那是資料
// 不變式被違反——run 是這個交易剛建的，不可能已經終態——呼叫端應當視為錯誤。
func (r *Repo) FinishLedgerRunForPublication(ctx context.Context, runID int64, buyerID identity.BuyerID) (bool, error) {
	return r.terminalizeRunTx(ctx, TerminalizeRunInput{
		RunID: runID, BuyerID: buyerID,
		Status:            domain.RunStatusSucceeded(),
		TerminalEventType: codemeta.ProcurementLedgerEventTypeRunFinished.Value,
		// 字面 `{}`，同 candidate_search 成功路徑實際寫進去的內容（那邊呼叫
		// domain.RunFailedPayload("")，空 reason 就回 `{}`）。這一型 run 沒有可
		// 宣告的 outcome 細節，內容由 RFQ_PUBLICATION_RESULT artifact 承載。
		PublicPayloadJSON: []byte(`{}`),
	})
}

func (r *Repo) FindPublicationByID(ctx context.Context, id int64) (Publication, error) {
	row, err := r.q(ctx).FindProcurementRfqPublicationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Publication{}, ErrPublicationNotFound
		}
		return Publication{}, err
	}
	return toPublicationEntity(row), nil
}

func (r *Repo) FindPublicationInviteByID(ctx context.Context, id int64) (PublicationInvite, error) {
	row, err := r.q(ctx).FindProcurementRfqPublicationInviteByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicationInvite{}, ErrInviteNotFound
		}
		return PublicationInvite{}, err
	}
	return toPublicationInviteEntity(row), nil
}

// FindPublicationInviteForStore 是 PLATFORM 賣家對一張 publication 的可見性判準。
// 找不到即「這個賣家看不到這張」——呼叫端一律回 404，不回 403（doc 64 §9.3：403 會
// 洩漏該 id 存在）。
func (r *Repo) FindPublicationInviteForStore(ctx context.Context, publicationID, storeID int64) (PublicationInvite, error) {
	row, err := r.q(ctx).FindProcurementRfqPublicationInviteByPublicationAndStore(ctx, gen.FindProcurementRfqPublicationInviteByPublicationAndStoreParams{
		PublicationID: publicationID, StoreID: &storeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicationInvite{}, ErrInviteNotFound
		}
		return PublicationInvite{}, err
	}
	return toPublicationInviteEntity(row), nil
}

// PublicationForStore 是賣家清單的一列：publication 本體加上讓這個賣家看得到它的
// 那筆 invite。
type LatestPublicationForRequest struct {
	ID          int64
	Version     int32
	PublishedAt time.Time
}

func (r *Repo) FindLatestPublicationForRequest(ctx context.Context, requestID int64, buyerID identity.BuyerID) (LatestPublicationForRequest, error) {
	row, err := r.q(ctx).FindLatestProcurementRfqPublicationByRequest(ctx, gen.FindLatestProcurementRfqPublicationByRequestParams{
		RequestID: requestID, BuyerID: int64(buyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LatestPublicationForRequest{}, ErrPublicationNotFound
		}
		return LatestPublicationForRequest{}, err
	}
	return LatestPublicationForRequest{
		ID: row.ID, Version: row.Version, PublishedAt: row.PublishedAt.Time,
	}, nil
}

type PublicationForStore struct {
	Publication Publication
	Invite      PublicationInvite
	// SellerView 與詳情端點走同一個轉換：一串不帶品名的 id 對賣家沒有用處。
	SellerView              InviteSellerView
	LatestVersion           *int32
	WithdrawnVersion        *int32
	LatestVersionValidUntil *time.Time
	HasDraft                bool
}

func (r *Repo) ListPublicationsForStore(ctx context.Context, storeID int64, limit int32) ([]PublicationForStore, error) {
	rows, err := r.q(ctx).ListProcurementRfqPublicationsForStore(ctx, gen.ListProcurementRfqPublicationsForStoreParams{
		StoreID: &storeID, RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PublicationForStore, 0, len(rows))
	for _, row := range rows {
		entry := PublicationForStore{
			Publication: toPublicationEntity(row.ProcurementRfqPublication),
			Invite:      toPublicationInviteEntity(row.ProcurementRfqPublicationInvite),
		}
		entry.LatestVersion = row.LatestVersion
		entry.WithdrawnVersion = row.WithdrawnVersion
		entry.LatestVersionValidUntil = row.LatestVersionValidUntil
		entry.HasDraft = row.HasDraft
		// 解不開視同沒有內容：一列壞掉的 payload 不該讓整份清單失敗。
		if view, viewErr := toInviteSellerView(row.SellerFacingPayloadJson, row.ResponseDeadline,
			row.ProcurementRfqPublication.Version); viewErr == nil {
			entry.SellerView = view
		}
		out = append(out, entry)
	}
	return out, nil
}
