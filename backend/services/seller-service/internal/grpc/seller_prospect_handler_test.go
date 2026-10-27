package grpc

import (
	"context"
	"testing"
	"time"

	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

type stubProspectSearcher struct {
	hits      []domain.ProspectSearchHit
	gotParams domain.ProspectSearchParams
}

func (s *stubProspectSearcher) Search(_ context.Context, p domain.ProspectSearchParams) ([]domain.ProspectSearchHit, error) {
	s.gotParams = p
	return s.hits, nil
}

func TestSearchSellerProspects_ProjectsHitFlagsAndStringifiesID(t *testing.T) {
	stub := &stubProspectSearcher{hits: []domain.ProspectSearchHit{{
		Prospect:    domain.SellerProspect{ID: 7001, DisplayName: "梓官區漁會", ProductSummary: "虱目魚"},
		HitKeywords: true,
	}}}
	srv := &SellerServer{prospectSearcher: stub}

	resp, err := srv.SearchSellerProspects(context.Background(), &sellerpb.SearchSellerProspectsRequest{
		Keywords: "虱目魚",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetData()) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(resp.GetData()))
	}
	hit := resp.GetData()[0]
	if hit.GetProspect().GetId() != "7001" {
		t.Errorf("id = %q, want \"7001\"（依規範以字串承載）", hit.GetProspect().GetId())
	}
	if !hit.GetHitKeywords() || hit.GetHitCategory() || hit.GetHitRegion() {
		t.Errorf("facet 旗標投影錯誤: %+v", hit)
	}
	if stub.gotParams.Keywords != "虱目魚" {
		t.Errorf("Keywords = %q, want 虱目魚", stub.gotParams.Keywords)
	}
}

func TestSearchSellerProspects_ZeroLimitFallsBackToServiceDefault(t *testing.T) {
	stub := &stubProspectSearcher{}
	srv := &SellerServer{prospectSearcher: stub}

	if _, err := srv.SearchSellerProspects(context.Background(), &sellerpb.SearchSellerProspectsRequest{
		Keywords: "香蕉", Limit: 0,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotParams.Limit != 0 {
		t.Errorf("Limit = %d, want 0 交由 repository 套用預設", stub.gotParams.Limit)
	}
}

// stubProspectCreator 讓 CreateSellerProspect 的測試不必組出整個
// usecase／repository 依賴；createCalled 用來證明 dry_run=true 時
// handler 真的沒有走到寫入這一步。
type stubProspectCreator struct {
	matches      []usecase.ProspectMatchCandidate
	matchErr     error
	createCalled bool
	createIn     domain.SellerProspect
	createOut    domain.SellerProspect
	createErr    error
}

func (s *stubProspectCreator) Match(_ context.Context, _ domain.ProspectContactInput) ([]usecase.ProspectMatchCandidate, error) {
	return s.matches, s.matchErr
}

func (s *stubProspectCreator) Create(_ context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	s.createCalled = true
	s.createIn = in
	return s.createOut, s.createErr
}

// dry_run 是遷移提案階段與階段二 admin 表單即時查重共用的路徑；
// 只要這個旗標是 true，handler 絕不能呼叫 Create——這條測試直接斷言
// Create 沒被呼叫，而不只是斷言回應形狀符合預期。
func TestCreateSellerProspect_DryRunDoesNotWrite(t *testing.T) {
	stub := &stubProspectCreator{matches: []usecase.ProspectMatchCandidate{{
		Prospect:  domain.SellerProspect{ID: 42, DisplayName: "既有業者"},
		Tier:      usecase.ProspectMatchTierTaxID,
		MatchedOn: "tax_id",
	}}}
	srv := &SellerServer{prospectCreator: stub}

	resp, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
		DisplayName: "測試漁行",
		TaxId:       "12345678",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.createCalled {
		t.Fatalf("dry_run=true 不應呼叫 Create，但被呼叫了，in=%+v", stub.createIn)
	}
	if resp.GetData() != nil {
		t.Errorf("dry_run 回應的 Data 應為空，got %+v", resp.GetData())
	}
	if len(resp.GetMatches()) != 1 || resp.GetMatches()[0].GetProspect().GetId() != "42" {
		t.Errorf("Matches 投影錯誤: %+v", resp.GetMatches())
	}
	if got := resp.GetMatches()[0].GetTier(); got != "T1_TAX_ID" {
		t.Errorf("Tier = %q, want \"T1_TAX_ID\"（上線只會出現 T1_TAX_ID 或 T2_CONTACT）", got)
	}
}

// 對照組：dry_run=false 時必須真的呼叫到 Create，確保上面那條測試
// 不是靠 handler 整條路徑都沒接上而巧合通過。
func TestCreateSellerProspect_NonDryRunCallsCreate(t *testing.T) {
	stub := &stubProspectCreator{
		createOut: domain.SellerProspect{ID: 99, DisplayName: "測試漁行"},
	}
	srv := &SellerServer{prospectCreator: stub}

	resp, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
		DisplayName: "測試漁行",
		TaxId:       "12345678",
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.createCalled {
		t.Fatalf("dry_run=false 應該呼叫 Create，但沒有")
	}
	if resp.GetData().GetId() != "99" {
		t.Errorf("id = %q, want \"99\"", resp.GetData().GetId())
	}
	if stub.createIn.TaxID == nil || *stub.createIn.TaxID != "12345678" {
		t.Errorf("createIn.TaxID = %v, want \"12345678\"", stub.createIn.TaxID)
	}
}

// atoiOrZero 的核心承諾：星等是排序的次要訊號，格式解不出來絕不能讓
// 整筆業者資料建不進去。直接送一個非數字字串，斷言 Create 仍被呼叫、
// 呼叫仍成功（不是只看解析結果，而是連 error 都要斷言為 nil），且
// 傳進 Create 的星等被容錯成 0。
func TestCreateSellerProspect_GarbagePriorityStarsDoesNotFailCreate(t *testing.T) {
	stub := &stubProspectCreator{
		createOut: domain.SellerProspect{ID: 100, DisplayName: "測試漁行"},
	}
	srv := &SellerServer{prospectCreator: stub}

	if _, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
		DisplayName:   "測試漁行",
		PriorityStars: "not-a-number",
	}); err != nil {
		t.Fatalf("PriorityStars 格式錯誤不應讓 create 失敗，卻回傳 error: %v", err)
	}
	if !stub.createCalled {
		t.Fatalf("PriorityStars 格式錯誤不應阻止呼叫 Create，但沒被呼叫")
	}
	if stub.createIn.PriorityStars != 0 {
		t.Errorf("createIn.PriorityStars = %d, want 0", stub.createIn.PriorityStars)
	}
}

