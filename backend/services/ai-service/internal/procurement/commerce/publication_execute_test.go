package commerce

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

const (
	execBuyerID         = identity.BuyerID(77)
	execRequestID       = int64(4200)
	execRevisionID      = int64(9)
	execApprovalID      = int64(7001)
	execApprovalVersion = 1
	execSnapshotID      = int64(5100)
	execCandSetID       = int64(900)
	execPayloadHash     = "seller-facing-payload-hash"
	execClientRequestID = "execute-key-1"
)

// execNow 是所有測試共用的固定時鐘。approval 的 expires_at 相對它設定，
// 「過期」與「還沒過期」因此是確定的，不受測試執行速度影響。
var execNow = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

// ---- fakes ----

// fakeExecTxRunner 具備真的 rollback 語意，不是「呼叫 fn 就好」的空殼——沒有它，
// 「失敗的 execute 不留下被污染的冪等鍵」只能靠呼叫次數旁證。
type fakeExecTxRunner struct {
	depth int

	txCalls int

	// undo 讓這個 fake 真的具備 rollback 語意：fake repository 每次 mutate 都登記
	// 一個復原動作，交易失敗時逆序執行。沒有它，「零寫入」只能靠呼叫次數旁證，
	// 證不到「失敗的 execute 不會留下被污染的冪等鍵」。
	undo []func()
}

func (f *fakeExecTxRunner) WithTx(ctx context.Context, fn func(context.Context) error) error {
	f.txCalls++
	f.depth++
	mark := len(f.undo)
	err := fn(ctx)
	f.depth--
	if err != nil {
		for i := len(f.undo) - 1; i >= mark; i-- {
			f.undo[i]()
		}
		f.undo = f.undo[:mark]
		return err
	}
	f.undo = f.undo[:mark]
	return nil
}

// stmt 是 fake repository 每個方法的第一行：交易已中止時任何語句都失敗。
func (f *fakeExecTxRunner) onRollback(undo func()) { f.undo = append(f.undo, undo) }

type fakeExecCommandRepo struct {
	tx *fakeExecTxRunner

	commands     map[string]repository.PublicationCommand
	publications map[int64]repository.Publication

	nextCommandID     int64
	nextPublicationID int64

	claimCalls             int
	insertPublicationCalls int
	backfillCalls          int
	lastInsertInput        repository.InsertPublicationInput
	backfilled             map[int64]int64
}

func newFakeExecCommandRepo(tx *fakeExecTxRunner) *fakeExecCommandRepo {
	return &fakeExecCommandRepo{
		tx:                tx,
		commands:          map[string]repository.PublicationCommand{},
		publications:      map[int64]repository.Publication{},
		backfilled:        map[int64]int64{},
		nextCommandID:     8100,
		nextPublicationID: 9100,
	}
}

// execCommandKey 是 uq_procurement_rfq_publication_commands_idempotency 的
// (buyer_id, client_request_id) 在 fake 裡的對應鍵。
func execCommandKey(buyerID identity.BuyerID, clientRequestID string) string {
	return fmt.Sprintf("%d|%s", int64(buyerID), clientRequestID)
}

// ClaimPublicationCommand 模擬 repository/publication.go 的真實實作：INSERT ... ON
// CONFLICT DO UPDATE，撞冪等鍵回既有列且不拋錯。真 DB 哨兵是 repository 套件的
// TestClaimPublicationCommand_ReplaysInsideCallerTransaction。
func (f *fakeExecCommandRepo) ClaimPublicationCommand(ctx context.Context, in repository.ClaimPublicationCommandInput) (repository.PublicationCommand, bool, error) {
	f.claimCalls++
	key := execCommandKey(in.BuyerID, in.ClientRequestID)

	// 對齊真實實作：INSERT ... ON CONFLICT DO UPDATE ⇒ 撞冪等鍵**不拋錯**，交易保持
	// 健康，直接回既有那一列（PostgreSQL 規範第十二章）。這裡刻意不呼叫 f.tx.abort()
	// ——模擬成會 abort 就是在測一個真實實作已經不會發生的狀態。
	if existing, ok := f.commands[key]; ok {
		if err := existing.CheckReplayable(in.CanonicalRequestHash); err != nil {
			return repository.PublicationCommand{}, false, err
		}
		return existing, false, nil
	}
	f.nextCommandID++
	cmd := repository.PublicationCommand{
		ID: f.nextCommandID, BuyerID: in.BuyerID, RequestID: in.RequestID,
		ApprovalID: in.ApprovalID, ApprovalVersion: in.ApprovalVersion,
		ClientRequestID: in.ClientRequestID, CanonicalRequestHash: in.CanonicalRequestHash,
	}
	f.commands[key] = cmd
	f.tx.onRollback(func() { delete(f.commands, key) })
	return cmd, true, nil
}

