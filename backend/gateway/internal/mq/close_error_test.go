package mq

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestConnCloseError 釘住 connCloseError 的 nil 處置與「包值不包指標」。
// 理由與 pkg/mq 的同名測試相同：原位置在 runOnce 的 select 裡，要真的連上
// broker 才走得到，內聯的話這兩件事沒有任何觀測點。
func TestConnCloseError(t *testing.T) {
	t.Run("nil 不得 panic，且回一個沒有 amqp 原因的 error", func(t *testing.T) {
		// 這正是原本的缺口：舊碼只判 channel 是否關閉，NotifyClose 送 nil 時會把
		// 一個 nil *amqp.Error 包進 error 介面，之後呼叫 Error() 就 panic。
		err := connCloseError(nil)
		if err == nil {
			t.Fatal("不該回 nil")
		}
		if got, want := err.Error(), "amqp connection closed"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
		var target amqp.Error
		if errors.As(err, &target) {
			t.Error("沒有 amqp 原因時不該解得出 amqp.Error")
		}
	})

	t.Run("非 nil：解得出值型別的 amqp.Error", func(t *testing.T) {
		e := &amqp.Error{Code: amqp.ConnectionForced, Reason: "CONNECTION_FORCED - broker forced connection closure"}
		err := connCloseError(e)

		var target amqp.Error
		if !errors.As(err, &target) {
			t.Fatal("應該解得出 amqp.Error（值型別）——包指標的話這裡會失敗")
		}
		if target.Code != amqp.ConnectionForced {
			t.Errorf("Code = %d, want %d", target.Code, amqp.ConnectionForced)
		}
	})
}
