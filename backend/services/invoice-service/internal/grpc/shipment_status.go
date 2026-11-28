package grpc

import (
	"context"

	"go.uber.org/zap"
)

// ShipmentStatusResolver 是出貨狀態的出向面（實作在 client/shipping）。
//
// 介面宣告在消費端：本層只需要「給我一批 order_no、還我狀態碼」，
// 不需要知道 shipment 還有哪些欄位。
type ShipmentStatusResolver interface {
	BatchGetShipmentStatus(ctx context.Context, orderNos []string) (map[string]string, error)
}

func (s *InvoiceServer) resolveShipmentStatuses(ctx context.Context, orderNos []string) (map[string]string, bool) {
	ids := dedupeStrings(orderNos)
	if s.shipmentStatuses == nil || len(ids) == 0 {
		return map[string]string{}, false
	}
	statuses, err := s.shipmentStatuses.BatchGetShipmentStatus(ctx, ids)
	if err != nil {
		if s.log != nil {
			s.log.Warn("invoice.query.shipment_status_unavailable",
				zap.Int("order_count", len(ids)),
				zap.String("degrade", "shipment_status 整批退回空字串，回應帶 shipmentStatusDegraded=true"),
				zap.Error(err))
		}
		return map[string]string{}, true
	}
	return statuses, false
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
