// Package read 是 im 的讀取路徑：對話列表、訊息、未讀。讀取方法不寫任何狀態（已讀只走 MarkRead）。
package read

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Deps 是讀取路徑的依賴。
type Deps struct{ DB *db.DB }

// Service 是讀取路徑的 use case。
type Service struct{ d Deps }

// New 建立讀取路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func pageSize(size int32) int32 {
	switch {
	case size <= 0:
		return model.DefaultPageSize
	case size > model.MaxPageSize:
		return model.MaxPageSize
	}
	return size
}

// authorized 讀對話並確認呼叫者是其中一方；不存在與不屬於呼叫者同為 NotFound（不洩漏對話存在）。
func (s *Service) authorized(ctx context.Context, id uuid.UUID, p model.Party) (store.ImConversation, error) {
	if err := p.Authorize(); err != nil {
		return store.ImConversation{}, err
	}
	c, err := s.q(ctx).GetConversation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ImConversation{}, apperr.NotFound("%s", text.ImConversationNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return store.ImConversation{}, err
	}
	if (p.Kind == model.PartyBuyer && c.BuyerID != p.BuyerID) || (p.Kind == model.PartyStore && c.StoreID != p.Store.StoreID) {
		return store.ImConversation{}, apperr.NotFound("%s", text.ImConversationNotFound(uitext.LocaleFrom(ctx)))
	}
	return c, nil
}

// ListConversations 是呼叫者這一方的對話，依 (last_message_at, id) 新的在前做 keyset；不回總數，也不因封鎖隱藏對話。
func (s *Service) ListConversations(ctx context.Context, p model.Party, after *model.Cursor, size int32) (model.ConversationPage, error) {
	if err := p.Authorize(); err != nil {
		return model.ConversationPage{}, err
	}
	n := pageSize(size)
	var afterAt *time.Time
	var afterID *uuid.UUID
	if after != nil {
		afterAt = &after.At
		afterID = &after.ID
	}
	var items []model.Conversation
	if p.Kind == model.PartyBuyer {
		rs, err := s.q(ctx).ListConversationsBuyer(ctx, store.ListConversationsBuyerParams{BuyerID: p.BuyerID, AfterAt: afterAt, AfterID: afterID, MaxRows: n + 1})
		if err != nil {
			return model.ConversationPage{}, err
		}
		for _, r := range rs {
			items = append(items, conversation(r.ID, r.BuyerID, r.StoreID, r.LastSeq, r.ReadSeq, r.LastMessageAt, r.SenderKind, r.MessageType, r.Content, r.ErasedAt, r.Hidden))
		}
	} else {
		rs, err := s.q(ctx).ListConversationsStore(ctx, store.ListConversationsStoreParams{StoreID: p.Store.StoreID, AfterAt: afterAt, AfterID: afterID, MaxRows: n + 1})
		if err != nil {
			return model.ConversationPage{}, err
		}
		for _, r := range rs {
			items = append(items, conversation(r.ID, r.BuyerID, r.StoreID, r.LastSeq, r.ReadSeq, r.LastMessageAt, r.SenderKind, r.MessageType, r.Content, r.ErasedAt, r.Hidden))
		}
	}
	page := model.ConversationPage{Items: items}
	if len(items) > int(n) {
		page.Items = items[:n]
		last := page.Items[n-1]
		page.Next = &model.Cursor{At: last.LastMessageAt, ID: last.ID}
	}
	return page, nil
}

func conversation(id uuid.UUID, buyerID, storeID, lastSeq, readSeq int64, at time.Time, senderKind, msgType string, content *string, erasedAt *time.Time, hidden bool) model.Conversation {
	return model.Conversation{
		ID: id, BuyerID: buyerID, StoreID: storeID, LastSeq: lastSeq, Unread: lastSeq - readSeq, LastMessageAt: at,
		Preview: rows.PreviewOf(senderKind, msgType, content, erasedAt, hidden),
	}
}

// ListMessages 是對話裡 seq 小於 beforeSeq 的訊息（新的在前；beforeSeq 為 0 從最新開始），以 seq 做 keyset，不回總數。
func (s *Service) ListMessages(ctx context.Context, conversationID uuid.UUID, p model.Party, beforeSeq int64, size int32) (model.MessagePage, error) {
	if _, err := s.authorized(ctx, conversationID, p); err != nil {
		return model.MessagePage{}, err
	}
	n := pageSize(size)
	rs, err := s.q(ctx).ListMessagesBefore(ctx, store.ListMessagesBeforeParams{ConversationID: conversationID, BeforeSeq: beforeSeq, MaxRows: n + 1})
	if err != nil {
		return model.MessagePage{}, err
	}
	items := rows.Before(rs)
	return page(items, n), nil
}

// MessagesAfter 是對話裡 seq 大於 afterSeq 的訊息（舊的在前），給 2b 的斷線補洞。
func (s *Service) MessagesAfter(ctx context.Context, conversationID uuid.UUID, p model.Party, afterSeq int64, size int32) (model.MessagePage, error) {
	if _, err := s.authorized(ctx, conversationID, p); err != nil {
		return model.MessagePage{}, err
	}
	n := pageSize(size)
	rs, err := s.q(ctx).ListMessagesAfter(ctx, store.ListMessagesAfterParams{ConversationID: conversationID, AfterSeq: afterSeq, MaxRows: n + 1})
	if err != nil {
		return model.MessagePage{}, err
	}
	return page(rows.After(rs), n), nil
}

func page(items []model.Message, n int32) model.MessagePage {
	if len(items) > int(n) {
		return model.MessagePage{Items: items[:n], HasMore: true}
	}
	return model.MessagePage{Items: items}
}

// UnreadTotal 是呼叫者這一方的總未讀數（對話列上 last_seq 與自己游標的差，不 join 訊息）。
func (s *Service) UnreadTotal(ctx context.Context, p model.Party) (int64, error) {
	if err := p.Authorize(); err != nil {
		return 0, err
	}
	if p.Kind == model.PartyBuyer {
		return s.q(ctx).UnreadTotalBuyer(ctx, p.BuyerID)
	}
	return s.q(ctx).UnreadTotalStore(ctx, p.Store.StoreID)
}

// MessageForRelay 讀單則訊息與它的對話，不套可見性（Hidden、Erased 標出）；不存在是 NotFound。
func (s *Service) MessageForRelay(ctx context.Context, id uuid.UUID) (model.RelayMessage, error) {
	r, err := s.q(ctx).GetMessageForRelay(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RelayMessage{}, apperr.NotFound("%s", text.ImConversationNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return model.RelayMessage{}, err
	}
	m := rows.Message(rows.MessageRow{
		ID: r.ID, ConversationID: r.ConversationID, Seq: r.Seq, SenderKind: r.SenderKind, MessageType: r.MessageType, Content: r.Content,
		ImageAssetID: r.ImageAssetID, ImageObjectKey: r.ImageObjectKey, CreatedAt: r.CreatedAt, ErasedAt: r.ErasedAt, Hidden: r.Hidden,
	})
	return model.RelayMessage{
		Message: m, BuyerID: r.BuyerID, StoreID: r.StoreID,
		Preview: rows.PreviewOf(r.SenderKind, r.MessageType, r.Content, r.ErasedAt, r.Hidden),
	}, nil
}
