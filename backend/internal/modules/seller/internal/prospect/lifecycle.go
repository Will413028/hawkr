package prospect

import (
	"context"
	"errors"

	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// Archive 封存業者（只有 ACTIVE 的可以）：版本要與讀到的相符（不符回 VersionConflict，先於狀態檢查），讓出統編的唯一性與被比對（聯絡方式留著），
// 圖片欄位清成 NULL，並在同一個 transaction 把清掉的圖記成退場、排入引用的同步。已經封存、而且帶目前版本的封存回傳目前的業者、不寫入；
// 不推斷重送：封存生效後帶舊版本再封存回 VersionConflict。已轉成店家的回 FailedPrecondition。
func (s *Service) Archive(ctx context.Context, in model.ArchiveProspectInput) (model.Prospect, error) {
	if err := model.Positive(in.ProspectID, in.AdminID); err != nil {
		return model.Prospect{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Prospect{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	var out model.Prospect
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := lockOne(ctx, q, in.ProspectID)
		if err != nil {
			return err
		}
		if row.Version != in.ExpectedVersion {
			return errVersionConflict()
		}
		if row.Status == string(model.ProspectStatusConverted) {
			return apperr.FailedPrecondition("已轉成店家的業者不能封存")
		}
		if row.Status == string(model.ProspectStatusActive) {
			locked := row
			if row, err = q.ArchiveProspect(ctx, store.ArchiveProspectParams{ProspectID: row.ProspectID, AdminID: &in.AdminID, Now: s.d.Now().UTC()}); err != nil {
				return err
			}
			if err := retireImages(ctx, q, s.d.Jobs, s.d.Now().UTC(), locked); err != nil {
				return err
			}
			if err := events.Publish(ctx, s.d.Events, model.ProspectArchived{
				EventID: uuid.NewV7(), OccurredAt: s.d.Now().UTC(), ProspectID: row.ProspectID,
			}); err != nil {
				return err
			}
		}
		out, err = load(ctx, q, row)
		return err
	})
	if err != nil {
		return model.Prospect{}, conflictError(err)
	}
	return out, nil
}

// Restore 還原封存的業者（不帶版本：依據的是封存這個狀態，不是內容）：被合併的不可還原（FailedPrecondition）；封存期間統編被別的
// 未封存業者拿走回 FailedPrecondition（reason PROSPECT_TAX_ID_HELD，是同一家的話不需要還原）；聯絡方式的比對鍵被其他未封存業者佔走、而且沒有標記為不是同一家時回
// PROSPECT_MATCH_FOUND，ConfirmedDistinct 涵蓋全部相符的業者時才還原並記下配對。已經是 ACTIVE 的重送回傳目前的業者。圖片在
// 封存時已釋放，還原後沒有圖片。
func (s *Service) Restore(ctx context.Context, in model.RestoreProspectInput) (model.Prospect, error) {
	if err := model.Positive(in.ProspectID, in.AdminID); err != nil {
		return model.Prospect{}, err
	}
	if err := model.Positive(in.ConfirmedDistinct...); err != nil {
		return model.Prospect{}, err
	}
	var out model.Prospect
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := lockOne(ctx, q, in.ProspectID)
		if err != nil {
			return err
		}
		switch model.ProspectStatus(row.Status) {
		case model.ProspectStatusConverted:
			return apperr.FailedPrecondition("已轉成店家的業者不需要還原")
		case model.ProspectStatusActive:
			// 重送：已經是進行中，回傳目前的業者。
		case model.ProspectStatusArchived:
			if row.MergedInto != nil {
				return apperr.FailedPrecondition("已被合併的業者不可還原")
			}
			if holder, held, err := taxIDHolder(ctx, q, row.ProspectID, row.TaxID); err != nil {
				return err
			} else if held {
				return errTaxIDHeld(holder, opRestore)
			}
			contacts, err := loadContacts(ctx, q, row.ProspectID)
			if err != nil {
				return err
			}
			confirmed, err := resolveMatches(ctx, q, row.ProspectID, model.ContactMatchKeys(contacts), in.ConfirmedDistinct)
			if err != nil {
				return err
			}
			now := s.d.Now().UTC()
			if row, err = q.RestoreProspect(ctx, store.RestoreProspectParams{ProspectID: row.ProspectID, AdminID: &in.AdminID, Now: now}); err != nil {
				return err
			}
			if err := confirmDistinct(ctx, q, row.ProspectID, in.AdminID, now, confirmed); err != nil {
				return err
			}
		}
		out, err = load(ctx, q, row)
		return err
	})
	if err != nil {
		return model.Prospect{}, err
	}
	return out, nil
}

