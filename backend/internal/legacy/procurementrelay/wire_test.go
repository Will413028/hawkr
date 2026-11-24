package procurementrelay

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

const oldPublisher = "../../../services/ai-service/internal/procurement/commerce/publication_execute.go"

// 舊 struct 在另一個 Go module，單體不能 import：讀它的原始碼，逐欄位比對 json tag 與型別。
func TestPayloadMatchesTheLegacyStruct(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), oldPublisher, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var st *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "inviteCreatedPayload" {
			st, _ = ts.Type.(*ast.StructType)
		}
		return true
	})
	if st == nil {
		t.Fatal("inviteCreatedPayload not found in the legacy publisher")
	}
	want := map[string]string{}
	for _, f := range st.Fields.List {
		tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
		want[tag] = f.Type.(*ast.Ident).Name
	}
	got := map[string]string{}
	typ := reflect.TypeOf(inviteCreatedPayload{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		got[f.Tag.Get("json")] = f.Type.String()
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("payload fields differ from the legacy struct\n old %v\n new %v", want, got)
	}
}
