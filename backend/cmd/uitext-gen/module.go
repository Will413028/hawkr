package main

import (
	"sort"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext/icu"
)

// ModuleModel 是一個模組 text/ 目錄的句子（§2.6）。
type ModuleModel struct {
	Dir  string
	Keys []*ModuleKey
	// Files 是正規化後的來源檔（語系 → 檔），給 -write 用。
	Files map[string]*File
}

// ModuleKey 是模組的一句：key 是 lower_snake 的點分路徑。
type ModuleKey struct {
	Key    string
	Params []Param
	Msgs   map[string]*Msg
	// Staff 表示這句的受眾是後台員工：宣告在來源語系檔的 staff 段，只有 zh-TW（ADR D1）。
	Staff bool
}

// buildModule 驗證並建立模組句子（§2.2 第 9 條：key 是 lower_snake 的點分路徑，其餘同 3–6）。
func buildModule(cfg *Config, mi ModuleInput, errs *errList) *ModuleModel {
	mm := &ModuleModel{Dir: mi.Dir, Files: mi.Files}
	src := mi.Files[cfg.SourceLocale]
	if src == nil {
		errs.addf("%s：缺來源語系 %s 的檔", mi.Dir, cfg.SourceLocale)
		return nil
	}
	for loc := range mi.Files {
		if !contains(cfg.Locales, loc) {
			errs.addf("%s：語系 %s 不在 config.json 的 locales", mi.Dir, loc)
		}
	}
	for loc, f := range mi.Files {
		if len(f.Variants) > 0 {
			errs.addf("%s/%s.json：模組句子沒有 variants", mi.Dir, loc)
		}
		if len(f.Staff) > 0 && loc != staffLocale {
			errs.addf("%s/%s.json：staff 句子只有 %s（ADR D1：staff 受眾的文字只有 zh-TW）", mi.Dir, loc, staffLocale)
		}
	}
	byKey := map[string]*ModuleKey{}
	srcEntries := map[string]Entry{}
	isStaff := map[string]bool{}
	var keys []string
	for key, e := range src.Default {
		keys = append(keys, key)
		srcEntries[key] = e
	}
	for key, e := range src.Staff {
		if cfg.SourceLocale != staffLocale {
			break
		}
		if _, dup := srcEntries[key]; dup {
			errs.addf("%s/%s.json %s：同時在 default 與 staff 段（一句只有一種受眾）", mi.Dir, cfg.SourceLocale, key)
			continue
		}
		keys = append(keys, key)
		srcEntries[key] = e
		isStaff[key] = true
	}
	sort.Strings(keys)
	for _, key := range keys {
		e := srcEntries[key]
		where := mi.Dir + "/" + cfg.SourceLocale + ".json " + key
		okKey := true
		for _, seg := range strings.Split(key, ".") {
			if !lowerSnake.MatchString(seg) {
				errs.addf("%s：key 要是 lower_snake 的點分路徑（%q 不合）", where, seg)
				okKey = false
				break
			}
		}
		if !okKey {
			continue
		}
		mk := &ModuleKey{Key: key, Msgs: map[string]*Msg{}, Staff: isStaff[key]}
		byKey[key] = mk
		mm.Keys = append(mm.Keys, mk)
		if strings.TrimSpace(e.Description) == "" {
			errs.addf("%s：缺 description", where)
		}
		for name, spec := range e.Params {
			if !paramName.MatchString(name) || reservedParams[name] {
				errs.addf("%s：參數名稱 %q 不合法", where, name)
				continue
			}
			switch {
			case spec.Type == "string" && spec.Enum == "" && spec.Noun == "":
				mk.Params = append(mk.Params, Param{Name: name, Kind: ParamString})
			case spec.Type == "integer" && spec.Enum == "" && spec.Noun == "":
				mk.Params = append(mk.Params, Param{Name: name, Kind: ParamInteger})
			default:
				errs.addf("%s：模組句子的參數只能是 string 或 integer（%s）", where, name)
			}
		}
		sort.Slice(mk.Params, func(i, j int) bool { return mk.Params[i].Name < mk.Params[j].Name })
	}
	for _, loc := range cfg.Locales {
		f := mi.Files[loc]
		if f == nil {
			if contains(cfg.Required, loc) {
				errs.addf("%s：缺 required 語系 %s 的檔", mi.Dir, loc)
			}
			continue
		}
		entries := f.Default
		if loc == staffLocale {
			entries = map[string]Entry{}
			for key, e := range f.Default {
				entries[key] = e
			}
			for key, e := range f.Staff {
				if _, dup := entries[key]; !dup {
					entries[key] = e
				}
			}
		}
		for key, e := range entries {
			where := mi.Dir + "/" + loc + ".json " + key
			mk := byKey[key]
			if mk == nil {
				if _, inSrc := srcEntries[key]; !inSrc {
					errs.addf("%s：%s 沒有這個 key（多餘的 key）", where, cfg.SourceLocale)
				}
				continue
			}
			if mk.Staff && loc != cfg.SourceLocale {
				errs.addf("%s：staff 句子只有 %s，不能有 %s 的文字", where, staffLocale, loc)
				continue
			}
			if loc != cfg.SourceLocale && (len(e.Params) > 0 || e.Description != "" || e.Note != "" || e.DescriptionOverride != "") {
				errs.addf("%s：非來源語系只放 message", where)
			}
			ast, err := icu.Parse(e.Message)
			if err != nil {
				errs.addf("%s：訊息不合子集：%v", where, err)
				continue
			}
			mk.Msgs[loc] = &Msg{Text: e.Message, AST: ast}
			checkModuleUses(where, loc, mk, srcEntries[key], ast, errs)
		}
		if contains(cfg.Required, loc) {
			for _, key := range keys {
				if byKey[key] != nil && byKey[key].Staff && loc != cfg.SourceLocale {
					continue
				}
				if entries[key].Message == "" {
					errs.addf("%s/%s.json %s：缺 required 語系的文字", mi.Dir, loc, key)
				}
			}
		}
	}
	return mm
}

func checkModuleUses(where, loc string, mk *ModuleKey, src Entry, ast *icu.Message, errs *errList) {
	uses, conflict := ast.Uses()
	if conflict != "" {
		errs.addf("%s：參數 %s 同時用作不同的型別", where, conflict)
	}
	declared := map[string]Param{}
	for _, p := range mk.Params {
		declared[p.Name] = p
	}
	used := map[string]bool{}
	for _, u := range uses {
		used[u.Name] = true
		p, ok := declared[u.Name]
		if !ok {
			errs.addf("%s：訊息用了沒有宣告的參數 %s", where, u.Name)
			continue
		}
		switch {
		case u.Kind == icu.UseSelect:
			errs.addf("%s：模組句子沒有 enum 參數，不能用 select（%s）", where, u.Name)
		case u.Kind == icu.UsePlural && p.Kind != ParamInteger:
			errs.addf("%s：plural 參數 %s 要是 integer", where, u.Name)
		case u.Kind == icu.UsePlural:
			cats, _ := icu.Categories(loc)
			for _, s := range u.Selectors {
				if !contains(cats, s) {
					errs.addf("%s：參數 %s 的 plural 分支 %q 不屬於 %s", where, u.Name, s, loc)
				}
			}
		}
	}
	for name := range src.Params {
		if !used[name] {
			errs.addf("%s：宣告了參數 %s 但訊息沒有用到（每個語系的參數集合必須與來源相同）", where, name)
		}
	}
}
