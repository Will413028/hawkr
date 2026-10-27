package media

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestInternalAuthForwardsAccountHeaders 證明 forwardAccountHeaders 這個旗標真的有作用。
//
// 🔴 為什麼這一支 client 與 invoice-service 其他三支不同：
// media-service 的 FinalizeImages 用 `GrpcAuthContext.requireAccountId()` 取
// **使用者**身分，再與 staged token 的 `account_id` 比對；不一致回「資產不存在」
// （防枚舉，與真的不存在同碼）。其他三支的對端授權的是**服務**身分。
// 差異來自對端怎麼授權，不是慣例——所以四支長得一樣時，最容易被「順手統一」改壞。
//
// 2026-09-01 dev 實測：旗標是 false 時端點 18（存草稿）**每一次**都在 finalize
// 稅籍證明時 500，而症狀完全誤導——upload_session 是 COMPLETED、物件真的上傳了，
// 只有 media_asset 一列都沒有，錯誤訊息是「資產不存在」。
func TestInternalAuthForwardsAccountHeaders(t *testing.T) {
	for _, tc := range []struct {
		name    string
		forward bool
		want    bool
	}{
		{"forward=true 轉發使用者身分", true, true},
		{"forward=false 不轉發（其他 peer client 的語義）", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ic := interceptor.InternalAuthClientInterceptor("invoice-service", tc.forward)
			in := metadata.Pairs("x-account-id", "2642010415", "x-account-type", "2")
			ctx := metadata.NewIncomingContext(context.Background(), in)

			var got metadata.MD
			invoker := func(c context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				got, _ = metadata.FromOutgoingContext(c)
				return nil
			}
			if err := ic(ctx, "/m/M", nil, nil, nil, invoker); err != nil {
				t.Fatalf("interceptor: %v", err)
			}
			has := len(got.Get("x-account-id")) > 0 && got.Get("x-account-id")[0] == "2642010415"
			if has != tc.want {
				t.Fatalf("x-account-id 轉發=%v, want %v（md=%v）", has, tc.want, got)
			}
		})
	}
}

// TestNewClientUsesAccountForwarding 守 NewClient 真的傳 true。
//
// interceptor 是閉包，撥號後拿不回它的參數 ⇒ 只能錨在原始碼。這比不驗好：
// 上面那支證明「旗標有用」，這一支證明「我們用了它」，缺任一半都擋不住迴歸。
func TestNewClientUsesAccountForwarding(t *testing.T) {
	src, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	if !strings.Contains(string(src), `InternalAuthClientInterceptor("invoice-service", true)`) {
		t.Fatal(`media peer client 沒有傳 forwardAccountHeaders=true：` +
			`FinalizeEvidence 會每次都回「資產不存在」，而端點 18 因此永遠存不了草稿`)
	}
}
