package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// limitsDefinitionFile 是 Limits 與 limits 擴充欄位的定義檔。
func limitsDefinitionFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:       proto.String("hawkr/common/v1alpha1/limits.proto"),
		Package:    proto.String("hawkr.common.v1alpha1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/descriptor.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Limits"),
			Field: []*descriptorpb.FieldDescriptorProto{
				field("max_items", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", false),
				field("max_runes", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", false),
			},
		}},
		Extension: []*descriptorpb.FieldDescriptorProto{{
			Name:     proto.String("limits"),
			Number:   proto.Int32(50002),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String(".hawkr.common.v1alpha1.Limits"),
			Extendee: proto.String(".google.protobuf.FieldOptions"),
			JsonName: proto.String("limits"),
		}},
	}
}

// limitOption 編出 limits 選項的 unknown fields（和 buf image 讀進來、沒有編譯進型別的情況相同）。
func limitOption(items, runes int64) *descriptorpb.FieldOptions {
	var msg []byte
	if items != 0 {
		msg = protowire.AppendTag(msg, 1, protowire.VarintType)
		msg = protowire.AppendVarint(msg, uint64(items))
	}
	if runes != 0 {
		msg = protowire.AppendTag(msg, 2, protowire.VarintType)
		msg = protowire.AppendVarint(msg, uint64(runes))
	}
	var raw []byte
	raw = protowire.AppendTag(raw, 50002, protowire.BytesType)
	raw = protowire.AppendBytes(raw, msg)
	opts := &descriptorpb.FieldOptions{}
	opts.ProtoReflect().SetUnknown(protoreflect.RawFields(raw))
	return opts
}

type limitSpec struct {
	name         string
	repeated     bool
	items, runes int64
}

// limitsImage 造一個 image：catalog.ProductContentLimits 的欄位掛了 specs，另有一個沒掛選項的欄位。
func limitsImage(specs ...limitSpec) *descriptorpb.FileDescriptorSet {
	var fs []*descriptorpb.FieldDescriptorProto
	for i, sp := range specs {
		f := field(sp.name, int32(i+1), descriptorpb.FieldDescriptorProto_TYPE_STRING, "", sp.repeated)
		f.Options = limitOption(sp.items, sp.runes)
		fs = append(fs, f)
	}
	fs = append(fs, field("plain", int32(len(specs)+1), descriptorpb.FieldDescriptorProto_TYPE_STRING, "", false))
	catalog := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("hawkr/catalog/v1alpha1/catalog.proto"),
		Package:     proto.String("hawkr.catalog.v1alpha1"),
		Syntax:      proto.String("proto3"),
		Dependency:  []string{"hawkr/common/v1alpha1/limits.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("ProductContentLimits"), Field: fs}},
	}
	return &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto), limitsDefinitionFile(), catalog,
	}}
}

func TestLimitFieldsReadsTheFieldOption(t *testing.T) {
	s, err := LoadSchema(limitsImage(
		limitSpec{name: "title", runes: 100},
		limitSpec{name: "skus", repeated: true, items: 100},
		limitSpec{name: "alt", runes: 200},
	))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LimitFields()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range got {
		names = append(names, l.goName()+"="+string(rune('0'+l.Value()/100)))
	}
	want := "MaxTitleRunes=1 MaxSkus=1 MaxAltRunes=2"
	if strings.Join(names, " ") != want {
		t.Fatalf("欄位 = %v，預期 %s（宣告順序，沒掛 option 的欄位不算）", names, want)
	}
	goOut, err := EmitGoLimits(got)
	if err != nil {
		t.Fatal(err)
	}
	src := string(goOut["backend/internal/modules/catalog/internal/model/limits_gen.go"])
	for _, w := range []string{"package model", "MaxTitleRunes = 100", "MaxSkus = 100", "MaxAltRunes = 200"} {
		if !strings.Contains(src, w) {
			t.Errorf("Go 輸出缺 %q\n%s", w, src)
		}
	}
	if ts := string(EmitTSLimits(got)); !strings.Contains(ts, "export const CATALOG_PRODUCT_CONTENT_LIMITS_TITLE_MAX_RUNES = 100;") ||
		!strings.Contains(ts, "CATALOG_PRODUCT_CONTENT_LIMITS_SKUS_MAX_ITEMS = 100;") {
		t.Errorf("TS 輸出不對\n%s", ts)
	}
	if dart := string(EmitDartLimits(got)); !strings.Contains(dart, "const int catalogProductContentLimitsTitleMaxRunes = 100;") ||
		!strings.Contains(dart, "catalogProductContentLimitsSkusMaxItems = 100;") {
		t.Errorf("Dart 輸出不對\n%s", dart)
	}
}

func TestLimitFieldsRejectsBadOptions(t *testing.T) {
	for name, sp := range map[string]limitSpec{
		"兩個都設":                   {name: "x", runes: 5, items: 5},
		"兩個都沒設":                  {name: "x"},
		"負數":                     {name: "x", runes: -1},
		"max_items 掛在非 repeated": {name: "x", items: 5},
		"max_runes 掛在 repeated":  {name: "x", repeated: true, runes: 5},
	} {
		s, err := LoadSchema(limitsImage(sp))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.LimitFields(); err == nil {
			t.Errorf("%s：應該報錯", name)
		}
	}
}

func TestLimitFieldsAbsentExtensionIsError(t *testing.T) {
	img := fixtureImage(func(fds *descriptorpb.FileDescriptorSet) {
		for _, f := range fds.File {
			if f.GetName() == "hawkr/common/v1alpha1/limits.proto" {
				f.Extension = nil
			}
		}
	})
	s, err := LoadSchema(img)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.LimitFields(); err == nil {
		t.Fatalf("沒有 option 定義的 image 應該報錯，got %v", got)
	}
}

func TestLimitFieldsRejectsUnregisteredMessage(t *testing.T) {
	img := limitsImage(limitSpec{name: "title", runes: 10})
	img.File[2].MessageType[0].Name = proto.String("Elsewhere")
	s, err := LoadSchema(img)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.LimitFields(); err == nil {
		t.Fatalf("沒登記輸出位置的訊息應該報錯，got %v", got)
	}
}
