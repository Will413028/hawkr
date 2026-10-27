package icu

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var pluralKeywords = map[string]bool{"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true}

// Parse 解析受限子集；超出子集的語法一律回錯誤，訊息指出位置（rune 位移）與原因。
func Parse(src string) (*Message, error) {
	p := &parser{src: []rune(src)}
	nodes, err := p.nodes(0, false)
	if err != nil {
		return nil, err
	}
	return &Message{Nodes: nodes}, nil
}

type parser struct {
	src []rune
	pos int
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("位置 %d：%s", p.pos, fmt.Sprintf(format, args...))
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) peek() rune {
	if p.eof() {
		return utf8.RuneError
	}
	return p.src[p.pos]
}

// nodes 讀到分支結尾的 }（inBranch）或字串結尾。depth 是目前 select／plural 的巢狀層數。
func (p *parser) nodes(depth int, inBranch bool) ([]Node, error) {
	var out []Node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, Text{Value: text.String()})
			text.Reset()
		}
	}
	for !p.eof() {
		r := p.src[p.pos]
		switch r {
		case '\'':
			if p.pos+1 < len(p.src) {
				switch p.src[p.pos+1] {
				case '{', '}':
					return nil, p.errorf("' 緊接 %c：ICU 的 apostrophe 規則會改變語意，子集禁止", p.src[p.pos+1])
				case '\'':
					return nil, p.errorf("連續兩個 '：子集不支援 ''，單獨的 ' 直接寫一次")
				}
			}
			text.WriteRune(r)
			p.pos++
		case '#', '<', '>':
			return nil, p.errorf("子集禁止 %q（數字一律寫 {arg}；< > 會被 use-intl 當 rich text）", r)
		case '}':
			if !inBranch {
				return nil, p.errorf("多出的 }")
			}
			flush()
			return out, nil
		case '{':
			flush()
			p.pos++
			n, err := p.argument(depth)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		default:
			text.WriteRune(r)
			p.pos++
		}
	}
	if inBranch {
		return nil, p.errorf("分支沒有以 } 結束")
	}
	flush()
	return out, nil
}

func (p *parser) skipSpace() {
	for !p.eof() && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}
}

func isIdent(r rune, first bool) bool {
	if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
		return true
	}
	return !first && r >= '0' && r <= '9'
}

func (p *parser) ident(what string) (string, error) {
	start := p.pos
	for !p.eof() && isIdent(p.src[p.pos], p.pos == start) {
		p.pos++
	}
	if p.pos == start {
		return "", p.errorf("缺少%s", what)
	}
	return string(p.src[start:p.pos]), nil
}

// argument 在 { 之後。
func (p *parser) argument(depth int) (Node, error) {
	p.skipSpace()
	name, err := p.ident("參數名稱")
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.peek() == '}' {
		p.pos++
		return Arg{Name: name}, nil
	}
	if p.peek() != ',' {
		return nil, p.errorf("參數 %s 之後只能是 } 或 ,", name)
	}
	p.pos++
	p.skipSpace()
	kind, err := p.ident("參數型別")
	if err != nil {
		return nil, err
	}
	if kind != "select" && kind != "plural" {
		return nil, p.errorf("子集不支援參數型別 %q（只有 select、plural）", kind)
	}
	if depth+1 > MaxDepth {
		return nil, p.errorf("select／plural 巢狀超過 %d 層", MaxDepth)
	}
	p.skipSpace()
	if p.peek() != ',' {
		return nil, p.errorf("%s 之後缺少 ,", kind)
	}
	p.pos++
	var branches []Branch
	seen := map[string]bool{}
	for {
		p.skipSpace()
		if p.eof() {
			return nil, p.errorf("%s 沒有以 } 結束", kind)
		}
		if p.peek() == '}' {
			p.pos++
			break
		}
		selector, err := p.selector(kind)
		if err != nil {
			return nil, err
		}
		if seen[selector] {
			return nil, p.errorf("分支 %q 重複", selector)
		}
		seen[selector] = true
		p.skipSpace()
		if p.peek() != '{' {
			return nil, p.errorf("分支 %q 之後缺少 {", selector)
		}
		p.pos++
		body, err := p.nodes(depth+1, true)
		if err != nil {
			return nil, err
		}
		p.pos++ // 吃掉分支結尾的 }
		branches = append(branches, Branch{Selector: selector, Body: body})
	}
	if !seen["other"] {
		return nil, p.errorf("%s 缺少 other 分支", kind)
	}
	if kind == "select" {
		return Select{Name: name, Branches: branches}, nil
	}
	return Plural{Name: name, Branches: branches}, nil
}

func (p *parser) selector(kind string) (string, error) {
	start := p.pos
	if p.peek() == '=' {
		return "", p.errorf("子集不支援精確分支 =N")
	}
	for !p.eof() && (unicode.IsLetter(p.src[p.pos]) || unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '_') {
		p.pos++
	}
	if p.pos == start {
		return "", p.errorf("缺少分支名稱")
	}
	s := string(p.src[start:p.pos])
	if kind == "plural" && !pluralKeywords[s] {
		return "", p.errorf("plural 的分支只能是 zero、one、two、few、many、other，不是 %q", s)
	}
	return s, nil
}
