package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 本檔驗 M4 寫入面的**型別邊界、Tier A 出口形狀與明文外洩**；業務規則本身在
// internal/usecase/provider_account_test.go。
//
// 🔴 兩支核心斷言：
//
//	① 越權與查無的回應**逐位元相同**（proto.Marshal，不是比 code）。比 code 只證明兩者
//	   同碼，證明不了 message、data 或任何未來新增的欄位沒有洩漏「這家店存在，只是不是你的」。
//	② **AppKey 不出現在序列化後的 wire bytes 裡**。proto 的 reserved 90 讓「加回欄位」
//	   變成 buf build 失敗，但它擋不住有人把明文塞進 status_notice 這種既有的字串欄
//	   ——只有掃 wire bytes 才抓得到那個形狀。

const (
	pxStoreA    int64 = 2685914682
	pxStoreB    int64 = 2632049831
	pxAppKey          = "amego-app-key-SECRET-9f2c"
	pxTaxID           = "12345678"
	pxAccountID int64 = 3001
)

var pxNow = time.Date(2026, 8, 30, 8, 0, 0, 0, time.UTC)

// pxIssuers 照 SQL 述詞回應：只有 pxStoreA 有 issuer 列。
type pxIssuers struct{ domain.IssuerRepository }

func (pxIssuers) GetByRef(_ context.Context, issuerType string, ref int64) (domain.Issuer, error) {
	if issuerType != codemeta.InvoiceIssuerTypeSeller.Value || ref != pxStoreA {
		return domain.Issuer{}, pkgerrors.NewBusiness(bizcode.InvoiceProviderAccountNotFound,
			bizcode.Message(bizcode.InvoiceProviderAccountNotFound))
	}
	return domain.Issuer{ID: 900, IssuerType: issuerType, IssuerRefID: ref}, nil
}

type pxProfiles struct {
	domain.SellerTaxProfileRepository
}

func (pxProfiles) GetByIssuer(context.Context, int64) (domain.SellerTaxProfile, error) {
	taxID := pxTaxID
	return domain.SellerTaxProfile{
		ID: 500, IssuerID: 900, TaxID: &taxID,
		ReviewStatus: codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
	}, nil
}

// pxAccounts 是一份最小的 provider 帳號 repo：它記下每一次收到的明文 AppKey，
// 讓「明文有到 repository」與「明文沒有到回應」兩件事都能被斷言。
type pxAccounts struct {
	domain.ProviderAccountRepository
	row      domain.ProviderAccount
	exists   bool
	seenKeys []string
}

func (r *pxAccounts) GetByIssuer(context.Context, int64, string) (domain.ProviderAccount, error) {
	if !r.exists {
		return domain.ProviderAccount{}, pkgerrors.NewBusiness(bizcode.InvoiceProviderAccountNotFound,
			bizcode.Message(bizcode.InvoiceProviderAccountNotFound))
	}
	return r.row, nil
}

func (r *pxAccounts) Create(_ context.Context, in domain.NewProviderAccount) error {
	r.seenKeys = append(r.seenKeys, in.AppKey)
	r.row = domain.ProviderAccount{
		ID: in.ID, IssuerID: in.IssuerID, Provider: in.Provider, ProviderTaxID: in.ProviderTaxID,
		Status: in.Status, CredentialLast4: "9f2c",
	}
	r.exists = true
	return nil
}

func (r *pxAccounts) ResubmitCredential(_ context.Context, in domain.ResubmitProviderCredential) (bool, bool, error) {
	r.seenKeys = append(r.seenKeys, in.AppKey)
	r.row.Status = codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value
	r.row.Version++
	return true, false, nil
}

func (r *pxAccounts) StartVerification(context.Context, int64, int64, time.Time) (bool, error) {
	r.row.Status = codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value
	r.row.Version++
	return true, nil
}

func (r *pxAccounts) LandVerification(_ context.Context, in domain.LandProviderVerification) (bool, error) {
	r.row.Status = in.NextStatus
	r.row.StatusReason = &in.StatusReason
	r.row.Version++
	return true, nil
}

func (r *pxAccounts) SetAutomationCutover(_ context.Context, _, _ int64, cutoverAt, _ time.Time) (bool, error) {
	r.row.AutomationCutoverAt = &cutoverAt
	r.row.Version++
	return true, nil
}

func (r *pxAccounts) WithDecryptedCredential(_ context.Context, _ int64, fn func(string, string) error) error {
	return fn(pxTaxID, pxAppKey)
}

type pxOps struct{ domain.OperationRepository }

func (pxOps) CountOpenForIssuer(context.Context, int64) (int64, error) { return 0, nil }

