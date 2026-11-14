//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// 在已經有申請的資料庫上套用 KYC 登記資料查詢的 migration：它先加 submission_seq、把已送件的申請（送出、核准、退回）回填成 1，
// 才加「送件過的申請 ≥ 1」的 CHECK，所以套用成功；草稿與已取消的維持 0，之後重新查詢插入的查詢列（submission_seq 取申請的值）合法。
func TestRegistrationChecksMigrationBackfillsSubmissionSeqOfExistingApplications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	// 以檔名找到這個 migration，套用到它的前一個；它之後新增的 migration 與這個測試無關。
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_kyc_registration_checks.sql") })
	if at < 0 {
		t.Fatalf("找不到 kyc_registration_checks 的 migration：%v", files)
	}
	last := files[at]
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	// 套用之前，各種狀態的申請（一個店家一筆；送件過的帶完整的必填欄位）。
	statuses := []string{"DRAFT", "SUBMITTED", "APPROVED", "REJECTED", "CANCELLED"}
	for i, status := range statuses {
		exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, 'x', now(), now())", 2600000100+i, 500+i)
		exec(`INSERT INTO seller.kyc_applications (
				application_id, store_id, status, seller_type, real_name, english_name, bank_name, bank_code, bank_account_enc, bank_account_hash,
				bank_account_hash_key, contact_email, contact_phone, id_card_no_enc, id_card_hash, id_card_hash_key,
				submitted_at, decided_at, created_at, updated_at)
			SELECT uuidv7(), store_id, $1::text, 'INDIVIDUAL', '王小明', 'Wang', '臺灣銀行', '004', '\x01', decode(repeat('ab', 32), 'hex'),
				'k', 'owner@example.com', '0912345678', '\x02', decode(repeat('cd', 32), 'hex'), 'k',
				CASE WHEN $1::text IN ('DRAFT', 'CANCELLED') THEN NULL ELSE now() END,
				CASE WHEN $1::text IN ('APPROVED', 'REJECTED') THEN now() END, now(), now()
			FROM seller.stores WHERE store_code = $2`, status, 2600000100+i)
	}
	exec(body(last))
	for _, status := range statuses {
		want := int64(1)
		if status == "DRAFT" || status == "CANCELLED" {
			want = 0
		}
		var got int64
		must(t, d.Pool().QueryRow(ctx, "SELECT submission_seq FROM seller.kyc_applications WHERE status = $1", status).Scan(&got))
		if got != want {
			t.Errorf("%s 的 submission_seq=%d，預期 %d", status, got, want)
		}
	}
	// 回填的輪次可以直接用：已核准的申請插入查詢列（取申請的 submission_seq）合法；草稿的是 0，被 CHECK 擋下。
	exec(`INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq)
		SELECT uuidv7(), application_id, 1, 'SUBMIT', now(), submission_seq FROM seller.kyc_applications WHERE status = 'APPROVED'`)
	if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq)
		SELECT uuidv7(), application_id, 1, 'SUBMIT', now(), submission_seq FROM seller.kyc_applications WHERE status = 'DRAFT'`); err == nil {
		t.Error("草稿的輪次是 0，查詢列的 submission_seq 必須 ≥ 1")
	}
}

// 在已有 company_type 與含它的比對、退回紀錄的資料庫上套用刪除 company_type 的 migration：先把紀錄裡的欄位名清掉，
// 才收緊 CHECK，所以套用成功；其他欄位名與其餘資料保留，欄位本身不存在了。
func TestKYCCompanyTypeDropMigrationCleansExistingRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_kyc_company_type_drop.sql") })
	if at < 0 {
		t.Fatalf("找不到 kyc_company_type_drop 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000200, 600, 'x', now(), now())")
	exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, company_type, company_name, created_at, updated_at)
		SELECT '0199aaaa-0000-7000-8000-000000000001', store_id, 'DRAFT', 'COMPANY', '有限公司', '範例小農有限公司', now(), now()
		FROM seller.stores WHERE store_code = 2600000200`)
	exec(`INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq, outcome, queried_at,
			tax_id, name, address_text, status, status_text, mismatched_fields, unverifiable_fields)
		VALUES ('0199aaaa-0000-7000-8000-000000000002', '0199aaaa-0000-7000-8000-000000000001', 1, 'SUBMIT', now(), 1, 'FOUND', now(),
			'33330333', '範例小農有限公司', '臺北市', 'ACTIVE', '核准設立',
			ARRAY['business.company_type', 'business.company_name'], ARRAY['business.company_address'])`)
	exec(`INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq, outcome, queried_at,
			tax_id, name, address_text, status, status_text, mismatched_fields, unverifiable_fields)
		VALUES ('0199aaaa-0000-7000-8000-000000000003', '0199aaaa-0000-7000-8000-000000000001', 2, 'SUBMIT', now(), 1, 'FOUND', now(),
			'33330333', '範例小農有限公司', '臺北市', 'ACTIVE', '核准設立',
			ARRAY['business.company_register_date'], ARRAY['business.company_type'])`)
	exec(`INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, reason, required_fields, decided_at)
		VALUES (uuidv7(), '0199aaaa-0000-7000-8000-000000000001', 'REJECT', 1, '補件', ARRAY['business.company_type', 'business.tax_id'], now())`)

	exec(body(files[at]))

	var mismatched, unverifiable []string
	must(t, d.Pool().QueryRow(ctx, "SELECT mismatched_fields, unverifiable_fields FROM seller.kyc_registration_checks WHERE seq = 1").Scan(&mismatched, &unverifiable))
	if !slices.Equal(mismatched, []string{"business.company_name"}) || !slices.Equal(unverifiable, []string{"business.company_address"}) {
		t.Errorf("seq 1 的比對欄位 mismatched=%v unverifiable=%v", mismatched, unverifiable)
	}
	must(t, d.Pool().QueryRow(ctx, "SELECT mismatched_fields, unverifiable_fields FROM seller.kyc_registration_checks WHERE seq = 2").Scan(&mismatched, &unverifiable))
	if !slices.Equal(mismatched, []string{"business.company_register_date"}) || len(unverifiable) != 0 {
		t.Errorf("seq 2 的比對欄位 mismatched=%v unverifiable=%v", mismatched, unverifiable)
	}
	var required []string
	must(t, d.Pool().QueryRow(ctx, "SELECT required_fields FROM seller.kyc_reviews").Scan(&required))
	if !slices.Equal(required, []string{"business.tax_id"}) {
		t.Errorf("退回要補的欄位 %v，預期只剩 business.tax_id", required)
	}
	var columns int
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'kyc_applications' AND column_name = 'company_type'").Scan(&columns))
	if columns != 0 {
		t.Error("kyc_applications.company_type 應已刪除")
	}
	if _, err := d.Pool().Exec(ctx, "UPDATE seller.kyc_registration_checks SET mismatched_fields = ARRAY['business.company_type'] WHERE seq = 2"); err == nil {
		t.Error("比對欄位不可再出現 business.company_type")
	}
}

// 在已有 KYC 申請與 PayUni 註冊副本的資料庫上套用「聯絡資料併進店家」的 migration：舊的聯絡欄位刪除（系統還沒上線，資料不搬到店家，
// 本機資料由 dev seed 提供），店家的新欄位是 NULL，退回紀錄裡舊的欄位名被清掉，新的 CHECK 生效（信箱至多 50 字，PayUni 的上限）。
func TestContactToStoreMigrationDropsTheOldCopiesAndTightensTheChecks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_contact_to_store.sql") })
	if at < 0 {
		t.Fatalf("找不到 seller_contact_to_store 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000301, 701, 'x', now(), now())")
	exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, real_name, contact_email, contact_phone, created_at, updated_at)
		SELECT '0199bbbb-0000-7000-8000-000000000001', store_id, 'CANCELLED', 'INDIVIDUAL', '王小明', 'app@example.com', '0911111111', now(), now()
		FROM seller.stores WHERE store_code = 2600000301`)
	exec(`INSERT INTO seller.payuni_registrations (store_id, mer_id, application_id, contact_name, contact_mobile, mer_email, mer_tel, created_at, updated_at)
		SELECT store_id, 'HAWK2600000301', '0199bbbb-0000-7000-8000-000000000001', '副本', '0922222222', 'reg@example.com', '0299999999', now(), now()
		FROM seller.stores WHERE store_code = 2600000301`)
	exec(`INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, reason, required_fields, decided_at)
		VALUES (uuidv7(), '0199bbbb-0000-7000-8000-000000000001', 'REJECT', 1, '補件', ARRAY['contact_email', 'real_name', 'contact_phone'], now())`)

	exec(body(files[at]))

	var name, mobile, email *string
	must(t, d.Pool().QueryRow(ctx, "SELECT contact_name, contact_mobile, support_email FROM seller.stores WHERE store_code = 2600000301").Scan(&name, &mobile, &email))
	if name != nil || mobile != nil || email != nil {
		t.Errorf("店家的新欄位應該是 NULL：%v %v %v", name, mobile, email)
	}
	var required []string
	must(t, d.Pool().QueryRow(ctx, "SELECT required_fields FROM seller.kyc_reviews").Scan(&required))
	if !slices.Equal(required, []string{"real_name"}) {
		t.Errorf("退回要補的欄位 %v，預期只剩 real_name", required)
	}
	var columns int
	must(t, d.Pool().QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND (
		(table_name = 'kyc_applications' AND column_name IN ('contact_email', 'contact_phone')) OR
		(table_name = 'payuni_registrations' AND column_name IN ('contact_name', 'contact_mobile', 'contact_tel', 'mer_email', 'mer_tel', 'version')))`).Scan(&columns))
	if columns != 0 {
		t.Errorf("舊的聯絡欄位還在：%d 欄", columns)
	}
	for name, q := range map[string]string{
		"聯絡人超過 20 字":  "UPDATE seller.stores SET contact_name = repeat('字', 21) WHERE store_code = 2600000301",
		"聯絡手機是市話":     "UPDATE seller.stores SET contact_mobile = '0229010572' WHERE store_code = 2600000301",
		"客服信箱沒有網域":    "UPDATE seller.stores SET support_email = 'user@localhost' WHERE store_code = 2600000301",
		"客服信箱超過 50 字": "UPDATE seller.stores SET support_email = repeat('a', 42) || '@example.com' WHERE store_code = 2600000301",
	} {
		if _, err := d.Pool().Exec(ctx, q); err == nil {
			t.Errorf("%s：新的 CHECK 沒有擋下", name)
		}
	}
	exec("UPDATE seller.stores SET support_email = repeat('a', 38) || '@example.com' WHERE store_code = 2600000301")
}