func (f *fakeExecCommandRepo) InsertPublication(ctx context.Context, in repository.InsertPublicationInput) (repository.Publication, error) {
	f.insertPublicationCalls++
	f.lastInsertInput = in
	f.nextPublicationID++
	pub := repository.Publication{
		ID: f.nextPublicationID, Version: 1, BuyerID: in.BuyerID,
		RequestID: in.RequestID, RequestRevisionID: in.RequestRevisionID,
		ApprovalID: in.ApprovalID, ApprovalVersion: in.ApprovalVersion,
		CommandID: in.CommandID, ConfirmedRfqSnapshotID: in.ConfirmedRfqSnapshotID,
		SellerFacingPayloadHash: in.SellerFacingPayloadHash,
		AudienceMode:            in.AudienceMode, AudienceHash: in.AudienceHash,
		State: in.State, SchemaVersion: in.SchemaVersion, PublishedAt: in.PublishedAt,
	}
	f.publications[in.CommandID] = pub
	f.tx.onRollback(func() { delete(f.publications, in.CommandID) })
	return pub, nil
}

func (f *fakeExecCommandRepo) FindPublicationByCommandID(ctx context.Context, commandID int64) (repository.Publication, error) {
	if pub, ok := f.publications[commandID]; ok {
		return pub, nil
	}
	return repository.Publication{}, repository.ErrPublicationNotFound
}

func (f *fakeExecCommandRepo) SetPublicationCommandPublicationID(ctx context.Context, commandID, publicationID int64) error {
	f.backfillCalls++
	f.backfilled[commandID] = publicationID
	f.tx.onRollback(func() { delete(f.backfilled, commandID) })
	return nil
}

type fakeExecApprovalRepo struct {
	tx *fakeExecTxRunner

	approval      repository.PublicationApproval
	recipients    []domain.PublicationRecipient
	recipientsErr error

	claimCalls   int
	listCalls    int
	succeedCalls int
}

func (f *fakeExecApprovalRepo) ClaimPublicationApprovalForExecute(ctx context.Context, approvalID int64, expectedVersion int, buyerID identity.BuyerID) (repository.PublicationApproval, error) {
	f.claimCalls++
	// 述詞逐字對齊 ClaimProcurementRfqPublicationApprovalForExecute 的
	// WHERE id = $1 AND version = $2 AND buyer_id = $3 AND state = 'PENDING'。
	if approvalID != f.approval.ID || expectedVersion != f.approval.Version ||
		buyerID != f.approval.BuyerID ||
		f.approval.State != codemeta.ProcurementRfqPublicationApprovalStatePending.Value {
		return repository.PublicationApproval{}, repository.ErrPublicationApprovalCASFailed
	}
	prev := f.approval
	f.approval.State = codemeta.ProcurementRfqPublicationApprovalStateExecuting.Value
	f.approval.Version++
	f.tx.onRollback(func() { f.approval = prev })
	return f.approval, nil
}

func (f *fakeExecApprovalRepo) MarkPublicationApprovalSucceeded(ctx context.Context, approvalID int64, expectedVersion int) error {
	f.succeedCalls++
	if approvalID != f.approval.ID || expectedVersion != f.approval.Version ||
		f.approval.State != codemeta.ProcurementRfqPublicationApprovalStateExecuting.Value {
		return repository.ErrPublicationApprovalCASFailed
	}
	prev := f.approval
	f.approval.State = codemeta.ProcurementRfqPublicationApprovalStateSucceeded.Value
	f.approval.Version++
	f.tx.onRollback(func() { f.approval = prev })
	return nil
}

func (f *fakeExecApprovalRepo) ListPublicationApprovalRecipients(ctx context.Context, approvalID int64) ([]domain.PublicationRecipient, error) {
	f.listCalls++
	if f.recipientsErr != nil {
		return nil, f.recipientsErr
	}
	return f.recipients, nil
}

type fakeExecRunWriter struct {
	tx *fakeExecTxRunner

	nextRunID int64

	createCalls   int
	artifactCalls int
	finishCalls   int

	lastRunInput  repository.CreateLedgerRunForPublicationInput
	lastArtifact  repository.UpsertRunArtifactInput
	artifactErr   error
	finishNotDone bool

	// order 記下 ledger 三個動作的先後，用來釘住「artifact 必須排在終態事件之前」
	// ——排在終態後面等於不存在，ledger 消費端讀到終態就收線。
	order []string
}

func (f *fakeExecRunWriter) CreateLedgerRunForPublication(ctx context.Context, in repository.CreateLedgerRunForPublicationInput) (int64, error) {
	f.createCalls++
	f.order = append(f.order, "create")
	f.lastRunInput = in
	f.nextRunID++
	return f.nextRunID, nil
}