// pxCutovers 是切點歷史的替身。它只需要回無誤：端點 26 的回應不含歷史，
// 歷史的內容由 usecase 層與 integration 測試驗。
type pxCutovers struct {
	domain.ProviderCutoverRepository
}

func (pxCutovers) CloseOpen(context.Context, int64, time.Time) (int64, error) { return 0, nil }
func (pxCutovers) Create(_ context.Context, in domain.NewProviderCutover) (domain.ProviderCutover, error) {
	return domain.ProviderCutover{ProviderAccountID: in.ProviderAccountID}, nil
}

type pxAudit struct{ domain.AuditRepository }

func (pxAudit) AppendStatusHistory(context.Context, domain.NewStatusHistoryEntry) error { return nil }
func (pxAudit) AppendAdminAuditLog(context.Context, domain.NewAdminAuditLogEntry) (int64, error) {
	return 55, nil
}

// pxProvider 是一份會通過三項檢查的 AMEGO 替身。
type pxProvider struct{}

func (pxProvider) IssueInvoice(context.Context, amego.Credential, amego.IssueRequest) (*amego.IssueResult, error) {
	return nil, nil
}
func (pxProvider) VerifyMobileBarcode(context.Context, amego.Credential, string) error { return nil }
func (pxProvider) QueryCompanyNames(context.Context, amego.Credential, []string) (map[string]string, error) {
	return map[string]string{pxTaxID: "測試股份有限公司"}, nil
}
func (pxProvider) ListTracks(context.Context, amego.Credential, int, int) ([]amego.Track, error) {
	return []amego.Track{{Code: "AB", TrackAPICode: "api-1"}}, nil
}

func newProviderServer(exists bool, status string) (*InvoiceServer, *pxAccounts) {
	accounts := &pxAccounts{exists: exists}
	if exists {
		verified := pxNow.Add(-time.Hour)
		accounts.row = domain.ProviderAccount{
			ID: pxAccountID, IssuerID: 900, Provider: "AMEGO", ProviderTaxID: pxTaxID,
			Status: status, Version: 0, CredentialLast4: "9f2c", VerifiedAt: &verified,
		}
	}
	uc := usecase.NewProviderAccount(xTx{}, pxIssuers{}, pxProfiles{}, accounts,
		// adjustments：本檔驗的是 gRPC 邊界（授權、參數、回應形狀），不走 failOpen
		// 那一段 ⇒ nil 就是誠實的替身。傳一個「什麼都回得出來」的 fake 反而會讓
		// 「有沒有真的呼叫它」變得看不出來。
		pxOps{}, nil, pxAudit{}, pxCutovers{}, pxProvider{}, func() int64 { return 4242 }, zap.NewNop()).
		WithClock(func() time.Time { return pxNow })
	return NewInvoiceServer(nil, nil, nil, nil, nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), accounts
}

// ---------------------------------------------------------------------------
// Tier A：越權與查無逐位元相同
// ---------------------------------------------------------------------------

// TestProviderWritesForeignAndMissingAreByteIdentical 是本檔最重要的一支。
//
// pxStoreB 是一家**真的存在**的店（測試常數表裡的賣家 B），但它沒有 issuer 列；
// pxStoreA 有 issuer 但本案例下沒有 provider 帳號列。兩者的回應必須逐位元相同——
// 差一個 byte 就等於在回應裡放了一個「這家店有沒有設定過」的旗標。
func TestProviderWritesForeignAndMissingAreByteIdentical(t *testing.T) {
	cases := []struct {
		name string
		call func(*InvoiceServer, int64) (*pb.SellerProviderAccountResponse, error)
	}{
		{"25 verify", func(s *InvoiceServer, store int64) (*pb.SellerProviderAccountResponse, error) {
			return s.VerifySellerProviderAccount(context.Background(), &pb.VerifySellerProviderAccountRequest{
				StoreId: store, ExpectedVersion: "0",
			})
		}},
		{"26 activate", func(s *InvoiceServer, store int64) (*pb.SellerProviderAccountResponse, error) {
			return s.ActivateSellerProviderAccount(context.Background(), &pb.ActivateSellerProviderAccountRequest{
				StoreId: store, CutoverAt: pxNow.Add(time.Hour).Format(time.RFC3339), ExpectedVersion: "0",
			})
		}},
		{"27 rotate", func(s *InvoiceServer, store int64) (*pb.SellerProviderAccountResponse, error) {
			return s.RotateSellerProviderCredentials(context.Background(), &pb.RotateSellerProviderCredentialsRequest{
				StoreId: store, AppKey: "another-key", ExpectedVersion: "0",
			})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 越權：pxStoreB 有店但沒有 issuer 列。
			foreignSrv, _ := newProviderServer(false, "")
			foreign, err := tc.call(foreignSrv, pxStoreB)
			if err != nil {
				t.Fatalf("越權應回 envelope 不是 gRPC error：%v", err)
			}
			// 查無：pxStoreA 有 issuer 但沒有 provider 帳號列。
			missingSrv, _ := newProviderServer(false, "")
			missing, err := tc.call(missingSrv, pxStoreA)
			if err != nil {
				t.Fatalf("查無應回 envelope 不是 gRPC error：%v", err)
			}
			if foreign.GetCode() != bizcode.InvoiceProviderAccountNotFound {
				t.Fatalf("code = %d, want 9330", foreign.GetCode())
			}
			a, _ := proto.Marshal(foreign)
			b, _ := proto.Marshal(missing)
			if string(a) != string(b) {
				t.Fatalf("越權與查無的回應不同：\n越權 %+v\n查無 %+v", foreign, missing)
			}
		})
	}
}

