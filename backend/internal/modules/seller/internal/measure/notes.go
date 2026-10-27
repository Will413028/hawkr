package measure

import (
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// CreateNote 新增措施的一則內部註記。註記只新增、不帶版本，也不動 measures（version 與 current_revision 都不變），所以賣家與
// 後台依舊 etag 的寫入都不受影響；措施已撤銷或已申覆都可以寫。措施不存在由外鍵擋下，回 NotFound。
func (s *Service) CreateNote(ctx context.Context, in model.CreateMeasureNoteInput) (model.GovernanceNote, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.GovernanceNote{}, err
	}
	body, err := model.NoteBody(in.Body, "Body")
	if err != nil {
		return model.GovernanceNote{}, err
	}
	row, err := store.New(s.d.DB.Conn(ctx)).InsertGovernanceNote(ctx, store.InsertGovernanceNoteParams{
		NoteID: uuid.NewV7(), MeasureID: &in.MeasureID, AdminID: in.AdminID, Body: body, Now: s.now(),
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" && pg.ConstraintName == "governance_notes_measure_id_fkey" {
		return model.GovernanceNote{}, errMeasureNotFound()
	}
	if err != nil {
		return model.GovernanceNote{}, err
	}
	// 回應取資料庫存下的值（微秒），之後 Notes 讀到的才會完全相同
	return model.GovernanceNote{ID: row.NoteID, AdminID: row.AdminID, Body: row.Body, CreatedAt: row.CreatedAt}, nil
}

// Notes 是措施的內部註記，舊到新（keyset）；措施不存在回 NotFound。多讀一筆判斷有沒有下一頁。
func (s *Service) Notes(ctx context.Context, in model.MeasureNoteListInput) (model.NotePage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.NotePage{}, err
	}
	params := store.MeasureNotesParams{MeasureID: &in.MeasureID, PageSize: size + 1}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
	}
	var page model.NotePage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.MeasureByID(ctx, in.MeasureID); errors.Is(err, pgx.ErrNoRows) {
			return errMeasureNotFound()
		} else if err != nil {
			return err
		}
		rows, err := q.MeasureNotes(ctx, params)
		if err != nil {
			return err
		}
		page.Items = make([]model.GovernanceNote, len(rows))
		for i, n := range rows {
			page.Items[i] = model.GovernanceNote{ID: n.NoteID, AdminID: n.AdminID, Body: n.Body, CreatedAt: n.CreatedAt}
		}
		return nil
	})
	if err != nil {
		return model.NotePage{}, err
	}
	if len(page.Items) > int(size) {
		page.Items = page.Items[:size]
		last := page.Items[size-1]
		page.Next = &model.NoteCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}
