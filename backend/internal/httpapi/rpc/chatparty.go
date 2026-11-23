package rpc

import (
	"errors"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

var (
	// ErrNotChatParty 是呼叫者不是這段對話的任何一方；呼叫端要把它當 not_found 回，與對話或訊息不存在相同。
	ErrNotChatParty = errors.New("httpapi/rpc：呼叫者不是這段對話的一方")
	// ErrBothChatParties 是呼叫者同時是對話的買家方與店家方：不變量被破壞（模組擋下自己傳訊息給自己的店），呼叫端記錄後回沒有分類的 internal。
	ErrBothChatParties = errors.New("httpapi/rpc：呼叫者同時是對話的買家方與店家方")
)

// ChatPartyOf 由對話的歸屬推導呼叫者是買家方或店家方：buyerID 是呼叫者的買家身分（沒有為 0），store 是呼叫者此刻的店（沒有為 nil）。
// 聊天與檢舉共用這條推導；parties 只用在這裡，不得進任何回應。
func ChatPartyOf(buyerID int64, store *seller.Store, parties im.ConversationParties) (im.Party, error) {
	isBuyer := buyerID != 0 && parties.BuyerID == buyerID
	isStore := store != nil && parties.StoreID == store.StoreID
	switch {
	case isBuyer && isStore:
		return im.Party{}, ErrBothChatParties
	case isBuyer:
		return im.Party{Kind: im.PartyBuyer, BuyerID: buyerID}, nil
	case isStore:
		return im.Party{Kind: im.PartyStore, Store: store}, nil
	}
	return im.Party{}, ErrNotChatParty
}