// 逐項問題的 migration 套用在已有退回紀錄的資料庫上：審核紀錄保留（理由、動作都在），舊的 required_fields 欄位刪除（系統還沒上線，
// 舊的欄位名不轉成問題），新的內部備註是 NULL、問題表是空的，之後的 CHECK 生效。
func TestKYCReviewIssuesMigrationKeepsReviewsAndDropsRequiredFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_kyc_review_issues.sql") })
	if at < 1 {
		t.Fatalf("找不到 kyc_review_issues 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	for _, name := range files[:at] {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		exec(string(raw))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000401, 801, 'x', now(), now())")
	exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, real_name, created_at, updated_at)
		SELECT '0199cccc-0000-7000-8000-000000000001', store_id, 'DRAFT', 'INDIVIDUAL', '王小明', now(), now()
		FROM seller.stores WHERE store_code = 2600000401`)
	exec(`INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, reason, required_fields, decided_at)
		VALUES ('0199cccc-0000-7000-8000-000000000002', '0199cccc-0000-7000-8000-000000000001', 'REJECT', 1, '補件', ARRAY['bank_book'], now())`)

	raw, err := fs.ReadFile(Migrations(), files[at])
	must(t, err)
	exec(string(raw))

	var reason string
	var note *string
	must(t, d.Pool().QueryRow(ctx, "SELECT reason, internal_note FROM seller.kyc_reviews").Scan(&reason, &note))
	if reason != "補件" || note != nil {
		t.Errorf("審核紀錄 reason=%q internal_note=%v", reason, note)
	}
	var columns, issues int
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'kyc_reviews' AND column_name = 'required_fields'").Scan(&columns))
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM seller.kyc_review_issues").Scan(&issues))
	if columns != 0 || issues != 0 {
		t.Errorf("required_fields 欄位 %d 個、問題 %d 筆，預期都是 0", columns, issues)
	}
	exec(`INSERT INTO seller.kyc_review_issues (issue_id, review_id, position, field, code, message)
		VALUES (uuidv7(), '0199cccc-0000-7000-8000-000000000002', 0, 'bank_book', 'MISSING', '請補存摺')`)
}

// 在已有登記資料查詢（含還在 PENDING 的舊列）的資料庫上套用加 deadline_at 的 migration：既有的列回填 requested_at 加 15 分鐘，
// 才設 NOT NULL 與 CHECK，所以套用成功；之後不帶期限的插入被擋下，掃描用的部分索引存在。
func TestRegistryDeadlineMigrationBackfillsExistingChecks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_kyc_registry_deadline.sql") })
	if at < 0 {
		t.Fatalf("找不到 kyc_registry_deadline 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000300, 900, 'x', now(), now())")
	exec(`INSERT INTO seller.kyc_applications (
			application_id, store_id, status, seller_type, real_name, english_name, bank_name, bank_code, bank_account_enc, bank_account_hash,
			bank_account_hash_key, id_card_no_enc, id_card_hash, id_card_hash_key,
			submitted_at, submission_seq, created_at, updated_at)
		SELECT uuidv7(), store_id, 'SUBMITTED', 'INDIVIDUAL', '王小明', 'Wang', '臺灣銀行', '004', '\x01', decode(repeat('ab', 32), 'hex'),
			'k', '\x02', decode(repeat('cd', 32), 'hex'), 'k', now(), 1, now(), now()
		FROM seller.stores WHERE store_code = 2600000300`)
	exec(`INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq)
		SELECT uuidv7(), application_id, 1, 'SUBMIT', timestamptz '2026-10-01 00:00:00+00', 1 FROM seller.kyc_applications`)
	exec(`INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq, outcome, queried_at)
		SELECT uuidv7(), application_id, 2, 'SUBMIT', timestamptz '2026-10-02 00:00:00+00', 1, 'UNAVAILABLE', timestamptz '2026-10-02 00:10:00+00' FROM seller.kyc_applications`)
	exec(body(files[at]))
	var wrong int
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM seller.kyc_registration_checks WHERE deadline_at <> requested_at + interval '15 minutes'").Scan(&wrong))
	if wrong != 0 {
		t.Errorf("%d 列的 deadline_at 不是 requested_at 加 15 分鐘", wrong)
	}
	if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, submission_seq)
		SELECT uuidv7(), application_id, 3, 'SUBMIT', now(), 1 FROM seller.kyc_applications`); err == nil {
		t.Error("沒有 deadline_at 的插入必須被擋下")
	}
	if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.kyc_registration_checks (check_id, application_id, seq, request_kind, requested_at, deadline_at, submission_seq)
		SELECT uuidv7(), application_id, 3, 'SUBMIT', now(), now(), 1 FROM seller.kyc_applications`); err == nil {
		t.Error("deadline_at 必須晚於 requested_at")
	}
	var index string
	must(t, d.Pool().QueryRow(ctx, "SELECT indexdef FROM pg_indexes WHERE schemaname = 'seller' AND indexname = 'kyc_registration_checks_pending'").Scan(&index))
	if !strings.Contains(index, "deadline_at") || !strings.Contains(index, "PENDING") {
		t.Errorf("index=%s", index)
	}
}

// 在已有違規、修改紀錄與申覆的資料庫上套用註記與版本表的 migration：回填不比對時鐘，由 violation_changes 的版本序列重建。
// 七種資料形狀：沒改過的；申覆之前改過（含只改內部說明的修改，它不產生版本）；申覆之後又改過；只改過內部說明的；駁回申覆之後又被撤銷的；接受申覆（撤銷包在裁定裡）；沒申覆就被直接撤銷的。
// 每個違規斷言回填的版本（賣家可見的完整內容與作者、時間）、註記（內部說明的歷史）與申覆的版本。
func TestViolationNotesRevisionsMigrationBackfillsFromTheChangeRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_violation_notes_revisions.sql") })
	if at < 0 {
		t.Fatalf("找不到 seller_violation_notes_revisions 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	ts := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	ids := map[string]uuid.UUID{}
	violation := func(name string, store int64, version int64, description, statement string, endsAt time.Time, appeal string) {
		t.Helper()
		id := uuid.NewV7()
		ids[name] = id
		exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, 'x', now(), now())", 2700000000+store, 700+store)
		exec(`INSERT INTO seller.violations (id, store_id, category, severity, penalty, description, statement_to_seller, version, created_by_admin_id, created_at, updated_at)
			SELECT $1, store_id, 'FRAUD', 'HIGH', 'SUSPENSION', $2, $3, $4, 5, $5, $5 FROM seller.stores WHERE store_code = $6`,
			id, description, statement, version, ts(1, 0), 2700000000+store)
		exec(`INSERT INTO seller.suspensions (suspension_id, store_id, suspended_by_admin_id, suspended_at, source, violation_id, ends_at)
			SELECT uuidv7(), store_id, 5, $2, 'VIOLATION', $1, $3 FROM seller.stores WHERE store_code = $4`, id, ts(1, 0), endsAt, 2700000000+store)
		switch appeal {
		case "PENDING":
			exec("UPDATE seller.violations SET appeal_status = 'PENDING', appeal_reason = 'r', appeal_submitted_at = $2 WHERE id = $1", id, ts(3, 0))
		case "REJECTED":
			exec(`UPDATE seller.violations SET appeal_status = 'REJECTED', appeal_reason = 'r', appeal_submitted_at = $2, appeal_decision_reason = 'no',
				appeal_decided_by_admin_id = 6, appeal_decided_at = $3 WHERE id = $1`, id, ts(3, 0), ts(4, 0))
		case "REJECTED_THEN_REVOKED":
			exec(`UPDATE seller.violations SET appeal_status = 'REJECTED', appeal_reason = 'r', appeal_submitted_at = $2, appeal_decision_reason = 'no',
				appeal_decided_by_admin_id = 6, appeal_decided_at = $3, status = 'REVOKED', revoked_reason = 'OTHER', revoked_by_admin_id = 6, revoked_at = $4
				WHERE id = $1`, id, ts(3, 0), ts(4, 0), ts(6, 0))
		case "ACCEPTED":
			// 接受申覆的撤銷包在裁定那一次，不另外加一。
			exec(`UPDATE seller.violations SET appeal_status = 'ACCEPTED', appeal_reason = 'r', appeal_submitted_at = $2, appeal_decision_reason = 'yes',
				appeal_decided_by_admin_id = 6, appeal_decided_at = $3, status = 'REVOKED', revoked_reason = 'APPEAL_ACCEPTED', revoked_by_admin_id = 6, revoked_at = $3
				WHERE id = $1`, id, ts(3, 0), ts(4, 0))
		case "REVOKED_NO_APPEAL":
			exec(`UPDATE seller.violations SET status = 'REVOKED', revoked_reason = 'OTHER', revoked_by_admin_id = 6, revoked_at = $2 WHERE id = $1`, id, ts(6, 0))
		}
	}
	change := func(name string, version int64, admin int64, changedAt time.Time, descriptionBefore, statementBefore *string, endsBefore, endsAfter *time.Time) {
		t.Helper()
		exec(`INSERT INTO seller.violation_changes (change_id, violation_id, version, admin_id, changed_at, description_before, statement_before, ends_at_before, ends_at_after)
			VALUES (uuidv7(), $1, $2, $3, $4, $5, $6, $7, $8)`, ids[name], version, admin, changedAt, descriptionBefore, statementBefore, endsBefore, endsAfter)
	}
	text := func(s string) *string { return &s }
	pt := func(v time.Time) *time.Time { return &v }
	e0, e1 := ts(20, 0), ts(25, 0)

	// A：沒改過。
	violation("never", 1, 1, "d0", "s0", e0, "")
	// B：版本 2 改理由與到期時間，版本 3 只改內部說明，版本 4 是申覆（待裁定）。
	violation("before", 2, 4, "d1", "s1", e1, "PENDING")
	change("before", 2, 11, ts(2, 1), nil, text("s0"), pt(e0), pt(e1))
	change("before", 3, 12, ts(2, 2), text("d0"), nil, nil, nil)
	// C：版本 2 改理由，版本 3 是申覆、版本 4 是駁回，版本 5 改理由與到期時間，版本 6 只改內部說明。
	violation("after", 3, 6, "d2", "s2", e1, "REJECTED")
	change("after", 2, 11, ts(2, 1), nil, text("s0"), nil, nil)
	change("after", 5, 13, ts(5, 1), nil, text("s1"), pt(e0), pt(e1))
	change("after", 6, 14, ts(5, 2), text("d0"), nil, nil, nil)
	// D：只改過兩次內部說明。
	violation("notes", 4, 3, "d2", "s0", e0, "")
	change("notes", 2, 15, ts(2, 3), text("d0"), nil, nil, nil)
	change("notes", 3, 16, ts(2, 4), text("d1"), nil, nil, nil)
	// E：版本 2 改理由，版本 3 是申覆、版本 4 是駁回，版本 5 是 admin 撤銷（駁回之後的直接撤銷也加一）。
	violation("revoked", 5, 5, "d0", "s1", e0, "REJECTED_THEN_REVOKED")
	change("revoked", 2, 11, ts(2, 1), nil, text("s0"), nil, nil)

	// F：版本 2 改理由，版本 3 是申覆、版本 4 是接受申覆（撤銷包在裡面，不再加一）。
	violation("accepted", 6, 4, "d0", "s1", e0, "ACCEPTED")
	change("accepted", 2, 11, ts(2, 1), nil, text("s0"), nil, nil)
	// G：版本 2 改理由，版本 3 是沒有申覆的 admin 直接撤銷。
	violation("direct", 7, 3, "d0", "s1", e0, "REVOKED_NO_APPEAL")
	change("direct", 2, 11, ts(2, 1), nil, text("s0"), nil, nil)

	exec(body(files[at]))

	type revision struct {
		n         int32
		statement string
		ends      time.Time
		admin     int64
		at        time.Time
	}
	type note struct {
		body  string
		admin int64
		at    time.Time
	}
	for name, want := range map[string]struct {
		current int32
		appeal  *int32
		revs    []revision
		notes   []note
	}{
		"never": {1, nil, []revision{{1, "s0", e0, 5, ts(1, 0)}}, []note{{"d0", 5, ts(1, 0)}}},
		"before": {2, ptr(int32(2)),
			[]revision{{1, "s0", e0, 5, ts(1, 0)}, {2, "s1", e1, 11, ts(2, 1)}},
			[]note{{"d0", 5, ts(1, 0)}, {"d1", 12, ts(2, 2)}}},
		"after": {3, ptr(int32(2)),
			[]revision{{1, "s0", e0, 5, ts(1, 0)}, {2, "s1", e0, 11, ts(2, 1)}, {3, "s2", e1, 13, ts(5, 1)}},
			[]note{{"d0", 5, ts(1, 0)}, {"d2", 14, ts(5, 2)}}},
		"notes": {1, nil, []revision{{1, "s0", e0, 5, ts(1, 0)}},
			[]note{{"d0", 5, ts(1, 0)}, {"d1", 15, ts(2, 3)}, {"d2", 16, ts(2, 4)}}},
		"revoked":  {2, ptr(int32(2)), []revision{{1, "s0", e0, 5, ts(1, 0)}, {2, "s1", e0, 11, ts(2, 1)}}, []note{{"d0", 5, ts(1, 0)}}},
		"accepted": {2, ptr(int32(2)), []revision{{1, "s0", e0, 5, ts(1, 0)}, {2, "s1", e0, 11, ts(2, 1)}}, []note{{"d0", 5, ts(1, 0)}}},
		"direct":   {2, nil, []revision{{1, "s0", e0, 5, ts(1, 0)}, {2, "s1", e0, 11, ts(2, 1)}}, []note{{"d0", 5, ts(1, 0)}}},
	} {
		id := ids[name]
		var current int32
		var appeal *int32
		must(t, d.Pool().QueryRow(ctx, "SELECT current_revision, appeal_revision FROM seller.violations WHERE id = $1", id).Scan(&current, &appeal))
		if current != want.current || (appeal == nil) != (want.appeal == nil) || (appeal != nil && *appeal != *want.appeal) {
			t.Errorf("%s：current=%d appeal=%v，預期 %d %v", name, current, appeal, want.current, want.appeal)
		}
		rows, err := d.Pool().Query(ctx, "SELECT revision, statement_to_seller, sanction_ends_at, admin_id, created_at FROM seller.violation_revisions WHERE violation_id = $1 ORDER BY revision", id)
		must(t, err)
		var gotRevs []revision
		for rows.Next() {
			var r revision
			must(t, rows.Scan(&r.n, &r.statement, &r.ends, &r.admin, &r.at))
			r.ends, r.at = r.ends.UTC(), r.at.UTC()
			gotRevs = append(gotRevs, r)
		}
		must(t, rows.Err())
		if !slices.Equal(gotRevs, want.revs) {
			t.Errorf("%s 的版本：\n got %+v\nwant %+v", name, gotRevs, want.revs)
		}
		rows, err = d.Pool().Query(ctx, "SELECT body, admin_id, created_at FROM seller.violation_notes WHERE violation_id = $1 ORDER BY created_at, note_id", id)
		must(t, err)
		var gotNotes []note
		for rows.Next() {
			var n note
			must(t, rows.Scan(&n.body, &n.admin, &n.at))
			n.at = n.at.UTC()
			gotNotes = append(gotNotes, n)
		}
		must(t, rows.Err())
		if !slices.Equal(gotNotes, want.notes) {
			t.Errorf("%s 的註記：\n got %+v\nwant %+v", name, gotNotes, want.notes)
		}
	}
	var tables int
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name = 'violation_changes'").Scan(&tables))
	if tables != 0 {
		t.Error("violation_changes 應該已經刪除")
	}
}

// 回填的前提是 version 的每一次加一都有 violation_changes 解釋：version 從 20261002120004 起就有，violation_changes 要到
// 20261004021301 才出現，兩者之間的修改沒有紀錄，會被當成申覆而回填出錯誤的歷史。遇到對不上的違規，migration 整個失敗並列出
// 違規的 ID，什麼都不寫（同一個 transaction）；解釋得了的違規不受影響。
func TestViolationNotesRevisionsMigrationRefusesVersionsItCannotExplain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_violation_notes_revisions.sql") })
	if at < 0 {
		t.Fatalf("找不到 seller_violation_notes_revisions 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) error {
		_, err := d.Pool().Exec(ctx, sql, args...)
		return err
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		must(t, exec(body(name)))
	}
	at0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	ids := map[string]uuid.UUID{}
	violation := func(name string, store, version int64) {
		t.Helper()
		id := uuid.NewV7()
		ids[name] = id
		must(t, exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $2, 'x', now(), now())", 2800000000+store, 800+store))
		must(t, exec(`INSERT INTO seller.violations (id, store_id, category, severity, penalty, description, statement_to_seller, version, created_by_admin_id, created_at, updated_at)
			SELECT $1, store_id, 'FRAUD', 'HIGH', 'WARNING', 'd', 's', $2, 5, $3, $3 FROM seller.stores WHERE store_code = $4`, id, version, at0, 2800000000+store))
	}
	// 解釋得了：版本 2 有修改紀錄。
	violation("explained", 1, 2)
	must(t, exec(`INSERT INTO seller.violation_changes (change_id, violation_id, version, admin_id, changed_at, statement_before) VALUES (uuidv7(), $1, 2, 11, $2, 's0')`, ids["explained"], at0))
	// 解釋不了：版本是 3，卻只有版本 2 的修改紀錄（版本 3 的修改發生在 violation_changes 出現之前，或紀錄遺失）。
	violation("gap", 2, 3)
	must(t, exec(`INSERT INTO seller.violation_changes (change_id, violation_id, version, admin_id, changed_at, statement_before) VALUES (uuidv7(), $1, 2, 11, $2, 's0')`, ids["gap"], at0))
	// 解釋不了：版本是 2，沒有任何修改紀錄、也沒有申覆。
	violation("bare", 3, 2)

	err = exec(body(files[at]))
	if err == nil {
		t.Fatal("有無法解釋的版本時 migration 必須失敗")
	}
	for _, want := range []string{ids["gap"].String(), ids["bare"].String()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("錯誤訊息要列出無法解釋的違規 %s：%v", want, err)
		}
	}
	if strings.Contains(err.Error(), ids["explained"].String()) {
		t.Errorf("解釋得了的違規不該被列出：%v", err)
	}
	var tables int
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name IN ('violation_revisions', 'violation_notes')").Scan(&tables))
	var changes int
	must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM seller.violation_changes").Scan(&changes))
	if tables != 0 || changes != 2 {
		t.Errorf("失敗的 migration 不能留下任何東西：tables=%d violation_changes=%d", tables, changes)
	}
}

// beforeMeasuresMigration 套用「措施」migration 之前的全部 migration，回傳資料庫與套用那個 migration 的函式。
func beforeMeasuresMigration(t *testing.T) (*db.DB, func() error) {
	t.Helper()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_measures.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_measures 的 migration：%v", files)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		if _, err := d.Pool().Exec(context.Background(), body(name)); err != nil {
			t.Fatal(err)
		}
	}
	return d, func() error {
		_, err := d.Pool().Exec(context.Background(), body(files[at]))
		return err
	}
}

// legacyViolation 是套用措施的 migration 之前的一筆違規：版本、處分、申覆、佐證與身分快照。
type legacyViolation struct {
	name, penalty, status string
	storeID               int64
	version               int64
	revision              int32
	escalated             bool
	revokedReason         string
	appeal                string
	appealRevision        int32
	decisionReason        *string
	sanctionEnds          *time.Time
	sanctionLifted        bool
	statements            []string
	endsPerRevision       []*time.Time
	evidence              int
	identities            int
}

func insertLegacyViolation(t *testing.T, d *db.DB, id uuid.UUID, c legacyViolation, at func(day, hour int) time.Time) {
	t.Helper()
	ctx := context.Background()
	tx, err := d.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := tx.Exec(ctx, sql, args...)
		must(t, err)
	}
	severity, escalatedFrom := "LOW", (*string)(nil)
	if c.escalated {
		severity, escalatedFrom = "HIGH", ptr("MEDIUM")
	}
	var revoked, revokedBy, revokedAt any
	if c.revokedReason != "" {
		revoked, revokedBy, revokedAt = c.revokedReason, int64(6), at(6, 0)
	}
	var appealStatus, appealReason, appealSubmitted, appealDecided, appealDecidedBy, appealRevision, decision any
	if c.appeal != "" {
		appealStatus, appealReason, appealSubmitted, appealRevision = c.appeal, "我的申訴", at(3, 0), c.appealRevision
		if c.appeal != "PENDING" {
			appealDecided, appealDecidedBy = at(4, 0), int64(6)
			decision = c.decisionReason
		}
	}
	exec(`INSERT INTO seller.violations (id, store_id, category, severity, escalated_from, penalty, status, version, current_revision, created_by_admin_id, created_at, updated_at,
		revoked_reason, revoked_by_admin_id, revoked_at, appeal_status, appeal_reason, appeal_submitted_at, appeal_decision_reason, appeal_decided_by_admin_id, appeal_decided_at, appeal_revision)
		VALUES ($1, $2, 'FRAUD', $3, $4, $5, $6, $7, $8, 5, $9, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		id, c.storeID, severity, escalatedFrom, c.penalty, c.status, c.version, c.revision, at(1, 0),
		revoked, revokedBy, revokedAt, appealStatus, appealReason, appealSubmitted, decision, appealDecidedBy, appealDecided, appealRevision)
	for i, statement := range c.statements {
		exec(`INSERT INTO seller.violation_revisions (violation_id, revision, statement_to_seller, sanction_ends_at, admin_id, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			id, i+1, statement, c.endsPerRevision[i], int64(5+i), at(1+i, 1))
	}
	exec(`INSERT INTO seller.violation_notes (note_id, violation_id, admin_id, body, created_at) VALUES (uuidv7(), $1, 5, $2, $3)`, id, "內部註記 "+c.name, at(1, 2))
	var suspensionID uuid.UUID
	if c.penalty == "SUSPENSION" {
		suspensionID = uuid.NewV7()
		var lifted, liftedBy any
		if c.sanctionLifted {
			lifted, liftedBy = at(6, 0), int64(6)
		}
		exec(`INSERT INTO seller.suspensions (suspension_id, store_id, account_id, source, violation_id, suspended_by_admin_id, suspended_at, ends_at, lifted_at, lifted_by_admin_id)
			SELECT $1, $2, account_id, 'VIOLATION', $3, 5, $4, $5, $6, $7 FROM seller.stores WHERE store_id = $2`, suspensionID, c.storeID, id, at(1, 0), c.sanctionEnds, lifted, liftedBy)
		for i := range c.identities {
			exec(`INSERT INTO seller.suspension_identities (suspension_id, kind, key_id, hash) VALUES ($1, $2, 'k1', decode(repeat($3, 32), 'hex'))`,
				suspensionID, []string{"ID_CARD", "BANK_ACCOUNT", "TAX_ID"}[i], fmt.Sprintf("%02x", i+1))
		}
	}
	for i := range c.evidence {
		exec(`INSERT INTO seller.appeal_evidence (violation_id, position, asset_id, object_key) VALUES ($1, $2, $3, $4)`, id, i, fmt.Sprintf("asset-%d", i), fmt.Sprintf("secure/%d", i))
	}
	if c.evidence > 1 {
		exec(`INSERT INTO seller.evidence_accesses (access_id, violation_id, position, admin_id, accessed_at) VALUES (uuidv7(), $1, 1, 9, $2)`, id, at(5, 0))
	}
	must(t, tx.Commit(ctx))
}

// 在已有違規、處分、申覆與佐證的資料庫上套用措施的 migration：違規 1:1 回填成措施（id、版本、時間、撤銷紀錄照搬，認定留在
// violations），申覆搬成請求（id 沿用違規 ID；接受時沒附理由的補上照實描述的佔位文字），版本、註記、佐證與身分快照改掛措施，
// 讀取紀錄改指請求，舊的處分表刪除。
func TestMeasuresMigrationBackfillsViolationsOneToOneWithTheirIDs(t *testing.T) {
	t.Parallel()
	d, apply := beforeMeasuresMigration(t)
	ctx := context.Background()
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	ends0, ends1 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC)
	if _, err := d.Pool().Exec(ctx, "INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2800000001, 801, 'a', now(), now()), (2800000002, 802, 'b', now(), now())"); err != nil {
		t.Fatal(err)
	}
	var store1, store2 int64
	must(t, d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_code = 2800000001").Scan(&store1))
	must(t, d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_code = 2800000002").Scan(&store2))
	if _, err := d.Pool().Exec(ctx, "INSERT INTO seller.suspension_periods (store_id, started_at) VALUES ($1, $2)", store2, at(1, 0)); err != nil {
		t.Fatal(err)
	}
	cases := []legacyViolation{
		{name: "warning", penalty: "WARNING", status: "ENFORCED", storeID: store1, version: 1, revision: 1, statements: []string{"警告的理由"}, endsPerRevision: []*time.Time{nil}},
		{name: "active", penalty: "SUSPENSION", status: "ENFORCED", storeID: store1, version: 3, revision: 2, appeal: "PENDING", appealRevision: 2, sanctionEnds: &ends1,
			statements: []string{"第一版", "第二版"}, endsPerRevision: []*time.Time{&ends0, &ends1}, evidence: 2, identities: 2},
		{name: "revoked", penalty: "SUSPENSION", status: "REVOKED", storeID: store2, version: 2, revision: 1, revokedReason: "ADMIN_CORRECTION", sanctionEnds: &ends0, sanctionLifted: true,
			statements: []string{"撤銷的理由"}, endsPerRevision: []*time.Time{&ends0}},
		{name: "accepted", penalty: "SUSPENSION", status: "REVOKED", storeID: store2, version: 3, revision: 1, revokedReason: "APPEAL_ACCEPTED", appeal: "ACCEPTED", appealRevision: 1,
			sanctionEnds: &ends0, sanctionLifted: true, statements: []string{"接受申訴的理由"}, endsPerRevision: []*time.Time{&ends0}},
		{name: "rejected", penalty: "SUSPENSION", status: "ENFORCED", storeID: store2, version: 3, revision: 1, appeal: "REJECTED", appealRevision: 1, decisionReason: ptr("證據不足"),
			sanctionEnds: &ends1, statements: []string{"駁回的理由"}, endsPerRevision: []*time.Time{&ends1}},
		{name: "escalated", penalty: "WARNING", status: "ENFORCED", storeID: store2, version: 1, revision: 1, escalated: true, statements: []string{"升級的理由"}, endsPerRevision: []*time.Time{nil}},
	}
	ids := map[string]uuid.UUID{}
	for _, c := range cases {
		ids[c.name] = uuid.NewV7()
		insertLegacyViolation(t, d, ids[c.name], c, at)
	}
	must(t, apply())

	for _, c := range cases {
		id := ids[c.name]
		var kind, basis, status string
		var lead, origin uuid.UUID
		var version int64
		var current int32
		var account, findingAccount int64
		var createdAt, updatedAt time.Time
		var revoked *string
		var reviewBy, riskGround *string
		must(t, d.Pool().QueryRow(ctx, `SELECT m.kind, m.basis, m.status, m.lead_finding_id, m.version, m.current_revision, m.account_id, m.created_at, m.updated_at, m.revoked_reason, m.review_by::text, m.risk_ground,
			v.origin_measure_id, v.account_id FROM seller.measures m JOIN seller.violations v ON v.id = m.id WHERE m.id = $1`, id).
			Scan(&kind, &basis, &status, &lead, &version, &current, &account, &createdAt, &updatedAt, &revoked, &reviewBy, &riskGround, &origin, &findingAccount))
		if kind != c.penalty || basis != "VIOLATION" || status != c.status || lead != id || origin != id || version != c.version || current != c.revision ||
			!createdAt.Equal(at(1, 0)) || !updatedAt.Equal(at(1, 0)) || reviewBy != nil || riskGround != nil || account != findingAccount || (c.revokedReason == "") != (revoked == nil) ||
			(revoked != nil && *revoked != c.revokedReason) {
			t.Errorf("%s：kind=%s basis=%s status=%s lead=%v origin=%v version=%d revision=%d account=%d/%d revoked=%v", c.name, kind, basis, status, lead, origin, version, current, account, findingAccount, revoked)
		}
		if n := countOf(t, d, "SELECT count(*) FROM seller.measure_findings WHERE measure_id = $1 AND violation_id = $1", id); n != 1 {
			t.Errorf("%s：measure_findings=%d", c.name, n)
		}
		last := len(c.statements) - 1
		var currentStatement string
		var currentEnds *time.Time
		must(t, d.Pool().QueryRow(ctx, "SELECT statement_to_seller, ends_at FROM seller.measures WHERE id = $1", id).Scan(&currentStatement, &currentEnds))
		if currentStatement != c.statements[last] || (currentEnds == nil) != (c.endsPerRevision[last] == nil) || (currentEnds != nil && !currentEnds.Equal(*c.endsPerRevision[last])) {
			t.Errorf("%s：measures 的目前值 %s %v 不是最後一版 %s %v", c.name, currentStatement, currentEnds, c.statements[last], c.endsPerRevision[last])
		}
		rows, err := d.Pool().Query(ctx, "SELECT revision, statement_to_seller, ends_at FROM seller.measure_revisions WHERE measure_id = $1 ORDER BY revision", id)
		must(t, err)
		i := 0
		for rows.Next() {
			var n int32
			var statement string
			var ends *time.Time
			must(t, rows.Scan(&n, &statement, &ends))
			if int(n) != i+1 || statement != c.statements[i] || (ends == nil) != (c.endsPerRevision[i] == nil) || (ends != nil && !ends.Equal(*c.endsPerRevision[i])) {
				t.Errorf("%s 第 %d 版：%s %v", c.name, n, statement, ends)
			}
			i++
		}
		rows.Close()
		if i != len(c.statements) {
			t.Errorf("%s：版本數 %d", c.name, i)
		}
		if n := countOf(t, d, "SELECT count(*) FROM seller.measure_notes WHERE measure_id = $1 AND body = $2", id, "內部註記 "+c.name); n != 1 {
			t.Errorf("%s：註記=%d", c.name, n)
		}
		var state, reason string
		var decisionReason *string
		var decidedBy *int64
		var revision int32
		err = d.Pool().QueryRow(ctx, "SELECT state, reason, decision_reason, decided_by_admin_id, revision FROM seller.measure_requests WHERE id = $1 AND measure_id = $1 AND kind = 'APPEAL'", id).Scan(&state, &reason, &decisionReason, &decidedBy, &revision)
		if c.appeal == "" {
			if err == nil {
				t.Errorf("%s：沒有申訴卻有請求", c.name)
			}
			continue
		}
		must(t, err)
		want := c.decisionReason
		if c.appeal == "ACCEPTED" {
			want = ptr("（裁定時未記錄理由）")
		}
		if state != c.appeal || reason != "我的申訴" || revision != c.appealRevision || (want == nil) != (decisionReason == nil) || (want != nil && *want != *decisionReason) {
			t.Errorf("%s：請求 state=%s reason=%s revision=%d decision=%v", c.name, state, reason, revision, decisionReason)
		}
		if c.appeal == "PENDING" && decidedBy != nil || c.appeal != "PENDING" && (decidedBy == nil || *decidedBy != 6) {
			t.Errorf("%s：裁定者=%v", c.name, decidedBy)
		}
	}
	active := ids["active"]
	if n := countOf(t, d, "SELECT count(*) FROM seller.measure_request_evidence WHERE request_id = $1", active); n != 2 {
		t.Errorf("佐證改掛請求：%d", n)
	}
	if n := countOf(t, d, "SELECT count(*) FROM seller.evidence_accesses WHERE request_id = $1 AND position = 1 AND admin_id = 9", active); n != 1 {
		t.Errorf("讀取紀錄改指請求：%d", n)
	}
	if n := countOf(t, d, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1", active); n != 2 {
		t.Errorf("身分快照改掛措施：%d", n)
	}
	if n := countOf(t, d, "SELECT count(*) FROM seller.suspension_periods WHERE store_id = $1 AND ended_at IS NULL", store2); n != 1 {
		t.Errorf("停權期間不動：%d", n)
	}
	// 回填之後「生效中」的推導（EffectiveMeasures 的定義）：未撤銷、已開始、結束時間在未來的停權：active 與 rejected。
	if n := countOf(t, d, `SELECT count(*) FROM seller.measures m
		WHERE m.status = 'ENFORCED' AND m.kind = 'SUSPENSION' AND m.created_at <= now() AND (m.ends_at IS NULL OR m.ends_at > now())`); n != 2 {
		t.Errorf("生效中的停權=%d，預期 active 與 rejected", n)
	}
	for _, table := range []string{"suspensions", "suspension_identities", "violation_revisions", "violation_notes", "appeal_evidence"} {
		if n := countOf(t, d, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name = $1", table); n != 0 {
			t.Errorf("舊的表 %s 還在", table)
		}
	}
	if n := countOf(t, d, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'violations' AND column_name IN ('penalty', 'version', 'current_revision', 'appeal_status', 'appeal_reason')"); n != 0 {
		t.Errorf("violations 還留著措施與申訴的欄位：%d", n)
	}
}

func countOf(t *testing.T, d *db.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	must(t, d.Pool().QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

// 既有的管理員停權列（含已解除的）沒有誠實可用的風險類型（W2）：migration 整個失敗並列出每一個 ID，什麼都不改；清掉這些列
// 之後可以重跑。
func TestMeasuresMigrationRefusesAdminSuspensionRowsAndListsTheirIDs(t *testing.T) {
	t.Parallel()
	d, apply := beforeMeasuresMigration(t)
	ctx := context.Background()
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	if _, err := d.Pool().Exec(ctx, "INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2800000011, 811, 'a', now(), now())"); err != nil {
		t.Fatal(err)
	}
	var storeID int64
	must(t, d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_code = 2800000011").Scan(&storeID))
	active, lifted := uuid.NewV7(), uuid.NewV7()
	for _, row := range []struct {
		id     uuid.UUID
		lifted bool
	}{{active, false}, {lifted, true}} {
		var liftedAt, liftedBy any
		if row.lifted {
			liftedAt, liftedBy = at(3, 0), int64(6)
		}
		if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.suspensions (suspension_id, store_id, account_id, source, reason, suspended_by_admin_id, suspended_at, lifted_at, lifted_by_admin_id)
			VALUES ($1, $2, 811, 'ADMIN', 'POLICY_VIOLATION', 5, $3, $4, $5)`, row.id, storeID, at(1, 0), liftedAt, liftedBy); err != nil {
			t.Fatal(err)
		}
	}
	err := apply()
	if err == nil || !strings.Contains(err.Error(), active.String()) || !strings.Contains(err.Error(), lifted.String()) {
		t.Fatalf("migration 要失敗並列出每一個管理員停權列的 ID：%v", err)
	}
	if countOf(t, d, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name = 'measures'") != 0 ||
		countOf(t, d, "SELECT count(*) FROM seller.suspensions") != 2 {
		t.Fatal("失敗的 migration 不能留下任何改動")
	}
	if _, err := d.Pool().Exec(ctx, "DELETE FROM seller.suspensions"); err != nil {
		t.Fatal(err)
	}
	if err := apply(); err != nil {
		t.Fatalf("清掉管理員停權列之後可以重跑：%v", err)
	}
}

