package grpc

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// ==================== toSellerInfoProto ====================

func TestToSellerInfoProto_Nil(t *testing.T) {
	if got := toSellerInfoProto(nil, "https://test-media.example.com"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestToSellerInfoProto_Fields(t *testing.T) {
	p := &dto.SellerProfile{
		AccountID:     2600000001,
		StoreID:       2600000099,
		StoreName:     "Test Store",
		Logo:          "public/other/2600000001/logo.png",
		Description:   "desc",
		ContactPhone:  "0912345678",
		City:          "台北市",
		District:      "中山區",
		AddressDetail: "中山路1號",
		PostalCode:    "104",
		Rating:        4.5,
		CoverKey:      "public/other/2600000001/cover.png",
		Prize:         "prize",
	}
	got := toSellerInfoProto(p, "https://test-media.example.com")
	if got.Name != "Test Store" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Store")
	}
	if got.Rating != 4.5 {
		t.Errorf("Rating = %f, want 4.5", got.Rating)
	}
	if got.LogoKey != "public/other/2600000001/logo.png" {
		t.Errorf("LogoKey = %q, want raw objectKey", got.LogoKey)
	}
	if got.CoverKey != "public/other/2600000001/cover.png" {
		t.Errorf("CoverKey = %q, want raw objectKey", got.CoverKey)
	}
	if got.City != "台北市" {
		t.Errorf("City = %q, want %q", got.City, "台北市")
	}
	if got.District != "中山區" {
		t.Errorf("District = %q, want %q", got.District, "中山區")
	}
	if got.PostalCode != "104" {
		t.Errorf("PostalCode = %q, want %q", got.PostalCode, "104")
	}
	if got.StoreId != "2600000099" {
		t.Errorf("StoreId = %q, want %q", got.StoreId, "2600000099")
	}
}

// TestToSellerInfoProto_StoreCode proves the mapper forwards the display
// code (dto.SellerProfile.StoreCode, ultimately domain.Seller.StoreCode)
// onto the wire as its own field, distinct from StoreID — not silently
// dropped, and not aliased to StoreID. Uses different values (2600000099 vs
// 3012345678) so the assertion can't pass by accident.
func TestToSellerInfoProto_StoreCode(t *testing.T) {
	p := &dto.SellerProfile{
		AccountID: 2600000001,
		StoreID:   2600000099,
		StoreCode: 3012345678,
		StoreName: "Test Store",
	}
	got := toSellerInfoProto(p, "https://test-media.example.com")
	if got.StoreCode != "3012345678" {
		t.Errorf("StoreCode = %q, want %q (dto.SellerProfile.StoreCode, numeric string)", got.StoreCode, "3012345678")
	}
	if got.StoreId != "2600000099" {
		t.Errorf("StoreId = %q, want %q (unchanged)", got.StoreId, "2600000099")
	}
}

func TestToSellerInfoProto_StoreIDAndLifecycle(t *testing.T) {
	p := &dto.SellerProfile{
		AccountID: 42,
		StoreID:   777,
		Status:    "closed",
		StoreName: "老王鮮魚",
	}
	got := toSellerInfoProto(p, "https://cdn")
	if got.StoreId != "777" {
		t.Errorf("StoreId = %q, want \"777\"", got.StoreId)
	}
	if got.LifecycleStatus != "closed" {
		t.Errorf("LifecycleStatus = %q, want closed", got.LifecycleStatus)
	}
}

func TestBuildCdnUrl(t *testing.T) {
	tests := []struct {
		name      string
		cdnBase   string
		objectKey string
		want      string
	}{
		{"empty key", "https://test-media.example.com", "", ""},
		{"object key", "https://test-media.example.com", "public/other/123/logo.png", "https://test-media.example.com/i/original/public/other/123/logo.png"},
		{"already http", "https://test-media.example.com", "https://example.com/img.png", "https://example.com/img.png"},
		{"trailing slash", "https://test-media.example.com/", "public/logo.png", "https://test-media.example.com/i/original/public/logo.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mediaurl.BuildURL(tt.cdnBase, tt.objectKey); got != tt.want {
				t.Errorf("mediaurl.BuildURL(%q, %q) = %q, want %q", tt.cdnBase, tt.objectKey, got, tt.want)
			}
		})
	}
}

