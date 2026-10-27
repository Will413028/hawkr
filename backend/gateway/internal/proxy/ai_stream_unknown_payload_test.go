package proxy

import (
	"bytes"
	"testing"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// writeStreamEvent 的 9 個 oneof case 每個都有專屬測試（最新的 Activity 在
// ai_assistant_stream_test.go 的 TestWriteStreamEvent_Activity_*），只有 default 分支沒有。
// 這支補上，鎖住兩件事：
//
//  1. 未知 payload **不得**產生任何 frame。default 分支的註解說明了理由——翻譯層不知道
//     該把它譯成哪個 AG-UI 事件，硬造一個 CUSTOM 只會讓消費端收到形狀不明的資料。
//     沒有測試的話，日後有人「順手」在 default 補一個 frame 不會有任何東西變紅。
//  2. 不得 panic。這條路徑跑在 SSE relay 迴圈裡，panic 會直接斷掉整條串流。
//
// 用 Payload 為 nil 的 StreamEvent 觸發 default：type switch 對 nil 介面值會落進
// default，且不需要為了測試在 proto 裡新增一個假的 oneof 分支。
func TestWriteStreamEvent_UnknownPayload_EmitsNothing(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}

	writeStreamEvent(&buf, &aipb.StreamEvent{}, state, "thread-1", "run-1")

	if out := buf.String(); out != "" {
		t.Fatalf("未知 payload 不得寫出任何 frame，實際寫出 %q\n"+
			"（若這是刻意改動：翻譯層無法得知未知 payload 的語意，造 frame 等於把形狀不明的\n"+
			"資料送給消費端；正確做法是先在此新增對應的 case）", out)
	}
	if state.runEnded {
		t.Fatal("未知 payload 不得把 run 標成已結束——那會讓後續真正的終端事件被吞掉")
	}
}
