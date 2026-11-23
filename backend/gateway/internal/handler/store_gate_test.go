package handler

import (
	"context"
	"sync/atomic"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
)

func storeGateHolder(getSeller, meta *atomic.Int32, m map[int64]*sellerpb.SellerMeta) *Holder {
	return &Holder{Seller: grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
		GetSellerFn: func(context.Context, *sellerpb.GetSellerRequest, ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			getSeller.Add(1)
			return &sellerpb.SellerResponse{Data: &sellerpb.SellerInfo{StoreId: "100"}}, nil
		},
		BatchGetSellerMetaFn: func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			meta.Add(1)
			return &sellerpb.BatchGetSellerMetaResponse{Sellers: m}, nil
		},
	})}
}

func TestCheckStoreGate_RPCCountAndMode(t *testing.T) {
	cases := []struct {
		name      string
		mode      storeGateMode
		meta      *sellerpb.SellerMeta
		wantOK    bool
		wantGet   int32
		wantMetaN int32
	}{
		{"exposable ok: meta only", gateExposable, &sellerpb.SellerMeta{CanAcceptOrders: true}, true, 0, 1},
		{"exposable not accepting", gateExposable, &sellerpb.SellerMeta{HasPublicCard: true}, false, 0, 1},
		{"exposable missing entry", gateExposable, nil, false, 0, 1},
		{"card ok", gateCard, &sellerpb.SellerMeta{HasPublicCard: true}, true, 1, 1},
		{"card never public", gateCard, &sellerpb.SellerMeta{CanAcceptOrders: true}, false, 1, 1},
		{"exists: GetSeller only", gateExists, nil, true, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var g, m atomic.Int32
			mm := map[int64]*sellerpb.SellerMeta{}
			if tc.meta != nil {
				mm[100] = tc.meta
			}
			h := storeGateHolder(&g, &m, mm)
			gerr := h.checkStoreGate(context.Background(), 100, tc.mode)
			if (gerr == nil) != tc.wantOK {
				t.Fatalf("ok = %v, want %v (%+v)", gerr == nil, tc.wantOK, gerr)
			}
			if g.Load() != tc.wantGet || m.Load() != tc.wantMetaN {
				t.Fatalf("GetSeller=%d meta=%d, want %d/%d", g.Load(), m.Load(), tc.wantGet, tc.wantMetaN)
			}
		})
	}
}

// 已關閉的店：GetSeller 回 code != 0（或沒有 data），gateExists 與 gateCard 都必須擋下。
func TestCheckStoreGate_ClosedStoreFromGetSeller_Denied(t *testing.T) {
	for _, mode := range []storeGateMode{gateExists, gateCard} {
		for name, resp := range map[string]*sellerpb.SellerResponse{
			"code":    {Code: 9233, Message: "商家不存在或已關閉"},
			"no data": {},
		} {
			h := &Holder{Seller: grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
				GetSellerFn: func(context.Context, *sellerpb.GetSellerRequest, ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
					return resp, nil
				},
				BatchGetSellerMetaFn: func(context.Context, *sellerpb.BatchGetSellerMetaRequest, ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
					return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{100: {HasPublicCard: true}}}, nil
				},
			})}
			if gerr := h.checkStoreGate(context.Background(), 100, mode); gerr == nil {
				t.Fatalf("mode %d / %s: closed store allowed", mode, name)
			}
		}
	}
}
