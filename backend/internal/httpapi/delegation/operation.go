package delegation

import (
	"fmt"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// ParseOperation 解析每個代行寫入共同的稽核資料（所有 <模組>DelegatedService 共用）：request_id 必填（UUIDv4）；操作原因 1 到 100 字；notify_seller 不設是通知。
func ParseOperation(op *commonpb.DelegatedOperation) (moddelegation.DelegatedOperation, error) {
	if !op.ProtoReflect().IsValid() {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation", "operation 必填")
	}
	id, err := rpc.ParseRequestID(op.GetRequestId())
	if err != nil || id == nil {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation.request_id", "operation.request_id 必填，而且必須是 UUIDv4")
	}
	reason, ok := moddelegation.LegalOperationReason(op.GetOperationReason())
	if !ok {
		return moddelegation.DelegatedOperation{}, rpc.RequiredField("operation.operation_reason",
			fmt.Sprintf("operation.operation_reason 必填，去除首尾空白後 1 到 %d 個字元，不得含控制字元", moddelegation.OperationReasonMaxRunes))
	}
	return moddelegation.DelegatedOperation{RequestID: *id, Reason: reason, NotifySeller: op.NotifySeller == nil || op.GetNotifySeller()}, nil
}
