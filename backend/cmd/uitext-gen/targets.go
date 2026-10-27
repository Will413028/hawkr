package main

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// enumTypeName 是 protoc-gen-es 與 protoc_plugin 共同的型別名稱：巢狀型別用底線接上外層名稱。
func enumTypeName(e *EnumInfo) string {
	pkg := string(e.Desc.ParentFile().Package())
	return strings.ReplaceAll(strings.TrimPrefix(string(e.Desc.FullName()), pkg+"."), ".", "_")
}

// tsModule 是列舉所在的 TS 生成檔（@hawkr/api 的 subpath）。
func tsModule(e *EnumInfo) string {
	path := strings.TrimSuffix(e.Desc.ParentFile().Path(), ".proto")
	return "@hawkr/api/gen/" + path + "_pb"
}

// dartLibrary 是列舉所在的 Dart 生成檔。
func dartLibrary(e *EnumInfo) string {
	path := strings.TrimSuffix(e.Desc.ParentFile().Path(), ".proto")
	return "package:hawkr_api/gen/" + path + ".pbenum.dart"
}

// dartAlias 是 import 的前綴：由檔案路徑決定，不會和別的檔相撞。
func dartAlias(e *EnumInfo) string {
	path := strings.TrimSuffix(e.Desc.ParentFile().Path(), ".proto")
	return strings.ReplaceAll(path, "/", "_")
}

func isAdminPackage(e *EnumInfo) bool {
	return strings.HasPrefix(string(e.Desc.ParentFile().Package()), "hawkr.admin.")
}

// enumNames 回列舉的值名（依值號，含零值）。
func enumNames(e *EnumInfo) []protoreflect.EnumValueDescriptor {
	vs := e.Desc.Values()
	out := make([]protoreflect.EnumValueDescriptor, 0, vs.Len())
	for i := range vs.Len() {
		out = append(out, vs.Get(i))
	}
	return out
}

// usedEnums 回目錄的函式用到的所有列舉（名詞、變體與參數），依全名排序。
func (m *Model) usedEnums() []*EnumInfo {
	seen := map[protoreflect.FullName]*EnumInfo{}
	for _, f := range m.Funcs {
		if f.Enum != nil {
			seen[f.Enum.Desc.FullName()] = f.Enum
		}
		for _, p := range f.Params {
			if p.Enum != nil {
				seen[p.Enum.Desc.FullName()] = p.Enum
			}
		}
	}
	var out []*EnumInfo
	for _, e := range m.Schema.enums {
		if seen[e.Desc.FullName()] != nil {
			out = append(out, e)
		}
	}
	return out
}

// nameTableEnums 回要生成「值號 → 值名」表的列舉：被當作 select 參數的列舉。
func (m *Model) selectEnums() []*EnumInfo {
	seen := map[protoreflect.FullName]*EnumInfo{}
	for _, f := range m.Funcs {
		for _, p := range f.Params {
			if p.Kind == ParamEnum {
				seen[p.Enum.Desc.FullName()] = p.Enum
			}
		}
	}
	var out []*EnumInfo
	for _, e := range m.Schema.enums {
		if seen[e.Desc.FullName()] != nil {
			out = append(out, e)
		}
	}
	return out
}

func protoreflectName(s string) protoreflect.Name { return protoreflect.Name(s) }
