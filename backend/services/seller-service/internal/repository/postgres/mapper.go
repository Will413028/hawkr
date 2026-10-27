package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres/gen"
)

func encryptIDCardNo(enc *crypto.Cipher, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return enc.EncryptToBase64(plaintext)
}

func decryptIDCardNo(enc *crypto.Cipher, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	return enc.DecryptFromBase64(stored)
}

func encryptBankAccount(enc *crypto.Cipher, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return enc.EncryptToBase64(plaintext)
}

func decryptBankAccount(enc *crypto.Cipher, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plaintext, err := enc.DecryptFromBase64(stored)
	if err != nil {
		return "", errors.New("bank_account did not decrypt (unencrypted row, corrupted value, or wrong encryption key)")
	}
	return plaintext, nil
}

var sensitiveStepDataKeys = map[string]struct{}{
	"idNumber":     {},
	"id_card_no":   {},
	"idCardNo":     {},
	"bankAccount":  {},
	"bank_account": {},
}

func encryptStepData(enc *crypto.Cipher, raw json.RawMessage) (json.RawMessage, error) {
	return mapSensitiveStepDataValues(raw, enc.EncryptToBase64)
}

func decryptStepData(enc *crypto.Cipher, raw json.RawMessage) (json.RawMessage, error) {
	return mapSensitiveStepDataValues(raw, func(stored string) (string, error) {
		return decryptStepDataValue(enc, stored)
	})
}

func decryptStepDataValue(enc *crypto.Cipher, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plaintext, err := enc.DecryptFromBase64(stored)
	if err != nil {
		return "", errors.New("step_data holds a value that did not decrypt (unencrypted value, corrupted value, or wrong encryption key)")
	}
	return plaintext, nil
}

func mapSensitiveStepDataValues(raw json.RawMessage, fn func(string) (string, error)) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return raw, nil
	}

	changed := false
	for key := range sensitiveStepDataKeys {
		encoded, present := fields[key]
		if !present {
			continue
		}
		var value string
		if err := json.Unmarshal(encoded, &value); err != nil {
			continue
		}
		if value == "" {
			continue
		}
		next, err := fn(value)
		if err != nil {
			return nil, fmt.Errorf("step_data key %q: %w", key, err)
		}
		if next == value {
			continue
		}
		replacement, err := json.Marshal(next)
		if err != nil {
			return nil, fmt.Errorf("step_data key %q: re-encode: %w", key, err)
		}
		fields[key] = replacement
		changed = true
	}
	if !changed {
		return raw, nil
	}

	out, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("re-encode step_data: %w", err)
	}
	return out, nil
}

func toDomainSeller(row gen.Seller) (*domain.Seller, error) {
	sellerType, err := parseStringEnum(stringValue(row.SellerType), domain.SellerTypeString)
	if err != nil {
		return nil, fmt.Errorf("invalid seller_type: %w", err)
	}
	m := &domain.Seller{
		AccountID:               row.AccountID,
		StoreID:                 row.StoreID,
		StoreCode:               row.StoreCode,
		StoreName:               row.StoreName,
		Description:             stringValue(row.Description),
		Logo:                    stringValue(row.Logo),
		CoverKey:                stringValue(row.CoverKey),
		SeoTitle:                stringValue(row.SeoTitle),
		SeoDescription:          stringValue(row.SeoDescription),
		WebsiteURL:              stringValue(row.WebsiteUrl),
		IsProcessedFood:         row.IsProcessedFood,
		FoodBusinessRegNo:       row.FoodBusinessRegNo,
		Email:                   stringValue(row.Email),
		ContactPhone:            stringValue(row.ContactPhone),
		City:                    stringValue(row.City),
		District:                stringValue(row.District),
		AddressDetail:           stringValue(row.AddressDetail),
		PostalCode:              stringValue(row.PostalCode),
		Birthday:                dateToString(row.Birthday),
		Gender:                  stringValue(row.Gender),
		Prize:                   stringValue(row.Prize),
		Rating:                  float64Value(row.Rating),
		SellerType:              sellerType,
		OnboardingCompleted:     boolValue(row.OnboardingCompleted),
		CertifiedAt:             row.CertifiedAt,
		CreatedAt:               timeValue(row.CreatedAt),
		UpdatedAt:               timeValue(row.UpdatedAt),
		DeletedAt:               row.DeletedAt,
		Status:                  domain.SellerStatus(row.Status),
		PIIClearedAt:            row.PiiClearedAt,
		PayuniAccountUnlinkedAt: row.PayuniAccountUnlinkedAt,
		Version:                 row.Version,
	}
	if row.CurrentCertificationID != nil {
		m.CurrentCertificationID = row.CurrentCertificationID
	}
	return m, nil
}

