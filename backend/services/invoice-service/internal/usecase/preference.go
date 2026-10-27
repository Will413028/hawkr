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

// 本檔是 B08「我的發票資料」的寫入面（§6.1.1 端點 2／3／4／5）。讀取面在 query.go。
//
// 三條貫穿全檔的約束：
//
//	① **Tier A（§8.1.4(b)）**：buyerID 一律進 SQL 的 WHERE，不是查回來再比對。
//	   「不存在」與「不屬於你」共用 9302、同一段 message、同一行程式碼。
//	② **明碼不落 log、不出回應**：載具明碼只從 request 走到 repository 的 Create，
//	   在那一層加密；回應一律用 carrier_value_mask（preferenceDisplayValue）。
//	③ **設預設是兩段操作**（ClearDefault ＋ SetDefault），必須同交易——否則會出現
//	   「零個預設」或撞 uq_bip_default 的窗口，而兩者都不會有人發現。
//
// 逐欄驗證**不另寫一份**：§6.1.1 的驗證表已經有唯一實作 validateIdentityShape
// （checkout.go），本檔把輸入折成同一個 checkoutIdentity 交給它。兩份實作漂移的代價是
// 結帳擋得住、B08 存得進去（或反過來），而那張發票已經開出去了。

// profileLabelMaxRunes 是 buyer_invoice_preferences.label 的 varchar(30)。
//
// 在這裡擋而不是讓 DB 擋：22001 是 error 不是業務碼，前端會拿到 500 而不是
// 「備註名稱太長」。
const profileLabelMaxRunes = 30

