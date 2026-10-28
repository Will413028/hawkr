package delegation

import (
	"context"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	"go.uber.org/zap"
)

// BatchItem is one requested element of a batch mutation.
type BatchItem struct {
	ResourceID      string
	ExpectedVersion string
}

// BatchWriteItem is what the domain write hop receives per element: the audit row id
// is filled in by the Gateway after RecordAuditIntentBatch, because the domain service
// cannot know it (the audit rows live in the delegation module) and yet has to echo it back
// so the front end can reconcile each element with its own audit entry.
type BatchWriteItem struct {
	ResourceID      string
	ExpectedVersion string
	AuditLogID      string
}

// BatchWriteResult is the domain service's per-element outcome.
type BatchWriteResult struct {
	ResourceID string
	OK         bool
	Code       int32
	Version    string
	// Fields is the post-write projection input for this element (empty when the
	// element failed).
	Fields map[string]any
}

// BatchResultItem is what the handler renders per element.
type BatchResultItem struct {
	ResourceID string
	OK         bool
	Code       int32
	Version    string
	AuditLogID string
}

// BatchSpec describes an N-element delegated mutation.
//
// The shape is N independent delegated writes carried by O(1) batch RPCs — six hops
// regardless of N. The per-element shape (1+4N, i.e. 201 hops at N=50) is functionally
// identical and only shows up as RPC count, which is exactly why the stress test has
// to assert the count rather than the outcome. prod, the delegation module runs a connection
// pool of 2; 100 short transactions against it spill latency onto unrelated requests.
type BatchSpec struct {
	Session      *accountpb.OperationSessionSummary
	Action       string
	ResourceType string
	ReadCap      authz.AdminCapability
	WriteCap     authz.AdminCapability

	OperationReason string
	// NotifySeller is one flag for the whole batch, mirroring operationReason. It is
	// then written into EVERY one of the N intent items: notify_seller hangs off each
	// item, and proto3 makes "unset" indistinguishable from false, so the items that
	// miss it build no outbox row at all while their audit rows look complete.
	NotifySeller   bool
	IdempotencyKey string
	PayloadHash    string

	// SnapshotFields is the per-element comparison scope. Semantics, rationale and the
	// "must be filled explicitly" rule are identical to Spec.SnapshotFields.
	//
	// 批次的形狀讓它比單筆更需要這一欄：AfterFields 是**手寫的**衍生投影（批次狀態變更
	// 只回新版本，沒有資源），手寫的那份必然只有一兩個鍵，而 before 來自真實讀取。
	SnapshotFields []string

	RequestID     string
	CorrelationID string
	UserAgent     string

	Items []BatchItem

	// ReadBatch fetches the pre-images, keyed by resource id. Runs before the intents
	// exist, therefore with ReadCap and an empty audit_log_id.
	ReadBatch func(ctx context.Context, dctx *commonpb.DelegationContext) (map[string]*Resource, *Denial)
	// WriteBatch performs the domain mutation for the elements whose intent succeeded.
	WriteBatch func(ctx context.Context, dctx *commonpb.DelegationContext, items []BatchWriteItem) ([]BatchWriteResult, *Denial)
	// AfterFields derives the post-write projection for a successful element when the
	// domain service does not return one (batch status changes only report the new
	// version). before is that element's pre-image projection input.
	AfterFields func(before map[string]any) map[string]any
}

