package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext/icu"
)

func jsString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

const tsHeader = "// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。\n"

// tsImports 管理列舉型別的 import：同名但來自不同檔的用別名。
type tsImports struct {
	names map[string]string // 全名 → 在檔案裡使用的名稱
	used  map[string]string // 使用的名稱 → 全名
	enums []*EnumInfo
}

func newTSImports(enums []*EnumInfo) *tsImports {
	t := &tsImports{names: map[string]string{}, used: map[string]string{}}
	for _, e := range enums {
		name := enumTypeName(e)
		if prev, clash := t.used[name]; clash && prev != string(e.Desc.FullName()) {
			name = pascal(e.Path)
		}
		t.names[string(e.Desc.FullName())] = name
		t.used[name] = string(e.Desc.FullName())
		t.enums = append(t.enums, e)
	}
	return t
}

func (t *tsImports) name(e *EnumInfo) string { return t.names[string(e.Desc.FullName())] }

func (t *tsImports) write(b *strings.Builder) {
	byModule := map[string][]string{}
	var modules []string
	for _, e := range t.enums {
		mod := tsModule(e)
		if _, ok := byModule[mod]; !ok {
			modules = append(modules, mod)
		}
		spec := enumTypeName(e)
		if n := t.name(e); n != spec {
			spec += " as " + n
		}
		byModule[mod] = append(byModule[mod], spec)
	}
	sort.Strings(modules)
	for _, mod := range modules {
		specs := byModule[mod]
		sort.Strings(specs)
		fmt.Fprintf(b, "import type { %s } from %s;\n", strings.Join(specs, ", "), jsString(mod))
	}
}

