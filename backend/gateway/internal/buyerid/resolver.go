package buyerid

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/calleridentity"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNoCurrentBuyer means the account has no current buyer stint.
var ErrNoCurrentBuyer = errors.New("buyerid: account has no current buyer")

type BuyerGetter interface {
	GetProfile(ctx context.Context, accountID int64) (*pb.BuyerInfo, error)
}

// GetterFunc adapts a plain func to BuyerGetter（測試 seam）。
type GetterFunc func(ctx context.Context, accountID int64) (*pb.BuyerInfo, error)

// GetProfile implements BuyerGetter.
func (f GetterFunc) GetProfile(ctx context.Context, accountID int64) (*pb.BuyerInfo, error) {
	return f(ctx, accountID)
}

type Resolver struct{ buyer BuyerGetter }

// New 建立業務身分解析器。
func New(buyer BuyerGetter) *Resolver { return &Resolver{buyer: buyer} }

// CurrentBuyerID resolves accountID to the CURRENT buyer_id.
// 回傳 ErrNoCurrentBuyer 表示無現任買家 stint。
func (r *Resolver) CurrentBuyerID(ctx context.Context, accountID identity.AccountID) (identity.BuyerID, error) {
	rawAccountID := int64(accountID)

	if caller, ok := calleridentity.From(ctx); ok && !caller.IsAdmin && caller.AccountID == rawAccountID {
		if caller.BuyerID == 0 {
			return 0, ErrNoCurrentBuyer
		}
		return identity.BuyerID(caller.BuyerID), nil
	}

	resp, err := r.buyer.GetProfile(ctx, rawAccountID)
	if err != nil {
		// EnvelopeGuardInterceptor 已把「gRPC OK + resp.Code!=0」轉成真正的 gRPC
		// error，resp 在這裡永遠拿不到帶 code!=0 的值——判斷一律走 status.Code(err)
		// （storeid resolver 曾因誤判 resp.Code 犯過同款死碼 bug，PR #1560 merge
		// 後 E2E 才挖到）。
		if status.Code(err) == codes.NotFound {
			return 0, ErrNoCurrentBuyer
		}
		return 0, err
	}

	// err == nil 保證 resp.GetCode() == 0（成功）；resp 為 nil/empty 是防禦性
	// 分支（buyer 模組的 handler 現行邏輯不會走到，但保留給未來 nil-Data 成功
	// response 的可能）。
	if resp == nil || resp.GetId() == "" {
		return 0, ErrNoCurrentBuyer
	}
	id, perr := strconv.ParseInt(resp.GetId(), 10, 64)
	if perr != nil || id == 0 {
		return 0, fmt.Errorf("buyerid: buyer module returned empty identity for account %d", rawAccountID)
	}
	return identity.BuyerID(id), nil
}
