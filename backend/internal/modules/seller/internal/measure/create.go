package measure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/closing"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	violationField   = "Violation"
	terminationField = "Termination"
	takedownField    = "Takedown"
)

func citedFindingsField(prefix string) string { return prefix + ".CitedFindingIDs" }

func sourceReportsField(prefix string) string { return prefix + ".Finding.SourceReportIDs" }

func sourceModerationReportsField(prefix string) string {
	return prefix + ".Finding.SourceModerationReportIDs"
}

func sourceAuthorityOrdersField(prefix string) string {
	return prefix + ".Finding.SourceAuthorityOrderIDs"
}

// validateSourceAuthorityOrders 檢查新認定引用的來函清單的形狀：筆數上限、不可為空 ID、不可重複。來函是否存在、有沒有指名這個措施的帳號在鎖住帳號之後檢查（checkNamingOrders）。
func validateSourceAuthorityOrders(field string, ids []uuid.UUID) error {
	if len(ids) > model.MaxSourceAuthorityOrders {
		return apperr.InvalidInputOn([]string{field}, "%s", text.MeasureSourceAuthorityOrdersCount().Text)
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, id := range ids {
		if id == uuid.Nil() || seen[id] {
			return apperr.InvalidInputOn([]string{apperr.IndexedField(field, i)}, "%s", text.MeasureSourceAuthorityOrderDuplicate().Text)
		}
		seen[id] = true
	}
	return nil
}

// validateSourceReports 檢查新認定的來源通報清單的形狀：筆數上限、不可為空 ID、不可重複。通報屬於哪個店家、狀態是否成立在鎖住通報之後檢查（findings）。
func validateSourceReports(field string, ids []uuid.UUID) error {
	if len(ids) > model.MaxSourceReports {
		return apperr.InvalidInputOn([]string{field}, "一個認定最多 %d 筆來源通報", model.MaxSourceReports)
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, id := range ids {
		if id == uuid.Nil() || seen[id] {
			return apperr.InvalidInputOn([]string{apperr.IndexedField(field, i)}, "來源通報不可為空或重複")
		}
		seen[id] = true
	}
	return nil
}

// validateSourceModerationReports 檢查新認定的 moderation 來源檢舉清單的形狀：筆數上限、不可為空 ID、不可重複。這些 ID 是不透明的外部參照，seller 不驗證它們存在。
func validateSourceModerationReports(field string, ids []uuid.UUID) error {
	if len(ids) > model.MaxSourceModerationReports {
		return apperr.InvalidInputOn([]string{field}, "一個認定最多 %d 筆來源檢舉", model.MaxSourceModerationReports)
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, id := range ids {
		if id == uuid.Nil() || seen[id] {
			return apperr.InvalidInputOn([]string{apperr.IndexedField(field, i)}, "來源檢舉不可為空或重複")
		}
		seen[id] = true
	}
	return nil
}

// validateCreate 是建立措施的無狀態驗證：格式、必填、enum、依據的形狀、欄位之間的規則。結果與時間和資料庫無關，所以冪等的
// 重送一定與第一次相同（先於 request_id 的查詢）。與時間或資料有關的驗證（結束時間與複審期限的範圍、店家、引用的認定）在
// 查過 request_id 之後（create）。
func validateCreate(in *model.CreateMeasureInput) error {
	if err := model.Positive(in.StoreID, in.AdminID); err != nil {
		return err
	}
	if err := model.OneOf(in.Kind, model.MeasureKindValues(), "Kind", "措施種類不合法"); err != nil {
		return err
	}
	var err error
	if in.StatementToSeller, err = model.Required(in.StatementToSeller, 2000, "StatementToSeller", "給賣家的理由"); err != nil {
		return err
	}
	if in.Note, err = model.Required(in.Note, 2000, "Note", "內部註記"); err != nil {
		return err
	}
	in.EndsAt, in.ReviewBy = atMicrosecond(in.EndsAt), atMicrosecond(in.ReviewBy)
	bases := 0
	for _, present := range []bool{in.Violation != nil, in.Risk != nil, in.Termination != nil} {
		if present {
			bases++
		}
	}
	switch {
	case bases != 1:
		return apperr.InvalidInputOn([]string{violationField}, "依據必須是違規的認定、風險類型或終止事由其中一種")
	case (in.Kind == model.MeasureTermination) != (in.Termination != nil):
		return apperr.InvalidInputOn([]string{"Kind"}, "終止事由和措施種類必須對應：只有終止合作可以（而且必須）帶終止事由")
	case (in.Kind == model.MeasureContentTakedown) != (in.Takedown != nil):
		return apperr.InvalidInputOn([]string{"Kind"}, "內容對象和措施種類必須對應：只有內容下架可以（而且必須）帶內容對象")
	}
	if in.Takedown != nil {
		if err := model.ValidateContentTargets(takedownField+".Contents", in.Takedown.Contents); err != nil {
			return err
		}
	}
	switch {
	case in.Violation != nil:
		if err := validateViolationBasis(violationField, in.Violation); err != nil {
			return err
		}
		if in.ReviewBy != nil {
			return apperr.InvalidInputOn([]string{"ReviewBy"}, "依據認定的措施沒有複審期限")
		}
	case in.Risk != nil:
		if err := model.OneOf(in.Risk.Ground, model.RiskGroundValues(), "Risk.Ground", "風險類型不合法"); err != nil {
			return err
		}
		if in.Kind != model.MeasureSuspension {
			return apperr.InvalidInputOn([]string{"Kind"}, "依據風險類型的措施只能是停權")
		}
		if in.ReviewBy == nil {
			return apperr.InvalidInputOn([]string{"ReviewBy"}, "依據風險類型的措施必須有複審期限")
		}
	default:
		if err := validateTermination(in); err != nil {
			return err
		}
	}
	switch {
	case in.Kind == model.MeasureWarning && in.EndsAt != nil:
		return apperr.InvalidInputOn([]string{"EndsAt"}, "警告沒有結束時間")
	case in.Kind == model.MeasureContentTakedown && in.EndsAt != nil:
		return apperr.InvalidInputOn([]string{"EndsAt"}, "內容下架沒有結束時間")
	case in.Kind == model.MeasureSuspension && in.Violation != nil && in.EndsAt == nil:
		return apperr.InvalidInputOn([]string{"EndsAt"}, "依據認定的停權必須有結束時間")
	}
	return nil
}

// validateTermination 是終止合作的無狀態驗證（條款第 21.2、21.3 條）：事由合法；終止沒有結束時間與複審期限（結束時間只有系統在預告期間內關店時寫入）；
// 預告終止與關聯帳號只能依據契約條款（不帶認定）；重大違約必須引用認定（引用的認定夠不夠久、有沒有撤銷是有狀態的驗證，在 create）；關聯帳號必須指定來源措施，
// 其他事由不可以。
func validateTermination(in *model.CreateMeasureInput) error {
	t := in.Termination
	if err := model.OneOf(t.Ground, model.TerminationGroundValues(), terminationField+".Ground", "終止事由不合法"); err != nil {
		return err
	}
	switch {
	case in.EndsAt != nil:
		return apperr.InvalidInputOn([]string{"EndsAt"}, "終止合作沒有結束時間")
	case in.ReviewBy != nil:
		return apperr.InvalidInputOn([]string{"ReviewBy"}, "終止合作沒有複審期限")
	case t.Ground.ViolationRequirement() == model.ViolationForbidden && t.Violation != nil:
		// 事由的名稱來自共用文案目錄（後台受眾）；OneOf 已確認事由合法，目錄沒有文字是程式錯誤。
		groundText, ok := uitext.Staff().SellerTerminationGround(string(t.Ground))
		if !ok {
			return fmt.Errorf("seller：文案目錄沒有終止事由 %q 的名稱", t.Ground)
		}
		return apperr.InvalidInputOn([]string{terminationField + ".Violation"}, "%s只能依據契約條款，不可以帶認定", groundText)
	case t.Ground.ViolationRequirement() == model.ViolationRequired && (t.Violation == nil || len(t.Violation.CitedFindingIDs) == 0):
		return apperr.InvalidInputOn([]string{citedFindingsField(terminationField + ".Violation")}, "%s必須引用至少一筆已存在 %d 日以上的認定", groundTextOf(t.Ground), citedMinDays(t.Ground))
	case (t.Ground == model.GroundLinkedAccount) != (t.SourceMeasureID != nil):
		return apperr.InvalidInputOn([]string{terminationField + ".SourceMeasureID"}, "只有關聯帳號的終止要指定來源措施，而且一定要")
	}
	if t.Violation != nil {
		return validateViolationBasis(terminationField+".Violation", t.Violation)
	}
	return nil
}

func validateViolationBasis(prefix string, b *model.ViolationBasis) error {
	if b.Finding == nil && len(b.CitedFindingIDs) == 0 {
		return apperr.InvalidInputOn([]string{prefix}, "依據至少要有一筆認定")
	}
	if f := b.Finding; f != nil {
		if err := validateSourceReports(sourceReportsField(prefix), f.SourceReportIDs); err != nil {
			return err
		}
		if err := validateSourceAuthorityOrders(sourceAuthorityOrdersField(prefix), f.SourceAuthorityOrderIDs); err != nil {
			return err
		}
		if err := validateSourceModerationReports(sourceModerationReportsField(prefix), f.SourceModerationReportIDs); err != nil {
			return err
		}
	}
	cited := citedFindingsField(prefix)
	if len(b.CitedFindingIDs) > model.MaxCitedFindings {
		return apperr.InvalidInputOn([]string{cited}, "一次最多引用 %d 筆既有的認定", model.MaxCitedFindings)
	}
	seen := make(map[uuid.UUID]bool, len(b.CitedFindingIDs))
	for i, id := range b.CitedFindingIDs {
		if id == uuid.Nil() || seen[id] {
			return apperr.InvalidInputOn([]string{apperr.IndexedField(cited, i)}, "引用的認定不可為空或重複")
		}
		seen[id] = true
	}
	if f := b.Finding; f != nil {
		if err := model.OneOf(f.Category, policy.Assessable(), prefix+".Finding.Category", "違規類別不合法"); err != nil {
			return err
		}
		if err := model.OneOf(f.Severity, model.SeverityValues(), prefix+".Finding.Severity", "嚴重度不合法"); err != nil {
			return err
		}
	}
	return nil
}

// sortedUUIDs 回傳 ids 依 UUID 的大端位元組順序（與資料庫的 uuid 排序相同）遞增排序的複本，不改動呼叫端的清單。
func sortedUUIDs(ids []uuid.UUID) []uuid.UUID {
	return slices.SortedFunc(slices.Values(ids), uuid.UUID.Compare)
}

// sortedContents 回傳內容對象依（種類位元組順序、UUID 大端位元組順序）遞增排序的複本，不改動呼叫端的清單；對象是集合，順序沒有意義。
func sortedContents(refs []model.ContentRef) []model.ContentRef {
	return slices.SortedFunc(slices.Values(refs), func(a, b model.ContentRef) int {
		if c := strings.Compare(string(a.Kind), string(b.Kind)); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
}

// requestHash 是正規化後的建立輸入（不含 request_id；時間已截到微秒）以固定欄位順序編碼的 SHA-256：冪等重送比對「內容是否相同」的唯一
// 定義。所有 id 清單（來源通報、引用的既有認定）都是集合，不看輸入順序，編碼排序後的複本，沒有例外（主要依據由規則決定，不編碼在位置裡，
// 見 findings）；其他欄位照輸入。排序不改動呼叫端的清單。
func requestHash(in model.CreateMeasureInput) []byte {
	h := sha256.New()
	put := func(parts ...string) {
		for _, p := range parts {
			_, _ = fmt.Fprintf(h, "%d:%s|", len(p), p)
		}
	}
	at := func(t *time.Time) string {
		if t == nil {
			return "-"
		}
		return t.UTC().Format(time.RFC3339Nano)
	}
	put(strconv.FormatInt(in.StoreID, 10), string(in.Kind), in.StatementToSeller, at(in.EndsAt), at(in.ReviewBy), in.Note)
	violation := func(v *model.ViolationBasis) {
		put("VIOLATION")
		if f := v.Finding; f != nil {
			sources := sortedUUIDs(f.SourceReportIDs)
			put("FINDING", string(f.Category), string(f.Severity), strconv.Itoa(len(sources)))
			for _, id := range sources {
				put(id.String())
			}
			// 只在有來源來函時編碼：沒有來源來函的請求 hash 與加這個欄位之前相同。來函也是集合，編碼排序後的複本。
			if orders := sortedUUIDs(f.SourceAuthorityOrderIDs); len(orders) > 0 {
				put("AUTHORITY_ORDERS", strconv.Itoa(len(orders)))
				for _, id := range orders {
					put(id.String())
				}
			}
			if reports := sortedUUIDs(f.SourceModerationReportIDs); len(reports) > 0 {
				put("MODERATION_REPORTS", strconv.Itoa(len(reports)))
				for _, id := range reports {
					put(id.String())
				}
			}
		} else {
			put("NO_FINDING")
		}
		cited := sortedUUIDs(v.CitedFindingIDs)
		put(strconv.Itoa(len(cited)))
		for _, id := range cited {
			put(id.String())
		}
	}
	if v := in.Violation; v != nil {
		violation(v)
	}
	if r := in.Risk; r != nil {
		put("RISK", string(r.Ground))
	}
	if t := in.Termination; t != nil {
		source := "-"
		if t.SourceMeasureID != nil {
			source = t.SourceMeasureID.String()
		}
		put("TERMINATION", string(t.Ground), source)
		if t.Violation != nil {
			violation(t.Violation)
		}
	}
	if in.Takedown != nil {
		contents := sortedContents(in.Takedown.Contents)
		put("TAKEDOWN", strconv.Itoa(len(contents)))
		for _, c := range contents {
			put(string(c.Kind), c.ID.String())
		}
	}
	if in.SourceCaseID != nil {
		// 只在有來源案件時編碼：同一個 request_id 帶不同的案件是不同的請求，而沒有來源案件的請求 hash 與加這個欄位之前相同。
		put("SOURCE_CASE", in.SourceCaseID.String())
	}
	if in.SourceAuthorityOrderID != nil {
		// 同上：同一個 request_id 帶不同的來函是不同的請求。
		put("SOURCE_AUTHORITY_ORDER", in.SourceAuthorityOrderID.String())
	}
	return h.Sum(nil)
}

// Create 建立措施（admin 的 handler 路徑）。帶 request_id 時冪等（AIP-155）：同一位管理員、同一個 request_id、同樣的內容回傳原本的措施的目前狀態
// （stale success 例外：措施本身就是回應的內容，不另存回應本文），內容不同回 InvalidInput；沒命中才驗證並寫入，驗證失敗不留紀錄。
// 整個流程在同一個 transaction：先鎖住目標帳號全部的 stint，再查 (管理員, request_id)，插入用 ON CONFLICT DO NOTHING，不 catch 23505 後繼續
// （單體規則 3）。同一個帳號的並行重送因為 stint 鎖串行，後到的一定查得到先到的；只有目標在不同帳號的並行請求才會插入落空，那時內容一定不同（hash 含店家），回 InvalidInput。
// 已關閉的店家回 NotFound；非預告的終止合作可以建在帳號最近的已關閉 stint，關聯帳號的終止先依 account_id 升序鎖來源帳號與目標帳號。升級規則的計數與新增
// 因此串行：帳號 90 天內第 3 次 MEDIUM 升為 HIGH（認定帶原本與套用的嚴重度）。新的認定、措施、依據、第 1 版與第一則內部註記同一個 transaction 寫入。
// 來源通報在店家列之後鎖住（鎖順序：店家列先於通報列）再檢查：必須是同一個店家、不能是不成立的。限制效果的措施（停權、終止合作）的副作用見 takeEffect
// （身分快照、結束或生效的 job、重算停權期間）；發出 MeasureCreated。
func (s *Service) Create(ctx context.Context, in model.CreateMeasureInput) (model.CreateMeasureResult, error) {
	if err := validateCreate(&in); err != nil {
		return model.CreateMeasureResult{}, err
	}
	var hash []byte
	if in.RequestID != nil {
		hash = requestHash(in)
	}
	var result model.CreateMeasureResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = s.create(ctx, store.New(s.d.DB.Conn(ctx)), in, hash)
		return err
	})
	return result, err
}

// CreateInTx 在呼叫方已開的 transaction 內建立措施（跨模組的上層 use case，例如 moderation 處理檢舉，A7-D7 第 10 項）：不開自己的 transaction，呼叫方回滾時
// 措施、認定、對象、內部註記與事件的效果一起消失。不收 request_id：冪等由呼叫方的狀態轉移擁有。建立者仍是 admin，由 in.AdminID 傳入處理的管理員。
// ctx 沒有 transaction 是呼叫端的程式錯誤，回錯誤而不是自己另開。鎖順序與 Create 相同（目標帳號全部的 stint 先於其他列）。
func (s *Service) CreateInTx(ctx context.Context, in model.CreateMeasureInput) (model.CreateMeasureResult, error) {
	if in.RequestID != nil {
		return model.CreateMeasureResult{}, apperr.InvalidInputOn([]string{"RequestID"}, "在呼叫方的 transaction 內建立不收 request_id")
	}
	if _, ok := db.TxFrom(ctx); !ok {
		return model.CreateMeasureResult{}, errors.New("seller：CreateInTx 必須在呼叫方的 transaction 內呼叫")
	}
	if err := validateCreate(&in); err != nil {
		return model.CreateMeasureResult{}, err
	}
	return s.create(ctx, store.New(s.d.DB.Conn(ctx)), in, nil)
}

// replayOf 在已鎖住目標帳號之後查 (管理員, request_id)：命中且內容相同回傳措施目前的狀態；內容不同回 InvalidInput；沒命中回 false。
func (s *Service) replayOf(ctx context.Context, q *store.Queries, in model.CreateMeasureInput, hash []byte) (model.CreateMeasureResult, bool, error) {
	row, err := q.MeasureByRequest(ctx, store.MeasureByRequestParams{AdminID: in.AdminID, RequestID: in.RequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CreateMeasureResult{}, false, nil
	}
	if err != nil {
		return model.CreateMeasureResult{}, false, err
	}
	if !bytes.Equal(row.RequestHash, hash) {
		return model.CreateMeasureResult{}, false, apperr.InvalidInputOn([]string{"RequestID"}, "這個 request_id 已經用在內容不同的請求")
	}
	measure, err := s.readRow(ctx, q, row, detailAdmin, s.now())
	return model.CreateMeasureResult{Measure: measure, Replayed: true}, true, err
}

func (s *Service) create(ctx context.Context, q *store.Queries, in model.CreateMeasureInput, hash []byte) (model.CreateMeasureResult, error) {
	now := s.now()
	locked, err := s.lockStints(ctx, q, in)
	if err != nil {
		return model.CreateMeasureResult{}, err
	}
	if in.RequestID != nil {
		if result, ok, err := s.replayOf(ctx, q, in, hash); err != nil || ok {
			return result, err
		}
	}
	if err := s.checkTarget(ctx, q, in, locked); err != nil {
		return model.CreateMeasureResult{}, err
	}
	if in.EndsAt != nil && !in.EndsAt.After(now) {
		return model.CreateMeasureResult{}, apperr.InvalidInputOn([]string{"EndsAt"}, "結束時間必須晚於現在")
	}
	if in.ReviewBy != nil && (!in.ReviewBy.After(now) || in.ReviewBy.After(now.Add(model.ReviewWindow))) {
		return model.CreateMeasureResult{}, apperr.InvalidInputOn([]string{"ReviewBy"}, "複審期限必須晚於現在、不超過 30 天")
	}
	if in.SourceCaseID != nil {
		if err := checkSourceCase(ctx, q, *in.SourceCaseID, in.StoreID); err != nil {
			return model.CreateMeasureResult{}, err
		}
	}
	if in.SourceAuthorityOrderID != nil {
		if err := checkNamingOrders(ctx, q, []uuid.UUID{*in.SourceAuthorityOrderID}, locked.AccountID, in.StoreID, func(int) string { return "SourceAuthorityOrderID" }); err != nil {
			return model.CreateMeasureResult{}, err
		}
	}
	violation, field := in.Violation, violationField
	if in.Termination != nil {
		violation, field = in.Termination.Violation, terminationField+".Violation"
		if err := s.checkTermination(ctx, q, in, locked, now); err != nil {
			return model.CreateMeasureResult{}, err
		}
	}
	measureID := uuid.NewV7()
	var lead uuid.UUID
	var findingIDs []uuid.UUID
	if violation != nil {
		if lead, findingIDs, err = s.findings(ctx, q, locked, measureID, in, violation, field, now); err != nil {
			return model.CreateMeasureResult{}, err
		}
	}
	params := store.InsertMeasureParams{
		ID: measureID, AccountID: locked.AccountID, StoreID: in.StoreID, Kind: string(in.Kind), Basis: string(model.BasisRisk),
		ReviewBy: in.ReviewBy, StatementToSeller: in.StatementToSeller, StartsAt: now, EndsAt: in.EndsAt, RequestID: in.RequestID, AdminID: in.AdminID, Now: now,
		SourceCaseID: in.SourceCaseID, SourceAuthorityOrderID: in.SourceAuthorityOrderID,
	}
	if in.RequestID != nil {
		params.RequestHash = hash
	}
	switch {
	case in.Risk != nil:
		params.RiskGround = (*string)(&in.Risk.Ground)
	case violation != nil:
		params.Basis = string(model.BasisViolation)
		params.LeadFindingID = &lead
	default:
		params.Basis = string(model.BasisContract)
	}
	if t := in.Termination; t != nil {
		params.TerminationGround = (*string)(&t.Ground)
		params.SourceMeasureID = t.SourceMeasureID
		params.StartsAt = model.TerminationStart(t.Ground, now)
	}
	row, err := q.InsertMeasure(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) && in.RequestID != nil {
		return model.CreateMeasureResult{}, s.requestIDTakenElsewhere(ctx, q, in, hash)
	}
	if err != nil {
		return model.CreateMeasureResult{}, err
	}
	for _, id := range findingIDs {
		if err := q.InsertMeasureFinding(ctx, store.InsertMeasureFindingParams{MeasureID: measureID, ViolationID: id}); err != nil {
			return model.CreateMeasureResult{}, err
		}
	}
	var contents []model.ContentRef
	if in.Takedown != nil {
		contents = sortedContents(in.Takedown.Contents)
		kinds, ids := make([]string, len(contents)), make([]uuid.UUID, len(contents))
		for i, c := range contents {
			kinds[i], ids[i] = string(c.Kind), c.ID
		}
		if err := q.InsertMeasureContents(ctx, store.InsertMeasureContentsParams{MeasureID: measureID, Kinds: kinds, Ids: ids}); err != nil {
			return model.CreateMeasureResult{}, err
		}
	}
	if _, err := q.InsertGovernanceNote(ctx, store.InsertGovernanceNoteParams{
		NoteID: uuid.NewV7(), MeasureID: &measureID, AdminID: in.AdminID, Body: in.Note, Now: now,
	}); err != nil {
		return model.CreateMeasureResult{}, err
	}
	created := model.MeasureCreated{
		EventID: uuid.NewV7(), OccurredAt: now, MeasureID: measureID, StoreID: in.StoreID, AccountID: locked.AccountID,
		Kind: in.Kind, Basis: model.Basis(row.Basis), StartsAt: row.StartsAt, EndsAt: in.EndsAt, Contents: contents,
	}
	if in.Risk != nil {
		created.RiskGround = &in.Risk.Ground
	}
	if in.Termination != nil {
		created.TerminationGround = &in.Termination.Ground
	}
	if err := s.takeEffect(ctx, q, in, store.SellerMeasure(row), locked, now); err != nil {
		return model.CreateMeasureResult{}, err
	}
	if err := events.Publish(ctx, s.d.Events, created); err != nil {
		return model.CreateMeasureResult{}, err
	}
	measure, err := s.readRow(ctx, q, store.SellerMeasure(row), detailAdmin, now)
	return model.CreateMeasureResult{Measure: measure}, err
}

// requestIDTakenElsewhere 處理插入因 (管理員, request_id) 已存在而落空：stint 鎖串行了同一個帳號的重送，所以走到這裡的是目標在不同帳號的並行請求，
// 內容一定與先到的不同（hash 含店家）。回傳的錯誤讓 transaction 回滾，這個請求寫下的認定與註記一起消失；不在同一個 transaction 繼續。
func (s *Service) requestIDTakenElsewhere(ctx context.Context, q *store.Queries, in model.CreateMeasureInput, hash []byte) error {
	existing, err := q.MeasureByRequest(ctx, store.MeasureByRequestParams{AdminID: in.AdminID, RequestID: in.RequestID})
	if err != nil {
		return err
	}
	if bytes.Equal(existing.RequestHash, hash) {
		return fmt.Errorf("seller：request_id 已存在且內容相同，卻沒有在鎖住帳號後查到（措施 %s）", existing.ID)
	}
	return apperr.InvalidInputOn([]string{"RequestID"}, "這個 request_id 已經用在內容不同的請求")
}

// takeEffect 是限制效果的措施（停權與終止合作）建立時的副作用，都在建立的 transaction 內：建立即生效且有禁止效果的措施（停權，以及非預告的終止
// 合作，含關聯帳號）快照身分（逐措施、在建立時取，不靠 Recompute：帳號已因其他措施停權時 Recompute 不會「發現」新措施；預告終止不禁止日後重新
// 申請，不取）、有結束時間的停權排入到期 job、終止合作排入生效 job（排在 starts_at，生效時關店）、立即生效的重算停權期間。
func (s *Service) takeEffect(ctx context.Context, q *store.Queries, in model.CreateMeasureInput, row store.SellerMeasure, locked store.SellerStore, now time.Time) error {
	if !in.Kind.Restrictive() {
		return nil
	}
	immediate := in.Termination == nil || in.Termination.Ground != model.GroundNotice
	if immediate {
		if err := suspension.RecordIdentities(ctx, q, row.ID, locked.AccountID); err != nil {
			return err
		}
	}
	if in.Kind == model.MeasureSuspension && in.EndsAt != nil {
		if err := suspension.ScheduleExpiry(ctx, s.d.Jobs, row.ID, *in.EndsAt); err != nil {
			return err
		}
	}
	if in.Kind == model.MeasureTermination {
		if err := closing.ScheduleTermination(ctx, s.d.Jobs, row.ID, row.StartsAt); err != nil {
			return err
		}
	}
	if immediate {
		return suspension.Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, locked.AccountID, now)
	}
	return nil
}

// lockStints 鎖住建立措施要碰的帳號全部的 stint 並回傳措施所在的 stint（鎖住之後才讀）。一般措施只鎖目標帳號；關聯帳號的終止要先依 account_id
// 升序鎖住來源帳號與目標帳號（與連帶撤銷同一個順序，不會互鎖）。目標能不能建立措施在鎖住之後由 checkTarget 檢查。
func (s *Service) lockStints(ctx context.Context, q *store.Queries, in model.CreateMeasureInput) (store.SellerStore, error) {
	if err := model.Positive(in.StoreID); err != nil {
		return store.SellerStore{}, err
	}
	owner, err := q.StoreByID(ctx, in.StoreID)
	if err != nil {
		return store.SellerStore{}, model.Error(err)
	}
	accounts := []int64{owner.AccountID}
	if t := in.Termination; t != nil && t.SourceMeasureID != nil {
		source, err := q.MeasureByID(ctx, *t.SourceMeasureID)
		if errors.Is(err, pgx.ErrNoRows) {
			return store.SellerStore{}, apperr.InvalidInputOn([]string{terminationField + ".SourceMeasureID"}, "找不到來源措施")
		}
		if err != nil {
			return store.SellerStore{}, err
		}
		accounts = append(accounts, source.AccountID)
	}
	if err := stint.LockAccounts(ctx, q, accounts); err != nil {
		return store.SellerStore{}, err
	}
	locked, err := q.StoreByID(ctx, in.StoreID)
	if err != nil {
		return store.SellerStore{}, model.Error(err)
	}
	return locked, nil
}

// checkTarget 檢查已鎖住的 stint 能不能建立這個措施：stint 必須還沒關閉；非預告的終止合作可以建在已關閉的 stint 上，只限帳號最近的一個、而且帳號沒有目前的
// stint（店關閉之後才發現的 21.3 事由，效果只剩禁止入駐）；預告終止只能建在開著的店。重送查詢先於這個檢查，所以措施建立後店家關閉（終止合作生效），重送仍回原本的措施。
func (s *Service) checkTarget(ctx context.Context, q *store.Queries, in model.CreateMeasureInput, locked store.SellerStore) error {
	target := model.TerminationTarget{Closed: locked.Status == string(model.Closed)}
	if !target.Closed {
		return nil
	}
	if in.Termination == nil {
		if reasons := stint.ToStore(locked).ActionUnavailableReasons(model.StoreActionCreateMeasure); len(reasons) > 0 {
			return model.ErrStoreActionUnavailable(reasons, model.StoreActionCreateMeasure)
		}
		return nil
	}
	latest, err := q.LatestStoreOfAccount(ctx, locked.AccountID)
	if err != nil {
		return err
	}
	target.LatestOfAccount = latest.StoreID == locked.StoreID
	switch target.BlockFor(in.Termination.Ground) {
	case model.TerminationBlockStoreClosed:
		return apperr.FailedPrecondition("%s", text.TerminationUnavailableStoreClosed().Text)
	case model.TerminationBlockStoreSuperseded:
		return apperr.FailedPrecondition("%s", text.TerminationUnavailableStoreSuperseded().Text)
	}
	return nil
}

// checkTermination 是終止合作的有狀態驗證（帳號與來源措施已鎖住）：重大違約引用的認定至少一筆未撤銷、屬於這個帳號、而且建立它的措施早於現在七日
// （條款第 21.3 條第 1 款要求通知後七日內未改善）；關聯帳號的來源措施必須未撤銷、是另一個帳號、事由不是預告或關聯帳號（不成鏈）的終止，而且本帳號的 KYC
// 身分確實命中來源措施的身分快照（伺服器再驗證一次，與 admin 看到的 linked_accounts 同一支查詢）；同一個來源對同一個帳號第二筆在鎖住帳號後檢查並回 AlreadyExists（唯一索引只是保底）。
func (s *Service) checkTermination(ctx context.Context, q *store.Queries, in model.CreateMeasureInput, locked store.SellerStore, now time.Time) error {
	t := in.Termination
	if age := t.Ground.CitedFindingMinAge(); age > 0 {
		qualifying, err := q.QualifyingCitedFindings(ctx, store.QualifyingCitedFindingsParams{
			Ids: t.Violation.CitedFindingIDs, AccountID: locked.AccountID, OlderThan: now.Add(-age),
		})
		if err != nil {
			return err
		}
		if len(qualifying) == 0 {
			return apperr.InvalidInputOn([]string{citedFindingsField(terminationField + ".Violation")}, "%s引用的認定必須至少有一筆未撤銷、已存在 %d 日以上", groundTextOf(t.Ground), citedMinDays(t.Ground))
		}
	}
	if t.Ground != model.GroundLinkedAccount {
		return nil
	}
	sourceField := terminationField + ".SourceMeasureID"
	source, err := q.MeasureByID(ctx, *t.SourceMeasureID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.InvalidInputOn([]string{sourceField}, "找不到來源措施")
	}
	if err != nil {
		return err
	}
	switch model.LinkedSourceOf(source.Status, source.Kind, source.TerminationGround, source.AccountID).BlockFor(locked.AccountID) {
	case model.LinkedSourceRevoked:
		return apperr.InvalidInputOn([]string{sourceField}, "來源措施已撤銷")
	case model.LinkedSourceNotEligible:
		return apperr.InvalidInputOn([]string{sourceField}, "來源措施必須是第 21.3 條事由的終止合作（預告終止與關聯帳號的終止不能當來源）")
	case model.LinkedSourceSameAccount:
		return apperr.InvalidInputOn([]string{sourceField}, "來源措施必須在另一個帳號")
	}
	exists, err := q.LinkedMeasureExists(ctx, store.LinkedMeasureExistsParams{SourceMeasureID: &source.ID, AccountID: locked.AccountID})
	if err != nil {
		return err
	}
	if exists {
		return apperr.AlreadyExists("這個帳號已經有以這筆措施為來源的關聯終止")
	}
	hits, err := s.d.Linked.LinkedAccountHits(ctx, q, source.ID, source.AccountID, &locked.AccountID)
	if err != nil {
		return err
	}
	// 銀行帳號命中只列出，不能當依據（LinkedIdentity.CountsAsGround，與列表的 can_create_linked_termination 同一個判斷）。
	if !slices.ContainsFunc(hits, func(h model.LinkedHit) bool { return h.Identity.CountsAsGround() }) {
		return apperr.InvalidInputOn([]string{sourceField}, "這個帳號的身分和來源措施的身分快照沒有相同")
	}
	return nil
}

// findings 寫入這個措施的新認定並驗證引用的既有認定，回傳主要依據 lead（lead_finding_id）與全部依據的 id。所有清單都是集合，順序沒有意義：
// 主要依據由規則決定，不看 CitedFindingIDs 的位置。有新的認定時是新的認定（引用的認定再嚴重也不取代它）；否則（只引用既有認定，例如重大違約的
// 終止合作）是引用的認定中最嚴重的，同嚴重度取最新建立的（model.LeadFindingOf）。
// 引用的認定必須存在、屬於同一個帳號、仍生效；認定只由這裡建立，所以每一筆認定都有建立它的措施（origin_measure_id）。
func (s *Service) findings(ctx context.Context, q *store.Queries, locked store.SellerStore, measureID uuid.UUID, in model.CreateMeasureInput, v *model.ViolationBasis, field string, now time.Time) (uuid.UUID, []uuid.UUID, error) {
	var lead uuid.UUID
	var ids []uuid.UUID
	hasLead := false
	if f := v.Finding; f != nil {
		if err := lockSourceReports(ctx, q, in.StoreID, sourceReportsField(field), f.SourceReportIDs); err != nil {
			return uuid.Nil(), nil, err
		}
		if len(f.SourceAuthorityOrderIDs) > 0 {
			orders := sortedUUIDs(f.SourceAuthorityOrderIDs)
			if err := checkNamingOrders(ctx, q, orders, locked.AccountID, in.StoreID, func(i int) string {
				return apperr.IndexedField(sourceAuthorityOrdersField(field), slices.Index(f.SourceAuthorityOrderIDs, orders[i]))
			}); err != nil {
				return uuid.Nil(), nil, err
			}
		}
		severity, escalatedFrom, err := escalate(ctx, q, locked.AccountID, f.Severity, now)
		if err != nil {
			return uuid.Nil(), nil, err
		}
		row, err := q.InsertViolation(ctx, store.InsertViolationParams{
			ID: uuid.NewV7(), StoreID: in.StoreID, AccountID: locked.AccountID, Category: string(f.Category),
			Severity: string(severity), EscalatedFrom: (*string)(escalatedFrom), OriginMeasureID: measureID, AdminID: in.AdminID, Now: now,
		})
		if err != nil {
			return uuid.Nil(), nil, err
		}
		if len(f.SourceReportIDs) > 0 {
			if err := q.InsertFindingReports(ctx, store.InsertFindingReportsParams{ViolationID: row.ID, ReportIds: f.SourceReportIDs}); err != nil {
				return uuid.Nil(), nil, err
			}
		}
		if len(f.SourceModerationReportIDs) > 0 {
			if err := q.InsertFindingModerationReports(ctx, store.InsertFindingModerationReportsParams{ViolationID: row.ID, ReportIds: f.SourceModerationReportIDs}); err != nil {
				return uuid.Nil(), nil, err
			}
		}
		if len(f.SourceAuthorityOrderIDs) > 0 {
			if err := q.InsertFindingAuthorityOrders(ctx, store.InsertFindingAuthorityOrdersParams{ViolationID: row.ID, OrderIds: f.SourceAuthorityOrderIDs}); err != nil {
				return uuid.Nil(), nil, err
			}
		}
		ids = append(ids, row.ID)
		lead, hasLead = row.ID, true
	}
	if len(v.CitedFindingIDs) > 0 {
		rows, err := q.ViolationsByIDs(ctx, v.CitedFindingIDs)
		if err != nil {
			return uuid.Nil(), nil, err
		}
		byID := make(map[uuid.UUID]store.SellerViolation, len(rows))
		for _, r := range rows {
			byID[r.ID] = r
		}
		cited := make([]model.Violation, 0, len(v.CitedFindingIDs))
		for i, id := range v.CitedFindingIDs {
			row, ok := byID[id]
			if !ok || row.AccountID != locked.AccountID || row.Status != string(model.ViolationStatusEnforced) {
				return uuid.Nil(), nil, apperr.InvalidInputOn([]string{apperr.IndexedField(citedFindingsField(field), i)}, "引用的認定不存在、不屬於這個帳號或已撤銷")
			}
			cited = append(cited, toViolation(row, nil))
		}
		ids = append(ids, v.CitedFindingIDs...)
		if !hasLead {
			picked, _ := model.LeadFindingOf(cited)
			lead, hasLead = picked.ID, true
		}
	}
	if !hasLead {
		// validateViolationBasis 保證依據至少有一筆認定；走到這裡是程式錯誤。
		return uuid.Nil(), nil, errors.New("seller：依據沒有任何認定")
	}
	return lead, ids, nil
}

// lockSourceReports 依 id 升序鎖住來源通報（鎖順序：店家列先於通報列，多筆通報依 id 升序，與其他鎖多筆通報的流程不會互鎖），
// 並檢查每一筆都存在、是同一個店家的、結果沒有否定它（Report.Supporting：不是未違規、內容不足或濫報；與 API 的 can_cite_as_source 同一個規則）。
// ids 的形狀（筆數、重複）已由 validateSourceReports 檢查。
func lockSourceReports(ctx context.Context, q *store.Queries, storeID int64, field string, ids []uuid.UUID) error {
	for _, id := range sortedUUIDs(ids) {
		row, err := q.LockReport(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.ReportNotFound().Text)
		}
		if err != nil {
			return err
		}
		if row.StoreID != storeID {
			return apperr.InvalidInputOn([]string{apperr.IndexedField(field, slices.Index(ids, id))}, "來源通報不是這個店家的")
		}
		report := model.Report{}
		if row.Outcome != nil {
			outcome := model.Outcome(*row.Outcome)
			report.Outcome = &outcome
		}
		if !report.Supporting() {
			return apperr.FailedPrecondition("%s", text.SourceReportNegated().Text)
		}
	}
	return nil
}

// checkSourceCase 檢查建立措施帶的來源案件：必須存在（invalid_argument，SourceCaseID）、是措施的店家的案件（invalid_argument）、而且能連結措施
// （failed_precondition，句子取第一個原因）。能不能連結只問 model.CaseFacts.Availability().LinkMeasure，與 API 的 can_link_measure 是同一份判斷；
// 這裡只帶案件列本身的事實（LinkMeasure 只看案件是否進行中，其餘事實與它無關）。
// 不鎖案件：關閉案件的指令都先鎖同一組 stint，而呼叫方已經鎖住了它。
func checkSourceCase(ctx context.Context, q *store.Queries, caseID uuid.UUID, storeID int64) error {
	c, err := q.CaseByID(ctx, caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.InvalidInputOn([]string{"SourceCaseID"}, "%s", text.CaseNotFound().Text)
	}
	if err != nil {
		return err
	}
	if c.StoreID != storeID {
		return apperr.InvalidInputOn([]string{"SourceCaseID"}, "%s", text.MeasureSourceCaseOtherStore().Text)
	}
	facts := model.CaseFacts{Case: model.Case{ID: c.ID, StoreID: c.StoreID, Category: policy.Category(c.Category), ClosedAt: c.ClosedAt}}
	if blocks := facts.Availability().LinkMeasure; len(blocks) > 0 {
		return apperr.FailedPrecondition("%s", blocks[0].Text(model.CaseActionLinkMeasure).Text)
	}
	return nil
}

// checkNamingOrders 檢查每一份來函都存在、指名了這個措施的帳號（指名店家時只算措施的那一家），而且沒有撤銷登記：不存在或沒有指名、已撤銷登記各是
// invalid_argument，field(i) 是第 i 個 id 的欄位。來函與它的對象建立後不能改；撤銷登記會改，所以讀取時以 FOR SHARE 鎖住來函列（呼叫方已鎖住措施帳號的
// 店家列，來函列在後），撤銷登記與建立措施在這一列上串行。
func checkNamingOrders(ctx context.Context, q *store.Queries, ids []uuid.UUID, accountID, storeID int64, field func(int) string) error {
	named, err := q.OrdersNamingMeasureTarget(ctx, store.OrdersNamingMeasureTargetParams{OrderIds: ids, AccountID: accountID, StoreID: &storeID})
	if err != nil {
		return err
	}
	for i, id := range ids {
		j := slices.IndexFunc(named, func(r store.OrdersNamingMeasureTargetRow) bool { return r.ID == id })
		switch {
		case j < 0:
			return apperr.InvalidInputOn([]string{field(i)}, "%s", text.MeasureSourceAuthorityOrderNotNaming().Text)
		case named[j].Revoked:
			return apperr.InvalidInputOn([]string{field(i)}, "%s", text.MeasureSourceAuthorityOrderRevoked().Text)
		}
	}
	return nil
}

// escalate 套用升級規則：帳號在 EscalationWindow 內已有 EscalationCount-1 筆未撤銷、原本是 MEDIUM 的認定時，這一筆 MEDIUM
// 升為 HIGH。呼叫方已鎖住帳號的全部 stint（新 stint 繼承舊 stint 的累計：以店計會讓關店重開歸零）。
func escalate(ctx context.Context, q *store.Queries, accountID int64, severity model.Severity, now time.Time) (model.Severity, *model.Severity, error) {
	if severity != model.SeverityMedium {
		return severity, nil, nil
	}
	n, err := q.CountRecentMediumViolations(ctx, store.CountRecentMediumViolationsParams{
		AccountID: accountID, Since: now.Add(-model.EscalationWindow),
	})
	if err != nil {
		return "", nil, err
	}
	if n+1 < model.EscalationCount {
		return severity, nil, nil
	}
	from := model.SeverityMedium
	return model.SeverityHigh, &from, nil
}

// groundTextOf 是終止事由在錯誤句裡的名稱（共用文案目錄，後台受眾）；目錄沒有時退回事由的代碼（不會發生：事由已由 OneOf 確認合法）。
func groundTextOf(g model.TerminationGround) string {
	if text, ok := uitext.Staff().SellerTerminationGround(string(g)); ok {
		return text
	}
	return string(g)
}

// citedMinDays 是事由引用的認定至少要存在的天數（model.TerminationGround.CitedFindingMinAge，錯誤句與說明句同一個來源）。
func citedMinDays(g model.TerminationGround) int {
	return int(g.CitedFindingMinAge() / (24 * time.Hour))
}
