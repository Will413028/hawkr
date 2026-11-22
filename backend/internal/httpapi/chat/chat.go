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

// cursor 是對話列表的 page token 內容：上一頁最後一列的 (last_message_at, id)。
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

// pageSize 檢查 page_size：負數是呼叫方的錯（API 設計規範（Connect）§六），0 與過大由模組用預設與上限。
func pageSize(n int32) (int32, error) {
	if n < 0 {
		return 0, rpc.RequiredField("page_size", "page_size 不能是負數")
	}
	return n, nil
}

// reasons 是模組的業務原因對到契約的 ErrorReason（apperr.Reason）：只有 client 會依它分支的才對應。
// 詞典拒絕（ReasonModerationRejected）是欄位錯誤，不帶 reason。
var reasons = map[im.Reason]apperr.Reason{
	im.ReasonSelfMessageNotAllowed: apperr.ReasonSelfMessageNotAllowed,
	im.ReasonSellerSuspended:       apperr.ReasonSellerSuspended,
	im.ReasonStoreClosed:           apperr.ReasonStoreClosed,
	im.ReasonChatBlocked:           apperr.ReasonChatBlocked,
	im.ReasonReceiverDeleted:       apperr.ReasonReceiverDeleted,
}

// 模組回報的欄位名對到請求的欄位路徑；CounterpartID 依方法對到 store_id 或 buyer_id。
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

// failure 把模組的錯誤轉成對外的錯誤：業務原因附上契約的 ErrorReason，欄位對到請求路徑。
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

// buyerParty 回傳呼叫者的買家身分；只有顧客帳號能呼叫（沒有登入由 interceptor 擋，管理員回 permission_denied）。
func (h *App) buyerParty(ctx context.Context) (im.Party, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return im.Party{}, err
	}
	id, err := h.buyers.CurrentBuyerID(ctx, p.ID)
	if err != nil {
		return im.Party{}, rpc.BuyerInvariant(p.ID, err)
	}
	return im.Party{Kind: im.PartyBuyer, BuyerID: id}, nil
}

// storeParty 回傳呼叫者此刻的店的身分（seller.CurrentStore）；沒有店回 not_found。賣家能力由模組檢查，宣告見 declarations。
func (h *App) storeParty(ctx context.Context) (im.Party, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return im.Party{}, err
	}
	st, err := h.stores.CurrentStore(ctx, p.ID)
	if err != nil {
		return im.Party{}, err
	}
	return im.Party{Kind: im.PartyStore, Store: &st}, nil
}

func conversationNotFound() error { return apperr.NotFound("找不到這個對話") }

// partyOf 由對話的歸屬推導呼叫者是買家方或店家方：對話不存在與呼叫者不是任何一方回同一個 not_found（不洩漏對話存在）；
// ConversationParties 的結果只用在這裡，不進任何回應。兩邊都是（同一帳號是買家方又擁有該店）是不變量被破壞：模組擋下自己傳訊息給自己的店。
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
	isBuyer := parties.BuyerID == buyerID
	isStore := store != nil && parties.StoreID == store.StoreID
	switch {
	case isBuyer && isStore:
		h.log.Error("聊天對話的兩方是同一個帳號（不變量被破壞）", "accountID", acct.ID, "conversationID", conversationID.String())
		return im.Party{}, fmt.Errorf("對話 %s 的買家方與店家方都是帳號 %d（不變量被破壞）", conversationID, acct.ID)
	case isBuyer:
		return im.Party{Kind: im.PartyBuyer, BuyerID: buyerID}, nil
	case isStore:
		return im.Party{Kind: im.PartyStore, Store: store}, nil
	}
	return im.Party{}, conversationNotFound()
}

// toMessage 把訊息轉成回應。隱藏與抹除的訊息是墓碑，不帶內容（模組已保證，這裡不依賴它：墓碑一律不讀 Text 與 Image）。
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

// buyerConversations 組買家版的對話：對方是店家，卡片以一次批次查詢補上（沒有卡片的店不設 store）。
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

// storeConversations 組店家版的對話：對方是買家，名稱與頭像以一次批次查詢補上（已關閉或查不到的買家是 deleted）。
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
