// Package devseed 寫入本機開發的種子店家：固定的 store_id 與 store_code、已核准的 KYC、logo，以及
// 開店時一併建立的公告列（version 0；由 ReannounceExposures 全量公告）。只給本機開發（cmd/hawkr 的 dev-seed）使用：它直接寫列，
// 繞過開店、送出與審核的狀態轉移（不寫那些轉移的事件；審核紀錄只寫送出那一筆，入駐同意要綁定它）；還原 profile 時發 StoreProfileChanged（與模組自己寫 profile 同一個事件），
// 刪掉的 KYC 證件的 media 資產在同一個 transaction 內 MarkDeleted，治理佐證的資產從不 MarkDeleted。
//
// 明文的 fixture 是來源：KYC 的欄位經 kyc.Service.SeedApproved（SaveDraft 同一支驗證與統編 hash）、以執行中的金鑰寫入，種子檔不含 hash。
package devseed

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/agreement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/kyc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Store 是一家種子店家（明文）。StoreID 與 StoreCode 固定（Java 資料庫的 psql 種子寫死 store_id，MerID 是 HAWK 加 StoreCode）；
// KYC 是核准的申請內容。
type Store struct {
	StoreID, StoreCode, AccountID                            int64
	Name, Description, ContactPhone, TownCode, AddressDetail string
	ContactName, ContactMobile, SupportEmail                 string
	LogoKey                                                  string
	LogoAssetID                                              uuid.UUID
	KYC                                                      model.KYCDraftInput
}

// Deps 外部依賴。
type Deps struct {
	DB     *db.DB
	KYC    *kyc.Service
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Media  model.Media
	Now    func() time.Time
}

// Service 寫入種子店家。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.DB == nil || d.KYC == nil || d.Events == nil || d.Jobs == nil || d.Media == nil || d.Now == nil {
		panic("seller/devseed: missing dependency")
	}
	return &Service{d: d}
}

var clearChildrenSQL = []string{
	`DELETE FROM seller.evidence_accesses WHERE report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1)) OR request_id IN (SELECT id FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))) OR inquiry_id IN (SELECT id FROM seller.case_inquiries WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_request_evidence WHERE request_id IN (SELECT id FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.measure_requests WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_linked_reviews WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_identities WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.governance_notes WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1)) OR case_id IN (SELECT id FROM seller.cases WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_findings WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_revisions WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measure_contents WHERE measure_id IN (SELECT id FROM seller.measures WHERE store_id = ANY($1))`,
	`DELETE FROM seller.measures WHERE store_id = ANY($1)`,
	`DELETE FROM seller.finding_authority_orders WHERE violation_id IN (SELECT id FROM seller.violations WHERE store_id = ANY($1))`,
	`DELETE FROM seller.authority_target_notice_history WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_order_prepurged_stores WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_order_hold_extensions WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_order_targets WHERE order_id IN (SELECT order_id FROM seller.authority_order_targets WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.authority_orders WHERE id NOT IN (SELECT order_id FROM seller.authority_order_targets) AND cardinality($1::bigint[]) >= 0`,
	`DELETE FROM seller.finding_reports WHERE violation_id IN (SELECT id FROM seller.violations WHERE store_id = ANY($1))`,
	`DELETE FROM seller.finding_moderation_reports WHERE violation_id IN (SELECT id FROM seller.violations WHERE store_id = ANY($1))`,
	`DELETE FROM seller.violations WHERE store_id = ANY($1)`,
	`DELETE FROM seller.suspension_periods WHERE store_id = ANY($1)`,
	`DELETE FROM seller.store_blocks WHERE store_id = ANY($1)`,
	`DELETE FROM seller.report_case_moves WHERE store_id = ANY($1)`,
	`DELETE FROM seller.report_evidence WHERE report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1))`,
	`DELETE FROM seller.report_supplements WHERE report_id IN (SELECT id FROM seller.reports WHERE store_id = ANY($1))`,
	`DELETE FROM seller.reports WHERE store_id = ANY($1)`,
	`DELETE FROM seller.case_inquiry_reply_evidence WHERE inquiry_id IN (SELECT id FROM seller.case_inquiries WHERE store_id = ANY($1))`,
	`DELETE FROM seller.case_inquiries WHERE store_id = ANY($1)`,
	`DELETE FROM seller.cases WHERE store_id = ANY($1)`,
	`DELETE FROM seller.payout_capabilities WHERE store_id = ANY($1)`,
	`DELETE FROM seller.agreement_acceptances WHERE store_id = ANY($1)`,
	`DELETE FROM seller.kyc_document_accesses WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_review_issues WHERE review_id IN (SELECT review_id FROM seller.kyc_reviews WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1)))`,
	`DELETE FROM seller.kyc_reviews WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_registration_checks WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.kyc_documents WHERE application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))`,
	`DELETE FROM seller.account_identity_hashes WHERE source_store_id = ANY($1)`,
	`DELETE FROM seller.kyc_applications WHERE store_id = ANY($1)`,
	`DELETE FROM seller.product_views_daily WHERE store_id = ANY($1)`,
	`DELETE FROM seller.product_view_events WHERE store_id = ANY($1)`,
}

