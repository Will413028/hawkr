package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔驗端點 28／29 的 Layer 5：兩支必須回**完全一樣**的形狀，
// 以及 fail-closed 三態不得在 mapper 這一層被抹平。

const (
	raStoreID  = int64(2685914682)
	raIssuerID = int64(900)
	raBasisKey = "invoice/tax/2685914682/approval.pdf"
)

var raNow = time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)

type raProfiles struct {
	domain.SellerTaxProfileRepository
	row domain.SellerTaxProfile
}

func (r *raProfiles) GetByIssuer(context.Context, int64) (domain.SellerTaxProfile, error) {
	return r.row, nil
}
func (r *raProfiles) GetByID(context.Context, int64) (domain.SellerTaxProfile, error) {
	return r.row, nil
}
func (r *raProfiles) ConfirmReceiptAuthorization(_ context.Context, in domain.ConfirmReceiptAuthorization) (bool, error) {
	if r.row.ReviewStatus != codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value {
		return false, nil
	}
	if r.row.Version != in.ExpectedVersion {
		return false, nil
	}
	r.row.ReceiptAuthorizedAt = in.AuthorizedAt
	r.row.ReceiptConsentVersion = in.ConsentVersion
	r.row.Version++
	return true, nil
}

type raAudit struct{ domain.AuditRepository }

func (raAudit) AppendStatusHistory(context.Context, domain.NewStatusHistoryEntry) error { return nil }

func raProfile(status string, basisKey string) domain.SellerTaxProfile {
	approved := raNow.Add(-24 * time.Hour)
	validUntil := raNow.Add(300 * 24 * time.Hour)
	p := domain.SellerTaxProfile{
		ID: 55, IssuerID: raIssuerID, Version: 3, ReviewStatus: status,
		ApprovedAt: &approved, ApprovalValidUntil: &validUntil,
		EvidenceObjectKeys: json.RawMessage(`["` + raBasisKey + `"]`),
	}
	if basisKey != "" {
		k := basisKey
		id := int64(778899)
		p.ReceiptApprovalBasisKey = &k
		p.ReceiptApprovalBasisAssetID = &id
	}
	return p
}

func raServer(p domain.SellerTaxProfile) *InvoiceServer {
	profiles := &raProfiles{row: p}
	issuers := qIssuers{
		byRef: map[int64]domain.Issuer{raStoreID: {ID: raIssuerID, IssuerType: "SELLER", IssuerRefID: raStoreID}},
		byID:  map[int64]domain.Issuer{raIssuerID: {ID: raIssuerID, IssuerType: "SELLER", IssuerRefID: raStoreID}},
	}
	q := usecase.NewQuery(
		issuers, profiles, qAccounts{}, qPrefs{}, qDocuments{}, qOperations{}, qComplaints{},
		&qAudit{}, qAdjustments{}, qCorrections{}, qOverview{}, qBizDays{}, qCutovers{}, qUnlocks{},
		qCancelled{},
		usecase.IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		nil,
	).WithClock(func() time.Time { return raNow })
	tp := usecase.NewTaxProfile(nopTx{}, issuers, profiles, raAudit{}, nil, nil,
		func() int64 { return 1 }, zap.NewNop()).
		WithClock(func() time.Time { return raNow })
	return NewInvoiceServer(nil, q, nil, nil, tp, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
}

// ---------------------------------------------------------------------------
// 端點 28
// ---------------------------------------------------------------------------

func TestGetSellerReceiptAuthorization_EligibleWithBasis(t *testing.T) {
	s := raServer(raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, raBasisKey))

	resp, err := s.GetSellerReceiptAuthorization(context.Background(),
		&pb.GetSellerReceiptAuthorizationRequest{StoreId: raStoreID})
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	d := resp.GetData()
	if !d.GetEligible() || d.GetIneligibleReason() != "" {
		t.Fatalf("應為 eligible，實得 %+v", d)
	}
	if !d.GetApprovalBasisAvailable() || d.GetApprovalBasisAssetId() != "778899" {
		t.Fatalf("依據文件應可用且帶 assetId，實得 %+v", d)
	}
	if d.GetVersion() != "3" {
		t.Fatalf("version 是端點 29 的 expectedVersion 來源，實得 %q", d.GetVersion())
	}
}

