package handler

import (
	"context"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// storeGateMode 決定一個公開入口要店家滿足什麼條件。
type storeGateMode int

const (
	// gateExists：店家存在且沒有關閉（只打 GetSeller 一支 RPC）。
	gateExists storeGateMode = iota
	// gateCard：存在、沒有關閉，而且曾經公開過（has_public_card；買家聊天對象）。
	gateCard
	// gateExposable：店家此刻可曝光（can_accept_orders；公開評價的父資源）。
	gateExposable
)

type storeGateKind int

const (
	gateInvalidID storeGateKind = iota + 1
	gateNotFound
	gateRPCError  // 呼叫失敗（err 有值）
	gateRPCResult // 回應 code != 0（code、message 有值）
)

// storeGateError 是 gin 無關的失敗描述，REST 與 WebSocket 各自對應成自己的回應。
type storeGateError struct {
	err     error
	message string
	code    int32
	kind    storeGateKind
}

// checkStoreGate 是公開評價與聊天入口的店家檢查（gateway 的過渡層；退場條件見
// docs/plans/2026-09-29-a3-buyer-seller.md 第 3b 刀完成的說明）。不碰 gin。
//
// 呼叫數：gateExposable 只打一次 BatchGetSellerMeta（缺項＝不存在，關閉的店
// Exposable 為 false）；gateExists 只打 GetSeller；gateCard 要 GetSeller 加
// BatchGetSellerMeta 兩次，因為 SellerMeta 沒有「已關閉」欄位，而已關閉但曾公開過
// 的店 has_public_card 仍為 true，規則是聊天與店面讀取要濾掉已關閉的店
// （step10d-decision-brief D2）。SellerMeta 加上 status 後 gateCard 也能降成一次。
func (h *Holder) checkStoreGate(ctx context.Context, storeID int64, mode storeGateMode) *storeGateError {
	if storeID <= 0 {
		return &storeGateError{kind: gateInvalidID}
	}
	if mode != gateExposable {
		resp, err := h.Seller.GetSellerByStoreID(ctx, storeID)
		if err != nil {
			return &storeGateError{kind: gateRPCError, err: err}
		}
		if resp.Code != 0 || resp.Data == nil {
			return &storeGateError{kind: gateRPCResult, code: resp.Code, message: resp.Message}
		}
		if mode == gateExists {
			return nil
		}
	}
	resp, err := h.Seller.BatchGetSellerMeta(ctx, []int64{storeID})
	if err != nil {
		return &storeGateError{kind: gateRPCError, err: err}
	}
	if resp.Code != 0 {
		return &storeGateError{kind: gateRPCResult, code: resp.Code, message: resp.Message}
	}
	meta := resp.Sellers[storeID]
	if meta == nil {
		return &storeGateError{kind: gateNotFound}
	}
	if mode == gateCard && !meta.HasPublicCard || mode == gateExposable && !meta.CanAcceptOrders {
		return &storeGateError{kind: gateNotFound}
	}
	return nil
}

// writeStoreGateError 把 checkStoreGate 的失敗寫成 REST 回應；notFoundMsg 是
// gateNotFound 的訊息（各入口的既有文案）。
func writeStoreGateError(c *gin.Context, gerr *storeGateError, notFoundMsg string) {
	switch gerr.kind {
	case gateInvalidID:
		response.BadRequest(c, "invalid seller id")
	case gateRPCError:
		pkglogger.Info(c.Request.Context(), "Failed to check store", zap.Error(gerr.err))
		response.GrpcErr(c, "seller", gerr.err)
	case gateRPCResult:
		response.GrpcResult(c, gerr.code, gerr.message, nil)
	default:
		response.NotFound(c, notFoundMsg)
	}
}

// requireStore 是 checkStoreGate 的 gin 薄包裝：解析路徑上的 store_id（不收
// store_code）、檢查、失敗時寫好回應。呼叫端在 ok 為 false 時必須立刻 return。
func (h *Holder) requireStore(c *gin.Context, idParam string, mode storeGateMode) (int64, bool) {
	storeID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || storeID <= 0 {
		response.BadRequest(c, "invalid seller id")
		return 0, false
	}
	if gerr := h.checkStoreGate(c.Request.Context(), storeID, mode); gerr != nil {
		writeStoreGateError(c, gerr, "seller not found")
		return 0, false
	}
	return storeID, true
}