// TestAdminProviderWritesForeignAndMissingAreByteIdentical 是 ADM 三支的同一件事。
//
// ADM 看得到全平台，所以這裡的「越權」語意是「store_id 根本不存在」——但兩者仍必須
// 同碼同形狀，否則 ADM 介面會變成一個可以列舉平台上哪些店設定過發票的探針。
func TestAdminProviderWritesForeignAndMissingAreByteIdentical(t *testing.T) {
	srvA, _ := newProviderServer(false, "")
	srvB, _ := newProviderServer(false, "")

	unknown, err := srvA.HealthCheckProviderAccountForAdmin(context.Background(),
		&pb.HealthCheckProviderAccountForAdminRequest{ActorAccountId: 42, StoreId: "999"})
	if err != nil {
		t.Fatalf("應回 envelope：%v", err)
	}
	noAccount, err := srvB.HealthCheckProviderAccountForAdmin(context.Background(),
		&pb.HealthCheckProviderAccountForAdminRequest{ActorAccountId: 42, StoreId: "2685914682"})
	if err != nil {
		t.Fatalf("應回 envelope：%v", err)
	}
	if unknown.GetCode() != bizcode.InvoiceProviderAccountNotFound {
		t.Fatalf("code = %d, want 9330", unknown.GetCode())
	}
	a, _ := proto.Marshal(unknown)
	b, _ := proto.Marshal(noAccount)
	if string(a) != string(b) {
		t.Fatalf("兩種查無的回應不同：\n%+v\n%+v", unknown, noAccount)
	}
}

// ---------------------------------------------------------------------------
// 明文外洩
// ---------------------------------------------------------------------------

// TestPutCredentialsResponseNeverCarriesAppKey 掃**序列化後的 wire bytes**。
//
// 🔴 為什麼不是逐欄比對：proto 的 reserved 90 已經讓「加回 app_key 欄位」變成 buf build
// 失敗，所以真正的風險不是那個欄位，而是有人把明文塞進 status_notice／message 之類的
// 既有字串欄——逐欄斷言只會檢查你想得到的那幾欄，掃 bytes 才涵蓋全部。
func TestPutCredentialsResponseNeverCarriesAppKey(t *testing.T) {
	s, accounts := newProviderServer(false, "")

	resp, err := s.PutSellerProviderCredentials(context.Background(), &pb.PutSellerProviderCredentialsRequest{
		StoreId: pxStoreA, Provider: "AMEGO", SellerTaxId: pxTaxID, AppKey: pxAppKey,
		ConsentVersion: "v1", ExpectedVersion: "0", IdempotencyKey: "idem-24",
	})
	if err != nil {
		t.Fatalf("PutSellerProviderCredentials 失敗：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("應成功，實得 code=%d message=%q", resp.GetCode(), resp.GetMessage())
	}
	// ① 明文確實有到 repository —— 少了這一句，本測試會在「根本沒寫」時假綠。
	if len(accounts.seenKeys) != 1 || accounts.seenKeys[0] != pxAppKey {
		t.Fatalf("repository 應收到一次明文 AppKey，實得 %v", accounts.seenKeys)
	}
	// ② 序列化後的每一個 byte 都不得含它。
	raw, err := proto.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), pxAppKey) {
		t.Fatalf("回應的 wire bytes 洩漏 AppKey：%q", raw)
	}
	// ③ last4 是刻意保留的自我確認欄位（§8.1.1(f) U2），它不是明文的一部分。
	if resp.GetData().GetCredentialLast4() == "" {
		t.Fatalf("credential_last4 應回傳，供賣家自我確認")
	}
}

