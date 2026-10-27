package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// ==================== Router Setup ====================

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeJWTAuth is a minimal middleware that extracts accountId and accountType from headers
// (set by tests), like middleware.JWTAuth trusting the monolith's identity headers.
func fakeJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.GetHeader("X-Account-Id")
		if idStr == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "msg": "missing token"})
			return
		}
		var accountID int64
		for _, ch := range idStr {
			accountID = accountID*10 + int64(ch-'0')
		}
		c.Set("accountId", accountID)
		if typeStr := c.GetHeader("X-Account-Type"); typeStr != "" {
			var accountType int
			for _, ch := range typeStr {
				accountType = accountType*10 + int(ch-'0')
			}
			c.Set("accountType", accountType)
		}
		if roleStr := c.GetHeader("X-Login-Role"); roleStr != "" {
			var loginRole int
			for _, ch := range roleStr {
				loginRole = loginRole*10 + int(ch-'0')
			}
			c.Set("loginRole", loginRole)
		}
		c.Next()
	}
}

// ==================== JSON Helpers ====================

func parseJSON(t *testing.T, body io.Reader) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	return m
}

func assertCode(t *testing.T, body map[string]interface{}, expected int) {
	t.Helper()
	got, ok := body["code"].(float64)
	if !ok {
		t.Fatalf("response missing 'code' field or not a number: %v", body)
	}
	if int(got) != expected {
		t.Errorf("code = %d, want %d; body = %v", int(got), expected, body)
	}
}

func assertHTTPStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("HTTP status = %d, want %d; body = %s", w.Code, expected, w.Body.String())
	}
}

// pageData 取分頁 envelope 的 data 物件，並斷言標準四欄（records/total/page/pageSize）
// 全都在 data 之下、且 total/page/pageSize 為字串（10 檔 §2 統一 Envelope ＋ §4 對外數字
// 一律 string）。頂層一旦出現任一分頁欄位即視為 Page-trap 回歸，直接 fail。
func pageData(t *testing.T, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	for _, k := range []string{"records", "total", "page", "pageSize", "pageSize"} {
		if _, leaked := body[k]; leaked {
			t.Fatalf("envelope 頂層不得出現分頁欄位 %q（Page-trap 回歸）；body = %v", k, body)
		}
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object; body = %v", body)
	}
	for _, k := range []string{"total", "page", "pageSize"} {
		if _, ok := data[k].(string); !ok {
			t.Fatalf("data.%s 必須是字串（§4）；got %T = %v", k, data[k], data[k])
		}
	}
	return data
}

// pageRecords 取分頁 envelope 的 data.records 陣列（先過 pageData 的形狀斷言）。
func pageRecords(t *testing.T, body map[string]interface{}) []interface{} {
	t.Helper()
	data := pageData(t, body)
	rows, ok := data["records"].([]interface{})
	if !ok {
		t.Fatalf("data.records is not an array; data = %v", data)
	}
	return rows
}

// ==================== Mock AccountServiceClient ====================

type mockAccountServiceClient struct {
	GoogleOAuthFn             func(ctx context.Context, in *accountpb.GoogleOAuthRequest, opts ...grpc.CallOption) (*accountpb.AuthResponse, error)
	AppleOAuthFn              func(ctx context.Context, in *accountpb.AppleOAuthRequest, opts ...grpc.CallOption) (*accountpb.AuthResponse, error)
	ValidateUserFn            func(ctx context.Context, in *accountpb.ValidateUserRequest, opts ...grpc.CallOption) (*accountpb.ValidateUserResponse, error)
	VerifyPhoneFn             func(ctx context.Context, in *accountpb.VerifyPhoneRequest, opts ...grpc.CallOption) (*accountpb.VerifyPhoneResponse, error)
	SendOtpFn                 func(ctx context.Context, in *accountpb.SendOtpRequest, opts ...grpc.CallOption) (*accountpb.SendOtpResponse, error)
	RefreshTokenFn            func(ctx context.Context, in *accountpb.RefreshTokenRequest, opts ...grpc.CallOption) (*accountpb.RefreshTokenResponse, error)
	CheckPhoneFn              func(ctx context.Context, in *accountpb.CheckPhoneRequest, opts ...grpc.CallOption) (*accountpb.CheckPhoneResponse, error)
	LinkConfirmFn             func(ctx context.Context, in *accountpb.LinkConfirmRequest, opts ...grpc.CallOption) (*accountpb.LinkConfirmResponse, error)
	ListAuthProvidersFn       func(ctx context.Context, in *accountpb.ListAuthProvidersRequest, opts ...grpc.CallOption) (*accountpb.ListAuthProvidersResponse, error)
	BindAuthProviderFn        func(ctx context.Context, in *accountpb.BindAuthProviderRequest, opts ...grpc.CallOption) (*accountpb.BindAuthProviderResponse, error)
	UnbindAuthProviderFn      func(ctx context.Context, in *accountpb.UnbindAuthProviderRequest, opts ...grpc.CallOption) (*accountpb.UnbindAuthProviderResponse, error)
	GetAccountByPhoneFn       func(ctx context.Context, in *accountpb.GetAccountByPhoneRequest, opts ...grpc.CallOption) (*accountpb.AccountInfoResponse, error)
	GetAccountInfoFn          func(ctx context.Context, in *accountpb.GetAccountInfoRequest, opts ...grpc.CallOption) (*accountpb.AccountInfoResponse, error)
	SuspendAccountFn          func(ctx context.Context, in *accountpb.SuspendAccountRequest, opts ...grpc.CallOption) (*accountpb.SuspendAccountResponse, error)
	UnsuspendAccountFn        func(ctx context.Context, in *accountpb.UnsuspendAccountRequest, opts ...grpc.CallOption) (*accountpb.UnsuspendAccountResponse, error)
	ListAccountsForAdminFn    func(ctx context.Context, in *accountpb.ListAccountsForAdminRequest, opts ...grpc.CallOption) (*accountpb.ListAccountsForAdminResponse, error)
	GetAccountForAdminFn      func(ctx context.Context, in *accountpb.GetAccountForAdminRequest, opts ...grpc.CallOption) (*accountpb.GetAccountForAdminResponse, error)
	BindBuyerFn               func(ctx context.Context, in *accountpb.BindBuyerRequest, opts ...grpc.CallOption) (*accountpb.BindBuyerResponse, error)
	PersistWebSessionFn       func(ctx context.Context, in *accountpb.PersistWebSessionRequest, opts ...grpc.CallOption) (*accountpb.PersistWebSessionResponse, error)
	TouchWebSessionFn         func(ctx context.Context, in *accountpb.TouchWebSessionRequest, opts ...grpc.CallOption) (*accountpb.TouchWebSessionResponse, error)
	GetWebSessionFn           func(ctx context.Context, in *accountpb.GetWebSessionRequest, opts ...grpc.CallOption) (*accountpb.GetWebSessionResponse, error)
	DeleteWebSessionFn        func(ctx context.Context, in *accountpb.DeleteWebSessionRequest, opts ...grpc.CallOption) (*accountpb.DeleteWebSessionResponse, error)
	RevokeTokenFn             func(ctx context.Context, in *accountpb.RevokeTokenRequest, opts ...grpc.CallOption) (*accountpb.RevokeTokenResponse, error)
	RevokeAllFn               func(ctx context.Context, in *accountpb.RevokeAllCredentialsRequest, opts ...grpc.CallOption) (*accountpb.RevokeAllCredentialsResponse, error)
	CheckCredentialFn         func(ctx context.Context, in *accountpb.CheckCredentialRequest, opts ...grpc.CallOption) (*accountpb.CheckCredentialResponse, error)
	GetAccountStatsForAdminFn func(ctx context.Context, in *accountpb.GetAccountStatsForAdminRequest, opts ...grpc.CallOption) (*accountpb.GetAccountStatsForAdminResponse, error)
	AdminGoogleLoginFn        func(ctx context.Context, in *accountpb.AdminGoogleLoginRequest, opts ...grpc.CallOption) (*accountpb.AdminLoginResponse, error)
}

func (m *mockAccountServiceClient) GoogleOAuth(ctx context.Context, in *accountpb.GoogleOAuthRequest, opts ...grpc.CallOption) (*accountpb.AuthResponse, error) {
	return m.GoogleOAuthFn(ctx, in, opts...)
}