func toDomainCertificationApplication(row gen.CertificationApplication, enc *crypto.Cipher) (*domain.CertificationApplication, error) {
	status, err := domain.CertificationStatusString(row.Status)
	if err != nil {
		return nil, fmt.Errorf("invalid certification status %q: %w", row.Status, err)
	}
	idCardNo, err := decryptIDCardNo(enc, stringValue(row.IDCardNo))
	if err != nil {
		return nil, fmt.Errorf("decrypt id_card_no (application %d): %w", row.ID, err)
	}
	bankAccount, err := decryptBankAccount(enc, stringValue(row.BankAccount))
	if err != nil {
		return nil, fmt.Errorf("decrypt bank_account (application %d): %w", row.ID, err)
	}
	stepData, err := decryptStepData(enc, pgxutil.JSONRawMessage(row.StepData))
	if err != nil {
		return nil, fmt.Errorf("decrypt step_data (application %d): %w", row.ID, err)
	}
	app := &domain.CertificationApplication{
		ID:                   row.ID,
		AccountID:            row.AccountID,
		ApplicationNo:        row.ApplicationNo,
		Status:               status,
		SellerType:           row.SellerType,
		RealName:             stringValue(row.RealName),
		IDCardNo:             idCardNo,
		BirthDate:            pgxutil.DateValue(row.BirthDate),
		IDCardIssueDate:      pgxutil.DateValue(row.IDCardIssueDate),
		IDCardIssueType:      stringValue(row.IDCardIssueType),
		IDCardIssueLocation:  stringValue(row.IDCardIssueLocation),
		EnglishName:          stringValue(row.EnglishName),
		IDCardFrontAssetID:   stringValue(row.IDCardFrontAssetID),
		IDCardBackAssetID:    stringValue(row.IDCardBackAssetID),
		BankBookAssetID:      stringValue(row.BankBookAssetID),
		DocumentAssetIDs:     decodeStringSlice(row.DocumentAssetIds),
		BankName:             stringValue(row.BankName),
		BankCode:             stringValue(row.BankCode),
		BankAccount:          bankAccount,
		CompanyType:          stringValue(row.CompanyType),
		CompanyName:          stringValue(row.CompanyName),
		TaxID:                stringValue(row.TaxID),
		CompanyRegisterDate:  pgxutil.DateValue(row.CompanyRegisterDate),
		CompanyCity:          stringValue(row.CompanyCity),
		CompanyDistrict:      stringValue(row.CompanyDistrict),
		CompanyAddressDetail: stringValue(row.CompanyAddressDetail),
		CompanyEmail:         stringValue(row.CompanyEmail),
		CompanyPhone:         stringValue(row.CompanyPhone),
		ProductType:          stringValue(row.ProductType),
		IsProcessedFood:      boolValue(row.IsProcessedFood),
		FoodBusinessRegNo:    row.FoodBusinessRegNo,
		PayuniWebsiteURL:     stringValue(row.PayuniWebsiteUrl),
		CancelReason:         stringValue(row.CancelReason),
		CurrentStep:          int32Value(row.CurrentStep),
		TotalSteps:           int32Value(row.TotalSteps),
		StepData:             stepData,
		Version:              int(row.Version),
		SubmittedAt:          row.SubmittedAt,
		ForwardedAt:          row.ForwardedAt,
		RejectedAt:           row.RejectedAt,
		CreatedAt:            timeValue(row.CreatedAt),
		UpdatedAt:            timeValue(row.UpdatedAt),
	}
	app.InitFSM()
	return app, nil
}

func toDomainCertificationReview(row gen.CertificationReview) (*domain.CertificationReview, error) {
	action, err := domain.ReviewActionString(row.Action)
	if err != nil {
		return nil, fmt.Errorf("invalid review action %q: %w", row.Action, err)
	}
	result, err := parseStringEnum(stringValue(row.Result), domain.ReviewResultString)
	if err != nil {
		return nil, fmt.Errorf("invalid review result %q: %w", stringValue(row.Result), err)
	}
	review := &domain.CertificationReview{
		ID:              row.ID,
		ApplicationID:   row.ApplicationID,
		ReviewerName:    stringValue(row.ReviewerName),
		Action:          action,
		Result:          result,
		Comment:         stringValue(row.Comment),
		RejectionReason: stringValue(row.RejectionReason),
		RequiredFields:  pgxutil.JSONRawMessage(row.RequiredFields),
		CreatedAt:       timeValue(row.CreatedAt),
	}
	if row.ReviewerID != nil {
		review.ReviewerID = row.ReviewerID
	}
	return review, nil
}

