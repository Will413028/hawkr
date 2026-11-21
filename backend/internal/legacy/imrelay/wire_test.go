package imrelay

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// 舊 publisher 的 struct 在另一個 Go module（im-service），單體不能 import：這裡讀它的原始碼，逐欄位比對 json tag 與 Go 型別，
// relay 的 payload struct 只要和舊的有一個欄位不同就失敗。chat.message.sent 的 struct 在 im-service 的 usecase，不在 pkg/mq/events.go。
const legacyEvent = "../../../services/im-service/internal/usecase/message.go"

type wireField struct{ typ, tag string }

func oldFields(t *testing.T, path, name string) map[string]wireField {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var st *ast.StructType
	ast.Inspect(file, func(node ast.Node) bool {
		if ts, ok := node.(*ast.TypeSpec); ok && ts.Name.Name == name {
			st, _ = ts.Type.(*ast.StructType)
		}
		return true
	})
	if st == nil {
		t.Fatalf("%s: struct %s not found", path, name)
	}
	out := map[string]wireField{}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			t.Fatalf("%s 有內嵌欄位：測試要先支援展開", name)
		}
		ident, ok := f.Type.(*ast.Ident)
		if !ok {
			t.Fatalf("%s.%s 的型別不是簡單型別", name, f.Names[0].Name)
		}
		key, opts, _ := strings.Cut(reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json"), ",")
		out[key] = wireField{typ: ident.Name, tag: opts}
	}
	return out
}

func newFields(typ reflect.Type) map[string]wireField {
	out := map[string]wireField{}
	for i := range typ.NumField() {
		f := typ.Field(i)
		key, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		out[key] = wireField{typ: f.Type.String(), tag: opts}
	}
	return out
}

func TestPayloadMatchesTheLegacyStruct(t *testing.T) {
	want, got := oldFields(t, legacyEvent, "ChatMessageSentEvent"), newFields(reflect.TypeOf(chatMessageSent{}))
	if len(want) != 10 {
		t.Fatalf("舊 struct 有 %d 個欄位，預期 10 個：%v", len(want), want)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("chat.message.sent: payload fields differ from ChatMessageSentEvent\n old %v\n new %v", want, got)
	}
}
