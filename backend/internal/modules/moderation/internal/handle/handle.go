// Package handle 是案件的處理：管理員對一個進行中的案件下決定（違規加處置，或不違規），處置與結案在同一個 transaction（A7-D14、A7-D7 第 10 項）。
//
// 順序（單體規則 4，由下往上鎖列）：先讀案件與作者（不鎖）、再呼叫擁有處分的模組（social 的隱藏或警告、accountsuspension 的停權、seller 的內容下架措施，
// 每一次處理只走其中一條），moderation 自己的案件列最後才寫：結案 UPDATE 以 (id, version, closed_at IS NULL) 轉移，0 列就是已被結掉或版本過期，回
// VersionConflict，整個 transaction（含處置）回滾。案件結案是終態，不重開也不改判；更正在擁有處分的模組做。
package handle

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/view"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// SourceRefPrefix 是寫進各處分紀錄的來源參照的前綴，後面接案件 ID；其他模組把它當不透明的字串。
const SourceRefPrefix = "moderation:case:"

// Deps 是處理案件的依賴，全部必填。
type Deps struct {
	DB         *db.DB
	Social     *social.Module
	Seller     *seller.Module
	Buyer      *buyer.Module
	Suspension *accountsuspension.Module
	Now        func() time.Time
}

// Service 是案件處理的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Validate 檢查輸入的形狀並正規化理由（去頭尾空白）；不查資料庫。
func Validate(in model.DecideInput) (model.DecideInput, error) {
	switch {
	case in.CaseID == uuid.UUID{}:
		return in, apperr.InvalidInputOn([]string{"CaseID"}, "%s", text.DecideCaseRequired())
	case in.Version < 1:
		return in, apperr.InvalidInputOn([]string{"Version"}, "%s", text.DecideVersionRequired())
	case in.AdminID <= 0:
		return in, apperr.InvalidInputOn([]string{"AdminID"}, "%s", text.DecideAdminRequired())
	}
	in.Text = trimtext.Trim(in.Text)
	switch in.Decision {
	case model.DecisionNoViolation:
		if in.Category != "" || in.Action != "" || in.Text != "" || in.StoreID != 0 || in.Severity != "" {
			return in, apperr.InvalidInputOn([]string{"Decision"}, "%s", text.DecideNoViolationShape())
		}
	case model.DecisionViolation:
		if !slices.Contains(model.Actions(), in.Action) {
			return in, apperr.InvalidInputOn([]string{"Action"}, "%s", text.DecideActionInvalid())
		}
		if !in.Action.CategoryAllowed(in.Category) {
			return in, apperr.InvalidInputOn([]string{"Category"}, "%s", text.DecideCategoryNotForAction())
		}
		if in.Text == "" {
			return in, apperr.InvalidInputOn([]string{"Text"}, "%s", text.DecideReasonRequired())
		}
		if utf8.RuneCountInString(in.Text) > model.MaxTextRunes {
			return in, apperr.InvalidInputOn([]string{"Text"}, "%s", text.DecideReasonTooLong(text.DecideReasonTooLongArgs{Max: model.MaxTextRunes}))
		}
		if in.Action == model.ActionTakedown {
			if in.StoreID <= 0 {
				return in, apperr.InvalidInputOn([]string{"StoreID"}, "%s", text.DecideStoreRequired())
			}
			if !slices.Contains(seller.SeverityValues(), seller.Severity(in.Severity)) {
				return in, apperr.InvalidInputOn([]string{"Severity"}, "%s", text.DecideSeverityInvalid())
			}
		} else if in.StoreID != 0 || in.Severity != "" {
			return in, apperr.InvalidInputOn([]string{"Action"}, "%s", text.DecideStoreOnlyTakedown())
		}
	default:
		return in, apperr.InvalidInputOn([]string{"Decision"}, "%s", text.DecideDecisionInvalid())
	}
	return in, nil
}

// Decide 處理案件並結案。案件不存在回 NotFound；案件已結案或版本過期回 VersionConflict（處置整個回滾）；處置種類不適用於對象、作者種類
// 不符、內容不屬於指定的店家回 InvalidInput 或 FailedPrecondition，什麼都不寫；帳號擁有店家時停權回 ACCOUNT_HAS_STORE。
func (s *Service) Decide(ctx context.Context, in model.DecideInput) (model.CaseDetail, error) {
	in, err := Validate(in)
	if err != nil {
		return model.CaseDetail{}, err
	}
	var out model.CaseDetail
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := q.GetCase(ctx, in.CaseID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.CaseNotFound())
		}
		if err != nil {
			return fmt.Errorf("moderation：讀取案件 %s：%w", in.CaseID, err)
		}
		c := view.Case(row)
		if c.ClosedAt != nil {
			return apperr.VersionConflict("%s", text.CaseClosed())
		}
		if in.Decision == model.DecisionViolation {
			if !in.Action.AllowedOn(c.Target.Kind) {
				return apperr.InvalidInputOn([]string{"Action"}, "%s", text.DecideActionNotForTarget())
			}
			if err := s.apply(ctx, q, c, in); err != nil {
				return err
			}
		}
		if err := s.close(ctx, q, in); err != nil {
			return err
		}
		out, err = view.Detail(ctx, q, in.CaseID)
		return err
	})
	return out, err
}

