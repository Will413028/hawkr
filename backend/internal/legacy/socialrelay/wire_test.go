package socialrelay

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// 舊 publisher 的 struct 在另一個 Go module（pkg），單體不能 import：這裡讀它的原始碼，逐欄位比對 json key、型別與 tag 選項。
const mqEvents = "../../../pkg/mq/events.go"

type wireField struct{ typ, tag string }

func oldFields(t *testing.T, name string) map[string]wireField {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), mqEvents, nil, 0)
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
				if inner := find(typeName(f.Type)); inner != nil {
					collect(inner, into)
				}
				continue
			}
			tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
			key, opts, _ := strings.Cut(tag, ",")
			into[key] = wireField{typ: typeName(f.Type), tag: opts}
		}
	}
	st := find(name)
	if st == nil {
		t.Fatalf("struct %s not found in %s", name, mqEvents)
	}
	out := map[string]wireField{}
	collect(st, out)
	// BaseEvent 的 correlation_id、metadata 是 omitempty 的空值，從不出現在 wire 上，relay 的 baseEvent 不帶。
	delete(out, "correlation_id")
	delete(out, "metadata")
	return out
}

func typeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		if v.Name == "EventType" {
			return "string"
		}
		return v.Name
	case *ast.StarExpr:
		return "*" + typeName(v.X)
	case *ast.SelectorExpr:
		return typeName(v.X) + "." + v.Sel.Name
	}
	return "?"
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
		name := f.Type.String()
		if f.Type.Kind() == reflect.Pointer {
			name = "*" + f.Type.Elem().Name()
		}
		out[key] = wireField{typ: name, tag: opts}
	}
	return out
}

// uuidKeys 是 A7 把 int64 改成 UUID 字串的 id 欄位（計畫不變量，第 76 行；notification 以 toStringVal 當字串用）：型別 int64 → string，
// 舊的 ,string 選項不再需要（字串本來就是 JSON 字串）；json key、omitempty 都不變。
var uuidKeys = map[string]bool{"post_id": true, "postId": true, "commentId": true, "parentCommentId": true, "parentId": true, "replyToId": true}

// intentionallyAbsentOnTheWire 是 relay 的 struct 有、但永遠不送（omitempty 的空值）的欄位：clientRequestId 是舊留言 SSE 的去重用欄位，
// notification-service 不讀它（rg -n clientRequestId services/notification-service 無命中）。
//
// 這個欄位（clientRequestId）留在 struct 裡、永遠是空值，所以 json key 與 omitempty 仍和舊的逐欄位相同。
//
// 另兩個刻意的差別（不是 wire 形狀）：留言預覽以 rune 截斷（舊的以位元組截斷，會切在 UTF-8 的中間），沒有的 parentId、replyToId 是空字串（舊的
// 是 "0"，只有已刪除的 SSE 讀它）。

func expected(old map[string]wireField) map[string]wireField {
	out := map[string]wireField{}
	for k, v := range old {
		switch {
		case v.typ == "*CommentSnapshot":
			v.typ = "*commentSnapshot"
		case uuidKeys[k]:
			v.typ = "string"
			opts := []string{}
			for _, o := range strings.Split(v.tag, ",") {
				if o != "" && o != "string" {
					opts = append(opts, o)
				}
			}
			v.tag = strings.Join(opts, ",")
		}
		// 欄位的 Go 型別：舊的 int64 在新 struct 同樣是 int64，其他同名。
		out[k] = v
	}
	return out
}

func TestPayloadsMatchTheLegacyStructs(t *testing.T) {
	for name, c := range map[string]struct {
		old  string
		mine reflect.Type
	}{
		"post.created":            {"PostCreatedEvent", reflect.TypeOf(postCreated{})},
		"comment.created":         {"CommentCreatedEvent", reflect.TypeOf(commentCreated{})},
		"comment.replied":         {"CommentRepliedEvent", reflect.TypeOf(commentReplied{})},
		"post.liked":              {"PostLikedEvent", reflect.TypeOf(postLiked{})},
		"comment.like_changed":    {"CommentLikeChangedEvent", reflect.TypeOf(commentLikeChanged{})},
		"post.hidden_by_platform": {"PostHiddenByPlatformEvent", reflect.TypeOf(postHiddenByPlatform{})},
		"CommentSnapshot":         {"CommentSnapshot", reflect.TypeOf(commentSnapshot{})},
	} {
		want, got := expected(oldFields(t, c.old)), newFields(c.mine)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: payload fields differ from %s\n old(expected) %v\n new           %v", name, c.old, want, got)
		}
	}
}

// 舊 BaseEvent 帶 correlation_id 與 metadata（omitempty）：relay 的 baseEvent 少了這兩個，是因為舊 publisher 從來不設它們。
// 這個測試守住「舊的 omitempty」這個前提。
func TestLegacyBaseEventOmitsTheKeysTheRelayDoesNotCarry(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), mqEvents, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "BaseEvent" {
			for _, f := range ts.Type.(*ast.StructType).Fields.List {
				tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
				key, opts, _ := strings.Cut(tag, ",")
				found[key] = opts
			}
		}
		return true
	})
	mine := newFields(reflect.TypeOf(baseEvent{}))
	for k, opts := range found {
		if _, ok := mine[k]; ok {
			continue
		}
		if (k != "correlation_id" && k != "metadata") || opts != "omitempty" {
			t.Errorf("relay baseEvent lacks %q (%q) that the legacy BaseEvent always writes", k, opts)
		}
	}
}
