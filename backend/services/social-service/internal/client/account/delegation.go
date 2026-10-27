package account

import (
	"context"
	"fmt"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

// One *Client satisfies both interfaces, which is what lets internal/client/di.go
// bind them to a single constructor and therefore a single grpc.ClientConn.
// Asserted here because fx is lazy: with no consumer wired yet, a broken binding
// would not surface until the first component asks for DelegationAuditClient.
var (
	_ usecase.AccountClient         = (*Client)(nil)
	_ usecase.DelegationAuditClient = (*Client)(nil)
)

// delegationAuditTimeout bounds both audit calls. They sit on the critical path
// of a delegated write, so an unbounded wait would hold the write open with no
// upper limit; §29 also requires every call inside a scheduler lease to have a
// deadline, or the lease's lower bound cannot be established at all.
const delegationAuditTimeout = 5 * time.Second

// RecordAuditIntent writes the PENDING audit row that must exist before the
// domain write happens (56 §7.2, 支柱四).
//
// Unlike the read-path account lookups in client.go, this one does NOT degrade
// gracefully: a failed intent means the write must not proceed. Swallowing the
// error would produce exactly the outcome the two-phase design exists to
// prevent — a delegated mutation with no audit row and no signal.
func (c *Client) RecordAuditIntent(ctx context.Context, in usecase.DelegationAuditIntent) (*usecase.DelegationAuditIntentResult, error) {
	ctx, cancel := context.WithTimeout(ctx, delegationAuditTimeout)
	defer cancel()

	resp, err := c.delegationSvc.RecordAuditIntent(ctx, &pb.RecordAuditIntentRequest{
		SessionId:             in.SessionID,
		Capability:            in.Capability,
		Action:                in.Action,
		ResourceType:          in.ResourceType,
		ResourceId:            in.ResourceID,
		ResourceName:          in.ResourceName,
		OperationReason:       in.OperationReason,
		ResourceVersionBefore: in.ResourceVersionBefore,
		BeforeSnapshotJson:    in.BeforeSnapshotJSON,
		IdempotencyKey:        in.IdempotencyKey,
		RequestId:             in.RequestID,
		CorrelationId:         in.CorrelationID,
		UserAgent:             in.UserAgent,
		PayloadHash:           in.PayloadHash,
		NotifySeller:          in.NotifySeller,
		Origin:                in.Origin,
	})
	if err != nil {
		return nil, fmt.Errorf("record audit intent: %w", err)
	}
	if resp.GetCode() != 0 {
		// 業務碼要能被 errors.As 取回，不能只留在字串裡：排程 worker 必須把 9271
		// （工作階段已被撤銷）與其他拒絕分開處理——前者是終態取消，後者是退避重試。
		// 靠 strings.Contains 去認 code 的話，訊息一改分支就靜默失效。
		return nil, fmt.Errorf("record audit intent rejected: %w",
			pkgerrors.NewBusiness(resp.GetCode(), resp.GetMessage()))
	}
	data := resp.GetData()
	if data == nil || data.GetAuditLogId() == "" {
		// A zero code with no audit_log_id would otherwise flow on as an empty
		// string, and an empty audit_log_id is exactly what S1b rejects on the
		// next hop — far from where the fault actually is.
		return nil, fmt.Errorf("record audit intent returned no audit_log_id")
	}
	return &usecase.DelegationAuditIntentResult{
		AuditLogID: data.GetAuditLogId(),
		Replayed:   data.GetReplayed(),
		Result:     data.GetResult(),
	}, nil
}

// ConfirmAuditResult closes the audit row opened by RecordAuditIntent, in both
// the success and the failure case — a row left PENDING is indistinguishable
// from a crash mid-write.
func (c *Client) ConfirmAuditResult(ctx context.Context, in usecase.DelegationAuditConfirm) error {
	ctx, cancel := context.WithTimeout(ctx, delegationAuditTimeout)
	defer cancel()

	changed := make([]*pb.ChangedField, 0, len(in.ChangedFields))
	for _, f := range in.ChangedFields {
		changed = append(changed, &pb.ChangedField{
			Field:  f.Field,
			Label:  f.Label,
			Before: f.Before,
			After:  f.After,
		})
	}

	resp, err := c.delegationSvc.ConfirmAuditResult(ctx, &pb.ConfirmAuditResultRequest{
		AuditLogId:           in.AuditLogID,
		Result:               in.Result,
		BizCode:              in.BizCode,
		ResourceId:           in.ResourceID,
		ResourceVersionAfter: in.ResourceVersionAfter,
		AfterSnapshotJson:    in.AfterSnapshotJSON,
		ChangedFields:        changed,
	})
	if err != nil {
		return fmt.Errorf("confirm audit result: %w", err)
	}
	if resp.GetCode() != 0 {
		return fmt.Errorf("confirm audit result rejected: %w",
			pkgerrors.NewBusiness(resp.GetCode(), resp.GetMessage()))
	}
	return nil
}