func constName(path string) string {
	var b strings.Builder
	prevLower := false
	for _, r := range path {
		switch {
		case r == '.' || r == '_':
			b.WriteByte('_')
			prevLower = false
		case r >= 'A' && r <= 'Z':
			if prevLower {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			prevLower = false
		default:
			b.WriteString(strings.ToUpper(string(r)))
			prevLower = r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		}
	}
	return b.String()
}

func tsParamType(p Param, imp *tsImports) string {
	switch p.Kind { //nolint:exhaustive // 其餘種類走預設分支
	case ParamInteger:
		return "number"
	case ParamEnum, ParamNoun:
		return imp.name(p.Enum)
	}
	return "string"
}

func (m *Model) tsKeyUnion() []string {
	var keys []string
	for _, f := range m.Funcs {
		for _, c := range f.Cases {
			keys = append(keys, c.Key.Raw)
		}
	}
	sort.Strings(keys)
	return keys
}

// EmitTSKeys 生成 keys.gen.ts：UiTextKey 與 message 建構函式。
func (m *Model) EmitTSKeys() []byte {
	imp := newTSImports(m.usedEnums())
	var b strings.Builder
	b.WriteString(tsHeader)
	b.WriteString("import { Code } from \"@connectrpc/connect\";\n")
	imp.write(&b)
	b.WriteString("import type { UiMessage } from \"./message\";\n\n")
	b.WriteString("export type { DefaultErrorText, UiMessage } from \"./message\";\n\n")
	b.WriteString("export type UiTextKey =\n")
	for _, k := range m.tsKeyUnion() {
		fmt.Fprintf(&b, "\t| %s\n", jsString(k))
	}
	b.WriteString("\t;\n\n")
	// 值號 → 值名（select 參數用）。
	for _, e := range m.selectEnums() {
		fmt.Fprintf(&b, "/** %s 的值號到值名；ICU select 的分支用值名。 */\nexport const %s_NAME: Readonly<Record<number, string>> = {\n", e.Path, constName(e.Path))
		for _, v := range e.Values {
			fmt.Fprintf(&b, "\t%d: %s,\n", v.Number(), jsString(string(v.Name())))
		}
		b.WriteString("};\n\n")
	}
	for _, f := range m.Funcs {
		m.tsFunc(&b, f, imp, AudDefault)
	}
	return []byte(b.String())
}

// tsKeyTable 回 f 在受眾 a 的「值號 → key」表（omit 的值不在表裡）。
func (m *Model) tsKeyTable(b *strings.Builder, f *Func, a Audience, name string) {
	fmt.Fprintf(b, "const %s: Readonly<Partial<Record<number, UiTextKey>>> = {\n", name)
	for _, c := range f.Cases {
		if f.omitted(a, c.Value) {
			continue
		}
		v := f.Enum.Desc.Values().ByName(protoreflectName(c.Value))
		fmt.Fprintf(b, "\t%d: %s,\n", v.Number(), jsString(c.Key.Raw))
	}
	b.WriteString("};\n\n")
}

func (m *Model) tsFunc(b *strings.Builder, f *Func, imp *tsImports, aud Audience) {
	name := lowerFirst(pascal(f.ID))
	if aud == AudStaff {
		name += "Staff"
	}
	var args []string
	if f.Kind == FuncNoun || f.Kind == FuncVariant {
		args = append(args, "v: "+imp.name(f.Enum))
	}
	if len(f.Params) > 0 {
		var fields []string
		for _, p := range f.Params {
			fields = append(fields, p.Name+": "+tsParamType(p, imp))
		}
		args = append(args, "a: { "+strings.Join(fields, "; ")+" }")
	}
	hasNoun := false
	for _, p := range f.Params {
		if p.Kind == ParamNoun {
			hasNoun = true
		}
	}
	valuesExpr := func() string {
		if len(f.Params) == 0 {
			return ""
		}
		var parts []string
		for _, p := range f.Params {
			switch p.Kind { //nolint:exhaustive // 其餘種類走預設分支
			case ParamEnum:
				parts = append(parts, fmt.Sprintf("%s: %s_NAME[a.%s] ?? \"\"", p.Name, constName(p.Enum.Path), p.Name))
			case ParamNoun:
				parts = append(parts, fmt.Sprintf("%s: { key: %sKey }", p.Name, p.Name))
			default:
				parts = append(parts, fmt.Sprintf("%s: a.%s", p.Name, p.Name))
			}
		}
		return ", values: { " + strings.Join(parts, ", ") + " }"
	}
	nounPrelude := func() {
		for _, p := range f.Params {
			if p.Kind == ParamNoun {
				fmt.Fprintf(b, "\tconst %sKey = %s[a.%s];\n\tif (%sKey === undefined) return undefined;\n", p.Name, constName(p.Enum.Path), p.Name, p.Name)
			}
		}
	}
	switch f.Kind {
	case FuncNoun:
		table := constName(f.ID)
		m.tsKeyTable(b, f, aud, table)
		fmt.Fprintf(b, "/** %s 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */\n", f.ID)
		fmt.Fprintf(b, "export function %s(%s): UiMessage | undefined {\n", name, strings.Join(args, ", "))
		fmt.Fprintf(b, "\tconst key = %s[v];\n\tif (key === undefined) return undefined;\n\treturn { key%s };\n}\n\n", table, valuesExpr())
	case FuncVariant:
		table := constName(f.ID)
		if aud == AudStaff {
			table += "_STAFF"
		}
		m.tsKeyTable(b, f, aud, table)
		fmt.Fprintf(b, "/** %s；%s 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */\n", f.ID, aud)
		fmt.Fprintf(b, "export function %s(%s): UiMessage | undefined {\n", name, strings.Join(args, ", "))
		fmt.Fprintf(b, "\tconst key = %s[v];\n\tif (key === undefined) return undefined;\n\treturn { key%s };\n}\n\n", table, valuesExpr())
	case FuncSentence:
		key := f.Cases[0].Key.Raw
		ret := "UiMessage"
		if hasNoun {
			ret = "UiMessage | undefined"
		}
		fmt.Fprintf(b, "/** %s */\nexport function %s(%s): %s {\n", f.ID, name, strings.Join(args, ", "), ret)
		nounPrelude()
		fmt.Fprintf(b, "\treturn { key: %s%s };\n}\n\n", jsString(key), valuesExpr())
	case FuncUnknown:
		fmt.Fprintf(b, "/** 不認得的值與 UNSPECIFIED 一律顯示的文字。 */\nexport function %s(): UiMessage {\n\treturn { key: \"common.unknown\" };\n}\n\n", name)
	case FuncError:
		b.WriteString("const COMMON_ERROR: Readonly<Partial<Record<Code, UiTextKey>>> = {\n")
		for _, c := range f.Cases {
			fmt.Fprintf(b, "\t[Code.%s]: %s,\n", connectCodeName(c.Value), jsString(c.Key.Raw))
		}
		b.WriteString("};\n\n")
		b.WriteString("/** Connect code 的預設文字；Canceled 不顯示，回 undefined。 */\nexport function commonError(code: Code): UiMessage | undefined {\n\tconst key = COMMON_ERROR[code];\n\treturn key === undefined ? undefined : { key };\n}\n\n")
	}
}

// tsCompile 把訊息編成 zh-TW 格式化函式的運算式（values 的每個值已是字串或數字，名詞參數已解成文字）。
func tsCompile(nodes []icu.Node, ps map[string]Param) string {
	if len(nodes) == 0 {
		return `""`
	}
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		switch n := n.(type) {
		case icu.Text:
			parts = append(parts, jsString(n.Value))
		case icu.Arg:
			parts = append(parts, "String(v."+n.Name+")")
		case icu.Select:
			var b strings.Builder
			fmt.Fprintf(&b, "(() => {\nswitch (v.%s) {\n", n.Name)
			var other string
			for _, br := range n.Branches {
				body := tsCompile(br.Body, ps)
				if br.Selector == "other" {
					other = body
					continue
				}
				fmt.Fprintf(&b, "case %s:\nreturn %s;\n", jsString(br.Selector), body)
			}
			fmt.Fprintf(&b, "default:\nreturn %s;\n}\n})()", other)
			parts = append(parts, b.String())
		case icu.Plural:
			// zh-TW 的 plural 只有 other（icu.Categories）。
			for _, br := range n.Branches {
				if br.Selector == "other" {
					parts = append(parts, tsCompile(br.Body, ps))
				}
			}
		}
	}
	return strings.Join(parts, " + ")
}

