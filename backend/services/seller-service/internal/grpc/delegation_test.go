package grpc

// #2744：pkg/delegation.Validate 自身的拒絕（S1a–S5）一律改發 9289
// DELEGATION_INTERNAL_CONTRACT_MISMATCH，不再分散在 9260／9262。
//
// 理由：DelegationContext 由 Gateway 依工作階段組裝，操作者碰不到它。缺 session_id、
// capability 不在封閉列舉內、capability 與本 RPC 宣告不符 —— 每一條都是「我方組錯」，
// 而 9260 說「你填錯了」、9262 說「你沒有這個權限」，兩者都把操作者送去做一件不會
// 改變任何事的工作。9262 更嚴重：它會以 Result=FAILED 寫進 admin_operation_audit_logs，
// 讓稽核列指控一位其實沒做錯任何事的管理員。
//
// ⚠️ 本檔**沒有**全部改：領域服務自己的欄位驗證（pkg/delegation.InvalidCommand）仍是
// 9260，真正的能力拒絕仍是 9262。只有 Validate 內部那幾條變了。

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"google.golang.org/grpc/status"
)

const delegStoreID = int64(2685914682)

// storefrontWriteCtx has audit_log_id populated because S1b requires it on
// .write; storefrontReadCtx deliberately does not, which is legal for .read and
// is exactly fixture E13 — the shape GET .../storefront arrives in.
func storefrontWriteCtx(cap string) *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       "7100000000000001",
		OperatorAdminId: "9100000000000001",
		Capability:      cap,
		OperationReason: "協助賣家補齊店面介紹",
		AuditLogId:      "8100000000000001",
	}
}

func storefrontReadCtx(cap string) *commonpb.DelegationContext {
	return &commonpb.DelegationContext{
		SessionId:       "7100000000000001",
		OperatorAdminId: "9100000000000001",
		Capability:      cap,
	}
}

// assertDelegationBizStatus asserts the error is a gRPC status carrying the
// "[NNNN] " prefix the Gateway parses back into an envelope code. Asserting only
// that an error occurred would pass identically for all five rejection steps.
func assertDelegationBizStatus(t *testing.T, err error, wantCode int32) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error carrying business code %d, got nil", wantCode)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %T: %v", err, err)
	}
	want := "[" + strconv.FormatInt(int64(wantCode), 10) + "] "
	if !strings.HasPrefix(st.Message(), want) {
		t.Fatalf("message = %q, want prefix %q", st.Message(), want)
	}
}

// casArgsContain reports whether any recorded write carried expectedVersion == want.
func casArgsContain(args []*int64, want int64) bool {
	for _, a := range args {
		if a != nil && *a == want {
			return true
		}
	}
	return false
}

// allCASArgsNil reports whether every recorded write ran without a CAS predicate.
func allCASArgsNil(args []*int64) bool {
	for _, a := range args {
		if a != nil {
			return false
		}
	}
	return true
}

// delegAccountID is the login principal of the fixture seller and is
// deliberately NOT equal to delegStoreID.
//
// 兩者相等（首任賣家）是這個 repo 裡最常見的資料形狀，也正因如此，一支把 store_id
// 填進 account_id 欄位的代行寫入可以通過全部測試、上線後才寫到別家店。fixture 只要
// 讓兩軸同值，這一整類錯誤就沒有任何測試看得見。
const delegAccountID = int64(2600000001)

// delegTestRepo wires both resolvers, each answering only for its own axis.
// Feeding the wrong id yields not-found instead of quietly returning the same
// row — that is what keeps the two axes distinguishable in assertions.
func delegTestRepo(version int64) *testSellerRepo {
	return &testSellerRepo{
		getByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != delegAccountID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return delegTestSeller(version), nil
		},
		getCurrentByStoreIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != delegStoreID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return delegTestSeller(version), nil
		},
	}
}