func (f *fakeExecRunWriter) UpsertRunArtifact(ctx context.Context, in repository.UpsertRunArtifactInput) (repository.RunArtifact, error) {
	f.artifactCalls++
	if f.artifactErr != nil {
		return repository.RunArtifact{}, f.artifactErr
	}
	f.order = append(f.order, "artifact")
	f.lastArtifact = in
	return repository.RunArtifact{ArtifactID: 1, RunID: in.RunID, ArtifactType: in.ArtifactType, Version: 1}, nil
}

func (f *fakeExecRunWriter) FinishLedgerRunForPublication(ctx context.Context, runID int64, buyerID identity.BuyerID) (bool, error) {
	f.finishCalls++
	f.order = append(f.order, "finish")
	return !f.finishNotDone, nil
}

// fakeExecInviteRepo 是 DIRECT 投遞層的 fake。它模型化 InsertPublicationInvites 的行為：
// 一位收件人一列（順序即 position），起始 delivery_state 一律 QUEUED。
type fakeExecInviteRepo struct {
	tx *fakeExecTxRunner

	nextInviteID int64

	invites []repository.PublicationInvite
	outbox  []fakeExecOutboxRow
}

type fakeExecOutboxRow struct {
	inviteID  int64
	eventType string
	payload   []byte
}

func (f *fakeExecInviteRepo) InsertPublicationInvites(ctx context.Context, in repository.InsertPublicationInvitesInput) ([]repository.PublicationInvite, error) {
	out := make([]repository.PublicationInvite, 0, len(in.Recipients))
	for i, r := range in.Recipients {
		f.nextInviteID++
		out = append(out, repository.PublicationInvite{
			ID: f.nextInviteID, PublicationID: in.PublicationID, BuyerID: int64(in.BuyerID),
			Position: i, Lane: r.Lane, StoreID: r.StoreID, ProspectID: r.ProspectID,
			DeliveryState: repository.InviteDeliveryStateQueued, DeliveryStateAt: in.Now,
		})
	}
	mark := len(f.invites)
	f.invites = append(f.invites, out...)
	f.tx.onRollback(func() { f.invites = f.invites[:mark] })
	return out, nil
}

func (f *fakeExecInviteRepo) InsertPublicationOutbox(ctx context.Context, inviteID int64, eventType string, payload []byte, now time.Time) error {
	mark := len(f.outbox)
	f.outbox = append(f.outbox, fakeExecOutboxRow{inviteID: inviteID, eventType: eventType, payload: payload})
	f.tx.onRollback(func() { f.outbox = f.outbox[:mark] })
	return nil
}

// ---- fixtures ----

type execFixture struct {
	uc        *PublicationExecuteUseCase
	tx        *fakeExecTxRunner
	commands  *fakeExecCommandRepo
	approvals *fakeExecApprovalRepo
	runs      *fakeExecRunWriter
	invites   *fakeExecInviteRepo
}

// wishPoolApproval 是預設的健康 approval：WISH_POOL、參考價可得、尚未過期、
// hash 與 request 都對得上。每一支 rejection 測試**只**破壞它想證明的那一項，
// 其餘全部保持健康——否則測試會被更早的 gate 擋下，斷言的 sentinel 是對的，
// 但被驗證的不是它自己那道 gate。
func wishPoolApproval() repository.PublicationApproval {
	audienceHash, err := domain.WishPoolAudience().Hash()
	if err != nil {
		panic(err)
	}
	return repository.PublicationApproval{
		ID: execApprovalID, Version: execApprovalVersion, BuyerID: execBuyerID,
		RequestID: execRequestID, RequestRevisionID: execRevisionID,
		ConfirmedRfqSnapshotID: execSnapshotID,
		AudienceMode:           codemeta.ProcurementPublicationAudienceModeWishPool.Value,
		AudiencePolicyID:       codemeta.ProcurementWishPoolPolicyAllActiveSellersV1.Value,
		AudiencePolicyVersion:  domain.WishPoolPolicyVersionV1,
		AudienceHash:           audienceHash,

		SellerFacingPayloadHash: execPayloadHash,
		State:                   codemeta.ProcurementRfqPublicationApprovalStatePending.Value,
		ExpiresAt:               execNow.Add(1 * time.Hour),
	}
}

