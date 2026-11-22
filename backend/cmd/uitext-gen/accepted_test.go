package main

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// acceptedImage 造一個 image：PolicyCategory、accepted_policy_categories 的擴充欄位，以及一個欄位掛了 numbers 的請求訊息。
// 選項直接寫成 unknown fields（和 buf image 讀進來、沒有編譯進型別的情況相同）；packed 是 repeated enum 在 proto3 的預設編碼。
func acceptedImage(numbers []int32, packed bool) *descriptorpb.FileDescriptorSet {
	var raw []byte
	if packed {
		var payload []byte
		for _, n := range numbers {
			payload = protowire.AppendVarint(payload, uint64(n))
		}
		raw = protowire.AppendTag(raw, 50001, protowire.BytesType)
		raw = protowire.AppendBytes(raw, payload)
	} else {
		for _, n := range numbers {
			raw = protowire.AppendTag(raw, 50001, protowire.VarintType)
			raw = protowire.AppendVarint(raw, uint64(n))
		}
	}
	opts := &descriptorpb.FieldOptions{}
	opts.ProtoReflect().SetUnknown(protoreflect.RawFields(raw))

	policy := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("hawkr/common/v1alpha1/policy_category.proto"),
		Package: proto.String("hawkr.common.v1alpha1"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			enumProto("PolicyCategory", "POLICY_CATEGORY_", "FOOD_SAFETY", "SPAM", "OTHER"),
		},
	}
	ext := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("hawkr/common/v1alpha1/accepted_values.proto"),
		Package:    proto.String("hawkr.common.v1alpha1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/descriptor.proto", "hawkr/common/v1alpha1/policy_category.proto"},
		Extension: []*descriptorpb.FieldDescriptorProto{{
			Name:     proto.String("accepted_policy_categories"),
			Number:   proto.Int32(50001),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
			TypeName: proto.String(".hawkr.common.v1alpha1.PolicyCategory"),
			Extendee: proto.String(".google.protobuf.FieldOptions"),
			JsonName: proto.String("acceptedPolicyCategories"),
		}},
	}
	f := field("category", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".hawkr.common.v1alpha1.PolicyCategory", false)
	f.Options = opts
	req := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("hawkr/fx/v1alpha1/req.proto"),
		Package:    proto.String("hawkr.fx.v1alpha1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"hawkr/common/v1alpha1/accepted_values.proto", "hawkr/common/v1alpha1/policy_category.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name:  proto.String("CreateRequest"),
			Field: []*descriptorpb.FieldDescriptorProto{f, field("plain", 2, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".hawkr.common.v1alpha1.PolicyCategory", false)},
		}},
	}
	return &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto), policy, ext, req,
	}}
}

func acceptedNames(a *AcceptedField) []string {
	var out []string
	for _, v := range a.Values {
		out = append(out, string(v.Name()))
	}
	return out
}

// 欄位層級的 custom option 在 image 裡是 unknown fields，生成器要讀得到值與順序（packed 與逐個編碼都要）。
func TestAcceptedFieldsReadsTheFieldOption(t *testing.T) {
	want := []string{"POLICY_CATEGORY_OTHER", "POLICY_CATEGORY_FOOD_SAFETY"}
	for _, packed := range []bool{true, false} {
		s, err := LoadSchema(acceptedImage([]int32{3, 1}, packed))
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.AcceptedFields()
		if err != nil {
			t.Fatalf("packed=%v：%v", packed, err)
		}
		if len(got) != 1 || got[0].Path != "fx.CreateRequest.category" {
			t.Fatalf("packed=%v：欄位 = %+v，預期只有 fx.CreateRequest.category（沒掛 option 的欄位不算）", packed, got)
		}
		if !slices.Equal(acceptedNames(got[0]), want) {
			t.Errorf("packed=%v：值 = %v，預期 %v（順序同 proto）", packed, acceptedNames(got[0]), want)
		}
	}
}

func TestAcceptedFieldsRejectsBadLists(t *testing.T) {
	for name, numbers := range map[string][]int32{
		"UNSPECIFIED": {0, 1},
		"重複":          {1, 1},
		"不認得的值號":      {1, 99},
	} {
		s, err := LoadSchema(acceptedImage(numbers, true))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AcceptedFields(); err == nil {
			t.Errorf("%s：應該報錯", name)
		}
	}
}

func TestAcceptedFieldsAbsentExtensionIsEmpty(t *testing.T) {
	s, err := LoadSchema(fixtureImage())
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.AcceptedFields()
	if err != nil || len(got) != 0 {
		t.Fatalf("沒有 option 定義的 image：got %v，err %v", got, err)
	}
}

func TestAcceptedEmittersListUsableValuesOnly(t *testing.T) {
	s, err := LoadSchema(acceptedImage([]int32{3, 1}, true))
	if err != nil {
		t.Fatal(err)
	}
	fields, err := s.AcceptedFields()
	if err != nil {
		t.Fatal(err)
	}
	goSrc, err := EmitGoAccepted(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := gofmtCheck(goSrc); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{
		"go":   string(goSrc),
		"ts":   string(EmitTSAccepted(fields)),
		"dart": string(EmitDartAccepted(fields)),
	} {
		if strings.Contains(src, "UNSPECIFIED") {
			t.Errorf("%s：清單不得含 UNSPECIFIED", name)
		}
		if strings.Index(src, "OTHER") > strings.Index(src, "FOOD_SAFETY") || !strings.Contains(src, "OTHER") {
			t.Errorf("%s：順序要同 proto（OTHER 在 FOOD_SAFETY 前）\n%s", name, src)
		}
	}
	if !strings.Contains(string(goSrc), "acceptedFxCreateRequestCategory") ||
		!strings.Contains(string(EmitTSAccepted(fields)), "FX_CREATE_REQUEST_CATEGORY") ||
		!strings.Contains(string(EmitDartAccepted(fields)), "fxCreateRequestCategory") {
		t.Error("三端的常數名稱要由欄位路徑決定")
	}
}
