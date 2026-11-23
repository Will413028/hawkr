package imrelay

import (
	"reflect"
	"strings"
	"testing"
)

type wireField struct{ typ, tag string }

func newFields(typ reflect.Type) map[string]wireField {
	out := map[string]wireField{}
	for i := range typ.NumField() {
		f := typ.Field(i)
		key, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		out[key] = wireField{typ: f.Type.String(), tag: opts}
	}
	return out
}

// chat.message.sent 的 wire 形狀是 notification 消費端的契約：十個欄位全是 string、沒有 omitempty，沒有 BaseEvent 外殼。
// 舊 im-service 刪除後，這份清單就是唯一來源；模板讀的 key 另由 golden_test 對照。
func TestPayloadMatchesTheWireContract(t *testing.T) {
	want := map[string]wireField{}
	for _, key := range []string{
		"message_id", "conversation_id", "sender_id", "sender_name", "sender_avatar",
		"receiver_id", "receiver_type", "store_id", "content_preview", "sent_at",
	} {
		want[key] = wireField{typ: "string"}
	}
	if got := newFields(reflect.TypeOf(chatMessageSent{})); !reflect.DeepEqual(want, got) {
		t.Errorf("chat.message.sent: payload fields differ from the wire contract\n want %v\n got  %v", want, got)
	}
}