// directApproval 用真正的 domain.DirectRecipientsAudience 算出 audience_hash，
// 所以測試裡的期望值不是自己抄一份實作，而是與 preview 端同一支函式的產物。
func directApproval(recipients ...domain.PublicationRecipient) repository.PublicationApproval {
	a := wishPoolApproval()
	setID := execCandSetID
	policyVersion := 3
	audience := domain.DirectRecipientsAudience(setID, policyVersion, recipients)
	h, err := audience.Hash()
	if err != nil {
		panic(err)
	}
	a.AudienceMode = codemeta.ProcurementPublicationAudienceModeDirectRecipients.Value
	a.AudiencePolicyID = ""
	a.AudiencePolicyVersion = 0
	a.CandidateSetID = &setID
	a.CandidateSetPolicyVersion = &policyVersion
	a.AudienceHash = h
	return a
}

func newExecFixture(approval repository.PublicationApproval, recipients []domain.PublicationRecipient) *execFixture {
	tx := &fakeExecTxRunner{}
	commands := newFakeExecCommandRepo(tx)
	approvals := &fakeExecApprovalRepo{tx: tx, approval: approval, recipients: recipients}
	runs := &fakeExecRunWriter{tx: tx, nextRunID: 6600}
	invites := &fakeExecInviteRepo{tx: tx, nextInviteID: 7700}
	uc := NewPublicationExecuteUseCase(
		commands, approvals, runs, invites,
		testkit.FakeRevisionReader{RevisionID: execRevisionID},
		tx,
	)
	uc.now = func() time.Time { return execNow }
	return &execFixture{uc: uc, tx: tx, commands: commands, approvals: approvals, runs: runs, invites: invites}
}

func execInput() ExecuteRfqPublicationInput {
	return ExecuteRfqPublicationInput{
		BuyerID: execBuyerID, RequestID: execRequestID,
		ClientRequestID: execClientRequestID,
		ApprovalID:      execApprovalID, ApprovalVersion: execApprovalVersion,
		ExpectedRevisionID: execRevisionID,
		PayloadHash:        execPayloadHash,
	}
}

// requireZeroWrites 是每一支 rejection 測試的共同斷言：拒絕路徑不得留下任何寫入。
// 不只看 InsertPublication——approval 的 CAS 也必須被 rollback 還原（否則買家的
// approval 會卡在 EXECUTING、再也沒有第二次機會），command 的冪等鍵也不得被污染
// （否則買家用同一把 key 重送會拿到回放或衝突，而第一次其實什麼都沒成功）。
//
// ⛔ **這裡是本檔「零寫入」的權威列舉，新增一個 port 就要在這裡加一行。** 約十支
// rejection 測試共用它，所以少列一個 port 不是少一個斷言，而是那十支同時對那個 port
// 失明：哪天有人把 invite 寫入移進 gate 或挪到 InsertPublication 之前，一次被拒絕的
// execute 會留下 invite 列，而那十支照樣全綠。
func requireZeroWrites(t *testing.T, f *execFixture) {
	t.Helper()
	require.Zero(t, f.commands.insertPublicationCalls, "拒絕路徑不得呼叫 InsertPublication")
	require.Empty(t, f.commands.publications, "拒絕路徑不得留下 publication")
	require.Empty(t, f.commands.commands, "拒絕路徑不得留下被污染的冪等鍵")
	require.Empty(t, f.commands.backfilled, "拒絕路徑不得回填 command.publication_id")
	require.Zero(t, f.runs.createCalls, "拒絕路徑不得建 ledger run")
	require.Zero(t, f.runs.artifactCalls, "拒絕路徑不得產 artifact")
	require.Empty(t, f.invites.invites, "拒絕路徑不得留下 invite")
	require.Empty(t, f.invites.outbox, "拒絕路徑不得留下 outbox command——"+
		"一個被拒絕的 execute 排出去的投遞任務，worker 會照著送")
	require.Zero(t, f.approvals.succeedCalls, "拒絕路徑不得把 approval 收成 SUCCEEDED")
	require.Equal(t, codemeta.ProcurementRfqPublicationApprovalStatePending.Value, f.approvals.approval.State,
		"拒絕路徑必須把 approval 的 CAS 一起 rollback，不得留在 EXECUTING")
	require.Equal(t, execApprovalVersion, f.approvals.approval.Version,
		"拒絕路徑必須把 approval 的 version 一起 rollback")
}

// ---- tests ----

