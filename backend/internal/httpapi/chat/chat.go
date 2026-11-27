// Package chat 是 im 模組的 Connect handler：前台的 hawkr.chat.v1alpha1（ChatAppService）。這裡只做欄位的轉換、身分的推導與顯示資料的組裝
// （店家卡片來自 seller、買家名稱與頭像來自 buyer、圖片網址由 rpc.Images 組成），規則在 internal/modules/im。
package chat

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Reads 是聊天的讀取來源（*im.Module）。
type Reads interface {
	ListConversations(ctx context.Context, p im.Party, after *im.Cursor, size int32) (im.ConversationPage, error)
	StoreConversationByBuyer(ctx context.Context, p im.Party, buyerID int64) (im.Conversation, error)
	ListMessages(ctx context.Context, conversationID uuid.UUID, p im.Party, beforeSeq int64, size int32) (im.MessagePage, error)
	MessagesAfter(ctx context.Context, conversationID uuid.UUID, p im.Party, afterSeq int64, size int32) (im.MessagePage, error)
	UnreadTotal(ctx context.Context, p im.Party) (int64, error)
	Presence(ctx context.Context, p im.Party, conversationIDs []uuid.UUID) ([]im.PresenceState, error)
	ConversationParties(ctx context.Context, conversationID uuid.UUID) (im.ConversationParties, error)
}

// Writes 是聊天的寫入來源（*im.Module）。
type Writes interface {
	Send(ctx context.Context, in im.SendInput) (im.Message, error)
	MarkRead(ctx context.Context, conversationID uuid.UUID, p im.Party, uptoSeq int64) error
	MarkAllRead(ctx context.Context, p im.Party) error
}

// Buyers 是呼叫者的買家身分與對話對方（買家）的顯示資料的來源（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
}

// Stores 是呼叫者此刻的店與對話對方（店家）的卡片的來源（*seller.Module）。
type Stores interface {
	CurrentStore(ctx context.Context, accountID int64) (seller.Store, error)
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
}

type cursor struct {
	At time.Time `json:"c"`
	ID uuid.UUID `json:"i"`
}

func (c cursor) Valid() bool { return !c.At.IsZero() && c.ID != uuid.UUID{} }

func decodeCursor(scope, token string) (*im.Cursor, error) {
	c, err := rpc.ValidCursor[cursor](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	return &im.Cursor{At: c.At, ID: c.ID}, nil
}

func nextToken(scope string, next *im.Cursor) string {
	if next == nil {
		return ""
	}
	return rpc.TokenFor(scope, cursor{At: next.At, ID: next.ID})
}

func pageSize(n int32) (int32, error) {
	if n < 0 {
		return 0, rpc.RequiredField("page_size", "page_size 不能是負數")
	}
	return n, nil
}

var reasons = map[im.Reason]apperr.Reason{
	im.ReasonSelfMessageNotAllowed: apperr.ReasonSelfMessageNotAllowed,
	im.ReasonSellerSuspended:       apperr.ReasonSellerSuspended,
	im.ReasonStoreClosed:           apperr.ReasonStoreClosed,
	im.ReasonChatBlocked:           apperr.ReasonChatBlocked,
	im.ReasonReceiverDeleted:       apperr.ReasonReceiverDeleted,
}

var (
	contentPaths = rpc.FieldPaths{
		"Text": "content.text", "StagedToken": "content.image_staged_token", "Type": "content",
		"RequestID": "request_id", "ConversationIDs": "conversation_ids",
	}
	toStorePaths = withPath(contentPaths, "CounterpartID", "store_id")
	toBuyerPaths = withPath(contentPaths, "CounterpartID", "buyer_id")
)

func withPath(base rpc.FieldPaths, k, v string) rpc.FieldPaths {
	out := rpc.FieldPaths{k: v}
	for bk, bv := range base {
		out[bk] = bv
	}
	return out
}

func (h *App) failure(err error, paths rpc.FieldPaths) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if r, ok := reasons[h.reasonOf(err)]; ok && errors.As(err, &ae) {
		ae.Reason = r
	}
	return rpc.WithFieldPaths(err, paths)
}

func (h *App) buyerParty(ctx context.Context) (im.Party, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return im.Party{}, err
	}
	return h.buyerPartyOf(ctx, p.ID)
}

func (h *App) buyerPartyOf(ctx context.Context, accountID int64) (im.Party, error) {
	id, err := h.buyers.CurrentBuyerID(ctx, accountID)
	if err != nil {
		return im.Party{}, rpc.BuyerInvariant(accountID, err)
	}
	return im.Party{Kind: im.PartyBuyer, BuyerID: id}, nil
}

func (h *App) storeParty(ctx context.Context) (im.Party, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return im.Party{}, err
	}
	return h.storePartyOf(ctx, p.ID)
}

func (h *App) storePartyOf(ctx context.Context, accountID int64) (im.Party, error) {
	st, err := h.stores.CurrentStore(ctx, accountID)
	if err != nil {
		return im.Party{}, err
	}
	return im.Party{Kind: im.PartyStore, Store: &st}, nil
}

func conversationNotFound() error { return apperr.NotFound("找不到這個對話") }