// TestVerifyResponseNeverCarriesAppKey 對驗證路徑做同一件事。
//
// 驗證路徑會**解出明文**（WithDecryptedCredential），所以它是三條路徑裡最有機會把
// 明文帶出來的那一條。
func TestVerifyResponseNeverCarriesAppKey(t *testing.T) {
	s, _ := newProviderServer(true, codemeta.InvoiceProviderAccountStatusCredentialSubmitted.Value)

	resp, err := s.VerifySellerProviderAccount(context.Background(), &pb.VerifySellerProviderAccountRequest{
		StoreId: pxStoreA, ExpectedVersion: "0", IdempotencyKey: "idem-25",
	})
	if err != nil {
		t.Fatalf("VerifySellerProviderAccount 失敗：%v", err)
	}
	if resp.GetCode() != 0 || resp.GetData().GetStatus() != codemeta.InvoiceProviderAccountStatusActive.Value {
		t.Fatalf("三項全過應回 ACTIVE，實得 code=%d status=%q", resp.GetCode(), resp.GetData().GetStatus())
	}
	raw, _ := proto.Marshal(resp)
	if strings.Contains(string(raw), pxAppKey) {
		t.Fatalf("驗證回應的 wire bytes 洩漏 AppKey")
	}
}

// ---------------------------------------------------------------------------
// 型別邊界
// ---------------------------------------------------------------------------

func TestProviderWritesRejectMalformedInput(t *testing.T) {
	s, _ := newProviderServer(true, codemeta.InvoiceProviderAccountStatusActive.Value)

	cases := []struct {
		name string
		call func() error
	}{
		{"缺 store_id", func() error {
			_, err := s.VerifySellerProviderAccount(context.Background(),
				&pb.VerifySellerProviderAccountRequest{ExpectedVersion: "0"})
			return err
		}},
		{"expected_version 非數字", func() error {
			_, err := s.VerifySellerProviderAccount(context.Background(),
				&pb.VerifySellerProviderAccountRequest{StoreId: pxStoreA, ExpectedVersion: "abc"})
			return err
		}},
		{"cutover_at 不是 ISO 8601", func() error {
			_, err := s.ActivateSellerProviderAccount(context.Background(),
				&pb.ActivateSellerProviderAccountRequest{
					StoreId: pxStoreA, CutoverAt: "2026-08-30", ExpectedVersion: "0"})
			return err
		}},
		{"統編少一碼", func() error {
			_, err := s.PutSellerProviderCredentials(context.Background(),
				&pb.PutSellerProviderCredentialsRequest{
					StoreId: pxStoreA, SellerTaxId: "1234567", AppKey: pxAppKey,
					ConsentVersion: "v1", ExpectedVersion: "0"})
			return err
		}},
		{"AppKey 空", func() error {
			_, err := s.PutSellerProviderCredentials(context.Background(),
				&pb.PutSellerProviderCredentialsRequest{
					StoreId: pxStoreA, SellerTaxId: pxTaxID, AppKey: "   ",
					ConsentVersion: "v1", ExpectedVersion: "0"})
			return err
		}},
		{"ADM 缺 confirmed", func() error {
			_, err := s.RevokeProviderAccountForAdmin(context.Background(),
				&pb.RevokeProviderAccountForAdminRequest{
					ActorAccountId: 42, StoreId: "2685914682", Reason: "x",
					Confirmed: false, ExpectedVersion: "0"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("契約違規應回 InvalidArgument（400），實得 %v", err)
			}
		})
	}
}

// VoidInvoice 讓 pxProvider 滿足加寬後的 usecase.ProviderClient。
// 這個 fake 服務的是 provider account 端點的測試，不該打到 f0501 ⇒ 一律拒絕。
func (pxProvider) VoidInvoice(context.Context, amego.Credential, amego.VoidRequest) (*amego.VoidAccepted, error) {
	return nil, errors.New("pxProvider: unexpected VoidInvoice")
}

// 折讓兩支同理（#3073）：憑證驗證路徑不得產生任何憑證，打到就是分派錯了。
func (pxProvider) IssueAllowance(context.Context, amego.Credential, amego.AllowanceRequest) (*amego.AllowanceResult, error) {
	return nil, errors.New("pxProvider: unexpected IssueAllowance")
}

func (pxProvider) QueryAllowance(context.Context, amego.Credential, string) (*amego.AllowanceDetail, error) {
	return nil, errors.New("pxProvider: unexpected QueryAllowance")
}

// QueryInvoiceByNumber 讓 pxProvider 滿足加寬後的 usecase.ProviderClient。唯讀查詢，
// 這組測試不該用到 ⇒ 一律拒絕（fake 的鑑別力來自它拒絕什麼）。
func (pxProvider) QueryInvoiceByNumber(context.Context, amego.Credential, string) (*amego.InvoiceDetail, error) {
	return nil, errors.New("pxProvider: unexpected QueryInvoiceByNumber")
}