// 對照組：確保上一條測試不是靠一個永遠回 0 的 atoiOrZero 巧合通過——
// 合法數字字串必須真的被解析成對應的整數才送進 Create。
func TestCreateSellerProspect_PriorityStarsParsesToInt(t *testing.T) {
	stub := &stubProspectCreator{
		createOut: domain.SellerProspect{ID: 101, DisplayName: "測試漁行"},
	}
	srv := &SellerServer{prospectCreator: stub}

	if _, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
		DisplayName:   "測試漁行",
		PriorityStars: "3",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.createIn.PriorityStars != 3 {
		t.Errorf("createIn.PriorityStars = %d, want 3", stub.createIn.PriorityStars)
	}
}

// 出向反方向：domain 端的整數星等經 toProspectItemPB 投影回 wire 上的
// 字串，必須是對應的十進位字串，不是空字串或原始整數的其他表示法。
func TestSearchSellerProspects_StringifiesPriorityStars(t *testing.T) {
	stub := &stubProspectSearcher{hits: []domain.ProspectSearchHit{{
		Prospect: domain.SellerProspect{ID: 8001, DisplayName: "測試魚行", PriorityStars: 3},
	}}}
	srv := &SellerServer{prospectSearcher: stub}

	resp, err := srv.SearchSellerProspects(context.Background(), &sellerpb.SearchSellerProspectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.GetData()[0].GetProspect().GetPriorityStars(); got != "3" {
		t.Errorf("PriorityStars = %q, want \"3\"", got)
	}
}

// TestSearchSellerProspects_PIIFieldsEmptyInSearchResponse 釘住 I2：
// SearchSellerProspects 的呼叫端是 ai-service（doc 55 §8.1 明文禁止候選聯絡
// 方式進入該模組任何一層）。stub 刻意把 PublicEmail／PublicPhone／PublicSocial
// 填非空值，證明是 handler 主動清空、不是「剛好沒被填過」而巧合通過。
// PublicWebsite 不受影響——它是候選卡的可驗證來源連結，允許留在 search 回應。
func TestSearchSellerProspects_PIIFieldsEmptyInSearchResponse(t *testing.T) {
	stub := &stubProspectSearcher{hits: []domain.ProspectSearchHit{{
		Prospect: domain.SellerProspect{
			ID: 9001, DisplayName: "測試魚行",
			PublicEmail: "leak@example.com", PublicPhone: "0912345678",
			PublicSocial: "threads:leak", PublicWebsite: "https://example.com",
		},
	}}}
	srv := &SellerServer{prospectSearcher: stub}

	resp, err := srv.SearchSellerProspects(context.Background(), &sellerpb.SearchSellerProspectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	item := resp.GetData()[0].GetProspect()
	if item.GetPublicEmail() != "" || item.GetPublicPhone() != "" || item.GetPublicSocial() != "" {
		t.Fatalf("PII 欄位必須在 search 回應中留空，got email=%q phone=%q social=%q",
			item.GetPublicEmail(), item.GetPublicPhone(), item.GetPublicSocial())
	}
	if item.GetPublicWebsite() != "https://example.com" {
		t.Errorf("PublicWebsite 不該被這條窄化清空，got %q", item.GetPublicWebsite())
	}
}

// TestCreateSellerProspect_CreatedByAccountIdReachesCreateInput 釘住 I1：
// created_by_account_id／updated_by_account_id 過去從未被 handler 設值，
// DB 稽核欄位永遠是 NULL。這裡直接斷言 Create 收到的 domain.SellerProspect
// 帶了正確的兩個指標值，而不只是回應形狀符合預期。
func TestCreateSellerProspect_CreatedByAccountIdReachesCreateInput(t *testing.T) {
	stub := &stubProspectCreator{createOut: domain.SellerProspect{ID: 200, DisplayName: "測試魚行"}}
	srv := &SellerServer{prospectCreator: stub}

	if _, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
		DisplayName:        "測試魚行",
		CreatedByAccountId: "555001",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.createIn.CreatedByAccountID == nil || *stub.createIn.CreatedByAccountID != 555001 {
		t.Errorf("CreatedByAccountID = %v, want *555001", stub.createIn.CreatedByAccountID)
	}
	if stub.createIn.UpdatedByAccountID == nil || *stub.createIn.UpdatedByAccountID != 555001 {
		t.Errorf("UpdatedByAccountID = %v, want *555001", stub.createIn.UpdatedByAccountID)
	}
}

// 對照組：空字串（遷移腳本送的值）與非數字／0 都必須讓兩個稽核欄位維持 nil，
// 不是把 0 這個假身分寫進 DB——同 GrantedByAccountId 在 buyer-service 的既有
// 處置（AssignMembership），格式問題不該讓整筆業者資料建不進去。
func TestCreateSellerProspect_EmptyOrZeroCreatedByAccountIdLeavesAuditFieldsNil(t *testing.T) {
	for _, raw := range []string{"", "0", "not-a-number"} {
		stub := &stubProspectCreator{createOut: domain.SellerProspect{ID: 201, DisplayName: "測試魚行"}}
		srv := &SellerServer{prospectCreator: stub}

		if _, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
			DisplayName:        "測試魚行",
			CreatedByAccountId: raw,
		}); err != nil {
			t.Fatalf("raw=%q: unexpected error: %v", raw, err)
		}
		if stub.createIn.CreatedByAccountID != nil {
			t.Errorf("raw=%q: CreatedByAccountID = %v, want nil", raw, *stub.createIn.CreatedByAccountID)
		}
		if stub.createIn.UpdatedByAccountID != nil {
			t.Errorf("raw=%q: UpdatedByAccountID = %v, want nil", raw, *stub.createIn.UpdatedByAccountID)
		}
	}
}