// ==================== toSellerMetaProto ====================

func TestToSellerMetaProto_Nil(t *testing.T) {
	if got := toSellerMetaProto(nil, "https://cdn.test"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestToSellerMetaProto_Fields(t *testing.T) {
	m := &dto.SellerMeta{
		AccountID:         100,
		SellerName:        "Shop A",
		LogoURL:           "logo.png",
		CanAcceptOrders:   true,
		CanManageProducts: true,
	}
	got := toSellerMetaProto(m, "https://cdn.test")
	if got.AccountId != 100 {
		t.Errorf("AccountId = %d, want 100", got.AccountId)
	}
	if got.SellerName != "Shop A" {
		t.Errorf("SellerName = %q, want %q", got.SellerName, "Shop A")
	}
	if !got.CanAcceptOrders {
		t.Errorf("CanAcceptOrders = false, want true")
	}
	if !got.CanManageProducts {
		t.Errorf("CanManageProducts = false, want true")
	}
	if got := toSellerMetaProto(&dto.SellerMeta{CanAcceptOrders: true}, ""); got.CanManageProducts {
		t.Errorf("CanManageProducts 不得由 CanAcceptOrders 推得")
	}
}

// TestToSellerMetaProto_StoreID proves the mapper forwards the store-stint
// surrogate (dto.SellerMeta.StoreID) onto the wire as its own field, distinct
// from AccountID — not silently dropped, and not aliased to AccountID. Uses different
// values (100 vs 8001) so the assertion can't pass by accident (② re-key discriminating
// test convention).
func TestToSellerMetaProto_StoreID(t *testing.T) {
	m := &dto.SellerMeta{
		AccountID:  100,
		StoreID:    8001,
		SellerName: "Shop A",
	}
	got := toSellerMetaProto(m, "https://cdn.test")
	if got.StoreId != "8001" {
		t.Errorf("StoreId = %q, want %q (dto.SellerMeta.StoreID, §6 numeric string); AccountId=%d", got.StoreId, "8001", got.AccountId)
	}
	if got.AccountId != 100 {
		t.Errorf("AccountId = %d, want 100 (unchanged)", got.AccountId)
	}
}

// TestToSellerMetaProto_StoreCode proves the mapper forwards the display
// code (dto.SellerMeta.StoreCode) onto the wire as its own field, distinct
// from StoreID. Uses different values (8001 vs 5099887766) so the assertion
// can't pass by accident (② re-key discriminating test convention).
func TestToSellerMetaProto_StoreCode(t *testing.T) {
	m := &dto.SellerMeta{
		AccountID:  100,
		StoreID:    8001,
		StoreCode:  5099887766,
		SellerName: "Shop A",
	}
	got := toSellerMetaProto(m, "https://cdn.test")
	if got.StoreCode != "5099887766" {
		t.Errorf("StoreCode = %q, want %q (dto.SellerMeta.StoreCode, numeric string)", got.StoreCode, "5099887766")
	}
	if got.StoreId != "8001" {
		t.Errorf("StoreId = %q, want %q (unchanged)", got.StoreId, "8001")
	}
}

// ==================== toPayuniStatusProto ====================

func TestToPayuniStatusProto_MerStatusString(t *testing.T) {
	profile := &dto.SellerProfile{
		AccountID:    2600000001,
		PayuniStatus: int(domain.PayuniMerStatusActive), // 1 → "Active"
	}
	registeredAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	approvedAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	lastSyncedAt := time.Date(2026, 8, 29, 3, 0, 0, 0, time.UTC)
	ps := &dto.PayuniStatus{
		PayuniMerID:    "HAWK2612345678",
		Status:         "active",
		MerStatus:      "1",
		CreditStatus:   "0",
		Reason:         "身分證影本模糊",
		ApplePayStatus: "1", GooglePayStatus: "0", ATMStatus: "1", ErrorMessage: "timeout",
		RegisteredAt: &registeredAt, ApprovedAt: &approvedAt, LastSyncedAt: &lastSyncedAt,
	}
	got := toPayuniStatusProto(profile, ps)
	if got.AccountId != "2600000001" {
		t.Errorf("AccountId = %s, want 2600000001", got.AccountId)
	}
	if got.PayuniMerStatus != "Active" {
		t.Errorf("PayuniMerStatus = %q, want %q", got.PayuniMerStatus, "Active")
	}
	if got.MerStatus != "1" {
		t.Errorf("MerStatus = %q, want %q", got.MerStatus, "1")
	}
	if got.Reason != "身分證影本模糊" {
		t.Errorf("Reason = %q, want %q", got.Reason, "身分證影本模糊")
	}
	if got.ApplepayStatus != "1" {
		t.Errorf("ApplepayStatus = %q, want %q", got.ApplepayStatus, "1")
	}
	if got.GooglepayStatus != "0" {
		t.Errorf("GooglepayStatus = %q, want %q", got.GooglepayStatus, "0")
	}
	if got.AtmStatus != "1" {
		t.Errorf("AtmStatus = %q, want %q", got.AtmStatus, "1")
	}
	if got.ErrorMessage != "timeout" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "timeout")
	}
	if got.RegisteredAt != registeredAt.Unix() {
		t.Errorf("RegisteredAt = %d, want %d", got.RegisteredAt, registeredAt.Unix())
	}
	if got.ApprovedAt != approvedAt.Unix() {
		t.Errorf("ApprovedAt = %d, want %d", got.ApprovedAt, approvedAt.Unix())
	}
	if got.LastSyncedAt != lastSyncedAt.Unix() {
		t.Errorf("LastSyncedAt = %d, want %d", got.LastSyncedAt, lastSyncedAt.Unix())
	}
	if got.Freshness != "" {
		t.Errorf("Freshness = %q, want empty (ps.Freshness not set in this fixture)", got.Freshness)
	}
}

