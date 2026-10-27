// Package prospect 是商家名單（站外業者的 BD 名單）：建立（含建立前的比對）、清單、讀取、部分更新、封存與還原、合併、
// 手動轉成店家、統編對帳、疑似重複、採購候選搜尋與主要聯絡 email，以及品牌圖片。來源是 seller-service 的 seller_prospects
// （遷移計畫 A3b 步驟 7c 階段 4）；聯絡方式與比對的模型見 ADR 2026-10-04-prospect-contact-points-and-matching。
//
// 聯絡方式（email、電話、網站、社群帳號）是 prospect_contacts 的清單：值是管理員輸入的原值，比對鍵由解析器推導（可為空），
// 某個類型有聯絡方式時恰好一個是主要的；建立與更新整批替換。比對鍵不唯一：建立、修改聯絡方式與還原時，與其他未封存的業者
// 有相同比對鍵、而且沒有標記為不是同一家的，回 PROSPECT_MATCH_FOUND；管理員確認不是同一家（ConfirmedDistinct）後在同一個
// transaction 記下 prospect_distinct_pairs 再寫入。合併把輸家的聯絡方式併入贏家、配對改掛贏家，輸家封存。統編是權威識別碼，
// 在未封存（進行中或已入駐）的業者之間由資料庫保證唯一。名稱相同與比對鍵相同的配對都只交給人判斷（建立前的比對與疑似重複清單），永不自動合併。
//
// 寫入以整列的 version 做樂觀鎖（文字、聯絡方式與圖片共用）。品牌圖片的 media 引用以 prospect_id 登記，釋放一律經收斂 job
// （封存與合併把圖片欄位清成 NULL，job 讀列、只保留目前那一張），與店家的 logo、封面同一種做法。
package prospect

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Media  media.Media
	Now    func() time.Time
}

// Service 是商家名單的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func errVersionConflict() error { return apperr.VersionConflict("業者資料已被修改") }

func errNotFound() error { return apperr.NotFound("找不到業者") }

// notFound 把查無資料轉成 NotFound。
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound()
	}
	return err
}

// uniqueViolation 回傳違反唯一性的約束名。寫入在 transaction 內撞唯一性時，呼叫方立刻回業務錯誤、不再下任何 SQL。
func uniqueViolation(err error) (string, bool) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return pg.ConstraintName, true
	}
	return "", false
}

// conflictError 把撞唯一性轉成業務錯誤：轉成店家的認領有明確的說法，沒有登記的約束照原樣傳回。
func conflictError(err error) error {
	if name, ok := uniqueViolation(err); ok && name == "prospects_converted_store" {
		return apperr.AlreadyExists("此店家已被其他業者認領")
	}
	return err
}