// Preference 是買家常用發票資料的寫入 usecase。
//
// 它有 TxManager 而 Query 沒有，理由只有一個：③。除此之外本 usecase 不打 provider、
// 不發事件——常用發票資料的變更不影響任何已開立的憑證。
type Preference struct {
	tx    database.TxManager
	prefs domain.BuyerPreferenceRepository
	// idem 是冪等鍵的耐久儲存（#3015 裁決 A）。**列進建構子參數而不是 setter**：
	// 忘了 wire 的話 setter 版本會留下一個 nil 欄位，而唯一安全的處置（nil 就跳過）
	// 正好等於「靜默退回沒有冪等」——零錯誤、零 log。列進簽章讓那件事變成編譯錯。
	idem domain.IdempotencyRepository

	// barcode 是手機條碼的**存在性**驗證（#3256 入口層）。可以是 nil ⇒ 整段跳過。
	//
	// 🔴 與開票前那一層是**兩道不同的防線，不是重複**：這一層給買家即時回饋
	// （打錯當下就知道），開票前那一層是最後防線（profile 建立後 AMEGO 端可能才失效，
	// 而且既有的 profile 是在本檢查存在之前建的）。少了這一層，買家要等到付款之後
	// 才發現載具是錯的，而那時只剩「轉人工」一條路。
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

// verifyBarcodeIfNeeded 是 §P12 的入口層：手機條碼存不存在，在**存進 DB 之前**問一次。
//
// 🔴 **fail-open**，判準與開票前那一層逐字相同：只有 AMEGO 明確回 9000112（格式錯誤）
// 或 9000113（不存在）才擋，逾時／限流／斷路器／未知碼一律放行。驗證服務掛掉時擋住
// 所有買家存載具，比不驗更糟——而且開票前那一層仍然會擋，錯的載具不會真的開出去。
//
// 三個碼的分辨力是實測來的（2026-09-08，測試統編 12345678）：
//
//	""       → 9000111 手機條碼不可為空
//	ABC1234  → 9000112 手機條碼格式錯誤
//	/ZZZZZZZ → 9000113 手機條碼不存在   ← 格式合法、確實查無此載具
//
// 9000111 不列入：validateProfileFields 已擋掉空值，走到這裡還是空代表上游有 bug。
//
// 回 9305（InvoiceCarrierInvalid）而不新增碼值：新碼要同時決定四份前端拷貝的顯示文案，
// 而兩種失敗對買家的處置是同一句「請確認手機條碼」。差別寫進 message。
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
		// 網路／逾時／斷路器：fail-open。記 warn 讓「驗證長期失效」看得見。
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
		// 值域外的 carrierType ⇒ 格式錯，不猜。CodeMeta 是 carrier_type 的值域 SSOT，
		// 這一行就是「寫入 DB 前必須走 validator」的落點。
		return CheckoutProfile{}, profileBiz(bizcode.InvoiceCarrierInvalid)
	}
	if code := validateProfileFields(kind, in.Value, in.BuyerTaxID, in.BuyerName); code != 0 {
		return CheckoutProfile{}, profileBiz(code)
	}
	if code := validateProfileLabel(in.Label); code != 0 {
		return CheckoutProfile{}, profileBiz(code)
	}
	// #3256 入口層：手機條碼存不存在，在存進 DB 之前問一次。放在交易**之外**——
	// 它是一次外部 HTTP，不該把交易撐開（同 §6.3.1「交易內不放網路呼叫」的理由）。
	if err := u.verifyBarcodeIfNeeded(ctx, kind, in.Value); err != nil {
		return CheckoutProfile{}, err
	}

	// 冪等指紋涵蓋**使用者改得動的每一欄**：漏掉任何一欄，那一種改法就繞得過
	// 「同鍵不同內容」的判定。SetDefault 也要進去——它會改別列的 is_default。
	fingerprint := RequestFingerprint(
		in.CarrierType, in.Value, in.BuyerTaxID, in.BuyerName, in.Label,
		strconv.FormatBool(in.SetDefault),
	)

	var out CheckoutProfile
	var replayRef int64
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		// 🔴 認領放在交易**最前面**且與業務寫入同一個交易：那是「同一把鍵絕不
		// 產生第二筆」唯一成立的方式。放交易外就退回 gateway 那層的 best-effort。
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
		// 上限檢查與寫入必須同交易：分開查有 TOCTOU 窗口，而 9303 沒有 DB 層兜底
		// （不像 uq_bip_default 有唯一索引），越界之後只會在 B08 列表的 cap+1 那行
		// WARN 裡出現。
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
			// 同交易內補結果：業務寫入 rollback 時這一列一起消失，
			// 不會留下「認領了但沒有結果」的孤兒。
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
		// 回放：讀第一次建立的那一列回傳，**不重跑業務寫入**。
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
			return err // 查無／越權共用 9302（repository 的 notFound）
		}
		kind, ok := codemeta.InvoiceCarrierTypeFromValue(old.PreferenceKind)
		if !ok {
			// DB 的 preference_kind 落在 CodeMeta 值域外 ＝ chk_bip_kind 與 CodeMeta
			// 已經漂移。這是故障不是業務結果，回 error 讓它進 log 與 500。
			u.logger().Error("invoice.preference.unknown_kind",
				zap.Int64("profile_id", old.ID), zap.String("preference_kind", old.PreferenceKind))
			return errUnknownPreferenceKind
		}
		// 🔴 Update 也要驗：買家改的正是 value 本身。少了這一格，入口層的保證只擋得住
		// 「新建」而擋不住「改成一個錯的」——而後者在 UI 上是同一顆按鈕。
		//
		// ⚠️ 這一次**在交易內**（kind 要先讀舊列才知道），與 Create 那次不同。可接受：
		// 它是唯讀查詢、有斷路器與逾時（AMEGO_HTTP_TIMEOUT_QUERY_MS=10s），而 fail-open
		// 保證它不會把交易卡在一個掛掉的外部服務上。
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
			// 舊列還在存活集合裡 ⇒ 值沒變的編輯會撞去重鍵。詳見交付回報。
			return errProfileDuplicate()
		}
		superseded, err := u.prefs.Supersede(ctx, old.ID, in.BuyerID, newID, now)
		if err != nil {
			return err
		}
		if !superseded {
			// 同一交易內剛讀到的列不可能消失；真的發生代表述詞與 GetByID 不一致。
			// 回錯讓整筆 rollback——放行會留下兩列存活的同一筆資料。
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

// ---------------------------------------------------------------------------
// 驗證與組裝
// ---------------------------------------------------------------------------

// validateProfileFields 是 §6.1.1 逐欄驗證表在寫入端點的入口。回 0 表示通過。
//
// 🔴 它**不重寫**那張表：格式判斷全部委託 validateIdentityShape（checkout.go 的唯一
// 實作）。本函式只多做一件結帳那側做不到的事——單筆 profile 的三選一（9306）。
func validateProfileFields(kind codemeta.InvoiceCarrierType, value, taxID, buyerName string) int32 {
	id := checkoutIdentity{CarrierType: kind.Value}
	switch kind.Value {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		id.BuyerTaxID = taxID
		id.BuyerName = buyerName
		// value 必須為空（表格逐字）；帶了值代表前端把載具塞進了統編那一格。
		id.CarrierValue = value
	case codemeta.InvoiceCarrierTypeDonation.Value:
		id.DonationCode = value
	default: // MOBILE_BARCODE / CITIZEN_CERT
		id.CarrierValue = value
		// CarrierValueVisible=true 才會真的跑正則：false 是「值是密文、看不到」，
		// 而寫入端點手上的就是明碼。
		id.CarrierValueVisible = true
		id.HasCarrier = true
	}
	if code := validateIdentityShape(id); code != 0 {
		return code
	}
	// 9306：非統編型卻夾帶統編／抬頭。AMEGO 不擋這三種組合（實測全過）⇒ 我方強制。
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

// newPreferenceOf 把驗過的輸入折成 repository 的寫入形狀。
//
// 每一型只填自己的欄位、其餘留空字串（repository 寫 NULL）：chk_bip_shape 的四個分支
// 就是靠 NULL 與否判定的。
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
		// carrier_type 走 domain 的對照表（chk_bip_shape 要求 MOBILE_BARCODE 恆為
		// '3J0002'、CITIZEN_CERT 恆為 'CQ0001'），不在這裡重打字面值。
		if amego, ok := domain.CarrierTypeToAmego(kind); ok {
			in.CarrierType = amego
		}
		in.CarrierValue = value
		in.CarrierMask = maskCarrierValue(value)
	}
	return in
}

// errUnknownPreferenceKind 是 DB 值域與 CodeMeta 漂移時的故障訊息。
//
// 🔴 刻意**不是** DomainError：bizOf 會把 DomainError 折成 envelope 的業務碼，而這件事
// 沒有業務碼可回、也不該讓前端看到一句業務文案。它是故障，走 codes.Internal。
var errUnknownPreferenceKind = errors.New("invoice: preference_kind is outside the CodeMeta value set")

// profileBiz 造一個帶業務碼的 DomainError；訊息一律取自 CodeMeta。
func profileBiz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func profileNotFound() error {
	return profileBiz(bizcode.InvoiceProfileNotFound)
}

// errProfileDuplicate 是 ON CONFLICT DO NOTHING 的出口。
//
// ⚠️ **CodeMeta 沒有「常用發票資料重複」這個碼**（§6.1.1 端點 2 的失敗清單只有
// 9303／9305／9306）。借 9306 是為了拿到正確的 HTTP 409 與可讀訊息，語意上並不精準
// ——已列進交付回報，正解是新增一個專屬碼。
func errProfileDuplicate() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceTargetConflict, "已存在相同的常用發票資料")
}

func (u *Preference) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}
