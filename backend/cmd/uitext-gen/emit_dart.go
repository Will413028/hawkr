package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext/icu"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func dartString(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '$':
			b.WriteString(`\$`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// dartLocale 是 UiLocale 的成員名稱：zh-TW → zhTW，en → en。
func dartLocale(loc string) string {
	parts := strings.Split(loc, "-")
	out := strings.ToLower(parts[0])
	for _, p := range parts[1:] {
		out += upperFirst(p)
	}
	return out
}

type dartGen struct {
	needIntl bool
}

func (g *dartGen) typeName(e *EnumInfo) string {
	return dartAlias(e) + "." + enumTypeName(e)
}

func (g *dartGen) paramType(p Param) string {
	switch p.Kind { //nolint:exhaustive // 其餘種類走預設分支
	case ParamInteger:
		return "int"
	case ParamEnum, ParamNoun:
		return g.typeName(p.Enum)
	}
	return "String"
}

func (g *dartGen) compile(nodes []icu.Node, loc string, ps map[string]Param) string {
	if len(nodes) == 0 {
		return "''"
	}
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		switch n := n.(type) {
		case icu.Text:
			parts = append(parts, dartString(n.Value))
		case icu.Arg:
			p := ps[n.Name]
			switch p.Kind { //nolint:exhaustive // 其餘種類走預設分支
			case ParamInteger:
				parts = append(parts, n.Name+".toString()")
			case ParamNoun:
				parts = append(parts, n.Name+"Text")
			default:
				parts = append(parts, n.Name)
			}
		case icu.Select:
			p := ps[n.Name]
			var b strings.Builder
			fmt.Fprintf(&b, "(switch (%s.value) {", n.Name)
			var other string
			for _, br := range n.Branches {
				body := g.compile(br.Body, loc, ps)
				if br.Selector == "other" {
					other = body
					continue
				}
				v := p.Enum.Desc.Values().ByName(protoreflect.Name(br.Selector))
				fmt.Fprintf(&b, "%d => %s,", v.Number(), body)
			}
			fmt.Fprintf(&b, "_ => %s,})", other)
			parts = append(parts, b.String())
		case icu.Plural:
			var other string
			var named []string
			for _, br := range n.Branches {
				body := g.compile(br.Body, loc, ps)
				if br.Selector == "other" {
					other = body
				} else {
					named = append(named, br.Selector+": "+body)
				}
			}
			if len(named) == 0 {
				parts = append(parts, other)
				continue
			}
			g.needIntl = true
			parts = append(parts, fmt.Sprintf("Intl.pluralLogic(%s, %s, other: %s, locale: %s)", n.Name, strings.Join(named, ", "), other, dartString(loc)))
		}
	}
	return strings.Join(parts, " + ")
}

// dartCase 是一個 case 在各語系的運算式，相同的語系合併成一個 pattern。
type dartArm struct {
	locs []string
	expr string
}

func (m *Model) dartArms(g *dartGen, f *Func, c *Case, ps map[string]Param) []dartArm {
	var arms []dartArm
	index := map[string]int{}
	for _, loc := range m.Config.Locales {
		msg := f.Effective(m, c, AudDefault, loc)
		if msg == nil {
			continue
		}
		expr := g.compile(msg.AST.Nodes, loc, ps)
		if i, ok := index[expr]; ok {
			arms[i].locs = append(arms[i].locs, loc)
			continue
		}
		index[expr] = len(arms)
		arms = append(arms, dartArm{locs: []string{loc}, expr: expr})
	}
	return arms
}

func localePattern(locs []string) string {
	parts := make([]string, len(locs))
	for i, l := range locs {
		parts[i] = "UiLocale." + dartLocale(l)
	}
	return strings.Join(parts, " || ")
}

