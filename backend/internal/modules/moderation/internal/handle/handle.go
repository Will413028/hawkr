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
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
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
	Audit      *adminaudit.Log
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
	case in.Actor.AdminID <= 0:
		return in, errors.New("moderation：處理案件的 Actor.AdminID 不是正數，handler 沒有帶管理員")
	case in.Actor.Action == "":
		return in, errors.New("moderation：處理案件的 Actor.Action 空白，handler 沒有帶 procedure 名稱")
	}
	in.Text = trimtext.Trim(in.Text)
	switch in.Decision {
	case model.DecisionNoViolation:
		if in.Category != "" || in.Action != "" || in.Text != "" || in.Severity != "" {
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
			if !slices.Contains(seller.SeverityValues(), seller.Severity(in.Severity)) {
				return in, apperr.InvalidInputOn([]string{"Severity"}, "%s", text.DecideSeverityInvalid())
			}
		} else if in.Severity != "" {
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
	var res applied
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
			if err := s.apply(ctx, q, c, in, &res); err != nil {
				return err
			}
		}
		if err := s.close(ctx, q, in, res.measureID); err != nil {
			return err
		}
		if err := s.audit(ctx, c, in, res); err != nil {
			return err
		}
		out, err = view.Detail(ctx, q, in.CaseID)
		return err
	})
	return out, err
}

// audit 在結案的同一個 transaction 寫後台動作紀錄：決定、處置與類別（不放理由文字）。
func (s *Service) audit(ctx context.Context, c model.Case, in model.DecideInput, res applied) error {
	detail := map[string]any{"decision": string(in.Decision), "target_kind": string(c.Target.Kind), "version": in.Version}
	if in.Decision == model.DecisionViolation {
		detail["action"] = string(in.Action)
		detail["category"] = string(in.Category)
		if in.Action == model.ActionTakedown {
			detail["store_id"] = res.storeID
			detail["measure_id"] = res.measureID.String()
			detail["severity"] = in.Severity
		}
	}
	return s.d.Audit.Record(ctx, adminaudit.Entry{
		Actor: in.Actor, TargetType: "moderation_case", TargetID: in.CaseID.String(), Detail: detail,
	})
}

// close 是最後一步：結案轉移。
func (s *Service) close(ctx context.Context, q *store.Queries, in model.DecideInput, measureID *uuid.UUID) error {
	now := s.d.Now().UTC()
	p := store.CloseCaseParams{
		ID: in.CaseID, Version: in.Version, Decision: ptr(string(in.Decision)), AdminID: &in.Actor.AdminID, ClosedAt: &now, MeasureID: measureID,
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

// applied 是處置的結果：內容下架時快照的店家與建立的措施（其他處置是零值）。
type applied struct {
	storeID   int64
	measureID *uuid.UUID
	frozen    *snapshot.Content
}

// apply 呼叫擁有處分的模組建立處分。
func (s *Service) apply(ctx context.Context, q *store.Queries, c model.Case, in model.DecideInput, res *applied) error {
	ref := SourceRefPrefix + c.ID.String()
	author, owner, err := s.author(ctx, q, c, res)
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
			Category: string(in.Category), Text: in.Text, SourceRef: ref, AdminID: in.Actor.AdminID,
		})
		return err
	case model.ActionWarn:
		if author.Kind != social.ActorBuyer {
			return apperr.FailedPrecondition("%s", text.DecideWarnNotBuyer())
		}
		_, err := s.d.Social.Warn(ctx, social.WarnInput{BuyerID: author.ID, Category: string(in.Category), Text: in.Text, SourceRef: ref, AdminID: in.Actor.AdminID})
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
		return s.takedown(ctx, q, c, owner, in, res)
	}
	return apperr.InvalidInputOn([]string{"Action"}, "%s", text.DecideActionInvalid())
}

// author 回傳對象的作者與歸屬：買家對象的作者就是那位買家；內容向 social 讀。內容已被刪除時（Exists 為假）改用案件快照裡凍結的作者與店家，
// 所以作者刪掉內容不能躲過警告、停權與內容下架的歸屬驗證；隱藏的目的（內容不可見）已達成，由呼叫端視為成功。
func (s *Service) author(ctx context.Context, q *store.Queries, c model.Case, res *applied) (social.Actor, social.ContentOwner, error) {
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
	frozen, err := res.frozenContent(ctx, q, c.ID, text.DecideSnapshotMissing)
	if err != nil {
		return social.Actor{}, social.ContentOwner{}, err
	}
	author := social.Actor{Kind: social.ActorKind(frozen.AuthorKind), ID: frozen.AuthorID}
	return author, social.ContentOwner{Author: author, StoreID: frozen.StoreID}, nil
}

