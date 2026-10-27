package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseKey(t *testing.T) {
	s, err := LoadSchema(fixtureImage())
	if err != nil {
		t.Fatal(err)
	}
	ok := []struct {
		key     string
		kind    KeyKind
		funcID  string
		value   string
		variant string
	}{
		{"fx.Color.COLOR_RED", KeyNoun, "fx.Color", "COLOR_RED", ""},
		{"fx.Box.Size.SIZE_SMALL", KeyNoun, "fx.Box.Size", "SIZE_SMALL", ""}, // 巢狀 enum
		{"fx.Color.label.COLOR_RED", KeyVariant, "fx.Color.label", "COLOR_RED", "label"},
		{"fx.Box.Size.label.SIZE_LARGE", KeyVariant, "fx.Box.Size.label", "SIZE_LARGE", "label"},
		{"fx.Box.title", KeySentence, "fx.Box.title", "", ""},
		{"admin.Role.ROLE_ROOT", KeyNoun, "admin.Role", "ROLE_ROOT", ""},
		{"admin.fx.Level.LEVEL_HIGH", KeyNoun, "admin.fx.Level", "LEVEL_HIGH", ""}, // admin.fx 優先於 admin
		{"common.unknown", KeyUnknown, "common.unknown", "", ""},
		{"common.error.NOT_FOUND", KeyError, "common.error", "NOT_FOUND", ""},
	}
	for _, c := range ok {
		k, err := s.ParseKey(c.key)
		if err != nil {
			t.Errorf("%s：%v", c.key, err)
			continue
		}
		if k.Kind != c.kind || k.FuncID != c.funcID || k.Value != c.value || k.Variant != c.variant {
			t.Errorf("%s：得到 %+v", c.key, k)
		}
	}
	bad := []struct{ key, want string }{
		{"nope.Color.COLOR_RED", "開頭不是任何 package"},
		{"fx.Missing.COLOR_RED", "沒有型別"},
		{"fx.Color.COLOR_PURPLE", "沒有值"},
		{"fx.Color.COLOR_UNSPECIFIED", "值號 0"},
		{"fx.Color", "缺少值名"},
		{"fx.Color.Label.COLOR_RED", "要是 lower_snake"},
		{"fx.Color.a.b.COLOR_RED", "多出"},
		{"fx.Box", "缺少句子名稱"},
		{"fx.Box.Title", "lower_snake"},
		{"fx.Box.a.b", "多出"},
		{"fx..x", "空的段"},
		{"common.error.CANCELED", "不顯示文字"},
		{"common.error.BOGUS", "不是 Connect 的 code"},
		{"fx", "package 之後沒有型別"},
	}
	for _, c := range bad {
		_, err := s.ParseKey(c.key)
		if err == nil {
			t.Errorf("%s：應該失敗", c.key)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s：錯誤 %q 不含 %q", c.key, err, c.want)
		}
	}
}

func TestSchemaRejectsSamePackageWithoutVersion(t *testing.T) {
	_, err := LoadSchema(fixtureImage(func(fds *descriptorSet) {
		fds.File = append(fds.File, &descriptorpb.FileDescriptorProto{
			Name:     ptr("hawkr/fx/v1/fx.proto"),
			Package:  ptr("hawkr.fx.v1"),
			Syntax:   ptr("proto3"),
			EnumType: []*descriptorpb.EnumDescriptorProto{enumProto("Other", "OTHER_", "X")},
		})
	}))
	if err == nil || !strings.Contains(err.Error(), "去掉版本後同名") {
		t.Fatalf("v1alpha1 與 v1 並存要失敗：%v", err)
	}
}

func TestServerOwnedAndShape(t *testing.T) {
	s, err := LoadSchema(fixtureImage())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hawkr.common.v1alpha1.ErrorReason", "hawkr.fx.v1alpha1.GadgetUnavailableReason"} {
		if !s.serverOwned[protoName(name)] {
			t.Errorf("%s 應該是伺服器擁有的", name)
		}
	}
	if s.serverOwned[protoName("hawkr.fx.v1alpha1.Color")] {
		t.Error("Color 不是伺服器擁有的")
	}
	if len(s.unavailableShapeErrors) != 0 {
		t.Errorf("reason 欄位是合法形狀：%v", s.unavailableShapeErrors)
	}
	// repeated XxxUnavailableReason 出現在別處 → 形狀違規。
	s2, err := LoadSchema(fixtureImage(func(fds *descriptorSet) {
		fds.File[0].MessageType = append(fds.File[0].MessageType, &messageProto{
			Name: ptr("Legacy"),
			Field: []*fieldProto{
				field("unavailable_reasons", 1, typeEnum, ".hawkr.fx.v1alpha1.GadgetUnavailableReason", true),
			},
		})
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.unavailableShapeErrors) != 1 || !strings.Contains(s2.unavailableShapeErrors[0], "Legacy.unavailable_reasons") {
		t.Errorf("形狀違規：%v", s2.unavailableShapeErrors)
	}
}
