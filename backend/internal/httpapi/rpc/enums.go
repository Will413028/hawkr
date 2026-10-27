package rpc

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// 契約的列舉與模組的值（ADR 2026-09-25-codemeta-retirement D3）：proto 的值名去掉型別的前綴就是模組的值，
// 例如 APP_TYPE_BUYER ↔ BUYER。前綴取自零值的名稱（APP_TYPE_UNSPECIFIED 去掉 UNSPECIFIED）。

type protoEnum interface {
	~int32
	protoreflect.Enum
}

// EnumFromProto 把請求裡的列舉轉成模組的值。UNSPECIFIED 轉成空字串，不認得的數字轉成它的十進位字串，
// 兩者都由模組回報不認得的值。
func EnumFromProto[T ~string, E protoEnum](e E) T {
	d := e.Descriptor()
	v := d.Values().ByNumber(e.Number())
	switch {
	case v == nil:
		return T(strconv.Itoa(int(e.Number())))
	case v.Number() == 0:
		return ""
	}
	return T(strings.TrimPrefix(string(v.Name()), enumPrefix(d)))
}

// EnumToProto 把模組的值轉成契約的列舉。契約沒有的值轉成 UNSPECIFIED；每個模組值域的 EnumMismatches
// 測試擋下這種情況。
func EnumToProto[E protoEnum, T ~string](t T) E {
	var zero E
	d := zero.Descriptor()
	v := d.Values().ByName(protoreflect.Name(enumPrefix(d) + string(t)))
	if v == nil {
		return zero
	}
	return E(v.Number())
}

// EnumMismatches 列出模組的值域 values 與契約的列舉 E 對不上的地方，給測試用：每個模組的值都要轉得成
// proto 的值再轉回來，每個 proto 的值（UNSPECIFIED 除外）也都要是模組的值。
func EnumMismatches[E protoEnum, T ~string](values []T) []string {
	var zero E
	d := zero.Descriptor()
	var out []string
	for _, t := range values {
		e := EnumToProto[E](t)
		if e == zero || EnumFromProto[T](e) != t {
			out = append(out, fmt.Sprintf("模組的 %q 在 %s 沒有對應的值", t, d.FullName()))
		}
	}
	vs := d.Values()
	for i := range vs.Len() {
		v := vs.Get(i)
		if v.Number() == 0 {
			continue
		}
		if !slices.Contains(values, EnumFromProto[T](E(v.Number()))) {
			out = append(out, fmt.Sprintf("%s 的 %s 不是模組的值", d.FullName(), v.Name()))
		}
	}
	return out
}

func enumPrefix(d protoreflect.EnumDescriptor) string {
	return strings.TrimSuffix(string(d.Values().ByNumber(0).Name()), "UNSPECIFIED")
}
