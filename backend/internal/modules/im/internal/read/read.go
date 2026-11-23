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

// Buyers 是在線狀態判定用到的 buyer 公開 API；正式組裝傳 *buyer.Module。
type Buyers interface {
	IsBlockingStore(ctx context.Context, buyerID, storeID int64) (bool, error)
}

// Stores 是在線狀態判定用到的 seller 公開 API；正式組裝傳 *seller.Module。
type Stores interface {
	StoreBlocksBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
}

// Deps 是讀取路徑的依賴；Buyers、Stores 只有 Presence 用。
type Deps struct {
	DB     *db.DB
	Buyers Buyers
	Stores Stores
}

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
			items = append(items, conversation(r.ID, r.BuyerID, r.StoreID, r.LastSeq, r.ReadSeq, r.LastMessageAt, r.SenderKind, r.MessageType, r.Content, r.ErasedAt, r.Hidden, r.PeerReadSeq))
		}
	} else {
		rs, err := s.q(ctx).ListConversationsStore(ctx, store.ListConversationsStoreParams{StoreID: p.Store.StoreID, AfterAt: afterAt, AfterID: afterID, MaxRows: n + 1})
		if err != nil {
			return model.ConversationPage{}, err
		}
		for _, r := range rs {
			items = append(items, conversation(r.ID, r.BuyerID, r.StoreID, r.LastSeq, r.ReadSeq, r.LastMessageAt, r.SenderKind, r.MessageType, r.Content, r.ErasedAt, r.Hidden, r.PeerReadSeq))
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

func conversation(id uuid.UUID, buyerID, storeID, lastSeq, readSeq int64, at time.Time, senderKind, msgType string, content *string, erasedAt *time.Time, hidden bool, peerReadSeq int64) model.Conversation {
	return model.Conversation{
		ID: id, BuyerID: buyerID, StoreID: storeID, LastSeq: lastSeq, Unread: lastSeq - readSeq, LastMessageAt: at,
		Preview: rows.PreviewOf(senderKind, msgType, content, erasedAt, hidden), PeerReadSeq: peerReadSeq,
	}
}

// ListMessages 是對話裡 seq 小於 beforeSeq 的訊息（新的在前；beforeSeq 為 0 從最新開始），以 seq 做 keyset，不回總數。
func (s *Service) ListMessages(ctx context.Context, conversationID uuid.UUID, p model.Party, beforeSeq int64, size int32) (model.MessagePage, error) {
	c, err := s.authorized(ctx, conversationID, p)
	if err != nil {
		return model.MessagePage{}, err
	}
	n := pageSize(size)
	rs, err := s.q(ctx).ListMessagesBefore(ctx, store.ListMessagesBeforeParams{ConversationID: conversationID, BeforeSeq: beforeSeq, MaxRows: n + 1})
	if err != nil {
		return model.MessagePage{}, err
	}
	return withPeer(page(rows.Before(rs), n), c, p), nil
}

// MessagesAfter 是對話裡 seq 大於 afterSeq 的訊息（舊的在前），給串流的斷線補洞。
func (s *Service) MessagesAfter(ctx context.Context, conversationID uuid.UUID, p model.Party, afterSeq int64, size int32) (model.MessagePage, error) {
	c, err := s.authorized(ctx, conversationID, p)
	if err != nil {
		return model.MessagePage{}, err
	}
	n := pageSize(size)
	rs, err := s.q(ctx).ListMessagesAfter(ctx, store.ListMessagesAfterParams{ConversationID: conversationID, AfterSeq: afterSeq, MaxRows: n + 1})
	if err != nil {
		return model.MessagePage{}, err
	}
	return withPeer(page(rows.After(rs), n), c, p), nil
}

// withPeer 填入對方的已讀游標。
func withPeer(pg model.MessagePage, c store.ImConversation, p model.Party) model.MessagePage {
	pg.PeerReadSeq = c.StoreLastReadSeq
	if p.Kind == model.PartyStore {
		pg.PeerReadSeq = c.BuyerLastReadSeq
	}
	return pg
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
		return model.RelayMessage{}, apperr.NotFound("%s", text.ImMessageNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return model.RelayMessage{}, err
	}
	m := rows.Message(rows.MessageRow{
		ID: r.ID, ConversationID: r.ConversationID, Seq: r.Seq, SenderKind: r.SenderKind, MessageType: r.MessageType, Content: r.Content,
		ImageAssetID: r.ImageAssetID, ImageObjectKey: r.ImageObjectKey, CreatedAt: r.CreatedAt, ErasedAt: r.ErasedAt, Hidden: r.Hidden,
	})
	return model.RelayMessage{
		Message: m, BuyerID: r.BuyerID, StoreID: r.StoreID, ReceiverInactive: r.ReceiverInactive,
		Preview: rows.PreviewOf(r.SenderKind, r.MessageType, r.Content, r.ErasedAt, r.Hidden),
	}, nil
}

// ActiveSince 是 p 這一方在 since（資料庫時間）之後有新訊息的對話，最多 limit 個；給監聽重連後的補洞。
func (s *Service) ActiveSince(ctx context.Context, p model.Party, since time.Time, limit int) ([]uuid.UUID, error) {
	if err := p.Authorize(); err != nil {
		return nil, err
	}
	if p.Kind == model.PartyBuyer {
		return s.q(ctx).ActiveConversationIDsBuyer(ctx, store.ActiveConversationIDsBuyerParams{BuyerID: p.BuyerID, Since: since, MaxRows: int64(limit)})
	}
	return s.q(ctx).ActiveConversationIDsStore(ctx, store.ActiveConversationIDsStoreParams{StoreID: p.Store.StoreID, Since: since, MaxRows: int64(limit)})
}

// Presence 回報呼叫者的每個對話的對方是否在線。授權單位是共有的對話：任何一個對話不屬於呼叫者，整個呼叫回 NotFound，
// 呼叫者無法用任意 ID 探測任意人。對方已刪帳或退場、任一方向有封鎖都回離線，與對方真的離線不可區分；封鎖查詢失敗回錯誤，不放行。
func (s *Service) Presence(ctx context.Context, p model.Party, conversationIDs []uuid.UUID) ([]model.PresenceState, error) {
	if err := p.Authorize(); err != nil {
		return nil, err
	}
	if len(conversationIDs) > model.MaxPresenceConversations {
		return nil, apperr.InvalidInputOn([]string{"ConversationIDs"}, "%s", text.ImPresenceTooMany(uitext.LocaleFrom(ctx)))
	}
	convs := make([]store.ImConversation, len(conversationIDs))
	for i, id := range conversationIDs {
		c, err := s.authorized(ctx, id, p)
		if err != nil {
			return nil, err
		}
		convs[i] = c
	}
	out := make([]model.PresenceState, len(convs))
	for i, c := range convs {
		online, err := s.peerOnline(ctx, p, c)
		if err != nil {
			return nil, err
		}
		out[i] = model.PresenceState{ConversationID: c.ID, Online: online}
	}
	return out, nil
}

func (s *Service) peerOnline(ctx context.Context, p model.Party, c store.ImConversation) (bool, error) {
	peerKind, peerID := string(model.PartyStore), c.StoreID
	if p.Kind == model.PartyStore {
		peerKind, peerID = string(model.PartyBuyer), c.BuyerID
	}
	q := s.q(ctx)
	inactive, err := q.PartyInactive(ctx, store.PartyInactiveParams{Kind: peerKind, PartyID: peerID})
	if err != nil || inactive {
		return false, err
	}
	blocking, err := s.d.Buyers.IsBlockingStore(ctx, c.BuyerID, c.StoreID)
	if err != nil || blocking {
		return false, err
	}
	blocked, err := s.d.Stores.StoreBlocksBuyer(ctx, c.StoreID, c.BuyerID)
	if err != nil || blocked {
		return false, err
	}
	return q.IsPartyOnline(ctx, store.IsPartyOnlineParams{Kind: peerKind, PartyID: peerID})
}

// MessageForReport 是檢舉用的證據讀取：呼叫者要是該訊息所屬對話的一方，否則（含訊息不存在）NotFound，不洩漏對話存在；被檢舉的訊息已隱藏或已抹除也是 NotFound；
// 前後各 window 則已隱藏或已抹除的是墓碑，不帶內容。被檢舉的訊息與前後文以一句 SQL 讀回（同一時點），不鎖列、加入呼叫方的 transaction（不開 InReadSnapshot）。
func (s *Service) MessageForReport(ctx context.Context, messageID uuid.UUID, reporter model.Party, window int) (model.ReportedMessage, error) {
	if err := reporter.Authorize(); err != nil {
		return model.ReportedMessage{}, err
	}
	if window < 0 {
		return model.ReportedMessage{}, errors.New("im：前後文的則數不能是負數")
	}
	notFound := apperr.NotFound("%s", text.ImMessageNotFound(uitext.LocaleFrom(ctx)))
	rs, err := s.q(ctx).MessagesForReport(ctx, store.MessagesForReportParams{MessageID: messageID, Radius: int64(window)})
	if err != nil {
		return model.ReportedMessage{}, err
	}
	var out model.ReportedMessage
	found := false
	for _, r := range rs {
		if (reporter.Kind == model.PartyBuyer && r.BuyerID != reporter.BuyerID) || (reporter.Kind == model.PartyStore && r.StoreID != reporter.Store.StoreID) {
			return model.ReportedMessage{}, notFound
		}
		m := model.ReportedChatMessage{
			ID: r.ID, Seq: r.Seq, SenderKind: model.PartyKind(r.SenderKind), Type: model.MessageType(r.MessageType), CreatedAt: r.CreatedAt,
			Hidden: r.Hidden, Erased: r.ErasedAt != nil,
		}
		m.SenderID = r.BuyerID
		if m.SenderKind == model.PartyStore {
			m.SenderID = r.StoreID
		}
		if !m.Hidden && !m.Erased {
			if r.Content != nil {
				m.Text = *r.Content
			}
			if r.ImageAssetID != nil {
				m.AssetID = r.ImageAssetID.String()
			}
		}
		if r.IsTarget {
			found = true
			if m.Hidden || m.Erased {
				return model.ReportedMessage{}, notFound
			}
			out.ConversationID, out.BuyerID, out.StoreID, out.Message = r.ConversationID, r.BuyerID, r.StoreID, m
			continue
		}
		out.Context = append(out.Context, m)
	}
	if !found {
		return model.ReportedMessage{}, notFound
	}
	return out, nil
}