// TestCreateSellerProspect_FactsAsOfParsedIntoUpdatedAt 釘住 I5：遷移腳本送的
// facts_as_of 必須原樣落進 in.UpdatedAt，repository.Create 才有值可覆寫
// updated_at（見 postgres/seller_prospect.go 的 FactsAsOf 轉譯）。
func TestCreateSellerProspect_FactsAsOfParsedIntoUpdatedAt(t *testing.T) {
	stub := &stubProspectCreator{createOut: domain.SellerProspect{ID: 300, DisplayName: "測試魚行"}}
	srv := &SellerServer{prospectCreator: stub}

	want := time.Date(2026, 8, 12, 3, 4, 5, 0, time.UTC)
	if _, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
		DisplayName: "測試魚行",
		FactsAsOf:   want.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.createIn.UpdatedAt.Equal(want) {
		t.Errorf("UpdatedAt = %v, want %v", stub.createIn.UpdatedAt, want)
	}
}

// 對照組：空字串（階段二 admin 建檔的一般情況）與解不出來的字串都必須讓
// in.UpdatedAt 維持零值，落回 repository/SQL 端的 now()——不能因為這個
// 只給遷移用的欄位存在，就改變階段二既有路徑的行為。
func TestCreateSellerProspect_EmptyOrInvalidFactsAsOfLeavesUpdatedAtZero(t *testing.T) {
	for _, raw := range []string{"", "not-a-timestamp"} {
		stub := &stubProspectCreator{createOut: domain.SellerProspect{ID: 301, DisplayName: "測試魚行"}}
		srv := &SellerServer{prospectCreator: stub}

		if _, err := srv.CreateSellerProspect(context.Background(), &sellerpb.CreateSellerProspectRequest{
			DisplayName: "測試魚行",
			FactsAsOf:   raw,
		}); err != nil {
			t.Fatalf("raw=%q: unexpected error: %v", raw, err)
		}
		if !stub.createIn.UpdatedAt.IsZero() {
			t.Errorf("raw=%q: UpdatedAt = %v, want zero value", raw, stub.createIn.UpdatedAt)
		}
	}
}
