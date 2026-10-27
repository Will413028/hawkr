package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext/icu"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Audience 是文案的受眾。
type Audience string

const (
	// AudDefault 是賣家與買家面。
	AudDefault Audience = "default"
	// AudStaff 是後台員工（只有 zh-TW）。
	AudStaff Audience = "staff"
)

const staffLocale = "zh-TW"

// ParamKind 是參數的型別。
type ParamKind int

const (
	// ParamString 是已格式化的字串。
	ParamString ParamKind = iota + 1
	// ParamInteger 是整數。
	ParamInteger
	// ParamEnum 是只能用在 select 的列舉值。
	ParamEnum
	// ParamNoun 是名詞參數：以同語系、同受眾的目錄名詞填入。
	ParamNoun
)

// Param 是函式的參數。
type Param struct {
	Name string
	Kind ParamKind
	Enum *EnumInfo
}

// Msg 是一則解析後的訊息。
type Msg struct {
	Text string
	AST  *icu.Message
}

// Case 是一個 key（函式的一個值，或單一訊息）。
type Case struct {
	Key   *Key
	Value string
	// Msgs 是明確寫在來源檔的訊息：受眾 → 語系 → 訊息。
	Msgs map[Audience]map[string]*Msg
	// Params 是這則訊息宣告的參數。
	Params map[string]Param
}

// FuncKind 是函式的種類。
type FuncKind int

const (
	// FuncNoun 是 enum 值的名詞。
	FuncNoun FuncKind = iota + 1
	// FuncVariant 是依值的句子。
	FuncVariant
	// FuncSentence 是訊息句。
	FuncSentence
	// FuncUnknown 是 common.unknown。
	FuncUnknown
	// FuncError 是 common.error.*。
	FuncError
)

// Func 是三端各生成一個 API 函式的單位。
type Func struct {
	ID      string
	Kind    FuncKind
	Enum    *EnumInfo
	Variant string
	// Params 是各 case 參數的聯集，依名稱排序。
	Params []Param
	Cases  []*Case
	// Omit 是依受眾「沒有文字」的值。
	Omit map[Audience]map[string]bool
	// HasStaff 表示這個函式有 staff 受眾的訊息。
	HasStaff bool
}

// Model 是通過驗證的目錄。
type Model struct {
	Schema *Schema
	Config *Config
	Funcs  []*Func
	// Catalog 是來源檔（-write 要寫回的正規形式）。
	Catalog map[string]*File
	Modules []*ModuleModel
	// Coverage 是非 required 語系的覆蓋率報告。
	Coverage []string
}

// Inputs 是生成器的輸入。
type Inputs struct {
	Image                   *descriptorpb.FileDescriptorSet
	Config                  *Config
	Catalog                 map[string]*File
	Modules                 []ModuleInput
	EnforceUnavailableShape bool
}

// ModuleInput 是一個模組的 text/ 目錄。
type ModuleInput struct {
	// Dir 是相對於 repo 根目錄的目錄，例如 backend/internal/modules/seller/internal/text。
	Dir   string
	Files map[string]*File
}

var paramName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// reservedParams 是各目標語言裡會和生成碼的區域名稱或關鍵字相撞的參數名稱。
var reservedParams = map[string]bool{
	"v": true, "t": true, "l": true, "ok": true, "a": true, "values": true, "locale": true, "key": true, "code": true,
	"value": true, "default": true, "func": true, "var": true, "type": true, "switch": true, "case": true,
	"return": true, "if": true, "else": true, "for": true, "range": true, "map": true, "go": true, "const": true,
	"string": true, "int": true, "bool": true, "new": true, "class": true, "enum": true, "final": true,
	"in": true, "is": true, "as": true, "this": true, "null": true, "true": true, "false": true, "void": true,
	"function": true, "let": true, "import": true, "export": true, "package": true, "interface": true,
	"struct": true, "select": true, "chan": true, "defer": true, "break": true, "continue": true,
	"fallthrough": true, "goto": true, "with": true, "do": true, "while": true, "try": true, "catch": true,
	"throw": true, "super": true, "static": true, "extends": true, "implements": true, "late": true, "required": true,
}

type errList []string

func (e *errList) addf(format string, args ...any) {
	*e = append(*e, fmt.Sprintf(format, args...))
}

func (e errList) err() error {
	if len(e) == 0 {
		return nil
	}
	sorted := append([]string(nil), e...)
	sort.Strings(sorted)
	return errors.New(strings.Join(sorted, "\n"))
}

