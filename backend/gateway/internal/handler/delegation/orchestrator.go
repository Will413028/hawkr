package delegation

import (
	"context"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	"go.uber.org/zap"
)

// auditResultPending is the audit row's open state. It is the value the delegation module
// echoes back on a replayed intent (`api/proto/account/delegation.proto:422`) and the
// one the pending-rows gauge counts, so gateway must compare against the same literal
// rather than "no result".
const auditResultPending = "PENDING"

// AuditClient is the slice of the account gRPC client the orchestrator needs. An
// interface so handler tests can drive the whole three-step sequence without a live
// the delegation module — and so the tests can assert what was SENT, which is the half that
// actually breaks (notify_seller travelling as false is invisible everywhere else).
type AuditClient interface {
	RecordAuditIntent(ctx context.Context, req *accountpb.RecordAuditIntentRequest, deadlineMs int) (*accountpb.RecordAuditIntentResponse, error)
	ConfirmAuditResult(ctx context.Context, req *accountpb.ConfirmAuditResultRequest, deadlineMs int) (*accountpb.ConfirmAuditResultResponse, error)
	RecordAuditIntentBatch(ctx context.Context, req *accountpb.RecordAuditIntentBatchRequest, deadlineMs int) (*accountpb.RecordAuditIntentBatchResponse, error)
	ConfirmAuditResultBatch(ctx context.Context, req *accountpb.ConfirmAuditResultBatchRequest, deadlineMs int) (*accountpb.ConfirmAuditResultBatchResponse, error)
}

// Orchestrator owns the only implementation of the three-step sequence
// (RecordAuditIntent → domain write → ConfirmAuditResult). Thirteen endpoints route
// through it. Letting each of them hand-copy the sequence guarantees that one day one
// of them skips the confirm, or records the intent AFTER the domain write — and the
// audit table looks perfectly healthy in both cases.
type Orchestrator struct {
	Audit AuditClient
	Cfg   config.DelegationConfig
}

// Denial is a business refusal carrying the numeric code the front end switches on.
// Message never reaches the browser (§6.1.0 replaces it client-side); it exists for
// logs and for whoever reads a raw response.
type Denial struct {
	Code    int32
	Message string
}

func Deny(code int32, message string) *Denial { return &Denial{Code: code, Message: message} }

// Resource is one snapshot of a delegated resource: what the audit row needs (id,
// name, version, projected fields) plus the DTO the handler will return.
type Resource struct {
	ID       string
	Name     string
	ImageKey string
	Version  string
	// Fields is the domain DTO already normalised into scalars. Project() keeps only
	// the registered ones.
	Fields map[string]any
	// Payload is the handler's own response DTO. The orchestrator never inspects it.
	Payload any
}

// Spec describes one delegated mutation.
//
// ReadCap and WriteCap are two fields, not one. The before-snapshot read happens
// BEFORE the audit intent exists, so its DelegationContext necessarily carries an
// empty audit_log_id — and S1b requires a non-empty audit_log_id for every .write /
// .publish capability. Sending the write capability on that hop makes the domain
// service reject it with 9260 and the endpoint is completely unusable, while every
// unit test that only feeds write fixtures stays green.
type Spec struct {
	Session      *accountpb.OperationSessionSummary
	Action       string
	ResourceType string
	ReadCap      authz.AdminCapability
	WriteCap     authz.AdminCapability

	OperationReason string
	// NotifySeller must be filled explicitly on every write endpoint, including when
	// the request body omitted it (then: true). proto3 cannot distinguish "unset"
	// from false, so a forgotten flag means no outbox row, no seller notification —
	// and an audit table with every row present and no error anywhere.
	NotifySeller   bool
	IdempotencyKey string
	PayloadHash    string
	Origin         string

	RequestID     string
	CorrelationID string
	UserAgent     string

	// ResourceID is known up-front for updates and empty for creates (confirm fills
	// it in afterwards).
	ResourceID string

	// ResourceName is the display name the audit row carries when there is no
	// pre-image to take it from. ReadBefore's name wins whenever it exists — that
	// one is the resource's own name at the moment of the operation.
	//
	// 訂單面必須填它：`admin_operation_audit_logs.resource_id` 是 bigint 而訂單編號
	// 是 `O` ＋ 9 位序號，delegation 模組對它 ParseInt 失敗後存 NULL（§11 ADR）。
	// 兩欄都空的稽核列指不回任何一筆訂單，而 PII 揭露正是 risk_level='high' 的那一群。
	ResourceName string

	// SnapshotFields is the set of projected fields this mutation can actually touch.
	// nil means the whole rule table, which is only correct when the Write hop returns
	// a COMPLETE resource projection.
	//
	// 它只收窄 **Diff 的比較範圍**，不收窄 Project：before_snapshot 仍存完整白名單投
	// 影，爭議時才答得出「上架當下這件商品長什麼樣」。收窄的理由見 Diff 的註解——狀態
	// 切換類端點的寫入回應是命令回執而不是資源全貌，兩側比較範圍不同時，缺席的欄位會
	// 被記成「被清空」（dev 實測：換一次 logo 記了 10 列，9 列是假的）。
	//
	// 新增第 14 支端點時這一欄**必須填**（要全表就明寫 nil）：漏填等於選了全表，而選錯
	// 的後果是稽核列虛報從未發生的變更 —— 沒有任何錯誤、沒有任何測試會告訴你。
	// TestDelegatedOrchestrateCallSites_AllDeclareSnapshotFields 就是那道機械閘門。
	SnapshotFields []string

	// SnapshotOverride replaces BOTH snapshots with a fixed object. Exactly one user:
	// ORDER_PII_REVEAL, whose snapshots must record {"revealedFields":[…]} and never
	// the PII itself. It cannot go through Project — the order rule table is empty by
	// design (nothing was changed), so every field would be dropped.
	SnapshotOverride map[string]any

	// ReadBefore fetches the pre-image. nil for create-type actions, which have no
	// before state — not "an empty one".
	ReadBefore func(ctx context.Context, dctx *commonpb.DelegationContext) (*Resource, *Denial)
	// Write performs the domain mutation with the audit-carrying context.
	Write func(ctx context.Context, dctx *commonpb.DelegationContext) (*Resource, *Denial)
}

