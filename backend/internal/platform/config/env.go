// Package config 從環境變數讀取設定。
//
// 缺少必填值或格式錯誤時，不退回預設值：錯誤先累積起來，由 Err 一次全部回報，
// 讓程式在啟動時就失敗。退回預設值會讓設定錯誤悄悄變成錯誤的行為。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env 讀取環境變數並累積問題。
type Env struct {
	lookup   func(string) (string, bool)
	problems []error
}

// NewEnv 回傳讀取 process 環境變數的 Env。
func NewEnv() *Env {
	return &Env{lookup: os.LookupEnv}
}

// FromMap 回傳讀取 m 的 Env，給測試用。
func FromMap(m map[string]string) *Env {
	return &Env{lookup: func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}}
}

func (e *Env) get(key string) (string, bool) {
	v, ok := e.lookup(key)
	return v, ok && v != ""
}

// String 回傳必填的字串設定。
func (e *Env) String(key string) string {
	v, ok := e.get(key)
	if !ok {
		e.problems = append(e.problems, fmt.Errorf("缺少必填的環境變數 %s", key))
	}
	return v
}

// StringOr 回傳字串設定；沒設定時回傳 def。
func (e *Env) StringOr(key, def string) string {
	if v, ok := e.get(key); ok {
		return v
	}
	return def
}

// Int 回傳整數設定；沒設定時回傳 def，格式錯誤時記一筆問題。
func (e *Env) Int(key string, def int) int {
	return parse(e, key, def, strconv.Atoi)
}

func (e *Env) Int32(key string, def int32) int32 {
	return parse(e, key, def, func(raw string) (int32, error) {
		value, err := strconv.ParseInt(raw, 10, 32)
		return int32(value), err //nolint:gosec // ParseInt with bitSize 32 bounds the value
	})
}

// Duration 回傳時間長度設定（例如 30s、5m）；沒設定時回傳 def，格式錯誤時記一筆問題。
func (e *Env) Duration(key string, def time.Duration) time.Duration {
	return parse(e, key, def, time.ParseDuration)
}

// Bool 回傳布林設定；沒設定時回傳 def，格式錯誤時記一筆問題。
func (e *Env) Bool(key string, def bool) bool {
	return parse(e, key, def, strconv.ParseBool)
}

func parse[T any](e *Env, key string, def T, fn func(string) (T, error)) T {
	raw, ok := e.get(key)
	if !ok {
		return def
	}
	v, err := fn(raw)
	if err != nil {
		e.problems = append(e.problems, fmt.Errorf("環境變數 %s 的值 %q 格式錯誤：%w", key, raw, err))
		return def
	}
	return v
}

// RequiredBool 回傳必填的布林設定。
func (e *Env) RequiredBool(key string) bool {
	raw := e.String(key)
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		e.problems = append(e.problems, fmt.Errorf("環境變數 %s 的值 %q 格式錯誤：%w", key, raw, err))
	}
	return v
}

// List 回傳逗號分隔的字串設定，去掉每一項前後的空白與空的項目；沒設定時回傳 nil。
func (e *Env) List(key string) []string {
	raw, ok := e.get(key)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Fail 記一筆設定的問題（例如整組設定只設了一部分），與其他問題一起由 Err 回報。
func (e *Env) Fail(format string, args ...any) {
	e.problems = append(e.problems, fmt.Errorf(format, args...))
}

// Err 回傳目前累積的所有問題；沒有問題時回傳 nil。
func (e *Env) Err() error {
	return errors.Join(e.problems...)
}