func (h *App) partyOf(ctx context.Context, conversationID uuid.UUID) (im.Party, error) {
	acct, err := rpc.RequireAccount(ctx)
	if err != nil {
		return im.Party{}, err
	}
	buyerID, err := h.buyers.CurrentBuyerID(ctx, acct.ID)
	if err != nil {
		return im.Party{}, rpc.BuyerInvariant(acct.ID, err)
	}
	var store *seller.Store
	switch st, err := h.stores.CurrentStore(ctx, acct.ID); {
	case err == nil:
		store = &st
	case apperr.KindOf(err) != apperr.KindNotFound:
		return im.Party{}, err
	}
	parties, err := h.reads.ConversationParties(ctx, conversationID)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return im.Party{}, conversationNotFound()
		}
		return im.Party{}, err
	}
	party, err := rpc.ChatPartyOf(buyerID, store, parties)
	switch {
	case errors.Is(err, rpc.ErrBothChatParties):
		h.log.Error("聊天對話的兩方是同一個帳號（不變量被破壞）", "accountID", acct.ID, "conversationID", conversationID.String())
		return im.Party{}, fmt.Errorf("對話 %s 的買家方與店家方都是帳號 %d（不變量被破壞）", conversationID, acct.ID)
	case errors.Is(err, rpc.ErrNotChatParty):
		return im.Party{}, conversationNotFound()
	case err != nil:
		return im.Party{}, err
	}
	return party, nil
}

func (h *App) toMessage(m im.Message) (*pb.Message, error) {
	out := &pb.Message{
		Id: m.ID.String(), ConversationId: m.ConversationID.String(), Seq: m.Seq,
		Sender: rpc.EnumToProto[pb.ChatParty](m.SenderKind), Kind: rpc.EnumToProto[pb.MessageKind](m.Type),
		Hidden: m.Hidden, Erased: m.Erased, CreateTime: timestamppb.New(m.CreatedAt),
	}
	if m.Hidden || m.Erased {
		return out, nil
	}
	if m.Text != nil {
		text := *m.Text
		out.Text = &text
	}
	if m.Image != nil {
		img, err := h.image(m.Image.ObjectKey)
		if err != nil {
			return nil, err
		}
		out.Image = img
	}
	return out, nil
}

func (h *App) toMessages(in []im.Message) ([]*pb.Message, error) {
	out := make([]*pb.Message, len(in))
	for i, m := range in {
		v, err := h.toMessage(m)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func toPreview(p im.Preview) *pb.Preview {
	out := &pb.Preview{
		Kind: rpc.EnumToProto[pb.MessageKind](p.Type), Hidden: p.Hidden, Erased: p.Erased,
		Sender: rpc.EnumToProto[pb.ChatParty](p.SenderKind),
	}
	if !p.Hidden && !p.Erased && p.Text != nil {
		text := *p.Text
		out.Text = &text
	}
	return out
}

func (h *App) image(key string) (*commonpb.Image, error) {
	if err := rpc.ObjectKey(key); err != nil {
		return nil, err
	}
	return h.images.Image(key), nil
}

func baseConversation(c im.Conversation) *pb.Conversation {
	return &pb.Conversation{
		Id: c.ID.String(), UnreadCount: c.Unread, LastMessageTime: timestamppb.New(c.LastMessageAt),
		Preview: toPreview(c.Preview), PeerReadSeq: c.PeerReadSeq,
	}
}

func (h *App) buyerConversations(ctx context.Context, in []im.Conversation) ([]*pb.Conversation, error) {
	cards := map[int64]seller.StoreCard{}
	if ids := uniqueIDs(in, func(c im.Conversation) int64 { return c.StoreID }); len(ids) > 0 {
		rows, err := h.stores.StoreCards(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, c := range rows {
			cards[c.StoreID] = c
		}
	}
	out := make([]*pb.Conversation, len(in))
	for i, c := range in {
		out[i] = baseConversation(c)
		if card, ok := cards[c.StoreID]; ok {
			var err error
			if out[i].Store, err = httpseller.ToStoreCard(h.images, card); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (h *App) storeConversations(ctx context.Context, in []im.Conversation) ([]*pb.Conversation, error) {
	views, err := blocks.BuyerViews(ctx, h.buyers, uniqueIDs(in, func(c im.Conversation) int64 { return c.BuyerID }))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Conversation, len(in))
	for i, c := range in {
		out[i] = baseConversation(c)
		v := views[c.BuyerID]
		out[i].Buyer = &pb.ChatBuyer{Id: rpc.FormatID(c.BuyerID), Deleted: v.Deleted}
		if v.Deleted {
			continue
		}
		if v.DisplayName != "" {
			name := v.DisplayName
			out[i].Buyer.Name = &name
		}
		if v.Avatar != nil {
			if out[i].Buyer.Avatar, err = h.image(*v.Avatar); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func uniqueIDs(in []im.Conversation, id func(im.Conversation) int64) []int64 {
	seen := make(map[int64]bool, len(in))
	out := make([]int64, 0, len(in))
	for _, c := range in {
		if v := id(c); !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
