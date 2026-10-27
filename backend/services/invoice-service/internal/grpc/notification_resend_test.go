package grpc

import (
	"context"
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

// 本檔驗端點 32／49／71 的 Layer 5。**核心只有一條**：
//
//	🔴 9352 必須走 `codes.ResourceExhausted` 而不是 envelope 的非零碼。
//	   Gateway 的 GrpcErr 以 gRPC status 決定 HTTP 狀態；回成 `Code: 9352` 的成功回應
//	   會變成 HTTP 200，前端的「請稍候再試」永遠不會出現，而按鈕看起來像成功了。

const (
	nrDocID   = int64(8001)
	nrStoreID = int64(2685914682)
	nrBuyerID = int64(2623900520)
)

var nrNow = time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)

type nrDocs struct {
	domain.DocumentRepository
	doc     domain.Document
	claimed int
	// windowOpen=false 模擬「還在 10 分鐘窗口內」（SQL 的 rows=0）。
	windowOpen bool
}

func (f *nrDocs) GetByID(context.Context, int64) (domain.Document, error) { return f.doc, nil }

func (f *nrDocs) GetForSeller(_ context.Context, id, storeID int64) (domain.SellerDocumentItem, error) {
	if id != f.doc.ID || storeID != nrStoreID {
		return domain.SellerDocumentItem{}, newBizErr(bizcode.InvoiceDocumentNotFound)
	}
	return domain.SellerDocumentItem{Document: f.doc}, nil
}

func (f *nrDocs) GetForBuyerByID(_ context.Context, id, buyerID int64) (domain.Document, error) {
	if id != f.doc.ID || buyerID != nrBuyerID {
		return domain.Document{}, newBizErr(bizcode.InvoiceDocumentNotFound)
	}
	return f.doc, nil
}

func (f *nrDocs) ClaimNotificationResend(context.Context, int64, time.Time, time.Time) (int32, bool, error) {
	if !f.windowOpen {
		return 0, false, nil
	}
	f.claimed++
	return int32(f.claimed), true, nil
}

func (f *nrDocs) WithDecryptedRecipient(_ context.Context, _ int64, fn func(domain.RecipientSecrets) error) error {
	return fn(domain.RecipientSecrets{Email: "buyer@example.com"})
}

type nrIssuers struct{ domain.IssuerRepository }

func (nrIssuers) GetByID(context.Context, int64) (domain.Issuer, error) {
	return domain.Issuer{ID: 900, IssuerType: "SELLER", IssuerRefID: nrStoreID}, nil
}

type nrOutbox struct{ domain.OutboxRepository }

func (nrOutbox) Insert(context.Context, domain.NewOutboxEvent) (domain.OutboxEvent, error) {
	return domain.OutboxEvent{}, nil
}

func nrServer(windowOpen bool) *InvoiceServer {
	buyer := nrBuyerID
	no := "ZA10068421"
	cipher := "ENCRYPTED-EMAIL"
	docs := &nrDocs{windowOpen: windowOpen, doc: domain.Document{
		ID: nrDocID, IssuerID: 900,
		Status:            codemeta.InvoiceDocumentStatusIssued.Value,
		DocumentKind:      codemeta.InvoiceDocumentKindB2c.Value,
		RecipientType:     codemeta.InvoiceRecipientTypeBuyer.Value,
		RecipientRefID:    &buyer,
		RecipientTaxID:    domain.B2CBuyerIdentifier,
		ProviderInvoiceNo: &no,
		// 有密文才會走遮罩路徑；沒有密文（載具發票／收據）回空字串是誠實的
		// 「沒有寄件對象」，另由 usecase 測試守。
		RecipientEmailCiphertext: &cipher,
		TotalAmount:              1000,
	}}
	uc := usecase.NewNotificationResend(nopTx{}, docs, nrIssuers{}, nrOutbox{}, zap.NewNop()).
		WithClock(func() time.Time { return nrNow })
	return NewInvoiceServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, zap.NewNop())
}