// EmitTSZhTW 生成 zh-tw.gen.ts：admin 用的 zh-TW 編譯版（含 staff）。
func (m *Model) EmitTSZhTW() []byte {
	var enums []*EnumInfo
	for _, f := range m.Funcs {
		if f.Kind == FuncVariant && f.HasStaff {
			enums = append(enums, f.Enum)
			for _, p := range f.Params {
				if p.Enum != nil {
					enums = append(enums, p.Enum)
				}
			}
		}
	}
	enums = dedupEnums(enums)
	imp := newTSImports(enums)
	var b strings.Builder
	b.WriteString(tsHeader)
	b.WriteString("import type { DefaultErrorText, UiMessage, UiTextKey } from \"./keys.gen\";\n")
	var nameTables []string
	for _, f := range m.Funcs {
		if f.Kind == FuncVariant && f.HasStaff {
			for _, p := range f.Params {
				if p.Kind == ParamEnum {
					nameTables = append(nameTables, constName(p.Enum.Path)+"_NAME")
				}
			}
		}
	}
	nameTables = append(nameTables, "commonError", "commonUnknown")
	sort.Strings(nameTables)
	nameTables = dedupStrings(nameTables)
	fmt.Fprintf(&b, "import { %s } from \"./keys.gen\";\n", strings.Join(nameTables, ", "))
	imp.write(&b)
	b.WriteString("\ntype Values = Readonly<Record<string, string | number>>;\ntype Formatter = (v: Values) => string;\n\n")
	emit := func(aud Audience, name string) {
		fmt.Fprintf(&b, "const %s: Readonly<Partial<Record<UiTextKey, Formatter>>> = {\n", name)
		for _, f := range m.Funcs {
			ps := map[string]Param{}
			for _, p := range f.Params {
				ps[p.Name] = p
			}
			for _, c := range f.Cases {
				var msg *Msg
				if aud == AudStaff {
					if f.HasStaff {
						msg = c.Msgs[AudStaff][staffLocale]
					}
				} else if !f.omitted(AudDefault, c.Value) {
					msg = c.Msgs[AudDefault][m.Config.SourceLocale]
				}
				if msg == nil {
					continue
				}
				expr := tsCompile(msg.AST.Nodes, ps)
				arg := "()"
				if strings.Contains(expr, "v.") {
					arg = "(v)"
				}
				fmt.Fprintf(&b, "\t%s: %s => %s,\n", jsString(c.Key.Raw), arg, expr)
			}
		}
		b.WriteString("};\n\n")
	}
	emit(AudDefault, "DEFAULT")
	emit(AudStaff, "STAFF")
	b.WriteString(`/**
 * 後台員工受眾的格式化（admin RPC 固定 zh-TW，ADR D1）：staff 有的 key 用 staff，沒有的用預設受眾；
 * 名詞參數也取 staff 優先。DEFAULT 只放預設受眾有文字的 key，staff 獨有的 key 在 STAFF。
 */
export function formatStaff(m: UiMessage): string {
	const values: Record<string, string | number> = {};
	for (const [name, value] of Object.entries(m.values ?? {})) {
		values[name] = typeof value === "object" ? formatStaff(value) : value;
	}
	const format = STAFF[m.key] ?? DEFAULT[m.key];
	if (format === undefined) {
		throw new Error("uitext: no zh-TW text for " + m.key);
	}
	return format(values);
}

/** Connect code 的預設文字（zh-TW）；Canceled 由呼叫端自己處理。 */
export const defaultErrorTextZhTW: DefaultErrorText = (code) =>
	formatStaff(commonError(code) ?? commonUnknown());

`)
	for _, f := range m.Funcs {
		if f.Kind == FuncVariant && f.HasStaff {
			m.tsFunc(&b, f, imp, AudStaff)
		}
	}
	return []byte(b.String())
}

