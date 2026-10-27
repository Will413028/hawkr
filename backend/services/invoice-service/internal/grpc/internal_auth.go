package grpc

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// errNotInternalCall 是內部限定 RPC 對使用者面呼叫的唯一回應。
//
// 訊息刻意不提「你缺了哪個 header」：那等於教外面的人怎麼把自己偽裝成內部呼叫。
var errNotInternalCall = status.Error(codes.PermissionDenied, "internal-only rpc")

// requireInternalCall 是內部限定 RPC 的閘門（§5.2）。
//
// **分工必須看清楚，否則會以為這一行就是驗證**：
//
//	pkg/middleware/grpc/interceptor.InternalAuthServerInterceptor  ← 認證（HMAC 驗簽）
//	requireInternalCall（本函式）                                    ← 授權（這支只給內部呼叫）
//
// interceptor 只對「帶 x-internal-service 的呼叫」驗簽，**不帶的一律放行**（那是給
// gateway→service 的使用者面路徑用的，走各自的 requireAccountType）。所以少了本函式，
// 這兩支 RPC 就是**任何人都打得到**的：gateway 只要有一條路由代理過來，
// 買家就能拿別人的 store_id／buyer_id 去問「這位賣家的接單上限用掉多少」。
//
// 🔴 反過來也要記得：本函式**不是**認證。x-internal-service 是明文字串，
// 「不可偽造」來自 interceptor 在 `INTERNAL_AUTH_MODE=enforcing` 下的驗簽。
// 在 off／permissive 期間這道閘門擋得住的是「使用者面誤接」，不是同 namespace 的 rogue pod
// —— 那是 rollout 的已知窗口（52 號設計書），不是本函式可以補的洞。
func requireInternalCall(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errNotInternalCall
	}
	for _, v := range md.Get(interceptor.HeaderInternalService) {
		if v != "" {
			return nil
		}
	}
	return errNotInternalCall
}