// MarkDistinct 標記一對業者不是同一家（疑似重複清單的候選，ADR 2026-10-04-prospect-contact-points-and-matching D4 的修訂）：
// 兩個 ID 不可相同；兩個都必須存在而且是進行中（ACTIVE，疑似重複清單的候選只在進行中的業者之間），否則找不到回 NotFound、
// 狀態不對回 FailedPrecondition。不帶版本。兩列依 ID 順序鎖。已經標記的重送回傳成功、不改寫第一次的管理員與時間；標記之後這一對
// 不再出現在清單，也不再擋建立、修改與還原。
func (s *Service) MarkDistinct(ctx context.Context, in model.MarkProspectsDistinctInput) error {
	if err := model.Positive(in.FirstID, in.SecondID, in.AdminID); err != nil {
		return err
	}
	if in.FirstID == in.SecondID {
		return apperr.InvalidInputOn([]string{"SecondID"}, "不能把業者標記成與自己不是同一家")
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.LockProspects(ctx, []int64{in.FirstID, in.SecondID})
		if err != nil {
			return err
		}
		if len(rows) != 2 {
			return errNotFound()
		}
		for _, row := range rows {
			if model.ProspectStatus(row.Status) != model.ProspectStatusActive {
				return apperr.FailedPrecondition("只有進行中的業者可以標記，業者 %d 不是進行中", row.ProspectID)
			}
		}
		return q.MarkDistinctPair(ctx, store.MarkDistinctPairParams{FirstID: in.FirstID, SecondID: in.SecondID, AdminID: in.AdminID, Now: s.d.Now().UTC()})
	})
}

// Merge 把輸家併進贏家（兩個都必須是 ACTIVE，兩邊的版本都要與讀到的相符，不符回 VersionConflict，先於狀態檢查；不推斷重送：合併生效後帶舊版本再合併一樣回 VersionConflict）：輸家的聯絡方式併入贏家
// （同一個比對鍵、或同類型同一個值的只留一列，贏家的主要聯絡方式不變；贏家在某個類型沒有聯絡方式時，輸家那個類型的主要的成為
// 主要的）、輸家的「不是同一家」配對改掛贏家（變成自己對自己或重複的刪除；明確合併一對標記為不是同一家的業者就是撤銷那個標記）、
// 分類補進贏家、已被輸家吸收的列改指贏家（merged_into 的鏈永遠只有一層）、統編只在贏家沒有時搬過去，輸家封存並記下 merged_into，
// 輸家的圖片在同一個 transaction 排入釋放。贏家的版本加一。回傳贏家。
func (s *Service) Merge(ctx context.Context, in model.MergeProspectsInput) (model.Prospect, error) {
	if err := model.Positive(in.WinnerID, in.LoserID, in.AdminID); err != nil {
		return model.Prospect{}, err
	}
	if in.WinnerID == in.LoserID {
		return model.Prospect{}, apperr.InvalidInputOn([]string{"LoserID"}, "不能把業者合併進自己")
	}
	if in.WinnerVersion <= 0 {
		return model.Prospect{}, apperr.InvalidInputOn([]string{"WinnerVersion"}, "必須提供贏家讀取時的版本")
	}
	if in.LoserVersion <= 0 {
		return model.Prospect{}, apperr.InvalidInputOn([]string{"LoserVersion"}, "必須提供輸家讀取時的版本")
	}
	var out model.Prospect
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.LockProspects(ctx, []int64{in.WinnerID, in.LoserID})
		if err != nil {
			return err
		}
		if len(rows) != 2 {
			return errNotFound()
		}
		winner, loser := rows[0], rows[1]
		if winner.ProspectID != in.WinnerID {
			winner, loser = loser, winner
		}
		if winner.Version != in.WinnerVersion || loser.Version != in.LoserVersion {
			return errVersionConflict()
		}
		for _, row := range rows {
			if row.Status != string(model.ProspectStatusActive) {
				return apperr.FailedPrecondition("業者 %d 不是進行中，只有進行中的業者可以合併", row.ProspectID)
			}
		}
		now := s.d.Now().UTC()
		if err := q.UnionProspectContacts(ctx, store.UnionProspectContactsParams{WinnerID: winner.ProspectID, LoserID: loser.ProspectID}); err != nil {
			return err
		}
		if err := q.EnsureProspectPrimaries(ctx, winner.ProspectID); err != nil {
			return err
		}
		if err := q.ClearProspectContacts(ctx, loser.ProspectID); err != nil {
			return err
		}
		if err := q.RepointDistinctPairs(ctx, store.RepointDistinctPairsParams{WinnerID: winner.ProspectID, LoserID: loser.ProspectID}); err != nil {
			return err
		}
		if err := q.DeleteDistinctPairsOf(ctx, loser.ProspectID); err != nil {
			return err
		}
		if err := q.CopyProspectCategories(ctx, store.CopyProspectCategoriesParams{WinnerID: winner.ProspectID, LoserID: loser.ProspectID}); err != nil {
			return err
		}
		if err := q.ClearProspectCategories(ctx, loser.ProspectID); err != nil {
			return err
		}
		if err := q.RepointMergedProspects(ctx, store.RepointMergedProspectsParams{WinnerID: &winner.ProspectID, LoserID: &loser.ProspectID}); err != nil {
			return err
		}
		// 輸家的統編在封存時清掉（兩列都 live，同一個號碼不能並存），贏家沒有統編才收下。
		if _, err := q.ArchiveMergedLoser(ctx, store.ArchiveMergedLoserParams{
			WinnerID: &winner.ProspectID, LoserID: loser.ProspectID, AdminID: &in.AdminID, Now: now,
		}); err != nil {
			return err
		}
		if err := retireImages(ctx, q, s.d.Jobs, now, loser); err != nil {
			return err
		}
		// 輸家的封存只由 ProspectMerged 表達（不另發 ProspectStatusArchived）。
		if err := events.Publish(ctx, s.d.Events, model.ProspectMerged{
			EventID: uuid.NewV7(), OccurredAt: now, SourceProspectID: loser.ProspectID, TargetProspectID: winner.ProspectID,
		}); err != nil {
			return err
		}
		var merged store.SellerProspect
		if loser.TaxID != nil && winner.TaxID == nil {
			merged, err = q.CarryProspectTaxID(ctx, store.CarryProspectTaxIDParams{TaxID: loser.TaxID, WinnerID: winner.ProspectID, AdminID: &in.AdminID, Now: now})
		} else {
			merged, err = q.TouchProspect(ctx, store.TouchProspectParams{ProspectID: winner.ProspectID, AdminID: &in.AdminID, Now: now})
		}
		if err != nil {
			return err
		}
		out, err = load(ctx, q, merged)
		return err
	})
	if err != nil {
		return model.Prospect{}, conflictError(err)
	}
	return out, nil
}

