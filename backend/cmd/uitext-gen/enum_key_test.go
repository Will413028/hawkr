package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

func enumInPackage(t *testing.T, pkg string, numbers ...int32) *EnumInfo {
	t.Helper()
	e := &descriptorpb.EnumDescriptorProto{Name: proto.String("Outcome")}
	names := []string{"OUTCOME_UNSPECIFIED", "OUTCOME_PENDING", "OUTCOME_DONE"}
	for i, n := range names {
		e.Value = append(e.Value, &descriptorpb.EnumValueDescriptorProto{Name: proto.String(n), Number: proto.Int32(numbers[i])})
	}
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String(strings.ReplaceAll(pkg, ".", "/") + "/o.proto"), Package: proto.String(pkg), Syntax: proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{e},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &EnumInfo{Desc: fd.Enums().Get(0), Path: pkg + ".Outcome"}
}

func TestEnumNumberTableKeepsSameNamedValuesOfDifferentPackagesApart(t *testing.T) {
	a := enumInPackage(t, "p.a.v1", 0, 1, 2)
	b := enumInPackage(t, "p.b.v1", 0, 6, 7)
	got := (&Model{}).enumNumberTable([]*EnumInfo{a, b}, func(s string) string { return `"` + s + `"` })
	for _, want := range []string{`"p.a.v1.Outcome.OUTCOME_PENDING": 1`, `"p.b.v1.Outcome.OUTCOME_PENDING": 6`, `"p.b.v1.Outcome.OUTCOME_DONE": 7`} {
		if !strings.Contains(got, want) {
			t.Fatalf("缺 %s：\n%s", want, got)
		}
	}
	if strings.Count(got, "OUTCOME_PENDING") != 2 {
		t.Fatalf("兩個列舉的同名值應各有一筆：\n%s", got)
	}
}

func TestDuplicateEnumKeysFlagsAnEnumListedTwiceAndNotSameNamedValuesOfDifferentPackages(t *testing.T) {
	a := enumInPackage(t, "p.a.v1", 0, 1, 2)
	b := enumInPackage(t, "p.b.v1", 0, 6, 7)
	if got := duplicateEnumKeys([]*EnumInfo{a, b}); len(got) != 0 {
		t.Fatalf("不同 package 的同名值不衝突：%v", got)
	}
	if got := duplicateEnumKeys([]*EnumInfo{a, b, a}); len(got) != len(enumNames(a)) || !strings.HasPrefix(got[0], "p.a.v1.Outcome.") {
		t.Fatalf("同一個列舉列兩次，每個值都該重複：%v", got)
	}
}
