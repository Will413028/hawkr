package main

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func enumTypeName(e *EnumInfo) string {
	pkg := string(e.Desc.ParentFile().Package())
	return strings.ReplaceAll(strings.TrimPrefix(string(e.Desc.FullName()), pkg+"."), ".", "_")
}

func tsModule(e *EnumInfo) string {
	path := strings.TrimSuffix(e.Desc.ParentFile().Path(), ".proto")
	return "@hawkr/api/gen/" + path + "_pb"
}

func dartLibrary(e *EnumInfo) string {
	path := strings.TrimSuffix(e.Desc.ParentFile().Path(), ".proto")
	return "package:hawkr_api/gen/" + path + ".pbenum.dart"
}

func dartAlias(e *EnumInfo) string {
	path := strings.TrimSuffix(e.Desc.ParentFile().Path(), ".proto")
	return strings.ReplaceAll(path, "/", "_")
}

func isAdminPackage(e *EnumInfo) bool {
	return strings.HasPrefix(string(e.Desc.ParentFile().Package()), "hawkr.admin.")
}

func enumNames(e *EnumInfo) []protoreflect.EnumValueDescriptor {
	vs := e.Desc.Values()
	out := make([]protoreflect.EnumValueDescriptor, 0, vs.Len())
	for i := range vs.Len() {
		out = append(out, vs.Get(i))
	}
	return out
}

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
