package rfq

import (
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

// 這個檔案是列到 rfqhash 輸入的唯一對照：所有 hash 都由不可變的列在這裡重算，讀取端（snapshot）與寫入端用同一份。

func priceOf(p store.ProcurementPricePreview) rfqhash.Price {
	return rfqhash.Price{
		RangeLowerAmount: p.RangeLowerAmount, RangeUpperAmount: p.RangeUpperAmount, BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit,
		TotalLowerAmount: p.TotalLowerAmount, TotalUpperAmount: p.TotalUpperAmount, TotalQuantity: p.TotalQuantity, TotalUnit: p.TotalUnit,
	}
}

// PreviewHash 回傳價格預覽的 hash；預覽沒有 schema_version 欄，用目前的版本。
func PreviewHash(p store.ProcurementPricePreview) (string, error) {
	return rfqhash.PricePreview(rfqhash.PricePreviewInput{
		SchemaVersion: rfqhash.SchemaV1, PricePreviewID: p.ID.String(), RequestRevisionID: p.RequestRevisionID.String(), Price: priceOf(p),
	})
}

// SnapshotHash 由確認快照、它確認的需求版本與價格預覽回傳快照的 hash；schemaVersion 是依據它的發布（或核准）的原像版本。
func SnapshotHash(snap store.ProcurementConfirmedRfqSnapshot, rev store.ProcurementRequestRevision, prev store.ProcurementPricePreview, schemaVersion string) (string, error) {
	return rfqhash.Snapshot(rfqhash.Input{
		SchemaVersion: schemaVersion, ConfirmedRfqSnapshotID: snap.ID.String(), RequestRevisionID: rev.ID.String(), PricePreviewID: prev.ID.String(), BuyerNote: snap.BuyerNote,
		Requirement: rfqhash.Slots{
			Item: rev.Item, Quantity: rev.Quantity, Unit: rev.Unit, Packaging: rev.Packaging, BudgetPerUnit: rev.BudgetPerUnit,
			DeliveryCity: rev.DeliveryCity, DeliveryDate: rfqhash.Date(rev.DeliveryDate), ExactMatchRequired: rev.ExactMatchRequired,
		},
		Price: priceOf(prev),
	})
}