const insertExposureSQL = `INSERT INTO seller.store_exposures (store_id) VALUES ($1) ON CONFLICT (store_id) DO NOTHING`

const profileExpr = `jsonb_build_array(s.store_name, s.description, s.contact_phone, s.contact_name, s.contact_mobile, s.support_email, s.town_code, s.address_detail, s.logo_key, s.logo_asset_id,
    s.prize, s.website_url, s.onboarding_completed)::text`

const lockStoresSQL = `SELECT s.store_id, ` + profileExpr + ` FROM seller.stores s WHERE s.store_id = ANY($1) ORDER BY s.store_id FOR UPDATE`

const profileSnapshotSQL = `SELECT s.store_id, ` + profileExpr + ` FROM seller.stores s WHERE s.store_id = ANY($1)`

const lockApplicationsSQL = `SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1) ORDER BY store_id, application_id FOR UPDATE`

const documentAssetsSQL = `
SELECT d.asset_id FROM seller.kyc_documents d
WHERE d.application_id IN (SELECT application_id FROM seller.kyc_applications WHERE store_id = ANY($1))
`

const upsertStoreSQL = `
INSERT INTO seller.stores (
    store_id, store_code, account_id, store_name, description, contact_phone, town_code, address_detail,
    logo_key, logo_asset_id, onboarding_completed, created_at, updated_at, contact_name, contact_mobile, support_email
) OVERRIDING SYSTEM VALUE
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, $11, $11, $12, $13, $14)
ON CONFLICT (store_id) DO UPDATE SET
    store_code = EXCLUDED.store_code, account_id = EXCLUDED.account_id,
    version = seller.stores.version + 1,
    store_name = EXCLUDED.store_name, description = EXCLUDED.description, contact_phone = EXCLUDED.contact_phone,
    contact_name = EXCLUDED.contact_name, contact_mobile = EXCLUDED.contact_mobile, support_email = EXCLUDED.support_email,
    town_code = EXCLUDED.town_code, address_detail = EXCLUDED.address_detail,
    logo_key = COALESCE(seller.stores.logo_key, EXCLUDED.logo_key), logo_asset_id = COALESCE(seller.stores.logo_asset_id, EXCLUDED.logo_asset_id),
    prize = NULL, website_url = NULL,
    onboarding_completed = true,
    updated_at = EXCLUDED.updated_at
WHERE seller.stores.status = 'ACTIVE' AND seller.stores.pii_purged_at IS NULL`

const advanceSequenceSQL = `
SELECT setval(seq, GREATEST(
    COALESCE(pg_sequence_last_value(seq::regclass), 0),
    COALESCE((SELECT max(store_id) FROM seller.stores), 0),
    9999))
FROM (SELECT pg_get_serial_sequence('seller.stores', 'store_id') AS seq) s`

const conflictingStoresSQL = `
SELECT store_id, account_id, store_code FROM seller.stores
WHERE store_id <> ALL($1)
  AND (store_code = ANY($2) OR (status <> 'CLOSED' AND account_id = ANY($3)))
ORDER BY store_id`

const lifecycleStatesSQL = `
SELECT s.store_id, '店家狀態 ' || s.status || CASE WHEN s.current_closed_at IS NOT NULL THEN '（已有關閉紀錄）' ELSE '' END FROM seller.stores s
WHERE s.store_id = ANY($1) AND (s.status <> 'ACTIVE' OR s.pii_purged_at IS NOT NULL)
UNION ALL
SELECT p.store_id, '進行中的停權期間' FROM seller.suspension_periods p
WHERE (p.store_id = ANY($1) OR p.store_id IN (SELECT store_id FROM seller.stores WHERE account_id = ANY($2)))
  AND p.ended_at IS NULL
ORDER BY 1, 2`

const seedAccountsSQL = `SELECT DISTINCT account_id FROM seller.stores WHERE store_id = ANY($1) ORDER BY 1`

