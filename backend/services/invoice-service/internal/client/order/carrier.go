package order

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 載具解析（§5.4.3、§6.1.2(b)、I-47）
// ---------------------------------------------------------------------------
//
// 🔴 **載具值不在 proto 裡。** `order_invoice_snapshots` 是零 PII 的表（§5.4.3 表註解），
// 它只帶 `carrier_selection_mode` ＋ `buyer_invoice_preference_id`；`carrier_type` /
// `carrier_id1` / `carrier_id2` 三個值一律留在 invoice-service。所以 ORD adapter 在填
// OrderFacts **之前**必須自己解析一次，解析結果當場快照進 invoice_documents，從此不可變。
//
// ⚠️ **fallback 鏈只有兩步，沒有第三步**（§6.1.2(b) 的紅字方框、D6／D7／ADR-22）：
//
//	① 買家有存活的 is_default profile → 用它
//	② 否則 → 無載具發票
//
// §5.4.3 的散文與 CodeMeta `InvoiceCarrierSelectionMode` 的 Description 都還寫著舊版的
// 四段鏈（「→ 手機（amego 載具）→ email」），那是**已被整體移除的設計**：本設計不代買家
// 配任何載具，`InvoiceCarrierType` 也只有 4 值、沒有承載 amego 會員載具的第 5 個值。
// 兩處殘留已列進交付回報，不得照它施工。

// ErrNoDefaultPreference 表示該買家沒有存活的預設常用發票資料。
//
// 它是**正常值不是錯誤**：對應 §6.1.2(b) 的第 ② 步「無載具發票」。具名而不是回零值，
// 是為了讓「沒有預設」與「查詢失敗」在 resolver 內分得開——後者必須往上拋。
var ErrNoDefaultPreference = stderrors.New("invoice: buyer has no default invoice preference")

// ErrPreferenceCapabilityUnavailable 是本波尚未落地的兩個 repository 能力的 fail-loud 訊號。
//
// 🔴 它刻意是 error 而不是「當成沒有載具繼續走」：兩者的後果不對稱。
// 「買家設了載具但我們開成無載具」是一張開錯的發票（不可回收、買家的載具歸戶落空）；
// 「這則 order.paid 進 DLQ」是看得見的（`mq_consume_total{result="dlq"}`）而且訊息不遺失，
// 補完能力之後可從 DLQ 重放。
//
// 缺的兩件事（都在 `internal/repository/postgres` 與 `db/queries`，不在本波可寫範圍）：
//   - `GetDefaultBuyerInvoicePreference`：`WHERE buyer_id = ? AND is_default
//     AND deleted_at IS NULL AND superseded_at IS NULL`（uq_bip_default 保證最多一列）
//   - `WithDecryptedCarrierValue`：`carrier_value_ciphertext` 的明文出口。
//     ⚠️ 不得用 `ListBuyerInvoicePreferences` ＋ Go 端過濾代替前者：那支帶 LIMIT，
//     預設列排在 limit 之外時會**靜默**回「沒有預設」，正是本檔要防的失效模式。
var ErrPreferenceCapabilityUnavailable = stderrors.New(
	"invoice: buyer preference capability is not wired yet (GetDefault / carrier value decryption)")

// BuyerPreferenceReader 是載具解析需要的三個讀取動作。
//
// 介面宣告在**消費端**（本 package）而不是 domain：解析是 ORD adapter 的職責，
// 而 `domain.BuyerPreferenceRepository` 今天只有 GetByID 一支。
type BuyerPreferenceReader interface {
	// GetByID 的 buyerID 是授權述詞的一部分（寫進 SQL 的 WHERE，不是查回來再比對）。
	// 查不到與不屬於你刻意不可分辨（Tier A，§8.1.4(b)）。
	GetByID(ctx context.Context, id, buyerID int64) (domain.BuyerPreference, error)
	// GetDefault 取該買家唯一存活的 is_default 列；沒有時回 ErrNoDefaultPreference。
	GetDefault(ctx context.Context, buyerID int64) (domain.BuyerPreference, error)
	// OpenCarrierValue 取一列的載具顯碼明文（AES-256-GCM 解密）。
	// 該列沒有載具值（TAX_ID／DONATION）時回空字串 ＋ nil error。
	OpenCarrierValue(ctx context.Context, p domain.BuyerPreference) (string, error)
}

// carrierResolution 是解析結果。它整包覆蓋 OrderFacts 上對應的六個欄位。
type carrierResolution struct {
	CarrierType  codemeta.InvoiceCarrierType
	CarrierID1   string
	CarrierID2   string
	DonationCode string
	BuyerTaxID   string
	BuyerTitle   string

	// OwnershipMismatch ＝ I-47 的歸屬驗證失敗。
	// 🔴 它**不是** fallback 成無載具：後者會把 IDOR 靜默吞掉。
	OwnershipMismatch bool
}