// Result is what a successful mutation reports back to the handler.
type Result struct {
	AuditLogID string
	Version    string
	Resource   any
}

// Orchestrate runs one delegated mutation end to end.
func (o *Orchestrator) Orchestrate(ctx context.Context, spec Spec) (*Result, *Denial) {
	if o == nil || o.Audit == nil {
		// An unwired audit dependency must never read as "authorized to write":
		// fail closed, exactly as a failed intent does.
		//
		// 9289 rather than 9280 (#2744): unwired means this build never connected the
		// dependency, so "try again shortly" is a lie that costs the operator an
		// unbounded number of retries and hides the fault from whoever is on call.
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Audit"), zap.String("op", "Orchestrate"))
		return nil, Deny(bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
	}

	var before *Resource
	if spec.ReadBefore != nil {
		res, denial := spec.ReadBefore(ctx, o.Context(spec, spec.ReadCap, ""))
		if denial != nil {
			// Q4 (resource does not belong to the target store) is a security
			// signal, not a plain error: it is the only trace an attempted
			// cross-store read leaves behind.
			if denial.Code == bizcode.DelegationResourceNotOwned {
				o.RecordReadDenial(ctx, ReadDenial{
					Session:       spec.Session,
					ResourceType:  spec.ResourceType,
					ResourceID:    spec.ResourceID,
					BizCode:       denial.Code,
					RequestID:     spec.RequestID,
					CorrelationID: spec.CorrelationID,
					UserAgent:     spec.UserAgent,
				})
			}
			return nil, denial
		}
		before = res
	}
	if before == nil {
		before = &Resource{}
	}

	resourceID := spec.ResourceID
	if resourceID == "" {
		resourceID = before.ID
	}
	resourceName := before.Name
	if resourceName == "" {
		resourceName = spec.ResourceName
	}
	beforeProjection := Project(spec.ResourceType, before.Fields)
	if spec.SnapshotOverride != nil {
		beforeProjection = spec.SnapshotOverride
	}
	beforeSnapshot, oversize := MarshalSnapshot(beforeProjection)
	if oversize {
		zap.L().Warn("DelegationDiff.project.truncated",
			zap.String("resourceType", spec.ResourceType), zap.String("phase", "before"))
	}

	intent, err := o.Audit.RecordAuditIntent(ctx, &accountpb.RecordAuditIntentRequest{
		SessionId:             spec.Session.GetSessionId(),
		Capability:            string(spec.WriteCap),
		Action:                spec.Action,
		ResourceType:          spec.ResourceType,
		ResourceId:            resourceID,
		ResourceName:          resourceName,
		OperationReason:       spec.OperationReason,
		ResourceVersionBefore: before.Version,
		BeforeSnapshotJson:    beforeSnapshot,
		IdempotencyKey:        spec.IdempotencyKey,
		RequestId:             spec.RequestID,
		CorrelationId:         spec.CorrelationID,
		UserAgent:             spec.UserAgent,
		PayloadHash:           spec.PayloadHash,
		NotifySeller:          spec.NotifySeller,
		Origin:                spec.origin(),
	}, o.Cfg.GrpcDeadlineAuditIntentMs)
	if err != nil {
		return nil, Deny(bizcode.DelegationAuditIntentFailed, "稽核紀錄寫入失敗，操作未執行")
	}
	if intent.GetCode() != 0 {
		return nil, Deny(intent.GetCode(), intent.GetMessage())
	}
	auditLogID := intent.GetData().GetAuditLogId()
	if intent.GetData().GetReplayed() {
		// Same key, same payload: the write already happened. Re-running the domain
		// mutation would apply it twice, and the version check would then fail on a
		// request the caller legitimately retried.
		//
		// If that earlier attempt never got its confirm through, the row is still
		// PENDING and every subsequent retry lands here — the retry that was supposed
		// to heal it is precisely the path that skips the healing. Close it now: the
		// snapshot we just read IS the post-write state, since the write is what made
		// this a replay. ChangedFields stays empty because the pre-write snapshot is
		// gone; an empty diff on a SUCCEEDED row is honest, a row stuck on PENDING is
		// not (it claims we do not know whether the write landed, and we do).
		if intent.GetData().GetResult() == auditResultPending {
			o.confirm(ctx, &accountpb.ConfirmAuditResultRequest{
				AuditLogId:           auditLogID,
				Result:               "SUCCEEDED",
				BizCode:              0,
				ResourceId:           resourceID,
				ResourceVersionAfter: before.Version,
				AfterSnapshotJson:    beforeSnapshot,
			})
		}
		return &Result{AuditLogID: auditLogID, Version: before.Version, Resource: before.Payload}, nil
	}

	after, denial := spec.Write(ctx, o.Context(spec, spec.WriteCap, auditLogID))
	if denial != nil {
		o.confirm(ctx, &accountpb.ConfirmAuditResultRequest{
			AuditLogId: auditLogID,
			Result:     "FAILED",
			BizCode:    denial.Code,
			ResourceId: resourceID,
		})
		return nil, denial
	}
	if after == nil {
		after = &Resource{}
	}

	afterProjection := Project(spec.ResourceType, after.Fields)
	if spec.SnapshotOverride != nil {
		afterProjection = spec.SnapshotOverride
	}
	afterSnapshot, oversizeAfter := MarshalSnapshot(afterProjection)
	if oversizeAfter {
		zap.L().Warn("DelegationDiff.project.truncated",
			zap.String("resourceType", spec.ResourceType), zap.String("phase", "after"))
	}
	confirmedResourceID := after.ID
	if confirmedResourceID == "" {
		confirmedResourceID = resourceID
	}

	o.confirm(ctx, &accountpb.ConfirmAuditResultRequest{
		AuditLogId:           auditLogID,
		Result:               "SUCCEEDED",
		BizCode:              0,
		ResourceId:           confirmedResourceID,
		ResourceVersionAfter: after.Version,
		AfterSnapshotJson:    afterSnapshot,
		ChangedFields:        Diff(spec.ResourceType, beforeProjection, afterProjection, spec.SnapshotFields),
	})

	return &Result{AuditLogID: auditLogID, Version: after.Version, Resource: after.Payload}, nil
}

// confirm closes an audit row. A failed confirm leaves the row PENDING, which is a
// legitimate terminal state the UI shows as 「結果未確認」 — it must not be reported
// to the caller as a failed write, because the write itself did happen.
//
// The context is detached from the caller's: by the time we get here the domain write
// has already landed, so the audit row must close even if the admin closed the tab a
// millisecond earlier. Keeping the request context would cancel this RPC on exactly
// the requests whose outcome is least obvious from the outside — the write is durable,
// the audit row says PENDING forever, and nothing in the response hints at it. The
// deadline still applies (ConfirmAuditResult applies GrpcDeadlineAuditConfirmMs);
// WithoutCancel drops the cancellation, not the timeout, and keeps trace values.
func (o *Orchestrator) confirm(ctx context.Context, req *accountpb.ConfirmAuditResultRequest) {
	ctx = context.WithoutCancel(ctx)
	resp, err := o.Audit.ConfirmAuditResult(ctx, req, o.Cfg.GrpcDeadlineAuditConfirmMs)
	if err != nil {
		zap.L().Warn("DelegationAudit.confirm.failed",
			zap.String("auditLogId", req.GetAuditLogId()), zap.Error(err))
		return
	}
	if resp.GetCode() != 0 {
		zap.L().Warn("DelegationAudit.confirm.rejected",
			zap.String("auditLogId", req.GetAuditLogId()), zap.Int32("code", resp.GetCode()))
	}
}

// Context builds the DelegationContext for one hop.
func (o *Orchestrator) Context(spec Spec, cap authz.AdminCapability, auditLogID string) *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       spec.Session.GetSessionId(),
		OperatorAdminId: spec.Session.GetActor().GetAdminId(),
		Capability:      string(cap),
		OperationReason: spec.OperationReason,
		AuditLogId:      auditLogID,
		IdempotencyKey:  spec.IdempotencyKey,
	}
}