// happy path：建出 publication、approval 收成 SUCCEEDED、command 指回 publication，
// 全部在同一個交易內。
func TestExecuteRfqPublication_HappyPath_PublishesAndSucceedsApproval(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)

	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.NoError(t, err)
	require.NotZero(t, got.PublicationID)
	require.Equal(t, codemeta.ProcurementPublicationStatePublished.Value, got.State)
	require.False(t, got.Replayed)
	require.Equal(t, execPayloadHash, got.SellerFacingPayloadHash)
	require.Equal(t, codemeta.ProcurementPublicationAudienceModeWishPool.Value, got.AudienceMode)
	require.Equal(t, execNow, got.PublishedAt)
	require.Equal(t, 1, got.PublicationVersion)

	require.Equal(t, 1, f.commands.insertPublicationCalls)
	require.Equal(t, codemeta.ProcurementRfqPublicationApprovalStateSucceeded.Value, f.approvals.approval.State)
	require.Equal(t, got.PublicationID, f.commands.backfilled[f.commands.lastInsertInput.CommandID],
		"command 必須指回它建出來的 publication")
	require.Equal(t, 1, f.tx.txCalls, "整段流程必須落在單一 transaction 內")

	// ledger：run → artifact → 終態，順序不可換。
	require.Equal(t, []string{"create", "artifact", "finish"}, f.runs.order)
	require.Equal(t, got.PublicationID, f.runs.lastRunInput.PublicationID)
	require.Equal(t, codemeta.ProcurementArtifactTypeRfqPublicationResult.Value, f.runs.lastArtifact.ArtifactType)
	require.Equal(t, codemeta.ProcurementArtifactStatusReady.Value, f.runs.lastArtifact.Status)
}

// 冪等回放：同 clientRequestId 同內容第二次呼叫回同一個 publication，不再 INSERT。
//
// ⛔ 這一支驗的是編排層的回放語意，不是「撞鍵不打爆交易」——後者現在由 query 的
// ON CONFLICT 保證，而 fake 無法有意義地模擬那件事（撞鍵根本不再產生錯誤），所以
// 那條斷言留在 repository 的整合測試：
// TestClaimPublicationCommand_ReplaysInsideCallerTransaction（真 Postgres）。
func TestExecuteRfqPublication_SameClientRequestID_ReplaysWithoutSecondInsert(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)

	first, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.NoError(t, err)

	second, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.NoError(t, err)

	require.True(t, second.Replayed, "同 key 同內容重送必須是回放")
	require.Equal(t, first.PublicationID, second.PublicationID)
	require.Equal(t, first.PublishedAt, second.PublishedAt)
	require.Equal(t, 1, f.commands.insertPublicationCalls, "回放不得再建一張 publication")
	require.Equal(t, 1, f.approvals.claimCalls, "回放必須在 approval CAS 之前就短路")
}

// approval version 過期（併發的另一個 execute 已經推進過 version）→ ErrApprovalStale。
func TestExecuteRfqPublication_StaleApprovalVersion_Rejects(t *testing.T) {
	approval := wishPoolApproval()
	approval.Version = 5 // 買家手上那把 CAS token（=1）已經不是現況
	f := newExecFixture(approval, nil)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.ErrorIs(t, err, ErrApprovalStale)
	require.Zero(t, f.commands.insertPublicationCalls)
	require.Empty(t, f.commands.publications)
	require.Empty(t, f.commands.commands, "拒絕路徑不得留下被污染的冪等鍵")
	require.Zero(t, f.runs.createCalls)
	require.Zero(t, f.approvals.succeedCalls)
}

// payloadHash 與 approval 凍下來的 seller_facing_payload_hash 不符 → ErrPayloadHashMismatch。
func TestExecuteRfqPublication_PayloadHashMismatch_Rejects(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)
	in := execInput()
	in.PayloadHash = "someone-elses-payload-hash"

	_, err := f.uc.ExecuteRfqPublication(context.Background(), in)

	require.ErrorIs(t, err, ErrPayloadHashMismatch)
	requireZeroWrites(t, f)
}

// expectedRevisionId 與 active revision 不符 → 重用 Slice 1 的 ErrRevisionStale。
func TestExecuteRfqPublication_RevisionMismatch_Rejects(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)
	in := execInput()
	in.ExpectedRevisionID = execRevisionID + 1

	_, err := f.uc.ExecuteRfqPublication(context.Background(), in)

	require.ErrorIs(t, err, ErrRevisionStale)
	requireZeroWrites(t, f)
	require.Zero(t, f.approvals.claimCalls, "revision 對不上時不該動 approval 的 CAS")
}

// 買家自訂價是唯一的定價方式：沒有任何價格依據的欄位會擋發布，發布只看 approval 的
// 狀態、版本、payloadHash、request、revision 與受眾。
func TestExecuteRfqPublication_BuyerEnteredPrice_Publishes(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)

	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	// 不只斷言「沒有錯誤」——一支 pass 型測試若只看 err == nil，任何提早 return 的實作
	// 都能讓它綠。這裡要求它真的走完並建出 publication、收掉 approval。
	require.NoError(t, err)
	require.NotZero(t, got.PublicationID)
	require.Equal(t, codemeta.ProcurementPublicationStatePublished.Value, got.State)
	require.Equal(t, 1, f.commands.insertPublicationCalls)
	require.Equal(t, codemeta.ProcurementRfqPublicationApprovalStateSucceeded.Value, f.approvals.approval.State)
}

