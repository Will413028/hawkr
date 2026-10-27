package main

import (
	"encoding/json"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// 測試用的 proto：不依賴 buf（離線也能跑）。
//
//	hawkr.fx.v1alpha1：Color、Mode、Box.Size（巢狀）、Note、Box、Gadget（有 reason 欄位的 *Unavailability）
//	hawkr.admin.v1alpha1：Role；hawkr.admin.fx.v1alpha1：Level（最長相符）
//	hawkr.common.v1alpha1：ErrorReason（伺服器擁有）
func enumProto(name, prefix string, values ...string) *descriptorpb.EnumDescriptorProto {
	e := &descriptorpb.EnumDescriptorProto{Name: proto.String(name)}
	e.Value = append(e.Value, &descriptorpb.EnumValueDescriptorProto{Name: proto.String(prefix + "UNSPECIFIED"), Number: proto.Int32(0)})
	for i, v := range values {
		e.Value = append(e.Value, &descriptorpb.EnumValueDescriptorProto{Name: proto.String(prefix + v), Number: proto.Int32(int32(i + 1))})
	}
	return e
}

func field(name string, number int32, typ descriptorpb.FieldDescriptorProto_Type, typeName string, repeated bool) *descriptorpb.FieldDescriptorProto {
	label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	if repeated {
		label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	}
	f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Number: proto.Int32(number), Type: typ.Enum(), Label: label.Enum(), JsonName: proto.String(name)}
	if typeName != "" {
		f.TypeName = proto.String(typeName)
	}
	return f
}

func locComment(path []int32, text string) *descriptorpb.SourceCodeInfo_Location {
	return &descriptorpb.SourceCodeInfo_Location{Path: path, Span: []int32{0, 0, 1}, LeadingComments: proto.String(text)}
}

func fixtureImage(opts ...func(*descriptorpb.FileDescriptorSet)) *descriptorpb.FileDescriptorSet {
	fx := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("hawkr/fx/v1alpha1/fx.proto"),
		Package: proto.String("hawkr.fx.v1alpha1"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			enumProto("Color", "COLOR_", "RED", "BLUE", "GREEN"),
			enumProto("Mode", "MODE_", "A", "B"),
			enumProto("Tone", "TONE_", "WARM"),
			enumProto("GadgetUnavailableReason", "GADGET_UNAVAILABLE_REASON_", "BROKEN"),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:     proto.String("Box"),
				EnumType: []*descriptorpb.EnumDescriptorProto{enumProto("Size", "SIZE_", "SMALL", "LARGE")},
				Field:    []*descriptorpb.FieldDescriptorProto{field("title", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", false)},
			},
			{Name: proto.String("Note")},
			{
				Name: proto.String("GadgetUnavailability"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("reason", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".hawkr.fx.v1alpha1.GadgetUnavailableReason", false),
				},
			},
		},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{
			locComment([]int32{5, 0}, " Color 是顏色。\n"),
			locComment([]int32{5, 0, 2, 1}, " 紅色。\n 第二行。\n"),
			locComment([]int32{5, 1}, " Mode 是模式。\n"),
		}},
	}
	adminRoot := &descriptorpb.FileDescriptorProto{
		Name:     proto.String("hawkr/admin/v1alpha1/admin.proto"),
		Package:  proto.String("hawkr.admin.v1alpha1"),
		Syntax:   proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{enumProto("Role", "ROLE_", "ROOT")},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{
			locComment([]int32{5, 0}, " Role 是角色。\n"),
		}},
	}
	adminFx := &descriptorpb.FileDescriptorProto{
		Name:     proto.String("hawkr/admin/fx/v1alpha1/fx.proto"),
		Package:  proto.String("hawkr.admin.fx.v1alpha1"),
		Syntax:   proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{enumProto("Level", "LEVEL_", "HIGH")},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{
			locComment([]int32{5, 0}, " Level 是等級。\n"),
		}},
	}
	common := &descriptorpb.FileDescriptorProto{
		Name:     proto.String("hawkr/common/v1alpha1/error_reason.proto"),
		Package:  proto.String("hawkr.common.v1alpha1"),
		Syntax:   proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{enumProto("ErrorReason", "", "BOOM")},
	}
	// ErrorReason 的零值名稱是 UNSPECIFIED（沒有前綴），enumProto 已處理。
	fds := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fx, adminRoot, adminFx, common}}
	for _, o := range opts {
		o(fds)
	}
	return fds
}

