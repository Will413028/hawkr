package snapshot

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// GetByRequest 回傳買家自己的需求的快照（以需求為鍵）：現行版本的欄位與缺漏的必填欄位、run、待回答的中斷（含品項消歧卡的分類名稱）與最新的發布結果。
// 需求不存在、不屬於買家，或它的對話已被買家刪除，回 NOT_FOUND（不洩漏存在）。呼叫端的 ctx 不可已在 transaction 內。
func (r Reader) GetByRequest(ctx context.Context, buyerID int64, requestID uuid.UUID) (model.RequestSnapshot, error) {
	var out model.RequestSnapshot
	err := r.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		out, err = r.readRequest(ctx, buyerID, requestID)
		return err
	})
	return out, err
}

func (r Reader) readRequest(ctx context.Context, buyerID int64, requestID uuid.UUID) (model.RequestSnapshot, error) {
	q := store.New(r.DB.Conn(ctx))
	owner := buyerID
	req, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: requestID, BuyerID: &owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RequestSnapshot{}, apperr.NotFound("找不到這個需求")
	}
	if err != nil {
		return model.RequestSnapshot{}, fmt.Errorf("snapshot：讀取需求：%w", err)
	}
	if _, err := q.GetConversationByRequest(ctx, store.GetConversationByRequestParams{RequestID: requestID, BuyerID: buyerID}); errors.Is(err, pgx.ErrNoRows) {
		return model.RequestSnapshot{}, apperr.NotFound("找不到這個需求")
	} else if err != nil {
		return model.RequestSnapshot{}, fmt.Errorf("snapshot：讀取需求的對話：%w", err)
	}
	rev, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: requestID, BuyerID: &owner})
	if err != nil {
		return model.RequestSnapshot{}, fmt.Errorf("snapshot：讀取需求 %s 的現行版本：%w", requestID, err)
	}
	set := slots.FromRevision(rev)
	p, err := r.progress(ctx, q, buyerID, requestID)
	if err != nil {
		return model.RequestSnapshot{}, err
	}
	return model.RequestSnapshot{
		RequestID: req.ID, RevisionID: rev.ID, Status: req.Status, Slots: slotsOf(set), MissingRequired: set.MissingRequired(),
		ItemDisambiguation: p.disambiguation, Runs: p.runs, ActiveRunID: p.active, OpenConfirmation: p.confirmation, PublishedRfq: p.published,
	}, nil
}

func slotsOf(s slots.Set) model.RequirementSlots {
	out := model.RequirementSlots{
		Item: s.Item, Quantity: s.Quantity, Unit: s.Unit, Packaging: s.Packaging, BudgetPerUnit: s.BudgetPerUnit,
		DeliveryCity: s.DeliveryCity, DeliveryDate: s.DeliveryDate, ExactMatchRequired: s.ExactMatchRequired,
	}
	if s.Resolution != nil {
		out.ItemResolution = &model.ItemResolution{Kind: string(s.Resolution.Kind), Label: s.Resolution.Label, DisplayValue: s.Resolution.DisplayValue()}
	}
	return out
}

// Messages 回傳買家自己的對話的訊息（由舊到新）與對話的需求狀態、標題；與 GetRuntimeSnapshot 的訊息同源。
// 對話不存在、已刪除或屬於別的買家回 NOT_FOUND。呼叫端的 ctx 不可已在 transaction 內。
func (r Reader) Messages(ctx context.Context, buyerID int64, conversationID uuid.UUID) (model.ConversationMessages, error) {
	var out model.ConversationMessages
	err := r.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(r.DB.Conn(ctx))
		conv, err := q.GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: conversationID, BuyerID: buyerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這個對話")
		}
		if err != nil {
			return fmt.Errorf("snapshot：讀取對話：%w", err)
		}
		owner := buyerID
		req, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: conv.RequestID, BuyerID: &owner})
		if err != nil {
			return fmt.Errorf("snapshot：讀取需求：%w", err)
		}
		turns, err := q.ListTurnsByRequest(ctx, store.ListTurnsByRequestParams{RequestID: req.ID, BuyerID: buyerID})
		if err != nil {
			return fmt.Errorf("snapshot：讀取輪次：%w", err)
		}
		out = model.ConversationMessages{ConversationID: conv.ID, RequestID: req.ID, Status: req.Status, Title: conv.Title}
		out.Messages, err = messagesOf(turns)
		return err
	})
	return out, err
}