// 違規與它的處分列對不上就整個失敗並列出違規 ID，不寫出錯誤的歷史：結束時間與目前版本不一致、解除與否與違規是否撤銷不一致、
// 停權的違規沒有處分、警告卻有處分、停權沒有結束時間（依據認定的停權一定有結束時間）。
func TestMeasuresMigrationRefusesViolationsWhoseSanctionDoesNotMatch(t *testing.T) {
	t.Parallel()
	d, apply := beforeMeasuresMigration(t)
	ctx := context.Background()
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	ends0, ends1 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC)
	if _, err := d.Pool().Exec(ctx, "INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2800000021, 821, 'a', now(), now())"); err != nil {
		t.Fatal(err)
	}
	var storeID int64
	must(t, d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_code = 2800000021").Scan(&storeID))
	good := uuid.NewV7()
	insertLegacyViolation(t, d, good, legacyViolation{name: "good", penalty: "SUSPENSION", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		sanctionEnds: &ends0, statements: []string{"x"}, endsPerRevision: []*time.Time{&ends0}}, at)
	bad := map[string]uuid.UUID{"結束時間不一致": uuid.NewV7(), "撤銷了卻沒解除": uuid.NewV7(), "停權沒有處分": uuid.NewV7(), "警告卻有處分": uuid.NewV7(), "停權沒有結束時間": uuid.NewV7()}
	insertLegacyViolation(t, d, bad["結束時間不一致"], legacyViolation{name: "a", penalty: "SUSPENSION", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		sanctionEnds: &ends1, statements: []string{"x"}, endsPerRevision: []*time.Time{&ends0}}, at)
	insertLegacyViolation(t, d, bad["撤銷了卻沒解除"], legacyViolation{name: "b", penalty: "SUSPENSION", status: "REVOKED", revokedReason: "OTHER", storeID: storeID, version: 2, revision: 1,
		sanctionEnds: &ends0, statements: []string{"x"}, endsPerRevision: []*time.Time{&ends0}}, at)
	insertLegacyViolation(t, d, bad["停權沒有處分"], legacyViolation{name: "c", penalty: "WARNING", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		statements: []string{"x"}, endsPerRevision: []*time.Time{nil}}, at)
	if _, err := d.Pool().Exec(ctx, "UPDATE seller.violations SET penalty = 'SUSPENSION' WHERE id = $1", bad["停權沒有處分"]); err != nil {
		t.Fatal(err)
	}
	insertLegacyViolation(t, d, bad["警告卻有處分"], legacyViolation{name: "d", penalty: "SUSPENSION", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		sanctionEnds: &ends0, statements: []string{"x"}, endsPerRevision: []*time.Time{&ends0}}, at)
	if _, err := d.Pool().Exec(ctx, "UPDATE seller.violations SET penalty = 'WARNING' WHERE id = $1", bad["警告卻有處分"]); err != nil {
		t.Fatal(err)
	}
	insertLegacyViolation(t, d, bad["停權沒有結束時間"], legacyViolation{name: "e", penalty: "SUSPENSION", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		sanctionEnds: &ends0, statements: []string{"x"}, endsPerRevision: []*time.Time{nil}}, at)
	err := apply()
	if err == nil {
		t.Fatal("對不上的違規必須讓 migration 失敗")
	}
	for name, id := range bad {
		if !strings.Contains(err.Error(), id.String()) {
			t.Errorf("%s：錯誤沒有列出 %s：%v", name, id, err)
		}
	}
	if strings.Contains(err.Error(), good.String()) {
		t.Errorf("一致的違規不該被列出：%v", err)
	}
	if countOf(t, d, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name = 'measures'") != 0 {
		t.Fatal("失敗的 migration 不能留下任何改動")
	}
}

