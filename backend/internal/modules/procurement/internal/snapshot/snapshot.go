// Package snapshot 是 GetRuntimeSnapshot 的讀取模型（A9-D30）：把一個對話畫面需要的事實在同一份快照內讀齊。
package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	slotKeyPrefix = "SLOT_KEY_"
	roleUser      = "USER"
	roleAssistant = "ASSISTANT"
)

// Reader 讀對話的快照。
type Reader struct {
	DB     *db.DB
	Ledger run.Ledger
}

// Get 回傳買家自己的對話的快照；不存在、已刪除或屬於別的買家回 NOT_FOUND（不洩漏存在）。
// 呼叫端的 ctx 不可已在 transaction 內（InReadSnapshot）。
func (r Reader) Get(ctx context.Context, buyerID int64, conversationID uuid.UUID) (model.RuntimeSnapshot, error) {
	var out model.RuntimeSnapshot
	err := r.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		out, err = r.read(ctx, buyerID, conversationID)
		return err
	})
	return out, err
}

func (r Reader) read(ctx context.Context, buyerID int64, conversationID uuid.UUID) (model.RuntimeSnapshot, error) {
	q := store.New(r.DB.Conn(ctx))
	conv, err := q.GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: conversationID, BuyerID: buyerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RuntimeSnapshot{}, apperr.NotFound("找不到這個對話")
	}
	if err != nil {
		return model.RuntimeSnapshot{}, fmt.Errorf("snapshot：讀取對話：%w", err)
	}
	owner := buyerID
	req, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: conv.RequestID, BuyerID: &owner})
	if err != nil {
		return model.RuntimeSnapshot{}, fmt.Errorf("snapshot：讀取需求：%w", err)
	}
	out := model.RuntimeSnapshot{ConversationID: conv.ID, RequestID: req.ID, Status: req.Status, Title: conv.Title}

	turns, err := q.ListTurnsByRequest(ctx, store.ListTurnsByRequestParams{RequestID: req.ID, BuyerID: buyerID})
	if err != nil {
		return model.RuntimeSnapshot{}, fmt.Errorf("snapshot：讀取輪次：%w", err)
	}
	if out.Messages, err = messagesOf(turns); err != nil {
		return model.RuntimeSnapshot{}, err
	}

	runs, err := q.ListRunsByRequest(ctx, store.ListRunsByRequestParams{RequestID: req.ID, BuyerID: buyerID, MaxRows: model.RuntimeSnapshotMaxRuns})
	if err != nil {
		return model.RuntimeSnapshot{}, fmt.Errorf("snapshot：讀取 run：%w", err)
	}
	for _, x := range runs {
		out.Runs = append(out.Runs, model.RunSummary{
			RunID: x.ID, RunType: x.RunType, Status: x.Status, LastCursor: run.Cursor(x.LastEventSeq), Terminal: run.IsTerminal(x.Status),
		})
	}

	active, err := q.GetInProgressRunForRequest(ctx, store.GetInProgressRunForRequestParams{RequestID: req.ID, BuyerID: buyerID})
	switch {
	case err == nil:
		out.ActiveRunID = &active.ID
	case !errors.Is(err, pgx.ErrNoRows):
		return model.RuntimeSnapshot{}, fmt.Errorf("snapshot：讀取進行中的 run：%w", err)
	}

	irq, found, err := r.Ledger.OpenInterruptForRequest(ctx, buyerID, req.ID)
	if err != nil {
		return model.RuntimeSnapshot{}, err
	}
	if found {
		if out.OpenConfirmation, err = confirmationOf(irq); err != nil {
			return model.RuntimeSnapshot{}, err
		}
	}

	pub, err := q.GetLatestPublicationByRequest(ctx, store.GetLatestPublicationByRequestParams{RequestID: req.ID, BuyerID: &owner})
	switch {
	case err == nil:
		out.PublishedRfq = &model.PublishedRfq{
			PublicationID: pub.ID, Version: pub.Version, PublishTime: pub.PublishedAt,
			RecipientCount: pub.RecipientCount, QueuedCount: pub.QueuedCount, ConfirmedRfqSnapshotID: pub.ConfirmedRfqSnapshotID,
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return model.RuntimeSnapshot{}, fmt.Errorf("snapshot：讀取發布結果：%w", err)
	}
	return out, nil
}

type recordedWire struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// messagesOf 把每一輪拆成買家與助理兩則訊息（空的那則不出）。
func messagesOf(turns []store.ProcurementTurn) ([]model.ConversationMessage, error) {
	var out []model.ConversationMessage
	for _, t := range turns {
		if t.BuyerMessage != "" {
			out = append(out, model.ConversationMessage{ID: t.ClientRequestID, Role: roleUser, Text: t.BuyerMessage, CreateTime: t.CreatedAt})
		}
		var rec []recordedWire
		if err := json.Unmarshal(t.RecordedThisTurn, &rec); err != nil {
			return nil, fmt.Errorf("snapshot：解析輪次 %s 的 recorded_this_turn：%w", t.ID, err)
		}
		m := model.ConversationMessage{ID: t.ID, Role: roleAssistant, Text: t.AssistantReply, CreateTime: t.CreatedAt}
		for _, f := range rec {
			m.RecordedThisTurn = append(m.RecordedThisTurn, model.RecordedField{Key: slotKeyOf(f.Key), Label: f.Label, Value: f.Value})
		}
		if m.Text != "" || len(m.RecordedThisTurn) > 0 {
			out = append(out, m)
		}
	}
	return out, nil
}

func confirmationOf(i run.OpenInterrupt) (*model.OpenConfirmation, error) {
	var p turn.InterruptPayload
	if err := json.Unmarshal(i.Payload, &p); err != nil {
		return nil, fmt.Errorf("snapshot：解析中斷 %s 的 payload：%w", i.ID, err)
	}
	return &model.OpenConfirmation{
		InterruptID: i.ID, Kind: i.InterruptKind, Processing: i.Processing,
		Question: model.ClarifyQuestion{
			SlotKey: slotKeyOf(p.Question.SlotKey), Prompt: p.Question.Prompt,
			Options: optionsOf(p.Question.Options), UnitOptions: optionsOf(p.Question.UnitOptions),
			AllowFreeText: p.Question.AllowFreeText, CanSkip: p.Question.CanSkip,
		},
	}, nil
}

func optionsOf(in []turn.OptionJSON) []model.QuestionOption {
	var out []model.QuestionOption
	for _, o := range in {
		out = append(out, model.QuestionOption{OptionID: o.OptionID, Label: o.Label})
	}
	return out
}

func slotKeyOf(wire string) model.SlotKey {
	return model.SlotKey(strings.TrimPrefix(wire, slotKeyPrefix))
}