func toProspect(row store.SellerProspect) model.Prospect {
	return model.Prospect{
		ID: row.ProspectID, Version: row.Version, DisplayName: row.DisplayName, VendorType: model.VendorType(row.VendorType),
		Status: model.ProspectStatus(row.Status), OutreachStatus: model.OutreachStatus(row.OutreachStatus), TaxID: row.TaxID,
		ConvertedStore: row.ConvertedStoreID, ConvertedAt: row.ConvertedAt, MergedInto: row.MergedInto,
		Category: row.Category, Region: row.Region, ProductSummary: row.ProductSummary, Positioning: row.Positioning,
		ResearchSummary: row.ResearchSummary, PriorityLetter: row.PriorityLetter, PriorityStars: row.PriorityStars,
		LogoKey: row.LogoKey, LogoSourceURL: row.LogoSourceUrl, CoverKey: row.CoverKey, CoverSourceURL: row.CoverSourceUrl,
		CreatedByAdminID: row.CreatedByAdminID, UpdatedByAdminID: row.UpdatedByAdminID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func toProspects(rows []store.SellerProspect) []model.Prospect {
	out := make([]model.Prospect, len(rows))
	for i, row := range rows {
		out[i] = toProspect(row)
	}
	return out
}

// load 補上單筆讀取才帶的聯絡方式與分類。
func load(ctx context.Context, q *store.Queries, row store.SellerProspect) (model.Prospect, error) {
	p := toProspect(row)
	var err error
	if p.Contacts, err = loadContacts(ctx, q, row.ProspectID); err != nil {
		return model.Prospect{}, err
	}
	if p.CategoryIDs, err = q.ProspectCategories(ctx, row.ProspectID); err != nil {
		return model.Prospect{}, err
	}
	return p, nil
}

// Create 建立一個業者；聯絡方式與分類在同一個 transaction 寫入。統編已被別的未封存業者持有時回 AlreadyExists（reason PROSPECT_TAX_ID_HELD，metadata prospectIds 是持有者的
// ID）；聯絡方式的比對鍵與其他未封存業者相同、而且沒有標記為不是同一家時回 PROSPECT_MATCH_FOUND，ConfirmedDistinct 涵蓋全部
// 相符的業者時才建立，並在同一個 transaction 記下這些配對（建立之前用 Preview 看有哪些可能的重複）。
func (s *Service) Create(ctx context.Context, in model.CreateProspectInput) (model.Prospect, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.Prospect{}, err
	}
	if err := model.Positive(in.ConfirmedDistinct...); err != nil {
		return model.Prospect{}, err
	}
	fields := in.Fields
	if err := model.NormalizeProspect(&fields, true); err != nil {
		return model.Prospect{}, err
	}
	categories, err := model.NormalizeCategoryIDs(in.CategoryIDs)
	if err != nil {
		return model.Prospect{}, err
	}
	contacts, err := model.NormalizeContacts(in.Contacts)
	if err != nil {
		return model.Prospect{}, err
	}
	var out model.Prospect
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if holder, held, err := taxIDHolder(ctx, q, 0, fields.TaxID); err != nil {
			return err
		} else if held {
			return errTaxIDHeld(holder, opCreate)
		}
		confirmed, err := resolveMatches(ctx, q, 0, model.ContactMatchKeys(contacts), in.ConfirmedDistinct)
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		row, err := q.CreateProspect(ctx, store.CreateProspectParams{
			DisplayName: fields.DisplayName, VendorType: string(fields.VendorType), OutreachStatus: string(fields.OutreachStatus),
			TaxID: fields.TaxID, Category: fields.Category, Region: fields.Region,
			ProductSummary: fields.ProductSummary, Positioning: fields.Positioning, ResearchSummary: fields.ResearchSummary,
			PriorityLetter: fields.PriorityLetter, PriorityStars: fields.PriorityStars, AdminID: &in.AdminID, Now: now,
		})
		if err != nil {
			return err
		}
		if err := addContacts(ctx, q, row.ProspectID, contacts); err != nil {
			return err
		}
		if err := q.AddProspectCategories(ctx, store.AddProspectCategoriesParams{ProspectID: row.ProspectID, CategoryIds: categories}); err != nil {
			return err
		}
		if err := confirmDistinct(ctx, q, row.ProspectID, in.AdminID, now, confirmed); err != nil {
			return err
		}
		out, err = load(ctx, q, row)
		return err
	})
	if err != nil {
		return model.Prospect{}, conflictError(err)
	}
	return out, nil
}

