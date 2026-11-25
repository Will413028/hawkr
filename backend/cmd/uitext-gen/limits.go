package main

import (
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

// limitsGoTargets 是 package 短名到 Go 常數檔所在目錄（相對於 repo 根目錄）與 Go package 名：package 內任何訊息的欄位掛了 limits，
// 常數都生成進擁有它的模組。掛了 limits 的 package 不在這張表裡就報錯。
var limitsGoTargets = map[string]struct{ Dir, Package string }{
	"catalog": {"backend/internal/modules/catalog/internal/model", "model"},
}

// 上限的種類，對應 Limits 訊息的欄位。
const (
	// limitItems 是 max_items：repeated 欄位的元素個數。
	limitItems = "items"
	// limitRunes 是 max_runes：string 欄位的 code point 數；用在 repeated string 時是每個元素的上限。
	limitRunes = "runes"
	// limitTotalRunes 是 max_total_runes：repeated 訊息欄位的所有元素裡文字內容的 code point 總數（排除圖片的替代文字）。
	limitTotalRunes = "total_runes"
	// limitImageItems 是 max_image_items：repeated 訊息欄位的元素裡帶圖片的個數。
	limitImageItems = "image_items"
)

// limitKinds 依輸出順序列出每種上限：Limits 的欄位名、Go 常數的字尾、Dart 常數的字尾。
var limitKinds = []struct{ Kind, Field, GoSuffix, DartSuffix string }{
	{limitItems, "max_items", "", "MaxItems"},
	{limitRunes, "max_runes", "Runes", "MaxRunes"},
	{limitTotalRunes, "max_total_runes", "TotalRunes", "MaxTotalRunes"},
	{limitImageItems, "max_image_items", "ImageItems", "MaxImageItems"},
}

// LimitField 是一個欄位上的一個上限。一個欄位可以同時掛幾種上限（例如 repeated string 的個數與每個元素的長度），各自是一個 LimitField。
type LimitField struct {
	// Message 是 key 風格的訊息路徑，例如 catalog.ProductInput；第一段是 package 短名。
	Message string
	// Name 是欄位名（lower_snake）。
	Name string
	// Kind 是上限的種類：limitItems、limitRunes、limitTotalRunes 或 limitImageItems。
	Kind string
	// Count 是上限值，大於 0。
	Count int64
}

// Value 是欄位的上限值。
func (l *LimitField) Value() int64 { return l.Count }

// Path 是欄位的 key 風格路徑。
func (l *LimitField) Path() string { return l.Message + "." + l.Name }

// unit 是上限的單位名稱（種類名）。
func (l *LimitField) unit() string { return l.Kind }

func (l *LimitField) pkg() string { return strings.SplitN(l.Message, ".", 2)[0] }

func (l *LimitField) kindInfo() (goSuffix, dartSuffix string) {
	for _, k := range limitKinds {
		if k.Kind == l.Kind {
			return k.GoSuffix, k.DartSuffix
		}
	}
	return "", ""
}

// goName 是 Go 常數名：Max<訊息><欄位><字尾>，字尾依種類為空、Runes、TotalRunes、ImageItems。
// 訊息路徑不含 package：catalog.ProductInput 的 title 的 max_runes 是 MaxProductInputTitleRunes。
func (l *LimitField) goName() string {
	suffix, _ := l.kindInfo()
	_, msg, _ := strings.Cut(l.Message, ".")
	return "Max" + pascal(msg) + pascal(l.Name) + suffix
}

// dartName 是 Dart 常數名：<package><訊息><欄位>Max<Items|Runes|TotalRunes|ImageItems>。
func (l *LimitField) dartName() string {
	_, suffix := l.kindInfo()
	return lowerFirst(pascal(l.Path())) + suffix
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
	kindFields := map[string]protoreflect.FieldDescriptor{}
	for _, k := range limitKinds {
		fd := xd.Message().Fields().ByName(protoreflect.Name(k.Field))
		if fd == nil {
			return nil, fmt.Errorf("limits 的訊息 Limits 必須有 %s", k.Field)
		}
		kindFields[k.Kind] = fd
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
				out = append(out, s.limitFields(f, v, kindFields, &errs)...)
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
		if _, ok := limitsGoTargets[l.pkg()]; !ok {
			errs.addf("%s：package %s 沒有登記 Go 常數的輸出位置（limitsGoTargets）", l.Path(), l.pkg())
		}
		if seen[l.pkg()+"/"+l.goName()] {
			errs.addf("%s：生成的常數名 %s 重複", l.Path(), l.goName())
		}
		seen[l.pkg()+"/"+l.goName()] = true
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	// 同一訊息的欄位依宣告順序；跨訊息依訊息路徑。
	sort.SliceStable(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out, nil
}

// limitFields 把欄位上的 Limits 訊息拆成每種上限一個 LimitField，並檢查每種上限掛得合不合欄位：
// max_items 只能掛 repeated 欄位；max_runes 掛單值 string 或 repeated string（每個元素）；
// max_total_runes 與 max_image_items 只能掛 repeated 訊息欄位；每個值不得為負，至少設一種。
func (s *Schema) limitFields(f protoreflect.FieldDescriptor, v protoreflect.Message, kindFields map[string]protoreflect.FieldDescriptor, errs *errList) []*LimitField {
	pkg := string(f.ParentFile().Package())
	msg := s.pkgShort[pkg] + "." + strings.TrimPrefix(string(f.Parent().FullName()), pkg+".")
	var out []*LimitField
	ok := true
	for _, k := range limitKinds {
		n := v.Get(kindFields[k.Kind]).Int()
		if n == 0 {
			continue
		}
		var fits bool
		switch k.Kind {
		case limitItems:
			fits = f.IsList()
		case limitRunes:
			fits = f.Kind() == protoreflect.StringKind
		default:
			fits = f.IsList() && f.Kind() == protoreflect.MessageKind
		}
		switch {
		case n < 0:
			errs.addf("%s：%s 的 %s 不得為負", f.FullName(), limitsOption, k.Field)
			ok = false
		case !fits:
			errs.addf("%s：%s 不能掛在這種欄位（max_items 要 repeated；max_runes 要 string 或 repeated string；max_total_runes、max_image_items 要 repeated 訊息）", f.FullName(), k.Field)
			ok = false
		default:
			out = append(out, &LimitField{Message: msg, Name: string(f.Name()), Kind: k.Kind, Count: n})
		}
	}
	if len(out) == 0 && ok {
		errs.addf("%s：%s 必須至少設一種上限（大於 0）", f.FullName(), limitsOption)
	}
	if !ok {
		return nil
	}
	return out
}

// EmitGoLimits 生成每個目標目錄的常數檔：路徑 → 內容。
func EmitGoLimits(fields []*LimitField) (map[string][]byte, error) {
	byDir := map[string][]*LimitField{}
	for _, l := range fields {
		byDir[limitsGoTargets[l.pkg()].Dir] = append(byDir[limitsGoTargets[l.pkg()].Dir], l)
	}
	out := map[string][]byte{}
	for dir, fs := range byDir {
		var b strings.Builder
		fmt.Fprintf(&b, "// Code generated by backend/cmd/uitext-gen; DO NOT EDIT.\n// %s\n\npackage %s\n\nconst (\n", generatedHeader, limitsGoTargets[fs[0].pkg()].Package)
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

// EmitTSLimits 生成 @hawkr/ui-text 的 limits.gen.ts：每個上限一個 number 常數，名稱 <訊息路徑>_<欄位>_MAX_<ITEMS|RUNES|TOTAL_RUNES|IMAGE_ITEMS>。
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

// EmitDartLimits 生成 hawkr_ui_text 的 limits.g.dart：每個上限一個 int 常數，名稱 <package><訊息><欄位>Max<Items|Runes|TotalRunes|ImageItems>。
func EmitDartLimits(fields []*LimitField) []byte {
	var b strings.Builder
	b.WriteString("// dart format off\n// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。\n")
	for _, l := range fields {
		fmt.Fprintf(&b, "\n/// %s 的上限（%s，%s）。\nconst int %s = %d;\n", l.Path(), limitsOption, l.unit(), l.dartName(), l.Value())
	}
	return []byte(b.String())
}