func effectiveLines(ctx context.Context, conn db.DBTX, storeIDs, seedAccounts []int64, now time.Time) ([]string, error) {
	owned, err := lines(ctx, conn, seedAccountsSQL, func(rows scanner) (string, error) {
		var id int64
		err := rows.Scan(&id)
		return strconv.FormatInt(id, 10), err
	}, storeIDs)
	if err != nil {
		return nil, err
	}
	accounts := slices.Clone(seedAccounts)
	for _, raw := range owned {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, id)
	}
	slices.Sort(accounts)
	accounts = slices.Compact(accounts)
	byAccount, err := stint.EffectiveMany(ctx, store.New(conn), accounts, now)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, id := range accounts {
		for _, m := range byAccount[id].Measures {
			what := "生效中的停權措施（" + string(m.Basis) + "）"
			if m.Kind == model.MeasureTermination {
				what = "生效中的終止合作措施"
			}
			out = append(out, fmt.Sprintf("  種子帳號 %d：%s %s", id, what, m.MeasureID))
		}
	}
	return out, nil
}

const mixedOrdersSQL = `
WITH touching AS (
    SELECT DISTINCT order_id FROM seller.authority_order_targets
    WHERE store_id = ANY($1) OR account_id IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1))
)
SELECT t.order_id::text,
       EXISTS (SELECT 1 FROM seller.authority_order_targets x WHERE x.order_id = t.order_id
               AND CASE WHEN x.store_id IS NULL
                        THEN x.account_id NOT IN (SELECT account_id FROM seller.stores WHERE store_id = ANY($1))
                        ELSE x.store_id <> ALL($1) END),
       EXISTS (SELECT 1 FROM seller.measures m WHERE m.source_authority_order_id = t.order_id AND m.store_id <> ALL($1)),
       EXISTS (SELECT 1 FROM seller.finding_authority_orders f JOIN seller.violations v ON v.id = f.violation_id
               WHERE f.order_id = t.order_id AND v.store_id <> ALL($1))
FROM touching t
ORDER BY 1`

// Check 在不寫任何資料的前提下確認種子可以安全執行：種子帳號不擁有種子以外的未關閉店家，沒有種子以外的店家佔用種子的
// store_code，而且種子店家沒有還在進行的生命週期狀態（停權措施、關閉、等待刪除）。不成立時回錯誤（列出店家與處理方式），不自動刪店、不還原狀態。
// （種子帳號的刪帳申請由 accountdeletion 擁有，呼叫方在 cmd 另外檢查。）
func (s *Service) Check(ctx context.Context, stores []Store, seedAccountIDs []int64) error {
	return check(ctx, s.d.DB.Conn(ctx), stores, seedAccountIDs, s.d.Now().UTC())
}

