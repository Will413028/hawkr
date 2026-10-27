package handler

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/gin-gonic/gin"
)

// currentBuyerIDOrZero 回傳帳號的現任 buyer_id；訪客（accountID == 0）與沒有買家身分的帳號回 0、nil。
// 只有後端查詢失敗才回 error。不寫回應，呼叫端自己決定要擋下還是降級。
func (h *Holder) currentBuyerIDOrZero(ctx context.Context, accountID int64) (int64, error) {
	if accountID == 0 {
		return 0, nil
	}
	id, err := h.BuyerID.CurrentBuyerID(ctx, identity.AccountID(accountID))
	switch {
	case err == nil:
		return int64(id), nil
	case errors.Is(err, buyerid.ErrNoCurrentBuyer):
		return 0, nil
	default:
		return 0, err
	}
}

// socialBuyerID 回傳呼叫者的現任 buyer_id；訪客與沒有買家身分的帳號回 0。
// 貼文讚永遠是本人身分，貼文列表與詳情的 is_liked 只看這個 id。
// ok=false 表示已寫出回應（後端錯誤），呼叫端必須立刻 return。
func (h *Holder) socialBuyerID(c *gin.Context, accountID int64) (int64, bool) {
	id, err := h.currentBuyerIDOrZero(c.Request.Context(), accountID)
	if err != nil {
		response.GrpcErr(c, "buyer", err)
		return 0, false
	}
	return id, true
}

// socialCaller 回傳呼叫者此刻的 buyer_id 與 store_id（單體帶來，沒有的是 0）；訪客為零值。
// 互動身分（本人或店家）由 social 依目標的歸屬推導，這裡不看 active role，也不退回 account_id。
// ok=false 表示已寫出回應（後端錯誤），呼叫端必須立刻 return。
func (h *Holder) socialCaller(c *gin.Context, accountID int64) (grpcclient.SocialCaller, bool) {
	buyerID, ok := h.socialBuyerID(c, accountID)
	if !ok {
		return grpcclient.SocialCaller{}, false
	}
	if accountID == 0 {
		return grpcclient.SocialCaller{}, true
	}
	storeID, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID))
	switch {
	case err == nil:
		return grpcclient.SocialCaller{BuyerID: buyerID, StoreID: int64(storeID)}, true
	case errors.Is(err, storeid.ErrNoCurrentStore):
		return grpcclient.SocialCaller{BuyerID: buyerID}, true
	default:
		response.GrpcErr(c, "seller", err)
		return grpcclient.SocialCaller{}, false
	}
}
