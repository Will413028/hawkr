package main

import (
	"errors"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// limitsOption 是欄位選項 hawkr.common.v1alpha1.limits 的全名（api/hawkr/common/v1alpha1/limits.proto）。
const limitsOption protoreflect.FullName = "hawkr.common.v1alpha1.limits"

// limitsGoTargets 是掛了 limits 的訊息（短 package.訊息名）到 Go 常數檔所在目錄（相對於 repo 根目錄）與 package 名。
// 掛了 limits 的訊息不在這張表裡就報錯：常數要放進擁有它的模組。
var limitsGoTargets = map[string]struct{ Dir, Package string }{
	"catalog.ProductContentLimits": {"backend/internal/modules/catalog/internal/model", "model"},
}

// LimitField 是一個掛了 limits 的欄位。
type LimitField struct {
	// Message 是 key 風格的訊息路徑，例如 catalog.ProductContentLimits。
	Message string
	// Name 是欄位名（lower_snake）。
	Name string
	// Items 是 max_items（repeated 欄位），Runes 是 max_runes（string 欄位）；恰好一個大於 0。
	Items, Runes int64
}

// Value 是欄位的上限值。
func (l *LimitField) Value() int64 {
	if l.Items > 0 {
		return l.Items
	}
	return l.Runes
}

// Path 是欄位的 key 風格路徑。
func (l *LimitField) Path() string { return l.Message + "." + l.Name }

// unit 是上限的單位：items 或 runes。
func (l *LimitField) unit() string {
	if l.Items > 0 {
		return "items"
	}
	return "runes"
}

// goName 是 Go 常數名：max_runes 生成 Max<欄位>Runes，max_items 生成 Max<欄位>。
func (l *LimitField) goName() string {
	if l.Items > 0 {
		return "Max" + pascal(l.Name)
	}
	return "Max" + pascal(l.Name) + "Runes"
}

// LimitFields 讀出 image 裡所有掛了 limits 的欄位，依訊息與欄位宣告順序排列。
// 讀法同 AcceptedFields：擴充欄位沒有編譯進來的型別，用 image 自己的描述重新解碼欄位選項。
func (s *Schema) LimitFields() ([]*LimitField, error) {
	d, err := s.files.FindDescriptorByName(limitsOption)
	if err != nil {
		return nil, fmt.Errorf("image 沒有 %s 的定義：生成的常數會是空的", limitsOption)
	}
	xd, ok := d.(protoreflect.ExtensionDescriptor)
	if !ok || xd.IsList() || xd.Message() == nil || xd.Message().FullName() != "hawkr.common.v1alpha1.Limits" {
		return nil, fmt.Errorf("%s 必須是單值 Limits 的 FieldOptions 擴充欄位", limitsOption)
	}
	maxItems := xd.Message().Fields().ByName("max_items")
	maxRunes := xd.Message().Fields().ByName("max_runes")
	if maxItems == nil || maxRunes == nil {
		return nil, errors.New("limits 的訊息 Limits 必須有 max_items 與 max_runes")
	}
	xt := dynamicpb.NewExtensionType(xd)
	resolver := &protoregistry.Types{}
	if err := resolver.RegisterExtension(xt); err != nil {
		return nil, err
	}
	var out []*LimitField
	var errs errList
	var walk func(fd protoreflect.FileDescriptor, ms protoreflect.MessageDescriptors, known bool)
	walk = func(fd protoreflect.FileDescriptor, ms protoreflect.MessageDescriptors, known bool) {
		for i := range ms.Len() {
			m := ms.Get(i)
			fs := m.Fields()
			for j := range fs.Len() {
				f := fs.Get(j)
				raw, ok := f.Options().(*descriptorpb.FieldOptions)
				if !ok || raw == nil {
					continue
				}
				b, err := proto.Marshal(raw)
				if err != nil {
					errs.addf("%s：%v", f.FullName(), err)
					continue
				}
				var opts descriptorpb.FieldOptions
				if err := (proto.UnmarshalOptions{Resolver: resolver}).Unmarshal(b, &opts); err != nil {
					errs.addf("%s：%v", f.FullName(), err)
					continue
				}
				if !opts.ProtoReflect().Has(xt.TypeDescriptor()) {
					continue
				}
				if !known {
					errs.addf("%s：欄位掛了 %s，但 package %s 不在生成器認得的範圍", f.FullName(), limitsOption, fd.Package())
					continue
				}
				v := opts.ProtoReflect().Get(xt.TypeDescriptor()).Message()
				lf := s.limitField(f, v.Get(maxItems).Int(), v.Get(maxRunes).Int(), &errs)
				if lf != nil {
					out = append(out, lf)
				}
			}
			walk(fd, m.Messages(), known)
		}
	}
	s.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		_, known := s.pkgShort[string(fd.Package())]
		walk(fd, fd.Messages(), known)
		return true
	})
	if err := errs.err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, l := range out {
		if _, ok := limitsGoTargets[l.Message]; !ok {
			errs.addf("%s：訊息 %s 沒有登記 Go 常數的輸出位置（limitsGoTargets）", l.Path(), l.Message)
		}
		if seen[l.Message+"/"+l.goName()] {
			errs.addf("%s：生成的常數名 %s 重複", l.Path(), l.goName())
		}
		seen[l.Message+"/"+l.goName()] = true
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	// 同一訊息的欄位依宣告順序；跨訊息依訊息路徑。
	sort.SliceStable(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out, nil
}

func (s *Schema) limitField(f protoreflect.FieldDescriptor, items, runes int64, errs *errList) *LimitField {
	pkg := string(f.ParentFile().Package())
	msg := s.pkgShort[pkg] + "." + strings.TrimPrefix(string(f.Parent().FullName()), pkg+".")
	lf := &LimitField{Message: msg, Name: string(f.Name()), Items: items, Runes: runes}
	switch {
	case items < 0 || runes < 0:
		errs.addf("%s：%s 的上限不得為負", f.FullName(), limitsOption)
	case (items > 0) == (runes > 0):
		errs.addf("%s：%s 必須恰好設 max_items 或 max_runes 其中一個（大於 0）", f.FullName(), limitsOption)
	case items > 0 && !f.IsList():
		errs.addf("%s：max_items 只能掛在 repeated 欄位", f.FullName())
	case runes > 0 && (f.IsList() || f.Kind() != protoreflect.StringKind):
		errs.addf("%s：max_runes 只能掛在單值的 string 欄位", f.FullName())
	default:
		return lf
	}
	return nil
}

// EmitGoLimits 生成每個目標目錄的常數檔：路徑 → 內容。
func EmitGoLimits(fields []*LimitField) (map[string][]byte, error) {
	byDir := map[string][]*LimitField{}
	for _, l := range fields {
		byDir[limitsGoTargets[l.Message].Dir] = append(byDir[limitsGoTargets[l.Message].Dir], l)
	}
	out := map[string][]byte{}
	for dir, fs := range byDir {
		var b strings.Builder
		fmt.Fprintf(&b, "// Code generated by backend/cmd/uitext-gen; DO NOT EDIT.\n// %s\n\npackage %s\n\nconst (\n", generatedHeader, limitsGoTargets[fs[0].Message].Package)
		for _, l := range fs {
			fmt.Fprintf(&b, "\t// %s 是 %s 的上限（%s，%s）。\n\t%s = %d\n", l.goName(), l.Path(), limitsOption, l.unit(), l.goName(), l.Value())
		}
		b.WriteString(")\n")
		src, err := format.Source([]byte(b.String()))
		if err != nil {
			return nil, err
		}
		out[dir+"/limits_gen.go"] = src
	}
	return out, nil
}

// EmitTSLimits 生成 @hawkr/ui-text 的 limits.gen.ts：每個上限一個 number 常數，名稱 <訊息路徑>_<欄位>_MAX_<ITEMS|RUNES>。
func EmitTSLimits(fields []*LimitField) []byte {
	var b strings.Builder
	b.WriteString(tsHeader)
	for _, l := range fields {
		fmt.Fprintf(&b, "\n/** %s 的上限（%s，%s）。 */\nexport const %s = %d;\n", l.Path(), limitsOption, l.unit(), limitConstName(l), l.Value())
	}
	return []byte(b.String())
}

func limitConstName(l *LimitField) string {
	return constName(l.Path()) + "_MAX_" + strings.ToUpper(l.unit())
}

// EmitDartLimits 生成 hawkr_ui_text 的 limits.g.dart：每個上限一個 int 常數，名稱 <訊息路徑><欄位>Max<Items|Runes>。
func EmitDartLimits(fields []*LimitField) []byte {
	var b strings.Builder
	b.WriteString("// dart format off\n// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。\n")
	for _, l := range fields {
		fmt.Fprintf(&b, "\n/// %s 的上限（%s，%s）。\nconst int %s = %d;\n", l.Path(), limitsOption, l.unit(), lowerFirst(pascal(l.Path()))+"Max"+upperFirst(l.unit()), l.Value())
	}
	return []byte(b.String())
}
