package main

import (
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

func fixtureFiles(t *testing.T) []string {
	t.Helper()
	int64Field := func(field string, num int32) *descriptorpb.FieldDescriptorProto {
		return &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(field),
			Number: proto.Int32(num),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
		}
	}
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Req"), Field: []*descriptorpb.FieldDescriptorProto{int64Field("id", 1)}},
			{Name: proto.String("Nested"), Field: []*descriptorpb.FieldDescriptorProto{int64Field("inner_id", 1)}},
			{Name: proto.String("Resp"), Field: []*descriptorpb.FieldDescriptorProto{
				int64Field("account_id", 1),
				{Name: proto.String("code"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
				{Name: proto.String("name"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
				{Name: proto.String("nested"), Number: proto.Int32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".test.Nested")},
			}},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("S"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("Do"),
				InputType:  proto.String(".test.Req"),
				OutputType: proto.String(".test.Resp"),
			}},
		}},
	}
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}})
	if err != nil {
		t.Fatalf("build descriptors: %v", err)
	}
	vs := findViolations(files)
	keys := make([]string, len(vs))
	for i, v := range vs {
		keys[i] = v.key
	}
	sort.Strings(keys)
	return keys
}

func TestFindViolations(t *testing.T) {
	got := fixtureFiles(t)
	want := []string{"test.Nested.inner_id", "test.Resp.account_id"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