func (s Spec) origin() string {
	if s.Origin == "" {
		return "interactive"
	}
	return s.Origin
}

// readDenialAction maps a resource face to its READ denial action. Those four actions
// exist for exactly one purpose: leaving a trace when someone tried to read across
// stores. Losing that trace is the failure mode this function guards.
func readDenialAction(resourceType string) string {
	switch resourceType {
	case "storefront":
		return "STOREFRONT_READ"
	case "product":
		return "PRODUCT_READ"
	case "post":
		return "POST_READ"
	case "order":
		return "ORDER_READ"
	default:
		return ""
	}
}

// ReadDenialCapability maps a resource face to the read capability recorded on a
// denial row. The row records what was ATTEMPTED, which is always the read.
func ReadDenialCapability(resourceType string) authz.AdminCapability {
	switch resourceType {
	case "storefront":
		return authz.AdminCapStorefrontRead
	case "product":
		return authz.AdminCapProductsRead
	case "post":
		return authz.AdminCapPostsRead
	case "order":
		return authz.AdminCapOrdersRead
	default:
		return ""
	}
}

// ReadDenial is one refused delegated read: which face, which resource, under which
// capability, and the code it was refused with.
//
// It is a struct rather than a parameter list because the transport columns
// (request_id / correlation_id / user_agent) are exactly the ones a call
// site forgets — and forgetting them is silent: the row lands, every constraint
// passes, and the device field is lost from the denial row.
type ReadDenial struct {
	Session      *accountpb.OperationSessionSummary
	ResourceType string
	ResourceID   string

	// Capability is what was ATTEMPTED and refused, which on the guard's two codes is
	// the capability the ROUTE declared — 「請求了一個不被授予的 capability」這件事
	// 本身正是那一列要記的（§7.1.3）。Recording the face's read token instead would
	// under-report a refused write as a refused browse. Empty falls back to the read
	// token, which is right for Q4: there the read capability was held and it was the
	// resource that did not belong.
	Capability authz.AdminCapability
	BizCode    int32

	RequestID     string
	CorrelationID string
	UserAgent     string
}

