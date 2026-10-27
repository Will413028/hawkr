package kyc

import (
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

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

func clearIndividual(r *store.SellerKycApplication) {
	r.IDCardNoEnc, r.IDCardHash, r.IDCardHashKey = nil, nil, nil
}

func clearCompany(r *store.SellerKycApplication) {
	r.CompanyName, r.TaxID, r.TaxIDHash, r.TaxIDHashKey = nil, nil, nil, nil
	r.CompanyRegisterDate, r.CompanyTownCode, r.CompanyAddressDetail, r.CompanyPhone = nil, nil, nil, nil
	r.RepIDCardNoEnc, r.RepIDCardHash, r.RepIDCardHashKey = nil, nil, nil
}

// applyDraft 是寫入路徑：合併並驗證（mergeDraft），再回報這次真的改了哪些欄位（draftDiff，create 時沒有「改了什麼」，是空的）。
func (p protector) applyDraft(base store.SellerKycApplication, in model.KYCDraftInput, create bool, now time.Time) (store.SellerKycApplication, draftDiff, error) {
	next, err := p.mergeDraft(base, in, create, now)
	if err != nil || create {
		return next, draftDiff{}, err
	}
	diff, err := p.diffRows(base, next)
	return next, diff, err
}

// mergeDraft 把部分更新合併到 base 並驗證每一個欄位，回傳要寫入的列；任何一欄不合法就整筆不寫入。個資在這裡加密、
// 計算 blind index（沒有動到的欄位保留原本的密文與 hash，不解密）。create 為 true 時 base 是空白的新列，必須給 SellerType。
func (p protector) mergeDraft(base store.SellerKycApplication, in model.KYCDraftInput, create bool, now time.Time) (store.SellerKycApplication, error) {
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
			if next.SellerType == string(model.KYCIndividual) {
				clearIndividual(&next)
			}
		default:
			return next, apperr.InvalidInputOn([]string{"SellerType"}, "賣家型態不合法")
		}
		next.SellerType = string(*in.SellerType)
	}
	individual := next.SellerType == string(model.KYCIndividual)
	if individual && (in.CompanyName != nil || in.TaxID != nil || in.CompanyRegisterDate != nil ||
		in.CompanyTownCode != nil || in.CompanyAddressDetail != nil || in.CompanyPhone != nil || in.RepIDCardNo != nil) {
		return next, apperr.InvalidInputOn([]string{"SellerType"}, "個人賣家不填公司資料")
	}
	if !individual && in.IDCardNo != nil {
		return next, apperr.InvalidInputOn([]string{"IDCardNo"}, "企業賣家填的是負責人的身分證號")
	}
	for _, step := range []func() error{
		func() error {
			return setText(&next.RealName, in.RealName, 100, "RealName", "姓名不可超過 100 字")
		},
		func() error {
			return setDate(&next.BirthDate, in.BirthDate, now, "BirthDate", "生日不是有效的日期（YYYY-MM-DD，不可晚於今天）")
		},
		func() error {
			return setDate(&next.IDCardIssueDate, in.IDCardIssueDate, now, "IDCardIssueDate", "發證日期不是有效的日期（YYYY-MM-DD，不可晚於今天）")
		},
		func() error {
			return setText(&next.IDCardIssueType, in.IDCardIssueType, 20, "IDCardIssueType", "領補換類別不可超過 20 字")
		},
		func() error {
			return setText(&next.IDCardIssueLocation, in.IDCardIssueLocation, 100, "IDCardIssueLocation", "發證地點不可超過 100 字")
		},
		func() error {
			return setNormalized(&next.EnglishName, trimmed(in.EnglishName), "EnglishName", "商店英文名稱僅容許英數、半形空白與符號，至多 50 字",
				func(s string) (string, bool) { return s, model.ValidEnglishName(s) })
		},
		func() error {
			return setText(&next.BankName, in.BankName, 100, "BankName", "銀行名稱不可超過 100 字")
		},
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
	if err := p.applyIdentity(&next, in, individual); err != nil {
		return next, err
	}
	return next, nil
}

// trimmed 回傳去掉前後空白的值（nil 仍是 nil）。
func trimmed(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	return &t
}