func delegTestSeller(version int64) *domain.Seller {
	now := time.Now()
	return &domain.Seller{
		AccountID: delegAccountID,
		StoreID:   delegStoreID,
		StoreName: "梨山盛花園",
		Status:    domain.SellerStatusActive,
		Version:   version,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ==================== GetSeller: storefront.read ====================

// E13: a delegated read with an empty audit_log_id must pass. If S1b were made
// unconditional, every delegated read endpoint would fail closed on day one
// while the write-only fixtures kept the suite green.
func TestGetSeller_DelegatedRead_EmptyAuditLogID_Allowed(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			s := delegTestSeller(7)
			s.AccountID = id
			return s, nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{
		AccountId:  delegStoreID,
		Delegation: storefrontReadCtx("storefront.read"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, want 0", resp.GetCode())
	}
	// The delegated storefront editor reads version here and sends it back as
	// expectedVersion; an empty or "0" value would silently disable the CAS.
	if got := resp.GetData().GetVersion(); got != "7" {
		t.Fatalf("version = %q, want \"7\"", got)
	}
}

func TestGetSeller_Delegated_WrongCapability_Denied(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(_ context.Context, _ int64) (*domain.Seller, error) {
			return delegTestSeller(1), nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	// posts.read is a legal token (S2 passes) but not what this RPC declares.
	_, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{
		AccountId:  delegStoreID,
		Delegation: storefrontReadCtx("posts.read"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

func TestGetSeller_Delegated_UnknownCapability_Denied(t *testing.T) {
	repo := &testSellerRepo{
		getByAccountIDFn: func(_ context.Context, _ int64) (*domain.Seller, error) {
			return delegTestSeller(1), nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	// Upper case is a different token: the allow-list is compared verbatim, so
	// the Gateway cannot mint a spelling the admin front end does not know.
	_, err := client.GetSeller(context.Background(), &pb.GetSellerRequest{
		AccountId:  delegStoreID,
		Delegation: storefrontReadCtx("STOREFRONT.READ"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

// ==================== UpdateSeller: storefront.write ====================

// S1b: a mutating capability with no audit_log_id means the two-phase audit was
// bypassed. Rejecting is the only correct answer — treating it as "the seller's
// own operation" would let the bypass through.
func TestUpdateSeller_Delegated_MissingAuditLogID_Denied(t *testing.T) {
	repo := &testSellerRepo{}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	d := storefrontWriteCtx("storefront.write")
	d.AuditLogId = ""
	_, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		StoreId:    delegStoreID,
		Name:       "新店名",
		Delegation: d,
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

func TestUpdateSeller_Delegated_MissingOwnerKey_Denied(t *testing.T) {
	repo := &testSellerRepo{}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		Name:       "新店名",
		Delegation: storefrontWriteCtx("storefront.write"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInvalidCommand)
}

func TestUpdateSeller_Delegated_ExpectedVersionReachesRepository(t *testing.T) {
	repo := delegTestRepo(7)
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	if _, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		StoreId:         delegStoreID,
		Name:            "梨山盛花園",
		ExpectedVersion: "7",
		Delegation:      storefrontWriteCtx("storefront.write"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Not "the last write": GetByAccountID may follow up with its own
	// contact_phone backfill, which legitimately carries no expectedVersion.
	if !casArgsContain(repo.casArgs, 7) {
		t.Fatalf("no write carried expectedVersion 7 (recorded: %d writes) — the CAS predicate was disabled, so a delegated write would overwrite the seller's concurrent edit with no error", len(repo.casArgs))
	}
	assertWritesTargetStore(t, repo, delegStoreID)
}

func TestUpdateSeller_SellerOwnPath_NoCAS(t *testing.T) {
	repo := delegTestRepo(7)
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	if _, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		AccountId: delegAccountID,
		Name:      "梨山盛花園",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allCASArgsNil(repo.casArgs) {
		t.Fatal("the seller's own path must not send a CAS predicate — it has no expectedVersion to compare against")
	}
	// 賣家自己的路徑用 account 軸「找到」那一列，但寫入鍵仍是該列的 store_id。
	assertWritesTargetStore(t, repo, delegStoreID)
}

// 代行請求填錯欄位（把 store_id 塞進 account_id）必須被擋下來，而不是照著寫。
// S4 的 owner-key 檢查讀的是 store_id，所以一個沒填 store_id 的代行寫入 fail-closed。
func TestUpdateSeller_Delegated_StoreIDInAccountIDField_Denied(t *testing.T) {
	repo := delegTestRepo(7)
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		AccountId:       delegStoreID, // 錯軸：這是 store_id
		Name:            "梨山盛花園",
		ExpectedVersion: "7",
		Delegation:      storefrontWriteCtx("storefront.write"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
	if len(repo.writeStoreIDs) != 0 {
		t.Fatalf("a delegated write with no store_id reached the repository (%v) — it would have landed on "+
			"whichever stint has account_id = %d", repo.writeStoreIDs, delegStoreID)
	}
}

// 兩軸同時給是矛盾的指令，不得由服務端自行挑一個。
func TestUpdateSeller_BothIDAxes_Denied(t *testing.T) {
	repo := delegTestRepo(7)
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		AccountId: delegAccountID,
		StoreId:   delegStoreID,
		Name:      "梨山盛花園",
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInvalidCommand)
	if len(repo.writeStoreIDs) != 0 {
		t.Fatalf("an ambiguous request still wrote: %v", repo.writeStoreIDs)
	}
}

func assertWritesTargetStore(t *testing.T, repo *testSellerRepo, want int64) {
	t.Helper()
	if len(repo.writeStoreIDs) == 0 {
		t.Fatal("no write reached the repository at all")
	}
	for i, got := range repo.writeStoreIDs {
		if got != want {
			t.Fatalf("write #%d keyed on store_id = %d, want %d — the write must land on the targeted stint",
				i, got, want)
		}
	}
}

func TestUpdateSeller_Delegated_VersionConflict_Returns9266(t *testing.T) {
	repo := delegTestRepo(7)
	repo.updateFn = func(_ context.Context, _ *domain.Seller) error {
		// What the repository produces when the CAS matched zero rows.
		return domain.ErrVersionConflict
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	resp, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		StoreId:         delegStoreID,
		Name:            "梨山盛花園",
		ExpectedVersion: "3",
		Delegation:      storefrontWriteCtx("storefront.write"),
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.DelegationResourceVersionConflict {
		t.Fatalf("code = %d, want %d (DELEGATION_RESOURCE_VERSION_CONFLICT)",
			resp.GetCode(), bizcode.DelegationResourceVersionConflict)
	}
}

func TestUpdateSeller_Delegated_MalformedExpectedVersion_Denied(t *testing.T) {
	repo := delegTestRepo(7)
	client, cleanup := newTestGRPCClient(t, newSellerServer(repo, nil))
	defer cleanup()

	_, err := client.UpdateSeller(context.Background(), &pb.UpdateSellerRequest{
		StoreId:         delegStoreID,
		Name:            "梨山盛花園",
		ExpectedVersion: "-1",
		Delegation:      storefrontWriteCtx("storefront.write"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInvalidCommand)
	if len(repo.casArgs) != 0 {
		t.Fatal("a malformed expectedVersion must not reach the repository at all")
	}
}

// ==================== UpdateLogo / UpdateCover ====================

// logo and cover are storefront endpoints, so they take storefront.write —
// not some separate media capability — and they carry expectedVersion too.
func TestUpdateLogo_Delegated_WrongCapability_Denied(t *testing.T) {
	client, cleanup := newTestGRPCClient(t, newSellerServer(&testSellerRepo{}, nil))
	defer cleanup()

	_, err := client.UpdateLogo(context.Background(), &pb.UpdateLogoRequest{
		StoreId:     delegStoreID,
		StagedToken: "stg_x",
		Delegation:  storefrontWriteCtx("products.write"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}

func TestUpdateCover_Delegated_WrongCapability_Denied(t *testing.T) {
	client, cleanup := newTestGRPCClient(t, newSellerServer(&testSellerRepo{}, nil))
	defer cleanup()

	_, err := client.UpdateCover(context.Background(), &pb.UpdateCoverRequest{
		StoreId:     delegStoreID,
		StagedToken: "stg_x",
		Delegation:  storefrontWriteCtx("products.write"),
	})
	assertDelegationBizStatus(t, err, bizcode.DelegationInternalContractMismatch)
}