// validFiles 是一組可以通過驗證的來源檔。
func validFiles() (*Config, map[string]*File) {
	cfg := &Config{SourceLocale: "zh-TW", Locales: []string{"zh-TW", "en"}, Required: []string{"zh-TW"}}
	zh := &File{
		Default: map[string]Entry{
			"fx.Color.COLOR_RED":   {Message: "紅"},
			"fx.Color.COLOR_BLUE":  {Message: "藍"},
			"fx.Color.COLOR_GREEN": {Message: "綠"},
			"fx.Mode.MODE_A": {
				Message: "{color, select, COLOR_RED {紅色模式} other {模式甲}}",
				Params:  map[string]ParamSpec{"color": {Enum: "fx.Color"}},
				Note:    "color 是目前的顏色。",
			},
			"fx.Mode.MODE_B":             {Message: "模式乙"},
			"fx.Box.Size.SIZE_SMALL":     {Message: "小", DescriptionOverride: "盒子的尺寸：小。"},
			"fx.Box.Size.SIZE_LARGE":     {Message: "大", DescriptionOverride: "盒子的尺寸：大。"},
			"admin.Role.ROLE_ROOT":       {Message: "根"},
			"admin.fx.Level.LEVEL_HIGH":  {Message: "高"},
			"fx.Color.label.COLOR_RED":   {Message: "顏色：{n}", Params: map[string]ParamSpec{"n": {Type: "string"}}},
			"fx.Color.label.COLOR_BLUE":  {Message: "藍色"},
			"fx.Color.label.COLOR_GREEN": {Message: "綠色"},
			"fx.Box.title":               {Message: "盒子：{color}", Params: map[string]ParamSpec{"color": {Noun: "fx.Color"}}, Description: "盒子的標題。"},
			"fx.Note.count":              {Message: "{n, plural, other {共 {n} 筆}}", Params: map[string]ParamSpec{"n": {Type: "integer"}}, Description: "筆數。"},
			"common.unknown":             {Message: "未知", Description: "未知。"},
		},
		Staff: map[string]Entry{
			"fx.Mode.MODE_A": {Message: "{color, select, COLOR_RED {後台紅} other {後台甲}}"},
			"fx.Mode.MODE_B": {Message: "後台乙"},
		},
		Variants: map[string]VariantDecl{
			"fx.Color.label": {Omit: map[string][]string{"default": {"COLOR_GREEN"}}, Description: "顏色的標籤。"},
		},
	}
	// green 的 label 被 omit，不能有文字。
	delete(zh.Default, "fx.Color.label.COLOR_GREEN")
	for _, c := range connectCodes {
		zh.Default["common.error."+c] = Entry{Message: "錯誤", Description: "預設錯誤文字。"}
	}
	en := &File{Default: map[string]Entry{"fx.Color.COLOR_RED": {Message: "Red"}}}
	return cfg, map[string]*File{"zh-TW": zh, "en": en}
}

// clone 深複製來源檔，讓每個失敗案例各自修改。
func cloneFiles(files map[string]*File) map[string]*File {
	out := map[string]*File{}
	for loc, f := range files {
		data, err := json.Marshal(f)
		if err != nil {
			panic(err)
		}
		var c File
		if err := json.Unmarshal(data, &c); err != nil {
			panic(err)
		}
		out[loc] = &c
	}
	return out
}

func validInputs() Inputs {
	cfg, files := validFiles()
	return Inputs{Image: fixtureImage(), Config: cfg, Catalog: files}
}

// buildWith 複製一份有效輸入，套用 mutate 後 Build。
func buildWith(t *testing.T, mutate func(*Inputs)) (*Model, error) {
	t.Helper()
	in := validInputs()
	in.Catalog = cloneFiles(in.Catalog)
	cfg := *in.Config
	in.Config = &cfg
	if mutate != nil {
		mutate(&in)
	}
	return Build(in)
}

func mustBuild(t *testing.T, mutate func(*Inputs)) *Model {
	t.Helper()
	m, err := buildWith(t, mutate)
	if err != nil {
		t.Fatalf("Build 失敗：\n%v", err)
	}
	return m
}

// wantError 斷言 Build 失敗，且錯誤訊息逐一包含 substrs（點名 key 與原因）。
func wantError(t *testing.T, mutate func(*Inputs), substrs ...string) {
	t.Helper()
	m, err := buildWith(t, mutate)
	if err == nil {
		t.Fatalf("應該失敗，但 Build 通過了（%d 個函式）", len(m.Funcs))
	}
	for _, s := range substrs {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("錯誤訊息缺 %q：\n%v", s, err)
		}
	}
}

// 測試裡常用的別名。
type (
	descriptorSet = descriptorpb.FileDescriptorSet
	messageProto  = descriptorpb.DescriptorProto
	fieldProto    = descriptorpb.FieldDescriptorProto
)

const typeEnum = descriptorpb.FieldDescriptorProto_TYPE_ENUM

func ptr[T any](v T) *T { return &v }

func protoName(s string) protoreflect.FullName { return protoreflect.FullName(s) }
