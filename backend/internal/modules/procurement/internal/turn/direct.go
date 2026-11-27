package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const sourceRewind = "REWIND"

// Direct 是不經 run 的 SubmitAnswer 與 Rewind（A9-D31）：同步、不建 run、不扣額度；同一個 transaction 先鎖買家的送出、現行版本、需求列，
// 再檢查守衛並寫新版本與輪次（含 turn_result）。分類樹讀取（ITEM 回答）在 transaction 外。重送回放第一次存下的 turn_result，不重算（A9-D33(2)）。
type Direct struct {
	DB *db.DB
	// Resolver 是 ITEM 回答查分類樹用的；沒給時 ITEM 回答回錯誤（組裝錯誤）。
	Resolver Resolver
	Now      func() time.Time
}

type commandKey struct {
	request  uuid.UUID
	source   string
	message  string
	expected uuid.UUID
	slot     model.SlotKey
}

type directState struct {
	prior     priorState
	active    store.ProcurementRequestRevision
	revisions []store.ProcurementRequestRevision
}

type command struct {
	buyer           int64
	request         uuid.UUID
	clientRequestID uuid.UUID
	expected        uuid.UUID
	key             commandKey
	history         bool
	compute         func(directState) (decision, uuid.UUID, error)
}

func errReplayMismatch() error {
	return apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
}

// Answer 回答一個釐清問題（SubmitAnswer）。回傳 TurnResult 的 wire JSON。
// 錯誤：形狀不合（planAnswer、品項文字沒有恰好對到一個末層分類，欄位 FreeText）INVALID_ARGUMENT；不是這位買家的需求或沒有對話 NOT_FOUND；
// 有待回覆的中斷 FAILED_PRECONDITION／APPROVAL_PENDING；同需求有進行中的 run FAILED_PRECONDITION／PROCUREMENT_RUN_IN_PROGRESS；
// 讀到的版本不是現行版本 ABORTED／VERSION_CONFLICT（conflictTarget 是 request_revision）。重送同一個 ClientRequestID 回第一次的結果，內容不同 INVALID_ARGUMENT。
func (d Direct) Answer(ctx context.Context, in AnswerSlotInput) ([]byte, error) {
	plan, err := planAnswer(in)
	if err != nil {
		return nil, err
	}
	c := command{
		buyer: in.BuyerID, request: in.RequestID, clientRequestID: in.ClientRequestID, expected: in.ExpectedRevisionID,
		key: commandKey{request: in.RequestID, source: sourceAnswer, message: plan.message, expected: in.ExpectedRevisionID, slot: plan.slot},
	}
	if out, ok, err := d.replay(ctx, store.New(d.DB.Conn(ctx)), c); err != nil || ok {
		return out, err
	}
	var res *slots.Resolution
	if plan.slot == slots.Item {
		if res, err = d.resolveItem(ctx, plan.text); err != nil {
			return nil, err
		}
	}
	c.compute = func(st directState) (decision, uuid.UUID, error) {
		dec, err := answerDecision(in.Locale, d.Now(), st.prior, plan, res)
		return dec, st.active.ID, err
	}
	return d.execute(ctx, c, sourceAnswer, plan.message)
}

func (d Direct) resolveItem(ctx context.Context, text string) (*slots.Resolution, error) {
	if d.Resolver == nil {
		return nil, errors.New("turn：ITEM 回答需要 Direct.Resolver")
	}
	res, err := d.Resolver.Resolve(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("turn：解析品項：%w", err)
	}
	cands := usable(text, res.Matches, d.Now())
	if len(cands) != 1 {
		return nil, apperr.InvalidInputOn([]string{fieldFreeText}, "這個品項說法對不到唯一的分類，請改用一般訊息描述")
	}
	return cands[0].set.Resolution, nil
}

