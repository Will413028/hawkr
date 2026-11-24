package quote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ListInput 是 ListSellerResponses 的輸入。
type ListInput struct {
	BuyerID                  int64
	RequestID, PublicationID uuid.UUID
	PageSize                 int32
	PageToken                string
}

// 分頁 token 的列表種類。
const (
	TokenSellerResponses    = "seller_responses"
	TokenSellerPublications = "seller_publications"
)

// Token 是 next_page_token 的內容：列表種類與上一頁最後一列的 (排序時間, ID)，base64url 的 JSON，對 client 不透明。
type Token struct {
	Kind      string    `json:"k"` // 發出這個 token 的列表；別的列表的 token 不收
	CreatedAt int64     `json:"c"` // Unix 微秒（與 timestamptz 同精度）
	ID        uuid.UUID `json:"i"`
}

func EncodeToken(kind string, t time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(Token{Kind: kind, CreatedAt: t.UnixMicro(), ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

// minTokenMicro 是 Postgres timestamptz 的下界（4713 BC）；範圍外的值送進查詢會變成 INTERNAL。上界不用檢查：int64 微秒最多到 294247 AD，在 timestamptz 的上界之內。
var minTokenMicro = time.Date(-4712, 1, 2, 0, 0, 0, 0, time.UTC).UnixMicro()

func DecodeToken(kind, raw string) (Token, bool) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return Token{}, false
	}
	var t Token
	if err := json.Unmarshal(b, &t); err != nil || t.Kind != kind || t.ID == (uuid.UUID{}) || t.CreatedAt < minTokenMicro {
		return Token{}, false
	}
	return t, true
}

// List 回傳一次發布收到的賣家回價，每位賣家只列目前版本（撤回的目前版本照列、標狀態），依回價身分列的 (created_at DESC, 回價 ID DESC) 分頁（身分列不可變，賣家改版不影響翻頁）。
// pageSize 為 0 用預設，超過上限以上限計，負數回 INVALID_ARGUMENT（page_size）；pageToken 不是這個方法發出的 token 回 INVALID_ARGUMENT（page_token）。
// 發布不存在、不屬於這位買家或不屬於這張需求回 NOT_FOUND。
func (s Service) List(ctx context.Context, in ListInput) (model.SellerResponsePage, error) {
	pageSize := in.PageSize
	switch {
	case pageSize < 0:
		return model.SellerResponsePage{}, apperr.InvalidInputOn([]string{"page_size"}, "page_size 不能是負數")
	case pageSize == 0:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}
	params := store.ListCurrentSellerResponsesParams{RequestID: in.RequestID, PublicationID: in.PublicationID, BuyerID: &in.BuyerID, PageSize: pageSize + 1}
	if in.PageToken != "" {
		t, ok := DecodeToken(TokenSellerResponses, in.PageToken)
		if !ok {
			return model.SellerResponsePage{}, apperr.InvalidInputOn([]string{"page_token"}, "page_token 不是有效的分頁 token")
		}
		after, id := time.UnixMicro(t.CreatedAt).UTC(), t.ID
		params.AfterCreatedAt, params.AfterID = &after, &id
	}
	var out model.SellerResponsePage
	err := s.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		buyer := in.BuyerID
		pub, err := q.GetPublication(ctx, store.GetPublicationParams{ID: in.PublicationID, BuyerID: &buyer})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && pub.RequestID != in.RequestID) {
			return apperr.NotFound("找不到這次發布")
		}
		if err != nil {
			return fmt.Errorf("quote：讀取發布：%w", err)
		}
		approval, err := q.GetApproval(ctx, store.GetApprovalParams{ID: pub.ApprovalID, RequestID: in.RequestID, BuyerID: &buyer})
		if err != nil {
			return fmt.Errorf("quote：讀取發布 %s 的核准：%w", pub.ID, err)
		}
		rev, err := q.GetRevision(ctx, store.GetRevisionParams{ID: approval.RequestRevisionID, RequestID: in.RequestID, BuyerID: &buyer})
		if err != nil {
			return fmt.Errorf("quote：讀取核准 %s 的需求版本：%w", approval.ID, err)
		}
		basisQty, basisUnit := int32(0), ""
		if rev.Unit != nil {
			basisQty, basisUnit = rfq.Basis(*rev.Unit)
		}
		_, liveErr := q.GetLiveAcceptance(ctx, store.GetLiveAcceptanceParams{RequestID: in.RequestID, BuyerID: &buyer})
		if liveErr != nil && !errors.Is(liveErr, pgx.ErrNoRows) {
			return fmt.Errorf("quote：讀取進行中的成交：%w", liveErr)
		}
		rows, err := q.ListCurrentSellerResponses(ctx, params)
		if err != nil {
			return fmt.Errorf("quote：列出賣家回價：%w", err)
		}
		more := len(rows) > int(pageSize)
		if more {
			rows = rows[:pageSize]
		}
		now := s.Now()
		for _, r := range rows {
			v, resp := r.ProcurementSellerResponseVersion, r.ProcurementSellerResponse
			qt, err := QuoteOf(v)
			if err != nil {
				return err
			}
			hash, err := PayloadHash(qt)
			if err != nil {
				return err
			}
			status := VersionStatus(v.Version, resp.LatestVersion, resp.WithdrawnVersion, v.ValidUntil, now)
			out.Responses = append(out.Responses, model.BuyerSellerResponse{
				ID: resp.ID, Version: v.Version, Status: status, PayloadHash: hash, SubmittedAt: v.SubmittedAt, StoreID: resp.StoreID, Quote: qt,
				OutsideBuyerRequestedRange: OutsideRange(approval.RangeLowerAmount, approval.RangeUpperAmount, basisQty, basisUnit, v),
				Shortlisted:                r.Shortlisted,
				Acceptability:              Acceptability(status, resp.WriteClosedAt != nil, liveErr == nil),
			})
		}
		if more {
			last := rows[len(rows)-1]
			out.NextPageToken = EncodeToken(TokenSellerResponses, last.ProcurementSellerResponse.CreatedAt, last.ProcurementSellerResponse.ID)
		}
		return nil
	})
	return out, err
}
