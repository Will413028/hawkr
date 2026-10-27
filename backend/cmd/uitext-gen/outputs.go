package main

import (
	"fmt"
	"sort"
	"strings"
)

// 輸出路徑（相對於 repo 根目錄）。
const (
	catalogDir = "backend/internal/platform/uitext/catalog"
	goDir      = "backend/internal/platform/uitext"
	tsDir      = "packages/hawkr-ui-text-ts"
	dartDir    = "packages/hawkr_ui_text"
)

// Outputs 回所有生成檔：路徑 → 內容。
func (m *Model) Outputs() (map[string][]byte, error) {
	if err := m.checkNames(); err != nil {
		return nil, err
	}
	if err := m.checkEnumValueNames(); err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	var err error
	if out[goDir+"/catalog_gen.go"], err = m.EmitGo(); err != nil {
		return nil, fmt.Errorf("catalog_gen.go：%w", err)
	}
	if out[goDir+"/golden_calls_gen_test.go"], err = m.EmitGoGoldenCalls(); err != nil {
		return nil, fmt.Errorf("golden_calls_gen_test.go：%w", err)
	}
	golden, err := m.Golden()
	if err != nil {
		return nil, err
	}
	gb, err := MarshalGolden(golden)
	if err != nil {
		return nil, err
	}
	out[goDir+"/testdata/golden.gen.json"] = gb
	out[tsDir+"/tests/golden.gen.json"] = gb
	out[dartDir+"/test/golden.g.json"] = gb

	for _, loc := range m.Config.Locales {
		b, err := m.EmitMessages(loc)
		if err != nil {
			return nil, err
		}
		out[tsDir+"/src/messages/"+loc+".gen.json"] = b
	}
	out[tsDir+"/src/keys.gen.ts"] = m.EmitTSKeys()
	out[tsDir+"/src/zh-tw.gen.ts"] = m.EmitTSZhTW()
	out[tsDir+"/tests/golden-calls.gen.ts"] = m.EmitTSGoldenCalls()
	if out[tsDir+"/src/enum-manifest.gen.json"], err = m.Manifest(false); err != nil {
		return nil, err
	}
	out[dartDir+"/lib/src/ui_text.g.dart"] = m.EmitDart()
	out[dartDir+"/test/golden_calls.g.dart"] = m.EmitDartGoldenCalls()
	if out[dartDir+"/tool/enum_manifest.g.json"], err = m.Manifest(true); err != nil {
		return nil, err
	}
	for _, mm := range m.Modules {
		b, err := m.EmitModule(mm)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", mm.Dir, err)
		}
		out["backend/"+strings.TrimPrefix(mm.Dir, "backend/")+"/text_gen.go"] = b
	}
	return out, nil
}

// checkEnumValueNames 擋下 golden 與分派表依賴的前提：目錄用到的列舉，值名全域唯一。
func (m *Model) checkEnumValueNames() error {
	seen := map[string]string{}
	var errs errList
	for _, e := range m.usedEnums() {
		for _, v := range enumNames(e) {
			name := string(v.Name())
			if prev, ok := seen[name]; ok && prev != string(e.Desc.FullName()) {
				errs.addf("值名 %s 同時屬於 %s 與 %s，golden 的分派表以值名查值號", name, prev, e.Desc.FullName())
			}
			seen[name] = string(e.Desc.FullName())
		}
	}
	return errs.err()
}

// SourceFiles 回 -write 要寫回的來源檔（正規形式）。
func (m *Model) SourceFiles() (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, loc := range m.Config.Locales {
		b, err := MarshalFile(m.Catalog[loc])
		if err != nil {
			return nil, err
		}
		out[catalogDir+"/"+loc+".json"] = b
	}
	for _, mm := range m.Modules {
		var locs []string
		for loc := range mm.Files {
			locs = append(locs, loc)
		}
		sort.Strings(locs)
		for _, loc := range locs {
			b, err := MarshalFile(mm.Files[loc])
			if err != nil {
				return nil, err
			}
			out[mm.Dir+"/"+loc+".json"] = b
		}
	}
	return out, nil
}
