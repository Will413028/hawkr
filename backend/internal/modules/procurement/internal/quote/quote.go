// Package quote 是買家對賣家回價的使用：列出一次發布收到的回價（每位賣家的目前版本）、入圍清單、接受報價。
// 這裡只讀回價列（寫入回價的是賣家端的 use case）；接受報價只寫接受單並在同一個 transaction 排入建單的 job，job 的 worker 與 ordering 呼叫不在這個套件（A9-D31 Q10）。
// 狀態與可否接受都由列推導（版本只 INSERT、沒有 status 欄），outside_buyer_requested_range 對發布核准凍結的單價區間現算（A9-D35(2)）。
// 版本衝突的 conflictTarget：seller_response（買家手上那一版已不是目前、可接受的那一版）、publication（買家畫面上的詢價內容與賣家報價時依據的那份不同）。
package quote

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	// DefaultPageSize 是 page_size 為 0 時的每頁筆數；MaxPageSize 是每頁上限，超過以上限計。
	DefaultPageSize = 50
	MaxPageSize     = 100
)

// 衝突目標（ErrorInfo.metadata 的 conflictTarget）。
const (
	TargetSellerResponse = "seller_response"
	TargetPublication    = "publication"
)

// Service 是買家使用賣家回價的 use case。Jobs 排入建單的 job（AcceptSellerQuote 需要；*jobs.Client 滿足它）。
type Service struct {
	DB   *db.DB
	Now  func() time.Time
	Jobs run.JobQueue
}

// OrderProvisioningArgs 是接受報價之後建立待付款訂單的 job：每筆接受單恰好一個 job，在寫入接受單的同一個 transaction 排入。
// 接受單的寫入（冪等鍵與 live 鎖的唯一索引）就是排入的閘門，重送走回放、不再排入，所以不設 job 層的唯一鍵；worker 與 ordering 呼叫不在這個套件。
type OrderProvisioningArgs struct {
	AcceptanceID uuid.UUID `json:"acceptanceId"`
}

// Kind 是 job 的種類。
func (OrderProvisioningArgs) Kind() string { return "procurement.order_provisioning" }

// VersionConflict 是 ABORTED／VERSION_CONFLICT，conflictTarget 指出哪個物件過時（買家端與賣家端共用）。
func VersionConflict(msg, target string) error {
	return apperr.VersionConflict("%s", msg).WithMetadata("conflictTarget", target)
}

// IdempotencyMismatch 是同一個 client_request_id 帶了不同內容（INVALID_ARGUMENT，欄位 client_request_id）。
func IdempotencyMismatch() error {
	return apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
}

// VersionStatus 推導一個版本的狀態：已撤回、被新版取代、過了有效期限，否則是進行中。
func VersionStatus(version, latest int32, withdrawn *int32, validUntil, now time.Time) string {
	switch {
	case withdrawn != nil && *withdrawn == version:
		return model.ResponseWithdrawn
	case version < latest:
		return model.ResponseSuperseded
	case now.After(validUntil):
		return model.ResponseExpired
	default:
		return model.ResponseActive
	}
}

// Acceptability 回傳買家能不能接受：只有進行中的版本、賣家沒有停止寫入、這張需求沒有進行中的成交時可以。
func Acceptability(status string, writeClosed, requestHasLiveAcceptance bool) string {
	switch status {
	case model.ResponseWithdrawn:
		return model.AcceptabilityWithdrawn
	case model.ResponseSuperseded:
		return model.AcceptabilitySuperseded
	case model.ResponseExpired:
		return model.AcceptabilityExpired
	}
	if writeClosed || requestHasLiveAcceptance {
		return model.AcceptabilityIneligible
	}
	return model.AcceptabilityAcceptable
}

// OutsideRange 回傳報價是否超出買家預期的單價區間：區間是每 basisQuantity 個 basisUnit 的貨款單價；報價數量的單位與 basisUnit 相同時，
// 區間依報價數量等比放大（bound × 報價數量 ÷ basisQuantity，四捨五入到元，與 rfq.Total 同規則），貨款（不含運費與稅）超出放大後的區間就是 true。
// 報價數量可以不是基準的整數倍。單位不同（換算說明 conversion_basis 是自由文字，讀不出換算）、數量不是正的有限數時回 false（不能比較，不標示）。
func OutsideRange(lower, upper int64, basisQuantity int32, basisUnit string, v store.ProcurementSellerResponseVersion) bool {
	offered, ok := milli(v.OfferedQuantity)
	if !ok || basisQuantity <= 0 || offered <= 0 || v.OfferedQuantityUnit != basisUnit {
		return false
	}
	basis := big.NewInt(int64(basisQuantity) * 1000)
	goods := big.NewInt(v.GoodsAmount)
	return goods.Cmp(scaleBound(lower, offered, basis)) < 0 || goods.Cmp(scaleBound(upper, offered, basis)) > 0
}

