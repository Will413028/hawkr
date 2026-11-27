package main

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

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
				field("max_total_runes", 3, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", false),
				field("max_image_items", 4, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", false),
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

func limitOption(sp limitSpec) *descriptorpb.FieldOptions {
	var msg []byte
	for num, v := range []int64{sp.items, sp.runes, sp.totalRunes, sp.imageItems} {
		if v == 0 {
			continue
		}
		msg = protowire.AppendTag(msg, protowire.Number(num+1), protowire.VarintType)
		msg = protowire.AppendVarint(msg, uint64(v))
	}
	var raw []byte
	raw = protowire.AppendTag(raw, 50002, protowire.BytesType)
	raw = protowire.AppendBytes(raw, msg)
	opts := &descriptorpb.FieldOptions{}
	opts.ProtoReflect().SetUnknown(protoreflect.RawFields(raw))
	return opts
}

type limitSpec struct {
	name                                 string
	repeated, message                    bool
	items, runes, totalRunes, imageItems int64
}

func limitsImage(specs ...limitSpec) *descriptorpb.FileDescriptorSet {
	var fs []*descriptorpb.FieldDescriptorProto
	for i, sp := range specs {
		typ, typeName := descriptorpb.FieldDescriptorProto_TYPE_STRING, ""
		if sp.message {
			typ, typeName = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".hawkr.catalog.v1alpha1.Block"
		}
		f := field(sp.name, int32(i+1), typ, typeName, sp.repeated)
		f.Options = limitOption(sp)
		fs = append(fs, f)
	}
	fs = append(fs, field("plain", int32(len(specs)+1), descriptorpb.FieldDescriptorProto_TYPE_STRING, "", false))
	catalog := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("hawkr/catalog/v1alpha1/catalog.proto"),
		Package:    proto.String("hawkr.catalog.v1alpha1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"hawkr/common/v1alpha1/limits.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("ProductInput"), Field: fs},
			{Name: proto.String("Block")},
		},
	}
	return &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto), limitsDefinitionFile(), catalog,
	}}
}

func TestLimitFieldsReadsTheFieldOption(t *testing.T) {
	s, err := LoadSchema(limitsImage(
		limitSpec{name: "title", runes: 100},
		limitSpec{name: "skus", repeated: true, items: 100},
		limitSpec{name: "values", repeated: true, items: 50, runes: 40},
		limitSpec{name: "description", repeated: true, message: true, items: 7, totalRunes: 3000, imageItems: 20},
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
		names = append(names, fmt.Sprintf("%s=%d", l.goName(), l.Value()))
	}
	want := "MaxProductInputTitleRunes=100 MaxProductInputSkus=100 MaxProductInputValues=50 MaxProductInputValuesRunes=40 " +
		"MaxProductInputDescription=7 MaxProductInputDescriptionTotalRunes=3000 MaxProductInputDescriptionImageItems=20"
	if strings.Join(names, " ") != want {
		t.Fatalf("欄位 = %v，預期 %s（宣告順序，沒掛 option 的欄位不算）", names, want)
	}
	goOut, err := EmitGoLimits(got)
	if err != nil {
		t.Fatal(err)
	}
	src := string(goOut["backend/internal/modules/catalog/internal/model/limits_gen.go"])
	for _, w := range []string{"package model", "MaxProductInputTitleRunes = 100", "MaxProductInputValuesRunes = 40", "MaxProductInputDescriptionTotalRunes = 3000"} {
		if !strings.Contains(src, w) {
			t.Errorf("Go 輸出缺 %q\n%s", w, src)
		}
	}
	ts := string(EmitTSLimits(got))
	for _, w := range []string{"CATALOG_PRODUCT_INPUT_TITLE_MAX_RUNES = 100;", "CATALOG_PRODUCT_INPUT_SKUS_MAX_ITEMS = 100;", "CATALOG_PRODUCT_INPUT_DESCRIPTION_MAX_IMAGE_ITEMS = 20;", "CATALOG_PRODUCT_INPUT_DESCRIPTION_MAX_TOTAL_RUNES = 3000;"} {
		if !strings.Contains(ts, w) {
			t.Errorf("TS 輸出缺 %q\n%s", w, ts)
		}
	}
	dart := string(EmitDartLimits(got))
	for _, w := range []string{"const int catalogProductInputTitleMaxRunes = 100;", "catalogProductInputSkusMaxItems = 100;", "catalogProductInputDescriptionMaxImageItems = 20;", "catalogProductInputDescriptionMaxTotalRunes = 3000;"} {
		if !strings.Contains(dart, w) {
			t.Errorf("Dart 輸出缺 %q\n%s", w, dart)
		}
	}
}

func TestLimitFieldsRejectsBadOptions(t *testing.T) {
	for name, sp := range map[string]limitSpec{
		"都沒設":                                {name: "x"},
		"負數":                                 {name: "x", runes: -1},
		"max_items 掛在非 repeated":             {name: "x", items: 5},
		"max_runes 掛在 repeated 訊息":           {name: "x", repeated: true, message: true, runes: 5},
		"max_runes 掛在單值訊息":                   {name: "x", message: true, runes: 5},
		"max_total_runes 掛在 string":          {name: "x", totalRunes: 5},
		"max_total_runes 掛在 repeated string": {name: "x", repeated: true, totalRunes: 5},
		"max_image_items 掛在 repeated string": {name: "x", repeated: true, imageItems: 5},
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

func TestLimitFieldsRejectsUnregisteredPackage(t *testing.T) {
	img := limitsImage(limitSpec{name: "title", runes: 10})
	delete(limitsGoTargets, "catalog")
	t.Cleanup(func() {
		limitsGoTargets["catalog"] = struct{ Dir, Package string }{"backend/internal/modules/catalog/internal/model", "model"}
	})
	s, err := LoadSchema(img)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.LimitFields(); err == nil {
		t.Fatalf("沒登記輸出位置的 package 應該報錯，got %v", got)
	}
}
