// pkg/grpcclient/dial_test.go
package grpcclient

import (
	"testing"

	"google.golang.org/grpc"
)

// TestDial_PrependsInsecureTransportCredentials 鎖住 Dial 的核心價值：呼叫端不用
// 再自己傳 grpc.WithTransportCredentials(insecure.NewCredentials())。若 Dial 沒有
// 自動補上，grpc.NewClient 會同步回傳 "no transport security set" 錯誤（已用一支
// 獨立程式對本專案實際的 grpc-go 版本驗證過這個行為）。
func TestDial_PrependsInsecureTransportCredentials(t *testing.T) {
	conn, err := Dial("localhost:50051")
	if err != nil {
		t.Fatalf("Dial() error = %v, want nil (insecure credentials should be applied automatically)", err)
	}
	defer conn.Close()
}

// TestDial_PassesTargetThrough 鎖住 target 字串原樣傳給 grpc.NewClient，不被 Dial
// 改寫、截斷或加上任何前後綴。
func TestDial_PassesTargetThrough(t *testing.T) {
	const target = "media-service.internal:9090"
	conn, err := Dial(target)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()
	if got := conn.Target(); got != target {
		t.Errorf("conn.Target() = %q, want %q", got, target)
	}
}

// TestDial_ForwardsCallerSuppliedDialOptions 證明呼叫端傳入的 opts 真的被轉發給
// grpc.NewClient，不是被 Dial 悄悄丟掉。用一個語法錯誤的
// grpc.WithDefaultServiceConfig JSON 當探針：grpc.NewClient 對它會在建連線當下
// （同步、不需要真的連網路）直接回錯；若 Dial 沒把 opts 轉發下去，這裡就不會出錯。
func TestDial_ForwardsCallerSuppliedDialOptions(t *testing.T) {
	_, err := Dial("localhost:50051", grpc.WithDefaultServiceConfig(`{not valid json`))
	if err == nil {
		t.Fatal("Dial() error = nil, want an error from the malformed default service config passed through opts")
	}
}
