package main

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// KeyKind 是 key 的種類（§1.2）。
type KeyKind int

const (
	// KeyNoun 是 enum 值的名詞：<pkg>.<型別路徑>.<VALUE>。
	KeyNoun KeyKind = iota + 1
	// KeyVariant 是依值的句子：<pkg>.<Enum 型別路徑>.<variant>.<VALUE>。
	KeyVariant
	// KeySentence 是訊息句：<pkg>.<Message 型別路徑>.<name>。
	KeySentence
	// KeyUnknown 是 common.unknown。
	KeyUnknown
	// KeyError 是 common.error.<CODE>。
	KeyError
)

// Key 是解析後的 key。
type Key struct {
	Raw     string
	Kind    KeyKind
	Enum    *EnumInfo
	Value   string // enum 值名（Noun、Variant）或 Connect code 名（Error）
	Variant string // lower_snake（Variant）
	Message string // Sentence 所屬 message 的型別路徑
	Name    string // Sentence 的名稱
	// FuncID 是同一個 API 函式的識別：Noun 是型別路徑，Variant 是 型別路徑.variant，Sentence 是 型別路徑.name。
	FuncID string
}

var lowerSnake = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// connectCodes 是 Connect 的 code（proto 名），依 code 值排序；CANCELED 不顯示文字，不在目錄。
var connectCodes = []string{
	"UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED", "NOT_FOUND", "ALREADY_EXISTS", "PERMISSION_DENIED",
	"RESOURCE_EXHAUSTED", "FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE", "UNIMPLEMENTED", "INTERNAL",
	"UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED",
}

func isConnectCode(s string) bool {
	for _, c := range connectCodes {
		if c == s {
			return true
		}
	}
	return false
}

// ParseKey 依 §1.2 解析 key。失敗訊息不含 key 本身，由呼叫端點名。
func (s *Schema) ParseKey(raw string) (*Key, error) {
	if raw == "common.unknown" {
		return &Key{Raw: raw, Kind: KeyUnknown, FuncID: raw}, nil
	}
	if code, ok := strings.CutPrefix(raw, "common.error."); ok {
		if code == "CANCELED" {
			return nil, fmt.Errorf("CANCELED 不顯示文字，不能放進目錄")
		}
		if !isConnectCode(code) {
			return nil, fmt.Errorf("%q 不是 Connect 的 code", code)
		}
		return &Key{Raw: raw, Kind: KeyError, Value: code, FuncID: "common.error"}, nil
	}
	tokens := strings.Split(raw, ".")
	for _, t := range tokens {
		if t == "" {
			return nil, fmt.Errorf("key 有空的段")
		}
	}
	t, rest, err := s.resolveType(tokens)
	if err != nil {
		return nil, err
	}
	if t.enum != nil {
		e := t.enum
		switch len(rest) {
		case 1:
			v, err := valueOf(e, rest[0])
			if err != nil {
				return nil, err
			}
			return &Key{Raw: raw, Kind: KeyNoun, Enum: e, Value: v, FuncID: e.Path}, nil
		case 2:
			if !lowerSnake.MatchString(rest[0]) {
				return nil, fmt.Errorf("變體名稱 %q 要是 lower_snake", rest[0])
			}
			v, err := valueOf(e, rest[1])
			if err != nil {
				return nil, err
			}
			return &Key{Raw: raw, Kind: KeyVariant, Enum: e, Variant: rest[0], Value: v, FuncID: e.Path + "." + rest[0]}, nil
		case 0:
			return nil, fmt.Errorf("key 只到列舉 %s，缺少值名", e.Path)
		}
		return nil, fmt.Errorf("列舉 %s 之後多出 %d 段（只能是 VALUE 或 variant.VALUE）", e.Path, len(rest))
	}
	if len(rest) != 1 {
		if len(rest) == 0 {
			return nil, fmt.Errorf("key 只到 message %s，缺少句子名稱", t.path)
		}
		return nil, fmt.Errorf("message %s 之後多出 %d 段（只能是一段 lower_snake 名稱）", t.path, len(rest))
	}
	if !lowerSnake.MatchString(rest[0]) {
		return nil, fmt.Errorf("句子名稱 %q 要是 lower_snake（或型別 %s 沒有這個巢狀型別或值）", rest[0], t.path)
	}
	return &Key{Raw: raw, Kind: KeySentence, Message: t.path, Name: rest[0], FuncID: t.path + "." + rest[0]}, nil
}

func valueOf(e *EnumInfo, name string) (string, error) {
	v := e.Desc.Values().ByName(protoreflect.Name(name))
	if v == nil {
		return "", fmt.Errorf("列舉 %s 沒有值 %s", e.Path, name)
	}
	if v.Number() == 0 {
		return "", fmt.Errorf("%s 是值號 0（UNSPECIFIED 不是有效值，不放進目錄）", name)
	}
	return name, nil
}
