package rfq

import (
	"context"
	"fmt"
	"time"

	"uuid"

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

// compile 讀核准對應的需求版本與受眾名單，連同核准凍結的單價範圍、補充說明與回價期限組出賣家看得到的內容並重算 hash；任何一列讀不到是資料不一致。
func compile(ctx context.Context, q *store.Queries, buyerID int64, a store.ProcurementRfqPublicationApproval) (compiled, error) {
	owner := buyerID
	rev, err := q.GetRevision(ctx, store.GetRevisionParams{ID: a.RequestRevisionID, RequestID: a.RequestID, BuyerID: &owner})
	if err != nil {
		return compiled{}, fmt.Errorf("rfq：讀取核准 %s 的需求版本：%w", a.ID, err)
	}
	if rev.Unit == nil {
		return compiled{}, fmt.Errorf("rfq：核准 %s 的需求版本沒有單位", a.ID)
	}
	recipients, err := q.ListApprovalRecipients(ctx, store.ListApprovalRecipientsParams{ApprovalID: a.ID, BuyerID: &owner})
	if err != nil {
		return compiled{}, fmt.Errorf("rfq：讀取核准 %s 的受眾：%w", a.ID, err)
	}
	c := compiled{policyID: a.AudiencePolicyID, policyVersion: a.AudiencePolicyVersion, storeIDs: make([]int64, 0, len(recipients))}
	for _, r := range recipients {
		c.storeIDs = append(c.storeIDs, r.StoreID)
	}
	return assemble(a, rev, c)
}

// assemble 由核准、它的需求版本與受眾名單組出賣家看得到的內容並重算兩個 hash（純函式，沒有 I/O）：賣家可見的每個欄位不是來自核准凍結的欄位就是來自不可變的需求版本。
func assemble(a store.ProcurementRfqPublicationApproval, rev store.ProcurementRequestRevision, c compiled) (compiled, error) {
	basisQty, basisUnit := Basis(*rev.Unit)
	c.payload = model.SellerFacing{
		Item: deref(rev.Item), ItemLabel: deref(rev.ItemResolutionLabel), Quantity: rev.Quantity, Unit: rev.Unit, Packaging: rev.Packaging,
		DeliveryCity: deref(rev.DeliveryCity), DeliveryDate: rev.DeliveryDate, ResponseDate: a.ResponseDeadline, BuyerNote: a.BuyerNote,
		BasisQuantity: basisQty, BasisUnit: basisUnit, LowerBoundAmount: a.RangeLowerAmount, UpperBoundAmount: a.RangeUpperAmount,
	}
	var err error
	if c.sellerHash, err = rfqhash.SellerFacing(sellerInput(a.SchemaVersion, c.payload)); err != nil {
		return compiled{}, err
	}
	c.audienceHash, err = rfqhash.Audience(audienceInput(a, c.storeIDs))
	return c, err
}

// SellerFacingHash 回傳核准對應的賣家可見內容的 hash（由核准與它的需求版本重算）；讀取模型的 PublishedRfq 用它，與預覽、執行回的是同一個值。
func SellerFacingHash(ctx context.Context, q *store.Queries, buyerID int64, requestID, approvalID uuid.UUID) (string, error) {
	owner := buyerID
	a, err := q.GetApproval(ctx, store.GetApprovalParams{ID: approvalID, RequestID: requestID, BuyerID: &owner})
	if err != nil {
		return "", fmt.Errorf("rfq：讀取發布 %s 的核准：%w", approvalID, err)
	}
	c, err := compile(ctx, q, buyerID, a)
	return c.sellerHash, err
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