func toDomainPayuniRegistration(row gen.PayuniRegistration) (*domain.PayuniRegistration, error) {
	status, err := parseStringEnum(stringValue(row.RegistrationStatus), domain.PayuniStatusString)
	if err != nil {
		return nil, fmt.Errorf("invalid payuni status: %w", err)
	}
	return &domain.PayuniRegistration{
		ID:                     row.ID,
		AccountID:              row.AccountID,
		StoreID:                row.StoreID,
		ApplicationID:          row.ApplicationID,
		PayuniMerID:            stringValue(row.PayuniMerID),
		Status:                 status,
		PaymentEnabled:         boolValue(row.PaymentEnabled),
		MerStatus:              stringValue(row.MerStatus),
		MemberStatus:           stringValue(row.MemberStatus),
		CreditStatus:           stringValue(row.CreditStatus),
		ApplePayStatus:         stringValue(row.ApplepayStatus),
		GooglePayStatus:        stringValue(row.GooglepayStatus),
		ATMStatus:              stringValue(row.AtmStatus),
		Reason:                 stringValue(row.Reason),
		ErrorMessage:           stringValue(row.ErrorMessage),
		ErrorCode:              stringValue(row.ErrorCode),
		RawRequest:             pgxutil.JSONRawMessage(row.RawRequest),
		RawResponse:            pgxutil.JSONRawMessage(row.RawResponse),
		RegisteredAt:           row.RegisteredAt,
		ApprovedAt:             row.ApprovedAt,
		LastSyncedAt:           row.LastSyncedAt,
		LastClaimedAt:          row.LastClaimedAt,
		UpdatedAt:              timeValue(row.UpdatedAt),
		ContactName:            stringValue(row.ContactName),
		ContactMobile:          stringValue(row.ContactMobile),
		ContactTel:             stringValue(row.ContactTel),
		MerEmail:               stringValue(row.MerEmail),
		MerAddr:                stringValue(row.MerAddr),
		MerAddrPost:            stringValue(row.MerAddrPost),
		MerTel:                 stringValue(row.MerTel),
		MerchantProfileVersion: row.MerchantProfileVersion,
	}, nil
}

func toCreateApplicationParams(app *domain.CertificationApplication, enc *crypto.Cipher) (gen.CreateApplicationParams, error) {
	idCardNo, err := encryptIDCardNo(enc, app.IDCardNo)
	if err != nil {
		return gen.CreateApplicationParams{}, fmt.Errorf("encrypt id_card_no: %w", err)
	}
	bankAccount, err := encryptBankAccount(enc, app.BankAccount)
	if err != nil {
		return gen.CreateApplicationParams{}, fmt.Errorf("encrypt bank_account: %w", err)
	}
	stepData, err := encryptStepData(enc, app.StepData)
	if err != nil {
		return gen.CreateApplicationParams{}, fmt.Errorf("encrypt step_data: %w", err)
	}
	return gen.CreateApplicationParams{
		ID:                   app.ID,
		AccountID:            app.AccountID,
		ApplicationNo:        app.ApplicationNo,
		Status:               app.Status.String(),
		SellerType:           app.SellerType,
		RealName:             nullableText(app.RealName),
		IDCardNo:             nullableText(idCardNo),
		BirthDate:            pgxutil.DateParam(app.BirthDate),
		IDCardIssueDate:      pgxutil.DateParam(app.IDCardIssueDate),
		IDCardIssueType:      nullableText(app.IDCardIssueType),
		IDCardIssueLocation:  nullableText(app.IDCardIssueLocation),
		EnglishName:          nullableText(app.EnglishName),
		IDCardFrontAssetID:   nullableText(app.IDCardFrontAssetID),
		IDCardBackAssetID:    nullableText(app.IDCardBackAssetID),
		BankBookAssetID:      nullableText(app.BankBookAssetID),
		DocumentAssetIds:     encodeStringSlice(app.DocumentAssetIDs),
		BankName:             nullableText(app.BankName),
		BankCode:             nullableText(app.BankCode),
		BankAccount:          nullableText(bankAccount),
		CompanyType:          nullableText(app.CompanyType),
		CompanyName:          nullableText(app.CompanyName),
		TaxID:                nullableText(app.TaxID),
		CompanyRegisterDate:  pgxutil.DateParam(app.CompanyRegisterDate),
		CompanyCity:          nullableText(app.CompanyCity),
		CompanyDistrict:      nullableText(app.CompanyDistrict),
		CompanyAddressDetail: nullableText(app.CompanyAddressDetail),
		CompanyEmail:         nullableText(app.CompanyEmail),
		CompanyPhone:         nullableText(app.CompanyPhone),
		ProductType:          nullableText(app.ProductType),
		IsProcessedFood:      boolPointer(app.IsProcessedFood),
		FoodBusinessRegNo:    app.FoodBusinessRegNo,
		CurrentStep:          int32PointerFromInt(app.CurrentStep),
		TotalSteps:           int32PointerFromInt(app.TotalSteps),
		StepData:             pgxutil.JSONBytes(stepData),
		SubmittedAt:          nullableTimePointer(app.SubmittedAt),
		RejectedAt:           nullableTimePointer(app.RejectedAt),
		CreatedAt:            nullableTime(app.CreatedAt),
		UpdatedAt:            nullableTime(app.UpdatedAt),
	}, nil
}