func dartParamNamed(f *Func, g *dartGen) string {
	if len(f.Params) == 0 {
		return ""
	}
	var parts []string
	for _, p := range f.Params {
		parts = append(parts, "required "+g.paramType(p)+" "+p.Name)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func dartParamPass(f *Func) string {
	var parts []string
	for _, p := range f.Params {
		parts = append(parts, p.Name)
	}
	return strings.Join(parts, ", ")
}

// EmitDart 生成 ui_text.g.dart。
func (m *Model) EmitDart() []byte {
	g := &dartGen{}
	enums := m.usedEnums()
	var body strings.Builder
	// 名詞、變體依列舉分組成 extension；訊息句、通用文字是頂層函式。
	byEnum := map[string][]*Func{}
	var enumOrder []string
	var tops []*Func
	for _, f := range m.Funcs {
		if f.Enum != nil {
			if _, ok := byEnum[f.Enum.Path]; !ok {
				enumOrder = append(enumOrder, f.Enum.Path)
			}
			byEnum[f.Enum.Path] = append(byEnum[f.Enum.Path], f)
		} else {
			tops = append(tops, f)
		}
	}
	sort.Strings(enumOrder)
	for _, path := range enumOrder {
		m.dartEnum(g, &body, byEnum[path])
	}
	for _, f := range tops {
		m.dartTop(g, &body, f)
	}
	var out strings.Builder
	out.WriteString("// dart format off\n")
	out.WriteString("// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。\n\n")
	out.WriteString("// ignore_for_file: unnecessary_parenthesis, unused_import, prefer_single_quotes, unnecessary_string_interpolations\n\n")
	out.WriteString("import 'package:connectrpc/connect.dart' show Code;\n")
	libs := map[string]*EnumInfo{}
	var libOrder []string
	for _, e := range enums {
		lib := dartLibrary(e)
		if _, ok := libs[lib]; !ok {
			libs[lib] = e
			libOrder = append(libOrder, lib)
		}
	}
	sort.Strings(libOrder)
	for _, lib := range libOrder {
		fmt.Fprintf(&out, "import '%s' as %s;\n", lib, dartAlias(libs[lib]))
	}
	if g.needIntl {
		out.WriteString("import 'package:intl/intl.dart' show Intl;\n")
	}
	out.WriteString("\nimport '../hawkr_ui_text.dart' show UiLocale;\n\n")
	out.WriteString(body.String())
	return []byte(out.String())
}

func (m *Model) dartEnum(g *dartGen, b *strings.Builder, funcs []*Func) {
	e := funcs[0].Enum
	typ := g.typeName(e)
	ext := pascal(e.Path) + "UiText"
	// 沒有 staff 的 Dart 只生成預設受眾。
	var methods strings.Builder
	for _, f := range funcs {
		helper := "_" + lowerFirst(pascal(f.ID))
		ps := map[string]Param{}
		for _, p := range f.Params {
			ps[p.Name] = p
		}
		// helper
		var sig []string
		sig = append(sig, "UiLocale l", "int value")
		for _, p := range f.Params {
			sig = append(sig, g.paramType(p)+" "+p.Name)
		}
		fmt.Fprintf(b, "String? %s(%s) {\n", helper, strings.Join(sig, ", "))
		for _, p := range f.Params {
			if p.Kind == ParamNoun {
				fmt.Fprintf(b, "final %sText = _%s(l, %s.value);\nif (%sText == null) return null;\n", p.Name, lowerFirst(pascal(p.Enum.Path)), p.Name, p.Name)
			}
		}
		b.WriteString("return switch ((l, value)) {\n")
		for _, c := range f.Cases {
			v := e.Desc.Values().ByName(protoreflect.Name(c.Value))
			for _, arm := range m.dartArms(g, f, c, ps) {
				fmt.Fprintf(b, "(%s, %d) => %s,\n", localePattern(arm.locs), v.Number(), arm.expr)
			}
		}
		b.WriteString("_ => null,\n};\n}\n\n")
		// method
		named := dartParamNamed(f, g)
		args := "l, value"
		if p := dartParamPass(f); p != "" {
			args += ", " + p
		}
		lparams := "UiLocale l"
		if named != "" {
			lparams += ", " + named
		}
		if f.Kind == FuncNoun {
			fmt.Fprintf(&methods, "  /// %s 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。\n  String? uiText(%s) => %s(%s);\n\n", f.ID, lparams, helper, args)
		} else {
			fmt.Fprintf(&methods, "  /// %s；沒有文字的值回 null。\n  String? %s(%s) => %s(%s);\n\n", f.ID, lowerCamel(f.Variant), lparams, helper, args)
		}
	}
	fmt.Fprintf(b, "extension %s on %s {\n%s}\n\n", ext, typ, strings.TrimRight(methods.String(), "\n")+"\n")
}

func (m *Model) dartTop(g *dartGen, b *strings.Builder, f *Func) {
	name := lowerFirst(pascal(f.ID))
	switch f.Kind { //nolint:exhaustive // 其餘種類走預設分支
	case FuncError:
		b.WriteString("/// Connect code 的預設文字；canceled 不顯示，回 null。\nString? commonErrorText(Code code, UiLocale l) => switch ((l, code)) {\n")
		for _, c := range f.Cases {
			for _, arm := range m.dartArms(g, f, c, nil) {
				fmt.Fprintf(b, "(%s, Code.%s) => %s,\n", localePattern(arm.locs), lowerFirst(connectCodeName(c.Value)), arm.expr)
			}
		}
		b.WriteString("_ => null,\n};\n\n")
	case FuncUnknown:
		b.WriteString("/// 不認得的值與 UNSPECIFIED 一律顯示的文字。\nString commonUnknown(UiLocale l) => switch (l) {\n")
		for _, arm := range m.dartArms(g, f, f.Cases[0], nil) {
			fmt.Fprintf(b, "%s => %s,\n", localePattern(arm.locs), arm.expr)
		}
		b.WriteString("};\n\n")
	case FuncSentence:
		ps := map[string]Param{}
		hasNoun := false
		for _, p := range f.Params {
			ps[p.Name] = p
			if p.Kind == ParamNoun {
				hasNoun = true
			}
		}
		ret := "String"
		if hasNoun {
			ret = "String?"
		}
		lparams := "UiLocale l"
		if n := dartParamNamed(f, g); n != "" {
			lparams += ", " + n
		}
		fmt.Fprintf(b, "/// %s\n%s %s(%s) {\n", f.ID, ret, name, lparams)
		for _, p := range f.Params {
			if p.Kind == ParamNoun {
				fmt.Fprintf(b, "final %sText = _%s(l, %s.value);\nif (%sText == null) return null;\n", p.Name, lowerFirst(pascal(p.Enum.Path)), p.Name, p.Name)
			}
		}
		b.WriteString("return switch (l) {\n")
		for _, arm := range m.dartArms(g, f, f.Cases[0], ps) {
			fmt.Fprintf(b, "%s => %s,\n", localePattern(arm.locs), arm.expr)
		}
		b.WriteString("};\n}\n\n")
	}
}
