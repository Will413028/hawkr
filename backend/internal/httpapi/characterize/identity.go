package characterize

import (
	"net/http"
	"os"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// 呼叫者的身分是 Bearer token：顧客是資料庫裡的 dev session（BuyerSession），後台是本機金鑰簽的 dev token（DevAuth）。兩種 driver 都以 Authorization: Bearer 帶。REST driver 打單體的入口（/api/v1 由
// 單體轉給 gateway，身分 header 由單體設上），Connect driver 直接打單體。

// Auth 是直接打單體的測試要裝進 identity 的 dev 驗證，與各呼叫者的 token。
type Auth struct {
	// Providers 放進 identity.Deps.Providers。
	Providers identity.Providers
	Tokens    map[Caller]string
}

// DevAuth 產生一把測試用的金鑰，回傳接受它的 identity 設定與後台的 token（第一次請求時建立管理員）。顧客的 token 是資料庫裡的
// session，identity 建好之後用 BuyerSession 建。管理員建立時沒有任何角色（ADR 2026-10-04-admin-roles）：直接打單體的測試要在第一次認證之後以 GrantAdminOwner 授予 OWNER。
func DevAuth(t *testing.T) Auth {
	t.Helper()
	d, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := d.AdminToken("dev_characterize_admin")
	if err != nil {
		t.Fatal(err)
	}
	return Auth{
		Providers: identity.Providers{Admin: d.Admins()},
		Tokens:    map[Caller]string{Admin: admin},
	}
}

// GrantAdminOwner 授予管理員 OWNER：先以他的 token 認證一次建立管理員，再授予角色。要打 admin API 或 /api/files/admin 的測試
// 都要有它，否則管理員沒有任何權限。
func GrantAdminOwner(t *testing.T, m *identity.Module, adminToken string) {
	t.Helper()
	ctx := t.Context()
	a, err := m.Authenticate(ctx, adminToken)
	if err != nil {
		t.Fatalf("認證管理員：%v", err)
	}
	_, err = m.GrantAdminRole(ctx, identity.AdminRoleChangeInput{
		AdminID: a.Principal.ID, Role: adminperm.RoleOwner, Reason: "測試的管理員", Source: identity.AdminRoleSourceCLI,
	})
	if err != nil {
		t.Fatalf("授予 OWNER：%v", err)
	}
}

// Bearer 帶入 tokens 裡這個呼叫者的 session token。
func Bearer(tokens map[Caller]string) func(Caller, http.Header) {
	return func(c Caller, h http.Header) {
		if v := tokens[c]; v != "" {
			h.Set("Authorization", "Bearer "+v)
		}
	}
}

// 經入口執行時由 scripts/characterize.sh 設定的環境變數。
const (
	// EnvEntryURL 是本機起的單體（入口）的位址：Connect 直接打它，舊的 REST 經它轉給 gateway。
	EnvEntryURL = "ENTRY_URL"
	// EnvBuyerToken、EnvAdminToken 是腳本以 hawkr dev-token 簽的 token。
	EnvBuyerToken = "CHARACTERIZE_BUYER_TOKEN"
	EnvAdminToken = "CHARACTERIZE_ADMIN_TOKEN"
	// EnvSellerToken、EnvSeller2Token、EnvBuyer2Token 只有需要多個身分的模組的 legacy.sh 才設定（讀不到就不放進 Tokens）。
	EnvSellerToken  = "CHARACTERIZE_SELLER_TOKEN"
	EnvSeller2Token = "CHARACTERIZE_SELLER2_TOKEN"
	EnvBuyer2Token  = "CHARACTERIZE_BUYER2_TOKEN"
	// EnvFakeMediaAddr 是舊服務與單體設定的 media-service 位址（host:port），由測試以 ServeFakeMedia 聽。
	EnvFakeMediaAddr = "FAKE_MEDIA_ADDR"
)

// RequireEnv 讀經入口執行時的設定。這些測試只在 scripts/characterize.sh 起服務後執行（build tag gateway），
// 缺設定就是跑錯了，直接失敗，不略過。
func RequireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("缺少 %s：用 scripts/characterize.sh 執行", key)
	}
	return v
}

// EnvTokens 讀腳本簽的 token。
func EnvTokens(t *testing.T) map[Caller]string {
	t.Helper()
	tokens := map[Caller]string{Buyer: RequireEnv(t, EnvBuyerToken), Admin: RequireEnv(t, EnvAdminToken)}
	for c, key := range map[Caller]string{Seller: EnvSellerToken, Seller2: EnvSeller2Token, Buyer2: EnvBuyer2Token} {
		if v := os.Getenv(key); v != "" {
			tokens[c] = v
		}
	}
	return tokens
}

// BuyerSession 為 dev 買家在 m 建一個 session 並回傳 token（帳號沒有時建立）。
func BuyerSession(t *testing.T, m *identity.Module) string {
	t.Helper()
	_, tok, err := m.DevNewSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return tok
}
