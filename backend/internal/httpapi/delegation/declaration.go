package delegation

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/delegation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/delegation/v1alpha1/delegationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// declarations 是 DelegationDelegatedService 每個方法需要的代行能力（API 設計規範（Connect）§二的 Delegated），key 是生成的 procedure 常數。
// 方法與宣告必須一一對應（checkDeclarations 在 NewInterceptor 時檢查，缺了或多了都不能啟動）。interceptor 只讓 session 帶有該能力的請求進 handler；
// use case 在 delegation 模組內以同一組常數再檢查一次（模組的其他呼叫端不能繞過）。沒有豁免：這個 service 的每個方法都以代行 session 為憑證。
var declarations = map[string]moddelegation.Capability{
	delegationv1alpha1connect.DelegationDelegatedServiceListPostsProcedure:     moddelegation.CapabilityPostsRead,
	delegationv1alpha1connect.DelegationDelegatedServiceGetPostProcedure:       moddelegation.CapabilityPostsRead,
	delegationv1alpha1connect.DelegationDelegatedServiceCreatePostProcedure:    moddelegation.CapabilityPostsWrite,
	delegationv1alpha1connect.DelegationDelegatedServiceUpdatePostProcedure:    moddelegation.CapabilityPostsWrite,
	delegationv1alpha1connect.DelegationDelegatedServicePublishPostProcedure:   moddelegation.CapabilityPostsPublish,
	delegationv1alpha1connect.DelegationDelegatedServiceUnpublishPostProcedure: moddelegation.CapabilityPostsPublish,
	delegationv1alpha1connect.DelegationDelegatedServiceSchedulePostProcedure:  moddelegation.CapabilityPostsPublish,
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_delegation_v1alpha1_delegation_proto.Services().ByName("DelegationDelegatedService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告是代行的能力。
func checkDeclarations(table map[string]moddelegation.Capability, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/delegation", table, []protoreflect.ServiceDescriptor{svc}, func(c moddelegation.Capability) string {
		if !c.Valid() {
			return "宣告了不存在的能力 " + string(c)
		}
		return ""
	})
}
