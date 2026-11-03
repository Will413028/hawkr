package kyc

import (
	"bytes"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// draftDiff 是一次草稿寫入真的改掉了什麼，由寫入路徑（applyDraft）回報，是「值有沒有變」的唯一來源：退回問題的「已處理」判定
// （resolveIssues）與「PayUni 已收下之後不能改身分欄位」（checkIdentityUnchanged）都用它。比的是明文與正規化後的值，個資密文
// 每次加密都不同、blind index 隨金鑰版本與銀行代碼變，都不能拿來比。文件類欄位由證件的合併（mergeDocuments）判斷，不在這裡。
type draftDiff struct {
	// fields 是值有變的申請欄位（KYCFieldNames 的路徑，文件類除外）。
	fields map[string]bool
	// sellerType 是賣家型態有變。
	sellerType bool
}

// identity 回報這次寫入有沒有改到身分欄位。
func (d draftDiff) identity() bool {
	if d.sellerType {
		return true
	}
	for _, f := range model.LockedIdentityFields() {
		if d.fields[f] {
			return true
		}
	}
	return false
}

type fieldDiff func(p protector, a, b store.SellerKycApplication) (bool, error)

func plain(get func(store.SellerKycApplication) *string) fieldDiff {
	return func(_ protector, a, b store.SellerKycApplication) (bool, error) {
		return model.Deref(get(a)) != model.Deref(get(b)), nil
	}
}

func date(get func(store.SellerKycApplication) *time.Time) fieldDiff {
	return func(_ protector, a, b store.SellerKycApplication) (bool, error) {
		x, y := get(a), get(b)
		if x == nil || y == nil {
			return x != y, nil
		}
		return !x.Equal(*y), nil
	}
}

// secret 比一個加密欄位的明文：密文相同就沒變，一邊是空就變了，兩邊都有才解密比（只有重填那個欄位時才會走到）。
func secret(column string, get func(store.SellerKycApplication) []byte) fieldDiff {
	return func(p protector, a, b store.SellerKycApplication) (bool, error) {
		x, y := get(a), get(b)
		if bytes.Equal(x, y) {
			return false, nil
		}
		if x == nil || y == nil {
			return true, nil
		}
		px, err := p.open(a.ApplicationID, column, x)
		if err != nil {
			return false, err
		}
		py, err := p.open(b.ApplicationID, column, y)
		if err != nil {
			return false, err
		}
		return model.Deref(px) != model.Deref(py), nil
	}
}

// fieldDiffs 是寫入路徑能回報的欄位：每個非文件類的 KYCField 一項（TestFieldDiffsCoverEveryNonDocumentField 守著，新增欄位沒接上就紅）。
var fieldDiffs = map[string]fieldDiff{
	"real_name":              plain(func(r store.SellerKycApplication) *string { return r.RealName }),
	"birth_date":             date(func(r store.SellerKycApplication) *time.Time { return r.BirthDate }),
	"id_card_issue_date":     date(func(r store.SellerKycApplication) *time.Time { return r.IDCardIssueDate }),
	"id_card_issue_type":     plain(func(r store.SellerKycApplication) *string { return r.IDCardIssueType }),
	"id_card_issue_location": plain(func(r store.SellerKycApplication) *string { return r.IDCardIssueLocation }),
	"english_name":           plain(func(r store.SellerKycApplication) *string { return r.EnglishName }),
	"bank_name":              plain(func(r store.SellerKycApplication) *string { return r.BankName }),
	"bank_code":              plain(func(r store.SellerKycApplication) *string { return r.BankCode }),
	"bank_account":           secret(colBankAccount, func(r store.SellerKycApplication) []byte { return r.BankAccountEnc }),
	"product_type":           plain(func(r store.SellerKycApplication) *string { return r.ProductType }),
	"website_url":            plain(func(r store.SellerKycApplication) *string { return r.WebsiteUrl }),

	"individual.id_card_no":          secret(colIDCardNo, func(r store.SellerKycApplication) []byte { return r.IDCardNoEnc }),
	"business.company_name":          plain(func(r store.SellerKycApplication) *string { return r.CompanyName }),
	"business.tax_id":                plain(func(r store.SellerKycApplication) *string { return r.TaxID }),
	"business.company_register_date": date(func(r store.SellerKycApplication) *time.Time { return r.CompanyRegisterDate }),
	"business.company_address": func(_ protector, a, b store.SellerKycApplication) (bool, error) {
		return model.Deref(a.CompanyTownCode) != model.Deref(b.CompanyTownCode) ||
			model.Deref(a.CompanyAddressDetail) != model.Deref(b.CompanyAddressDetail), nil
	},
	"business.company_phone":  plain(func(r store.SellerKycApplication) *string { return r.CompanyPhone }),
	"business.rep_id_card_no": secret(colRepIDCardNo, func(r store.SellerKycApplication) []byte { return r.RepIDCardNoEnc }),
}

// diffRows 比對寫入前後的申請列。
func (p protector) diffRows(base, next store.SellerKycApplication) (draftDiff, error) {
	d := draftDiff{fields: map[string]bool{}, sellerType: base.SellerType != next.SellerType}
	for field, differs := range fieldDiffs {
		changed, err := differs(p, base, next)
		if err != nil {
			return draftDiff{}, err
		}
		d.fields[field] = changed
	}
	return d, nil
}
