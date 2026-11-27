package usecase

import (
	"context"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)


const profileLabelMaxRunes = 30

// Preference 是買家常用發票資料的寫入 usecase。
//
// 它有 TxManager 而 Query 沒有，理由只有一個：③。除此之外本 usecase 不打 provider、
// 不發事件——常用發票資料的變更不影響任何已開立的憑證。
type Preference struct {
	tx    database.TxManager
	prefs domain.BuyerPreferenceRepository
	idem domain.IdempotencyRepository

	barcode MobileBarcodeVerifier

	newID func() int64
	now   func() time.Time
	log   *zap.Logger
}

// MobileBarcodeVerifier 是 /json/barcode 的窄介面（唯讀，不產生任何發票）。
//
// 🔴 用**平台**憑證而不是賣家的：買家建立常用發票資料時沒有賣家上下文，而載具驗證
// 與統編無關（2026-09-08 實測：同一組憑證查任意載具都拿得到可分辨的結果）。
type MobileBarcodeVerifier interface {
	VerifyMobileBarcode(ctx context.Context, barcode string) error
}

// WithBarcodeVerifier 掛上入口層的載具驗證（#3256）。
//
// ⚠️ 與上面 idem 那段的紅字**不衝突**：idem 是 nil 就等於靜默失去冪等（看不見的正確性
// 退化），所以必須列進簽章；barcode 是 nil 只少一道即時回饋，開票前那層仍會擋——
// 退化是可見的（買家付款後才知道），不是靜默的。
func (u *Preference) WithBarcodeVerifier(v MobileBarcodeVerifier) *Preference {
	u.barcode = v
	return u
}

func (u *Preference) verifyBarcodeIfNeeded(ctx context.Context, kind codemeta.InvoiceCarrierType, value string) error {
	if u.barcode == nil || kind.Code != codemeta.InvoiceCarrierTypeMobileBarcode.Code || value == "" {
		return nil
	}
	err := u.barcode.VerifyMobileBarcode(ctx, value)
	if err == nil {
		return nil
	}
	var ae *amego.APIError
	if !errors.As(err, &ae) {
		u.logger().Warn("invoice.preference.barcode_verify_unavailable", zap.Error(err))
		return nil
	}
	switch ae.Code {
	case amegoCodeBarcodeMalformed:
		return pkgerrors.NewBusiness(bizcode.InvoiceCarrierInvalid, "手機條碼格式不正確，請確認後重新輸入")
	case amegoCodeBarcodeNotFound:
		return pkgerrors.NewBusiness(bizcode.InvoiceCarrierInvalid, "查無此手機條碼，請確認是否已在財政部平台完成註冊")
	default:
		u.logger().Warn("invoice.preference.barcode_verify_unexpected_code",
			zap.String("code", ae.Code), zap.Error(err))
		return nil
	}
}

// NewPreference 組出寫入 usecase。
func NewPreference(
	tx database.TxManager,
	prefs domain.BuyerPreferenceRepository,
	idem domain.IdempotencyRepository,
	newID func() int64,
	log *zap.Logger,
) *Preference {
	return &Preference{
		tx:    tx,
		prefs: prefs,
		idem:  idem,
		newID: newID,
		now:   func() time.Time { return time.Now().UTC() },
		log:   log,
	}
}

// WithClock 注入時間來源（測試用）。
func (u *Preference) WithClock(now func() time.Time) *Preference {
	if now != nil {
		u.now = now
	}
	return u
}

// CreateProfileInput 是端點 2 的輸入。Value 是明碼（載具顯碼／捐贈碼），只在此入站。
type CreateProfileInput struct {
	BuyerID     int64
	CarrierType string
	Value       string
	BuyerTaxID  string
	BuyerName   string
	Label       string
	SetDefault  bool
	// IdempotencyKey 是呼叫端送的 `Idempotency-Key`（#3015 裁決 A）。
	//
	// 🔴 這一欄在 2026-09-02 之前**根本不存在** —— gateway 強制要求那個 header、
	// 一路傳到 proto，而本層完全沒讀它 ⇒ 同一把鍵送不同 body 會建立第二筆資料。
	// 空字串合法（內部呼叫端不帶鍵），見 ClaimIdempotency。
	IdempotencyKey string
}

// UpdateProfileInput 是端點 3 的輸入。
//
// 它**沒有** CarrierType：載具型別不可變更（§6.1.1「要換類型＝刪除後新增」），
// 型別一律取自被編輯的那一列。proto 的 UpdateBuyerInvoiceProfileRequest 同樣沒有這個
// 欄位 ⇒ 「送不同值 → 400」在本服務沒有可判定的輸入（見交付回報）。
type UpdateProfileInput struct {
	BuyerID    int64
	ProfileID  int64
	Value      string
	BuyerTaxID string
	BuyerName  string
	Label      string
}

