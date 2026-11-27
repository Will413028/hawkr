package reviewrelay

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

const mqEvents = "../../../pkg/mq/events.go"

type wireField struct{ typ, tag string }

func oldFields(t *testing.T, path, name string) map[string]wireField {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	find := func(n string) *ast.StructType {
		var out *ast.StructType
		ast.Inspect(file, func(node ast.Node) bool {
			if ts, ok := node.(*ast.TypeSpec); ok && ts.Name.Name == n {
				out, _ = ts.Type.(*ast.StructType)
			}
			return true
		})
		return out
	}
	var collect func(*ast.StructType, map[string]wireField)
	collect = func(st *ast.StructType, into map[string]wireField) {
		for _, f := range st.Fields.List {
			if len(f.Names) == 0 {
				if inner := find(types(f.Type)); inner != nil {
					collect(inner, into)
				}
				continue
			}
			tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
			key, opts, _ := strings.Cut(tag, ",")
			into[key] = wireField{typ: normalize(types(f.Type)), tag: opts}
		}
	}
	st := find(name)
	if st == nil {
		t.Fatalf("%s: struct %s not found", path, name)
	}
	out := map[string]wireField{}
	collect(st, out)
	return out
}

func types(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return types(v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + types(v.Elt)
	}
	return "?"
}

func normalize(typ string) string {
	if typ == "EventType" {
		return "string"
	}
	return typ
}

func newFields(typ reflect.Type) map[string]wireField {
	out := map[string]wireField{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Anonymous {
			for k, v := range newFields(f.Type) {
				out[k] = v
			}
			continue
		}
		key, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		out[key] = wireField{typ: f.Type.String(), tag: opts}
	}
	return out
}

func TestPayloadsMatchTheLegacyStructs(t *testing.T) {
	for name, c := range map[string]struct {
		old  string
		mine reflect.Type
	}{
		"review.created": {"ReviewCreatedEvent", reflect.TypeOf(reviewCreated{})},
	} {
		want, got := oldFields(t, mqEvents, c.old), newFields(c.mine)
		delete(want, "correlation_id")
		delete(want, "metadata")
		if f, ok := want["product_id"]; ok && f.typ == "int64" {
			f.typ = "string"
			want["product_id"] = f
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: payload fields differ from %s\n old %v\n new %v", name, c.old, want, got)
		}
	}
}

// 舊的 BaseEvent 帶 correlation_id 與 metadata（omitempty）：relay 的 baseEvent 少了這兩個，是因為舊 publisher 從來不設它們。
// 這個測試守住「舊的 omitempty」這個前提。
func TestLegacyBaseEventOmitsTheKeysTheRelayDoesNotCarry(t *testing.T) {
	old := oldFields(t, mqEvents, "BaseEvent")
	mine := newFields(reflect.TypeOf(baseEvent{}))
	for k, v := range old {
		if _, ok := mine[k]; ok {
			continue
		}
		if k != "correlation_id" && k != "metadata" || v.tag != "omitempty" {
			t.Errorf("relay baseEvent lacks %q (%+v) that the legacy BaseEvent always writes", k, v)
		}
	}
}