// 警告的目前版本不該有結束時間（警告沒有期間）：只有這一個條件不成立時（沒有處分列），migration 也要失敗並列出違規 ID。
func TestMeasuresMigrationRefusesWarningWhoseCurrentRevisionHasAnEndTime(t *testing.T) {
	t.Parallel()
	d, apply := beforeMeasuresMigration(t)
	ctx := context.Background()
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	ends := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := d.Pool().Exec(ctx, "INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2800000031, 831, 'a', now(), now())"); err != nil {
		t.Fatal(err)
	}
	var storeID int64
	must(t, d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_code = 2800000031").Scan(&storeID))
	good, bad := uuid.NewV7(), uuid.NewV7()
	insertLegacyViolation(t, d, good, legacyViolation{name: "good", penalty: "WARNING", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		statements: []string{"x"}, endsPerRevision: []*time.Time{nil}}, at)
	insertLegacyViolation(t, d, bad, legacyViolation{name: "bad", penalty: "WARNING", status: "ENFORCED", storeID: storeID, version: 1, revision: 1,
		statements: []string{"x"}, endsPerRevision: []*time.Time{&ends}}, at)
	if n := countOf(t, d, "SELECT count(*) FROM seller.suspensions WHERE violation_id = $1", bad); n != 0 {
		t.Fatalf("這個案例不該有處分列：%d", n)
	}
	err := apply()
	if err == nil || !strings.Contains(err.Error(), bad.String()) {
		t.Fatalf("警告的目前版本有結束時間必須讓 migration 失敗並列出 ID：%v", err)
	}
	if strings.Contains(err.Error(), good.String()) {
		t.Errorf("一致的警告不該被列出：%v", err)
	}
}

// 解除請求的 migration 套用在已有請求的資料庫上：basis 由措施回填（複合外鍵之後要求兩者一致）、既有的 CLOSED 回填成 REVOKED（它們都是措施
// 撤銷時結案的，都有 admin），待處理與其他狀態的 close_cause 維持空；之後解除請求才能寫入（只限 RISK）。
func TestRemediationMigrationBackfillsBasisAndCloseCause(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_measure_remediation.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_measure_remediation 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		exec(body(name))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000400, 901, 'x', now(), now())")
	tx, err := d.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, id := range []string{"00000000-0000-7000-8000-000000000001", "00000000-0000-7000-8000-000000000002"} {
		for _, sql := range []string{
			`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at)
				SELECT $1::uuid, account_id, store_id, 'SUSPENSION', 'RISK', 'FRAUD', now() + interval '7 days', 's', 1, 1, now() - interval '1 hour', now() - interval '1 hour'
				FROM seller.stores WHERE store_code = 2600000400`,
			`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 1, 's', 1, now() - interval '1 hour')`,
		} {
			_, err := tx.Exec(ctx, sql, id)
			must(t, err)
		}
	}
	for _, sql := range []string{
		`INSERT INTO seller.violations (id, store_id, account_id, category, severity, status, origin_measure_id, created_by_admin_id, created_at, updated_at)
			SELECT '00000000-0000-7000-8000-000000000003'::uuid, store_id, account_id, 'FRAUD', 'LOW', 'ENFORCED', '00000000-0000-7000-8000-000000000003'::uuid, 1, now() - interval '1 hour', now() - interval '1 hour'
			FROM seller.stores WHERE store_code = 2600000400`,
		`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, lead_finding_id, statement_to_seller, ends_at, current_revision, created_by_admin_id, created_at, updated_at)
			SELECT '00000000-0000-7000-8000-000000000003'::uuid, account_id, store_id, 'SUSPENSION', 'VIOLATION', '00000000-0000-7000-8000-000000000003'::uuid, 's',
			       now() + interval '10 days', 1, 1, now() - interval '1 hour', now() - interval '1 hour'
			FROM seller.stores WHERE store_code = 2600000400`,
		`INSERT INTO seller.measure_findings (measure_id, violation_id) VALUES ('00000000-0000-7000-8000-000000000003', '00000000-0000-7000-8000-000000000003')`,
		`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at)
			VALUES ('00000000-0000-7000-8000-000000000003', 1, 's', now() + interval '10 days', 1, now() - interval '1 hour')`,
	} {
		_, err := tx.Exec(ctx, sql)
		must(t, err)
	}
	must(t, tx.Commit(ctx))
	exec(`INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at)
		VALUES (uuidv7(), '00000000-0000-7000-8000-000000000001', 'APPEAL', 'PENDING', 'r', 1, now() - interval '30 minutes')`)
	exec(`INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, decision_reason, decided_by_admin_id, decided_at)
		VALUES (uuidv7(), '00000000-0000-7000-8000-000000000003', 'APPEAL', 'REJECTED', 'r', 1, now() - interval '30 minutes', 'x', 5, now() - interval '10 minutes')`)
	exec(`INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, decided_by_admin_id, decided_at)
		VALUES (uuidv7(), '00000000-0000-7000-8000-000000000002', 'APPEAL', 'CLOSED', 'r', 1, now() - interval '30 minutes', 5, now() - interval '10 minutes')`)
	exec(body(files[at]))
	rows, err := d.Pool().Query(ctx, "SELECT state, basis, close_cause FROM seller.measure_requests ORDER BY state, basis")
	must(t, err)
	defer rows.Close()
	type row struct {
		state, basis string
		cause        *string
	}
	var got []row
	for rows.Next() {
		var r row
		must(t, rows.Scan(&r.state, &r.basis, &r.cause))
		got = append(got, r)
	}
	must(t, rows.Err())
	if len(got) != 3 || got[0].state != "CLOSED" || got[0].basis != "RISK" || got[0].cause == nil || *got[0].cause != "REVOKED" ||
		got[1].state != "PENDING" || got[1].basis != "RISK" || got[1].cause != nil ||
		got[2].state != "REJECTED" || got[2].basis != "VIOLATION" || got[2].cause != nil {
		t.Fatalf("rows=%+v", got)
	}
	exec(`UPDATE seller.measure_requests SET state = 'REJECTED', decision_reason = 'x', decided_by_admin_id = 5, decided_at = now() WHERE state = 'PENDING'`)
	exec(`INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
		VALUES (uuidv7(), '00000000-0000-7000-8000-000000000001', 'REMEDIATION', 'PENDING', 'r', 1, now(), 'RISK')`)
}

// 終止合作的 migration：既有的管理員關店列（closed_by = 'ADMIN'）沒有對應的終止措施，整個 migration 失敗並列出 store_id（先例：suspensions.source = 'ADMIN'）；
// 處理掉之後套用成功：既有措施的 starts_at 回填成 created_at，其他關店不動、管理員與理由欄位消失。
func TestTerminationMigrationRefusesAdminClosedStoresAndBackfillsStartsAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_termination.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_termination 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) error {
		_, err := d.Pool().Exec(ctx, sql, args...)
		return err
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		must(t, exec(body(name)))
	}
	must(t, exec(`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000500, 911, 'open', now(), now())`))
	must(t, exec(`INSERT INTO seller.stores (store_code, account_id, store_name, status, deleted_at, closed_by, created_at, updated_at)
		VALUES (2600000501, 912, 'owner', 'CLOSED', now(), 'ACCOUNT_OWNER', now(), now())`))
	must(t, exec(`INSERT INTO seller.stores (store_code, account_id, store_name, status, deleted_at, closed_by, closed_by_admin_id, close_reason, created_at, updated_at)
		VALUES (2600000502, 913, 'admin', 'CLOSED', now(), 'ADMIN', 9, '違規經營', now(), now())`))
	var adminStore int64
	must(t, d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_code = 2600000502").Scan(&adminStore))
	tx, err := d.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, sql := range []string{
		`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at)
			SELECT '00000000-0000-7000-8000-000000000011'::uuid, account_id, store_id, 'SUSPENSION', 'RISK', 'FRAUD', now() + interval '7 days', 's', 1, 1, now() - interval '3 hours', now() - interval '3 hours'
			FROM seller.stores WHERE store_code = 2600000500`,
		`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ('00000000-0000-7000-8000-000000000011', 1, 's', 1, now() - interval '3 hours')`,
	} {
		_, err := tx.Exec(ctx, sql)
		must(t, err)
	}
	must(t, tx.Commit(ctx))
	err = exec(body(files[at]))
	if err == nil || !strings.Contains(err.Error(), "closed_by = ADMIN") || !strings.Contains(err.Error(), strconv.FormatInt(adminStore, 10)) {
		t.Fatalf("管理員關店的列要讓 migration 失敗並列出 store_id %d：%v", adminStore, err)
	}
	must(t, exec(`DELETE FROM seller.stores WHERE store_id = $1`, adminStore))
	must(t, exec(body(files[at])))
	var startsAt, createdAt time.Time
	must(t, d.Pool().QueryRow(ctx, "SELECT starts_at, created_at FROM seller.measures WHERE id = '00000000-0000-7000-8000-000000000011'").Scan(&startsAt, &createdAt))
	if !startsAt.Equal(createdAt) {
		t.Fatalf("starts_at=%v created_at=%v：既有的措施建立即生效", startsAt, createdAt)
	}
	var closedBy *string
	must(t, d.Pool().QueryRow(ctx, "SELECT closed_by FROM seller.stores WHERE store_code = 2600000501").Scan(&closedBy))
	if closedBy == nil || *closedBy != "ACCOUNT_OWNER" {
		t.Fatalf("賣家自己關的店不動：%v", closedBy)
	}
	for _, column := range []string{"closed_by_admin_id", "close_reason"} {
		var n int
		must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='seller' AND table_name='stores' AND column_name=$1", column).Scan(&n))
		if n != 0 {
			t.Errorf("stores.%s 應該已刪除", column)
		}
	}
	if err := exec(`UPDATE seller.stores SET closed_by = 'ADMIN' WHERE store_code = 2600000501`); err == nil {
		t.Fatal("ADMIN 已不是合法的關店原因")
	}
	if err := exec(`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, system_cause, created_at)
		VALUES ('00000000-0000-7000-8000-000000000011', 2, 's', NULL, NULL, now())`); err == nil {
		t.Fatal("沒有管理員也沒有系統原因的版本要被擋下")
	}
}

