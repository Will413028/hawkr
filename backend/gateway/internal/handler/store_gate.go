package handler

import (
	"context"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type storeGateMode int

const (
	gateExists storeGateMode = iota
	gateCard
	gateExposable
)

type storeGateKind int

const (
	gateInvalidID storeGateKind = iota + 1
	gateNotFound
	gateRPCError
	gateRPCResult
)

type storeGateError struct {
	err     error
	message string
	code    int32
	kind    storeGateKind
}

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
