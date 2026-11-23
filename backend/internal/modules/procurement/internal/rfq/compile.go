package rfq

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
)

// compiled 是一份核准對賣家與受眾的內容，與它的兩個 hash：全部由核准參照的不可變列重算（核准只存參照）。
type compiled struct {
	payload       model.SellerFacing
	sellerHash    string
	audienceHash  string
	storeIDs      []int64
	policyID      *string
	policyVersion *int32
}

// compile 讀核准參照的確認快照、需求版本、價格預覽與受眾名單，組出賣家看得到的內容並重算 hash；任何一列讀不到是資料不一致。
func compile(ctx context.Context, q *store.Queries, buyerID int64, a store.ProcurementRfqPublicationApproval) (compiled, error) {
	owner := buyerID
	snap, err := q.GetConfirmedSnapshot(ctx, store.GetConfirmedSnapshotParams{ID: a.ConfirmedRfqSnapshotID, RequestID: a.RequestID, BuyerID: &owner})
	if err != nil {
		return compiled{}, fmt.Errorf("rfq：讀取核准 %s 的確認快照：%w", a.ID, err)
	}
	rev, err := q.GetRevision(ctx, store.GetRevisionParams{ID: snap.RequestRevisionID, RequestID: a.RequestID, BuyerID: &owner})
	if err != nil {
		return compiled{}, fmt.Errorf("rfq：讀取核准 %s 的需求版本：%w", a.ID, err)
	}
	prev, err := q.GetPricePreview(ctx, store.GetPricePreviewParams{ID: snap.PricePreviewID, RequestID: a.RequestID, BuyerID: &owner})
	if err != nil {
		return compiled{}, fmt.Errorf("rfq：讀取核准 %s 的價格預覽：%w", a.ID, err)
	}
	recipients, err := q.ListApprovalRecipients(ctx, store.ListApprovalRecipientsParams{ApprovalID: a.ID, BuyerID: &owner})
	if err != nil {
		return compiled{}, fmt.Errorf("rfq：讀取核准 %s 的受眾：%w", a.ID, err)
	}
	c := compiled{policyID: a.AudiencePolicyID, policyVersion: a.AudiencePolicyVersion, storeIDs: make([]int64, 0, len(recipients))}
	for _, r := range recipients {
		c.storeIDs = append(c.storeIDs, r.StoreID)
	}
	c.payload = model.SellerFacing{
		Item: deref(rev.Item), ItemLabel: deref(rev.ItemResolutionLabel), Quantity: rev.Quantity, Unit: rev.Unit, Packaging: rev.Packaging,
		DeliveryCity: deref(rev.DeliveryCity), DeliveryDate: rev.DeliveryDate, ResponseDate: a.ResponseDeadline, BuyerNote: snap.BuyerNote,
		BasisQuantity: prev.BasisQuantity, BasisUnit: prev.BasisUnit, LowerBoundAmount: prev.RangeLowerAmount, UpperBoundAmount: prev.RangeUpperAmount,
	}
	if c.sellerHash, err = rfqhash.SellerFacing(sellerInput(a.SchemaVersion, c.payload)); err != nil {
		return compiled{}, err
	}
	c.audienceHash, err = rfqhash.Audience(audienceInput(a, c.storeIDs))
	return c, err
}

func audienceInput(a store.ProcurementRfqPublicationApproval, storeIDs []int64) rfqhash.AudienceInput {
	return rfqhash.AudienceInput{SchemaVersion: a.SchemaVersion, Mode: a.AudienceMode, PolicyID: a.AudiencePolicyID, PolicyVersion: a.AudiencePolicyVersion, StoreIDs: storeIDs}
}

func sellerInput(schemaVersion string, p model.SellerFacing) rfqhash.SellerFacingInput {
	return rfqhash.SellerFacingInput{
		SchemaVersion: schemaVersion, Item: p.Item, ItemLabel: p.ItemLabel, Quantity: p.Quantity, Unit: p.Unit, Packaging: p.Packaging,
		DeliveryCity: p.DeliveryCity, DeliveryDate: rfqhash.Date(p.DeliveryDate), ResponseDeadline: rfqhash.Date(p.ResponseDate), BuyerNote: p.BuyerNote,
		Range: rfqhash.SellerRange{BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit, LowerBoundAmount: p.LowerBoundAmount, UpperBoundAmount: p.UpperBoundAmount},
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// dateOnly 把時間正規化成當日 UTC 午夜（DATE 欄位的值）。
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