// 關店紀錄併入 store_closures 的 migration：既有已關閉的 stint 回填一列目前的紀錄（closed_at 是原本的 deleted_at、原因與措施照搬），
// 已恢復的歷史列照舊，營業中的 stint 沒有目前的列；stores 的三個關店欄位消失、一個 stint 只能有一列目前的紀錄。
func TestStoreClosuresMigrationBackfillsTheCurrentClosure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_store_closures_single_record.sql") })
	if at < 1 {
		t.Fatalf("找不到 store_closures 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) error {
		_, err := d.Pool().Exec(ctx, sql, args...)
		return err
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		must(t, exec(body(name)))
	}
	closedAt := time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	must(t, exec(`INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000600, 921, 'open', now(), now())`))
	must(t, exec(`INSERT INTO seller.stores (store_code, account_id, store_name, status, deleted_at, closed_by, created_at, updated_at)
		VALUES (2600000601, 922, 'owner', 'CLOSED', $1, 'ACCOUNT_OWNER', now(), now())`, closedAt))
	must(t, exec(`INSERT INTO seller.stores (store_code, account_id, store_name, status, deleted_at, closed_by, created_at, updated_at)
		VALUES (2600000602, 923, 'deletion', 'CLOSED', $1, 'ACCOUNT_DELETION', now(), now())`, closedAt.Add(time.Hour)))
	must(t, exec(`INSERT INTO seller.store_closures (store_id, closed_at, closed_by, reopened_at)
		SELECT store_id, $1, 'ACCOUNT_OWNER', $2 FROM seller.stores WHERE store_code = 2600000601`, closedAt.Add(-48*time.Hour), closedAt.Add(-24*time.Hour)))
	must(t, exec(body(files[at])))
	type row struct {
		code     int64
		closedAt time.Time
		by       string
		reopened *time.Time
	}
	rows, err := d.Pool().Query(ctx, `SELECT s.store_code, c.closed_at, c.closed_by, c.reopened_at FROM seller.store_closures c JOIN seller.stores s USING (store_id) ORDER BY s.store_code, c.closed_at`)
	must(t, err)
	defer rows.Close()
	var got []row
	for rows.Next() {
		var r row
		must(t, rows.Scan(&r.code, &r.closedAt, &r.by, &r.reopened))
		got = append(got, r)
	}
	must(t, rows.Err())
	if len(got) != 3 ||
		got[0].code != 2600000601 || !got[0].closedAt.Equal(closedAt.Add(-48*time.Hour)) || got[0].reopened == nil ||
		got[1].code != 2600000601 || !got[1].closedAt.Equal(closedAt) || got[1].by != "ACCOUNT_OWNER" || got[1].reopened != nil ||
		got[2].code != 2600000602 || !got[2].closedAt.Equal(closedAt.Add(time.Hour)) || got[2].by != "ACCOUNT_DELETION" || got[2].reopened != nil {
		t.Fatalf("closures=%+v", got)
	}
	for _, column := range []string{"deleted_at", "closed_by", "closed_by_measure_id"} {
		var n int
		must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='seller' AND table_name='stores' AND column_name=$1", column).Scan(&n))
		if n != 0 {
			t.Errorf("stores.%s 應該已刪除", column)
		}
	}
	if err := exec(`INSERT INTO seller.store_closures (store_id, closed_at, closed_by) SELECT store_id, now(), 'ACCOUNT_OWNER' FROM seller.stores WHERE store_code = 2600000601`); err == nil {
		t.Fatal("一個 stint 同時只能有一筆目前的關店紀錄")
	}
}

// 身分比對索引的 migration：既有申請已存的 hash 與金鑰 ID 回填成一列（欄位對應：身分證號、負責人身分證號、統編、銀行帳號），沒有值的欄位沒有列；
// 之後換鑰新增的金鑰由 RehashKYCIdentities 補。
func TestKYCIdentityHashesMigrationBackfillsTheStoredHashes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_kyc_identity_hashes.sql") })
	if at < 1 {
		t.Fatalf("找不到 kyc_identity_hashes 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	for _, name := range files[:at] {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		exec(string(raw))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000610, 931, 'x', now(), now())")
	exec(`INSERT INTO seller.kyc_applications (
			application_id, store_id, status, seller_type, real_name, english_name, bank_name, bank_code, bank_account_enc, bank_account_hash,
			bank_account_hash_key, id_card_no_enc, id_card_hash, id_card_hash_key, submitted_at, submission_seq, created_at, updated_at)
		SELECT uuidv7(), store_id, 'SUBMITTED', 'INDIVIDUAL', '王小明', 'Wang', '臺灣銀行', '004', '\x01', decode(repeat('ab', 32), 'hex'),
			'k1', '\x02', decode(repeat('cd', 32), 'hex'), 'k2', now(), 1, now(), now()
		FROM seller.stores WHERE store_code = 2600000610`)
	raw, err := fs.ReadFile(Migrations(), files[at])
	must(t, err)
	exec(string(raw))
	rows, err := d.Pool().Query(ctx, `SELECT field, key_id, encode(hash, 'hex') FROM seller.kyc_identity_hashes ORDER BY field`)
	must(t, err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var field, key, hash string
		must(t, rows.Scan(&field, &key, &hash))
		got = append(got, field+"/"+key+"/"+hash[:4])
	}
	must(t, rows.Err())
	if want := []string{"BANK_ACCOUNT/k1/abab", "ID_CARD_NO/k2/cdcd"}; !slices.Equal(got, want) {
		t.Fatalf("hashes=%v want %v", got, want)
	}
}

// 政策分類的 migration（platform/policy）：UNAUTHORIZED_SALE 在通報與認定都改名 UNLICENSED_SALE，其他值不動；通報的 CHECK 不再接受
// PLATFORM_ABUSE、認定的 CHECK 不再接受 OTHER。認定的類別是 OTHER 的列沒有誠實的對應，整個 migration 失敗並列出認定的 ID，什麼都不改。
func TestPolicyCategoryMigrationRenamesTheValueAndRefusesOtherFindings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_policy_category.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_policy_category 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) error {
		_, err := d.Pool().Exec(ctx, sql, args...)
		return err
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		must(t, exec(body(name)))
	}
	must(t, exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000700, 941, 'x', now(), now())"))
	// 認定的建立措施（origin_measure）與這個 migration 無關：丟掉外鍵，免得為了認定先造措施。
	must(t, exec("ALTER TABLE seller.violations DROP CONSTRAINT violations_origin_measure_fkey"))
	report := func(buyer int64, category string) {
		t.Helper()
		must(t, exec(`INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at)
			SELECT uuidv7(), store_id, $2, 'x', $1, now(), now() FROM seller.stores WHERE store_code = 2600000700`, buyer, category))
	}
	finding := func(id, category string) {
		t.Helper()
		must(t, exec(`INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at)
			SELECT $1::uuid, s.store_id, s.account_id, $2, 'LOW', uuidv7(), 7, now(), now() FROM seller.stores s WHERE s.store_code = 2600000700`, id, category))
	}
	report(1, "UNAUTHORIZED_SALE")
	report(2, "FRAUD")
	report(3, "OTHER") // 通報的 OTHER 是檢舉人選的，合法，不動
	finding("0199aaaa-0000-7000-8000-000000000001", "UNAUTHORIZED_SALE")
	finding("0199aaaa-0000-7000-8000-000000000002", "FRAUD")
	otherFinding := "0199aaaa-0000-7000-8000-000000000003"
	finding(otherFinding, "OTHER")

	err = exec(body(files[at]))
	if err == nil {
		t.Fatal("有類別是 OTHER 的認定時 migration 必須失敗")
	}
	if !strings.Contains(err.Error(), otherFinding) {
		t.Errorf("錯誤訊息要列出類別是 OTHER 的認定 %s：%v", otherFinding, err)
	}
	for _, unrelated := range []string{"0199aaaa-0000-7000-8000-000000000001", "0199aaaa-0000-7000-8000-000000000002"} {
		if strings.Contains(err.Error(), unrelated) {
			t.Errorf("類別不是 OTHER 的認定不該被列出：%v", err)
		}
	}
	if n := countOf(t, d, "SELECT count(*) FROM seller.reports WHERE category = 'UNAUTHORIZED_SALE'") + countOf(t, d, "SELECT count(*) FROM seller.violations WHERE category = 'UNAUTHORIZED_SALE'"); n != 2 {
		t.Fatalf("失敗的 migration 不能改任何列：UNAUTHORIZED_SALE 剩 %d 列，預期 2 列", n)
	}

	// 人工處理掉 OTHER 的認定之後可以重跑。
	must(t, exec("DELETE FROM seller.violations WHERE id = $1::uuid", otherFinding))
	must(t, exec(body(files[at])))
	for _, table := range []string{"reports", "violations"} {
		var renamed, old int64
		must(t, d.Pool().QueryRow(ctx, "SELECT count(*) FILTER (WHERE category = 'UNLICENSED_SALE'), count(*) FILTER (WHERE category = 'UNAUTHORIZED_SALE') FROM seller."+table).Scan(&renamed, &old))
		if renamed != 1 || old != 0 {
			t.Errorf("%s：UNLICENSED_SALE=%d UNAUTHORIZED_SALE=%d，預期 1 與 0", table, renamed, old)
		}
		if n := countOf(t, d, "SELECT count(*) FROM seller."+table+" WHERE category = 'FRAUD'"); n != 1 {
			t.Errorf("%s：FRAUD 的列不能被動到：%d", table, n)
		}
	}
	if n := countOf(t, d, "SELECT count(*) FROM seller.reports WHERE category = 'OTHER'"); n != 1 {
		t.Errorf("通報的 OTHER 要保留：%d", n)
	}

	// 兩個 CHECK 各接受自己的子集：通報不接受 PLATFORM_ABUSE，認定不接受 OTHER；舊值兩邊都不再接受。
	for name, tt := range map[string]struct {
		sql      string
		accepted bool
	}{
		"通報可以是 IP_INFRINGEMENT":    {"INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at) SELECT uuidv7(), store_id, 'IP_INFRINGEMENT', 'x', 9, now(), now() FROM seller.stores", true},
		"通報不能是 PLATFORM_ABUSE":     {"INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at) SELECT uuidv7(), store_id, 'PLATFORM_ABUSE', 'x', 9, now(), now() FROM seller.stores", false},
		"通報不能再是 UNAUTHORIZED_SALE": {"INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at) SELECT uuidv7(), store_id, 'UNAUTHORIZED_SALE', 'x', 9, now(), now() FROM seller.stores", false},
		"認定可以是 PLATFORM_ABUSE":     {"INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at) SELECT uuidv7(), store_id, account_id, 'PLATFORM_ABUSE', 'LOW', uuidv7(), 7, now(), now() FROM seller.stores", true},
		"認定不能是 OTHER":              {"INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at) SELECT uuidv7(), store_id, account_id, 'OTHER', 'LOW', uuidv7(), 7, now(), now() FROM seller.stores", false},
		"認定不能再是 UNAUTHORIZED_SALE": {"INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at) SELECT uuidv7(), store_id, account_id, 'UNAUTHORIZED_SALE', 'LOW', uuidv7(), 7, now(), now() FROM seller.stores", false},
	} {
		if err := exec(tt.sql); (err == nil) != tt.accepted {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}

// 通報與案件分開（seller_report_cases）之前的資料庫：套用到它的前一個 migration 為止，備好 A、B 兩家店；apply 套用 seller_report_cases。
// 通報用舊的狀態機（PENDING／INVESTIGATING／VERIFIED／DISMISSED）寫入，時間以 2026-10-01 起算的小時表示（0：沒有）。
type beforeReportCasesDB struct {
	t     *testing.T
	d     *db.DB
	apply func() error
	buyer int64
}

const (
	reportCasesStoreA = 2600000801
	reportCasesStoreB = 2600000802
)

func beforeReportCases(t *testing.T) *beforeReportCasesDB {
	t.Helper()
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_report_cases.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_report_cases 的 migration：%v", files)
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	b := &beforeReportCasesDB{t: t, d: pg.NewDB(t)}
	for _, name := range files[:at] {
		must(t, b.exec(body(name)))
	}
	must(t, b.exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000801, 951, 'x', now(), now()), (2600000802, 952, 'x', now(), now())"))
	b.apply = func() error { return b.exec(body(files[at])) }
	return b
}

func (b *beforeReportCasesDB) exec(sql string, args ...any) error {
	_, err := b.d.Pool().Exec(context.Background(), sql, args...)
	return err
}

func reportCasesTime(hour int) time.Time {
	return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(hour) * time.Hour)
}

