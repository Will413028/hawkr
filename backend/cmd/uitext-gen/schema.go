package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var versionSegment = regexp.MustCompile(`^v[0-9]+((alpha|beta)[0-9]+)?$`)

// Schema 是 buf image 的索引：package 的短名、型別、列舉與註解。
type Schema struct {
	files *protoregistry.Files
	// pkgShort 是 proto package 到短名（去掉 hawkr. 與版本段）。
	pkgShort map[string]string
	// shortPkg 是短名到 proto package。
	shortPkg map[string]string
	// enums 依全名排序。
	enums    []*EnumInfo
	enumByFn map[protoreflect.FullName]*EnumInfo
	// serverOwned 是伺服器擁有文字的列舉（§2.3）。
	serverOwned map[protoreflect.FullName]bool
	// unavailableShapeErrors 是 §2.2 第 8 條的違規清單。
	unavailableShapeErrors []string
}

// EnumInfo 是一個 hawkr.* 列舉。
type EnumInfo struct {
	Desc protoreflect.EnumDescriptor
	// Path 是 key 裡的型別路徑，例如 seller.MeasureKind、identity.Account.DeletionState。
	Path string
	// Prefix 是值名的前綴（零值名去掉 UNSPECIFIED），也是模組字串值要加上的前綴。
	Prefix string
	// Values 依值號排序，不含零值。
	Values []protoreflect.EnumValueDescriptor
}

// LoadSchema 由 buf image 建立索引。
func LoadSchema(fds *descriptorpb.FileDescriptorSet) (*Schema, error) {
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("image 不合法：%w", err)
	}
	s := &Schema{
		files:       files,
		pkgShort:    map[string]string{},
		shortPkg:    map[string]string{},
		enumByFn:    map[protoreflect.FullName]*EnumInfo{},
		serverOwned: map[protoreflect.FullName]bool{},
	}
	pkgs := map[string]bool{}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if p := string(fd.Package()); strings.HasPrefix(p, "hawkr.") {
			pkgs[p] = true
		}
		return true
	})
	var names []string
	for p := range pkgs {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		short := shortPackage(p)
		if other, dup := s.shortPkg[short]; dup {
			return nil, fmt.Errorf("package %s 與 %s 去掉版本後同名（%s）", other, p, short)
		}
		s.shortPkg[short] = p
		s.pkgShort[p] = short
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if _, ok := s.pkgShort[string(fd.Package())]; !ok {
			return true
		}
		s.collectEnums(fd.Enums())
		s.collectMessages(fd.Messages())
		return true
	})
	sort.Slice(s.enums, func(i, j int) bool { return s.enums[i].Desc.FullName() < s.enums[j].Desc.FullName() })
	s.findServerOwned()
	return s, nil
}

func shortPackage(pkg string) string {
	parts := strings.Split(pkg, ".")
	parts = parts[1:] // hawkr
	if n := len(parts); n > 0 && versionSegment.MatchString(parts[n-1]) {
		parts = parts[:n-1]
	}
	return strings.Join(parts, ".")
}

func (s *Schema) addEnum(e protoreflect.EnumDescriptor) {
	pkg := string(e.ParentFile().Package())
	rel := strings.TrimPrefix(string(e.FullName()), pkg+".")
	info := &EnumInfo{Desc: e, Path: s.pkgShort[pkg] + "." + rel}
	vs := e.Values()
	if z := vs.ByNumber(0); z != nil {
		info.Prefix = strings.TrimSuffix(string(z.Name()), "UNSPECIFIED")
	}
	for i := range vs.Len() {
		if v := vs.Get(i); v.Number() != 0 {
			info.Values = append(info.Values, v)
		}
	}
	sort.Slice(info.Values, func(i, j int) bool { return info.Values[i].Number() < info.Values[j].Number() })
	s.enums = append(s.enums, info)
	s.enumByFn[e.FullName()] = info
}

func (s *Schema) collectEnums(es protoreflect.EnumDescriptors) {
	for i := range es.Len() {
		s.addEnum(es.Get(i))
	}
}

func (s *Schema) collectMessages(ms protoreflect.MessageDescriptors) {
	for i := range ms.Len() {
		m := ms.Get(i)
		s.collectEnums(m.Enums())
		s.collectMessages(m.Messages())
	}
}