// resolveCarrier 依 carrier_selection_mode 解析載具（§5.4.3）。
//
// buyerID **只能取自訂單**（`orders.buyer_id`，經 order.paid payload 傳入），
// 不得取自任何請求欄位——那正是 I-47 要防的那條路。
func resolveCarrier(
	ctx context.Context,
	prefs BuyerPreferenceReader,
	mode string,
	preferenceID int64,
	buyerID int64,
	snapshotTaxID string,
	snapshotTitle string,
	snapshotDonationCode string,
) (carrierResolution, error) {
	switch mode {
	case codemeta.InvoiceCarrierSelectionModeNone.Value:
		// B2B 一律走這一格：統編與抬頭由結帳快照直接帶，沒有載具要解析。
		return carrierResolution{BuyerTaxID: snapshotTaxID, BuyerTitle: snapshotTitle}, nil

	case codemeta.InvoiceCarrierSelectionModeDonation.Value:
		if snapshotDonationCode == "" {
			// chk_ois_donation_pair 是雙向等式 ⇒ 走到這裡代表快照本身壞了。
			// 靜默開成無載具會把買家的捐贈意願吞掉，而發票不可回收。
			return carrierResolution{}, fmt.Errorf(
				"invoice: carrier mode DONATION with empty donation_code (chk_ois_donation_pair violated upstream)")
		}
		return carrierResolution{
			CarrierType:  codemeta.InvoiceCarrierTypeDonation,
			DonationCode: snapshotDonationCode,
		}, nil

	case codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value:
		if preferenceID <= 0 {
			// chk_ois_pref_pair 是雙向等式，同上。
			return carrierResolution{}, fmt.Errorf(
				"invoice: carrier mode EXPLICIT_PREFERENCE with empty buyer_invoice_preference_id (chk_ois_pref_pair violated upstream)")
		}
		if buyerID <= 0 {
			// 沒有 buyer_id 就驗不了歸屬。這裡**不得**退化成「不驗」。
			return carrierResolution{}, fmt.Errorf(
				"invoice: carrier mode EXPLICIT_PREFERENCE requires a buyer_id from the order (got %d)", buyerID)
		}
		pref, err := prefs.GetByID(ctx, preferenceID, buyerID)
		if err != nil {
			if isBizCode(err, bizcode.InvoiceProfileNotFound) {
				// 🔴 I-47：查不到 ＝ 不屬於這個買家（buyer_id 在 SQL 的 WHERE 內）
				// ⇒ **拒絕開票、轉 AWAITING_MANUAL**，不是 fallback 成無載具。
				return carrierResolution{OwnershipMismatch: true}, nil
			}
			return carrierResolution{}, fmt.Errorf(
				"invoice: resolve explicit preference (id=%d buyer_id=%d): %w", preferenceID, buyerID, err)
		}
		return fromPreference(ctx, prefs, pref)

	case codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value:
		if buyerID <= 0 {
			return carrierResolution{}, fmt.Errorf(
				"invoice: carrier mode USE_BUYER_DEFAULT requires a buyer_id from the order (got %d)", buyerID)
		}
		pref, err := prefs.GetDefault(ctx, buyerID)
		if err != nil {
			if stderrors.Is(err, ErrNoDefaultPreference) || isBizCode(err, bizcode.InvoiceProfileNotFound) {
				// §6.1.2(b) 第 ② 步：沒有預設 ⇒ 無載具發票。這是正常分支。
				return carrierResolution{}, nil
			}
			return carrierResolution{}, fmt.Errorf(
				"invoice: resolve default preference (buyer_id=%d): %w", buyerID, err)
		}
		return fromPreference(ctx, prefs, pref)

	default:
		// 值域由 chk_ois_mode 正面列舉。多出來的值只可能是上游改了契約沒同步，
		// 猜一個等於在憑證上寫錯買受人身分。
		return carrierResolution{}, fmt.Errorf("invoice: unknown carrier_selection_mode %q", mode)
	}
}

