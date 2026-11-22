package grpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type calRepo struct {
	domain.BusinessDayRepository
	deletes int
	rows    int
}

func (r *calRepo) DeleteAll(context.Context) (int64, error)           { r.deletes++; return 0, nil }
func (r *calRepo) Insert(context.Context, time.Time, time.Time) error { r.rows++; return nil }
func (r *calRepo) SetCoverage(context.Context, time.Time, time.Time, time.Time) error {
	return nil
}

type calTx struct{}

func (calTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func calServer(repo *calRepo) *InvoiceServer {
	uc := usecase.NewBusinessCalendarMirror(repo, calTx{}, zap.NewNop(), time.Now)
	return (&InvoiceServer{log: zap.NewNop()}).WithCalendarMirror(uc)
}

func TestReplaceBusinessDayMirror_RequiresInternalCall(t *testing.T) {
	repo := &calRepo{}
	_, err := calServer(repo).ReplaceBusinessDayMirror(context.Background(),
		&pb.ReplaceBusinessDayMirrorRequest{CoveredFrom: "2026-01-01", CoveredTo: "2026-12-31"})
	if status.Code(err) != codes.PermissionDenied || repo.deletes != 0 {
		t.Fatalf("使用者面呼叫必須被擋下且不得動鏡像：%v deletes=%d", err, repo.deletes)
	}
}

func TestReplaceBusinessDayMirror_BadDateIsInvalidArgumentAndNoWrite(t *testing.T) {
	repo := &calRepo{}
	_, err := calServer(repo).ReplaceBusinessDayMirror(internalCtx(), &pb.ReplaceBusinessDayMirrorRequest{
		CoveredFrom: "2026-01-01", CoveredTo: "2026-12-31", HolidayDates: []string{"2026-02-30"},
	})
	if status.Code(err) != codes.InvalidArgument || repo.deletes != 0 {
		t.Fatalf("壞日期整批拒絕：%v deletes=%d", err, repo.deletes)
	}
}

func TestReplaceBusinessDayMirror_OK(t *testing.T) {
	repo := &calRepo{}
	resp, err := calServer(repo).ReplaceBusinessDayMirror(internalCtx(), &pb.ReplaceBusinessDayMirrorRequest{
		CoveredFrom: "2026-01-01", CoveredTo: "2026-12-31", HolidayDates: []string{"2026-01-01", "2026-02-17"},
	})
	if err != nil || resp.GetCode() != 0 || resp.GetData().GetHolidayCount() != "2" || repo.rows != 2 {
		t.Fatalf("resp=%v err=%v rows=%d", resp, err, repo.rows)
	}
}

func TestReplaceBusinessDayMirror_NotConfiguredIsUnavailable(t *testing.T) {
	_, err := (&InvoiceServer{log: zap.NewNop()}).ReplaceBusinessDayMirror(internalCtx(), &pb.ReplaceBusinessDayMirrorRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("未接線回 Unavailable：%v", err)
	}
}
