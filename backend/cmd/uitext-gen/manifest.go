package main

import "google.golang.org/protobuf/reflect/protoreflect"

type manifestEnum struct {
	Name        string `json:"name"`
	TSName      string `json:"tsName,omitempty"`
	TSModule    string `json:"tsModule,omitempty"`
	DartClass   string `json:"dartClass,omitempty"`
	DartLibrary string `json:"dartLibrary,omitempty"`
	InCatalog   bool   `json:"inCatalog"`
	ServerOwned bool   `json:"serverOwned"`
}

type manifest struct {
	Enums []manifestEnum `json:"enums"`
}

// Manifest 回守門工具用的 manifest：每個 hawkr.* proto enum 的全名、各端的名稱、是否在目錄、是否伺服器擁有。
// Dart 版不含 hawkr.admin.*（packages/hawkr_api/buf.gen.yaml 不生成後台的 API）。
func (m *Model) Manifest(dart bool) ([]byte, error) {
	inCatalog := map[protoreflect.FullName]bool{}
	for _, f := range m.Funcs {
		if f.Kind == FuncNoun {
			inCatalog[f.Enum.Desc.FullName()] = true
		}
	}
	var out manifest
	out.Enums = []manifestEnum{}
	for _, e := range m.Schema.enums {
		if dart && isAdminPackage(e) {
			continue
		}
		me := manifestEnum{
			Name:        string(e.Desc.FullName()),
			InCatalog:   inCatalog[e.Desc.FullName()],
			ServerOwned: m.Schema.serverOwned[e.Desc.FullName()],
		}
		if dart {
			me.DartClass = enumTypeName(e)
			me.DartLibrary = dartLibrary(e)
		} else {
			me.TSName = enumTypeName(e)
			me.TSModule = tsModule(e)
		}
		out.Enums = append(out.Enums, me)
	}
	return marshalJSON(out)
}
