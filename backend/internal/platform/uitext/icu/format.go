package icu

import (
	"fmt"
	"strconv"
	"strings"
)

// Categories 是語系的 plural 類別（CLDR 的整數規則）：zh 只有 other，en 是 one、other。
func Categories(locale string) ([]string, error) {
	switch locale {
	case "zh-TW":
		return []string{"other"}, nil
	case "en":
		return []string{"one", "other"}, nil
	}
	return nil, fmt.Errorf("不認得的語系 %q", locale)
}

// Category 是整數 n 在語系的 plural 類別。這是參考實作，與 uitext 執行期（plural.go）各寫一份，由 golden 對照。
func Category(locale string, n int) (string, error) {
	switch locale {
	case "zh-TW":
		return "other", nil
	case "en":
		if n == 1 {
			return "one", nil
		}
		return "other", nil
	}
	return "", fmt.Errorf("不認得的語系 %q", locale)
}

// Format 是參考格式器。args 的值是 string 或 int：select 用 string，plural 用 int，{arg} 兩者皆可
// （整數一律輸出十進位 ASCII、不分組）。缺參數、型別不對都回錯誤。
func Format(m *Message, locale string, args map[string]any) (string, error) {
	var b strings.Builder
	if err := formatNodes(&b, m.Nodes, locale, args); err != nil {
		return "", err
	}
	return b.String(), nil
}

func formatNodes(b *strings.Builder, nodes []Node, locale string, args map[string]any) error {
	for _, n := range nodes {
		switch n := n.(type) {
		case Text:
			b.WriteString(n.Value)
		case Arg:
			v, ok := args[n.Name]
			if !ok {
				return fmt.Errorf("缺參數 %s", n.Name)
			}
			switch v := v.(type) {
			case string:
				b.WriteString(v)
			case int:
				b.WriteString(strconv.Itoa(v))
			default:
				return fmt.Errorf("參數 %s 的型別 %T 不是 string 或 int", n.Name, v)
			}
		case Select:
			v, ok := args[n.Name].(string)
			if !ok {
				return fmt.Errorf("select 參數 %s 要是 string", n.Name)
			}
			body, err := branch(n.Branches, v)
			if err != nil {
				return err
			}
			if err := formatNodes(b, body, locale, args); err != nil {
				return err
			}
		case Plural:
			v, ok := args[n.Name].(int)
			if !ok {
				return fmt.Errorf("plural 參數 %s 要是 int", n.Name)
			}
			cat, err := Category(locale, v)
			if err != nil {
				return err
			}
			body, err := branch(n.Branches, cat)
			if err != nil {
				return err
			}
			if err := formatNodes(b, body, locale, args); err != nil {
				return err
			}
		}
	}
	return nil
}

// branch 取 selector 的分支，沒有就取 other。
func branch(bs []Branch, selector string) ([]Node, error) {
	for _, b := range bs {
		if b.Selector == selector {
			return b.Body, nil
		}
	}
	for _, b := range bs {
		if b.Selector == "other" {
			return b.Body, nil
		}
	}
	return nil, fmt.Errorf("沒有 other 分支")
}