// RecordReadDenial writes the FAILED audit row for a refused delegated read (Q2
// target mismatch / Q3 capability denied / Q4 resource not owned).
//
// Four things are locked together here rather than left to each call site:
// notify_seller = false, result = FAILED, both snapshots empty, operation_reason
// empty. notify_seller in particular: today W5 only builds outbox rows for SUCCEEDED,
// so true would be harmless — but the day someone widens that, these rows would start
// pushing "somebody tried to read your data across stores" to a third party, and that
// signal must never travel down the seller notification channel.
//
// The row is written as intent-then-confirm because those are the only two RPCs that
// exist; the pair is issued back to back and the row's terminal state is FAILED
// either way. BizCode is limited to 9262 / 9263 / 9275 — the four session-level
// refusals (9270 / 9271 / 9272 / 9265) never reach here, since a terminated session
// cannot write an intent at all.
func (o *Orchestrator) RecordReadDenial(ctx context.Context, d ReadDenial) {
	if o == nil || o.Audit == nil || d.Session == nil {
		// 這一支不回錯（讀取拒絕的稽核是 best-effort），但未接線一樣要留痕。
		if o == nil || o.Audit == nil {
			zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Audit"), zap.String("op", "RecordReadDenial"))
		}
		return
	}
	action := readDenialAction(d.ResourceType)
	if action == "" {
		return
	}
	capability := d.Capability
	if capability == "" {
		capability = ReadDenialCapability(d.ResourceType)
	}
	intent, err := o.Audit.RecordAuditIntent(ctx, &accountpb.RecordAuditIntentRequest{
		SessionId:       d.Session.GetSessionId(),
		Capability:      string(capability),
		Action:          action,
		ResourceType:    d.ResourceType,
		ResourceId:      d.ResourceID,
		OperationReason: "",
		NotifySeller:    false,
		Origin:          "interactive",
		RequestId:       d.RequestID,
		CorrelationId:   d.CorrelationID,
		UserAgent:       d.UserAgent,
	}, o.Cfg.GrpcDeadlineAuditIntentMs)
	if err != nil || intent.GetCode() != 0 {
		zap.L().Warn("DelegationAudit.readDenial.intentFailed",
			zap.String("resourceType", d.ResourceType), zap.Int32("bizCode", d.BizCode))
		return
	}
	o.confirm(ctx, &accountpb.ConfirmAuditResultRequest{
		AuditLogId: intent.GetData().GetAuditLogId(),
		Result:     "FAILED",
		BizCode:    d.BizCode,
		ResourceId: d.ResourceID,
	})
}