type legacyReport struct {
	ID           string
	Store        int64  // stores.store_code
	Category     string // 政策分類的值
	Status       string // 舊的狀態
	Severity     string // 空字串：NULL
	AckHour      int    // 確認收件的時間；0：沒有
	AckBy        int64
	ResolvedHour int // 結案（成立或不成立）的時間；0：沒有
	ResolvedBy   int64
	Note         string // resolution_note；空字串：NULL
	FDA          string // "<channel>/<reference_no>"；空字串：沒有轉報
	FDAHour      int
	FDABy        int64
	UpdatedHour  int // 0：和建立時間（第 1 小時）相同
}

// insert 寫入一筆舊格式的通報。建立時間是第 1 小時、通報人是一個各自不同的登入買家（避開每日去重的唯一索引）；調查中與成立的通報
// 記調查時間（確認收件後 1 小時），符合舊的 reports_status_times_check。
func (b *beforeReportCasesDB) insert(r legacyReport) {
	b.t.Helper()
	b.buyer++
	hour := func(h int) *time.Time {
		if h == 0 {
			return nil
		}
		v := reportCasesTime(h)
		return &v
	}
	text := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	num := func(n int64) *int64 {
		if n == 0 {
			return nil
		}
		return &n
	}
	var investigating *time.Time
	if r.Status == "INVESTIGATING" || r.Status == "VERIFIED" {
		investigating = hour(r.AckHour + 1)
	}
	var channel, reference *string
	if ch, ref, ok := strings.Cut(r.FDA, "/"); ok {
		channel, reference = text(ch), text(ref)
	}
	updated := r.UpdatedHour
	if updated == 0 {
		updated = 1
	}
	must(b.t, b.exec(`INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, status, severity,
			acknowledged_at, acknowledged_by_admin_id, investigating_at, resolved_at, resolved_by_admin_id, resolution_note,
			fda_reported_at, fda_channel, fda_reference_no, fda_reported_by_admin_id, created_at, updated_at)
		SELECT $1::uuid, store_id, $3::text, 'x', $4::bigint, $5::text, $6::text,
			$7::timestamptz, $8::bigint, $9::timestamptz, $10::timestamptz, $11::bigint, $12::text,
			$13::timestamptz, $14::text, $15::text, $16::bigint, $17::timestamptz, $18::timestamptz
		FROM seller.stores WHERE store_code = $2::bigint`,
		r.ID, r.Store, r.Category, 1000+b.buyer, r.Status, text(r.Severity),
		hour(r.AckHour), num(r.AckBy), investigating, hour(r.ResolvedHour), num(r.ResolvedBy), text(r.Note),
		hour(r.FDAHour), channel, reference, num(r.FDABy), reportCasesTime(1), reportCasesTime(updated)))
}