// applyIdentity 套用識別欄位（身分證號、負責人身分證號、統編、銀行帳號）：加密並計算 blind index。銀行帳號的 blind index
// 輸入帶銀行代碼，所以銀行代碼改變時要以原本的帳號重算（解密一次）；清掉銀行代碼時帳號要一併清掉。
func (p protector) applyIdentity(next *store.SellerKycApplication, in model.KYCDraftInput, individual bool) error {
	id := next.ApplicationID
	if in.IDCardNo != nil && individual {
		if err := p.setID(in.IDCardNo, id, colIDCardNo, "IDCardNo", "身分證字號格式不符", &next.IDCardNoEnc, &next.IDCardHash, &next.IDCardHashKey); err != nil {
			return err
		}
	}
	if in.RepIDCardNo != nil {
		if err := p.setID(in.RepIDCardNo, id, colRepIDCardNo, "RepIDCardNo", "負責人身分證字號格式不符", &next.RepIDCardNoEnc, &next.RepIDCardHash, &next.RepIDCardHashKey); err != nil {
			return err
		}
	}
	if in.TaxID != nil {
		if err := setNormalized(&next.TaxID, in.TaxID, "TaxID", "統一編號格式不符，需為 8 碼數字", model.NormalizeTaxID); err != nil {
			return err
		}
		next.TaxIDHash, next.TaxIDHashKey = nil, nil
		if next.TaxID != nil {
			h := p.mac.Sum(taxInput(*next.TaxID))
			next.TaxIDHash, next.TaxIDHashKey = h.Sum, &h.KeyID
		}
	}
	return p.applyBank(next, in)
}

func (p protector) setID(v *string, applicationID uuid.UUID, column, field, msg string, enc, hash *[]byte, keyID **string) error {
	if strings.TrimSpace(*v) == "" {
		*enc, *hash, *keyID = nil, nil, nil
		return nil
	}
	n, ok := model.NormalizeIDCardNo(*v)
	if !ok {
		return apperr.InvalidInputOn([]string{field}, "%s", msg)
	}
	prot, err := p.protect(applicationID, column, n, idInput(n))
	if err != nil {
		return err
	}
	*enc, *hash, *keyID = prot.Enc, prot.Hash, prot.HashID
	return nil
}

func (p protector) applyBank(next *store.SellerKycApplication, in model.KYCDraftInput) error {
	if in.BankCode != nil {
		code := strings.TrimSpace(*in.BankCode)
		switch {
		case code == "":
			next.BankCode = nil
		case !model.ValidBankCode(code):
			return apperr.InvalidInputOn([]string{"BankCode"}, "銀行代碼需為 3 碼，或 3 碼加 4 碼分行代碼")
		default:
			next.BankCode = &code
		}
	}
	if in.BankAccount != nil {
		if strings.TrimSpace(*in.BankAccount) == "" {
			next.BankAccountEnc, next.BankAccountHash, next.BankAccountHashKey = nil, nil, nil
		} else {
			account, ok := model.NormalizeBankAccount(*in.BankAccount)
			if !ok {
				return apperr.InvalidInputOn([]string{"BankAccount"}, "銀行帳號需為 6 到 20 碼數字")
			}
			if next.BankCode == nil {
				return apperr.InvalidInputOn([]string{"BankCode"}, "設定銀行帳號前要先填銀行代碼")
			}
			prot, err := p.protect(next.ApplicationID, colBankAccount, account, bankInput(*next.BankCode, account))
			if err != nil {
				return err
			}
			next.BankAccountEnc, next.BankAccountHash, next.BankAccountHashKey = prot.Enc, prot.Hash, prot.HashID
			return nil
		}
	}
	if in.BankCode == nil || next.BankAccountEnc == nil {
		return nil
	}
	// 只改了銀行代碼：以原本的帳號重算 blind index（密文不變）。
	if next.BankCode == nil {
		return apperr.InvalidInputOn([]string{"BankCode"}, "清掉銀行代碼時要一併清掉銀行帳號")
	}
	account, err := p.open(next.ApplicationID, colBankAccount, next.BankAccountEnc)
	if err != nil {
		return err
	}
	h := p.mac.Sum(bankInput(*next.BankCode, *account))
	next.BankAccountHash, next.BankAccountHashKey = h.Sum, &h.KeyID
	return nil
}