func toUpdateApplicationParams(app *domain.CertificationApplication, enc *crypto.Cipher) (gen.UpdateApplicationParams, error) {
	idCardNo, err := encryptIDCardNo(enc, app.IDCardNo)
	if err != nil {
		return gen.UpdateApplicationParams{}, fmt.Errorf("encrypt id_card_no: %w", err)
	}
	bankAccount, err := encryptBankAccount(enc, app.BankAccount)
	if err != nil {
		return gen.UpdateApplicationParams{}, fmt.Errorf("encrypt bank_account: %w", err)
	}
	stepData, err := encryptStepData(enc, app.StepData)
	if err != nil {
		return gen.UpdateApplicationParams{}, fmt.Errorf("encrypt step_data: %w", err)
	}
	return gen.UpdateApplicationParams{
		ID:                   app.ID,
		Status:               app.Status.String(),
		SellerType:           app.SellerType,
		RealName:             nullableText(app.RealName),
		IDCardNo:             nullableText(idCardNo),
		BirthDate:            pgxutil.DateParam(app.BirthDate),
		IDCardIssueDate:      pgxutil.DateParam(app.IDCardIssueDate),
		IDCardIssueType:      nullableText(app.IDCardIssueType),
		IDCardIssueLocation:  nullableText(app.IDCardIssueLocation),
		EnglishName:          nullableText(app.EnglishName),
		IDCardFrontAssetID:   nullableText(app.IDCardFrontAssetID),
		IDCardBackAssetID:    nullableText(app.IDCardBackAssetID),
		BankBookAssetID:      nullableText(app.BankBookAssetID),
		DocumentAssetIds:     encodeStringSlice(app.DocumentAssetIDs),
		BankName:             nullableText(app.BankName),
		BankCode:             nullableText(app.BankCode),
		BankAccount:          nullableText(bankAccount),
		CompanyType:          nullableText(app.CompanyType),
		CompanyName:          nullableText(app.CompanyName),
		TaxID:                nullableText(app.TaxID),
		CompanyRegisterDate:  pgxutil.DateParam(app.CompanyRegisterDate),
		CompanyCity:          nullableText(app.CompanyCity),
		CompanyDistrict:      nullableText(app.CompanyDistrict),
		CompanyAddressDetail: nullableText(app.CompanyAddressDetail),
		CompanyEmail:         nullableText(app.CompanyEmail),
		CompanyPhone:         nullableText(app.CompanyPhone),
		ProductType:          nullableText(app.ProductType),
		IsProcessedFood:      boolPointer(app.IsProcessedFood),
		FoodBusinessRegNo:    app.FoodBusinessRegNo,
		CurrentStep:          int32PointerFromInt(app.CurrentStep),
		TotalSteps:           int32PointerFromInt(app.TotalSteps),
		StepData:             pgxutil.JSONBytes(stepData),
		SubmittedAt:          nullableTimePointer(app.SubmittedAt),
		ForwardedAt:          nullableTimePointer(app.ForwardedAt),
		RejectedAt:           nullableTimePointer(app.RejectedAt),
		UpdatedAt:            nullableTime(app.UpdatedAt),
		PayuniWebsiteUrl:     nullableText(app.PayuniWebsiteURL),
		CancelReason:         nullableText(app.CancelReason),
		Version:              int32(app.Version),
	}, nil
}

func dateToString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

func stringToDate(s string) pgtype.Date {
	var zeroDate pgtype.Date
	if s == "" {
		return zeroDate
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return zeroDate
	}
	return pgxutil.DateParam(&t)
}

func decodeStringSlice(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func encodeStringSlice(s []string) []byte {
	if len(s) == 0 {
		return nil
	}
	data, _ := json.Marshal(s)
	return data
}
