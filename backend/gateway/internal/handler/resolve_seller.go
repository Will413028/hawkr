package handler

import (
	"context"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
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

// resolveStoreIDFor 是 2-arg 變體：call site 已有自己的 identity guard
// （要保留原錯誤碼）、手上已有驗過的 accountID 時用（section X 的 sweep 站點）。
// 錯誤對映與 resolveStoreID 相同，只少 401 分支。
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
			return noCurrentSellerFilterSentinel, true // 哨兵值：保證查無結果，而非略過篩選（"0" 會被 Java 端當成未篩選，見上方註解）
		}
		response.GrpcErr(c, "seller", err)
		return "", false
	}
	return strconv.FormatInt(int64(storeID), 10), true
}

// isSellerRole 回報呼叫者是否具賣家權限（accountType code 2 或 3）。
func isSellerRole(c *gin.Context) bool {
	accountType, _ := c.Get("accountType")
	code, _ := accountType.(int)
	return middleware.IsSeller(code)
}

func (h *Holder) chatWireIdentity(c *gin.Context) (int64, bool) {
	v, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return 0, false
	}
	id := parseAccountID(v)
	switch parseAccountType(c) {
	case 2:
		storeID, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(id))
		if err != nil {
			if errors.Is(err, storeid.ErrNoCurrentStore) {
				response.Forbidden(c, "seller access required")
				return 0, false
			}
			response.GrpcErr(c, "seller", err)
			return 0, false
		}
		return int64(storeID), true
	default:
		buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(id))
		if err != nil {
			if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
				response.Forbidden(c, "buyer access required")
				return 0, false
			}
			response.GrpcErr(c, "buyer", err)
			return 0, false
		}
		return int64(buyerID), true
	}
}

// resolveOptionalBuyerID is the optional-auth (personalization-only) identity
// lookup used by the review path: accountID==0 means anonymous (caller wasn't
// logged in at all — OptionalJWTAuth path), and loginRole is the caller's
// active role (1=buyer, 2=seller, 0=unresolved). It resolves the CURRENT id
// for that role (buyer_id or store_id); when the role is unresolved it tries
// the buyer axis first, then the store axis.
//
// Neither ErrNoCurrentBuyer nor ErrNoCurrentStore ever errors here — both
// degrade to (0, nil), "treat as anonymous" — because a read-path
// personalization lookup must never 403 a page load; only a genuine backend
// error is returned for the caller to decide how to degrade. The social
// handlers do not use this: they take buyer_id and store_id separately
// (socialCaller) and social derives the acting identity from resource
// ownership.
func (h *Holder) resolveOptionalBuyerID(ctx context.Context, accountID int64, loginRole int) (int64, error) {
	if accountID == 0 {
		return accountID, nil
	}
	switch loginRole {
	case 1:
		resolved, err := h.BuyerID.CurrentBuyerID(ctx, identity.AccountID(accountID))
		if err != nil {
			if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
				return 0, nil
			}
			return 0, err
		}
		return int64(resolved), nil
	case 2:
		resolved, err := h.Store.CurrentStoreID(ctx, identity.AccountID(accountID))
		if err != nil {
			if errors.Is(err, storeid.ErrNoCurrentStore) {
				return 0, nil // degrade to anonymous — never block a read-path personalization lookup
			}
			return 0, err
		}
		return int64(resolved), nil
	default:
		// loginRole 未定（雙角色帳號、沒有 hint 也沒有持久化角色）：先買家後賣家。
		// 兩軸皆無 → 回 0（匿名），維持讀側「永遠不因解析失敗而擋住頁面」的契約；
		// 後端錯誤原樣回傳，由呼叫端決定怎麼降級。
		if resolved, err := h.BuyerID.CurrentBuyerID(ctx, identity.AccountID(accountID)); err == nil {
			return int64(resolved), nil
		} else if !errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			return 0, err
		}
		if resolved, err := h.Store.CurrentStoreID(ctx, identity.AccountID(accountID)); err == nil {
			return int64(resolved), nil
		} else if !errors.Is(err, storeid.ErrNoCurrentStore) {
			return 0, err
		}
		return 0, nil
	}
}
