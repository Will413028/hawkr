package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// admin 投影必須帶識別鍵與狀態欄位——那正是它與 SellerProspectItem 的
// 全部差別。可空欄位以空字串表達，不是 "0" 或 "<nil>"。
func TestToAdminProspectItemPB_ProjectsNullableFields(t *testing.T) {
	taxID := "12345678"
	storeID := int64(9001)
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)

	full := toAdminProspectItemPB(domain.SellerProspect{
		ID: 1, DisplayName: "測試", VendorType: "business",
		Status: "CONVERTED", OutreachStatus: "CONVERTED",
		TaxID: &taxID, ConvertedStoreID: &storeID, ConvertedAt: &at,
		CreatedAt: at, UpdatedAt: at,
	})
	if full.GetTaxId() != "12345678" {
		t.Errorf("TaxId = %q, want %q", full.GetTaxId(), "12345678")
	}
	if full.GetConvertedStoreId() != "9001" {
		t.Errorf("ConvertedStoreId = %q, want %q", full.GetConvertedStoreId(), "9001")
	}
	if full.GetConvertedAt() != "2026-09-06T01:02:03Z" {
		t.Errorf("ConvertedAt = %q, want RFC3339 UTC", full.GetConvertedAt())
	}

	empty := toAdminProspectItemPB(domain.SellerProspect{
		ID: 2, Status: "ACTIVE", CreatedAt: at, UpdatedAt: at,
	})
	for name, got := range map[string]string{
		"TaxId":                empty.GetTaxId(),
		"ConvertedStoreId":     empty.GetConvertedStoreId(),
		"ConvertedAt":          empty.GetConvertedAt(),
		"MergedIntoProspectId": empty.GetMergedIntoProspectId(),
	} {
		if got != "" {
			t.Errorf("%s = %q, want empty string for nil field", name, got)
		}
	}
}

// stubProspectAdmin 嵌入 prospectAdminUseCase 而不是完整實作它：只覆寫各
// 測試用得到的方法，其餘方法呼叫會透過 nil 的內嵌介面 panic，而不是悄悄
// 回零值假裝成功——漏寫一支測試的 override 會立刻在該測試炸開，不會被
// 誤判成「這支方法本來就該回零值」。
//
// 這個檔案目前把 prospectAdminUseCase 現有的六支方法全覆寫了，linter 因此
// 看不到任何呼叫真的落到內嵌欄位上，回報 unused；保留這個欄位而不是刪掉，
// 是因為 Task 12 會替介面加上 ListDuplicates／Merge／ConvertForAdmin／
// ListTaxIDMatches 四支新方法，屆時任何一支沒被新測試覆寫，都要靠這個
// 內嵌欄位在呼叫當下 panic，而不是刪了欄位讓 *stubProspectAdmin 直接編譯
// 不過或（更糟）被迫每次都補一支回零值的方法，把「這支方法本來就該回
// 零值」跟「這支方法忘了覆寫」混在一起。
type stubProspectAdmin struct {
	prospectAdminUseCase

	gotListParams domain.ProspectAdminListParams
	listRows      []domain.SellerProspect
	listTotal     int64

	getErr error

	setArchivedErr error

	revokeInviteErr error

	updateErr error

	createInviteErr error

	duplicatePairs     []usecase.ProspectDuplicatePair
	duplicateTruncated bool
	convertErr         error
	mergeErr           error

	// gotConvertOperatorID 記錄 AdminConvertSellerProspect 最後一次呼叫實際
	// 傳給 ConvertForAdmin 的 operatorID，供
	// TestAdminConvertSellerProspect_ForwardsOperatorAccountID 斷言 handler
	// 有解析 request 的 operator_account_id 並往下傳，不是漏讀（同 Update／
	// Archive／Merge 三支既有 RPC 都會讀這個欄位）。
	gotConvertOperatorID *int64
}

func (s *stubProspectAdmin) ListForAdmin(_ context.Context, p domain.ProspectAdminListParams) ([]domain.SellerProspect, int64, error) {
	s.gotListParams = p
	return s.listRows, s.listTotal, nil
}

func (s *stubProspectAdmin) GetForAdmin(_ context.Context, _ int64) (domain.SellerProspect, []domain.ProspectInvite, error) {
	return domain.SellerProspect{}, nil, s.getErr
}

func (s *stubProspectAdmin) SetArchived(_ context.Context, _ int64, _ bool, _ *int64) error {
	return s.setArchivedErr
}

