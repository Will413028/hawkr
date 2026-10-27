package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/gin-gonic/gin"
)

func (h *Holder) resolveBuyerID(c *gin.Context) (identity.BuyerID, bool) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return 0, false
	}
	return h.resolveBuyerIDFor(c, accountID)
}

// resolveBuyerIDFor 是 2-arg 變體：call site 已有自己驗過的
// accountID（保留原有的 401 錯誤碼與訊息）時用。錯誤對映與
// resolveBuyerID 相同，只少 401 分支。
func (h *Holder) resolveBuyerIDFor(c *gin.Context, accountID int64) (identity.BuyerID, bool) {
	buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return 0, false
		}
		response.GrpcErr(c, "buyer", err)
		return 0, false
	}
	return buyerID, true
}

func (h *Holder) resolveOptionalBuyerFilterQuery(c *gin.Context, rawIDStr string) (string, bool) {
	if rawIDStr == "" {
		return "", true
	}
	rawID, err := strconv.ParseInt(rawIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid buyer_id")
		return "", false
	}
	buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(rawID))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			return "-1", true // 哨兵值：保證查無結果，而非略過篩選（"0" 會被 Java 端當成未篩選，見上方註解）
		}
		response.GrpcErr(c, "buyer", err)
		return "", false
	}
	return strconv.FormatInt(int64(buyerID), 10), true
}