func (m *mockAccountServiceClient) AppleOAuth(ctx context.Context, in *accountpb.AppleOAuthRequest, opts ...grpc.CallOption) (*accountpb.AuthResponse, error) {
	if m.AppleOAuthFn != nil {
		return m.AppleOAuthFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAccountServiceClient) ValidateUser(ctx context.Context, in *accountpb.ValidateUserRequest, opts ...grpc.CallOption) (*accountpb.ValidateUserResponse, error) {
	return m.ValidateUserFn(ctx, in, opts...)
}

func (m *mockAccountServiceClient) VerifyPhone(ctx context.Context, in *accountpb.VerifyPhoneRequest, opts ...grpc.CallOption) (*accountpb.VerifyPhoneResponse, error) {
	return m.VerifyPhoneFn(ctx, in, opts...)
}

func (m *mockAccountServiceClient) SendOtp(ctx context.Context, in *accountpb.SendOtpRequest, opts ...grpc.CallOption) (*accountpb.SendOtpResponse, error) {
	if m.SendOtpFn != nil {
		return m.SendOtpFn(ctx, in, opts...)
	}
	return &accountpb.SendOtpResponse{}, nil
}

func (m *mockAccountServiceClient) RefreshToken(ctx context.Context, in *accountpb.RefreshTokenRequest, opts ...grpc.CallOption) (*accountpb.RefreshTokenResponse, error) {
	return m.RefreshTokenFn(ctx, in, opts...)
}

func (m *mockAccountServiceClient) GetAccountInfo(ctx context.Context, in *accountpb.GetAccountInfoRequest, opts ...grpc.CallOption) (*accountpb.AccountInfoResponse, error) {
	if m.GetAccountInfoFn != nil {
		return m.GetAccountInfoFn(ctx, in, opts...)
	}
	return &accountpb.AccountInfoResponse{Data: &accountpb.AccountInfo{}}, nil
}

func (m *mockAccountServiceClient) GetAccountInfoBatch(ctx context.Context, in *accountpb.GetAccountInfoBatchRequest, opts ...grpc.CallOption) (*accountpb.AccountInfoBatchResponse, error) {
	return nil, nil
}

func (m *mockAccountServiceClient) CheckPhone(ctx context.Context, in *accountpb.CheckPhoneRequest, opts ...grpc.CallOption) (*accountpb.CheckPhoneResponse, error) {
	return m.CheckPhoneFn(ctx, in, opts...)
}

func (m *mockAccountServiceClient) BindSeller(ctx context.Context, in *accountpb.BindSellerRequest, opts ...grpc.CallOption) (*accountpb.BindSellerResponse, error) {
	return &accountpb.BindSellerResponse{}, nil
}

func (m *mockAccountServiceClient) BindBuyer(ctx context.Context, in *accountpb.BindBuyerRequest, opts ...grpc.CallOption) (*accountpb.BindBuyerResponse, error) {
	if m.BindBuyerFn != nil {
		return m.BindBuyerFn(ctx, in, opts...)
	}
	return &accountpb.BindBuyerResponse{}, nil
}

func (m *mockAccountServiceClient) RemoveRole(ctx context.Context, in *accountpb.RemoveRoleRequest, opts ...grpc.CallOption) (*accountpb.RemoveRoleResponse, error) {
	return nil, nil
}

func (m *mockAccountServiceClient) AdminLogin(ctx context.Context, in *accountpb.AdminLoginRequest, opts ...grpc.CallOption) (*accountpb.AdminLoginResponse, error) {
	return nil, nil
}

func (m *mockAccountServiceClient) AdminGoogleLogin(ctx context.Context, in *accountpb.AdminGoogleLoginRequest, opts ...grpc.CallOption) (*accountpb.AdminLoginResponse, error) {
	if m.AdminGoogleLoginFn != nil {
		return m.AdminGoogleLoginFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAccountServiceClient) GetAccountByPhone(ctx context.Context, in *accountpb.GetAccountByPhoneRequest, opts ...grpc.CallOption) (*accountpb.AccountInfoResponse, error) {
	if m.GetAccountByPhoneFn != nil {
		return m.GetAccountByPhoneFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAccountServiceClient) LinkConfirm(ctx context.Context, in *accountpb.LinkConfirmRequest, opts ...grpc.CallOption) (*accountpb.LinkConfirmResponse, error) {
	if m.LinkConfirmFn != nil {
		return m.LinkConfirmFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAccountServiceClient) ListAuthProviders(ctx context.Context, in *accountpb.ListAuthProvidersRequest, opts ...grpc.CallOption) (*accountpb.ListAuthProvidersResponse, error) {
	if m.ListAuthProvidersFn != nil {
		return m.ListAuthProvidersFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAccountServiceClient) BindAuthProvider(ctx context.Context, in *accountpb.BindAuthProviderRequest, opts ...grpc.CallOption) (*accountpb.BindAuthProviderResponse, error) {
	if m.BindAuthProviderFn != nil {
		return m.BindAuthProviderFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAccountServiceClient) UnbindAuthProvider(ctx context.Context, in *accountpb.UnbindAuthProviderRequest, opts ...grpc.CallOption) (*accountpb.UnbindAuthProviderResponse, error) {
	if m.UnbindAuthProviderFn != nil {
		return m.UnbindAuthProviderFn(ctx, in, opts...)
	}
	return nil, nil
}

// SuspendAccount defaults to a successful suspension so a test that has not
// opted in is not tripped by an unrelated call site, while SuspendAccountFn
// gives suspension-path tests somewhere to inject failures and assert calls
// (there was previously no hook at all, so those tests could not be written).
func (m *mockAccountServiceClient) SuspendAccount(ctx context.Context, in *accountpb.SuspendAccountRequest, opts ...grpc.CallOption) (*accountpb.SuspendAccountResponse, error) {
	if m.SuspendAccountFn != nil {
		return m.SuspendAccountFn(ctx, in, opts...)
	}
	return &accountpb.SuspendAccountResponse{Success: true}, nil
}

// UnsuspendAccount mirrors SuspendAccount: default success, injectable failure.
// The two must stay symmetric — a default that only one of them has is how a
// test ends up asserting against the wrong half of the pair.
func (m *mockAccountServiceClient) UnsuspendAccount(ctx context.Context, in *accountpb.UnsuspendAccountRequest, opts ...grpc.CallOption) (*accountpb.UnsuspendAccountResponse, error) {
	if m.UnsuspendAccountFn != nil {
		return m.UnsuspendAccountFn(ctx, in, opts...)
	}
	return &accountpb.UnsuspendAccountResponse{Success: true}, nil
}

// ListAccountsForAdmin / GetAccountForAdmin default to an EMPTY answer rather
// than a populated one: inventing a member out of an empty backing store is the
// same mistake as GetWebSession defaulting to "found".
func (m *mockAccountServiceClient) ListAccountsForAdmin(ctx context.Context, in *accountpb.ListAccountsForAdminRequest, opts ...grpc.CallOption) (*accountpb.ListAccountsForAdminResponse, error) {
	if m.ListAccountsForAdminFn != nil {
		return m.ListAccountsForAdminFn(ctx, in, opts...)
	}
	return &accountpb.ListAccountsForAdminResponse{Data: &accountpb.AdminAccountListData{}}, nil
}

func (m *mockAccountServiceClient) GetAccountForAdmin(ctx context.Context, in *accountpb.GetAccountForAdminRequest, opts ...grpc.CallOption) (*accountpb.GetAccountForAdminResponse, error) {
	if m.GetAccountForAdminFn != nil {
		return m.GetAccountForAdminFn(ctx, in, opts...)
	}
	return &accountpb.GetAccountForAdminResponse{}, nil
}

// PersistWebSession defaults to "persisted": a test that has not opted in
// should fail on its own assertions, not on a nil-func panic from a call site
// it was not written to exercise.
func (m *mockAccountServiceClient) PersistWebSession(ctx context.Context, in *accountpb.PersistWebSessionRequest, opts ...grpc.CallOption) (*accountpb.PersistWebSessionResponse, error) {
	if m.PersistWebSessionFn != nil {
		return m.PersistWebSessionFn(ctx, in, opts...)
	}
	return &accountpb.PersistWebSessionResponse{}, nil
}

// TouchWebSession defaults to "row found": the role-sync call sites treat it as
// best-effort, and a test that has not opted in should not have its session
// silently reported as revoked.
func (m *mockAccountServiceClient) TouchWebSession(ctx context.Context, in *accountpb.TouchWebSessionRequest, opts ...grpc.CallOption) (*accountpb.TouchWebSessionResponse, error) {
	if m.TouchWebSessionFn != nil {
		return m.TouchWebSessionFn(ctx, in, opts...)
	}
	return &accountpb.TouchWebSessionResponse{Found: true}, nil
}

// GetWebSession defaults to NotFound — the answer a source of truth that holds
// no row for this token gives. A test that has not opted in must not have an
// identity conjured for it: this is the gateway's last-resort read-through, so
// defaulting to "found" would manufacture a logged-in user out of an empty
// backing store. NotFound is also what makes the default safe rather than merely
// non-panicking (the previous nil-func call panicked instead of failing on the
// test's own assertions).
func (m *mockAccountServiceClient) GetWebSession(ctx context.Context, in *accountpb.GetWebSessionRequest, opts ...grpc.CallOption) (*accountpb.GetWebSessionResponse, error) {
	if m.GetWebSessionFn != nil {
		return m.GetWebSessionFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.NotFound, "web session not found")
}

// DeleteWebSession defaults to a successful delete, matching the real RPC's
// idempotent contract (removing an absent row is not an error).
func (m *mockAccountServiceClient) DeleteWebSession(ctx context.Context, in *accountpb.DeleteWebSessionRequest, opts ...grpc.CallOption) (*accountpb.DeleteWebSessionResponse, error) {
	if m.DeleteWebSessionFn != nil {
		return m.DeleteWebSessionFn(ctx, in, opts...)
	}
	return &accountpb.DeleteWebSessionResponse{}, nil
}

// RevokeToken defaults to success: a test that is not about #2035 revocation
// should not have to opt in to being able to log out. Logout is fail-closed, so
// an unimplemented stub would turn every unrelated logout assertion into a 503.
func (m *mockAccountServiceClient) RevokeToken(ctx context.Context, in *accountpb.RevokeTokenRequest, opts ...grpc.CallOption) (*accountpb.RevokeTokenResponse, error) {
	if m.RevokeTokenFn != nil {
		return m.RevokeTokenFn(ctx, in, opts...)
	}
	return &accountpb.RevokeTokenResponse{}, nil
}

func (m *mockAccountServiceClient) RevokeAllCredentials(ctx context.Context, in *accountpb.RevokeAllCredentialsRequest, opts ...grpc.CallOption) (*accountpb.RevokeAllCredentialsResponse, error) {
	if m.RevokeAllFn != nil {
		return m.RevokeAllFn(ctx, in, opts...)
	}
	return &accountpb.RevokeAllCredentialsResponse{}, nil
}

// CheckCredential defaults to NOT revoked, mirroring GetAccountInfo's default:
// the gateway only reaches it when Redis could not answer, and a test that is
// not about revocation should not be logged out by the stub.
func (m *mockAccountServiceClient) CheckCredential(ctx context.Context, in *accountpb.CheckCredentialRequest, opts ...grpc.CallOption) (*accountpb.CheckCredentialResponse, error) {
	if m.CheckCredentialFn != nil {
		return m.CheckCredentialFn(ctx, in, opts...)
	}
	return &accountpb.CheckCredentialResponse{Revoked: false}, nil
}

func (m *mockAccountServiceClient) GetAccountStatsForAdmin(ctx context.Context, in *accountpb.GetAccountStatsForAdminRequest, opts ...grpc.CallOption) (*accountpb.GetAccountStatsForAdminResponse, error) {
	if m.GetAccountStatsForAdminFn != nil {
		return m.GetAccountStatsForAdminFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Mock SellerServiceClient ====================

type mockSellerServiceClient struct {
	MatchCertificateHolderFn func(context.Context, *sellerpb.MatchCertificateHolderRequest, ...grpc.CallOption) (*sellerpb.MatchCertificateHolderResponse, error)
	// SearchSellerProspects：Gateway 不呼叫，mock 只是為了滿足 SellerServiceClient 介面。
	SearchSellerProspectsFn          func(context.Context, *sellerpb.SearchSellerProspectsRequest, ...grpc.CallOption) (*sellerpb.SearchSellerProspectsResponse, error)
	GetSellerFn                      func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error)
	GetSellerStatsFn                 func(ctx context.Context, in *sellerpb.GetSellerStatsRequest, opts ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error)
	RecordStoreVisitFn               func(ctx context.Context, in *sellerpb.RecordStoreVisitRequest, opts ...grpc.CallOption) (*sellerpb.RecordStoreVisitResponse, error)
	GetSellerBatchFn                 func(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error)
	GetRecommendedSellersFn          func(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error)
	GetCertificationApplicationFn    func(ctx context.Context, in *sellerpb.GetCertificationApplicationRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error)
	CreateCertificationDraftFn       func(ctx context.Context, in *sellerpb.CreateCertificationDraftRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error)
	SaveCertificationProgressFn      func(ctx context.Context, in *sellerpb.SaveCertificationProgressRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error)
	SubmitCertificationApplicationFn func(ctx context.Context, in *sellerpb.SubmitCertificationApplicationRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error)
	SuggestAddressFn                 func(ctx context.Context, in *sellerpb.SuggestAddressRequest, opts ...grpc.CallOption) (*sellerpb.SuggestAddressResponse, error)
	ValidateAddressFn                func(ctx context.Context, in *sellerpb.ValidateAddressRequest, opts ...grpc.CallOption) (*sellerpb.ValidateAddressResponse, error)
	GetCurrentStoreIDFn              func(ctx context.Context, in *sellerpb.GetCurrentStoreIDRequest, opts ...grpc.CallOption) (*sellerpb.GetCurrentStoreIDResponse, error)
	ListPublicStoreIDsByCityFn       func(ctx context.Context, in *sellerpb.ListPublicStoreIDsByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoreIDsByCityResponse, error)
	ListPublicStoresByCityFn         func(ctx context.Context, in *sellerpb.ListPublicStoresByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoresByCityResponse, error)
	// Vendor Portal（#2539 E1）。EnsureVendorPortalInvite／MarkVendorPortalInviteSecretDeliveredFn
	// 由 notification-service 呼叫，mock 只是為了滿足 SellerServiceClient 介面。
	EnsureVendorPortalInviteFn              func(ctx context.Context, in *sellerpb.EnsureVendorPortalInviteRequest, opts ...grpc.CallOption) (*sellerpb.EnsureVendorPortalInviteResponse, error)
	MarkVendorPortalInviteSecretDeliveredFn func(ctx context.Context, in *sellerpb.MarkVendorPortalInviteSecretDeliveredRequest, opts ...grpc.CallOption) (*sellerpb.MarkVendorPortalInviteSecretDeliveredResponse, error)
}

func (m *mockSellerServiceClient) GetSeller(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	if m.GetSellerFn != nil {
		return m.GetSellerFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.NotFound, "seller not found")
}
func (m *mockSellerServiceClient) GetSellerByMemberId(ctx context.Context, in *sellerpb.GetSellerByMemberIdRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerByUserId(ctx context.Context, in *sellerpb.GetSellerByUserIdRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerBatch(ctx context.Context, in *sellerpb.GetSellerBatchRequest, opts ...grpc.CallOption) (*sellerpb.SellerBatchResponse, error) {
	if m.GetSellerBatchFn != nil {
		return m.GetSellerBatchFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) ListPublicStoreIDsByCity(ctx context.Context, in *sellerpb.ListPublicStoreIDsByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoreIDsByCityResponse, error) {
	if m.ListPublicStoreIDsByCityFn != nil {
		return m.ListPublicStoreIDsByCityFn(ctx, in, opts...)
	}
	return &sellerpb.ListPublicStoreIDsByCityResponse{}, nil
}

func (m *mockSellerServiceClient) ListPublicStoresByCity(ctx context.Context, in *sellerpb.ListPublicStoresByCityRequest, opts ...grpc.CallOption) (*sellerpb.ListPublicStoresByCityResponse, error) {
	if m.ListPublicStoresByCityFn != nil {
		return m.ListPublicStoresByCityFn(ctx, in, opts...)
	}
	return &sellerpb.ListPublicStoresByCityResponse{}, nil
}
func (m *mockSellerServiceClient) ValidateSeller(ctx context.Context, in *sellerpb.ValidateSellerRequest, opts ...grpc.CallOption) (*sellerpb.ValidateSellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerStats(ctx context.Context, in *sellerpb.GetSellerStatsRequest, opts ...grpc.CallOption) (*sellerpb.SellerStatsResponse, error) {
	if m.GetSellerStatsFn != nil {
		return m.GetSellerStatsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) RegisterPayuni(ctx context.Context, in *sellerpb.RegisterPayuniRequest, opts ...grpc.CallOption) (*sellerpb.PayuniStatusResponse, error) {
	return nil, nil
}

// gateway 已不呼叫 GetPayuniStatus、GetCertificationApplicationForAdmin、GetCertificationReviews（後台的 KYC 審核與 PayUni 改走
// 單體的 SellerAdminService）；方法留著只為滿足 SellerServiceClient 介面，go-pkg 的 proto 在 seller-service 退場時一併刪除。
func (m *mockSellerServiceClient) GetPayuniStatus(ctx context.Context, in *sellerpb.GetPayuniStatusRequest, opts ...grpc.CallOption) (*sellerpb.PayuniStatusResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ResolvePrincipals(ctx context.Context, in *sellerpb.ResolvePrincipalsRequest, opts ...grpc.CallOption) (*sellerpb.ResolvePrincipalsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListStoreIDsByAccount(ctx context.Context, in *sellerpb.ListStoreIDsByAccountRequest, opts ...grpc.CallOption) (*sellerpb.ListStoreIDsByAccountResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) RetryPayuniRegistration(ctx context.Context, in *sellerpb.RetryPayuniRequest, opts ...grpc.CallOption) (*sellerpb.PayuniStatusResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) RefreshPayuniStatus(ctx context.Context, in *sellerpb.RefreshPayuniStatusRequest, opts ...grpc.CallOption) (*sellerpb.PayuniStatusResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerPayuniConfig(ctx context.Context, in *sellerpb.GetSellerPayuniConfigRequest, opts ...grpc.CallOption) (*sellerpb.SellerPayuniConfigResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) CreateSeller(ctx context.Context, in *sellerpb.CreateSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateSeller(ctx context.Context, in *sellerpb.UpdateSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateMerchantProfile(ctx context.Context, in *sellerpb.UpdateMerchantProfileRequest, opts ...grpc.CallOption) (*sellerpb.UpdateMerchantProfileResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateSellerFoodRegistration(ctx context.Context, in *sellerpb.UpdateSellerFoodRegistrationRequest, opts ...grpc.CallOption) (*sellerpb.UpdateSellerFoodRegistrationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateSellerSEO(ctx context.Context, in *sellerpb.UpdateSellerSEORequest, opts ...grpc.CallOption) (*sellerpb.UpdateSellerSEOResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetCertificationDraft(ctx context.Context, in *sellerpb.GetDraftRequest, opts ...grpc.CallOption) (*sellerpb.CertificationDraftResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) SaveCertificationDraft(ctx context.Context, in *sellerpb.SaveDraftRequest, opts ...grpc.CallOption) (*sellerpb.CertificationDraftResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) SubmitCertification(ctx context.Context, in *sellerpb.SubmitCertificationRequest, opts ...grpc.CallOption) (*sellerpb.CertificationDraftResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) CreateCertificationDraft(ctx context.Context, in *sellerpb.CreateCertificationDraftRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error) {
	if m.CreateCertificationDraftFn != nil {
		return m.CreateCertificationDraftFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) SaveCertificationProgress(ctx context.Context, in *sellerpb.SaveCertificationProgressRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error) {
	if m.SaveCertificationProgressFn != nil {
		return m.SaveCertificationProgressFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) CancelCertification(ctx context.Context, in *sellerpb.CancelCertificationRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) SubmitCertificationApplication(ctx context.Context, in *sellerpb.SubmitCertificationApplicationRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error) {
	if m.SubmitCertificationApplicationFn != nil {
		return m.SubmitCertificationApplicationFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) SuggestAddress(ctx context.Context, in *sellerpb.SuggestAddressRequest, opts ...grpc.CallOption) (*sellerpb.SuggestAddressResponse, error) {
	if m.SuggestAddressFn != nil {
		return m.SuggestAddressFn(ctx, in, opts...)
	}
	return &sellerpb.SuggestAddressResponse{Code: 0}, nil
}
func (m *mockSellerServiceClient) ValidateAddress(ctx context.Context, in *sellerpb.ValidateAddressRequest, opts ...grpc.CallOption) (*sellerpb.ValidateAddressResponse, error) {
	if m.ValidateAddressFn != nil {
		return m.ValidateAddressFn(ctx, in, opts...)
	}
	return &sellerpb.ValidateAddressResponse{Code: 0, Data: &sellerpb.AddressValidationData{}}, nil
}
func (m *mockSellerServiceClient) GetCertificationApplication(ctx context.Context, in *sellerpb.GetCertificationApplicationRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error) {
	if m.GetCertificationApplicationFn != nil {
		return m.GetCertificationApplicationFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) GetCertificationApplicationForAdmin(ctx context.Context, in *sellerpb.GetCertificationApplicationForAdminRequest, opts ...grpc.CallOption) (*sellerpb.CertificationApplicationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) LookupBusinessRegistration(ctx context.Context, in *sellerpb.LookupBusinessRegistrationRequest, opts ...grpc.CallOption) (*sellerpb.LookupBusinessRegistrationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ApproveCertification(ctx context.Context, in *sellerpb.ApproveCertificationRequest, opts ...grpc.CallOption) (*sellerpb.ReviewResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) RejectCertification(ctx context.Context, in *sellerpb.RejectCertificationRequest, opts ...grpc.CallOption) (*sellerpb.ReviewResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetCertificationReviews(ctx context.Context, in *sellerpb.GetCertificationReviewsRequest, opts ...grpc.CallOption) (*sellerpb.CertificationReviewsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListSellersForAdmin(ctx context.Context, in *sellerpb.ListSellersForAdminRequest, opts ...grpc.CallOption) (*sellerpb.ListSellersForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerStatsForAdmin(ctx context.Context, in *sellerpb.GetSellerStatsForAdminRequest, opts ...grpc.CallOption) (*sellerpb.GetSellerStatsForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListCertificationsForAdmin(ctx context.Context, in *sellerpb.ListCertificationsForAdminRequest, opts ...grpc.CallOption) (*sellerpb.ListCertificationsForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) BatchGetSellerMeta(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) CompleteOnboarding(ctx context.Context, in *sellerpb.CompleteOnboardingRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerShippingProfile(ctx context.Context, in *sellerpb.GetSellerShippingProfileRequest, opts ...grpc.CallOption) (*sellerpb.GetSellerShippingProfileResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateLogo(ctx context.Context, in *sellerpb.UpdateLogoRequest, opts ...grpc.CallOption) (*sellerpb.UpdateLogoResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateCover(ctx context.Context, in *sellerpb.UpdateCoverRequest, opts ...grpc.CallOption) (*sellerpb.UpdateCoverResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) RecordStoreVisit(ctx context.Context, in *sellerpb.RecordStoreVisitRequest, opts ...grpc.CallOption) (*sellerpb.RecordStoreVisitResponse, error) {
	if m.RecordStoreVisitFn != nil {
		return m.RecordStoreVisitFn(ctx, in, opts...)
	}
	return &sellerpb.RecordStoreVisitResponse{Code: 0}, nil
}
func (m *mockSellerServiceClient) GetStoreVisitors(ctx context.Context, in *sellerpb.GetStoreVisitorsRequest, opts ...grpc.CallOption) (*sellerpb.StoreVisitorListResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetRecommendedSellers(ctx context.Context, in *sellerpb.GetRecommendedSellersRequest, opts ...grpc.CallOption) (*sellerpb.GetRecommendedSellersResponse, error) {
	if m.GetRecommendedSellersFn != nil {
		return m.GetRecommendedSellersFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) RecordProductView(ctx context.Context, in *sellerpb.RecordProductViewRequest, opts ...grpc.CallOption) (*sellerpb.RecordProductViewResponse, error) {
	return &sellerpb.RecordProductViewResponse{Code: 0}, nil
}
func (m *mockSellerServiceClient) GetProductViewStats(ctx context.Context, in *sellerpb.GetProductViewStatsRequest, opts ...grpc.CallOption) (*sellerpb.ProductViewStatsResponse, error) {
	return &sellerpb.ProductViewStatsResponse{Code: 0}, nil
}
func (m *mockSellerServiceClient) DeleteSellerAccount(ctx context.Context, in *sellerpb.DeleteSellerAccountRequest, opts ...grpc.CallOption) (*sellerpb.DeleteSellerAccountResponse, error) {
	return &sellerpb.DeleteSellerAccountResponse{Code: 0}, nil
}

// ---- B-169 governance / food-safety / appeal stubs (added with PR #727) ----
// 這 18 個 RPC 在 gateway test 目前沒被直接呼叫；補空 stub 讓 mock 滿足 interface 即可。
// 真實單元測試另外在 b169_test.go 用獨立 mock。

func (m *mockSellerServiceClient) CreateViolationReport(ctx context.Context, in *sellerpb.CreateViolationReportRequest, opts ...grpc.CallOption) (*sellerpb.CreateViolationReportResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListMyViolationReports(ctx context.Context, in *sellerpb.ListMyViolationReportsRequest, opts ...grpc.CallOption) (*sellerpb.ListMyViolationReportsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) CreateFoodSafetyReport(ctx context.Context, in *sellerpb.CreateFoodSafetyReportRequest, opts ...grpc.CallOption) (*sellerpb.CreateFoodSafetyReportResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) SellerListMyViolations(ctx context.Context, in *sellerpb.SellerListMyViolationsRequest, opts ...grpc.CallOption) (*sellerpb.SellerListMyViolationsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) SellerSubmitAppeal(ctx context.Context, in *sellerpb.SellerSubmitAppealRequest, opts ...grpc.CallOption) (*sellerpb.SellerSubmitAppealResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) SellerGetAppeal(ctx context.Context, in *sellerpb.SellerGetAppealRequest, opts ...grpc.CallOption) (*sellerpb.SellerGetAppealResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetPublicSellerViolations(ctx context.Context, in *sellerpb.GetPublicSellerViolationsRequest, opts ...grpc.CallOption) (*sellerpb.GetPublicSellerViolationsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminListViolationReports(ctx context.Context, in *sellerpb.AdminListViolationReportsRequest, opts ...grpc.CallOption) (*sellerpb.AdminListViolationReportsResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminPatchSellerViolationReportStatus(ctx context.Context, in *sellerpb.AdminPatchReportStatusRequest, opts ...grpc.CallOption) (*sellerpb.AdminPatchReportStatusResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminPatchFoodSafetyReportStatus(ctx context.Context, in *sellerpb.AdminPatchReportStatusRequest, opts ...grpc.CallOption) (*sellerpb.AdminPatchReportStatusResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminAcknowledgeFoodSafetyReport(ctx context.Context, in *sellerpb.AdminAcknowledgeRequest, opts ...grpc.CallOption) (*sellerpb.AdminAcknowledgeResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminGenerateFDAReport(ctx context.Context, in *sellerpb.AdminGenerateFDAReportRequest, opts ...grpc.CallOption) (*sellerpb.AdminGenerateFDAReportResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminMarkFDAReported(ctx context.Context, in *sellerpb.AdminMarkFDAReportedRequest, opts ...grpc.CallOption) (*sellerpb.AdminMarkFDAReportedResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminNotifySeller(ctx context.Context, in *sellerpb.AdminNotifySellerRequest, opts ...grpc.CallOption) (*sellerpb.AdminNotifySellerResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminCreateViolation(ctx context.Context, in *sellerpb.AdminCreateViolationRequest, opts ...grpc.CallOption) (*sellerpb.AdminCreateViolationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminGetViolation(ctx context.Context, in *sellerpb.AdminGetViolationRequest, opts ...grpc.CallOption) (*sellerpb.AdminGetViolationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminUpdateViolation(ctx context.Context, in *sellerpb.AdminUpdateViolationRequest, opts ...grpc.CallOption) (*sellerpb.AdminUpdateViolationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminRevokeViolation(ctx context.Context, in *sellerpb.AdminRevokeViolationRequest, opts ...grpc.CallOption) (*sellerpb.AdminRevokeViolationResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AdminDecideAppeal(ctx context.Context, in *sellerpb.AdminDecideAppealRequest, opts ...grpc.CallOption) (*sellerpb.AdminDecideAppealResponse, error) {
	return nil, nil
}

// 以下四支停權 RPC 的 gateway 已不呼叫（停權改走單體的 SellerAdminService）；方法留著只為滿足
// SellerServiceClient 介面，go-pkg 的 proto 在 seller-service 退場時一併刪除。
func (m *mockSellerServiceClient) SuspendSellerForAdmin(ctx context.Context, in *sellerpb.SuspendSellerForAdminRequest, opts ...grpc.CallOption) (*sellerpb.SuspendSellerForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) LiftSellerSuspensionForAdmin(ctx context.Context, in *sellerpb.LiftSellerSuspensionForAdminRequest, opts ...grpc.CallOption) (*sellerpb.LiftSellerSuspensionForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListSellerSuspensionsForAdmin(ctx context.Context, in *sellerpb.ListSellerSuspensionsForAdminRequest, opts ...grpc.CallOption) (*sellerpb.ListSellerSuspensionsForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetActiveSellerSuspensionForAdmin(ctx context.Context, in *sellerpb.GetActiveSellerSuspensionForAdminRequest, opts ...grpc.CallOption) (*sellerpb.GetActiveSellerSuspensionForAdminResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) AssignSellerMembershipPlan(ctx context.Context, in *sellerpb.AssignSellerMembershipPlanRequest, opts ...grpc.CallOption) (*sellerpb.SellerMembershipPlanResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) GetSellerMembershipPlan(ctx context.Context, in *sellerpb.GetSellerMembershipPlanRequest, opts ...grpc.CallOption) (*sellerpb.SellerMembershipPlanResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ListSellerMembershipPlans(ctx context.Context, in *sellerpb.ListSellerMembershipPlansRequest, opts ...grpc.CallOption) (*sellerpb.ListSellerMembershipPlansResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) UpdateSellerMembershipPlan(ctx context.Context, in *sellerpb.UpdateSellerMembershipPlanRequest, opts ...grpc.CallOption) (*sellerpb.UpdateSellerMembershipPlanResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) ResolveSellerAccountID(ctx context.Context, in *sellerpb.ResolveSellerAccountIDRequest, opts ...grpc.CallOption) (*sellerpb.ResolveSellerAccountIDResponse, error) {
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "ResolveSellerAccountID not wired in test")
}
func (m *mockSellerServiceClient) GetCurrentStoreID(ctx context.Context, in *sellerpb.GetCurrentStoreIDRequest, opts ...grpc.CallOption) (*sellerpb.GetCurrentStoreIDResponse, error) {
	if m.GetCurrentStoreIDFn != nil {
		return m.GetCurrentStoreIDFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "GetCurrentStoreIDFn not wired in test")
}

// MatchCertificateHolder 供 19 號 §8.5.1 的業者名稱歸屬比對。Gateway 自己不呼叫它
// （product-service 才是消費者），mock 只是為了滿足 SellerServiceClient 介面。
func (m *mockSellerServiceClient) MatchCertificateHolder(ctx context.Context, in *sellerpb.MatchCertificateHolderRequest, opts ...grpc.CallOption) (*sellerpb.MatchCertificateHolderResponse, error) {
	if m.MatchCertificateHolderFn != nil {
		return m.MatchCertificateHolderFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "MatchCertificateHolderFn not wired in test")
}

// SearchSellerProspects 的消費端是 ai-service 的 EXTERNAL lane 與一次性遷移腳本，Gateway 自己不呼叫，mock 只是為了滿足介面。
func (m *mockSellerServiceClient) SearchSellerProspects(ctx context.Context, in *sellerpb.SearchSellerProspectsRequest, opts ...grpc.CallOption) (*sellerpb.SearchSellerProspectsResponse, error) {
	if m.SearchSellerProspectsFn != nil {
		return m.SearchSellerProspectsFn(ctx, in, opts...)
	}
	return nil, grpcstatus.Error(grpccodes.Unimplemented, "SearchSellerProspectsFn not wired in test")
}

// gateway 已不呼叫 CreateSellerProspect、AdminListSellerProspects、AdminGetSellerProspect、AdminUpdateSellerProspect、
// AdminArchiveSellerProspect、AdminUpdateSellerProspectMedia、AdminCreateProspectInvite、AdminRevokeProspectInvite、
// AdminListDuplicateProspects、AdminMergeSellerProspects、AdminConvertSellerProspect、AdminListProspectTaxIdMatches（後台的站外業者
// 改走單體的 SellerProspectAdminService）；方法留著只為滿足 SellerServiceClient 介面，go-pkg 的 proto 在 seller-service 退場時一併刪除。
func (m *mockSellerServiceClient) CreateSellerProspect(ctx context.Context, in *sellerpb.CreateSellerProspectRequest, opts ...grpc.CallOption) (*sellerpb.CreateSellerProspectResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminListSellerProspects(ctx context.Context, in *sellerpb.AdminListSellerProspectsRequest, opts ...grpc.CallOption) (*sellerpb.AdminListSellerProspectsResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminGetSellerProspect(ctx context.Context, in *sellerpb.AdminGetSellerProspectRequest, opts ...grpc.CallOption) (*sellerpb.AdminGetSellerProspectResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminUpdateSellerProspect(ctx context.Context, in *sellerpb.AdminUpdateSellerProspectRequest, opts ...grpc.CallOption) (*sellerpb.AdminGetSellerProspectResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminArchiveSellerProspect(ctx context.Context, in *sellerpb.AdminArchiveSellerProspectRequest, opts ...grpc.CallOption) (*sellerpb.AdminGetSellerProspectResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminUpdateSellerProspectMedia(ctx context.Context, in *sellerpb.AdminUpdateSellerProspectMediaRequest, opts ...grpc.CallOption) (*sellerpb.AdminGetSellerProspectResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminListDuplicateProspects(ctx context.Context, in *sellerpb.AdminListDuplicateProspectsRequest, opts ...grpc.CallOption) (*sellerpb.AdminListDuplicateProspectsResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminMergeSellerProspects(ctx context.Context, in *sellerpb.AdminMergeSellerProspectsRequest, opts ...grpc.CallOption) (*sellerpb.AdminMergeSellerProspectsResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminConvertSellerProspect(ctx context.Context, in *sellerpb.AdminConvertSellerProspectRequest, opts ...grpc.CallOption) (*sellerpb.AdminGetSellerProspectResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminListProspectTaxIdMatches(ctx context.Context, in *sellerpb.AdminListProspectTaxIdMatchesRequest, opts ...grpc.CallOption) (*sellerpb.AdminListProspectTaxIdMatchesResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminCreateProspectInvite(ctx context.Context, in *sellerpb.AdminCreateProspectInviteRequest, opts ...grpc.CallOption) (*sellerpb.AdminCreateProspectInviteResponse, error) {
	return nil, nil
}

func (m *mockSellerServiceClient) AdminRevokeProspectInvite(ctx context.Context, in *sellerpb.AdminRevokeProspectInviteRequest, opts ...grpc.CallOption) (*sellerpb.AdminRevokeProspectInviteResponse, error) {
	return nil, nil
}
func (m *mockSellerServiceClient) EnsureVendorPortalInvite(ctx context.Context, in *sellerpb.EnsureVendorPortalInviteRequest, opts ...grpc.CallOption) (*sellerpb.EnsureVendorPortalInviteResponse, error) {
	if m.EnsureVendorPortalInviteFn != nil {
		return m.EnsureVendorPortalInviteFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockSellerServiceClient) MarkVendorPortalInviteSecretDelivered(ctx context.Context, in *sellerpb.MarkVendorPortalInviteSecretDeliveredRequest, opts ...grpc.CallOption) (*sellerpb.MarkVendorPortalInviteSecretDeliveredResponse, error) {
	if m.MarkVendorPortalInviteSecretDeliveredFn != nil {
		return m.MarkVendorPortalInviteSecretDeliveredFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Mock BuyerServiceClient ====================

type mockBuyerServiceClient struct {
	GetBuyerProfileFn                func(ctx context.Context, in *buyerpb.GetBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error)
	GetBuyerProfileByBuyerIDFn       func(ctx context.Context, in *buyerpb.GetBuyerProfileByBuyerIDRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error)
	EnsureBuyerProfileFn             func(ctx context.Context, in *buyerpb.EnsureBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error)
	BatchGetBuyerMetaByBuyerIDsFn    func(ctx context.Context, in *buyerpb.BatchGetBuyerMetaByBuyerIDsRequest, opts ...grpc.CallOption) (*buyerpb.BatchGetBuyerMetaByBuyerIDsResponse, error)
	BatchGetBuyerDisplayByBuyerIDsFn func(context.Context, *buyerpb.BatchGetBuyerDisplayByBuyerIDsRequest, ...grpc.CallOption) (*buyerpb.BatchGetBuyerDisplayByBuyerIDsResponse, error)
	GetBuyerProfileBatchFn           func(ctx context.Context, in *buyerpb.GetBuyerProfileBatchRequest, opts ...grpc.CallOption) (*buyerpb.GetBuyerProfileBatchResponse, error)
	UpdateBuyerProfileFn             func(ctx context.Context, in *buyerpb.UpdateBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error)
	GetAddressListFn                 func(ctx context.Context, in *buyerpb.GetAddressListRequest, opts ...grpc.CallOption) (*buyerpb.AddressListResponse, error)
	GetAddressFn                     func(ctx context.Context, in *buyerpb.GetAddressRequest, opts ...grpc.CallOption) (*buyerpb.GetAddressResponse, error)
	AddAddressFn                     func(ctx context.Context, in *buyerpb.AddAddressRequest, opts ...grpc.CallOption) (*buyerpb.AddressOperationResponse, error)
	UpdateAddressFn                  func(ctx context.Context, in *buyerpb.UpdateAddressRequest, opts ...grpc.CallOption) (*buyerpb.AddressOperationResponse, error)
	DeleteAddressFn                  func(ctx context.Context, in *buyerpb.DeleteAddressRequest, opts ...grpc.CallOption) (*buyerpb.AddressOperationResponse, error)
	GetFavoriteListFn                func(ctx context.Context, in *buyerpb.GetFavoriteListRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteListResponse, error)
	CheckFavoriteFn                  func(ctx context.Context, in *buyerpb.CheckFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.CheckFavoriteResponse, error)
	BatchCheckFavoritesFn            func(ctx context.Context, in *buyerpb.BatchCheckFavoritesRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error)
	AddFavoriteFn                    func(ctx context.Context, in *buyerpb.AddFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteOperationResponse, error)
	RemoveFavoriteFn                 func(ctx context.Context, in *buyerpb.RemoveFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteOperationResponse, error)
	GetFavoriteCountFn               func(ctx context.Context, in *buyerpb.GetFavoriteCountRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteCountResponse, error)
	GetSubscriptionListFn            func(ctx context.Context, in *buyerpb.GetSubscriptionListRequest, opts ...grpc.CallOption) (*buyerpb.SubscriptionListResponse, error)
	SubscribeSellerFn                func(ctx context.Context, in *buyerpb.SubscribeSellerRequest, opts ...grpc.CallOption) (*buyerpb.SubscribeSellerResponse, error)
	UnsubscribeSellerFn              func(ctx context.Context, in *buyerpb.UnsubscribeSellerRequest, opts ...grpc.CallOption) (*buyerpb.UnsubscribeSellerResponse, error)
	CheckSubscriptionFn              func(ctx context.Context, in *buyerpb.CheckSubscriptionRequest, opts ...grpc.CallOption) (*buyerpb.CheckSubscriptionResponse, error)
	GetSubscribersListFn             func(ctx context.Context, in *buyerpb.GetSubscribersListRequest, opts ...grpc.CallOption) (*buyerpb.SubscribersListResponse, error)
	GetVisitListFn                   func(ctx context.Context, in *buyerpb.GetVisitListRequest, opts ...grpc.CallOption) (*buyerpb.VisitListResponse, error)
	RecordVisitFn                    func(ctx context.Context, in *buyerpb.RecordVisitRequest, opts ...grpc.CallOption) (*buyerpb.VisitOperationResponse, error)
	ClearVisitHistoryFn              func(ctx context.Context, in *buyerpb.ClearVisitHistoryRequest, opts ...grpc.CallOption) (*buyerpb.VisitOperationResponse, error)
	GetMemberCardsFn                 func(ctx context.Context, in *buyerpb.GetMemberCardsRequest, opts ...grpc.CallOption) (*buyerpb.GetMemberCardsResponse, error)
	GetSpendingRecordsFn             func(ctx context.Context, in *buyerpb.GetSpendingRecordsRequest, opts ...grpc.CallOption) (*buyerpb.GetSpendingRecordsResponse, error)
	SubscribeRestockFn               func(ctx context.Context, in *buyerpb.SubscribeRestockRequest, opts ...grpc.CallOption) (*buyerpb.SubscribeRestockResponse, error)
	UnsubscribeRestockFn             func(ctx context.Context, in *buyerpb.UnsubscribeRestockRequest, opts ...grpc.CallOption) (*buyerpb.UnsubscribeRestockResponse, error)
	ListRestockSubscriptionsFn       func(ctx context.Context, in *buyerpb.ListRestockSubscriptionsRequest, opts ...grpc.CallOption) (*buyerpb.ListRestockSubscriptionsResponse, error)
	BatchCheckRestockSubscriptionsFn func(ctx context.Context, in *buyerpb.BatchCheckRestockSubscriptionsRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error)
	SuggestAddressFn                 func(ctx context.Context, in *buyerpb.SuggestAddressRequest, opts ...grpc.CallOption) (*buyerpb.SuggestAddressResponse, error)
	ValidateAddressFn                func(ctx context.Context, in *buyerpb.ValidateAddressRequest, opts ...grpc.CallOption) (*buyerpb.ValidateAddressResponse, error)
	GetBuyerStatsForAdminFn          func(ctx context.Context, in *buyerpb.GetBuyerStatsForAdminRequest, opts ...grpc.CallOption) (*buyerpb.GetBuyerStatsForAdminResponse, error)
	UpdateBuyerAvatarFn              func(ctx context.Context, in *buyerpb.UpdateBuyerAvatarRequest, opts ...grpc.CallOption) (*buyerpb.UpdateBuyerAvatarResponse, error)
	GetSavedItemsFn                  func(ctx context.Context, in *buyerpb.GetSavedItemsRequest, opts ...grpc.CallOption) (*buyerpb.SavedItemsResponse, error)
	GetBuyerEntitlementFn            func(ctx context.Context, in *buyerpb.GetBuyerEntitlementRequest, opts ...grpc.CallOption) (*buyerpb.BuyerEntitlement, error)
	AssignMembershipFn               func(ctx context.Context, in *buyerpb.AssignMembershipRequest, opts ...grpc.CallOption) (*buyerpb.BuyerEntitlement, error)
	ListMembershipPlansFn            func(ctx context.Context, in *buyerpb.ListMembershipPlansRequest, opts ...grpc.CallOption) (*buyerpb.ListMembershipPlansResponse, error)
	UpdateMembershipPlanFn           func(ctx context.Context, in *buyerpb.UpdateMembershipPlanRequest, opts ...grpc.CallOption) (*buyerpb.UpdateMembershipPlanResponse, error)
}

func (m *mockBuyerServiceClient) GetBuyerProfile(ctx context.Context, in *buyerpb.GetBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error) {
	if m.GetBuyerProfileFn != nil {
		return m.GetBuyerProfileFn(ctx, in, opts...)
	}
	return &buyerpb.BuyerProfileResponse{Code: 0, Data: &buyerpb.BuyerInfo{Id: "1"}}, nil
}
func (m *mockBuyerServiceClient) GetBuyerProfileByBuyerID(ctx context.Context, in *buyerpb.GetBuyerProfileByBuyerIDRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error) {
	if m.GetBuyerProfileByBuyerIDFn != nil {
		return m.GetBuyerProfileByBuyerIDFn(ctx, in, opts...)
	}
	return &buyerpb.BuyerProfileResponse{Code: 0, Data: &buyerpb.BuyerInfo{Id: "1"}}, nil
}
func (m *mockBuyerServiceClient) GetBuyerProfileBatch(ctx context.Context, in *buyerpb.GetBuyerProfileBatchRequest, opts ...grpc.CallOption) (*buyerpb.GetBuyerProfileBatchResponse, error) {
	if m.GetBuyerProfileBatchFn != nil {
		return m.GetBuyerProfileBatchFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) UpdateBuyerProfile(ctx context.Context, in *buyerpb.UpdateBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error) {
	if m.UpdateBuyerProfileFn != nil {
		return m.UpdateBuyerProfileFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetAddressList(ctx context.Context, in *buyerpb.GetAddressListRequest, opts ...grpc.CallOption) (*buyerpb.AddressListResponse, error) {
	if m.GetAddressListFn != nil {
		return m.GetAddressListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetAddress(ctx context.Context, in *buyerpb.GetAddressRequest, opts ...grpc.CallOption) (*buyerpb.GetAddressResponse, error) {
	if m.GetAddressFn != nil {
		return m.GetAddressFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) AddAddress(ctx context.Context, in *buyerpb.AddAddressRequest, opts ...grpc.CallOption) (*buyerpb.AddressOperationResponse, error) {
	if m.AddAddressFn != nil {
		return m.AddAddressFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) UpdateAddress(ctx context.Context, in *buyerpb.UpdateAddressRequest, opts ...grpc.CallOption) (*buyerpb.AddressOperationResponse, error) {
	if m.UpdateAddressFn != nil {
		return m.UpdateAddressFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) DeleteAddress(ctx context.Context, in *buyerpb.DeleteAddressRequest, opts ...grpc.CallOption) (*buyerpb.AddressOperationResponse, error) {
	if m.DeleteAddressFn != nil {
		return m.DeleteAddressFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetFavoriteList(ctx context.Context, in *buyerpb.GetFavoriteListRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteListResponse, error) {
	if m.GetFavoriteListFn != nil {
		return m.GetFavoriteListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) CheckFavorite(ctx context.Context, in *buyerpb.CheckFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.CheckFavoriteResponse, error) {
	if m.CheckFavoriteFn != nil {
		return m.CheckFavoriteFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) AddFavorite(ctx context.Context, in *buyerpb.AddFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteOperationResponse, error) {
	if m.AddFavoriteFn != nil {
		return m.AddFavoriteFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) RemoveFavorite(ctx context.Context, in *buyerpb.RemoveFavoriteRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteOperationResponse, error) {
	if m.RemoveFavoriteFn != nil {
		return m.RemoveFavoriteFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetFavoriteCount(ctx context.Context, in *buyerpb.GetFavoriteCountRequest, opts ...grpc.CallOption) (*buyerpb.FavoriteCountResponse, error) {
	if m.GetFavoriteCountFn != nil {
		return m.GetFavoriteCountFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) ListFavoritedProductIds(ctx context.Context, in *buyerpb.ListFavoritedProductIdsRequest, opts ...grpc.CallOption) (*buyerpb.ListFavoritedProductIdsResponse, error) {
	return nil, nil
}
func (m *mockBuyerServiceClient) ListSubscribedSellerIds(ctx context.Context, in *buyerpb.ListSubscribedSellerIdsRequest, opts ...grpc.CallOption) (*buyerpb.ListSubscribedSellerIdsResponse, error) {
	return nil, nil
}
func (m *mockBuyerServiceClient) GetSubscriptionList(ctx context.Context, in *buyerpb.GetSubscriptionListRequest, opts ...grpc.CallOption) (*buyerpb.SubscriptionListResponse, error) {
	if m.GetSubscriptionListFn != nil {
		return m.GetSubscriptionListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) SubscribeSeller(ctx context.Context, in *buyerpb.SubscribeSellerRequest, opts ...grpc.CallOption) (*buyerpb.SubscribeSellerResponse, error) {
	if m.SubscribeSellerFn != nil {
		return m.SubscribeSellerFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) UnsubscribeSeller(ctx context.Context, in *buyerpb.UnsubscribeSellerRequest, opts ...grpc.CallOption) (*buyerpb.UnsubscribeSellerResponse, error) {
	if m.UnsubscribeSellerFn != nil {
		return m.UnsubscribeSellerFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) CheckSubscription(ctx context.Context, in *buyerpb.CheckSubscriptionRequest, opts ...grpc.CallOption) (*buyerpb.CheckSubscriptionResponse, error) {
	if m.CheckSubscriptionFn != nil {
		return m.CheckSubscriptionFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetSubscribersList(ctx context.Context, in *buyerpb.GetSubscribersListRequest, opts ...grpc.CallOption) (*buyerpb.SubscribersListResponse, error) {
	if m.GetSubscribersListFn != nil {
		return m.GetSubscribersListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetSubscriberDashboardStats(ctx context.Context, in *buyerpb.GetSubscriberDashboardStatsRequest, opts ...grpc.CallOption) (*buyerpb.SubscriberDashboardStatsResponse, error) {
	return &buyerpb.SubscriberDashboardStatsResponse{}, nil
}
func (m *mockBuyerServiceClient) BatchGetSubscriberCounts(_ context.Context, _ *buyerpb.BatchGetSubscriberCountsRequest, _ ...grpc.CallOption) (*buyerpb.BatchGetSubscriberCountsResponse, error) {
	return &buyerpb.BatchGetSubscriberCountsResponse{Counts: map[int64]int64{}}, nil
}
func (m *mockBuyerServiceClient) GetVisitList(ctx context.Context, in *buyerpb.GetVisitListRequest, opts ...grpc.CallOption) (*buyerpb.VisitListResponse, error) {
	if m.GetVisitListFn != nil {
		return m.GetVisitListFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) RecordVisit(ctx context.Context, in *buyerpb.RecordVisitRequest, opts ...grpc.CallOption) (*buyerpb.VisitOperationResponse, error) {
	if m.RecordVisitFn != nil {
		return m.RecordVisitFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) ClearVisitHistory(ctx context.Context, in *buyerpb.ClearVisitHistoryRequest, opts ...grpc.CallOption) (*buyerpb.VisitOperationResponse, error) {
	if m.ClearVisitHistoryFn != nil {
		return m.ClearVisitHistoryFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) SuggestAddress(ctx context.Context, in *buyerpb.SuggestAddressRequest, opts ...grpc.CallOption) (*buyerpb.SuggestAddressResponse, error) {
	if m.SuggestAddressFn != nil {
		return m.SuggestAddressFn(ctx, in, opts...)
	}
	return &buyerpb.SuggestAddressResponse{Code: 0}, nil
}
func (m *mockBuyerServiceClient) ValidateAddress(ctx context.Context, in *buyerpb.ValidateAddressRequest, opts ...grpc.CallOption) (*buyerpb.ValidateAddressResponse, error) {
	if m.ValidateAddressFn != nil {
		return m.ValidateAddressFn(ctx, in, opts...)
	}
	return &buyerpb.ValidateAddressResponse{Code: 0, Data: &buyerpb.AddressValidationData{}}, nil
}

func (m *mockBuyerServiceClient) UpdateBuyerAvatar(ctx context.Context, in *buyerpb.UpdateBuyerAvatarRequest, opts ...grpc.CallOption) (*buyerpb.UpdateBuyerAvatarResponse, error) {
	if m.UpdateBuyerAvatarFn != nil {
		return m.UpdateBuyerAvatarFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetMemberCards(ctx context.Context, in *buyerpb.GetMemberCardsRequest, opts ...grpc.CallOption) (*buyerpb.GetMemberCardsResponse, error) {
	if m.GetMemberCardsFn != nil {
		return m.GetMemberCardsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) GetSpendingRecords(ctx context.Context, in *buyerpb.GetSpendingRecordsRequest, opts ...grpc.CallOption) (*buyerpb.GetSpendingRecordsResponse, error) {
	if m.GetSpendingRecordsFn != nil {
		return m.GetSpendingRecordsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) BatchGetBuyerDisplayByBuyerIDs(ctx context.Context, in *buyerpb.BatchGetBuyerDisplayByBuyerIDsRequest, opts ...grpc.CallOption) (*buyerpb.BatchGetBuyerDisplayByBuyerIDsResponse, error) {
	if m.BatchGetBuyerDisplayByBuyerIDsFn != nil {
		return m.BatchGetBuyerDisplayByBuyerIDsFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) BatchGetBuyerMetaByBuyerIDs(ctx context.Context, in *buyerpb.BatchGetBuyerMetaByBuyerIDsRequest, opts ...grpc.CallOption) (*buyerpb.BatchGetBuyerMetaByBuyerIDsResponse, error) {
	if m.BatchGetBuyerMetaByBuyerIDsFn != nil {
		return m.BatchGetBuyerMetaByBuyerIDsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockBuyerServiceClient) BatchCheckFavorites(ctx context.Context, in *buyerpb.BatchCheckFavoritesRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckFavoritesResponse, error) {
	if m.BatchCheckFavoritesFn != nil {
		return m.BatchCheckFavoritesFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Restock Subscription (#942) ====================

func (m *mockBuyerServiceClient) SubscribeRestock(ctx context.Context, in *buyerpb.SubscribeRestockRequest, opts ...grpc.CallOption) (*buyerpb.SubscribeRestockResponse, error) {
	if m.SubscribeRestockFn != nil {
		return m.SubscribeRestockFn(ctx, in, opts...)
	}
	return &buyerpb.SubscribeRestockResponse{Code: 0}, nil
}

func (m *mockBuyerServiceClient) UnsubscribeRestock(ctx context.Context, in *buyerpb.UnsubscribeRestockRequest, opts ...grpc.CallOption) (*buyerpb.UnsubscribeRestockResponse, error) {
	if m.UnsubscribeRestockFn != nil {
		return m.UnsubscribeRestockFn(ctx, in, opts...)
	}
	return &buyerpb.UnsubscribeRestockResponse{Code: 0}, nil
}

func (m *mockBuyerServiceClient) ListRestockSubscriptions(ctx context.Context, in *buyerpb.ListRestockSubscriptionsRequest, opts ...grpc.CallOption) (*buyerpb.ListRestockSubscriptionsResponse, error) {
	if m.ListRestockSubscriptionsFn != nil {
		return m.ListRestockSubscriptionsFn(ctx, in, opts...)
	}
	return &buyerpb.ListRestockSubscriptionsResponse{Code: 0}, nil
}

func (m *mockBuyerServiceClient) BatchCheckRestockSubscriptions(ctx context.Context, in *buyerpb.BatchCheckRestockSubscriptionsRequest, opts ...grpc.CallOption) (*buyerpb.BatchCheckRestockSubscriptionsResponse, error) {
	if m.BatchCheckRestockSubscriptionsFn != nil {
		return m.BatchCheckRestockSubscriptionsFn(ctx, in, opts...)
	}
	return &buyerpb.BatchCheckRestockSubscriptionsResponse{Code: 0, Subscribed: map[string]bool{}}, nil
}

func (m *mockBuyerServiceClient) EnsureBuyerProfile(ctx context.Context, in *buyerpb.EnsureBuyerProfileRequest, opts ...grpc.CallOption) (*buyerpb.BuyerProfileResponse, error) {
	if m.EnsureBuyerProfileFn != nil {
		return m.EnsureBuyerProfileFn(ctx, in, opts...)
	}
	return &buyerpb.BuyerProfileResponse{Code: 0}, nil
}

// ListBuyerIDsByAccount is an internal-only RPC (im-service PII-purge
// consumer, D8); the gateway never calls it, so this is a plain interface
// stub — no XxxFn field, matching the other never-called-from-gateway stubs
// above (BatchCheckRestockSubscriptions etc.).
func (m *mockBuyerServiceClient) ListBuyerIDsByAccount(_ context.Context, _ *buyerpb.ListBuyerIDsByAccountRequest, _ ...grpc.CallOption) (*buyerpb.ListBuyerIDsByAccountResponse, error) {
	return &buyerpb.ListBuyerIDsByAccountResponse{Code: 0}, nil
}

// ResolvePrincipals is an internal-only cross-service RPC (im-service
// self-message guard, ② buyer re-key W2/W3); the gateway never calls it, so
// this is a plain interface stub — no XxxFn field, same pattern as
// ListBuyerIDsByAccount above.
func (m *mockBuyerServiceClient) ResolvePrincipals(_ context.Context, _ *buyerpb.ResolvePrincipalsRequest, _ ...grpc.CallOption) (*buyerpb.ResolvePrincipalsResponse, error) {
	return &buyerpb.ResolvePrincipalsResponse{Code: 0}, nil
}

func (m *mockBuyerServiceClient) GetBuyerStatsForAdmin(ctx context.Context, in *buyerpb.GetBuyerStatsForAdminRequest, opts ...grpc.CallOption) (*buyerpb.GetBuyerStatsForAdminResponse, error) {
	if m.GetBuyerStatsForAdminFn != nil {
		return m.GetBuyerStatsForAdminFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) GetSavedItems(ctx context.Context, in *buyerpb.GetSavedItemsRequest, opts ...grpc.CallOption) (*buyerpb.SavedItemsResponse, error) {
	if m.GetSavedItemsFn != nil {
		return m.GetSavedItemsFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) GetBuyerEntitlement(ctx context.Context, in *buyerpb.GetBuyerEntitlementRequest, opts ...grpc.CallOption) (*buyerpb.BuyerEntitlement, error) {
	if m.GetBuyerEntitlementFn != nil {
		return m.GetBuyerEntitlementFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) AssignMembership(ctx context.Context, in *buyerpb.AssignMembershipRequest, opts ...grpc.CallOption) (*buyerpb.BuyerEntitlement, error) {
	if m.AssignMembershipFn != nil {
		return m.AssignMembershipFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) ListMembershipPlans(ctx context.Context, in *buyerpb.ListMembershipPlansRequest, opts ...grpc.CallOption) (*buyerpb.ListMembershipPlansResponse, error) {
	if m.ListMembershipPlansFn != nil {
		return m.ListMembershipPlansFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockBuyerServiceClient) UpdateMembershipPlan(ctx context.Context, in *buyerpb.UpdateMembershipPlanRequest, opts ...grpc.CallOption) (*buyerpb.UpdateMembershipPlanResponse, error) {
	if m.UpdateMembershipPlanFn != nil {
		return m.UpdateMembershipPlanFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== Mock AIImageServiceClient / RecommendationServiceClient ====================
//
// #3519 把 legacy AIService 依授權軸拆成三個 service，所以這裡也跟著拆：一個
// mock 對一個授權軸。AIAdminServiceClient 目前沒有 handler 測試用得到，等第一個
// 需要的測試再加，不先放一份沒人用的。

type mockAIImageServiceClient struct {
	TransformImageFn func(ctx context.Context, in *aipb.TransformImageRequest, opts ...grpc.CallOption) (*aipb.TransformImageResponse, error)
	ListAiStylesFn   func(ctx context.Context, in *aipb.ListAiStylesRequest, opts ...grpc.CallOption) (*aipb.ListAiStylesResponse, error)
}

func (m *mockAIImageServiceClient) TransformImage(ctx context.Context, in *aipb.TransformImageRequest, opts ...grpc.CallOption) (*aipb.TransformImageResponse, error) {
	if m.TransformImageFn != nil {
		return m.TransformImageFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAIImageServiceClient) ListAiStyles(ctx context.Context, in *aipb.ListAiStylesRequest, opts ...grpc.CallOption) (*aipb.ListAiStylesResponse, error) {
	if m.ListAiStylesFn != nil {
		return m.ListAiStylesFn(ctx, in, opts...)
	}
	return nil, nil
}

type mockRecommendationServiceClient struct {
	PersonalizedRecommendFn func(ctx context.Context, in *aipb.PersonalizedRequest, opts ...grpc.CallOption) (*aipb.PersonalizedRecommendResponse, error)
}

func (m *mockRecommendationServiceClient) PersonalizedRecommend(ctx context.Context, in *aipb.PersonalizedRequest, opts ...grpc.CallOption) (*aipb.PersonalizedRecommendResponse, error) {
	if m.PersonalizedRecommendFn != nil {
		return m.PersonalizedRecommendFn(ctx, in, opts...)
	}
	return nil, nil
}

// ==================== mock clients ====================

type mockOrderServiceClient struct {
	orderpb.OrderServiceClient
	GetBuyerOrdersForSellerFn    func(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error)
	GetMyOrdersFn                func(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error)
	GetMyOrderStatusCountsFn     func(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error)
	GetOrderFn                   func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error)
	GetSellerOrdersFn            func(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error)
	GetSellerOrderDetailFn       func(ctx context.Context, in *orderpb.GetSellerOrderDetailRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error)
	CancelOrderFn                func(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error)
	ConfirmReceiptFn             func(ctx context.Context, in *orderpb.ConfirmReceiptRequest, opts ...grpc.CallOption) (*orderpb.ConfirmReceiptResponse, error)
	CheckoutValidateFn           func(ctx context.Context, in *orderpb.CheckoutValidateRequest, opts ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error)
	CheckoutPreviewFn            func(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error)
	CheckoutConfirmFn            func(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error)
	SnapshotPaymentInfoFn        func(ctx context.Context, in *orderpb.SnapshotPaymentInfoRequest, opts ...grpc.CallOption) (*orderpb.SnapshotPaymentInfoResponse, error)
	CreateOrderFn                func(ctx context.Context, in *orderpb.CreateOrderRequest, opts ...grpc.CallOption) (*orderpb.CreateOrderResponse, error)
	SearchOrdersFn               func(ctx context.Context, in *orderpb.SearchOrdersRequest, opts ...grpc.CallOption) (*orderpb.SearchOrdersResponse, error)
	ListAbnormalOrdersForAdminFn func(ctx context.Context, in *orderpb.ListAbnormalOrdersForAdminRequest, opts ...grpc.CallOption) (*orderpb.ListAbnormalOrdersForAdminResponse, error)
	CheckoutShippingFeeFn        func(ctx context.Context, in *orderpb.CheckoutShippingFeeRequest, opts ...grpc.CallOption) (*orderpb.CheckoutShippingFeeResponse, error)
}

func (m *mockOrderServiceClient) GetBuyerOrdersForSeller(ctx context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, opts ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
	if m.GetBuyerOrdersForSellerFn != nil {
		return m.GetBuyerOrdersForSellerFn(ctx, in, opts...)
	}
	return &orderpb.GetBuyerOrdersForSellerResponse{}, nil
}

func (m *mockOrderServiceClient) GetSellerOrders(ctx context.Context, in *orderpb.GetSellerOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
	if m.GetSellerOrdersFn != nil {
		return m.GetSellerOrdersFn(ctx, in, opts...)
	}
	return &orderpb.GetSellerOrdersResponse{Code: 0, Meta: &orderpb.PageMeta{}}, nil
}

func (m *mockOrderServiceClient) GetSellerOrderDetail(ctx context.Context, in *orderpb.GetSellerOrderDetailRequest, opts ...grpc.CallOption) (*orderpb.GetSellerOrderDetailResponse, error) {
	if m.GetSellerOrderDetailFn != nil {
		return m.GetSellerOrderDetailFn(ctx, in, opts...)
	}
	return &orderpb.GetSellerOrderDetailResponse{}, nil
}

func (m *mockOrderServiceClient) GetMyOrders(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
	if m.GetMyOrdersFn != nil {
		return m.GetMyOrdersFn(ctx, in, opts...)
	}
	return &orderpb.GetMyOrdersResponse{}, nil
}

func (m *mockOrderServiceClient) GetMyOrderStatusCounts(ctx context.Context, in *orderpb.GetMyOrderStatusCountsRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrderStatusCountsResponse, error) {
	if m.GetMyOrderStatusCountsFn != nil {
		return m.GetMyOrderStatusCountsFn(ctx, in, opts...)
	}
	return &orderpb.GetMyOrderStatusCountsResponse{}, nil
}

func (m *mockOrderServiceClient) GetOrder(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
	if m.GetOrderFn != nil {
		return m.GetOrderFn(ctx, in, opts...)
	}
	return &orderpb.GetOrderResponse{}, nil
}

func (m *mockOrderServiceClient) CancelOrder(ctx context.Context, in *orderpb.CancelOrderRequest, opts ...grpc.CallOption) (*orderpb.CancelOrderResponse, error) {
	if m.CancelOrderFn != nil {
		return m.CancelOrderFn(ctx, in, opts...)
	}
	return &orderpb.CancelOrderResponse{}, nil
}

func (m *mockOrderServiceClient) ConfirmReceipt(ctx context.Context, in *orderpb.ConfirmReceiptRequest, opts ...grpc.CallOption) (*orderpb.ConfirmReceiptResponse, error) {
	if m.ConfirmReceiptFn != nil {
		return m.ConfirmReceiptFn(ctx, in, opts...)
	}
	return &orderpb.ConfirmReceiptResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutValidate(ctx context.Context, in *orderpb.CheckoutValidateRequest, opts ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error) {
	if m.CheckoutValidateFn != nil {
		return m.CheckoutValidateFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutValidateResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutPreview(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
	if m.CheckoutPreviewFn != nil {
		return m.CheckoutPreviewFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutPreviewResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutConfirm(ctx context.Context, in *orderpb.CheckoutConfirmRequest, opts ...grpc.CallOption) (*orderpb.CheckoutConfirmResponse, error) {
	if m.CheckoutConfirmFn != nil {
		return m.CheckoutConfirmFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutConfirmResponse{}, nil
}

func (m *mockOrderServiceClient) SnapshotPaymentInfo(ctx context.Context, in *orderpb.SnapshotPaymentInfoRequest, opts ...grpc.CallOption) (*orderpb.SnapshotPaymentInfoResponse, error) {
	if m.SnapshotPaymentInfoFn != nil {
		return m.SnapshotPaymentInfoFn(ctx, in, opts...)
	}
	return &orderpb.SnapshotPaymentInfoResponse{}, nil
}

func (m *mockOrderServiceClient) CreateOrder(ctx context.Context, in *orderpb.CreateOrderRequest, opts ...grpc.CallOption) (*orderpb.CreateOrderResponse, error) {
	if m.CreateOrderFn != nil {
		return m.CreateOrderFn(ctx, in, opts...)
	}
	return &orderpb.CreateOrderResponse{}, nil
}

func (m *mockOrderServiceClient) SearchOrders(ctx context.Context, in *orderpb.SearchOrdersRequest, opts ...grpc.CallOption) (*orderpb.SearchOrdersResponse, error) {
	if m.SearchOrdersFn != nil {
		return m.SearchOrdersFn(ctx, in, opts...)
	}
	return &orderpb.SearchOrdersResponse{}, nil
}

func (m *mockOrderServiceClient) ListAbnormalOrdersForAdmin(ctx context.Context, in *orderpb.ListAbnormalOrdersForAdminRequest, opts ...grpc.CallOption) (*orderpb.ListAbnormalOrdersForAdminResponse, error) {
	if m.ListAbnormalOrdersForAdminFn != nil {
		return m.ListAbnormalOrdersForAdminFn(ctx, in, opts...)
	}
	return &orderpb.ListAbnormalOrdersForAdminResponse{}, nil
}

func (m *mockOrderServiceClient) CheckoutShippingFee(ctx context.Context, in *orderpb.CheckoutShippingFeeRequest, opts ...grpc.CallOption) (*orderpb.CheckoutShippingFeeResponse, error) {
	if m.CheckoutShippingFeeFn != nil {
		return m.CheckoutShippingFeeFn(ctx, in, opts...)
	}
	return &orderpb.CheckoutShippingFeeResponse{}, nil
}
