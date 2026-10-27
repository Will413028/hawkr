package config

import "testing"

// backend#3329 起，TGOS 四項與 PayUni 代理商憑證**不再是啟動必要條件**。
//
// 本測試取代原本的 TestValidate_RequiresTGOSFields（斷言相反）。
//
// ⚠️ 順帶記一筆：那支舊測試在本次改動之後其實**仍然會通過**——它的 fixture 沒有填
// MediaBaseURL，所以 Validate 會因為 MEDIA_BASE_URL 而回錯，跟 TGOS 一點關係都沒有。
// 也就是說它對「TGOS 到底必不必填」從頭到尾零鑑別力。直接刪掉而不是留著，
// 理由是留著會讓下一個人以為那件事有被驗過。
func TestValidate_AllowsUnprovisionedOptionalDependencies(t *testing.T) {
	cfg := &Config{
		Database:            DatabaseConfig{Host: "h", Port: "5432", User: "u", Password: "p", DBName: "d"},
		Server:              ServerConfig{GRPCPort: "10020"},
		EncryptionMasterKey: "test-master-key",
		MediaBaseURL:        "https://test-media.example.com",
		// TGOS 四項與 PAYUNI_AGENT_HASH_KEY／IV 刻意全空 ＝ 尚未開通。
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("TGOS 與 PayUni 未開通是合法的部署狀態，Validate 不得失敗：%v", err)
	}
	if cfg.GeocodeConfigured() || cfg.PayuniMerchantConfigured() {
		t.Fatal("全空時兩個 Configured 都必須為 false")
	}
	if cfg.GeocodePartiallyConfigured() || cfg.PayuniMerchantPartiallyConfigured() {
		t.Fatal("全空是「尚未開通」不是「配置錯誤」，兩個 PartiallyConfigured 都必須為 false")
	}
}

// 陰性對照：真正必要的設定不得被順手放寬。
//
// 少了這一格，「把 Validate 改成 return nil」也會讓上面那支全綠——本次要放寬的
// 只有業務前置條件，不是所有設定。
func TestValidate_StillRejectsGenuinelyRequired(t *testing.T) {
	base := func() *Config {
		return &Config{
			Database:            DatabaseConfig{Host: "h", Port: "5432", User: "u", Password: "p", DBName: "d"},
			Server:              ServerConfig{GRPCPort: "10020"},
			EncryptionMasterKey: "test-master-key",
			MediaBaseURL:        "https://test-media.example.com",
		}
	}
	cases := map[string]func(*Config){
		"ENCRYPTION_MASTER_KEY": func(c *Config) { c.EncryptionMasterKey = "" },
		"MEDIA_BASE_URL":        func(c *Config) { c.MediaBaseURL = "" },
		"DB_HOST":               func(c *Config) { c.Database.Host = "" },
		"GRPC_PORT":             func(c *Config) { c.Server.GRPCPort = "" },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := base()
			breakIt(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("%s 是真正的必要設定，缺值時 Validate 必須失敗", name)
			}
		})
	}
}

// PayUni 憑證的三分支：全有＝已開通、全無＝尚未開通（皆合法）、只有一半＝配置錯誤。
func TestPayuniMerchantPartiallyConfigured(t *testing.T) {
	cases := []struct {
		name                       string
		key, iv                    string
		wantConfigured, wantPartly bool
	}{
		{"全空＝尚未開通", "", "", false, false},
		{"全有＝已開通", "12345678901234567890123456789012", "1234567890123456", true, false},
		{"只有 key＝配置錯誤", "12345678901234567890123456789012", "", false, true},
		{"只有 iv＝配置錯誤", "", "1234567890123456", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{Payuni: PayuniConfig{HashKey: c.key, HashIV: c.iv}}
			if got := cfg.PayuniMerchantConfigured(); got != c.wantConfigured {
				t.Fatalf("PayuniMerchantConfigured 應為 %v，實際 %v", c.wantConfigured, got)
			}
			if got := cfg.PayuniMerchantPartiallyConfigured(); got != c.wantPartly {
				t.Fatalf("PayuniMerchantPartiallyConfigured 應為 %v，實際 %v", c.wantPartly, got)
			}
		})
	}
}

func TestValidate_AllTGOSFieldsPresent_NoError(t *testing.T) {
	cfg := &Config{
		Database:               DatabaseConfig{Host: "h", Port: "5432", User: "u", Password: "p", DBName: "d"},
		Server:                 ServerConfig{GRPCPort: "10020"},
		Payuni:                 PayuniConfig{HashKey: "12345678901234567890123456789012", HashIV: "1234567890123456"},
		EncryptionMasterKey:    "test-master-key",
		TGOSBaseURL:            "https://tgos-proxy.example.com",
		TGOSQueryAddrAppID:     "test-appid",
		TGOSQueryAddrAPIKey:    "test-apikey",
		TGOSProxyCallerKeyFile: "/secrets/tgos-proxy-caller/key.json",
		MediaBaseURL:           "https://test-media.example.com",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingEncryptionMasterKey_FailsClosed(t *testing.T) {
	cfg := &Config{
		Database:               DatabaseConfig{Host: "h", Port: "5432", User: "u", Password: "p", DBName: "d"},
		Server:                 ServerConfig{GRPCPort: "10020"},
		Payuni:                 PayuniConfig{HashKey: "12345678901234567890123456789012", HashIV: "1234567890123456"},
		TGOSBaseURL:            "https://tgos-proxy.example.com",
		TGOSQueryAddrAppID:     "test-appid",
		TGOSQueryAddrAPIKey:    "test-apikey",
		TGOSProxyCallerKeyFile: "/secrets/tgos-proxy-caller/key.json",
		// EncryptionMasterKey intentionally empty — #608 requires fail-closed.
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when ENCRYPTION_MASTER_KEY is empty (fail-closed)")
	}
}
