package kyc

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// draftDiff 是一次草稿寫入真的改掉了什麼，由寫入路徑（applyDraft）回報，是「值有沒有變」的唯一來源：退回問題的「已處理」判定
// （resolveIssues）用它。比的是明文與正規化後的值（blind index 隨金鑰版本變，不能拿來比）。文件類欄位由證件的合併（mergeDocuments）判斷，不在這裡。
type draftDiff struct {
	// fields 是值有變的申請欄位（KYCFieldNames 的路徑，文件類除外）。
	fields map[string]bool
}

type fieldDiff func(a, b store.SellerKycApplication) bool

func plain(get func(store.SellerKycApplication) *string) fieldDiff {
	return func(a, b store.SellerKycApplication) bool { return model.Deref(get(a)) != model.Deref(get(b)) }
}

func date(get func(store.SellerKycApplication) *time.Time) fieldDiff {
	return func(a, b store.SellerKycApplication) bool {
		x, y := get(a), get(b)
		if x == nil || y == nil {
			return x != y
		}
		return !x.Equal(*y)
	}
}

// fieldDiffs 是寫入路徑能回報的欄位：每個非文件類的 KYCField 一項（TestFieldDiffsCoverEveryNonDocumentField 守著，新增欄位沒接上就紅）。
// 統編直接比明文（Stripe 計畫 D42）。
var fieldDiffs = map[string]fieldDiff{
	"product_type": plain(func(r store.SellerKycApplication) *string { return r.ProductType }),
	"website_url":  plain(func(r store.SellerKycApplication) *string { return r.WebsiteUrl }),

	"business.company_name":          plain(func(r store.SellerKycApplication) *string { return r.CompanyName }),
	"business.tax_id":                plain(func(r store.SellerKycApplication) *string { return r.TaxID }),
	"business.company_register_date": date(func(r store.SellerKycApplication) *time.Time { return r.CompanyRegisterDate }),
	"business.company_address": func(a, b store.SellerKycApplication) bool {
		return model.Deref(a.CompanyTownCode) != model.Deref(b.CompanyTownCode) ||
			model.Deref(a.CompanyAddressDetail) != model.Deref(b.CompanyAddressDetail)
	},
	"business.company_phone": plain(func(r store.SellerKycApplication) *string { return r.CompanyPhone }),
}

// diffRows 比對寫入前後的申請列。
func diffRows(base, next store.SellerKycApplication) draftDiff {
	d := draftDiff{fields: map[string]bool{}}
	for field, differs := range fieldDiffs {
		d.fields[field] = differs(base, next)
	}
	return d
}
