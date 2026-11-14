package kyc

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// setText 套用一個文字欄位：nil 保留，空白清成沒有值，超過長度拒絕。
func setText(dst **string, v *string, limit int, field, msg string) error {
	if v == nil {
		return nil
	}
	t := trimtext.Trim(*v)
	if t == "" {
		*dst = nil
		return nil
	}
	if utf8.RuneCountInString(t) > limit {
		return apperr.InvalidInputOn([]string{field}, "%s", msg)
	}
	*dst = &t
	return nil
}

// setNormalized 套用一個要正規化並驗證的欄位：nil 保留，空白清成沒有值，格式不合拒絕。
func setNormalized(dst **string, v *string, field, msg string, normalize func(string) (string, bool)) error {
	if v == nil {
		return nil
	}
	if strings.TrimSpace(*v) == "" {
		*dst = nil
		return nil
	}
	n, ok := normalize(*v)
	if !ok {
		return apperr.InvalidInputOn([]string{field}, "%s", msg)
	}
	*dst = &n
	return nil
}

// setDate 套用一個日期欄位（YYYY-MM-DD）：解析不了、不存在的日期、早於 1900 年或晚於今天都拒絕，整筆不寫入，
// 不會沿用舊值或留空（盤點 D12 的回歸）。
func setDate(dst **time.Time, v *string, now time.Time, field, msg string) error {
	if v == nil {
		return nil
	}
	if strings.TrimSpace(*v) == "" {
		*dst = nil
		return nil
	}
	d, ok := model.ParseDate(*v, now)
	if !ok {
		return apperr.InvalidInputOn([]string{field}, "%s", msg)
	}
	*dst = &d
	return nil
}

func clearCompany(r *store.SellerKycApplication) {
	r.CompanyName, r.TaxID = nil, nil
	r.CompanyRegisterDate, r.CompanyTownCode, r.CompanyAddressDetail, r.CompanyPhone = nil, nil, nil, nil
}

// applyDraft 是寫入路徑：合併並驗證（mergeDraft），再回報這次真的改了哪些欄位（draftDiff，create 時沒有「改了什麼」，是空的）。
func applyDraft(base store.SellerKycApplication, in model.KYCDraftInput, create bool, now time.Time) (store.SellerKycApplication, draftDiff, error) {
	next, err := mergeDraft(base, in, create, now)
	if err != nil || create {
		return next, draftDiff{}, err
	}
	return next, diffRows(base, next), nil
}

// mergeDraft 把部分更新合併到 base 並驗證每一個欄位，回傳要寫入的列；任何一欄不合法就整筆不寫入。create 為 true 時 base 是空白的新列，
// 必須給 SellerType。
func mergeDraft(base store.SellerKycApplication, in model.KYCDraftInput, create bool, now time.Time) (store.SellerKycApplication, error) {
	next := base
	if create && in.SellerType == nil {
		return next, apperr.InvalidInputOn([]string{"SellerType"}, "建立申請時必須指定賣家型態")
	}
	if in.SellerType != nil {
		switch *in.SellerType {
		case model.KYCIndividual:
			if next.SellerType == string(model.KYCCompany) {
				clearCompany(&next)
			}
		case model.KYCCompany:
		default:
			return next, apperr.InvalidInputOn([]string{"SellerType"}, "賣家型態不合法")
		}
		next.SellerType = string(*in.SellerType)
	}
	individual := next.SellerType == string(model.KYCIndividual)
	if individual && (in.CompanyName != nil || in.TaxID != nil || in.CompanyRegisterDate != nil ||
		in.CompanyTownCode != nil || in.CompanyAddressDetail != nil || in.CompanyPhone != nil) {
		return next, apperr.InvalidInputOn([]string{"SellerType"}, "個人賣家不填公司資料")
	}
	for _, step := range []func() error{
		func() error {
			return setText(&next.ProductType, in.ProductType, 100, "ProductType", "產品類型不可超過 100 字")
		},
		func() error {
			return setNormalized(&next.WebsiteUrl, in.WebsiteURL, "WebsiteURL", "網站網址格式不正確", model.NormalizeWebsiteURL)
		},
		func() error {
			return setText(&next.CompanyName, in.CompanyName, 200, "CompanyName", "公司名稱不可超過 200 字")
		},
		func() error {
			return setNormalized(&next.TaxID, in.TaxID, "TaxID", "統一編號格式不符，需為 8 碼數字", model.NormalizeTaxID)
		},
		func() error {
			return setDate(&next.CompanyRegisterDate, in.CompanyRegisterDate, now, "CompanyRegisterDate", "公司登記日期不是有效的日期（YYYY-MM-DD，不可晚於今天）")
		},
		func() error {
			return setNormalized(&next.CompanyTownCode, in.CompanyTownCode, "CompanyTownCode", "鄉鎮市區代碼不合法",
				func(s string) (string, bool) { s = strings.TrimSpace(s); return s, model.ValidCompanyTownCode(s) })
		},
		func() error {
			return setText(&next.CompanyAddressDetail, in.CompanyAddressDetail, 255, "CompanyAddressDetail", "公司地址不可超過 255 字")
		},
		func() error {
			return setNormalized(&next.CompanyPhone, in.CompanyPhone, "CompanyPhone", "公司電話格式不符，需為市話", model.NormalizeLandline)
		},
	} {
		if err := step(); err != nil {
			return next, err
		}
	}
	if next.CompanyAddressDetail != nil && next.CompanyTownCode == nil {
		return next, apperr.InvalidInputOn([]string{"CompanyAddressDetail"}, "填寫公司地址前要先選鄉鎮市區")
	}
	return next, nil
}