// OrchestrateBatch runs the six-hop batch sequence. A Denial return means the WHOLE
// batch was refused before any element ran — that is what 9276 is for. Partial
// failure is never a Denial: it comes back as code 0 with per-element results, because
// the front end discards data whenever the envelope code is non-zero and the operator
// would lose every auditLogId along with it.
func (o *Orchestrator) OrchestrateBatch(ctx context.Context, spec BatchSpec) ([]BatchResultItem, *Denial) {
	// 9289 而不是 9280（#2744）：與 Orchestrate 同一個理由，未接線是永久失敗。
	if o == nil || o.Audit == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Audit"), zap.String("op", "OrchestrateBatch"))
		return nil, Deny(bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
	}

	befores := map[string]*Resource{}
	if spec.ReadBatch != nil {
		res, denial := spec.ReadBatch(ctx, o.batchContext(spec, spec.ReadCap, ""))
		if denial != nil {
			return nil, denial
		}
		befores = res
	}

	intentReq := &accountpb.RecordAuditIntentBatchRequest{
		Items: make([]*accountpb.RecordAuditIntentRequest, 0, len(spec.Items)),
	}
	beforeProjections := make([]map[string]any, len(spec.Items))
	for i, item := range spec.Items {
		before := befores[item.ResourceID]
		if before == nil {
			before = &Resource{}
		}
		beforeProjections[i] = Project(spec.ResourceType, before.Fields)
		snapshot, oversize := MarshalSnapshot(beforeProjections[i])
		if oversize {
			zap.L().Warn("DelegationDiff.project.truncated",
				zap.String("resourceType", spec.ResourceType), zap.String("phase", "before"))
		}
		intentReq.Items = append(intentReq.Items, &accountpb.RecordAuditIntentRequest{
			SessionId:             spec.Session.GetSessionId(),
			Capability:            string(spec.WriteCap),
			Action:                spec.Action,
			ResourceType:          spec.ResourceType,
			ResourceId:            item.ResourceID,
			ResourceName:          before.Name,
			OperationReason:       spec.OperationReason,
			ResourceVersionBefore: before.Version,
			BeforeSnapshotJson:    snapshot,
			// Per-element key: one header value must not collide across the N rows
			// of one batch, or the uniqueness constraint would collapse them.
			IdempotencyKey: spec.IdempotencyKey + ":" + item.ResourceID,
			RequestId:      spec.RequestID,
			CorrelationId:  spec.CorrelationID,
			UserAgent:      spec.UserAgent,
			PayloadHash:    spec.PayloadHash,
			NotifySeller:   spec.NotifySeller,
			Origin:         "interactive",
		})
	}

	intentResp, err := o.Audit.RecordAuditIntentBatch(ctx, intentReq, o.Cfg.GrpcDeadlineBatchAuditIntentMs)
	if err != nil {
		return nil, Deny(bizcode.DelegationAuditIntentFailed, "稽核紀錄寫入失敗，操作未執行")
	}
	if intentResp.GetCode() != 0 {
		return nil, Deny(intentResp.GetCode(), intentResp.GetMessage())
	}

	results := make([]BatchResultItem, len(spec.Items))
	writeItems := make([]BatchWriteItem, 0, len(spec.Items))
	indexByResource := map[string]int{}
	for i, item := range spec.Items {
		results[i] = BatchResultItem{ResourceID: item.ResourceID}
		indexByResource[item.ResourceID] = i

		var itemResult *accountpb.RecordAuditIntentItemResult
		if i < len(intentResp.GetResults()) {
			itemResult = intentResp.GetResults()[i]
		}
		if itemResult == nil || !itemResult.GetOk() {
			// Per-element intent failure. The single-write path aborts entirely here;
			// the batch path must NOT — the other elements are unaffected and the
			// operator needs to know which one did not even get an audit row.
			results[i].OK = false
			results[i].Code = bizcode.DelegationAuditIntentFailed
			if before := befores[item.ResourceID]; before != nil {
				results[i].Version = before.Version
			}
			continue
		}
		results[i].AuditLogID = itemResult.GetData().GetAuditLogId()
		writeItems = append(writeItems, BatchWriteItem{
			ResourceID:      item.ResourceID,
			ExpectedVersion: item.ExpectedVersion,
			AuditLogID:      results[i].AuditLogID,
		})
	}

	if len(writeItems) == 0 {
		return results, nil
	}

	writeResults, denial := spec.WriteBatch(ctx, o.batchContext(spec, spec.WriteCap, ""), writeItems)
	if denial != nil {
		// The whole domain hop failed: every element that had an intent is closed as
		// FAILED so no row is left dangling in PENDING.
		confirms := make([]*accountpb.ConfirmAuditResultRequest, 0, len(writeItems))
		for _, wi := range writeItems {
			i := indexByResource[wi.ResourceID]
			results[i].OK = false
			results[i].Code = denial.Code
			confirms = append(confirms, &accountpb.ConfirmAuditResultRequest{
				AuditLogId: wi.AuditLogID,
				Result:     "FAILED",
				BizCode:    denial.Code,
				ResourceId: wi.ResourceID,
			})
		}
		o.confirmBatch(ctx, confirms)
		return results, nil
	}

	confirms := make([]*accountpb.ConfirmAuditResultRequest, 0, len(writeResults))
	// Which intents this loop actually closes. The loop walks writeResults, not
	// writeItems, so an element that got an intent but no matching result silently
	// falls out — see the sweep after the loop.
	confirmed := make(map[string]bool, len(writeResults))
	for _, wr := range writeResults {
		i, ok := indexByResource[wr.ResourceID]
		if !ok {
			continue
		}
		confirmed[wr.ResourceID] = true
		results[i].OK = wr.OK
		results[i].Code = wr.Code
		results[i].Version = wr.Version

		confirm := &accountpb.ConfirmAuditResultRequest{
			AuditLogId: results[i].AuditLogID,
			ResourceId: wr.ResourceID,
			BizCode:    wr.Code,
		}
		if wr.OK {
			afterFields := wr.Fields
			if afterFields == nil && spec.AfterFields != nil {
				afterFields = spec.AfterFields(befores[wr.ResourceID].fields())
			}
			afterProjection := Project(spec.ResourceType, afterFields)
			snapshot, oversize := MarshalSnapshot(afterProjection)
			if oversize {
				zap.L().Warn("DelegationDiff.project.truncated",
					zap.String("resourceType", spec.ResourceType), zap.String("phase", "after"))
			}
			confirm.Result = "SUCCEEDED"
			confirm.BizCode = 0
			confirm.ResourceVersionAfter = wr.Version
			confirm.AfterSnapshotJson = snapshot
			confirm.ChangedFields = Diff(spec.ResourceType, beforeProjections[i], afterProjection, spec.SnapshotFields)
		} else {
			confirm.Result = "FAILED"
		}
		confirms = append(confirms, confirm)
	}

	// Every intent we wrote must reach a terminal state, including the ones the domain
	// service did not report back on. Today the Java side loops per request item and so
	// returns 1:1, but nothing on this side enforces that: if it ever returns short, or
	// echoes a resourceId we did not send, those rows sit on PENDING with no log line
	// and no failed response — the batch looks entirely successful. Close them as
	// FAILED, which is what "we asked for this write and never learned whether it
	// happened" means for an audit trail.
	for _, wi := range writeItems {
		if confirmed[wi.ResourceID] {
			continue
		}
		i, ok := indexByResource[wi.ResourceID]
		if !ok {
			continue
		}
		zap.L().Warn("DelegationAudit.batch.resultMissing",
			zap.String("resourceType", spec.ResourceType), zap.String("resourceId", wi.ResourceID))
		results[i].OK = false
		if results[i].Code == 0 {
			results[i].Code = bizcode.DelegationUpstreamUnavailable
		}
		confirms = append(confirms, &accountpb.ConfirmAuditResultRequest{
			AuditLogId: results[i].AuditLogID,
			ResourceId: wi.ResourceID,
			Result:     "FAILED",
			BizCode:    results[i].Code,
		})
	}

	o.confirmBatch(ctx, confirms)

	return results, nil
}