// Create 新增一列常用發票資料（端點 2）。
//
// 順序是刻意的：先驗格式（9305／9306）再進交易，交易內才數上限（9303）並寫入。
// 格式錯的請求根本不該開交易。
func (u *Preference) Create(ctx context.Context, in CreateProfileInput) (CheckoutProfile, error) {
	kind, ok := codemeta.InvoiceCarrierTypeFromValue(in.CarrierType)
	if !ok {
		return CheckoutProfile{}, profileBiz(bizcode.InvoiceCarrierInvalid)
	}
	if code := validateProfileFields(kind, in.Value, in.BuyerTaxID, in.BuyerName); code != 0 {
		return CheckoutProfile{}, profileBiz(code)
	}
	if code := validateProfileLabel(in.Label); code != 0 {
		return CheckoutProfile{}, profileBiz(code)
	}
	if err := u.verifyBarcodeIfNeeded(ctx, kind, in.Value); err != nil {
		return CheckoutProfile{}, err
	}

	fingerprint := RequestFingerprint(
		in.CarrierType, in.Value, in.BuyerTaxID, in.BuyerName, in.Label,
		strconv.FormatBool(in.SetDefault),
	)

	var out CheckoutProfile
	var replayRef int64
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		claim, cerr := ClaimIdempotency(ctx, u.idem, domain.ClaimIdempotency{
			Scope:       IdemScopeBuyerProfileCreate,
			ActorID:     in.BuyerID,
			Key:         in.IdempotencyKey,
			RequestHash: fingerprint,
			Now:         u.now(),
		})
		if cerr != nil {
			return cerr
		}
		if !claim.FirstTime {
			replayRef = claim.ResultRef
			return nil
		}
		alive, err := u.prefs.ListByBuyer(ctx, in.BuyerID, buyerProfileCap+1)
		if err != nil {
			return err
		}
		if len(alive) >= buyerProfileCap {
			return profileBiz(bizcode.InvoiceProfileLimitExceeded)
		}

		now := u.now()
		if in.SetDefault {
			if _, err := u.prefs.ClearDefault(ctx, in.BuyerID, now); err != nil {
				return err
			}
		}
		row, inserted, err := u.prefs.Create(ctx, newPreferenceOf(u.newID(), in.BuyerID, kind,
			in.Value, in.BuyerTaxID, in.BuyerName, in.Label, in.SetDefault), now)
		if err != nil {
			return err
		}
		if !inserted {
			return errProfileDuplicate()
		}
		if in.IdempotencyKey != "" && u.idem != nil {
			if serr := u.idem.SetResult(ctx, IdemScopeBuyerProfileCreate,
				in.BuyerID, in.IdempotencyKey, row.ID); serr != nil {
				return serr
			}
		}
		out = toCheckoutProfile(row)
		return nil
	})
	if err != nil {
		return CheckoutProfile{}, err
	}
	if replayRef != 0 {
		prev, gerr := u.prefs.GetByID(ctx, replayRef, in.BuyerID)
		if gerr != nil {
			return CheckoutProfile{}, gerr
		}
		return toCheckoutProfile(prev), nil
	}
	return out, nil
}

// Update 編輯一列（端點 3）。
//
// 🔴 **不是 UPDATE 值欄位**：I-14「值欄位一旦寫入不再變更」的落實方式是 append-only
// ——新增一列、把舊列 superseded 過去。順序只能是「先建新列、再指過去」，因為
// fk_bip_superseded 不是 DEFERRABLE（domain 介面的 Supersede 註解逐字寫了這條）。
//
// 新列的 is_default 繼承舊列：編輯自己的預設載具之後預設消失，下一次結帳會靜默退成
// 無載具發票，而買家什麼都沒改。繼承的做法是「先以 is_default=false 建列 → supersede
// 舊列（同一句清掉它的 is_default）→ 再把新列設成預設」，中間不會有兩列同時為真而撞
// uq_bip_default。
func (u *Preference) Update(ctx context.Context, in UpdateProfileInput) (CheckoutProfile, error) {
	if code := validateProfileLabel(in.Label); code != 0 {
		return CheckoutProfile{}, profileBiz(code)
	}

	var out CheckoutProfile
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		old, err := u.prefs.GetByID(ctx, in.ProfileID, in.BuyerID)
		if err != nil {
			return err
		}
		kind, ok := codemeta.InvoiceCarrierTypeFromValue(old.PreferenceKind)
		if !ok {
			u.logger().Error("invoice.preference.unknown_kind",
				zap.Int64("profile_id", old.ID), zap.String("preference_kind", old.PreferenceKind))
			return errUnknownPreferenceKind
		}
		if verr := u.verifyBarcodeIfNeeded(ctx, kind, in.Value); verr != nil {
			return verr
		}
		if code := validateProfileFields(kind, in.Value, in.BuyerTaxID, in.BuyerName); code != 0 {
			return profileBiz(code)
		}

		now := u.now()
		newID := u.newID()
		row, inserted, err := u.prefs.Create(ctx, newPreferenceOf(newID, in.BuyerID, kind,
			in.Value, in.BuyerTaxID, in.BuyerName, in.Label, false), now)
		if err != nil {
			return err
		}
		if !inserted {
			return errProfileDuplicate()
		}
		superseded, err := u.prefs.Supersede(ctx, old.ID, in.BuyerID, newID, now)
		if err != nil {
			return err
		}
		if !superseded {
			return profileNotFound()
		}
		if old.IsDefault {
			ok, err := u.prefs.SetDefault(ctx, newID, in.BuyerID, now)
			if err != nil {
				return err
			}
			if !ok {
				return profileNotFound()
			}
			row.IsDefault = true
		}
		out = toCheckoutProfile(row)
		return nil
	})
	if err != nil {
		return CheckoutProfile{}, err
	}
	return out, nil
}