// snapshotReader 是讀案件快照的查詢（*store.Queries）。
type snapshotReader interface {
	LatestSnapshotContent(ctx context.Context, caseID uuid.UUID) (string, error)
}

// frozenContent 讀案件最近一份凍結的內容，一次處理只讀一次；沒有快照回 FailedPrecondition，句子由呼叫端依情境給。
func (a *applied) frozenContent(ctx context.Context, q snapshotReader, caseID uuid.UUID, missing func() uitext.Message) (snapshot.Content, error) {
	if a.frozen != nil {
		return *a.frozen, nil
	}
	body, err := q.LatestSnapshotContent(ctx, caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return snapshot.Content{}, apperr.FailedPrecondition("%s", missing())
	}
	if err != nil {
		return snapshot.Content{}, fmt.Errorf("moderation：讀取案件 %s 的快照：%w", caseID, err)
	}
	frozen, err := snapshot.Decode(body)
	if err != nil {
		return snapshot.Content{}, fmt.Errorf("moderation：解讀案件 %s 的快照：%w", caseID, err)
	}
	a.frozen = &frozen
	return frozen, nil
}

// CheckTakedownStore 檢查內容的作者與歸屬都是快照凍結的店家（內容還在時 owner 是 social 現在的答案，已刪除時是快照本身）。
func CheckTakedownStore(owner social.ContentOwner, frozenStoreID int64) error {
	if frozenStoreID <= 0 || owner.Author.Kind != social.ActorStore || owner.Author.ID != frozenStoreID || owner.StoreID != frozenStoreID {
		return apperr.FailedPrecondition("%s", text.DecideTakedownWrongStore())
	}
	return nil
}

func (s *Service) takedown(ctx context.Context, q *store.Queries, c model.Case, owner social.ContentOwner, in model.DecideInput, res *applied) error {
	frozen, err := res.frozenContent(ctx, q, c.ID, text.DecideTakedownSnapshotMissing)
	if err != nil {
		return err
	}
	storeID := frozen.StoreID
	if err := CheckTakedownStore(owner, storeID); err != nil {
		return err
	}
	ref := seller.ContentRef{Kind: seller.ContentKind(c.Target.Kind), ID: c.Target.ContentID}
	// 內容已在生效中的下架措施之下（例如後台貼文頁先下架）：不再建第二個措施（恢復時只撤銷一個、內容仍下架），案件綁定既有措施並結案；
	// 這個案件的檢舉不另外產生認定（認定隨措施建立，既有措施的處分已經涵蓋這則內容）。
	if existing, err := s.d.Seller.ActiveTakedownOfContent(ctx, ref); err != nil {
		return err
	} else if existing != nil {
		res.storeID, res.measureID = storeID, existing
		return nil
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
	sn, err := measureSnapshot(c.Target.Kind, storeID, frozen)
	if err != nil {
		return err
	}
	created, err := s.d.Seller.CreateMeasureInTx(ctx, seller.CreateMeasureInput{
		AdminID:           in.Actor.AdminID,
		StoreID:           storeID,
		Kind:              seller.MeasureContentTakedown,
		StatementToSeller: in.Text,
		Note:              "moderation 案件 " + c.ID.String() + "（" + strconv.FormatInt(total, 10) + " 筆來源檢舉）",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{
			Category: in.Category, Severity: seller.Severity(in.Severity), SourceModerationReportIDs: ids,
		}},
		Takedown: &seller.ContentTakedown{Contents: []seller.TakedownContent{{ContentRef: ref, Snapshot: &sn}}},
	})
	if err != nil {
		return err
	}
	res.storeID, res.measureID = storeID, &created.Measure.ID
	return nil
}

// measureSnapshot 把案件最近一份凍結的內容轉成措施的內容快照（處分依據的內容版本，A7-D29）：組裝與格式檢查都是 seller 的（NewContentSnapshot、ValidateContentSnapshot），
// 這裡只解析凍結的字串 id。凍結的內容缺連結（舊格式）或不合格式時回錯，不傳半個快照給 seller。
func measureSnapshot(kind model.TargetKind, storeID int64, f snapshot.Content) (seller.ContentSnapshot, error) {
	post, _ := uuid.Parse(f.PostID)     // 空字串與壞格式都是零值，由 ValidateContentSnapshot 依種類判斷缺不缺
	thread, _ := uuid.Parse(f.ThreadID) // 同上
	k := seller.ContentKind(kind)
	sn := seller.NewContentSnapshot(k, f.Body, f.AuthoredAt, len(f.MediaRefs), storeID, post, thread)
	if err := seller.ValidateContentSnapshot(k, sn); err != nil {
		return seller.ContentSnapshot{}, apperr.FailedPrecondition("%s", text.DecideTakedownSnapshotMissing())
	}
	return sn, nil
}