// confirmBatch is all-or-nothing on the server side: either all N rows close or all N
// stay PENDING. That is this shape's single regression versus the per-element form and
// it must be logged as such, never swallowed.
func (o *Orchestrator) confirmBatch(ctx context.Context, items []*accountpb.ConfirmAuditResultRequest) {
	if len(items) == 0 {
		return
	}
	// Detached for the same reason as the single-write confirm: the domain writes have
	// already landed, so an admin closing the tab must not be what decides whether the
	// audit rows close. The deadline still applies.
	ctx = context.WithoutCancel(ctx)
	resp, err := o.Audit.ConfirmAuditResultBatch(ctx, &accountpb.ConfirmAuditResultBatchRequest{Items: items},
		o.Cfg.GrpcDeadlineBatchAuditConfirmMs)
	if err != nil {
		zap.L().Warn("DelegationAudit.confirmBatch.failed",
			zap.Int("items", len(items)), zap.Error(err))
		return
	}
	if resp.GetCode() != 0 {
		zap.L().Warn("DelegationAudit.confirmBatch.rejected",
			zap.Int("items", len(items)), zap.Int32("code", resp.GetCode()))
	}
}

func (o *Orchestrator) batchContext(spec BatchSpec, cap authz.AdminCapability, auditLogID string) *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       spec.Session.GetSessionId(),
		OperatorAdminId: spec.Session.GetActor().GetAdminId(),
		Capability:      string(cap),
		OperationReason: spec.OperationReason,
		AuditLogId:      auditLogID,
		IdempotencyKey:  spec.IdempotencyKey,
	}
}

func (r *Resource) fields() map[string]any {
	if r == nil {
		return nil
	}
	return r.Fields
}
