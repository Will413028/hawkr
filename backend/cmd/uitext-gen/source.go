package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Config 是 catalog/config.json。
type Config struct {
	SourceLocale string   `json:"sourceLocale"`
	Locales      []string `json:"locales"`
	Required     []string `json:"required"`
}

// ParamSpec 是來源語系宣告的參數型別：enum（只能用在 select）、noun（名詞參數）、string、integer。
type ParamSpec struct {
	Enum string `json:"enum,omitempty"`
	Noun string `json:"noun,omitempty"`
	Type string `json:"type,omitempty"`
}

// Entry 是一則訊息。其他語系的 Entry 只有 Message。
type Entry struct {
	Message             string               `json:"message"`
	Params              map[string]ParamSpec `json:"params,omitempty"`
	Description         string               `json:"description,omitempty"`
	DescriptionOverride string               `json:"descriptionOverride,omitempty"`
	Note                string               `json:"note,omitempty"`
}

// VariantDecl 宣告一個變體；Omit 依受眾列出「沒有文字」的值（值名）。
type VariantDecl struct {
	Omit                map[string][]string `json:"omit,omitempty"`
	Description         string              `json:"description,omitempty"`
	DescriptionOverride string              `json:"descriptionOverride,omitempty"`
	Note                string              `json:"note,omitempty"`
}

// File 是一個語系的來源檔。
type File struct {
	Default  map[string]Entry       `json:"default,omitempty"`
	Staff    map[string]Entry       `json:"staff,omitempty"`
	Variants map[string]VariantDecl `json:"variants,omitempty"`
}

func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("JSON 之後還有內容")
	}
	return nil
}

// ParseFile 解析一個語系的來源檔。
func ParseFile(data []byte) (*File, error) {
	var f File
	if err := decodeStrict(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// ParseConfig 解析 config.json。
func ParseConfig(data []byte) (*Config, error) {
	var c Config
	if err := decodeStrict(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// MarshalFile 寫成正規形式：key 排序（encoding/json 對 map 的行為）、兩格縮排、不跳脫 HTML、結尾換行。
func MarshalFile(f *File) ([]byte, error) {
	return marshalJSON(f)
}

func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
