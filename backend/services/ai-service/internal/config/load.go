package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const defaultTag = "default"

func fillFromEnv(dst any) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("fillFromEnv 需要 *struct，實得 %T", dst)
	}
	return fillStruct(v.Elem(), v.Elem().Type().Name())
}

func fillStruct(v reflect.Value, path string) error {
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fv := v.Field(i)
		fieldPath := path + "." + field.Name

		if field.Type.Kind() == reflect.Struct {
			if err := fillStruct(fv, fieldPath); err != nil {
				return err
			}
			continue
		}

		tag := field.Tag.Get(envTagKey)
		if tag == "" {
			continue
		}
		if !fv.CanSet() {
			return fmt.Errorf("%s 帶 env tag 但無法設值（未匯出欄位？）", fieldPath)
		}

		raw, name := lookupChain(tag)
		if raw == "" {
			raw = field.Tag.Get(defaultTag)
			name = strings.TrimSpace(strings.Split(tag, ",")[0]) + "（default）"
		}
		if err := setField(fv, raw); err != nil {
			return fmt.Errorf("%s：%s=%q %w", fieldPath, name, raw, err)
		}
	}
	return nil
}

func lookupChain(tag string) (value, name string) {
	for _, n := range strings.Split(tag, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if v := os.Getenv(n); v != "" {
			return v, n
		}
	}
	return "", ""
}

func setField(fv reflect.Value, raw string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
		return nil
	case reflect.Bool:
		if raw == "" {
			fv.SetBool(false)
			return nil
		}
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("不是合法的布林值（可用 true/false/1/0）")
		}
		fv.SetBool(b)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if raw == "" {
			fv.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(raw, 10, fv.Type().Bits())
		if err != nil {
			// ⛔ 不得靜默退回 0：那會讓「設定打錯字」與「刻意不設定」變成同一件事，
			// 而 0 對 store id 這類欄位是有意義的哨兵（＝未設定，fail-closed）。
			return fmt.Errorf("不是合法的整數")
		}
		fv.SetInt(n)
		return nil
	default:
		return fmt.Errorf("不支援的欄位型別 %s——在 setField 補一個 case，不要回到逐欄手寫", fv.Kind())
	}
}
