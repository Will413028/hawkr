package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/gin-gonic/gin"
)

func (h *Holder) resolveStoreID(c *gin.Context) (identity.StoreID, bool) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return 0, false
	}
	return h.resolveStoreIDFor(c, accountID)
}

func (h *Holder) resolveStoreIDFor(c *gin.Context, accountID int64) (identity.StoreID, bool) {
	storeID, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if errors.Is(err, storeid.ErrNoCurrentStore) {
			response.Forbidden(c, "seller access required")
			return 0, false
		}
		response.GrpcErr(c, "seller", err)
		return 0, false
	}
	return storeID, true
}

const noCurrentSellerFilterSentinel = "-1"

func (h *Holder) resolveOptionalSellerFilterQuery(c *gin.Context, rawIDStr string) (string, bool) {
	if rawIDStr == "" {
		return "", true
	}
	rawID, err := strconv.ParseInt(rawIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid store_id")
		return "", false
	}
	storeID, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(rawID))
	if err != nil {
		if errors.Is(err, storeid.ErrNoCurrentStore) {
			return noCurrentSellerFilterSentinel, true
		}
		response.GrpcErr(c, "seller", err)
		return "", false
	}
	return strconv.FormatInt(int64(storeID), 10), true
}

func isSellerRole(c *gin.Context) bool {
	accountType, _ := c.Get("accountType")
	code, _ := accountType.(int)
	return middleware.IsSeller(code)
}