// Rewind 讓需求退回某個欄位重新回答（A9-D31、A9-D33(3)(4)）：新增一個內容同目標版的新版本（來源 REWIND，祖先是目標版），不重新啟用舊版本。
// 錯誤同 Answer；這個欄位沒有可倒回的內容回 FAILED_PRECONDITION（沒有 reason）。回傳 TurnResult 的 wire JSON。
func (d Direct) Rewind(ctx context.Context, in RewindInput) ([]byte, error) {
	slot, err := rewindSlot(in.Slot)
	if err != nil {
		return nil, err
	}
	c := command{
		buyer: in.BuyerID, request: in.RequestID, clientRequestID: in.ClientRequestID, expected: in.ExpectedRevisionID,
		key:     commandKey{request: in.RequestID, source: sourceRewind, expected: in.ExpectedRevisionID, slot: slot},
		history: true,
	}
	c.compute = func(st directState) (decision, uuid.UUID, error) {
		target, err := rewindTarget(st.revisions, st.active, slot)
		if err != nil {
			return decision{}, uuid.UUID{}, err
		}
		return rewindDecision(in.Locale, st.prior, setFromRevision(*target), slot), target.ID, nil
	}
	return d.execute(ctx, c, sourceRewind, "")
}

func (d Direct) replay(ctx context.Context, q *store.Queries, c command) ([]byte, bool, error) {
	t, err := q.GetTurnByClientRequest(ctx, store.GetTurnByClientRequestParams{BuyerID: c.buyer, ClientRequestID: c.clientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := q.GetRunByIdempotencyKey(ctx, store.GetRunByIdempotencyKeyParams{BuyerID: c.buyer, ClientRequestID: c.clientRequestID}); err == nil {
			return nil, false, errReplayMismatch()
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, fmt.Errorf("turn：讀取重送的 run：%w", err)
		}
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("turn：讀取重送的輪次：%w", err)
	}
	same, err := d.sameCommand(ctx, q, t, c)
	if err != nil {
		return nil, false, err
	}
	if !same {
		return nil, false, errReplayMismatch()
	}
	return t.TurnResult, true, nil
}

func (d Direct) sameCommand(ctx context.Context, q *store.Queries, t store.ProcurementTurn, c command) (bool, error) {
	k := c.key
	if t.RequestID != k.request || t.Source != k.source || t.BuyerMessage != k.message {
		return false, nil
	}
	buyer := c.buyer
	revs, err := q.ListRevisions(ctx, store.ListRevisionsParams{RequestID: t.RequestID, BuyerID: &buyer})
	if err != nil {
		return false, fmt.Errorf("turn：讀取版本：%w", err)
	}
	var result *store.ProcurementRequestRevision
	for i := range revs {
		if revs[i].ID == t.ResultRevisionID {
			result = &revs[i]
		}
	}
	if result == nil {
		return false, fmt.Errorf("turn：輪次 %s 的結果版本不存在", t.ID)
	}
	if result.RevisionNo < 2 || !hasRevision(revs, result.RevisionNo-1, k.expected) {
		return false, nil
	}
	return t.CommandSlot != nil && *t.CommandSlot == string(k.slot), nil
}

func hasRevision(revs []store.ProcurementRequestRevision, no int32, id uuid.UUID) bool {
	for _, r := range revs {
		if r.RevisionNo == no {
			return r.ID == id
		}
	}
	return false
}

func (d Direct) execute(ctx context.Context, c command, source, message string) ([]byte, error) {
	var out []byte
	err := d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(d.DB.Conn(ctx))
		if err := q.LockBuyerSubmits(ctx, c.buyer); err != nil {
			return fmt.Errorf("turn：鎖買家的送出：%w", err)
		}
		if stored, ok, err := d.replay(ctx, q, c); err != nil || ok {
			out = stored
			return err
		}
		st, req, conv, err := d.lockState(ctx, q, c)
		if err != nil {
			return err
		}
		dec, parent, err := c.compute(st)
		if err != nil {
			return err
		}
		now := d.Now()
		id := ids{request: c.request, conversation: conv.ID, revision: uuid.NewV7(), turn: uuid.NewV7()}
		tr, err := turnResultWire(dec, id, now, uuid.UUID{})
		if err != nil {
			return err
		}
		result, err := json.Marshal(tr)
		if err != nil {
			return fmt.Errorf("turn：組 turn_result：%w", err)
		}
		if err := writeTurn(ctx, q, turnWrite{
			ids: id, buyerID: c.buyer, parent: &parent, d: dec, source: source, message: message,
			clientRequestID: c.clientRequestID, fromStatus: req.Status, result: result, commandSlot: c.key.slot, now: now,
		}); err != nil {
			return err
		}
		t, err := q.GetTurnByClientRequest(ctx, store.GetTurnByClientRequestParams{BuyerID: c.buyer, ClientRequestID: c.clientRequestID})
		if err != nil {
			return fmt.Errorf("turn：讀回輪次：%w", err)
		}
		out = t.TurnResult
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d Direct) lockState(ctx context.Context, q *store.Queries, c command) (directState, store.ProcurementRequest, store.ProcurementConversation, error) {
	buyer := c.buyer
	none := func(err error) (directState, store.ProcurementRequest, store.ProcurementConversation, error) {
		return directState{}, store.ProcurementRequest{}, store.ProcurementConversation{}, err
	}
	if _, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: c.request, BuyerID: &buyer}); errors.Is(err, pgx.ErrNoRows) {
		return none(apperr.NotFound("找不到這張需求"))
	} else if err != nil {
		return none(fmt.Errorf("turn：讀取需求：%w", err))
	}
	conv, err := q.GetConversationByRequest(ctx, store.GetConversationByRequestParams{RequestID: c.request, BuyerID: c.buyer})
	if errors.Is(err, pgx.ErrNoRows) {
		return none(apperr.NotFound("找不到這張需求的對話"))
	}
	if err != nil {
		return none(fmt.Errorf("turn：讀取對話：%w", err))
	}
	active, err := q.LockActiveRevision(ctx, store.LockActiveRevisionParams{RequestID: c.request, BuyerID: &buyer})
	hasActive := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return none(fmt.Errorf("turn：鎖現行版本：%w", err))
	}
	req, err := q.LockRequestForBuyer(ctx, store.LockRequestForBuyerParams{ID: c.request, BuyerID: &buyer})
	if err != nil {
		return none(fmt.Errorf("turn：鎖需求：%w", err))
	}
	if _, err := q.GetOpenInterruptForRequest(ctx, store.GetOpenInterruptForRequestParams{RequestID: c.request, BuyerID: c.buyer}); err == nil {
		return none(apperr.FailedPrecondition("這個需求有待回答的問題，請用 ResumeRun 回答").WithReason(apperr.ReasonApprovalPending))
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return none(fmt.Errorf("turn：讀取待回覆的中斷：%w", err))
	}
	if _, err := q.GetInProgressRunForRequest(ctx, store.GetInProgressRunForRequestParams{RequestID: c.request, BuyerID: c.buyer}); err == nil {
		return none(apperr.FailedPrecondition("這個需求已有進行中的 run").WithReason(apperr.ReasonProcurementRunInProgress))
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return none(fmt.Errorf("turn：讀取進行中的 run：%w", err))
	}
	if !hasActive || active.ID != c.expected {
		return none(apperr.VersionConflict("需求已被改版，請重新載入").WithMetadata("conflictTarget", "request_revision"))
	}
	st := directState{active: active}
	if st.prior, err = loadPrior(ctx, q, req, &active, c.buyer, d.Now()); err != nil {
		return none(err)
	}
	if c.history {
		if st.revisions, err = q.ListRevisions(ctx, store.ListRevisionsParams{RequestID: c.request, BuyerID: &buyer}); err != nil {
			return none(fmt.Errorf("turn：讀取版本：%w", err))
		}
	}
	return st, req, conv, nil
}
