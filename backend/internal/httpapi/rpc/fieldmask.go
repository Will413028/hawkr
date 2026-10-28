package rpc

import (
	"slices"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ApplyUpdateMask 把 src 裡 update_mask 列出的欄位複製到 dst（Google AIP-134），dst 通常是目前的資源。
//   - mask 為 nil 或沒有路徑：視為列出 src 裡所有非零值的欄位。
//   - "*"：所有欄位。
//   - 只支援最上層的欄位；不認得或巢狀的路徑回 INVALID_ARGUMENT。
//
// 欄位的行為取自 proto 的 google.api.field_behavior（AIP-203）：OUTPUT_ONLY 與 IDENTIFIER 的欄位不複製，
// 列在 mask 裡也不回錯；IMMUTABLE 的欄位與 dst 的值不同時回 INVALID_ARGUMENT，相同時不動。
func ApplyUpdateMask(dst, src proto.Message, mask *fieldmaskpb.FieldMask) error {
	d, s := dst.ProtoReflect(), src.ProtoReflect()
	targets, err := updateMaskFields(src, mask)
	if err != nil {
		return err
	}
	for _, fd := range targets {
		switch {
		case notWritable(fd):
			continue
		case HasBehavior(fd, annotations.FieldBehavior_IMMUTABLE):
			if !s.Get(fd).Equal(d.Get(fd)) {
				return Error(connect.CodeInvalidArgument, "%s 建立後不能修改", fd.Name())
			}
			continue
		}
		if s.Has(fd) {
			d.Set(fd, s.Get(fd))
		} else {
			d.Clear(fd)
		}
	}
	return nil
}

// notWritable 是呼叫者不能寫入的欄位（OUTPUT_ONLY、IDENTIFIER）：列在 mask 裡也略過、不回錯。這個判斷只在這裡。
func notWritable(fd protoreflect.FieldDescriptor) bool {
	return HasBehavior(fd, annotations.FieldBehavior_OUTPUT_ONLY) || HasBehavior(fd, annotations.FieldBehavior_IDENTIFIER)
}

// WritableFields 回傳 update_mask 選到、而且呼叫者可以寫入的欄位路徑（以 . 分隔），資源有 INPUT_ONLY 或只讀得到遮蔽值的
// 欄位、不能「先合併再整筆送」時，用它決定哪些欄位要送給模組（API 設計規範 §七）。可寫的欄位集合完全取自 proto 的 descriptor
// 與 field_behavior，沒有手寫的清單：
//   - OUTPUT_ONLY 與 IDENTIFIER 的欄位略過（列在 mask 裡也不回錯）；這個判斷只在這裡（notWritable）。
//   - IMMUTABLE 的欄位：第二種做法拿不到目前值，無法比對新舊，所以 mask 明列它時回 INVALID_ARGUMENT；沒帶 mask 或 "*"
//     展開時略過。
//   - 巢狀路徑只支援下到 oneof 的子訊息（例如 individual.id_card_no）；其他子訊息（日期、證件）整個換掉，路徑停在
//     那個欄位。列到 oneof 的子訊息本身（individual）是它全部可寫的欄位。
//   - 沒帶 mask：src 裡有值的最上層欄位。"*"：所有最上層欄位，oneof 只展開有設定的那一支。
func WritableFields(src proto.Message, mask *fieldmaskpb.FieldMask) ([]string, error) {
	desc := src.ProtoReflect().Descriptor()
	var out []string
	add := func(paths ...string) {
		for _, p := range paths {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	paths := mask.GetPaths()
	switch {
	case len(paths) == 0:
		fields := desc.Fields()
		for i := range fields.Len() {
			if fd := fields.Get(i); src.ProtoReflect().Has(fd) {
				add(expandWritable(src.ProtoReflect(), fd, "", modeImplicit)...)
			}
		}
	case len(paths) == 1 && paths[0] == "*":
		fields := desc.Fields()
		for i := range fields.Len() {
			add(expandWritable(src.ProtoReflect(), fields.Get(i), "", modeWildcard)...)
		}
	default:
		for _, p := range paths {
			expanded, err := explicitWritable(desc, p)
			if err != nil {
				return nil, err
			}
			add(expanded...)
		}
	}
	return out, nil
}

// isBranch 回報欄位是 oneof 的子訊息（可以在 mask 裡下到它裡面）。
func isBranch(fd protoreflect.FieldDescriptor) bool {
	return fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() && fd.ContainingOneof() != nil && !fd.ContainingOneof().IsSynthetic()
}

// expandMode 是沒帶 mask（implicit：只有有值的欄位）與 "*"（wildcard：所有欄位，oneof 只展開有設定的一支）的展開方式。
type expandMode int

const (
	modeImplicit expandMode = iota
	modeWildcard
)

// expandWritable 把一個欄位展開成可寫的路徑：oneof 的子訊息展開成它裡面可寫的欄位。
func expandWritable(msg protoreflect.Message, fd protoreflect.FieldDescriptor, prefix string, mode expandMode) []string {
	if notWritable(fd) || HasBehavior(fd, annotations.FieldBehavior_IMMUTABLE) {
		return nil
	}
	path := prefix + string(fd.Name())
	if !isBranch(fd) {
		return []string{path}
	}
	if mode == modeWildcard && !msg.Has(fd) {
		return nil
	}
	child := msg.Get(fd).Message()
	var out []string
	children := fd.Message().Fields()
	for i := range children.Len() {
		c := children.Get(i)
		if mode == modeImplicit && !child.Has(c) {
			continue
		}
		// 展開到子訊息裡面之後，欄位都是「列出的欄位」：wildcard 的子訊息裡所有可寫的欄位都算。
		out = append(out, expandWritable(child, c, path+".", modeWildcard)...)
	}
	return out
}

// explicitWritable 解析 mask 明列的一條路徑。
func explicitWritable(desc protoreflect.MessageDescriptor, path string) ([]string, error) {
	segments := strings.Split(path, ".")
	prefix := ""
	for i, name := range segments {
		fd := desc.Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return nil, Error(connect.CodeInvalidArgument, "update_mask 的 %q 不是可以修改的欄位", path)
		}
		last := i == len(segments)-1
		switch {
		case !last && !isBranch(fd):
			return nil, Error(connect.CodeInvalidArgument, "update_mask 的 %q 不支援：巢狀路徑只能下到 oneof 的子訊息，其他子訊息整個換掉", path)
		case !last:
			desc = fd.Message()
			prefix += name + "."
			continue
		case notWritable(fd):
			return nil, nil
		case HasBehavior(fd, annotations.FieldBehavior_IMMUTABLE):
			return nil, Error(connect.CodeInvalidArgument, "%s 建立後不能修改", path)
		case !isBranch(fd):
			return []string{prefix + name}, nil
		}
		var out []string
		children := fd.Message().Fields()
		for j := range children.Len() {
			child := children.Get(j)
			if notWritable(child) {
				continue
			}
			if HasBehavior(child, annotations.FieldBehavior_IMMUTABLE) {
				return nil, Error(connect.CodeInvalidArgument, "%s.%s 建立後不能修改", path, child.Name())
			}
			out = append(out, prefix+name+"."+string(child.Name()))
		}
		return out, nil
	}
	return nil, nil
}

// updateMaskFields 回傳 update_mask 選到的 src 的欄位（Google AIP-134），不看欄位的行為標註：
//   - mask 為 nil 或沒有路徑：src 裡所有有值的欄位。
//   - "*"：所有欄位。
//   - 只支援最上層的欄位；不認得或巢狀的路徑回 INVALID_ARGUMENT。
//
// 呼叫方用 ApplyUpdateMask 或 WritableFields。
func updateMaskFields(src proto.Message, mask *fieldmaskpb.FieldMask) ([]protoreflect.FieldDescriptor, error) {
	s := src.ProtoReflect()
	fields := s.Descriptor().Fields()
	var targets []protoreflect.FieldDescriptor
	switch paths := mask.GetPaths(); {
	case len(paths) == 0:
		s.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			targets = append(targets, fd)
			return true
		})
	case len(paths) == 1 && paths[0] == "*":
		for i := range fields.Len() {
			targets = append(targets, fields.Get(i))
		}
	default:
		for _, p := range paths {
			fd := fields.ByName(protoreflect.Name(p))
			if fd == nil {
				return nil, Error(connect.CodeInvalidArgument, "update_mask 的 %q 不是可以修改的欄位（只支援最上層的欄位）", p)
			}
			targets = append(targets, fd)
		}
	}
	return targets, nil
}

// HasBehavior 回報欄位在 proto 上是否標了 google.api.field_behavior 的 b（AIP-203）。
func HasBehavior(fd protoreflect.FieldDescriptor, b annotations.FieldBehavior) bool {
	bs, _ := proto.GetExtension(fd.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
	return slices.Contains(bs, b)
}
