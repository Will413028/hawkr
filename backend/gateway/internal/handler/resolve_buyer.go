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
			return "-1", true
		}
		response.GrpcErr(c, "buyer", err)
		return "", false
	}
	return strconv.FormatInt(int64(buyerID), 10), true
}

func parseSellerViewBuyerFilter(c *gin.Context, rawIDStr string) (string, bool) {
	if rawIDStr == "" {
		return "", true
	}
	id, err := strconv.ParseInt(rawIDStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid buyer_id")
		return "", false
	}
	return strconv.FormatInt(id, 10), true
}
