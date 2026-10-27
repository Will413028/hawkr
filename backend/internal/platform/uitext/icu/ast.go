// Package icu 是共用文案目錄的 ICU MessageFormat 受限子集：parser、AST 與參考格式器。
//
// 子集（設計 step10e-Lprime-design.md §1.4）：純文字、{arg}、{arg, select, … other {…}}、
// {arg, plural, [one] {…} other {…}}，巢狀最多兩層。生成器用它驗證訊息，共用 golden 的期望值由
// 這裡的參考格式器算出；Go、TS、Dart 三個執行期各自實作，golden 測試斷言它們與參考格式器一致。
package icu

// MaxDepth 是 select 與 plural 允許的巢狀層數（最外層算第一層）。
const MaxDepth = 2

// Message 是解析後的訊息。
type Message struct {
	Nodes []Node
}

// Node 是 Text、Arg、Select、Plural 其中之一。
type Node interface{ isNode() }

// Text 是字面文字（apostrophe 規則處理後的值）。
type Text struct{ Value string }

// Arg 是 {name}。
type Arg struct{ Name string }

// Select 是 {name, select, …}。Branches 依來源順序，必定含 other。
type Select struct {
	Name     string
	Branches []Branch
}

// Plural 是 {name, plural, …}。Branches 依來源順序，必定含 other。
type Plural struct {
	Name     string
	Branches []Branch
}

// Branch 是一個分支；Selector 對 select 是值名或 other，對 plural 是 CLDR 類別。
type Branch struct {
	Selector string
	Body     []Node
}

func (Text) isNode()   {}
func (Arg) isNode()    {}
func (Select) isNode() {}
func (Plural) isNode() {}

// UseKind 是參數在訊息裡的用法。
type UseKind int

const (
	// UseArg 是 {name}。
	UseArg UseKind = iota + 1
	// UseSelect 是 {name, select, …}。
	UseSelect
	// UsePlural 是 {name, plural, …}。
	UsePlural
)

// Use 是一個參數在訊息裡的用法與（select、plural 時的）分支名稱。
type Use struct {
	Name      string
	Kind      UseKind
	Selectors []string
}

// Uses 依第一次出現的順序列出訊息用到的參數；同名參數合併，Kind 不同時以 conflict 回報。
func (m *Message) Uses() (uses []Use, conflict string) {
	index := map[string]int{}
	var walk func(nodes []Node)
	add := func(name string, kind UseKind, selectors []string) {
		i, ok := index[name]
		if !ok {
			index[name] = len(uses)
			uses = append(uses, Use{Name: name, Kind: kind, Selectors: append([]string(nil), selectors...)})
			return
		}
		switch {
		case uses[i].Kind == kind:
		case uses[i].Kind == UsePlural && kind == UseArg:
			// plural 的分支裡用 {n} 顯示同一個數字。
			return
		case uses[i].Kind == UseArg && kind == UsePlural:
			uses[i].Kind = UsePlural
		default:
			conflict = name
			return
		}
		for _, s := range selectors {
			found := false
			for _, e := range uses[i].Selectors {
				if e == s {
					found = true
					break
				}
			}
			if !found {
				uses[i].Selectors = append(uses[i].Selectors, s)
			}
		}
	}
	walk = func(nodes []Node) {
		for _, n := range nodes {
			switch n := n.(type) {
			case Arg:
				add(n.Name, UseArg, nil)
			case Select:
				add(n.Name, UseSelect, selectorsOf(n.Branches))
				for _, b := range n.Branches {
					walk(b.Body)
				}
			case Plural:
				add(n.Name, UsePlural, selectorsOf(n.Branches))
				for _, b := range n.Branches {
					walk(b.Body)
				}
			}
		}
	}
	walk(m.Nodes)
	return uses, conflict
}

func selectorsOf(bs []Branch) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Selector
	}
	return out
}
