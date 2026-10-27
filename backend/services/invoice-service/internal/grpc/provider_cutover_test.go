package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔驗端點 46（A09 自動開立切點歷史）。
//
// 兩件事在這裡才看得到：
//   ① 查無 provider account 回 **9330** 而不是空清單
//   ② `deactivatedAt` 為空字串 ＝ 至今仍有效（proto3 沒有 null），前端據此區分現行與歷史

const (
	pcStoreID   = int64(2685914682)
	pcIssuerID  = int64(900)
	pcAccountID = int64(77)
)

var pcNow = time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)

func pcQuery(rows []domain.ProviderCutover, withAccount bool) *usecase.Query {
	accounts := qAccounts{byIssuer: map[int64]domain.ProviderAccount{}}
	if withAccount {
		accounts.byStore = []qStoreAccount{{
			storeID: pcStoreID,
			account: domain.ProviderAccount{ID: pcAccountID, IssuerID: pcIssuerID, Provider: "AMEGO", Status: "ACTIVE"},
		}}
	}
	return usecase.NewQuery(
		qIssuers{}, qProfiles{}, accounts, qPrefs{}, qDocuments{}, qOperations{}, qComplaints{},
		&qAudit{}, qAdjustments{}, qCorrections{}, qOverview{}, qBizDays{},
		qCutovers{byAccount: map[int64][]domain.ProviderCutover{pcAccountID: rows}},
		qUnlocks{},
		qCancelled{},
		usecase.IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		nil,
	)
}

func TestListProviderCutovers_Success(t *testing.T) {
	deactivated := pcNow.Add(-24 * time.Hour)
	rows := []domain.ProviderCutover{
		{ID: 2, ProviderAccountID: pcAccountID, IssuerID: pcIssuerID, AccountVersion: 9,
			CutoverAt: pcNow.Add(-time.Hour), Mode: "AMEGO_AUTO",
			CreatedByAccountID: pcStoreID, CreatedAt: pcNow.Add(-2 * time.Hour)},
		{ID: 1, ProviderAccountID: pcAccountID, IssuerID: pcIssuerID, AccountVersion: 7,
			CutoverAt: pcNow.Add(-48 * time.Hour), DeactivatedAt: &deactivated, Mode: "AMEGO_AUTO",
			CreatedByAccountID: pcStoreID, CreatedAt: pcNow.Add(-72 * time.Hour)},
	}
	s := NewInvoiceServer(nil, pcQuery(rows, true), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.ListProviderCutoversForAdmin(context.Background(), &pb.ListProviderCutoversForAdminRequest{
		ActorAccountId: 90001, StoreId: "2685914682", Page: "1", PageSize: "20",
	})
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("成功應為 code 0，實得 %d", resp.GetCode())
	}
	got := resp.GetData().GetRecords()
	if len(got) != 2 || resp.GetData().GetTotal() != "2" {
		t.Fatalf("應回兩列，實得 %d 列 total=%s", len(got), resp.GetData().GetTotal())
	}
	// 現行有效那一列的 deactivatedAt 必須是空字串——前端靠它辨識「現在生效的切點是哪一個」。
	if got[0].GetDeactivatedAt() != "" {
		t.Fatalf("現行列的 deactivatedAt 應為空字串，實得 %q", got[0].GetDeactivatedAt())
	}
	if got[1].GetDeactivatedAt() == "" {
		t.Fatal("已停用那一列必須帶 deactivatedAt，否則前端會顯示成兩個都生效")
	}
	if got[0].GetVersion() != "9" {
		t.Fatalf("version 應是帳戶當時那一版，實得 %q", got[0].GetVersion())
	}
	if got[0].GetMode() != "AMEGO_AUTO" {
		t.Fatalf("mode 不符：%q", got[0].GetMode())
	}
	if got[0].GetCreatedByAccountId() != "2685914682" {
		t.Fatalf("createdBy 不符：%q", got[0].GetCreatedByAccountId())
	}
}

// 🔴 沒有 provider account ⇒ 9330，不是空清單：兩者對 ADM 的下一步不同
// （前者去看 A04 串接狀態，後者等賣家自己按端點 26）。
func TestListProviderCutovers_NoAccountReturns9330(t *testing.T) {
	s := NewInvoiceServer(nil, pcQuery(nil, false), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.ListProviderCutoversForAdmin(context.Background(), &pb.ListProviderCutoversForAdminRequest{
		ActorAccountId: 90001, StoreId: "2685914682",
	})
	if err != nil {
		t.Fatalf("業務碼不得走 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceProviderAccountNotFound {
		t.Fatalf("應回 9330，實得 %d", resp.GetCode())
	}
	if len(resp.GetData().GetRecords()) != 0 {
		t.Fatal("9330 不得帶任何列")
	}
}

// 帳戶存在但一次都沒設過切點 ⇒ 空 records ＋ total=0，**那是正常結果不是 9330**。
func TestListProviderCutovers_AccountWithoutHistoryIsEmptyNotError(t *testing.T) {
	s := NewInvoiceServer(nil, pcQuery(nil, true), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.ListProviderCutoversForAdmin(context.Background(), &pb.ListProviderCutoversForAdminRequest{
		ActorAccountId: 90001, StoreId: "2685914682",
	})
	if err != nil {
		t.Fatalf("不應回 gRPC error：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("應回 code 0，實得 %d", resp.GetCode())
	}
	if resp.GetData().GetTotal() != "0" || len(resp.GetData().GetRecords()) != 0 {
		t.Fatalf("應是空清單，實得 total=%s len=%d", resp.GetData().GetTotal(), len(resp.GetData().GetRecords()))
	}
}

func TestListProviderCutovers_RejectsMissingActor(t *testing.T) {
	s := NewInvoiceServer(nil, pcQuery(nil, true), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if _, err := s.ListProviderCutoversForAdmin(context.Background(), &pb.ListProviderCutoversForAdminRequest{
		StoreId: "2685914682",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 actor_account_id 應回 InvalidArgument，實得 %v", err)
	}
}
