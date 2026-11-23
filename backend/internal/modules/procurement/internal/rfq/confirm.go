package rfq

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// ConfirmInput 是 ConfirmRfqSnapshot 的輸入：PayloadHash 是價格預覽回應的 hash，BuyerNote 是買家的補充說明。
type ConfirmInput struct {
	BuyerID                                                        int64
	RequestID, ClientRequestID, ExpectedRevisionID, PricePreviewID uuid.UUID
	PayloadHash, BuyerNote                                         string
}

// ConfirmSnapshot 對一份價格預覽產生不可變的確認快照（SnapshotTTL 有效）。
// buyer_note 去頭尾空白後超過 MaxBuyerNoteRunes 回 INVALID_ARGUMENT（欄位 BuyerNote）；需求不屬於買家、預覽不屬於買家回 NOT_FOUND；
// 讀到的版本不是現行版本回 ABORTED／VERSION_CONFLICT（request_revision）；預覽屬於別的需求、對應的版本不是現行版本、已過期、PayloadHash 與預覽不符回 ABORTED／VERSION_CONFLICT（price_preview）。
// 重送同一個 ClientRequestID 回第一次的快照（先於版本與預覽檢查），內容不同回 INVALID_ARGUMENT（欄位 client_request_id）。
func (s Service) ConfirmSnapshot(ctx context.Context, in ConfirmInput) (model.ConfirmedRfqSnapshot, error) {
	note := trimtext.Trim(in.BuyerNote)
	if utf8.RuneCountInString(note) > MaxBuyerNoteRunes {
		return model.ConfirmedRfqSnapshot{}, apperr.InvalidInputOn([]string{"BuyerNote"}, "補充說明最多 %d 個字", MaxBuyerNoteRunes)
	}
	var out model.ConfirmedRfqSnapshot
	err := s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		var err error
		if out, err = replaySnapshot(ctx, q, in, note); err != nil || out.ID != (uuid.UUID{}) {
			return err
		}
		_, rev, err := lockRequest(ctx, q, in.BuyerID, in.RequestID, in.ExpectedRevisionID)
		if err != nil {
			return err
		}
		prev, err := q.GetPricePreviewForBuyer(ctx, store.GetPricePreviewForBuyerParams{ID: in.PricePreviewID, BuyerID: &in.BuyerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這份價格預覽")
		}
		if err != nil {
			return fmt.Errorf("rfq：讀取價格預覽：%w", err)
		}
		now := s.Now()
		hash, err := PreviewHash(prev)
		if err != nil {
			return err
		}
		if prev.RequestID != in.RequestID || prev.RequestRevisionID != rev.ID || !now.Before(prev.ExpiresAt) || hash != in.PayloadHash {
			return versionConflict("價格預覽已過時，請重新試算", TargetPricePreview)
		}
		row := store.InsertConfirmedSnapshotParams{
			ID: uuid.NewV7(), BuyerID: &in.BuyerID, RequestID: in.RequestID, RequestRevisionID: rev.ID, PricePreviewID: prev.ID, BuyerNote: note,
			ClientRequestID: in.ClientRequestID, Now: now, ExpiresAt: now.Add(SnapshotTTL),
		}
		n, err := q.InsertConfirmedSnapshot(ctx, row)
		if err != nil {
			return fmt.Errorf("rfq：寫入確認快照：%w", err)
		}
		if n == 0 { // 同一個鍵的另一個請求搶先寫入並已提交：以它為準
			if out, err = replaySnapshot(ctx, q, in, note); err != nil || out.ID != (uuid.UUID{}) {
				return err
			}
			return fmt.Errorf("rfq：確認快照的冪等鍵衝突卻讀不到列")
		}
		snap, err := q.GetConfirmedSnapshot(ctx, store.GetConfirmedSnapshotParams{ID: row.ID, RequestID: in.RequestID, BuyerID: &in.BuyerID})
		if err != nil {
			return fmt.Errorf("rfq：讀回確認快照：%w", err)
		}
		out, err = snapshotOf(snap, rev, prev)
		return err
	})
	return out, err
}

// replaySnapshot 以冪等鍵找第一次的快照：沒有回零值；有但內容（需求、預覽、讀到的版本、補充說明）不同回 INVALID_ARGUMENT。
func replaySnapshot(ctx context.Context, q *store.Queries, in ConfirmInput, note string) (model.ConfirmedRfqSnapshot, error) {
	snap, err := q.GetConfirmedSnapshotByClientRequest(ctx, store.GetConfirmedSnapshotByClientRequestParams{BuyerID: &in.BuyerID, ClientRequestID: in.ClientRequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ConfirmedRfqSnapshot{}, nil
	}
	if err != nil {
		return model.ConfirmedRfqSnapshot{}, fmt.Errorf("rfq：讀取確認快照的冪等鍵：%w", err)
	}
	if snap.RequestID != in.RequestID || snap.PricePreviewID != in.PricePreviewID || snap.RequestRevisionID != in.ExpectedRevisionID || snap.BuyerNote != note {
		return model.ConfirmedRfqSnapshot{}, idempotencyMismatch()
	}
	owner := in.BuyerID
	rev, err := q.GetRevision(ctx, store.GetRevisionParams{ID: snap.RequestRevisionID, RequestID: snap.RequestID, BuyerID: &owner})
	if err != nil {
		return model.ConfirmedRfqSnapshot{}, fmt.Errorf("rfq：讀取快照的需求版本：%w", err)
	}
	prev, err := q.GetPricePreview(ctx, store.GetPricePreviewParams{ID: snap.PricePreviewID, RequestID: snap.RequestID, BuyerID: &owner})
	if err != nil {
		return model.ConfirmedRfqSnapshot{}, fmt.Errorf("rfq：讀取快照的價格預覽：%w", err)
	}
	return snapshotOf(snap, rev, prev)
}

func snapshotOf(snap store.ProcurementConfirmedRfqSnapshot, rev store.ProcurementRequestRevision, prev store.ProcurementPricePreview) (model.ConfirmedRfqSnapshot, error) {
	hash, err := SnapshotHash(snap, rev, prev, rfqhash.SchemaV1)
	if err != nil {
		return model.ConfirmedRfqSnapshot{}, err
	}
	return model.ConfirmedRfqSnapshot{ID: snap.ID, PayloadHash: hash, ExpiresAt: snap.ExpiresAt}, nil
}