// Preview 建立之前的比對：回傳可能是同一家的業者，每個業者只出現一次、取最高的層級（T1 統編相同；T2 聯絡方式的比對鍵相同；
// T3 名稱相同，只供人判斷）。T1 讓 Create 被拒絕；T2 讓 Create 回 PROSPECT_MATCH_FOUND（確認不是同一家才建立）。聯絡方式
// 只用來推導比對鍵，不要求主要的。
func (s *Service) Preview(ctx context.Context, in model.ProspectPreviewInput) ([]model.ProspectMatch, error) {
	for _, c := range in.Contacts {
		if !slices.Contains(model.ContactKindValues(), c.Kind) {
			return nil, apperr.InvalidInputOn([]string{"Contacts"}, "聯絡方式的類型不合法")
		}
	}
	q := store.New(s.d.DB.Conn(ctx))
	type found struct {
		tier      model.ProspectMatchTier
		matchedOn []model.MatchReason
	}
	byID := map[int64]*found{}
	note := func(id int64, tier model.ProspectMatchTier, on model.MatchReason) {
		f := byID[id]
		if f == nil {
			f = &found{tier: tier}
			byID[id] = f
		}
		if tierRank(tier) < tierRank(f.tier) {
			f.tier = tier
		}
		f.matchedOn = append(f.matchedOn, on)
	}
	if taxID := model.Text(in.TaxID); taxID != nil {
		holder, err := q.LiveProspectByTaxID(ctx, taxID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return nil, err
		default:
			note(holder.ProspectID, model.MatchTaxID, model.ReasonTaxID)
		}
	}
	holders, err := sharedHolders(ctx, q, 0, model.ContactMatchKeys(in.Contacts))
	if err != nil {
		return nil, err
	}
	for _, h := range holders {
		kind, _, _ := strings.Cut(h.MatchKey, ":")
		note(h.ProspectID, model.MatchContact, model.MatchReason(kind))
	}
	if name := trimtext.Trim(in.DisplayName); name != "" {
		named, err := q.ProspectsByName(ctx, name)
		if err != nil {
			return nil, err
		}
		for _, row := range named {
			note(row.ProspectID, model.MatchName, model.ReasonName)
		}
	}
	ids := make([]int64, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	rows, err := q.ProspectsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]model.ProspectMatch, 0, len(rows))
	for _, row := range rows {
		f := byID[row.ProspectID]
		out = append(out, model.ProspectMatch{Prospect: toProspect(row), Tier: f.tier, MatchedOn: f.matchedOn})
	}
	slices.SortStableFunc(out, func(a, b model.ProspectMatch) int { return tierRank(a.Tier) - tierRank(b.Tier) })
	return out, nil
}

func tierRank(t model.ProspectMatchTier) int {
	switch t {
	case model.MatchTaxID:
		return 0
	case model.MatchContact:
		return 1
	case model.MatchName:
	}
	return 2
}

