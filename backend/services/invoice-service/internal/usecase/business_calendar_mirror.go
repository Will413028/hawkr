package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// ErrInvalidBusinessDayMirror 表示推送的涵蓋區間或假日清單不合：整批拒絕、鏡像不動。
var ErrInvalidBusinessDayMirror = errors.New("invoice: invalid business day mirror batch")

const isoDateLayout = "2006-01-02"

// BusinessCalendarMirror 是行事曆鏡像的整段覆蓋（§5.1.14）。
//
// 資料的擁有者是單體的 platform/bizcalendar，它在每次匯入成功後與每天一次的對帳推送「涵蓋區間＋區間內的假日」。
// 這支 usecase 隨 A5（invoice 搬進單體）刪除。
type BusinessCalendarMirror struct {
	repo domain.BusinessDayRepository
	tx   database.TxManager
	log  *zap.Logger
	now  func() time.Time
}

// NewBusinessCalendarMirror 建立 usecase。
func NewBusinessCalendarMirror(repo domain.BusinessDayRepository, tx database.TxManager, log *zap.Logger, now func() time.Time) *BusinessCalendarMirror {
	return &BusinessCalendarMirror{repo: repo, tx: tx, log: orNop(log), now: now}
}

// Replace 驗證後在**同一個交易**內刪除全部鏡像列、寫入假日並覆蓋涵蓋區間，回傳寫入的假日天數（去重後）。
//
// 驗證：兩端都是 YYYY-MM-DD 的真實日期、from <= to、每個假日都落在 [from, to] 內；任何一個解析失敗或越界
// 就整批拒絕（ErrInvalidBusinessDayMirror）——少一天假日會讓 SLA 算偏早，所以不跳過壞的那一筆。
// 失敗（含交易失敗）一律保留舊鏡像。
func (u *BusinessCalendarMirror) Replace(ctx context.Context, coveredFrom, coveredTo string, holidayDates []string) (int, error) {
	from, err := time.Parse(isoDateLayout, coveredFrom)
	if err != nil {
		return 0, fmt.Errorf("%w: covered_from %q", ErrInvalidBusinessDayMirror, coveredFrom)
	}
	to, err := time.Parse(isoDateLayout, coveredTo)
	if err != nil {
		return 0, fmt.Errorf("%w: covered_to %q", ErrInvalidBusinessDayMirror, coveredTo)
	}
	if from.After(to) {
		return 0, fmt.Errorf("%w: covered_from %s is after covered_to %s", ErrInvalidBusinessDayMirror, coveredFrom, coveredTo)
	}
	seen := make(map[string]struct{}, len(holidayDates))
	holidays := make([]time.Time, 0, len(holidayDates))
	for _, raw := range holidayDates {
		d, perr := time.Parse(isoDateLayout, raw)
		if perr != nil {
			return 0, fmt.Errorf("%w: holiday date %q", ErrInvalidBusinessDayMirror, raw)
		}
		if d.Before(from) || d.After(to) {
			return 0, fmt.Errorf("%w: holiday date %s is outside [%s, %s]", ErrInvalidBusinessDayMirror, raw, coveredFrom, coveredTo)
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		seen[raw] = struct{}{}
		holidays = append(holidays, d)
	}

	now := u.now()
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		if _, dErr := u.repo.DeleteAll(ctx); dErr != nil {
			return dErr
		}
		for _, d := range holidays {
			if iErr := u.repo.Insert(ctx, d, now); iErr != nil {
				return iErr
			}
		}
		return u.repo.SetCoverage(ctx, from, to, now)
	})
	if err != nil {
		u.log.Error("invoice.calendar.replace_failed", zap.Error(err), zap.Int("holidays", len(holidays)))
		return 0, err
	}
	u.log.Info("invoice.calendar.replaced",
		zap.Int("holidays", len(holidays)), zap.String("from", coveredFrom), zap.String("to", coveredTo))
	return len(holidays), nil
}