// approval 已過期（expires_at 早於 now）→ ErrApprovalExpired。
//
// approval 的其餘每一項都是健康的（state PENDING、version 對得上、payloadHash 相符、
// requestId 相符、WISH_POOL 的 audienceHash 相符），
// 唯一被破壞的是 expires_at——沒有 expiry gate 的實作會一路走到 InsertPublication。
func TestExecuteRfqPublication_ExpiredApproval_Rejects(t *testing.T) {
	approval := wishPoolApproval()
	approval.ExpiresAt = execNow.Add(-1 * time.Second)
	f := newExecFixture(approval, nil)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.ErrorIs(t, err, ErrApprovalExpired)
	requireZeroWrites(t, f)
}

// DIRECT 的 ordered recipient list 被竄改（這裡是**換順序**，不是換人）→ 重算的
// audienceHash 與 approval 上的不符 → ErrApprovalStale。
//
// 用換順序而不是換人，是為了同時釘住「順序是契約的一部分」：一個只比對「集合相同」
// 的實作會在這裡放行。
func TestExecuteRfqPublication_TamperedRecipientOrder_Rejects(t *testing.T) {
	approved := []domain.PublicationRecipient{
		domain.PlatformRecipient(501),
		domain.PlatformRecipient(502),
		domain.ExternalRecipient(9001),
	}
	tampered := []domain.PublicationRecipient{
		domain.PlatformRecipient(502),
		domain.PlatformRecipient(501),
		domain.ExternalRecipient(9001),
	}
	f := newExecFixture(directApproval(approved...), tampered)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.ErrorIs(t, err, ErrApprovalStale)
	require.Equal(t, 1, f.approvals.listCalls, "DIRECT 必須真的把名單讀回來重算")
	requireZeroWrites(t, f)
}

// DIRECT 且名單沒有被動過 → 重算的 hash 相符，正常發布。
// 這一支是上一支的對照組：證明上一支紅的原因是「順序被換了」，不是「DIRECT 一律被擋」。
func TestExecuteRfqPublication_DirectRecipients_IntactList_Publishes(t *testing.T) {
	approved := []domain.PublicationRecipient{
		domain.PlatformRecipient(501),
		domain.ExternalRecipient(9001),
	}
	f := newExecFixture(directApproval(approved...), approved)

	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.NoError(t, err)
	require.Equal(t, codemeta.ProcurementPublicationAudienceModeDirectRecipients.Value, got.AudienceMode)
	require.Equal(t, 1, f.commands.insertPublicationCalls)

	// Vendor Portal E1：兩個 lane 現在同形，每位收件人一列 invite ＋ 一列 outbox。
	require.Len(t, f.invites.invites, 2, "每位收件人一列 invite")
	require.Len(t, f.invites.outbox, 2, "PLATFORM 與 EXTERNAL 都排投遞")
	require.Equal(t, f.invites.invites[0].ID, f.invites.outbox[0].inviteID,
		"第一列 outbox 綁在 PLATFORM 的 invite 上")
	require.Equal(t, repository.PublicationOutboxEventInviteCreated, f.invites.outbox[0].eventType)
	require.Equal(t, f.invites.invites[1].ID, f.invites.outbox[1].inviteID,
		"第二列 outbox 綁在 EXTERNAL 的 invite 上")
	require.Equal(t, repository.PublicationOutboxEventInviteExternalCreated, f.invites.outbox[1].eventType)
}

// EXTERNAL 現在有真實投遞管道（Vendor Portal E1），所以與 PLATFORM 同形：起始
// QUEUED 且照樣寫 outbox 列。這翻掉 Slice 2b 的決定，前提是那時不存在管道。
func TestExecuteRfqPublication_ExternalWritesQueuedInviteAndOutbox(t *testing.T) {
	approved := []domain.PublicationRecipient{domain.ExternalRecipient(9001)}
	f := newExecFixture(directApproval(approved...), approved)

	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.NoError(t, err)

	require.Len(t, f.invites.invites, 1)
	inv := f.invites.invites[0]
	require.Equal(t, domain.QuoteLaneExternal, inv.Lane)
	require.Equal(t, repository.InviteDeliveryStateQueued, inv.DeliveryState)

	require.Len(t, f.invites.outbox, 1)
	row := f.invites.outbox[0]
	require.Equal(t, inv.ID, row.inviteID)
	require.Equal(t, repository.PublicationOutboxEventInviteExternalCreated, row.eventType)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(row.payload, &payload))
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	require.Equal(t, []string{"expiresAt", "inviteId", "prospectId", "publicationId"}, keys)
	require.Equal(t, strconv.FormatInt(inv.ID, 10), payload["inviteId"])
	require.Equal(t, strconv.FormatInt(got.PublicationID, 10), payload["publicationId"])
	require.Equal(t, strconv.FormatInt(inv.ProspectID, 10), payload["prospectId"])
	require.NotContains(t, payload, "storeId", "EXTERNAL 沒有 storeId，混用會讓收件人解析不到")

	require.Equal(t, execNow.Add(30*24*time.Hour).Format(time.RFC3339), payload["expiresAt"],
		"沒有 response_deadline 時到期規則是 now+30 天")
}

