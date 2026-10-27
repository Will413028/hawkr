package governance

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// MarkSupplementsSeen 記下管理員看過這筆通報此刻的全部補充（通報頁顯示補充時呼叫；GetReport 本身不改狀態）。
// 補充讓通報的版本前進，所以 ExpectedVersion 相符就代表看過的是目前全部的補充；不符回版本衝突、不記（那則新補充還沒被看到）。
// **不讓版本前進**：已讀標記不是對通報的處理，不該讓別的分頁或對話框讀到的 etag 過時。冪等：已經看過、沒有補充或已有結果的通報不寫入，照樣回成功。
// 只鎖通報列（不改案件、不碰店家列，鎖順序是店家列 → 通報列 → 案件列的前綴子集）。
func (s *Service) MarkSupplementsSeen(ctx context.Context, in model.MarkSupplementsSeenInput) (model.AdminReport, error) {
	var result model.AdminReport
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := lockReport(ctx, q, in.ReportID, in.ExpectedVersion); err != nil {
			return err
		}
		_, err := q.MarkReportSupplementsSeen(ctx, in.ReportID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		result, err = readReport(ctx, q, in.ReportID, s.now())
		return err
	})
	return result, err
}