func check(ctx context.Context, conn db.DBTX, stores []Store, seedAccountIDs []int64, now time.Time) error {
	if err := validate(stores); err != nil {
		return err
	}
	ids, codes := make([]int64, len(stores)), make([]int64, len(stores))
	for _, st := range stores {
		if !slices.Contains(seedAccountIDs, st.AccountID) {
			return fmt.Errorf("seller：種子店家 %d 的擁有者帳號 %d 不在種子帳號清單內", st.StoreID, st.AccountID)
		}
	}
	accountIDs := seedAccountIDs
	for i, st := range stores {
		ids[i], codes[i] = st.StoreID, st.StoreCode
	}
	conflicts, err := lines(ctx, conn, conflictingStoresSQL, func(rows scanner) (string, error) {
		var id, account, code int64
		if err := rows.Scan(&id, &account, &code); err != nil {
			return "", err
		}
		return fmt.Sprintf("  店家 %d（帳號 %d、store_code %d）", id, account, code), nil
	}, ids, codes, accountIDs)
	if err != nil {
		return err
	}
	states, err := lines(ctx, conn, lifecycleStatesSQL, func(rows scanner) (string, error) {
		var id int64
		var what string
		if err := rows.Scan(&id, &what); err != nil {
			return "", err
		}
		return fmt.Sprintf("  種子帳號名下的店家 %d：%s", id, what), nil
	}, ids, accountIDs)
	if err != nil {
		return err
	}
	measures, err := effectiveLines(ctx, conn, ids, accountIDs, now)
	if err != nil {
		return err
	}
	states = append(states, measures...)
	mixed, err := lines(ctx, conn, mixedOrdersSQL, func(rows scanner) (string, error) {
		var id string
		var targets, derived, findings bool
		if err := rows.Scan(&id, &targets, &derived, &findings); err != nil {
			return "", err
		}
		var what []string
		if targets {
			what = append(what, "有不碰種子的對象")
		}
		if derived {
			what = append(what, "非種子店家的措施依它而生")
		}
		if findings {
			what = append(what, "非種子店家的認定引用它")
		}
		if len(what) == 0 {
			return "", nil
		}
		return fmt.Sprintf("  主管機關來函 %s：%s", id, strings.Join(what, "、")), nil
	}, ids)
	if err != nil {
		return err
	}
	mixed = slices.DeleteFunc(mixed, func(l string) bool { return l == "" })
	var sections []string
	if len(conflicts) > 0 {
		sections = append(sections, "下列店家是種子帳號名下種子以外的未關閉店家，或佔用種子的 store_code；種子店家寫入會撞上一帳號一家店。\n"+strings.Join(conflicts, "\n")+"\n"+
			"請重建本機資料庫；手動測試改用 make dev-token 不帶 -account 建立的新帳號（.agents/skills/backend-e2e-verify/SKILL.md §2），不要用種子的固定帳號")
	}
	if len(states) > 0 {
		sections = append(sections, "種子店家或種子帳號（含只當買家的種子帳號）名下的店家有還在進行的生命週期狀態；它們有下游消費者（事件、停權期間、刪帳流程），種子不繞過它們去還原。\n"+strings.Join(states, "\n")+"\n"+
			"處理：種子帳號名下的停權措施用後台 API 結束或撤銷（SellerGovernanceAdminService.EndMeasure、RevokeMeasure 或裁定申覆，"+
			"到期的停權要等 worker 結束停權期間）；關店與等待刪除沒有還原的 API，請重建本機資料庫")
	}
	if len(mixed) > 0 {
		sections = append(sections, "下列主管機關來函同時碰到種子店家與非種子的資料；種子只能整份刪除來函，會連帶刪掉不屬於它的資料，所以不清除。\n"+strings.Join(mixed, "\n")+"\n"+
			"請重建本機資料庫，或先在後台處理掉非種子的措施與認定")
	}
	if len(sections) == 0 {
		return nil
	}
	return errors.New("dev-seed 中止，沒有寫入任何資料：" + strings.Join(sections, "\n\n"))
}

type scanner interface{ Scan(dest ...any) error }

func lines(ctx context.Context, conn db.DBTX, query string, line func(scanner) (string, error), args ...any) ([]string, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("seller：檢查種子的前提：%w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		l, err := line(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Seed 在一個 transaction 內把種子店家還原成 fixture（upsert，見檔頭的說明）；先鎖店家列再做 Check，前提不成立時不寫任何資料。
// 必須在 transaction 外呼叫。
func (s *Service) Seed(ctx context.Context, stores []Store, seedAccountIDs []int64) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errors.New("seller：種子必須在 transaction 外開始")
	}
	if len(stores) == 0 {
		return errors.New("seller：沒有種子店家")
	}
	if err := validate(stores); err != nil {
		return err
	}
	ids := make([]int64, len(stores))
	for i, st := range stores {
		ids[i] = st.StoreID
	}
	now := s.d.Now().UTC()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		conn := s.d.DB.Conn(ctx)
		before, err := snapshots(ctx, conn, lockStoresSQL, ids)
		if err != nil {
			return fmt.Errorf("seller：鎖住種子店家：%w", err)
		}
		if err := check(ctx, conn, stores, seedAccountIDs, now); err != nil {
			return err
		}
		if err := lockApplications(ctx, conn, ids); err != nil {
			return fmt.Errorf("seller：鎖住種子店家的申請：%w", err)
		}
		q := store.New(conn)
		if err := s.retireDocuments(ctx, conn, ids); err != nil {
			return err
		}
		for _, stmt := range clearChildrenSQL {
			if _, err := conn.Exec(ctx, stmt, ids); err != nil {
				return fmt.Errorf("seller：清除種子店家的使用痕跡：%w", err)
			}
		}
		for _, st := range stores {
			if err := s.seedOne(ctx, q, st, now); err != nil {
				return fmt.Errorf("seller：種子店家 %d：%w", st.StoreID, err)
			}
		}
		if err := s.publishProfileChanges(ctx, conn, q, stores, ids, before, now); err != nil {
			return err
		}
		_, err = conn.Exec(ctx, advanceSequenceSQL)
		return err
	})
}

func snapshots(ctx context.Context, conn db.DBTX, query string, ids []int64) (map[int64]string, error) {
	rows, err := conn.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var snap string
		if err := rows.Scan(&id, &snap); err != nil {
			return nil, err
		}
		out[id] = snap
	}
	return out, rows.Err()
}