// inviteExpiry 的兩條分支：有 response_deadline 用該日 23:59:59（+08:00）加 7 天寬限；
// 沒有則是 now+30 天（對齊 v6.0 §5.3 的 read_until 慣例）。
func TestInviteExpiry(t *testing.T) {
	now := time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC)

	t.Run("no response deadline uses now plus 30 days", func(t *testing.T) {
		got := inviteExpiry(repository.PublicationApproval{}, now)
		require.Equal(t, now.Add(30*24*time.Hour), got)
	})

	t.Run("response deadline uses Asia/Taipei end of day plus 7 days grace", func(t *testing.T) {
		deadline := "2026-09-15"
		got := inviteExpiry(repository.PublicationApproval{ResponseDeadline: &deadline}, now)

		loc := time.FixedZone("Asia/Taipei", 8*3600)
		want := time.Date(2026, 9, 15, 23, 59, 59, 0, loc).Add(7 * 24 * time.Hour).UTC()
		require.Equal(t, want, got)
	})
}

// inviteCreatedPayload 的三個 JSON key 是**跨服務線材**，不是內部欄位名：Task 7 的
// notification-service template 用 inviteId 綁 reference_id，storeId 決定這張 RFQ 送到
// 哪一家店。打錯字（inviteID）或把兩個 id 填反都編得過、也不會讓任何既有測試變紅——
// 只會在別的服務或別的賣家身上爆開，所以這裡逐字釘住。
//
// ⚠️ 相鄰的 legacy publisher 發的是 {"inviteId":…,"store_id":…}（store key 是
// snake_case）。**這一支不是那個形狀**，別讓那個
// 慣例滲進來。
//
// 整個 map 一起比對而不是逐 key assert：多一個 key 也會紅，allowlist 因此同時成立
// （同 TestExecuteRfqPublication_ResultArtifact_PublicDataIsAllowlisted 的理由）。
func TestExecuteRfqPublication_InviteCreatedPayload_PinsWireKeys(t *testing.T) {
	approved := []domain.PublicationRecipient{domain.PlatformRecipient(501)}
	f := newExecFixture(directApproval(approved...), approved)

	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.NoError(t, err)
	require.Len(t, f.invites.invites, 1)
	require.Len(t, f.invites.outbox, 1)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(f.invites.outbox[0].payload, &wire))
	require.Equal(t, map[string]any{
		"inviteId":      strconv.FormatInt(f.invites.invites[0].ID, 10),
		"publicationId": strconv.FormatInt(got.PublicationID, 10),
		"storeId":       "501",
	}, wire, "outbox payload 恰好這三個 camelCase key，值都是字串化的 id（避免 JavaScript 精度問題）")
}

// approval 屬於另一個 request → ErrApprovalStale（別把 A 案的核准發成 B 案的詢價）。
func TestExecuteRfqPublication_ApprovalBelongsToAnotherRequest_Rejects(t *testing.T) {
	approval := wishPoolApproval()
	approval.RequestID = execRequestID + 1
	f := newExecFixture(approval, nil)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.ErrorIs(t, err, ErrApprovalStale)
	requireZeroWrites(t, f)
}

// approval 是針對更早的 revision 核准的，即使呼叫端帶的 expectedRevisionId 是最新的
// → ErrRevisionStale。
//
// ⛔ 這一支不在 task brief 的清單裡，但它補的是一個真的洞：expectedRevisionId 由呼叫端
// 提供，所以「買家改了需求 → revision 前進 → 前端帶新的 revisionId ＋ 舊的 approvalId」
// 這條路徑會通過 gate 2（rev.ID == expected），而 approval 凍的仍是舊 revision 的內容。
// 沒有這道比對，舊需求會被當成新需求發出去。
func TestExecuteRfqPublication_ApprovalFrozenOnOlderRevision_Rejects(t *testing.T) {
	approval := wishPoolApproval()
	approval.RequestRevisionID = execRevisionID - 1
	f := newExecFixture(approval, nil)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.ErrorIs(t, err, ErrRevisionStale)
	requireZeroWrites(t, f)
}