func (s *stubProspectAdmin) RevokeInvite(_ context.Context, _, _ int64) error {
	return s.revokeInviteErr
}

func (s *stubProspectAdmin) UpdateForAdmin(_ context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	return in, s.updateErr
}

func (s *stubProspectAdmin) CreateInvite(_ context.Context, _ int64, _ int) (domain.ProspectInvite, string, error) {
	return domain.ProspectInvite{}, "", s.createInviteErr
}

func (s *stubProspectAdmin) ListDuplicates(_ context.Context) ([]usecase.ProspectDuplicatePair, bool, error) {
	return s.duplicatePairs, s.duplicateTruncated, nil
}

func (s *stubProspectAdmin) ConvertForAdmin(_ context.Context, _, _ int64, operatorID *int64) error {
	s.gotConvertOperatorID = operatorID
	return s.convertErr
}

func (s *stubProspectAdmin) Merge(_ context.Context, _, _ int64, _ *int64) (domain.SellerProspect, error) {
	return domain.SellerProspect{}, s.mergeErr
}

// 未送 page／page_size 時退回第 1 頁、每頁 20——這兩個預設值是 wire 契約的
// 一部分，前端不送就是要這個結果，不是「未定義行為」。
func TestAdminListSellerProspects_DefaultsPaging(t *testing.T) {
	stub := &stubProspectAdmin{}
	srv := &SellerServer{prospectAdmin: stub}

	resp, err := srv.AdminListSellerProspects(context.Background(), &sellerpb.AdminListSellerProspectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotListParams.Page != 1 {
		t.Errorf("Page = %d, want 1", stub.gotListParams.Page)
	}
	if stub.gotListParams.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", stub.gotListParams.PageSize)
	}
	if resp.GetPage() != "1" || resp.GetPageSize() != "20" {
		t.Errorf("回應的分頁欄位 = (%q, %q)，want (\"1\", \"20\")", resp.GetPage(), resp.GetPageSize())
	}
}

// page_size 超過上限一律收斂到 500，而不是照送——沒有這道，一個
// page_size=100000 的請求就能把整份名單一次拉走。
func TestAdminListSellerProspects_CapsPageSize(t *testing.T) {
	stub := &stubProspectAdmin{}
	srv := &SellerServer{prospectAdmin: stub}

	if _, err := srv.AdminListSellerProspects(context.Background(), &sellerpb.AdminListSellerProspectsRequest{
		PageSize: "100000",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotListParams.PageSize != 500 {
		t.Errorf("PageSize = %d, want 500", stub.gotListParams.PageSize)
	}
}

func TestAdminArchiveSellerProspect_MapsErrProspectMergedToFailedPrecondition(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{setArchivedErr: usecase.ErrProspectMerged}}

	_, err := srv.AdminArchiveSellerProspect(context.Background(), &sellerpb.AdminArchiveSellerProspectRequest{
		Id: "1", Archived: false,
	})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition (err = %v)", got, err)
	}
}

func TestAdminRevokeProspectInvite_MapsNotRevocableToFailedPrecondition(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{revokeInviteErr: usecase.ErrProspectInviteNotRevocable}}

	_, err := srv.AdminRevokeProspectInvite(context.Background(), &sellerpb.AdminRevokeProspectInviteRequest{
		ProspectId: "1", InviteId: "2",
	})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition (err = %v)", got, err)
	}
}

// 查無此列是 NotFound 不是 Internal。GetByID 回的是 pgx.ErrNoRows（見
// domain.SellerProspectRepository 上的註解），handler 要用 errors.Is 辨識。
func TestAdminGetSellerProspect_MapsNoRowsToNotFound(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{getErr: pgx.ErrNoRows}}

	_, err := srv.AdminGetSellerProspect(context.Background(), &sellerpb.AdminGetSellerProspectRequest{Id: "1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("code = %v, want NotFound (err = %v)", got, err)
	}
}

// 反向：沒有被辨識的錯誤必須維持 Internal。少了這一條，一個把所有錯誤都
// 當成 FailedPrecondition 的 handler 也會讓上面三支測試通過，而真正的 DB
// 故障會被呈現成「操作被拒」，操作者以為是自己做錯了。
func TestAdminArchiveSellerProspect_KeepsGenuineFailureInternal(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{setArchivedErr: errors.New("connection reset by peer")}}

	_, err := srv.AdminArchiveSellerProspect(context.Background(), &sellerpb.AdminArchiveSellerProspectRequest{
		Id: "1", Archived: false,
	})
	if got := status.Code(err); got != codes.Internal {
		t.Errorf("code = %v, want Internal (err = %v)", got, err)
	}
}

