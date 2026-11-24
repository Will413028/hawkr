package sellerside

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// TargetDraft 是回價草稿過時的 conflictTarget。
const TargetDraft = "draft"

// ParseVersion 解析版本字串（伺服器發的版本是十進位正整數）；空字串回 0（沒有版本）。格式不對回 INVALID_ARGUMENT，指向 field。
func ParseVersion(field, raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v <= 0 {
		return 0, apperr.InvalidInputOn([]string{field}, "%s 不是有效的版本", field)
	}
	return int32(v), nil
}

// UpsertDraft 建立或更新這家店對這筆詢價的回價草稿（整份覆寫）。草稿只驗形狀與長度（NormalizeDraft），不驗必填欄位與金額恆等式，買家讀不到。
// expectedDraftVersion 是空字串表示建立第一版，草稿已存在卻帶空、或帶的版本不是目前的回 ABORTED／VERSION_CONFLICT（conflictTarget 是 draft）；版本格式不對回 INVALID_ARGUMENT（expected_draft_version）。
// 詢價不是發給這家店的回 NOT_FOUND；停止寫入（見套件說明）回 FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED。
func (s Service) UpsertDraft(ctx context.Context, storeID int64, publicationID uuid.UUID, expectedDraftVersion string, in model.QuoteInput) (model.SellerDraft, error) {
	if err := s.check(); err != nil {
		return model.SellerDraft{}, err
	}
	expected, err := ParseVersion("expected_draft_version", expectedDraftVersion)
	if err != nil {
		return model.SellerDraft{}, err
	}
	normalized, payload, err := NormalizeDraft(in)
	if err != nil {
		return model.SellerDraft{}, err
	}
	var out model.SellerDraft
	err = s.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.guardWrite(ctx, storeID); err != nil {
			return err
		}
		q := store.New(s.DB.Conn(ctx))
		v, err := loadView(ctx, q, storeID, publicationID)
		if err != nil {
			return err
		}
		resp, err := s.ensureAndLock(ctx, q, storeID, publicationID)
		if err != nil {
			return err
		}
		if err := checkWritable(ctx, q, resp, v, s.Now()); err != nil {
			return err
		}
		var row store.ProcurementSellerResponseDraft
		if expected == 0 {
			row, err = q.InsertSellerDraft(ctx, store.InsertSellerDraftParams{SellerResponseID: resp.ID, Payload: payload, Now: s.Now()})
		} else {
			row, err = q.UpdateSellerDraft(ctx, store.UpdateSellerDraftParams{SellerResponseID: resp.ID, Payload: payload, ExpectedVersion: expected, Now: s.Now()})
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return quote.VersionConflict("草稿已經被更新過，請重新載入", TargetDraft)
		}
		if err != nil {
			return fmt.Errorf("sellerside：寫入草稿：%w", err)
		}
		out = model.SellerDraft{Version: row.Version, UpdatedAt: row.UpdatedAt, Quote: normalized}
		return nil
	})
	return out, err
}