func TestToPayuniStatusProto_FreshnessAndAllowedActions(t *testing.T) {
	profile := &dto.SellerProfile{AccountID: 2600000001}
	ps := &dto.PayuniStatus{Freshness: "stale", AllowedActions: []string{"resync"}}
	got := toPayuniStatusProto(profile, ps)
	if got.Freshness != "stale" {
		t.Errorf("Freshness = %q, want %q", got.Freshness, "stale")
	}
	if len(got.AllowedActions) != 1 || got.AllowedActions[0] != "resync" {
		t.Errorf("AllowedActions = %v, want [resync]", got.AllowedActions)
	}
}

// TestToPayuniStatusProto_MerchantProfileFields covers issue #2806 Phase 2's
// GetPayuniStatus extension: the 7 contact/address fields plus the
// optimistic-concurrency version, exposed so the Web edit form can GET the
// current values before PATCHing.
func TestToPayuniStatusProto_MerchantProfileFields(t *testing.T) {
	profile := &dto.SellerProfile{AccountID: 2600000001}
	ps := &dto.PayuniStatus{
		MerchantProfileVersion: 3,
		ContactName:            "王小明",
		ContactMobile:          "0912345678",
		ContactTel:             "",
		MerEmail:               "seller@example.com",
		MerAddr:                "台北市大安區某路 1 號",
		MerAddrPost:            "106",
		MerTel:                 "",
	}
	got := toPayuniStatusProto(profile, ps)
	if got.MerchantProfileVersion != "3" {
		t.Errorf("MerchantProfileVersion = %q, want %q", got.MerchantProfileVersion, "3")
	}
	if got.ContactName != "王小明" {
		t.Errorf("ContactName = %q, want %q", got.ContactName, "王小明")
	}
	if got.ContactMobile != "0912345678" {
		t.Errorf("ContactMobile = %q, want %q", got.ContactMobile, "0912345678")
	}
	if got.MerEmail != "seller@example.com" {
		t.Errorf("MerEmail = %q, want %q", got.MerEmail, "seller@example.com")
	}
	if got.MerAddr != "台北市大安區某路 1 號" {
		t.Errorf("MerAddr = %q, want %q", got.MerAddr, "台北市大安區某路 1 號")
	}
	if got.MerAddrPost != "106" {
		t.Errorf("MerAddrPost = %q, want %q", got.MerAddrPost, "106")
	}
}