// 還原撞 uq_seller_prospects_tax_id 要落在 AlreadyExists，不是 Internal——
// 操作者需要知道的事實是「這個統編已經被別人用了」，不是「系統錯誤」。
// usecase.SetArchived 把 repository 的 *domain.ErrDuplicate 轉成具名
// ErrProspectTaxIDConflict，這裡驗證 handler 的 prospectConflictStatus
// 有接住它——同 TestAdminArchiveSellerProspect_KeepsGenuineFailureInternal
// 互為正反對照。
func TestAdminArchiveSellerProspect_MapsTaxIDConflictToAlreadyExists(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{setArchivedErr: usecase.ErrProspectTaxIDConflict}}

	_, err := srv.AdminArchiveSellerProspect(context.Background(), &sellerpb.AdminArchiveSellerProspectRequest{
		Id: "1", Archived: false,
	})
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Errorf("code = %v, want AlreadyExists (err = %v)", got, err)
	}
}

// 對一個不存在的 id 建邀請是呼叫端送錯了 id，是客戶端錯誤不是伺服端故障。
// usecase.CreateInvite 內部第一步是 GetByID 確認 prospect 存在，查無資料時
// 冒出同一個 pgx.ErrNoRows，這裡驗證它被轉譯成 NotFound 而不是 Internal。
func TestAdminCreateProspectInvite_MapsNoRowsToNotFound(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{createInviteErr: pgx.ErrNoRows}}

	_, err := srv.AdminCreateProspectInvite(context.Background(), &sellerpb.AdminCreateProspectInviteRequest{ProspectId: "1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("code = %v, want NotFound (err = %v)", got, err)
	}
}

// 反向：CreateInvite 上沒被辨識的錯誤一樣要維持 Internal，同
// TestAdminArchiveSellerProspect_KeepsGenuineFailureInternal 的理由。
func TestAdminCreateProspectInvite_KeepsGenuineFailureInternal(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{createInviteErr: errors.New("connection reset by peer")}}

	_, err := srv.AdminCreateProspectInvite(context.Background(), &sellerpb.AdminCreateProspectInviteRequest{ProspectId: "1"})
	if got := status.Code(err); got != codes.Internal {
		t.Errorf("code = %v, want Internal (err = %v)", got, err)
	}
}

// UpdateForAdmin 的 SQL 是 `:one` 查詢、述詞是 status = 'ACTIVE'：id 不存在
// 或列存在但非 ACTIVE，兩種情況都回同一個 pgx.ErrNoRows，這裡驗證兩種都被
// 轉譯成 NotFound（訊息涵蓋兩種讀法，見 handler 上的註解），而不是 Internal。
func TestAdminUpdateSellerProspect_MapsNoRowsToNotFound(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{updateErr: pgx.ErrNoRows}}

	_, err := srv.AdminUpdateSellerProspect(context.Background(), &sellerpb.AdminUpdateSellerProspectRequest{Id: "1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("code = %v, want NotFound (err = %v)", got, err)
	}
}

// truncated 必須誠實回報。admin 看到 500 對而不知道還有更多，會以為自己
// 已經處理完了——而「重複清單回空」正是本輪 dev 驗收的判準。
func TestAdminListDuplicateProspects_ReportsTruncation(t *testing.T) {
	pairs := make([]usecase.ProspectDuplicatePair, 500)
	for i := range pairs {
		pairs[i] = usecase.ProspectDuplicatePair{
			Left:      domain.SellerProspect{ID: int64(i + 1)},
			Right:     domain.SellerProspect{ID: int64(i + 1001)},
			MatchedOn: "email",
		}
	}
	stub := &stubProspectAdmin{duplicatePairs: pairs, duplicateTruncated: true}
	srv := &SellerServer{prospectAdmin: stub}

	resp, err := srv.AdminListDuplicateProspects(context.Background(), &sellerpb.AdminListDuplicateProspectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetTruncated() {
		t.Error("Truncated = false, want true")
	}
	if len(resp.GetData()) != 500 {
		t.Errorf("len(data) = %d, want 500", len(resp.GetData()))
	}
	if resp.GetData()[0].GetMatchedOn() != "email" {
		t.Errorf("MatchedOn = %q, want %q", resp.GetData()[0].GetMatchedOn(), "email")
	}
}

// 歸因的兩種失敗要落在可辨識的 gRPC status 上。回 Internal 會讓 admin
// 看到「系統錯誤」而不是「這筆已經入駐了」——那是操作者需要知道的事實。
func TestAdminConvertSellerProspect_MapsDomainErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"store 不存在", usecase.ErrStoreNotFound, codes.InvalidArgument},
		{"prospect 非 ACTIVE", usecase.ErrProspectNotConvertible, codes.FailedPrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &SellerServer{prospectAdmin: &stubProspectAdmin{convertErr: tc.err}}

			_, err := srv.AdminConvertSellerProspect(context.Background(), &sellerpb.AdminConvertSellerProspectRequest{
				Id: "1", StoreId: "2",
			})
			if status.Code(err) != tc.want {
				t.Errorf("code = %v, want %v (err = %v)", status.Code(err), tc.want, err)
			}
		})
	}
}

