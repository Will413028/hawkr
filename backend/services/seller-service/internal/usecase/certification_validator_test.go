package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// ─────────────────────────────────────────────────────────────────────────────
// Email
// ─────────────────────────────────────────────────────────────────────────────

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"user.name+tag@example.co.uk", true},
		{"a@b.co", true},
		{"not-an-email", false},
		{"missing@domain", false},
		{"@nobody.com", false},
		{"", false},
		{"user@", false},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := isValidEmail(tt.email); got != tt.want {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TW Mobile / Landline
// ─────────────────────────────────────────────────────────────────────────────

func TestIsTWMobile(t *testing.T) {
	tests := []struct {
		phone string
		want  bool
	}{
		{"0912345678", true},
		{"0912-345678", true},
		{"0987-654321", true},
		{"091234567", false},   // 9 digits
		{"09123456789", false}, // 11 digits
		{"0812345678", false},  // not starting with 09
		{"+886912345678", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			if got := isTWMobile(tt.phone); got != tt.want {
				t.Errorf("isTWMobile(%q) = %v, want %v", tt.phone, got, tt.want)
			}
		})
	}
}

func TestIsTWLandline(t *testing.T) {
	tests := []struct {
		phone string
		want  bool
	}{
		{"02-29888888", true},
		{"02-9888888", true},
		{"02-9888888#123", true},
		{"037-123456", true},
		{"0229888888", true},   // 無 hyphen
		{"0229010572", true},   // 無 hyphen（使用者常見輸入）
		{"0371234567", true},   // 無 hyphen 三碼區號
		{"02-12345", false},    // 5 digits (too short)
		{"0912-345678", false}, // mobile shape mismatch (mobile has area 09XX)
		{"+886-2-9888888", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			if got := isTWLandline(tt.phone); got != tt.want {
				t.Errorf("isTWLandline(%q) = %v, want %v", tt.phone, got, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PayUni English name
// ─────────────────────────────────────────────────────────────────────────────

func TestIsPayuniEnglishName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Hawkr", true},
		{"Hawkr Market", true},
		{"O'Brien's Store", true},
		{"Store #1", true},
		{"A&B Co., Ltd.", true},
		{"示範商店", false}, // Chinese
		{"", false},
		{"Store\ttab", false}, // tab not in charset
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPayuniEnglishName(tt.name); got != tt.want {
				t.Errorf("isPayuniEnglishName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TW ID card checksum
// ─────────────────────────────────────────────────────────────────────────────

func TestIsValidTWID(t *testing.T) {
	tests := []struct {
		id   string
		note string
		want bool
	}{
		// Valid examples (checksums computed manually)
		{"A123456789", "A male", true},
		{"A123456789", "B male", true},
		{"F131104093", "F male", true},

		// Invalid checksum (last digit wrong)
		{"A123456788", "A male, bad checksum", false},
		{"A123456780", "A male, bad checksum", false},

		// Format errors
		{"a123456789", "lowercase letter", false},
		{"A12345678", "9 chars total", false},
		{"A1234567890", "11 chars total", false},
		{"1234567890", "no letter", false},
		{"A323456789", "gender digit 3 invalid", false},
		{"A023456789", "gender digit 0 invalid", false},

		// Edge letters (I=34, O=35, Z=33)
		{"M123456789", "I letter (maps to 34)", true},
		{"G133333333", "O letter (maps to 35)", true},
		{"J111111111", "Z letter (maps to 33)", true},

		// Empty
		{"", "empty", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := isValidTWID(tt.id)
			if got != tt.want {
				t.Errorf("isValidTWID(%q) = %v, want %v (%s)", tt.id, got, tt.want, tt.note)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// validatePayuniFields — integration (multiple field aggregation)
// ─────────────────────────────────────────────────────────────────────────────

// individualSellerHappy returns a valid individual-seller certification application
// and seller. Tests can mutate specific fields to test validation failures.
func individualSellerHappy() (*domain.CertificationApplication, *domain.Seller) {
	app := &domain.CertificationApplication{
		AccountID:          1,
		SellerType:         "individual",
		RealName:           "王小明",
		IDCardNo:           "A123456789",
		EnglishName:        "Xiao Ming Wang",
		IDCardFrontAssetID: "asset-id-card-front",
		IDCardBackAssetID:  "asset-id-card-back",
		BankBookAssetID:    "asset-bank-book",
	}
	seller := &domain.Seller{
		AccountID:    1,
		StoreName:    "示範小店",
		Description:  "新鮮蔬果",
		ContactPhone: "0912345678",
		City:         "台北市",
		District:     "信義區",
		PostalCode:   "110",
	}
	return app, seller
}

func TestValidatePayuniFields_Happy(t *testing.T) {
	app, seller := individualSellerHappy()
	err := validatePayuniFields(app, seller, "user@example.com", "0912345678")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidatePayuniFields_MissingIdentityDocuments(t *testing.T) {
	app, seller := individualSellerHappy()
	app.IDCardFrontAssetID = ""
	app.IDCardBackAssetID = ""
	app.BankBookAssetID = ""
	err := validatePayuniFields(app, seller, "user@example.com", "0912345678")
	if err == nil {
		t.Fatal("expected error when identity document asset IDs are missing")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(verr.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d: %v", len(verr.Messages), verr.Messages)
	}
	joined := verr.Error()
	for _, expected := range []string{"身分證正面照片", "身分證反面照片", "存摺封面"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("expected %q in error, got %q", expected, joined)
		}
	}
}

func TestValidatePayuniFields_SingleField_EmailInvalid(t *testing.T) {
	app, seller := individualSellerHappy()
	err := validatePayuniFields(app, seller, "not-an-email", "0912345678")
	if err == nil {
		t.Fatal("expected error")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(verr.Messages) != 1 {
		t.Errorf("expected 1 message, got %d: %v", len(verr.Messages), verr.Messages)
	}
	if !strings.Contains(verr.Error(), "電子信箱") {
		t.Errorf("expected 電子信箱 in error, got %q", verr.Error())
	}
}

func TestValidatePayuniFields_MultipleFields(t *testing.T) {
	app, seller := individualSellerHappy()
	app.EnglishName = "示範商店"    // Chinese — fails charset
	app.IDCardNo = "A000000000" // bad checksum
	seller.ContactPhone = "bad" // so fallback also fails
	err := validatePayuniFields(app, seller, "bad-email", "091234567")

	if err == nil {
		t.Fatal("expected error")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	// Expect at least 4 field errors
	if len(verr.Messages) < 4 {
		t.Errorf("expected ≥4 messages, got %d: %v", len(verr.Messages), verr.Messages)
	}
	joined := verr.Error()
	for _, expected := range []string{"電子信箱", "聯絡手機", "英文名稱", "身分證字號"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("expected %q in error, got %q", expected, joined)
		}
	}
	// Must use full-width semicolon separator
	if !strings.Contains(joined, "；") {
		t.Errorf("expected full-width semicolon separator in %q", joined)
	}
}

func TestValidatePayuniFields_FallsBackToSellerPhone(t *testing.T) {
	// account-service phone is bad, but seller.ContactPhone is valid.
	// Should pass (we allow either source).
	app, seller := individualSellerHappy()
	seller.ContactPhone = "0987654321"
	err := validatePayuniFields(app, seller, "user@example.com", "bad-phone")
	if err != nil {
		t.Errorf("expected nil (seller phone fallback), got %v", err)
	}
}

func TestValidatePayuniFields_BusinessSeller(t *testing.T) {
	app := &domain.CertificationApplication{
		AccountID:            1,
		SellerType:           "business",
		CompanyName:          "示範有限公司",
		TaxID:                "11111101",   // valid
		IDCardNo:             "A123456789", // responsible person
		CompanyPhone:         "02-29888888",
		CompanyCity:          "台北市",
		CompanyDistrict:      "信義區",
		CompanyAddressDetail: "信義路五段 7 號",
		EnglishName:          "Hawkr Ltd",
		IDCardFrontAssetID:   "asset-id-card-front",
		IDCardBackAssetID:    "asset-id-card-back",
		BankBookAssetID:      "asset-bank-book",
	}
	seller := &domain.Seller{
		AccountID:   1,
		StoreName:   "示範商店",
		Description: "新鮮",
	}
	err := validatePayuniFields(app, seller, "biz@example.com", "0912345678")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidatePayuniFields_BusinessSeller_BadTaxID(t *testing.T) {
	app := &domain.CertificationApplication{
		AccountID:       1,
		SellerType:      "business",
		CompanyName:     "Test",
		TaxID:           "1234", // bad format (not 8 digits); checksum no longer validated
		IDCardNo:        "A123456789",
		CompanyPhone:    "02-29888888",
		CompanyCity:     "台北市",
		CompanyDistrict: "信義區",
		EnglishName:     "Test",
	}
	seller := &domain.Seller{AccountID: 1}
	err := validatePayuniFields(app, seller, "biz@example.com", "0912345678")
	if err == nil {
		t.Fatal("expected tax ID error")
	}
	if !strings.Contains(err.Error(), "統一編號") {
		t.Errorf("expected 統一編號 error, got %q", err.Error())
	}
}

func TestValidationError_ErrorFormat(t *testing.T) {
	verr := &ValidationError{Messages: []string{"A", "B", "C"}}
	if got := verr.Error(); got != "A；B；C" {
		t.Errorf("got %q, want %q", got, "A；B；C")
	}
}

func TestValidationError_ImplementsDomainCoder(t *testing.T) {
	verr := &ValidationError{Messages: []string{"test"}}
	if verr.DomainCode() != 400 {
		t.Errorf("DomainCode() = %d, want 400", verr.DomainCode())
	}
	if verr.DomainMessage() != "test" {
		t.Errorf("DomainMessage() = %q, want %q", verr.DomainMessage(), "test")
	}
}