// TestToPayuniStatusProto_MerchantProfileVersionZero locks in that a fresh
// registration (never PATCHed) reports version "0", not an empty string —
// distinguishing it from the "no registration at all" branch in
// SellerServer.GetPayuniStatus, which bypasses this mapper entirely and
// leaves the proto field at its zero value ("").
func TestToPayuniStatusProto_MerchantProfileVersionZero(t *testing.T) {
	profile := &dto.SellerProfile{AccountID: 2600000001}
	ps := &dto.PayuniStatus{MerchantProfileVersion: 0}
	got := toPayuniStatusProto(profile, ps)
	if got.MerchantProfileVersion != "0" {
		t.Errorf("MerchantProfileVersion = %q, want %q", got.MerchantProfileVersion, "0")
	}
}

func TestToPayuniStatusProto_Unregistered(t *testing.T) {
	profile := &dto.SellerProfile{
		AccountID:    2600000001,
		PayuniStatus: int(domain.PayuniMerStatusUnregistered), // 0 → "Unregistered"
	}
	ps := &dto.PayuniStatus{}
	got := toPayuniStatusProto(profile, ps)
	if got.PayuniMerStatus != "Unregistered" {
		t.Errorf("PayuniMerStatus = %q, want %q", got.PayuniMerStatus, "Unregistered")
	}
}

func TestToPayuniStatusProto_NilTimestamps(t *testing.T) {
	profile := &dto.SellerProfile{AccountID: 2600000001}
	ps := &dto.PayuniStatus{}
	got := toPayuniStatusProto(profile, ps)
	if got.RegisteredAt != 0 || got.ApprovedAt != 0 || got.LastSyncedAt != 0 {
		t.Errorf("timestamps = %d/%d/%d, want all 0 for nil *time.Time", got.RegisteredAt, got.ApprovedAt, got.LastSyncedAt)
	}
}

// ==================== toSellerPayuniConfigProto ====================

func TestToSellerPayuniConfigProto_CreditEnabled(t *testing.T) {
	profile := &dto.SellerProfile{AccountID: 2600000001, StoreName: "Shop"}
	ps := &dto.PayuniStatus{PayuniMerID: "HAWK2612345678", PaymentEnabled: true, CreditStatus: "1"}

	got := toSellerPayuniConfigProto(profile, ps)
	if !got.CreditEnabled {
		t.Error("CreditEnabled = false, want true when CreditStatus==\"1\"")
	}
	if !got.IsEnabled {
		t.Error("IsEnabled = false, want true when PaymentEnabled==true")
	}
}

func TestToSellerPayuniConfigProto_CreditDisabled(t *testing.T) {
	profile := &dto.SellerProfile{AccountID: 2600000001, StoreName: "Shop"}
	ps := &dto.PayuniStatus{CreditStatus: "0"}

	got := toSellerPayuniConfigProto(profile, ps)
	if got.CreditEnabled {
		t.Error("CreditEnabled = true, want false when CreditStatus==\"0\"")
	}
}

// ==================== toCertificationApplicationProto ====================