func TestResend_SellerHandlerSuccess(t *testing.T) {
	s := nrServer(true)
	resp, err := s.ResendInvoiceNotification(context.Background(), &pb.ResendInvoiceNotificationRequest{
		StoreId: nrStoreID, DocumentId: "8001", IdempotencyKey: "idem-32",
	})
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 || !resp.GetData().GetResent() {
		t.Fatalf("回應不符：%+v", resp)
	}
	if resp.GetData().GetSentAt() != "2026-08-30T06:00:00Z" {
		t.Fatalf("sentAt 不符：%q", resp.GetData().GetSentAt())
	}
	// 收件人一律遮罩：本欄名字（toMasked）就是它的型別契約。
	if got := resp.GetData().GetToMasked(); got == "" || got == "buyer@example.com" {
		t.Fatalf("toMasked 必須是遮罩後的值，實得 %q", got)
	}
}

// 🔴 三支端點的 9352 都必須是 ResourceExhausted（HTTP 429），且 message 帶
// `[9352] ` 前綴讓 Gateway 還原得出業務碼。
func TestResend_TooFrequentIsResourceExhausted(t *testing.T) {
	ctx := context.Background()
	cases := map[string]func(*InvoiceServer) error{
		"端點 32 賣家": func(s *InvoiceServer) error {
			_, err := s.ResendInvoiceNotification(ctx, &pb.ResendInvoiceNotificationRequest{
				StoreId: nrStoreID, DocumentId: "8001"})
			return err
		},
		"端點 49 ADM": func(s *InvoiceServer) error {
			_, err := s.ResendInvoiceNotificationForAdmin(ctx, &pb.ResendInvoiceNotificationForAdminRequest{
				ActorAccountId: 90001, DocumentId: "8001"})
			return err
		},
		"端點 71 買家": func(s *InvoiceServer) error {
			_, err := s.ResendBuyerInvoiceNotification(ctx, &pb.ResendBuyerInvoiceNotificationRequest{
				BuyerId: nrBuyerID, DocumentId: "8001"})
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			err := run(nrServer(false))
			if status.Code(err) != codes.ResourceExhausted {
				t.Fatalf("9352 必須是 ResourceExhausted（否則 Gateway 給 HTTP 200），實得 %v", err)
			}
			if msg := status.Convert(err).Message(); len(msg) < 6 || msg[:6] != "[9352]" {
				t.Fatalf("message 必須帶 [9352] 前綴讓 Gateway 還原業務碼，實得 %q", msg)
			}
		})
	}
}

// Tier A：越權與查無同回 9301（envelope 業務碼，不是 gRPC error）。
func TestResend_ForeignDocumentIsNotFound(t *testing.T) {
	s := nrServer(true)
	resp, err := s.ResendInvoiceNotification(context.Background(), &pb.ResendInvoiceNotificationRequest{
		StoreId: 9999999, DocumentId: "8001",
	})
	if err != nil {
		t.Fatalf("業務碼不得走 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceDocumentNotFound {
		t.Fatalf("應回 9301，實得 %d", resp.GetCode())
	}
	if resp.GetData() != nil {
		t.Fatal("9301 不得帶 data")
	}
}

func TestResend_RejectsMissingPrincipal(t *testing.T) {
	s := nrServer(true)
	ctx := context.Background()
	cases := map[string]func() error{
		"端點 32 缺 store_id": func() error {
			_, err := s.ResendInvoiceNotification(ctx, &pb.ResendInvoiceNotificationRequest{DocumentId: "8001"})
			return err
		},
		"端點 71 缺 buyer_id": func() error {
			_, err := s.ResendBuyerInvoiceNotification(ctx, &pb.ResendBuyerInvoiceNotificationRequest{DocumentId: "8001"})
			return err
		},
		"端點 49 缺 actor": func() error {
			_, err := s.ResendInvoiceNotificationForAdmin(ctx, &pb.ResendInvoiceNotificationForAdminRequest{DocumentId: "8001"})
			return err
		},
		"document_id 空字串": func() error {
			_, err := s.ResendInvoiceNotification(ctx, &pb.ResendInvoiceNotificationRequest{StoreId: nrStoreID})
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			if err := run(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("應回 InvalidArgument，實得 %v", err)
			}
		})
	}
}