func lockApplications(ctx context.Context, conn db.DBTX, ids []int64) error {
	rows, err := conn.Query(ctx, lockApplicationsSQL, ids)
	if err != nil {
		return err
	}
	rows.Close()
	return rows.Err()
}

func (s *Service) retireDocuments(ctx context.Context, conn db.DBTX, ids []int64) error {
	rows, err := conn.Query(ctx, documentAssetsSQL, ids)
	if err != nil {
		return fmt.Errorf("seller：讀種子店家的證件：%w", err)
	}
	var assets []uuid.UUID
	for rows.Next() {
		var asset uuid.UUID
		if err := rows.Scan(&asset); err != nil {
			rows.Close()
			return err
		}
		assets = append(assets, asset)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(assets) == 0 {
		return nil
	}
	if err := s.d.Media.MarkDeleted(ctx, assets); err != nil {
		return fmt.Errorf("seller：刪除種子店家的證件資產：%w", err)
	}
	return nil
}

func (s *Service) publishProfileChanges(ctx context.Context, conn db.DBTX, q *store.Queries, stores []Store, ids []int64, before map[int64]string, now time.Time) error {
	after, err := snapshots(ctx, conn, profileSnapshotSQL, ids)
	if err != nil {
		return err
	}
	for _, st := range stores {
		if prev, ok := before[st.StoreID]; ok && prev == after[st.StoreID] {
			continue
		}
		if err := events.Publish(ctx, s.d.Events, model.StoreProfileChanged{EventID: uuid.NewV7(), OccurredAt: now, StoreID: st.StoreID}); err != nil {
			return err
		}
		row, err := q.StoreByID(ctx, st.StoreID)
		if err != nil {
			return err
		}
		if err := stint.Invalidate(ctx, s.d.Jobs, row); err != nil {
			return err
		}
	}
	return nil
}

func validate(stores []Store) error {
	var ids, codes, accounts []int64
	for _, st := range stores {
		if st.StoreID <= 0 || st.StoreCode <= 0 || st.AccountID <= 0 {
			return fmt.Errorf("seller：種子店家的 store_id、store_code、account_id 必須是正整數：%+v", st.StoreID)
		}
		if slices.Contains(ids, st.StoreID) || slices.Contains(codes, st.StoreCode) || slices.Contains(accounts, st.AccountID) {
			return fmt.Errorf("seller：種子店家 %d 的 store_id、store_code 或 account_id 重複", st.StoreID)
		}
		ids, codes, accounts = append(ids, st.StoreID), append(codes, st.StoreCode), append(accounts, st.AccountID)
		if st.Name == "" || st.TownCode == "" || st.LogoKey == "" || st.LogoAssetID == uuid.Nil() {
			return fmt.Errorf("seller：種子店家 %d 缺少店名、鄉鎮市區代碼或 logo（可曝光的條件）", st.StoreID)
		}
	}
	return nil
}

func (s *Service) seedOne(ctx context.Context, q *store.Queries, st Store, now time.Time) error {
	conn := s.d.DB.Conn(ctx)
	tag, err := conn.Exec(ctx, upsertStoreSQL, st.StoreID, st.StoreCode, st.AccountID, st.Name, st.Description, st.ContactPhone,
		st.TownCode, st.AddressDetail, st.LogoKey, st.LogoAssetID, now,
		nullable(st.ContactName), nullable(st.ContactMobile), nullable(st.SupportEmail))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("店家列不在 fixture 的生命週期狀態（ACTIVE、沒有關閉紀錄），種子不還原它")
	}
	if _, err := conn.Exec(ctx, insertExposureSQL, st.StoreID); err != nil {
		return err
	}
	_, submitID, err := s.d.KYC.SeedApproved(ctx, q, st.StoreID, st.KYC, now)
	if err != nil {
		return err
	}
	if err := agreement.Record(ctx, q, st.StoreID, st.AccountID, current(model.ApplyAgreements), now, agreement.Subject{}, agreement.Source{}); err != nil {
		return err
	}
	if err := agreement.Record(ctx, q, st.StoreID, st.AccountID, current(model.SubmitKYCAgreements), now, agreement.Subject{SubmissionReviewID: &submitID}, agreement.Source{}); err != nil {
		return err
	}
	return nil
}

func current(kinds []model.AgreementKind) []model.AgreementAcceptance {
	out := make([]model.AgreementAcceptance, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, model.AgreementAcceptance{Kind: k, Version: model.CurrentAgreements[k].Version})
	}
	return out
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
