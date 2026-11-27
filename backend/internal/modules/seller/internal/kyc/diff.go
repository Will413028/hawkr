package kyc

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

type draftDiff struct {
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

func diffRows(base, next store.SellerKycApplication) draftDiff {
	d := draftDiff{fields: map[string]bool{}}
	for field, differs := range fieldDiffs {
		d.fields[field] = differs(base, next)
	}
	return d
}