// 🔴 三態不得在 mapper 抹平：不可用時**不得回 assetId**（Gateway 用它組下載路徑，
// 回了就等於給賣家一個點下去必定 404 的連結）。
func TestGetSellerReceiptAuthorization_BasisUnavailableCarriesNoAssetID(t *testing.T) {
	cases := map[string]struct {
		basisKey   string
		wantReason string
	}{
		"ADM 未指名":  {"", "NOT_DESIGNATED"},
		"上傳者不是本賣家": {"invoice/tax/9999/admin.pdf", "UPLOADER_NOT_SELLER"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := raServer(raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, tc.basisKey))
			resp, err := s.GetSellerReceiptAuthorization(context.Background(),
				&pb.GetSellerReceiptAuthorizationRequest{StoreId: raStoreID})
			if err != nil {
				t.Fatalf("不應回 gRPC error：%v", err)
			}
			d := resp.GetData()
			if d.GetApprovalBasisAvailable() {
				t.Fatal("應為不可用")
			}
			if d.GetApprovalBasisUnavailableReason() != tc.wantReason {
				t.Fatalf("理由應為 %s，實得 %q", tc.wantReason, d.GetApprovalBasisUnavailableReason())
			}
			if d.GetApprovalBasisAssetId() != "" {
				t.Fatalf("不可用時不得回 assetId，實得 %q", d.GetApprovalBasisAssetId())
			}
		})
	}
}

func TestGetSellerReceiptAuthorization_RejectsMissingStore(t *testing.T) {
	s := raServer(raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, ""))
	if _, err := s.GetSellerReceiptAuthorization(context.Background(),
		&pb.GetSellerReceiptAuthorizationRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 store_id 應回 InvalidArgument，實得 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 29
// ---------------------------------------------------------------------------

func TestConfirmSellerReceiptAuthorization_Success(t *testing.T) {
	s := raServer(raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, raBasisKey))

	resp, err := s.ConfirmSellerReceiptAuthorization(context.Background(),
		&pb.ConfirmSellerReceiptAuthorizationRequest{
			StoreId: raStoreID, ConsentVersion: "v2026-08", Authorized: true,
			ExpectedVersion: "3", IdempotencyKey: "idem-29",
		})
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("成功應為 code 0，實得 %d（%s）", resp.GetCode(), resp.GetMessage())
	}
	d := resp.GetData()
	if d.GetAuthorizedAt() != "2026-08-30T06:00:00Z" || d.GetConsentVersion() != "v2026-08" {
		t.Fatalf("授權後兩欄必須同時有值，實得 %+v", d)
	}
	if d.GetVersion() != "4" {
		t.Fatalf("version 應遞增，實得 %q", d.GetVersion())
	}
}

// 非免用統一發票核准賣家 ⇒ 9335 走 envelope，不是 gRPC error。
func TestConfirmSellerReceiptAuthorization_NotEligibleReturns9335(t *testing.T) {
	s := raServer(raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, ""))

	resp, err := s.ConfirmSellerReceiptAuthorization(context.Background(),
		&pb.ConfirmSellerReceiptAuthorizationRequest{
			StoreId: raStoreID, ConsentVersion: "v2026-08", Authorized: true, ExpectedVersion: "3",
		})
	if err != nil {
		t.Fatalf("業務碼不得走 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceReceiptAuthNotEligible {
		t.Fatalf("應回 9335，實得 %d", resp.GetCode())
	}
}

func TestConfirmSellerReceiptAuthorization_RejectsBadInput(t *testing.T) {
	s := raServer(raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, ""))
	ctx := context.Background()

	cases := map[string]*pb.ConfirmSellerReceiptAuthorizationRequest{
		"缺 store_id":        {ConsentVersion: "v1", Authorized: true, ExpectedVersion: "3"},
		"缺 expectedVersion": {StoreId: raStoreID, ConsentVersion: "v1", Authorized: true},
		"授權但沒帶條款版本":         {StoreId: raStoreID, Authorized: true, ExpectedVersion: "3"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.ConfirmSellerReceiptAuthorization(ctx, req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("應回 InvalidArgument，實得 %v", err)
			}
		})
	}
}

// 取消授權（authorized=false）是正常業務結果，且不需要 consentVersion。
func TestConfirmSellerReceiptAuthorization_Revoke(t *testing.T) {
	authorized := raNow.Add(-time.Hour)
	ver := "v2026-07"
	p := raProfile(codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value, "")
	p.ReceiptAuthorizedAt = &authorized
	p.ReceiptConsentVersion = &ver
	s := raServer(p)

	resp, err := s.ConfirmSellerReceiptAuthorization(context.Background(),
		&pb.ConfirmSellerReceiptAuthorizationRequest{
			StoreId: raStoreID, Authorized: false, ExpectedVersion: "3",
		})
	if err != nil {
		t.Fatalf("取消授權不應是錯誤：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("應為 code 0，實得 %d", resp.GetCode())
	}
	if resp.GetData().GetAuthorizedAt() != "" || resp.GetData().GetConsentVersion() != "" {
		t.Fatalf("取消後兩欄必須同時清空，實得 %+v", resp.GetData())
	}
}