// Get 讀一個業者（任何狀態），帶聯絡方式與分類。
func (s *Service) Get(ctx context.Context, id int64) (model.Prospect, error) {
	if err := model.Positive(id); err != nil {
		return model.Prospect{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.ProspectByID(ctx, id)
	if err != nil {
		return model.Prospect{}, notFound(err)
	}
	return load(ctx, q, row)
}

// List 是 admin 的清單：預設只看 ACTIVE，新到舊的 keyset，預設 20、上限 100。
func (s *Service) List(ctx context.Context, in model.ProspectListInput) (model.ProspectPage, error) {
	size, err := pagination.PageSize(in.PageSize, 20, 100)
	if err != nil {
		return model.ProspectPage{}, err
	}
	params := store.ListProspectsParams{KeywordPattern: model.LikePattern(in.Keyword), PageSize: size + 1}
	if in.Status != nil {
		if !slices.Contains(model.ProspectStatusValues(), *in.Status) {
			return model.ProspectPage{}, apperr.InvalidInputOn([]string{"Status"}, "狀態不合法")
		}
		params.Status = ptr(string(*in.Status))
	}
	if in.OutreachStatus != nil {
		if !slices.Contains(model.OutreachStatusValues(), *in.OutreachStatus) {
			return model.ProspectPage{}, apperr.InvalidInputOn([]string{"OutreachStatus"}, "招商狀態不合法")
		}
		params.OutreachStatus = ptr(string(*in.OutreachStatus))
	}
	if in.VendorType != nil {
		if !slices.Contains(model.VendorTypeValues(), *in.VendorType) {
			return model.ProspectPage{}, apperr.InvalidInputOn([]string{"VendorType"}, "業者類型不合法")
		}
		params.VendorType = ptr(string(*in.VendorType))
	}
	params.Category, params.Region = model.Text(in.Category), model.Text(in.Region)
	if in.After != nil {
		params.AfterUpdatedAt, params.AfterID = &in.After.UpdatedAt, &in.After.ProspectID
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).ListProspects(ctx, params)
	if err != nil {
		return model.ProspectPage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	page := model.ProspectPage{Items: toProspects(rows)}
	if hasMore {
		last := page.Items[len(page.Items)-1]
		page.Next = &model.ProspectCursor{UpdatedAt: last.UpdatedAt, ProspectID: last.ID}
	}
	return page, nil
}

func ptr[T any](v T) *T { return &v }

// updatePlan 是 Update 在讀到的列上算出來的結果：要寫什麼、要記下哪些「不是同一家」，或什麼都不用寫。
type updatePlan struct {
	row               store.SellerProspect
	fields            model.ProspectFields
	contacts          []model.ContactPoint
	contactsChanged   bool
	categories        []string
	categoriesChanged bool
	confirmed         []int64
	noop              bool
}

// planUpdate 在 row 上做 Update 所有不需要寫入就能判斷的檢查與計算：狀態、版本、合併後的驗證、統編、聯絡方式的比對。換圖的
// 轉正之前先呼叫一次（被拒絕的請求不消耗 staged token），transaction 內以列鎖內的資料再呼叫一次，以後者為準。contacts 與
// categories 是已正規化的輸入（nil 保留）。
func planUpdate(ctx context.Context, q *store.Queries, row store.SellerProspect, in model.UpdateProspectInput, contacts *[]model.ContactPoint, categories []string) (updatePlan, error) {
	if row.Status != string(model.ProspectStatusActive) {
		return updatePlan{}, apperr.FailedPrecondition("只有進行中的業者可以修改")
	}
	if row.Version != in.ExpectedVersion {
		return updatePlan{}, errVersionConflict()
	}
	before := model.ProspectFieldsFor(toProspect(row))
	fields := model.MergeProspect(before, in)
	if err := model.NormalizeProspect(&fields, true); err != nil {
		return updatePlan{}, err
	}
	plan := updatePlan{row: row, fields: fields}
	existingContacts, err := loadContacts(ctx, q, row.ProspectID)
	if err != nil {
		return updatePlan{}, err
	}
	if contacts != nil {
		plan.contacts = *contacts
		plan.contactsChanged = !model.SameContacts(existingContacts, *contacts)
	}
	existingCategories, err := q.ProspectCategories(ctx, row.ProspectID)
	if err != nil {
		return updatePlan{}, err
	}
	plan.categories = categories
	plan.categoriesChanged = in.CategoryIDs != nil && !slices.Equal(existingCategories, categories)
	if reflect.DeepEqual(fields, before) && !plan.contactsChanged && !plan.categoriesChanged && in.Logo == nil && in.Cover == nil {
		plan.noop = true
		return plan, nil
	}
	if !reflect.DeepEqual(fields.TaxID, before.TaxID) {
		if holder, held, err := taxIDHolder(ctx, q, row.ProspectID, fields.TaxID); err != nil {
			return updatePlan{}, err
		} else if held {
			return updatePlan{}, errTaxIDHeld(holder, opUpdate)
		}
	}
	if plan.contactsChanged {
		if plan.confirmed, err = resolveMatches(ctx, q, row.ProspectID, model.ContactMatchKeys(plan.contacts), in.ConfirmedDistinct); err != nil {
			return updatePlan{}, err
		}
	}
	return plan, nil
}

// Update 部分更新（只有 ACTIVE 的業者可以改）：nil 保留原值，空字串清成沒有值，合併後整體再驗證一次；ExpectedVersion 不符回
// VersionConflict，沒有任何改變時回傳目前的業者、不寫入（版本不變，過期的版本照樣衝突）。Contacts 非 nil 時整批替換聯絡方式：
// 改變時新的比對鍵與其他未封存業者相同、而且沒有標記為不是同一家，回 PROSPECT_MATCH_FOUND（ConfirmedDistinct 涵蓋全部相符的
// 業者時才寫入，並在同一個 transaction 記下配對）；統編被別的未封存業者持有回 FailedPrecondition（reason PROSPECT_TAX_ID_HELD）。
//
// Logo、Cover 非 nil 時換圖：先在 transaction 外轉正（不建立引用；轉正前先做所有不需寫入的判斷：狀態、版本、驗證、統編、聯絡方式
// 的比對，被拒絕的請求不消耗 staged token），再於同一個版本條件的 transaction 內寫入文字、聯絡方式與圖片的 key 與 asset ID、把被換掉的
// 圖記成退場並排入引用的同步（mediaref）；已退場的圖不能寫回。換圖時必須在 transaction 外呼叫。
func (s *Service) Update(ctx context.Context, in model.UpdateProspectInput) (model.Prospect, error) {
	if err := model.Positive(in.ProspectID, in.AdminID); err != nil {
		return model.Prospect{}, err
	}
	if err := model.Positive(in.ConfirmedDistinct...); err != nil {
		return model.Prospect{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Prospect{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	var categories []string
	if in.CategoryIDs != nil {
		var err error
		if categories, err = model.NormalizeCategoryIDs(*in.CategoryIDs); err != nil {
			return model.Prospect{}, err
		}
	}
	var contacts *[]model.ContactPoint
	if in.Contacts != nil {
		normalized, err := model.NormalizeContacts(*in.Contacts)
		if err != nil {
			return model.Prospect{}, err
		}
		contacts = &normalized
	}
	uploads, err := s.finalizeUploads(ctx, in, contacts, categories)
	if err != nil {
		return model.Prospect{}, err
	}
	var out model.Prospect
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := lockOne(ctx, q, in.ProspectID)
		if err != nil {
			return err
		}
		plan, err := planUpdate(ctx, q, locked, in, contacts, categories)
		if err != nil {
			return err
		}
		if plan.noop {
			out, err = load(ctx, q, locked)
			return err
		}
		for _, u := range uploads {
			if err := mediaref.Reject(ctx, q, mediaref.ProspectRef(u.kind.refType, in.ProspectID), media.SyncCandidate{Field: u.kind.field + ".StagedToken", AssetID: u.asset.ID}); err != nil {
				return err
			}
		}
		fields, now := plan.fields, s.d.Now().UTC()
		params := store.UpdateProspectParams{
			ProspectID: locked.ProspectID, ExpectedVersion: in.ExpectedVersion,
			DisplayName: fields.DisplayName, VendorType: string(fields.VendorType), OutreachStatus: string(fields.OutreachStatus),
			TaxID: fields.TaxID, Category: fields.Category, Region: fields.Region,
			ProductSummary: fields.ProductSummary, Positioning: fields.Positioning, ResearchSummary: fields.ResearchSummary,
			PriorityLetter: fields.PriorityLetter, PriorityStars: fields.PriorityStars,
			AdminID: &in.AdminID, Now: now,
		}
		for _, u := range uploads {
			u.kind.set(&params, u)
		}
		updated, err := q.UpdateProspect(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return errVersionConflict()
		}
		if err != nil {
			return err
		}
		if plan.contactsChanged {
			if err := replaceContacts(ctx, q, locked.ProspectID, plan.contacts); err != nil {
				return err
			}
		}
		if plan.categoriesChanged {
			if err := q.ClearProspectCategories(ctx, locked.ProspectID); err != nil {
				return err
			}
			if err := q.AddProspectCategories(ctx, store.AddProspectCategoriesParams{ProspectID: locked.ProspectID, CategoryIds: plan.categories}); err != nil {
				return err
			}
		}
		if err := confirmDistinct(ctx, q, locked.ProspectID, in.AdminID, now, plan.confirmed); err != nil {
			return err
		}
		for _, u := range uploads {
			var retired []string
			if old := u.kind.assetOf(locked); old != nil && *old != u.asset.ID {
				retired = append(retired, *old)
			}
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.ProspectRef(u.kind.refType, in.ProspectID), retired...); err != nil {
				return err
			}
		}
		out, err = load(ctx, q, updated)
		return err
	})
	if err != nil {
		return model.Prospect{}, err
	}
	return out, nil
}

// lockOne 鎖定一個業者；找不到回 NotFound。
func lockOne(ctx context.Context, q *store.Queries, id int64) (store.SellerProspect, error) {
	rows, err := q.LockProspects(ctx, []int64{id})
	if err != nil {
		return store.SellerProspect{}, err
	}
	if len(rows) == 0 {
		return store.SellerProspect{}, errNotFound()
	}
	return rows[0], nil
}