// scaleBound 回傳 bound × offered ÷ basis 四捨五入到整數（offered 與 basis 同為千分位整數）。
func scaleBound(bound, offered int64, basis *big.Int) *big.Int {
	product := new(big.Int).Mul(big.NewInt(bound), big.NewInt(offered))
	product.Add(product, new(big.Int).Div(basis, big.NewInt(2)))
	return product.Quo(product, basis)
}

// milli 把 NUMERIC(14, 3) 讀回的值換成千分位整數（3 -> 3000）；不是有限的數或不是整千分位時 ok 為 false。
func milli(n pgtype.Numeric) (int64, bool) {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite || n.Int == nil {
		return 0, false
	}
	v := new(big.Int).Set(n.Int)
	if e := n.Exp + 3; e >= 0 {
		v.Mul(v, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(e)), nil))
	} else {
		q, r := new(big.Int).QuoRem(v, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-e)), nil), new(big.Int))
		if r.Sign() != 0 {
			return 0, false
		}
		v = q
	}
	if !v.IsInt64() {
		return 0, false
	}
	return v.Int64(), true
}

func decimal3(n pgtype.Numeric) (string, error) {
	m, ok := milli(n)
	if !ok || m < 0 {
		return "", fmt.Errorf("quote：數量不是三位小數內的非負數")
	}
	return fmt.Sprintf("%d.%03d", m/1000, m%1000), nil
}

// QuoteOf 把一個版本的列轉成報價內容。
func QuoteOf(v store.ProcurementSellerResponseVersion) (model.SellerQuote, error) {
	unitBasis, err := decimal3(v.UnitBasisQuantity)
	if err != nil {
		return model.SellerQuote{}, fmt.Errorf("quote：版本 %s 的計價基準數量：%w", v.ID, err)
	}
	offered, err := decimal3(v.OfferedQuantity)
	if err != nil {
		return model.SellerQuote{}, fmt.Errorf("quote：版本 %s 的報價數量：%w", v.ID, err)
	}
	return model.SellerQuote{
		ResponseType: v.ResponseType, SpecNote: v.SpecNote,
		UnitBasisQuantity: unitBasis, UnitBasisUnit: v.UnitBasisUnit, OfferedQuantity: offered, OfferedQuantityUnit: v.OfferedQuantityUnit,
		ConversionBasis: v.ConversionBasis, Currency: v.Currency, GoodsAmount: v.GoodsAmount, ShippingAmount: v.ShippingAmount, TaxAmount: v.TaxAmount,
		TaxInclusionBasis: v.TaxInclusionBasis, TotalAmount: v.TotalAmount, LeadTimeDays: v.LeadTimeDays, PromisedDelivery: v.PromisedDeliveryAt,
		ValidUntil: v.ValidUntil, TermsNote: v.TermsNote, DeviationDisclosure: v.DeviationDisclosure,
	}, nil
}

// PayloadHash 回傳一個版本的內容 hash（A9-D36）：報價內容與幣別，不含 ID、版本號與送出時間。
func PayloadHash(q model.SellerQuote) (string, error) {
	return rfqhash.SellerResponse(rfqhash.SellerResponseInput{
		SchemaVersion: rfqhash.SchemaV1, ResponseType: q.ResponseType, SpecNote: q.SpecNote,
		UnitBasisQuantity: q.UnitBasisQuantity, UnitBasisUnit: q.UnitBasisUnit, OfferedQuantity: q.OfferedQuantity, OfferedQuantityUnit: q.OfferedQuantityUnit,
		ConversionBasis: q.ConversionBasis, Currency: q.Currency, GoodsAmount: q.GoodsAmount, ShippingAmount: q.ShippingAmount, TaxAmount: q.TaxAmount,
		TaxInclusionBasis: q.TaxInclusionBasis, TotalAmount: q.TotalAmount, LeadTimeDays: q.LeadTimeDays, PromisedDelivery: q.PromisedDelivery,
		ValidUntil: q.ValidUntil, TermsNote: q.TermsNote, DeviationDisclosure: q.DeviationDisclosure,
	})
}

// owned 把「沒有列」換成 NOT_FOUND（不屬於這位買家與不存在不分）。
func owned(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到這%s", what)
	}
	return err
}