// Convert 把業者手動轉成店家（admin 指定 store_id）：店家必須是目前的 stint；只有 ACTIVE 的業者可以轉，轉過的（二次轉換）、
// 封存的與被合併的回 FailedPrecondition；版本要與讀到的相符（不符回 VersionConflict，先於狀態檢查，不推斷重送）；同一個店家只能被一個業者認領（AlreadyExists）。
func (s *Service) Convert(ctx context.Context, in model.ConvertProspectInput) (model.Prospect, error) {
	if err := model.Positive(in.ProspectID, in.StoreID, in.AdminID); err != nil {
		return model.Prospect{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Prospect{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	var out model.Prospect
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := lockOne(ctx, q, in.ProspectID)
		if err != nil {
			return err
		}
		if row.Version != in.ExpectedVersion {
			return errVersionConflict()
		}
		switch model.ProspectStatus(row.Status) {
		case model.ProspectStatusConverted:
			return apperr.FailedPrecondition("業者已經轉成店家")
		case model.ProspectStatusArchived:
			return apperr.FailedPrecondition("已封存或被合併的業者不能轉成店家")
		case model.ProspectStatusActive:
		}
		exists, err := q.CurrentStoreExists(ctx, in.StoreID)
		if err != nil {
			return err
		}
		if !exists {
			return apperr.NotFound("店家不存在或已關閉")
		}
		now := s.d.Now().UTC()
		converted, err := q.ConvertProspect(ctx, store.ConvertProspectParams{ProspectID: row.ProspectID, StoreID: &in.StoreID, Now: &now, AdminID: &in.AdminID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.FailedPrecondition("業者已經不是進行中")
		}
		if err != nil {
			return err
		}
		out, err = load(ctx, q, converted)
		return err
	})
	if err != nil {
		return model.Prospect{}, conflictError(err)
	}
	return out, nil
}