// TestAdminConvertSellerProspect_ForwardsOperatorAccountID 是四支 admin 寫入
// RPC 裡最後一支才讀 operator_account_id 的回歸測試：手動歸因決定一筆真實
// store 認哪一筆 prospect 當前身，不可逆，spec §5.3／§6 要求記下操作者。
// 這裡只驗證 handler 有把 wire 上的字串解析出來往下傳——usecase 層的
// TestConvertForAdminRecordsOperatorAccountID 驗證它真的落地到
// updated_by_account_id，兩層各自驗證各自的職責，不重複。
func TestAdminConvertSellerProspect_ForwardsOperatorAccountID(t *testing.T) {
	stub := &stubProspectAdmin{}
	srv := &SellerServer{prospectAdmin: stub}

	_, err := srv.AdminConvertSellerProspect(context.Background(), &sellerpb.AdminConvertSellerProspectRequest{
		Id: "1", StoreId: "2", OperatorAccountId: "9001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotConvertOperatorID == nil || *stub.gotConvertOperatorID != 9001 {
		t.Errorf("gotConvertOperatorID = %v, want *9001", stub.gotConvertOperatorID)
	}
}

// 合併被拒不是系統故障。回 Internal 會讓 admin 看到「系統錯誤」，而實際發生的
// 是「這家業者已經入駐了，不能被合併掉」——那個區別正是這個畫面需要人來操作
// 的理由。真正的 DB 故障沒有包 sentinel，仍然是 Internal。
func TestAdminMergeSellerProspects_MapsRejectionToFailedPrecondition(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{
		mergeErr: fmt.Errorf("%w: prospect 1 is CONVERTED", usecase.ErrProspectMergeNotAllowed),
	}}

	_, err := srv.AdminMergeSellerProspects(context.Background(), &sellerpb.AdminMergeSellerProspectsRequest{
		WinnerId: "1", LoserId: "2",
	})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition (err = %v)", got, err)
	}
	// repository.Merge 為四種拒絕情境各寫一句具體原因，不能被壓成同一句
	// 泛用訊息——那會讓操作者無從分辨自己撞的是哪一種（4 種情境見
	// domain.SellerProspectRepository.Merge 的文件註解）。
	if !strings.Contains(err.Error(), "prospect 1 is CONVERTED") {
		t.Errorf("err = %v, want message to contain the specific rejection reason from the repository", err)
	}
}

// 沒有包 sentinel 的錯誤必須維持 Internal——否則一個真正的 DB 故障會被當成
// 「操作被拒」呈現給操作者，他會以為是自己做錯了。
func TestAdminMergeSellerProspects_KeepsGenuineFailureInternal(t *testing.T) {
	srv := &SellerServer{prospectAdmin: &stubProspectAdmin{
		mergeErr: errors.New("connection reset by peer"),
	}}

	_, err := srv.AdminMergeSellerProspects(context.Background(), &sellerpb.AdminMergeSellerProspectsRequest{
		WinnerId: "1", LoserId: "2",
	})
	if got := status.Code(err); got != codes.Internal {
		t.Errorf("code = %v, want Internal (err = %v)", got, err)
	}
}

// fullProspectAdminStub 實作 prospectAdminUseCase 的每一支方法，回零值。
// 刻意不嵌入介面：嵌入的話漏實作會是 nil panic，而這支測試要分辨的正是
// 「handler 有沒有被實作」，不能讓 stub 自己的漏洞混進訊號裡。
type fullProspectAdminStub struct{}

func (fullProspectAdminStub) ListForAdmin(context.Context, domain.ProspectAdminListParams) ([]domain.SellerProspect, int64, error) {
	return nil, 0, nil
}