// 每種舊狀態逐欄斷言設計 §2.3 的表：同店同類別的進行中通報合併成一個案件（嚴重度取最嚴重的、開案取最早的確認收件並在同時間取 id 小的、
// updated_at 取成員最晚的）、OTHER 類別的確認收件與調查中退回待分案、查證屬實每筆一個已結案的案件、不成立只留結果、FDA 轉報搬到案件；
// 既有措施的 source_case_id 是 NULL，配合要求與佐證不動，舊欄位消失，所有通報 version = 1、discarded_submissions = 0。
func TestReportCasesMigrationMapsEveryStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := beforeReportCases(t)
	const A, B = reportCasesStoreA, reportCasesStoreB
	id := func(n int) string { return fmt.Sprintf("0199cccc-0000-7000-8000-%012d", n) }
	for _, r := range []legacyReport{
		// 1：待處理而沒確認收件 → 待分案。
		{ID: id(1), Store: A, Category: "FRAUD", Status: "PENDING"},
		// 2、3、4：同店同類別的三筆進行中（待處理已確認收件、調查中）→ 一個案件。2、3 的確認收件時間相同：開案者取 id 小的 2（管理員 12），不是管理員編號小的 3（管理員 11）。
		{ID: id(2), Store: A, Category: "FRAUD", Status: "PENDING", Severity: "LOW", AckHour: 10, AckBy: 12, UpdatedHour: 20},
		{ID: id(3), Store: A, Category: "FRAUD", Status: "INVESTIGATING", Severity: "HIGH", AckHour: 10, AckBy: 11, UpdatedHour: 30},
		{ID: id(4), Store: A, Category: "FRAUD", Status: "PENDING", Severity: "MEDIUM", AckHour: 15, AckBy: 13, UpdatedHour: 25},
		// 5、6：類別是 OTHER 的已確認收件與調查中 → 待分案（確認收件與嚴重度不留：它們從沒判定過政策類別）。
		{ID: id(5), Store: A, Category: "OTHER", Status: "PENDING", Severity: "MEDIUM", AckHour: 10, AckBy: 14},
		{ID: id(6), Store: A, Category: "OTHER", Status: "INVESTIGATING", Severity: "HIGH", AckHour: 11, AckBy: 14},
		// 7、8：查證屬實，每筆一個已結案的案件（兩筆同店同類別，不合併）。
		{ID: id(7), Store: A, Category: "FRAUD", Status: "VERIFIED", Severity: "CRITICAL", AckHour: 5, AckBy: 15, ResolvedHour: 40, ResolvedBy: 21, Note: "v1"},
		{ID: id(8), Store: A, Category: "FRAUD", Status: "VERIFIED", Severity: "LOW", AckHour: 6, AckBy: 16, ResolvedHour: 41, ResolvedBy: 22},
		// 9、10：不成立 → 沒有案件，結果是未違規；結案者與說明保留。
		{ID: id(9), Store: A, Category: "FRAUD", Status: "DISMISSED", Severity: "LOW", AckHour: 7, AckBy: 17, ResolvedHour: 42, ResolvedBy: 23, Note: "d1"},
		{ID: id(10), Store: A, Category: "COUNTERFEIT", Status: "DISMISSED", ResolvedHour: 43, ResolvedBy: 24},
		// 11、12：食安類別的進行中，11 有 FDA 轉報 → 一個案件，轉報搬到案件。
		{ID: id(11), Store: A, Category: "FOOD_SAFETY", Status: "INVESTIGATING", Severity: "HIGH", AckHour: 8, AckBy: 18, FDA: "HOTLINE_1919/F-1", FDAHour: 9, FDABy: 31, UpdatedHour: 50},
		{ID: id(12), Store: A, Category: "FOOD_SAFETY", Status: "PENDING", Severity: "LOW", AckHour: 9, AckBy: 19, UpdatedHour: 45},
		// 13：食安類別查證屬實並有 FDA 轉報 → 已結案的案件帶轉報。
		{ID: id(13), Store: A, Category: "FOOD_SAFETY", Status: "VERIFIED", Severity: "MEDIUM", AckHour: 3, AckBy: 20, ResolvedHour: 44, ResolvedBy: 25, Note: "v3", FDA: "ONLINE_FORM/F-2", FDAHour: 4, FDABy: 32},
		// 14：另一家店同類別 → 自己的案件；updated_at 早於確認收件（舊資料可以），案件的 updated_at 取確認收件的時間（案件的 CHECK 要求不早於開案）。
		{ID: id(14), Store: B, Category: "FRAUD", Status: "PENDING", Severity: "MEDIUM", AckHour: 12, AckBy: 26},
		// 15：另一家店沒確認收件 → 待分案。
		{ID: id(15), Store: B, Category: "PROHIBITED_GOODS", Status: "PENDING"},
	} {
		b.insert(r)
	}
	// 既有的措施（以前沒有案件，導出關係由認定推導）、配合要求（在進案件的通報上）、佐證。
	tx, err := b.d.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, sql := range []string{
		`INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, starts_at, current_revision, created_by_admin_id, created_at, updated_at)
			SELECT '0199cccc-0000-7000-8000-00000000f001'::uuid, account_id, store_id, 'SUSPENSION', 'RISK', 'FRAUD', now() + interval '7 days', 's', now() - interval '1 hour', 1, 1, now() - interval '1 hour', now() - interval '1 hour'
			FROM seller.stores WHERE store_code = 2600000801`,
		`INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ('0199cccc-0000-7000-8000-00000000f001', 1, 's', 1, now() - interval '1 hour')`,
	} {
		_, err := tx.Exec(ctx, sql)
		must(t, err)
	}
	must(t, tx.Commit(ctx))
	must(t, b.exec(`INSERT INTO seller.report_cooperation_requests (report_id, template, requested_by_admin_id, requested_at)
		VALUES ($1::uuid, 'ASSIST_INVESTIGATION', 5, now()), ($2::uuid, 'TEMPORARY_TAKEDOWN_SELF_CHECK', 5, now())`, id(3), id(7)))
	must(t, b.exec(`INSERT INTO seller.report_evidence (report_id, position, asset_id, object_key) VALUES ($1::uuid, 0, 'a', 'k')`, id(2)))

	must(t, b.apply())

	type reportRow struct {
		caseID, assessed, outcome, note *string
		triagedAt, resolvedAt           *time.Time
		triagedBy, resolvedBy           *int64
		version                         int64
		discarded                       int32
	}
	got := map[string]reportRow{}
	rows, err := b.d.Pool().Query(ctx, `SELECT id::text, case_id::text, assessed_category, triaged_at, triaged_by_admin_id, outcome, resolved_at, resolved_by_admin_id, resolution_note, version, discarded_submissions
		FROM seller.reports`)
	must(t, err)
	for rows.Next() {
		var key string
		var r reportRow
		must(t, rows.Scan(&key, &r.caseID, &r.assessed, &r.triagedAt, &r.triagedBy, &r.outcome, &r.resolvedAt, &r.resolvedBy, &r.note, &r.version, &r.discarded))
		got[key] = r
	}
	must(t, rows.Err())
	rows.Close()
	if len(got) != 15 {
		t.Fatalf("轉換不能增減通報：%d 筆，預期 15", len(got))
	}

	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	num := func(p *int64) int64 {
		if p == nil {
			return 0
		}
		return *p
	}
	hourOf := func(p *time.Time) int {
		if p == nil {
			return 0
		}
		return int(p.Sub(reportCasesTime(0)) / time.Hour)
	}
	type wantReport struct {
		group        int // 同一個案件的通報用同一個代表編號；0：沒有案件
		assessed     string
		triagedHour  int
		triagedBy    int64
		outcome      string
		resolvedHour int
		resolvedBy   int64
		note         string
	}
	want := map[int]wantReport{
		1:  {},
		2:  {group: 2, assessed: "FRAUD", triagedHour: 10, triagedBy: 12},
		3:  {group: 2, assessed: "FRAUD", triagedHour: 10, triagedBy: 11},
		4:  {group: 2, assessed: "FRAUD", triagedHour: 15, triagedBy: 13},
		5:  {},
		6:  {},
		7:  {group: 7, assessed: "FRAUD", triagedHour: 5, triagedBy: 15, outcome: "ACTION_TAKEN", resolvedHour: 40, resolvedBy: 21, note: "v1"},
		8:  {group: 8, assessed: "FRAUD", triagedHour: 6, triagedBy: 16, outcome: "ACTION_TAKEN", resolvedHour: 41, resolvedBy: 22},
		9:  {outcome: "NO_VIOLATION", resolvedHour: 42, resolvedBy: 23, note: "d1"},
		10: {outcome: "NO_VIOLATION", resolvedHour: 43, resolvedBy: 24},
		11: {group: 11, assessed: "FOOD_SAFETY", triagedHour: 8, triagedBy: 18},
		12: {group: 11, assessed: "FOOD_SAFETY", triagedHour: 9, triagedBy: 19},
		13: {group: 13, assessed: "FOOD_SAFETY", triagedHour: 3, triagedBy: 20, outcome: "ACTION_TAKEN", resolvedHour: 44, resolvedBy: 25, note: "v3"},
		14: {group: 14, assessed: "FRAUD", triagedHour: 12, triagedBy: 26},
		15: {},
	}
	caseIDs := map[int]string{}
	for n := 1; n <= 15; n++ {
		g, w := got[id(n)], want[n]
		if g.version != 1 || g.discarded != 0 {
			t.Errorf("通報 %d：version=%d discarded_submissions=%d，預期 1 與 0", n, g.version, g.discarded)
		}
		if str(g.assessed) != w.assessed || hourOf(g.triagedAt) != w.triagedHour || num(g.triagedBy) != w.triagedBy {
			t.Errorf("通報 %d 的分案：assessed=%q triaged=%d/%d，預期 %q %d/%d", n, str(g.assessed), hourOf(g.triagedAt), num(g.triagedBy), w.assessed, w.triagedHour, w.triagedBy)
		}
		if str(g.outcome) != w.outcome || hourOf(g.resolvedAt) != w.resolvedHour || num(g.resolvedBy) != w.resolvedBy || str(g.note) != w.note {
			t.Errorf("通報 %d 的結果：outcome=%q resolved=%d/%d note=%q，預期 %q %d/%d %q", n, str(g.outcome), hourOf(g.resolvedAt), num(g.resolvedBy), str(g.note), w.outcome, w.resolvedHour, w.resolvedBy, w.note)
		}
		if (w.group == 0) != (g.caseID == nil) {
			t.Errorf("通報 %d 的案件：%v，預期%s案件", n, str(g.caseID), map[bool]string{true: "沒有", false: "有"}[w.group == 0])
			continue
		}
		if w.group == 0 {
			continue
		}
		if n == w.group {
			caseIDs[n] = *g.caseID
		}
	}
	for n := 1; n <= 15; n++ {
		if w := want[n]; w.group != 0 && n != w.group && str(got[id(n)].caseID) != caseIDs[w.group] {
			t.Errorf("通報 %d 要和通報 %d 在同一個案件：%s 對 %s", n, w.group, str(got[id(n)].caseID), caseIDs[w.group])
		}
	}
	distinct := map[string]bool{}
	for _, c := range caseIDs {
		distinct[c] = true
	}
	if len(distinct) != 6 {
		t.Fatalf("案件應該是 6 個（三個進行中、三個已結案），不同的案件 ID 有 %d 個：%v", len(distinct), caseIDs)
	}
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.cases"); n != 6 {
		t.Errorf("案件總數 %d，預期 6", n)
	}
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.cases WHERE closed_at IS NULL"); n != 3 {
		t.Errorf("進行中的案件 %d 個，預期 3", n)
	}

	type wantReferral struct {
		channel, reference string
		hour               int
		admin              int64
	}
	type wantCase struct {
		store              int64
		category, severity string
		openedHour         int
		openedBy           int64
		closedHour         int
		closedBy           int64
		decision, note     string
		updatedHour        int
		referral           *wantReferral
	}
	wantCases := map[int]wantCase{
		// 嚴重度取最嚴重的 HIGH、開案取最早的確認收件（第 10 小時，id 小的 2 的管理員 12）、updated_at 取成員最晚的（第 30 小時）。
		2:  {store: A, category: "FRAUD", severity: "HIGH", openedHour: 10, openedBy: 12, updatedHour: 30},
		7:  {store: A, category: "FRAUD", severity: "CRITICAL", openedHour: 5, openedBy: 15, closedHour: 40, closedBy: 21, decision: "ACTION_TAKEN", note: "v1", updatedHour: 40},
		8:  {store: A, category: "FRAUD", severity: "LOW", openedHour: 6, openedBy: 16, closedHour: 41, closedBy: 22, decision: "ACTION_TAKEN", updatedHour: 41},
		11: {store: A, category: "FOOD_SAFETY", severity: "HIGH", openedHour: 8, openedBy: 18, updatedHour: 50, referral: &wantReferral{"HOTLINE_1919", "F-1", 9, 31}},
		13: {store: A, category: "FOOD_SAFETY", severity: "MEDIUM", openedHour: 3, openedBy: 20, closedHour: 44, closedBy: 25, decision: "ACTION_TAKEN", note: "v3", updatedHour: 44, referral: &wantReferral{"ONLINE_FORM", "F-2", 4, 32}},
		14: {store: B, category: "FRAUD", severity: "MEDIUM", openedHour: 12, openedBy: 26, updatedHour: 12},
	}
	for rep, w := range wantCases {
		var (
			store                               int64
			category, severity                  string
			openedAt, updatedAt                 time.Time
			openedBy, version                   int64
			closedAt                            *time.Time
			closedBy                            *int64
			closeReason, decision, decisionNote *string
		)
		must(t, b.d.Pool().QueryRow(ctx, `SELECT s.store_code, c.category, c.severity, c.opened_at, c.opened_by_admin_id, c.closed_at, c.closed_by_admin_id, c.close_reason, c.decision, c.decision_note, c.updated_at, c.version
			FROM seller.reports r JOIN seller.cases c ON c.id = r.case_id JOIN seller.stores s ON s.store_id = c.store_id WHERE r.id = $1::uuid`, id(rep)).
			Scan(&store, &category, &severity, &openedAt, &openedBy, &closedAt, &closedBy, &closeReason, &decision, &decisionNote, &updatedAt, &version))
		if store != w.store || category != w.category || severity != w.severity || hourOf(&openedAt) != w.openedHour || openedBy != w.openedBy ||
			hourOf(closedAt) != w.closedHour || num(closedBy) != w.closedBy || str(decision) != w.decision || str(decisionNote) != w.note ||
			hourOf(&updatedAt) != w.updatedHour || version != 1 {
			t.Errorf("通報 %d 的案件：store=%d %s/%s opened=%d/%d closed=%d/%d decision=%q note=%q updated=%d version=%d，預期 %+v",
				rep, store, category, severity, hourOf(&openedAt), openedBy, hourOf(closedAt), num(closedBy), str(decision), str(decisionNote), hourOf(&updatedAt), version, w)
		}
		if (w.closedHour != 0) != (closeReason != nil) || (closeReason != nil && *closeReason != "DECIDED") {
			t.Errorf("通報 %d 的案件結案原因：%q，已結案的是 DECIDED、進行中的是 NULL", rep, str(closeReason))
		}
		var (
			channel, reference string
			referredAt         time.Time
			admin              int64
		)
		err := b.d.Pool().QueryRow(ctx, `SELECT f.channel, f.reference_no, f.referred_at, f.admin_id
			FROM seller.reports r JOIN seller.case_referrals f ON f.case_id = r.case_id WHERE r.id = $1::uuid`, id(rep)).Scan(&channel, &reference, &referredAt, &admin)
		switch {
		case w.referral == nil && err == nil:
			t.Errorf("通報 %d 的案件不該有轉報：%s/%s", rep, channel, reference)
		case w.referral == nil:
			// 沒有轉報（no rows）。
		case err != nil:
			t.Errorf("通報 %d 的案件要有轉報 %+v：%v", rep, *w.referral, err)
		case channel != w.referral.channel || reference != w.referral.reference || hourOf(&referredAt) != w.referral.hour || admin != w.referral.admin:
			t.Errorf("通報 %d 的轉報：%s/%s %d/%d，預期 %+v", rep, channel, reference, hourOf(&referredAt), admin, *w.referral)
		}
	}
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.case_referrals"); n != 2 {
		t.Errorf("轉報 %d 筆，預期 2（11 與 13 各一）", n)
	}

	// 轉換後的資料符合模型的不變量：進行中的案件至少有一筆通報，決定採取措施的案件至少有一筆「已採取措施」的通報（I1、I4 的資料面）。
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.cases c WHERE NOT EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id)"); n != 0 {
		t.Errorf("%d 個案件沒有任何通報", n)
	}
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.cases c WHERE c.decision = 'ACTION_TAKEN' AND NOT EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id AND r.outcome = 'ACTION_TAKEN')"); n != 0 {
		t.Errorf("%d 個決定採取措施的案件沒有「已採取措施」的通報", n)
	}
	// 既有措施的 source_case_id 是 NULL；配合要求與佐證不動（supplement_id 是 NULL）；舊欄位消失。
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.measures WHERE source_case_id IS NULL"); n != 1 {
		t.Errorf("既有措施的 source_case_id 應該是 NULL：%d 筆，預期 1", n)
	}
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.report_cooperation_requests"); n != 2 {
		t.Errorf("配合要求 %d 筆，預期 2（轉換不動它）", n)
	}
	if n := countOf(t, b.d, "SELECT count(*) FROM seller.report_evidence WHERE supplement_id IS NULL"); n != 1 {
		t.Errorf("佐證 %d 筆，預期 1（supplement_id 是 NULL）", n)
	}
	if n := countOf(t, b.d, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'reports'
		AND column_name IN ('status', 'severity', 'acknowledged_at', 'acknowledged_by_admin_id', 'investigating_at', 'fda_reported_at', 'fda_channel', 'fda_reference_no', 'fda_reported_by_admin_id')`); n != 0 {
		t.Errorf("通報還留著 %d 個舊欄位", n)
	}
}

// 沒有誠實的轉換就不寫（W2 先例）：R-a 到 R-d 各一個反例，migration 整個失敗並列出每一個該處理的通報 ID（不列沒問題的），什麼都不改；
// 人工處理掉那些列之後可以重跑。
func TestReportCasesMigrationRefusesRowsWithoutAnHonestCase(t *testing.T) {
	t.Parallel()
	const A, B = reportCasesStoreA, reportCasesStoreB
	id := func(n int) string { return fmt.Sprintf("0199dddd-0000-7000-8000-%012d", n) }
	fda := func(r legacyReport) legacyReport {
		r.FDA, r.FDAHour, r.FDABy = "HOTLINE_1919/F-1", 9, 31
		return r
	}
	for name, tt := range map[string]struct {
		rows    []legacyReport
		coop    []int // 掛配合要求的通報
		refused []int // 必須被列出的通報（依 id 排序）
		spared  []int // 不能被列出的通報
	}{
		"R-a 查證屬實而類別是 OTHER": {
			rows: []legacyReport{
				{ID: id(1), Store: A, Category: "OTHER", Status: "VERIFIED", Severity: "LOW", AckHour: 3, AckBy: 7, ResolvedHour: 9, ResolvedBy: 8},
				{ID: id(2), Store: A, Category: "FRAUD", Status: "VERIFIED", Severity: "LOW", AckHour: 3, AckBy: 7, ResolvedHour: 9, ResolvedBy: 8},
				{ID: id(3), Store: A, Category: "OTHER", Status: "DISMISSED", ResolvedHour: 9, ResolvedBy: 8},            // OTHER 的不成立：只留結果
				{ID: id(4), Store: A, Category: "OTHER", Status: "PENDING", Severity: "LOW", AckHour: 3, AckBy: 7},       // OTHER 的已確認收件：退回待分案
				{ID: id(5), Store: A, Category: "OTHER", Status: "INVESTIGATING", Severity: "LOW", AckHour: 3, AckBy: 7}, // OTHER 的調查中：退回待分案
			},
			refused: []int{1},
			spared:  []int{2, 3, 4, 5},
		},
		"R-b 有 FDA 轉報卻不會進任何案件": {
			rows: []legacyReport{
				fda(legacyReport{ID: id(1), Store: A, Category: "FOOD_SAFETY", Status: "DISMISSED", ResolvedHour: 9, ResolvedBy: 8}),
				fda(legacyReport{ID: id(2), Store: A, Category: "FOOD_SAFETY", Status: "PENDING"}),
				fda(legacyReport{ID: id(3), Store: B, Category: "FOOD_SAFETY", Status: "PENDING", Severity: "LOW", AckHour: 3, AckBy: 7}), // 已確認收件：進案件
				{ID: id(4), Store: A, Category: "FOOD_SAFETY", Status: "DISMISSED", ResolvedHour: 9, ResolvedBy: 8},                       // 沒有轉報
			},
			refused: []int{1, 2},
			spared:  []int{3, 4},
		},
		"R-c 合併進同一個進行中案件的通報有兩筆以上 FDA 轉報": {
			rows: []legacyReport{
				fda(legacyReport{ID: id(1), Store: A, Category: "FOOD_SAFETY", Status: "INVESTIGATING", Severity: "LOW", AckHour: 3, AckBy: 7}),
				fda(legacyReport{ID: id(2), Store: A, Category: "FOOD_SAFETY", Status: "PENDING", Severity: "LOW", AckHour: 4, AckBy: 7}),
				fda(legacyReport{ID: id(3), Store: B, Category: "FOOD_SAFETY", Status: "PENDING", Severity: "LOW", AckHour: 3, AckBy: 7}),                                  // 另一家店：各自的案件
				fda(legacyReport{ID: id(4), Store: A, Category: "FOOD_SAFETY", Status: "VERIFIED", Severity: "LOW", AckHour: 3, AckBy: 7, ResolvedHour: 9, ResolvedBy: 8}), // 查證屬實：自己的已結案案件，不合併
				{ID: id(5), Store: A, Category: "FOOD_SAFETY", Status: "PENDING", Severity: "LOW", AckHour: 5, AckBy: 7},                                                   // 合併進同一個案件但沒有轉報
			},
			refused: []int{1, 2},
			spared:  []int{3, 4, 5},
		},
		"R-d 配合要求掛在不會進任何案件的通報上": {
			rows: []legacyReport{
				{ID: id(1), Store: A, Category: "FRAUD", Status: "DISMISSED", ResolvedHour: 9, ResolvedBy: 8},
				{ID: id(2), Store: A, Category: "FRAUD", Status: "PENDING"},
				{ID: id(3), Store: A, Category: "OTHER", Status: "PENDING", Severity: "LOW", AckHour: 3, AckBy: 7},
				{ID: id(4), Store: A, Category: "OTHER", Status: "INVESTIGATING", Severity: "LOW", AckHour: 3, AckBy: 7},
				{ID: id(5), Store: A, Category: "FRAUD", Status: "PENDING", Severity: "LOW", AckHour: 3, AckBy: 7},                                  // 進案件
				{ID: id(6), Store: A, Category: "FRAUD", Status: "VERIFIED", Severity: "LOW", AckHour: 3, AckBy: 7, ResolvedHour: 9, ResolvedBy: 8}, // 進已結案的案件
				{ID: id(7), Store: A, Category: "FRAUD", Status: "DISMISSED", ResolvedHour: 9, ResolvedBy: 8},                                       // 不成立但沒有配合要求
			},
			coop:    []int{1, 2, 3, 4, 5, 6},
			refused: []int{1, 2, 3, 4},
			spared:  []int{5, 6, 7},
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := beforeReportCases(t)
			for _, r := range tt.rows {
				b.insert(r)
			}
			for _, n := range tt.coop {
				must(t, b.exec(`INSERT INTO seller.report_cooperation_requests (report_id, template, requested_by_admin_id, requested_at) VALUES ($1::uuid, 'ASSIST_INVESTIGATION', 5, now())`, id(n)))
			}
			ids := func(ns []int) []string {
				out := make([]string, len(ns))
				for i, n := range ns {
					out[i] = id(n)
				}
				return out
			}

			err := b.apply()
			if err == nil {
				t.Fatal("有沒有誠實對應的列時 migration 必須失敗")
			}
			if listed := strings.Join(ids(tt.refused), ", "); !strings.Contains(err.Error(), listed) {
				t.Errorf("錯誤訊息要依序列出該處理的通報 %s：%v", listed, err)
			}
			for _, spared := range ids(tt.spared) {
				if strings.Contains(err.Error(), spared) {
					t.Errorf("沒問題的通報 %s 不該被列出：%v", spared, err)
				}
			}
			// 失敗的 migration 什麼都不改：新表沒有建起來、舊欄位還在、通報數不變。
			if n := countOf(t, b.d, "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'seller' AND c.relname IN ('cases', 'case_referrals', 'report_supplements', 'report_case_moves')"); n != 0 {
				t.Errorf("失敗的 migration 留下了 %d 張新表", n)
			}
			if n := countOf(t, b.d, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'reports' AND column_name IN ('status', 'severity', 'case_id', 'outcome')"); n != 2 {
				t.Errorf("失敗的 migration 改了通報的欄位：status、severity 該還在、case_id、outcome 該還沒有（找到 %d 個）", n)
			}
			if n := countOf(t, b.d, "SELECT count(*) FROM seller.reports"); n != int64(len(tt.rows)) {
				t.Errorf("通報 %d 筆，預期 %d", n, len(tt.rows))
			}

			// 人工處理掉被列出的通報（連同它們的配合要求）之後可以重跑。
			must(t, b.exec("DELETE FROM seller.report_cooperation_requests WHERE report_id = ANY($1::uuid[])", ids(tt.refused)))
			must(t, b.exec("DELETE FROM seller.reports WHERE id = ANY($1::uuid[])", ids(tt.refused)))
			if err := b.apply(); err != nil {
				t.Fatalf("處理掉被列出的通報後 migration 應該成功：%v", err)
			}
			if n := countOf(t, b.d, "SELECT count(*) FROM seller.reports"); n != int64(len(tt.rows)-len(tt.refused)) {
				t.Errorf("成功之後通報 %d 筆，預期 %d", n, len(tt.rows)-len(tt.refused))
			}
		})
	}
}

// 佐證改以「第幾次送出」編號的 migration（設計審查 #7）：舊的佐證位置在整筆通報內連號（補充的接在原通報後面，每次送出的張數不一定是 5 的倍數），
// 新的鍵是 (通報, 第幾次送出, 次內位置)。補充依建立時間編號 1..n；佐證在各自那次送出內從 0 重新編號；讀取紀錄跟著換成新的鍵，仍指向同一張佐證；
// 請求（申覆）的佐證讀取紀錄不動。
func TestSubmissionSeqMigrationRenumbersEvidenceWithinEachSubmission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_report_submission_seq.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_report_submission_seq 的 migration：%v", files)
	}
	d := pg.NewDB(t)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	for _, name := range files[:at] {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		exec(string(raw))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000803, 953, 'x', now(), now())")
	const report, other = "0199dddd-0000-7000-8000-000000000001", "0199dddd-0000-7000-8000-000000000002"
	supplement := func(n int) string { return fmt.Sprintf("0199dddd-0000-7000-8000-0000000001%02d", n) }
	for i, id := range []string{report, other} {
		exec(`INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at)
			SELECT $1::uuid, store_id, 'FRAUD', 'x', $2::bigint, $3::timestamptz, $3::timestamptz FROM seller.stores WHERE store_code = 2600000803`, id, 2000+i, reportCasesTime(1))
	}
	// 補充的建立時間故意和 id 的順序相反：編號依建立時間。
	for n, hour := range map[int]int{3: 2, 2: 3, 1: 4} {
		exec(`INSERT INTO seller.report_supplements (id, report_id, description, created_at) VALUES ($1::uuid, $2::uuid, 'x', $3::timestamptz)`, supplement(n), report, reportCasesTime(hour))
	}
	exec(`INSERT INTO seller.report_supplements (id, report_id, description, created_at) VALUES ($1::uuid, $2::uuid, 'x', $3::timestamptz)`, supplement(9), other, reportCasesTime(2))
	// 原通報 2 張（位置 0、1）；最早的補充（id 3）3 張（2、3、4）；第二則（id 2）沒有佐證；最晚的（id 1）1 張（5）。
	evidence := func(reportID string, position int, supplementID *string) {
		exec(`INSERT INTO seller.report_evidence (report_id, position, supplement_id, asset_id, object_key) VALUES ($1::uuid, $2, $3::uuid, $4, 'k')`,
			reportID, position, supplementID, fmt.Sprintf("%s-%d", reportID[len(reportID)-2:], position))
	}
	s3, s1, s9 := supplement(3), supplement(1), supplement(9)
	evidence(report, 0, nil)
	evidence(report, 1, nil)
	for _, p := range []int{2, 3, 4} {
		evidence(report, p, &s3)
	}
	evidence(report, 5, &s1)
	evidence(other, 0, nil)
	evidence(other, 1, &s9)
	// 讀取紀錄：通報的第 3 張（補充的第 2 張）與第 5 張，以及另一筆通報的第 1 張。
	access := func(n int) string { return fmt.Sprintf("0199dddd-0000-7000-8000-0000000002%02d", n) }
	for n, a := range []struct {
		reportID string
		position int
	}{{report, 3}, {report, 5}, {other, 1}} {
		exec(`INSERT INTO seller.evidence_accesses (access_id, report_id, position, admin_id, accessed_at) VALUES ($1::uuid, $2::uuid, $3, 9, now())`, access(n+1), a.reportID, a.position)
	}

	raw, err := fs.ReadFile(Migrations(), files[at])
	must(t, err)
	exec(string(raw))

	type key struct {
		report string
		seq    int
		pos    int
	}
	rows, err := d.Pool().Query(ctx, `SELECT report_id::text, submission_seq, position, asset_id, coalesce(supplement_id::text, '') FROM seller.report_evidence`)
	must(t, err)
	got := map[key]string{}
	for rows.Next() {
		var k key
		var asset, supp string
		must(t, rows.Scan(&k.report, &k.seq, &k.pos, &asset, &supp))
		got[k] = asset + "|" + supp
	}
	must(t, rows.Err())
	rows.Close()
	want := map[key]string{
		{report, 0, 0}: "01-0|", {report, 0, 1}: "01-1|",
		{report, 1, 0}: "01-2|" + s3, {report, 1, 1}: "01-3|" + s3, {report, 1, 2}: "01-4|" + s3,
		{report, 3, 0}: "01-5|" + s1,
		{other, 0, 0}:  "02-0|", {other, 1, 0}: "02-1|" + s9,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("佐證 %v，預期 %v", got, want)
	}
	// 補充依建立時間編號：id 3（最早）是 1，id 2 是 2，id 1 是 3。
	for n, seq := range map[int]int{3: 1, 2: 2, 1: 3, 9: 1} {
		if c := countOf(t, d, "SELECT count(*) FROM seller.report_supplements WHERE id = $1::uuid AND seq = $2", supplement(n), seq); c != 1 {
			t.Errorf("補充 %d 的 seq 不是 %d", n, seq)
		}
	}
	// 讀取紀錄跟著換鍵、仍指向同一張佐證（asset 相同）。
	for n, want := range map[int]string{1: "01-3", 2: "01-5", 3: "02-1"} {
		var asset string
		must(t, d.Pool().QueryRow(ctx, `SELECT e.asset_id FROM seller.evidence_accesses a JOIN seller.report_evidence e
			ON e.report_id = a.report_id AND e.submission_seq = a.submission_seq AND e.position = a.position
			WHERE a.access_id = $1::uuid`, access(n)).Scan(&asset))
		if asset != want {
			t.Errorf("讀取紀錄 %d 指向 %s，預期 %s", n, asset, want)
		}
	}
}

// 「已看過的補充」標記從時間改成編號的 migration：舊標記之前（含同一時間）建立的補充中最大的編號就是新標記；從沒記過的是 0；
// 舊標記晚於全部補充的是最後一則的編號；沒有補充的是 0。舊欄位移除，新欄位的範圍由 CHECK 擋住（0..10）。
func TestSupplementsSeenSeqMigrationConvertsTheTimeMarkerToTheLatestSeenSupplement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_supplements_seen_seq.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_supplements_seen_seq 的 migration：%v", files)
	}
	d := pg.NewDB(t)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	for _, name := range files[:at] {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		exec(string(raw))
	}
	exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000804, 954, 'x', now(), now())")
	report := func(n int) string { return fmt.Sprintf("0199eeee-0000-7000-8000-0000000000%02d", n) }
	supplement := func(r, n int) string { return fmt.Sprintf("0199eeee-0000-7000-8000-00000001%02d%02d", r, n) }
	// r1：標記落在第 2 則與第 3 則之間；r2：標記與第 2 則同一時間；r3：從沒記過；r4：標記晚於全部；r5：沒有補充但有標記。
	seen := map[int]*time.Time{}
	at2 := reportCasesTime(3)
	late := reportCasesTime(9)
	seen[1], seen[2], seen[4], seen[5] = &at2, &at2, &late, &late
	between := reportCasesTime(3).Add(30 * time.Minute)
	seen[1] = &between
	count := map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 0}
	for r := 1; r <= 5; r++ {
		exec(`INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, supplements_seen_at, created_at, updated_at)
			SELECT $1::uuid, store_id, 'FRAUD', 'x', $2::bigint, $3::timestamptz, $4::timestamptz, $4::timestamptz FROM seller.stores WHERE store_code = 2600000804`,
			report(r), 3000+r, seen[r], reportCasesTime(1))
		for n := 1; n <= count[r]; n++ {
			exec(`INSERT INTO seller.report_supplements (id, report_id, description, created_at, seq) VALUES ($1::uuid, $2::uuid, 'x', $3::timestamptz, $4)`,
				supplement(r, n), report(r), reportCasesTime(1+n), n)
		}
	}
	raw, err := fs.ReadFile(Migrations(), files[at])
	must(t, err)
	exec(string(raw))
	for r, want := range map[int]int{1: 2, 2: 2, 3: 0, 4: 3, 5: 0} {
		var got int
		must(t, d.Pool().QueryRow(ctx, "SELECT supplements_seen_seq FROM seller.reports WHERE id = $1::uuid", report(r)).Scan(&got))
		if got != want {
			t.Errorf("通報 %d 的 supplements_seen_seq=%d，預期 %d", r, got, want)
		}
	}
	if n := countOf(t, d, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'reports' AND column_name = 'supplements_seen_at'"); n != 0 {
		t.Error("舊的 supplements_seen_at 要移除")
	}
	if _, err := d.Pool().Exec(ctx, "UPDATE seller.reports SET supplements_seen_seq = 11"); err == nil {
		t.Error("supplements_seen_seq 超過 10 要被 CHECK 擋下")
	}
}

// 配合要求（report_cooperation_requests）的退場（3c-b，B22）：舊的配合要求沒有給賣家的說明與期限，不能誠實地轉成詢問，所以有列時 migration 失敗並列出通報 ID
// （表與列都不動），沒有列才刪表。
func TestCooperationRequestsMigrationRefusesExistingRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_drop_cooperation_requests.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_drop_cooperation_requests 的 migration：%v", files)
	}
	exec := func(sql string, args ...any) error {
		_, err := d.Pool().Exec(ctx, sql, args...)
		return err
	}
	body := func(name string) string {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		return string(raw)
	}
	for _, name := range files[:at] {
		must(t, exec(body(name)))
	}
	const first, second = "0199dddd-0000-7000-8000-000000000001", "0199dddd-0000-7000-8000-000000000002"
	must(t, exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000901, 961, 'x', now(), now())"))
	for i, id := range []string{first, second} {
		must(t, exec(`INSERT INTO seller.reports (id, store_id, category, description, reporter_buyer_id, created_at, updated_at)
			SELECT $1::uuid, store_id, 'FRAUD', 'x', $2::bigint, now(), now() FROM seller.stores WHERE store_code = 2600000901`, id, 4000+i))
	}
	must(t, exec(`INSERT INTO seller.report_cooperation_requests (report_id, template, requested_by_admin_id, requested_at)
		VALUES ($1::uuid, 'ASSIST_INVESTIGATION', 5, now()), ($1::uuid, 'TEMPORARY_TAKEDOWN_SELF_CHECK', 5, now()), ($2::uuid, 'ASSIST_INVESTIGATION', 5, now())`, first, second))

	err = exec(body(files[at]))
	if err == nil || !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), second) || strings.Count(err.Error(), first) != 1 {
		t.Fatalf("有列時要失敗並列出每個通報 ID 一次：%v", err)
	}
	if n := countOf(t, d, "SELECT count(*) FROM seller.report_cooperation_requests"); n != 3 {
		t.Fatalf("失敗的 migration 不能動資料：%d 列", n)
	}
	must(t, exec("DELETE FROM seller.report_cooperation_requests"))
	must(t, exec(body(files[at])))
	if n := countOf(t, d, "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'seller' AND c.relname = 'report_cooperation_requests'"); n != 0 {
		t.Fatal("沒有列時要刪表")
	}
}

// 轉報 v2（3c-b-2）：既有的轉報列一律成為已轉報（REFERRED），追蹤日期是 NULL，欄名改成 recorded_at、recorded_by_admin_id，
// 之後可以記回覆；不轉報的列要有原因、沒有管道。
func TestCaseReferralV2KeepsExistingReferralsAsReferred(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	at := slices.IndexFunc(files, func(name string) bool { return strings.HasSuffix(name, "_seller_case_referral_v2.sql") })
	if at < 1 {
		t.Fatalf("找不到 seller_case_referral_v2 的 migration：%v", files)
	}
	d := pg.NewDB(t)
	exec := func(sql string, args ...any) error {
		_, err := d.Pool().Exec(ctx, sql, args...)
		return err
	}
	for _, name := range files[:at] {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		must(t, exec(string(raw)))
	}
	must(t, exec("INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES (2600000950, 971, 'x', now(), now())"))
	const first, second = "0199ffff-0000-7000-8000-000000000001", "0199ffff-0000-7000-8000-000000000002"
	for _, id := range []string{first, second} {
		must(t, exec(`INSERT INTO seller.cases (id, store_id, category, severity, opened_at, opened_by_admin_id, updated_at)
			SELECT $1::uuid, store_id, 'FOOD_SAFETY', 'LOW', now() - interval '2 days', 1, now() - interval '2 days' FROM seller.stores WHERE store_code = 2600000950`, id))
		// 同一家店同類別只有一個進行中的案件：第一個先結案。
		if id == first {
			must(t, exec(`UPDATE seller.cases SET closed_at = now() - interval '1 day', closed_by_admin_id = 1, close_reason = 'NO_REPORTS' WHERE id = $1::uuid`, id))
		}
	}
	must(t, exec(`INSERT INTO seller.case_referrals (case_id, category, channel, reference_no, referred_at, admin_id) VALUES ($1::uuid, 'FOOD_SAFETY', 'HOTLINE_1919', 'F-1', '2026-10-02 03:04:05+00', 9)`, first))
	must(t, exec(string(mustRead(t, files[at]))))
	var outcome, channel, ref string
	var followUp *time.Time
	var admin int64
	var recorded time.Time
	must(t, d.Pool().QueryRow(ctx, "SELECT outcome, channel, reference_no, follow_up_on, recorded_by_admin_id, recorded_at FROM seller.case_referrals WHERE case_id = $1::uuid", first).
		Scan(&outcome, &channel, &ref, &followUp, &admin, &recorded))
	if outcome != "REFERRED" || channel != "HOTLINE_1919" || ref != "F-1" || followUp != nil || admin != 9 || !recorded.Equal(time.Date(2026, 10, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("既有的轉報：%s %s %s %v %d %v", outcome, channel, ref, followUp, admin, recorded)
	}
	must(t, exec("INSERT INTO seller.case_referral_replies (id, case_id, body, received_on, recorded_by_admin_id, recorded_at) VALUES (uuidv7(), $1::uuid, 'x', current_date, 1, now())", first))
	if err := exec("INSERT INTO seller.case_referral_replies (id, case_id, body, received_on, recorded_by_admin_id, recorded_at) VALUES (uuidv7(), $1::uuid, 'x', current_date, 1, now())", second); err == nil {
		t.Error("沒有轉報紀錄的案件不能有回覆")
	}
	must(t, exec(`INSERT INTO seller.case_referrals (case_id, category, outcome, waiver_reason, recorded_at, recorded_by_admin_id) VALUES ($1::uuid, 'FOOD_SAFETY', 'WAIVED', '主管機關已經介入', now(), 1)`, second))
	if err := exec("INSERT INTO seller.case_referral_replies (id, case_id, body, received_on, recorded_by_admin_id, recorded_at) VALUES (uuidv7(), $1::uuid, 'x', current_date, 1, now())", second); err == nil {
		t.Error("不轉報的案件不能有回覆")
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := fs.ReadFile(Migrations(), name)
	must(t, err)
	return raw
}
