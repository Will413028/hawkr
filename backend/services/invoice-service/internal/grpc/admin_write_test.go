package grpc

import (
	"context"
	"testing"
	"time"

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

// 🔴 requirePrincipal 的 0 一律拒絕：actor_account_id=0 的真正意義是 **Gateway 沒注入**，
// 那是契約破了，不是「有一位 id 為 0 的管理員」。放行的話，一支需要 ADM 身分的寫入端點
// 會在沒有身分的情況下真的執行下去，而稽核列會記成 actor 0。
func TestAdminWrite_RejectsMissingActor(t *testing.T) {
	s := NewInvoiceServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	ctx := context.Background()

	cases := map[string]func() error{
		"端點 52": func() error {
			_, err := s.RetryInvoiceJobForAdmin(ctx, &pb.RetryInvoiceJobForAdminRequest{JobId: "1", ExpectedVersion: "0"})
			return err
		},
		"端點 53": func() error {
			_, err := s.CancelInvoiceJobForAdmin(ctx, &pb.CancelInvoiceJobForAdminRequest{JobId: "1", ExpectedVersion: "0"})
			return err
		},
		"端點 56": func() error {
			_, err := s.ReportManualResultForAdmin(ctx, &pb.ReportManualResultForAdminRequest{JobId: "1", ExpectedVersion: "0"})
			return err
		},
		"端點 57": func() error {
			_, err := s.CreateManualIssuanceForAdmin(ctx, &pb.CreateManualIssuanceForAdminRequest{Confirmed: true})
			return err
		},
		"端點 60": func() error {
			_, err := s.ActOnInvoiceComplaintForAdmin(ctx, &pb.ActOnInvoiceComplaintForAdminRequest{ComplaintId: "1", ExpectedVersion: "0"})
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			err := run()
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("缺 actor_account_id 應回 InvalidArgument，實得 %v", err)
			}
		})
	}
}

// expected_version 是 §6.1.0(10) 的必填欄。缺席不是「用 0 試試看」——0 會是一件從未被
// 更新過的作業的合法版本，那時 CAS 會通過而 ADM 看到的是三分鐘前那一版。
func TestAdminWrite_RequiresExpectedVersion(t *testing.T) {
	s := NewInvoiceServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if _, err := s.RetryInvoiceJobForAdmin(context.Background(), &pb.RetryInvoiceJobForAdminRequest{
		ActorAccountId: 42, JobId: "7001",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 expected_version 應回 InvalidArgument，實得 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 9340（作業不存在）的回應必須逐位元相同
// ---------------------------------------------------------------------------

type notFoundOps struct{ domain.OperationRepository }

func (notFoundOps) GetByID(context.Context, int64) (domain.Operation, error) {
	return domain.Operation{}, pkgerrors.NewBusiness(bizcode.InvoiceJobNotFound,
		bizcode.Message(bizcode.InvoiceJobNotFound))
}

type nopTx struct{}

func (nopTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// 三支 job 端點對「作業不存在」必須回**完全一樣**的 envelope。
//
// 為什麼要逐位元比：ADM 面雖然沒有 Tier A 的越權維度，但三支端點只要有一支多帶了一個
// 欄位（例如把 job_id 回填進 data），就等於對外洩漏「這個 id 存在與否」以外的資訊，
// 而且會讓前端對同一件事寫出三種處理。proto.Equal 只比語意，Marshal 比的是實際 wire。
func TestAdminWrite_JobNotFoundIsByteIdentical(t *testing.T) {
	uc := usecase.NewAdminJob(nopTx{}, notFoundOps{}, nil, nil, nil, nil, nil, nil, nil, nil, usecase.IssuanceConfig{}, func() int64 { return 1 }, zap.NewNop()).
		WithClock(func() time.Time { return time.Unix(0, 0).UTC() })
	s := NewInvoiceServer(nil, nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	ctx := context.Background()

	retry, err := s.RetryInvoiceJobForAdmin(ctx, &pb.RetryInvoiceJobForAdminRequest{
		ActorAccountId: 42, JobId: "7001", Reason: "r", ExpectedVersion: "3",
	})
	if err != nil {
		t.Fatalf("9340 應走 envelope 不是 gRPC error：%v", err)
	}
	if retry.GetCode() != bizcode.InvoiceJobNotFound {
		t.Fatalf("期望 9340，實得 %d", retry.GetCode())
	}
	if retry.GetData() != nil {
		t.Fatal("9340 不是 preserveData 碼，不得帶 data")
	}

	cancel, err := s.CancelInvoiceJobForAdmin(ctx, &pb.CancelInvoiceJobForAdminRequest{
		ActorAccountId: 42, JobId: "7001", ReasonDetail: "d", ExpectedVersion: "3",
	})
	if err != nil {
		t.Fatalf("9340 應走 envelope：%v", err)
	}
	report, err := s.ReportManualResultForAdmin(ctx, &pb.ReportManualResultForAdminRequest{
		ActorAccountId: 42, JobId: "7001", InvoiceNo: "AB12345678",
		InvoiceDate: "2026-08-25", TotalAmount: "1200", ExpectedVersion: "3",
	})
	if err != nil {
		t.Fatalf("9340 應走 envelope：%v", err)
	}

	want, err := proto.Marshal(retry)
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]*pb.InvoiceJobMutationResponse{"端點 53": cancel, "端點 56": report} {
		b, err := proto.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(want) {
			t.Fatalf("%s 的 9340 回應與端點 52 不是逐位元相同：\n got=%q\nwant=%q", name, b, want)
		}
	}
}
