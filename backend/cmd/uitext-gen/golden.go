package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext/icu"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GoldenCase 是共用 golden 的一筆：期望值由 icu 參考格式器算出，Go、TS、Dart 三個執行期各自斷言。
// Want 是 null 表示「沒有文字」（omit 的值，或名詞參數不認得）。
type GoldenCase struct {
	Key      string         `json:"key"`
	Fn       string         `json:"fn"`
	Value    string         `json:"value,omitempty"`
	Audience Audience       `json:"audience"`
	Locale   string         `json:"locale"`
	Params   map[string]any `json:"params,omitempty"`
	Want     *string        `json:"want"`
}

var goldenStrings = []string{"示例", "A / B 12:30"}
var goldenIntegers = []int{1, 2, 1234}

func (m *Model) funcByID(id string) *Func {
	for _, f := range m.Funcs {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// usedParams 回 case 的所有訊息（各受眾、語系）用到的參數與（select 的）分支。
func (c *Case) usedParams() map[string][]string {
	out := map[string][]string{}
	for _, byLoc := range c.Msgs {
		for _, msg := range byLoc {
			uses, _ := msg.AST.Uses()
			for _, u := range uses {
				out[u.Name] = append(out[u.Name], u.Selectors...)
			}
		}
	}
	return out
}

// Golden 產生共用 golden。
func (m *Model) Golden() ([]GoldenCase, error) {
	var out []GoldenCase
	for _, f := range m.Funcs {
		for _, c := range f.Cases {
			used := c.usedParams()
			combos := [][]paramValue{{}}
			for _, p := range f.Params {
				values := m.goldenValues(p, used)
				var next [][]paramValue
				for _, combo := range combos {
					for _, v := range values {
						next = append(next, append(append([]paramValue(nil), combo...), v))
					}
				}
				combos = next
			}
			type audLoc struct {
				aud Audience
				loc string
			}
			var matrix []audLoc
			for _, loc := range m.Config.Locales {
				matrix = append(matrix, audLoc{AudDefault, loc})
			}
			if f.HasStaff {
				matrix = append(matrix, audLoc{AudStaff, staffLocale})
			}
			for _, al := range matrix {
				for _, combo := range combos {
					params := map[string]any{}
					for i, p := range f.Params {
						params[p.Name] = combo[i].json
					}
					want, err := m.formatCase(f, c, al.aud, al.loc, combo)
					if err != nil {
						return nil, fmt.Errorf("golden %s：%w", c.Key.Raw, err)
					}
					gc := GoldenCase{Key: c.Key.Raw, Fn: f.ID, Value: c.Value, Audience: al.aud, Locale: al.loc, Want: want}
					if len(params) > 0 {
						gc.Params = params
					}
					out = append(out, gc)
				}
			}
		}
	}
	return out, nil
}

type paramValue struct {
	param Param
	// json 是 golden 檔裡的值：字串、整數，或 enum／名詞的值名。
	json any
}

func (m *Model) goldenValues(p Param, used map[string][]string) []paramValue {
	if _, ok := used[p.Name]; !ok {
		// 這個 key 不用這個參數：只給一個值，避免組合爆炸。
		switch p.Kind { //nolint:exhaustive // 其餘種類走預設分支
		case ParamInteger:
			return []paramValue{{p, goldenIntegers[0]}}
		case ParamEnum, ParamNoun:
			return []paramValue{{p, string(p.Enum.Desc.Values().ByNumber(0).Name())}}
		}
		return []paramValue{{p, goldenStrings[0]}}
	}
	var out []paramValue
	switch p.Kind {
	case ParamString:
		for _, s := range goldenStrings {
			out = append(out, paramValue{p, s})
		}
	case ParamInteger:
		for _, n := range goldenIntegers {
			out = append(out, paramValue{p, n})
		}
	case ParamEnum:
		seen := map[string]bool{}
		for _, s := range used[p.Name] {
			if s != "other" && !seen[s] {
				seen[s] = true
			}
		}
		var names []string
		for s := range seen {
			names = append(names, s)
		}
		sort.Slice(names, func(i, j int) bool {
			return p.Enum.Desc.Values().ByName(protoreflect.Name(names[i])).Number() < p.Enum.Desc.Values().ByName(protoreflect.Name(names[j])).Number()
		})
		for _, s := range names {
			out = append(out, paramValue{p, s})
		}
		// 不在任何分支裡的值（走 other）：UNSPECIFIED。
		out = append(out, paramValue{p, string(p.Enum.Desc.Values().ByNumber(0).Name())})
	case ParamNoun:
		for _, v := range p.Enum.Values {
			out = append(out, paramValue{p, string(v.Name())})
		}
		out = append(out, paramValue{p, string(p.Enum.Desc.Values().ByNumber(0).Name())})
	}
	return out
}

// formatCase 用參考格式器算期望值。
func (m *Model) formatCase(f *Func, c *Case, aud Audience, loc string, combo []paramValue) (*string, error) {
	msg := f.Effective(m, c, aud, loc)
	if msg == nil {
		return nil, nil
	}
	args := map[string]any{}
	for _, pv := range combo {
		switch pv.param.Kind {
		case ParamString:
			args[pv.param.Name] = pv.json
		case ParamInteger:
			args[pv.param.Name] = pv.json
		case ParamEnum:
			args[pv.param.Name] = pv.json
		case ParamNoun:
			name := pv.json.(string)
			nf := m.funcByID(pv.param.Enum.Path)
			var noun *Case
			for _, nc := range nf.Cases {
				if nc.Value == name {
					noun = nc
				}
			}
			if noun == nil {
				return nil, nil // UNSPECIFIED 或不認得：整個建構函式沒有文字
			}
			nm := nf.Effective(m, noun, aud, loc)
			if nm == nil {
				return nil, nil
			}
			text, err := icu.Format(nm.AST, loc, nil)
			if err != nil {
				return nil, fmt.Errorf("名詞 %s：%w", noun.Key.Raw, err)
			}
			args[pv.param.Name] = text
		}
	}
	text, err := icu.Format(msg.AST, loc, args)
	if err != nil {
		return nil, err
	}
	return &text, nil
}

// MarshalGolden 一筆一行，diff 容易讀。
func MarshalGolden(cases []GoldenCase) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, c := range cases {
		var line bytes.Buffer
		enc := json.NewEncoder(&line)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(c); err != nil {
			return nil, err
		}
		buf.WriteString("  ")
		buf.WriteString(strings.TrimRight(line.String(), "\n"))
		if i < len(cases)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("]\n")
	return buf.Bytes(), nil
}