// Build 驗證輸入（§2.2）並建立模型。任何一條失敗都回錯誤，訊息逐行點名 key。
func Build(in Inputs) (*Model, error) {
	var errs errList
	if err := validateConfig(in.Config); err != nil {
		return nil, err
	}
	schema, err := LoadSchema(in.Image)
	if err != nil {
		return nil, err
	}
	m := &Model{Schema: schema, Config: in.Config, Catalog: in.Catalog}
	if in.EnforceUnavailableShape {
		for _, e := range schema.unavailableShapeErrors {
			errs.addf("D-L2 形狀：%s", e)
		}
	}
	for _, loc := range in.Config.Locales {
		if in.Catalog[loc] == nil {
			errs.addf("catalog 缺少語系 %s 的來源檔", loc)
		}
	}
	if len(errs) > 0 {
		return nil, errs.err()
	}
	b := &builder{m: m, errs: &errs, funcs: map[string]*Func{}, broken: map[string]bool{}}
	b.collect(in)
	b.checkFuncs()
	m.Funcs = b.sorted()
	b.completeness(in)
	b.coverage(in)
	for _, mi := range in.Modules {
		mm := buildModule(in.Config, mi, &errs)
		if mm != nil {
			m.Modules = append(m.Modules, mm)
		}
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	if err := m.SyncDescriptions(); err != nil {
		return nil, err
	}
	return m, nil
}

func validateConfig(c *Config) error {
	if c.SourceLocale == "" || len(c.Locales) == 0 || len(c.Required) == 0 {
		return errors.New("config.json：sourceLocale、locales、required 都要有值")
	}
	has := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	if !has(c.Locales, c.SourceLocale) {
		return fmt.Errorf("config.json：sourceLocale %s 不在 locales", c.SourceLocale)
	}
	if !has(c.Required, c.SourceLocale) {
		return fmt.Errorf("config.json：required 要包含 sourceLocale %s", c.SourceLocale)
	}
	for _, l := range c.Required {
		if !has(c.Locales, l) {
			return fmt.Errorf("config.json：required 的 %s 不在 locales", l)
		}
	}
	for _, l := range c.Locales {
		if _, err := icu.Categories(l); err != nil {
			return fmt.Errorf("config.json：%w", err)
		}
	}
	if c.SourceLocale != staffLocale {
		return fmt.Errorf("config.json：sourceLocale 要是 %s（staff 受眾固定 zh-TW）", staffLocale)
	}
	return nil
}

type builder struct {
	m     *Model
	errs  *errList
	funcs map[string]*Func
	// decls 是來源語系宣告的變體：FuncID → 宣告。
	decls map[string]VariantDecl
	// broken 是已經回報過錯誤的 key（語系|受眾|key）；完整性檢查不再為它們重複回報「缺」。
	broken map[string]bool
}

func (b *builder) funcFor(k *Key) *Func {
	f := b.funcs[k.FuncID]
	if f != nil {
		return f
	}
	f = &Func{ID: k.FuncID, Enum: k.Enum, Variant: k.Variant, Omit: map[Audience]map[string]bool{}}
	switch k.Kind {
	case KeyNoun:
		f.Kind = FuncNoun
	case KeyVariant:
		f.Kind = FuncVariant
	case KeySentence:
		f.Kind = FuncSentence
	case KeyUnknown:
		f.Kind = FuncUnknown
	case KeyError:
		f.Kind = FuncError
	}
	b.funcs[k.FuncID] = f
	return f
}

func (f *Func) caseFor(k *Key) *Case {
	for _, c := range f.Cases {
		if c.Key.Raw == k.Raw {
			return c
		}
	}
	c := &Case{Key: k, Value: k.Value, Msgs: map[Audience]map[string]*Msg{}, Params: map[string]Param{}}
	f.Cases = append(f.Cases, c)
	return c
}

func (c *Case) set(a Audience, loc string, msg *Msg) {
	if c.Msgs[a] == nil {
		c.Msgs[a] = map[string]*Msg{}
	}
	c.Msgs[a][loc] = msg
}

func (b *builder) parseKey(where, raw string) *Key {
	k, err := b.m.Schema.ParseKey(raw)
	if err != nil {
		b.errs.addf("%s %s：%v", where, raw, err)
		return nil
	}
	if k.Enum != nil && b.m.Schema.serverOwned[k.Enum.Desc.FullName()] {
		b.errs.addf("%s %s：%s 的文字由伺服器擁有（§2.3），不能放進共用目錄", where, raw, k.Enum.Path)
		return nil
	}
	return k
}

// collect 讀所有語系的 key，建立函式與 case，並解析、驗證訊息。
func (b *builder) collect(in Inputs) {
	src := in.Catalog[in.Config.SourceLocale]
	b.decls = map[string]VariantDecl{}
	// 變體宣告。
	for raw, d := range src.Variants {
		k := b.parseVariantDecl(raw)
		if k == nil {
			continue
		}
		b.decls[k.FuncID] = d
	}
	for _, loc := range in.Config.Locales {
		f := in.Catalog[loc]
		isSource := loc == in.Config.SourceLocale
		if !isSource {
			if len(f.Staff) > 0 {
				b.errs.addf("%s.json：staff 只有 %s（ADR：staff 只有 zh-TW）", loc, in.Config.SourceLocale)
			}
			if len(f.Variants) > 0 {
				b.errs.addf("%s.json：variants 只在來源語系 %s 宣告", loc, in.Config.SourceLocale)
			}
		}
		for raw, e := range f.Default {
			b.addEntry(in, loc, AudDefault, raw, e)
		}
		if isSource {
			for raw, e := range f.Staff {
				b.addEntry(in, loc, AudStaff, raw, e)
			}
		}
	}
}

func (b *builder) parseVariantDecl(raw string) *Key {
	// 變體宣告的 key 是 <型別路徑>.<variant>：借用 ParseKey 的列舉解析，補一個假的值名。
	i := strings.LastIndex(raw, ".")
	if i < 0 {
		b.errs.addf("variants %s：要是 <型別路徑>.<variant>", raw)
		return nil
	}
	e, err := b.m.Schema.EnumByPath(raw[:i])
	if err != nil {
		b.errs.addf("variants %s：%v", raw, err)
		return nil
	}
	if b.m.Schema.serverOwned[e.Desc.FullName()] {
		b.errs.addf("variants %s：%s 的文字由伺服器擁有（§2.3）", raw, e.Path)
		return nil
	}
	name := raw[i+1:]
	if !lowerSnake.MatchString(name) {
		b.errs.addf("variants %s：變體名稱 %q 要是 lower_snake", raw, name)
		return nil
	}
	return &Key{Raw: raw, Kind: KeyVariant, Enum: e, Variant: name, FuncID: raw}
}

func (b *builder) addEntry(in Inputs, loc string, aud Audience, raw string, e Entry) {
	where := loc + "." + string(aud)
	isSource := loc == in.Config.SourceLocale
	k := b.parseKey(where, raw)
	if k == nil {
		return
	}
	if strings.TrimSpace(e.Message) == "" {
		b.errs.addf("%s %s：message 是空的", where, raw)
		return
	}
	if !isSource {
		if len(e.Params) > 0 || e.Description != "" || e.DescriptionOverride != "" || e.Note != "" {
			b.errs.addf("%s %s：非來源語系只放 message（params、description、note 在 %s 來源檔）", where, raw, in.Config.SourceLocale)
		}
		if _, ok := in.Catalog[in.Config.SourceLocale].Default[raw]; !ok {
			b.errs.addf("%s %s：%s 沒有這個 key（多餘的 key）", where, raw, in.Config.SourceLocale)
			return
		}
	}
	ast, err := icu.Parse(e.Message)
	if err != nil {
		b.errs.addf("%s %s：訊息不合子集：%v", where, raw, err)
		b.broken[loc+"|"+string(aud)+"|"+raw] = true
		return
	}
	if k.Kind == KeyVariant {
		if _, ok := b.decls[k.FuncID]; !ok {
			b.errs.addf("%s %s：變體 %s 沒有在 variants 宣告", where, raw, k.FuncID)
			return
		}
	}
	_, hasDefault := in.Catalog[in.Config.SourceLocale].Default[raw]
	if aud == AudStaff && hasDefault && len(e.Params) > 0 {
		b.errs.addf("%s %s：預設受眾已宣告 params，staff 不重複宣告", where, raw)
	}
	f := b.funcFor(k)
	c := f.caseFor(k)
	c.set(aud, loc, &Msg{Text: e.Message, AST: ast})
	if aud == AudStaff {
		f.HasStaff = true
	}
	if isSource && (aud == AudDefault || !hasDefault) {
		b.declareParams(c, where, raw, e)
	}
	b.recordUses(in, c, loc, aud, where, raw, ast)
}

func (b *builder) declareParams(c *Case, where, raw string, e Entry) {
	for name, spec := range e.Params {
		if !paramName.MatchString(name) || reservedParams[name] {
			b.errs.addf("%s %s：參數名稱 %q 不合法（lower camel，且不能是保留字）", where, raw, name)
			continue
		}
		set := 0
		for _, s := range []string{spec.Enum, spec.Noun, spec.Type} {
			if s != "" {
				set++
			}
		}
		if set != 1 {
			b.errs.addf("%s %s：參數 %s 要剛好指定 enum、noun、type 其中一個", where, raw, name)
			continue
		}
		p := Param{Name: name}
		switch {
		case spec.Enum != "":
			en, err := b.m.Schema.EnumByPath(spec.Enum)
			if err != nil {
				b.errs.addf("%s %s：參數 %s：%v", where, raw, name, err)
				continue
			}
			p.Kind, p.Enum = ParamEnum, en
		case spec.Noun != "":
			en, err := b.m.Schema.EnumByPath(spec.Noun)
			if err != nil {
				b.errs.addf("%s %s：參數 %s：%v", where, raw, name, err)
				continue
			}
			p.Kind, p.Enum = ParamNoun, en
		case spec.Type == "string":
			p.Kind = ParamString
		case spec.Type == "integer":
			p.Kind = ParamInteger
		default:
			b.errs.addf("%s %s：參數 %s 的 type %q 只能是 string 或 integer", where, raw, name, spec.Type)
			continue
		}
		c.Params[name] = p
	}
}

// recordUses 檢查訊息用到的參數與宣告一致（來源語系）或與來源語系相同（其他語系）、型別與用法相符、plural 分支合乎語系。
func (b *builder) recordUses(in Inputs, c *Case, loc string, aud Audience, where, raw string, ast *icu.Message) {
	uses, conflict := ast.Uses()
	if conflict != "" {
		b.errs.addf("%s %s：參數 %s 同時用作不同的型別", where, raw, conflict)
	}
	used := map[string]bool{}
	for _, u := range uses {
		used[u.Name] = true
		if u.Kind == icu.UsePlural {
			cats, _ := icu.Categories(loc)
			for _, s := range u.Selectors {
				if !contains(cats, s) {
					b.errs.addf("%s %s：參數 %s 的 plural 分支 %q 不屬於 %s（CLDR：%s）", where, raw, u.Name, s, loc, strings.Join(cats, "、"))
				}
			}
		}
	}
	// 以來源語系預設受眾的宣告為準；staff 與其他語系的參數集合必須相同。
	srcMsg, ok := in.Catalog[in.Config.SourceLocale].Default[raw]
	if !ok {
		srcMsg = in.Catalog[in.Config.SourceLocale].Staff[raw]
	}
	declared := map[string]bool{}
	for name := range srcMsg.Params {
		declared[name] = true
	}
	for name := range used {
		if !declared[name] {
			b.errs.addf("%s %s：訊息用了沒有宣告的參數 %s", where, raw, name)
		}
	}
	for name := range declared {
		if !used[name] {
			b.errs.addf("%s %s：宣告了參數 %s 但訊息沒有用到（每個語系的參數集合必須與來源相同）", where, raw, name)
		}
	}
	// 型別與用法。
	for _, u := range uses {
		p, ok := c.Params[u.Name]
		if !ok {
			continue
		}
		switch p.Kind {
		case ParamEnum:
			if u.Kind != icu.UseSelect {
				b.errs.addf("%s %s：enum 參數 %s 只能用在 select", where, raw, u.Name)
				continue
			}
			for _, s := range u.Selectors {
				if s == "other" {
					continue
				}
				v := p.Enum.Desc.Values().ByName(protoreflect.Name(s))
				if v == nil || v.Number() == 0 {
					b.errs.addf("%s %s：select 的分支 %q 不是列舉 %s 的值", where, raw, s, p.Enum.Path)
				}
			}
		case ParamNoun:
			if u.Kind != icu.UseArg {
				b.errs.addf("%s %s：名詞參數 %s 只能寫成 {%s}", where, raw, u.Name, u.Name)
			}
		case ParamString:
			if u.Kind != icu.UseArg {
				b.errs.addf("%s %s：string 參數 %s 只能寫成 {%s}", where, raw, u.Name, u.Name)
			}
		case ParamInteger:
			if u.Kind == icu.UseSelect {
				b.errs.addf("%s %s：integer 參數 %s 不能用在 select", where, raw, u.Name)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// checkFuncs 檢查函式層級的一致性：參數聯集、名詞參數的目標、變體宣告與 omit。
func (b *builder) checkFuncs() {
	for _, f := range b.funcs {
		union := map[string]Param{}
		for _, c := range f.Cases {
			for name, p := range c.Params {
				if prev, ok := union[name]; ok {
					if prev.Kind != p.Kind || prev.Enum != p.Enum {
						b.errs.addf("%s：參數 %s 在 %s 與其他 key 的型別不同", f.ID, name, c.Key.Raw)
					}
					continue
				}
				union[name] = p
			}
		}
		sort.Slice(f.Cases, func(i, j int) bool { return caseLess(f, f.Cases[i], f.Cases[j]) })
		// 參數依在來源訊息裡第一次出現的順序排列（簽章讀起來和句子一致）。
		for _, c := range f.Cases {
			for _, msg := range []*Msg{c.Msgs[AudDefault][b.m.Config.SourceLocale], c.Msgs[AudStaff][staffLocale]} {
				if msg == nil {
					continue
				}
				uses, _ := msg.AST.Uses()
				for _, u := range uses {
					if p, ok := union[u.Name]; ok {
						f.Params = append(f.Params, p)
						delete(union, u.Name)
					}
				}
			}
		}
		rest := make([]Param, 0, len(union))
		for _, p := range union {
			rest = append(rest, p)
		}
		sort.Slice(rest, func(i, j int) bool { return rest[i].Name < rest[j].Name })
		f.Params = append(f.Params, rest...)
		if f.Kind == FuncVariant {
			b.applyOmit(f)
		}
	}
	for id := range b.decls {
		if b.funcs[id] == nil {
			b.errs.addf("variants %s：宣告了變體但沒有任何 key", id)
		}
	}
	// 名詞參數的目標必須是目錄裡沒有參數的名詞。
	for _, f := range b.funcs {
		for _, p := range f.Params {
			if p.Kind != ParamNoun {
				continue
			}
			nf := b.funcs[p.Enum.Path]
			if nf == nil || nf.Kind != FuncNoun {
				b.errs.addf("%s：名詞參數 %s 指向 %s，它沒有名詞在目錄裡", f.ID, p.Name, p.Enum.Path)
			} else if len(nf.Params) > 0 {
				b.errs.addf("%s：名詞參數 %s 指向的 %s 本身有參數，不支援", f.ID, p.Name, p.Enum.Path)
			}
		}
	}
	// omit 宣告的受眾與值。
	for id, d := range b.decls {
		for aud := range d.Omit {
			a := Audience(aud)
			if a != AudDefault && a != AudStaff {
				b.errs.addf("variants %s：omit 的受眾 %q 只能是 default 或 staff", id, aud)
				continue
			}
			f := b.funcs[id]
			if f == nil {
				continue
			}
			if a == AudStaff && !f.HasStaff {
				b.errs.addf("variants %s：這個變體沒有 staff 的 key，omit.staff 沒有意義", id)
			}
		}
	}
}

func caseLess(f *Func, a, b *Case) bool {
	if f.Enum != nil {
		va := f.Enum.Desc.Values().ByName(protoreflect.Name(a.Value))
		vb := f.Enum.Desc.Values().ByName(protoreflect.Name(b.Value))
		if va != nil && vb != nil {
			return va.Number() < vb.Number()
		}
	}
	return a.Key.Raw < b.Key.Raw
}

func (b *builder) applyOmit(f *Func) {
	d, ok := b.decls[f.ID]
	if !ok {
		return
	}
	for aud, values := range d.Omit {
		a := Audience(aud)
		if a != AudDefault && a != AudStaff {
			continue
		}
		set := map[string]bool{}
		for _, name := range values {
			v := f.Enum.Desc.Values().ByName(protoreflect.Name(name))
			if v == nil || v.Number() == 0 {
				b.errs.addf("variants %s：omit.%s 的 %q 不是列舉 %s 的有效值", f.ID, aud, name, f.Enum.Path)
				continue
			}
			set[name] = true
		}
		f.Omit[a] = set
	}
}

func (b *builder) sorted() []*Func {
	var out []*Func
	for _, f := range b.funcs {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// omitted 回這個受眾是否把 value 列為「沒有文字」。
func (f *Func) omitted(a Audience, value string) bool {
	return f.Omit[a][value]
}

// completeness 是 §2.2 第 3、4、7 條。
func (b *builder) completeness(in Inputs) {
	for _, loc := range in.Config.Required {
		if b.funcs["common.unknown"] == nil || b.funcs["common.unknown"].Cases[0].Msgs[AudDefault][loc] == nil {
			b.errs.addf("common.unknown：缺 %s 的文字", loc)
		}
	}
	errf := b.funcs["common.error"]
	for _, code := range connectCodes {
		key := "common.error." + code
		var c *Case
		if errf != nil {
			for _, x := range errf.Cases {
				if x.Key.Raw == key {
					c = x
				}
			}
		}
		for _, loc := range in.Config.Required {
			if c == nil || c.Msgs[AudDefault][loc] == nil {
				b.errs.addf("%s：缺 %s 的文字（common.error.* 剛好是 Connect 的 %d 個 code，CANCELED 除外）", key, loc, len(connectCodes))
			}
		}
	}
	for _, f := range b.funcs {
		switch f.Kind { //nolint:exhaustive // 其餘種類走預設分支
		case FuncNoun, FuncVariant:
		default:
			if f.Kind == FuncSentence {
				for _, c := range f.Cases {
					for _, loc := range in.Config.Required {
						if c.Msgs[AudDefault][loc] == nil {
							b.errs.addf("%s：缺 %s 的文字", c.Key.Raw, loc)
						}
					}
				}
			}
			continue
		}
		has := map[string]*Case{}
		for _, c := range f.Cases {
			has[c.Value] = c
		}
		for _, v := range f.Enum.Values {
			name := string(v.Name())
			c := has[name]
			for _, aud := range []Audience{AudDefault, AudStaff} {
				if aud == AudStaff && !f.HasStaff {
					continue
				}
				omit := f.omitted(aud, name)
				locs := in.Config.Required
				if aud == AudStaff {
					locs = []string{staffLocale}
				}
				for _, loc := range locs {
					var got *Msg
					if c != nil {
						got = c.Msgs[aud][loc]
					}
					switch {
					case got == nil && !omit && b.broken[loc+"|"+string(aud)+"|"+keyOf(f, name)]:
					case got == nil && !omit:
						b.errs.addf("%s：缺 [%s %s] 的文字（%s.%s）", keyOf(f, name), aud, loc, f.Enum.Path, name)
					case got != nil && omit:
						b.errs.addf("%s：omit.%s 列了 %s，卻仍有 %s 的文字", keyOf(f, name), aud, name, loc)
					}
				}
			}
		}
	}
}

func keyOf(f *Func, value string) string {
	return f.ID + "." + value
}

func (b *builder) coverage(in Inputs) {
	for _, loc := range in.Config.Locales {
		required := false
		for _, r := range in.Config.Required {
			if r == loc {
				required = true
			}
		}
		if required {
			continue
		}
		total, have := 0, 0
		for _, f := range b.funcs {
			for _, c := range f.Cases {
				if c.Msgs[AudDefault][in.Config.SourceLocale] == nil {
					continue
				}
				total++
				if c.Msgs[AudDefault][loc] != nil {
					have++
				}
			}
		}
		b.m.Coverage = append(b.m.Coverage, fmt.Sprintf("%s：%d/%d key 有翻譯（缺的以 %s 的文字補）", loc, have, total, in.Config.SourceLocale))
	}
}

// Effective 回受眾 a、語系 loc 在 case c 的有效訊息；沒有文字（omit）回 nil。
// 非 required 語系缺 key 時退回來源語系的文字（§2.4）；staff 沒有的 key 退回預設受眾。
func (f *Func) Effective(m *Model, c *Case, a Audience, loc string) *Msg {
	src := m.Config.SourceLocale
	if a == AudStaff {
		if !f.HasStaff {
			return f.Effective(m, c, AudDefault, src)
		}
		if f.omitted(AudStaff, c.Value) {
			return nil
		}
		return c.Msgs[AudStaff][staffLocale]
	}
	if f.omitted(AudDefault, c.Value) {
		return nil
	}
	if msg := c.Msgs[AudDefault][loc]; msg != nil {
		return msg
	}
	return c.Msgs[AudDefault][src]
}
