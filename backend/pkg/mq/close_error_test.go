package mq

import (
	"errors"
	"fmt"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestCloseError 釘住 closeError 的兩件事：nil 的處置，以及「包指標」。
//
// 後者是本檔存在的主因：原本這段內聯在 connectAndConsume 的 select 裡，那條
// 路徑要真的連上 broker 才走得到，於是包裝形狀完全沒有觀測點——把 %w 的運算元
// 從指標改成值，整個 package 仍然全綠。
func TestCloseError(t *testing.T) {
	t.Run("nil 代表正常收掉，不帶原因", func(t *testing.T) {
		err := closeError("connection", nil)
		if err == nil {
			t.Fatal("closeError(nil) 不該回 nil——重連迴圈靠回傳的 error 判斷這一輪結束了")
		}
		if got, want := err.Error(), "connection closed"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
		var target *amqp.Error
		if errors.As(err, &target) {
			t.Error("沒有 amqp 原因時不該解得出 amqp.Error")
		}
	})

	t.Run("非 nil：訊息帶原因，且解得出 *amqp.Error", func(t *testing.T) {
		e := &amqp.Error{Code: amqp.ChannelError, Reason: "CHANNEL_ERROR - second 'channel.open'"}
		err := closeError("channel", e)

		if got := err.Error(); got != "channel: "+e.Error() {
			t.Errorf("message = %q, want prefix+reason", got)
		}

		// 包成值的話這兩個斷言會失敗——errors.As 的目標型別對不上、
		// errors.Is 比的不是同一個指標。
		var target *amqp.Error
		if !errors.As(err, &target) {
			t.Fatal("應該解得出 *amqp.Error")
		}
		if target.Code != amqp.ChannelError {
			t.Errorf("Code = %d, want %d", target.Code, amqp.ChannelError)
		}
		if !errors.Is(err, e) {
			t.Error("errors.Is 對同一個 *amqp.Error 應該成立")
		}
	})

	t.Run("再包一層仍然解得出來", func(t *testing.T) {
		e := &amqp.Error{Code: amqp.NotFound, Reason: "NOT_FOUND"}
		err := fmt.Errorf("outer: %w", closeError("connection", e))

		var target *amqp.Error
		if !errors.As(err, &target) || target.Code != amqp.NotFound {
			t.Error("多層包裝之後 errors.As 仍必須解得到——這是包裝而非只印字串的唯一理由")
		}
	})
}