func (fullProspectAdminStub) GetForAdmin(context.Context, int64) (domain.SellerProspect, []domain.ProspectInvite, error) {
	return domain.SellerProspect{}, nil, nil
}

func (fullProspectAdminStub) UpdateForAdmin(_ context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	return in, nil
}

func (fullProspectAdminStub) SetArchived(context.Context, int64, bool, *int64) error {
	return nil
}

func (fullProspectAdminStub) CreateInvite(context.Context, int64, int) (domain.ProspectInvite, string, error) {
	return domain.ProspectInvite{}, "", nil
}

func (fullProspectAdminStub) RevokeInvite(context.Context, int64, int64) error {
	return nil
}

func (fullProspectAdminStub) UpdateProspectMedia(context.Context, usecase.ProspectMediaInput) (domain.SellerProspect, error) {
	return domain.SellerProspect{}, nil
}

func (fullProspectAdminStub) ListDuplicates(context.Context) ([]usecase.ProspectDuplicatePair, bool, error) {
	return nil, false, nil
}

func (fullProspectAdminStub) Merge(context.Context, int64, int64, *int64) (domain.SellerProspect, error) {
	return domain.SellerProspect{}, nil
}

func (fullProspectAdminStub) ConvertForAdmin(context.Context, int64, int64, *int64) error {
	return nil
}

func (fullProspectAdminStub) ListTaxIDMatches(context.Context) ([]domain.ProspectTaxIDMatch, error) {
	return nil, nil
}

// 每一支 admin RPC 都必須真的定義在 *SellerServer 上。handler.go:25 嵌入了
// pb.UnimplementedSellerServiceServer，所以漏一支或名字打錯時 go build 永遠
// 不會紅——只會在 runtime 回 Unimplemented。這支測試是那道缺失的編譯期
// 防線的替代品：任何一支沒被實作，promoted 方法會回 codes.Unimplemented。
func TestAdminProspectRPCsAreAllImplemented(t *testing.T) {
	srv := &SellerServer{prospectAdmin: fullProspectAdminStub{}}
	ctx := context.Background()

	calls := map[string]func() error{
		"AdminListSellerProspects": func() error {
			_, err := srv.AdminListSellerProspects(ctx, &sellerpb.AdminListSellerProspectsRequest{})
			return err
		},
		"AdminGetSellerProspect": func() error {
			_, err := srv.AdminGetSellerProspect(ctx, &sellerpb.AdminGetSellerProspectRequest{Id: "1"})
			return err
		},
		"AdminUpdateSellerProspect": func() error {
			_, err := srv.AdminUpdateSellerProspect(ctx, &sellerpb.AdminUpdateSellerProspectRequest{Id: "1", DisplayName: "x", VendorType: "business", OutreachStatus: "NOT_CONTACTED"})
			return err
		},
		"AdminArchiveSellerProspect": func() error {
			_, err := srv.AdminArchiveSellerProspect(ctx, &sellerpb.AdminArchiveSellerProspectRequest{Id: "1", Archived: true})
			return err
		},
		"AdminListDuplicateProspects": func() error {
			_, err := srv.AdminListDuplicateProspects(ctx, &sellerpb.AdminListDuplicateProspectsRequest{})
			return err
		},
		"AdminMergeSellerProspects": func() error {
			_, err := srv.AdminMergeSellerProspects(ctx, &sellerpb.AdminMergeSellerProspectsRequest{WinnerId: "1", LoserId: "2"})
			return err
		},
		"AdminConvertSellerProspect": func() error {
			_, err := srv.AdminConvertSellerProspect(ctx, &sellerpb.AdminConvertSellerProspectRequest{Id: "1", StoreId: "2"})
			return err
		},
		"AdminListProspectTaxIdMatches": func() error {
			_, err := srv.AdminListProspectTaxIdMatches(ctx, &sellerpb.AdminListProspectTaxIdMatchesRequest{})
			return err
		},
		"AdminCreateProspectInvite": func() error {
			_, err := srv.AdminCreateProspectInvite(ctx, &sellerpb.AdminCreateProspectInviteRequest{ProspectId: "1"})
			return err
		},
		"AdminRevokeProspectInvite": func() error {
			_, err := srv.AdminRevokeProspectInvite(ctx, &sellerpb.AdminRevokeProspectInviteRequest{ProspectId: "1", InviteId: "2"})
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if got := status.Code(call()); got == codes.Unimplemented {
				t.Errorf("%s 仍是從 UnimplementedSellerServiceServer 繼承來的——handler 沒實作或名字打錯", name)
			}
		})
	}
}
