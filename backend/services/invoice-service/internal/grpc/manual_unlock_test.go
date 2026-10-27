package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 本檔驗端點 54／55／46 的 Layer 5：碼與 data 的對應、缺身分的 400、
// 以及「不存在」與「不屬於本 job」逐位元相同。

var muNow = time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)

const (
	muJobID     = int64(7001)
	muUnlockID  = int64(9100001)
	muRequester = int64(4201)
	muApprover  = int64(4202)
	muCanceller = int64(4203)
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type muOps struct {
	domain.OperationRepository
	op       domain.Operation
	notFound bool
	allowed  int
}

func (f *muOps) GetByID(context.Context, int64) (domain.Operation, error) {
	if f.notFound {
		return domain.Operation{}, pkgerrors.NewBusiness(bizcode.InvoiceJobNotFound,
			bizcode.Message(bizcode.InvoiceJobNotFound))
	}
	return f.op, nil
}

func (f *muOps) AllowManualProcess(_ context.Context, _, _, actor int64, _ time.Time) (bool, error) {
	f.allowed++
	if f.op.CancelRequestedBy != nil && *f.op.CancelRequestedBy == actor {
		return false, nil
	}
	f.op.Status = "MANUAL_PROCESS_ALLOWED"
	f.op.Version++
	return true, nil
}

type muUnlocks struct {
	domain.ManualUnlockRepository
	rows []domain.ManualUnlock
}

func (f *muUnlocks) ExpireStale(context.Context, int64, time.Time) (int64, error) { return 0, nil }

func (f *muUnlocks) Create(_ context.Context, in domain.NewManualUnlock) (domain.ManualUnlock, bool, error) {
	row := domain.ManualUnlock{
		ID: muUnlockID, OperationID: in.OperationID,
		Status:                domain.ManualUnlockAwaitingSecondApproval,
		RequestedByAccountID:  in.RequestedByAccountID,
		ProviderCheckEvidence: in.ProviderCheckEvidence,
		ProviderCheckedAt:     in.ProviderCheckedAt,
		RequestedAt:           in.Now, ExpiresAt: in.ExpiresAt,
	}
	f.rows = append(f.rows, row)
	return row, false, nil
}

func (f *muUnlocks) GetByID(_ context.Context, id int64) (domain.ManualUnlock, error) {
	for _, r := range f.rows {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.ManualUnlock{}, pkgerrors.NewBusiness(bizcode.InvoiceJobNotFound,
		bizcode.Message(bizcode.InvoiceJobNotFound))
}

func (f *muUnlocks) GetOpen(_ context.Context, operationID int64, now time.Time) (domain.ManualUnlock, bool, error) {
	for _, r := range f.rows {
		if r.OperationID == operationID && r.Status == domain.ManualUnlockAwaitingSecondApproval && now.Before(r.ExpiresAt) {
			return r, true, nil
		}
	}
	return domain.ManualUnlock{}, false, nil
}

func (f *muUnlocks) Decide(_ context.Context, in domain.DecideManualUnlock) (bool, error) {
	for i := range f.rows {
		r := &f.rows[i]
		if r.ID != in.ID || r.RequestedByAccountID == in.ApprovedByAccountID {
			continue
		}
		approver := in.ApprovedByAccountID
		r.Status = in.NextStatus
		r.ApprovedByAccountID = &approver
		decided := in.Now
		r.DecidedAt = &decided
		return true, nil
	}
	return false, nil
}

type muAudit struct{ domain.AuditRepository }

func (muAudit) AppendStatusHistory(context.Context, domain.NewStatusHistoryEntry) error { return nil }
func (muAudit) AppendAdminAuditLog(context.Context, domain.NewAdminAuditLogEntry) (int64, error) {
	return 77, nil
}

func muServer(op domain.Operation, unlocks *muUnlocks) (*InvoiceServer, *muOps) {
	ops := &muOps{op: op}
	// adjustments 傳 nil：本檔驗的是雙人解鎖（端點 54／55），那條路徑不碰 adjustment
	// ——advanceAdjustmentForAdmin 對 nil 直接 return（同 op.AdjustmentID 為 nil）。
	uc := usecase.NewAdminJob(nopTx{}, ops, nil, nil, muAudit{}, nil, unlocks, nil, nil,
		nil, usecase.IssuanceConfig{}, func() int64 { return muUnlockID }, zap.NewNop()).
		WithClock(func() time.Time { return muNow })
	return NewInvoiceServer(nil, nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop()), ops
}

func muCancelledOp() domain.Operation {
	canceller := muCanceller
	at := muNow.Add(-time.Hour)
	return domain.Operation{
		// OperationType 必填：DB 的 chk_invop_type 不接受空字串，而空字串會讓
		// 「只有開立類能解鎖」那道守衛在測試裡失明（#3112）。
		ID: muJobID, DocumentID: 8001, Status: "CANCELLED_BY_ADMIN", Version: 5,
		OperationType:     codemeta.InvoiceOperationTypeIssue.Value,
		CancelRequestedBy: &canceller, CancelRequestedAt: &at,
	}
}

func muRequestReq() *pb.RequestManualUnlockForAdminRequest {
	return &pb.RequestManualUnlockForAdminRequest{
		ActorAccountId: muRequester, JobId: "7001",
		ProviderCheckEvidence: "致電光貿客服確認未產生發票",
		ProviderCheckedAt:     "2026-08-30T05:00:00Z",
		Reason:                "AMEGO 逾時，已查證",
		ExpectedVersion:       "5",
		IdempotencyKey:        "idem-mu-1",
	}
}

// ---------------------------------------------------------------------------
// 端點 54
// ---------------------------------------------------------------------------

func TestRequestManualUnlock_Success(t *testing.T) {
	s, _ := muServer(muCancelledOp(), &muUnlocks{})

	resp, err := s.RequestManualUnlockForAdmin(context.Background(), muRequestReq())
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("成功應為 code 0，實得 %d（%s）", resp.GetCode(), resp.GetMessage())
	}
	d := resp.GetData()
	if d.GetUnlockId() != "9100001" {
		t.Fatalf("unlockId 不符：%q", d.GetUnlockId())
	}
	if d.GetStatus() != domain.ManualUnlockAwaitingSecondApproval {
		t.Fatalf("status 不符：%q", d.GetStatus())
	}
	// 24 小時窗口必須送到前端：58A 的倒數只有這一個來源。
	if d.GetExpiresAt() != "2026-08-31T06:00:00Z" {
		t.Fatalf("expiresAt 應為發起後 24 小時，實得 %q", d.GetExpiresAt())
	}
	if d.GetRequestedByAccountId() != "4201" {
		t.Fatalf("requestedBy 不符：%q", d.GetRequestedByAccountId())
	}
	// 尚未覆核 ⇒ approvedBy 必須是空字串（proto3 沒有 null）。
	if d.GetApprovedByAccountId() != "" {
		t.Fatalf("尚未覆核時 approvedBy 必須空，實得 %q", d.GetApprovedByAccountId())
	}
}

// 🔴 9344 是 preserveData 碼：code 非零，但 data 必須帶既有那一列的 unlockId。
// 少了 data，ADM 前端拿不到導向覆核對話框的 id ⇒ A4 走不下去。
func TestRequestManualUnlock_Duplicate_Returns9344WithData(t *testing.T) {
	unlocks := &muUnlocks{}
	s, _ := muServer(muCancelledOp(), unlocks)
	if _, err := s.RequestManualUnlockForAdmin(context.Background(), muRequestReq()); err != nil {
		t.Fatalf("第一次發起失敗：%v", err)
	}

	resp, err := s.RequestManualUnlockForAdmin(context.Background(), muRequestReq())
	if err != nil {
		t.Fatalf("9344 是 preserveData 碼，不得回 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceManualUnlockNeedsSecondApproval {
		t.Fatalf("應回 9344，實得 %d", resp.GetCode())
	}
	if resp.GetData().GetUnlockId() != "9100001" {
		t.Fatalf("9344 必須帶既有那一列的 unlockId，實得 %q", resp.GetData().GetUnlockId())
	}
}

func TestRequestManualUnlock_RejectsBadInput(t *testing.T) {
	s, _ := muServer(muCancelledOp(), &muUnlocks{})
	ctx := context.Background()

	cases := map[string]*pb.RequestManualUnlockForAdminRequest{
		"缺 actor": func() *pb.RequestManualUnlockForAdminRequest {
			r := muRequestReq()
			r.ActorAccountId = 0
			return r
		}(),
		"缺 expectedVersion": func() *pb.RequestManualUnlockForAdminRequest {
			r := muRequestReq()
			r.ExpectedVersion = ""
			return r
		}(),
		"providerCheckedAt 非 ISO 8601": func() *pb.RequestManualUnlockForAdminRequest {
			r := muRequestReq()
			r.ProviderCheckedAt = "2026-08-30"
			return r
		}(),
		"查證說明空白": func() *pb.RequestManualUnlockForAdminRequest {
			r := muRequestReq()
			r.ProviderCheckEvidence = "  "
			return r
		}(),
		"查證時間在未來": func() *pb.RequestManualUnlockForAdminRequest {
			r := muRequestReq()
			r.ProviderCheckedAt = "2026-08-30T07:00:00Z"
			return r
		}(),
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.RequestManualUnlockForAdmin(ctx, req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("應回 InvalidArgument，實得 %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 端點 55
// ---------------------------------------------------------------------------

func muApproveReq(actor int64, approved bool) *pb.ApproveManualUnlockForAdminRequest {
	return &pb.ApproveManualUnlockForAdminRequest{
		ActorAccountId: actor, JobId: "7001", UnlockId: "9100001",
		Approved: approved, Reason: "已複核查證紀錄", ExpectedVersion: "5",
		IdempotencyKey: "idem-mu-2",
	}
}

func TestApproveManualUnlock_Success(t *testing.T) {
	unlocks := &muUnlocks{}
	s, ops := muServer(muCancelledOp(), unlocks)
	if _, err := s.RequestManualUnlockForAdmin(context.Background(), muRequestReq()); err != nil {
		t.Fatalf("前置發起失敗：%v", err)
	}

	resp, err := s.ApproveManualUnlockForAdmin(context.Background(), muApproveReq(muApprover, true))
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("成功應為 code 0，實得 %d（%s）", resp.GetCode(), resp.GetMessage())
	}
	if resp.GetData().GetStatus() != domain.ManualUnlockApproved {
		t.Fatalf("status 應為 APPROVED，實得 %q", resp.GetData().GetStatus())
	}
	if resp.GetData().GetApprovedByAccountId() != "4202" {
		t.Fatalf("approvedBy 不符：%q", resp.GetData().GetApprovedByAccountId())
	}
	if ops.op.Status != "MANUAL_PROCESS_ALLOWED" {
		t.Fatalf("job 必須同時解鎖，實得 %q", ops.op.Status)
	}
}

// 9345 走 409（envelope 業務碼），不是 gRPC error。
func TestApproveManualUnlock_SameActorReturns9345(t *testing.T) {
	unlocks := &muUnlocks{}
	s, ops := muServer(muCancelledOp(), unlocks)
	if _, err := s.RequestManualUnlockForAdmin(context.Background(), muRequestReq()); err != nil {
		t.Fatalf("前置發起失敗：%v", err)
	}

	resp, err := s.ApproveManualUnlockForAdmin(context.Background(), muApproveReq(muRequester, true))
	if err != nil {
		t.Fatalf("業務碼不得走 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceManualUnlockSameActor {
		t.Fatalf("應回 9345，實得 %d", resp.GetCode())
	}
	if ops.op.Status != "CANCELLED_BY_ADMIN" {
		t.Fatalf("被 9345 擋下時 job 不得轉態，實得 %q", ops.op.Status)
	}
	if ops.allowed != 0 {
		t.Fatalf("被 9345 擋下時不得呼叫 J29，實得 %d 次", ops.allowed)
	}
}

// 端點 54 與 55 對「作業不存在」必須回逐位元相同的 envelope。
//
// 為什麼逐位元比：只要有一支多帶了一個欄位（例如把 unlock_id 回填進 data），
// 就等於對外洩漏「這個 id 存在與否」以外的資訊，而前端會對同一件事寫出兩種處理。
func TestManualUnlock_JobNotFoundIsByteIdentical(t *testing.T) {
	ops := &muOps{notFound: true}
	uc := usecase.NewAdminJob(nopTx{}, ops, nil, nil, muAudit{}, nil, &muUnlocks{}, nil, nil,
		nil, usecase.IssuanceConfig{}, func() int64 { return muUnlockID }, zap.NewNop()).
		WithClock(func() time.Time { return muNow })
	s := NewInvoiceServer(nil, nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	ctx := context.Background()

	req, err := s.RequestManualUnlockForAdmin(ctx, muRequestReq())
	if err != nil {
		t.Fatalf("端點 54：%v", err)
	}
	app, err := s.ApproveManualUnlockForAdmin(ctx, muApproveReq(muApprover, true))
	if err != nil {
		t.Fatalf("端點 55：%v", err)
	}
	if req.GetCode() != bizcode.InvoiceJobNotFound {
		t.Fatalf("端點 54 應回 9340，實得 %d", req.GetCode())
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal 54：%v", err)
	}
	appBytes, err := proto.Marshal(app)
	if err != nil {
		t.Fatalf("marshal 55：%v", err)
	}
	if string(reqBytes) != string(appBytes) {
		t.Fatalf("兩支端點的 9340 回應必須逐位元相同：\n54=%v\n55=%v", req, app)
	}
}

// unlock_id 不屬於這個 job ⇒ 與「不存在」同回 9340，且 data 必須是空的。
func TestApproveManualUnlock_ForeignUnlockIsNotFound(t *testing.T) {
	unlocks := &muUnlocks{rows: []domain.ManualUnlock{{
		ID: muUnlockID, OperationID: 999999,
		Status:               domain.ManualUnlockAwaitingSecondApproval,
		RequestedByAccountID: muRequester,
		ExpiresAt:            muNow.Add(time.Hour),
	}}}
	s, ops := muServer(muCancelledOp(), unlocks)

	resp, err := s.ApproveManualUnlockForAdmin(context.Background(), muApproveReq(muApprover, true))
	if err != nil {
		t.Fatalf("不得走 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceJobNotFound {
		t.Fatalf("應回 9340，實得 %d", resp.GetCode())
	}
	if resp.GetData() != nil {
		t.Fatalf("9340 不得帶 data（會洩漏該 id 存在），實得 %v", resp.GetData())
	}
	if ops.allowed != 0 {
		t.Fatalf("不得呼叫 J29，實得 %d 次", ops.allowed)
	}
}