// Delete 軟刪一列（端點 4）。
//
// 🔴 刪除預設一律擋（9304），**不自動改指**：自動改指會讓下一次結帳靜默換成另一種
// 載具，而發票開錯載具只能作廢重開（§6.1.1 端點 4）。
func (u *Preference) Delete(ctx context.Context, buyerID, profileID int64) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		row, err := u.prefs.GetByID(ctx, profileID, buyerID)
		if err != nil {
			return err
		}
		if row.IsDefault {
			return profileBiz(bizcode.InvoiceProfileDefaultRequired)
		}
		ok, err := u.prefs.SoftDelete(ctx, profileID, buyerID, u.now())
		if err != nil {
			return err
		}
		if !ok {
			return profileNotFound()
		}
		return nil
	})
}

// SetDefault 把一列設成預設（端點 5）。
//
// ClearDefault 與 SetDefault 在同一交易內，且 SetDefault 不成立時整筆 rollback ——
// 否則「設一個不存在的 id」會把買家原本的預設清掉，回 404 而預設真的沒了。
func (u *Preference) SetDefault(ctx context.Context, buyerID, profileID int64) (CheckoutProfile, error) {
	var out CheckoutProfile
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		now := u.now()
		if _, err := u.prefs.ClearDefault(ctx, buyerID, now); err != nil {
			return err
		}
		ok, err := u.prefs.SetDefault(ctx, profileID, buyerID, now)
		if err != nil {
			return err
		}
		if !ok {
			return profileNotFound()
		}
		row, err := u.prefs.GetByID(ctx, profileID, buyerID)
		if err != nil {
			return err
		}
		out = toCheckoutProfile(row)
		return nil
	})
	if err != nil {
		return CheckoutProfile{}, err
	}
	return out, nil
}

// ClearDefault 是 profileId = "0" 的語意：買家「我不要任何預設載具」（§6.1.1 端點 5）。
//
// 單句 UPDATE，不需要交易。清完之後結帳的缺省鏈走第 ② 步（無載具發票）。
func (u *Preference) ClearDefault(ctx context.Context, buyerID int64) error {
	_, err := u.prefs.ClearDefault(ctx, buyerID, u.now())
	return err
}


func validateProfileFields(kind codemeta.InvoiceCarrierType, value, taxID, buyerName string) int32 {
	id := checkoutIdentity{CarrierType: kind.Value}
	switch kind.Value {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		id.BuyerTaxID = taxID
		id.BuyerName = buyerName
		id.CarrierValue = value
	case codemeta.InvoiceCarrierTypeDonation.Value:
		id.DonationCode = value
	default:
		id.CarrierValue = value
		id.CarrierValueVisible = true
		id.HasCarrier = true
	}
	if code := validateIdentityShape(id); code != 0 {
		return code
	}
	if kind.Value != codemeta.InvoiceCarrierTypeTaxId.Value && (taxID != "" || buyerName != "") {
		return bizcode.InvoiceTargetConflict
	}
	return 0
}

func validateProfileLabel(label string) int32 {
	if utf8.RuneCountInString(label) > profileLabelMaxRunes {
		return bizcode.InvoiceCarrierInvalid
	}
	return 0
}

func newPreferenceOf(
	id, buyerID int64, kind codemeta.InvoiceCarrierType,
	value, taxID, buyerName, label string, isDefault bool,
) domain.NewBuyerPreference {
	in := domain.NewBuyerPreference{
		ID:             id,
		BuyerID:        buyerID,
		PreferenceKind: kind.Value,
		Label:          label,
		IsDefault:      isDefault,
	}
	switch kind.Value {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		in.TaxID = taxID
		in.TaxTitle = buyerName
	case codemeta.InvoiceCarrierTypeDonation.Value:
		in.DonationCode = value
	default:
		if amego, ok := domain.CarrierTypeToAmego(kind); ok {
			in.CarrierType = amego
		}
		in.CarrierValue = value
		in.CarrierMask = maskCarrierValue(value)
	}
	return in
}

var errUnknownPreferenceKind = errors.New("invoice: preference_kind is outside the CodeMeta value set")

func profileBiz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func profileNotFound() error {
	return profileBiz(bizcode.InvoiceProfileNotFound)
}

func errProfileDuplicate() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceTargetConflict, "已存在相同的常用發票資料")
}

func (u *Preference) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}