// close 是最後一步：結案轉移。
func (s *Service) close(ctx context.Context, q *store.Queries, in model.DecideInput) error {
	now := s.d.Now().UTC()
	p := store.CloseCaseParams{
		ID: in.CaseID, Version: in.Version, Decision: ptr(string(in.Decision)), AdminID: &in.AdminID, ClosedAt: &now,
	}
	if in.Decision == model.DecisionViolation {
		p.DecisionCategory, p.DecisionAction = ptr(string(in.Category)), ptr(string(in.Action))
	}
	n, err := q.CloseCase(ctx, p)
	if err != nil {
		return fmt.Errorf("moderation：結案 %s：%w", in.CaseID, err)
	}
	if n == 0 {
		return apperr.VersionConflict("%s", text.CaseStale())
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// apply 呼叫擁有處分的模組建立處分。
func (s *Service) apply(ctx context.Context, q *store.Queries, c model.Case, in model.DecideInput) error {
	ref := SourceRefPrefix + c.ID.String()
	author, owner, err := s.author(ctx, q, c)
	if err != nil {
		return err
	}
	switch in.Action {
	case model.ActionHide:
		if author.Kind != social.ActorBuyer {
			return apperr.FailedPrecondition("%s", text.DecideHideStoreContent())
		}
		if !owner.Exists {
			return nil
		}
		_, err := s.d.Social.Hide(ctx, social.HideInput{
			Target:   social.Target{Kind: social.TargetKind(c.Target.Kind), ID: c.Target.ContentID},
			Category: string(in.Category), Text: in.Text, SourceRef: ref, AdminID: in.AdminID,
		})
		return err
	case model.ActionWarn:
		if author.Kind != social.ActorBuyer {
			return apperr.FailedPrecondition("%s", text.DecideWarnNotBuyer())
		}
		_, err := s.d.Social.Warn(ctx, social.WarnInput{BuyerID: author.ID, Category: string(in.Category), Text: in.Text, SourceRef: ref, AdminID: in.AdminID})
		return err
	case model.ActionBan:
		if author.Kind != social.ActorBuyer {
			return apperr.FailedPrecondition("%s", text.DecideBanNotBuyer())
		}
		accountID, err := s.d.Buyer.AccountIDByBuyerID(ctx, author.ID)
		if err != nil {
			return err
		}
		_, err = s.d.Suspension.Suspend(ctx, accountID, accountsuspension.Record{Category: in.Category, Reason: in.Text, SourceRef: ref})
		return err
	case model.ActionTakedown:
		return s.takedown(ctx, q, c, owner, in)
	}
	return apperr.InvalidInputOn([]string{"Action"}, "%s", text.DecideActionInvalid())
}

// author 回傳對象的作者與歸屬：買家對象的作者就是那位買家；內容向 social 讀。內容已被刪除時（Exists 為假）改用案件快照裡凍結的作者與店家，
// 所以作者刪掉內容不能躲過警告、停權與內容下架的歸屬驗證；隱藏的目的（內容不可見）已達成，由呼叫端視為成功。
func (s *Service) author(ctx context.Context, q *store.Queries, c model.Case) (social.Actor, social.ContentOwner, error) {
	t := c.Target
	if t.Kind == model.TargetBuyer {
		return social.Actor{Kind: social.ActorBuyer, ID: t.BuyerID}, social.ContentOwner{}, nil
	}
	owner, err := s.d.Social.ContentOwner(ctx, social.Target{Kind: social.TargetKind(t.Kind), ID: t.ContentID})
	if err != nil {
		return social.Actor{}, social.ContentOwner{}, err
	}
	if owner.Exists {
		return owner.Author, owner, nil
	}
	body, err := q.LatestSnapshotContent(ctx, c.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return social.Actor{}, social.ContentOwner{}, apperr.FailedPrecondition("%s", text.DecideSnapshotMissing())
	}
	if err != nil {
		return social.Actor{}, social.ContentOwner{}, fmt.Errorf("moderation：讀取案件 %s 的快照：%w", c.ID, err)
	}
	frozen, err := snapshot.Decode(body)
	if err != nil {
		return social.Actor{}, social.ContentOwner{}, fmt.Errorf("moderation：解讀案件 %s 的快照：%w", c.ID, err)
	}
	author := social.Actor{Kind: social.ActorKind(frozen.AuthorKind), ID: frozen.AuthorID}
	return author, social.ContentOwner{Author: author, StoreID: frozen.StoreID}, nil
}

func (s *Service) takedown(ctx context.Context, q *store.Queries, c model.Case, owner social.ContentOwner, in model.DecideInput) error {
	if owner.Author.Kind != social.ActorStore || owner.Author.ID != in.StoreID || owner.StoreID != in.StoreID {
		return apperr.FailedPrecondition("%s", text.DecideTakedownWrongStore())
	}
	// 引用最早的 MaxSourceReports 筆，備註寫真正的檢舉數。檢舉集合在結案的版本比對下才算定案：這之後掛進來的檢舉會讓版本加一，結案回 VersionConflict、處置整個回滾。
	ids, err := q.ReportIDsOfCase(ctx, store.ReportIDsOfCaseParams{CaseID: c.ID, MaxRows: model.MaxSourceReports})
	if err != nil {
		return fmt.Errorf("moderation：讀取案件的檢舉：%w", err)
	}
	total, err := q.CountReportsOfCase(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("moderation：計算案件的檢舉數：%w", err)
	}
	_, err = s.d.Seller.CreateMeasureInTx(ctx, seller.CreateMeasureInput{
		AdminID:           in.AdminID,
		StoreID:           in.StoreID,
		Kind:              seller.MeasureContentTakedown,
		StatementToSeller: in.Text,
		Note:              "moderation 案件 " + c.ID.String() + "（" + strconv.FormatInt(total, 10) + " 筆來源檢舉）",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{
			Category: in.Category, Severity: seller.Severity(in.Severity), SourceModerationReportIDs: ids,
		}},
		Takedown: &seller.ContentTakedown{Contents: []seller.ContentRef{{Kind: seller.ContentKind(c.Target.Kind), ID: c.Target.ContentID}}},
	})
	return err
}
