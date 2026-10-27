package httpapi

import (
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	_ "google.golang.org/protobuf/types/known/emptypb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
)

// 實際掛載的 admin service 與宣告表一一對應：connectServices 啟動時比對，Procedures 的數量就是掛載的方法數。
func TestMountedAdminServicesMatchTheDeclarations(t *testing.T) {
	services, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	descriptors, err := adminServiceDescriptors(services)
	if err != nil {
		t.Fatal(err)
	}
	methods := 0
	for _, d := range descriptors {
		methods += d.Methods().Len()
	}
	if len(descriptors) != 7 || methods != len(adminauth.Procedures()) {
		t.Fatalf("掛載 %d 個 admin service、%d 支方法；宣告 %d 支", len(descriptors), methods, len(adminauth.Procedures()))
	}
}

// 掛上一個宣告表沒有它的方法的 admin service，connectServices 的比對必須失敗（新增第 7 個 AdminService 沒補宣告的情境）。
func TestMountedAdminServiceWithoutDeclarationsIsRejected(t *testing.T) {
	const pkg = "hawkr.admin.fakeundeclared.v1alpha1"
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("hawkr/admin/fakeundeclared/v1alpha1/fake.proto"),
		Package:    proto.String(pkg),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/empty.proto"},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("FakeAdminService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("DoThing"),
				InputType:  proto.String(".google.protobuf.Empty"),
				OutputType: proto.String(".google.protobuf.Empty"),
			}},
		}},
	}, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
		t.Fatal(err)
	}

	real, err := connectServices(Deps{Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	mounted := append(slices.Clone(real), service{path: "/" + pkg + ".FakeAdminService/", handler: http.NotFoundHandler()})
	err = checkAdminDeclarations(mounted)
	if err == nil || !strings.Contains(err.Error(), "/"+pkg+".FakeAdminService/DoThing 沒有授權宣告") {
		t.Fatalf("掛了沒有宣告的 admin service：err=%v", err)
	}

	// 沒有 descriptor 的 admin service 也不能啟動。
	err = checkAdminDeclarations([]service{{path: "/hawkr.admin.nodescriptor.v1alpha1.NoneAdminService/", handler: http.NotFoundHandler()}})
	if err == nil || !strings.Contains(err.Error(), "找不到 admin service") {
		t.Fatalf("沒有 descriptor：err=%v", err)
	}

	// 宣告了但沒有掛載的方法（少掛一個 service）同樣失敗。
	if err := checkAdminDeclarations(real[:1]); err == nil {
		t.Fatal("少掛 admin service 卻沒有失敗")
	}
}
