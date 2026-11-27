package handler

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
)

func mustNotResolveBuyer(t *testing.T) *buyerid.Resolver {
	t.Helper()
	return buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		t.Fatal("buyer resolver must not be called for anonymous request (accountID==0)")
		return nil, nil
	}))
}