// fromPreference 把一列常用發票資料攤成解析結果。
//
// `preference_kind` ＝ CodeMeta `InvoiceCarrierType` 的 Value（三層三名的第一名，
// I-59 第 2 組）：DB 的 `carrier_type` 存的是 AMEGO 原生詞彙 '3J0002'/'CQ0001'，
// **不是**本 enum 的 Value，兩者不可互相代入。
func fromPreference(ctx context.Context, prefs BuyerPreferenceReader, p domain.BuyerPreference) (carrierResolution, error) {
	kind, ok := codemeta.InvoiceCarrierTypeFromValue(p.PreferenceKind)
	if !ok {
		return carrierResolution{}, fmt.Errorf(
			"invoice: preference %d has unknown preference_kind %q", p.ID, p.PreferenceKind)
	}

	switch kind.Code {
	case codemeta.InvoiceCarrierTypeTaxId.Code:
		return carrierResolution{
			CarrierType: kind,
			BuyerTaxID:  deref(p.TaxID),
			BuyerTitle:  deref(p.TaxTitle),
		}, nil

	case codemeta.InvoiceCarrierTypeDonation.Code:
		return carrierResolution{CarrierType: kind, DonationCode: deref(p.DonationCode)}, nil

	default: // MOBILE_BARCODE / CITIZEN_CERT
		value, err := prefs.OpenCarrierValue(ctx, p)
		if err != nil {
			return carrierResolution{}, fmt.Errorf("invoice: open carrier value (preference=%d): %w", p.ID, err)
		}
		if value == "" {
			// chk_bip_shape 保證這兩種 kind 的密文非空 ⇒ 空明文代表解密路徑壞了。
			// 送一張沒有載具號碼的載具發票，AMEGO 會收下（它不擋），而買家歸不到戶。
			return carrierResolution{}, fmt.Errorf(
				"invoice: preference %d (%s) decrypted to an empty carrier value", p.ID, p.PreferenceKind)
		}
		return carrierResolution{CarrierType: kind, CarrierID1: value}, nil
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// isBizCode 回報 err 是不是指定業務碼的 DomainError。
//
// 🔴 只認明確列出的碼——用 pgx.ErrNoRows 之類的泛化判斷會把「連線斷了」也吞成
// 「這筆常用資料不屬於你」，而那兩件事的正確處置相反（重試 vs 轉人工）。
func isBizCode(err error, code int32) bool {
	var de *pkgerrors.DomainError
	return stderrors.As(err, &de) && int32(de.Code) == code
}

// ---------------------------------------------------------------------------
// 尚未落地的兩個 repository 能力：一律 fail-loud
// ---------------------------------------------------------------------------

// RepoPreferenceReader 把 `domain.BuyerPreferenceRepository` 接成 BuyerPreferenceReader。
//
// 三支全部落地（2026-08-30）：`GetDefaultBuyerInvoicePreference` query 與
// `WithDecryptedCarrierValue` 明文出口都已在 repository 層補齊。
// 在那之前這裡是 fail-loud 佔位，`USE_BUYER_DEFAULT` 與載具型 EXPLICIT_PREFERENCE
// 會整批進 DLQ。
type RepoPreferenceReader struct {
	Repo domain.BuyerPreferenceRepository
}

var _ BuyerPreferenceReader = RepoPreferenceReader{}

// GetByID 直接委派（buyer_id 已在它的 SQL WHERE 內，I-47）。
func (r RepoPreferenceReader) GetByID(ctx context.Context, id, buyerID int64) (domain.BuyerPreference, error) {
	if r.Repo == nil {
		return domain.BuyerPreference{}, ErrPreferenceCapabilityUnavailable
	}
	return r.Repo.GetByID(ctx, id, buyerID)
}

// GetDefault 把 repository 的 (pref, found, err) 三元組翻成本介面的 sentinel 形狀。
//
// 🔴 `found == false` 是**正常分支**（多數買家沒設過預設），必須翻成
// ErrNoDefaultPreference 讓呼叫端走「無載具發票」那一步；翻成一般 error 會讓
// order.paid 整批進 DLQ，而那些買家的發票本來就該開成無載具。
func (r RepoPreferenceReader) GetDefault(ctx context.Context, buyerID int64) (domain.BuyerPreference, error) {
	if r.Repo == nil {
		return domain.BuyerPreference{}, ErrPreferenceCapabilityUnavailable
	}
	pref, found, err := r.Repo.GetDefault(ctx, buyerID)
	if err != nil {
		return domain.BuyerPreference{}, err
	}
	if !found {
		return domain.BuyerPreference{}, ErrNoDefaultPreference
	}
	return pref, nil
}

// OpenCarrierValue 取載具顯碼明文。
//
// 明文只在這個函式的回傳值上活著，由呼叫端立刻填進 carrierResolution ⇒ 不進 log、
// 不進 domain struct。TAX_ID／DONATION 沒有載具值 ⇒ 回空字串 ＋ nil，這條路不受影響。
func (r RepoPreferenceReader) OpenCarrierValue(ctx context.Context, p domain.BuyerPreference) (string, error) {
	if p.CarrierValueCiphertext == nil {
		return "", nil
	}
	if r.Repo == nil {
		return "", ErrPreferenceCapabilityUnavailable
	}
	var plain string
	if err := r.Repo.WithDecryptedCarrierValue(ctx, p.ID, p.BuyerID, func(_, v string) error {
		plain = v
		return nil
	}); err != nil {
		return "", err
	}
	return plain, nil
}