// 同一把 clientRequestId 換了內容 → repository.ErrPublicationCommandConflict（Scope 7），
// 零 mutation。
func TestExecuteRfqPublication_SameKeyDifferentContent_ReturnsConflict(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.NoError(t, err)

	// 這裡不能用 requireZeroWrites：上面那次成功呼叫本來就合法留下了 claim、
	// backfill 與 approval SUCCEEDED。改成記下那次成功呼叫留下的狀態，斷言衝突
	// 路徑不會在這個基礎上再多做任何 mutation——而不是斷言「什麼都沒發生」。
	wantClaimCalls := f.approvals.claimCalls
	wantSucceedCalls := f.approvals.succeedCalls
	wantApprovalState := f.approvals.approval.State
	wantApprovalVersion := f.approvals.approval.Version
	wantBackfillCalls := f.commands.backfillCalls

	// 同一把 key，但換了 approval——canonical request hash 因此不同。
	in := execInput()
	in.ApprovalID = execApprovalID + 1
	_, err = f.uc.ExecuteRfqPublication(context.Background(), in)

	require.ErrorIs(t, err, repository.ErrPublicationCommandConflict)
	require.Equal(t, 1, f.commands.insertPublicationCalls, "衝突不得建第二張 publication")
	require.Equal(t, wantClaimCalls, f.approvals.claimCalls, "衝突不得再 claim 一次 approval")
	require.Equal(t, wantSucceedCalls, f.approvals.succeedCalls, "衝突不得把 approval 再收成 SUCCEEDED 一次")
	require.Equal(t, wantApprovalState, f.approvals.approval.State, "衝突不得推進 approval 的狀態")
	require.Equal(t, wantApprovalVersion, f.approvals.approval.Version, "衝突不得推進 approval 的 version")
	require.Equal(t, wantBackfillCalls, f.commands.backfillCalls, "衝突不得再 backfill 一次 command.publication_id")
}

// 被拒絕的 execute 不得污染冪等鍵：同一把 key 在問題修正後必須還能用。
//
// 一次失敗的 execute 若把 command 留在庫裡，買家用同一把 key 重送會被當成回放
// （而那次根本沒有 publication）或衝突——等於一次可復原的 stale 拒絕永久燒掉一把 key。
func TestExecuteRfqPublication_RejectedAttempt_DoesNotBurnIdempotencyKey(t *testing.T) {
	approval := wishPoolApproval()
	approval.ExpiresAt = execNow.Add(-1 * time.Second)
	f := newExecFixture(approval, nil)

	_, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.ErrorIs(t, err, ErrApprovalExpired)
	require.Empty(t, f.commands.commands)

	// 買家重新核准（新的 approval 有效期），用同一把 execute key 再送一次。
	f.approvals.approval = wishPoolApproval()
	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())

	require.NoError(t, err)
	require.False(t, got.Replayed, "上一次失敗沒有留下 command，這次是全新的一次 execute")
	require.NotZero(t, got.PublicationID)
}

// RFQ_PUBLICATION_RESULT 的 publicDataJson 是 allowlist：恰好六欄，不多不少
// （Scope 9、doc 64 §4.4）。⛔ 不得含 contact、token、買家預算、內部定價資料、
// prompt／reasoning。
//
// 這支跑的是 wishPoolApproval（無收件人名單），刻意用它釘住 WISH_POOL 這條路徑的
// 三個 count 欄位：不是欄位消失（沒有 omitempty），是恆為 0——買家看到的永遠是同一個
// 完整形狀，只是這次發布沒有可數的收件人。
func TestExecuteRfqPublication_ResultArtifact_PublicDataIsAllowlisted(t *testing.T) {
	f := newExecFixture(wishPoolApproval(), nil)

	got, err := f.uc.ExecuteRfqPublication(context.Background(), execInput())
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(f.runs.lastArtifact.PublicDataJSON, &payload))

	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	require.ElementsMatch(t,
		[]string{
			"publicationId", "version", "sellerFacingPayloadHash",
			"recipientCount", "queuedCount", "notSentCount",
		}, keys, "publicDataJson 是 allowlist，恰好這六欄")
	require.Equal(t, execPayloadHash, payload["sellerFacingPayloadHash"])
	require.Equal(t, float64(0), payload["recipientCount"], "WISH_POOL 沒有收件人名單")
	require.Equal(t, float64(0), payload["queuedCount"], "WISH_POOL 沒有收件人名單")
	require.Equal(t, float64(0), payload["notSentCount"], "WISH_POOL 沒有收件人名單")
	require.Equal(t, domain.ArtifactChecksum(f.runs.lastArtifact.PublicDataJSON), f.runs.lastArtifact.Checksum)
	require.Equal(t, "publicationId", f.runs.lastArtifact.DomainRefType)
	require.NotZero(t, got.PublicationID)
}