// findServerOwned 依 §2.3 的結構規則找出伺服器擁有文字的列舉，並依 §2.2 第 8 條檢查 UnavailableReason 的形狀。
func (s *Schema) findServerOwned() {
	if e, ok := s.enumByName("hawkr.common.v1alpha1.ErrorReason"); ok {
		s.serverOwned[e.Desc.FullName()] = true
	}
	// *Unavailability message 的 reason 欄位。
	allowed := map[protoreflect.FullName]bool{}
	var walk func(ms protoreflect.MessageDescriptors)
	walk = func(ms protoreflect.MessageDescriptors) {
		for i := range ms.Len() {
			m := ms.Get(i)
			if strings.HasSuffix(string(m.Name()), "Unavailability") {
				if f := m.Fields().ByName("reason"); f != nil && f.Enum() != nil {
					s.serverOwned[f.Enum().FullName()] = true
					allowed[f.FullName()] = true
				}
			}
			walk(m.Messages())
		}
	}
	// 形狀守門：名字以 UnavailableReason 結尾的列舉，只能出現在 *Unavailability.reason。
	var check func(ms protoreflect.MessageDescriptors)
	check = func(ms protoreflect.MessageDescriptors) {
		for i := range ms.Len() {
			m := ms.Get(i)
			fs := m.Fields()
			for j := range fs.Len() {
				f := fs.Get(j)
				if f.Enum() != nil && strings.HasSuffix(string(f.Enum().Name()), "UnavailableReason") && !allowed[f.FullName()] {
					s.unavailableShapeErrors = append(s.unavailableShapeErrors,
						fmt.Sprintf("%s 使用 %s：*UnavailableReason 只能是 *Unavailability message 的 reason 欄位", f.FullName(), f.Enum().FullName()))
				}
			}
			check(m.Messages())
		}
	}
	s.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if _, ok := s.pkgShort[string(fd.Package())]; ok {
			walk(fd.Messages())
		}
		return true
	})
	s.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if _, ok := s.pkgShort[string(fd.Package())]; ok {
			check(fd.Messages())
		}
		return true
	})
	sort.Strings(s.unavailableShapeErrors)
}

func (s *Schema) enumByName(full string) (*EnumInfo, bool) {
	e, ok := s.enumByFn[protoreflect.FullName(full)]
	return e, ok
}

// EnumByPath 以 key 裡的型別路徑（例如 seller.MeasureKind）找列舉。
func (s *Schema) EnumByPath(path string) (*EnumInfo, error) {
	t, rest, err := s.resolveType(strings.Split(path, "."))
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 || t.enum == nil {
		return nil, fmt.Errorf("%q 不是列舉的型別路徑", path)
	}
	return t.enum, nil
}

// typeRef 是 key 解析出的型別。
type typeRef struct {
	enum *EnumInfo
	msg  protoreflect.MessageDescriptor
	path string
}

// resolveType 依 §1.2 的解析規則：最長相符的 package，再逐段往下找巢狀型別，直到不再是型別名。
// 回傳最後找到的型別與剩下的段。
func (s *Schema) resolveType(tokens []string) (typeRef, []string, error) {
	var pkg string
	n := 0
	for i := len(tokens); i >= 1; i-- {
		if p, ok := s.shortPkg[strings.Join(tokens[:i], ".")]; ok {
			pkg, n = p, i
			break
		}
	}
	if n == 0 {
		return typeRef{}, nil, fmt.Errorf("開頭不是任何 package")
	}
	rest := tokens[n:]
	if len(rest) == 0 {
		return typeRef{}, nil, fmt.Errorf("package 之後沒有型別")
	}
	full := protoreflect.FullName(pkg + "." + rest[0])
	var cur typeRef
	var scopeMsg protoreflect.MessageDescriptor
	d, err := s.files.FindDescriptorByName(full)
	if err != nil {
		return typeRef{}, nil, fmt.Errorf("package %s 沒有型別 %s", pkg, rest[0])
	}
	idx := 1
	for {
		switch d := d.(type) {
		case protoreflect.EnumDescriptor:
			cur = typeRef{enum: s.enumByFn[d.FullName()]}
			if cur.enum == nil {
				return typeRef{}, nil, fmt.Errorf("%s 不是 hawkr 的列舉", d.FullName())
			}
			cur.path = cur.enum.Path
			return cur, rest[idx:], nil
		case protoreflect.MessageDescriptor:
			scopeMsg = d
			cur = typeRef{msg: d, path: s.pkgShort[pkg] + "." + strings.TrimPrefix(string(d.FullName()), pkg+".")}
		default:
			return typeRef{}, nil, fmt.Errorf("%s 不是型別", full)
		}
		if idx >= len(rest) {
			return cur, nil, nil
		}
		next := protoreflect.FullName(string(scopeMsg.FullName()) + "." + rest[idx])
		nd, err := s.files.FindDescriptorByName(next)
		if err != nil {
			// 找不到巢狀型別：型別路徑到此為止，剩下的段是值名、變體或句子名稱。
			return cur, rest[idx:], nil //nolint:nilerr // 找不到不是錯誤
		}
		switch nd.(type) {
		case protoreflect.EnumDescriptor, protoreflect.MessageDescriptor:
		default:
			return cur, rest[idx:], nil
		}
		d = nd
		idx++
	}
}

// comment 取描述子的前導註解，各行去掉前後空白後以換行相接。
func comment(d protoreflect.Descriptor) string {
	loc := d.ParentFile().SourceLocations().ByDescriptor(d)
	text := strings.TrimSpace(loc.LeadingComments)
	if text == "" {
		return ""
	}
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		lines = append(lines, strings.TrimSpace(l))
	}
	return strings.Join(lines, "\n")
}
