package main

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

const staffNote = "受眾：後台員工"

// SyncDescriptions 依 §1.3 把 description 同步進來源語系的來源檔（記憶體中）：
// enum 值的名詞、變體、staff 的 description ＝ descriptionOverride ?? proto 註解，再接 note；staff 附受眾。
// 沒有 proto 來源的 key（訊息句、common.*）的 description 是手寫的，這裡只檢查非空。
// 值沒有註解時退回列舉型別的註解加上值名，兩者都沒有就失敗（每個 key 都要有說明，§2.2 第 6 條）。
func (m *Model) SyncDescriptions() error {
	var errs errList
	src := m.Catalog[m.Config.SourceLocale]
	keys := func(entries map[string]Entry) []string {
		var out []string
		for k := range entries {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	for _, aud := range []Audience{AudDefault, AudStaff} {
		entries := src.Default
		if aud == AudStaff {
			entries = src.Staff
		}
		for _, raw := range keys(entries) {
			e := entries[raw]
			k, err := m.Schema.ParseKey(raw)
			if err != nil {
				continue
			}
			desc, err := m.describe(k, e)
			if err != nil {
				errs.addf("%s %s：%v", aud, raw, err)
				continue
			}
			if desc != "" {
				if aud == AudStaff {
					desc += "\n" + staffNote
				}
				e.Description = desc
			} else if strings.TrimSpace(e.Description) == "" {
				errs.addf("%s %s：沒有說明（這種 key 的 description 要手寫）", aud, raw)
				continue
			}
			entries[raw] = e
		}
	}
	for raw, d := range src.Variants {
		if strings.TrimSpace(d.DescriptionOverride) == "" && strings.TrimSpace(d.Description) == "" {
			errs.addf("variants %s：缺 description（變體的說明要手寫）", raw)
		}
	}
	return errs.err()
}

// describe 回由 proto 或變體宣告算出的說明；沒有 proto 來源的 key 回空字串。
func (m *Model) describe(k *Key, e Entry) (string, error) {
	var base string
	switch k.Kind { //nolint:exhaustive // 其餘種類走預設分支
	case KeyNoun:
		base = e.DescriptionOverride
		if base == "" {
			v := k.Enum.Desc.Values().ByName(protoreflect.Name(k.Value))
			base = comment(v)
		}
		if base == "" {
			enumDoc := comment(k.Enum.Desc)
			if enumDoc == "" {
				return "", fmt.Errorf("值 %s 與列舉 %s 都沒有註解，請寫 descriptionOverride", k.Value, k.Enum.Path)
			}
			base = fmt.Sprintf("%s（值 %s）", enumDoc, k.Value)
		}
	case KeyVariant:
		d := m.Catalog[m.Config.SourceLocale].Variants[k.Enum.Path+"."+k.Variant]
		base = e.DescriptionOverride
		if base == "" {
			base = d.DescriptionOverride
		}
		if base == "" {
			base = d.Description
		}
		if base == "" {
			return "", fmt.Errorf("變體 %s.%s 沒有 description", k.Enum.Path, k.Variant)
		}
		base = fmt.Sprintf("%s（值 %s）", base, k.Value)
	default:
		return "", nil
	}
	if e.Note != "" {
		base += "\n" + e.Note
	}
	return base, nil
}
