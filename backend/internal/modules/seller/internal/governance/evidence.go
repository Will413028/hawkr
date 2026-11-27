package governance

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func errEvidenceNotFound() error { return apperr.NotFound("找不到這張佐證") }

// ReadEvidence 讀一張佐證（後台）。授權是呼叫方的事（後台的權限，見 httpapi/adminauth）；這裡只要求管理員 ID，沒有理由或
// 限時授權（看佐證是審案的本體）。通報或請求不存在、位置沒有佐證是 NotFound。
// 順序是安全性質（與 KYC 證件的讀取相同）：（1）transaction 內寫一列 append-only 的讀取紀錄（誰、哪一張、何時；以外鍵指向
// 那一張佐證，佐證寫入後不可改，所以就是送出的檔；不含內容），commit 之後才（2）向 media 讀內容。紀錄寫不進去就不讀（fail closed）；讀取失敗時紀錄仍在（嘗試也是紀錄）。
// media 讀不到時的結果見 model.ReadSecure。
func (s *Service) ReadEvidence(ctx context.Context, in model.ReadEvidenceInput) (model.FileContent, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.FileContent{}, err
	}
	limit := model.MaxEvidence
	if in.Kind == model.EvidenceReport {
		limit = model.MaxReportEvidence
	}
	if in.Position < 0 || in.Position >= limit {
		return model.FileContent{}, errEvidenceNotFound()
	}
	var evidence model.Evidence
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		found, err := evidenceAt(ctx, q, in.Kind, in.RefID, in.Position)
		if err != nil {
			return err
		}
		evidence = found.Evidence
		if in.Kind == model.EvidenceInquiry {
			return q.InsertInquiryEvidenceAccess(ctx, store.InsertInquiryEvidenceAccessParams{
				AccessID: uuid.NewV7(), InquiryID: &in.RefID, Position: found.rowPosition, AdminID: in.AdminID, Now: s.d.Now().UTC(),
			})
		}
		access := store.InsertEvidenceAccessParams{
			AccessID: uuid.NewV7(), Position: found.rowPosition, AdminID: in.AdminID, Now: s.d.Now().UTC(),
		}
		if in.Kind == model.EvidenceReport {
			access.ReportID, access.SubmissionSeq = &in.RefID, &found.submissionSeq
		} else {
			access.RequestID = &in.RefID
		}
		return q.InsertEvidenceAccess(ctx, access)
	})
	if err != nil {
		return model.FileContent{}, err
	}
	return model.ReadSecure(ctx, s.d.Media, evidence.AssetID)
}

type evidenceRow struct {
	model.Evidence
	submissionSeq int16
	rowPosition   int32
}

func evidenceAt(ctx context.Context, q *store.Queries, kind model.EvidenceKind, refID uuid.UUID, n int) (evidenceRow, error) {
	switch kind {
	case model.EvidenceReport:
		rows, err := q.ReportEvidence(ctx, refID)
		if err != nil {
			return evidenceRow{}, err
		}
		if n < len(rows) {
			r := rows[n]
			return evidenceRow{Evidence: model.Evidence{AssetID: r.AssetID, ObjectKey: r.ObjectKey}, submissionSeq: r.SubmissionSeq, rowPosition: r.Position}, nil
		}
	case model.EvidenceRequest:
		rows, err := q.RequestEvidence(ctx, refID)
		if err != nil {
			return evidenceRow{}, err
		}
		for _, r := range rows {
			if int(r.Position) == n {
				return evidenceRow{Evidence: model.Evidence{AssetID: r.AssetID, ObjectKey: r.ObjectKey}, rowPosition: r.Position}, nil
			}
		}
	case model.EvidenceInquiry:
		rows, err := q.CaseInquiryEvidence(ctx, []uuid.UUID{refID})
		if err != nil {
			return evidenceRow{}, err
		}
		for _, r := range rows {
			if int(r.Position) == n {
				return evidenceRow{Evidence: model.Evidence{AssetID: r.AssetID, ObjectKey: r.ObjectKey}, rowPosition: r.Position}, nil
			}
		}
	}
	return evidenceRow{}, errEvidenceNotFound()
}