func TestToCertificationApplicationProto_Nil(t *testing.T) {
	if got := toCertificationApplicationProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestToCertificationApplicationProto_SubmittedAtNil(t *testing.T) {
	now := time.Now()
	app := &dto.CertificationApplication{
		ID:          42,
		AccountID:   100,
		Status:      "submitted",
		CurrentStep: 3,
		TotalSteps:  8,
		CreatedAt:   now,
		UpdatedAt:   now,
		SubmittedAt: nil, // nil → should produce empty string
	}
	got := toCertificationApplicationProto(app)
	if got.Id != "42" {
		t.Errorf("Id = %s, want 42", got.Id)
	}
	if got.CurrentStep != 3 {
		t.Errorf("CurrentStep = %d, want 3", got.CurrentStep)
	}
	if got.SubmittedAt != "" {
		t.Errorf("SubmittedAt = %q, want empty string for nil input", got.SubmittedAt)
	}
}

func TestToCertificationApplicationProto_SubmittedAtSet(t *testing.T) {
	now := time.Now().Truncate(time.Second) // RFC3339 only has second precision
	app := &dto.CertificationApplication{
		ID:          42,
		CreatedAt:   now,
		UpdatedAt:   now,
		SubmittedAt: &now,
	}
	got := toCertificationApplicationProto(app)
	if got.SubmittedAt == "" {
		t.Error("SubmittedAt should not be empty when SubmittedAt is set")
	}
	want := now.UTC().Format(time.RFC3339)
	if got.SubmittedAt != want {
		t.Errorf("SubmittedAt = %q, want %q", got.SubmittedAt, want)
	}
}

// Regression: admin 審核頁「出生年月日」與「公司地址」一直顯示 '-'，
// root cause 是這個 mapper 漏寫 BirthDate / CompanyCity / CompanyDistrict /
// CompanyAddressDetail 四個 proto setter。此 test 覆蓋所有頁面實際使用的欄位，
// 預期未來新增欄位時補 mapper + 補這個 test。
func TestToCertificationApplicationProto_AdminPageFields(t *testing.T) {
	birth := time.Date(1990, 5, 18, 0, 0, 0, 0, time.UTC)
	issue := time.Date(2018, 7, 1, 0, 0, 0, 0, time.UTC)
	reg := time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC)
	app := &dto.CertificationApplication{
		ID:                   1,
		AccountID:            2600000001,
		ApplicationNo:        "APP-001",
		Status:               "submitted",
		SellerType:           "business",
		RealName:             "王小明",
		IDCardNo:             "A123456789",
		BirthDate:            &birth,
		IDCardFrontAssetID:   "front.png",
		IDCardBackAssetID:    "back.png",
		BankBookAssetID:      "bank.png",
		DocumentAssetIDs:     []string{"doc1.png", "doc2.png"},
		BankName:             "玉山銀行",
		BankCode:             "808",
		BankAccount:          "1234567890",
		CompanyName:          "Hawkr Co.",
		TaxID:                "12345678",
		CompanyCity:          "台北市",
		CompanyDistrict:      "中山區",
		CompanyAddressDetail: "中山路 130 號 5 樓",
		CompanyType:          "limited",
		CompanyEmail:         "ops@example.com",
		CompanyPhone:         "02-1234-5678",
		ProductType:          "fresh-produce",
		FoodBusinessRegNo:    "F-12345",
		IDCardIssueDate:      &issue,
		IDCardIssueType:      "renewal",
		IDCardIssueLocation:  "台北市",
		CompanyRegisterDate:  &reg,
	}
	got := toCertificationApplicationProto(app)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ApplicationNo", got.ApplicationNo, "APP-001"},
		{"Status", got.Status, "submitted"},
		{"SellerType", got.SellerType, "business"},
		{"RealName", got.RealName, "王小明"},
		{"IdCardNo", got.IdCardNo, "A123456789"},
		{"BirthDate", got.BirthDate, "079/05/18"},
		{"IdCardFrontAssetId", got.IdCardFrontAssetId, "front.png"},
		{"IdCardBackAssetId", got.IdCardBackAssetId, "back.png"},
		{"BankBookAssetId", got.BankBookAssetId, "bank.png"},
		{"BankName", got.BankName, "玉山銀行"},
		{"BankCode", got.BankCode, "808"},
		{"BankAccount", got.BankAccount, "1234567890"},
		{"CompanyName", got.CompanyName, "Hawkr Co."},
		{"TaxId", got.TaxId, "12345678"},
		{"CompanyCity", got.CompanyCity, "台北市"},
		{"CompanyDistrict", got.CompanyDistrict, "中山區"},
		{"CompanyAddressDetail", got.CompanyAddressDetail, "中山路 130 號 5 樓"},
		{"CompanyType", got.CompanyType, "limited"},
		{"CompanyEmail", got.CompanyEmail, "ops@example.com"},
		{"CompanyPhone", got.CompanyPhone, "02-1234-5678"},
		{"ProductType", got.ProductType, "fresh-produce"},
		{"FoodBusinessRegNo", got.FoodBusinessRegNo, "F-12345"},
		{"IdCardIssueDate", got.IdCardIssueDate, "107/07/01"},
		{"IdCardIssueType", got.IdCardIssueType, "renewal"},
		{"IdCardIssueLocation", got.IdCardIssueLocation, "台北市"},
		{"CompanyRegisterDate", got.CompanyRegisterDate, "109/03/15"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if len(got.DocumentAssetIds) != 2 || got.DocumentAssetIds[0] != "doc1.png" {
		t.Errorf("DocumentAssetIds = %v, want [doc1.png doc2.png]", got.DocumentAssetIds)
	}
}

func TestToCertificationApplicationProto_BirthDateNil(t *testing.T) {
	app := &dto.CertificationApplication{ID: 1, BirthDate: nil}
	got := toCertificationApplicationProto(app)
	if got.BirthDate != "" {
		t.Errorf("BirthDate = %q, want empty string for nil input", got.BirthDate)
	}
}

// ==================== toCertificationDraftProto ====================

func TestToCertificationDraftProto_Nil(t *testing.T) {
	if got := toCertificationDraftProto(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestToCertificationDraftProto_Fields(t *testing.T) {
	app := &dto.CertificationApplication{
		ID:          10,
		AccountID:   200,
		Status:      "draft",
		CurrentStep: 2,
		TotalSteps:  8,
	}
	got := toCertificationDraftProto(app)
	if got.Id != "10" {
		t.Errorf("Id = %s, want 10", got.Id)
	}
	if got.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", got.CurrentStep)
	}
	if got.Status != "draft" {
		t.Errorf("Status = %q, want %q", got.Status, "draft")
	}
}

// ==================== formatTime ====================

func TestFormatTime_Zero(t *testing.T) {
	if got := formatTime(time.Time{}); got != "" {
		t.Errorf("formatTime(zero) = %q, want empty string", got)
	}
}

func TestFormatTime_Normal(t *testing.T) {
	ts := time.Date(2026, 2, 18, 10, 30, 0, 0, time.UTC)
	got := formatTime(ts)
	want := "2026-02-18T10:30:00Z"
	if got != want {
		t.Errorf("formatTime = %q, want %q", got, want)
	}
}

// TestToPayuniStatusProto_StatusMappingBaseline proves every code in issue
// #2806's status-mapping acceptance baseline (MerStatus/MemberStatus/channel
// codes, empty Reason, and an unrecognized code) survives toPayuniStatusProto
// verbatim — the mapper must never coerce, drop, or guess at a code it
// doesn't recognize; that's the client's job (per-code label lookup with an
// honest "unknown" fallback), not the backend's.
func TestToPayuniStatusProto_StatusMappingBaseline(t *testing.T) {
	merStatusCodes := []string{"1", "2", "8", "9", "91", "92", "99", "77" /* unknown */}
	memberStatusCodes := []string{"1", "5", "8", "9", "92", "99", "77" /* unknown */}
	channelCodes := []string{"1", "2", "3", "4", "9", "92", "99", "77" /* unknown */}

	profile := &dto.SellerProfile{AccountID: 2600000001}

	for _, mer := range merStatusCodes {
		t.Run("MerStatus_"+mer, func(t *testing.T) {
			ps := &dto.PayuniStatus{MerStatus: mer}
			got := toPayuniStatusProto(profile, ps)
			if got.MerStatus != mer {
				t.Errorf("MerStatus = %q, want %q (verbatim passthrough)", got.MerStatus, mer)
			}
		})
	}
	for _, mem := range memberStatusCodes {
		t.Run("MemberStatus_"+mem, func(t *testing.T) {
			ps := &dto.PayuniStatus{MemberStatus: mem}
			got := toPayuniStatusProto(profile, ps)
			if got.MemberStatus != mem {
				t.Errorf("MemberStatus = %q, want %q (verbatim passthrough)", got.MemberStatus, mem)
			}
		})
	}
	for _, ch := range channelCodes {
		t.Run("CreditStatus_"+ch, func(t *testing.T) {
			ps := &dto.PayuniStatus{CreditStatus: ch}
			got := toPayuniStatusProto(profile, ps)
			if got.CreditStatus != ch {
				t.Errorf("CreditStatus = %q, want %q (verbatim passthrough)", got.CreditStatus, ch)
			}
		})
		t.Run("ApplePayStatus_"+ch, func(t *testing.T) {
			ps := &dto.PayuniStatus{ApplePayStatus: ch}
			got := toPayuniStatusProto(profile, ps)
			if got.ApplepayStatus != ch {
				t.Errorf("ApplepayStatus = %q, want %q (verbatim passthrough)", got.ApplepayStatus, ch)
			}
		})
	}

	t.Run("EmptyReason", func(t *testing.T) {
		ps := &dto.PayuniStatus{Reason: ""}
		got := toPayuniStatusProto(profile, ps)
		if got.Reason != "" {
			t.Errorf("Reason = %q, want empty string (must not fabricate a reason)", got.Reason)
		}
	})
}