func dedupEnums(in []*EnumInfo) []*EnumInfo {
	seen := map[string]bool{}
	var out []*EnumInfo
	for _, e := range in {
		if !seen[string(e.Desc.FullName())] {
			seen[string(e.Desc.FullName())] = true
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Desc.FullName() < out[j].Desc.FullName() })
	return out
}

func dedupStrings(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// EmitMessages 生成 next-intl 的巢狀 messages（預設受眾）。非 source 語系已與來源語系合併。
func (m *Model) EmitMessages(loc string) ([]byte, error) {
	root := map[string]any{}
	for _, f := range m.Funcs {
		for _, c := range f.Cases {
			msg := f.Effective(m, c, AudDefault, loc)
			if msg == nil {
				continue
			}
			parts := strings.Split(c.Key.Raw, ".")
			node := root
			for _, p := range parts[:len(parts)-1] {
				next, ok := node[p]
				if !ok {
					next = map[string]any{}
					node[p] = next
				}
				child, isMap := next.(map[string]any)
				if !isMap {
					return nil, fmt.Errorf("key %s 的前綴 %s 同時是訊息與節點", c.Key.Raw, p)
				}
				node = child
			}
			last := parts[len(parts)-1]
			if _, dup := node[last]; dup {
				return nil, fmt.Errorf("key %s 在同一個節點重複或同時是訊息與節點", c.Key.Raw)
			}
			node[last] = msg.Text
		}
	}
	return marshalJSON(root)
}
