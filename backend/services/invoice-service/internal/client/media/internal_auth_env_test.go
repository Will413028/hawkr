package media

import (
	"os"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
)

// INTERNAL_AUTH_MODE 沒有預設值（見 interceptor.ResolveInternalAuthMode：缺值與
// 無法辨識的值都是 error，因為三個模式裡沒有一個同時是安全的又不會擋住未設定的
// 環境）。因此每個會建構 gRPC server 或內部 gRPC client 的測試二進位，都必須自己
// 宣告一次要跑在哪個模式底下。
//
// 這裡選 off：本套件測的不是 HMAC 驗簽本身——那些在
// pkg/middleware/grpc/interceptor 自己的測試裡，各自 t.Setenv 指定模式。
//
// 用 init() 而不是 TestMain，是為了不與套件既有的 TestMain（或不同 build tag 下
// 各自一份的 TestMain）相撞。
func init() {
	if err := os.Setenv("INTERNAL_AUTH_MODE", interceptor.InternalAuthModeOff); err != nil {
		panic(err)
	}
}
